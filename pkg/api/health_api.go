package api

import (

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/common"
	"github.com/voilet/quic-flow/pkg/monitoring"
)

// HealthAPI 健康检查 API
type HealthAPI struct {
	logger *monitoring.Logger
}

// NewHealthAPI 创建健康检查 API
func NewHealthAPI(logger *monitoring.Logger) *HealthAPI {
	return &HealthAPI{
		logger: logger,
	}
}

// RegisterRoutes 注册路由
func (api *HealthAPI) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/health", api.HealthCheck)
	r.GET("/ready", api.ReadinessCheck)
	r.GET("/live", api.LivenessCheck)
}

// HealthCheck 健康检查
func (api *HealthAPI) HealthCheck(c *gin.Context) {
	common.SuccessResp(c, gin.H{
		"status":  "healthy",
		"service": "quic-flow-task",
	})
}

// ReadinessCheck 就绪检查
func (api *HealthAPI) ReadinessCheck(c *gin.Context) {
	// TODO: 检查数据库连接、调度器状态等
	common.SuccessResp(c, gin.H{
		"status":  "ready",
		"service": "quic-flow-task",
	})
}

// LivenessCheck 存活检查
func (api *HealthAPI) LivenessCheck(c *gin.Context) {
	common.SuccessResp(c, gin.H{
		"status":  "alive",
		"service": "quic-flow-task",
	})
}
