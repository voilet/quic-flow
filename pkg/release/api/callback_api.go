package api

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/release/callback"
	"github.com/voilet/quic-flow/pkg/release/models"
	"gorm.io/gorm"
	"github.com/voilet/quic-flow/pkg/common"
)

// ==================== 回调配置管理 API ====================

// CreateCallbackConfig 创建回调配置
// @Summary 创建回调配置
// @Tags callback
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param config body models.CallbackConfig true "回调配置"
// @Success 200 {object} models.CallbackConfig
// @Router /api/v1/release/projects/{id}/callbacks [post]
func (api *ReleaseAPI) CreateCallbackConfig(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	projectID := c.Param("id")
	if projectID == "" {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Project ID is required")
		return
	}

	// 验证项目是否存在
	var project models.Project
	if err := api.db.First(&project, "id = ?", projectID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Project not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
		}
		return
	}

	var config models.CallbackConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 设置项目ID
	config.ProjectID = projectID

	// 验证渠道配置
	if len(config.Channels) == 0 {
		common.ErrorResp(c, common.CodeServiceUnavailable, "At least one callback channel is required")
		return
	}

	// 验证渠道URL安全性
	if err := api.validateChannelURLs(config.Channels); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 验证事件类型
	if len(config.Events) == 0 {
		common.ErrorResp(c, common.CodeServiceUnavailable, "At least one event type is required")
		return
	}

	// 验证事件类型是否有效
	validEvents := map[string]bool{
		string(models.CallbackEventCanaryStarted):   true,
		string(models.CallbackEventCanaryCompleted): true,
		string(models.CallbackEventFullCompleted):   true,
	}
	for _, event := range config.Events {
		if !validEvents[event] {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Invalid event type: ")
			return
		}
	}

	// 创建配置
	if err := api.db.Create(&config).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"data": config})
}

// ListCallbackConfigs 列出项目的回调配置
// @Summary 列出项目的回调配置
// @Tags callback
// @Produce json
// @Param project_id path string true "项目ID"
// @Success 200 {array} models.CallbackConfig
// @Router /api/v1/release/projects/{project_id}/callbacks [get]
func (api *ReleaseAPI) ListCallbackConfigs(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	projectID := c.Param("id")

	var configs []models.CallbackConfig
	if err := api.db.Where("project_id = ?", projectID).Find(&configs).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"data": configs})
}

// GetCallbackConfig 获取回调配置详情
// @Summary 获取回调配置详情
// @Tags callback
// @Produce json
// @Param id path string true "配置ID"
// @Success 200 {object} models.CallbackConfig
// @Router /api/v1/release/callbacks/{id} [get]
func (api *ReleaseAPI) GetCallbackConfig(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	id := c.Param("id")
	var config models.CallbackConfig
	if err := api.db.First(&config, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Callback config not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
		}
		return
	}

	common.SuccessResp(c, gin.H{"data": config})
}

// UpdateCallbackConfig 更新回调配置
// @Summary 更新回调配置
// @Tags callback
// @Accept json
// @Produce json
// @Param id path string true "配置ID"
// @Param config body models.CallbackConfig true "回调配置"
// @Success 200 {object} models.CallbackConfig
// @Router /api/v1/release/callbacks/{id} [put]
func (api *ReleaseAPI) UpdateCallbackConfig(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	id := c.Param("id")

	// 检查是否存在
	var existing models.CallbackConfig
	if err := api.db.First(&existing, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Callback config not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
		}
		return
	}

	var update models.CallbackConfig
	if err := c.ShouldBindJSON(&update); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 保留原有ID和项目ID
	update.ID = existing.ID
	update.ProjectID = existing.ProjectID

	// 验证渠道配置
	if len(update.Channels) == 0 {
		common.ErrorResp(c, common.CodeServiceUnavailable, "At least one callback channel is required")
		return
	}

	// 验证渠道URL安全性
	if err := api.validateChannelURLs(update.Channels); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 验证事件类型
	if len(update.Events) == 0 {
		common.ErrorResp(c, common.CodeServiceUnavailable, "At least one event type is required")
		return
	}

	// 验证事件类型是否有效
	validEvents := map[string]bool{
		string(models.CallbackEventCanaryStarted):   true,
		string(models.CallbackEventCanaryCompleted): true,
		string(models.CallbackEventFullCompleted):   true,
	}
	for _, event := range update.Events {
		if !validEvents[event] {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Invalid event type: ")
			return
		}
	}

	// 更新
	if err := api.db.Save(&update).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"data": update})
}

