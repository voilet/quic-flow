package api

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/release/models"
	"gorm.io/gorm"
	"github.com/voilet/quic-flow/pkg/common"
)

// CredentialListResponse 凭证列表响应
type CredentialListResponse struct {
	Success bool                      `json:"success"`
	Data    []*CredentialListItem     `json:"data"`
}

// CredentialListItem 凭证列表项（不包含敏感数据）
type CredentialListItem struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Type        models.CredentialType   `json:"type"`
	Scope       models.CredentialScope  `json:"scope"`
	ProjectID   *string                  `json:"project_id,omitempty"`
	ProjectName string                   `json:"project_name,omitempty"`
	Description string                   `json:"description"`
	UseCount    int                      `json:"use_count"`
	LastUsedAt  *string                  `json:"last_used_at,omitempty"`
	CreatedAt   string                   `json:"created_at"`

	// 用于前端显示的服务器地址和用户名（不包含密码）
	ServerURL string `json:"server_url,omitempty"`
	Username  string `json:"username,omitempty"`
}

// CredentialDetailResponse 凭证详情响应
type CredentialDetailResponse struct {
	Success bool                `json:"success"`
	Data    *CredentialDetail   `json:"data"`
}

// CredentialDetail 凭证详情
type CredentialDetail struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Type        models.CredentialType   `json:"type"`
	Scope       models.CredentialScope  `json:"scope"`
	ProjectID   *string                  `json:"project_id,omitempty"`
	ProjectName string                   `json:"project_name,omitempty"`
	Description string                   `json:"description"`
	UseCount    int                      `json:"use_count"`
	LastUsedAt  *string                  `json:"last_used_at,omitempty"`
	CreatedAt   string                   `json:"created_at"`
	CreatedBy   string                   `json:"created_by"`

	// 非敏感数据（用于前端显示）
	ServerURL string `json:"server_url,omitempty"`
	Username  string `json:"username,omitempty"`
}

// CreateCredentialRequest 创建凭证请求
type CreateCredentialRequest struct {
	Name          string                   `json:"name" binding:"required"`
	Type          models.CredentialType   `json:"type" binding:"required"`
	Scope         models.CredentialScope  `json:"scope" binding:"required"`
	ProjectID     *string                  `json:"project_id"`
	Description   string                   `json:"description"`
	ServerURL     string                   `json:"server_url"`
	Username      string                   `json:"username"`
	Password      string                   `json:"password"`
	SSHKey        string                   `json:"ssh_key"`
	SSHPassphrase string                   `json:"ssh_passphrase"`
}

// UpdateCredentialRequest 更新凭证请求
type UpdateCredentialRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListCredentials 列出凭证
// GET /api/release/credentials
func (s *ReleaseAPI) ListCredentials(c *gin.Context) {
	scope := c.Query("scope")
	ctype := c.Query("type")

	var creds []*models.Credential
	query := s.db.Model(&models.Credential{})

	if scope != "" {
		query = query.Where("scope = ?", scope)
	}
	if ctype != "" {
		query = query.Where("type = ?", ctype)
	}

	if err := query.Order("created_at DESC").Find(&creds).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to list credentials")
		return
	}

	items := make([]*CredentialListItem, 0, len(creds))
	for _, cred := range creds {
		// 解密获取非敏感信息用于显示
		data, _ := s.credentialMgr.DecryptData(cred.ID)

		item := &CredentialListItem{
			ID:          cred.ID,
			Name:        cred.Name,
			Type:        cred.Type,
			Scope:       cred.Scope,
			ProjectID:   cred.ProjectID,
			Description: cred.Description,
			UseCount:    cred.UseCount,
			CreatedAt:    cred.CreatedAt.Format("2006-01-02 15:04:05"),
		}

		if cred.LastUsedAt != nil {
			t := cred.LastUsedAt.Format("2006-01-02 15:04:05")
			item.LastUsedAt = &t
		}

		if data != nil {
			item.ServerURL = data.ServerURL
			item.Username = data.Username
		}

		// 获取项目名称
		if cred.ProjectID != nil {
			var proj models.Project
			if s.db.Where("id = ?", *cred.ProjectID).First(&proj).Error == nil {
				item.ProjectName = proj.Name
			}
		}

			items = append(items, item)
	}

	common.SuccessResp(c, gin.H{"data": items})
}

// GetCredential 获取凭证详情
// GET /api/release/credentials/:id
func (s *ReleaseAPI) GetCredential(c *gin.Context) {
	id := c.Param("id")

	cred, data, err := s.credentialMgr.GetWithData(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInternalError, "Credential not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, "Failed to get credential")
		}
		return
	}

	detail := &CredentialDetail{
		ID:          cred.ID,
		Name:        cred.Name,
		Type:        cred.Type,
		Scope:       cred.Scope,
		ProjectID:   cred.ProjectID,
		Description: cred.Description,
		UseCount:    cred.UseCount,
		CreatedAt:   cred.CreatedAt.Format("2006-01-02 15:04:05"),
		CreatedBy:   cred.CreatedBy,
	}

	if cred.LastUsedAt != nil {
		t := cred.LastUsedAt.Format("2006-01-02 15:04:05")
		detail.LastUsedAt = &t
	}

	if data != nil {
		detail.ServerURL = data.ServerURL
		detail.Username = data.Username
	}

	// 获取项目名称
	if cred.ProjectID != nil {
		var proj models.Project
		if s.db.Where("id = ?", *cred.ProjectID).First(&proj).Error == nil {
			detail.ProjectName = proj.Name
		}
	}

	common.SuccessResp(c, detail)
}

