package service

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.saser.se/postgres/postgrestest"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func taskLessFunc(t1, t2 *pb.Task) bool {
	return t1.GetName() < t2.GetName()
}

type testClient struct {
	pb.TasksClient
}

func (c *testClient) GetTaskT(ctx context.Context, t *testing.T, req *pb.GetTaskRequest) *pb.Task {
	t.Helper()
	task, err := c.GetTask(ctx, req)
	if err != nil {
		t.Fatalf("GetTask(%v) err = %v; want nil", req, err)
	}
	return task
}

func (c *testClient) ListTasksT(ctx context.Context, t *testing.T, req *pb.ListTasksRequest) *pb.ListTasksResponse {
	t.Helper()
	res, err := c.ListTasks(ctx, req)
	if err != nil {
		t.Fatalf("ListTasks(%v) err = %v; want nil", req, err)
	}
	return res
}

func (c *testClient) ListAllTasksT(ctx context.Context, t *testing.T, req *pb.ListTasksRequest) []*pb.Task {
	t.Helper()
	var tasks []*pb.Task
	req = proto.Clone(req).(*pb.ListTasksRequest)
	for {
		res := c.ListTasksT(ctx, t, req)
		tasks = append(tasks, res.GetTasks()...)
		token := res.GetNextPageToken()
		if token == "" {
			break
		}
		req.PageToken = token
	}
	return tasks
}

func (c *testClient) CreateTaskT(ctx context.Context, t *testing.T, req *pb.CreateTaskRequest) *pb.Task {
	t.Helper()
	task, err := c.CreateTask(ctx, req)
	if err != nil {
		t.Fatalf("CreateTask(%v) err = %v; want nil", req, err)
	}
	return task
}

func (c *testClient) CreateTasksT(ctx context.Context, t *testing.T, tasks []*pb.Task) []*pb.Task {
	t.Helper()
	var created []*pb.Task
	for _, task := range tasks {
		created = append(created, c.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
			Task: task,
		}))
	}
	return created
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
	svc := New(postgrestest.Open(ctx, t, "tasks/postgres/schema.sql"))
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

func TestService_GetTask(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := setup(ctx, t)

	task := c.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:       "Get this",
			Description: "Be sure to get this!!!",
		},
	})

	// Getting the task by name should produce the same result.
	req := &pb.GetTaskRequest{
		Name: task.GetName(),
	}
	got, err := c.GetTask(ctx, req)
	if err != nil {
		t.Fatalf("GetTask(%v) err = %v; want nil", req, err)
	}
	if diff := cmp.Diff(task, got, protocmp.Transform()); diff != "" {
		t.Errorf("GetTask(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func TestService_GetTask_AfterDeletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := setup(ctx, t)

	task := c.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:       "Get this",
			Description: "Be sure to get this!!!",
		},
	})

	// Getting the task by name should produce the same result.
	{
		req := &pb.GetTaskRequest{
			Name: task.GetName(),
		}
		got := c.GetTaskT(ctx, t, req)
		if diff := cmp.Diff(task, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetTask(%v): unexpected result (-want +got)\n%s", req, diff)
		}
	}

	// After deleting the task, getting the task by name should produce a "not
	// found" error.
	{
		c.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
			Name: task.GetName(),
		})

		req := &pb.GetTaskRequest{
			Name: task.GetName(),
		}
		_, err := c.GetTask(ctx, req)
		if got, want := status.Code(err), codes.NotFound; got != want {
			t.Errorf("GetTask(%v) code = %v; want %v", req, got, want)
		}
	}
}

func TestService_GetTask_Error(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := setup(ctx, t)
	for _, tt := range []struct {
		name string
		req  *pb.GetTaskRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req:  &pb.GetTaskRequest{Name: ""},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName",
			req:  &pb.GetTaskRequest{Name: "invalid/123"},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req:  &pb.GetTaskRequest{Name: "tasks/999"},
			want: codes.NotFound,
		},
		{
			name: "NotFound_DifferentResourceIDFormat",
			req: &pb.GetTaskRequest{
				// This is a valid name -- there is no guarantee what format the
				// resource ID (the segment after the slash) will have. But it
				// probably won't be arbitrary strings.
				Name: "tasks/invalidlol",
			},
			want: codes.NotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.GetTask(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("GetTask(%v) code = %v; want %v", tt.req, got, tt.want)
			}
		})
	}
}

