// Package fake provides an in-memory implementation of the Tasks service. It is
// intended to be used for integration tests or other places where the full
// SQL-backed implementation isn't appropriate.
package fake

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"
	pb "go.saser.se/tasks/tasks_go_proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/klog/v2"
)

// maxPageSize is the maximum number of tasks the server will return on a call
// to ListTasks. Any request for more than maxPageSize tasks will only return
// (at most) maxPageSize tasks.
const maxPageSize = 1000

// labelRE matches all valid label strings. It is based on the proto definition of
// what valid characters are.
var labelRE = regexp.MustCompile(`^[a-zA-Z0-9\:\-\_\@]+$`)

// taskUpdatableMask contains the fields that can be updated by UpdateTask. It must
// be kept in sync with the proto definition.
var taskUpdatableMask *fieldmaskpb.FieldMask

func init() {
	m, err := fieldmaskpb.New(&pb.Task{},
		"title",
		"description",
	)
	if err != nil {
		klog.Exit(err)
	}
	taskUpdatableMask = m
}

// projectUpdatableMask contains the fields that can be updated by UpdateProject. It must
// be kept in sync with the proto definition.
var projectUpdatableMask *fieldmaskpb.FieldMask

func init() {
	m, err := fieldmaskpb.New(&pb.Project{},
		"title",
		"description",
	)
	if err != nil {
		klog.Exit(err)
	}
	projectUpdatableMask = m
}

// labelUpdatableMask contains the fields that can be updated by UpdateLabel. It must
// be kept in sync with the proto definition.
var labelUpdatableMask *fieldmaskpb.FieldMask

func init() {
	m, err := fieldmaskpb.New(&pb.Label{},
		"label",
	)
	if err != nil {
		klog.Exit(err)
	}
	labelUpdatableMask = m
}

type pageToken struct {
	MinimumIndex int
	ShowDeleted  bool
}

// Fake implements the Tasks service using only in-memory data structures.
type Fake struct {
	pb.UnimplementedTasksServer

	mu             sync.Mutex
	nextTaskID     int
	tasks          []*pb.Task
	taskIndices    map[string]int       // task name -> index in `tasks`
	taskPageTokens map[string]pageToken // token (UUID) -> minimum task ID and whether to show deleted

	nextProjectID     int
	projects          []*pb.Project
	projectIndices    map[string]int       // project name -> index in `projects`
	projectPageTokens map[string]pageToken // token (UUID) -> minimum project ID and whether to show deleted

	nextLabelID     int
	labels          []*pb.Label
	labelIndices    map[string]int       // label name -> index in `labels`
	labelPageTokens map[string]pageToken // token (UUID) -> minimum label ID and whether to show deleted

	// Only used in testing. Nil otherwise.
	clock clockwork.FakeClock
}

// New creates a new Fake ready to use.
func New() *Fake {
	return &Fake{
		nextTaskID:     1,
		tasks:          nil,
		taskIndices:    make(map[string]int),
		taskPageTokens: make(map[string]pageToken),

		nextProjectID:     1,
		projects:          nil,
		projectIndices:    make(map[string]int),
		projectPageTokens: make(map[string]pageToken),

		nextLabelID:     1,
		labels:          nil,
		labelIndices:    make(map[string]int),
		labelPageTokens: make(map[string]pageToken),
	}
}

// validateTaskName returns an error if name isn't a valid task name.
func validateTaskName(name string) error {
	const prefix = "tasks/"
	if !strings.HasPrefix(name, prefix) {
		return &invalidNameError{
			Name:   name,
			Reason: fmt.Sprintf("name doesn't have prefix %q", prefix),
		}
	}
	if id := strings.TrimPrefix(name, prefix); id == "" {
		return &invalidNameError{
			Name:   name,
			Reason: fmt.Sprintf("name doesn't have a resource ID after %q", prefix),
		}
	}
	return nil
}

