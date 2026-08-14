package supplier

import (
	"log"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ==================== 阶段二：写操作 ====================

// CreateAccountRequest 供应商创建账号请求（不含 owner_id，owner_id 由 context 强制注入）。
type CreateAccountRequest struct {
	Name                    string         `json:"name" binding:"required"`
	Notes                   *string        `json:"notes"`
	Platform                string         `json:"platform" binding:"required"`
	Type                    string         `json:"type" binding:"required,oneof=oauth setup-token apikey upstream bedrock service_account"`
	Credentials             map[string]any `json:"credentials" binding:"required"`
	Extra                   map[string]any `json:"extra"`
	ProxyID                 *int64         `json:"proxy_id"`
	Concurrency             int            `json:"concurrency"`
	Priority                int            `json:"priority"`
	RateMultiplier          *float64       `json:"rate_multiplier"`
	LoadFactor              *int           `json:"load_factor"`
	GroupIDs                []int64        `json:"group_ids"`
	ExpiresAt               *int64         `json:"expires_at"`
	AutoPauseOnExpired      *bool          `json:"auto_pause_on_expired"`
	ProbeEnabled            *bool          `json:"upstream_billing_probe_enabled"`
	ConfirmMixedChannelRisk *bool          `json:"confirm_mixed_channel_risk"`
}

// UpdateAccountRequest 供应商编辑账号请求（不含 owner_id，供应商不能改归属）。
type UpdateAccountRequest struct {
	Name                    string         `json:"name"`
	Notes                   *string        `json:"notes"`
	Type                    string         `json:"type" binding:"omitempty,oneof=oauth setup-token apikey upstream bedrock service_account"`
	Credentials             map[string]any `json:"credentials"`
	Extra                   map[string]any `json:"extra"`
	ProxyID                 *int64         `json:"proxy_id"`
	Concurrency             *int           `json:"concurrency"`
	Priority                *int           `json:"priority"`
	RateMultiplier          *float64       `json:"rate_multiplier"`
	LoadFactor              *int           `json:"load_factor"`
	Status                  string         `json:"status" binding:"omitempty,oneof=active inactive error"`
	GroupIDs                *[]int64       `json:"group_ids"`
	ExpiresAt               *int64         `json:"expires_at"`
	AutoPauseOnExpired      *bool          `json:"auto_pause_on_expired"`
	ProbeEnabled            *bool          `json:"upstream_billing_probe_enabled"`
	RateSyncEnabled         *bool          `json:"upstream_billing_rate_sync_enabled"`
	ConfirmMixedChannelRisk *bool          `json:"confirm_mixed_channel_risk"`
}

// TestAccountRequest 测试账号请求。
type TestAccountRequest struct {
	ModelID      string `json:"model_id"`
	Prompt       string `json:"prompt"`
	Mode         string `json:"mode"`
	ImageDataURL string `json:"image_data_url"`
	AudioDataURL string `json:"audio_data_url"`
}

// currentOwnerID 从 context 获取当前供应商的 owner_id。
// supplier 视角返回自己的 user.ID；admin 视角返回 0（admin 创建的账号为平台托管）。
func currentOwnerID(c *gin.Context) int64 {
	ownerID, _ := middleware.GetOwnerFilterFromContext(c)
	return ownerID
}

// Create 创建账号（供应商强制 owner_id = 自己）。
// POST /api/v1/supplier/accounts
func (h *AccountHandler) Create(c *gin.Context) {
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := service.ValidateOpenAILongContextBillingExtra(req.Platform, req.Extra); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}

	skipCheck := req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk

	// 强制 owner_id：supplier 用自己的 user.ID，admin 创建的为平台托管（nil）
	var ownerID *int64
	if oid := currentOwnerID(c); oid > 0 {
		ownerID = &oid
	}

	// 校验 proxy_id 归属：供应商只能绑定自己的代理或平台托管代理（owner_id IS NULL）
	if req.ProxyID != nil && *req.ProxyID > 0 && ownerID != nil {
		proxy, err := h.proxyRepo.GetByID(c.Request.Context(), *req.ProxyID)
		if err != nil {
			response.BadRequest(c, "proxy not found")
			return
		}
		if proxy.OwnerID != nil && *proxy.OwnerID != *ownerID {
			response.BadRequest(c, "proxy does not belong to current supplier")
			return
		}
	}

	account, err := h.adminService.CreateAccount(c.Request.Context(), &service.CreateAccountInput{
		Name:                  req.Name,
		Notes:                 req.Notes,
		Platform:              req.Platform,
		Type:                  req.Type,
		Credentials:           req.Credentials,
		Extra:                 req.Extra,
		ProxyID:               req.ProxyID,
		Concurrency:           req.Concurrency,
		Priority:              req.Priority,
		RateMultiplier:        req.RateMultiplier,
		LoadFactor:            req.LoadFactor,
		GroupIDs:              req.GroupIDs,
		ExpiresAt:             req.ExpiresAt,
		AutoPauseOnExpired:    req.AutoPauseOnExpired,
		ProbeEnabled:          req.ProbeEnabled,
		SkipMixedChannelCheck: skipCheck,
		OwnerID:               ownerID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// OAuth 账号创建后设置隐私
	h.adminService.ForceAntigravityPrivacy(c.Request.Context(), account)
	h.adminService.ForceOpenAIPrivacy(c.Request.Context(), account)

	response.Success(c, dto.AccountFromService(account))
}

// Update 编辑账号（先校验归属）。
// PUT /api/v1/supplier/accounts/:id
func (h *AccountHandler) Update(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Abort404(c, "Invalid account ID")
		return
	}
	if _, ok := h.checkAccountOwnership(c, accountID); !ok {
		return
	}

	var req UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}

	skipCheck := req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk

	// 供应商不能修改 owner_id（不传 OwnerID 字段）
	account, err := h.adminService.UpdateAccount(c.Request.Context(), accountID, &service.UpdateAccountInput{
		Name:                  req.Name,
		Notes:                 req.Notes,
		Type:                  req.Type,
		Credentials:           req.Credentials,
		Extra:                 req.Extra,
		ProxyID:               req.ProxyID,
		Concurrency:           req.Concurrency,
		Priority:              req.Priority,
		RateMultiplier:        req.RateMultiplier,
		LoadFactor:            req.LoadFactor,
		Status:                req.Status,
		GroupIDs:              req.GroupIDs,
		ExpiresAt:             req.ExpiresAt,
		AutoPauseOnExpired:    req.AutoPauseOnExpired,
		ProbeEnabled:          req.ProbeEnabled,
		RateSyncEnabled:       req.RateSyncEnabled,
		SkipMixedChannelCheck: skipCheck,
		// OwnerID 不传：供应商不能改归属
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.AccountFromService(account))
}

// Delete 删除账号（先校验归属）。
// DELETE /api/v1/supplier/accounts/:id
func (h *AccountHandler) Delete(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Abort404(c, "Invalid account ID")
		return
	}
	if _, ok := h.checkAccountOwnership(c, accountID); !ok {
		return
	}

	if err := h.adminService.DeleteAccount(c.Request.Context(), accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Account deleted successfully"})
}

// Test 测试账号连接（先校验归属）。
// POST /api/v1/supplier/accounts/:id/test
func (h *AccountHandler) Test(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Abort404(c, "Invalid account ID")
		return
	}
	if _, ok := h.checkAccountOwnership(c, accountID); !ok {
		return
	}

	var req TestAccountRequest
	_ = c.ShouldBindJSON(&req)

	opts := service.AccountTestOptions{
		ImageDataURL: req.ImageDataURL,
		AudioDataURL: req.AudioDataURL,
	}

	if err := h.accountTestService.TestAccountConnection(c, accountID, req.ModelID, req.Prompt, req.Mode, opts); err != nil {
		return
	}

	if h.rateLimitService != nil {
		if _, err := h.rateLimitService.RecoverAccountAfterSuccessfulTest(c.Request.Context(), accountID); err != nil {
			_ = c.Error(err)
		}
	}
}

