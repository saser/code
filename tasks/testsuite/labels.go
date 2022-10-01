package testsuite

import (
	"context"
	"fmt"
	"strings"
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

func (s *Suite) TestGetLabel() {
	t := s.T()
	ctx := context.Background()

	label := s.client.CreateLabelT(ctx, t, &pb.CreateLabelRequest{
		Label: &pb.Label{
			Label: "email",
		},
	})

	// Getting the label by name should produce the same result.
	req := &pb.GetLabelRequest{
		Name: label.GetName(),
	}
	got, err := s.client.GetLabel(ctx, req)
	if err != nil {
		t.Fatalf("GetLabel(%v) err = %v; want nil", req, err)
	}
	if diff := cmp.Diff(label, got, protocmp.Transform()); diff != "" {
		t.Errorf("GetLabel(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func (s *Suite) TestGetLabel_AfterDeletion() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	label := s.client.CreateLabelT(ctx, t, &pb.CreateLabelRequest{
		Label: &pb.Label{
			Label: "email",
		},
	})

	// Getting the label by name should produce the same result.
	{
		req := &pb.GetLabelRequest{
			Name: label.GetName(),
		}
		got := s.client.GetLabelT(ctx, t, req)
		if diff := cmp.Diff(label, got, protocmp.Transform()); diff != "" {
			t.Errorf("Before deletion: GetLabel(%v): unexpected result (-want +got)\n%s", req, diff)
		}
	}

	// After deleting the label, getting the label by name should fail.
	{
		s.client.DeleteLabelT(ctx, t, &pb.DeleteLabelRequest{
			Name: label.GetName(),
		})
		req := &pb.GetLabelRequest{
			Name: label.GetName(),
		}
		_, err := s.client.GetLabel(ctx, req)
		if got, want := status.Code(err), codes.NotFound; got != want {
			t.Errorf("After deletion: GetLabel(%v) err = %v; want code %v", req, err, want)
		}
	}
}

func (s *Suite) TestGetLabel_Error() {
	t := s.T()
	ctx := context.Background()

	for _, tt := range []struct {
		name string
		req  *pb.GetLabelRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req:  &pb.GetLabelRequest{Name: ""},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName",
			req:  &pb.GetLabelRequest{Name: "invalid/123"},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName_NoResourceID",
			req: &pb.GetLabelRequest{
				Name: "labels/",
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req:  &pb.GetLabelRequest{Name: "labels/999"},
			want: codes.NotFound,
		},
		{
			name: "NotFound_DifferentResourceIDFormat",
			req: &pb.GetLabelRequest{
				// This is a valid name -- there is no guarantee what format the
				// resource ID (the segment after the slash) will have. But it
				// probably won't be arbitrary strings.
				Name: "labels/invalidlol",
			},
			want: codes.NotFound,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.GetLabel(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("GetLabel(%v) code = %v; want %v", tt.req, got, tt.want)
			}
		})
	}
}

func (s *Suite) TestListLabels() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	want := s.client.CreateLabelsT(ctx, t, []*pb.Label{
		{Label: "email"},
		{Label: "phonecall"},
		{Label: "home"},
	})

	req := &pb.ListLabelsRequest{
		PageSize: int32(len(want)),
	}
	res, err := s.client.ListLabels(ctx, req)
	if err != nil {
		t.Fatalf("ListLabels(%v) err = %v; want nil", req, err)
	}
	if diff := cmp.Diff(want, res.GetLabels(), protocmp.Transform(), cmpopts.SortSlices(labelLessFunc)); diff != "" {
		t.Errorf("ListLabels(%v): unexpected result (-want +got)\n%s", req, diff)
	}
	if got, want := res.GetNextPageToken(), ""; got != want {
		t.Errorf("ListLabels(%v) next_page_token = %q; want %q", req, got, want)
	}
}

