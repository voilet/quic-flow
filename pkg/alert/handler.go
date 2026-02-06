package alert

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"github.com/voilet/quic-flow/pkg/common"
)

// Handler 告警系统 API 处理器
type Handler struct {
	store Store
}

// NewHandler 创建告警系统 API 处理器
func NewHandler(store Store) *Handler {
	return &Handler{
		store: store,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	// ========== 告警实例 ==========
	r.GET("/alerts", h.ListAlerts)
	r.GET("/alerts/stats", h.GetAlertStats)
	r.GET("/alerts/:id", h.GetAlert)
	r.POST("/alerts/:id/resolve", h.ResolveAlert)
	r.POST("/alerts/:id/silence", h.SilenceAlert)
	r.POST("/alerts/batch-resolve", h.BatchResolveAlerts)
	r.POST("/alerts/batch-silence", h.BatchSilenceAlerts)
	r.DELETE("/alerts/:id", h.DeleteAlert)

	// ========== 告警规则 ==========
	r.GET("/alert/rules", h.ListRules)
	r.GET("/alert/rules/:id", h.GetRule)
	r.POST("/alert/rules", h.CreateRule)
	r.PUT("/alert/rules/:id", h.UpdateRule)
	r.DELETE("/alert/rules/:id", h.DeleteRule)
	r.PUT("/alert/rules/:id/toggle", h.ToggleRule)
	r.POST("/alert/rules/test", h.TestRule)

	// ========== 通知渠道 ==========
	r.GET("/alert/channels", h.ListChannels)
	r.GET("/alert/channels/:id", h.GetChannel)
	r.POST("/alert/channels", h.CreateChannel)
	r.PUT("/alert/channels/:id", h.UpdateChannel)
	r.DELETE("/alert/channels/:id", h.DeleteChannel)

	// ========== 抑制规则 ==========
	r.GET("/alert/silences", h.ListSilences)
	r.GET("/alert/silences/:id", h.GetSilence)
	r.POST("/alert/silences", h.CreateSilence)
	r.DELETE("/alert/silences/:id", h.DeleteSilence)

	// ========== 值班管理 ==========
	r.GET("/alert/oncall/schedules", h.ListOnCallSchedules)
	r.GET("/alert/oncall/schedules/:id", h.GetOnCallSchedule)
	r.POST("/alert/oncall/schedules", h.CreateOnCallSchedule)
	r.PUT("/alert/oncall/schedules/:id", h.UpdateOnCallSchedule)
	r.DELETE("/alert/oncall/schedules/:id", h.DeleteOnCallSchedule)

	r.GET("/alert/oncall/users", h.ListOnCallUsers)
	r.GET("/alert/oncall/users/:id", h.GetOnCallUser)
	r.POST("/alert/oncall/users", h.CreateOnCallUser)
	r.PUT("/alert/oncall/users/:id", h.UpdateOnCallUser)
	r.DELETE("/alert/oncall/users/:id", h.DeleteOnCallUser)

	// ========== 告警分组 ==========
	r.GET("/alerts/groups", h.ListAlertGroups)
	r.GET("/alerts/groups/:group_key", h.GetAlertGroup)
}

// ========== 告警实例 ==========

// ListAlertsRequest 告警列表请求
type ListAlertsRequest struct {
	Status    string `form:"status"`
	Severity  string `form:"severity"`
	RuleID    string `form:"rule_id"`
	RuleName  string `form:"rule_name"`
	AlertName string `form:"alert_name"`
	GroupKey  string `form:"group_key"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
	Page      int    `form:"page,default=1"`
	PageSize  int    `form:"page_size,default=20"`
}

// ListAlerts 获取告警列表
func (h *Handler) ListAlerts(c *gin.Context) {
	var req ListAlertsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	filter := &AlertFilter{
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	// 解析状态
	if req.Status != "" {
		status := AlertStatus(req.Status)
		filter.Status = &status
	}

	// 解析严重程度
	if req.Severity != "" {
		severity := AlertSeverity(req.Severity)
		filter.Severity = &severity
	}

	// 解析规则ID
	if req.RuleID != "" {
		ruleID, err := strconv.ParseUint(req.RuleID, 10, 32)
		if err == nil {
			id := uint(ruleID)
			filter.RuleID = &id
		}
	}

	// 解析规则名称
	if req.RuleName != "" {
		filter.RuleName = req.RuleName
	}

	// 解析告警名称（从 labels 中匹配）
	if req.AlertName != "" {
		// 这里可以根据需要实现标签匹配逻辑
		// 暂时先忽略，因为 AlertFilter 中没有 AlertName 字段
	}

	// 解析分组键
	if req.GroupKey != "" {
		filter.GroupKey = req.GroupKey
	}

	// 解析时间范围
	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			filter.StartTime = &t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			filter.EndTime = &t
		}
	}

	alerts, total, err := h.store.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
			"items":    alerts,
			"total":    total,
			"page":     req.Page,
			"page_size": req.PageSize,
		})
}

// GetAlert 获取告警详情
func (h *Handler) GetAlert(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid alert ID")
		return
	}

	alert, err := h.store.GetAlert(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "alert not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, alert)
}

// ResolveAlertRequest 解决告警请求
type ResolveAlertRequest struct {
	Comment string `json:"comment"`
}

// ResolveAlert 解决告警
func (h *Handler) ResolveAlert(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid alert ID")
		return
	}

	var req ResolveAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	err = h.store.ResolveAlert(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "alert not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{}, "alert resolved")
}

// SilenceAlertRequest 抑制告警请求
type SilenceAlertRequest struct {
	Duration string `json:"duration"` // 如 "1h", "30m"
	Comment  string `json:"comment"`
}

// SilenceAlert 抑制告警
func (h *Handler) SilenceAlert(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid alert ID")
		return
	}

	var req SilenceAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 获取告警信息
	alert, err := h.store.GetAlert(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "alert not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 解析持续时间
	duration := 1 * time.Hour // 默认1小时
	if req.Duration != "" {
		if d, err := time.ParseDuration(req.Duration); err == nil {
			duration = d
		}
	}

	// 创建抑制规则
	silence := &SilenceRule{
		Name:       "silence-" + alert.Fingerprint,
		Comment:    req.Comment,
		MatchLabels: alert.Labels,
		StartAt:    time.Now(),
		EndAt:      time.Now().Add(duration),
		Enabled:    true,
	}

	err = h.store.CreateSilence(c.Request.Context(), silence)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, silence, "alert silenced")
}

// BatchResolveAlertsRequest 批量解决告警请求
type BatchResolveAlertsRequest struct {
	IDs []string `json:"ids"` // fingerprint 列表
}

// BatchResolveAlerts 批量解决告警
func (h *Handler) BatchResolveAlerts(c *gin.Context) {
	var req BatchResolveAlertsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	err := h.store.BatchResolveAlerts(c.Request.Context(), req.IDs)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{}, "alerts resolved")
}

// BatchSilenceAlertsRequest 批量抑制告警请求
type BatchSilenceAlertsRequest struct {
	IDs      []string `json:"ids"`
	Duration string   `json:"duration"`
	Comment  string   `json:"comment"`
}

// BatchSilenceAlerts 批量抑制告警
func (h *Handler) BatchSilenceAlerts(c *gin.Context) {
	var req BatchSilenceAlertsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 解析持续时间
	duration := 1 * time.Hour
	if req.Duration != "" {
		if d, err := time.ParseDuration(req.Duration); err == nil {
			duration = d
		}
	}

	// 为每个告警创建抑制规则
	for _, fingerprint := range req.IDs {
		alert, err := h.store.GetAlertByFingerprint(c.Request.Context(), fingerprint)
		if err != nil {
			continue
		}

		silence := &SilenceRule{
			Name:       "silence-" + fingerprint,
			Comment:    req.Comment,
			MatchLabels: alert.Labels,
			StartAt:    time.Now(),
			EndAt:      time.Now().Add(duration),
			Enabled:    true,
		}

		_ = h.store.CreateSilence(c.Request.Context(), silence)
	}

	common.SuccessRespWithMsg(c, gin.H{}, "alerts silenced")
}

// DeleteAlert 删除告警
func (h *Handler) DeleteAlert(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid alert ID")
		return
	}

	err = h.store.DeleteAlert(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "alert not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{}, "alert deleted")
}

// GetAlertStatsRequest 告警统计请求
type GetAlertStatsRequest struct {
	Status   string `form:"status"`
	Severity string `form:"severity"`
	RuleID   string `form:"rule_id"`
	RuleName string `form:"rule_name"`
	AlertName string `form:"alert_name"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
}

// GetAlertStats 获取告警统计
func (h *Handler) GetAlertStats(c *gin.Context) {
	var req GetAlertStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	var startTime, endTime *time.Time
	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			startTime = &t
		}
	}
	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			endTime = &t
		}
	}

	stats, err := h.store.GetAlertStats(c.Request.Context(), startTime, endTime)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, stats)
}

