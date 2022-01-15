package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"go.saser.se/postgres"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
	id, err := uuid.Parse(strings.TrimPrefix(name, "tasks/"))
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
	uuid = $1
`)
		return tx.QueryRow(ctx, sql, id).Scan(
			&task.Title,
			&task.Description,
			&task.Completed,
			&createTime,
		)
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
		}
		return nil, status.Error(codes.Internal, "Something went wrong.")
	}
	task.CreateTime = timestamppb.New(createTime)
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
INSERT INTO tasks (uuid, title, description, completed, create_time)
VALUES            ($1,   $2,    $3,          $4,        NOW()      )
RETURNING create_time
`)
		var createTime time.Time
		err := tx.QueryRow(ctx, sql,
			id,                    // $1
			task.GetTitle(),       // $2
			task.GetDescription(), // $3
			task.GetCompleted(),   // $4
		).Scan(&createTime)
		if err != nil {
			log.Print(err)
		} else {
			task.CreateTime = timestamppb.New(createTime)
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
		sql := strings.TrimSpace(`
DELETE FROM tasks
WHERE uuid = $1
`)
		tag, err := tx.Exec(ctx, sql, id)
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
		return nil, status.Error(codes.Internal, "Something went wrong.")
	}
	return &emptypb.Empty{}, nil
}
