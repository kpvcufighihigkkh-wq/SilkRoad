package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/edge"
	"github.com/yourusername/igh-silkroad/internal/database/ent/lot"
	"github.com/yourusername/igh-silkroad/internal/database/ent/spinningline"
	"github.com/yourusername/igh-silkroad/internal/database/ent/user"

	"entgo.io/ent/dialect"
	_ "github.com/lib/pq"
)

func main() {
	// 连接数据库
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://igh:igh_dev_password@localhost:5432/igh?sslmode=disable"
	}

	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		log.Fatalf("failed opening connection to postgres: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	log.Println("🌱 Seeding database...")

	// 创建线体
	if err := seedSpinningLines(ctx, client); err != nil {
		log.Fatalf("failed seeding spinning lines: %v", err)
	}

	// 创建用户
	if err := seedUsers(ctx, client); err != nil {
		log.Fatalf("failed seeding users: %v", err)
	}

	// 创建核心业务数据
	if err := seedCoreData(ctx, client); err != nil {
		log.Fatalf("failed seeding core data: %v", err)
	}

	log.Println("✅ Database seeded successfully!")
}

func seedSpinningLines(ctx context.Context, client *ent.Client) error {
	log.Println("🏭 Creating spinning lines...")

	lines := []struct {
		Name     string
		Number   string
		Capacity int
	}{
		{"Line 1", "L001", 48},
		{"Line 2", "L002", 48},
		{"Line 3", "L003", 48},
		{"Line 4", "L004", 48},
	}

	for _, line := range lines {
		_, err := client.SpinningLine.Create().
			SetLineName(line.Name).
			SetLineNumber(line.Number).
			SetCapacity(line.Capacity).
			SetStatus("idle").
			Save(ctx)

		if err != nil {
			return err
		}
		log.Printf("  ✓ Created line: %s", line.Name)
	}

	return nil
}

func seedUsers(ctx context.Context, client *ent.Client) error {
	log.Println("👤 Creating users...")

	users := []struct {
		Username string
		FullName string
		Role     user.Role
		Password string // SHA256 hash
	}{
		{"admin", "系统管理员", user.RoleAdmin, "240be518fabd2724ddb6f04eeb1da5967448d7e831c08c8fa822809f74c720a9"},     // admin123
		{"operator1", "操作员1", user.RoleOperator, "5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8"},     // password
		{"inspector1", "质检员1", user.RoleInspector, "5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8"}, // password
		{"viewer1", "查看员1", user.RoleViewer, "5e884898da28047151d0e56f8dc6292773603d0d6aabbdd62a11ef721d1542d8"},        // password
	}

	for _, u := range users {
		_, err := client.User.Create().
			SetUsername(u.Username).
			SetPasswordHash(u.Password).
			SetFullName(u.FullName).
			SetRole(u.Role).
			SetIsActive(true).
			Save(ctx)

		if err != nil {
			return err
		}
		log.Printf("  ✓ Created user: %s (%s)", u.Username, u.Role)
	}

	return nil
}

// seedCoreData 创建核心业务测试数据
func seedCoreData(ctx context.Context, client *ent.Client) error {
	log.Println("🌱 Seeding core data...")

	// 边端设备
	edge, err := client.Edge.Create().
		SetEdgeCode("edge-001").
		SetEdgeName("一号边端").
		SetIPAddress("192.168.1.101").
		SetStatus(edge.StatusOnline).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create edge: %w", err)
	}
	log.Printf("  ✓ Created edge: %s (ID: %s)", edge.EdgeCode, edge.ID)

	// 纺丝线体
	line, err := client.SpinningLine.Create().
		SetLineName("A线").
		SetLineNumber("LINE-A").
		SetEdgeID(edge.ID).
		SetLocation("一车间").
		SetCapacity(48).
		SetStatus(spinningline.StatusRunning).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create spinning line: %w", err)
	}
	log.Printf("  ✓ Created spinning line: %s (ID: %s)", line.LineName, line.ID)

	// 等级基础数据
	for _, g := range []struct {
		code string
		name string
	}{
		{"A", "优等品"},
		{"B", "一等品"},
		{"C", "合格品"},
		{"D", "等外品"},
	} {
		created, err := client.Grade.Create().
			SetGradeType("final").
			SetGradeCode(g.code).
			SetGradeName(g.name).
			SetSortOrder(0).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create grade %s: %w", g.code, err)
		}
		log.Printf("  ✓ Created grade: %s", created.GradeCode)
	}

	// 批次
	lot, err := client.Lot.Create().
		SetLotNumber("FDY-2026-001-01").
		SetEdgeID(edge.ID).
		SetPlcLotNumber("PLC-2026-001").
		SetProductType(lot.ProductTypeFDY).
		SetProductSpec("150D/48F").
		SetPlannedQuantity(4800).
		SetActualQuantity(0).
		SetStatus(lot.StatusInProgress).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create lot: %w", err)
	}
	log.Printf("  ✓ Created lot: %s (ID: %s)", lot.LotNumber, lot.ID)

	return nil
}
