package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/common"
	"github.com/voilet/quic-flow/pkg/monitoring"
	"github.com/voilet/quic-flow/pkg/recording"
)

// RecordingAPI handles recording API endpoints
type RecordingAPI struct {
	store       *recording.Store
	dbStore     *recording.DBStore
	logger      *monitoring.Logger
	useDatabase bool
}

// NewRecordingAPI creates a new recording API handler
func NewRecordingAPI(store *recording.Store, logger *monitoring.Logger) *RecordingAPI {
	return &RecordingAPI{
		store:       store,
		logger:      logger,
		useDatabase: false,
	}
}

// SetDBStore sets the database store and enables database mode
func (a *RecordingAPI) SetDBStore(dbStore *recording.DBStore) {
	a.dbStore = dbStore
	a.useDatabase = true
	a.logger.Info("Recording API now using database storage")
}

// RegisterRoutes registers recording API routes
func (a *RecordingAPI) RegisterRoutes(router *gin.RouterGroup) {
	recGroup := router.Group("/recordings")
	{
		recGroup.GET("", a.ListRecordings)
		recGroup.GET("/:id", a.GetRecording)
		recGroup.GET("/:id/download", a.DownloadRecording)
		recGroup.GET("/:id/stream", a.StreamRecording)
		recGroup.DELETE("/:id", a.DeleteRecording)
		recGroup.GET("/stats", a.GetStats)
		recGroup.DELETE("/cleanup", a.CleanupOldRecordings)
	}
}

// ListRecordings returns a list of recordings
// @Summary List recordings
// @Tags recordings
// @Param session_id query string false "Filter by session ID"
// @Param client_id query string false "Filter by client ID"
// @Param username query string false "Filter by username"
// @Param start_time query string false "Filter by start time (RFC3339)"
// @Param end_time query string false "Filter by end time (RFC3339)"
// @Param limit query int false "Max results (default 100)"
// @Param offset query int false "Pagination offset"
// @Success 200 {object} map[string]interface{}
// @Router /api/recordings [get]
func (a *RecordingAPI) ListRecordings(c *gin.Context) {
	filter := &recording.RecordingFilter{}

	filter.SessionID = c.Query("session_id")
	filter.ClientID = c.Query("client_id")
	filter.Username = c.Query("username")

	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filter.StartTime = &t
		}
	}

	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filter.EndTime = &t
		}
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			filter.Limit = l
		}
	}

	if offset := c.Query("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil {
			filter.Offset = o
		}
	}

	var recordings []*recording.RecordingMeta
	var err error

	// Use database store if available, otherwise use file store
	if a.useDatabase && a.dbStore != nil {
		recordings, err = a.dbStore.List(c.Request.Context(), filter)
	} else {
		recordings, err = a.store.List(c.Request.Context(), filter)
	}

	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"recordings": recordings,
		"count":      len(recordings),
	})
}

// GetRecording returns a specific recording metadata
// @Summary Get recording metadata
// @Tags recordings
// @Param id path string true "Recording ID"
// @Success 200 {object} recording.RecordingMeta
// @Router /api/recordings/{id} [get]
func (a *RecordingAPI) GetRecording(c *gin.Context) {
	id := c.Param("id")

	var meta *recording.RecordingMeta
	var err error

	// Use database store if available, otherwise use file store
	if a.useDatabase && a.dbStore != nil {
		meta, err = a.dbStore.Get(c.Request.Context(), id)
	} else {
		meta, err = a.store.Get(c.Request.Context(), id)
	}

	if err != nil {
		common.ErrorResp(c, common.CodeRecordingNotFound, err.Error())
		return
	}

	common.SuccessResp(c, meta)
}

// DownloadRecording downloads the raw recording file
// @Summary Download recording
// @Tags recordings
// @Param id path string true "Recording ID"
// @Success 200 {file} file
// @Router /api/recordings/{id}/download [get]
func (a *RecordingAPI) DownloadRecording(c *gin.Context) {
	id := c.Param("id")

	file, err := a.store.ReadFile(id)
	if err != nil {
		common.ErrorResp(c, common.CodeRecordingNotFound, err.Error())
		return
	}
	defer file.Close()

	c.Header("Content-Type", "application/x-asciicast")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.cast", id))

	io.Copy(c.Writer, file)
}

