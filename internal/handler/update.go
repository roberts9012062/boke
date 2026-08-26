// internal/handler/update.go
// 站点更新控制器：版本检查 / 触发更新 / 进度查询（后台左下角更新徽标数据源）。
//
// 架构：后端不直接执行更新（容器无宿主机控制权）——写更新任务到数据目录
// （挂载卷共享），宿主机更新代理（scripts/update-agent.sh，systemd timer 每分钟
// 轮询）读取任务后执行「拉取代码 → 重建镜像 → 重启服务」，进度写回状态文件，
// 本控制器读取转发给前端轮询展示。
package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/internal/ghclient"
	"github.com/roberts9012062/boke/internal/update"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// 主仓库（版本检测与源码链接指向；本项目自有更新通道）。
const (
	repoOwner = "roberts9012062"
	repoName  = "boke"
	repoURL   = "https://github.com/roberts9012062/boke"
)

// UpdateHandler 站点更新控制器（连接器类）。
type UpdateHandler struct {
	gh      *ghclient.Client // GitHub 客户端（Release 查询）
	dataDir string           // 数据目录（任务/状态/版本文件位置）
}

// NewUpdateHandler 创建站点更新控制器。
func NewUpdateHandler(gh *ghclient.Client, dataDir string) *UpdateHandler {
	return &UpdateHandler{gh: gh, dataDir: dataDir}
}

// UpdateCheckResult 版本检查响应。
type UpdateCheckResult struct {
	CurrentVersion string        `json:"current_version"`   // 当前部署版本（dev=未标记）
	LatestVersion  string        `json:"latest_version"`    // 仓库最新 Release 版本
	HasUpdate      bool          `json:"has_update"`        // 是否有新版本
	ReleaseNotes   string        `json:"release_notes"`     // 更新日志（Release 说明）
	RepoURL        string        `json:"repo_url"`          // 源码仓库链接
	RunningUpdate  update.Status `json:"running_update"`    // 进行中的更新进度（无任务为 idle）
}

// Check 处理版本检查（GET /api/v1/admin/update/check）。
// 拉取仓库最新 Release 与当前版本比较；同时附带进行中的更新进度。
func (h *UpdateHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	current := update.CurrentVersion(h.dataDir)
	result := UpdateCheckResult{
		CurrentVersion: current,
		RepoURL:        repoURL,
		RunningUpdate:  update.ReadStatus(h.dataDir),
	}

	release, err := h.gh.FetchLatestRelease(ctx, repoOwner, repoName)
	if err != nil {
		// 检测失败不阻断徽标展示（前端显示当前版本 + 检测失败提示）
		result.LatestVersion = current
		resp.OK(c, result)
		return
	}
	result.LatestVersion = release.TagName
	result.HasUpdate = update.IsNewer(release.TagName, current)
	result.ReleaseNotes = release.Body
	resp.OK(c, result)
}

// UpdateStartReq 触发更新请求体。
type UpdateStartReq struct {
	Version string `json:"version" binding:"required"` // 目标版本 tag（来自版本检查结果）
}

// Start 处理触发更新（POST /api/v1/admin/update/start）。
// 写任务文件 + 初始化进度状态；实际执行由宿主机代理异步完成（约 1 分钟内被调度）。
func (h *UpdateHandler) Start(c *gin.Context) {
	var req UpdateStartReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, 400, errs.ErrBadRequest)
		return
	}
	// 目标版本须为 semver（防任意字符串注入任务文件）
	if !update.IsSemver(req.Version) {
		resp.Fail(c, 400, errs.New(errs.CodeBadRequest, "目标版本格式不正确（应为 vX.Y.Z）"))
		return
	}
	if err := update.CreateTask(h.dataDir, req.Version); err != nil {
		resp.Fail(c, 409, errs.New(errs.CodeStateConflict, err.Error()))
		return
	}
	// 重置旧进度并写入排队状态（代理接管前前端有即时反馈）
	update.ClearStatus(h.dataDir)
	_ = update.WriteStatus(h.dataDir, update.Status{
		State:   update.StateRunning,
		Stage:   "更新任务已提交，等待执行",
		Percent: 1,
		Version: req.Version,
	})
	resp.OK(c, gin.H{"started": true, "version": req.Version})
}

// Status 处理更新进度查询（GET /api/v1/admin/update/status；前端轮询）。
func (h *UpdateHandler) Status(c *gin.Context) {
	resp.OK(c, gin.H{
		"status":         update.ReadStatus(h.dataDir),
		"current_version": update.CurrentVersion(h.dataDir),
	})
}
