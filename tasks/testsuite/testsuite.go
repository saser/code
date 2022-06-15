package testsuite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func taskLessFunc(t1, t2 *pb.Task) bool {
	return t1.GetName() < t2.GetName()
}

// Suite contains a suite of tests for an implementation of Tasks service.
type Suite struct {
	suite.Suite

	client      *testClient
	truncater   Truncater
	clock       clockwork.FakeClock
	maxPageSize int
}

// Truncater defines Truncate, a method used to clear out any backing data
// stores.
type Truncater interface {
	// Truncate should clear out any backing data stores used by the
	// implementation under test. Resetting things like ID counters is allowed
	// but not necessary.
	Truncate(ctx context.Context) error
}

// New constructs a new test suite. The truncater should be connected to the
// same data stores as the implementation under test.
func New(client pb.TasksClient, truncater Truncater, clock clockwork.FakeClock, maxPageSize int) *Suite {
	return &Suite{
		client:      &testClient{TasksClient: client},
		truncater:   truncater,
		clock:       clock,
		maxPageSize: maxPageSize,
	}
}

// TearDownTest will truncate all backing data stores after every test.
func (s *Suite) TearDownTest() {
	s.truncate(context.Background())
}

func (s *Suite) truncate(ctx context.Context) {
	t := s.T()
	t.Helper()
	if err := s.truncater.Truncate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

///////////////////////////////////////////////////////////////////////////////
// Actual tests start below.
///////////////////////////////////////////////////////////////////////////////

func (s *Suite) TestGetTask() {
	t := s.T()
	ctx := context.Background()

	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:       "Get this",
			Description: "Be sure to get this!!!",
		},
	})

	// Getting the task by name should produce the same result.
	req := &pb.GetTaskRequest{
		Name: task.GetName(),
	}
	got, err := s.client.GetTask(ctx, req)
	if err != nil {
		t.Fatalf("GetTask(%v) err = %v; want nil", req, err)
	}
	if diff := cmp.Diff(task, got, protocmp.Transform()); diff != "" {
		t.Errorf("GetTask(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func (s *Suite) TestGetTask_AfterDeletion() {
	t := s.T()
	ctx := context.Background()

	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
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
		got := s.client.GetTaskT(ctx, t, req)
		if diff := cmp.Diff(task, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetTask(%v): unexpected result (-want +got)\n%s", req, diff)
		}
	}

	// After soft deleting the task, getting the task by name should succeed and
	// produce the same task.
	{
		want := s.client.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
			Name: task.GetName(),
		})
		task = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
			Name: task.GetName(),
		})
		if diff := cmp.Diff(want, task, protocmp.Transform()); diff != "" {
			t.Errorf("GetTask: unexpected result of getting soft deleted task (-want +got)\n%s", diff)
		}
	}

	// After the task has expired we shouldn't be able to get it anymore.
	s.clock.Advance(task.GetExpiryTime().AsTime().Sub(s.clock.Now()))
	s.clock.Advance(1 * time.Minute)
	req := &pb.GetTaskRequest{
		Name: task.GetName(),
	}
	_, err := s.client.GetTask(ctx, req)
	if got, want := status.Code(err), codes.NotFound; got != want {
		t.Errorf("after expiration: GetTask(%v) code = %v; want %v", req, got, want)
		t.Logf("err = %v", err)
	}
}

func (s *Suite) TestGetTask_Error() {
	t := s.T()
	ctx := context.Background()
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
			name: "InvalidName_NoResourceID",
			req: &pb.GetTaskRequest{
				Name: "tasks/",
			},
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
			_, err := s.client.GetTask(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("GetTask(%v) code = %v; want %v", tt.req, got, tt.want)
			}
		})
	}
}

