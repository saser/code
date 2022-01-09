package service

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type Service struct {
	pb.UnimplementedTasksServer

	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service {
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
	id, err := uuid.Parse(strings.TrimPrefix(name, "tasks/"))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	task := &pb.Task{
		Name: name,
	}
	if err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		sql := strings.TrimSpace(`
SELECT
    title,
	description,
	completed
FROM
    tasks
WHERE
	uuid = $1
`)
		return tx.QueryRow(ctx, sql, id).Scan(&task.Title, &task.Description, &task.Completed)
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		return nil, status.Error(codes.Internal, "Something went wrong.")
	}
	return task, nil
}

func (s *Service) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.Task, error) {
	task := req.GetTask()
	if task.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "The task must have a title.")
	}
	if task.GetCompleted() {
		return nil, status.Error(codes.InvalidArgument, "The task must not already be completed.")
	}
	id := uuid.New()
	task.Name = "tasks/" + id.String()
	err := s.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		sql := strings.TrimSpace(`
INSERT INTO tasks (uuid, title, description, completed)
VALUES            ($1,   $2,    $3,          $4       )
`)
		_, err := tx.Exec(ctx, sql,
			id,                    // $1
			task.GetTitle(),       // $2
			task.GetDescription(), // $3
			task.GetCompleted(),   // $4
		)
		if err != nil {
			log.Print(err)
		}
		return err
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "Something went wrong.")
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
	id, err := uuid.Parse(strings.TrimPrefix(name, "tasks/"))
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	errNotFound := errors.New("not found")
	txFunc := func(tx pgx.Tx) error {
		// Check first if the task exists. If not, we should return a
		// codes.NotFound error. There's probably a more intelligent way of
		// doing this by checking the output from the DELETE statement, but this
		// is pretty straightforward and also a bit more defensive.
		{
			sql := strings.TrimSpace(`
SELECT COUNT(*)
	FROM tasks
WHERE uuid = $1
`)
			var n int
			err := tx.QueryRow(ctx, sql, id).Scan(&n)
			if err != nil {
				log.Print(err)
				return err
			}
			if n != 1 {
				return errNotFound
			}
		}
		// Now that we know the task exists we can delete it.
		{
			sql := strings.TrimSpace(`
DELETE FROM tasks
WHERE uuid = $1
`)
			_, err := tx.Exec(ctx, sql, id)
			if err != nil {
				log.Print(err)
				return err
			}
		}
		return nil
	}
	if err := s.pool.BeginFunc(ctx, txFunc); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		return nil, status.Error(codes.Internal, "Something went wrong.")
	}
	return &emptypb.Empty{}, nil
}
