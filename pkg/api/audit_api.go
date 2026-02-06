package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/voilet/quic-flow/pkg/audit"
	"github.com/voilet/quic-flow/pkg/common"
	"github.com/voilet/quic-flow/pkg/monitoring"
)

// AuditAPI handles audit log API endpoints
type AuditAPI struct {
	store  audit.Store
	logger *monitoring.Logger
}

// NewAuditAPI creates a new audit API handler
func NewAuditAPI(store audit.Store, logger *monitoring.Logger) *AuditAPI {
	return &AuditAPI{
		store:  store,
		logger: logger,
	}
}

// SetStore updates the audit store (used when database becomes available)
func (a *AuditAPI) SetStore(store audit.Store) {
	a.store = store
}

// RegisterRoutes registers audit API routes
func (a *AuditAPI) RegisterRoutes(router *gin.RouterGroup) {
	auditGroup := router.Group("/audit")
	{
		auditGroup.GET("/commands", a.ListCommands)
		auditGroup.GET("/commands/:session_id", a.GetCommandsBySession)
		auditGroup.GET("/stats", a.GetStats)
		auditGroup.GET("/export", a.ExportCommands)
		auditGroup.DELETE("/cleanup", a.CleanupOldRecords)
	}
}

// ListCommands returns a list of audit logs
// @Summary List audit commands
// @Tags audit
// @Param session_id query string false "Filter by session ID"
// @Param client_id query string false "Filter by client ID"
// @Param username query string false "Filter by username"
// @Param command query string false "Filter by command substring"
// @Param start_time query string false "Filter by start time (RFC3339)"
// @Param end_time query string false "Filter by end time (RFC3339)"
// @Param limit query int false "Max results (default 100)"
// @Param offset query int false "Pagination offset"
// @Success 200 {object} map[string]interface{}
// @Router /api/audit/commands [get]
func (a *AuditAPI) ListCommands(c *gin.Context) {
	filter := &audit.CommandFilter{}

	filter.SessionID = c.Query("session_id")
	filter.ClientID = c.Query("client_id")
	filter.Username = c.Query("username")
	filter.Command = c.Query("command")

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

	commands, err := a.store.QueryCommands(c.Request.Context(), filter)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"commands": commands,
		"count":    len(commands),
	})
}

// GetCommandsBySession returns commands for a specific session
// @Summary Get commands by session
// @Tags audit
// @Param session_id path string true "Session ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/audit/commands/{session_id} [get]
func (a *AuditAPI) GetCommandsBySession(c *gin.Context) {
	sessionID := c.Param("session_id")

	commands, err := a.store.GetCommandsBySession(c.Request.Context(), sessionID)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"session_id": sessionID,
		"commands":   commands,
		"count":      len(commands),
	})
}

// GetStats returns audit statistics
// @Summary Get audit statistics
// @Tags audit
// @Success 200 {object} audit.AuditStats
// @Router /api/audit/stats [get]
func (a *AuditAPI) GetStats(c *gin.Context) {
	stats, err := a.store.GetStats(c.Request.Context())
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, stats)
}

// ExportCommands exports audit logs
// @Summary Export audit logs
// @Tags audit
// @Param format query string false "Export format (json, csv)" default(json)
// @Success 200 {file} file
// @Router /api/audit/export [get]
func (a *AuditAPI) ExportCommands(c *gin.Context) {
	format := c.DefaultQuery("format", "json")

	commands, err := a.store.QueryCommands(c.Request.Context(), &audit.CommandFilter{
		Limit: 100000, // Large limit for export
	})
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=audit_export.csv")
		c.Writer.WriteString("id,session_id,client_id,username,command,executed_at,exit_code,duration_ms,remote_ip\n")
		for _, cmd := range commands {
			c.Writer.WriteString(formatCSVLine(cmd))
		}
	default:
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", "attachment; filename=audit_export.json")
		c.JSON(http.StatusOK, commands)
	}
}

// formatCSVLine formats a command log as CSV line
func formatCSVLine(cmd *audit.CommandLog) string {
	return cmd.ID + "," +
		cmd.SessionID + "," +
		cmd.ClientID + "," +
		cmd.Username + "," +
		"\"" + cmd.Command + "\"," +
		cmd.ExecutedAt.Format(time.RFC3339) + "," +
		strconv.Itoa(cmd.ExitCode) + "," +
		strconv.FormatInt(cmd.DurationMs, 10) + "," +
		cmd.RemoteIP + "\n"
}

// CleanupOldRecords deletes old audit records
// @Summary Cleanup old records
// @Tags audit
// @Param days query int false "Delete records older than X days" default(90)
// @Success 200 {object} map[string]interface{}
// @Router /api/audit/cleanup [delete]
func (a *AuditAPI) CleanupOldRecords(c *gin.Context) {
	days := 90
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	deleted, err := a.store.DeleteOldRecords(c.Request.Context(), days)
	if err != nil {
		common.ErrorResp(c, common.CodeInternalError, err.Error())
		return
	}

	common.SuccessResp(c, gin.H{
		"deleted_records": deleted,
		"older_than_days": days,
	})
}