func (s *Suite) TestListTasks() {
	t := s.T()
	ctx := context.Background()

	want := s.client.CreateTasksT(ctx, t, []*pb.Task{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	req := &pb.ListTasksRequest{
		PageSize: int32(len(want)),
	}
	res, err := s.client.ListTasks(ctx, req)
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

func (s *Suite) TestListTasks_MaxPageSize() {
	t := s.T()
	ctx := context.Background()

	tasks := make([]*pb.Task, s.maxPageSize*2-s.maxPageSize/2)
	for i := range tasks {
		tasks[i] = s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
			Task: &pb.Task{
				Title: fmt.Sprint(i),
			},
		})
	}

	req := &pb.ListTasksRequest{
		PageSize: int32(len(tasks)), // more than maxPageSize
	}

	res := s.client.ListTasksT(ctx, t, req)
	wantFirstPage := tasks[:s.maxPageSize]
	if diff := cmp.Diff(wantFirstPage, res.GetTasks(), protocmp.Transform(), cmpopts.SortSlices(taskLessFunc)); diff != "" {
		t.Errorf("[first page] ListTasks(%v): unexpected result (-want +got)\n%s", req, diff)
	}

	req.PageToken = res.GetNextPageToken()
	res = s.client.ListTasksT(ctx, t, req)
	wantSecondPage := tasks[s.maxPageSize:]
	if diff := cmp.Diff(wantSecondPage, res.GetTasks(), protocmp.Transform(), cmpopts.SortSlices(taskLessFunc)); diff != "" {
		t.Errorf("[second page] ListTasks(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func (s *Suite) TestListTasks_DifferentPageSizes() {
	t := s.T()
	ctx := context.Background()

	// 7 tasks. Number chosen arbitrarily.
	tasks := s.client.CreateTasksT(ctx, t, []*pb.Task{
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
			for i, size := range sizes {
				req.PageSize = size
				res := s.client.ListTasksT(ctx, t, req)
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

func (s *Suite) TestListTasks_WithDeletions() {
	t := s.T()
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
			defer s.truncate(ctx)
			tasks := s.client.CreateTasksT(ctx, t, seed)

			// Get the first page and assert that it matches what we want.
			req := &pb.ListTasksRequest{
				PageSize: tt.firstPageSize,
			}
			res := s.client.ListTasksT(ctx, t, req)
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
			s.client.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
				Name: tasks[tt.deleteIndex].GetName(),
			})

			// Get the second page and assert that it matches what we want. Also
			// assert that there are no more tasks.
			req.PageSize = int32(len(tasks)) // Make sure we get the remaining tasks.
			res = s.client.ListTasksT(ctx, t, req)
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

func (s *Suite) TestListTasks_WithDeletions_ShowDeleted() {
	t := s.T()
	ctx := context.Background()

	want := s.client.CreateTasksT(ctx, t, []*pb.Task{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	// Soft delete one of the tasks.
	want[1] = s.client.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
		Name: want[1].GetName(),
	})

	// Listing the tasks with show_deleted = true should include the soft
	// deleted task.
	got := s.client.ListAllTasksT(ctx, t, &pb.ListTasksRequest{
		PageSize:    int32(len(want)),
		ShowDeleted: true,
	})
	if diff := cmp.Diff(want, got, protocmp.Transform(), cmpopts.SortSlices(taskLessFunc)); diff != "" {
		t.Errorf("unexpected result of ListTasks with show_deleted = true (-want +got)\n%s", diff)
	}

	// After the soft deleted task has expired it should no longer show up in
	// ListTasks.
	s.clock.Advance(want[1].GetExpiryTime().AsTime().Sub(s.clock.Now()))
	s.clock.Advance(1 * time.Minute)
	wantAfterExpiry := []*pb.Task{
		want[0],
		want[2],
	}
	got = s.client.ListAllTasksT(ctx, t, &pb.ListTasksRequest{
		PageSize:    int32(len(want)),
		ShowDeleted: true,
	})
	if diff := cmp.Diff(wantAfterExpiry, got, protocmp.Transform(), cmpopts.SortSlices(taskLessFunc)); diff != "" {
		t.Errorf("after expiration: unexpected result of ListTasks with show_deleted = true (-want +got)\n%s", diff)
	}
}

func (s *Suite) TestListTasks_WithAdditions() {
	t := s.T()
	ctx := context.Background()

	tasks := s.client.CreateTasksT(ctx, t, []*pb.Task{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	firstPageSize := len(tasks) - 1

	// Get the first page.
	res := s.client.ListTasksT(ctx, t, &pb.ListTasksRequest{
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
	tasks = append(tasks, s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{Title: "Feed sourdough"},
	}))

	// Get the second page, which should contain the new task.
	res = s.client.ListTasksT(ctx, t, &pb.ListTasksRequest{
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

func (s *Suite) TestListTasks_SamePageTokenTwice() {
	t := s.T()
	ctx := context.Background()

	tasks := s.client.CreateTasksT(ctx, t, []*pb.Task{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	// Get the first page.
	res := s.client.ListTasksT(ctx, t, &pb.ListTasksRequest{
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
	res = s.client.ListTasksT(ctx, t, req)
	wantSecondPage := tasks[len(tasks)-1:]
	if diff := cmp.Diff(wantSecondPage, res.GetTasks(), protocmp.Transform(), protocmp.SortRepeated(taskLessFunc)); diff != "" {
		t.Errorf("unexpected second page (-want +got)\n%s", diff)
	}
	if got, want := res.GetNextPageToken(), ""; got != want {
		t.Errorf("second page: next_page_token = %q; want %q", got, want)
	}

	// Now try getting the second page again. This shouldn't work -- the last
	// page token should have been "consumed".
	_, err := s.client.ListTasks(ctx, req)
	if got, want := status.Code(err), codes.InvalidArgument; got != want {
		t.Errorf("second page again: return code = %v; want %v", got, want)
	}
}

func (s *Suite) TestListTasks_ChangeRequestBetweenPages() {
	t := s.T()
	ctx := context.Background()

	tasks := s.client.CreateTasksT(ctx, t, []*pb.Task{
		{Title: "Buy milk"},
		{Title: "Get swole"},
	})

	req := &pb.ListTasksRequest{
		PageSize:    1,
		ShowDeleted: false,
	}

	// Getting the first page should succeed without problems.
	{
		res := s.client.ListTasksT(ctx, t, req)
		want := tasks[:1]
		if diff := cmp.Diff(want, res.GetTasks(), protocmp.Transform(), cmpopts.SortSlices(taskLessFunc)); diff != "" {
			t.Errorf("first page: unexpected results (-want +got)\n%s", diff)
		}
		req.PageToken = res.GetNextPageToken()
	}

	// Now we change the request parameters between pages, which should cause an error.
	req.ShowDeleted = true
	_, err := s.client.ListTasks(ctx, req)
	if got, want := status.Code(err), codes.InvalidArgument; got != want {
		t.Errorf("after changing request: ListTasks(%v) code = %v; want %v", req, got, want)
		t.Logf("err = %v", err)
	}
}

// Regression test for a bug. The Postgres implementation had didn't set the
// `update_time` and `completed` fields correctly when listing tasks.
func (s *Suite) TestListTasks_IncludesCompleted() {
	t := s.T()
	ctx := context.Background()

	tasks := s.client.CreateTasksT(ctx, t, []*pb.Task{
		{Title: "kick ass"},
		{Title: "chew bubblegum"},
	})
	s.clock.Advance(30 * time.Hour)
	tasks[0] = s.client.CompleteTaskT(ctx, t, &pb.CompleteTaskRequest{Name: tasks[0].GetName()})

	res := s.client.ListTasksT(ctx, t, &pb.ListTasksRequest{})
	less := func(t1, t2 *pb.Task) bool { return t1.GetName() < t2.GetName() }
	if diff := cmp.Diff(tasks, res.GetTasks(), protocmp.Transform(), cmpopts.SortSlices(less)); diff != "" {
		t.Fatalf("Unexpected diff when listing tasks (-want +got)\n%s", diff)
	}
}

func (s *Suite) TestListTasks_Error() {
	t := s.T()
	ctx := context.Background()
	for _, tt := range []struct {
		name string
		req  *pb.ListTasksRequest
		want codes.Code
	}{
		{
			name: "NegativePageSize",
			req: &pb.ListTasksRequest{
				PageSize: -10,
			},
			want: codes.InvalidArgument,
		},
		{
			name: "BogusPageToken",
			req: &pb.ListTasksRequest{
				PageToken: "this is some complete bonkers",
			},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.ListTasks(ctx, tt.req)
			if got, want := status.Code(err), tt.want; got != want {
				t.Errorf("ListTasks(%v) code = %v; want %v", tt.req, got, want)
				t.Logf("err = %v", err)
			}
		})
	}
}

func (s *Suite) TestCreateTask() {
	t := s.T()
	ctx := context.Background()

	task := &pb.Task{Title: "Hello Tasks"}
	req := &pb.CreateTaskRequest{
		Task: task,
	}
	got, err := s.client.CreateTask(ctx, req)
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

func (s *Suite) TestCreateTask_WithParent() {
	t := s.T()
	ctx := context.Background()

	parent := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{Title: "Parent task"},
	})

	child := &pb.Task{
		Title:  "Child task",
		Parent: parent.GetName(),
	}
	req := &pb.CreateTaskRequest{
		Task: child,
	}
	got, err := s.client.CreateTask(ctx, req)
	if err != nil {
		t.Fatalf("CreateTask(%v) err = %v; want nil", req, err)
	}
	if err := got.GetCreateTime().CheckValid(); err != nil {
		t.Errorf("got.GetCreateTime() is invalid: %v", err)
	}
	if got, want := got.GetCreateTime().AsTime().IsZero(), false; got != want {
		t.Errorf("got.GetCreateTime().AsTime().IsZero() = %v; want %v", got, want)
	}
	if diff := cmp.Diff(child, got, protocmp.Transform(), protocmp.IgnoreFields(child, "name", "create_time")); diff != "" {
		t.Errorf("CreateTask(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func (s *Suite) TestCreateTask_Error() {
	t := s.T()
	ctx := context.Background()
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
		{
			name: "InvalidParent",
			req: &pb.CreateTaskRequest{
				Task: &pb.Task{
					Title:  "Invalid parent",
					Parent: "foobar/123",
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFoundParent_TextResourceID",
			req: &pb.CreateTaskRequest{
				Task: &pb.Task{
					Title:  "Parent doesn't exist",
					Parent: "tasks/notfound",
				},
			},
			want: codes.NotFound,
		},
		{
			name: "NotFoundParent_NumericalResourceID",
			req: &pb.CreateTaskRequest{
				Task: &pb.Task{
					Title:  "Parent doesn't exist",
					Parent: "tasks/999",
				},
			},
			want: codes.NotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.CreateTask(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("CreateTask(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}
		})
	}
}

func (s *Suite) TestUpdateTask() {
	t := s.T()
	ctx := context.Background()
	// Clock will be reset to createTime before the task is created.
	createTime := s.clock.Now()
	createTimeMessage := timestamppb.New(createTime)
	// Clock will be advanced to updateTime before the task is updated but after
	// it has been created.
	updateTime := createTime.Add(30 * time.Minute)
	updateTimeMessage := timestamppb.New(updateTime)
	for _, tt := range []struct {
		name string
		task *pb.Task
		req  *pb.UpdateTaskRequest // will be updated in-place with the created task name
		want *pb.Task              // will be updated in-place with the created task name
	}{
		{
			name: "EmptyUpdate_NilUpdateMask",
			task: &pb.Task{
				Title: "Before the update",
			},
			req: &pb.UpdateTaskRequest{
				Task:       &pb.Task{},
				UpdateMask: nil,
			},
			want: &pb.Task{
				Title:      "Before the update",
				CreateTime: createTimeMessage,
				UpdateTime: nil, // Task shouldn't be updated.
			},
		},
		{
			name: "EmptyUpdate_EmptyUpdateMask",
			task: &pb.Task{
				Title: "Before the update",
			},
			req: &pb.UpdateTaskRequest{
				Task:       &pb.Task{},
				UpdateMask: &fieldmaskpb.FieldMask{},
			},
			want: &pb.Task{
				Title:      "Before the update",
				CreateTime: createTimeMessage,
				UpdateTime: nil, // Task shouldn't be updated.
			},
		},
		{
			name: "UpdateTitle_NilUpdateMask",
			task: &pb.Task{
				Title: "Before the update",
			},
			req: &pb.UpdateTaskRequest{
				Task:       &pb.Task{Title: "After the update"},
				UpdateMask: nil,
			},
			want: &pb.Task{
				Title:      "After the update",
				CreateTime: createTimeMessage,
				UpdateTime: updateTimeMessage,
			},
		},
		{
			name: "UpdateTitle_EmptyUpdateMask",
			task: &pb.Task{
				Title: "Before the update",
			},
			req: &pb.UpdateTaskRequest{
				Task:       &pb.Task{Title: "After the update"},
				UpdateMask: &fieldmaskpb.FieldMask{},
			},
			want: &pb.Task{
				Title:      "After the update",
				CreateTime: createTimeMessage,
				UpdateTime: updateTimeMessage,
			},
		},
		{
			name: "UpdateTitle_MultipleFieldsPresent",
			task: &pb.Task{
				Title: "Before the update",
			},
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Title:       "After the update",
					Description: "You should never see this",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"title"},
				},
			},
			want: &pb.Task{
				Title:      "After the update",
				CreateTime: createTimeMessage,
				UpdateTime: updateTimeMessage,
			},
		},
		{
			name: "UpdateMultipleFields_NilUpdateMask",
			task: &pb.Task{
				Title: "Before the update",
			},
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Title:       "After the update",
					Description: "Added a description",
				},
				UpdateMask: nil,
			},
			want: &pb.Task{
				Title:       "After the update",
				Description: "Added a description",
				CreateTime:  createTimeMessage,
				UpdateTime:  updateTimeMessage,
			},
		},
		{
			name: "UpdateMultipleFields_EmptyUpdateMask",
			task: &pb.Task{
				Title: "Before the update",
			},
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Title:       "After the update",
					Description: "Added a description",
				},
				UpdateMask: &fieldmaskpb.FieldMask{},
			},
			want: &pb.Task{
				Title:       "After the update",
				Description: "Added a description",
				CreateTime:  createTimeMessage,
				UpdateTime:  updateTimeMessage,
			},
		},
		{
			name: "UpdateMultipleFields_NonEmptyUpdateMask",
			task: &pb.Task{
				Title: "Before the update",
			},
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Title:       "After the update",
					Description: "Added a description",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{
						"title",
						"description",
					},
				},
			},
			want: &pb.Task{
				Title:       "After the update",
				Description: "Added a description",
				CreateTime:  createTimeMessage,
				UpdateTime:  updateTimeMessage,
			},
		},
		{
			name: "UpdateMultipleFields_StarMask",
			task: &pb.Task{
				Title: "Before the update",
			},
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Title:       "After the update",
					Description: "Added a description",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"*"},
				},
			},
			want: &pb.Task{
				Title:       "After the update",
				Description: "Added a description",
				CreateTime:  createTimeMessage,
				UpdateTime:  updateTimeMessage,
			},
		},
		{
			// An empty/default value for `description` with a wildcard update
			// mask should result in description being cleared.
			name: "RemoveDescription",
			task: &pb.Task{
				Title:       "Before the update",
				Description: "This is a description",
			},
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Title:       "After the update",
					Description: "",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"*"},
				},
			},
			want: &pb.Task{
				Title:       "After the update",
				Description: "",
				CreateTime:  createTimeMessage,
				UpdateTime:  updateTimeMessage,
			},
		},
		{
			// Trying to update the task with identical values should be a
			// no-op. This should be indicated by a missing `update_time` value.
			name: "IdenticalTitle",
			task: &pb.Task{
				Title: "Before the update",
			},
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Title:       "Before the update",
					Description: "",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"title"},
				},
			},
			want: &pb.Task{
				Title:      "Before the update",
				CreateTime: createTimeMessage,
				UpdateTime: nil, // Task shouldn't be updated.
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// We need to reset the time to creatTime.
			// We want to find `d` such that `now + d = createTime.`
			// Therefore `d = createTime - now.`
			s.clock.Advance(createTime.Sub(s.clock.Now()))

			task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
				Task: tt.task,
			})

			// Before we do the update we advance time, so that `update_time` is
			// not the same as `create_time`.
			s.clock.Advance(30 * time.Minute)

			// Below we do the actual update.
			tt.req.Task.Name = task.GetName()
			tt.want.Name = task.GetName()
			got := s.client.UpdateTaskT(ctx, t, tt.req)
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected result of update (-want +got)\n%s", diff)
			}
			// Getting the task again should produce the same result as after
			// the update.
			got = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
				Name: task.GetName(),
			})
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected result of GetTask after update (-want +got)\n%s", diff)
			}
		})
	}
}

