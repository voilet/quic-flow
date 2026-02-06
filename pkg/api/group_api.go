package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/common"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/task/models"
	"github.com/voilet/quic-flow/pkg/task/store"
)

// GroupAPI 分组管理 API
type GroupAPI struct {
	groupStore store.GroupStore
	logger     *monitoring.Logger
}

// NewGroupAPI 创建分组管理 API
func NewGroupAPI(groupStore store.GroupStore, logger *monitoring.Logger) *GroupAPI {
	return &GroupAPI{
		groupStore: groupStore,
		logger:     logger,
	}
}

// RegisterRoutes 注册路由
func (api *GroupAPI) RegisterRoutes(r *gin.RouterGroup) {
	groups := r.Group("/groups")
	{
		groups.GET("", api.GetGroups)
		groups.POST("", api.CreateGroup)
		groups.GET("/:id", api.GetGroup)
		groups.PUT("/:id", api.UpdateGroup)
		groups.DELETE("/:id", api.DeleteGroup)
		groups.GET("/:id/clients", api.GetGroupClients)
		groups.POST("/:id/clients", api.AddGroupClients)
		groups.DELETE("/:id/clients/:client_id", api.RemoveGroupClient)
	}
}

// GetGroups 获取分组列表
func (api *GroupAPI) GetGroups(c *gin.Context) {
	groups, err := api.groupStore.List(c.Request.Context())
	if err != nil {
		api.logger.Error("Failed to list groups", "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, groups)
}

// CreateGroup 创建分组
func (api *GroupAPI) CreateGroup(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Tags        string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	group := &models.TaskGroup{
		Name:        req.Name,
		Description: req.Description,
		Tags:        req.Tags,
	}

	if err := api.groupStore.Create(c.Request.Context(), group); err != nil {
		api.logger.Error("Failed to create group", "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, group)
}

// GetGroup 获取分组详情
func (api *GroupAPI) GetGroup(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid group id")
		return
	}

	group, err := api.groupStore.GetByID(c.Request.Context(), groupID)
	if err != nil {
		api.logger.Error("Failed to get group", "group_id", groupID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "group not found",
		})
		return
	}

	common.SuccessResp(c, group)
}

// UpdateGroup 更新分组
func (api *GroupAPI) UpdateGroup(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid group id")
		return
	}

	var req struct {
		Name        *string `json:"name" binding:"required"`
		Description *string `json:"description"`
		Tags        *string `json:"tags"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	group, err := api.groupStore.GetByID(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "group not found",
		})
		return
	}

	if req.Name != nil {
		group.Name = *req.Name
	}
	if req.Description != nil {
		group.Description = *req.Description
	}
	if req.Tags != nil {
		group.Tags = *req.Tags
	}

	if err := api.groupStore.Update(c.Request.Context(), group); err != nil {
		api.logger.Error("Failed to update group", "group_id", groupID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, group)
}

// DeleteGroup 删除分组
func (api *GroupAPI) DeleteGroup(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid group id")
		return
	}

	if err := api.groupStore.Delete(c.Request.Context(), groupID); err != nil {
		api.logger.Error("Failed to delete group", "group_id", groupID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "group deleted",
	})
}

// GetGroupClients 获取分组下的客户端列表
func (api *GroupAPI) GetGroupClients(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid group id")
		return
	}

	clients, err := api.groupStore.GetClients(c.Request.Context(), groupID)
	if err != nil {
		api.logger.Error("Failed to get group clients", "group_id", groupID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, clients)
}

// AddGroupClients 添加客户端到分组
func (api *GroupAPI) AddGroupClients(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid group id")
		return
	}

	var req struct {
		ClientIDs []string `json:"client_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	if err := api.groupStore.AddClients(c.Request.Context(), groupID, req.ClientIDs); err != nil {
		api.logger.Error("Failed to add clients to group", "group_id", groupID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "clients added to group",
	})
}

// RemoveGroupClient 从分组移除客户端
func (api *GroupAPI) RemoveGroupClient(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid group id")
		return
	}

	clientID := c.Param("client_id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "client_id is required",
		})
		return
	}

	if err := api.groupStore.RemoveClient(c.Request.Context(), groupID, clientID); err != nil {
		api.logger.Error("Failed to remove client from group", "group_id", groupID, "client_id", clientID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "client removed from group",
	})
}