func TestService_ListTasks(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := setup(ctx, t)

	want := c.CreateTasksT(ctx, t, []*pb.Task{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	req := &pb.ListTasksRequest{
		PageSize: int32(len(want)),
	}
	res, err := c.ListTasks(ctx, req)
	if err != nil {
		t.Fatalf("ListTasks(%v) err = %v; want nil", req, err)
	}
	if diff := cmp.Diff(want, res.GetTasks(), protocmp.Transform(), cmpopts.SortSlices(taskLessFunc)); diff != "" {
		t.Errorf("ListTasks(%v): unexpected result (-want +got)\n%s", req, diff)
	}
	if got, want := res.GetNextPageToken(), ""; got != want {
		t.Errorf("ListTasks(%v) next_page_token = %q; want %q", req, got, want)
	}
}

func TestService_ListTasks_MaxPageSize(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := setup(ctx, t)

	tasks := make([]*pb.Task, maxPageSize*2-maxPageSize/2)
	for i := range tasks {
		tasks[i] = &pb.Task{
			Title: fmt.Sprint(i),
		}
	}
	tasks = c.CreateTasksT(ctx, t, tasks)

	req := &pb.ListTasksRequest{
		PageSize: int32(len(tasks)), // more than maxPageSize
	}

	res := c.ListTasksT(ctx, t, req)
	wantFirstPage := tasks[:maxPageSize]
	if diff := cmp.Diff(wantFirstPage, res.GetTasks(), protocmp.Transform(), cmpopts.SortSlices(taskLessFunc)); diff != "" {
		t.Errorf("[first page] ListTasks(%v): unexpected result (-want +got)\n%s", req, diff)
	}

	req.PageToken = res.GetNextPageToken()
	res = c.ListTasksT(ctx, t, req)
	wantSecondPage := tasks[maxPageSize:]
	if diff := cmp.Diff(wantSecondPage, res.GetTasks(), protocmp.Transform(), cmpopts.SortSlices(taskLessFunc)); diff != "" {
		t.Errorf("[second page] ListTasks(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func TestService_ListTasks_DifferentPageSizes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := setup(ctx, t)

	// 7 tasks. Number chosen arbitrarily.
	tasks := c.CreateTasksT(ctx, t, []*pb.Task{
		{Title: "Buy milk"},
		{Title: "Make pancakes"},
		{Title: "Read a book"},
		{Title: "Get swole"},
		{Title: "Drink water"},
		{Title: "Get even swoler"},
		{Title: "Order a new chair"},
	})
	for _, sizes := range [][]int32{
		{1, 1, 1, 1, 1, 1, 1},
		{7},
		{8},
		{1, 6},
		{6, 1},
		{6, 7},
		{1, 7},
		{2, 2, 2, 2},
	} {
		sizes := sizes
		t.Run(fmt.Sprint(sizes), func(t *testing.T) {
			t.Parallel()
			// Sanity check: make sure the sizes add up to at least the number
			// of tasks, and that we won't try to get more pages after the last one.
			{
				sum := int32(0)
				for i, s := range sizes {
					if s <= 0 {
						t.Errorf("sizes[%d] = %v; want a positive number", i, s)
					}
					sum += s
				}
				n := int32(len(tasks))
				if sum < n {
					t.Errorf("sum(%v) = %v; want at least %v", sizes, sum, n)
				}
				if subsum := sum - sizes[len(sizes)-1]; subsum > n {
					t.Errorf("[everything except last element] sum(%v) = %v; want less than %v", sizes[:len(sizes)-1], subsum, n)
				}
				if t.Failed() {
					t.FailNow()
				}
			}
			// Now we can start listing tasks.
			req := &pb.ListTasksRequest{}
			var got []*pb.Task
			for i, s := range sizes {
				req.PageSize = s
				res := c.ListTasksT(ctx, t, req)
				got = append(got, res.GetTasks()...)
				token := res.GetNextPageToken()
				if i < len(sizes)-1 && token == "" {
					// This error does not apply for the last page.
					t.Fatalf("[after page %d]: ListTasks(%v) next_page_token = %q; want non-empty", i, req, token)
				}
				req.PageToken = token
			}
			// After all the page sizes the page token should be empty.
			if got, want := req.GetPageToken(), ""; got != want {
				t.Fatalf("[after all pages] page_token = %q; want %q", got, want)
			}
			if diff := cmp.Diff(tasks, got, protocmp.Transform(), cmpopts.SortSlices(taskLessFunc)); diff != "" {
				t.Errorf("unexpected result (-want +got)\n%s", diff)
			}
		})
	}
}

func TestService_ListTasks_WithDeletions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	seed := []*pb.Task{
		{Title: "First task"},
		{Title: "Second task"},
		{Title: "Third task"},
	}

	for _, tt := range []struct {
		name                  string
		firstPageSize         int32
		wantFirstPageIndices  []int // indices into created tasks
		deleteIndex           int
		wantSecondPageIndices []int // indices into created tasks
	}{
		{
			name:                  "DeleteInFirstPage_TwoTasksInFirstPage",
			firstPageSize:         2,
			wantFirstPageIndices:  []int{0, 1},
			deleteIndex:           1,
			wantSecondPageIndices: []int{2},
		},
		{
			name:                  "DeleteInFirstPage_OneTaskInFirstPage",
			firstPageSize:         1,
			wantFirstPageIndices:  []int{0},
			deleteIndex:           0,
			wantSecondPageIndices: []int{1, 2},
		},
		{
			name:                  "DeleteInSecondPage_DeleteFirst",
			firstPageSize:         1,
			wantFirstPageIndices:  []int{0},
			deleteIndex:           1,
			wantSecondPageIndices: []int{2},
		},
		{
			name:                  "DeleteInSecondPage_DeleteSecond",
			firstPageSize:         1,
			wantFirstPageIndices:  []int{0},
			deleteIndex:           2,
			wantSecondPageIndices: []int{1},
		},
		{
			name:                  "DeleteInSecondPage_TwoTasksInFirstPage",
			firstPageSize:         2,
			wantFirstPageIndices:  []int{0, 1},
			deleteIndex:           2,
			wantSecondPageIndices: []int{},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := setup(ctx, t)
			tasks := c.CreateTasksT(ctx, t, seed)

			// Get the first page and assert that it matches what we want.
			req := &pb.ListTasksRequest{
				PageSize: tt.firstPageSize,
			}
			res := c.ListTasksT(ctx, t, req)
			wantFirstPage := make([]*pb.Task, 0, len(tt.wantFirstPageIndices))
			for _, idx := range tt.wantFirstPageIndices {
				wantFirstPage = append(wantFirstPage, tasks[idx])
			}
			if diff := cmp.Diff(wantFirstPage, res.GetTasks(), protocmp.Transform(), protocmp.SortRepeated(taskLessFunc)); diff != "" {
				t.Fatalf("first page: unexpected tasks (-want +got)\n%s", diff)
			}
			token := res.GetNextPageToken()
			if token == "" {
				t.Fatal("no next page token from first page")
			}
			req.PageToken = token

			// Delete one of the tasks.
			c.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
				Name: tasks[tt.deleteIndex].GetName(),
			})

			// Get the second page and assert that it matches what we want. Also
			// assert that there are no more tasks.
			req.PageSize = int32(len(tasks)) // Make sure we get the remaining tasks.
			res = c.ListTasksT(ctx, t, req)
			wantSecondPage := make([]*pb.Task, 0, len(tt.wantSecondPageIndices))
			for _, idx := range tt.wantSecondPageIndices {
				wantSecondPage = append(wantSecondPage, tasks[idx])
			}
			if diff := cmp.Diff(wantSecondPage, res.GetTasks(), cmpopts.EquateEmpty(), protocmp.Transform(), protocmp.SortRepeated(taskLessFunc)); diff != "" {
				t.Fatalf("second page: unexpected tasks (-want +got)\n%s", diff)
			}
			if got, want := res.GetNextPageToken(), ""; got != want {
				t.Errorf("second page: next_page_token = %q; want %q", got, want)
			}
		})
	}
}

