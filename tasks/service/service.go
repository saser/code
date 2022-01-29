package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/golang/glog"
	"github.com/google/uuid"
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
)

// maxPageSize is the maximum number of tasks the server will return on a call
// to ListTasks. Any request for more than maxPageSize tasks will only return
// (at most) maxPageSize tasks.
const maxPageSize = 1000

// internalError should be returned whenever something goes wrong with serving a
// request, and where the error cannot be attributed to the user making an
// invalid request, something cannot be found, etc.
var internalError = status.Error(codes.Internal, "Something went wrong.")

// updatableMask contains the fields that can be updated by UpdateTask. It must
// be kept in sync with the proto definition.
var updatableMask *fieldmaskpb.FieldMask

func init() {
	m, err := fieldmaskpb.New(&pb.Task{},
		"title",
		"description",
	)
	if err != nil {
		glog.Exit(err)
	}
	updatableMask = m
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
	id, err := strconv.ParseInt(strings.TrimPrefix(name, "tasks/"), 10, 64)
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
		t, err := taskByID(ctx, tx, id, true /*showDeleted*/)
		if err != nil {
			return err
		}
		task = t
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		return nil, internalError
	}
	if expiry := task.GetExpiryTime(); expiry.IsValid() && now.After(expiry.AsTime()) {
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
		// stored in the `page_tokens` database table, and the `page_token`
		// field in the request contains the key to that table.
		minID := int64(0)
		showDeleted := req.GetShowDeleted()
		if token := req.GetPageToken(); token != "" {
			// We could do a SELECT and then a DELETE, but since Postgres
			// supports the RETURNING clause, we can do it in just one
			// statement. Neat!
			sql, args, err := postgres.StatementBuilder.
				Delete("page_tokens").
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
			id                     int64
			title                  string
			description            string
			completed              bool
			createTime             time.Time
			deleteTime, expiryTime pgtype.Timestamptz
			// To use for the next page, if any.
			nextMinID int64
		)
		st := postgres.StatementBuilder.
			Select(
				"id",
				"title",
				"description",
				"completed",
				"create_time",
				"delete_time",
				"expiry_time",
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
					"expiry_time": nil,
				},
				squirrel.Gt{
					"expiry_time": now,
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
				Completed:   completed,
				CreateTime:  timestamppb.New(createTime),
			}
			if deleteTime.Status == pgtype.Present {
				task.DeleteTime = timestamppb.New(deleteTime.Time)
			}
			if expiryTime.Status == pgtype.Present {
				task.ExpiryTime = timestamppb.New(expiryTime.Time)
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
				&completed,
				&createTime,
				&deleteTime,
				&expiryTime,
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
			Insert("page_tokens").
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
		glog.Error(err)
		return nil, internalError
	}
	return res, nil
}