// ========== 告警规则 ==========

// ListRules 获取告警规则列表
func (h *Handler) ListRules(c *gin.Context) {
	var filter RuleFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 设置默认分页
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	rules, total, err := h.store.ListRules(c.Request.Context(), &filter)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
			"items":    rules,
			"total":    total,
			"page":     filter.Page,
			"page_size": filter.PageSize,
		})
}

// GetRule 获取告警规则详情
func (h *Handler) GetRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid rule ID")
		return
	}

	rule, err := h.store.GetRule(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "rule not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, rule)
}

// CreateRule 创建告警规则
func (h *Handler) CreateRule(c *gin.Context) {
	var rule AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	err := h.store.CreateRule(c.Request.Context(), &rule)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, rule)
}

// UpdateRule 更新告警规则
func (h *Handler) UpdateRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid rule ID")
		return
	}

	var rule AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	rule.ID = uint(id)
	err = h.store.UpdateRule(c.Request.Context(), &rule)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "rule not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, rule)
}

// DeleteRule 删除告警规则
func (h *Handler) DeleteRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid rule ID")
		return
	}

	err = h.store.DeleteRule(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "rule not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{}, "rule deleted")
}

// ToggleRuleRequest 启用/禁用规则请求
type ToggleRuleRequest struct {
	Enabled bool `json:"enabled"`
}

