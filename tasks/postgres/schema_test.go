package schema_test

import (
	"context"
	"testing"

	"go.saser.se/postgres/postgrestest"
)

func TestSchema(t *testing.T) {
	// Open creates a Postgres database and executes the SQL statements in the
	// schema. If the execution fails, Open fails the test.
	postgrestest.Open(context.Background(), t, "tasks/postgres/schema.sql")
}
