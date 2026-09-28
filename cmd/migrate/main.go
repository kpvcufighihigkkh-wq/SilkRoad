package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent_edge"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate [center|edge] [postgres_dsn|sqlite_path]")
		fmt.Println("Example:")
		fmt.Println("  migrate center 'postgres://user:pass@localhost/igh?sslmode=disable'")
		fmt.Println("  migrate edge './edge.db'")
		os.Exit(1)
	}

	target := os.Args[1]
	dsn := ""
	if len(os.Args) > 2 {
		dsn = os.Args[2]
	}

	switch target {
	case "center":
		if dsn == "" {
			dsn = "postgres://igh:igh@localhost:5432/igh?sslmode=disable"
		}
		migrateCenter(dsn)
	case "edge":
		if dsn == "" {
			dsn = "file:edge.db?cache=shared&_fk=1"
		}
		migrateEdge(dsn)
	default:
		log.Fatalf("Unknown target: %s", target)
	}
}

func migrateCenter(dsn string) {
	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		log.Fatalf("failed opening connection to postgres: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 运行自动迁移
	if err := client.Schema.Create(
		ctx,
		schema.WithDropIndex(true),
		schema.WithDropColumn(true),
	); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}

	log.Println("✅ Center database schema migrated successfully")
}

func migrateEdge(dsn string) {
	client, err := ent_edge.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("failed opening connection to sqlite: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 运行自动迁移
	if err := client.Schema.Create(
		ctx,
		schema.WithDropIndex(true),
		schema.WithDropColumn(true),
	); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}

	log.Println("✅ Edge database schema migrated successfully")
}
