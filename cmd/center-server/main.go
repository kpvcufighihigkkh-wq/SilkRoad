package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
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
	dsn := getEnv("DATABASE_URL", "postgres://igh:igh_dev_password@localhost:5432/igh?sslmode=disable")
	client, err := ent.Open(dialect.Postgres, dsn)
	if err != nil {
		log.Fatalf("❌ Failed connecting to database: %v", err)
	}
	defer client.Close()

	log.Println("✅ Database connected")

	// 自动迁移（仅显式开启时）
	//
	// 原来按 ENV=development 自动跑，双机热备下这是错的：备机提升为主机后
	// 两个 Center 都可能对同一个库跑 DDL，并发 DDL 会互相锁表，而 Schema.Create
	// 在部分失败时留下半迁移状态。生产必须由发布流程单独跑迁移，
	// 因此改为独立开关 AUTO_MIGRATE，且默认关闭。
	if getEnv("AUTO_MIGRATE", "false") == "true" {
		if getEnv("ENV", "development") != "development" {
			log.Fatalf("❌ AUTO_MIGRATE 在生产环境被拒绝：请在发布流程中单独执行迁移，" +
				"不要让业务进程跑 DDL")
		}
		if err := client.Schema.Create(context.Background()); err != nil {
			log.Printf("⚠️  Schema migration warning: %v", err)
		}
	}

	// 创建服务器
	//
	// JWT_SECRET 在生产环境必须显式配置且不得使用默认值：双机热备下两台 Center
	// 必须用同一个密钥，否则切换后所有 Edge 立刻 401，而 401 不计重试 ——
	// 上传静默停止而 /health 仍返回 ok，是最难排查的一类故障。
	// 默认值在这里不是「方便本地启动」，而是「生产上必然出错」，故直接拒启。
	const defaultJWTSecret = "igh-secret-key-change-in-production"
	jwtSecret := getEnv("JWT_SECRET", defaultJWTSecret)
	if getEnv("ENV", "development") != "development" && jwtSecret == defaultJWTSecret {
		log.Fatalf("❌ 生产环境必须设置 JWT_SECRET（当前为默认值）。" +
			"主备两台 Center 必须使用同一密钥，否则设备凭证全部失效")
	}

	// 端口可配置：默认 8080，但该端口常被同机的其它服务占用（例如 Edge
	// 或并行的开发实例），写死会让本地联调时必须改代码、重新编译。
	port := 8080
	if raw := getEnv("PORT", ""); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 || parsed > 65535 {
			log.Fatalf("❌ Invalid PORT %q：必须是 1-65535 之间的整数", raw)
		}
		port = parsed
	}

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
	log.Println("   - Lots:   http://localhost:8080/v1/lots")

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
