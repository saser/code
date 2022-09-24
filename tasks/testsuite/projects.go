package testsuite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Suite) TestGetProject() {
	t := s.T()
	ctx := context.Background()

	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title:       "Get this",
			Description: "Be sure to get this!!!",
		},
	})

	// Getting the project by name should produce the same result.
	req := &pb.GetProjectRequest{
		Name: project.GetName(),
	}
	got, err := s.client.GetProject(ctx, req)
	if err != nil {
		t.Fatalf("GetProject(%v) err = %v; want nil", req, err)
	}
	if diff := cmp.Diff(project, got, protocmp.Transform()); diff != "" {
		t.Errorf("GetProject(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func (s *Suite) TestGetProject_AfterDeletion() {
	t := s.T()
	ctx := context.Background()

	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title:       "Get this",
			Description: "Be sure to get this!!!",
		},
	})

	// Getting the project by name should produce the same result.
	{
		req := &pb.GetProjectRequest{
			Name: project.GetName(),
		}
		got := s.client.GetProjectT(ctx, t, req)
		if diff := cmp.Diff(project, got, protocmp.Transform()); diff != "" {
			t.Errorf("GetProject(%v): unexpected result (-want +got)\n%s", req, diff)
		}
	}

	// After soft deleting the project, getting the project by name should succeed and
	// produce the same project.
	{
		want := s.client.DeleteProjectT(ctx, t, &pb.DeleteProjectRequest{
			Name: project.GetName(),
		})
		project = s.client.GetProjectT(ctx, t, &pb.GetProjectRequest{
			Name: project.GetName(),
		})
		if diff := cmp.Diff(want, project, protocmp.Transform()); diff != "" {
			t.Errorf("GetProject: unexpected result of getting soft deleted project (-want +got)\n%s", diff)
		}
	}

	// After the project has expired we shouldn't be able to get it anymore.
	s.clock.Advance(project.GetExpireTime().AsTime().Sub(s.clock.Now()))
	s.clock.Advance(1 * time.Minute)
	req := &pb.GetProjectRequest{
		Name: project.GetName(),
	}
	_, err := s.client.GetProject(ctx, req)
	if got, want := status.Code(err), codes.NotFound; got != want {
		t.Errorf("after expiration: GetProject(%v) code = %v; want %v", req, got, want)
		t.Logf("err = %v", err)
	}
}

func (s *Suite) TestGetProject_Error() {
	t := s.T()
	ctx := context.Background()
	for _, tt := range []struct {
		name string
		req  *pb.GetProjectRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req:  &pb.GetProjectRequest{Name: ""},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName",
			req:  &pb.GetProjectRequest{Name: "invalid/123"},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName_NoResourceID",
			req: &pb.GetProjectRequest{
				Name: "projects/",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req:  &pb.GetProjectRequest{Name: "projects/999"},
			want: codes.NotFound,
		},
		{
			name: "NotFound_DifferentResourceIDFormat",
			req: &pb.GetProjectRequest{
				// This is a valid name -- there is no guarantee what format the
				// resource ID (the segment after the slash) will have. But it
				// probably won't be arbitrary strings.
				Name: "projects/invalidlol",
			},
			want: codes.NotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.GetProject(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("GetProject(%v) code = %v; want %v", tt.req, got, tt.want)
			}
		})
	}
}

