// Package supplier provides HTTP handlers for supplier-role operations.
//
// 供应商面板 handler：复用底层 service（AdminService、AccountUsageService、UsageService、
// OAuthService、RateLimitService、AccountTestService），但所有查询和操作强制按 context
// 中的 owner_id 过滤，只能看到/操作自己名下的账号。
package supplier

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"

	"github.com/gin-gonic/gin"
)

// AccountHandler handles supplier account operations.
type AccountHandler struct {
	adminService          service.AdminService
	accountUsageService   *service.AccountUsageService
	usageService          *service.UsageService
	dashboardService      *service.DashboardService
	rateLimitService      *service.RateLimitService
	accountTestService    *service.AccountTestService
	tokenCacheInvalidator service.TokenCacheInvalidator
	adminAccountHandler   *admin.AccountHandler
	proxyRepo             service.ProxyRepository
}

// NewAccountHandler creates a new supplier account handler.
func NewAccountHandler(
	adminService service.AdminService,
	accountUsageService *service.AccountUsageService,
	usageService *service.UsageService,
	dashboardService *service.DashboardService,
	rateLimitService *service.RateLimitService,
	accountTestService *service.AccountTestService,
	tokenCacheInvalidator service.TokenCacheInvalidator,
	adminAccountHandler *admin.AccountHandler,
	proxyRepo service.ProxyRepository,
) *AccountHandler {
	return &AccountHandler{
		adminService:          adminService,
		accountUsageService:   accountUsageService,
		usageService:          usageService,
		dashboardService:      dashboardService,
		rateLimitService:      rateLimitService,
		accountTestService:    accountTestService,
		tokenCacheInvalidator: tokenCacheInvalidator,
		adminAccountHandler:   adminAccountHandler,
		proxyRepo:             proxyRepo,
	}
}

// requireOwnerFilter 从 context 读取供应商归属过滤约束。
// admin 视角返回 (0, false) 表示看全部；supplier 视角返回 (ownerID, true)。
// 若 context 无认证信息则直接写错误响应并返回 ok=false。
func requireOwnerFilter(c *gin.Context) (int64, bool, bool) {
	ownerID, filtered := middleware.GetOwnerFilterFromContext(c)
	if !filtered {
		// admin 或未注入过滤：检查是否已认证
		if _, ok := middleware.GetAuthSubjectFromContext(c); !ok {
			Abort401(c, "User not found in context")
			return 0, false, false
		}
	}
	return ownerID, filtered, true
}

// Abort401 writes a 401 error response.
func Abort401(c *gin.Context, msg string) {
	response.Error(c, 401, msg)
}

// Abort403 writes a 403 error response.
func Abort403(c *gin.Context, msg string) {
	response.Error(c, 403, msg)
}

// Abort404 writes a 404 error response（账号不存在或不归属当前供应商时统一返回 404，避免枚举）。
func Abort404(c *gin.Context, msg string) {
	response.Error(c, 404, msg)
}

// checkAccountOwnership 校验账号是否归属当前供应商。
// admin 视角直接放行（看全部）；supplier 视角要求 account.OwnerID == ownerID。
// 返回 (account, true) 表示校验通过，false 表示已写错误响应。
func (h *AccountHandler) checkAccountOwnership(c *gin.Context, accountID int64) (*service.Account, bool) {
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil || account == nil {
		Abort404(c, "Account not found")
		return nil, false
	}
	ownerID, filtered := middleware.GetOwnerFilterFromContext(c)
	if filtered {
		if account.OwnerID == nil || *account.OwnerID != ownerID {
			Abort404(c, "Account not found")
			return nil, false
		}
	}
	return account, true
}

