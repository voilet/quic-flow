package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/release/models"
	"github.com/voilet/quic-flow/pkg/release/webhook"
	"gorm.io/gorm"
	"github.com/voilet/quic-flow/pkg/common"
)

// ==================== Webhook 配置管理 API ====================

// ListWebhooks 列出项目的 Webhook 配置
// GET /api/release/projects/:id/webhooks
func (s *ReleaseAPI) ListWebhooks(c *gin.Context) {
	projectID := c.Param("id")

	var webhooks []*models.WebhookConfig
	err := s.db.Where("project_id = ?", projectID).
		Order("created_at DESC").
		Find(&webhooks).Error

	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to list webhooks")
		return
	}

	// 构造响应
	items := make([]gin.H, 0, len(webhooks))
	for _, wh := range webhooks {
		item := gin.H{
			"id":            wh.ID,
			"name":          wh.Name,
			"enabled":       wh.Enabled,
			"source":        wh.Source,
			"branch_filter": wh.BranchFilter,
			"event_types":   wh.EventTypes,
			"action":        wh.Action,
			"target_env":    wh.TargetEnv,
			"auto_deploy":   wh.AutoDeploy,
			"url":           wh.URL,
			"trigger_count": wh.TriggerCount,
			"created_at":    wh.CreatedAt.Format("2006-01-02 15:04:05"),
		}

		if wh.LastTriggerAt != nil {
			t := wh.LastTriggerAt.Format("2006-01-02 15:04:05")
			item["last_trigger_at"] = t
		}

		items = append(items, item)
	}

	common.SuccessResp(c, gin.H{"data": items})
}

// GetWebhook 获取 Webhook 详情
// GET /api/release/webhooks/:id
func (s *ReleaseAPI) GetWebhook(c *gin.Context) {
	id := c.Param("id")

	var wh models.WebhookConfig
	err := s.db.Where("id = ?", id).First(&wh).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInternalError, "Webhook not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, "Failed to get webhook")
		}
		return
	}

	common.SuccessResp(c, gin.H{
		"id":             wh.ID,
		"project_id":     wh.ProjectID,
		"name":           wh.Name,
		"enabled":        wh.Enabled,
		"source":         wh.Source,
		"branch_filter":  wh.BranchFilter,
		"event_types":    wh.EventTypes,
		"action":         wh.Action,
		"target_env":     wh.TargetEnv,
		"auto_deploy":    wh.AutoDeploy,
		"url":            wh.URL,
		"trigger_count":  wh.TriggerCount,
		"last_trigger_at": wh.LastTriggerAt,
		"created_at":     wh.CreatedAt.Format("2006-01-02 15:04:05"),
		"created_by":     wh.CreatedBy,
	})
}

// CreateWebhookRequest 创建 Webhook 请求
type CreateWebhookRequest struct {
	ProjectID    string             `json:"project_id" binding:"required"`
	Name         string             `json:"name" binding:"required"`
	Source       models.WebhookSource `json:"source" binding:"required"`
	BranchFilter []string           `json:"branch_filter"`
	EventTypes   []string           `json:"event_types"`
	Action       string             `json:"action" binding:"required"`
	TargetEnv    string             `json:"target_env"`
	AutoDeploy   bool               `json:"auto_deploy"`
}

// CreateWebhook 创建 Webhook 配置
// POST /api/release/webhooks
func (s *ReleaseAPI) CreateWebhook(c *gin.Context) {
	var req CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 生成随机密钥
	secret, err := generateWebhookSecret()
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to generate secret")
		return
	}

	// 构造 Webhook URL
	baseURL := c.GetHeader("X-Base-URL")
	if baseURL == "" {
		// 从请求中获取基础 URL
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL = scheme + "://" + c.Request.Host
	}
	webhookURL := baseURL + "/api/release/webhook/incoming/" + secret[:8]

	wh := &models.WebhookConfig{
		ProjectID:    req.ProjectID,
		Name:         req.Name,
		Enabled:      true,
		Source:       req.Source,
		BranchFilter: models.StringSlice(req.BranchFilter),
		EventTypes:   models.StringSlice(req.EventTypes),
		Action:       req.Action,
		TargetEnv:    req.TargetEnv,
		AutoDeploy:   req.AutoDeploy,
		Secret:       secret,
		URL:          webhookURL,
		CreatedBy:    getCurrentUser(c),
	}

	if err := s.db.Create(wh).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{"id": wh.ID, "url": wh.URL, "secret": secret}, "Webhook created successfully")
}

