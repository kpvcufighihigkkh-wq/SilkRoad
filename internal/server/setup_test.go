package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	_ "modernc.org/sqlite"
)

var serverTestDBCounter int64

func newServerTestClient(t *testing.T) *ent.Client {
	t.Helper()

	name := fmt.Sprintf("file:servertest%d?mode=memory&cache=shared&_fk=1",
		atomic.AddInt64(&serverTestDBCounter, 1))

	db, err := sql.Open("sqlite", name)
	if err != nil {
		t.Fatalf("failed opening sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("failed enabling foreign keys: %v", err)
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { client.Close() })

	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	return client
}

// testEdgeAuth 测试专用的 Edge 认证中间件。
//
// 生产中由 CenterServer.jwtMiddleware() 承担，但那是 server 包内的方法，
// 测试无法直接复用，因此这里复刻其行为：解析 Bearer token，再用 c.Set
// 把 claims 存入 gin 上下文。必须用 c.Set 而非 context.WithValue ——
// authenticateEdge 用 c.Get 读取（与 api/center/v1/user.go:130 一致）。
func testEdgeAuth(jwtAuth *middleware.JWTAuth) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未提供认证token"))
			c.Abort()
			return
		}

		claims, err := jwtAuth.ParseToken(authHeader)
		if err != nil {
			c.JSON(http.StatusUnauthorized, api.Error(api.CodeTokenInvalid, "token无效"))
			c.Abort()
			return
		}

		c.Set(string(middleware.ClaimsKey), claims)
		c.Next()
	}
}
