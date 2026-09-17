// Package supplier provides HTTP handlers for supplier-role operations.
package supplier

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ProxyHandler 供应商代理管理 handler。
// 所有操作强制按 context 中的 owner_id 过滤，供应商只能管理自己的代理。
type ProxyHandler struct {
	proxyRepo    service.ProxyRepository
	adminService service.AdminService
}

// NewProxyHandler creates a new supplier proxy handler.
func NewProxyHandler(proxyRepo service.ProxyRepository, adminService service.AdminService) *ProxyHandler {
	return &ProxyHandler{
		proxyRepo:    proxyRepo,
		adminService: adminService,
	}
}

// CreateProxyRequest 创建代理请求
type CreateProxyRequest struct {
	Name           string `json:"name" binding:"required"`
	Protocol       string `json:"protocol" binding:"required,oneof=http https socks5 socks5h"`
	Host           string `json:"host" binding:"required"`
	Port           int    `json:"port" binding:"required,min=1,max=65535"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	ExpiresAt      *int64 `json:"expires_at"`
	FallbackMode   string `json:"fallback_mode" binding:"omitempty,oneof=none proxy direct"`
	BackupProxyID  *int64 `json:"backup_proxy_id"`
	ExpiryWarnDays int    `json:"expiry_warn_days" binding:"omitempty,min=0"`
}

// UpdateProxyRequest 更新代理请求
type UpdateProxyRequest struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol" binding:"omitempty,oneof=http https socks5 socks5h"`
	Host     string `json:"host"`
	Port     int    `json:"port" binding:"omitempty,min=1,max=65535"`
	Username string `json:"username"`
	// Password 为 nil/缺省表示保持原密码不变（接口不再回传密码原文）；
	// 传空字符串表示清除密码。
	Password       *string `json:"password"`
	Status         string  `json:"status" binding:"omitempty,oneof=active inactive"`
	ExpiresAt      *int64  `json:"expires_at"`
	FallbackMode   string  `json:"fallback_mode" binding:"omitempty,oneof=none proxy direct"`
	BackupProxyID  *int64  `json:"backup_proxy_id"`
	ExpiryWarnDays int     `json:"expiry_warn_days" binding:"omitempty,min=0"`
}

