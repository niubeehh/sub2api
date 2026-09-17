package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ReadOnlyGuard 只读角色写操作拦截中间件。
// 必须在认证中间件（adminAuth）之后使用。
//
// readonly 角色可查看管理面全部数据；除 GET/HEAD/OPTIONS 外的请求方法
// （POST/PUT/DELETE/PATCH 等变更类操作）一律 403。
func ReadOnlyGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRoleFromContext(c)
		if !ok {
			AbortWithError(c, 401, "UNAUTHORIZED", "User not found in context")
			return
		}

		if role == service.RoleReadOnly {
			switch c.Request.Method {
			case "GET", "HEAD", "OPTIONS":
				// 只读角色允许查询类请求
			default:
				AbortWithError(c, 403, "READONLY_FORBIDDEN", "Read-only role cannot perform this operation")
				return
			}
		}

		c.Next()
	}
}