// Refresh 刷新账号凭据（先校验归属）。
// POST /api/v1/supplier/accounts/:id/refresh
func (h *AccountHandler) Refresh(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Abort404(c, "Invalid account ID")
		return
	}
	account, ok := h.checkAccountOwnership(c, accountID)
	if !ok {
		return
	}

	updatedAccount, err := h.adminService.RefreshAccountCredentials(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 清除 token 缓存
	if h.tokenCacheInvalidator != nil && account.IsOAuth() {
		if invalidateErr := h.tokenCacheInvalidator.InvalidateToken(c.Request.Context(), updatedAccount); invalidateErr != nil {
			log.Printf("[WARN] Failed to invalidate token cache for account %d: %v", accountID, invalidateErr)
		}
	}

	response.Success(c, dto.AccountFromService(updatedAccount))
}

// ClearError 清除账号错误状态（先校验归属）。
// POST /api/v1/supplier/accounts/:id/clear-error
func (h *AccountHandler) ClearError(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Abort404(c, "Invalid account ID")
		return
	}
	if _, ok := h.checkAccountOwnership(c, accountID); !ok {
		return
	}

	account, err := h.adminService.ClearAccountError(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if h.tokenCacheInvalidator != nil && account.IsOAuth() {
		if invalidateErr := h.tokenCacheInvalidator.InvalidateToken(c.Request.Context(), account); invalidateErr != nil {
			log.Printf("[WARN] Failed to invalidate token cache for account %d: %v", accountID, invalidateErr)
		}
	}

	response.Success(c, dto.AccountFromService(account))
}