func (s *Suite) TestListLabels_MaxPageSize() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	labels := make([]*pb.Label, s.maxPageSize*2-s.maxPageSize/2)
	for i := range labels {
		labels[i] = s.client.CreateLabelT(ctx, t, &pb.CreateLabelRequest{
			Label: &pb.Label{
				Label: fmt.Sprint(i),
			},
		})
	}

	req := &pb.ListLabelsRequest{
		PageSize: int32(len(labels)), // more than maxPageSize
	}

	res := s.client.ListLabelsT(ctx, t, req)
	wantFirstPage := labels[:s.maxPageSize]
	if diff := cmp.Diff(wantFirstPage, res.GetLabels(), protocmp.Transform(), cmpopts.SortSlices(labelLessFunc)); diff != "" {
		t.Errorf("[first page] ListLabels(%v): unexpected result (-want +got)\n%s", req, diff)
	}

	req.PageToken = res.GetNextPageToken()
	res = s.client.ListLabelsT(ctx, t, req)
	wantSecondPage := labels[s.maxPageSize:]
	if diff := cmp.Diff(wantSecondPage, res.GetLabels(), protocmp.Transform(), cmpopts.SortSlices(labelLessFunc)); diff != "" {
		t.Errorf("[second page] ListLabels(%v): unexpected result (-want +got)\n%s", req, diff)
	}
}

func (s *Suite) TestListLabels_DifferentPageSizes() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	// 7 labels. Number chosen arbitrarily.
	labels := s.client.CreateLabelsT(ctx, t, []*pb.Label{
		{Label: "email"},
		{Label: "phonecall"},
		{Label: "home"},
		{Label: "office"},
		{Label: "agendas:boss"},
		{Label: "agendas:partner"},
		{Label: "misc"},
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
			// of labels, and that we won't try to get more pages after the
			// last one.
			{
				sum := int32(0)
				for i, s := range sizes {
					if s <= 0 {
						t.Errorf("sizes[%d] = %v; want a positive number", i, s)
					}
					sum += s
				}
				n := int32(len(labels))
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
			// Now we can start listing labels.
			req := &pb.ListLabelsRequest{}
			var got []*pb.Label
			for i, size := range sizes {
				req.PageSize = size
				res := s.client.ListLabelsT(ctx, t, req)
				got = append(got, res.GetLabels()...)
				token := res.GetNextPageToken()
				if i < len(sizes)-1 && token == "" {
					// This error does not apply for the last page.
					t.Fatalf("[after page %d]: ListLabels(%v) next_page_token = %q; want non-empty", i, req, token)
				}
				req.PageToken = token
			}
			// After all the page sizes the page token should be empty.
			if got, want := req.GetPageToken(), ""; got != want {
				t.Fatalf("[after all pages] page_token = %q; want %q", got, want)
			}
			if diff := cmp.Diff(labels, got, protocmp.Transform(), cmpopts.SortSlices(labelLessFunc)); diff != "" {
				t.Errorf("unexpected result (-want +got)\n%s", diff)
			}
		})
	}
}

