package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jonboulle/clockwork"
	"go.saser.se/postgres"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog/v2"
)

// maxPageSize is the maximum number of tasks the server will return on a call
// to ListTasks. Any request for more than maxPageSize tasks will only return
// (at most) maxPageSize tasks.
const maxPageSize = 1000

// internalError should be returned whenever something goes wrong with serving a
// request, and where the error cannot be attributed to the user making an
// invalid request, something cannot be found, etc.
var internalError = status.Error(codes.Internal, "Something went wrong.")

// taskUpdatableMask contains the fields that can be updated by UpdateTask. It must
// be kept in sync with the proto definition.
var taskUpdatableMask *fieldmaskpb.FieldMask

func init() {
	m, err := fieldmaskpb.New(&pb.Task{},
		"title",
		"description",
	)
	if err != nil {
		klog.Exit(err)
	}
	taskUpdatableMask = m
}

// projectUpdatableMask contains the fields that can be updated by UpdateProject. It must
// be kept in sync with the proto definition.
var projectUpdatableMask *fieldmaskpb.FieldMask

func init() {
	m, err := fieldmaskpb.New(&pb.Task{},
		"title",
		"description",
	)
	if err != nil {
		klog.Exit(err)
	}
	projectUpdatableMask = m
}

type Service struct {
	pb.UnimplementedTasksServer

	pool *postgres.Pool

	// Only used for testing. Nil otherwise.
	clock clockwork.FakeClock
}

func New(pool *postgres.Pool) *Service {
	return &Service{
		pool: pool,
	}
}

