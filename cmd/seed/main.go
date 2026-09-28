package main

import (
	"context"
	"log"
	"os"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/user"

	"entgo.io/ent/dialect"
	_ "github.com/lib/pq"
)

func main() {
	// 连接数据库
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://igh:igh@localhost:5432/igh?sslmode=disable"
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

	// 创建示例项目
	if err := seedProjects(ctx, client); err != nil {
		log.Fatalf("failed seeding projects: %v", err)
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

func seedProjects(ctx context.Context, client *ent.Client) error {
	log.Println("📋 Creating sample project...")

	project, err := client.Project.Create().
		SetProjectName("Demo Project").
		SetCustomerName("Demo Customer").
		SetStatus("active").
		Save(ctx)

	if err != nil {
		return err
	}

	log.Printf("  ✓ Created project: %s (ID: %s)", project.ProjectName, project.ID)

	// 创建示例订单
	order, err := client.Order.Create().
		SetOrderNumber("ORD-2024-001").
		SetProjectID(project.ID).
		SetProductType("FDY").
		SetProductSpec("150D/48F").
		SetTargetQuantity(1000).
		SetStatus("pending").
		Save(ctx)

	if err != nil {
		return err
	}

	log.Printf("  ✓ Created order: %s", order.OrderNumber)

	return nil
}
