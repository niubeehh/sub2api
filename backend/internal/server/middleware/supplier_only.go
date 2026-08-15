package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SupplierOnly 供应商面板权限中间件。
// 必须在 JWTAuth 中间件之后使用。
//
// 放行 admin 与 supplier 角色：
//   - admin：不注入归属过滤，可查看/操作全部账号（管理面全局视角）
//   - supplier：注入 ContextKeyOwnerFilter = user.ID，handler 据此强制按 owner_id 过滤，
//     只能看到/操作自己名下的账号
//
// 其他角色（user）直接 403。
func SupplierOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRoleFromContext(c)
		if !ok {
			AbortWithError(c, 401, "UNAUTHORIZED", "User not found in context")
			return
		}

		switch role {
		case service.RoleAdmin:
			// admin 不注入 owner 过滤，可访问全部账号
		case service.RoleSupplier:
			// supplier 强制按自己的 user.ID 过滤账号
			subject, ok := GetAuthSubjectFromContext(c)
			if !ok || subject.UserID <= 0 {
				AbortWithError(c, 401, "UNAUTHORIZED", "Supplier identity not found in context")
				return
			}
			c.Set(string(ContextKeyOwnerFilter), subject.UserID)
		default:
			AbortWithError(c, 403, "FORBIDDEN", "Supplier or admin access required")
			return
		}

		c.Next()
	}
}

// GetOwnerFilterFromContext 读取供应商归属过滤约束。
// 返回 (ownerID, true) 表示当前请求需按 owner_id 过滤（supplier 视角）；
// 返回 (_, false) 表示无过滤约束（admin 视角，可看全部）。
func GetOwnerFilterFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(string(ContextKeyOwnerFilter))
	if !exists {
		return 0, false
	}
	ownerID, ok := value.(int64)
	if !ok || ownerID <= 0 {
		return 0, false
	}
	return ownerID, true
}