func (s *Service) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.Task, error) {
	task := req.GetTask()
	if task.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "The task must have a title.")
	}
	if task.GetCompleted() {
		return nil, status.Error(codes.InvalidArgument, "The task must not already be completed.")
	}
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		now, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		sql, args, err := postgres.StatementBuilder.
			Insert("tasks").
			Columns("title", "description", "completed", "create_time").
			Values(task.GetTitle(), task.GetDescription(), task.GetCompleted(), now).
			Suffix("RETURNING id, create_time").
			ToSql()
		if err != nil {
			return err
		}
		var (
			id         int64
			createTime time.Time
		)
		if err := tx.QueryRow(ctx, sql, args...).Scan(
			&id,
			&createTime,
		); err != nil {
			return err
		}
		task.Name = "tasks/" + fmt.Sprint(id)
		task.CreateTime = timestamppb.New(createTime)
		return nil
	}); err != nil {
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
		updateMask = proto.Clone(updatableMask).(*fieldmaskpb.FieldMask)
	}
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "completed", "create_time", "name":
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

	updatedTask := &pb.Task{
		Name: name,
	}
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		// Special case: the patch is empty so we should just return the current
		// version of the task.
		if proto.Equal(patch, &pb.Task{Name: name} /* empty patch except for the name */) {
			var err error
			updatedTask, err = taskByID(ctx, tx, id, false /*showDeleted*/)
			return err
		}
		// Update only the columns corresponding to the fields in the patch.
		q := postgres.StatementBuilder.
			Update("tasks").
			Where(squirrel.Eq{
				"id":          id,
				"delete_time": nil,
			})
		for _, path := range updateMask.GetPaths() {
			switch path {
			case "title":
				q = q.Set("title", patch.GetTitle())
			case "description":
				q = q.Set("description", patch.GetDescription())
			}
		}
		q = q.Suffix("RETURNING title, description, completed, create_time")

		sql, args, err := q.ToSql()
		if err != nil {
			return err
		}
		var createTime time.Time
		if err := tx.QueryRow(ctx, sql, args...).Scan(
			&updatedTask.Title,
			&updatedTask.Description,
			&updatedTask.Completed,
			&createTime,
		); err != nil {
			return err
		}
		updatedTask.CreateTime = timestamppb.New(createTime)
		return nil
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", patch.GetName())
		}
		glog.Error(err)
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
	deleted := &pb.Task{
		Name: name,
	}
	var createTime, deleteTime, expiryTime time.Time

	txFunc := func(tx pgx.Tx) error {
		// I opted out of using the statement builder here because I couldn't
		// get "NOW() + interval '30 days'" to work with it (it tries to parse
		// the string "NOW() + interval '30 days'" as a TIMESTAMP WITH TIME
		// ZONE).
		sqlTmpl := strings.TrimSpace(`
UPDATE
    tasks
SET
    delete_time = %s, -- deleteTimeParameter
    expiry_time = %s  -- expiryTimeParameter
WHERE
    id = $1
    AND delete_time IS NULL
RETURNING
    title,
    description,
    completed,
    create_time,
    delete_time,
    expiry_time
`)
		args := []interface{}{
			id, // $1
		}
		var deleteTimeParameter, expiryTimeParameter string
		if s.clock != nil {
			now := s.clock.Now()
			args = append(args, now) // $2
			deleteTimeParameter = "$2"
			args = append(args, now.AddDate(0 /*years*/, 0 /*months*/, 30 /*days*/)) // $3
			expiryTimeParameter = "$3"
		} else {
			deleteTimeParameter = "NOW()"
			expiryTimeParameter = "NOW() + interval '30 days'"
		}
		sql := fmt.Sprintf(sqlTmpl, deleteTimeParameter, expiryTimeParameter)
		return tx.QueryRow(ctx, sql, args...).Scan(
			&deleted.Title,
			&deleted.Description,
			&deleted.Completed,
			&createTime,
			&deleteTime,
			&expiryTime,
		)
	}
	if err := s.pool.BeginFunc(ctx, txFunc); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		return nil, internalError
	}
	deleted.CreateTime = timestamppb.New(createTime)
	deleted.DeleteTime = timestamppb.New(deleteTime)
	deleted.ExpiryTime = timestamppb.New(expiryTime)
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
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		now, err := s.now(ctx, tx)
		if err != nil {
			return err
		}
		task, err = taskByID(ctx, tx, id, true /*showDeleted*/)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errNotFound
			}
			return err
		}
		if !task.GetDeleteTime().IsValid() {
			return errNotDeleted
		}
		if now.After(task.GetExpiryTime().AsTime()) {
			return errExpired
		}
		sql, args, err := postgres.StatementBuilder.
			Update("tasks").
			SetMap(map[string]interface{}{
				"delete_time": nil,
				"expiry_time": nil,
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
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		if errors.Is(err, errNotDeleted) {
			return nil, status.Errorf(codes.AlreadyExists, "A task with name %q already exists.", name)
		}
		glog.Error(err)
		return nil, internalError
	}
	task.DeleteTime = nil
	task.ExpiryTime = nil
	return task, nil
}

// taskByID the database within the given transaction for the task with the
// given ID. Any errors from database driver is returned. For example, if no
// task is found by the given ID, pgx.ErrNoRows is returned, and callers should
// check for it using errors.Is.
func taskByID(ctx context.Context, tx pgx.Tx, id int64, showDeleted bool) (*pb.Task, error) {
	task := &pb.Task{
		Name: "tasks/" + fmt.Sprint(id),
	}
	var createTime time.Time
	var deleteTime, expiryTime pgtype.Timestamptz
	st := postgres.StatementBuilder.
		Select(
			"title",
			"description",
			"completed",
			"create_time",
			"delete_time",
			"expiry_time",
		).
		From("tasks").
		Where(squirrel.Eq{
			"id": id,
		})
	if !showDeleted {
		st = st.Where(squirrel.Eq{
			"delete_time": nil,
		})
	}
	sql, args, err := st.ToSql()
	if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, sql, args...).Scan(
		&task.Title,
		&task.Description,
		&task.Completed,
		&createTime,
		&deleteTime,
		&expiryTime,
	); err != nil {
		return nil, err
	}
	task.CreateTime = timestamppb.New(createTime)
	if deleteTime.Status == pgtype.Present {
		task.DeleteTime = timestamppb.New(deleteTime.Time)
	}
	if expiryTime.Status == pgtype.Present {
		task.ExpiryTime = timestamppb.New(expiryTime.Time)
	}
	return task, nil
}

func (s *Service) now(ctx context.Context, tx pgx.Tx) (time.Time, error) {
	if s.clock != nil {
		return s.clock.Now(), nil
	}
	var now time.Time
	err := tx.QueryRow(ctx, "SELECT NOW()").Scan(&now)
	return now, err
}