// DeleteCallbackConfig 删除回调配置
// @Summary 删除回调配置
// @Tags callback
// @Produce json
// @Param id path string true "配置ID"
// @Success 200
// @Router /api/v1/release/callbacks/{id} [delete]
func (api *ReleaseAPI) DeleteCallbackConfig(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	id := c.Param("id")

	// 软删除
	if err := api.db.Delete(&models.CallbackConfig{}, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// TestCallbackConfig 测试回调配置
// @Summary 测试回调配置
// @Tags callback
// @Accept json
// @Produce json
// @Param id path string true "配置ID"
// @Param channel body string true "渠道类型 (feishu/dingtalk/wechat/custom)"
// @Success 200
// @Router /api/v1/release/callbacks/{id}/test [post]
func (api *ReleaseAPI) TestCallbackConfig(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	id := c.Param("id")

	var config models.CallbackConfig
	if err := api.db.First(&config, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Callback config not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
		}
		return
	}

	// 获取要测试的渠道类型
	var req struct {
		ChannelType models.CallbackType `json:"channel_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 查找对应的渠道配置
	var channel *models.CallbackChannel
	for i := range config.Channels {
		if config.Channels[i].Type == req.ChannelType && config.Channels[i].Enabled {
			channel = &config.Channels[i]
			break
		}
	}

	if channel == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Channel not found or disabled")
		return
	}

	// 构造测试消息
	testPayload := models.CallbackPayload{
		EventType:   models.CallbackEventFullCompleted,
		Timestamp:   config.CreatedAt,
		Environment: "test",
		Project: models.CallbackProject{
			ID:          config.ProjectID,
			Name:        "测试项目",
			Description: "这是一个测试回调消息",
		},
		Version: models.CallbackVersion{
			ID:          "test-version-id",
			Name:        "v1.0.0-test",
			Description: "测试版本",
		},
		Task: models.CallbackTask{
			ID:      "test-task-id",
			Type:    models.OperationTypeDeploy,
			Status:  "success",
		},
		Deployment: models.CallbackDeployment{
			TotalCount:     10,
			CompletedCount: 10,
			FailedCount:    0,
			Hosts:          []string{"test-host-1", "test-host-2"},
		},
	}

	// 调用对应的发送器进行测试
	var err error
	switch req.ChannelType {
	case models.CallbackTypeFeishu:
		if config, ok := channel.Config.(*models.FeishuCallbackConfig); ok {
			notifier := callback.NewFeishuNotifier(config)
			err = notifier.Send(testPayload)
		} else {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Invalid feishu config")
			return
		}
	case models.CallbackTypeDingTalk:
		if config, ok := channel.Config.(*models.DingTalkCallbackConfig); ok {
			notifier := callback.NewDingTalkNotifier(config)
			err = notifier.Send(testPayload)
		} else {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Invalid dingtalk config")
			return
		}
	case models.CallbackTypeWeChat:
		if config, ok := channel.Config.(*models.WeChatCallbackConfig); ok {
			notifier := callback.NewWeChatNotifier(config)
			err = notifier.Send(testPayload)
		} else {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Invalid wechat config")
			return
		}
	case models.CallbackTypeCustom:
		if config, ok := channel.Config.(*models.CustomCallbackConfig); ok {
			notifier := callback.NewCustomNotifier(config)
			err = notifier.Send(testPayload)
		} else {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Invalid custom callback config")
			return
		}
	default:
		common.ErrorResp(c, common.CodeServiceUnavailable, "Unsupported channel type")
		return
	}

	if err != nil {
		common.SuccessResp(c, gin.H{
			"success": false,
			"message": "Test callback failed",
			"error":   err.Error(),
			"data": gin.H{
				"channel": req.ChannelType,
				"payload": testPayload,
			},
		})
		return
	}

	common.SuccessRespWithMsg(c, gin.H{
			"channel": req.ChannelType,
			"payload": testPayload,
		}, "Test callback sent successfully")
}

// TestCallbackDirect 直接测试回调配置（不保存）
// @Summary 直接测试回调配置
// @Tags callback
// @Accept json
// @Produce json
// @Param config body object true "回调配置"
// @Success 200
// @Router /api/v1/release/callbacks/test-direct [post]
func (api *ReleaseAPI) TestCallbackDirect(c *gin.Context) {
	var req struct {
		ChannelType   models.CallbackType `json:"channel_type"`
		ChannelConfig struct {
			Feishu   *models.FeishuCallbackConfig   `json:"feishu,omitempty"`
			DingTalk *models.DingTalkCallbackConfig `json:"dingtalk,omitempty"`
			WeChat   *models.WeChatCallbackConfig   `json:"wechat,omitempty"`
		} `json:"channel_config"`
		ProjectName string `json:"project_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 构造测试消息
	testPayload := models.CallbackPayload{
		EventType:   models.CallbackEventFullCompleted,
		Timestamp:   time.Now(),
		Environment: "test",
		Project: models.CallbackProject{
			ID:          "test-project-id",
			Name:        req.ProjectName,
			Description: "测试回调消息",
		},
		Version: models.CallbackVersion{
			ID:          "test-version-id",
			Name:        "v1.0.0-test",
			Description: "测试版本",
		},
		Task: models.CallbackTask{
			ID:     "test-task-id",
			Type:   models.OperationTypeDeploy,
			Status: "success",
		},
		Deployment: models.CallbackDeployment{
			TotalCount:     10,
			CompletedCount: 10,
			FailedCount:    0,
			Hosts:          []string{"test-host-1", "test-host-2"},
		},
	}

	// 根据渠道类型发送测试消息
	var err error
	switch req.ChannelType {
	case models.CallbackTypeFeishu:
		if req.ChannelConfig.Feishu == nil || req.ChannelConfig.Feishu.WebhookURL == "" {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Feishu webhook URL is required")
			return
		}
		// 验证URL安全性
		if result := callback.ValidateCallbackURL(req.ChannelConfig.Feishu.WebhookURL); !result.Valid {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Invalid feishu webhook URL: ")
			return
		}
		notifier := callback.NewFeishuNotifier(req.ChannelConfig.Feishu)
		err = notifier.Send(testPayload)
	case models.CallbackTypeDingTalk:
		if req.ChannelConfig.DingTalk == nil || req.ChannelConfig.DingTalk.WebhookURL == "" {
			common.ErrorResp(c, common.CodeServiceUnavailable, "DingTalk webhook URL is required")
			return
		}
		// 验证URL安全性
		if result := callback.ValidateCallbackURL(req.ChannelConfig.DingTalk.WebhookURL); !result.Valid {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Invalid dingtalk webhook URL: ")
			return
		}
		notifier := callback.NewDingTalkNotifier(req.ChannelConfig.DingTalk)
		err = notifier.Send(testPayload)
	case models.CallbackTypeWeChat:
		if req.ChannelConfig.WeChat == nil || req.ChannelConfig.WeChat.CorpID == "" {
			common.ErrorResp(c, common.CodeServiceUnavailable, "WeChat config is required")
			return
		}
		notifier := callback.NewWeChatNotifier(req.ChannelConfig.WeChat)
		err = notifier.Send(testPayload)
	default:
		common.ErrorResp(c, common.CodeServiceUnavailable, "Unsupported channel type")
		return
	}

	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// ListCallbackHistory 列出回调历史
// @Summary 列出回调历史
// @Tags callback
// @Produce json
// @Param task_id query string false "任务ID"
// @Param config_id query string false "配置ID"
// @Param event_type query string false "事件类型"
// @Param channel query string false "渠道类型"
// @Param status query string false "状态"
// @Param page query int false "页码"
// @Param page_size query int false "每页数量"
// @Success 200 {array} models.CallbackHistory
// @Router /api/v1/release/callbacks/history [get]
func (api *ReleaseAPI) ListCallbackHistory(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	query := api.db.Model(&models.CallbackHistory{})

	// 筛选条件
	if taskID := c.Query("task_id"); taskID != "" {
		query = query.Where("task_id = ?", taskID)
	}
	if configID := c.Query("config_id"); configID != "" {
		query = query.Where("config_id = ?", configID)
	}
	if eventType := c.Query("event_type"); eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if channel := c.Query("channel"); channel != "" {
		query = query.Where("channel = ?", channel)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	// 分页
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var total int64
	query.Count(&total)

	var history []models.CallbackHistory
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&history).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"success": true,
		"data": gin.H{
			"items": history,
			"total": total,
			"page":  page,
			"page_size": pageSize,
		},
	})
}

// ListTaskCallbackHistory 列出任务的回调历史
// @Summary 列出任务的回调历史
// @Tags callback
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {array} models.CallbackHistory
// @Router /api/v1/release/tasks/{id}/callbacks [get]
func (api *ReleaseAPI) ListTaskCallbackHistory(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	taskID := c.Param("id")

	var history []models.CallbackHistory
	if err := api.db.Where("task_id = ?", taskID).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"data": history})
}