func (s *Suite) TestListLabels_WithDeletions() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	seed := []*pb.Label{
		{Label: "first"},
		{Label: "second"},
		{Label: "third"},
	}

	for _, tt := range []struct {
		name                  string
		firstPageSize         int32
		wantFirstPageIndices  []int // indices into created labels
		deleteIndex           int
		wantSecondPageIndices []int // indices into created labels
	}{
		{
			name:                  "DeleteInFirstPage_TwoLabelsInFirstPage",
			firstPageSize:         2,
			wantFirstPageIndices:  []int{0, 1},
			deleteIndex:           1,
			wantSecondPageIndices: []int{2},
		},
		{
			name:                  "DeleteInFirstPage_OneLabelInFirstPage",
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
			name:                  "DeleteInSecondPage_TwoLabelsInFirstPage",
			firstPageSize:         2,
			wantFirstPageIndices:  []int{0, 1},
			deleteIndex:           2,
			wantSecondPageIndices: []int{},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			defer s.truncate(ctx)
			labels := s.client.CreateLabelsT(ctx, t, seed)

			// Get the first page and assert that it matches what we want.
			req := &pb.ListLabelsRequest{
				PageSize: tt.firstPageSize,
			}
			res := s.client.ListLabelsT(ctx, t, req)
			wantFirstPage := make([]*pb.Label, 0, len(tt.wantFirstPageIndices))
			for _, idx := range tt.wantFirstPageIndices {
				wantFirstPage = append(wantFirstPage, labels[idx])
			}
			if diff := cmp.Diff(wantFirstPage, res.GetLabels(), protocmp.Transform(), protocmp.SortRepeated(labelLessFunc)); diff != "" {
				t.Fatalf("first page: unexpected labels (-want +got)\n%s", diff)
			}
			token := res.GetNextPageToken()
			if token == "" {
				t.Fatal("no next page token from first page")
			}
			req.PageToken = token

			// Delete one of the labels.
			s.client.DeleteLabelT(ctx, t, &pb.DeleteLabelRequest{
				Name: labels[tt.deleteIndex].GetName(),
			})

			// Get the second page and assert that it matches what we want. Also
			// assert that there are no more labels.
			req.PageSize = int32(len(labels)) // Make sure we get the remaining labels.
			res = s.client.ListLabelsT(ctx, t, req)
			wantSecondPage := make([]*pb.Label, 0, len(tt.wantSecondPageIndices))
			for _, idx := range tt.wantSecondPageIndices {
				wantSecondPage = append(wantSecondPage, labels[idx])
			}
			if diff := cmp.Diff(wantSecondPage, res.GetLabels(), cmpopts.EquateEmpty(), protocmp.Transform(), protocmp.SortRepeated(labelLessFunc)); diff != "" {
				t.Fatalf("second page: unexpected labels (-want +got)\n%s", diff)
			}
			if got, want := res.GetNextPageToken(), ""; got != want {
				t.Errorf("second page: next_page_token = %q; want %q", got, want)
			}
		})
	}
}

func (s *Suite) TestListLabels_WithAdditions() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	labels := s.client.CreateLabelsT(ctx, t, []*pb.Label{
		{Label: "email"},
		{Label: "phonecall"},
		{Label: "home"},
	})

	firstPageSize := len(labels) - 1

	// Get the first page.
	res := s.client.ListLabelsT(ctx, t, &pb.ListLabelsRequest{
		PageSize: int32(firstPageSize), // Make sure we don't get everything.
	})
	wantFirstPage := labels[:firstPageSize]
	if diff := cmp.Diff(wantFirstPage, res.GetLabels(), protocmp.Transform(), protocmp.SortRepeated(labelLessFunc)); diff != "" {
		t.Errorf("unexpected first page (-want +got)\n%s", diff)
	}
	token := res.GetNextPageToken()
	if token == "" {
		t.Fatalf("first page returned empty next_page_token")
	}

	// Add a new label.
	labels = append(labels, s.client.CreateLabelT(ctx, t, &pb.CreateLabelRequest{
		Label: &pb.Label{Label: "office"},
	}))

	// Get the second page, which should contain the new label.
	res = s.client.ListLabelsT(ctx, t, &pb.ListLabelsRequest{
		PageSize:  int32(len(labels)), // Try to make sure we get everything.
		PageToken: token,
	})
	wantSecondPage := labels[firstPageSize:]
	if diff := cmp.Diff(wantSecondPage, res.GetLabels(), protocmp.Transform(), protocmp.SortRepeated(labelLessFunc)); diff != "" {
		t.Errorf("unexpected second page (-want +got)\n%s", diff)
	}
	if got, want := res.GetNextPageToken(), ""; got != want {
		t.Errorf("second page: next_page_token = %q; want %q", got, want)
	}
}

