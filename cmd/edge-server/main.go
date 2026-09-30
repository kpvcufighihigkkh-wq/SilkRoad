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
	"github.com/yourusername/igh-silkroad/internal/sync/edge"

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

	// 访问 Center 上传端点的设备凭证。缺失时上传会收到 401，
	// 因此这里只告警不退出 —— 服务本身仍可提供本地查询功能。
	centerToken := os.Getenv("CENTER_TOKEN")
	if centerToken == "" {
		log.Println("⚠️  CENTER_TOKEN 未配置，向 Center 上传将被拒绝 (401)")
	}

	// 兜底重试间隔，实时上传由业务操作触发
	syncInterval := 5 * time.Minute
	if v := os.Getenv("SYNC_INTERVAL"); v != "" {
		interval, err := time.ParseDuration(v)
		if err != nil {
			log.Printf("⚠️  SYNC_INTERVAL 解析失败 (%v)，回退到 %s", err, syncInterval)
		} else {
			syncInterval = interval
		}
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

	edgeServer := server.NewEdgeServer(client, edgeID, centerURL, jwtSecret, centerToken, portInt)

	// 同步调度器（网络故障后的兜底重试）
	syncCtx, syncCancel := context.WithCancel(context.Background())
	defer syncCancel()

	scheduler := edge.NewScheduler(edgeServer.Uploader(), syncInterval)
	scheduler.Start(syncCtx)

	log.Printf("✅ Edge server initialized (ID: %s)", edgeID)
	log.Printf("🔄 兜底同步间隔: %s", syncInterval)
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

	// 先停调度器，避免关闭过程中仍有上传在飞行
	scheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := edgeServer.Shutdown(ctx); err != nil {
		log.Printf("❌ Server forced to shutdown: %v", err)
	}

	log.Println("✅ Edge server stopped")
}
