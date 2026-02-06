package alert

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/voilet/quic-flow/pkg/monitoring"
)

// Evaluator 告警评估器
// 持续评估告警规则，当条件满足时触发告警
type Evaluator struct {
	store Store
	engine *Engine

	// 规则管理
	rules sync.Map // map[string]*CompiledRule

	// 活跃的告警状态（用于 for 持续时间判断）
	activeAlerts sync.Map // map[string]*FiringAlert

	// 监控
	logger *monitoring.Logger

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// FiringAlert 正在触发的告警
type FiringAlert struct {
	RuleID      string
	Labels      map[string]string
	FirstFired  time.Time
	LastEvaluated time.Time
	LastMatched  bool
	Value        interface{}
}

// EvaluatorConfig 评估器配置
type EvaluatorConfig struct {
	EvalInterval   time.Duration // 评估间隔
	Logger         *monitoring.Logger
}

// NewEvaluator 创建告警评估器
func NewEvaluator(store Store, config *EvaluatorConfig) *Evaluator {
	if config == nil {
		config = &EvaluatorConfig{}
	}
	if config.EvalInterval == 0 {
		config.EvalInterval = 15 * time.Second
	}
	if config.Logger == nil {
		config.Logger = monitoring.NewDefaultLogger()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Evaluator{
		store:  store,
		engine: NewEngine(),
		logger: config.Logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动评估器
func (e *Evaluator) Start() error {
	// 加载所有启用的规则
	if err := e.loadRules(); err != nil {
		return fmt.Errorf("failed to load rules: %w", err)
	}

	// 启动评估循环
	e.wg.Add(1)
	go e.evalLoop()

	e.logger.Info("Alert evaluator started")
	return nil
}

// Stop 停止评估器
func (e *Evaluator) Stop() {
	e.cancel()
	e.wg.Wait()
	e.logger.Info("Alert evaluator stopped")
}

// loadRules 加载告警规则
func (e *Evaluator) loadRules() error {
	ctx, cancel := context.WithTimeout(e.ctx, 30*time.Second)
	defer cancel()

	// 获取启用的规则（enabled 为 true 的规则）
	// 注意：需要根据实际的 AlertRule 模型调整
	rules, _, err := e.store.ListRules(ctx, &RuleFilter{})
	if err != nil {
		return err
	}

	// 预编译规则
	for _, rule := range rules {
		// 只处理启用的规则（需要根据实际模型调整）
		// 这里假设 Enabled 字段存在
		// if !rule.Enabled {
		//     continue
		// }

		program, err := e.engine.Compile(rule.Condition)
		if err != nil {
			e.logger.Error("Failed to compile alert rule",
				"rule", rule.Name,
				"condition", rule.Condition,
				"error", err)
			continue
		}

		e.rules.Store(rule.Name, &CompiledRule{
			RuleID:    rule.Name,
			Program:   program,
			Condition: rule.Condition,
		})
	}

	e.logger.Info("Loaded alert rules", "count", len(rules))
	return nil
}

// ReloadRules 重新加载规则
func (e *Evaluator) ReloadRules() error {
	e.rules = sync.Map{}
	return e.loadRules()
}

// evalLoop 评估循环
func (e *Evaluator) evalLoop() {
	defer e.wg.Done()

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.evaluateAllRules()
		}
	}
}

// evaluateAllRules 评估所有规则
func (e *Evaluator) evaluateAllRules() {
	ctx := context.Background()

	// 收集指标数据（实际应该从监控系统获取）
	metrics := e.collectMetrics()

	e.rules.Range(func(_, value interface{}) bool {
		rule := value.(*CompiledRule)

		// 评估规则
		result := e.engine.Evaluate(ctx, rule.Condition, &EvaluationContext{
			Metrics:   metrics,
			Timestamp: time.Now(),
		})

		// 处理评估结果
		e.handleEvaluationResult(rule, result)

		return true
	})
}

// handleEvaluationResult 处理评估结果
func (e *Evaluator) handleEvaluationResult(rule *CompiledRule, result *EvaluationResult) {
	now := time.Now()

	// 生成告警键
	key := rule.RuleID

	if result.Error != nil {
		e.logger.Error("Failed to evaluate rule",
			"rule", rule.RuleID,
			"error", result.Error)
		return
	}

	// 检查是否匹配
	if result.Matched {
		// 规则匹配，检查是否需要触发告警
		if firing, ok := e.activeAlerts.Load(key); ok {
			// 告警已存在，更新状态
			fa := firing.(*FiringAlert)
			fa.LastEvaluated = now
			fa.LastMatched = true
			fa.Value = result.Value
		} else {
			// 新告警
			fa := &FiringAlert{
				RuleID:       rule.RuleID,
				FirstFired:   now,
				LastEvaluated: now,
				LastMatched:  true,
				Value:        result.Value,
			}
			e.activeAlerts.Store(key, fa)

			e.logger.Info("Alert fired",
				"rule", rule.RuleID,
				"value", result.Value)
		}
	} else {
		// 规则不匹配
		if firing, ok := e.activeAlerts.Load(key); ok {
			fa := firing.(*FiringAlert)
			fa.LastEvaluated = now
			fa.LastMatched = false

			// 如果持续一段时间不匹配，清除告警
			if now.Sub(fa.FirstFired) > 5*time.Minute {
				e.activeAlerts.Delete(key)
				e.logger.Info("Alert resolved",
					"rule", rule.RuleID)
			}
		}
	}
}

// collectMetrics 收集指标数据
// 实际实现中应该从 Prometheus 或其他监控系统获取
func (e *Evaluator) collectMetrics() map[string]interface{} {
	// TODO: 从监控系统获取实时指标
	// 这里只是示例
	return map[string]interface{}{
		"cpu_usage_percent":    45.0,
		"memory_usage_percent": 60.0,
		"disk_usage_percent":   75.0,
		"error_rate":          0.01,
		"response_time_ms":    150,
		"qps":                 1000,
		"connection_count":    50,
		"status":              "up",
	}
}

// EvaluateRule 评估单个规则
func (e *Evaluator) EvaluateRule(ruleID string, metrics map[string]interface{}) (*EvaluationResult, error) {
	rule, ok := e.rules.Load(ruleID)
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", ruleID)
	}

	compiledRule := rule.(*CompiledRule)
	result := e.engine.Evaluate(context.Background(), compiledRule.Condition, &EvaluationContext{
		Metrics:   metrics,
		Timestamp: time.Now(),
	})

	return result, nil
}

// GetActiveAlerts 获取当前活跃的告警
func (e *Evaluator) GetActiveAlerts() []*FiringAlert {
	alerts := make([]*FiringAlert, 0)

	e.activeAlerts.Range(func(_, value interface{}) bool {
		alerts = append(alerts, value.(*FiringAlert))
		return true
	})

	return alerts
}

// AddRule 动态添加规则
func (e *Evaluator) AddRule(rule *AlertRule) error {
	program, err := e.engine.Compile(rule.Condition)
	if err != nil {
		return fmt.Errorf("failed to compile rule: %w", err)
	}

	e.rules.Store(rule.Name, &CompiledRule{
		RuleID:    rule.Name,
		Program:   program,
		Condition: rule.Condition,
	})

	return nil
}

// RemoveRule 移除规则
func (e *Evaluator) RemoveRule(ruleID string) {
	e.rules.Delete(ruleID)
	e.activeAlerts.Delete(ruleID)
}

// ValidateRule 验证规则
func (e *Evaluator) ValidateRule(rule *AlertRule) error {
	// 验证 CEL 表达式
	if err := e.engine.ValidateExpression(rule.Condition); err != nil {
		return fmt.Errorf("invalid condition expression: %w", err)
	}

	// TODO: 验证其他字段

	return nil
}

// ==================== 辅助函数 ====================

func boolPtr(b bool) *bool {
	return &b
}
