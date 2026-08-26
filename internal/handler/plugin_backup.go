// internal/handler/plugin_backup.go
// 插件备份文件下载（v1.3.0 配套：备份助手等插件产出的 backups/ 目录文件直出）。
//
// 说明：插件自定义 API 代理固定 JSON 响应且有 10s 超时，不适合大文件下载；
// 本端点由宿主流式直出（FileAttachment），目录定位复用 PublicAssetDir（仅
// running 插件可下载），文件名白名单校验防路径穿越。
package handler

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// isBackupFileName 备份文件名白名单：backup- 前缀 + .zip 后缀 + 不含路径分隔符。
// （与备份助手的产物命名约定一致；拒绝任何形式的路径穿越）
func isBackupFileName(name string) bool {
	if strings.ContainsAny(name, `/\`) || name != filepath.Base(name) {
		return false
	}
	return strings.HasPrefix(name, "backup-") && strings.HasSuffix(name, ".zip")
}

// DownloadBackup 下载插件备份文件（GET /api/v1/admin/plugins/:id/backups/:file/download）。
func (h *PluginHandler) DownloadBackup(c *gin.Context) {
	pluginID := c.Param("id")
	file := c.Param("file")
	if !isBackupFileName(file) {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "非法的备份文件名"))
		return
	}
	// 仅 running 插件返回目录（PublicAssetDir 内部含状态与 ID 合法性校验）
	dir := h.plugins.PublicAssetDir(c.Request.Context(), pluginID)
	if dir == "" {
		resp.Fail(c, 404, errs.New(errs.CodeNotFound, "插件不存在或未运行"))
		return
	}
	target := filepath.Join(dir, "backups", file)
	if _, err := os.Stat(target); err != nil {
		resp.Fail(c, 404, errs.New(errs.CodeNotFound, "备份文件不存在"))
		return
	}
	c.FileAttachment(target, file)
}
