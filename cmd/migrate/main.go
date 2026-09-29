package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/yourusername/igh-silkroad/internal/database/ent"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
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
			dsn = "postgres://igh:igh_dev_password@localhost:5432/igh?sslmode=disable"
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

	// 执行迁移
	log.Println("\n[1/2] Running schema migration...")
	log.Println("  Note: Old tables (orders, projects) will be kept but disconnected")
	log.Println("  Use manual cleanup if needed: DROP TABLE orders CASCADE; DROP TABLE projects CASCADE;")

	opts := []schema.MigrateOption{
		schema.WithDropIndex(true),
		schema.WithDropColumn(true),
	}

	if *dryRun {
		log.Println("⚠️  DRY RUN MODE is not supported in this version")
		log.Println("   Migration will be executed normally")
	}

	if err := client.Schema.Create(ctx, opts...); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}

	// 打印统计
	if !*dryRun {
		log.Println("\n[2/2] Database statistics:")
		printCenterStats(ctx, client)
	}

	log.Println("\n✅ Center database schema migrated successfully")

	if *dropOld {
		log.Println("\n⚠️  Manual cleanup required:")
		log.Println("   Please run the following SQL manually:")
		log.Println("   DROP TABLE IF EXISTS orders CASCADE;")
		log.Println("   DROP TABLE IF EXISTS projects CASCADE;")
	}
}

func migrateEdge(dsn string) {
	log.Println("=== Edge Database Migration ===")
	log.Printf("DSN: %s", dsn)

	// Edge使用Center的ent包 (SQLite版本)
	// 使用与edge-server相同的方式连接
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("failed opening connection to sqlite: %v", err)
	}

	// 启用外键约束
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Fatalf("failed to enable foreign keys: %v", err)
	}

	// 创建Ent客户端
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))
	defer client.Close()

	ctx := context.Background()

	// 执行迁移
	log.Println("\n[1/2] Running schema migration...")
	log.Println("  Note: Old tables (orders, projects) will be kept but disconnected")
	log.Println("  Use manual cleanup if needed: DROP TABLE orders; DROP TABLE projects;")

	opts := []schema.MigrateOption{
		schema.WithDropIndex(true),
		schema.WithDropColumn(true),
	}

	if *dryRun {
		log.Println("⚠️  DRY RUN MODE is not supported in this version")
		log.Println("   Migration will be executed normally")
	}

	if err := client.Schema.Create(ctx, opts...); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}

	// 打印统计
	if !*dryRun {
		log.Println("\n[2/2] Database statistics:")
		printEdgeStatsNew(ctx, client)
	}

	log.Println("\n✅ Edge database schema migrated successfully")

	if *dropOld {
		log.Println("\n⚠️  Manual cleanup required:")
		log.Println("   Please run the following SQL manually:")
		log.Println("   DROP TABLE IF EXISTS orders;")
		log.Println("   DROP TABLE IF EXISTS projects;")
	}
}

// printEdgeStatsNew 使用ent包打印Edge数据库统计
func printEdgeStatsNew(ctx context.Context, client *ent.Client) {
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

// 删除旧的dropOldTables函数

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
