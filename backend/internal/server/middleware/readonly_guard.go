package middleware

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// readOnlyGuardAllowedPaths 是只读角色也允许访问的 POST 端点白名单。
// 准入标准：请求体仅携带 ID 列表/过滤条件、响应为查询结果、无任何数据变更。
//   - 合规确认：AdminComplianceGuard 对未确认用户返回 423，而确认动作本身是 POST；
//     不放行会让 readonly 永远无法进入管理面（死锁）
//   - 批量用量/统计/属性查询：各列表页加载表格聚合列时的查询型 POST
var readOnlyGuardAllowedPaths = map[string]bool{
	"/api/v1/admin/compliance/accept":          true,
	"/api/v1/admin/dashboard/users-usage":      true,
	"/api/v1/admin/dashboard/api-keys-usage":   true,
	"/api/v1/admin/accounts/usage/batch":       true,
	"/api/v1/admin/accounts/today-stats/batch": true,
	"/api/v1/admin/user-attributes/batch":      true,
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
