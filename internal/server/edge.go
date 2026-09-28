package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	edgev1 "github.com/yourusername/igh-silkroad/api/edge/v1"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/service"
	"github.com/yourusername/igh-silkroad/internal/sync/edge"
)

// EdgeServer 边端HTTP服务器
type EdgeServer struct {
	router      *gin.Engine
	server      *http.Server
	client      *ent.Client
	edgeID      string
	centerURL   string
	jwtAuth     *middleware.JWTAuth
	syncClient  *edge.SyncClient
}

// NewEdgeServer 创建边端服务器
func NewEdgeServer(client *ent.Client, edgeID, centerURL, jwtSecret string, port int) *EdgeServer {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// CORS中间件
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, api.Success(gin.H{
			"status":  "ok",
			"edge_id": edgeID,
			"time":    time.Now().Format(time.RFC3339),
		}))
	})

	// JWT认证器
	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:       jwtSecret,
		ExpireDuration:  2 * time.Hour,
		RefreshDuration: 24 * time.Hour,
	})

	// 同步客户端
	syncClient := edge.NewSyncClient(edgeID, centerURL, client)

	s := &EdgeServer{
		router:     router,
		client:     client,
		edgeID:     edgeID,
		centerURL:  centerURL,
		jwtAuth:    jwtAuth,
		syncClient: syncClient,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: router,
		},
	}

	// 注册路由
	s.registerRoutes()

	return s
}

// registerRoutes 注册路由
func (s *EdgeServer) registerRoutes() {
	// API v1路由组
	v1 := s.router.Group("/v1")

	// 公开路由（不需要认证）
	{
		v1.POST("/login", s.handleLogin)
	}

	// 认证路由（需要JWT）
	authorized := v1.Group("")
	authorized.Use(s.jwtMiddleware())
	{
		// 落纱操作
		doffingService := service.NewDoffingService(s.client)
		doffingHandler := edgev1.NewDoffingHandler(doffingService)

		doffing := authorized.Group("/doffing")
		{
			doffing.POST("", doffingHandler.CreateDoffing)           // 创建落纱操作
			doffing.GET("", doffingHandler.ListDoffing)              // 查询落纱记录
			doffing.GET("/:id", doffingHandler.GetDoffing)           // 获取落纱详情
			doffing.PUT("/:id/confirm", doffingHandler.ConfirmDoffing) // 确认落纱
			doffing.PUT("/:id/cancel", doffingHandler.CancelDoffing)   // 取消落纱
		}

		// 丝锭管理
		bobbinService := service.NewBobbinService(s.client)
		bobbinHandler := edgev1.NewBobbinHandler(bobbinService)

		bobbins := authorized.Group("/bobbins")
		{
			bobbins.GET("", bobbinHandler.ListBobbins)               // 查询丝锭
			bobbins.GET("/:id", bobbinHandler.GetBobbin)             // 获取丝锭详情
			bobbins.PUT("/:id/weigh", bobbinHandler.WeighBobbin)     // 称重
			bobbins.PUT("/:id/inspect", bobbinHandler.InspectBobbin) // 质检
		}

		// 数据同步
		sync := authorized.Group("/sync")
		{
			sync.POST("/upload", s.handleSyncUpload)     // 上传数据到中心
			sync.POST("/download", s.handleSyncDownload) // 从中心下载数据
		}
	}
}

// jwtMiddleware JWT认证中间件
func (s *EdgeServer) jwtMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未提供认证token"))
			c.Abort()
			return
		}

		claims, err := s.jwtAuth.ParseToken(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, api.Error(api.CodeTokenInvalid, "token无效"))
			c.Abort()
			return
		}

		// 将claims存入gin.Context
		c.Set(string(middleware.ClaimsKey), claims)

		c.Next()
	}
}

// handleLogin 登录处理（简化版，使用本地验证）
func (s *EdgeServer) handleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, "参数错误"))
		return
	}

	// TODO: 实现本地用户验证
	// 目前使用硬编码的测试账号
	if req.Username == "edge" && req.Password == "edge123" {
		token, err := s.jwtAuth.GenerateToken("edge-user-id", req.Username, []string{"operator"})
		if err != nil {
			c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, "生成token失败"))
			return
		}

		c.JSON(http.StatusOK, api.Success(gin.H{
			"token": token,
			"user": gin.H{
				"username": req.Username,
				"edge_id":  s.edgeID,
				"role":     "operator",
			},
		}))
		return
	}

	c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "用户名或密码错误"))
}

// handleSyncUpload 处理数据上传
func (s *EdgeServer) handleSyncUpload(c *gin.Context) {
	// TODO: 调用syncClient上传数据
	log.Println("📤 Uploading data to center...")

	c.JSON(http.StatusOK, api.Success(gin.H{
		"uploaded": 0,
		"failed":   0,
	}))
}

// handleSyncDownload 处理数据下载
func (s *EdgeServer) handleSyncDownload(c *gin.Context) {
	// TODO: 调用syncClient下载数据
	log.Println("📥 Downloading data from center...")

	c.JSON(http.StatusOK, api.Success(gin.H{
		"downloaded": 0,
		"applied":    0,
	}))
}

// Start 启动服务器
func (s *EdgeServer) Start() error {
	log.Printf("🚀 Edge server starting on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown 优雅关闭服务器
func (s *EdgeServer) Shutdown(ctx context.Context) error {
	log.Println("⏹️  Shutting down edge server...")
	return s.server.Shutdown(ctx)
}
