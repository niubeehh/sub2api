package supplier

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// GroupHandler handles supplier group operations (read-only).
//
// 供应商只能查看活跃分组列表（用于创建/编辑账号时选择分组），
// 不能创建、修改或删除分组。
type GroupHandler struct {
	adminService service.AdminService
}

// NewGroupHandler creates a new supplier group handler.
func NewGroupHandler(adminService service.AdminService) *GroupHandler {
	return &GroupHandler{adminService: adminService}
}

// GetAll handles getting all active groups without pagination.
// Optional ?platform= filter.
// GET /api/v1/supplier/groups/all
func (h *GroupHandler) GetAll(c *gin.Context) {
	platform := c.Query("platform")

	var groups []service.Group
	var err error

	if platform != "" {
		groups, err = h.adminService.GetAllGroupsByPlatform(c.Request.Context(), platform)
	} else {
		groups, err = h.adminService.GetAllGroups(c.Request.Context())
	}

	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	outGroups := make([]dto.AdminGroup, 0, len(groups))
	for i := range groups {
		outGroups = append(outGroups, *dto.GroupFromServiceAdmin(&groups[i]))
	}
	response.Success(c, outGroups)
}