func (s *Suite) TestListProjects() {
	t := s.T()
	ctx := context.Background()

	want := s.client.CreateProjectsT(ctx, t, []*pb.Project{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	req := &pb.ListProjectsRequest{
		PageSize: int32(len(want)),
	}
	res, err := s.client.ListProjects(ctx, req)
	if err != nil {
		t.Fatalf("ListProjects(%v) err = %v; want nil", req, err)
	}
	if diff := cmp.Diff(want, res.GetProjects(), protocmp.Transform(), cmpopts.SortSlices(projectLessFunc)); diff != "" {
		t.Errorf("ListProjects(%v): unexpected result (-want +got)\n%s", req, diff)
	}
	if got, want := res.GetNextPageToken(), ""; got != want {
		t.Errorf("ListProjects(%v) next_page_token = %q; want %q", req, got, want)
	}
}

func (s *Suite) TestListProjects_MaxPageSize() {
	t := s.T()
	ctx := context.Background()

	projects := make([]*pb.Project, s.maxPageSize*2-s.maxPageSize/2)
	for i := range projects {
		projects[i] = s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
			Project: &pb.Project{
				Title: fmt.Sprint(i),
			},
		})
	}

	req := &pb.ListProjectsRequest{
		PageSize: int32(len(projects)), // more than maxPageSize
	}

	res := s.client.ListProjectsT(ctx, t, req)
	wantFirstPage := projects[:s.maxPageSize]
	if diff := cmp.Diff(wantFirstPage, res.GetProjects(), protocmp.Transform(), cmpopts.SortSlices(projectLessFunc)); diff != "" {
		t.Errorf("[first page] ListProjects(%v): unexpected result (-want +got)\n%s", req, diff)
	}

	req.PageToken = res.GetNextPageToken()
	res = s.client.ListProjectsT(ctx, t, req)
	wantSecondPage := projects[s.maxPageSize:]
	if diff := cmp.Diff(wantSecondPage, res.GetProjects(), protocmp.Transform(), cmpopts.SortSlices(projectLessFunc)); diff != "" {
		t.Errorf("[second page] ListProjects(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func (s *Suite) TestListProjects_DifferentPageSizes() {
	t := s.T()
	ctx := context.Background()

	// 7 projects. Number chosen arbitrarily.
	projects := s.client.CreateProjectsT(ctx, t, []*pb.Project{
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
			// of projects, and that we won't try to get more pages after the
			// last one.
			{
				sum := int32(0)
				for i, s := range sizes {
					if s <= 0 {
						t.Errorf("sizes[%d] = %v; want a positive number", i, s)
					}
					sum += s
				}
				n := int32(len(projects))
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
			// Now we can start listing projects.
			req := &pb.ListProjectsRequest{}
			var got []*pb.Project
			for i, size := range sizes {
				req.PageSize = size
				res := s.client.ListProjectsT(ctx, t, req)
				got = append(got, res.GetProjects()...)
				token := res.GetNextPageToken()
				if i < len(sizes)-1 && token == "" {
					// This error does not apply for the last page.
					t.Fatalf("[after page %d]: ListProjects(%v) next_page_token = %q; want non-empty", i, req, token)
				}
				req.PageToken = token
			}
			// After all the page sizes the page token should be empty.
			if got, want := req.GetPageToken(), ""; got != want {
				t.Fatalf("[after all pages] page_token = %q; want %q", got, want)
			}
			if diff := cmp.Diff(projects, got, protocmp.Transform(), cmpopts.SortSlices(projectLessFunc)); diff != "" {
				t.Errorf("unexpected result (-want +got)\n%s", diff)
			}
		})
	}
}

func (s *Suite) TestListProjects_WithDeletions() {
	t := s.T()
	ctx := context.Background()

	seed := []*pb.Project{
		{Title: "First project"},
		{Title: "Second project"},
		{Title: "Third project"},
	}

	for _, tt := range []struct {
		name                  string
		firstPageSize         int32
		wantFirstPageIndices  []int // indices into created projects
		deleteIndex           int
		wantSecondPageIndices []int // indices into created projects
	}{
		{
			name:                  "DeleteInFirstPage_TwoProjectsInFirstPage",
			firstPageSize:         2,
			wantFirstPageIndices:  []int{0, 1},
			deleteIndex:           1,
			wantSecondPageIndices: []int{2},
		},
		{
			name:                  "DeleteInFirstPage_OneProjectInFirstPage",
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
			name:                  "DeleteInSecondPage_TwoProjectsInFirstPage",
			firstPageSize:         2,
			wantFirstPageIndices:  []int{0, 1},
			deleteIndex:           2,
			wantSecondPageIndices: []int{},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			defer s.truncate(ctx)
			projects := s.client.CreateProjectsT(ctx, t, seed)

			// Get the first page and assert that it matches what we want.
			req := &pb.ListProjectsRequest{
				PageSize: tt.firstPageSize,
			}
			res := s.client.ListProjectsT(ctx, t, req)
			wantFirstPage := make([]*pb.Project, 0, len(tt.wantFirstPageIndices))
			for _, idx := range tt.wantFirstPageIndices {
				wantFirstPage = append(wantFirstPage, projects[idx])
			}
			if diff := cmp.Diff(wantFirstPage, res.GetProjects(), protocmp.Transform(), protocmp.SortRepeated(projectLessFunc)); diff != "" {
				t.Fatalf("first page: unexpected projects (-want +got)\n%s", diff)
			}
			token := res.GetNextPageToken()
			if token == "" {
				t.Fatal("no next page token from first page")
			}
			req.PageToken = token

			// Delete one of the projects.
			s.client.DeleteProjectT(ctx, t, &pb.DeleteProjectRequest{
				Name: projects[tt.deleteIndex].GetName(),
			})

			// Get the second page and assert that it matches what we want. Also
			// assert that there are no more projects.
			req.PageSize = int32(len(projects)) // Make sure we get the remaining projects.
			res = s.client.ListProjectsT(ctx, t, req)
			wantSecondPage := make([]*pb.Project, 0, len(tt.wantSecondPageIndices))
			for _, idx := range tt.wantSecondPageIndices {
				wantSecondPage = append(wantSecondPage, projects[idx])
			}
			if diff := cmp.Diff(wantSecondPage, res.GetProjects(), cmpopts.EquateEmpty(), protocmp.Transform(), protocmp.SortRepeated(projectLessFunc)); diff != "" {
				t.Fatalf("second page: unexpected projects (-want +got)\n%s", diff)
			}
			if got, want := res.GetNextPageToken(), ""; got != want {
				t.Errorf("second page: next_page_token = %q; want %q", got, want)
			}
		})
	}
}

func (s *Suite) TestListProjects_WithDeletions_ShowDeleted() {
	t := s.T()
	ctx := context.Background()

	want := s.client.CreateProjectsT(ctx, t, []*pb.Project{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	// Soft delete one of the projects.
	want[1] = s.client.DeleteProjectT(ctx, t, &pb.DeleteProjectRequest{
		Name: want[1].GetName(),
	})

	// Listing the projects with show_deleted = true should include the soft
	// deleted project.
	got := s.client.ListAllProjectsT(ctx, t, &pb.ListProjectsRequest{
		PageSize:    int32(len(want)),
		ShowDeleted: true,
	})
	if diff := cmp.Diff(want, got, protocmp.Transform(), cmpopts.SortSlices(projectLessFunc)); diff != "" {
		t.Errorf("unexpected result of ListProjects with show_deleted = true (-want +got)\n%s", diff)
	}

	// After the soft deleted project has expired it should no longer show up in
	// ListProjects.
	s.clock.Advance(want[1].GetExpireTime().AsTime().Sub(s.clock.Now()))
	s.clock.Advance(1 * time.Minute)
	wantAfterExpiration := []*pb.Project{
		want[0],
		want[2],
	}
	got = s.client.ListAllProjectsT(ctx, t, &pb.ListProjectsRequest{
		PageSize:    int32(len(want)),
		ShowDeleted: true,
	})
	if diff := cmp.Diff(wantAfterExpiration, got, protocmp.Transform(), cmpopts.SortSlices(projectLessFunc)); diff != "" {
		t.Errorf("after expiration: unexpected result of ListProjects with show_deleted = true (-want +got)\n%s", diff)
	}
}

func (s *Suite) TestListProjects_WithAdditions() {
	t := s.T()
	ctx := context.Background()

	projects := s.client.CreateProjectsT(ctx, t, []*pb.Project{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	firstPageSize := len(projects) - 1

	// Get the first page.
	res := s.client.ListProjectsT(ctx, t, &pb.ListProjectsRequest{
		PageSize: int32(firstPageSize), // Make sure we don't get everything.
	})
	wantFirstPage := projects[:firstPageSize]
	if diff := cmp.Diff(wantFirstPage, res.GetProjects(), protocmp.Transform(), protocmp.SortRepeated(projectLessFunc)); diff != "" {
		t.Errorf("unexpected first page (-want +got)\n%s", diff)
	}
	token := res.GetNextPageToken()
	if token == "" {
		t.Fatalf("first page returned empty next_page_token")
	}

	// Add a new project.
	projects = append(projects, s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{Title: "Feed sourdough"},
	}))

	// Get the second page, which should contain the new project.
	res = s.client.ListProjectsT(ctx, t, &pb.ListProjectsRequest{
		PageSize:  int32(len(projects)), // Try to make sure we get everything.
		PageToken: token,
	})
	wantSecondPage := projects[firstPageSize:]
	if diff := cmp.Diff(wantSecondPage, res.GetProjects(), protocmp.Transform(), protocmp.SortRepeated(projectLessFunc)); diff != "" {
		t.Errorf("unexpected second page (-want +got)\n%s", diff)
	}
	if got, want := res.GetNextPageToken(), ""; got != want {
		t.Errorf("second page: next_page_token = %q; want %q", got, want)
	}
}

func (s *Suite) TestListProjects_SamePageTokenTwice() {
	t := s.T()
	ctx := context.Background()

	projects := s.client.CreateProjectsT(ctx, t, []*pb.Project{
		{Title: "Buy milk"},
		{Title: "Do the laundry"},
		{Title: "Get swole"},
	})

	// Get the first page.
	res := s.client.ListProjectsT(ctx, t, &pb.ListProjectsRequest{
		PageSize: int32(len(projects) - 1), // Make sure we need at least one other page.
	})
	wantFirstPage := projects[:len(projects)-1]
	if diff := cmp.Diff(wantFirstPage, res.GetProjects(), protocmp.Transform(), protocmp.SortRepeated(projectLessFunc)); diff != "" {
		t.Errorf("unexpected first page (-want +got)\n%s", diff)
	}
	token := res.GetNextPageToken()
	if token == "" {
		t.Fatalf("first page returned empty next_page_token")
	}

	// Get the second page.
	req := &pb.ListProjectsRequest{
		PageSize:  int32(len(projects)), // Make sure we try to get everything.
		PageToken: token,
	}
	res = s.client.ListProjectsT(ctx, t, req)
	wantSecondPage := projects[len(projects)-1:]
	if diff := cmp.Diff(wantSecondPage, res.GetProjects(), protocmp.Transform(), protocmp.SortRepeated(projectLessFunc)); diff != "" {
		t.Errorf("unexpected second page (-want +got)\n%s", diff)
	}
	if got, want := res.GetNextPageToken(), ""; got != want {
		t.Errorf("second page: next_page_token = %q; want %q", got, want)
	}

	// Now try getting the second page again. This shouldn't work -- the last
	// page token should have been "consumed".
	_, err := s.client.ListProjects(ctx, req)
	if got, want := status.Code(err), codes.InvalidArgument; got != want {
		t.Errorf("second page again: return code = %v; want %v", got, want)
	}
}

func (s *Suite) TestListProjects_ChangeRequestBetweenPages() {
	t := s.T()
	ctx := context.Background()

	projects := s.client.CreateProjectsT(ctx, t, []*pb.Project{
		{Title: "Buy milk"},
		{Title: "Get swole"},
	})

	req := &pb.ListProjectsRequest{
		PageSize:    1,
		ShowDeleted: false,
	}

	// Getting the first page should succeed without problems.
	{
		res := s.client.ListProjectsT(ctx, t, req)
		want := projects[:1]
		if diff := cmp.Diff(want, res.GetProjects(), protocmp.Transform(), cmpopts.SortSlices(projectLessFunc)); diff != "" {
			t.Errorf("first page: unexpected results (-want +got)\n%s", diff)
		}
		req.PageToken = res.GetNextPageToken()
	}

	// Now we change the request parameters between pages, which should cause an error.
	req.ShowDeleted = true
	_, err := s.client.ListProjects(ctx, req)
	if got, want := status.Code(err), codes.InvalidArgument; got != want {
		t.Errorf("after changing request: ListProjects(%v) code = %v; want %v", req, got, want)
		t.Logf("err = %v", err)
	}
}

// Regression test for a bug. The Postgres implementation didn't set the
// `update_time` field correctly when listing projects.
func (s *Suite) TestListProjects_IncludesArchived() {
	t := s.T()
	ctx := context.Background()
	t.SkipNow()

	projects := s.client.CreateProjectsT(ctx, t, []*pb.Project{
		{Title: "kick ass"},
		{Title: "chew bubblegum"},
	})
	s.clock.Advance(30 * time.Hour)
	projects[0] = s.client.ArchiveProjectT(ctx, t, &pb.ArchiveProjectRequest{Name: projects[0].GetName()})

	res := s.client.ListProjectsT(ctx, t, &pb.ListProjectsRequest{})
	less := func(p1, p2 *pb.Project) bool { return p1.GetName() < p2.GetName() }
	if diff := cmp.Diff(projects, res.GetProjects(), protocmp.Transform(), cmpopts.SortSlices(less)); diff != "" {
		t.Fatalf("Unexpected diff when listing projects (-want +got)\n%s", diff)
	}
}

func (s *Suite) TestListProjects_Error() {
	t := s.T()
	ctx := context.Background()
	for _, tt := range []struct {
		name string
		req  *pb.ListProjectsRequest
		want codes.Code
	}{
		{
			name: "NegativePageSize",
			req: &pb.ListProjectsRequest{
				PageSize: -10,
			},
			want: codes.InvalidArgument,
		},
		{
			name: "BogusPageToken",
			req: &pb.ListProjectsRequest{
				PageToken: "this is some complete bonkers",
			},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.ListProjects(ctx, tt.req)
			if got, want := status.Code(err), tt.want; got != want {
				t.Errorf("ListProjects(%v) code = %v; want %v", tt.req, got, want)
				t.Logf("err = %v", err)
			}
		})
	}
}

func (s *Suite) TestCreateProject() {
	t := s.T()
	ctx := context.Background()

	project := &pb.Project{Title: "Hello Projects"}
	req := &pb.CreateProjectRequest{
		Project: project,
	}
	got, err := s.client.CreateProject(ctx, req)
	if err != nil {
		t.Fatalf("CreateProject(%v) err = %v; want nil", req, err)
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
	if diff := cmp.Diff(project, got, protocmp.Transform(), protocmp.IgnoreFields(project, "name", "create_time")); diff != "" {
		t.Errorf("CreateProject(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func (s *Suite) TestCreateProject_Error() {
	t := s.T()
	ctx := context.Background()
	for _, tt := range []struct {
		name string
		req  *pb.CreateProjectRequest
		want codes.Code
	}{
		{
			name: "EmptyTitle",
			req: &pb.CreateProjectRequest{
				Project: &pb.Project{
					Title: "",
				},
			},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.CreateProject(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("CreateProject(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}
		})
	}
}

func (s *Suite) TestUpdateProject() {
	t := s.T()
	ctx := context.Background()
	// Clock will be reset to createTime before the project is created.
	createTime := s.clock.Now()
	createTimeMessage := timestamppb.New(createTime)
	// Clock will be advanced to updateTime before the project is updated but after
	// it has been created.
	updateTime := createTime.Add(30 * time.Minute)
	updateTimeMessage := timestamppb.New(updateTime)
	for _, tt := range []struct {
		name    string
		project *pb.Project
		req     *pb.UpdateProjectRequest // will be updated in-place with the created project name
		want    *pb.Project              // will be updated in-place with the created project name
	}{
		{
			name: "EmptyUpdate_NilUpdateMask",
			project: &pb.Project{
				Title: "Before the update",
			},
			req: &pb.UpdateProjectRequest{
				Project:    &pb.Project{},
				UpdateMask: nil,
			},
			want: &pb.Project{
				Title:      "Before the update",
				CreateTime: createTimeMessage,
				UpdateTime: nil, // Project shouldn't be updated.
			},
		},
		{
			name: "EmptyUpdate_EmptyUpdateMask",
			project: &pb.Project{
				Title: "Before the update",
			},
			req: &pb.UpdateProjectRequest{
				Project:    &pb.Project{},
				UpdateMask: &fieldmaskpb.FieldMask{},
			},
			want: &pb.Project{
				Title:      "Before the update",
				CreateTime: createTimeMessage,
				UpdateTime: nil, // Project shouldn't be updated.
			},
		},
		{
			name: "UpdateTitle_NilUpdateMask",
			project: &pb.Project{
				Title: "Before the update",
			},
			req: &pb.UpdateProjectRequest{
				Project:    &pb.Project{Title: "After the update"},
				UpdateMask: nil,
			},
			want: &pb.Project{
				Title:      "After the update",
				CreateTime: createTimeMessage,
				UpdateTime: updateTimeMessage,
			},
		},
		{
			name: "UpdateTitle_EmptyUpdateMask",
			project: &pb.Project{
				Title: "Before the update",
			},
			req: &pb.UpdateProjectRequest{
				Project:    &pb.Project{Title: "After the update"},
				UpdateMask: &fieldmaskpb.FieldMask{},
			},
			want: &pb.Project{
				Title:      "After the update",
				CreateTime: createTimeMessage,
				UpdateTime: updateTimeMessage,
			},
		},
		{
			name: "UpdateTitle_MultipleFieldsPresent",
			project: &pb.Project{
				Title: "Before the update",
			},
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Title:       "After the update",
					Description: "You should never see this",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"title"},
				},
			},
			want: &pb.Project{
				Title:      "After the update",
				CreateTime: createTimeMessage,
				UpdateTime: updateTimeMessage,
			},
		},
		{
			name: "UpdateMultipleFields_NilUpdateMask",
			project: &pb.Project{
				Title: "Before the update",
			},
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Title:       "After the update",
					Description: "Added a description",
				},
				UpdateMask: nil,
			},
			want: &pb.Project{
				Title:       "After the update",
				Description: "Added a description",
				CreateTime:  createTimeMessage,
				UpdateTime:  updateTimeMessage,
			},
		},
		{
			name: "UpdateMultipleFields_EmptyUpdateMask",
			project: &pb.Project{
				Title: "Before the update",
			},
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Title:       "After the update",
					Description: "Added a description",
				},
				UpdateMask: &fieldmaskpb.FieldMask{},
			},
			want: &pb.Project{
				Title:       "After the update",
				Description: "Added a description",
				CreateTime:  createTimeMessage,
				UpdateTime:  updateTimeMessage,
			},
		},
		{
			name: "UpdateMultipleFields_NonEmptyUpdateMask",
			project: &pb.Project{
				Title: "Before the update",
			},
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
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
			want: &pb.Project{
				Title:       "After the update",
				Description: "Added a description",
				CreateTime:  createTimeMessage,
				UpdateTime:  updateTimeMessage,
			},
		},
		{
			name: "UpdateMultipleFields_StarMask",
			project: &pb.Project{
				Title: "Before the update",
			},
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Title:       "After the update",
					Description: "Added a description",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"*"},
				},
			},
			want: &pb.Project{
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
			project: &pb.Project{
				Title:       "Before the update",
				Description: "This is a description",
			},
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Title:       "After the update",
					Description: "",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"*"},
				},
			},
			want: &pb.Project{
				Title:       "After the update",
				Description: "",
				CreateTime:  createTimeMessage,
				UpdateTime:  updateTimeMessage,
			},
		},
		{
			// Trying to update the project with identical values should be a
			// no-op. This should be indicated by a missing `update_time` value.
			name: "IdenticalTitle",
			project: &pb.Project{
				Title: "Before the update",
			},
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Title:       "Before the update",
					Description: "",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"title"},
				},
			},
			want: &pb.Project{
				Title:      "Before the update",
				CreateTime: createTimeMessage,
				UpdateTime: nil, // Project shouldn't be updated.
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// We need to reset the time to createTime.
			// We want to find `d` such that `now + d = createTime.`
			// Therefore `d = createTime - now.`
			s.clock.Advance(createTime.Sub(s.clock.Now()))

			project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
				Project: tt.project,
			})

			// Before we do the update we advance time, so that `update_time` is
			// not the same as `create_time`.
			s.clock.Advance(30 * time.Minute)

			// Below we do the actual update.
			tt.req.Project.Name = project.GetName()
			tt.want.Name = project.GetName()
			got := s.client.UpdateProjectT(ctx, t, tt.req)
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected result of update (-want +got)\n%s", diff)
			}
			// Getting the project again should produce the same result as after
			// the update.
			got = s.client.GetProjectT(ctx, t, &pb.GetProjectRequest{
				Name: project.GetName(),
			})
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected result of GetProject after update (-want +got)\n%s", diff)
			}
		})
	}
}

