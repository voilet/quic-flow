package profiling

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/voilet/quic-flow/pkg/common"
)

// Handler 性能分析 API 处理器
type Handler struct {
	profiler *Profiler
	analyzer *Analyzer
}

// NewHandler 创建 API 处理器
func NewHandler(profiler *Profiler) *Handler {
	return &Handler{
		profiler: profiler,
		analyzer: NewAnalyzer(profiler),
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r gin.IRouter) {
	api := r.Group("/profiling")
	{
		// 采集操作
		api.POST("/cpu", h.StartCPUProfile)
		api.POST("/cpu/upload", h.UploadCPUProfile) // 上传已采集的 CPU profile
		api.POST("/memory", h.CaptureMemoryProfile)
		api.POST("/goroutine", h.CaptureGoroutineProfile)

		// 查询操作
		api.GET("/list", h.ListProfiles)
		api.GET("/profiles/:id", h.GetProfile)

		// 火焰图
		api.GET("/flamegraph/:id", h.GetFlameGraph)
		api.POST("/flamegraph/:id/generate", h.GenerateFlameGraph)

		// 分析
		api.POST("/analyze/:id", h.AnalyzeProfile)

		// 管理
		api.DELETE("/profiles/:id", h.DeleteProfile)
		api.POST("/cleanup", h.CleanupOldProfiles)
	}
}

// StartCPUProfile 启动 CPU 采集
// POST /api/profiling/cpu
// Request: {"name": "test", "duration": 30}
func (h *Handler) StartCPUProfile(c *gin.Context) {
	var req StartCPUProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid request parameters")
		return
	}

	// 获取当前用户
	username := "admin"
	if u, exists := c.Get("username"); exists {
		if name, ok := u.(string); ok {
			username = name
		}
	}

	profile, err := h.profiler.StartCPUProfile(req.Name, req.Duration, username)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{"profile_id": profile.ID}, "CPU profiling started")
}

// UploadCPUProfile 上传已采集的 CPU profile
// POST /api/profiling/cpu/upload
// Form-Data: file=@profile.pb, name="profile-name"
func (h *Handler) UploadCPUProfile(c *gin.Context) {
	// 获取上传的文件
	fileHeader, err := c.FormFile("file")
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "No file uploaded")
		return
	}

	// 获取名称
	name := c.PostForm("name")
	if name == "" {
		name = fmt.Sprintf("cpu-upload-%d", time.Now().Unix())
	}

	// 获取当前用户
	username := "admin"
	if u, exists := c.Get("username"); exists {
		if name, ok := u.(string); ok {
			username = name
		}
	}

	// 使用 profiler 的方法保存上传的 profile
	profile, err := h.profiler.SaveUploadedCPUProfile(fileHeader, name, username)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{"profile": profile}, "CPU profile uploaded successfully")
}

// CaptureMemoryProfile 采集内存快照
// POST /api/profiling/memory
// Request: {"name": "mem-test"}
func (h *Handler) CaptureMemoryProfile(c *gin.Context) {
	var req CaptureProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid request parameters")
		return
	}

	username := "admin"
	if u, exists := c.Get("username"); exists {
		if name, ok := u.(string); ok {
			username = name
		}
	}

	profile, err := h.profiler.CaptureMemoryProfile(req.Name, username)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{"profile_id": profile.ID}, "Memory profile captured")
}

// CaptureGoroutineProfile 采集 Goroutine 快照
// POST /api/profiling/goroutine
// Request: {"name": "goroutine-test"}
func (h *Handler) CaptureGoroutineProfile(c *gin.Context) {
	var req CaptureProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ErrorResp(c, common.CodeInvalidParams, "Invalid request parameters")
		return
	}

	username := "admin"
	if u, exists := c.Get("username"); exists {
		if name, ok := u.(string); ok {
			username = name
		}
	}

	profile, err := h.profiler.CaptureGoroutineProfile(req.Name, username)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{"profile_id": profile.ID}, "Goroutine profile captured")
}

// ListProfiles 获取采集列表
// GET /api/profiling/list?type=cpu&status=completed&page=1&page_size=20
func (h *Handler) ListProfiles(c *gin.Context) {
	var req ProfileListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		req = ProfileListRequest{
			Page:     1,
			PageSize: 20,
		}
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 20
	}

	profiles, total, err := h.profiler.ListProfiles(req.Type, req.Status, req.Page, req.PageSize)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to list profiles")
		return
	}

	common.SuccessResp(c, gin.H{
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
		"items":     profiles,
	})
}