func (s *Service) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if !strings.HasPrefix(name, "tasks/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	resourceID := strings.TrimPrefix(name, "tasks/")
	if resourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task does not contain a resource ID after "tasks/".`)
	}
	id, err := strconv.ParseInt(resourceID, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	var (
		task *pb.Task
		now  time.Time
	)
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		var err error
		now, err = s.now(ctx, tx)
		if err != nil {
			return err
		}
		t, err := queryTaskByID(ctx, tx, id, true /*showDeleted*/)
		if err != nil {
			return err
		}
		task = t
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	if expire := task.GetExpireTime(); expire.IsValid() && now.After(expire.AsTime()) {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	return task, nil
}

func (s *Service) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
	pageSize := req.GetPageSize()
	if pageSize < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "The page size must not be negative; was %d.", pageSize)
	}
	if pageSize == 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	if token := req.GetPageToken(); token != "" {
		if _, err := uuid.Parse(token); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", req.GetPageToken())
		}
	}

	res := &pb.ListTasksResponse{}
	errNoToken := errors.New("page token given but not found")
	errChangedRequest := errors.New("request changed between pages")
	txFunc := func(tx pgx.Tx) error {
		now, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		// First find out what the minimum ID to use in this page is. If this is
		// the first page, it will be 0. If it is not, then it will be a value
		// stored in the `task_page_tokens` database table, and the `page_token`
		// field in the request contains the key to that table.
		minID := int64(0)
		showDeleted := req.GetShowDeleted()
		if token := req.GetPageToken(); token != "" {
			// We could do a SELECT and then a DELETE, but since Postgres
			// supports the RETURNING clause, we can do it in just one
			// statement. Neat!
			sql, args, err := postgres.StatementBuilder.
				Delete("task_page_tokens").
				Where(squirrel.Eq{
					"token": token,
				}).
				Suffix("RETURNING minimum_id, show_deleted").
				ToSql()
			if err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, sql, args...).Scan(&minID, &showDeleted); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return errNoToken
				}
				return err
			}
			if req.GetShowDeleted() != showDeleted {
				return errChangedRequest
			}
		}

		// Now that we know the minimum ID, we can run a SELECT to list tasks.
		// We set a limit of pageSize+1 so that we may get the first task in the
		// next page (if any). This allows us to do one query that gives us
		//     1. if there is a next page, and if so,
		//     2. what the minimum ID will be for that page.
		var (
			// The eventual list of tasks to return.
			tasks []*pb.Task
			// The columns in the row.
			id                                 int64
			title                              string
			description                        string
			completeTime                       pgtype.Timestamptz
			createTime                         time.Time
			updateTime, deleteTime, expireTime pgtype.Timestamptz
			// To use for the next page, if any.
			nextMinID int64
		)
		st := postgres.StatementBuilder.
			Select(
				"id",
				"title",
				"description",
				"complete_time",
				"create_time",
				"update_time",
				"delete_time",
				"expire_time",
			).
			From("tasks").
			Where(squirrel.GtOrEq{
				"id": minID,
			})
		if !showDeleted {
			st = st.Where(squirrel.Eq{
				"delete_time": nil,
			})
		} else {
			st = st.Where(squirrel.Or{
				squirrel.Eq{
					"expire_time": nil,
				},
				squirrel.Gt{
					"expire_time": now,
				},
			})
		}
		st = st.
			OrderBy("id ASC").
			Limit(uint64(pageSize) + 1)
		sql, args, err := st.ToSql()
		if err != nil {
			return err
		}
		// qf is called for every row returned by the above query, after
		// scanning has completed successfully.
		qf := func(qfr pgx.QueryFuncRow) error {
			if id > nextMinID {
				nextMinID = id
			}
			task := &pb.Task{
				Name:        "tasks/" + fmt.Sprint(id),
				Title:       title,
				Description: description,
				CreateTime:  timestamppb.New(createTime),
			}
			if completeTime.Status == pgtype.Present {
				task.CompleteTime = timestamppb.New(completeTime.Time)
			}
			if updateTime.Status == pgtype.Present {
				task.UpdateTime = timestamppb.New(updateTime.Time)
			}
			if deleteTime.Status == pgtype.Present {
				task.DeleteTime = timestamppb.New(deleteTime.Time)
			}
			if expireTime.Status == pgtype.Present {
				task.ExpireTime = timestamppb.New(expireTime.Time)
			}
			tasks = append(tasks, task)
			return nil
		}
		// Here is where the actual query happens.
		if _, err := tx.QueryFunc(ctx, sql,
			args,
			[]interface{}{
				&id,
				&title,
				&description,
				&completeTime,
				&createTime,
				&updateTime,
				&deleteTime,
				&expireTime,
			},
			qf,
		); err != nil {
			return err
		}

		// If the number of tasks from the above query is less than or equal to
		// pageSize, we know that there will be no more pages We can then do an
		// early return.
		if int32(len(tasks)) <= pageSize {
			res.Tasks = tasks
			return nil
		}

		// We know at this point that there will be at least one more page, so
		// we limit the tasks in this page to the pageSize and then create the
		// token for the next page.
		res.Tasks = tasks[:pageSize]
		token := uuid.New()
		res.NextPageToken = token.String()
		sql, args, err = postgres.StatementBuilder.
			Insert("task_page_tokens").
			Columns("token", "minimum_id", "show_deleted").
			Values(token, nextMinID, showDeleted).
			ToSql()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return err
		}
		return nil
	}
	if err := s.pool.BeginFunc(ctx, txFunc); err != nil {
		if errors.Is(err, errNoToken) || errors.Is(err, errChangedRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", req.GetPageToken())
		}
		klog.Error(err)
		return nil, internalError
	}
	return res, nil
}

func (s *Service) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.Task, error) {
	task := req.GetTask()
	if task.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "The task must have a title.")
	}
	if task.GetCompleteTime().IsValid() {
		return nil, status.Error(codes.InvalidArgument, "The task must not already be completed.")
	}
	parent := task.GetParent()
	parentID := int64(-1)
	if parent != "" {
		if !strings.HasPrefix(parent, "tasks/") {
			return nil, status.Errorf(codes.InvalidArgument, `The parent field must have the format "tasks/{task}": %q`, parent)
		}
		id, err := strconv.ParseInt(strings.TrimPrefix(parent, "tasks/"), 10, 64)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "A parent task with name %q does not exist.", parent)
		}
		parentID = id
	}
	errParentNotFound := errors.New("parent not found")
	// This constraint name should be taken from the schema file.
	const parentReferencesID = "parent_references_id"
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		now, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		set := map[string]interface{}{
			"title":       task.GetTitle(),
			"description": task.GetDescription(),
			"create_time": now,
		}
		if parentID != -1 {
			set["parent"] = parentID
		}
		sql, args, err := postgres.StatementBuilder.
			Insert("tasks").
			SetMap(set).
			Suffix("RETURNING id").
			ToSql()
		if err != nil {
			return err
		}
		var id int64
		if err := tx.QueryRow(ctx, sql, args...).Scan(
			&id,
		); err != nil {
			if e := (*pgconn.PgError)(nil); errors.As(err, &e) {
				if e.Code != pgerrcode.ForeignKeyViolation {
					return err
				}
				if e.ConstraintName != parentReferencesID {
					return err
				}
				return errParentNotFound
			}
			return err
		}
		task.Name = "tasks/" + fmt.Sprint(id)
		task.CreateTime = timestamppb.New(now)
		return nil
	}); err != nil {
		if errors.Is(err, errParentNotFound) {
			return nil, status.Errorf(codes.NotFound, "A parent task with name %q does not exist.", parent)
		}
		klog.Error(err)
		return nil, internalError
	}
	return task, nil
}

func (s *Service) UpdateTask(ctx context.Context, req *pb.UpdateTaskRequest) (*pb.Task, error) {
	// First we do stateless validation, i.e., look for errors that we can find
	// by only looking at the request message.
	patch := req.GetTask()
	name := patch.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if !strings.HasPrefix(name, "tasks/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(name, "tasks/"), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	updateMask := req.GetUpdateMask()
	if updateMask == nil {
		// This is not really necessary, but makes downstream handling easier by
		// not having to be careful about nil derefs.
		updateMask = &fieldmaskpb.FieldMask{}
	}
	// Handle two special cases:
	// 1. The update mask is nil or empty. Then it should be equivalent to
	//    updating all non-empty fields in the patch.
	// 2. The update mask contains a single path that is the wildcard ("*").
	// 	  Then it should be treated as specifying all updatable paths.
	switch paths := updateMask.GetPaths(); {
	case len(paths) == 0:
		if v := patch.GetTitle(); v != "" {
			updateMask.Paths = append(updateMask.GetPaths(), "title")
		}
		if v := patch.GetDescription(); v != "" {
			updateMask.Paths = append(updateMask.GetPaths(), "description")
		}
	case len(paths) == 1 && paths[0] == "*":
		updateMask = proto.Clone(taskUpdatableMask).(*fieldmaskpb.FieldMask)
	}
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "parent", "completed", "create_time", "name":
			return nil, status.Errorf(codes.InvalidArgument, "The field %q cannot be updated with UpdateTask.")
		case "*":
			// We handled the only valid case of giving a wildcard path above,
			// i.e., when it is the only path.
			return nil, status.Error(codes.InvalidArgument, "A wildcard can only be used if it is the single path in the update mask.")
		}
	}
	if updateMask != nil && !updateMask.IsValid(&pb.Task{}) {
		return nil, status.Error(codes.InvalidArgument, "The given update mask is invalid.")
	}
	// At this point we know that updateMask is not empty and is a valid mask.
	// The path(s) fully specify what we should get from the patch. It may still
	// be the case that the patch is empty.

	// updatedTask is the new version of the task that should eventually be
	// returned as the result of the update operation -- even if it is a no-op.
	var updatedTask *pb.Task

	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		// Eventually, we need to return either an error or the task, regardless
		// of whether it has been updated or not. So let's fetch it here, so we
		// quickly find out if it doesn't exist. If it does exist, we also get
		// all the details we eventually need to return about it.
		updatedTask, err = queryTaskByID(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}
		// Special case: the patch is empty so we should just return the current
		// version of the task which we fetched above.
		if proto.Equal(patch, &pb.Task{Name: name} /* empty patch except for the name */) {
			return nil
		}
		// Special case: the update mask is empty, meaning that the operation
		// will be a no-op even if the patch isn't empty.
		if len(updateMask.GetPaths()) == 0 {
			return nil
		}
		// Special case: the patch isn't empty and at least one path is
		// specified, but the applying the patch will yield an identical
		// resource.
		afterPatch := proto.Clone(updatedTask).(*pb.Task)
		proto.Merge(afterPatch, patch)
		if proto.Equal(afterPatch, updatedTask) {
			return nil
		}

		updateTime, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		updatedTask.UpdateTime = timestamppb.New(updateTime)

		// Update only the columns corresponding to the fields in the patch.
		q := postgres.StatementBuilder.
			Update("tasks").
			Where(squirrel.Eq{
				"id": id,
			}).
			Set("update_time", updateTime)
		for _, path := range updateMask.GetPaths() {
			switch path {
			case "title":
				v := patch.GetTitle()
				q = q.Set("title", v)
				updatedTask.Title = v
			case "description":
				v := patch.GetDescription()
				q = q.Set("description", v)
				updatedTask.Description = v
			}
		}

		sql, args, err := q.ToSql()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", patch.GetName())
		}
		klog.Error(err)
		return nil, internalError
	}

	return updatedTask, nil
}

func (s *Service) DeleteTask(ctx context.Context, req *pb.DeleteTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if !strings.HasPrefix(name, "tasks/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(name, "tasks/"), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	// deleted will eventually be returned as the updated version of the task.
	var deleted *pb.Task

	errForceRequired := errors.New("force: true is required")
	txFunc := func(tx pgx.Tx) error {
		var err error

		// We must do two things:
		//     1. Ensure that the task being deleted exists.
		//     2. Return the new version of the task when it has been deleted.
		// To kill both these birds with one stone, we get the task from the
		// database here. If it doesn't exist, we will get an error. If it does
		// exist, we will get all the details and don't need to query for them
		// later.
		deleted, err = queryTaskByID(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}

		// We also need to find out if there are any descendant tasks, and
		// return an error if there are such tasks and the request doesn't
		// contain `force: true`.
		descIDs, err := queryDescendantIDs(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}
		if len(descIDs) > 0 && !req.GetForce() {
			return errForceRequired
		}
		// As descIDs doesn't include the ID of the task being deleted, we add
		// it here.
		descIDs = append(descIDs, id)
		// Now we are ready to make updates.

		// We "delete" tasks by setting their `delete_time` and `expire_time`
		// fields. `delete_time` should be set to the current time, and
		// `expire_time` is arbitrarily chosen to be some point in the future.
		deleteTime, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		expireTime := deleteTime.AddDate(0 /* years */, 0 /* months */, 30 /* days */)

		// These new timestamps should be reflected in the returned version of
		// the task.
		deleted.DeleteTime = timestamppb.New(deleteTime)
		deleted.ExpireTime = timestamppb.New(expireTime)

		// Below is the actual update in the database. We only update and don't
		// return anything back, because we have already fetched everything
		// using taskByID above.
		sql, args, err := postgres.StatementBuilder.
			Update("tasks").
			SetMap(map[string]interface{}{
				"delete_time": deleteTime,
				"expire_time": expireTime,
			}).
			Where(squirrel.Eq{
				"id": descIDs,
			}).
			ToSql()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}
	if err := s.pool.BeginFunc(ctx, txFunc); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		if errors.Is(err, errForceRequired) {
			return nil, status.Errorf(codes.FailedPrecondition, "Task %q has children; not deleting without `force: true`.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	return deleted, nil
}

func (s *Service) UndeleteTask(ctx context.Context, req *pb.UndeleteTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if !strings.HasPrefix(name, "tasks/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(name, "tasks/"), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	var task *pb.Task
	errNotFound := errors.New("task does not exist")
	errNotDeleted := errors.New("task has not been deleted")
	errExpired := errors.New("task has expired")
	errUndeleteAncestorsRequired := errors.New("`undelete_ancestors: true` is required")
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		now, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		task, err = queryTaskByID(ctx, tx, id, true /*showDeleted*/)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errNotFound
			}
			return err
		}
		if !task.GetDeleteTime().IsValid() {
			return errNotDeleted
		}
		if now.After(task.GetExpireTime().AsTime()) {
			return errExpired
		}

		// We know the task itself is valid for undeletion. Now we want to
		// validate whether the `undelete_ancestor` argument is correct in the
		// request. We do that by fetching all ancestors -- deleted or not --
		// and then collecting the ones that are deleted. If there are at least
		// one and `undelete_ancestors` isn't set to true, we return an error to
		// the user.
		var toUndeleteIDs []int64
		ancestorIDs, err := queryAncestorIDs(ctx, tx, id, true /* showDeleted */)
		if err != nil {
			return err
		}
		for _, ancestorID := range ancestorIDs {
			ancestor, err := queryTaskByID(ctx, tx, ancestorID, true /* showDeleted */)
			if err != nil {
				return err
			}
			if ancestor.GetDeleteTime().IsValid() {
				toUndeleteIDs = append(toUndeleteIDs, ancestorID)
			}
		}
		if len(toUndeleteIDs) > 0 && !req.GetUndeleteAncestors() {
			return errUndeleteAncestorsRequired
		}
		// Now, if we should also undelete any descendants, we find their IDs
		// here.
		if req.GetUndeleteDescendants() {
			descIDs, err := queryDescendantIDs(ctx, tx, id, true /* showDeleted */)
			if err != nil {
				return err
			}
			toUndeleteIDs = append(toUndeleteIDs, descIDs...)
		}
		// Finally, we add the ID of the task itself to the list of IDs that
		// should be undeleted.
		toUndeleteIDs = append(toUndeleteIDs, id)
		sql, args, err := postgres.StatementBuilder.
			Update("tasks").
			SetMap(map[string]interface{}{
				"delete_time": nil,
				"expire_time": nil,
			}).
			Where(squirrel.Eq{
				"id": toUndeleteIDs,
			}).
			ToSql()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}); err != nil {
		if errors.Is(err, errNotFound) || errors.Is(err, errExpired) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		if errors.Is(err, errNotDeleted) {
			return nil, status.Errorf(codes.AlreadyExists, "A task with name %q already exists.", name)
		}
		if errors.Is(err, errUndeleteAncestorsRequired) {
			return nil, status.Errorf(codes.FailedPrecondition, "Task %q has deleted ancestors but `undelete_ancestors` was not set to `true`.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	task.DeleteTime = nil
	task.ExpireTime = nil
	return task, nil
}

func (s *Service) CompleteTask(ctx context.Context, req *pb.CompleteTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if !strings.HasPrefix(name, "tasks/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	resourceID := strings.TrimPrefix(name, "tasks/")
	if resourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(resourceID, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}

	var task *pb.Task
	errForceRequired := errors.New("`force: true` is required")
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		var err error
		task, err = queryTaskByID(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}
		// Special case: a completed task can be completed again, which is a
		// no-op.
		if task.GetCompleteTime().IsValid() {
			return nil
		}
		completeTime, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		descendantIDs, err := queryDescendantIDs(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}
		var toCompleteIDs []int64
		for _, descID := range descendantIDs {
			descendant, err := queryTaskByID(ctx, tx, descID, false /* showDeleted */)
			if err != nil {
				return err
			}
			if descendant.GetCompleteTime().IsValid() {
				continue
			}
			toCompleteIDs = append(toCompleteIDs, descID)
		}
		if len(toCompleteIDs) > 0 && !req.GetForce() {
			return errForceRequired
		}
		toCompleteIDs = append(toCompleteIDs, id)
		task.CompleteTime = timestamppb.New(completeTime)
		task.UpdateTime = timestamppb.New(completeTime)
		sql, args, err := postgres.StatementBuilder.
			Update("tasks").
			SetMap(map[string]interface{}{
				"complete_time": completeTime,
				"update_time":   completeTime,
			}).
			Where(squirrel.Eq{
				"id": toCompleteIDs,
			}).
			ToSql()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		if errors.Is(err, errForceRequired) {
			return nil, status.Errorf(codes.FailedPrecondition, "Task %q has uncompleted children but `force` was not set to true.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	return task, nil
}

func (s *Service) UncompleteTask(ctx context.Context, req *pb.UncompleteTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if !strings.HasPrefix(name, "tasks/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	resourceID := strings.TrimPrefix(name, "tasks/")
	if resourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(resourceID, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}

	var task *pb.Task
	errUncompleteAncestorsRequired := errors.New("`uncomplete_ancestors: true` is required")
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		var err error
		task, err = queryTaskByID(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}
		// Special case: uncompleting an uncompleted task is a no-op.
		if !task.GetCompleteTime().IsValid() {
			return nil
		}
		var toUncompleteIDs []int64
		ancestorIDs, err := queryAncestorIDs(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}
		for _, id := range ancestorIDs {
			ancestor, err := queryTaskByID(ctx, tx, id, false /* showDeleted */)
			if err != nil {
				return err
			}
			if !ancestor.GetCompleteTime().IsValid() {
				continue
			}
			toUncompleteIDs = append(toUncompleteIDs, id)
		}
		if len(toUncompleteIDs) > 0 && !req.GetUncompleteAncestors() {
			return errUncompleteAncestorsRequired
		}
		if req.GetUncompleteDescendants() {
			descendantIDs, err := queryDescendantIDs(ctx, tx, id, false /* showDeleted */)
			if err != nil {
				return err
			}
			// Assumed invariant: if the task is completed, then all its
			// descendants are also completed. Therefore we can blindly add all
			// descendant IDs here without checking whether they are actually
			// completed.
			toUncompleteIDs = append(toUncompleteIDs, descendantIDs...)
		}
		toUncompleteIDs = append(toUncompleteIDs, id)
		updateTime, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		task.CompleteTime = nil
		task.UpdateTime = timestamppb.New(updateTime)
		sql, args, err := postgres.StatementBuilder.
			Update("tasks").
			SetMap(map[string]interface{}{
				"complete_time": nil,
				"update_time":   updateTime,
			}).
			Where(squirrel.Eq{
				"id": toUncompleteIDs,
			}).
			ToSql()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		if errors.Is(err, errUncompleteAncestorsRequired) {
			return nil, status.Errorf(codes.FailedPrecondition, "Task %q has completed ancestors but `uncomplete_ancestors` was not set to true.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	return task, nil
}

func (s *Service) GetProject(ctx context.Context, req *pb.GetProjectRequest) (*pb.Project, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if !strings.HasPrefix(name, "projects/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}
	resourceID := strings.TrimPrefix(name, "projects/")
	if resourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project does not contain a resource ID after "projects/".`)
	}
	id, err := strconv.ParseInt(resourceID, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	var (
		project *pb.Project
		now     time.Time
	)
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		var err error
		now, err = s.now(ctx, tx)
		if err != nil {
			return err
		}
		t, err := queryProjectByID(ctx, tx, id, true /*showDeleted*/)
		if err != nil {
			return err
		}
		project = t
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	if expire := project.GetExpireTime(); expire.IsValid() && now.After(expire.AsTime()) {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	return project, nil
}

func (s *Service) ListProjects(ctx context.Context, req *pb.ListProjectsRequest) (*pb.ListProjectsResponse, error) {
	pageSize := req.GetPageSize()
	if pageSize < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "The page size must not be negative; was %d.", pageSize)
	}
	if pageSize == 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	if token := req.GetPageToken(); token != "" {
		if _, err := uuid.Parse(token); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", req.GetPageToken())
		}
	}

	res := &pb.ListProjectsResponse{}
	errNoToken := errors.New("page token given but not found")
	errChangedRequest := errors.New("request changed between pages")
	txFunc := func(tx pgx.Tx) error {
		now, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		// First find out what the minimum ID to use in this page is. If this is
		// the first page, it will be 0. If it is not, then it will be a value
		// stored in the `project_page_tokens` database table, and the `page_token`
		// field in the request contains the key to that table.
		minID := int64(0)
		showDeleted := req.GetShowDeleted()
		if token := req.GetPageToken(); token != "" {
			// We could do a SELECT and then a DELETE, but since Postgres
			// supports the RETURNING clause, we can do it in just one
			// statement. Neat!
			sql, args, err := postgres.StatementBuilder.
				Delete("project_page_tokens").
				Where(squirrel.Eq{
					"token": token,
				}).
				Suffix("RETURNING minimum_id, show_deleted").
				ToSql()
			if err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, sql, args...).Scan(&minID, &showDeleted); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return errNoToken
				}
				return err
			}
			if req.GetShowDeleted() != showDeleted {
				return errChangedRequest
			}
		}

		// Now that we know the minimum ID, we can run a SELECT to list projects.
		// We set a limit of pageSize+1 so that we may get the first project in the
		// next page (if any). This allows us to do one query that gives us
		//     1. if there is a next page, and if so,
		//     2. what the minimum ID will be for that page.
		var (
			// The eventual list of projects to return.
			projects []*pb.Project
			// The columns in the row.
			id                                 int64
			title                              string
			description                        string
			archiveTime                        pgtype.Timestamptz
			createTime                         time.Time
			updateTime, deleteTime, expireTime pgtype.Timestamptz
			// To use for the next page, if any.
			nextMinID int64
		)
		st := postgres.StatementBuilder.
			Select(
				"id",
				"title",
				"description",
				"archive_time",
				"create_time",
				"update_time",
				"delete_time",
				"expire_time",
			).
			From("projects").
			Where(squirrel.GtOrEq{
				"id": minID,
			})
		if !showDeleted {
			st = st.Where(squirrel.Eq{
				"delete_time": nil,
			})
		} else {
			st = st.Where(squirrel.Or{
				squirrel.Eq{
					"expire_time": nil,
				},
				squirrel.Gt{
					"expire_time": now,
				},
			})
		}
		st = st.
			OrderBy("id ASC").
			Limit(uint64(pageSize) + 1)
		sql, args, err := st.ToSql()
		if err != nil {
			return err
		}
		// qf is called for every row returned by the above query, after
		// scanning has completed successfully.
		qf := func(qfr pgx.QueryFuncRow) error {
			if id > nextMinID {
				nextMinID = id
			}
			project := &pb.Project{
				Name:        "projects/" + fmt.Sprint(id),
				Title:       title,
				Description: description,
				CreateTime:  timestamppb.New(createTime),
			}
			if archiveTime.Status == pgtype.Present {
				project.ArchiveTime = timestamppb.New(archiveTime.Time)
			}
			if updateTime.Status == pgtype.Present {
				project.UpdateTime = timestamppb.New(updateTime.Time)
			}
			if deleteTime.Status == pgtype.Present {
				project.DeleteTime = timestamppb.New(deleteTime.Time)
			}
			if expireTime.Status == pgtype.Present {
				project.ExpireTime = timestamppb.New(expireTime.Time)
			}
			projects = append(projects, project)
			return nil
		}
		// Here is where the actual query happens.
		if _, err := tx.QueryFunc(ctx, sql,
			args,
			[]interface{}{
				&id,
				&title,
				&description,
				&archiveTime,
				&createTime,
				&updateTime,
				&deleteTime,
				&expireTime,
			},
			qf,
		); err != nil {
			return err
		}

		// If the number of projects from the above query is less than or equal to
		// pageSize, we know that there will be no more pages We can then do an
		// early return.
		if int32(len(projects)) <= pageSize {
			res.Projects = projects
			return nil
		}

		// We know at this point that there will be at least one more page, so
		// we limit the projects in this page to the pageSize and then create the
		// token for the next page.
		res.Projects = projects[:pageSize]
		token := uuid.New()
		res.NextPageToken = token.String()
		sql, args, err = postgres.StatementBuilder.
			Insert("project_page_tokens").
			Columns("token", "minimum_id", "show_deleted").
			Values(token, nextMinID, showDeleted).
			ToSql()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			return err
		}
		return nil
	}
	if err := s.pool.BeginFunc(ctx, txFunc); err != nil {
		if errors.Is(err, errNoToken) || errors.Is(err, errChangedRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", req.GetPageToken())
		}
		klog.Error(err)
		return nil, internalError
	}
	return res, nil
}