// List returns accounts owned by the current supplier (or all for admin).
// GET /api/v1/supplier/accounts
func (h *AccountHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	platform := c.Query("platform")
	accountType := c.Query("type")
	status := c.Query("status")
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		search = search[:100]
	}
	privacyMode := strings.TrimSpace(c.Query("privacy_mode"))
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")

	var groupID int64
	if groupIDStr := c.Query("group"); groupIDStr != "" {
		if groupIDStr == "ungrouped" {
			groupID = service.AccountListGroupUngrouped
		} else if parsed, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil && parsed > 0 {
			groupID = parsed
		}
	}

	ownerID, filtered, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	var accounts []service.Account
	var total int64
	var err error
	if filtered {
		accounts, total, err = h.adminService.ListAccountsByOwner(c.Request.Context(), ownerID, page, pageSize, platform, accountType, status, search, groupID, privacyMode, sortBy, sortOrder)
	} else {
		accounts, total, err = h.adminService.ListAccounts(c.Request.Context(), page, pageSize, platform, accountType, status, search, groupID, privacyMode, sortBy, sortOrder)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 转换为 DTO（精简版：不带调度分数/并发等运行态）
	items := make([]*dto.Account, 0, len(accounts))
	for i := range accounts {
		items = append(items, dto.AccountFromService(&accounts[i]))
	}

	response.Success(c, gin.H{
		"items": items,
		"total": total,
		"page":  page,
		"page_size": pageSize,
	})
}

// GetByID returns a single account owned by the current supplier.
// GET /api/v1/supplier/accounts/:id
func (h *AccountHandler) GetByID(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Abort404(c, "Invalid account ID")
		return
	}
	account, ok := h.checkAccountOwnership(c, accountID)
	if !ok {
		return
	}
	response.Success(c, dto.AccountFromService(account))
}

// GetStats returns usage stats for a single account.
// GET /api/v1/supplier/accounts/:id/stats
func (h *AccountHandler) GetStats(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Abort404(c, "Invalid account ID")
		return
	}
	if _, ok := h.checkAccountOwnership(c, accountID); !ok {
		return
	}
	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 90 {
			days = d
		}
	}
	now := timezone.Now()
	endTime := timezone.StartOfDay(now.AddDate(0, 0, 1))
	startTime := timezone.StartOfDay(now.AddDate(0, 0, -days+1))

	stats, err := h.accountUsageService.GetAccountUsageStats(c.Request.Context(), accountID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

// GetUsage returns usage info for a single account.
// GET /api/v1/supplier/accounts/:id/usage
func (h *AccountHandler) GetUsage(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Abort404(c, "Invalid account ID")
		return
	}
	if _, ok := h.checkAccountOwnership(c, accountID); !ok {
		return
	}
	source := c.DefaultQuery("source", "active")
	force := c.Query("force") == "true"

	var usage *service.UsageInfo
	if source == "passive" {
		usage, err = h.accountUsageService.GetPassiveUsage(c.Request.Context(), accountID)
	} else {
		usage, err = h.accountUsageService.GetUsage(c.Request.Context(), accountID, force)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, usage)
}