func (s *Suite) TestUpdateTask_WithChildren() {
	t := s.T()
	ctx := context.Background()

	parent := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "parent",
		},
	})
	child := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Parent: parent.GetName(),
			Title:  "child",
		},
	})

	// Ensure update happens after creation.
	s.clock.Advance(30 * time.Minute)

	// Update the parent. Assert that only the parent changed.
	{
		const newTitle = "parent, now updated"
		parent.Title = newTitle
		parent.UpdateTime = timestamppb.New(s.clock.Now())
		gotParent := s.client.UpdateTaskT(ctx, t, &pb.UpdateTaskRequest{
			Task: &pb.Task{
				Name:  parent.GetName(),
				Title: newTitle,
			},
		})
		if diff := cmp.Diff(parent, gotParent, protocmp.Transform()); diff != "" {
			t.Errorf("update parent: unexpected result of update (-want +got)\n%s", diff)
		}
		gotChild := s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
			Name: child.GetName(),
		})
		if diff := cmp.Diff(child, gotChild, protocmp.Transform()); diff != "" {
			t.Errorf("update parent: unexpected result of getting child (-want +got)\n%s", diff)
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	// Ensure update happens after previous update.
	s.clock.Advance(30 * time.Minute)

	// Update the child. Assert that only the child changed.
	{
		const newTitle = "child, now updated"
		child.Title = newTitle
		child.UpdateTime = timestamppb.New(s.clock.Now())
		gotChild := s.client.UpdateTaskT(ctx, t, &pb.UpdateTaskRequest{
			Task: &pb.Task{
				Name:  child.GetName(),
				Title: newTitle,
			},
		})
		if diff := cmp.Diff(child, gotChild, protocmp.Transform()); diff != "" {
			t.Errorf("update child: unexpected result of update (-want +got)\n%s", diff)
		}
		gotParent := s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
			Name: parent.GetName(),
		})
		if diff := cmp.Diff(parent, gotParent, protocmp.Transform()); diff != "" {
			t.Errorf("update parent: unexpected result of getting child (-want +got)\n%s", diff)
		}
	}
}

