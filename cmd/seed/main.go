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
		Name   string
		Number string
	}{
		{"Line 1", "L001"},
		{"Line 2", "L002"},
		{"Line 3", "L003"},
		{"Line 4", "L004"},
	}

	for _, line := range lines {
		_, err := client.SpinningLine.Create().
			SetLineName(line.Name).
			SetLineNumber(line.Number).
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
		Password string
	}{
		{"admin", "系统管理员", user.RoleAdmin, "$2a$10$admin_hash"},
		{"operator1", "操作员1", user.RoleOperator, "$2a$10$operator_hash"},
		{"inspector1", "质检员1", user.RoleQualityInspector, "$2a$10$inspector_hash"},
		{"viewer1", "查看员1", user.RoleViewer, "$2a$10$viewer_hash"},
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
		SetProjectNumber("PRJ-2024-001").
		SetProjectName("Demo Project").
		SetProductType("FDY").
		SetProductSpec("150D/48F").
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
		SetOrderQuantity(1000).
		SetStatus("pending").
		Save(ctx)

	if err != nil {
		return err
	}

	log.Printf("  ✓ Created order: %s", order.OrderNumber)

	return nil
}