// StreamRecording streams recording events via SSE
// @Summary Stream recording events
// @Tags recordings
// @Param id path string true "Recording ID"
// @Param speed query float64 false "Playback speed (default 1.0)"
// @Success 200 {object} recording.Event
// @Router /api/recordings/{id}/stream [get]
func (a *RecordingAPI) StreamRecording(c *gin.Context) {
	id := c.Param("id")

	speed := 1.0
	if s := c.Query("speed"); s != "" {
		if parsed, err := strconv.ParseFloat(s, 64); err == nil && parsed > 0 {
			speed = parsed
		}
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Get file path
	var filePath string
	var err error
	if a.useDatabase && a.dbStore != nil {
		filePath, err = a.dbStore.GetFilePath(c.Request.Context(), id)
	} else {
		filePath, err = a.store.GetFilePath(id)
	}
	if err != nil {
		c.SSEvent("error", gin.H{"error": err.Error()})
		return
	}

	file, err := a.store.ReadFile(id)
	if err != nil {
		c.SSEvent("error", gin.H{"error": err.Error()})
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	// Send header
	if scanner.Scan() {
		var header recording.Header
		if err := json.Unmarshal(scanner.Bytes(), &header); err == nil {
			c.SSEvent("header", header)
		}
	}

	c.Writer.Flush()

	// Send events with timing
	var lastTime float64 = 0

	for scanner.Scan() {
		select {
		case <-c.Request.Context().Done():
			return
		default:
		}

		var raw []interface{}
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			continue
		}

		if len(raw) < 3 {
			continue
		}

		eventTime, ok1 := raw[0].(float64)
		eventType, ok2 := raw[1].(string)
		eventData, ok3 := raw[2].(string)

		if !ok1 || !ok2 || !ok3 {
			continue
		}

		// Wait for the appropriate delay
		delay := time.Duration((eventTime - lastTime) / speed * float64(time.Second))
		if delay > 0 && delay < 10*time.Second {
			time.Sleep(delay)
		}
		lastTime = eventTime

		event := recording.Event{
			Time: eventTime,
			Type: eventType,
			Data: eventData,
		}

		c.SSEvent("event", event)
		c.Writer.Flush()
	}

	c.SSEvent("end", gin.H{"file": filePath})
	c.Writer.Flush()
}

// DeleteRecording deletes a recording
// @Summary Delete recording
// @Tags recordings
// @Param id path string true "Recording ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/recordings/{id} [delete]
func (a *RecordingAPI) DeleteRecording(c *gin.Context) {
	id := c.Param("id")

	err := a.store.Delete(c.Request.Context(), id)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, struct{}{})
}

// GetStats returns recording statistics
// @Summary Get recording statistics
// @Tags recordings
// @Success 200 {object} recording.StoreStats
// @Router /api/recordings/stats [get]
func (a *RecordingAPI) GetStats(c *gin.Context) {
	var stats *recording.StoreStats
	var err error

	// Use database store if available, otherwise use file store
	if a.useDatabase && a.dbStore != nil {
		stats, err = a.dbStore.GetStats(c.Request.Context())
	} else {
		stats, err = a.store.GetStats(c.Request.Context())
	}

	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, stats)
}

// CleanupOldRecordings deletes old recordings
// @Summary Cleanup old recordings
// @Tags recordings
// @Param days query int false "Delete recordings older than X days" default(30)
// @Success 200 {object} map[string]interface{}
// @Router /api/recordings/cleanup [delete]
func (a *RecordingAPI) CleanupOldRecordings(c *gin.Context) {
	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	deleted, err := a.store.DeleteOldRecordings(c.Request.Context(), days)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"deleted_recordings": deleted,
		"older_than_days":    days,
	})
}