// List 获取供应商的代理列表
// GET /api/v1/supplier/proxies
func (h *ProxyHandler) List(c *gin.Context) {
	ownerID, _, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	page, pageSize := response.ParsePagination(c)
	protocol := c.Query("protocol")
	status := c.Query("status")
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		search = search[:100]
	}

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "id"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}

	proxies, result, err := h.proxyRepo.ListByOwnerWithAccountCount(c.Request.Context(), params, ownerID, protocol, status, search)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.AdminProxyWithAccountCount, 0, len(proxies))
	for i := range proxies {
		out = append(out, *dto.ProxyWithAccountCountFromServiceAdmin(&proxies[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// GetAll 获取供应商的所有活跃代理（不分页）
// GET /api/v1/supplier/proxies/all
func (h *ProxyHandler) GetAll(c *gin.Context) {
	ownerID, _, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	proxies, err := h.proxyRepo.ListActiveByOwner(c.Request.Context(), ownerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.AdminProxy, 0, len(proxies))
	for i := range proxies {
		out = append(out, *dto.ProxyFromServiceAdmin(&proxies[i]))
	}
	response.Success(c, out)
}

// GetByID 获取供应商的单个代理
// GET /api/v1/supplier/proxies/:id
func (h *ProxyHandler) GetByID(c *gin.Context) {
	ownerID, _, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	proxy, err := h.proxyRepo.GetByIDAndOwner(c.Request.Context(), proxyID, ownerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.ProxyFromServiceAdmin(proxy))
}

// Create 创建代理
// POST /api/v1/supplier/proxies
func (h *ProxyHandler) Create(c *gin.Context) {
	ownerID, _, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	var req CreateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt > 0 {
		t := time.Unix(*req.ExpiresAt, 0).UTC()
		expiresAt = &t
	}

	// 规范化 fallback_mode
	mode := strings.TrimSpace(req.FallbackMode)
	if mode == "" {
		mode = service.FallbackModeNone
	}
	if mode == service.FallbackModeProxy && req.BackupProxyID == nil {
		response.BadRequest(c, "backup proxy required when fallback_mode=proxy")
		return
	}

	proxy := &service.Proxy{
		Name:           strings.TrimSpace(req.Name),
		Protocol:       strings.TrimSpace(req.Protocol),
		Host:           strings.TrimSpace(req.Host),
		Port:           req.Port,
		Username:       strings.TrimSpace(req.Username),
		Password:       strings.TrimSpace(req.Password),
		Status:         service.StatusActive,
		ExpiresAt:      expiresAt,
		FallbackMode:   mode,
		BackupProxyID:  req.BackupProxyID,
		ExpiryWarnDays: req.ExpiryWarnDays,
		OwnerID:        &ownerID,
	}

	if err := h.proxyRepo.Create(c.Request.Context(), proxy); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.ProxyFromServiceAdmin(proxy))
}

// Update 更新代理
// PUT /api/v1/supplier/proxies/:id
func (h *ProxyHandler) Update(c *gin.Context) {
	ownerID, _, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	// 先校验归属
	existing, err := h.proxyRepo.GetByIDAndOwner(c.Request.Context(), proxyID, ownerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	var req UpdateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt > 0 {
		t := time.Unix(*req.ExpiresAt, 0).UTC()
		expiresAt = &t
	}

	// 保留 OwnerID 不变
	existing.Name = strings.TrimSpace(req.Name)
	existing.Protocol = strings.TrimSpace(req.Protocol)
	existing.Host = strings.TrimSpace(req.Host)
	existing.Port = req.Port
	existing.Username = strings.TrimSpace(req.Username)
	if req.Password != nil {
		existing.Password = strings.TrimSpace(*req.Password)
	}
	if req.Status != "" {
		existing.Status = strings.TrimSpace(req.Status)
	}
	existing.ExpiresAt = expiresAt
	if req.FallbackMode != "" {
		existing.FallbackMode = strings.TrimSpace(req.FallbackMode)
	}
	existing.BackupProxyID = req.BackupProxyID
	if req.ExpiryWarnDays > 0 {
		existing.ExpiryWarnDays = req.ExpiryWarnDays
	}

	if err := h.proxyRepo.Update(c.Request.Context(), existing); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.ProxyFromServiceAdmin(existing))
}

// Delete 删除代理
// DELETE /api/v1/supplier/proxies/:id
func (h *ProxyHandler) Delete(c *gin.Context) {
	ownerID, _, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	// 先校验归属
	if _, err := h.proxyRepo.GetByIDAndOwner(c.Request.Context(), proxyID, ownerID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if err := h.proxyRepo.Delete(c.Request.Context(), proxyID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"success": true})
}

// Test 测试代理连接
// POST /api/v1/supplier/proxies/:id/test
func (h *ProxyHandler) Test(c *gin.Context) {
	ownerID, _, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	// 先校验归属
	if _, err := h.proxyRepo.GetByIDAndOwner(c.Request.Context(), proxyID, ownerID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result, err := h.adminService.TestProxy(c.Request.Context(), proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// CheckQuality 检查代理质量
// POST /api/v1/supplier/proxies/:id/quality-check
func (h *ProxyHandler) CheckQuality(c *gin.Context) {
	ownerID, _, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	// 先校验归属
	if _, err := h.proxyRepo.GetByIDAndOwner(c.Request.Context(), proxyID, ownerID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result, err := h.adminService.CheckProxyQuality(c.Request.Context(), proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// GetProxyAccounts 获取使用此代理的账号列表
// GET /api/v1/supplier/proxies/:id/accounts
func (h *ProxyHandler) GetProxyAccounts(c *gin.Context) {
	ownerID, _, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	proxyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid proxy ID")
		return
	}

	// 先校验归属
	if _, err := h.proxyRepo.GetByIDAndOwner(c.Request.Context(), proxyID, ownerID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	accounts, err := h.adminService.GetProxyAccounts(c.Request.Context(), proxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 过滤：只返回属于当前供应商的账号
	filtered := make([]service.ProxyAccountSummary, 0, len(accounts))
	for i := range accounts {
		if accounts[i].OwnerID != nil && *accounts[i].OwnerID == ownerID {
			filtered = append(filtered, accounts[i])
		}
	}

	response.Success(c, filtered)
}

// ensureNoContextCancel 防止 context 取消导致后台探测失败（占位，实际由 adminService 处理）
var _ = context.Background
