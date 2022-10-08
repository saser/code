package testsuite

import (
	"context"
	"testing"

	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

type testClient struct {
	pb.TasksClient
}

///////////////////////////////////////////////////////////////////////////////
// Task operations.
///////////////////////////////////////////////////////////////////////////////

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

func (c *testClient) DeleteTaskT(ctx context.Context, tb testing.TB, req *pb.DeleteTaskRequest) *pb.Task {
	tb.Helper()
	task, err := c.DeleteTask(ctx, req)
	if err != nil {
		tb.Fatalf("DeleteTask(%v) err = %v; want nil", req, err)
	}
	return task
}

func (c *testClient) UndeleteTaskT(ctx context.Context, tb testing.TB, req *pb.UndeleteTaskRequest) *pb.Task {
	tb.Helper()
	task, err := c.UndeleteTask(ctx, req)
	if err != nil {
		tb.Fatalf("UndeleteTask(%v) err = %v; want nil", req, err)
	}
	return task
}

func (c *testClient) CompleteTaskT(ctx context.Context, tb testing.TB, req *pb.CompleteTaskRequest) *pb.Task {
	tb.Helper()
	task, err := c.CompleteTask(ctx, req)
	if err != nil {
		tb.Fatalf("CompleteTask(%v) err = %v; want nil", req, err)
	}
	return task
}

func (c *testClient) UncompleteTaskT(ctx context.Context, tb testing.TB, req *pb.UncompleteTaskRequest) *pb.Task {
	tb.Helper()
	task, err := c.UncompleteTask(ctx, req)
	if err != nil {
		tb.Fatalf("UncompleteTask(%v) err = %v; want nil", req, err)
	}
	return task
}

///////////////////////////////////////////////////////////////////////////////
// Project operations.
///////////////////////////////////////////////////////////////////////////////

func (c *testClient) GetProjectT(ctx context.Context, tb testing.TB, req *pb.GetProjectRequest) *pb.Project {
	tb.Helper()
	project, err := c.GetProject(ctx, req)
	if err != nil {
		tb.Fatalf("GetProject(%v) err = %v; want nil", req, err)
	}
	return project
}

func (c *testClient) ListProjectsT(ctx context.Context, tb testing.TB, req *pb.ListProjectsRequest) *pb.ListProjectsResponse {
	tb.Helper()
	res, err := c.ListProjects(ctx, req)
	if err != nil {
		tb.Fatalf("ListProjects(%v) err = %v; want nil", req, err)
	}
	return res
}

func (c *testClient) ListAllProjectsT(ctx context.Context, tb testing.TB, req *pb.ListProjectsRequest) []*pb.Project {
	tb.Helper()
	var projects []*pb.Project
	req = proto.Clone(req).(*pb.ListProjectsRequest)
	for {
		res := c.ListProjectsT(ctx, tb, req)
		projects = append(projects, res.GetProjects()...)
		token := res.GetNextPageToken()
		if token == "" {
			break
		}
		req.PageToken = token
	}
	return projects
}

func (c *testClient) CreateProjectT(ctx context.Context, tb testing.TB, req *pb.CreateProjectRequest) *pb.Project {
	tb.Helper()
	project, err := c.CreateProject(ctx, req)
	if err != nil {
		tb.Fatalf("CreateProject(%v) err = %v; want nil", req, err)
	}
	return project
}

func (c *testClient) CreateProjectsT(ctx context.Context, tb testing.TB, projects []*pb.Project) []*pb.Project {
	tb.Helper()
	var created []*pb.Project
	for _, project := range projects {
		created = append(created, c.CreateProjectT(ctx, tb, &pb.CreateProjectRequest{
			Project: project,
		}))
	}
	return created
}

func (c *testClient) UpdateProjectT(ctx context.Context, tb testing.TB, req *pb.UpdateProjectRequest) *pb.Project {
	tb.Helper()
	project, err := c.UpdateProject(ctx, req)
	if err != nil {
		tb.Fatalf("UpdateProject(%v) err = %v; want nil", req, err)
	}
	return project
}

func (c *testClient) DeleteProjectT(ctx context.Context, tb testing.TB, req *pb.DeleteProjectRequest) *pb.Project {
	tb.Helper()
	project, err := c.DeleteProject(ctx, req)
	if err != nil {
		tb.Fatalf("DeleteProject(%v) err = %v; want nil", req, err)
	}
	return project
}

func (c *testClient) UndeleteProjectT(ctx context.Context, tb testing.TB, req *pb.UndeleteProjectRequest) *pb.Project {
	tb.Helper()
	project, err := c.UndeleteProject(ctx, req)
	if err != nil {
		tb.Fatalf("UndeleteProject(%v) err = %v; want nil", req, err)
	}
	return project
}

func (c *testClient) ArchiveProjectT(ctx context.Context, tb testing.TB, req *pb.ArchiveProjectRequest) *pb.Project {
	tb.Helper()
	project, err := c.ArchiveProject(ctx, req)
	if err != nil {
		tb.Fatalf("ArchiveProject(%v) err = %v; want nil", req, err)
	}
	return project
}

func (c *testClient) UnarchiveProjectT(ctx context.Context, tb testing.TB, req *pb.UnarchiveProjectRequest) *pb.Project {
	tb.Helper()
	project, err := c.UnarchiveProject(ctx, req)
	if err != nil {
		tb.Fatalf("UnarchiveProject(%v) err = %v; want nil", req, err)
	}
	return project
}

///////////////////////////////////////////////////////////////////////////////
// Label operations.
///////////////////////////////////////////////////////////////////////////////

func (c *testClient) GetLabelT(ctx context.Context, tb testing.TB, req *pb.GetLabelRequest) *pb.Label {
	tb.Helper()
	label, err := c.GetLabel(ctx, req)
	if err != nil {
		tb.Fatalf("GetLabel(%v) err = %v; want nil", req, err)
	}
	return label
}

func (c *testClient) ListLabelsT(ctx context.Context, tb testing.TB, req *pb.ListLabelsRequest) *pb.ListLabelsResponse {
	tb.Helper()
	res, err := c.ListLabels(ctx, req)
	if err != nil {
		tb.Fatalf("ListLabels(%v) err = %v; want nil", req, err)
	}
	return res
}

func (c *testClient) ListAllLabelsT(ctx context.Context, tb testing.TB, req *pb.ListLabelsRequest) []*pb.Label {
	tb.Helper()
	var labels []*pb.Label
	req = proto.Clone(req).(*pb.ListLabelsRequest)
	for {
		res := c.ListLabelsT(ctx, tb, req)
		labels = append(labels, res.GetLabels()...)
		token := res.GetNextPageToken()
		if token == "" {
			break
		}
		req.PageToken = token
	}
	return labels
}

func (c *testClient) CreateLabelT(ctx context.Context, tb testing.TB, req *pb.CreateLabelRequest) *pb.Label {
	tb.Helper()
	label, err := c.CreateLabel(ctx, req)
	if err != nil {
		tb.Fatalf("CreateLabel(%v) err = %v; want nil", req, err)
	}
	return label
}

func (c *testClient) CreateLabelsT(ctx context.Context, tb testing.TB, labels []*pb.Label) []*pb.Label {
	tb.Helper()
	var created []*pb.Label
	for _, label := range labels {
		created = append(created, c.CreateLabelT(ctx, tb, &pb.CreateLabelRequest{
			Label: label,
		}))
	}
	return created
}

func (c *testClient) UpdateLabelT(ctx context.Context, tb testing.TB, req *pb.UpdateLabelRequest) *pb.Label {
	tb.Helper()
	label, err := c.UpdateLabel(ctx, req)
	if err != nil {
		tb.Fatalf("UpdateLabel(%v) err = %v; want nil", req, err)
	}
	return label
}

func (c *testClient) DeleteLabelT(ctx context.Context, tb testing.TB, req *pb.DeleteLabelRequest) *emptypb.Empty {
	tb.Helper()
	empty, err := c.DeleteLabel(ctx, req)
	if err != nil {
		tb.Fatalf("DeleteLabel(%v) err = %v; want nil", req, err)
	}
	return empty
}