// GetCallbackHistory 获取回调历史详情
// @Summary 获取回调历史详情
// @Tags callback
// @Produce json
// @Param id path string true "历史ID"
// @Success 200 {object} models.CallbackHistory
// @Router /api/v1/release/callbacks/history/{id} [get]
func (api *ReleaseAPI) GetCallbackHistory(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	id := c.Param("id")
	var history models.CallbackHistory
	if err := api.db.First(&history, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Callback history not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
		}
		return
	}

	common.SuccessResp(c, gin.H{"data": history})
}

// RetryCallbackHistory 重试失败的回调
// @Summary 重试失败的回调
// @Tags callback
// @Produce json
// @Param id path string true "历史ID"
// @Success 200
// @Router /api/v1/release/callbacks/history/{id}/retry [post]
func (api *ReleaseAPI) RetryCallbackHistory(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	id := c.Param("id")

	// 获取历史记录
	var history models.CallbackHistory
	if err := api.db.First(&history, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Callback history not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
		}
		return
	}

	// 只有失败的记录才能重试
	if history.Status != models.CallbackStatusFailed {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Only failed callbacks can be retried")
		return
	}

	// 获取回调配置
	var config models.CallbackConfig
	if err := api.db.First(&config, "id = ?", history.ConfigID).Error; err != nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Callback config not found")
		return
	}

	// 找到对应的渠道配置
	var channel *models.CallbackChannel
	for i := range config.Channels {
		if config.Channels[i].Type == history.Channel {
			channel = &config.Channels[i]
			break
		}
	}

	if channel == nil {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Channel not found in config")
		return
	}

	// 创建新的回调管理器并发送
	manager := callback.NewManager(api.db)
	defer manager.Close()

	err := manager.SendCallback(*channel, history.Request)
	if err != nil {
		// 更新历史记录
		history.RetryCount++
		history.Error = err.Error()
		api.db.Save(&history)

		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 重试成功，更新状态
	history.Status = models.CallbackStatusSuccess
	history.RetryCount++
	history.Error = ""
	api.db.Save(&history)

	common.SuccessResp(c, gin.H{})
}