// UpdateWebhookRequest 更新 Webhook 请求
type UpdateWebhookRequest struct {
	Name         string   `json:"name"`
	Enabled      *bool    `json:"enabled"`
	BranchFilter []string `json:"branch_filter"`
	EventTypes   []string `json:"event_types"`
	Action       string   `json:"action"`
	TargetEnv    string   `json:"target_env"`
	AutoDeploy   bool     `json:"auto_deploy"`
}

// UpdateWebhook 更新 Webhook 配置
// PUT /api/release/webhooks/:id
func (s *ReleaseAPI) UpdateWebhook(c *gin.Context) {
	id := c.Param("id")

	var req UpdateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 检查是否存在
	var wh models.WebhookConfig
	if err := s.db.Where("id = ?", id).First(&wh).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Webhook not found")
		return
	}

	// 构造更新数据
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.BranchFilter != nil {
		updates["branch_filter"] = models.StringSlice(req.BranchFilter)
	}
	if req.EventTypes != nil {
		updates["event_types"] = models.StringSlice(req.EventTypes)
	}
	if req.Action != "" {
		updates["action"] = req.Action
	}
	updates["target_env"] = req.TargetEnv
	updates["auto_deploy"] = req.AutoDeploy

	if err := s.db.Model(&wh).Updates(updates).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to update webhook")
		return
	}

	common.SuccessResp(c, gin.H{})
}

// DeleteWebhook 删除 Webhook 配置
// DELETE /api/release/webhooks/:id
func (s *ReleaseAPI) DeleteWebhook(c *gin.Context) {
	id := c.Param("id")

	if err := s.db.Delete(&models.WebhookConfig{}, "id = ?", id).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to delete webhook")
		return
	}

	common.SuccessResp(c, gin.H{})
}

// RegenerateWebhookSecret 重新生成 Webhook 密钥
// POST /api/release/webhooks/:id/regenerate-secret
func (s *ReleaseAPI) RegenerateWebhookSecret(c *gin.Context) {
	id := c.Param("id")

	// 检查是否存在
	var wh models.WebhookConfig
	if err := s.db.Where("id = ?", id).First(&wh).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Webhook not found")
		return
	}

	// 生成新密钥
	secret, err := generateWebhookSecret()
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to generate secret")
		return
	}

	// 更新密钥和 URL
	baseURL := c.GetHeader("X-Base-URL")
	if baseURL == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL = scheme + "://" + c.Request.Host
	}
	webhookURL := baseURL + "/api/release/webhook/incoming/" + secret[:8]

	if err := s.db.Model(&wh).Updates(map[string]interface{}{
		"secret": secret,
		"url":    webhookURL,
	}).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to update secret")
		return
	}

	common.SuccessRespWithMsg(c, gin.H{"url": webhookURL, "secret": secret}, "Secret regenerated successfully")
}

// ==================== Webhook 接收处理 ====================

