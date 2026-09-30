package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/api"
	centerv1 "github.com/yourusername/igh-silkroad/api/center/v1"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/service"
	"github.com/yourusername/igh-silkroad/internal/sync/center"
	"github.com/yourusername/igh-silkroad/internal/sync/models"
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
	edgeService   *service.EdgeService
}

// applyTrustedProxies 配置 Gin 的可信代理。
//
// 必须只信任反向代理那一层：全信任（默认的 0.0.0.0/0）会让攻击者用
// X-Forwarded-For 冒充任意 IP；设为 nil 则会让 ClientIP() 恒返回代理自身的
// IP，使基于 IP 的设备识别失效。两种情况都会破坏上传接口的鉴权。
func applyTrustedProxies(router *gin.Engine) error {
	raw := os.Getenv("TRUSTED_PROXIES")
	if raw == "" {
		// 未配置时不信任任何代理，只信本机回环
		raw = "127.0.0.1/32,::1/128"
	}

	proxies := make([]string, 0, 4)
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}

	return router.SetTrustedProxies(proxies)
}

// NewCenterServer 创建中心端服务器
func NewCenterServer(client *ent.Client, jwtSecret string, port int) *CenterServer {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	if err := applyTrustedProxies(router); err != nil {
		log.Fatalf("设置可信代理失败: %v", err)
	}

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

	// 边端设备服务：管理路由与上传鉴权共用同一实例
	edgeService := service.NewEdgeService(client)

	s := &CenterServer{
		router:        router,
		client:        client,
		jwtAuth:       jwtAuth,
		uploadHandler: uploadHandler,
		dataProvider:  dataProvider,
		userService:   userService,
		edgeService:   edgeService,
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

		// 边端设备管理（管理员操作，需用户 JWT）
		edgeAdminHandler := centerv1.NewEdgeHandler(s.edgeService, s.jwtAuth)

		edgesAdmin := authorized.Group("/edges")
		edgesAdmin.Use(requireUserPrincipal())
		{
			edgesAdmin.POST("", edgeAdminHandler.CreateEdge)
			edgesAdmin.GET("", edgeAdminHandler.ListEdges)
			edgesAdmin.GET("/:code", edgeAdminHandler.GetEdge)
			edgesAdmin.POST("/:code/token", edgeAdminHandler.IssueToken)
		}
	}

	// 边端数据同步路由（需要Edge Token）
	edge := v1.Group("/edges")
	edge.Use(s.jwtMiddleware())
	{
		edge.POST("/:code/upload", s.handleEdgeUpload)
		edge.GET("/:code/base-data", s.handleBaseDataPull)
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

// requireUserPrincipal 拒绝携带设备凭证（DeviceID 非空）的请求。
//
// Center 的用户 token 与 Edge token 用同一个 SecretKey 签名
// （api/middleware/jwt.go 中 GenerateToken 与 GenerateEdgeToken 均调用
// SignedString(j.config.SecretKey)），因此 jwtMiddleware 只能证明"签名有效"，
// 无法区分身份类型。若不在管理路由上加这道校验，持有自身 Edge token 的设备
// 就能调用 POST /v1/edges/:code/token，为任意其它已注册设备签发凭证。
func requireUserPrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, exists := c.Get(string(middleware.ClaimsKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未提供认证token"))
			c.Abort()
			return
		}

		claims, ok := raw.(*middleware.JWTClaims)
		if !ok || claims.DeviceID != "" {
			c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "设备凭证无权访问管理接口"))
			c.Abort()
			return
		}

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

// normalizeIP 把 IP 字符串规范化为可比较的形式。
//
// 必须规范化再比较：ClientIP() 返回的是 X-Forwarded-For 头里的原始字符串
// （而非 net.IP.String()），因此可能带前导零、IPv6 方括号或 IPv4 映射前缀
// （::ffff:203.0.113.9）。直接与注册值做字符串相等判断会把同一台正确的主机
// 误判为不匹配，产生难以排查的 403。
//
// 解析失败时返回空串，调用方据此拒绝请求 —— 规范化失败必须 fail closed。
func normalizeIP(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	// 去掉可能的端口，以及 IPv6 的方括号
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		trimmed = host
	}
	trimmed = strings.TrimPrefix(strings.TrimSuffix(trimmed, "]"), "[")

	ip := net.ParseIP(trimmed)
	if ip == nil {
		return ""
	}

	// 统一映射形式，使 ::ffff:203.0.113.9 与 203.0.113.9 可比
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	return ip.String()
}