func (s *Suite) TestListLabels_SamePageTokenTwice() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	labels := s.client.CreateLabelsT(ctx, t, []*pb.Label{
		{Label: "email"},
		{Label: "phonecall"},
		{Label: "home"},
	})

	// Get the first page.
	res := s.client.ListLabelsT(ctx, t, &pb.ListLabelsRequest{
		PageSize: int32(len(labels) - 1), // Make sure we need at least one other page.
	})
	wantFirstPage := labels[:len(labels)-1]
	if diff := cmp.Diff(wantFirstPage, res.GetLabels(), protocmp.Transform(), protocmp.SortRepeated(labelLessFunc)); diff != "" {
		t.Errorf("unexpected first page (-want +got)\n%s", diff)
	}
	token := res.GetNextPageToken()
	if token == "" {
		t.Fatalf("first page returned empty next_page_token")
	}

	// Get the second page.
	req := &pb.ListLabelsRequest{
		PageSize:  int32(len(labels)), // Make sure we try to get everything.
		PageToken: token,
	}
	res = s.client.ListLabelsT(ctx, t, req)
	wantSecondPage := labels[len(labels)-1:]
	if diff := cmp.Diff(wantSecondPage, res.GetLabels(), protocmp.Transform(), protocmp.SortRepeated(labelLessFunc)); diff != "" {
		t.Errorf("unexpected second page (-want +got)\n%s", diff)
	}
	if got, want := res.GetNextPageToken(), ""; got != want {
		t.Errorf("second page: next_page_token = %q; want %q", got, want)
	}

	// Now try getting the second page again. This shouldn't work -- the last
	// page token should have been "consumed".
	_, err := s.client.ListLabels(ctx, req)
	if got, want := status.Code(err), codes.InvalidArgument; got != want {
		t.Errorf("second page again: return code = %v; want %v", got, want)
	}
}

func (s *Suite) TestListLabels_Error() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	for _, tt := range []struct {
		name string
		req  *pb.ListLabelsRequest
		want codes.Code
	}{
		{
			name: "NegativePageSize",
			req: &pb.ListLabelsRequest{
				PageSize: -10,
			},
			want: codes.InvalidArgument,
		},
		{
			name: "BogusPageToken",
			req: &pb.ListLabelsRequest{
				PageToken: "this is some complete bonkers",
			},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.ListLabels(ctx, tt.req)
			if got, want := status.Code(err), tt.want; got != want {
				t.Errorf("ListLabels(%v) code = %v; want %v", tt.req, got, want)
				t.Logf("err = %v", err)
			}
		})
	}
}