func (s *Suite) TestUpdateProject_MultipleUpdates() {
	t := s.T()
	ctx := context.Background()

	// This test asserts that the update time is changed everytime the project is
	// updated.

	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title: "some project",
		},
	})

	// First update.
	{
		s.clock.Advance(15 * time.Minute)
		project.Title = "some project, now with an updated title"
		project.UpdateTime = timestamppb.New(s.clock.Now())
		gotProject := s.client.UpdateProjectT(ctx, t, &pb.UpdateProjectRequest{
			Project: project,
		})
		if diff := cmp.Diff(project, gotProject, protocmp.Transform()); diff != "" {
			t.Fatalf("Unexpected result after first update (-want +got)\n%s", diff)
		}
	}

	// Second update.
	{
		s.clock.Advance(2 * time.Hour)
		project.Description = "now with an added description"
		project.UpdateTime = timestamppb.New(s.clock.Now())
		gotProject := s.client.UpdateProjectT(ctx, t, &pb.UpdateProjectRequest{
			Project: project,
		})
		if diff := cmp.Diff(project, gotProject, protocmp.Transform()); diff != "" {
			t.Fatalf("Unexpected result after first update (-want +got)\n%s", diff)
		}
	}
}

func (s *Suite) TestUpdateProject_Error() {
	t := s.T()
	ctx := context.Background()
	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title:       "Some project",
			Description: "That also has a description",
		},
	})

	for _, tt := range []struct {
		name string
		req  *pb.UpdateProjectRequest
		want codes.Code
	}{
		{
			name: "NoName",
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Name:  "",
					Title: "I want to change the title",
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName",
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Name:  "invalidlolol/123",
					Title: "I want to change the title",
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Name:  "projects/123",
					Title: "I want to change the title",
				},
			},
			want: codes.NotFound,
		},
		{
			name: "InvalidFieldInUpdateMask",
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Name:  project.GetName(),
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
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Name:  project.GetName(),
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
			req: &pb.UpdateProjectRequest{
				Project: &pb.Project{
					Name: project.GetName(),
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"name"},
				},
			},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.UpdateProject(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("UpdateProject(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}

			// After the failed update the project should be intact.
			got := s.client.GetProjectT(ctx, t, &pb.GetProjectRequest{
				Name: project.GetName(),
			})
			if diff := cmp.Diff(project, got, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected project after failed update (-want +got)\n%s", diff)
			}
		})
	}
}