// validateProjectName returns an error if name isn't a valid task name.
func validateProjectName(name string) error {
	const prefix = "projects/"
	if !strings.HasPrefix(name, prefix) {
		return &invalidNameError{
			Name:   name,
			Reason: fmt.Sprintf("name doesn't have prefix %q", prefix),
		}
	}
	if id := strings.TrimPrefix(name, prefix); id == "" {
		return &invalidNameError{
			Name:   name,
			Reason: fmt.Sprintf("name doesn't have a resource ID after %q", prefix),
		}
	}
	return nil
}

// validateLabelName returns an error if name isn't a valid task name.
func validateLabelName(name string) error {
	const prefix = "labels/"
	if !strings.HasPrefix(name, prefix) {
		return &invalidNameError{
			Name:   name,
			Reason: fmt.Sprintf("name doesn't have prefix %q", prefix),
		}
	}
	if id := strings.TrimPrefix(name, prefix); id == "" {
		return &invalidNameError{
			Name:   name,
			Reason: fmt.Sprintf("name doesn't have a resource ID after %q", prefix),
		}
	}
	return nil
}

func (f *Fake) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if err := validateTaskName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.taskIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	task := f.tasks[idx]
	if expire := task.GetExpireTime(); expire.IsValid() && f.now().After(expire.AsTime()) {
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

	minIndex := 0
	if token := req.GetPageToken(); token != "" {
		pt, ok := f.taskPageTokens[token]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", req.GetPageToken())
		}
		if req.GetShowDeleted() != pt.ShowDeleted {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", req.GetPageToken())
		}
		minIndex = pt.MinimumIndex
		delete(f.taskPageTokens, token)
	}

	// Start adding tasks that we will return.
	res := &pb.ListTasksResponse{}
	for idx := minIndex; idx < len(f.tasks) && len(res.GetTasks()) <= int(pageSize); idx++ {
		task := f.tasks[idx]
		if expire := task.GetExpireTime(); expire.IsValid() && f.now().After(expire.AsTime()) {
			continue
		}
		if task.GetDeleteTime().IsValid() && !req.GetShowDeleted() {
			continue
		}
		res.Tasks = append(res.GetTasks(), proto.Clone(task).(*pb.Task))
	}

	// If there is one extra task, use it to create a new page token.
	if len(res.GetTasks()) == int(pageSize)+1 {
		nextTask := res.GetTasks()[len(res.GetTasks())-1]
		res.Tasks = res.GetTasks()[:pageSize]

		nextMinIndex := f.taskIndices[nextTask.GetName()]
		token := uuid.NewString()
		f.taskPageTokens[token] = pageToken{
			MinimumIndex: nextMinIndex,
			ShowDeleted:  req.GetShowDeleted(),
		}
		res.NextPageToken = token
	}
	return res, nil
}

func (f *Fake) CreateTask(ctx context.Context, req *pb.CreateTaskRequest) (*pb.Task, error) {
	task := req.GetTask()
	if task.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "The task must have a title.")
	}
	if task.GetCompleteTime().IsValid() {
		return nil, status.Error(codes.InvalidArgument, "The task must not already be completed.")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if parent := task.GetParent(); parent != "" {
		if err := validateTaskName(parent); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, `The name of the parent must follow the format "tasks/{task}", but it was %q.`, parent)
		}
		if _, ok := f.taskIndices[parent]; !ok {
			return nil, status.Errorf(codes.NotFound, "A parent task with name %q does not exist.", parent)
		}
	}

	created := proto.Clone(task).(*pb.Task)
	id := f.nextTaskID
	f.nextTaskID++
	created.Name = "tasks/" + fmt.Sprint(id)
	created.CreateTime = timestamppb.New(f.now())
	f.tasks = append(f.tasks, created)
	f.taskIndices[created.Name] = len(f.tasks) - 1
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
	if err := validateTaskName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
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
		updateMask = proto.Clone(taskUpdatableMask).(*fieldmaskpb.FieldMask)
	}
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "parent", "completed", "create_time", "name":
			return nil, status.Errorf(codes.InvalidArgument, "The field %q cannot be updated with UpdateTask.", path)
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

	idx, ok := f.taskIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	task := f.tasks[idx]
	if task.GetDeleteTime().IsValid() {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	updated := proto.Clone(task).(*pb.Task)
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "title":
			updated.Title = patch.GetTitle()
		case "description":
			updated.Description = patch.GetDescription()
		}
	}
	if !proto.Equal(task, updated) {
		updated.UpdateTime = timestamppb.New(f.now())
	}
	f.tasks[idx] = updated
	return updated, nil
}

