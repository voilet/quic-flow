package models

import (
	"time"

	"gorm.io/gorm"
)

// ClientTag 客户端标签表
type ClientTag struct {
	ID          uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string         `gorm:"size:64;uniqueIndex;not null;comment:标签名称" json:"name"`
	Color       string         `gorm:"size:16;default:'#409EFF';comment:标签颜色" json:"color"`
	Description string         `gorm:"size:256;comment:标签描述" json:"description"`
	CreatedAt   time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// 关联关系
	Clients []Client `gorm:"many2many:tb_client_tag_relation;" json:"clients,omitempty"`
}

// TableName 指定表名
func (ClientTag) TableName() string {
	return "tb_client_tag"
}

// ClientTagRelation 客户端标签关联表
type ClientTagRelation struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	ClientID  string    `gorm:"size:128;not null;uniqueIndex:idx_client_tag;comment:客户端ID" json:"client_id"`
	TagID     uint      `gorm:"not null;uniqueIndex:idx_client_tag;comment:标签ID" json:"tag_id"`
	CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
}

// TableName 指定表名
func (ClientTagRelation) TableName() string {
	return "tb_client_tag_relation"
}

// ClientTagWithCount 带客户端数量的标签（用于列表展示）
type ClientTagWithCount struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	Description string    `json:"description"`
	ClientCount int64     `json:"client_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
