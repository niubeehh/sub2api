// Package routes provides HTTP route registration and handlers.
package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterSupplierRoutes 注册供应商面板路由。
//
// 鉴权链：JWTAuth（解析 JWT 写入 user/role）→ SupplierOnly（放行 admin/supplier，
// supplier 注入 owner_id 过滤约束）→ AuditLog。
//
// 所有 handler 内部强制按 context 中的 owner_id 过滤，供应商只能看到/操作自己名下的账号。
func RegisterSupplierRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	jwtAuth middleware.JWTAuthMiddleware,
	auditLog middleware.AuditLogMiddleware,
) {
	supplier := v1.Group("/supplier")
	supplier.Use(gin.HandlerFunc(jwtAuth))
	supplier.Use(middleware.SupplierOnly())
	supplier.Use(gin.HandlerFunc(auditLog))
	{
		// 仪表盘：整体消耗汇总
		supplier.GET("/dashboard", h.Supplier.Account.GetDashboard)
		// 仪表盘图表：模型分布、趋势快照
		supplier.GET("/dashboard/models", h.Supplier.Account.GetModelStats)
		supplier.GET("/dashboard/snapshot-v2", h.Supplier.Account.GetSnapshotV2)

		// 账号管理
		accounts := supplier.Group("/accounts")
		{
			// 只读端点（阶段一）
			accounts.GET("", h.Supplier.Account.List)
			accounts.GET("/:id", h.Supplier.Account.GetByID)
			accounts.GET("/:id/stats", h.Supplier.Account.GetStats)
			accounts.GET("/:id/usage", h.Supplier.Account.GetUsage)
			accounts.GET("/:id/today-stats", h.Supplier.Account.GetTodayStats)

			// 写操作端点（阶段二）
			accounts.POST("", h.Supplier.Account.Create)
			accounts.PUT("/:id", h.Supplier.Account.Update)
			accounts.DELETE("/:id", h.Supplier.Account.Delete)

			// 账号操作端点（阶段二）
			accounts.POST("/:id/test", h.Supplier.Account.Test)
			accounts.POST("/:id/refresh", h.Supplier.Account.Refresh)
			accounts.POST("/:id/clear-error", h.Supplier.Account.ClearError)
			accounts.POST("/:id/clear-rate-limit", h.Supplier.Account.ClearRateLimit)

			// OAuth 流程端点（阶段二）——只换 token 不建账号
			accounts.POST("/generate-auth-url", h.Supplier.OAuth.GenerateAuthURL)
			accounts.POST("/generate-setup-token-url", h.Supplier.OAuth.GenerateSetupTokenURL)
			accounts.POST("/exchange-code", h.Supplier.OAuth.ExchangeCode)
			accounts.POST("/exchange-setup-token-code", h.Supplier.OAuth.ExchangeSetupTokenCode)
			accounts.POST("/cookie-auth", h.Supplier.OAuth.CookieAuth)
			accounts.POST("/setup-token-cookie-auth", h.Supplier.OAuth.SetupTokenCookieAuth)

			// Codex session 导入（阶段二）
			accounts.POST("/import/codex-session", h.Supplier.Account.ImportCodexSession)

			// 批量查询（阶段三）
			accounts.POST("/usage/batch", h.Supplier.Account.GetBatchUsage)
			accounts.POST("/today-stats/batch", h.Supplier.Account.GetBatchTodayStats)
		}

		// 用量明细列表
		supplier.GET("/usage", h.Supplier.Account.ListUsage)
		supplier.GET("/usage/stats", h.Supplier.Account.GetUsageStats)

		// 代理管理（阶段三）
		proxies := supplier.Group("/proxies")
		{
			proxies.GET("", h.Supplier.Proxy.List)
			proxies.GET("/all", h.Supplier.Proxy.GetAll)
			proxies.GET("/:id", h.Supplier.Proxy.GetByID)
			proxies.POST("", h.Supplier.Proxy.Create)
			proxies.PUT("/:id", h.Supplier.Proxy.Update)
			proxies.DELETE("/:id", h.Supplier.Proxy.Delete)
			proxies.POST("/:id/test", h.Supplier.Proxy.Test)
			proxies.POST("/:id/quality-check", h.Supplier.Proxy.CheckQuality)
			proxies.GET("/:id/accounts", h.Supplier.Proxy.GetProxyAccounts)
		}

		// 分组管理（只读，用于创建/编辑账号时选择分组）
		supplier.GET("/groups/all", h.Supplier.Group.GetAll)
	}
}