func (s *Suite) TestUpdateTask_MultipleUpdates() {
	t := s.T()
	ctx := context.Background()

	// This test asserts that the update time is changed everytime the task is
	// updated.

	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "some task",
		},
	})

	// First update.
	{
		s.clock.Advance(15 * time.Minute)
		task.Title = "some task, now with an updated title"
		task.UpdateTime = timestamppb.New(s.clock.Now())
		gotTask := s.client.UpdateTaskT(ctx, t, &pb.UpdateTaskRequest{
			Task: task,
		})
		if diff := cmp.Diff(task, gotTask, protocmp.Transform()); diff != "" {
			t.Fatalf("Unexpected result after first update (-want +got)\n%s", diff)
		}
	}

	// Second update.
	{
		s.clock.Advance(2 * time.Hour)
		task.Description = "now with an added description"
		task.UpdateTime = timestamppb.New(s.clock.Now())
		gotTask := s.client.UpdateTaskT(ctx, t, &pb.UpdateTaskRequest{
			Task: task,
		})
		if diff := cmp.Diff(task, gotTask, protocmp.Transform()); diff != "" {
			t.Fatalf("Unexpected result after first update (-want +got)\n%s", diff)
		}
	}
}

func (s *Suite) TestUpdateTask_Error() {
	t := s.T()
	ctx := context.Background()
	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:       "Some task",
			Description: "That also has a description",
		},
	})
	parent := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "Potential parent",
		},
	})

	for _, tt := range []struct {
		name string
		req  *pb.UpdateTaskRequest
		want codes.Code
	}{
		{
			name: "NoName",
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Name:  "",
					Title: "I want to change the title",
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName",
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Name:  "invalidlolol/123",
					Title: "I want to change the title",
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Name:  "tasks/123",
					Title: "I want to change the title",
				},
			},
			want: codes.NotFound,
		},
		{
			name: "InvalidFieldInUpdateMask",
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Name:  task.GetName(),
					Title: "I want to change the title",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"title_invalid"},
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "BothFieldsAndWildcardInUpdateMask",
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Name:  task.GetName(),
					Title: "I want to change the title",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{
						"title",
						"*",
					},
				},
			},
			want: codes.InvalidArgument,
		},
		{
			// Updating a name doesn't really make sense and we could just
			// ignore it, but it's better to return an error to make a user
			// aware of it.
			name: "UpdateName",
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Name: task.GetName(),
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"name"},
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "UpdateCompleted",
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Name:      task.GetName(),
					Completed: true,
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"completed"},
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "UpdateParent",
			req: &pb.UpdateTaskRequest{
				Task: &pb.Task{
					Name:   task.GetName(),
					Parent: parent.GetName(),
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"parent"},
				},
			},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.UpdateTask(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("UpdateTask(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}

			// After the failed update the task should be intact.
			got := s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
				Name: task.GetName(),
			})
			if diff := cmp.Diff(task, got, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected task after failed update (-want +got)\n%s", diff)
			}
		})
	}
}

func (s *Suite) TestUpdateTask_AfterDeletion() {
	t := s.T()
	ctx := context.Background()
	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:       "A task that will be deleted",
			Description: "This task is not long for this world",
		},
	})
	s.client.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
		Name: task.GetName(),
	})

	req := &pb.UpdateTaskRequest{
		Task: &pb.Task{
			Name:  task.GetName(),
			Title: "You should never see this",
		},
	}
	updated, err := s.client.UpdateTask(ctx, req)
	if got, want := status.Code(err), codes.NotFound; got != want {
		t.Errorf("after deletion: UpdateTask(%v) code = %v; want %v", req, got, want)
		t.Logf("after deletion: returned task: %v", updated)
	}
}

func (s *Suite) TestDeleteTask() {
	t := s.T()
	ctx := context.Background()

	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{Title: "This will be deleted"},
	})

	// Once the task has been created it should be deleted.
	{
		req := &pb.DeleteTaskRequest{Name: task.GetName()}
		deleted, err := s.client.DeleteTask(ctx, req)
		if err != nil {
			t.Fatalf("first deletion: DeleteTask(%v) err = %v; want nil", req, err)
		}
		if err := deleted.GetDeleteTime().CheckValid(); err != nil {
			t.Errorf("first deletion: delete_time is invalid: %v", err)
		}
		if err := deleted.GetExpiryTime().CheckValid(); err != nil {
			t.Errorf("first deletion: expiry_time is invalid: %v", err)
		}
		if delete, expiry := deleted.GetDeleteTime().AsTime(), deleted.GetExpiryTime().AsTime(); expiry.Before(delete) {
			t.Errorf("first deletion: delete_time = %v; wanted before expiry_time = %v", delete, expiry)
		}
	}

	// Deleting the task again should result in a NotFound error.
	{
		req := &pb.DeleteTaskRequest{Name: task.GetName()}
		_, err := s.client.DeleteTask(ctx, req)
		if got, want := status.Code(err), codes.NotFound; got != want {
			t.Fatalf("second deletion: DeleteTask(%v) code = %v; want %v", req, got, want)
		}
	}
}