// authenticateEdge 对上传请求做四重校验，返回权威的设备记录。
//
// 校验顺序与理由：
//  1. token 有效且带 DeviceID —— HMAC 签名，不可伪造
//  2. DeviceID 对应的设备已注册 —— 未注册设备不得被自动接纳
//  3. URL 的 :code 与 token 的 DeviceID 一致 —— 防 A 持己方 token 冒充 B
//  4. ClientIP 与注册的 ip_address 一致 —— 第二重约束
//
// 校验通过后，调用方必须使用返回记录中的 ID 作为数据归属，
// 不得采信请求体里的 edge_id。
//
// 校验失败时已写入响应，返回的 error 仅供调用方提前 return 使用。
func authenticateEdge(c *gin.Context, svc *service.EdgeService) (*service.EdgeResponse, error) {
	// 注意：Center 的 jwtMiddleware 用 c.Set 写入 gin 自己的 Keys map，
	// 而 middleware.GetClaimsFromContext 读的是 c.Request.Context()。
	// 二者不互通（gin 的 Value() 只对 string 类型的 key 回退查 Keys，
	// 而 ClaimsKey 是自定义类型），因此这里必须用 c.Get 读取 —— 与
	// api/center/v1/user.go:130 的既有写法保持一致。
	rawClaims, exists := c.Get(string(middleware.ClaimsKey))
	if !exists {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "缺少设备凭证"))
		return nil, errors.New("missing device claims")
	}

	claims, ok := rawClaims.(*middleware.JWTClaims)
	if !ok || claims.DeviceID == "" {
		c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "设备凭证无效"))
		return nil, errors.New("invalid device claims")
	}

	edgeRow, err := svc.GetEdgeByCode(c.Request.Context(), claims.DeviceID)
	if err != nil {
		if errors.Is(err, service.ErrEdgeNotRegistered) {
			c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "设备未注册"))
			return nil, err
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return nil, err
	}

	if c.Param("code") != claims.DeviceID {
		c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "凭证与请求设备不符"))
		return nil, errors.New("edge code mismatch")
	}

	clientIP := normalizeIP(c.ClientIP())
	registeredIP := normalizeIP(edgeRow.IPAddress)
	if clientIP == "" || registeredIP == "" || clientIP != registeredIP {
		c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "来源IP与注册地址不符"))
		return nil, errors.New("ip mismatch")
	}

	return edgeRow, nil
}

// handleEdgeUpload 处理边端数据上传
func (s *CenterServer) handleEdgeUpload(c *gin.Context) {
	var req struct {
		EdgeID  string `json:"edge_id"`
		Cursor  int64  `json:"cursor"`
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

	edgeRow, err := authenticateEdge(c, s.edgeService)
	if err != nil {
		return // authenticateEdge 已写响应
	}

	// 权威身份由 Center 推导，忽略请求体中的 edge_id。
	//
	// 这里必须传设备 UUID，不能传 EdgeCode：lot.edge_id 是
	// field.UUID("edge_id", uuid.UUID{})，指向 edges.id 的外键。
	// 传 "edge-001" 这类编码时 uuid.Parse 会返回
	// "invalid UUID length: 8"，调用方退化为 uuid.Nil，
	// SetEdgeID 的条件分支永不触发，外键恒为 NULL ——
	// 与「身份被伪造」在观测上无法区分，等于白做这层鉴权。
	uploadReq := &models.UploadRequest{
		EdgeID: edgeRow.ID, // UUID 字符串，非 EdgeCode
		Cursor: req.Cursor,
	}
	for _, e := range req.Entries {
		entryID, idErr := uuid.Parse(e.ID)
		if idErr != nil {
			c.JSON(http.StatusBadRequest, api.Error(api.CodeParamError,
				fmt.Sprintf("entry id 非法: %q", e.ID)))
			return
		}
		uploadReq.Entries = append(uploadReq.Entries, models.UploadEntry{
			Table:     e.Table,
			Operation: e.Operation,
			ID:        entryID,
			Data:      e.Data,
		})
	}

	resp, err := s.uploadHandler.HandleUpload(c.Request.Context(), uploadReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	// 心跳：鉴权成功即视为设备在线
	if hbErr := s.edgeService.UpdateHeartbeat(c.Request.Context(), edgeRow.EdgeCode); hbErr != nil {
		log.Printf("⚠️  刷新设备心跳失败 %s: %v", edgeRow.EdgeCode, hbErr)
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// handleBaseDataPull 处理基础数据拉取
//
// 与上传端点共用同一套设备鉴权：未接线前该端点直接返回空 map 却报 200，
// 既是假成功，也对未注册设备开放。
func (s *CenterServer) handleBaseDataPull(c *gin.Context) {
	edgeRow, err := authenticateEdge(c, s.edgeService)
	if err != nil {
		return // authenticateEdge 已写响应
	}

	// 权威身份取自鉴权结果，不读请求里的 edge_id。
	tables := c.QueryArray("tables")

	resp, err := s.dataProvider.HandlePullRequest(c.Request.Context(), &models.BaseDataPullRequest{
		EdgeID: edgeRow.EdgeCode,
		Tables: tables,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
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