func (f *Fake) DeleteTask(ctx context.Context, req *pb.DeleteTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if err := validateTaskName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.taskIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	root := f.tasks[idx]
	if root.GetDeleteTime().IsValid() {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	descendantIndices := f.descendantIndices(name)
	if len(descendantIndices) > 0 && !req.GetForce() {
		return nil, status.Errorf(codes.FailedPrecondition, "Task %q has children; not deleting without `force: true`.", name)
	}
	now := f.now()
	toDeleteIndices := append([]int{idx}, descendantIndices...)
	for _, i := range toDeleteIndices {
		deleted := f.tasks[i]
		// If one of the descendants has already been deleted earlier, skip over
		// it.
		if dt := deleted.GetDeleteTime(); dt.IsValid() && !dt.AsTime().IsZero() {
			continue
		}
		deleted.DeleteTime = timestamppb.New(now)
		deleted.ExpireTime = timestamppb.New(now.AddDate(0 /*years*/, 0 /*months*/, 30 /*days*/))
	}
	return proto.Clone(root).(*pb.Task), nil
}

func (f *Fake) UndeleteTask(ctx context.Context, req *pb.UndeleteTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if err := validateTaskName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.taskIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	if !f.tasks[idx].GetDeleteTime().IsValid() {
		return nil, status.Errorf(codes.AlreadyExists, "A task with name %q already exists.", name)
	}
	var toUndeleteIndices []int
	for _, ancestorIndex := range f.ancestorIndices(name) {
		if f.tasks[ancestorIndex].GetDeleteTime().IsValid() {
			toUndeleteIndices = append(toUndeleteIndices, ancestorIndex)
		}
	}
	if len(toUndeleteIndices) > 0 && !req.GetUndeleteAncestors() {
		return nil, status.Errorf(codes.FailedPrecondition, "Task %q has deleted ancestors but `undelete_ancestors` was not set to `true`.", name)
	}
	if req.GetUndeleteDescendants() {
		toUndeleteIndices = append(toUndeleteIndices, f.descendantIndices(name)...)
	}
	toUndeleteIndices = append(toUndeleteIndices, idx)
	for _, i := range toUndeleteIndices {
		task := f.tasks[i]
		task.DeleteTime = nil
		task.ExpireTime = nil
	}
	return proto.Clone(f.tasks[idx]).(*pb.Task), nil
}

func (f *Fake) CompleteTask(ctx context.Context, req *pb.CompleteTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if err := validateTaskName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.taskIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	task := f.tasks[idx]
	if task.GetDeleteTime().IsValid() {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	// Special case: a completed task can be completed again, which is a no-op.
	if task.GetCompleteTime().IsValid() {
		return proto.Clone(task).(*pb.Task), nil
	}
	var toCompleteIndices []int
	for _, idx := range f.descendantIndices(name) {
		if f.tasks[idx].GetCompleteTime().IsValid() {
			continue
		}
		toCompleteIndices = append(toCompleteIndices, idx)
	}
	if len(toCompleteIndices) > 0 && !req.GetForce() {
		return nil, status.Errorf(codes.FailedPrecondition, "Task %q has uncompleted children but `force` was not set to true.", name)
	}
	toCompleteIndices = append(toCompleteIndices, idx)
	now := f.now()
	for _, idx := range toCompleteIndices {
		completed := f.tasks[idx]
		completed.CompleteTime = timestamppb.New(now)
		completed.UpdateTime = timestamppb.New(now)
	}
	return proto.Clone(task).(*pb.Task), nil
}

func (f *Fake) UncompleteTask(ctx context.Context, req *pb.UncompleteTaskRequest) (*pb.Task, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the task is required.")
	}
	if err := validateTaskName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the task must have format "tasks/{task}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.taskIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	task := f.tasks[idx]
	if task.GetDeleteTime().IsValid() {
		return nil, status.Errorf(codes.NotFound, "A task with name %q does not exist.", name)
	}
	// Special case: uncompleting an uncompleted task is a no-op.
	if !task.GetCompleteTime().IsValid() {
		return proto.Clone(task).(*pb.Task), nil
	}
	var toUncompleteIndices []int
	for _, idx := range f.ancestorIndices(name) {
		if !f.tasks[idx].GetCompleteTime().IsValid() {
			continue
		}
		toUncompleteIndices = append(toUncompleteIndices, idx)
	}
	if len(toUncompleteIndices) > 0 && !req.GetUncompleteAncestors() {
		return nil, status.Errorf(codes.FailedPrecondition, "Task %q has completed ancestors but `uncomplete_ancestors` was not set to true.", name)
	}
	if req.GetUncompleteDescendants() {
		toUncompleteIndices = append(toUncompleteIndices, f.descendantIndices(name)...)
	}
	toUncompleteIndices = append(toUncompleteIndices, idx)
	now := f.now()
	for _, idx := range toUncompleteIndices {
		uncompleted := f.tasks[idx]
		uncompleted.CompleteTime = nil
		uncompleted.UpdateTime = timestamppb.New(now)
	}
	return proto.Clone(task).(*pb.Task), nil
}

func (f *Fake) GetProject(ctx context.Context, req *pb.GetProjectRequest) (*pb.Project, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if err := validateProjectName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.projectIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	project := f.projects[idx]
	if expire := project.GetExpireTime(); expire.IsValid() && f.now().After(expire.AsTime()) {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	return proto.Clone(project).(*pb.Project), nil
}

func (f *Fake) ListProjects(ctx context.Context, req *pb.ListProjectsRequest) (*pb.ListProjectsResponse, error) {
	pageSize := req.GetPageSize()
	if pageSize < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "The page size must not be negative; was %d.", pageSize)
	}
	if pageSize == 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	minIndex := 0
	if token := req.GetPageToken(); token != "" {
		pt, ok := f.projectPageTokens[token]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", token)
		}
		if req.GetShowDeleted() != pt.ShowDeleted {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", token)
		}
		minIndex = pt.MinimumIndex
		delete(f.projectPageTokens, token)
	}

	// Start adding projects that we will return.
	res := &pb.ListProjectsResponse{}
	for idx := minIndex; idx < len(f.projects) && len(res.GetProjects()) <= int(pageSize); idx++ {
		project := f.projects[idx]
		if expire := project.GetExpireTime(); expire.IsValid() && f.now().After(expire.AsTime()) {
			continue
		}
		if project.GetDeleteTime().IsValid() && !req.GetShowDeleted() {
			continue
		}
		res.Projects = append(res.GetProjects(), proto.Clone(project).(*pb.Project))
	}

	// If there is one extra project, use it to create a new page token.
	if len(res.GetProjects()) == int(pageSize)+1 {
		nextProject := res.GetProjects()[len(res.GetProjects())-1]
		res.Projects = res.GetProjects()[:pageSize]

		nextMinIndex := f.projectIndices[nextProject.GetName()]
		token := uuid.NewString()
		f.projectPageTokens[token] = pageToken{
			MinimumIndex: nextMinIndex,
			ShowDeleted:  req.GetShowDeleted(),
		}
		res.NextPageToken = token
	}
	return res, nil
}