func (s *Suite) TestCreateLabel() {
	t := s.T()
	ctx := context.Background()

	for _, req := range []*pb.CreateLabelRequest{
		{Label: &pb.Label{Label: "email"}},
		{Label: &pb.Label{Label: "PhoneCall"}},
		{Label: &pb.Label{Label: "Agendas:Boss"}},
		{Label: &pb.Label{Label: "@Agendas_Boss"}},
		{Label: &pb.Label{Label: "128931750"}},
		{Label: &pb.Label{Label: "-_@:"}},
	} {
		t.Run(req.GetLabel().GetLabel(), func(t *testing.T) {
			got, err := s.client.CreateLabel(ctx, req)
			if err != nil {
				t.Fatalf("CreateLabel(%v) err = %v; want nil", req, err)
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
			want := req.GetLabel()
			if diff := cmp.Diff(want, got, protocmp.Transform(), protocmp.IgnoreFields(&pb.Label{}, "name", "create_time")); diff != "" {
				t.Errorf("CreateLabel(%v): unexpected result (-want +got)\n%s", want, diff)
			}
		})
	}
}

func (s *Suite) TestCreateLabel_Duplicate() {
	t := s.T()
	ctx := context.Background()

	// We create the original label, which should succeed.
	original := s.client.CreateLabelT(ctx, t, &pb.CreateLabelRequest{
		Label: &pb.Label{
			Label: "original",
		},
	})

	// Now we try to create the duplicate label, which should fail.
	req := &pb.CreateLabelRequest{
		Label: &pb.Label{
			Label: original.GetLabel(),
		},
	}
	_, err := s.client.CreateLabel(ctx, req)
	if got, want := status.Code(err), codes.AlreadyExists; got != want {
		t.Fatalf("Creating duplicate: CreateLabel(%v) err = %v; want code %v", req, err, want)
	}
	// We want the error to point to the resource name of the existing label.
	if got, want := err.Error(), original.GetName(); !strings.Contains(got, want) {
		t.Errorf("Creating duplicate: CreateLabel(%v) err = %q; want substring %q", req, got, want)
	}
}

func (s *Suite) TestCreateLabel_Error() {
	t := s.T()
	ctx := context.Background()

	for _, tt := range []struct {
		name string
		req  *pb.CreateLabelRequest
		want codes.Code
	}{
		{
			name: "EmptyTitle",
			req: &pb.CreateLabelRequest{
				Label: &pb.Label{
					Label: "",
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "ForbiddenCharacters_OutsideAZ",
			req: &pb.CreateLabelRequest{
				Label: &pb.Label{
					Label: "bröd",
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "ForbiddenCharacters_OtherSpecialCharacters",
			req: &pb.CreateLabelRequest{
				Label: &pb.Label{
					Label: "!!!",
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "ForbiddenCharacters_Space",
			req: &pb.CreateLabelRequest{
				Label: &pb.Label{
					Label: "First label",
				},
			},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.CreateLabel(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("CreateLabel(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}
		})
	}
}

func (s *Suite) TestUpdateLabel() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	// Clock will be reset to createTime before the label is created.
	createTime := s.clock.Now()
	createTimeMessage := timestamppb.New(createTime)
	// Clock will be advanced to updateTime before the label is updated but after
	// it has been created.
	updateTime := createTime.Add(30 * time.Minute)
	updateTimeMessage := timestamppb.New(updateTime)
	for _, tt := range []struct {
		name  string
		label *pb.Label
		req   *pb.UpdateLabelRequest // will be updated in-place with the created label name
		want  *pb.Label              // will be updated in-place with the created label name
	}{
		{
			name: "EmptyUpdate_NilUpdateMask",
			label: &pb.Label{
				Label: "before",
			},
			req: &pb.UpdateLabelRequest{
				Label:      &pb.Label{},
				UpdateMask: nil,
			},
			want: &pb.Label{
				Label:      "before",
				CreateTime: createTimeMessage,
				UpdateTime: nil, // Label shouldn't be updated.
			},
		},
		{
			name: "EmptyUpdate_EmptyUpdateMask",
			label: &pb.Label{
				Label: "before",
			},
			req: &pb.UpdateLabelRequest{
				Label:      &pb.Label{},
				UpdateMask: &fieldmaskpb.FieldMask{},
			},
			want: &pb.Label{
				Label:      "before",
				CreateTime: createTimeMessage,
				UpdateTime: nil, // Label shouldn't be updated.
			},
		},
		{
			name: "UpdateTitle_NilUpdateMask",
			label: &pb.Label{
				Label: "before",
			},
			req: &pb.UpdateLabelRequest{
				Label:      &pb.Label{Label: "after"},
				UpdateMask: nil,
			},
			want: &pb.Label{
				Label:      "after",
				CreateTime: createTimeMessage,
				UpdateTime: updateTimeMessage,
			},
		},
		{
			name: "UpdateTitle_EmptyUpdateMask",
			label: &pb.Label{
				Label: "before",
			},
			req: &pb.UpdateLabelRequest{
				Label:      &pb.Label{Label: "after"},
				UpdateMask: &fieldmaskpb.FieldMask{},
			},
			want: &pb.Label{
				Label:      "after",
				CreateTime: createTimeMessage,
				UpdateTime: updateTimeMessage,
			},
		},
		{
			name: "UpdateTitle_StarMask",
			label: &pb.Label{
				Label: "before",
			},
			req: &pb.UpdateLabelRequest{
				Label: &pb.Label{
					Label: "after",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"*"},
				},
			},
			want: &pb.Label{
				Label:      "after",
				CreateTime: createTimeMessage,
				UpdateTime: updateTimeMessage,
			},
		},
		{
			// Trying to update the label with identical values should be a
			// no-op. This should be indicated by a missing `update_time` value.
			name: "IdenticalTitle",
			label: &pb.Label{
				Label: "before",
			},
			req: &pb.UpdateLabelRequest{
				Label: &pb.Label{
					Label: "before",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"title"},
				},
			},
			want: &pb.Label{
				Label:      "before",
				CreateTime: createTimeMessage,
				UpdateTime: nil, // Label shouldn't be updated.
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// We need to reset the time to createTime.
			// We want to find `d` such that `now + d = createTime.`
			// Therefore `d = createTime - now.`
			s.clock.Advance(createTime.Sub(s.clock.Now()))

			label := s.client.CreateLabelT(ctx, t, &pb.CreateLabelRequest{
				Label: tt.label,
			})

			// Before we do the update we advance time, so that `update_time` is
			// not the same as `create_time`.
			s.clock.Advance(30 * time.Minute)

			// Below we do the actual update.
			tt.req.Label.Name = label.GetName()
			tt.want.Name = label.GetName()
			got := s.client.UpdateLabelT(ctx, t, tt.req)
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected result of update (-want +got)\n%s", diff)
			}
			// Getting the label again should produce the same result as after
			// the update.
			got = s.client.GetLabelT(ctx, t, &pb.GetLabelRequest{
				Name: label.GetName(),
			})
			if diff := cmp.Diff(tt.want, got, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected result of GetLabel after update (-want +got)\n%s", diff)
			}
		})
	}
}

func (s *Suite) TestUpdateLabel_MultipleUpdates() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	// This test asserts that the update time is changed everytime the label is
	// updated.

	label := s.client.CreateLabelT(ctx, t, &pb.CreateLabelRequest{
		Label: &pb.Label{
			Label: "some-label",
		},
	})

	// First update.
	{
		s.clock.Advance(15 * time.Minute)
		label.Label = "updated-label"
		label.UpdateTime = timestamppb.New(s.clock.Now())
		gotLabel := s.client.UpdateLabelT(ctx, t, &pb.UpdateLabelRequest{
			Label: label,
		})
		if diff := cmp.Diff(label, gotLabel, protocmp.Transform()); diff != "" {
			t.Fatalf("Unexpected result after first update (-want +got)\n%s", diff)
		}
	}

	// Second update.
	{
		s.clock.Advance(2 * time.Hour)
		label.Label = "seriously-updated-label"
		label.UpdateTime = timestamppb.New(s.clock.Now())
		gotLabel := s.client.UpdateLabelT(ctx, t, &pb.UpdateLabelRequest{
			Label: label,
		})
		if diff := cmp.Diff(label, gotLabel, protocmp.Transform()); diff != "" {
			t.Fatalf("Unexpected result after first update (-want +got)\n%s", diff)
		}
	}
}

func (s *Suite) TestUpdateLabel_Error() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	label := s.client.CreateLabelT(ctx, t, &pb.CreateLabelRequest{
		Label: &pb.Label{
			Label: "some-label",
		},
	})

	for _, tt := range []struct {
		name string
		req  *pb.UpdateLabelRequest
		want codes.Code
	}{
		{
			name: "NoName",
			req: &pb.UpdateLabelRequest{
				Label: &pb.Label{
					Name:  "",
					Label: "updated-label",
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "InvalidName",
			req: &pb.UpdateLabelRequest{
				Label: &pb.Label{
					Name:  "invalidlolol/123",
					Label: "updated-label",
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &pb.UpdateLabelRequest{
				Label: &pb.Label{
					Name:  "labels/123",
					Label: "updated-label",
				},
			},
			want: codes.NotFound,
		},
		{
			name: "InvalidFieldInUpdateMask",
			req: &pb.UpdateLabelRequest{
				Label: &pb.Label{
					Name:  label.GetName(),
					Label: "updated-label",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"label_invalid"},
				},
			},
			want: codes.InvalidArgument,
		},
		{
			name: "BothFieldsAndWildcardInUpdateMask",
			req: &pb.UpdateLabelRequest{
				Label: &pb.Label{
					Name:  label.GetName(),
					Label: "updated-label",
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{
						"label",
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
			req: &pb.UpdateLabelRequest{
				Label: &pb.Label{
					Name: label.GetName(),
				},
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"name"},
				},
			},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.UpdateLabel(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("UpdateLabel(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}

			// After the failed update the label should be intact.
			got := s.client.GetLabelT(ctx, t, &pb.GetLabelRequest{
				Name: label.GetName(),
			})
			if diff := cmp.Diff(label, got, protocmp.Transform()); diff != "" {
				t.Errorf("unexpected label after failed update (-want +got)\n%s", diff)
			}
		})
	}
}

func (s *Suite) TestUpdateLabel_AfterDeletion() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	label := s.client.CreateLabelT(ctx, t, &pb.CreateLabelRequest{
		Label: &pb.Label{
			Label: "will_be_deleted",
		},
	})
	s.client.DeleteLabelT(ctx, t, &pb.DeleteLabelRequest{
		Name: label.GetName(),
	})

	req := &pb.UpdateLabelRequest{
		Label: &pb.Label{
			Name:  label.GetName(),
			Label: "should_never_show_up",
		},
	}
	_, err := s.client.UpdateLabel(ctx, req)
	if got, want := status.Code(err), codes.NotFound; got != want {
		t.Errorf("After deletion: UpdateLabel(%v) code = %v; want %v", req, got, want)
	}
}

func (s *Suite) TestDeleteLabel() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	label := s.client.CreateLabelT(ctx, t, &pb.CreateLabelRequest{
		Label: &pb.Label{
			Label: "will_be_deleted",
		},
	})

	// Once the label has been created it should be deleted.
	{
		req := &pb.DeleteLabelRequest{Name: label.GetName()}
		_, err := s.client.DeleteLabel(ctx, req)
		if err != nil {
			t.Fatalf("First deletion: DeleteLabel(%v) err = %v; want nil", req, err)
		}
	}

	// Deleting the label again should result in a NotFound error.
	{
		req := &pb.DeleteLabelRequest{Name: label.GetName()}
		_, err := s.client.DeleteLabel(ctx, req)
		if got, want := status.Code(err), codes.NotFound; got != want {
			t.Fatalf("Second deletion: DeleteLabel(%v) err = %v; want code %v", req, err, want)
		}
	}
}

func (s *Suite) TestDeleteLabel_Error() {
	t := s.T()
	ctx := context.Background()
	t.Skip("not implemented")

	for _, tt := range []struct {
		name string
		req  *pb.DeleteLabelRequest
		want codes.Code
	}{
		{
			name: "EmptyName",
			req:  &pb.DeleteLabelRequest{Name: ""},
			want: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req:  &pb.DeleteLabelRequest{Name: "labels/notfound"},
			want: codes.NotFound,
		},
		{
			name: "InvalidName",
			req:  &pb.DeleteLabelRequest{Name: "invalidlololol/1"},
			want: codes.InvalidArgument,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.client.DeleteLabel(ctx, tt.req)
			if got := status.Code(err); got != tt.want {
				t.Errorf("DeleteLabel(%v) code = %v; want %v", tt.req, got, tt.want)
				t.Logf("err = %v", err)
			}
		})
	}
}
