package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/api/middleware"
	"github.com/yourusername/igh-silkroad/internal/database/ent"
	"github.com/yourusername/igh-silkroad/internal/database/ent/user"
)

// requireActiveUser 用数据库中的权威状态刷新调用者身份。
//
// 为什么需要它：JWT 里的 roles 与 user_id 是签发那一刻的快照，token 有效期
// （默认 2 小时）内不会变。管理后台既然用角色做了真实授权，停用一个账号、
// 或把某个管理员降级，就必须立刻生效 —— 否则被停用的人还能在这两小时里
// 签发设备凭证。本中间件把「停用」与「角色变更」的生效延迟压到一次请求。
//
// 挂在 authorized 组、requireUserPrincipal 之后：设备凭证在上一步已被拒绝，
// 这里可以假定 claims.UserID 指向一个人类用户。设备路由走独立的 edge 组，
// 不经过本中间件，因此 30 天有效期的设备凭证不受影响。
//
// 失败策略：查不到用户（已删除）→ 401；被停用 → 403 + 20004；数据库错误
// → 500 且不放行（fail closed）。角色以数据库为准覆盖 claims.Roles，
// 下游的 requireRole 与 primaryRole 因此自动拿到最新角色。
func (s *CenterServer) requireActiveUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, exists := c.Get(string(middleware.ClaimsKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未登录"))
			c.Abort()
			return
		}

		claims, ok := raw.(*middleware.JWTClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "未登录"))
			c.Abort()
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "token 中的用户标识无效"))
			c.Abort()
			return
		}

		u, err := s.client.User.Query().
			Where(user.IDEQ(userID)).
			Only(c.Request.Context())
		if err != nil {
			if ent.IsNotFound(err) {
				c.JSON(http.StatusUnauthorized, api.Error(api.CodeUnauthorized, "账号不存在或已被删除"))
				c.Abort()
				return
			}
			c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, "校验账号状态失败"))
			c.Abort()
			return
		}

		if !u.IsActive {
			c.JSON(http.StatusForbidden, api.Error(api.CodeForbidden, "用户已停用"))
			c.Abort()
			return
		}

		// 用库里的角色覆盖 token 内的快照，让降级立即生效。
		claims.Roles = []string{string(u.Role)}

		c.Next()
	}
}