func (s *Suite) TestUpdateProject_AfterDeletion() {
	t := s.T()
	ctx := context.Background()
	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title:       "A project that will be deleted",
			Description: "This project is not long for this world",
		},
	})
	s.client.DeleteProjectT(ctx, t, &pb.DeleteProjectRequest{
		Name: project.GetName(),
	})

	req := &pb.UpdateProjectRequest{
		Project: &pb.Project{
			Name:  project.GetName(),
			Title: "You should never see this",
		},
	}
	updated, err := s.client.UpdateProject(ctx, req)
	if got, want := status.Code(err), codes.NotFound; got != want {
		t.Errorf("after deletion: UpdateProject(%v) code = %v; want %v", req, got, want)
		t.Logf("after deletion: returned project: %v", updated)
	}
}

func (s *Suite) TestDeleteProject() {
	t := s.T()
	ctx := context.Background()

	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{Title: "This will be deleted"},
	})

	// Once the project has been created it should be deleted.
	{
		req := &pb.DeleteProjectRequest{Name: project.GetName()}
		deleted, err := s.client.DeleteProject(ctx, req)
		if err != nil {
			t.Fatalf("first deletion: DeleteProject(%v) err = %v; want nil", req, err)
		}
		if err := deleted.GetDeleteTime().CheckValid(); err != nil {
			t.Errorf("first deletion: delete_time is invalid: %v", err)
		}
		if err := deleted.GetExpireTime().CheckValid(); err != nil {
			t.Errorf("first deletion: expire_time is invalid: %v", err)
		}
		if delete, expiry := deleted.GetDeleteTime().AsTime(), deleted.GetExpireTime().AsTime(); expiry.Before(delete) {
			t.Errorf("first deletion: delete_time = %v; wanted before expire_time = %v", delete, expiry)
		}
	}

	// Deleting the project again should result in a NotFound error.
	{
		req := &pb.DeleteProjectRequest{Name: project.GetName()}
		_, err := s.client.DeleteProject(ctx, req)
		if got, want := status.Code(err), codes.NotFound; got != want {
			t.Fatalf("second deletion: DeleteProject(%v) code = %v; want %v", req, got, want)
		}
	}
}

