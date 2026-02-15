package models

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// AllModels 所有需要迁移的模型
// 注意：TaskGroupRelation 由 GORM many2many 自动创建，不需要单独迁移
var AllModels = []interface{}{
	&Task{},
	&TaskGroup{},
	&Execution{},
	&Client{},
	&ClientTag{},
	&ClientTagRelation{},
}

// Migrate 执行数据库迁移
func Migrate(db *gorm.DB) error {
	// 检测数据库类型
	dbType := detectDBType(db)

	if dbType == "postgres" {
		// PostgreSQL: 启用必要的扩展
		extensions := []string{
			`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,
		}
		for _, ext := range extensions {
			if err := db.Exec(ext).Error; err != nil {
				// 忽略扩展创建错误（可能已存在或权限不足）
			}
		}
	}

	// 对于 PostgreSQL，需要先手动创建表结构（因为 GORM 的 AutoMigrate 可能无法正确处理类型转换）
	if dbType == "postgres" {
		// 先尝试删除可能存在的旧表（如果类型不兼容）
		// 注意：这会在开发环境中重置表，生产环境需要谨慎
		// 这里我们只处理类型转换问题，不删除表
	}

	// 自动迁移所有模型
	for _, model := range AllModels {
		if err := db.AutoMigrate(model); err != nil {
			// 对于 SQLite，忽略索引已存在的错误
			if dbType == "sqlite" {
				errStr := err.Error()
				if contains(errStr, "already exists") || contains(errStr, "duplicate") {
					continue
				}
			}
			// 对于 PostgreSQL，如果是类型错误，尝试修复
			if dbType == "postgres" {
				errStr := err.Error()
				if contains(errStr, "tinyint") || contains(errStr, "does not exist") {
					// 尝试修复类型问题
					if fixErr := fixPostgresTypes(db, model); fixErr != nil {
						return fmt.Errorf("failed to migrate %T: %w (fix error: %v)", model, err, fixErr)
					}
					// 修复后重试
					if err := db.AutoMigrate(model); err != nil {
						return fmt.Errorf("failed to migrate %T after fix: %w", model, err)
					}
					continue
				}
			}
			return err
		}
	}

	// 创建额外索引（仅对 PostgreSQL 和 MySQL）
	// 注意：GORM AutoMigrate 会自动创建单列索引，这里只创建复合索引
	// 对于 SQLite，跳过索引创建（测试环境）
	if dbType != "sqlite" {
		_ = createIndexes(db, dbType) // 忽略错误，索引可能已存在
	}

	return nil
}

// detectDBType 检测数据库类型
func detectDBType(db *gorm.DB) string {
	dialectorName := db.Dialector.Name()
	switch dialectorName {
	case "mysql":
		return "mysql"
	case "sqlite":
		return "sqlite"
	default:
		return "postgres"
	}
}

// createIndexes 创建额外索引（仅复合索引，单列索引由 GORM AutoMigrate 自动创建）
func createIndexes(db *gorm.DB, dbType string) error {
	// 复合索引（GORM AutoMigrate 不会自动创建）
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_task_group ON tb_task_group_relation(task_id, group_id)",
		"CREATE INDEX IF NOT EXISTS idx_execution_task_client ON tb_execution(task_id, client_id)",
		"CREATE INDEX IF NOT EXISTS idx_execution_status_time ON tb_execution(status, created_at)",
		"CREATE INDEX IF NOT EXISTS idx_client_tag_client ON tb_client_tag_relation(client_id)",
		"CREATE INDEX IF NOT EXISTS idx_client_tag_tag ON tb_client_tag_relation(tag_id)",
	}

	for _, idx := range indexes {
		if err := db.Exec(idx).Error; err != nil {
			// 忽略索引已存在的错误（不同数据库的错误信息可能不同）
			errStr := err.Error()
			if contains(errStr, "already exists") || contains(errStr, "duplicate") || 
			   contains(errStr, "IF NOT EXISTS") {
				continue
			}
			// 对于 SQLite，忽略所有索引创建错误（因为我们在 SQLite 中不创建索引）
			if dbType == "sqlite" {
				continue
			}
			// 其他错误需要返回
			return err
		}
	}

	return nil
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// fixPostgresTypes 修复 PostgreSQL 类型问题（将 tinyint 转换为 smallint）
func fixPostgresTypes(db *gorm.DB, model interface{}) error {
	// 获取表名
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return err
	}
	tableName := stmt.Schema.Table

	// 检查表是否存在
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		)
	`, tableName).Scan(&exists).Error; err != nil {
		return err
	}

	if !exists {
		// 表不存在，不需要修复
		return nil
	}

	// 检查列是否存在且类型为 tinyint，如果是则转换为 smallint
	alterSQLs := []string{}
	
	// 根据表名修复特定字段
	switch tableName {
	case "tb_task":
		// 检查并修复 executor_type
		var colType string
		if err := db.Raw(`
			SELECT data_type FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = 'tb_task' 
			AND column_name = 'executor_type'
		`).Scan(&colType).Error; err == nil {
			if colType == "smallint" || colType == "integer" {
				// 类型已经是正确的，不需要修复
			} else {
				// 尝试转换为 smallint
				alterSQLs = append(alterSQLs, 
					"ALTER TABLE tb_task ALTER COLUMN executor_type TYPE smallint USING CASE WHEN executor_type IS NULL THEN NULL ELSE executor_type::smallint END")
			}
		}
		
		// 检查并修复 status
		if err := db.Raw(`
			SELECT data_type FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = 'tb_task' 
			AND column_name = 'status'
		`).Scan(&colType).Error; err == nil {
			if colType == "smallint" || colType == "integer" {
				// 类型已经是正确的，不需要修复
			} else {
				alterSQLs = append(alterSQLs,
					"ALTER TABLE tb_task ALTER COLUMN status TYPE smallint USING CASE WHEN status IS NULL THEN NULL ELSE status::smallint END")
			}
		}
		
	case "tb_execution":
		// 检查并修复 execution_type
		var colType string
		if err := db.Raw(`
			SELECT data_type FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = 'tb_execution' 
			AND column_name = 'execution_type'
		`).Scan(&colType).Error; err == nil {
			if colType == "smallint" || colType == "integer" {
				// 类型已经是正确的，不需要修复
			} else {
				alterSQLs = append(alterSQLs,
					"ALTER TABLE tb_execution ALTER COLUMN execution_type TYPE smallint USING CASE WHEN execution_type IS NULL THEN NULL ELSE execution_type::smallint END")
			}
		}
		
		// 检查并修复 status
		if err := db.Raw(`
			SELECT data_type FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = 'tb_execution' 
			AND column_name = 'status'
		`).Scan(&colType).Error; err == nil {
			if colType == "smallint" || colType == "integer" {
				// 类型已经是正确的，不需要修复
			} else {
				alterSQLs = append(alterSQLs,
					"ALTER TABLE tb_execution ALTER COLUMN status TYPE smallint USING CASE WHEN status IS NULL THEN NULL ELSE status::smallint END")
			}
		}
	}

	// 执行 ALTER TABLE 语句
	for _, sql := range alterSQLs {
		if err := db.Exec(sql).Error; err != nil {
			// 如果字段不存在或类型已经是正确的，忽略错误
			errStr := err.Error()
			if contains(errStr, "does not exist") || contains(errStr, "cannot cast") || 
			   contains(errStr, "column") && contains(errStr, "does not exist") {
				continue
			}
			// 记录错误但不中断（可能是权限问题或其他原因）
		}
	}

	return nil
}