// ToggleRule 启用/禁用告警规则
func (h *Handler) ToggleRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid rule ID")
		return
	}

	var req ToggleRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	rule, err := h.store.GetRule(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "rule not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	rule.Enabled = req.Enabled
	err = h.store.UpdateRule(c.Request.Context(), rule)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, rule)
}

// TestRuleRequest 测试规则请求
type TestRuleRequest struct {
	Condition string                 `json:"condition"`
	Labels    map[string]interface{} `json:"labels"`
}

// TestRule 测试告警规则
func (h *Handler) TestRule(c *gin.Context) {
	var req TestRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// TODO: 实现规则测试逻辑
	common.SuccessRespWithMsg(c, gin.H{}, "rule test not implemented yet")
}

// ========== 通知渠道 ==========

// ListChannels 获取通知渠道列表
func (h *Handler) ListChannels(c *gin.Context) {
	var enabled *bool
	if enabledStr := c.Query("enabled"); enabledStr != "" {
		if enabledStr == "true" {
			b := true
			enabled = &b
		} else if enabledStr == "false" {
			b := false
			enabled = &b
		}
	}

	channels, err := h.store.ListChannels(c.Request.Context(), enabled)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, channels)
}

// GetChannel 获取通知渠道详情
func (h *Handler) GetChannel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid channel ID")
		return
	}

	channel, err := h.store.GetChannel(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "channel not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, channel)
}

// CreateChannel 创建通知渠道
func (h *Handler) CreateChannel(c *gin.Context) {
	var channel NotifyChannel
	if err := c.ShouldBindJSON(&channel); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	err := h.store.CreateChannel(c.Request.Context(), &channel)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, channel)
}

// UpdateChannel 更新通知渠道
func (h *Handler) UpdateChannel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid channel ID")
		return
	}

	var channel NotifyChannel
	if err := c.ShouldBindJSON(&channel); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	channel.ID = uint(id)
	err = h.store.UpdateChannel(c.Request.Context(), &channel)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "channel not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, channel)
}

// DeleteChannel 删除通知渠道
func (h *Handler) DeleteChannel(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid channel ID")
		return
	}

	err = h.store.DeleteChannel(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "channel not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{}, "channel deleted")
}

// ========== 抑制规则 ==========

// ListSilences 获取抑制规则列表
func (h *Handler) ListSilences(c *gin.Context) {
	var filter SilenceFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	silences, err := h.store.ListSilences(c.Request.Context(), &filter)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, silences)
}

// GetSilence 获取抑制规则详情
func (h *Handler) GetSilence(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid silence ID")
		return
	}

	silence, err := h.store.GetSilence(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "silence not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, silence)
}

// CreateSilence 创建抑制规则
func (h *Handler) CreateSilence(c *gin.Context) {
	var silence SilenceRule
	if err := c.ShouldBindJSON(&silence); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	err := h.store.CreateSilence(c.Request.Context(), &silence)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, silence)
}