// GetCallbackStats 获取回调统计
// @Summary 获取回调统计
// @Tags callback
// @Produce json
// @Param project_id query string false "项目ID"
// @Param start_time query string false "开始时间"
// @Param end_time query string false "结束时间"
// @Success 200
// @Router /api/v1/release/callbacks/stats [get]
func (api *ReleaseAPI) GetCallbackStats(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	// 构建基础查询条件
	baseQuery := api.db.Model(&models.CallbackHistory{})

	// 筛选条件
	projectID := c.Query("project_id")
	startTime := c.Query("start_time")
	endTime := c.Query("end_time")

	if projectID != "" {
		baseQuery = baseQuery.Joins("JOIN deploy_tasks ON callback_history.task_id = deploy_tasks.id").
			Where("deploy_tasks.project_id = ?", projectID)
	}
	if startTime != "" {
		baseQuery = baseQuery.Where("callback_history.created_at >= ?", startTime)
	}
	if endTime != "" {
		baseQuery = baseQuery.Where("callback_history.created_at <= ?", endTime)
	}

	// 使用一次查询获取所有状态的统计（优化：避免多次查询）
	type StatusCount struct {
		Status string `json:"status"`
		Count  int64  `json:"count"`
	}
	var statusCounts []StatusCount
	baseQuery.Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts)

	// 解析状态统计结果
	var totalCount, successCount, failedCount, retryingCount, pendingCount int64
	for _, sc := range statusCounts {
		totalCount += sc.Count
		switch models.CallbackStatus(sc.Status) {
		case models.CallbackStatusSuccess:
			successCount = sc.Count
		case models.CallbackStatusFailed:
			failedCount = sc.Count
		case models.CallbackStatusRetrying:
			retryingCount = sc.Count
		case models.CallbackStatusPending:
			pendingCount = sc.Count
		}
	}

	// 计算成功率
	var successRate float64
	if totalCount > 0 {
		successRate = float64(successCount) / float64(totalCount) * 100
	}

	// 统计平均延迟（使用相同的筛选条件）
	var avgDuration float64
	avgQuery := api.db.Model(&models.CallbackHistory{}).Where("status = ?", models.CallbackStatusSuccess)
	if projectID != "" {
		avgQuery = avgQuery.Joins("JOIN deploy_tasks ON callback_history.task_id = deploy_tasks.id").
			Where("deploy_tasks.project_id = ?", projectID)
	}
	if startTime != "" {
		avgQuery = avgQuery.Where("callback_history.created_at >= ?", startTime)
	}
	if endTime != "" {
		avgQuery = avgQuery.Where("callback_history.created_at <= ?", endTime)
	}
	avgQuery.Select("COALESCE(AVG(duration), 0)").Row().Scan(&avgDuration)

	// 按渠道统计（使用相同的筛选条件）
	var channelStats []struct {
		Channel string `json:"channel"`
		Count   int64  `json:"count"`
	}
	channelQuery := api.db.Model(&models.CallbackHistory{})
	if projectID != "" {
		channelQuery = channelQuery.Joins("JOIN deploy_tasks ON callback_history.task_id = deploy_tasks.id").
			Where("deploy_tasks.project_id = ?", projectID)
	}
	if startTime != "" {
		channelQuery = channelQuery.Where("callback_history.created_at >= ?", startTime)
	}
	if endTime != "" {
		channelQuery = channelQuery.Where("callback_history.created_at <= ?", endTime)
	}
	channelQuery.Select("channel, COUNT(*) as count").
		Group("channel").
		Scan(&channelStats)

	// 按事件类型统计（使用相同的筛选条件）
	var eventStats []struct {
		EventType string `json:"event_type"`
		Count     int64  `json:"count"`
	}
	eventQuery := api.db.Model(&models.CallbackHistory{})
	if projectID != "" {
		eventQuery = eventQuery.Joins("JOIN deploy_tasks ON callback_history.task_id = deploy_tasks.id").
			Where("deploy_tasks.project_id = ?", projectID)
	}
	if startTime != "" {
		eventQuery = eventQuery.Where("callback_history.created_at >= ?", startTime)
	}
	if endTime != "" {
		eventQuery = eventQuery.Where("callback_history.created_at <= ?", endTime)
	}
	eventQuery.Select("event_type, COUNT(*) as count").
		Group("event_type").
		Scan(&eventStats)

	// 获取重试队列状态（如果有）
	var queueStatus map[string]interface{}
	if api.callbackMgr != nil {
		queueStatus = api.callbackMgr.GetRetryQueueStatus()
	}

	common.SuccessResp(c, gin.H{
		"success": true,
		"data": gin.H{
			"total_count":    totalCount,
			"success_count":  successCount,
			"failed_count":   failedCount,
			"retrying_count": retryingCount,
			"pending_count":  pendingCount,
			"success_rate":   successRate,
			"avg_duration":   avgDuration,
			"by_channel":     channelStats,
			"by_event_type":  eventStats,
			"queue_status":   queueStatus,
		},
	})
}

