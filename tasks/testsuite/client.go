package testsuite

import (
	"context"
	"testing"

	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/protobuf/proto"
)

type testClient struct {
	pb.TasksClient
}

func (c *testClient) GetTaskT(ctx context.Context, tb testing.TB, req *pb.GetTaskRequest) *pb.Task {
	tb.Helper()
	task, err := c.GetTask(ctx, req)
	if err != nil {
		tb.Fatalf("GetTask(%v) err = %v; want nil", req, err)
	}
	return task
}

func (c *testClient) ListTasksT(ctx context.Context, tb testing.TB, req *pb.ListTasksRequest) *pb.ListTasksResponse {
	tb.Helper()
	res, err := c.ListTasks(ctx, req)
	if err != nil {
		tb.Fatalf("ListTasks(%v) err = %v; want nil", req, err)
	}
	return res
}

func (c *testClient) ListAllTasksT(ctx context.Context, tb testing.TB, req *pb.ListTasksRequest) []*pb.Task {
	tb.Helper()
	var tasks []*pb.Task
	req = proto.Clone(req).(*pb.ListTasksRequest)
	for {
		res := c.ListTasksT(ctx, tb, req)
		tasks = append(tasks, res.GetTasks()...)
		token := res.GetNextPageToken()
		if token == "" {
			break
		}
		req.PageToken = token
	}
	return tasks
}

func (c *testClient) CreateTaskT(ctx context.Context, tb testing.TB, req *pb.CreateTaskRequest) *pb.Task {
	tb.Helper()
	task, err := c.CreateTask(ctx, req)
	if err != nil {
		tb.Fatalf("CreateTask(%v) err = %v; want nil", req, err)
	}
	return task
}

func (c *testClient) CreateTasksT(ctx context.Context, tb testing.TB, tasks []*pb.Task) []*pb.Task {
	tb.Helper()
	var created []*pb.Task
	for _, task := range tasks {
		created = append(created, c.CreateTaskT(ctx, tb, &pb.CreateTaskRequest{
			Task: task,
		}))
	}
	return created
}

func (c *testClient) UpdateTaskT(ctx context.Context, tb testing.TB, req *pb.UpdateTaskRequest) *pb.Task {
	tb.Helper()
	task, err := c.UpdateTask(ctx, req)
	if err != nil {
		tb.Fatalf("UpdateTask(%v) err = %v; want nil", req, err)
	}
	return task
}

func (c *testClient) DeleteTaskT(ctx context.Context, tb testing.TB, req *pb.DeleteTaskRequest) {
	tb.Helper()
	_, err := c.DeleteTask(ctx, req)
	if err != nil {
		tb.Fatalf("DeleteTask(%v) err = %v; want nil", req, err)
	}
}
