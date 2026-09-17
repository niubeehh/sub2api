package middleware

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// readOnlyGuardAllowedPaths 是只读角色也允许访问的 POST 端点白名单：
//   - 合规确认：AdminComplianceGuard 对未确认用户返回 423，而确认动作本身是 POST；
//     不放行会让 readonly 永远无法进入管理面（死锁）
//   - 批量用量查询：用户/API Key 列表页加载时的查询型 POST（只读聚合，无数据变更）
var readOnlyGuardAllowedPaths = map[string]bool{
	"/api/v1/admin/compliance/accept":        true,
	"/api/v1/admin/dashboard/users-usage":    true,
	"/api/v1/admin/dashboard/api-keys-usage": true,
}

// ReadOnlyGuard 只读角色写操作拦截中间件。
// 必须在认证中间件（adminAuth）之后使用。
//
// readonly 角色可查看管理面全部数据；除 GET/HEAD/OPTIONS 外的请求方法
// （POST/PUT/DELETE/PATCH 等变更类操作）一律 403，仅白名单内的
// 准入类/查询类 POST 端点除外。
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
			case "POST":
				if !readOnlyGuardAllowedPaths[strings.TrimSuffix(c.Request.URL.Path, "/")] {
					AbortWithError(c, 403, "READONLY_FORBIDDEN", "Read-only role cannot perform this operation")
					return
				}
			default:
				AbortWithError(c, 403, "READONLY_FORBIDDEN", "Read-only role cannot perform this operation")
				return
			}
		}

		c.Next()
	}
}
