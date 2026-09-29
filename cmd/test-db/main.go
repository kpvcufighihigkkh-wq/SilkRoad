package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/yourusername/igh-silkroad/internal/database/ent"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: test-db [center|edge]")
		fmt.Println("Example:")
		fmt.Println("  test-db center")
		fmt.Println("  test-db edge")
		os.Exit(1)
	}

	target := os.Args[1]

	switch target {
	case "center":
		testCenter()
	case "edge":
		testEdge()
	default:
		log.Fatalf("Unknown target: %s", target)
	}
}

func testCenter() {
	log.Println("🧪 Testing center database (PostgreSQL)...")

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://igh:igh_dev_password@localhost:5432/igh?sslmode=disable"
	}

	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		log.Fatalf("❌ Failed connecting to postgres: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 测试连接
	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("❌ Failed creating schema: %v", err)
	}
	log.Println("✅ Database connection OK")

	log.Println("\n🎉 All center database tests passed!")
}

func testEdge() {
	log.Println("🧪 Testing edge database (SQLite)...")

	dsn := "file:test_edge.db?cache=shared&_fk=1"

	// 使用modernc.org/sqlite驱动
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("❌ Failed opening sqlite: %v", err)
	}

	// 使用ent的SQL包装器
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))
	defer client.Close()
	defer os.Remove("test_edge.db") // 清理测试文件

	ctx := context.Background()

	// 测试连接
	if err := client.Schema.Create(ctx); err != nil {
		log.Fatalf("❌ Failed creating schema: %v", err)
	}
	log.Println("✅ Database connection OK")

	// 测试 CRUD - Lot
	log.Println("\n📝 Testing Lot CRUD...")

	// Create
	lot, err := client.Lot.Create().
		SetLotNumber("LOT-TEST-001").
		SetProductType("FDY").
		SetProductSpec("150D/48F").
		SetPlannedQuantity(100).
		SetStatus("in_progress").
		Save(ctx)
	if err != nil {
		log.Fatalf("❌ Failed creating lot: %v", err)
	}
	log.Printf("✅ Created lot: %s (ID: %s)", lot.LotNumber, lot.ID)

	// Read
	found, err := client.Lot.Get(ctx, lot.ID)
	if err != nil {
		log.Fatalf("❌ Failed reading lot: %v", err)
	}
	log.Printf("✅ Read lot: %s", found.LotNumber)

	// Update
	updated, err := client.Lot.UpdateOne(lot).
		SetActualQuantity(50).
		Save(ctx)
	if err != nil {
		log.Fatalf("❌ Failed updating lot: %v", err)
	}
	log.Printf("✅ Updated lot actual quantity: %d", updated.ActualQuantity)

	// Query
	count, err := client.Lot.Query().
		Where().
		Count(ctx)
	if err != nil {
		log.Fatalf("❌ Failed counting lots: %v", err)
	}
	log.Printf("✅ Total lots: %d", count)

	// Test Bobbin relationship
	log.Println("\n📝 Testing Bobbin with relationship...")
	bobbin, err := client.Bobbin.Create().
		SetBobbinNumber("BOB-TEST-001").
		SetLotID(lot.ID).
		SetSpinningPosition(1).
		SetGrossWeight(5.5).
		SetNetWeight(5.3).
		SetStatus("producing").
		Save(ctx)
	if err != nil {
		log.Fatalf("❌ Failed creating bobbin: %v", err)
	}
	log.Printf("✅ Created bobbin: %s", bobbin.BobbinNumber)

	// Query with relationship
	bobbins, err := client.Bobbin.Query().
		Where().
		All(ctx)
	if err != nil {
		log.Fatalf("❌ Failed querying bobbins: %v", err)
	}
	log.Printf("✅ Total bobbins for lot: %d", len(bobbins))

	// Delete (先删除子实体bobbin，再删除父实体lot)
	if err := client.Bobbin.DeleteOne(bobbin).Exec(ctx); err != nil {
		log.Fatalf("❌ Failed deleting bobbin: %v", err)
	}
	log.Println("✅ Deleted bobbin")

	if err := client.Lot.DeleteOne(lot).Exec(ctx); err != nil {
		log.Fatalf("❌ Failed deleting lot: %v", err)
	}
	log.Println("✅ Deleted lot")

	log.Println("\n🎉 All edge database tests passed!")
}