func TestService_ListTasks_WithAdditions(t *testing.T) {
	ctx := context.Background()
	c := setup(ctx, t)

	tasks := c.CreateTasksT(ctx, t, []*pb.Task{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	firstPageSize := len(tasks) - 1

	// Get the first page.
	res := c.ListTasksT(ctx, t, &pb.ListTasksRequest{
		PageSize: int32(firstPageSize), // Make sure we don't get everything.
	})
	wantFirstPage := tasks[:firstPageSize]
	if diff := cmp.Diff(wantFirstPage, res.GetTasks(), protocmp.Transform(), protocmp.SortRepeated(taskLessFunc)); diff != "" {
		t.Errorf("unexpected first page (-want +got)\n%s", diff)
	}
	token := res.GetNextPageToken()
	if token == "" {
		t.Fatalf("first page returned empty next_page_token")
	}

	// Add a new task.
	tasks = append(tasks, c.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{Title: "Feed sourdough"},
	}))

	// Get the second page, which should contain the new task.
	res = c.ListTasksT(ctx, t, &pb.ListTasksRequest{
		PageSize:  int32(len(tasks)), // Try to make sure we get everything.
		PageToken: token,
	})
	wantSecondPage := tasks[firstPageSize:]
	if diff := cmp.Diff(wantSecondPage, res.GetTasks(), protocmp.Transform(), protocmp.SortRepeated(taskLessFunc)); diff != "" {
		t.Errorf("unexpected second page (-want +got)\n%s", diff)
	}
	if got, want := res.GetNextPageToken(), ""; got != want {
		t.Errorf("second page: next_page_token = %q; want %q", got, want)
	}
}