// ClearRateLimit 清除账号限流状态（先校验归属）。
// POST /api/v1/supplier/accounts/:id/clear-rate-limit
func (h *AccountHandler) ClearRateLimit(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Abort404(c, "Invalid account ID")
		return
	}
	if _, ok := h.checkAccountOwnership(c, accountID); !ok {
		return
	}

	if err := h.rateLimitService.ClearRateLimit(c.Request.Context(), accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountFromService(account))
}

// ImportCodexSession 导入 Codex session（强制 owner_id = 当前供应商）。
// 委托给 admin AccountHandler.ImportCodexSessionForOwner，复用全部解析/去重/创建逻辑。
// POST /api/v1/supplier/accounts/import/codex-session
func (h *AccountHandler) ImportCodexSession(c *gin.Context) {
	var ownerID *int64
	if oid := currentOwnerID(c); oid > 0 {
		ownerID = &oid
	}
	h.adminAccountHandler.ImportCodexSessionForOwner(c, ownerID)
}

// ==================== OAuth Handler ====================

// OAuthHandler 供应商 OAuth handler，复用 OAuthService。
// OAuth 流程只换 token 不建账号，不涉及 owner 归属，直接转发。
type OAuthHandler struct {
	oauthService *service.OAuthService
}

// NewOAuthHandler creates a new supplier OAuth handler.
func NewOAuthHandler(oauthService *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{oauthService: oauthService}
}

// GenerateAuthURL 生成 OAuth 授权 URL。
// POST /api/v1/supplier/accounts/generate-auth-url
func (h *OAuthHandler) GenerateAuthURL(c *gin.Context) {
	var req struct {
		ProxyID *int64 `json:"proxy_id"`
	}
	_ = c.ShouldBindJSON(&req)

	result, err := h.oauthService.GenerateAuthURL(c.Request.Context(), req.ProxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// GenerateSetupTokenURL 生成 setup token 授权 URL。
// POST /api/v1/supplier/accounts/generate-setup-token-url
func (h *OAuthHandler) GenerateSetupTokenURL(c *gin.Context) {
	var req struct {
		ProxyID *int64 `json:"proxy_id"`
	}
	_ = c.ShouldBindJSON(&req)

	result, err := h.oauthService.GenerateSetupTokenURL(c.Request.Context(), req.ProxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ExchangeCodeRequest OAuth code 交换请求。
type ExchangeCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Code      string `json:"code" binding:"required"`
	ProxyID   *int64 `json:"proxy_id"`
}

// ExchangeCode 交换授权码。
// POST /api/v1/supplier/accounts/exchange-code
func (h *OAuthHandler) ExchangeCode(c *gin.Context) {
	var req ExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.ExchangeCode(c.Request.Context(), &service.ExchangeCodeInput{
		SessionID: req.SessionID,
		Code:      req.Code,
		ProxyID:   req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

// ExchangeSetupTokenCode 交换 setup token 授权码。
// POST /api/v1/supplier/accounts/exchange-setup-token-code
func (h *OAuthHandler) ExchangeSetupTokenCode(c *gin.Context) {
	var req ExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.ExchangeCode(c.Request.Context(), &service.ExchangeCodeInput{
		SessionID: req.SessionID,
		Code:      req.Code,
		ProxyID:   req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

// CookieAuthRequest cookie 认证请求。
type CookieAuthRequest struct {
	SessionKey string `json:"code" binding:"required"`
	ProxyID    *int64 `json:"proxy_id"`
}

// CookieAuth cookie 认证。
// POST /api/v1/supplier/accounts/cookie-auth
func (h *OAuthHandler) CookieAuth(c *gin.Context) {
	var req CookieAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.CookieAuth(c.Request.Context(), &service.CookieAuthInput{
		SessionKey: req.SessionKey,
		ProxyID:    req.ProxyID,
		Scope:      "full",
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

// SetupTokenCookieAuth setup token cookie 认证。
// POST /api/v1/supplier/accounts/setup-token-cookie-auth
func (h *OAuthHandler) SetupTokenCookieAuth(c *gin.Context) {
	var req CookieAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.CookieAuth(c.Request.Context(), &service.CookieAuthInput{
		SessionKey: req.SessionKey,
		ProxyID:    req.ProxyID,
		Scope:      "inference",
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tokenInfo)
}

// 确保 admin 包被引用
var _ = admin.NewOAuthHandler
