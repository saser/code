// Package fake provides an in-memory implementation of the Tasks service. It is
// intended to be used for integration tests or other places where the full
// SQL-backed implementation isn't appropriate.
package fake

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/google/uuid"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// maxPageSize is the maximum number of tasks the server will return on a call
// to ListTasks. Any request for more than maxPageSize tasks will only return
// (at most) maxPageSize tasks.
const maxPageSize = 1000

// internalError should be returned whenever something goes wrong with serving a
// request, and where the error cannot be attributed to the user making an
// invalid request, something cannot be found, etc.
var internalError = status.Error(codes.Internal, "Something went wrong.")

// updatableMask contains the fields that can be updated by UpdateTask. It must
// be kept in sync with the proto definition.
var updatableMask *fieldmaskpb.FieldMask

func init() {
	m, err := fieldmaskpb.New(&pb.Task{},
		"title",
		"description",
	)
	if err != nil {
		glog.Exit(err)
	}
	updatableMask = m
}

// Fake implements the Tasks service using only in-memory data structures.
type Fake struct {
	pb.UnimplementedTasksServer

	mu         sync.Mutex
	nextID     int
	tasks      []*pb.Task     // a nil element corresponds to a deleted task
	pageTokens map[string]int // token (UUID) -> index into `tasks` of task with minimum ID
}

// New creates a new Fake ready to use.
func New() *Fake {
	return &Fake{
		nextID:     1,
		tasks:      nil,
		pageTokens: make(map[string]int),
	}
}

func (f *Fake) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if !strings.HasPrefix(name, "tasks/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	id, err := strconv.Atoi(strings.TrimPrefix(name, "tasks/"))
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if id >= f.nextID {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	task := f.tasks[id-1]
	if task == nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	return proto.Clone(task).(*pb.Task), nil
}

func (f *Fake) ListTasks(ctx context.Context, req *pb.ListTasksRequest) (*pb.ListTasksResponse, error) {
	pageSize := req.GetPageSize()
	if pageSize < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "The page size must not be negative; was %d.", pageSize)
	}
	if pageSize == 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	minID := 1
	if token := req.GetPageToken(); token != "" {
		var ok bool
		minID, ok = f.pageTokens[token]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", req.GetPageToken())
		}
		delete(f.pageTokens, token)
	}

	// Start adding tasks that we will return.
	res := &pb.ListTasksResponse{}
	for i := minID - 1; i < len(f.tasks) && len(res.GetTasks()) <= int(pageSize); i++ {
		if task := f.tasks[i]; task != nil {
			res.Tasks = append(res.GetTasks(), proto.Clone(task).(*pb.Task))
		}
	}

	// If there is one extra task, use it to create a new page token.
	if len(res.GetTasks()) == int(pageSize)+1 {
		nextTask := res.GetTasks()[len(res.GetTasks())-1]
		res.Tasks = res.GetTasks()[:pageSize]

		nextMinID, err := strconv.Atoi(strings.TrimPrefix(nextTask.GetName(), "tasks/"))
		if err != nil {
			glog.Error(err)
			return nil, internalError
		}
		token := uuid.NewString()
		f.pageTokens[token] = nextMinID
		res.NextPageToken = token
	}
	return res, nil
}

func (f *Fake) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.Task, error) {
	task := req.GetTask()
	if task.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "The task must have a title.")
	}
	if task.GetCompleted() {
		return nil, status.Error(codes.InvalidArgument, "The task must not already be completed.")
	}
	created := proto.Clone(task).(*pb.Task)
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextID
	f.nextID++
	created.Name = "tasks/" + fmt.Sprint(id)
	created.CreateTime = timestamppb.Now()
	f.tasks = append(f.tasks, created)
	return created, nil
}

func (f *Fake) UpdateTask(ctx context.Context, req *pb.UpdateTaskRequest) (*pb.Task, error) {
	// First we do stateless validation, i.e., look for errors that we can find
	// by only looking at the request message.
	patch := req.GetTask()
	name := patch.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if !strings.HasPrefix(name, "tasks/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(name, "tasks/"), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	updateMask := req.GetUpdateMask()
	if updateMask == nil {
		// This is not really necessary, but makes downstream handling easier by
		// not having to be careful about nil derefs.
		updateMask = &fieldmaskpb.FieldMask{}
	}
	// Handle two special cases:
	// 1. The update mask is nil or empty. Then it should be equivalent to
	//    updating all non-empty fields in the patch.
	// 2. The update mask contains a single path that is the wildcard ("*").
	// 	  Then it should be treated as specifying all updatable paths.
	switch paths := updateMask.GetPaths(); {
	case len(paths) == 0:
		if v := patch.GetTitle(); v != "" {
			updateMask.Paths = append(updateMask.GetPaths(), "title")
		}
		if v := patch.GetDescription(); v != "" {
			updateMask.Paths = append(updateMask.GetPaths(), "description")
		}
	case len(paths) == 1 && paths[0] == "*":
		updateMask = proto.Clone(updatableMask).(*fieldmaskpb.FieldMask)
	}
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "completed", "create_time", "name":
			return nil, status.Errorf(codes.InvalidArgument, "The field %q cannot be updated with UpdateTask.")
		case "*":
			// We handled the only valid case of giving a wildcard path above,
			// i.e., when it is the only path.
			return nil, status.Error(codes.InvalidArgument, "A wildcard can only be used if it is the single path in the update mask.")
		}
	}
	if updateMask != nil && !updateMask.IsValid(&pb.Task{}) {
		return nil, status.Error(codes.InvalidArgument, "The given update mask is invalid.")
	}
	// At this point we know that updateMask is not empty and is a valid mask.
	// The path(s) fully specify what we should get from the patch. It may still
	// be the case that the patch is empty.

	f.mu.Lock()
	defer f.mu.Unlock()

	idx := id - 1
	if int(idx) >= len(f.tasks) {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	task := f.tasks[idx]
	if task == nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	task = proto.Clone(task).(*pb.Task)
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "title":
			task.Title = patch.GetTitle()
		case "description":
			task.Description = patch.GetDescription()
		}
	}
	f.tasks[idx] = task
	return task, nil
}

func (f *Fake) DeleteTask(ctx context.Context, req *pb.DeleteTaskRequest) (*emptypb.Empty, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if !strings.HasPrefix(name, "tasks/") {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(name, "tasks/"), 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx := id - 1
	if f.tasks[idx] == nil {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	f.tasks[idx] = nil
	return &emptypb.Empty{}, nil
}