// ==================== 模板管理 API ====================

// PreviewCallbackTemplate 预览回调模板
// @Summary 预览回调模板
// @Tags callback
// @Accept json
// @Produce json
// @Param template body string true "模板内容"
// @Success 200
// @Router /api/v1/release/callbacks/template/preview [post]
func (api *ReleaseAPI) PreviewCallbackTemplate(c *gin.Context) {
	var req struct {
		Template string `json:"template" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	engine := callback.NewTemplateEngine()

	// 先验证模板
	validation := engine.ValidateTemplate(req.Template)
	if !validation.Valid {
		common.SuccessResp(c, gin.H{
			"success": false,
			"error":   "模板验证失败",
			"data": gin.H{
				"validation": validation,
			},
		})
		return
	}

	// 渲染预览
	rendered, err := engine.Preview(req.Template)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"success": true,
		"data": gin.H{
			"rendered":   rendered,
			"validation": validation,
		},
	})
}

// ValidateCallbackTemplate 验证回调模板
// @Summary 验证回调模板语法
// @Tags callback
// @Accept json
// @Produce json
// @Param template body string true "模板内容"
// @Success 200
// @Router /api/v1/release/callbacks/template/validate [post]
func (api *ReleaseAPI) ValidateCallbackTemplate(c *gin.Context) {
	var req struct {
		Template string `json:"template" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	engine := callback.NewTemplateEngine()
	validation := engine.ValidateTemplate(req.Template)

	common.SuccessResp(c, gin.H{"data": validation})
}

// GetCallbackTemplateVariables 获取模板变量列表
// @Summary 获取可用的模板变量
// @Tags callback
// @Produce json
// @Success 200
// @Router /api/v1/release/callbacks/template/variables [get]
func (api *ReleaseAPI) GetCallbackTemplateVariables(c *gin.Context) {
	engine := callback.NewTemplateEngine()
	variables := engine.GetAvailableVariables()
	examples := engine.GetConditionExamples()

	common.SuccessResp(c, gin.H{
		"success": true,
		"data": gin.H{
			"variables": variables,
			"examples":  examples,
		},
	})
}

// GetDefaultCallbackTemplates 获取默认模板
// @Summary 获取默认回调模板
// @Tags callback
// @Produce json
// @Success 200
// @Router /api/v1/release/callbacks/template/defaults [get]
func (api *ReleaseAPI) GetDefaultCallbackTemplates(c *gin.Context) {
	engine := callback.NewTemplateEngine()
	templates := engine.GetDefaultTemplates()

	common.SuccessResp(c, gin.H{"data": templates})
}

// ==================== 消息模板管理 API ====================

// ListMessageTemplates 获取消息模板列表
// @Summary 获取消息模板列表
// @Tags template
// @Produce json
// @Param project_id query string false "项目ID（空表示全局模板）"
// @Param channel query string false "渠道类型"
// @Param type query string false "模板类型（system/custom）"
// @Success 200
// @Router /api/v1/release/templates [get]
func (api *ReleaseAPI) ListMessageTemplates(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	query := api.db.Model(&models.MessageTemplate{})

	// 筛选项目
	projectID := c.Query("project_id")
	if projectID != "" {
		query = query.Where("project_id = ?", projectID)
	} else {
		// 如果不指定项目，则显示全局模板
		query = query.Where("project_id IS NULL")
	}

	// 筛选渠道
	if channel := c.Query("channel"); channel != "" {
		query = query.Where("channel = ? OR channel = ''", channel)
	}

	// 筛选类型
	if templateType := c.Query("type"); templateType != "" {
		query = query.Where("type = ?", templateType)
	}

	var templates []models.MessageTemplate
	if err := query.Order("is_default DESC, created_at DESC").Find(&templates).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"data": templates})
}

