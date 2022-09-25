package service

import (
	"context"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/suite"
	"go.saser.se/grpctest"
	"go.saser.se/postgres"
	"go.saser.se/postgres/postgrestest"
	pb "go.saser.se/tasks/tasks_go_proto"
	"go.saser.se/tasks/testsuite"
)

type poolTruncater struct {
	pool *postgres.Pool
}

func (pt *poolTruncater) Truncate(ctx context.Context) error {
	_, err := pt.pool.Exec(ctx, "TRUNCATE TABLE tasks, task_page_tokens, projects, project_page_tokens")
	return err
}

func TestService(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.Open(ctx, t, "tasks/postgres/schema.sql")
	svc := New(pool)
	clock := clockwork.NewFakeClock()
	svc.clock = clock
	srv := grpctest.New(ctx, t, grpctest.Options{
		ServiceDesc:    &pb.Tasks_ServiceDesc,
		Implementation: svc,
	})
	client := pb.NewTasksClient(srv.ClientConn)
	s := testsuite.New(client, &poolTruncater{pool: pool}, clock, maxPageSize)
	suite.Run(t, s)
}