// IncomingWebhook 接收 Git 平台 Webhook
// POST /api/release/webhook/incoming/:token
func (s *ReleaseAPI) IncomingWebhook(c *gin.Context) {
	token := c.Param("token")

	// 速率限制检查（使用token作为key）
	rateLimiter := webhook.GetGlobalRateLimiter()
	allowed, remaining, resetAt := rateLimiter.Allow(token)
	if !allowed {
		c.Header("X-RateLimit-Limit", "60")
		c.Header("X-RateLimit-Remaining", "0")
		c.Header("X-RateLimit-Reset", resetAt.Format(time.RFC3339))
		common.ErrorResp(c, common.CodeRateLimitExceeded, fmt.Sprintf("Rate limit exceeded. Reset at %s", resetAt.Format(time.RFC3339)))
		return
	}

	// 设置速率限制响应头
	c.Header("X-RateLimit-Limit", "60")
	c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	c.Header("X-RateLimit-Reset", resetAt.Format(time.RFC3339))

	// 读取原始 body
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to read request body")
		return
	}

	// 恢复 body 供后续使用
	c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	// 根据 token 查找 webhook 配置
	var wh models.WebhookConfig
	// token 是 secret 的前 8 位
	err = s.db.Where("SUBSTRING(secret, 1, 8) = ?", token).First(&wh).Error
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Webhook not found")
		return
	}

	if !wh.Enabled {
		common.ErrorResp(c, common.CodeInternalError, "Webhook is disabled")
		return
	}

	// 获取签名头
	signature := c.GetHeader("X-Hub-Signature") // GitHub
	if signature == "" {
		signature = c.GetHeader("X-Hub-Signature-256") // GitHub SHA256
	}
	if signature == "" {
		signature = c.GetHeader("X-Gitlab-Token") // GitLab (plain token)
	}
	if signature == "" {
		signature = c.GetHeader("X-Gitee-Token") // Gitee (plain token)
	}

	// 验证签名
	handler := webhook.GetHandler(webhook.WebhookSource(wh.Source), wh.Secret)
	if handler == nil {
		common.ErrorResp(c, common.CodeInternalError, "Unsupported webhook source")
		return
	}

	if signature != "" {
		if err := handler.VerifySignature(body, signature); err != nil {
			common.ErrorResp(c, common.CodeInternalError, err.Error())
			return
		}
	}

	// 获取事件类型
	eventType := c.GetHeader("X-GitHub-Event")
	if eventType == "" {
		eventType = c.GetHeader("X-Gitlab-Event")
	}
	if eventType == "" {
		eventType = c.GetHeader("X-Gitee-Event")
	}

	// 解析 payload
	pushInfo, err := handler.ParsePayload(body, eventType)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 检查分支过滤
	if len(wh.BranchFilter) > 0 {
		matched := false
		for _, filter := range wh.BranchFilter {
			if pushInfo.Branch == filter || pushInfo.Tag == filter {
				matched = true
				break
			}
		}
		if !matched {
			s.recordTrigger(&wh, pushInfo, "skipped", "Branch not matched")
			common.SuccessResp(c, gin.H{"skipped": true})
			return
		}
	}

	// 执行动作
	var taskID *string
	var status string
	var errorMsg string

	switch wh.Action {
	case "deploy":
		// 触发部署
		taskID, err = s.triggerDeploy(&wh, pushInfo)
		if err != nil {
			status = "failed"
			errorMsg = err.Error()
		} else {
			status = "success"
		}
	case "build":
		// 触发构建（暂不实现）
		status = "success"
	default:
		status = "skipped"
		errorMsg = "Unknown action"
	}

	// 记录触发
	s.recordTrigger(&wh, pushInfo, status, errorMsg)

	// 更新触发计数
	s.db.Model(&wh).Updates(map[string]interface{}{
		"trigger_count":   gorm.Expr("trigger_count + 1"),
		"last_trigger_at": time.Now(),
	})

	common.SuccessRespWithMsg(c, gin.H{"branch": pushInfo.Branch, "commit": pushInfo.ShortSHA, "status": status, "task_id": taskID}, "Webhook processed")
}