// GetMessageTemplate 获取模板详情
// @Summary 获取模板详情
// @Tags template
// @Produce json
// @Param id path string true "模板ID"
// @Success 200
// @Router /api/v1/release/templates/{id} [get]
func (api *ReleaseAPI) GetMessageTemplate(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	id := c.Param("id")
	var template models.MessageTemplate
	if err := api.db.First(&template, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Template not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
		}
		return
	}

	common.SuccessResp(c, gin.H{"data": template})
}

// CreateMessageTemplate 创建消息模板
// @Summary 创建消息模板
// @Tags template
// @Accept json
// @Produce json
// @Param project_id query string false "项目ID（空表示全局模板）"
// @Param template body models.CreateMessageTemplateRequest true "模板信息"
// @Success 200
// @Router /api/v1/release/templates [post]
func (api *ReleaseAPI) CreateMessageTemplate(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	var req models.CreateMessageTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 验证模板语法
	engine := callback.NewTemplateEngine()
	validation := engine.ValidateTemplate(req.Content)
	if !validation.Valid {
		common.ErrorResp(c, common.CodeServiceUnavailable, "Invalid template syntax")
		return
	}

	template := models.MessageTemplate{
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
		Type:        models.MessageTemplateTypeCustom,
		Channel:     req.Channel,
		IsDefault:   req.IsDefault,
	}

	// 设置项目ID
	projectID := c.Query("project_id")
	if projectID != "" {
		template.ProjectID = &projectID
	}

	// 提取使用的变量
	template.Variables = extractTemplateVariables(req.Content)

	if err := api.db.Create(&template).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"data": template})
}

