package store

import (
	"context"

	"github.com/voilet/quic-flow/pkg/task/models"
	"gorm.io/gorm"
)

// ClientTagStore 客户端标签存储接口
type ClientTagStore interface {
	// 标签管理
	Create(ctx context.Context, tag *models.ClientTag) error
	Update(ctx context.Context, tag *models.ClientTag) error
	Delete(ctx context.Context, tagID uint) error
	GetByID(ctx context.Context, tagID uint) (*models.ClientTag, error)
	GetByName(ctx context.Context, name string) (*models.ClientTag, error)
	List(ctx context.Context) ([]*models.ClientTag, error)
	ListWithCount(ctx context.Context) ([]*models.ClientTagWithCount, error)

	// 客户端标签关系
	GetClients(ctx context.Context, tagID uint) ([]string, error)
	GetClientTags(ctx context.Context, clientID string) ([]*models.ClientTag, error)
	AddClients(ctx context.Context, tagID uint, clientIDs []string) error
	RemoveClient(ctx context.Context, tagID uint, clientID string) error
	SetClientTags(ctx context.Context, clientID string, tagIDs []uint) error
	BatchSetClientTags(ctx context.Context, clientIDs []string, tagIDs []uint) error

	// 查询
	GetClientsByTags(ctx context.Context, tagIDs []uint) ([]string, error)
}

// clientTagStoreImpl 客户端标签存储实现
type clientTagStoreImpl struct {
	db *gorm.DB
}

// NewClientTagStore 创建客户端标签存储
func NewClientTagStore(db *gorm.DB) ClientTagStore {
	return &clientTagStoreImpl{db: db}
}

// Create 创建标签
func (s *clientTagStoreImpl) Create(ctx context.Context, tag *models.ClientTag) error {
	return s.db.WithContext(ctx).Create(tag).Error
}

// Update 更新标签
func (s *clientTagStoreImpl) Update(ctx context.Context, tag *models.ClientTag) error {
	return s.db.WithContext(ctx).Model(tag).Updates(map[string]interface{}{
		"name":        tag.Name,
		"color":       tag.Color,
		"description": tag.Description,
	}).Error
}

// Delete 删除标签（软删除）
func (s *clientTagStoreImpl) Delete(ctx context.Context, tagID uint) error {
	// 先删除关联关系
	if err := s.db.WithContext(ctx).
		Where("tag_id = ?", tagID).
		Delete(&models.ClientTagRelation{}).Error; err != nil {
		return err
	}
	// 再删除标签
	return s.db.WithContext(ctx).Delete(&models.ClientTag{}, tagID).Error
}

