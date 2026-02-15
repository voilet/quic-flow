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

// ClientTagAPI 客户端标签管理 API
type ClientTagAPI struct {
	tagStore store.ClientTagStore
	logger   *monitoring.Logger
}

// NewClientTagAPI 创建客户端标签管理 API
func NewClientTagAPI(tagStore store.ClientTagStore, logger *monitoring.Logger) *ClientTagAPI {
	return &ClientTagAPI{
		tagStore: tagStore,
		logger:   logger,
	}
}

// RegisterRoutes 注册路由
func (api *ClientTagAPI) RegisterRoutes(r *gin.RouterGroup) {
	tags := r.Group("/client-tags")
	{
		tags.GET("", api.GetTags)
		tags.POST("", api.CreateTag)
		tags.GET("/:id", api.GetTag)
		tags.PUT("/:id", api.UpdateTag)
		tags.DELETE("/:id", api.DeleteTag)
		tags.GET("/:id/clients", api.GetTagClients)
		tags.POST("/:id/clients", api.AddTagClients)
		tags.DELETE("/:id/clients/:client_id", api.RemoveTagClient)
	}

	// 客户端标签操作
	clientTags := r.Group("/clients")
	{
		clientTags.GET("/:client_id/tags", api.GetClientTags)
		clientTags.PUT("/:client_id/tags", api.SetClientTags)
		clientTags.POST("/batch-tags", api.BatchSetClientTags)
		clientTags.POST("/by-tags", api.GetClientsByTags)
	}
}

// GetTags 获取标签列表
func (api *ClientTagAPI) GetTags(c *gin.Context) {
	// 检查是否需要带数量统计
	withCount := c.Query("with_count") == "true"

	if withCount {
		tags, err := api.tagStore.ListWithCount(c.Request.Context())
		if err != nil {
			api.logger.Error("Failed to list tags with count", "error", err)
			common.ErrorResp(c, common.CodeInternalError, err.Error())
			return
		}
		common.SuccessResp(c, tags)
	} else {
		tags, err := api.tagStore.List(c.Request.Context())
		if err != nil {
			api.logger.Error("Failed to list tags", "error", err)
			common.ErrorResp(c, common.CodeInternalError, err.Error())
			return
		}
		common.SuccessResp(c, tags)
	}
}

// CreateTag 创建标签
func (api *ClientTagAPI) CreateTag(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Color       string `json:"color"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	// 检查名称是否已存在
	if existing, _ := api.tagStore.GetByName(c.Request.Context(), req.Name); existing != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "标签名称已存在")
		return
	}

	// 设置默认颜色
	color := req.Color
	if color == "" {
		color = "#409EFF"
	}

	tag := &models.ClientTag{
		Name:        req.Name,
		Color:       color,
		Description: req.Description,
	}

	if err := api.tagStore.Create(c.Request.Context(), tag); err != nil {
		api.logger.Error("Failed to create tag", "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, tag)
}

// GetTag 获取标签详情
func (api *ClientTagAPI) GetTag(c *gin.Context) {
	tagID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid tag id")
		return
	}

	tag, err := api.tagStore.GetByID(c.Request.Context(), uint(tagID))
	if err != nil {
		api.logger.Error("Failed to get tag", "tag_id", tagID, "error", err)
		c.JSON(http.StatusNotFound, gin.H{
			"code":    common.CodeNotFound,
			"message": "tag not found",
		})
		return
	}

	common.SuccessResp(c, tag)
}

// UpdateTag 更新标签
func (api *ClientTagAPI) UpdateTag(c *gin.Context) {
	tagID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid tag id")
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Color       *string `json:"color"`
		Description *string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	tag, err := api.tagStore.GetByID(c.Request.Context(), uint(tagID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    common.CodeNotFound,
			"message": "tag not found",
		})
		return
	}

	// 检查名称是否与其他标签冲突
	if req.Name != nil && *req.Name != tag.Name {
		if existing, _ := api.tagStore.GetByName(c.Request.Context(), *req.Name); existing != nil {
			common.ErrorResp(c, common.CodeInvalidParams, "标签名称已存在")
			return
		}
		tag.Name = *req.Name
	}

	if req.Color != nil {
		tag.Color = *req.Color
	}
	if req.Description != nil {
		tag.Description = *req.Description
	}

	if err := api.tagStore.Update(c.Request.Context(), tag); err != nil {
		api.logger.Error("Failed to update tag", "tag_id", tagID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, tag)
}

// DeleteTag 删除标签
func (api *ClientTagAPI) DeleteTag(c *gin.Context) {
	tagID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid tag id")
		return
	}

	if err := api.tagStore.Delete(c.Request.Context(), uint(tagID)); err != nil {
		api.logger.Error("Failed to delete tag", "tag_id", tagID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"message": "tag deleted",
	})
}

// GetTagClients 获取标签下的客户端列表
func (api *ClientTagAPI) GetTagClients(c *gin.Context) {
	tagID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid tag id")
		return
	}

	clientIDs, err := api.tagStore.GetClients(c.Request.Context(), uint(tagID))
	if err != nil {
		api.logger.Error("Failed to get tag clients", "tag_id", tagID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, clientIDs)
}

// AddTagClients 添加客户端到标签
func (api *ClientTagAPI) AddTagClients(c *gin.Context) {
	tagID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid tag id")
		return
	}

	var req struct {
		ClientIDs []string `json:"client_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	if err := api.tagStore.AddClients(c.Request.Context(), uint(tagID), req.ClientIDs); err != nil {
		api.logger.Error("Failed to add clients to tag", "tag_id", tagID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"message":     "clients added to tag",
		"added_count": len(req.ClientIDs),
	})
}