// CreateCredential 创建凭证
// POST /api/release/credentials
func (s *ReleaseAPI) CreateCredential(c *gin.Context) {
	var req CreateCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 构建凭证数据
	credData := &models.CredentialData{
		ServerURL: req.ServerURL,
		Username:  req.Username,
	}

	switch req.Type {
	case models.CredentialTypeDockerRegistry, models.CredentialTypeUsernamePassword:
		credData.Password = req.Password
	case models.CredentialTypeGitSSH:
		credData.SSHKey = req.SSHKey
		credData.SSHPassphrase = req.SSHPassphrase
	case models.CredentialTypeGitToken:
		credData.Token = req.Password // Token 用 Password 字段传递
	}

	// 创建凭证
	cred := &models.Credential{
		Name:        req.Name,
		Type:        req.Type,
		Scope:       req.Scope,
		ProjectID:   req.ProjectID,
		Description: req.Description,
		CreatedBy:   getCurrentUser(c),
	}

	if err := s.credentialMgr.Create(cred, credData); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{"id": cred.ID}, "Credential created successfully")
}

// UpdateCredential 更新凭证
// PUT /api/release/credentials/:id
func (s *ReleaseAPI) UpdateCredential(c *gin.Context) {
	id := c.Param("id")

	var req UpdateCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	cred, err := s.credentialMgr.Get(id)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Credential not found")
		return
	}

	cred.Name = req.Name
	cred.Description = req.Description

	if err := s.credentialMgr.Update(cred); err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to update credential")
		return
	}

	common.SuccessResp(c, gin.H{})
}

// DeleteCredential 删除凭证
// DELETE /api/release/credentials/:id
func (s *ReleaseAPI) DeleteCredential(c *gin.Context) {
	id := c.Param("id")

	if err := s.credentialMgr.Delete(id); err != nil {
		if err.Error() == "credential is in use" {
			common.ErrorResp(c, common.CodeInternalError, "Credential is in use, cannot delete")
		} else {
			common.ErrorResp(c, common.CodeInternalError, "Failed to delete credential")
		}
		return
	}

	common.SuccessResp(c, gin.H{})
}

// GetProjectCredentials 获取项目的可用凭证
// GET /api/release/projects/:id/credentials
func (s *ReleaseAPI) GetProjectCredentials(c *gin.Context) {
	projectID := c.Param("id")

	creds, err := s.credentialMgr.ListByProject(projectID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to list credentials")
		return
	}

	items := make([]*CredentialListItem, 0, len(creds))
	for _, cred := range creds {
		// 解密获取非敏感信息
		data, _ := s.credentialMgr.DecryptData(cred.ID)

		item := &CredentialListItem{
			ID:          cred.ID,
			Name:        cred.Name,
			Type:        cred.Type,
			Scope:       cred.Scope,
			Description: cred.Description,
		}

		if data != nil {
			item.ServerURL = data.ServerURL
			item.Username = data.Username
		}

		items = append(items, item)
	}

	common.SuccessResp(c, gin.H{"data": items})
}

// AddProjectCredential 关联凭证到项目
// POST /api/release/projects/:id/credentials
func (s *ReleaseAPI) AddProjectCredential(c *gin.Context) {
	projectID := c.Param("id")

	var req struct {
		CredentialID string `json:"credential_id" binding:"required"`
		Alias        string `json:"alias"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	if err := s.credentialMgr.AddToProject(projectID, req.CredentialID, req.Alias); err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to add credential to project")
		return
	}

	common.SuccessResp(c, gin.H{})
}

// RemoveProjectCredential 取消项目凭证关联
// DELETE /api/release/projects/:id/credentials/:credential_id
func (s *ReleaseAPI) RemoveProjectCredential(c *gin.Context) {
	projectID := c.Param("id")
	credentialID := c.Param("credential_id")

	if err := s.credentialMgr.RemoveFromProject(projectID, credentialID); err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to remove credential from project")
		return
	}

	common.SuccessResp(c, gin.H{})
}

// getCurrentUser 从上下文获取当前用户
func getCurrentUser(c *gin.Context) string {
	// 从 header 或 session 获取用户
	if user := c.GetHeader("X-User-ID"); user != "" {
		return user
	}
	if user := c.GetHeader("X-User-Name"); user != "" {
		return user
	}
	return "system"
}

// isSecretPath 检查是否是敏感路径（用于日志脱敏）
func isSecretPath(path string) bool {
	sensitivePaths := []string{
		"password", "secret", "token", "key", "credential",
		"ssh_key", "passphrase", "registry_pass",
	}
	lowerPath := strings.ToLower(path)
	for _, sp := range sensitivePaths {
		if strings.Contains(lowerPath, sp) {
			return true
		}
	}
	return false
}
