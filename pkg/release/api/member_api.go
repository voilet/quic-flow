package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/release/models"
	"gorm.io/gorm"
	"github.com/voilet/quic-flow/pkg/common"
)

// ==================== 成员管理 API ====================

// ListMembers 列出项目成员
// GET /api/release/projects/:id/members
func (s *ReleaseAPI) ListMembers(c *gin.Context) {
	projectID := c.Param("id")

	var members []*models.ProjectMember
	err := s.db.Where("project_id = ?", projectID).
		Order("added_at DESC").
		Find(&members).Error

	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to list members")
		return
	}

	items := make([]gin.H, 0, len(members))
	for _, m := range members {
		items = append(items, gin.H{
			"id":        m.ID,
			"user_id":   m.UserID,
			"role":      m.Role,
			"added_by":  m.AddedBy,
			"added_at":  m.AddedAt.Format("2006-01-02 15:04:05"),
		})
	}

	common.SuccessResp(c, gin.H{"data": items})
}

// AddMemberRequest 添加成员请求
type AddMemberRequest struct {
	UserID string                        `json:"user_id" binding:"required"`
	Role   models.ProjectMemberRole      `json:"role" binding:"required"`
}

// AddMember 添加项目成员
// POST /api/release/projects/:id/members
func (s *ReleaseAPI) AddMember(c *gin.Context) {
	projectID := c.Param("id")

	var req AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 检查用户是否存在（搜索 SysUser 表）
	type SysUser struct {
		ID       uint   `gorm:"primaryKey"`
		Username string `gorm:"size:50;uniqueIndex;not null"`
	}
	var user SysUser
	if err := s.db.Table("sys_users").Where("id = ? OR username = ?", req.UserID, req.UserID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInternalError, "User not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, "Failed to find user")
		}
		return
	}

	// 检查是否已是成员
	var existing models.ProjectMember
	userIDStr := strconv.FormatUint(uint64(user.ID), 10) // 转换为字符串存储
	err := s.db.Where("project_id = ? AND user_id = ?", projectID, userIDStr).First(&existing).Error
	if err == nil {
		common.ErrorResp(c, common.CodeInternalError, "User is already a member")
		return
	}

	// 创建成员
	member := &models.ProjectMember{
		ProjectID: projectID,
		UserID:    userIDStr,
		Role:      req.Role,
		AddedBy:   getCurrentUser(c),
	}

	if err := s.db.Create(member).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to add member")
		return
	}

	common.SuccessResp(c, gin.H{
		"success": true,
		"message": "Member added successfully",
		"data": gin.H{
			"id":      member.ID,
			"user_id": userIDStr,
			"role":    member.Role,
		},
	})
}

// UpdateMemberRequest 更新成员请求
type UpdateMemberRequest struct {
	Role models.ProjectMemberRole `json:"role" binding:"required"`
}

// UpdateMember 更新成员角色
// PUT /api/release/projects/:id/members/:user_id
func (s *ReleaseAPI) UpdateMember(c *gin.Context) {
	projectID := c.Param("id")
	userID := c.Param("user_id")

	var req UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 检查成员是否存在
	var member models.ProjectMember
	err := s.db.Where("project_id = ? AND user_id = ?", projectID, userID).First(&member).Error
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Member not found")
		return
	}

	// 更新角色
	if err := s.db.Model(&member).Update("role", req.Role).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to update member")
		return
	}

	common.SuccessResp(c, gin.H{})
}