// GetTodayStats returns today's stats for a single account.
// GET /api/v1/supplier/accounts/:id/today-stats
func (h *AccountHandler) GetTodayStats(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		Abort404(c, "Invalid account ID")
		return
	}
	if _, ok := h.checkAccountOwnership(c, accountID); !ok {
		return
	}
	stats, err := h.accountUsageService.GetTodayStats(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

// ListUsage returns usage logs for the current supplier's accounts.
// GET /api/v1/supplier/usage
func (h *AccountHandler) ListUsage(c *gin.Context) {
	ownerID, filtered, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	page, pageSize := response.ParsePagination(c)
	filters := usagestats.UsageLogFilters{}
	if filtered {
		// 使用 account_owner_id 冗余字段直接过滤，O(1) 索引查询，无需先查 account_ids
		filters.AccountOwnerID = ownerID
	}
	// 可选：按单个 account_id 进一步过滤
	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if aid, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil && aid > 0 {
			filters.AccountID = aid
		}
	}
	if model := c.Query("model"); model != "" {
		filters.Model = model
	}
	if billingMode := c.Query("billing_mode"); billingMode != "" {
		filters.BillingMode = billingMode
	}
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		if rt, err := service.ParseUsageRequestType(requestTypeStr); err == nil {
			v := int16(rt)
			filters.RequestType = &v
		}
	}

	// 日期范围过滤
	userTZ := c.Query("timezone")
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ); err == nil {
			filters.StartTime = &t
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ); err == nil {
			t = t.AddDate(0, 0, 1) // half-open [start, end)
			filters.EndTime = &t
		}
	}

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}
	logs, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 转换为 DTO，确保 JSON 字段名与 admin usage 接口一致
	items := make([]*dto.AdminUsageLog, 0, len(logs))
	for i := range logs {
		items = append(items, dto.UsageLogFromServiceAdmin(&logs[i]))
	}

	response.Success(c, gin.H{
		"items": items,
		"total": result.Total,
		"page":  page,
		"page_size": pageSize,
	})
}

// GetUsageStats returns aggregated usage stats for the current supplier's accounts.
// GET /api/v1/supplier/usage/stats
func (h *AccountHandler) GetUsageStats(c *gin.Context) {
	ownerID, filtered, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	filters := usagestats.UsageLogFilters{}
	if filtered {
		filters.AccountOwnerID = ownerID
	}
	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if aid, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil && aid > 0 {
			filters.AccountID = aid
		}
	}
	if model := c.Query("model"); model != "" {
		filters.Model = model
	}

	// 日期范围过滤
	userTZ := c.Query("timezone")
	if startDateStr := c.Query("start_date"); startDateStr != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", startDateStr, userTZ); err == nil {
			filters.StartTime = &t
		}
	}
	if endDateStr := c.Query("end_date"); endDateStr != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", endDateStr, userTZ); err == nil {
			t = t.AddDate(0, 0, 1)
			filters.EndTime = &t
		}
	}

	stats, err := h.usageService.GetStatsWithFilters(c.Request.Context(), filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

// GetDashboard returns aggregated usage stats for the current supplier's accounts.
// GET /api/v1/supplier/dashboard
func (h *AccountHandler) GetDashboard(c *gin.Context) {
	ownerID, filtered, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 90 {
			days = d
		}
	}
	now := timezone.Now()
	endTime := timezone.StartOfDay(now.AddDate(0, 0, 1))
	startTime := timezone.StartOfDay(now.AddDate(0, 0, -days+1))

	filters := usagestats.UsageLogFilters{
		StartTime: &startTime,
		EndTime:   &endTime,
	}
	if filtered {
		// 使用 account_owner_id 冗余字段直接过滤
		filters.AccountOwnerID = ownerID
	}
	stats, err := h.usageService.GetStatsWithFilters(c.Request.Context(), filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 账号数量仍需查 accounts 表
	accountCount := 0
	if filtered {
		accounts, err := h.adminService.ListAllAccountsByOwner(c.Request.Context(), ownerID, "", "", "", "", 0, "")
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		accountCount = len(accounts)
	}

	response.Success(c, gin.H{
		"total_requests":            stats.TotalRequests,
		"total_cost":                stats.TotalCost,
		"total_actual_cost":         stats.TotalActualCost,
		"total_input_tokens":        stats.TotalInputTokens,
		"total_output_tokens":       stats.TotalOutputTokens,
		"total_tokens":              stats.TotalTokens,
		"average_duration_ms":       stats.AverageDurationMs,
		"account_count":             accountCount,
		"start_time":                startTime,
		"end_time":                  endTime,
	})
}

// BatchUsageRequest 批量查询账号用量请求
type BatchUsageRequest struct {
	AccountIDs []int64 `json:"account_ids" binding:"required"`
	Force      bool    `json:"force"`
}

// GetBatchUsage 批量查询账号用量（Dashboard 展示每个账号的消耗明细）
// POST /api/v1/supplier/accounts/usage/batch
func (h *AccountHandler) GetBatchUsage(c *gin.Context) {
	ownerID, filtered, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	var req BatchUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	accountIDs := normalizeInt64IDList(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.Success(c, gin.H{
			"usage":  map[string]any{},
			"errors": map[string]string{},
		})
		return
	}

	// 供应商视角：校验所有账号归属
	if filtered {
		ownedAccounts, err := h.adminService.ListAllAccountsByOwner(c.Request.Context(), ownerID, "", "", "", "", 0, "")
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		ownedSet := make(map[int64]bool, len(ownedAccounts))
		for i := range ownedAccounts {
			ownedSet[ownedAccounts[i].ID] = true
		}
		filtered := accountIDs[:0]
		for _, id := range accountIDs {
			if ownedSet[id] {
				filtered = append(filtered, id)
			}
		}
		accountIDs = filtered
		if len(accountIDs) == 0 {
			response.Success(c, gin.H{
				"usage":  map[string]any{},
				"errors": map[string]string{},
			})
			return
		}
	}

	usageByAccount, errorsByAccount, err := h.accountUsageService.GetUsageBatch(c.Request.Context(), accountIDs, req.Force)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"usage":  usageByAccount,
		"errors": errorsByAccount,
	})
}