func (f *Fake) CreateProject(ctx context.Context, req *pb.CreateProjectRequest) (*pb.Project, error) {
	project := req.GetProject()
	if project.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "The project must have a title.")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	created := proto.Clone(project).(*pb.Project)
	id := f.nextProjectID
	f.nextProjectID++
	created.Name = "projects/" + fmt.Sprint(id)
	created.CreateTime = timestamppb.New(f.now())
	f.projects = append(f.projects, created)
	f.projectIndices[created.Name] = len(f.projects) - 1
	return created, nil
}

func (f *Fake) UpdateProject(ctx context.Context, req *pb.UpdateProjectRequest) (*pb.Project, error) {
	// First we do stateless validation, i.e., look for errors that we can find
	// by only looking at the request message.
	patch := req.GetProject()
	name := patch.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if err := validateProjectName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
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
		updateMask = proto.Clone(projectUpdatableMask).(*fieldmaskpb.FieldMask)
	}
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "parent", "completed", "create_time", "name":
			return nil, status.Errorf(codes.InvalidArgument, "The field %q cannot be updated with UpdateProject.", path)
		case "*":
			// We handled the only valid case of giving a wildcard path above,
			// i.e., when it is the only path.
			return nil, status.Error(codes.InvalidArgument, "A wildcard can only be used if it is the single path in the update mask.")
		}
	}
	if updateMask != nil && !updateMask.IsValid(&pb.Project{}) {
		return nil, status.Error(codes.InvalidArgument, "The given update mask is invalid.")
	}
	// At this point we know that updateMask is not empty and is a valid mask.
	// The path(s) fully specify what we should get from the patch. It may still
	// be the case that the patch is empty.

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.projectIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	project := f.projects[idx]
	if project.GetDeleteTime().IsValid() {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	updated := proto.Clone(project).(*pb.Project)
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "title":
			updated.Title = patch.GetTitle()
		case "description":
			updated.Description = patch.GetDescription()
		}
	}
	if !proto.Equal(project, updated) {
		updated.UpdateTime = timestamppb.New(f.now())
	}
	f.projects[idx] = updated
	return updated, nil
}