func (s *Suite) TestDeleteTask_WithChildren() {
	t := s.T()
	ctx := context.Background()

	// Set up a tasks hierarchy looking like this:
	//     r
	//    / \
	//   a   b
	//  / \
	// c1 c2
	r := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "r",
		},
	})
	a := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:  "a",
			Parent: r.GetName(),
		},
	})
	b := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:  "b",
			Parent: r.GetName(),
		},
	})
	c1 := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:  "c1",
			Parent: a.GetName(),
		},
	})
	c2 := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:  "c2",
			Parent: a.GetName(),
		},
	})

	// At this point
	// * we should _not_ be able to delete r or a without force = true, and
	// * we _should_ be able to delete b, c1, and c2 without force = true.
	// We test this by trying to do the disallowed deletes first, asserting that
	// they indeed fail, then doing the allowed deletes, asserting that they
	// succeed.
	for _, task := range []*pb.Task{r, a} {
		req := &pb.DeleteTaskRequest{
			Name:  task.GetName(),
			Force: false,
		}
		_, err := s.client.DeleteTask(ctx, req)
		if got, want := status.Code(err), codes.FailedPrecondition; got != want {
			t.Errorf("delete without force: DeleteTask(%v) err = %v, code = %v; want nil, code = %v", req, err, got, want)
		}
	}
	for _, task := range []*pb.Task{b, c1, c2} {
		req := &pb.DeleteTaskRequest{
			Name:  task.GetName(),
			Force: false,
		}
		// We don't assert anything about the actual result of the deletion,
		// other than it should succeed.
		if _, err := s.client.DeleteTask(ctx, req); err != nil {
			t.Errorf("delete without force: DeleteTask(%v) err = %v; want nil", req, err)
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	// At this point the tree looks like this with deleted tasks in brackets:
	//
	//        r
	//      /   \
	//     a    [b]
	//   /   \
	// [c1] [c2]
	//
	// We restore b and c1, and then delete r with force = true at a later
	// point. This should leave all tasks deleted, but c2 will have an earlier
	// deletion time than the other tasks.
	for _, task := range []*pb.Task{b, c1} {
		s.client.UndeleteTaskT(ctx, t, &pb.UndeleteTaskRequest{Name: task.GetName()})
	}
	s.clock.Advance(15 * time.Minute)
	req := &pb.DeleteTaskRequest{
		Name:  r.GetName(),
		Force: true,
	}
	if _, err := s.client.DeleteTask(ctx, req); err != nil {
		t.Fatalf("delete with force: DeleteTask(%v) err = %v; want nil", req, err)
	}
	r = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{Name: r.GetName()})
	a = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{Name: a.GetName()})
	b = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{Name: b.GetName()})
	c1 = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{Name: c1.GetName()})
	c2 = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{Name: c2.GetName()})
	// Now all tasks should have a delete_time. We create a mapping from
	// delete_time to task, and assert that r == a == b == c1 != c2.
	gotDeleteTimes := make(map[time.Time][]*pb.Task)
	for _, task := range []*pb.Task{r, a, b, c1, c2} {
		dt := task.GetDeleteTime().AsTime()
		gotDeleteTimes[dt] = append(gotDeleteTimes[dt], task)
	}
	wantDeleteTimes := map[time.Time][]*pb.Task{
		r.GetDeleteTime().AsTime():  {r, a, b, c1},
		c2.GetDeleteTime().AsTime(): {c2},
	}
	if diff := cmp.Diff(
		wantDeleteTimes, gotDeleteTimes,
		protocmp.Transform(), cmpopts.SortSlices(taskLessFunc),
	); diff != "" {
		t.Errorf("Unexpected delete times after cascading delete (-want +got)\n%s", diff)
	}
	// Also assert the time ordering here -- all tasks should have a later
	// delete_time than c2.
	for _, task := range []*pb.Task{r, a, b, c1} {
		taskTime := task.GetDeleteTime().AsTime()
		c2Time := c2.GetDeleteTime().AsTime()
		if !taskTime.After(c2Time) {
			t.Errorf("%q delete_time = %v; want after %v", task.GetName(), taskTime, c2Time)
		}
	}
}

func (s *Suite) TestDeleteTask_Error() {
	t := s.T()
	ctx := context.Background()
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
			_, err := s.client.DeleteTask(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("DeleteTask(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}
		})
	}
}

func (s *Suite) TestUndeleteTask() {
	t := s.T()
	ctx := context.Background()

	// Create task, soft delete it, then undelete it. The result should be the
	// same task as just after it was created.
	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:       "This will be deleted",
			Description: "And later undeleted, woohoo!",
		},
	})
	s.client.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
		Name: task.GetName(),
	})
	undeleted := s.client.UndeleteTaskT(ctx, t, &pb.UndeleteTaskRequest{
		Name: task.GetName(),
	})
	if diff := cmp.Diff(task, undeleted, protocmp.Transform()); diff != "" {
		t.Errorf("unexpected result after undeletion (-before +after)\n%s", diff)
	}
}

func (s *Suite) TestUndeleteTask_WithFamily() {
	t := s.T()
	ctx := context.Background()

	// Create root hierarchy that looks like
	//     root -> middle -> leaf
	// where "->" means "parent of".
	root := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "root",
		},
	})
	middle := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Parent: root.GetName(),
			Title:  "middle",
		},
	})
	leaf := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Parent: middle.GetName(),
			Title:  "leaf",
		},
	})

	// Each test case will start out with
	//     [root] -> [middle] -> [leaf]
	// where "[x]" means "task x is soft deleted".
	for _, tt := range []struct {
		name string
		req  *pb.UndeleteTaskRequest
		want map[string]bool // name -> whether it is left deleted
	}{
		{
			name: "Middle_UndeleteAncestors",
			req: &pb.UndeleteTaskRequest{
				Name:                middle.GetName(),
				UndeleteAncestors:   true,
				UndeleteDescendants: false,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): false,
				leaf.GetName():   true,
			},
		},
		{
			name: "Middle_UndeleteAncestors_UndeleteDescendants",
			req: &pb.UndeleteTaskRequest{
				Name:                middle.GetName(),
				UndeleteAncestors:   true,
				UndeleteDescendants: true,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): false,
				leaf.GetName():   false,
			},
		},
		{
			name: "Root_UndeleteAncestors",
			req: &pb.UndeleteTaskRequest{
				Name:                root.GetName(),
				UndeleteAncestors:   true,
				UndeleteDescendants: false,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): true,
				leaf.GetName():   true,
			},
		},
		{
			name: "Root_UndeleteAncestors_UndeleteDescendants",
			req: &pb.UndeleteTaskRequest{
				Name:                root.GetName(),
				UndeleteAncestors:   true,
				UndeleteDescendants: true,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): false,
				leaf.GetName():   false,
			},
		},
		{
			name: "Root_UndeleteDescendants",
			req: &pb.UndeleteTaskRequest{
				Name:                root.GetName(),
				UndeleteAncestors:   false,
				UndeleteDescendants: true,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): false,
				leaf.GetName():   false,
			},
		},
		{
			name: "Leaf_UndeleteAncestors",
			req: &pb.UndeleteTaskRequest{
				Name:                leaf.GetName(),
				UndeleteAncestors:   true,
				UndeleteDescendants: false,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): false,
				leaf.GetName():   false,
			},
		},
		{
			name: "Leaf_UndeleteAncestors_UndeleteDescendants",
			req: &pb.UndeleteTaskRequest{
				Name:                leaf.GetName(),
				UndeleteAncestors:   true,
				UndeleteDescendants: true,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): false,
				leaf.GetName():   false,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Make sure we start out with all tasks deleted.
			s.client.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
				Name:  root.GetName(),
				Force: true,
			})

			_, err := s.client.UndeleteTask(ctx, tt.req)
			if err != nil {
				t.Fatalf("UndeleteTask(%v) err = %v; want nil", tt.req, err)
			}
			got := make(map[string]bool)
			for _, task := range []string{
				root.GetName(),
				middle.GetName(),
				leaf.GetName(),
			} {
				got[task] = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{Name: task}).GetDeleteTime().IsValid()
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Unexpected result of undeleting (-want +got)\n%s", diff)
			}
		})
	}
}