// DeleteSilence 删除抑制规则
func (h *Handler) DeleteSilence(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid silence ID")
		return
	}

	err = h.store.DeleteSilence(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "silence not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{}, "silence deleted")
}

// ========== 值班管理 ==========

// ListOnCallSchedules 获取值班表列表
func (h *Handler) ListOnCallSchedules(c *gin.Context) {
	schedules, err := h.store.ListOnCallSchedules(c.Request.Context())
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, schedules)
}

// GetOnCallSchedule 获取值班表详情
func (h *Handler) GetOnCallSchedule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid schedule ID")
		return
	}

	schedule, err := h.store.GetOnCallSchedule(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "schedule not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, schedule)
}

// CreateOnCallSchedule 创建值班表
func (h *Handler) CreateOnCallSchedule(c *gin.Context) {
	var schedule OnCallSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	err := h.store.CreateOnCallSchedule(c.Request.Context(), &schedule)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, schedule)
}

// UpdateOnCallSchedule 更新值班表
func (h *Handler) UpdateOnCallSchedule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid schedule ID")
		return
	}

	var schedule OnCallSchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	schedule.ID = uint(id)
	err = h.store.UpdateOnCallSchedule(c.Request.Context(), &schedule)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "schedule not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, schedule)
}

// DeleteOnCallSchedule 删除值班表
func (h *Handler) DeleteOnCallSchedule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid schedule ID")
		return
	}

	err = h.store.DeleteOnCallSchedule(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "schedule not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{}, "schedule deleted")
}

// ListOnCallUsers 获取值班用户列表
func (h *Handler) ListOnCallUsers(c *gin.Context) {
	users, err := h.store.ListOnCallUsers(c.Request.Context())
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, users)
}

// GetOnCallUser 获取值班用户详情
func (h *Handler) GetOnCallUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid user ID")
		return
	}

	user, err := h.store.GetOnCallUser(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "user not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, user)
}

// CreateOnCallUser 创建值班用户
func (h *Handler) CreateOnCallUser(c *gin.Context) {
	var user OnCallUser
	if err := c.ShouldBindJSON(&user); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	err := h.store.CreateOnCallUser(c.Request.Context(), &user)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, user)
}

// UpdateOnCallUser 更新值班用户
func (h *Handler) UpdateOnCallUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid user ID")
		return
	}

	var user OnCallUser
	if err := c.ShouldBindJSON(&user); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	user.ID = uint(id)
	err = h.store.UpdateOnCallUser(c.Request.Context(), &user)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "user not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, user)
}

// DeleteOnCallUser 删除值班用户
func (h *Handler) DeleteOnCallUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "invalid user ID")
		return
	}

	err = h.store.DeleteOnCallUser(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInvalidParams, "user not found")
			return
		}
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{}, "user deleted")
}

// ========== 告警分组 ==========

// ListAlertGroupsRequest 告警分组列表请求
type ListAlertGroupsRequest struct {
	Page     int `form:"page,default=1"`
	PageSize int `form:"page_size,default=20"`
}

// ListAlertGroups 获取告警分组列表
func (h *Handler) ListAlertGroups(c *gin.Context) {
	var req ListAlertGroupsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 查询所有告警，按 group_key 分组
	filter := &AlertFilter{
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	alerts, _, err := h.store.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 按 group_key 分组
	groups := make(map[string][]*AlertInstance)
	for _, alert := range alerts {
		groupKey := alert.GroupKey
		if groupKey == "" {
			groupKey = "default"
		}
		groups[groupKey] = append(groups[groupKey], alert)
	}

	// 转换为列表格式
	groupList := make([]map[string]interface{}, 0, len(groups))
	for groupKey, groupAlerts := range groups {
		groupList = append(groupList, map[string]interface{}{
			"group_key": groupKey,
			"count":     len(groupAlerts),
			"alerts":    groupAlerts,
		})
	}

	common.SuccessResp(c, gin.H{
			"items":   groupList,
			"total":   len(groupList),
			"page":    req.Page,
			"page_size": req.PageSize,
		})
}

// GetAlertGroup 获取告警分组详情
func (h *Handler) GetAlertGroup(c *gin.Context) {
	groupKey := c.Param("group_key")

	filter := &AlertFilter{
		GroupKey: groupKey,
		Page:     1,
		PageSize: 1000, // 获取该分组的所有告警
	}

	alerts, count, err := h.store.ListAlerts(c.Request.Context(), filter)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
			"group_key": groupKey,
			"count":     count,
			"alerts":    alerts,
		})
}