func (f *Fake) DeleteProject(ctx context.Context, req *pb.DeleteProjectRequest) (*pb.Project, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if err := validateProjectName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.projectIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	deleted := f.projects[idx]
	if deleted.GetDeleteTime().IsValid() {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	now := f.now()
	deleted.DeleteTime = timestamppb.New(now)
	deleted.ExpireTime = timestamppb.New(now.AddDate(0 /*years*/, 0 /*months*/, 30 /*days*/))
	return proto.Clone(deleted).(*pb.Project), nil
}

func (f *Fake) UndeleteProject(ctx context.Context, req *pb.UndeleteProjectRequest) (*pb.Project, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if err := validateProjectName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.projectIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	undeleted := f.projects[idx]
	if !undeleted.GetDeleteTime().IsValid() {
		return nil, status.Errorf(codes.AlreadyExists, "A project with name %q already exists.", name)
	}
	undeleted.DeleteTime = nil
	undeleted.ExpireTime = nil
	return proto.Clone(undeleted).(*pb.Project), nil
}

func (f *Fake) ArchiveProject(ctx context.Context, req *pb.ArchiveProjectRequest) (*pb.Project, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if err := validateProjectName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.projectIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	project := f.projects[idx]
	if project.GetDeleteTime().IsValid() {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	// Special case: an archived project can be archived again, which is a no-op.
	if project.GetArchiveTime().IsValid() {
		return proto.Clone(project).(*pb.Project), nil
	}
	now := f.now()
	project.ArchiveTime = timestamppb.New(now)
	project.UpdateTime = timestamppb.New(now)
	return proto.Clone(project).(*pb.Project), nil
}

func (f *Fake) UnarchiveProject(ctx context.Context, req *pb.UnarchiveProjectRequest) (*pb.Project, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the project is required.")
	}
	if err := validateProjectName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the project must have format "projects/{project}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.projectIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	project := f.projects[idx]
	if project.GetDeleteTime().IsValid() {
		return nil, status.Errorf(codes.NotFound, "A project with name %q does not exist.", name)
	}
	// Special case: uncompleting an unarchived project is a no-op.
	if !project.GetArchiveTime().IsValid() {
		return proto.Clone(project).(*pb.Project), nil
	}
	now := f.now()
	project.ArchiveTime = nil
	project.UpdateTime = timestamppb.New(now)
	return proto.Clone(project).(*pb.Project), nil
}

func (f *Fake) GetLabel(ctx context.Context, req *pb.GetLabelRequest) (*pb.Label, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the label is required.")
	}
	if err := validateLabelName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the label must have format "labels/{label}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.labelIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A label with name %q does not exist.", name)
	}
	label := f.labels[idx]
	if label == nil {
		return nil, status.Errorf(codes.NotFound, "A label with name %q does not exist.", name)
	}
	return proto.Clone(label).(*pb.Label), nil
}

