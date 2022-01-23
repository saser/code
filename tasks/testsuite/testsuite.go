package testsuite

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/suite"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func taskLessFunc(t1, t2 *pb.Task) bool {
	return t1.GetName() < t2.GetName()
}

// Suite contains a suite of tests for an implementation of Tasks service.
type Suite struct {
	suite.Suite

	client      *testClient
	truncater   Truncater
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
func New(client pb.TasksClient, truncater Truncater, maxPageSize int) *Suite {
	return &Suite{
		client:      &testClient{TasksClient: client},
		truncater:   truncater,
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

	// After deleting the task, getting the task by name should produce a "not
	// found" error.
	{
		s.client.DeleteTaskT(ctx, t, &pb.DeleteTaskRequest{
			Name: task.GetName(),
		})

		req := &pb.GetTaskRequest{
			Name: task.GetName(),
		}
		_, err := s.client.GetTask(ctx, req)
		if got, want := status.Code(err), codes.NotFound; got != want {
			t.Errorf("GetTask(%v) code = %v; want %v", req, got, want)
		}
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
	for _, tt := range []struct {
		name string
		task *pb.Task
		req  *pb.UpdateTaskRequest // will be updated in-place with the created task name
		want *pb.Task              // will be updated in-place with the creation time
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
				Title: "Before the update",
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
				Title: "Before the update",
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
				Title: "After the update",
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
				Title: "After the update",
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
				Title: "After the update",
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
			},
		},
		{
			// An empty/default value for `description` with a wildcard update
			// mask shoul result in description being cleared.
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
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			task := s.client.CreateTaskT(ctx, t, &pb.CreateTaskRequest{
				Task: tt.task,
			})
			tt.req.Task.Name = task.GetName()
			tt.want.CreateTime = task.GetCreateTime()
			got := s.client.UpdateTaskT(ctx, t, tt.req)
			if diff := cmp.Diff(tt.want, got, protocmp.Transform(), protocmp.IgnoreFields(got, "name")); diff != "" {
				t.Errorf("unexpected result of update (-want +got)\n%s", diff)
			}
			// Getting the task again should produce the same result as after
			// the update.
			got = s.client.GetTaskT(ctx, t, &pb.GetTaskRequest{
				Name: task.GetName(),
			})
			if diff := cmp.Diff(tt.want, got, protocmp.Transform(), protocmp.IgnoreFields(got, "name")); diff != "" {
				t.Errorf("unexpected result of GetTask after update (-want +got)\n%s", diff)
			}
		})
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
		_, err := s.client.DeleteTask(ctx, req)
		if err != nil {
			t.Fatalf("first deletion: DeleteTask(%v) err = %v; want nil", req, err)
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
