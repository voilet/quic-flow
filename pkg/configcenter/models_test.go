package configcenter

import (
	"encoding/json"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 测试 StringArray 类型
func TestStringArray(t *testing.T) {
	// 测试 Value 方法
	arr := StringArray{"tag1", "tag2", "tag3"}
	val, err := arr.Value()
	if err != nil {
		t.Fatalf("StringArray.Value 失败: %v", err)
	}
	if val == nil {
		t.Fatal("期望非空值")
	}

	// 验证 JSON 编码
	var decoded []string
	if err := json.Unmarshal(val.([]byte), &decoded); err != nil {
		t.Fatalf("无法解码 JSON: %v", err)
	}
	if len(decoded) != len(arr) {
		t.Errorf("期望长度=%d, 实际=%d", len(arr), len(decoded))
	}

	// 测试 Scan 方法
	var scanArr StringArray
	if err := scanArr.Scan(val); err != nil {
		t.Fatalf("StringArray.Scan 失败: %v", err)
	}
	if len(scanArr) != len(arr) {
		t.Errorf("期望长度=%d, 实际=%d", len(arr), len(scanArr))
	}

	// 测试 nil 值
	var nilArr StringArray
	val, err = nilArr.Value()
	if err != nil {
		t.Fatalf("nil StringArray.Value 失败: %v", err)
	}
	if val != nil {
		t.Error("期望 nil 值")
	}

	t.Log("StringArray 测试通过")
}

// 测试 JSONMap 类型
func TestJSONMap(t *testing.T) {
	// 测试 Value 方法
	m := JSONMap{"key1": "value1", "key2": 123}
	val, err := m.Value()
	if err != nil {
		t.Fatalf("JSONMap.Value 失败: %v", err)
	}
	if val == nil {
		t.Fatal("期望非空值")
	}

	// 测试 Scan 方法
	var scanMap JSONMap
	if err := scanMap.Scan(val); err != nil {
		t.Fatalf("JSONMap.Scan 失败: %v", err)
	}
	if len(scanMap) != len(m) {
		t.Errorf("期望长度=%d, 实际=%d", len(m), len(scanMap))
	}

	// 测试 nil 值
	var nilMap JSONMap
	val, err = nilMap.Value()
	if err != nil {
		t.Fatalf("nil JSONMap.Value 失败: %v", err)
	}
	if val != nil {
		t.Error("期望 nil 值")
	}

	t.Log("JSONMap 测试通过")
}

// 测试模型常量
func TestModelConstants(t *testing.T) {
	// 测试 ConfigType 常量
	tests := []struct {
		name  string
		value ConfigType
	}{
		{"application", ConfigTypeApplication},
		{"system", ConfigTypeSystem},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.name {
				t.Errorf("ConfigType 常量值不正确: 期望=%s, 实际=%s", tt.name, tt.value)
			}
		})
	}

	// 测试 ConfigFormat 常量
	formatTests := []struct {
		name  string
		value ConfigFormat
	}{
		{"json", ConfigFormatJSON},
		{"yaml", ConfigFormatYAML},
		{"properties", ConfigFormatProperties},
		{"text", ConfigFormatTEXT},
		{"xml", ConfigFormatXML},
	}

	for _, tt := range formatTests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.name {
				t.Errorf("ConfigFormat 常量值不正确: 期望=%s, 实际=%s", tt.name, tt.value)
			}
		})
	}

	// 测试 ReleaseType 常量
	releaseTypeTests := []struct {
		name  string
		value ReleaseType
	}{
		{"full", ReleaseTypeFull},
		{"rollback", ReleaseTypeRollback},
		{"gray", ReleaseTypeGray},
	}

	for _, tt := range releaseTypeTests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.name {
				t.Errorf("ReleaseType 常量值不正确: 期望=%s, 实际=%s", tt.name, tt.value)
			}
		})
	}

	// 测试 ReleaseStatus 常量
	statusTests := []struct {
		name  string
		value ReleaseStatus
	}{
		{"pending", ReleaseStatusPending},
		{"publishing", ReleaseStatusPublishing},
		{"success", ReleaseStatusSuccess},
		{"failed", ReleaseStatusFailed},
		{"partial", ReleaseStatusPartial},
	}

	for _, tt := range statusTests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.value) != tt.name {
				t.Errorf("ReleaseStatus 常量值不正确: 期望=%s, 实际=%s", tt.name, tt.value)
			}
		})
	}

	t.Log("模型常量测试通过")
}

// TestTableNames 测试表名
func TestTableNames(t *testing.T) {
	tests := []struct {
		model     interface{}
		tableName string
	}{
		{Config{}, "configs"},
		{ConfigRelease{}, "config_releases"},
		{GrayRule{}, "gray_rules"},
		{ConfigSubscriber{}, "config_subscribers"},
		{ConfigChangeLog{}, "config_change_logs"},
		{ConfigPushMessage{}, "config_push_messages"},
		{ConfigSnapshot{}, "config_snapshots"},
		{ConfigEditLock{}, "config_edit_locks"},
	}

	for _, tt := range tests {
		t.Run(tt.tableName, func(t *testing.T) {
			var stmt gorm.Statement
			stmt.DB, _ = gorm.Open(postgres.Open(""), &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			})
			stmt.Parse(tt.model)

			if stmt.Table != tt.tableName {
				t.Errorf("表名不正确: 期望=%s, 实际=%s", tt.tableName, stmt.Table)
			}
		})
	}

	t.Log("表名测试通过")
}

// TestAllConfigModels 测试所有配置模型列表
func TestAllConfigModels(t *testing.T) {
	if len(AllConfigModels) == 0 {
		t.Fatal("AllConfigModels 不应为空")
	}

	expectedModels := 8 // 8个模型
	if len(AllConfigModels) != expectedModels {
		t.Errorf("模型数量不正确: 期望=%d, 实际=%d", expectedModels, len(AllConfigModels))
	}

	t.Logf("配置中心模型数量: %d", len(AllConfigModels))
}
