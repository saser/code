package service_test

import (
	"context"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"go.saser.se/postgres/postgrestest"
	"go.saser.se/tasks/service"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"
)

type testClient struct {
	pb.TasksClient
}

func (c *testClient) CreateTaskT(ctx context.Context, t *testing.T, req *pb.CreateTaskRequest) *pb.Task {
	t.Helper()
	task, err := c.CreateTask(ctx, req)
	if err != nil {
		t.Fatalf("CreateTask(%v) err = %v; want nil", req, err)
	}
	return task
}

func (c *testClient) DeleteTaskT(ctx context.Context, t *testing.T, req *pb.DeleteTaskRequest) {
	t.Helper()
	_, err := c.DeleteTask(ctx, req)
	if err != nil {
		t.Fatalf("DeleteTask(%v) err = %v; want nil", req, err)
	}
}

func setup(ctx context.Context, t *testing.T) *testClient {
	t.Helper()

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() {
		if err := lis.Close(); err != nil {
			t.Error(err)
		}
	})

	srv := grpc.NewServer()
	svc := service.New(postgrestest.Open(ctx, t, "tasks/postgres/schema.sql"))
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

	return &testClient{TasksClient: pb.NewTasksClient(cc)}
}

func TestService_CreateTask(t *testing.T) {
	t.Parallel()
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

func TestService_CreateTask_Error(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := setup(ctx, t)
	for _, tt := range []struct {
		name string
		req  *pb.CreateTaskRequest
		want codes.Code
	}{
		{
			name: "EmptyTitle",
			req: &pb.CreateTaskRequest{
				Task: &pb.Task{
					Title:     "",
					Completed: false,
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "AlreadyCompleted",
			req: &pb.CreateTaskRequest{
				Task: &pb.Task{
					Title:     "Something already done",
					Completed: true,
				},
			},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.CreateTask(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("CreateTask(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}
		})
	}
}

func TestService_DeleteTask(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := setup(ctx, t)

	task := c.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{Title: "This will be deleted"},
	})

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
	t.Parallel()
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
