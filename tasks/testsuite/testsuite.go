package testsuite

import (
	"context"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
	pb "go.saser.se/tasks/tasks_go_proto"
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
