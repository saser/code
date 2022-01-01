package service_test

import (
	"context"
	"net"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.saser.se/tasks/service"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"
)

func newPool(ctx context.Context, t *testing.T) *pgxpool.Pool {
	t.Helper()
	connString := os.Getenv("TASKS_TEST_DATABASE")
	if connString == "" {
		t.Fatal("environment variable $TASKS_TEST_DATABASE is empty")
	}
	pool, err := pgxpool.Connect(ctx, connString)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func setup(ctx context.Context, t *testing.T) pb.TasksClient {
	t.Helper()

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() {
		if err := lis.Close(); err != nil {
			t.Error(err)
		}
	})

	pool := newPool(ctx, t)
	t.Cleanup(func() {
		if err := pool.BeginFunc(ctx, func(tx pgx.Tx) error {
			sql := "TRUNCATE TABLE tasks"
			_, err := tx.Exec(ctx, sql)
			return err
		}); err != nil {
			t.Error(err)
		}
	})

	srv := grpc.NewServer()
	svc := service.New(newPool(ctx, t))
	pb.RegisterTasksServer(srv, svc)
	errc := make(chan error, 1)
	go func() {
		errc <- srv.Serve(lis)
	}()
	t.Cleanup(func() {
		srv.GracefulStop()
		if err := <-errc; err != nil {
			t.Error(err)
		}
	})

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	cc, err := grpc.DialContext(
		ctx,
		"bufconn",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cc.Close(); err != nil {
			t.Error(err)
		}
	})

	return pb.NewTasksClient(cc)
}

func TestService_CreateTask(t *testing.T) {
	ctx := context.Background()
	c := setup(ctx, t)

	task := &pb.Task{Title: "Hello Tasks"}
	req := &pb.CreateTaskRequest{
		Task: task,
	}
	got, err := c.CreateTask(ctx, req)
	if err != nil {
		t.Fatalf("CreateTask(%v) err = %v; want nil", req, err)
	}
	if got.GetName() == "" {
		t.Error("got.GetName() is empty")
	}
	if diff := cmp.Diff(task, got, protocmp.Transform(), protocmp.IgnoreFields(task, "name")); diff != "" {
		t.Errorf("CreateTask(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func TestService_DeleteTask(t *testing.T) {
	ctx := context.Background()
	c := setup(ctx, t)

	// Creating a task should succeed.
	var task *pb.Task
	{
		want := &pb.Task{Title: "This will be deleted"}
		req := &pb.CreateTaskRequest{
			Task: want,
		}
		got, err := c.CreateTask(ctx, req)
		if err != nil {
			t.Fatalf("CreateTask(%v) err = %v; want nil", req, err)
		}
		if got.GetName() == "" {
			t.Error("got.GetName() is empty")
		}
		if diff := cmp.Diff(want, got, protocmp.Transform(), protocmp.IgnoreFields(task, "name")); diff != "" {
			t.Errorf("CreateTask(%v): unexpected result (-want +got)\n%s", req, diff)
		}
		task = got
	}

	// Once the task has been created it should be deleted.
	{
		req := &pb.DeleteTaskRequest{Name: task.GetName()}
		_, err := c.DeleteTask(ctx, req)
		if err != nil {
			t.Fatalf("first deletion: DeleteTask(%v) err = %v; want nil", req, err)
		}
	}

	// Deleting the task again should result in a NotFound error.
	{
		req := &pb.DeleteTaskRequest{Name: task.GetName()}
		_, err := c.DeleteTask(ctx, req)
		if got, want := status.Code(err), codes.NotFound; got != want {
			t.Fatalf("second deletion: DeleteTask(%v) code = %v; want %v", req, got, want)
		}
	}
}

func TestService_DeleteTask_Error(t *testing.T) {
	ctx := context.Background()
	c := setup(ctx, t)
	for _, tt := range []struct {
		name string
		req  *pb.DeleteTaskRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req:  &pb.DeleteTaskRequest{Name: ""},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req:  &pb.DeleteTaskRequest{Name: "tasks/" + uuid.NewString()},
			want: codes.NotFound,
		},
		{
			name: "InvalidName",
			req:  &pb.DeleteTaskRequest{Name: "invalidlololol/" + uuid.NewString()},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.DeleteTask(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("DeleteTask(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}
		})
	}
}