// GetBatchTodayStats 批量查询账号今日统计
// POST /api/v1/supplier/accounts/today-stats/batch
func (h *AccountHandler) GetBatchTodayStats(c *gin.Context) {
	ownerID, filtered, ok := requireOwnerFilter(c)
	if !ok {
		return
	}

	var req BatchUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	accountIDs := normalizeInt64IDList(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.Success(c, map[string]any{})
		return
	}

	// 供应商视角：校验所有账号归属
	if filtered {
		ownedAccounts, err := h.adminService.ListAllAccountsByOwner(c.Request.Context(), ownerID, "", "", "", "", 0, "")
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		ownedSet := make(map[int64]bool, len(ownedAccounts))
		for i := range ownedAccounts {
			ownedSet[ownedAccounts[i].ID] = true
		}
		filtered := accountIDs[:0]
		for _, id := range accountIDs {
			if ownedSet[id] {
				filtered = append(filtered, id)
			}
		}
		accountIDs = filtered
		if len(accountIDs) == 0 {
			response.Success(c, map[string]any{})
			return
		}
	}

	stats, err := h.accountUsageService.GetTodayStatsBatch(c.Request.Context(), accountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// normalizeInt64IDList 去重 + 去零
func normalizeInt64IDList(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// ==================== Dashboard 图表端点（供应商版） ====================

// parseSupplierTimeRange 解析日期范围参数，返回 [startTime, endTime)。
func parseSupplierTimeRange(c *gin.Context) (time.Time, time.Time) {
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var startTime, endTime time.Time

	if startDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", startDate, userTZ); err == nil {
			startTime = t
		} else {
			startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
		}
	} else {
		startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
	}

	if endDate != "" {
		if t, err := timezone.ParseInUserLocation("2006-01-02", endDate, userTZ); err == nil {
			endTime = t.Add(24 * time.Hour)
		} else {
			endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
		}
	} else {
		endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	}

	return startTime, endTime
}

// parseSupplierBoolQuery 解析可选 bool 参数。
func parseSupplierBoolQuery(raw string, defaultVal bool) bool {
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return defaultVal
	}
	return v
}