func TestService_ListTasks_SamePageTokenTwice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	c := setup(ctx, t)

	tasks := c.CreateTasksT(ctx, t, []*pb.Task{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	// Get the first page.
	res := c.ListTasksT(ctx, t, &pb.ListTasksRequest{
		PageSize: int32(len(tasks) - 1), // Make sure we need at least one other page.
	})
	wantFirstPage := tasks[:len(tasks)-1]
	if diff := cmp.Diff(wantFirstPage, res.GetTasks(), protocmp.Transform(), protocmp.SortRepeated(taskLessFunc)); diff != "" {
		t.Errorf("unexpected first page (-want +got)\n%s", diff)
	}
	token := res.GetNextPageToken()
	if token == "" {
		t.Fatalf("first page returned empty next_page_token")
	}

	// Get the second page.
	req := &pb.ListTasksRequest{
		PageSize:  int32(len(tasks)), // Make sure we try to get everything.
		PageToken: token,
	}
	res = c.ListTasksT(ctx, t, req)
	wantSecondPage := tasks[len(tasks)-1:]
	if diff := cmp.Diff(wantSecondPage, res.GetTasks(), protocmp.Transform(), protocmp.SortRepeated(taskLessFunc)); diff != "" {
		t.Errorf("unexpected second page (-want +got)\n%s", diff)
	}
	if got, want := res.GetNextPageToken(), ""; got != want {
		t.Errorf("second page: next_page_token = %q; want %q", got, want)
	}

	// Now try getting the second page again. This shouldn't work -- the last
	// page token should have been "consumed".
	_, err := c.ListTasks(ctx, req)
	if got, want := status.Code(err), codes.InvalidArgument; got != want {
		t.Errorf("second page again: return code = %v; want %v", got, want)
	}
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
	if err := got.GetCreateTime().CheckValid(); err != nil {
		t.Errorf("got.GetCreateTime() is invalid: %v", err)
	}
	if got, want := got.GetCreateTime().AsTime().IsZero(), false; got != want {
		t.Errorf("got.GetCreateTime().AsTime().IsZero() = %v; want %v", got, want)
	}
	if diff := cmp.Diff(task, got, protocmp.Transform(), protocmp.IgnoreFields(task, "name", "create_time")); diff != "" {
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
			req:  &pb.DeleteTaskRequest{Name: "tasks/notfound"},
			want: codes.NotFound,
		},
		{
			name: "InvalidName",
			req:  &pb.DeleteTaskRequest{Name: "invalidlololol/1"},
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