// RemoveTagClient 从标签移除客户端
func (api *ClientTagAPI) RemoveTagClient(c *gin.Context) {
	tagID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid tag id")
		return
	}

	clientID := c.Param("client_id")
	if clientID == "" {
		common.ErrorResp(c, common.CodeInvalidParams, "client_id is required")
		return
	}

	if err := api.tagStore.RemoveClient(c.Request.Context(), uint(tagID), clientID); err != nil {
		api.logger.Error("Failed to remove client from tag", "tag_id", tagID, "client_id", clientID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"message": "client removed from tag",
	})
}

// GetClientTags 获取客户端的所有标签
func (api *ClientTagAPI) GetClientTags(c *gin.Context) {
	clientID := c.Param("client_id")
	if clientID == "" {
		common.ErrorResp(c, common.CodeInvalidParams, "client_id is required")
		return
	}

	tags, err := api.tagStore.GetClientTags(c.Request.Context(), clientID)
	if err != nil {
		api.logger.Error("Failed to get client tags", "client_id", clientID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, tags)
}

// SetClientTags 设置客户端的标签（覆盖）
func (api *ClientTagAPI) SetClientTags(c *gin.Context) {
	clientID := c.Param("client_id")
	if clientID == "" {
		common.ErrorResp(c, common.CodeInvalidParams, "client_id is required")
		return
	}

	var req struct {
		TagIDs []uint `json:"tag_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	if err := api.tagStore.SetClientTags(c.Request.Context(), clientID, req.TagIDs); err != nil {
		api.logger.Error("Failed to set client tags", "client_id", clientID, "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"message": "client tags updated",
	})
}

// BatchSetClientTags 批量设置客户端标签
func (api *ClientTagAPI) BatchSetClientTags(c *gin.Context) {
	var req struct {
		ClientIDs []string `json:"client_ids" binding:"required"`
		TagIDs    []uint   `json:"tag_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	if err := api.tagStore.BatchSetClientTags(c.Request.Context(), req.ClientIDs, req.TagIDs); err != nil {
		api.logger.Error("Failed to batch set client tags", "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"message":      "client tags updated",
		"client_count": len(req.ClientIDs),
		"tag_count":    len(req.TagIDs),
	})
}

// GetClientsByTags 根据标签获取客户端
func (api *ClientTagAPI) GetClientsByTags(c *gin.Context) {
	var req struct {
		TagIDs []uint `json:"tag_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, err.Error())
		return
	}

	clientIDs, err := api.tagStore.GetClientsByTags(c.Request.Context(), req.TagIDs)
	if err != nil {
		api.logger.Error("Failed to get clients by tags", "error", err)
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"client_ids":   clientIDs,
		"client_count": len(clientIDs),
	})
}
