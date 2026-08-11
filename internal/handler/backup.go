// internal/handler/backup.go
// 备份导出控制器（M4-报表，设计稿《备份导出》#237/#244）：
// 备份记录列表 / 创建备份 / 下载（附件流）/ 删除。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// BackupHandler 备份导出控制器（连接器类）。
type BackupHandler struct {
	backup *service.BackupService // 备份服务
	logger *zap.Logger            // 错误日志（5xx 留痕）
}

// NewBackupHandler 创建备份控制器。
func NewBackupHandler(backup *service.BackupService, logger *zap.Logger) *BackupHandler {
	return &BackupHandler{backup: backup, logger: logger}
}

// List 备份记录列表（GET /api/v1/admin/backups）。
func (h *BackupHandler) List(c *gin.Context) {
	items, err := h.backup.List(c.Request.Context())
	if err != nil {
		h.logger.Error("备份记录查询失败", zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"items": items})
}

// Create 创建备份（POST /api/v1/admin/backups，
// body: {backup_type, scope[], format, retention_days}）。
func (h *BackupHandler) Create(c *gin.Context) {
	var input service.BackupInput
	if err := c.ShouldBindJSON(&input); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	dto, err := h.backup.CreateBackup(c.Request.Context(), input)
	if err != nil {
		h.logger.Error("创建备份失败", zap.Any("input", input), zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, dto)
}

// Download 下载备份文件（GET /api/v1/admin/backups/:id/download；
// 附件流：Content-Disposition attachment，不走统一 JSON 包装）。
func (h *BackupHandler) Download(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	path, filename, _, err := h.backup.Download(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("备份下载失败", zap.Int64("id", id), zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.File(path)
}

// Delete 删除备份（DELETE /api/v1/admin/backups/:id，文件 + 记录）。
func (h *BackupHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	if err := h.backup.Delete(c.Request.Context(), id); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{"deleted": true})
}
