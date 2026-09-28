package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/server"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func main() {
	log.Println("🚀 Starting IGH Edge Server...")

	// 获取配置
	edgeID := os.Getenv("EDGE_ID")
	if edgeID == "" {
		edgeID = "edge-001" // 默认边端ID
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./edge.db" // 默认SQLite路径
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081" // 默认端口
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "edge-secret-key-change-in-production"
	}

	centerURL := os.Getenv("CENTER_URL")
	if centerURL == "" {
		centerURL = "http://localhost:8080" // 默认中心端地址
	}

	// 连接SQLite数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("❌ Failed opening connection to sqlite: %v", err)
	}

	// 启用外键约束
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Fatalf("❌ Failed to enable foreign keys: %v", err)
	}

	// 创建Ent客户端
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))
	defer client.Close()

	log.Println("✅ Database connected:", dbPath)

	// 自动迁移schema
	ctx := context.Background()
	if err := client.Schema.Create(ctx); err != nil {
		log.Printf("⚠️  Schema migration warning: %v", err)
	}

	// 创建Edge服务器
	portInt := 8081
	fmt.Sscanf(port, "%d", &portInt)

	edgeServer := server.NewEdgeServer(client, edgeID, centerURL, jwtSecret, portInt)

	log.Printf("✅ Edge server initialized (ID: %s)", edgeID)
	log.Println("📖 API endpoints:")
	log.Printf("   - Health: http://localhost:%s/health", port)
	log.Printf("   - Doffing: POST http://localhost:%s/v1/doffing", port)
	log.Printf("   - Bobbins: http://localhost:%s/v1/bobbins", port)

	// 启动服务器
	go func() {
		if err := edgeServer.Start(); err != nil {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⏹️  Shutting down edge server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := edgeServer.Shutdown(ctx); err != nil {
		log.Printf("❌ Server forced to shutdown: %v", err)
	}

	log.Println("✅ Edge server stopped")
}
