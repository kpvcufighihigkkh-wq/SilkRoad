package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent_edge"

	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

var (
	dropOld = flag.Bool("drop-old", false, "Drop old tables (orders, projects)")
	dryRun  = flag.Bool("dry-run", false, "Print SQL without executing")
)

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: migrate [flags] [center|edge] [postgres_dsn|sqlite_path]")
		fmt.Println("\nFlags:")
		fmt.Println("  -drop-old    Drop old tables (orders, projects)")
		fmt.Println("  -dry-run     Print SQL without executing")
		fmt.Println("\nExamples:")
		fmt.Println("  migrate center 'postgres://user:pass@localhost/igh?sslmode=disable'")
		fmt.Println("  migrate edge './edge.db'")
		fmt.Println("  migrate -drop-old center")
		os.Exit(1)
	}

	target := args[0]
	dsn := ""
	if len(args) > 1 {
		dsn = args[1]
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
	log.Println("=== Center Database Migration ===")
	log.Printf("DSN: %s", maskPassword(dsn))

	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		log.Fatalf("failed opening connection to postgres: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 删除旧表
	if *dropOld {
		log.Println("\n[1/3] Dropping old tables...")
		if err := dropOldTables(ctx, client); err != nil {
			log.Printf("⚠️  Warning: %v", err)
		}
	}

	// 执行迁移
	log.Println("\n[2/3] Running schema migration...")
	opts := []schema.MigrateOption{
		schema.WithDropIndex(true),
		schema.WithDropColumn(true),
	}

	if *dryRun {
		log.Println("DRY RUN MODE: SQL will be printed but not executed")
		opts = append(opts, schema.WithDryRun(true))
	}

	if err := client.Schema.Create(ctx, opts...); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}

	// 打印统计
	if !*dryRun {
		log.Println("\n[3/3] Database statistics:")
		printCenterStats(ctx, client)
	}

	log.Println("\n✅ Center database schema migrated successfully")
}

func migrateEdge(dsn string) {
	log.Println("=== Edge Database Migration ===")
	log.Printf("DSN: %s", dsn)

	client, err := ent_edge.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("failed opening connection to sqlite: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 删除旧表
	if *dropOld {
		log.Println("\n[1/3] Dropping old tables...")
		if err := dropOldTablesEdge(ctx, client); err != nil {
			log.Printf("⚠️  Warning: %v", err)
		}
	}

	// 执行迁移
	log.Println("\n[2/3] Running schema migration...")
	opts := []schema.MigrateOption{
		schema.WithDropIndex(true),
		schema.WithDropColumn(true),
	}

	if *dryRun {
		log.Println("DRY RUN MODE: SQL will be printed but not executed")
		opts = append(opts, schema.WithDryRun(true))
	}

	if err := client.Schema.Create(ctx, opts...); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}

	// 打印统计
	if !*dryRun {
		log.Println("\n[3/3] Database statistics:")
		printEdgeStats(ctx, client)
	}

	log.Println("\n✅ Edge database schema migrated successfully")
}

// dropOldTables 删除Center数据库的旧表
func dropOldTables(ctx context.Context, client *ent.Client) error {
	driver := client.Driver()

	// 删除orders表
	if _, err := driver.Exec(ctx, "DROP TABLE IF EXISTS orders CASCADE", nil, nil); err != nil {
		return fmt.Errorf("drop orders table: %w", err)
	}
	log.Println("  ✓ Dropped table: orders")

	// 删除projects表
	if _, err := driver.Exec(ctx, "DROP TABLE IF EXISTS projects CASCADE", nil, nil); err != nil {
		return fmt.Errorf("drop projects table: %w", err)
	}
	log.Println("  ✓ Dropped table: projects")

	return nil
}

// dropOldTablesEdge 删除Edge数据库的旧表
func dropOldTablesEdge(ctx context.Context, client *ent_edge.Client) error {
	driver := client.Driver()

	// SQLite不支持CASCADE，直接删除
	if _, err := driver.Exec(ctx, "DROP TABLE IF EXISTS orders", nil, nil); err != nil {
		return fmt.Errorf("drop orders table: %w", err)
	}
	log.Println("  ✓ Dropped table: orders")

	if _, err := driver.Exec(ctx, "DROP TABLE IF EXISTS projects", nil, nil); err != nil {
		return fmt.Errorf("drop projects table: %w", err)
	}
	log.Println("  ✓ Dropped table: projects")

	return nil
}

// printCenterStats 打印Center数据库统计
func printCenterStats(ctx context.Context, client *ent.Client) {
	tables := []struct {
		name  string
		count func() (int, error)
	}{
		{"Edges", func() (int, error) { return client.Edge.Query().Count(ctx) }},
		{"SpinningLines", func() (int, error) { return client.SpinningLine.Query().Count(ctx) }},
		{"Lots", func() (int, error) { return client.Lot.Query().Count(ctx) }},
		{"Doffings", func() (int, error) { return client.Doffing.Query().Count(ctx) }},
		{"Barrels", func() (int, error) { return client.Barrel.Query().Count(ctx) }},
		{"Bobbins", func() (int, error) { return client.Bobbin.Query().Count(ctx) }},
		{"Pallets", func() (int, error) { return client.Pallet.Query().Count(ctx) }},
		{"Modules", func() (int, error) { return client.Module.Query().Count(ctx) }},
		{"Grades", func() (int, error) { return client.Grade.Query().Count(ctx) }},
		{"Users", func() (int, error) { return client.User.Query().Count(ctx) }},
	}

	for _, table := range tables {
		count, err := table.count()
		if err != nil {
			log.Printf("  %-15s: Error - %v", table.name, err)
		} else {
			log.Printf("  %-15s: %d records", table.name, count)
		}
	}
}

// printEdgeStats 打印Edge数据库统计
func printEdgeStats(ctx context.Context, client *ent_edge.Client) {
	tables := []struct {
		name  string
		count func() (int, error)
	}{
		{"Lots", func() (int, error) { return client.Lot.Query().Count(ctx) }},
		{"Doffings", func() (int, error) { return client.Doffing.Query().Count(ctx) }},
		{"Bobbins", func() (int, error) { return client.Bobbin.Query().Count(ctx) }},
		{"Users", func() (int, error) { return client.User.Query().Count(ctx) }},
	}

	for _, table := range tables {
		count, err := table.count()
		if err != nil {
			log.Printf("  %-15s: Error - %v", table.name, err)
		} else {
			log.Printf("  %-15s: %d records", table.name, count)
		}
	}
}

// maskPassword 隐藏DSN中的密码
func maskPassword(dsn string) string {
	if strings.Contains(dsn, "@") {
		parts := strings.Split(dsn, "@")
		if len(parts) == 2 {
			userInfo := strings.Split(parts[0], ":")
			if len(userInfo) == 2 {
				return userInfo[0] + ":****@" + parts[1]
			}
		}
	}
	return dsn
}
