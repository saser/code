package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"go.saser.se/postgres"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
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

type Service struct {
	pb.UnimplementedTasksServer

	pool *postgres.Pool
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
	task := &pb.Task{
		Name: name,
	}
	var createTime time.Time
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		sql := strings.TrimSpace(`
SELECT
    title,
	description,
	completed,
	create_time
FROM
    tasks
WHERE
	id = $1
	AND delete_time IS NULL
`)
		return tx.QueryRow(ctx, sql, id /* $1 */).Scan(
			&task.Title,
			&task.Description,
			&task.Completed,
			&createTime,
		)
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		return nil, internalError
	}
	task.CreateTime = timestamppb.New(createTime)
	return task, nil
}

func (s *Service) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (res *pb.ListTasksResponse, err error) {
	pageSize := req.GetPageSize()
	if pageSize < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "The page size must not be negative; was %d.", pageSize)
	}
	if pageSize == 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		glog.Error(err)
		return nil, internalError
	}
	defer func() {
		var txErr error
		if err == nil {
			txErr = tx.Commit(ctx)
		} else {
			txErr = tx.Rollback(ctx)
		}
		if txErr != nil {
			glog.Error(err)
		}
	}()

	minID := int64(0)
	if token := req.GetPageToken(); token != "" {
		sql := strings.TrimSpace(`
DELETE
FROM
    page_tokens
WHERE
    token = $1
RETURNING
    minimum_id
		`)
		if err := tx.QueryRow(ctx, sql, token /* $1 */).Scan(&minID); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", token)
		}
	}

	// List the tasks. We increase the limit by 1 so that we know whether there
	// is at least one more task after the ones included in the current page,
	// meaning that we should return a non-empty next_page_token.
	var tasks []*pb.Task
	tasksSQL := strings.TrimSpace(`
SELECT
	id,
	title,
	description,
	completed,
	create_time
FROM
	tasks
WHERE
	id >= $1
	AND delete_time IS NULL
ORDER BY id ASC
LIMIT $2
	`)
	// These variables correspond to the selected columns.
	var (
		id          int64
		title       string
		description string
		completed   bool
		createTime  time.Time
	)
	// nextMinID is used to determine the minimum ID for the next page of
	// results, if any.
	var nextMinID int64
	_, err = tx.QueryFunc(ctx, tasksSQL,
		[]interface{}{
			minID,        // $1
			pageSize + 1, // $2 -- see comment above about why we increase the limit by one.
		},
		[]interface{}{
			&id,
			&title,
			&description,
			&completed,
			&createTime,
		},
		func(_ pgx.QueryFuncRow) error {
			tasks = append(tasks, &pb.Task{
				Name:        "tasks/" + fmt.Sprint(id),
				Title:       title,
				Description: description,
				Completed:   completed,
				CreateTime:  timestamppb.New(createTime),
			})
			// The last row returned will (possibly) be the minimum ID in the
			// next page. Instead of checking len(tasks) to figure out if we are
			// on the next page, we can just set this here.
			nextMinID = id
			return nil
		},
	)
	if err != nil {
		glog.Error(err)
		return nil, internalError
	}

	// If we listed no tasks (maybe because there are none) there will be no
	// more pages, so we can do an early return here.
	if len(tasks) == 0 {
		return &pb.ListTasksResponse{}, nil
	}

	// We saw at least one task. If the number of tasks is less than or equal to
	// pageSize, there will be no more pages.
	if int32(len(tasks)) <= pageSize {
		return &pb.ListTasksResponse{Tasks: tasks}, nil
	}

	// There will be at least one more page. Create the page token, use the ID
	// from the extra task as the mininum ID, and return the page token.
	token := uuid.New()
	tokenSQL := strings.TrimSpace(`
INSERT INTO page_tokens (token, minimum_id)
VALUES                  ($1,    $2        )
	`)
	_, err = tx.Exec(ctx, tokenSQL,
		token,     // $1
		nextMinID, // $2
	)
	if err != nil {
		glog.Error(err)
		return nil, internalError
	}

	return &pb.ListTasksResponse{
		Tasks:         tasks[:pageSize],
		NextPageToken: token.String(),
	}, nil
}

func (s *Service) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.Task, error) {
	task := req.GetTask()
	if task.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "The task must have a title.")
	}
	if task.GetCompleted() {
		return nil, status.Error(codes.InvalidArgument, "The task must not already be completed.")
	}
	err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		sql := strings.TrimSpace(`
INSERT INTO tasks (title, description, completed, create_time)
VALUES            ($1,    $2,          $3,        NOW()      )
RETURNING id, create_time
`)
		var (
			id         int64
			createTime time.Time
		)
		err := tx.QueryRow(ctx, sql,
			task.GetTitle(),       // $1
			task.GetDescription(), // $2
			task.GetCompleted(),   // $3
		).Scan(
			&id,
			&createTime,
		)
		if err != nil {
			log.Print(err)
		} else {
			task.Name = "tasks/" + fmt.Sprint(id)
			task.CreateTime = timestamppb.New(createTime)
		}
		return err
	})
	if err != nil {
		return nil, internalError
	}
	return task, nil
}

func (s *Service) DeleteTask(ctx context.Context, req *pb.DeleteTaskRequest) (*emptypb.Empty, error) {
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
	errNotFound := errors.New("not found")
	txFunc := func(tx pgx.Tx) error {
		sql := strings.TrimSpace(`
UPDATE tasks
SET
    delete_time = NOW()
WHERE
    id = $1
	AND delete_time IS NULL
`)
		tag, err := tx.Exec(ctx, sql, id /* $1 */)
		if err != nil {
			log.Print(err)
			return err
		}
		if tag.RowsAffected() == 0 {
			return errNotFound
		}
		return nil
	}
	if err := s.pool.BeginFunc(ctx, txFunc); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		return nil, internalError
	}
	return &emptypb.Empty{}, nil
}
