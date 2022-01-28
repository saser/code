package fake

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/suite"
	pb "go.saser.se/tasks/tasks_go_proto"
	"go.saser.se/tasks/testsuite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// setup sets up a gRPC server listening to an in-process buffer and serves the
// given Fake on it.
func setup(ctx context.Context, t *testing.T, svc *Fake) pb.TasksClient {
	t.Helper()

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() {
		if err := lis.Close(); err != nil {
			t.Error(err)
		}
	})

	srv := grpc.NewServer()
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

	return pb.NewTasksClient(cc)
}

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
	for k := range t.s.pageTokens {
		delete(t.s.pageTokens, k)
	}
	t.s.nextID = 1
	return nil
}

func TestService(t *testing.T) {
	ctx := context.Background()
	svc := New()
	client := setup(ctx, t, svc)
	s := testsuite.New(client, &truncater{s: svc}, maxPageSize)
	suite.Run(t, s)
}