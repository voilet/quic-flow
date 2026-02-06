package alert

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestJSONMap 测试 JSONMap 类型
func TestJSONMap(t *testing.T) {
	// 创建 JSONMap
	j := JSONMap{
		"key1": "value1",
		"key2": 123,
		"key3": true,
	}

	// 测试 Value 方法
	val, err := j.Value()
	assert.NoError(t, err)
	assert.NotNil(t, val)

	// 测试 Scan 方法
	var j2 JSONMap
	err = j2.Scan(val)
	assert.NoError(t, err)
	assert.Equal(t, j["key1"], j2["key1"])
	// JSON 数字反序列化为 float64
	assert.Equal(t, float64(123), j2["key2"])
	assert.Equal(t, j["key3"], j2["key3"])

	// 测试空 JSONMap 的 Scan
	var j3 JSONMap
	err = j3.Scan(nil)
	assert.NoError(t, err)
	assert.NotNil(t, j3)
}

// TestAlertSeverityString 测试告警严重程度字符串值
func TestAlertSeverityString(t *testing.T) {
	assert.Equal(t, "critical", string(AlertSeverityCritical))
	assert.Equal(t, "warning", string(AlertSeverityWarning))
	assert.Equal(t, "info", string(AlertSeverityInfo))
}

// TestAlertStatusString 测试告警状态字符串值
func TestAlertStatusString(t *testing.T) {
	assert.Equal(t, "firing", string(AlertStatusFiring))
	assert.Equal(t, "resolved", string(AlertStatusResolved))
	assert.Equal(t, "silenced", string(AlertStatusSilenced))
}

// TestNotifyChannelTypeString 测试通知渠道类型字符串值
func TestNotifyChannelTypeString(t *testing.T) {
	assert.Equal(t, "webhook", string(ChannelTypeWebhook))
	assert.Equal(t, "email", string(ChannelTypeEmail))
	assert.Equal(t, "dingtalk", string(ChannelTypeDingTalk))
	assert.Equal(t, "wechat", string(ChannelTypeWeChat))
	assert.Equal(t, "feishu", string(ChannelTypeFeishu))
	assert.Equal(t, "slack", string(ChannelTypeSlack))
}

// TestAlertRuleTableName 测试 AlertRule 表名
func TestAlertRuleTableName(t *testing.T) {
	rule := AlertRule{}
	assert.Equal(t, "alert_rules", rule.TableName())
}

// TestAlertInstanceTableName 测试 AlertInstance 表名
func TestAlertInstanceTableName(t *testing.T) {
	instance := AlertInstance{}
	assert.Equal(t, "alert_instances", instance.TableName())
}

// TestSilenceRuleTableName 测试 SilenceRule 表名
func TestSilenceRuleTableName(t *testing.T) {
	silence := SilenceRule{}
	assert.Equal(t, "alert_silence_rules", silence.TableName())
}

// TestNotifyChannelTableName 测试 NotifyChannel 表名
func TestNotifyChannelTableName(t *testing.T) {
	channel := NotifyChannel{}
	assert.Equal(t, "alert_notify_channels", channel.TableName())
}

// TestNotifyHistoryTableName 测试 NotifyHistory 表名
func TestNotifyHistoryTableName(t *testing.T) {
	history := NotifyHistory{}
	assert.Equal(t, "alert_notify_history", history.TableName())
}

// TestOnCallScheduleTableName 测试 OnCallSchedule 表名
func TestOnCallScheduleTableName(t *testing.T) {
	schedule := OnCallSchedule{}
	assert.Equal(t, "alert_oncall_schedules", schedule.TableName())
}

// TestOnCallUserTableName 测试 OnCallUser 表名
func TestOnCallUserTableName(t *testing.T) {
	user := OnCallUser{}
	assert.Equal(t, "alert_oncall_users", user.TableName())
}

// TestAlertRuleDefaultValues 测试 AlertRule 默认值
func TestAlertRuleDefaultValues(t *testing.T) {
	rule := &AlertRule{
		Name:        "test-rule",
		Condition:   "metric.value > 100",
		ForDuration: time.Minute * 5,
		Severity:    AlertSeverityWarning,
		Labels: JSONMap{
			"service": "test",
		},
		Annotations: JSONMap{
			"description": "Test rule",
		},
		CreatedBy: "test-user",
	}

	assert.Equal(t, "test-rule", rule.Name)
	assert.Equal(t, "metric.value > 100", rule.Condition)
	assert.Equal(t, AlertSeverityWarning, rule.Severity)
}

// TestAlertInstanceDefaultValues 测试 AlertInstance 默认值
func TestAlertInstanceDefaultValues(t *testing.T) {
	instance := &AlertInstance{
		RuleID:      1,
		RuleName:    "test-rule",
		Status:      AlertStatusFiring,
		Severity:    AlertSeverityCritical,
		Summary:     "Test alert",
		Description: "This is a test alert",
		StartedAt:   time.Now(),
		FiredAt:     time.Now(),
		Fingerprint: "test-fingerprint",
		GroupKey:    "test-group",
	}

	assert.Equal(t, uint(1), instance.RuleID)
	assert.Equal(t, "test-rule", instance.RuleName)
	assert.Equal(t, AlertStatusFiring, instance.Status)
	assert.Equal(t, AlertSeverityCritical, instance.Severity)
}
