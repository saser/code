package service

import (
	"context"
	"net"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
	"go.saser.se/postgres"
	"go.saser.se/postgres/postgrestest"
	pb "go.saser.se/tasks/tasks_go_proto"
	"go.saser.se/tasks/testsuite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func setup(ctx context.Context, t *testing.T, pool *postgres.Pool, clock clockwork.FakeClock) pb.TasksClient {
	t.Helper()

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	t.Cleanup(func() {
		if err := lis.Close(); err != nil {
			t.Error(err)
		}
	})

	srv := grpc.NewServer()
	svc := New(pool)
	svc.clock = clock
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

type poolTruncater struct {
	pool *postgres.Pool
}

func (pt *poolTruncater) Truncate(ctx context.Context) error {
	_, err := pt.pool.Exec(ctx, "TRUNCATE TABLE tasks, page_tokens")
	return err
}

func TestService(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.Open(ctx, t, "tasks/postgres/schema.sql")
	clock := clockwork.NewFakeClock()
	client := setup(ctx, t, pool, clock)
	s := testsuite.New(client, &poolTruncater{pool: pool}, clock, maxPageSize)
	suite.Run(t, s)
}