func (s *Suite) TestUndeleteTask_WithFamily_Error() {
	t := s.T()
	ctx := context.Background()

	// Create root hierarchy that looks like
	//     root -> middle -> leaf
	// where "->" means "parent of".
	root := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "root",
		},
	})
	middle := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Parent: root.GetName(),
			Title:  "middle",
		},
	})
	leaf := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Parent: middle.GetName(),
			Title:  "leaf",
		},
	})

	// Now, delete all tasks so that we end up with
	//     [root] -> [middle] -> [leaf]
	// where "[x]" means "task x is soft deleted".
	s.client.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
		Name:  root.GetName(),
		Force: true,
	})

	for _, tt := range []struct {
		name string
		req  *pb.UndeleteTaskRequest
		want codes.Code
	}{
		{
			name: "Leaf_NoUndeleteAncestors",
			req: &pb.UndeleteTaskRequest{
				Name:                leaf.GetName(),
				UndeleteAncestors:   false,
				UndeleteDescendants: false,
			},
			want: codes.FailedPrecondition,
		},
		{
			name: "Middle_NoUndeleteAncestors",
			req: &pb.UndeleteTaskRequest{
				Name:                middle.GetName(),
				UndeleteAncestors:   false,
				UndeleteDescendants: false,
			},
			want: codes.FailedPrecondition,
		},
		{
			name: "Leaf_NoUndeleteAncestors_UndeleteDescendants",
			req: &pb.UndeleteTaskRequest{
				Name:                leaf.GetName(),
				UndeleteAncestors:   false,
				UndeleteDescendants: true,
			},
			want: codes.FailedPrecondition,
		},
		{
			name: "Middle_NoUndeleteAncestors_UndeleteDescendants",
			req: &pb.UndeleteTaskRequest{
				Name:                middle.GetName(),
				UndeleteAncestors:   false,
				UndeleteDescendants: true,
			},
			want: codes.FailedPrecondition,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.UndeleteTask(ctx, tt.req)
			if got, want := status.Code(err), tt.want; got != want {
				t.Errorf("UndeleteTask(%v) err = %v; got code = %v; want %v", tt.req, err, got, want)
			}
		})
	}
}

func (s *Suite) TestUndeleteTask_Error() {
	t := s.T()
	ctx := context.Background()
	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "Buy milk",
		},
	})

	for _, tt := range []struct {
		name string
		req  *pb.UndeleteTaskRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req: &pb.UndeleteTaskRequest{
				Name: "",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &pb.UndeleteTaskRequest{
				Name: "tasks/notfound",
			},
			want: codes.NotFound,
		},
		{
			name: "InvalidName",
			req: &pb.UndeleteTaskRequest{
				Name: "invalidlololol/1",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotDeleted",
			req: &pb.UndeleteTaskRequest{
				Name: task.GetName(),
			},
			want: codes.AlreadyExists,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.UndeleteTask(ctx, tt.req)
			if got, want := status.Code(err), tt.want; got != want {
				t.Errorf("UndeleteTask(%v) code = %v; want %v", tt.req, got, want)
				t.Logf("err = %v", err)
			}
		})
	}
}

// Test ideas:
// [x] Completing a deleted task should return NotFound
// [x] Completing a task with uncompleted children should fail with FailedPrecondition
// [x] Completing a task with completed children and `force: false` should succeed.
// [x] Completing a task with children should complete all descendants with `force: true`
// [x] Uncompleting a task with completed ancestors should fail with FailedPrecondition
// [x] Uncompleting a task with uncompleted ancestors should succeed
// [x] Uncompleting a task with completed ancestors and `uncomplete_ancestors: true` should succeed
// [ ] Uncompleting an uncompleted task should be a no-op
// [ ] Uncompleting a completed task with completed children should only uncomplete the task itself
// [ ] Uncompleting a completed task with completed children should uncomplete all children with `uncomplete_children: true`
// [ ] Uncompleting a completed task with both completed ancestors and completed
//     children should uncomplete everything with `uncomplete_ancestors: true
//     uncomplete_descendants: true`
// [x] Uncompleting a deleted task should return NotFound
// [x] The usual errors with invalid names, empty names, etc

func (s *Suite) TestCompleteTask_UncompleteTask_ClearsCompleteTime() {
	t := s.T()
	ctx := context.Background()

	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "Get swole",
		},
	})

	// Complete the task after 30 minutes.
	{
		s.clock.Advance(30 * time.Minute)
		task.Completed = true
		now := s.clock.Now()
		task.CompleteTime = timestamppb.New(now)
		task.UpdateTime = timestamppb.New(now)
		req := &pb.CompleteTaskRequest{
			Name: task.GetName(),
		}
		got := s.client.CompleteTaskT(ctx, t, req)
		if diff := cmp.Diff(task, got, protocmp.Transform()); diff != "" {
			t.Fatalf("CompleteTask(%v) produced unexpected result (-want +got)\n%s", req, diff)
		}
	}

	// Uncomplete the task after another 30 minutes.
	{
		s.clock.Advance(30 * time.Minute)
		task.Completed = false
		task.CompleteTime = nil
		task.UpdateTime = timestamppb.New(s.clock.Now())
		req := &pb.UncompleteTaskRequest{
			Name: task.GetName(),
		}
		got := s.client.UncompleteTaskT(ctx, t, req)
		if diff := cmp.Diff(task, got, protocmp.Transform()); diff != "" {
			t.Fatalf("UncompleteTask(%v) produced unexpected result (-want +got)\n%s", req, diff)
		}
	}
}

func (s *Suite) TestCompleteTask_WithChildren() {
	t := s.T()
	ctx := context.Background()

	parent := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "parent",
		},
	})
	child := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:  "child",
			Parent: parent.GetName(),
		},
	})

	// Completing the parent with `force: false` should fail.
	{
		req := &pb.CompleteTaskRequest{
			Name:  parent.GetName(),
			Force: false,
		}
		_, err := s.client.CompleteTask(ctx, req)
		if got, want := status.Code(err), codes.FailedPrecondition; got != want {
			t.Fatalf("CompleteTask(%v) err = %v; want code %v", req, err, want)
		}
	}

	// Completing the parent with `force: true` should succeed and leave both
	// parent and child completed.
	{
		// We set up `parent` and `child` to be what we expect, and we will
		// compare against it later.
		now := s.clock.Now()
		parent.Completed = true
		parent.CompleteTime = timestamppb.New(now)
		parent.UpdateTime = timestamppb.New(now)
		child.Completed = true
		child.CompleteTime = timestamppb.New(now)
		child.UpdateTime = timestamppb.New(now)

		req := &pb.CompleteTaskRequest{
			Name:  parent.GetName(),
			Force: true,
		}
		gotParent := s.client.CompleteTaskT(ctx, t, req)
		if diff := cmp.Diff(parent, gotParent, protocmp.Transform()); diff != "" {
			t.Errorf("parent: unexpected result of CompleteTask(%v) (-want +got)\n%s", req, diff)
		}
		gotChild := s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
			Name: child.GetName(),
		})
		if diff := cmp.Diff(child, gotChild, protocmp.Transform()); diff != "" {
			t.Errorf("child: unexpected result of CompleteTask(%v) (-want +got)\n%s", req, diff)
		}
	}
}

