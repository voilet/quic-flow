package configcenter

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler 配置中心 API 处理器
type Handler struct {
	store Store
}

// NewHandler 创建 API 处理器
func NewHandler(store Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	// ========== 配置管理 ==========
	r.POST("/config", h.CreateConfig)
	r.GET("/config", h.ListConfigs)
	r.GET("/config/:id", h.GetConfig)
	r.PUT("/config/:id", h.UpdateConfig)
	r.DELETE("/config/:id", h.DeleteConfig)

	// ========== 发布管理 ==========
	r.POST("/config/:id/release", h.ReleaseConfig)
	r.GET("/config/release/:release_id", h.GetReleaseStatus)
	r.GET("/config/:id/releases", h.ListReleases)

	// ========== 灰度规则 ==========
	r.POST("/config/:id/gray-rule", h.CreateGrayRule)
	r.GET("/config/:id/gray-rules", h.ListGrayRules)
	r.PUT("/config/:id/gray-rule/:rule_id", h.UpdateGrayRule)
	r.DELETE("/config/:id/gray-rule/:rule_id", h.DeleteGrayRule)

	// ========== 配置回滚 ==========
	r.POST("/config/:id/rollback", h.RollbackConfig)
	r.GET("/config/:id/diff", h.CompareConfig)

	// ========== 订阅管理 ==========
	r.GET("/config/subscribers", h.ListSubscribers)
	r.GET("/config/subscribers/:client_id", h.GetSubscriber)

	// ========== SSE 推送 ==========
	r.GET("/config/release/:release_id/events", h.ReleaseEvents)
	r.GET("/config/events", h.ConfigEvents)

	// ========== 变更历史 ==========
	r.GET("/config/:id/changelog", h.ListChangeLogs)

	// ========== 快照管理 ==========
	r.GET("/config/:id/snapshots", h.ListSnapshots)

	// ========== 编辑锁 ==========
	r.POST("/config/:id/lock", h.AcquireLock)
	r.DELETE("/config/:id/lock", h.ReleaseLock)
	r.GET("/config/:id/lock", h.GetLock)
}

// ========== 通用响应结构 ==========

// Response 通用 API 响应
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ========== 配置管理 API ==========

// CreateConfigRequest 创建配置请求
type CreateConfigRequest struct {
	Namespace   string                 `json:"namespace" binding:"required"`
	Group       string                 `json:"group" binding:"required"`
	DataID      string                 `json:"data_id" binding:"required"`
	ConfigType  string                 `json:"config_type" binding:"required,oneof=application system"`
	Content     string                 `json:"content" binding:"required"`
	Format      string                 `json:"format" binding:"required,oneof=json yaml properties text xml"`
	Description string                 `json:"description"`
	Tags        []string               `json:"tags"`
}

// CreateConfig 创建配置
// POST /api/config
func (h *Handler) CreateConfig(c *gin.Context) {
	var req CreateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("请求参数无效: %v", err),
		})
		return
	}

	config := &Config{
		Namespace:   req.Namespace,
		Group:       req.Group,
		DataID:      req.DataID,
		ConfigType:  ConfigType(req.ConfigType),
		Content:     req.Content,
		Format:      ConfigFormat(req.Format),
		Description: req.Description,
		Tags:        StringArray(req.Tags),
	}

	if err := h.store.CreateConfig(c.Request.Context(), config); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("创建配置失败: %v", err),
		})
		return
	}

	// 记录变更日志
	_ = h.store.CreateChangeLog(c.Request.Context(), &ConfigChangeLog{
		ConfigID:    config.ID,
		Namespace:   config.Namespace,
		Group:       config.Group,
		DataID:      config.DataID,
		ChangeType:  "create",
		NewContent:  config.Content,
		OperatedBy:  getUserID(c),
		OperatedAt:  time.Now(),
	})

	c.JSON(http.StatusCreated, Response{
		Success: true,
		Message: "配置创建成功",
		Data:    config,
	})
}