func (s *Service) CreateProject(ctx context.Context, req *pb.CreateProjectRequest) (*pb.Project, error) {
	project := req.GetProject()
	if project.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "The project must have a title.")
	}
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		now, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		sql, args, err := postgres.StatementBuilder.
			Insert("projects").
			SetMap(map[string]interface{}{
				"title":       project.GetTitle(),
				"description": project.GetDescription(),
				"create_time": now,
			}).
			Suffix("RETURNING id").
			ToSql()
		if err != nil {
			return err
		}
		var id int64
		if err := tx.QueryRow(ctx, sql, args...).Scan(
			&id,
		); err != nil {
			return err
		}
		project.Name = "projects/" + fmt.Sprint(id)
		project.CreateTime = timestamppb.New(now)
		return nil
	}); err != nil {
		klog.Error(err)
		return nil, internalError
	}
	return project, nil
}

func (s *Service) UpdateProject(ctx context.Context, req *pb.UpdateProjectRequest) (*pb.Project, error) {
	// First we do stateless validation, i.e., look for errors that we can find
	// by only looking at the request message.
	patch := req.GetProject()
	name := patch.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if !strings.HasPrefix(name, "projects/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(name, "projects/"), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	updateMask := req.GetUpdateMask()
	if updateMask == nil {
		// This is not really necessary, but makes downstream handling easier by
		// not having to be careful about nil derefs.
		updateMask = &fieldmaskpb.FieldMask{}
	}
	// Handle two special cases:
	// 1. The update mask is nil or empty. Then it should be equivalent to
	//    updating all non-empty fields in the patch.
	// 2. The update mask contains a single path that is the wildcard ("*").
	// 	  Then it should be treated as specifying all updatable paths.
	switch paths := updateMask.GetPaths(); {
	case len(paths) == 0:
		if v := patch.GetTitle(); v != "" {
			updateMask.Paths = append(updateMask.GetPaths(), "title")
		}
		if v := patch.GetDescription(); v != "" {
			updateMask.Paths = append(updateMask.GetPaths(), "description")
		}
	case len(paths) == 1 && paths[0] == "*":
		updateMask = proto.Clone(projectUpdatableMask).(*fieldmaskpb.FieldMask)
	}
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "parent", "completed", "create_time", "name":
			return nil, status.Errorf(codes.InvalidArgument, "The field %q cannot be updated with UpdateProject.")
		case "*":
			// We handled the only valid case of giving a wildcard path above,
			// i.e., when it is the only path.
			return nil, status.Error(codes.InvalidArgument, "A wildcard can only be used if it is the single path in the update mask.")
		}
	}
	if updateMask != nil && !updateMask.IsValid(&pb.Project{}) {
		return nil, status.Error(codes.InvalidArgument, "The given update mask is invalid.")
	}
	// At this point we know that updateMask is not empty and is a valid mask.
	// The path(s) fully specify what we should get from the patch. It may still
	// be the case that the patch is empty.

	// updatedProject is the new version of the project that should eventually be
	// returned as the result of the update operation -- even if it is a no-op.
	var updatedProject *pb.Project

	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		// Eventually, we need to return either an error or the project, regardless
		// of whether it has been updated or not. So let's fetch it here, so we
		// quickly find out if it doesn't exist. If it does exist, we also get
		// all the details we eventually need to return about it.
		updatedProject, err = queryProjectByID(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}
		// Special case: the patch is empty so we should just return the current
		// version of the project which we fetched above.
		if proto.Equal(patch, &pb.Project{Name: name} /* empty patch except for the name */) {
			return nil
		}
		// Special case: the update mask is empty, meaning that the operation
		// will be a no-op even if the patch isn't empty.
		if len(updateMask.GetPaths()) == 0 {
			return nil
		}
		// Special case: the patch isn't empty and at least one path is
		// specified, but the applying the patch will yield an identical
		// resource.
		afterPatch := proto.Clone(updatedProject).(*pb.Project)
		proto.Merge(afterPatch, patch)
		if proto.Equal(afterPatch, updatedProject) {
			return nil
		}

		updateTime, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		updatedProject.UpdateTime = timestamppb.New(updateTime)

		// Update only the columns corresponding to the fields in the patch.
		q := postgres.StatementBuilder.
			Update("projects").
			Where(squirrel.Eq{
				"id": id,
			}).
			Set("update_time", updateTime)
		for _, path := range updateMask.GetPaths() {
			switch path {
			case "title":
				v := patch.GetTitle()
				q = q.Set("title", v)
				updatedProject.Title = v
			case "description":
				v := patch.GetDescription()
				q = q.Set("description", v)
				updatedProject.Description = v
			}
		}

		sql, args, err := q.ToSql()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", patch.GetName())
		}
		klog.Error(err)
		return nil, internalError
	}

	return updatedProject, nil
}