func (s *Suite) TestCompleteTask_WithChildren_AllChildrenCompleted() {
	t := s.T()
	ctx := context.Background()

	parent := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "parent",
		},
	})
	child := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:  "child",
			Parent: parent.GetName(),
		},
	})

	// Complete the child some time after it has been created.
	s.clock.Advance(15 * time.Minute)
	child = s.client.CompleteTaskT(ctx, t, &pb.CompleteTaskRequest{
		Name: child.GetName(),
	})

	// Let some time pass between completing the child and completing the
	// parent.
	s.clock.Advance(4 * time.Hour)

	// Completing the parent with `force: false` should succeed and leave both
	// parent and child completed.
	now := s.clock.Now()
	parent.Completed = true
	parent.CompleteTime = timestamppb.New(now)
	parent.UpdateTime = timestamppb.New(now)
	req := &pb.CompleteTaskRequest{
		Name:  parent.GetName(),
		Force: false,
	}
	gotParent := s.client.CompleteTaskT(ctx, t, req)
	if diff := cmp.Diff(parent, gotParent, protocmp.Transform()); diff != "" {
		t.Errorf("parent: unexpected result of CompleteTask(%v) (-want +got)\n%s", req, diff)
	}
	// The completion timestamp of child should not have been changed.
	gotChild := s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
		Name: child.GetName(),
	})
	if diff := cmp.Diff(child, gotChild, protocmp.Transform()); diff != "" {
		t.Errorf("child: unexpected result of CompleteTask(%v) (-want +got)\n%s", req, diff)
	}
}

func (s *Suite) TestCompleteTask_AlreadyCompleted() {
	t := s.T()
	ctx := context.Background()

	// When trying to complete a task that is already completed, it should be a
	// no-op and the task should be returned unmodified. We detect this by
	// simulating time passing, which should be the only change in the world
	// between the various operations on the task.

	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "Build stuff",
		},
	})

	s.clock.Advance(30 * time.Minute)

	first := s.client.CompleteTaskT(ctx, t, &pb.CompleteTaskRequest{
		Name: task.GetName(),
	})

	s.clock.Advance(30 * time.Minute)

	second := s.client.CompleteTaskT(ctx, t, &pb.CompleteTaskRequest{
		Name: task.GetName(),
	})
	if diff := cmp.Diff(first, second, protocmp.Transform()); diff != "" {
		t.Fatalf("Unexpected result of completing a second time (-first +second)\n%s", diff)
	}
}

func (s *Suite) TestCompleteTask_Deleted() {
	t := s.T()
	ctx := context.Background()

	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "should be deleted",
		},
	})
	task = s.client.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
		Name: task.GetName(),
	})

	req := &pb.CompleteTaskRequest{
		Name: task.GetName(),
	}
	_, err := s.client.CompleteTask(ctx, req)
	if got, want := status.Code(err), codes.NotFound; got != want {
		t.Fatalf("CompleteTask(%v) err = %v; want code %v", req, err, want)
	}
}

func (s *Suite) TestCompleteTask_Error() {
	t := s.T()
	ctx := context.Background()

	for _, tt := range []struct {
		name string
		req  *pb.CompleteTaskRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req: &pb.CompleteTaskRequest{
				Name: "",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName",
			req: &pb.CompleteTaskRequest{
				Name: "invalid/123",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "MissingResourceID",
			req: &pb.CompleteTaskRequest{
				Name: "tasks/",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &pb.CompleteTaskRequest{
				Name: "tasks/999",
			},
			want: codes.NotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.CompleteTask(ctx, tt.req)
			if got, want := status.Code(err), tt.want; got != want {
				t.Fatalf("CompleteTask(%v) err = %v; want code %v", tt.req, err, want)
			}
		})
	}
}

func (s *Suite) TestUncompleteTask_NotCompleted() {
	t := s.T()
	ctx := context.Background()

	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "some task",
		},
	})

	// Uncompleting a task that is not completed should be a no-op.
	got := s.client.UncompleteTaskT(ctx, t, &pb.UncompleteTaskRequest{
		Name: task.GetName(),
	})
	if diff := cmp.Diff(task, got, protocmp.Transform()); diff != "" {
		t.Fatalf("Uncompleting an uncompleted task wasn't a no-op (-want +got)\n%s", diff)
	}
}

func (s *Suite) TestUncompleteTask_WithParent() {
	t := s.T()
	ctx := context.Background()

	parent := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "parent",
		},
	})
	child := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:  "child",
			Parent: parent.GetName(),
		},
	})

	// Let some time pass after creation and then complete both with `force:
	// true`.
	s.clock.Advance(12 * time.Hour)
	parent = s.client.CompleteTaskT(ctx, t, &pb.CompleteTaskRequest{
		Name:  parent.GetName(),
		Force: true,
	})
	child = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
		Name: child.GetName(),
	})

	// Let some more time pass after completion.
	s.clock.Advance(3 * time.Hour)

	// Uncompleting `child` with `uncomplete_ancestors: false` should fail.
	{
		req := &pb.UncompleteTaskRequest{
			Name:                child.GetName(),
			UncompleteAncestors: false,
		}
		_, err := s.client.UncompleteTask(ctx, req)
		if got, want := status.Code(err), codes.FailedPrecondition; got != want {
			t.Fatalf("UncompleteTask(%v) err = %v; want code %v", req, err, want)
		}
	}

	// Uncompleting `child` with `uncomplete_ancestors: true` should succeed and
	// leave both `child` and `parent` completed.
	{
		// Set up `child` and `parent` so that we can compare with them later.
		now := s.clock.Now()
		child.Completed = false
		child.CompleteTime = nil
		child.UpdateTime = timestamppb.New(now)
		parent.Completed = false
		parent.CompleteTime = nil
		parent.UpdateTime = timestamppb.New(now)

		req := &pb.UncompleteTaskRequest{
			Name:                child.GetName(),
			UncompleteAncestors: true,
		}
		gotChild := s.client.UncompleteTaskT(ctx, t, req)
		if diff := cmp.Diff(child, gotChild, protocmp.Transform()); diff != "" {
			t.Errorf("child: unexpected result of UncompleteTask(%v) (-want +got)\n%s", req, diff)
		}
		gotParent := s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
			Name: parent.GetName(),
		})
		if diff := cmp.Diff(parent, gotParent, protocmp.Transform()); diff != "" {
			t.Errorf("parent: unexpected result of UncompleteTask(%v) (-want +got)\n%s", req, diff)
		}
	}
}

func (s *Suite) TestUncompleteTask_WithParent_ParentUncompleted() {
	t := s.T()
	ctx := context.Background()

	parent := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "parent",
		},
	})
	child := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:  "child",
			Parent: parent.GetName(),
		},
	})

	// Let some time pass after creation and then complete both with `force:
	// true`.
	s.clock.Advance(12 * time.Hour)
	parent = s.client.CompleteTaskT(ctx, t, &pb.CompleteTaskRequest{
		Name:  parent.GetName(),
		Force: true,
	})
	child = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
		Name: child.GetName(),
	})

	// Let some more time pass after completion and then uncomplete `parent`.
	// This should leave `child` still completed.
	s.clock.Advance(3 * time.Hour)
	{
		req := &pb.UncompleteTaskRequest{
			Name: parent.GetName(),
		}
		parent = s.client.UncompleteTaskT(ctx, t, req)
		gotChild := s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
			Name: child.GetName(),
		})
		if diff := cmp.Diff(child, gotChild, protocmp.Transform()); diff != "" {
			t.Fatalf("child: unexpected result of UncompleteTask(%v) (-want +got)\n%s", req, diff)
		}
	}

	// Let yet more time pass and then uncomplete `child` with
	// `uncomplete_ancestors: false`. This should succeed, and `parent` should
	// be left untouched.
	s.clock.Advance(14 * time.Hour)
	{
		// We will compare the result with `child`.
		child.Completed = false
		child.CompleteTime = nil
		child.UpdateTime = timestamppb.New(s.clock.Now())

		req := &pb.UncompleteTaskRequest{
			Name:                child.GetName(),
			UncompleteAncestors: false,
		}
		gotChild := s.client.UncompleteTaskT(ctx, t, req)
		if diff := cmp.Diff(child, gotChild, protocmp.Transform()); diff != "" {
			t.Errorf("child: unexpected result of UncompleteTask(%v) (-want +got)\n%s", req, diff)
		}
		// `parent` should be left untouched.
		gotParent := s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
			Name: parent.GetName(),
		})
		if diff := cmp.Diff(parent, gotParent, protocmp.Transform()); diff != "" {
			t.Errorf("parent: unexpected result of UncompleteTask(%v) (-want +got)\n%s", req, diff)
		}
	}
}