// GetProfile 获取单个采集
// GET /api/profiling/profiles/:id
func (h *Handler) GetProfile(c *gin.Context) {
	profileID := c.Param("id")
	if _, err := uuid.Parse(profileID); err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Invalid profile ID")
		return
	}

	profile, err := h.profiler.GetProfile(profileID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Profile not found")
		return
	}

	common.SuccessResp(c, profile)
}

// GetFlameGraph 获取火焰图 SVG
// GET /api/profiling/flamegraph/:id
func (h *Handler) GetFlameGraph(c *gin.Context) {
	profileID := c.Param("id")

	flamePath, err := h.profiler.GetFlameGraphPath(profileID)
	if err != nil {
		// 尝试自动生成
		flamePath, err = h.profiler.GenerateFlameGraph(profileID)
		if err != nil {
			common.ErrorResp(c, common.CodeInternalError, "Flame graph not available")
			return
		}
	}

	// 读取 SVG 文件
	c.File(flamePath)
}

// GenerateFlameGraph 生成火焰图
// POST /api/profiling/flamegraph/:id/generate
func (h *Handler) GenerateFlameGraph(c *gin.Context) {
	profileID := c.Param("id")

	flamePath, err := h.profiler.GenerateFlameGraph(profileID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessRespWithMsg(c, gin.H{"flame_path": flamePath}, "Flame graph generated")
}

// AnalyzeProfile 分析 profile
// POST /api/profiling/analyze/:id
func (h *Handler) AnalyzeProfile(c *gin.Context) {
	profileID := c.Param("id")

	report, err := h.analyzer.Analyze(profileID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, report)
}

// DeleteProfile 删除采集
// DELETE /api/profiling/profiles/:id
func (h *Handler) DeleteProfile(c *gin.Context) {
	profileID := c.Param("id")

	if err := h.profiler.DeleteProfile(profileID); err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to delete profile")
		return
	}

	common.SuccessRespWithMsg(c, gin.H{}, "Profile deleted")
}

// CleanupOldProfiles 清理旧采集
// POST /api/profiling/cleanup?days=7
func (h *Handler) CleanupOldProfiles(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		days = 7
	}

	count, err := h.profiler.CleanupOldProfiles(days)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to cleanup profiles")
		return
	}

	common.SuccessRespWithMsg(c, gin.H{"deleted_count": count}, fmt.Sprintf("Deleted %d old profiles", count))
}

// DownloadProfile 下载原始 profile 文件
// GET /api/profiling/download/:id
func (h *Handler) DownloadProfile(c *gin.Context) {
	profileID := c.Param("id")

	profile, err := h.profiler.GetProfile(profileID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Profile not found")
		return
	}

	if profile.Status != StatusCompleted {
		common.ErrorResp(c, common.CodeInternalError, "Profile is not completed")
		return
	}

	// 设置下载文件名
	filename := fmt.Sprintf("%s-%s.prof", profile.Type, profile.ID[:8])
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.File(profile.FilePath)
}

// GetProfileText 获取 profile 文本表示（用于调试）
// GET /api/profiling/text/:id
func (h *Handler) GetProfileText(c *gin.Context) {
	profileID := c.Param("id")

	profile, err := h.profiler.GetProfile(profileID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Profile not found")
		return
	}

	if profile.Status != StatusCompleted {
		common.ErrorResp(c, common.CodeInternalError, "Profile is not completed")
		return
	}

	// 使用 pprof 工具生成文本报告
	// 这里简化处理，直接返回文件信息
	common.SuccessResp(c, gin.H{
			"id":         profile.ID,
			"type":       profile.Type,
			"name":       profile.Name,
			"file_path":  profile.FilePath,
			"file_size":  profile.FileSize,
			"created_at": profile.CreatedAt,
		})
}

// StreamProfile 流式传输 profile 文件
// GET /api/profiling/stream/:id
func (h *Handler) StreamProfile(c *gin.Context) {
	profileID := c.Param("id")

	profile, err := h.profiler.GetProfile(profileID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Profile not found")
		return
	}

	if profile.Status != StatusCompleted {
		common.ErrorResp(c, common.CodeInternalError, "Profile is not completed")
		return
	}

	// 打开文件
	f, err := os.Open(profile.FilePath)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to open profile file")
		return
	}
	defer f.Close()

	// 获取文件信息
	info, err := f.Stat()
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, "Failed to read file info")
		return
	}

	// 设置响应头
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.prof"`, profile.Type, profile.ID[:8]))
	c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))

	// 流式传输
	http.ServeContent(c.Writer, c.Request, "", time.Now(), f)
}