func (s *Suite) TestDeleteProject_Error() {
	t := s.T()
	ctx := context.Background()
	for _, tt := range []struct {
		name string
		req  *pb.DeleteProjectRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req:  &pb.DeleteProjectRequest{Name: ""},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req:  &pb.DeleteProjectRequest{Name: "projects/notfound"},
			want: codes.NotFound,
		},
		{
			name: "InvalidName",
			req:  &pb.DeleteProjectRequest{Name: "invalidlololol/1"},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.DeleteProject(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("DeleteProject(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}
		})
	}
}

func (s *Suite) TestUndeleteProject() {
	t := s.T()
	ctx := context.Background()

	// Create project, soft delete it, then undelete it. The result should be the
	// same project as just after it was created.
	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title:       "This will be deleted",
			Description: "And later undeleted, woohoo!",
		},
	})
	s.client.DeleteProjectT(ctx, t, &pb.DeleteProjectRequest{
		Name: project.GetName(),
	})
	undeleted := s.client.UndeleteProjectT(ctx, t, &pb.UndeleteProjectRequest{
		Name: project.GetName(),
	})
	if diff := cmp.Diff(project, undeleted, protocmp.Transform()); diff != "" {
		t.Errorf("unexpected result after undeletion (-before +after)\n%s", diff)
	}
}