// triggerDeploy 触发部署
func (s *ReleaseAPI) triggerDeploy(wh *models.WebhookConfig, pushInfo *webhook.PushInfo) (*string, error) {
	// 查找或创建版本
	var version models.Version
	err := s.db.Where("project_id = ? AND version = ?", wh.ProjectID, pushInfo.Commit).First(&version).Error
	if err != nil {
		// 创建新版本
		version = models.Version{
			ProjectID: wh.ProjectID,
			Version:   pushInfo.Commit,
			Status:    string(models.VersionStatusActive),
			// 可以从 Git 获取更多信息
		}
		if err := s.db.Create(&version).Error; err != nil {
			return nil, err
		}
	}

	// 查找目标环境的客户端
	// 简化实现：查找项目下所有可用客户端
	var clients []struct {
		ID string
	}
	err = s.db.Table("clients").
		Select("id").
		Where("project_id = ?", wh.ProjectID).
		Limit(10).
		Find(&clients).Error

	if err != nil || len(clients) == 0 {
		return nil, err
	}

	clientIDs := make([]string, len(clients))
	for i, c := range clients {
		clientIDs[i] = c.ID
	}

	// 创建部署任务
	task := &models.DeployTask{
		ProjectID:    wh.ProjectID,
		VersionID:    version.ID,
		Version:      pushInfo.Commit,
		Operation:    models.OperationTypeInstall,
		ClientIDs:    models.StringSlice(clientIDs),
		Status:       "pending",
		CreatedBy:    "webhook:" + wh.Name,
	}

	if err := s.db.Create(task).Error; err != nil {
		return nil, err
	}

	// 记录关联到触发记录
	return &task.ID, nil
}

// recordTrigger 记录 Webhook 触发
func (s *ReleaseAPI) recordTrigger(wh *models.WebhookConfig, pushInfo *webhook.PushInfo, status, errorMsg string) {
	record := &models.TriggerRecord{
		WebhookID: wh.ID,
		Source:    wh.Source,
		Branch:    pushInfo.Branch,
		Tag:       pushInfo.Tag,
		Commit:    pushInfo.Commit,
		ShortSHA:  pushInfo.ShortSHA,
		Committer: pushInfo.Committer,
		Message:   pushInfo.Message,
		Status:    status,
		Error:     errorMsg,
		Payload:   "", // 不保存原始 payload
	}

	s.db.Create(record)
}

// ==================== 触发历史 API ====================

// ListTriggerHistory 列出 Webhook 触发历史
// GET /api/release/webhooks/:id/triggers
func (s *ReleaseAPI) ListTriggerHistory(c *gin.Context) {
	webhookID := c.Param("id")

	var records []*models.TriggerRecord
	err := s.db.Where("webhook_id = ?", webhookID).
		Order("triggered_at DESC").
		Limit(100).
		Find(&records).Error

	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to list trigger history")
		return
	}

	items := make([]gin.H, 0, len(records))
	for _, r := range records {
		item := gin.H{
			"id":         r.ID,
			"source":     r.Source,
			"branch":     r.Branch,
			"tag":        r.Tag,
			"short_sha":  r.ShortSHA,
			"committer":  r.Committer,
			"message":    r.Message,
			"status":     r.Status,
			"triggered_at": r.TriggeredAt.Format("2006-01-02 15:04:05"),
		}
		if r.TaskID != nil {
			item["task_id"] = *r.TaskID
		}
		if r.Error != "" {
			item["error"] = r.Error
		}
		items = append(items, item)
	}

	common.SuccessResp(c, gin.H{"data": items})
}

// TestWebhook 测试 Webhook
// POST /api/release/webhooks/:id/test
func (s *ReleaseAPI) TestWebhook(c *gin.Context) {
	id := c.Param("id")

	var wh models.WebhookConfig
	if err := s.db.Where("id = ?", id).First(&wh).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Webhook not found")
		return
	}

	// 创建测试触发记录
	testRecord := &models.TriggerRecord{
		WebhookID: wh.ID,
		Source:    wh.Source,
		Branch:    "test-branch",
		Commit:    "test-commit-sha-1234567890abcdef",
		ShortSHA:  "12345678",
		Committer: "test-user",
		Message:   "Test webhook trigger",
		Status:    "success",
	}

	s.db.Create(testRecord)

	common.SuccessRespWithMsg(c, gin.H{"trigger_id": testRecord.ID}, "Webhook test triggered")
}

// generateWebhookSecret 生成随机密钥
func generateWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
