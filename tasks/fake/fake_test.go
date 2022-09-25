package fake

import (
	"context"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
	"go.saser.se/grpctest"
	pb "go.saser.se/tasks/tasks_go_proto"
	"go.saser.se/tasks/testsuite"
)

// truncater implements testsuite.Truncater to clean out state between tests or
// whenever needed.
type truncater struct {
	s *Fake
}

// Truncate deletes all tasks, deletes all page tokens, and resets the nextID
// counter.
func (t *truncater) Truncate(ctx context.Context) error {
	t.s.mu.Lock()
	defer t.s.mu.Unlock()

	t.s.tasks = []*pb.Task{}
	for k := range t.s.taskPageTokens {
		delete(t.s.taskPageTokens, k)
	}
	for k := range t.s.taskIndices {
		delete(t.s.taskIndices, k)
	}
	t.s.nextTaskID = 1

	t.s.projects = []*pb.Project{}
	for k := range t.s.projectPageTokens {
		delete(t.s.projectPageTokens, k)
	}
	for k := range t.s.projectIndices {
		delete(t.s.projectIndices, k)
	}
	t.s.nextProjectID = 1
	return nil
}

func TestService(t *testing.T) {
	ctx := context.Background()
	svc := New()
	clock := clockwork.NewFakeClock()
	svc.clock = clock
	srv := grpctest.New(ctx, t, grpctest.Options{
		ServiceDesc:    &pb.Tasks_ServiceDesc,
		Implementation: svc,
	})
	client := pb.NewTasksClient(srv.ClientConn)
	s := testsuite.New(client, &truncater{s: svc}, clock, maxPageSize)
	suite.Run(t, s)
}
