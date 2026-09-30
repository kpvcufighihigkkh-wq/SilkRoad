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

// defaultSyncInterval SYNC_INTERVAL 未配置或取值非法时的兜底间隔
const defaultSyncInterval = 5 * time.Minute

// parseSyncInterval 解析 SYNC_INTERVAL，任何非法取值都回退到默认值。
//
// 必须同时校验解析错误与取值本身：time.ParseDuration("0")、"0s"、"-5m"
// 都返回 err == nil，但把非正值交给 time.NewTicker 会在调度 goroutine 内
// panic（"non-positive interval for NewTicker"）并终止进程。
func parseSyncInterval(raw string) time.Duration {
	if raw == "" {
		return defaultSyncInterval
	}

	interval, err := time.ParseDuration(raw)
	switch {
	case err != nil:
		log.Printf("⚠️  SYNC_INTERVAL 解析失败 (%v)，回退到 %s", err, defaultSyncInterval)
		return defaultSyncInterval
	case interval <= 0:
		log.Printf("⚠️  SYNC_INTERVAL 必须为正数 (得到 %s)，回退到 %s", interval, defaultSyncInterval)
		return defaultSyncInterval
	default:
		return interval
	}
}

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

	// 访问 Center 上传端点的设备凭证。
	//
	// 缺失时直接退出，而不是告警后继续：Center 会对缺少有效凭证的上传
	// 返回 401，而上传器把非 200 一律计入重试。凭默认 5m 间隔，约 25 分钟
	// 后所有 pending 记录都会被标记为 failed，且本仓库没有任何回收路径 ——
	// 那是不可恢复的数据丢失，比启动失败严重得多。
	// 这与本文件对其它致命配置的处理一致（如 sqlite 打开失败）。
	centerToken := os.Getenv("CENTER_TOKEN")
	if centerToken == "" {
		log.Fatal("❌ CENTER_TOKEN 未配置：无法向 Center 上传。" +
			"请先用 POST /v1/edges/:code/token 获取凭证后再启动")
	}

	// 兜底重试间隔，实时上传由业务操作触发
	syncInterval := parseSyncInterval(os.Getenv("SYNC_INTERVAL"))

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