// RemoveMember 移除项目成员
// DELETE /api/release/projects/:id/members/:user_id
func (s *ReleaseAPI) RemoveMember(c *gin.Context) {
	projectID := c.Param("id")
	userID := c.Param("user_id")

	// 检查是否是最后一个 owner
	var ownerCount int64
	s.db.Model(&models.ProjectMember{}).
		Where("project_id = ? AND role = ?", projectID, models.ProjectRoleOwner).
		Count(&ownerCount)

	var member models.ProjectMember
	err := s.db.Where("project_id = ? AND user_id = ?", projectID, userID).First(&member).Error
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Member not found")
		return
	}

	if member.Role == models.ProjectRoleOwner && ownerCount <= 1 {
		common.ErrorResp(c, common.CodeInternalError, "Cannot remove the last owner")
		return
	}

	// 删除成员
	if err := s.db.Where("project_id = ? AND user_id = ?", projectID, userID).Delete(&models.ProjectMember{}).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to remove member")
		return
	}

	common.SuccessResp(c, gin.H{})
}

// ==================== 用户管理 API ====================

// SearchUsers 搜索用户（搜索系统登录用户 SysUser）
// GET /api/release/users/search
func (s *ReleaseAPI) SearchUsers(c *gin.Context) {
	search := c.Query("q")
	if search == "" {
		common.SuccessResp(c, []interface{}{})
		return
	}

	// 搜索 SysUser 表（系统登录用户）
	query := s.db.Table("sys_users").Where("enable = ?", 1)
	query = query.Where("username LIKE ? OR nick_name LIKE ? OR email LIKE ?",
		"%"+search+"%", "%"+search+"%", "%"+search+"%")

	type SysUserResult struct {
		ID       uint   `json:"id"`
		Username string `json:"username"`
		NickName string `json:"nick_name"`
		Email    string `json:"email"`
		HeaderImg string `json:"header_img"`
	}

	var users []SysUserResult
	err := query.Limit(20).Find(&users).Error

	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to search users")
		return
	}

	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, gin.H{
			"id":          u.ID,
			"username":    u.Username,
			"display_name": u.NickName,
			"email":       u.Email,
			"avatar":      u.HeaderImg,
		})
	}

	common.SuccessResp(c, gin.H{"data": items})
}

// ListUsers 列出所有用户（用于搜索）
// GET /api/release/users
func (s *ReleaseAPI) ListUsers(c *gin.Context) {
	search := c.Query("search")

	query := s.db.Model(&models.User{}).Where("status = ?", "active")
	if search != "" {
		query = query.Where("username LIKE ? OR display_name LIKE ? OR email LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var users []*models.User
	err := query.Limit(50).Find(&users).Error

	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to list users")
		return
	}

	items := make([]gin.H, 0, len(users))
	for _, u := range users {
		items = append(items, gin.H{
			"id":          u.ID,
			"username":    u.Username,
			"display_name": u.DisplayName,
			"email":       u.Email,
			"avatar":      u.Avatar,
		})
	}

	common.SuccessResp(c, gin.H{"data": items})
}

// GetUser 获取用户详情
// GET /api/release/users/:id
func (s *ReleaseAPI) GetUser(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	err := s.db.Where("id = ? OR username = ?", id, id).First(&user).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			common.ErrorResp(c, common.CodeInternalError, "User not found")
		} else {
			common.ErrorResp(c, common.CodeInternalError, "Failed to get user")
		}
		return
	}

	common.SuccessResp(c, gin.H{
		"success": true,
		"data": gin.H{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"email":        user.Email,
			"avatar":       user.Avatar,
			"is_admin":     user.IsAdmin,
			"status":       user.Status,
			"created_at":   user.CreatedAt.Format("2006-01-02 15:04:05"),
		},
	})
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username    string `json:"username" binding:"required"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

// CreateUser 创建用户
// POST /api/release/users
func (s *ReleaseAPI) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	// 检查用户名是否已存在
	var existing models.User
	err := s.db.Where("username = ?", req.Username).First(&existing).Error
	if err == nil {
		common.ErrorResp(c, common.CodeInternalError, "Username already exists")
		return
	}

	user := &models.User{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Status:      "active",
	}

	if err := s.db.Create(user).Error; err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to create user")
		return
	}

	common.SuccessResp(c, gin.H{
		"success": true,
		"message": "User created successfully",
		"data": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}