func (s *Suite) TestUncompleteTask_InHierarchy() {
	t := s.T()
	ctx := context.Background()

	// Set up a hierarchy looking like
	//     root -> middle -> leaf
	// where "->" means "parent of".
	root := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "root",
		},
	})
	middle := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:  "middle",
			Parent: root.GetName(),
		},
	})
	leaf := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title:  "leaf",
			Parent: middle.GetName(),
		},
	})

	// Complete all tasks, leaving us with
	//     [root] -> [middle] -> [leaf]
	// where "[x]" means "task x is completed".
	root = s.client.CompleteTaskT(ctx, t, &pb.CompleteTaskRequest{
		Name:  root.GetName(),
		Force: true,
	})
	middle = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
		Name: middle.GetName(),
	})
	leaf = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
		Name: leaf.GetName(),
	})
	for _, task := range []*pb.Task{
		root,
		middle,
		leaf,
	} {
		if got, want := task.GetCompleted(), true; got != want {
			t.Errorf("Task %q has completed = %v; want %v", task.GetName(), got, want)
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	// First, we verify that a bunch of requests are invalid.
	for _, tt := range []struct {
		name string
		req  *pb.UncompleteTaskRequest
		want codes.Code
	}{
		{
			name: "Leaf_NoUncompleteAncestors",
			req: &pb.UncompleteTaskRequest{
				Name:                leaf.GetName(),
				UncompleteAncestors: false,
			},
			want: codes.FailedPrecondition,
		},
		{
			name: "Leaf_NoUncompleteAncestors_WithUncompleteDescendants",
			req: &pb.UncompleteTaskRequest{
				Name:                  leaf.GetName(),
				UncompleteAncestors:   false,
				UncompleteDescendants: true,
			},
			want: codes.FailedPrecondition,
		},
		{
			name: "Middle_NoUncompleteAncestors",
			req: &pb.UncompleteTaskRequest{
				Name:                middle.GetName(),
				UncompleteAncestors: false,
			},
			want: codes.FailedPrecondition,
		},
		{
			name: "Middle_NoUncompleteAncestors_WithUncompleteDescendants",
			req: &pb.UncompleteTaskRequest{
				Name:                  middle.GetName(),
				UncompleteAncestors:   false,
				UncompleteDescendants: true,
			},
			want: codes.FailedPrecondition,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.UncompleteTask(ctx, tt.req)
			if got, want := status.Code(err), tt.want; got != want {
				t.Errorf("UncompleteTask(%v) err = %v; want code %v", tt.req, err, want)
			}
		})
	}
	if t.Failed() {
		t.FailNow()
	}

	for _, task := range []*pb.Task{
		root,
		middle,
		leaf,
	} {
		if got, want := task.GetCompleted(), true; got != want {
			t.Errorf("Task %q has completed = %v; want %v", task.GetName(), got, want)
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	// Next, we set up a bunch of test cases all assuming a starting state of
	//     [root] -> [middle] -> [leaf].
	// The test cases will issue an UncompleteTask RPC and then verify the
	// `completed` state of the above tasks.
	for _, tt := range []struct {
		name string
		req  *pb.UncompleteTaskRequest
		want map[string]bool // task name -> whether it is _completed_
	}{
		{
			name: "Root",
			req: &pb.UncompleteTaskRequest{
				Name: root.GetName(),
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): true,
				leaf.GetName():   true,
			},
		},
		{
			name: "Root_UncompleteDescendants",
			req: &pb.UncompleteTaskRequest{
				Name:                  root.GetName(),
				UncompleteDescendants: true,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): false,
				leaf.GetName():   false,
			},
		},
		{
			name: "Leaf_UncompleteAncestors",
			req: &pb.UncompleteTaskRequest{
				Name:                leaf.GetName(),
				UncompleteAncestors: true,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): false,
				leaf.GetName():   false,
			},
		},
		{
			name: "Middle_UncompleteAncestors",
			req: &pb.UncompleteTaskRequest{
				Name:                middle.GetName(),
				UncompleteAncestors: true,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): false,
				leaf.GetName():   true,
			},
		},
		{
			name: "Middle_UncompleteDescendants_UncompleteAncestors",
			req: &pb.UncompleteTaskRequest{
				Name:                  middle.GetName(),
				UncompleteAncestors:   true,
				UncompleteDescendants: true,
			},
			want: map[string]bool{
				root.GetName():   false,
				middle.GetName(): false,
				leaf.GetName():   false,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange for all tasks to be restored to completed state when the
			// test ends.
			t.Cleanup(func() {
				s.client.CompleteTaskT(ctx, t, &pb.CompleteTaskRequest{
					Name:  root.GetName(),
					Force: true,
				})
			})

			s.client.UncompleteTaskT(ctx, t, tt.req)
			got := make(map[string]bool)
			for name := range tt.want {
				got[name] = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{Name: name}).GetCompleted()
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Unexpected result of UncompleteTask(%v), true == completed (-want +got)\n%s", tt.req, diff)
			}
		})
	}
}

func (s *Suite) TestUncompleteTask_Deleted() {
	t := s.T()
	ctx := context.Background()

	task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
		Task: &pb.Task{
			Title: "should be deleted",
		},
	})
	task = s.client.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
		Name: task.GetName(),
	})

	req := &pb.UncompleteTaskRequest{
		Name: task.GetName(),
	}
	_, err := s.client.UncompleteTask(ctx, req)
	if got, want := status.Code(err), codes.NotFound; got != want {
		t.Fatalf("UncompleteTask(%v) err = %v; want code %v", req, err, want)
	}
}

func (s *Suite) TestUncompleteTask_Error() {
	t := s.T()
	ctx := context.Background()

	for _, tt := range []struct {
		name string
		req  *pb.UncompleteTaskRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req: &pb.UncompleteTaskRequest{
				Name: "",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName",
			req: &pb.UncompleteTaskRequest{
				Name: "invalid/123",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "MissingResourceID",
			req: &pb.UncompleteTaskRequest{
				Name: "tasks/",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &pb.UncompleteTaskRequest{
				Name: "tasks/999",
			},
			want: codes.NotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.UncompleteTask(ctx, tt.req)
			if got, want := status.Code(err), tt.want; got != want {
				t.Fatalf("UncompleteTask(%v) err = %v; want code %v", tt.req, err, want)
			}
		})
	}
}
