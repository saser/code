package postgrestest

import (
	"context"
	"strings"
	"testing"
)

const schemaPath = "postgres/postgrestest/schema.sql"

func TestOpen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := Open(ctx, t, schemaPath)
	sql := strings.TrimSpace(`
INSERT INTO tasks (id, title)
VALUES            ($1, $2   )
`)
	if _, err := pool.Exec(ctx, sql,
		1,         // $1
		"A title", // $2
	); err != nil {
		t.Fatal(err)
	}
}