func (s *Suite) TestUndeleteProject_Error() {
	t := s.T()
	ctx := context.Background()
	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title: "Buy milk",
		},
	})

	for _, tt := range []struct {
		name string
		req  *pb.UndeleteProjectRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req: &pb.UndeleteProjectRequest{
				Name: "",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &pb.UndeleteProjectRequest{
				Name: "projects/notfound",
			},
			want: codes.NotFound,
		},
		{
			name: "InvalidName",
			req: &pb.UndeleteProjectRequest{
				Name: "invalidlololol/1",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotDeleted",
			req: &pb.UndeleteProjectRequest{
				Name: project.GetName(),
			},
			want: codes.AlreadyExists,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.UndeleteProject(ctx, tt.req)
			if got, want := status.Code(err), tt.want; got != want {
				t.Errorf("UndeleteProject(%v) code = %v; want %v", tt.req, got, want)
				t.Logf("err = %v", err)
			}
		})
	}
}

func (s *Suite) TestArchiveProject_UnarchiveProject_ClearsArchiveTime() {
	t := s.T()
	ctx := context.Background()
	t.SkipNow()

	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title: "Get swole",
		},
	})

	// Complete the project after 30 minutes.
	{
		s.clock.Advance(30 * time.Minute)
		now := s.clock.Now()
		project.ArchiveTime = timestamppb.New(now)
		project.UpdateTime = timestamppb.New(now)
		req := &pb.ArchiveProjectRequest{
			Name: project.GetName(),
		}
		got := s.client.ArchiveProjectT(ctx, t, req)
		if diff := cmp.Diff(project, got, protocmp.Transform()); diff != "" {
			t.Fatalf("ArchiveProject(%v) produced unexpected result (-want +got)\n%s", req, diff)
		}
	}

	// Uncomplete the project after another 30 minutes.
	{
		s.clock.Advance(30 * time.Minute)
		project.ArchiveTime = nil
		project.UpdateTime = timestamppb.New(s.clock.Now())
		req := &pb.UnarchiveProjectRequest{
			Name: project.GetName(),
		}
		got := s.client.UnarchiveProjectT(ctx, t, req)
		if diff := cmp.Diff(project, got, protocmp.Transform()); diff != "" {
			t.Fatalf("UnarchiveProject(%v) produced unexpected result (-want +got)\n%s", req, diff)
		}
	}
}

