package alert

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
)

// Engine CEL 规则引擎
type Engine struct {
	// 编译后的 CEL 程序缓存
	programs sync.Map // map[string]*cel.Program
}

// EvaluationContext 评估上下文
type EvaluationContext struct {
	// 指标数据
	Metrics map[string]interface{}

	// 标签
	Labels map[string]string

	// 时间戳
	Timestamp time.Time

	// 额外的自定义变量
	Vars map[string]interface{}
}

// EvaluationResult 评估结果
type EvaluationResult struct {
	Matched bool // 是否匹配告警规则
	Value   ref.Val
	Error   error
}

// NewEngine 创建新的 CEL 规则引擎
func NewEngine() *Engine {
	return &Engine{}
}

// Compile 编译 CEL 表达式
func (e *Engine) Compile(expression string) (cel.Program, error) {
	// 检查缓存
	if cached, ok := e.programs.Load(expression); ok {
		return cached.(cel.Program), nil
	}

	// 创建 CEL 环境
	env, err := e.createEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	// 解析表达式
	ast, issues := env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to parse expression: %w", issues.Err())
	}

	// 检查类型
	checked, issues := env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("type check error: %w", issues.Err())
	}

	// 生成程序
	program, err := env.Program(checked)
	if err != nil {
		return nil, fmt.Errorf("failed to generate program: %w", err)
	}

	// 缓存程序
	e.programs.Store(expression, program)

	return program, nil
}

// Evaluate 评估 CEL 表达式
func (e *Engine) Evaluate(ctx context.Context, expression string, evalCtx *EvaluationContext) *EvaluationResult {
	// 编译表达式
	program, err := e.Compile(expression)
	if err != nil {
		return &EvaluationResult{
			Matched: false,
			Error:   err,
		}
	}

	// 准备变量
	vars := e.prepareVariables(evalCtx)

	// 评估表达式
	out, _, err := program.Eval(vars)
	if err != nil {
		return &EvaluationResult{
			Matched: false,
			Error:   err,
		}
	}

	// 检查结果是否为布尔值
	matched := false
	if boolVal, ok := out.Value().(bool); ok {
		matched = boolVal
	}

	return &EvaluationResult{
		Matched: matched,
		Value:   out,
		Error:   nil,
	}
}

// EvaluateWithTimeout 评估 CEL 表达式（带超时）
func (e *Engine) EvaluateWithTimeout(ctx context.Context, expression string, evalCtx *EvaluationContext, timeout time.Duration) *EvaluationResult {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resultCh := make(chan *EvaluationResult, 1)

	go func() {
		resultCh <- e.Evaluate(ctx, expression, evalCtx)
	}()

	select {
	case <-ctx.Done():
		return &EvaluationResult{
			Matched: false,
			Error:   fmt.Errorf("evaluation timeout"),
		}
	case result := <-resultCh:
		return result
	}
}

// createEnvironment 创建 CEL 环境
func (e *Engine) createEnvironment() (*cel.Env, error) {
	// 基础选项
	opts := []cel.EnvOption{
		cel.EagerlyValidateDeclarations(true),
		cel.CrossTypeNumericComparisons(true),
		cel.DefaultUTCTimeZone(true),
	}

	return cel.NewEnv(opts...)
}

// prepareVariables 准备评估变量
func (e *Engine) prepareVariables(evalCtx *EvaluationContext) map[string]interface{} {
	vars := make(map[string]interface{})

	// 添加指标数据
	if evalCtx.Metrics != nil {
		for k, v := range evalCtx.Metrics {
			vars[k] = v
		}
	}

	// 添加标签
	if evalCtx.Labels != nil {
		for k, v := range evalCtx.Labels {
			vars["label_"+k] = v
		}
	}

	// 添加时间戳
	vars["now"] = evalCtx.Timestamp.Unix()
	vars["now_ms"] = evalCtx.Timestamp.UnixMilli()

	// 添加自定义变量
	if evalCtx.Vars != nil {
		for k, v := range evalCtx.Vars {
			vars[k] = v
		}
	}

	return vars
}

// ClearCache 清除编译缓存
func (e *Engine) ClearCache() {
	e.programs = sync.Map{}
}

// GetCacheSize 获取缓存大小
func (e *Engine) GetCacheSize() int {
	size := 0
	e.programs.Range(func(_, _ interface{}) bool {
		size++
		return true
	})
	return size
}

// ==================== 便捷方法 ====================