// UpdateMessageTemplate 更新消息模板
// @Summary 更新消息模板
// @Tags template
// @Accept json
// @Produce json
// @Param id path string true "模板ID"
// @Param template body models.UpdateMessageTemplateRequest true "模板信息"
// @Success 200
// @Router /api/v1/release/templates/{id} [put]
func (api *ReleaseAPI) UpdateMessageTemplate(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	id := c.Param("id")
	var template models.MessageTemplate
	if err := api.db.First(&template, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Template not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
		}
		return
	}

	// 系统模板不允许修改
	if template.Type == models.MessageTemplateTypeSystem {
		common.ErrorResp(c, common.CodeServiceUnavailable, "System templates cannot be modified")
		return
	}

	var req models.UpdateMessageTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 如果内容变更，验证模板语法
	if req.Content != "" && req.Content != template.Content {
		engine := callback.NewTemplateEngine()
		validation := engine.ValidateTemplate(req.Content)
		if !validation.Valid {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Invalid template syntax")
			return
		}
		template.Content = req.Content
		template.Variables = extractTemplateVariables(req.Content)
	}

	if req.Name != "" {
		template.Name = req.Name
	}
	if req.Description != "" {
		template.Description = req.Description
	}
	if req.Channel != "" {
		template.Channel = req.Channel
	}
	template.IsDefault = req.IsDefault

	if err := api.db.Save(&template).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"data": template})
}