func (s *Suite) TestArchiveProject_AlreadyArchived() {
	t := s.T()
	ctx := context.Background()
	t.SkipNow()

	// When trying to complete a project that is already completed, it should be a
	// no-op and the project should be returned unmodified. We detect this by
	// simulating time passing, which should be the only change in the world
	// between the various operations on the project.

	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title: "Build stuff",
		},
	})

	s.clock.Advance(30 * time.Minute)

	first := s.client.ArchiveProjectT(ctx, t, &pb.ArchiveProjectRequest{
		Name: project.GetName(),
	})

	s.clock.Advance(30 * time.Minute)

	second := s.client.ArchiveProjectT(ctx, t, &pb.ArchiveProjectRequest{
		Name: project.GetName(),
	})
	if diff := cmp.Diff(first, second, protocmp.Transform()); diff != "" {
		t.Fatalf("Unexpected result of completing a second time (-first +second)\n%s", diff)
	}
}

func (s *Suite) TestArchiveProject_Deleted() {
	t := s.T()
	ctx := context.Background()
	t.SkipNow()

	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title: "should be deleted",
		},
	})
	project = s.client.DeleteProjectT(ctx, t, &pb.DeleteProjectRequest{
		Name: project.GetName(),
	})

	req := &pb.ArchiveProjectRequest{
		Name: project.GetName(),
	}
	_, err := s.client.ArchiveProject(ctx, req)
	if got, want := status.Code(err), codes.NotFound; got != want {
		t.Fatalf("ArchiveProject(%v) err = %v; want code %v", req, err, want)
	}
}