// buildSupplierUsageFilters 从请求参数构建 UsageLogFilters，强制按 owner_id 过滤。
func buildSupplierUsageFilters(c *gin.Context, startTime, endTime time.Time) (usagestats.UsageLogFilters, bool) {
	ownerID, filtered, ok := requireOwnerFilter(c)
	if !ok {
		return usagestats.UsageLogFilters{}, false
	}

	filters := usagestats.UsageLogFilters{
		StartTime: &startTime,
		EndTime:   &endTime,
	}
	if filtered {
		filters.AccountOwnerID = ownerID
	}

	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if aid, err := strconv.ParseInt(accountIDStr, 10, 64); err == nil && aid > 0 {
			filters.AccountID = aid
		}
	}
	if model := c.Query("model"); model != "" {
		filters.Model = model
	}
	if billingMode := c.Query("billing_mode"); billingMode != "" {
		filters.BillingMode = billingMode
	}
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		if rt, err := service.ParseUsageRequestType(requestTypeStr); err == nil {
			v := int16(rt)
			filters.RequestType = &v
		}
	}
	if billingTypeStr := c.Query("billing_type"); billingTypeStr != "" {
		if v, err := strconv.ParseInt(billingTypeStr, 10, 8); err == nil {
			bt := int8(v)
			filters.BillingType = &bt
		}
	}

	return filters, true
}

// GetModelStats handles getting model usage statistics for the current supplier.
// GET /api/v1/supplier/dashboard/models
func (h *AccountHandler) GetModelStats(c *gin.Context) {
	startTime, endTime := parseSupplierTimeRange(c)

	filters, ok := buildSupplierUsageFilters(c, startTime, endTime)
	if !ok {
		return
	}

	modelSource := usagestats.ModelSourceRequested
	if rawModelSource := strings.TrimSpace(c.Query("model_source")); rawModelSource != "" {
		if !usagestats.IsValidModelSource(rawModelSource) {
			response.BadRequest(c, "Invalid model_source, use requested/upstream/mapping")
			return
		}
		modelSource = rawModelSource
	}

	stats, err := h.dashboardService.GetModelStatsWithUsageFiltersBySource(c.Request.Context(), startTime, endTime, filters, modelSource)
	if err != nil {
		response.Error(c, 500, "Failed to get model statistics")
		return
	}

	response.Success(c, gin.H{
		"models":     stats,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}

// GetSnapshotV2 returns an aggregated dashboard snapshot for the current supplier.
// GET /api/v1/supplier/dashboard/snapshot-v2
func (h *AccountHandler) GetSnapshotV2(c *gin.Context) {
	startTime, endTime := parseSupplierTimeRange(c)
	granularity := strings.TrimSpace(c.DefaultQuery("granularity", "day"))
	if granularity != "hour" {
		granularity = "day"
	}

	includeTrend := parseSupplierBoolQuery(c.Query("include_trend"), true)
	includeModels := parseSupplierBoolQuery(c.Query("include_model_stats"), false)
	includeGroups := parseSupplierBoolQuery(c.Query("include_group_stats"), true)

	filters, ok := buildSupplierUsageFilters(c, startTime, endTime)
	if !ok {
		return
	}

	resp := gin.H{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"start_date":   startTime.Format("2006-01-02"),
		"end_date":     endTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity":  granularity,
	}

	if includeTrend {
		trend, err := h.dashboardService.GetUsageTrendWithUsageFilters(c.Request.Context(), startTime, endTime, granularity, filters)
		if err != nil {
			response.Error(c, 500, "Failed to get usage trend")
			return
		}
		resp["trend"] = trend
	}

	if includeModels {
		models, err := h.dashboardService.GetModelStatsWithUsageFiltersBySource(c.Request.Context(), startTime, endTime, filters, usagestats.ModelSourceRequested)
		if err != nil {
			response.Error(c, 500, "Failed to get model statistics")
			return
		}
		resp["models"] = models
	}

	if includeGroups {
		groups, err := h.dashboardService.GetGroupStatsWithUsageFilters(c.Request.Context(), startTime, endTime, filters)
		if err != nil {
			response.Error(c, 500, "Failed to get group statistics")
			return
		}
		resp["groups"] = groups
	}

	response.Success(c, resp)
}
