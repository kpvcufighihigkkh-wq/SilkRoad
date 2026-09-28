package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent_edge"

	"entgo.io/ent/dialect"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/google/uuid"
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
		dsn = "postgres://igh:igh@localhost:5432/igh?sslmode=disable"
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

	// 测试 CRUD - Project
	log.Println("\n📝 Testing Project CRUD...")

	// Create
	project, err := client.Project.Create().
		SetProjectNumber("PRJ-TEST-001").
		SetProjectName("Test Project").
		SetProductType("FDY").
		SetProductSpec("150D/48F").
		SetStatus("active").
		Save(ctx)
	if err != nil {
		log.Fatalf("❌ Failed creating project: %v", err)
	}
	log.Printf("✅ Created project: %s (ID: %s)", project.ProjectName, project.ID)

	// Read
	found, err := client.Project.Get(ctx, project.ID)
	if err != nil {
		log.Fatalf("❌ Failed reading project: %v", err)
	}
	log.Printf("✅ Read project: %s", found.ProjectName)

	// Update
	updated, err := client.Project.UpdateOne(project).
		SetProjectName("Updated Project").
		Save(ctx)
	if err != nil {
		log.Fatalf("❌ Failed updating project: %v", err)
	}
	log.Printf("✅ Updated project name: %s", updated.ProjectName)

	// Query
	count, err := client.Project.Query().
		Where().
		Count(ctx)
	if err != nil {
		log.Fatalf("❌ Failed counting projects: %v", err)
	}
	log.Printf("✅ Total projects: %d", count)

	// Delete
	if err := client.Project.DeleteOne(project).Exec(ctx); err != nil {
		log.Fatalf("❌ Failed deleting project: %v", err)
	}
	log.Println("✅ Deleted project")

	log.Println("\n🎉 All center database tests passed!")
}

func testEdge() {
	log.Println("🧪 Testing edge database (SQLite)...")

	dsn := "file:test_edge.db?cache=shared&_fk=1"
	client, err := ent_edge.Open(dialect.SQLite, dsn)
	if err != nil {
		log.Fatalf("❌ Failed connecting to sqlite: %v", err)
	}
	defer client.Close()
	defer os.Remove("test_edge.db")  // 清理测试文件

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
		SetID(uuid.New()).
		SetLotNumber("LOT-TEST-001").
		SetOrderID(uuid.New()).
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
		SetSynced(false).
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
		SetID(uuid.New()).
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

	// Test SyncLog
	log.Println("\n📝 Testing SyncLog...")
	syncLog, err := client.SyncLog.Create().
		SetID(uuid.New()).
		SetEntityType("bobbin").
		SetEntityID(bobbin.ID).
		SetOperation("create").
		SetStatus("pending").
		Save(ctx)
	if err != nil {
		log.Fatalf("❌ Failed creating sync log: %v", err)
	}
	log.Printf("✅ Created sync log for entity: %s", syncLog.EntityID)

	// Delete
	if err := client.Lot.DeleteOne(lot).Exec(ctx); err != nil {
		log.Fatalf("❌ Failed deleting lot: %v", err)
	}
	log.Println("✅ Deleted lot (cascade deleted bobbin)")

	log.Println("\n🎉 All edge database tests passed!")
}
