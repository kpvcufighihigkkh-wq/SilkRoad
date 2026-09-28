package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/server"

	"entgo.io/ent/dialect"
	_ "github.com/lib/pq"
)

func main() {
	log.Println("🚀 Starting IGH Center Server...")

	// 连接数据库
	dsn := getEnv("DATABASE_URL", "postgres://igh:igh@localhost:5432/igh?sslmode=disable")
	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		log.Fatalf("❌ Failed connecting to database: %v", err)
	}
	defer client.Close()

	log.Println("✅ Database connected")

	// 自动迁移（开发环境）
	if getEnv("ENV", "development") == "development" {
		if err := client.Schema.Create(context.Background()); err != nil {
			log.Printf("⚠️  Schema migration warning: %v", err)
		}
	}

	// 创建服务器
	jwtSecret := getEnv("JWT_SECRET", "igh-secret-key-change-in-production")
	port := 8080

	srv := server.NewCenterServer(client, jwtSecret, port)

	// 启动服务器（goroutine）
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	log.Printf("✅ Center server started on port %d", port)
	log.Println("📖 API endpoints:")
	log.Println("   - Health: http://localhost:8080/health")
	log.Println("   - Login:  POST http://localhost:8080/v1/login")
	log.Println("   - Orders: http://localhost:8080/v1/orders")

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⏹️  Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("❌ Server shutdown error: %v", err)
	}

	log.Println("✅ Server stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
