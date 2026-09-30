package v1_test

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	_ "modernc.org/sqlite"
)

var testDBCounter int64

// setupTestClientShared 创建带完整 Schema 的内存 SQLite 测试客户端。
func setupTestClientShared(t *testing.T) *ent.Client {
	t.Helper()

	name := fmt.Sprintf("file:apitest%d?mode=memory&cache=shared&_fk=1",
		atomic.AddInt64(&testDBCounter, 1))

	db, err := sql.Open("sqlite", name)
	if err != nil {
		t.Fatalf("failed opening sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("failed enabling foreign keys: %v", err)
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { client.Close() })

	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	return client
}
