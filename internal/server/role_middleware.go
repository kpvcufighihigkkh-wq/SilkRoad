package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/api/middleware"
)

// requireRole 限制路由只能由指定角色访问。
//
// 与 requireUserPrincipal 的分工：后者只回答「是不是人类用户」（挡设备凭证），
// 本中间件回答「这个人类用户能不能做这件事」。两者需要同时挂载 ——
// requireRole 依赖 claims 存在，若单独使用而 claims 缺失，它会因读不到角色
// 而拒绝（fail closed），但错误信息不如 requireUserPrincipal 明确。
//
// 允许列表为空时拒绝一切：空列表是调用方的编程错误，
// 「没有角色需要检查」不能推导出「所有人都可以进」。
func requireRole(roles ...string) gin.HandlerFunc {
	// 预先转成集合，避免每请求线性扫描
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		raw, exists := c.Get(string(middleware.ClaimsKey))
		if !exists {
			c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "无权访问该接口"))
			c.Abort()
			return
		}

		claims, ok := raw.(*middleware.JWTClaims)
		if !ok {
			c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "无权访问该接口"))
			c.Abort()
			return
		}

		for _, r := range claims.Roles {
			if _, hit := allowed[r]; hit {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "无权访问该接口"))
		c.Abort()
	}
}