func (f *Fake) ListLabels(ctx context.Context, req *pb.ListLabelsRequest) (*pb.ListLabelsResponse, error) {
	pageSize := req.GetPageSize()
	if pageSize < 0 {
		return nil, status.Errorf(codes.InvalidArgument, "The page size must not be negative; was %d.", pageSize)
	}
	if pageSize == 0 || pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	minIndex := 0
	if token := req.GetPageToken(); token != "" {
		pt, ok := f.labelPageTokens[token]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "The page token %q is invalid.", token)
		}
		minIndex = pt.MinimumIndex
		delete(f.labelPageTokens, token)
	}

	// Start adding labels that we will return.
	res := &pb.ListLabelsResponse{}
	for idx := minIndex; idx < len(f.labels) && len(res.GetLabels()) <= int(pageSize); idx++ {
		label := f.labels[idx]
		if label == nil {
			continue
		}
		res.Labels = append(res.GetLabels(), proto.Clone(label).(*pb.Label))
	}

	// If there is one extra label, use it to create a new page token.
	if len(res.GetLabels()) == int(pageSize)+1 {
		nextLabel := res.GetLabels()[len(res.GetLabels())-1]
		res.Labels = res.GetLabels()[:pageSize]

		nextMinIndex := f.labelIndices[nextLabel.GetName()]
		token := uuid.NewString()
		f.labelPageTokens[token] = pageToken{
			MinimumIndex: nextMinIndex,
		}
		res.NextPageToken = token
	}
	return res, nil
}