// EvaluateBool 评估布尔表达式
func (e *Engine) EvaluateBool(expression string, metrics map[string]interface{}) (bool, error) {
	evalCtx := &EvaluationContext{
		Metrics:   metrics,
		Timestamp: time.Now(),
	}

	result := e.Evaluate(context.Background(), expression, evalCtx)
	if result.Error != nil {
		return false, result.Error
	}

	return result.Matched, nil
}

// ValidateExpression 验证表达式是否有效
func (e *Engine) ValidateExpression(expression string) error {
	_, err := e.Compile(expression)
	return err
}

// GetExpressionSuggestions 获取表达式建议
func (e *Engine) GetExpressionSuggestions() []string {
	return []string{
		// 比较运算符
		"cpu_usage > 80",
		"memory_usage > 90 && disk_usage > 85",
		"error_rate > 0.05",

		// 使用标签
		"cpu_usage > 80 && label_env == 'production'",
		"label_region == 'us-west' && response_time > 1000",

		// 时间相关
		"now - timestamp > 3600",  // 超过1小时
		"now_ms - last_update_ms > 60000",  // 超过1分钟

		// 复杂表达式
		"(cpu_usage + memory_usage) / 2 > 75",
		"error_count > 100 || error_rate > 0.1",
		"status == 'critical' && duration > 300",

		// 字符串匹配
		"message.contains('error')",
		"level == 'ERROR' || level == 'FATAL'",

		// 列表/集合操作
		"tags.contains('important')",
		"size(errors) > 0",
	}
}

// ==================== 规则预处理 ====================

// CompiledRule 编译后的规则
type CompiledRule struct {
	RuleID    string
	Program   cel.Program
	Condition string
}

// PrecompileRules 预编译一组规则
func (e *Engine) PrecompileRules(rules []*AlertRule) ([]*CompiledRule, error) {
	compiled := make([]*CompiledRule, 0, len(rules))

	for _, rule := range rules {
		// 假设 AlertRule 有一个 ID 字段和 Condition 字段
		program, err := e.Compile(rule.Condition)
		if err != nil {
			return nil, fmt.Errorf("failed to compile rule %s: %w", rule.Name, err)
		}

		compiled = append(compiled, &CompiledRule{
			RuleID:    rule.Name,
			Program:   program,
			Condition: rule.Condition,
		})
	}

	return compiled, nil
}

// BatchEvaluate 批量评估规则
func (e *Engine) BatchEvaluate(ctx context.Context, rules []*CompiledRule, evalCtx *EvaluationContext) map[string]bool {
	results := make(map[string]bool)

	for _, rule := range rules {
		out, _, err := rule.Program.Eval(e.prepareVariables(evalCtx))
		if err != nil {
			results[rule.RuleID] = false
			continue
		}

		if boolVal, ok := out.Value().(bool); ok {
			results[rule.RuleID] = boolVal
		} else {
			results[rule.RuleID] = false
		}
	}

	return results
}

// ==================== 类型检查和推断 ====================

// InferType 推断表达式结果类型
func (e *Engine) InferType(expression string) (string, error) {
	env, err := e.createEnvironment()
	if err != nil {
		return "", err
	}

	ast, issues := env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return "", issues.Err()
	}

	checked, issues := env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return "", issues.Err()
	}

	// 获取输出类型
	outputType := checked.OutputType()
	return outputType.String(), nil
}

// ==================== 表达式示例 ====================

// ExampleExpressions 返回常用的表达式示例
func ExampleExpressions() map[string]string {
	return map[string]string{
		"CPU高使用率":          "cpu_usage_percent > 80",
		"内存高使用率":          "memory_usage_percent > 85",
		"磁盘空间不足":          "disk_usage_percent > 90",
		"高错误率":           "error_rate > 0.05",
		"响应时间过长":          "response_time_ms > 1000",
		"生产环境关键告警":       "severity == 'critical' && label_env == 'production'",
		"持续5分钟高负载":       "cpu_usage_percent > 80 && duration_seconds > 300",
		"多条件组合":          "cpu_usage_percent > 80 || memory_usage_percent > 85",
		"包含特定错误消息":       "message.contains('out of memory')",
		"错误计数超过阈值":       "error_count > 100",
		"服务不可用":          "status == 'down'",
		"连接数过多":          "connection_count > max_connections * 0.9",
		"QPS低于预期":         "qps < expected_qps * 0.5",
		"延迟P99超过阈值":       "latency_p99_ms > 2000",
		"缓存命中率低":         "cache_hit_rate < 0.8",
	}
}
