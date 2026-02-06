package configcenter

import (
	"fmt"
	"strings"
	"gorm.io/gorm"
)

// MigrateConfigCenter 执行配置中心数据库迁移
func MigrateConfigCenter(db *gorm.DB) error {
	fmt.Println("开始配置中心数据库迁移...")

	// 自动迁移所有配置中心模型
	// 忽略列已存在的错误（可能是在更新现有表结构）
	if err := AutoMigrateConfig(db); err != nil {
		errStr := err.Error()
		// PostgreSQL: column "group" of relation "configs" already exists
		// MySQL: Duplicate column name 'group'
		if strings.Contains(errStr, "already exists") || strings.Contains(errStr, "Duplicate column") {
			fmt.Printf("警告: 部分列已存在，跳过创建: %v\n", err)
		} else {
			return fmt.Errorf("配置中心模型迁移失败: %w", err)
		}
	}

	// 创建额外索引
	if err := createConfigIndexes(db); err != nil {
		return fmt.Errorf("配置中心索引创建失败: %w", err)
	}

	fmt.Println("配置中心数据库迁移完成")
	return nil
}


// createConfigIndexes 创建配置中心额外索引
func createConfigIndexes(db *gorm.DB) error {
	// 检测数据库类型
	dbType := db.Dialector.Name()

	var indexes []string

	if dbType == "postgres" {
		// PostgreSQL 索引
		indexes = []string{
			// 配置表索引
			`CREATE INDEX IF NOT EXISTS idx_configs_namespace ON configs(namespace)`,
			`CREATE INDEX IF NOT EXISTS idx_configs_group ON configs("group")`,
			`CREATE INDEX IF NOT EXISTS idx_configs_type ON configs(config_type)`,
			`CREATE INDEX IF NOT EXISTS idx_configs_tags ON configs USING GIN(tags)`,

			// 发布记录索引
			`CREATE INDEX IF NOT EXISTS idx_config_releases_namespace ON config_releases(namespace)`,
			`CREATE INDEX IF NOT EXISTS idx_config_releases_type_status ON config_releases(release_type, status)`,
			`CREATE INDEX IF NOT EXISTS idx_config_releases_released_at ON config_releases(released_at DESC)`,

			// 灰度规则索引
			`CREATE INDEX IF NOT EXISTS idx_gray_rules_enabled ON gray_rules(enabled, priority DESC)`,

			// 订阅者索引
			`CREATE INDEX IF NOT EXISTS idx_config_subscribers_status ON config_subscribers(status)`,
			`CREATE INDEX IF NOT EXISTS idx_config_subscribers_last_active ON config_subscribers(last_active DESC)`,

			// 变更日志索引
			`CREATE INDEX IF NOT EXISTS idx_config_change_logs_operated_at ON config_change_logs(operated_at DESC)`,

			// 推送消息索引
			`CREATE INDEX IF NOT EXISTS idx_config_push_messages_status ON config_push_messages(status, created_at)`,

			// 快照索引
			`CREATE INDEX IF NOT EXISTS idx_config_snapshots_expires_at ON config_snapshots(expires_at)`,

			// 编辑锁索引
			`CREATE INDEX IF NOT EXISTS idx_config_edit_locks_expires_at ON config_edit_locks(expires_at)`,
		}
	} else {
		// MySQL 索引
		indexes = []string{
			// 配置表索引
			`CREATE INDEX IF NOT EXISTS idx_configs_namespace ON configs(namespace)`,
			`CREATE INDEX IF NOT EXISTS idx_configs_group ON configs(` + "`group`" + `)`,
			`CREATE INDEX IF NOT EXISTS idx_configs_type ON configs(config_type)`,

			// 发布记录索引
			`CREATE INDEX IF NOT EXISTS idx_config_releases_namespace ON config_releases(namespace)`,
			`CREATE INDEX IF NOT EXISTS idx_config_releases_type_status ON config_releases(release_type, status)`,
			`CREATE INDEX IF NOT EXISTS idx_config_releases_released_at ON config_releases(released_at)`,

			// 灰度规则索引
			`CREATE INDEX IF NOT EXISTS idx_gray_rules_enabled ON gray_rules(enabled, priority)`,

			// 订阅者索引
			`CREATE INDEX IF NOT EXISTS idx_config_subscribers_status ON config_subscribers(status)`,
			`CREATE INDEX IF NOT EXISTS idx_config_subscribers_last_active ON config_subscribers(last_active)`,

			// 变更日志索引
			`CREATE INDEX IF NOT EXISTS idx_config_change_logs_operated_at ON config_change_logs(operated_at)`,

			// 推送消息索引
			`CREATE INDEX IF NOT EXISTS idx_config_push_messages_status ON config_push_messages(status, created_at)`,

			// 快照索引
			`CREATE INDEX IF NOT EXISTS idx_config_snapshots_expires_at ON config_snapshots(expires_at)`,

			// 编辑锁索引
			`CREATE INDEX IF NOT EXISTS idx_config_edit_locks_expires_at ON config_edit_locks(expires_at)`,
		}
	}

	// 执行索引创建
	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			// 索引创建失败不中断，记录警告即可
			fmt.Printf("警告: 索引创建失败 (可能已存在): %v\n", err)
		}
	}

	return nil
}

// DropConfigTables 删除所有配置中心表 (仅用于测试)
func DropConfigTables(db *gorm.DB) error {
	tables := []string{
		"config_edit_locks",
		"config_snapshots",
		"config_push_messages",
		"config_change_logs",
		"config_subscribers",
		"gray_rules",
		"config_releases",
		"configs",
	}

	for _, table := range tables {
		if err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)).Error; err != nil {
			return err
		}
	}

	return nil
}