// DeleteMessageTemplate 删除消息模板
// @Summary 删除消息模板
// @Tags template
// @Produce json
// @Param id path string true "模板ID"
// @Success 200
// @Router /api/v1/release/templates/{id} [delete]
func (api *ReleaseAPI) DeleteMessageTemplate(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	id := c.Param("id")
	var template models.MessageTemplate
	if err := api.db.First(&template, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Template not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
		}
		return
	}

	// 系统模板不允许删除
	if template.Type == models.MessageTemplateTypeSystem {
		common.ErrorResp(c, common.CodeServiceUnavailable, "System templates cannot be deleted")
		return
	}

	if err := api.db.Delete(&template).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{})
}

// CopyMessageTemplate 复制消息模板
// @Summary 复制消息模板
// @Tags template
// @Produce json
// @Param id path string true "源模板ID"
// @Param project_id query string false "目标项目ID"
// @Success 200
// @Router /api/v1/release/templates/{id}/copy [post]
func (api *ReleaseAPI) CopyMessageTemplate(c *gin.Context) {
	if !api.checkDB(c) {
		return
	}

	id := c.Param("id")
	var source models.MessageTemplate
	if err := api.db.First(&source, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeServiceUnavailable, "Template not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
		}
		return
	}

	// 创建副本
	newTemplate := models.MessageTemplate{
		Name:        source.Name + " (副本)",
		Description: source.Description,
		Content:     source.Content,
		Type:        models.MessageTemplateTypeCustom,
		Channel:     source.Channel,
		IsDefault:   false,
		Variables:   source.Variables,
	}

	// 设置目标项目
	projectID := c.Query("project_id")
	if projectID != "" {
		newTemplate.ProjectID = &projectID
	}

	if err := api.db.Create(&newTemplate).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{"data": newTemplate})
}

// extractTemplateVariables 从模板内容中提取使用的变量
func extractTemplateVariables(content string) models.TemplateVariables {
	engine := callback.NewTemplateEngine()
	availableVars := engine.GetAvailableVariables()

	usedVars := []string{}
	for _, v := range availableVars {
		if contains(content, "{{"+v.Name+"}}") || contains(content, "{{"+v.Name+" ") {
			usedVars = append(usedVars, v.Name)
		}
	}

	return usedVars
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// validateChannelURLs 验证渠道配置中的URL安全性
func (api *ReleaseAPI) validateChannelURLs(channels []models.CallbackChannel) error {
	for i, channel := range channels {
		if !channel.Enabled {
			continue
		}

		var urlToValidate string
		switch channel.Type {
		case models.CallbackTypeFeishu:
			if config, ok := channel.Config.(*models.FeishuCallbackConfig); ok && config != nil {
				urlToValidate = config.WebhookURL
			}
		case models.CallbackTypeDingTalk:
			if config, ok := channel.Config.(*models.DingTalkCallbackConfig); ok && config != nil {
				urlToValidate = config.WebhookURL
			}
		case models.CallbackTypeCustom:
			if config, ok := channel.Config.(*models.CustomCallbackConfig); ok && config != nil {
				urlToValidate = config.URL
			}
		case models.CallbackTypeWeChat:
			// 企业微信使用 CorpID 而不是 URL，跳过验证
			continue
		}

		if urlToValidate != "" {
			result := callback.ValidateCallbackURL(urlToValidate)
			if !result.Valid {
				return fmt.Errorf("channel %d (%s): %s", i+1, channel.Type, result.Error)
			}
		}
	}
	return nil
}