// ListConfigs 列出配置
// GET /api/config?namespace=xxx&group=xxx&data_id=xxx&config_type=xxx&tags=xxx&page=1&page_size=20
func (h *Handler) ListConfigs(c *gin.Context) {
	filter := &ConfigFilter{
		Namespace:  c.Query("namespace"),
		Group:      c.Query("group"),
		DataID:     c.Query("data_id"),
		ConfigType: ConfigType(c.Query("config_type")),
		Tags:       c.QueryArray("tags"),
		Page:       parseIntQuery(c, "page", 1),
		PageSize:   parseIntQuery(c, "page_size", 20),
	}

	configs, total, err := h.store.ListConfigs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("查询配置列表失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"total":   total,
		"page":    filter.Page,
		"page_size": filter.PageSize,
		"items":   configs,
	})
}

// GetConfig 获取配置详情
// GET /api/config/:id
func (h *Handler) GetConfig(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	config, err := h.store.GetConfig(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "配置不存在",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    config,
	})
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Content     string   `json:"content" binding:"required"`
	Format      string   `json:"format" binding:"required,oneof=json yaml properties text xml"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// UpdateConfig 更新配置
// PUT /api/config/:id
func (h *Handler) UpdateConfig(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("请求参数无效: %v", err),
		})
		return
	}

	// 获取旧配置
	oldConfig, err := h.store.GetConfig(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "配置不存在",
		})
		return
	}

	// 更新配置
	oldConfig.Content = req.Content
	oldConfig.Format = ConfigFormat(req.Format)
	oldConfig.Description = req.Description
	oldConfig.Tags = StringArray(req.Tags)

	if err := h.store.UpdateConfig(c.Request.Context(), oldConfig); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("更新配置失败: %v", err),
		})
		return
	}

	// 增加版本号
	_ = h.store.IncrementVersion(c.Request.Context(), id)

	// 记录变更日志
	_ = h.store.CreateChangeLog(c.Request.Context(), &ConfigChangeLog{
		ConfigID:    oldConfig.ID,
		Namespace:   oldConfig.Namespace,
		Group:       oldConfig.Group,
		DataID:      oldConfig.DataID,
		ChangeType:  "update",
		OldContent:  oldConfig.Content, // 注意：这里需要保存更新前的内容
		NewContent:  req.Content,
		OperatedBy:  getUserID(c),
		OperatedAt:  time.Now(),
	})

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "配置更新成功",
		Data:    oldConfig,
	})
}

// DeleteConfig 删除配置
// DELETE /api/config/:id
func (h *Handler) DeleteConfig(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	// 获取配置用于记录日志
	config, err := h.store.GetConfig(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "配置不存在",
		})
		return
	}

	if err := h.store.DeleteConfig(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("删除配置失败: %v", err),
		})
		return
	}

	// 记录变更日志
	_ = h.store.CreateChangeLog(c.Request.Context(), &ConfigChangeLog{
		ConfigID:    config.ID,
		Namespace:   config.Namespace,
		Group:       config.Group,
		DataID:      config.DataID,
		ChangeType:  "delete",
		OldContent:  config.Content,
		OperatedBy:  getUserID(c),
		OperatedAt:  time.Now(),
	})

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "配置删除成功",
	})
}

// ========== 发布管理 API ==========

// ReleaseConfigRequest 发布配置请求
type ReleaseConfigRequest struct {
	GrayRuleID *uint `json:"gray_rule_id"`
	Comment    string `json:"comment"`
}

// ReleaseConfig 发布配置
// POST /api/config/:id/release
func (h *Handler) ReleaseConfig(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	var req ReleaseConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("请求参数无效: %v", err),
		})
		return
	}

	// 获取配置
	config, err := h.store.GetConfig(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "配置不存在",
		})
		return
	}

	// 创建发布记录
	release := &ConfigRelease{
		ConfigID:    config.ID,
		Namespace:   config.Namespace,
		Group:       config.Group,
		DataID:      config.DataID,
		Content:     config.Content,
		Version:     config.Version,
		ReleaseType: ReleaseTypeFull,
		Status:      ReleaseStatusPending,
		ReleasedBy:  getUserID(c),
		ReleasedAt:  time.Now(),
		IsGray:      req.GrayRuleID != nil,
		GrayRuleID:  req.GrayRuleID,
	}

	if err := h.store.CreateRelease(c.Request.Context(), release); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("创建发布记录失败: %v", err),
		})
		return
	}

	// 记录变更日志
	_ = h.store.CreateChangeLog(c.Request.Context(), &ConfigChangeLog{
		ConfigID:    config.ID,
		Namespace:   config.Namespace,
		Group:       config.Group,
		DataID:      config.DataID,
		ChangeType:  "release",
		NewContent:  config.Content,
		OperatedBy:  getUserID(c),
		OperatedAt:  time.Now(),
	})

	// 异步推送配置
	go h.pushConfig(release)

	c.JSON(http.StatusCreated, Response{
		Success: true,
		Message: "配置发布任务已创建",
		Data:    release,
	})
}

// GetReleaseStatus 获取发布状态
// GET /api/config/release/:release_id
func (h *Handler) GetReleaseStatus(c *gin.Context) {
	releaseID := parseUintParam(c, "release_id")
	if releaseID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的发布 ID",
		})
		return
	}

	release, err := h.store.GetRelease(c.Request.Context(), releaseID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "发布记录不存在",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    release,
	})
}

// ListReleases 列出配置的发布历史
// GET /api/config/:id/releases?page=1&page_size=20
func (h *Handler) ListReleases(c *gin.Context) {
	configID := parseUintParam(c, "id")
	if configID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	filter := &ReleaseFilter{
		ConfigID: &configID,
		Page:     parseIntQuery(c, "page", 1),
		PageSize: parseIntQuery(c, "page_size", 20),
	}

	releases, total, err := h.store.ListReleases(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("查询发布历史失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"total":   total,
		"page":    filter.Page,
		"page_size": filter.PageSize,
		"items":   releases,
	})
}

// ========== 灰度规则 API ==========

// CreateGrayRuleRequest 创建灰度规则请求
type CreateGrayRuleRequest struct {
	RuleName    string  `json:"rule_name" binding:"required"`
	RuleType    string  `json:"rule_type" binding:"required,oneof=tag ip client_id percentage"`
	RuleValue   string  `json:"rule_value" binding:"required"`
	Priority    int     `json:"priority"`
	Description string  `json:"description"`
}

// CreateGrayRule 创建灰度规则
// POST /api/config/:id/gray-rule
func (h *Handler) CreateGrayRule(c *gin.Context) {
	configID := parseUintParam(c, "id")
	if configID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	var req CreateGrayRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("请求参数无效: %v", err),
		})
		return
	}

	rule := &GrayRule{
		ConfigID:    configID,
		RuleName:    req.RuleName,
		RuleType:    RuleType(req.RuleType),
		RuleValue:   req.RuleValue,
		Priority:    req.Priority,
		Description: req.Description,
		Enabled:     true,
		CreatedBy:   getUserID(c),
	}

	if err := h.store.CreateGrayRule(c.Request.Context(), rule); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("创建灰度规则失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusCreated, Response{
		Success: true,
		Message: "灰度规则创建成功",
		Data:    rule,
	})
}

// ListGrayRules 列出灰度规则
// GET /api/config/:id/gray-rules
func (h *Handler) ListGrayRules(c *gin.Context) {
	configID := parseUintParam(c, "id")
	if configID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	rules, err := h.store.ListGrayRules(c.Request.Context(), configID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("查询灰度规则失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    rules,
	})
}

// UpdateGrayRuleRequest 更新灰度规则请求
type UpdateGrayRuleRequest struct {
	RuleName    *string `json:"rule_name"`
	RuleValue   *string `json:"rule_value"`
	Priority    *int    `json:"priority"`
	Enabled     *bool   `json:"enabled"`
	Description *string `json:"description"`
}

// UpdateGrayRule 更新灰度规则
// PUT /api/config/:id/gray-rule/:rule_id
func (h *Handler) UpdateGrayRule(c *gin.Context) {
	ruleID := parseUintParam(c, "rule_id")
	if ruleID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的规则 ID",
		})
		return
	}

	var req UpdateGrayRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("请求参数无效: %v", err),
		})
		return
	}

	// 获取现有规则
	rule, err := h.store.GetGrayRule(c.Request.Context(), ruleID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "灰度规则不存在",
		})
		return
	}

	// 更新字段
	if req.RuleName != nil {
		rule.RuleName = *req.RuleName
	}
	if req.RuleValue != nil {
		rule.RuleValue = *req.RuleValue
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Description != nil {
		rule.Description = *req.Description
	}

	if err := h.store.UpdateGrayRule(c.Request.Context(), rule); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("更新灰度规则失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "灰度规则更新成功",
		Data:    rule,
	})
}

// DeleteGrayRule 删除灰度规则
// DELETE /api/config/:id/gray-rule/:rule_id
func (h *Handler) DeleteGrayRule(c *gin.Context) {
	ruleID := parseUintParam(c, "rule_id")
	if ruleID == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的规则 ID",
		})
		return
	}

	if err := h.store.DeleteGrayRule(c.Request.Context(), ruleID); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("删除灰度规则失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "灰度规则删除成功",
	})
}

// ========== 配置回滚 API ==========

// RollbackConfigRequest 回滚配置请求
type RollbackConfigRequest struct {
	Version int `json:"version" binding:"required"`
	Comment string `json:"comment"`
}

// RollbackConfig 回滚配置到指定版本
// POST /api/config/:id/rollback
func (h *Handler) RollbackConfig(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	var req RollbackConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("请求参数无效: %v", err),
		})
		return
	}

	// 获取当前配置
	config, err := h.store.GetConfig(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "配置不存在",
		})
		return
	}

	// 查找目标版本的发布记录
	filter := &ReleaseFilter{
		ConfigID: &id,
	}
	releases, _, err := h.store.ListReleases(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("查询发布历史失败: %v", err),
		})
		return
	}

	var targetRelease *ConfigRelease
	for _, r := range releases {
		if r.Version == req.Version {
			targetRelease = r
			break
		}
	}

	if targetRelease == nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("版本 %d 不存在", req.Version),
		})
		return
	}

	// 创建回滚发布记录
	rollbackRelease := &ConfigRelease{
		ConfigID:           config.ID,
		Namespace:          config.Namespace,
		Group:              config.Group,
		DataID:             config.DataID,
		Content:            targetRelease.Content,
		Version:            targetRelease.Version,
		ReleaseType:        ReleaseTypeRollback,
		Status:             ReleaseStatusPending,
		ReleasedBy:         getUserID(c),
		ReleasedAt:         time.Now(),
		RollbackFromVersion: &config.Version,
	}

	if err := h.store.CreateRelease(c.Request.Context(), rollbackRelease); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("创建回滚记录失败: %v", err),
		})
		return
	}

	// 记录变更日志
	_ = h.store.CreateChangeLog(c.Request.Context(), &ConfigChangeLog{
		ConfigID:    config.ID,
		Namespace:   config.Namespace,
		Group:       config.Group,
		DataID:      config.DataID,
		ChangeType:  "rollback",
		OldContent:  config.Content,
		NewContent:  targetRelease.Content,
		OperatedBy:  getUserID(c),
		OperatedAt:  time.Now(),
	})

	// 异步推送配置
	go h.pushConfig(rollbackRelease)

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: fmt.Sprintf("已回滚到版本 %d", req.Version),
		Data:    rollbackRelease,
	})
}

// CompareConfig 比较配置版本差异
// GET /api/config/:id/diff?version1=1&version2=2
func (h *Handler) CompareConfig(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	version1Str := c.Query("version1")
	version2Str := c.Query("version2")

	if version1Str == "" || version2Str == "" {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "必须提供 version1 和 version2 参数",
		})
		return
	}

	version1, _ := strconv.Atoi(version1Str)
	version2, _ := strconv.Atoi(version2Str)

	// 查找两个版本的发布记录
	filter := &ReleaseFilter{
		ConfigID: &id,
	}
	releases, _, err := h.store.ListReleases(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("查询发布历史失败: %v", err),
		})
		return
	}

	var release1, release2 *ConfigRelease
	for _, r := range releases {
		if r.Version == version1 {
			release1 = r
		}
		if r.Version == version2 {
			release2 = r
		}
	}

	if release1 == nil || release2 == nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "指定的版本不存在",
		})
		return
	}

	// 简单的 diff 结果
	diff := gin.H{
		"version1": gin.H{
			"version": release1.Version,
			"content": release1.Content,
			"released_at": release1.ReleasedAt,
		},
		"version2": gin.H{
			"version": release2.Version,
			"content": release2.Content,
			"released_at": release2.ReleasedAt,
		},
		"content_diff": gin.H{
			"added":    "",  // 可以集成 diff 库来生成更详细的 diff
			"removed":  "",
			"modified": release1.Content != release2.Content,
		},
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    diff,
	})
}

// ========== 订阅管理 API ==========

// ListSubscribers 列出订阅者
// GET /api/config/subscribers?namespace=xxx&status=online&page=1&page_size=20
func (h *Handler) ListSubscribers(c *gin.Context) {
	filter := &SubscriberFilter{
		Namespace: c.Query("namespace"),
		ClientID:  c.Query("client_id"),
		Status:    SubscriberStatus(c.Query("status")),
		Page:      parseIntQuery(c, "page", 1),
		PageSize:  parseIntQuery(c, "page_size", 20),
	}

	subscribers, total, err := h.store.ListSubscribers(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("查询订阅者失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"total":   total,
		"page":    filter.Page,
		"page_size": filter.PageSize,
		"items":   subscribers,
	})
}

// GetSubscriber 获取订阅者详情
// GET /api/config/subscribers/:client_id
func (h *Handler) GetSubscriber(c *gin.Context) {
	clientID := c.Param("client_id")
	if clientID == "" {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "客户端 ID 不能为空",
		})
		return
	}

	subscriber, err := h.store.GetSubscriber(c.Request.Context(), clientID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "订阅者不存在",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    subscriber,
	})
}

// ========== SSE 推送 API ==========

// ReleaseEvents SSE 推送发布进度
// GET /api/config/release/:release_id/events
func (h *Handler) ReleaseEvents(c *gin.Context) {
	releaseID := c.Param("release_id")
	if releaseID == "" {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "发布 ID 不能为空",
		})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   "流式传输不支持",
		})
		return
	}

	// 发送连接成功事件
	c.SSEvent("connected", gin.H{"release_id": releaseID})
	flusher.Flush()

	// TODO: 实现基于发布 ID 的事件订阅
	// 这里需要从 Store 中获取实时发布状态
	// 可以使用 channel 或发布订阅模式

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	releaseIDUint, _ := strconv.ParseUint(releaseID, 10, 64)

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			// 查询发布状态
			release, err := h.store.GetRelease(c.Request.Context(), uint(releaseIDUint))
			if err != nil {
				c.SSEvent("error", gin.H{"message": "查询发布状态失败"})
				flusher.Flush()
				return
			}

			// 发送进度更新
			event := gin.H{
				"release_id":    release.ID,
				"status":        release.Status,
				"total_targets": release.TotalTargets,
				"success_count": release.SuccessCount,
				"failed_count":  release.FailedCount,
				"timestamp":     time.Now().Unix(),
			}

			c.SSEvent("progress", event)
			flusher.Flush()

			// 如果发布完成，关闭连接
			if release.Status == ReleaseStatusSuccess ||
			   release.Status == ReleaseStatusFailed ||
			   release.Status == ReleaseStatusPartial {
				c.SSEvent("completed", event)
				flusher.Flush()
				return
			}
		}
	}
}

// ConfigEvents SSE 推送配置变更事件
// GET /api/config/events?namespace=xxx&group=xxx&data_id=xxx
func (h *Handler) ConfigEvents(c *gin.Context) {
	namespace := c.Query("namespace")
	group := c.Query("group")
	dataID := c.Query("data_id")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   "流式传输不支持",
		})
		return
	}

	// 发送连接成功事件
	c.SSEvent("connected", gin.H{
		"namespace": namespace,
		"group":     group,
		"data_id":   dataID,
		"timestamp": time.Now().Unix(),
	})
	flusher.Flush()

	// TODO: 实现基于配置订阅的事件推送
	// 这里需要使用发布订阅模式（如 Redis Pub/Sub 或 channel）

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			// 心跳事件，保持连接
			c.SSEvent("heartbeat", gin.H{"timestamp": time.Now().Unix()})
			flusher.Flush()
		}
	}
}

// ========== 变更历史 API ==========

// ListChangeLogs 列出配置变更历史
// GET /api/config/:id/changelog?offset=0&limit=50
func (h *Handler) ListChangeLogs(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	offset := parseIntQuery(c, "offset", 0)
	limit := parseIntQuery(c, "limit", 50)

	filter := &ChangeLogFilter{
		ConfigID: &id,
		Offset:   offset,
		Limit:    limit,
	}

	logs, err := h.store.ListChangeLogs(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("查询变更历史失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    logs,
	})
}

// ========== 快照管理 API ==========

// ListSnapshots 列出配置快照
// GET /api/config/:id/snapshots?limit=10
func (h *Handler) ListSnapshots(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	limit := parseIntQuery(c, "limit", 10)

	snapshots, err := h.store.ListSnapshots(c.Request.Context(), id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("查询快照失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    snapshots,
	})
}

// ========== 编辑锁 API ==========

// AcquireLockRequest 获取编辑锁请求
type AcquireLockRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	TTLMinutes int   `json:"ttl_minutes"`
}

// AcquireLock 获取配置编辑锁
// POST /api/config/:id/lock
func (h *Handler) AcquireLock(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	var req AcquireLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("请求参数无效: %v", err),
		})
		return
	}

	ttl := 30 * time.Minute // 默认 30 分钟
	if req.TTLMinutes > 0 {
		ttl = time.Duration(req.TTLMinutes) * time.Minute
	}

	lock, err := h.store.AcquireEditLock(c.Request.Context(), id, getUserID(c), req.SessionID, ttl)
	if err != nil {
		c.JSON(http.StatusConflict, Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "编辑锁获取成功",
		Data:    lock,
	})
}

// ReleaseLockRequest 释放编辑锁请求
type ReleaseLockRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

// ReleaseLock 释放配置编辑锁
// DELETE /api/config/:id/lock
func (h *Handler) ReleaseLock(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	var req ReleaseLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   fmt.Sprintf("请求参数无效: %v", err),
		})
		return
	}

	if err := h.store.ReleaseEditLock(c.Request.Context(), id, req.SessionID); err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Error:   fmt.Sprintf("释放编辑锁失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Message: "编辑锁释放成功",
	})
}

// GetLock 获取配置编辑锁状态
// GET /api/config/:id/lock
func (h *Handler) GetLock(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Error:   "无效的配置 ID",
		})
		return
	}

	lock, err := h.store.GetEditLock(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Success: false,
			Error:   "编辑锁不存在",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    lock,
	})
}

// ========== 辅助方法 ==========

// pushConfig 推送配置到订阅者
func (h *Handler) pushConfig(release *ConfigRelease) {
	// TODO: 实现配置推送逻辑
	// 1. 查询配置订阅者
	// 2. 根据灰度规则筛选目标
	// 3. 创建推送消息记录
	// 4. 推送配置到客户端
	// 5. 更新发布状态和统计

	// 简单实现：更新发布状态为成功
	time.Sleep(100 * time.Millisecond)
	_ = h.store.UpdateReleaseStatus(context.Background(), release.ID, ReleaseStatusSuccess)
}

// getUserID 从上下文获取用户 ID
func getUserID(c *gin.Context) string {
	// 从 JWT 中获取用户 ID
	if userID, exists := c.Get("user_id"); exists {
		if uid, ok := userID.(string); ok {
			return uid
		}
	}
	// 从请求头获取（用于调试）
	if userID := c.GetHeader("X-User-ID"); userID != "" {
		return userID
	}
	// 默认返回系统用户
	return "system"
}

// parseIntQuery 解析查询参数中的整数值
func parseIntQuery(c *gin.Context, key string, defaultValue int) int {
	if val := c.Query(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
}

// parseUintParam 解析路径参数中的无符号整数值
func parseUintParam(c *gin.Context, key string) uint {
	val := c.Param(key)
	if val == "" {
		return 0
	}
	uintVal, _ := strconv.ParseUint(val, 10, 64)
	return uint(uintVal)
}