// GetByID 根据ID获取标签
func (s *clientTagStoreImpl) GetByID(ctx context.Context, tagID uint) (*models.ClientTag, error) {
	var tag models.ClientTag
	err := s.db.WithContext(ctx).First(&tag, tagID).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// GetByName 根据名称获取标签
func (s *clientTagStoreImpl) GetByName(ctx context.Context, name string) (*models.ClientTag, error) {
	var tag models.ClientTag
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&tag).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// List 列表查询标签
func (s *clientTagStoreImpl) List(ctx context.Context) ([]*models.ClientTag, error) {
	var tags []*models.ClientTag
	err := s.db.WithContext(ctx).
		Order("created_at DESC").
		Find(&tags).Error
	return tags, err
}

// ListWithCount 列表查询标签（带客户端数量）
func (s *clientTagStoreImpl) ListWithCount(ctx context.Context) ([]*models.ClientTagWithCount, error) {
	var tags []*models.ClientTagWithCount

	err := s.db.WithContext(ctx).
		Model(&models.ClientTag{}).
		Select("tb_client_tag.*, COUNT(tb_client_tag_relation.client_id) as client_count").
		Joins("LEFT JOIN tb_client_tag_relation ON tb_client_tag.id = tb_client_tag_relation.tag_id").
		Group("tb_client_tag.id").
		Order("tb_client_tag.created_at DESC").
		Scan(&tags).Error

	return tags, err
}

// GetClients 获取标签下的客户端列表
func (s *clientTagStoreImpl) GetClients(ctx context.Context, tagID uint) ([]string, error) {
	var clientIDs []string
	err := s.db.WithContext(ctx).
		Model(&models.ClientTagRelation{}).
		Where("tag_id = ?", tagID).
		Pluck("client_id", &clientIDs).Error
	return clientIDs, err
}

// GetClientTags 获取客户端的所有标签
func (s *clientTagStoreImpl) GetClientTags(ctx context.Context, clientID string) ([]*models.ClientTag, error) {
	var tags []*models.ClientTag
	err := s.db.WithContext(ctx).
		Joins("JOIN tb_client_tag_relation ON tb_client_tag.id = tb_client_tag_relation.tag_id").
		Where("tb_client_tag_relation.client_id = ?", clientID).
		Find(&tags).Error
	return tags, err
}

// AddClients 添加客户端到标签（批量）
func (s *clientTagStoreImpl) AddClients(ctx context.Context, tagID uint, clientIDs []string) error {
	if len(clientIDs) == 0 {
		return nil
	}

	// 获取已存在的关联，避免重复插入
	var existingClientIDs []string
	s.db.WithContext(ctx).
		Model(&models.ClientTagRelation{}).
		Where("tag_id = ? AND client_id IN ?", tagID, clientIDs).
		Pluck("client_id", &existingClientIDs)

	existingSet := make(map[string]bool)
	for _, id := range existingClientIDs {
		existingSet[id] = true
	}

	// 创建新的关联
	var relations []*models.ClientTagRelation
	for _, clientID := range clientIDs {
		if !existingSet[clientID] {
			relations = append(relations, &models.ClientTagRelation{
				ClientID: clientID,
				TagID:    tagID,
			})
		}
	}

	if len(relations) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Create(&relations).Error
}

// RemoveClient 从标签移除客户端
func (s *clientTagStoreImpl) RemoveClient(ctx context.Context, tagID uint, clientID string) error {
	return s.db.WithContext(ctx).
		Where("tag_id = ? AND client_id = ?", tagID, clientID).
		Delete(&models.ClientTagRelation{}).Error
}

// SetClientTags 设置客户端的标签（覆盖）
func (s *clientTagStoreImpl) SetClientTags(ctx context.Context, clientID string, tagIDs []uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除现有关联
		if err := tx.Where("client_id = ?", clientID).
			Delete(&models.ClientTagRelation{}).Error; err != nil {
			return err
		}

		// 创建新关联
		if len(tagIDs) == 0 {
			return nil
		}

		var relations []*models.ClientTagRelation
		for _, tagID := range tagIDs {
			relations = append(relations, &models.ClientTagRelation{
				ClientID: clientID,
				TagID:    tagID,
			})
		}

		return tx.Create(&relations).Error
	})
}

// BatchSetClientTags 批量设置客户端标签
func (s *clientTagStoreImpl) BatchSetClientTags(ctx context.Context, clientIDs []string, tagIDs []uint) error {
	if len(clientIDs) == 0 || len(tagIDs) == 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 删除现有关联
		if err := tx.Where("client_id IN ?", clientIDs).
			Delete(&models.ClientTagRelation{}).Error; err != nil {
			return err
		}

		// 创建新关联
		var relations []*models.ClientTagRelation
		for _, clientID := range clientIDs {
			for _, tagID := range tagIDs {
				relations = append(relations, &models.ClientTagRelation{
					ClientID: clientID,
					TagID:    tagID,
				})
			}
		}

		return tx.CreateInBatches(relations, 100).Error
	})
}

// GetClientsByTags 根据标签获取客户端（拥有所有指定标签的客户端）
func (s *clientTagStoreImpl) GetClientsByTags(ctx context.Context, tagIDs []uint) ([]string, error) {
	if len(tagIDs) == 0 {
		return []string{}, nil
	}

	var clientIDs []string

	// 查询拥有所有指定标签的客户端
	// SELECT client_id FROM tb_client_tag_relation
	// WHERE tag_id IN (?)
	// GROUP BY client_id
	// HAVING COUNT(DISTINCT tag_id) = ?
	err := s.db.WithContext(ctx).
		Model(&models.ClientTagRelation{}).
		Select("client_id").
		Where("tag_id IN ?", tagIDs).
		Group("client_id").
		Having("COUNT(DISTINCT tag_id) = ?", len(tagIDs)).
		Pluck("client_id", &clientIDs).Error

	return clientIDs, err
}