func (f *Fake) CreateLabel(ctx context.Context, req *pb.CreateLabelRequest) (*pb.Label, error) {
	label := req.GetLabel()
	labelString := label.GetLabel()
	if labelString == "" {
		return nil, status.Error(codes.InvalidArgument, "The label string cannot be empty.")
	}
	if !labelRE.MatchString(labelString) {
		return nil, status.Errorf(codes.InvalidArgument, "Label string %q contains invalid characters.", labelString)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	for _, existing := range f.labels {
		s := label.GetLabel()
		if s == existing.GetLabel() {
			return nil, status.Errorf(codes.AlreadyExists, "The label %q already exists as %q.", s, existing.GetName())
		}
	}

	created := proto.Clone(label).(*pb.Label)
	id := f.nextLabelID
	f.nextLabelID++
	created.Name = "labels/" + fmt.Sprint(id)
	created.CreateTime = timestamppb.New(f.now())
	f.labels = append(f.labels, created)
	f.labelIndices[created.Name] = len(f.labels) - 1
	return created, nil
}

func (f *Fake) UpdateLabel(ctx context.Context, req *pb.UpdateLabelRequest) (*pb.Label, error) {
	// First we do stateless validation, i.e., look for errors that we can find
	// by only looking at the request message.
	patch := req.GetLabel()
	name := patch.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the label is required.")
	}
	if err := validateLabelName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the label must have format "labels/{label}", but it was %q.`, name)
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
		if v := patch.GetLabel(); v != "" {
			updateMask.Paths = append(updateMask.GetPaths(), "label")
		}
	case len(paths) == 1 && paths[0] == "*":
		updateMask = proto.Clone(labelUpdatableMask).(*fieldmaskpb.FieldMask)
	}
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "name", "create_time", "update_time":
			return nil, status.Errorf(codes.InvalidArgument, "The field %q cannot be updated with UpdateLabel.", path)
		case "*":
			// We handled the only valid case of giving a wildcard path above,
			// i.e., when it is the only path.
			return nil, status.Error(codes.InvalidArgument, "A wildcard can only be used if it is the single path in the update mask.")
		}
	}
	if updateMask != nil && !updateMask.IsValid(&pb.Label{}) {
		return nil, status.Error(codes.InvalidArgument, "The given update mask is invalid.")
	}
	// At this point we know that updateMask is not empty and is a valid mask.
	// The path(s) fully specify what we should get from the patch. It may still
	// be the case that the patch is empty.

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.labelIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A label with name %q does not exist.", name)
	}
	label := f.labels[idx]
	if label == nil {
		return nil, status.Errorf(codes.NotFound, "A label with name %q does not exist.", name)
	}
	updated := proto.Clone(label).(*pb.Label)
	for _, path := range updateMask.GetPaths() {
		switch path {
		case "label":
			// If the new label string already exists, the update should fail.
			newLabel := patch.GetLabel()
			for _, existing := range f.labels {
				// We will eventually let be iterating over the label we're
				// trying to update, and in that case setting an identical label
				// string is okay (it's a no-op update).
				if existing.GetName() == patch.GetName() {
					continue
				}
				if newLabel == existing.GetLabel() {
					return nil, status.Errorf(codes.AlreadyExists, "The label %q already exists as %q.", patch.GetLabel(), existing.GetName())
				}
			}
			updated.Label = newLabel
		}
	}
	if !proto.Equal(label, updated) {
		updated.UpdateTime = timestamppb.New(f.now())
	}
	f.labels[idx] = updated
	return updated, nil
}

func (f *Fake) DeleteLabel(ctx context.Context, req *pb.DeleteLabelRequest) (*emptypb.Empty, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "The name of the label is required.")
	}
	if err := validateLabelName(name); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, `The name of the label must have format "labels/{label}", but it was %q.`, name)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx, ok := f.labelIndices[name]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "A label with name %q does not exist.", name)
	}
	if f.labels[idx] == nil {
		return nil, status.Errorf(codes.NotFound, "A label with name %q does not exist.", name)
	}
	f.labels[idx] = nil
	return &emptypb.Empty{}, nil
}

// now returns time.Now() except if f.clock is non-nil, then that clock is used
// instead. now assumes that the mutex is held when called.
func (f *Fake) now() time.Time {
	if f.clock != nil {
		return f.clock.Now()
	}
	return time.Now()
}

// childIndices returns indices into f.tasks for all tasks that are direct
// children to the task named parent. Note that this does not include parent
// itself, nor any transitive children.
func (f *Fake) childIndices(parent string) []int {
	var indices []int
	for i, task := range f.tasks {
		if task.GetParent() == parent {
			indices = append(indices, i)
		}
	}
	return indices
}

// descendantIndices returns indices into f.tasks for all tasks that are either
// direct or transitive descendants of the task named parent. Note that this
// does not include parent itself.
func (f *Fake) descendantIndices(parent string) []int {
	indices := f.childIndices(parent)
	for _, i := range indices {
		indices = append(indices, f.childIndices(f.tasks[i].GetName())...)
	}
	return indices
}

// ancestorIndices returns indices into f.tasks for all tasks that are either a
// direct or transitive ancestors of the task named child. Note that this does
// not include child itself.
func (f *Fake) ancestorIndices(child string) []int {
	var indices []int
	current := child
	for current != "" {
		parent := f.tasks[f.taskIndices[current]].GetParent()
		if parent != "" {
			indices = append(indices, f.taskIndices[parent])
			current = parent
		} else {
			break
		}
	}
	return indices
}
