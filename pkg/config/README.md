# 配置中心数据模型和存储层

## 概述

本包实现了配置中心的数据模型和数据库存储层，支持：

- 配置项管理 (CRUD)
- 配置发布与版本控制
- 灰度发布规则
- 客户端订阅管理
- 配置变更历史
- 配置推送消息
- 配置快照与回滚
- 编辑锁机制

## 数据模型

### 核心模型

1. **Config** - 配置项
   - 支持多格式：JSON、YAML、Properties、TEXT、XML
   - 版本控制
   - 标签分类
   - 加密存储

2. **ConfigRelease** - 配置发布记录
   - 发布类型：全量、回滚、灰度
   - 发布状态跟踪
   - 灰度关联

3. **GrayRule** - 灰度发布规则
   - 按标签匹配
   - 按 IP 匹配
   - 按客户端 ID 匹配
   - 按百分比匹配

4. **ConfigSubscriber** - 配置订阅者
   - 客户端连接管理
   - 心跳检测
   - 订阅关系

5. **ConfigChangeLog** - 配置变更日志
   - 完整的操作记录
   - Diff 对比

6. **ConfigPushMessage** - 配置推送消息
   - 推送状态跟踪
   - ACK 确认

7. **ConfigSnapshot** - 配置快照
   - 快速回滚
   - 过期清理

8. **ConfigEditLock** - 配置编辑锁
   - 防止并发编辑冲突

## 使用示例

### 初始化存储层

```go
import (
    "github.com/voilet/quic-flow/pkg/config"
    "gorm.io/gorm"
)

// 创建存储层
db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
store := config.NewStore(db)
```

### 配置管理

```go
ctx := context.Background()

// 创建配置
cfg := &config.Config{
    Namespace:  "application",
    Group:      "database",
    DataID:     "mysql-config",
    ConfigType: config.ConfigTypeApplication,
    Content:    `{"host":"localhost","port":3306}`,
    Format:     config.ConfigFormatJSON,
    Tags:       config.StringArray{"production", "mysql"},
}
err := store.CreateConfig(ctx, cfg)

// 查询配置
cfg, err := store.GetConfigByKeys(ctx, "application", "database", "mysql-config")

// 列出配置
filter := &config.ConfigFilter{
    Namespace:  "application",
    ConfigType: config.ConfigTypeApplication,
    Page:       1,
    PageSize:   20,
}
configs, total, err := store.ListConfigs(ctx, filter)
```

### 发布配置

```go
// 创建发布记录
release := &config.ConfigRelease{
    ConfigID:    cfg.ID,
    Namespace:   cfg.Namespace,
    Group:       cfg.Group,
    DataID:      cfg.DataID,
    Content:     cfg.Content,
    Version:     cfg.Version,
    ReleaseType: config.ReleaseTypeFull,
    Status:      config.ReleaseStatusSuccess,
    ReleasedBy:  "admin",
}
err := store.CreateRelease(ctx, release)

// 更新发布状态
err = store.UpdateReleaseStatus(ctx, release.ID, config.ReleaseStatusSuccess)
```

### 灰度发布

```go
// 创建灰度规则
rule := &config.GrayRule{
    ConfigID:    cfg.ID,
    RuleName:    "beta-testers",
    RuleType:    config.RuleTypeTag,
    RuleValue:   `["beta","test"]`,
    Enabled:     true,
    Priority:    10,
    Description: "Beta 用户灰度",
    CreatedBy:   "admin",
}
err := store.CreateGrayRule(ctx, rule)

// 查询启用的灰度规则
rules, err := store.GetEnabledGrayRules(ctx, cfg.ID)
```

### 订阅管理

```go
// 注册订阅者
subscriber := &config.ConfigSubscriber{
    ClientID:      "client-001",
    SDKType:       "go",
    Namespace:     "application",
    Subscriptions: config.StringArray{"database:mysql-config"},
    ClientTags:    config.StringArray{"beta", "production"},
    Status:        config.SubscriberStatusOnline,
}
err := store.RegisterSubscriber(ctx, subscriber)

// 更新心跳
err := store.UpdateSubscriberHeartbeat(ctx, "client-001")
```

### 编辑锁

```go
// 获取编辑锁（TTL 5分钟）
lock, err := store.AcquireEditLock(ctx, cfg.ID, "admin", "session-123", 5*time.Minute)
if err != nil {
    // 配置已被锁定
    log.Println(err)
}

// 释放编辑锁
err = store.ReleaseEditLock(ctx, cfg.ID, "session-123")
```

## 数据库迁移

### 自动迁移

在主程序启动时调用：

```go
import (
    "github.com/voilet/quic-flow/pkg/config"
    "gorm.io/gorm"
)

// 迁移配置中心表
err := config.MigrateConfigCenter(db)
```

### 手动迁移

```go
// 只迁移模型
err := config.AutoMigrateConfig(db)

// 迁移并创建索引
err := config.MigrateConfigCenter(db)
```

## 数据库支持

- PostgreSQL (推荐)
  - 支持 JSONB 类型
  - 支持 GIN 索引
  - 完整功能支持

- MySQL
  - 使用 JSON 类型
  - 基本功能支持

## 性能优化

1. **索引优化**
   - 唯一索引：(namespace, group, data_id)
   - 复合索引用于常见查询
   - JSONB GIN 索引用于标签查询

2. **缓存建议**
   - 配置内容可使用 Redis 缓存
   - 订阅者状态可本地缓存
   - 灰度规则可预加载到内存

3. **批量操作**
   - 推送消息支持批量查询
   - 订阅者列表支持分页

## 扩展性

### 添加新的配置格式

在 `ConfigFormat` 枚举中添加新值：

```go
const (
    // ... 现有格式
    ConfigFormatTOML ConfigFormat = "toml"
)
```

### 自定义灰度规则类型

在 `RuleType` 枚举中添加新类型：

```go
const (
    // ... 现有类型
    RuleTypeRegion RuleType = "region"
)
```

## 测试

运行单元测试：

```bash
go test -v ./pkg/config/...
```

## 注意事项

1. **编辑锁超时**
   - 默认 TTL 需要根据业务场景设置
   - 客户端断开时应主动释放锁

2. **快照过期**
   - 定期清理过期快照
   - 建议保留最近 30 天的快照

3. **推送消息重试**
   - 推送失败需要重试机制
   - 注意避免重复推送

4. **并发控制**
   - 使用编辑锁防止并发修改
   - 版本号确保配置一致性

## 相关文档

- [配置中心 API 设计](../../docs/plans/config-center-design.md)
- [配置中心前端页面](../../docs/plans/config-center-frontend.md)
