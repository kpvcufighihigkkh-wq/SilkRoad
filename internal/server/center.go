package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	centerv1 "github.com/yourusername/igh-silkroad/api/center/v1"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/service"
	"github.com/yourusername/igh-silkroad/internal/sync/center"
)

// CenterServer 中心端HTTP服务器
type CenterServer struct {
	router        *gin.Engine
	server        *http.Server
	client        *ent.Client
	jwtAuth       *middleware.JWTAuth
	uploadHandler *center.UploadHandler
	dataProvider  *center.BaseDataProvider
	userService   *service.UserService
}

// NewCenterServer 创建中心端服务器
func NewCenterServer(client *ent.Client, jwtSecret string, port int) *CenterServer {
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
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		}))
	})

	// JWT认证器
	jwtAuth := middleware.NewJWTAuth(&middleware.JWTConfig{
		SecretKey:       jwtSecret,
		ExpireDuration:  2 * time.Hour,
		RefreshDuration: 24 * time.Hour,
	})

	// 同步处理器
	uploadHandler := center.NewUploadHandler(client)
	dataProvider := center.NewBaseDataProvider(client)

	// 用户服务（用于登录验证）
	userService := service.NewUserService(client)

	s := &CenterServer{
		router:        router,
		client:        client,
		jwtAuth:       jwtAuth,
		uploadHandler: uploadHandler,
		dataProvider:  dataProvider,
		userService:   userService,
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
func (s *CenterServer) registerRoutes() {
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
		// 批次管理
		lotService := service.NewLotService(s.client)
		lotHandler := centerv1.NewLotHandler(lotService)

		lots := authorized.Group("/lots")
		{
			lots.POST("", lotHandler.CreateLot)
			lots.GET("", lotHandler.ListLots)
			lots.GET("/:id", lotHandler.GetLot)
			lots.PUT("/:id/status", lotHandler.UpdateLotStatus)
			lots.DELETE("/:id", lotHandler.DeleteLot)
		}

		// 丝锭管理
		bobbinService := service.NewBobbinService(s.client)
		bobbinHandler := centerv1.NewBobbinHandler(bobbinService)

		bobbins := authorized.Group("/bobbins")
		{
			bobbins.POST("", bobbinHandler.CreateBobbin)
			bobbins.GET("", bobbinHandler.ListBobbins)
			bobbins.GET("/:id", bobbinHandler.GetBobbin)
			bobbins.PUT("/:id/status", bobbinHandler.UpdateBobbinStatus)
			bobbins.POST("/:id/print", bobbinHandler.MarkBobbinPrinted)
			bobbins.DELETE("/:id", bobbinHandler.DeleteBobbin)
		}

		// 用户管理
		userService := service.NewUserService(s.client)
		userHandler := centerv1.NewUserHandler(userService)

		users := authorized.Group("/users")
		{
			users.POST("", userHandler.CreateUser)
			users.GET("", userHandler.ListUsers)
			users.GET("/me", userHandler.GetCurrentUser)
			users.GET("/:id", userHandler.GetUser)
			users.PUT("/:id", userHandler.UpdateUser)
			users.PUT("/:id/password", userHandler.ChangePassword)
			users.DELETE("/:id", userHandler.DeleteUser)
		}
	}

	// 边端数据同步路由（需要Edge Token）
	edge := v1.Group("/edges")
	edge.Use(s.jwtMiddleware())
	{
		edge.POST("/:id/upload", s.handleEdgeUpload)
		edge.GET("/base-data", s.handleBaseDataPull)
	}
}

// jwtMiddleware JWT认证中间件
func (s *CenterServer) jwtMiddleware() gin.HandlerFunc {
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

		// 将claims存入gin.Context（使用gin的Set方法）
		c.Set(string(middleware.ClaimsKey), claims)

		c.Next()
	}
}

// handleLogin 登录处理
func (s *CenterServer) handleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, "参数错误"))
		return
	}

	// 使用UserService验证登录
	user, err := s.userService.ValidateLogin(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusUnauthorized, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "用户名或密码错误"))
		return
	}

	// 生成JWT token
	token, err := s.jwtAuth.GenerateToken(user.ID.String(), user.Username, []string{string(user.Role)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeInternalError, "生成token失败"))
		return
	}

	c.JSON(http.StatusOK, api.Success(gin.H{
		"token": token,
		"user": gin.H{
			"id":        user.ID.String(),
			"username":  user.Username,
			"full_name": user.FullName,
			"role":      string(user.Role),
		},
	}))
}

// handleEdgeUpload 处理边端数据上传
func (s *CenterServer) handleEdgeUpload(c *gin.Context) {
	edgeID := c.Param("id")

	var req struct {
		EdgeID  string      `json:"edge_id"`
		Entries []struct {
			Table     string                 `json:"table"`
			Operation string                 `json:"operation"`
			ID        string                 `json:"id"`
			Data      map[string]interface{} `json:"data"`
		} `json:"entries"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, "参数错误"))
		return
	}

	req.EdgeID = edgeID

	// TODO: 调用uploadHandler处理
	log.Printf("📥 Received upload from edge: %s, entries: %d", edgeID, len(req.Entries))

	c.JSON(http.StatusOK, api.Success(gin.H{
		"applied":  len(req.Entries),
		"rejected": 0,
	}))
}

// handleBaseDataPull 处理基础数据拉取
func (s *CenterServer) handleBaseDataPull(c *gin.Context) {
	var req struct {
		EdgeID string   `json:"edge_id"`
		Tables []string `json:"tables"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError, "参数错误"))
		return
	}

	// TODO: 调用dataProvider处理
	log.Printf("📤 Edge %s requesting base data: %v", req.EdgeID, req.Tables)

	c.JSON(http.StatusOK, api.Success(gin.H{
		"data": make(map[string]interface{}),
	}))
}

// Start 启动服务器
func (s *CenterServer) Start() error {
	log.Printf("🚀 Center server starting on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown 优雅关闭服务器
func (s *CenterServer) Shutdown(ctx context.Context) error {
	log.Println("⏹️  Shutting down center server...")
	return s.server.Shutdown(ctx)
}