func (s *Suite) TestArchiveProject_Error() {
	t := s.T()
	ctx := context.Background()
	t.SkipNow()

	for _, tt := range []struct {
		name string
		req  *pb.ArchiveProjectRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req: &pb.ArchiveProjectRequest{
				Name: "",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName",
			req: &pb.ArchiveProjectRequest{
				Name: "invalid/123",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "MissingResourceID",
			req: &pb.ArchiveProjectRequest{
				Name: "projects/",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &pb.ArchiveProjectRequest{
				Name: "projects/999",
			},
			want: codes.NotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.ArchiveProject(ctx, tt.req)
			if got, want := status.Code(err), tt.want; got != want {
				t.Fatalf("ArchiveProject(%v) err = %v; want code %v", tt.req, err, want)
			}
		})
	}
}

func (s *Suite) TestUnarchiveProject_NotArchived() {
	t := s.T()
	ctx := context.Background()
	t.SkipNow()

	project := s.client.CreateProjectT(ctx, t, &pb.CreateProjectRequest{
		Project: &pb.Project{
			Title: "some project",
		},
	})

	// Unarchiving a project that is not archived should be a no-op.
	got := s.client.UnarchiveProjectT(ctx, t, &pb.UnarchiveProjectRequest{
		Name: project.GetName(),
	})
	if diff := cmp.Diff(project, got, protocmp.Transform()); diff != "" {
		t.Fatalf("Uncompleting an uncompleted project wasn't a no-op (-want +got)\n%s", diff)
	}
}
