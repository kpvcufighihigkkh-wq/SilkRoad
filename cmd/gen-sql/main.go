package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/migrate"
	"github.com/yourusername/igh-silkroad/internal/database/ent_edge"
	migrate_edge "github.com/yourusername/igh-silkroad/internal/database/ent_edge/migrate"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"

	atlas "ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/sqltool"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: gen-sql [center|edge] [output_dir]")
		fmt.Println("Example:")
		fmt.Println("  gen-sql center migrations/center")
		fmt.Println("  gen-sql edge migrations/edge")
		os.Exit(1)
	}

	target := os.Args[1]
	output := os.Args[2]

	switch target {
	case "center":
		generateCenterSQL(output)
	case "edge":
		generateEdgeSQL(output)
	default:
		log.Fatalf("Unknown target: %s", target)
	}
}

func generateCenterSQL(output string) {
	ctx := context.Background()

	// 创建 atlas 目录
	dir, err := atlas.NewLocalDir(output)
	if err != nil {
		log.Fatalf("failed creating atlas local dir: %v", err)
	}

	// 生成迁移文件
	opts := []schema.MigrateOption{
		schema.WithDir(dir),
		schema.WithDialect(dialect.Postgres),
		schema.WithFormatter(sqltool.PostgresFormatter),
	}

	if err := migrate.NamedDiff(ctx, "file://"+output, "init", opts...); err != nil {
		log.Fatalf("failed generating migration file: %v", err)
	}

	log.Printf("✅ Generated center SQL migration in: %s", output)
}

func generateEdgeSQL(output string) {
	ctx := context.Background()

	// 创建 atlas 目录
	dir, err := atlas.NewLocalDir(output)
	if err != nil {
		log.Fatalf("failed creating atlas local dir: %v", err)
	}

	// 生成迁移文件
	opts := []schema.MigrateOption{
		schema.WithDir(dir),
		schema.WithDialect(dialect.SQLite),
		schema.WithFormatter(sqltool.SqliteFormatter),
	}

	if err := migrate_edge.NamedDiff(ctx, "file://"+output, "init", opts...); err != nil {
		log.Fatalf("failed generating migration file: %v", err)
	}

	log.Printf("✅ Generated edge SQL migration in: %s", output)
}