func (s *Service) DeleteProject(ctx context.Context, req *pb.DeleteProjectRequest) (*pb.Project, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if !strings.HasPrefix(name, "projects/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(name, "projects/"), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	// deleted will eventually be returned as the updated version of the project.
	var deleted *pb.Project

	txFunc := func(tx pgx.Tx) error {
		var err error

		// We must do two things:
		//     1. Ensure that the project being deleted exists.
		//     2. Return the new version of the project when it has been deleted.
		// To kill both these birds with one stone, we get the project from the
		// database here. If it doesn't exist, we will get an error. If it does
		// exist, we will get all the details and don't need to query for them
		// later.
		deleted, err = queryProjectByID(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}

		// We "delete" projects by setting their `delete_time` and `expire_time`
		// fields. `delete_time` should be set to the current time, and
		// `expire_time` is arbitrarily chosen to be some point in the future.
		deleteTime, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		expireTime := deleteTime.AddDate(0 /* years */, 0 /* months */, 30 /* days */)

		// These new timestamps should be reflected in the returned version of
		// the project.
		deleted.DeleteTime = timestamppb.New(deleteTime)
		deleted.ExpireTime = timestamppb.New(expireTime)

		// Below is the actual update in the database. We only update and don't
		// return anything back, because we have already fetched everything
		// using projectByID above.
		sql, args, err := postgres.StatementBuilder.
			Update("projects").
			SetMap(map[string]interface{}{
				"delete_time": deleteTime,
				"expire_time": expireTime,
			}).
			Where(squirrel.Eq{
				"id": id,
			}).
			ToSql()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}
	if err := s.pool.BeginFunc(ctx, txFunc); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	return deleted, nil
}

func (s *Service) UndeleteProject(ctx context.Context, req *pb.UndeleteProjectRequest) (*pb.Project, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if !strings.HasPrefix(name, "projects/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(name, "projects/"), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	var project *pb.Project
	errNotFound := errors.New("project does not exist")
	errNotDeleted := errors.New("project has not been deleted")
	errExpired := errors.New("project has expired")
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		now, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		project, err = queryProjectByID(ctx, tx, id, true /*showDeleted*/)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errNotFound
			}
			return err
		}
		if !project.GetDeleteTime().IsValid() {
			return errNotDeleted
		}
		if now.After(project.GetExpireTime().AsTime()) {
			return errExpired
		}

		sql, args, err := postgres.StatementBuilder.
			Update("projects").
			SetMap(map[string]interface{}{
				"delete_time": nil,
				"expire_time": nil,
			}).
			Where(squirrel.Eq{
				"id": id,
			}).
			ToSql()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}); err != nil {
		if errors.Is(err, errNotFound) || errors.Is(err, errExpired) {
			return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
		}
		if errors.Is(err, errNotDeleted) {
			return nil, status.Errorf(codes.AlreadyExists, "A project with name %q already exists.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	project.DeleteTime = nil
	project.ExpireTime = nil
	return project, nil
}

func (s *Service) ArchiveProject(ctx context.Context, req *pb.ArchiveProjectRequest) (*pb.Project, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if !strings.HasPrefix(name, "projects/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}
	resourceID := strings.TrimPrefix(name, "projects/")
	if resourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(resourceID, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}

	var project *pb.Project
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		var err error
		project, err = queryProjectByID(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}
		// Special case: a archived project can be archived again, which is a
		// no-op.
		if project.GetArchiveTime().IsValid() {
			return nil
		}
		archiveTime, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		project.ArchiveTime = timestamppb.New(archiveTime)
		project.UpdateTime = timestamppb.New(archiveTime)
		sql, args, err := postgres.StatementBuilder.
			Update("projects").
			SetMap(map[string]interface{}{
				"archive_time": archiveTime,
				"update_time":  archiveTime,
			}).
			Where(squirrel.Eq{
				"id": id,
			}).
			ToSql()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	return project, nil
}

func (s *Service) GetLabel(ctx context.Context, req *pb.GetLabelRequest) (*pb.Label, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the label is required.")
	}
	if !strings.HasPrefix(name, "labels/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the label must have format "labels/{label}", but it was %q.`, name)
	}
	resourceID := strings.TrimPrefix(name, "labels/")
	if resourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the label does not contain a resource ID after "labels/".`)
	}
	id, err := strconv.ParseInt(resourceID, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A label with name %q does not exist.", name)
	}
	var label *pb.Label
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		t, err := queryLabelByID(ctx, tx, id)
		if err != nil {
			return err
		}
		label = t
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A label with name %q does not exist.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	return label, nil
}

func (s *Service) CreateLabel(ctx context.Context, req *pb.CreateLabelRequest) (*pb.Label, error) {
	label := req.GetLabel()
	if label.GetLabel() == "" {
		return nil, status.Error(codes.InvalidArgument, "The label must have a title.")
	}
	var existingID int64
	errDuplicateLabel := errors.New("duplicate label")
	errInvalidLabelString := errors.New("invalid label string")
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		now, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		// First check if a label already exists. We do this as a SELECT because
		// we need to return the resource name for the existing label in the
		// error message, and for that we need to find the ID. Without this
		// requirement, we could just do an INSERT and use a uniqueness
		// constraint violation as the indication.
		{
			sql, args, err := postgres.StatementBuilder.
				Select("id").
				From("labels").
				Where(squirrel.Eq{
					"label": label.GetLabel(),
				}).
				ToSql()
			if err != nil {
				return err
			}
			var id int64
			err = tx.QueryRow(ctx, sql, args...).Scan(&id)
			switch {
			case err == nil:
				// The query executed successfully and an existing label was
				// found.
				existingID = id
				return errDuplicateLabel
			case errors.Is(err, pgx.ErrNoRows):
				// The query executed successfully but no duplicate label was
				// found. Do nothing and proceed with INSERT.
			default:
				// The query did not execute successfully.
				return err
			}
		}
		// Now we expect no existing label to exist, so proceed with the INSERT
		// expecting no uniqueness violations.
		{
			sql, args, err := postgres.StatementBuilder.
				Insert("labels").
				SetMap(map[string]interface{}{
					"label":       label.GetLabel(),
					"create_time": now,
				}).
				Suffix("RETURNING id").
				ToSql()
			if err != nil {
				return err
			}
			var id int64
			if err := tx.QueryRow(ctx, sql, args...).Scan(
				&id,
			); err != nil {
				if e := (*pgconn.PgError)(nil); errors.As(err, &e) {
					if e.Code == pgerrcode.CheckViolation && e.ConstraintName == "label_contains_valid_characters" {
						return errInvalidLabelString
					}
				}
				return err
			}
			label.Name = "labels/" + fmt.Sprint(id)
			label.CreateTime = timestamppb.New(now)
			return nil
		}
	}); err != nil {
		if errors.Is(err, errInvalidLabelString) {
			return nil, status.Errorf(codes.InvalidArgument, "Label string %q contains invalid characters.", label.GetLabel())
		}
		if errors.Is(err, errDuplicateLabel) {
			existingName := "labels/" + fmt.Sprint(existingID)
			return nil, status.Errorf(codes.AlreadyExists, "The label %q already exists as %q.", label.GetLabel(), existingName)
		}
		klog.Error(err)
		return nil, internalError
	}
	return label, nil
}

func (s *Service) UnarchiveProject(ctx context.Context, req *pb.UnarchiveProjectRequest) (*pb.Project, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if !strings.HasPrefix(name, "projects/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}
	resourceID := strings.TrimPrefix(name, "projects/")
	if resourceID == "" {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(resourceID, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}

	var project *pb.Project
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		var err error
		project, err = queryProjectByID(ctx, tx, id, false /* showDeleted */)
		if err != nil {
			return err
		}
		// Special case: uncompleting an unarchived project is a no-op.
		if !project.GetArchiveTime().IsValid() {
			return nil
		}
		updateTime, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		project.ArchiveTime = nil
		project.UpdateTime = timestamppb.New(updateTime)
		sql, args, err := postgres.StatementBuilder.
			Update("projects").
			SetMap(map[string]interface{}{
				"archive_time": nil,
				"update_time":  updateTime,
			}).
			Where(squirrel.Eq{
				"id": id,
			}).
			ToSql()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, sql, args...)
		return err
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
		}
		klog.Error(err)
		return nil, internalError
	}
	return project, nil
}

// queryDescendantIDs returns the IDs of all tasks descending, directly or
// transitively, from rootID. Note that rootID is itself not included in the
// resulting slice. If showDeleted is true, IDs from deleted descendant tasks
// are also included.
func queryDescendantIDs(ctx context.Context, tx pgx.Tx, rootID int64, showDeleted bool) ([]int64, error) {
	view := "existing_tasks_descendants"
	if showDeleted {
		view = "tasks_descendants"
	}
	sql, args, err := postgres.StatementBuilder.
		Select("descendant").
		From(view).
		Where(squirrel.Eq{
			"task": rootID,
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	// SQL setup is done. Now we can run the query. We scan each row's result
	// into id, and then collect everything into ids.
	var (
		id  int64
		ids []int64
	)
	scans := []interface{}{&id}
	if _, err := tx.QueryFunc(ctx, sql, args, scans, func(_ pgx.QueryFuncRow) error {
		ids = append(ids, id)
		return nil
	}); err != nil {
		return nil, err
	}
	return ids, nil
}

func queryAncestorIDs(ctx context.Context, tx pgx.Tx, leafID int64, showDeleted bool) ([]int64, error) {
	view := "existing_tasks_ancestors"
	if showDeleted {
		view = "tasks_ancestors"
	}
	sql, args, err := postgres.StatementBuilder.
		Select("ancestor").
		From(view).
		Where(squirrel.Eq{
			"task": leafID,
		}).
		ToSql()
	if err != nil {
		return nil, err
	}

	// SQL setup is done. Now we can run the query. We scan each row's result
	// into id, and then collect everything into ids.
	var (
		id  int64
		ids []int64
	)
	scans := []interface{}{&id}
	if _, err := tx.QueryFunc(ctx, sql, args, scans, func(_ pgx.QueryFuncRow) error {
		ids = append(ids, id)
		return nil
	}); err != nil {
		return nil, err
	}
	return ids, nil
}

// queryTaskByID queries the database within the given transaction for the task
// with the given ID. Any errors from database driver is returned. For example,
// if no task is found by the given ID, pgx.ErrNoRows is returned, and callers
// should check for it using errors.Is.
func queryTaskByID(ctx context.Context, tx pgx.Tx, id int64, showDeleted bool) (*pb.Task, error) {
	task := &pb.Task{
		Name: "tasks/" + fmt.Sprint(id),
	}
	var parent *int64
	var completeTime pgtype.Timestamptz
	var createTime time.Time
	var deleteTime, expireTime, updateTime pgtype.Timestamptz
	st := postgres.StatementBuilder.
		Select(
			"parent",
			"title",
			"description",
			"complete_time",
			"create_time",
			"update_time",
			"delete_time",
			"expire_time",
		)

	from := "existing_tasks"
	if showDeleted {
		from = "tasks"
	}
	st = st.
		From(from).
		Where(squirrel.Eq{
			"id": id,
		})
	sql, args, err := st.ToSql()
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, sql, args...).Scan(
		&parent,
		&task.Title,
		&task.Description,
		&completeTime,
		&createTime,
		&updateTime,
		&deleteTime,
		&expireTime,
	); err != nil {
		return nil, err
	}
	if parent != nil {
		task.Parent = fmt.Sprintf("tasks/%d", *parent)
	}
	if completeTime.Status == pgtype.Present {
		task.CompleteTime = timestamppb.New(completeTime.Time)
	}
	task.CreateTime = timestamppb.New(createTime)
	if deleteTime.Status == pgtype.Present {
		task.DeleteTime = timestamppb.New(deleteTime.Time)
	}
	if expireTime.Status == pgtype.Present {
		task.ExpireTime = timestamppb.New(expireTime.Time)
	}
	if updateTime.Status == pgtype.Present {
		task.UpdateTime = timestamppb.New(updateTime.Time)
	}
	return task, nil
}

// queryProjectByID queries the database within the given transaction for the
// project with the given ID. Any errors from database driver is returned. For
// example, if no project is found by the given ID, pgx.ErrNoRows is returned, and
// callers should check for it using errors.Is.
func queryProjectByID(ctx context.Context, tx pgx.Tx, id int64, showDeleted bool) (*pb.Project, error) {
	project := &pb.Project{
		Name: "projects/" + fmt.Sprint(id),
	}
	var archiveTime pgtype.Timestamptz
	var createTime time.Time
	var deleteTime, expireTime, updateTime pgtype.Timestamptz
	st := postgres.StatementBuilder.
		Select(
			"title",
			"description",
			"archive_time",
			"create_time",
			"update_time",
			"delete_time",
			"expire_time",
		)

	from := "existing_projects"
	if showDeleted {
		from = "projects"
	}
	st = st.
		From(from).
		Where(squirrel.Eq{
			"id": id,
		})
	sql, args, err := st.ToSql()
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, sql, args...).Scan(
		&project.Title,
		&project.Description,
		&archiveTime,
		&createTime,
		&updateTime,
		&deleteTime,
		&expireTime,
	); err != nil {
		return nil, err
	}
	if archiveTime.Status == pgtype.Present {
		project.ArchiveTime = timestamppb.New(archiveTime.Time)
	}
	project.CreateTime = timestamppb.New(createTime)
	if deleteTime.Status == pgtype.Present {
		project.DeleteTime = timestamppb.New(deleteTime.Time)
	}
	if expireTime.Status == pgtype.Present {
		project.ExpireTime = timestamppb.New(expireTime.Time)
	}
	if updateTime.Status == pgtype.Present {
		project.UpdateTime = timestamppb.New(updateTime.Time)
	}
	return project, nil
}

// queryLabelByID queries the database within the given transaction for the
// label with the given ID. Any errors from database driver is returned. For
// example, if no label is found by the given ID, pgx.ErrNoRows is returned, and
// callers should check for it using errors.Is.
func queryLabelByID(ctx context.Context, tx pgx.Tx, id int64) (*pb.Label, error) {
	label := &pb.Label{
		Name: "labels/" + fmt.Sprint(id),
	}
	var (
		createTime time.Time
		updateTime pgtype.Timestamptz
	)
	sql, args, err := postgres.StatementBuilder.
		Select(
			"label",
			"create_time",
			"update_time",
		).
		From("labels").
		Where(squirrel.Eq{
			"id": id,
		}).ToSql()
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, sql, args...).Scan(
		&label.Label,
		&createTime,
		&updateTime,
	); err != nil {
		return nil, err
	}
	label.CreateTime = timestamppb.New(createTime)
	if updateTime.Status == pgtype.Present {
		label.UpdateTime = timestamppb.New(updateTime.Time)
	}
	return label, nil
}

func (s *Service) now(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	if s.clock != nil {
		return s.clock.Now(), nil
	}
	var now time.Time
	if err := tx.QueryRow(ctx, "SELECT NOW()").Scan(&now); err != nil {
		return time.Time{}, err
	}
	return now, nil
}
