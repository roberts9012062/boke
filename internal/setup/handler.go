// internal/setup/handler.go
// 安装向导 Gin 控制器：状态查询、环境检查、自动配置、数据库验证、执行安装。
// 仅在未安装（无 install.lock）时由安装模式服务挂载；安装完成后服务重启进入正常模式。
package setup

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/resp"
)

// Handler 安装向导控制器。
type Handler struct {
	dataDir string // 数据目录（锁文件/配置暂存位置）
}

// NewHandler 创建安装向导控制器。
func NewHandler(dataDir string) *Handler {
	return &Handler{dataDir: dataDir}
}

// Register 挂载安装向导路由（/api/setup/*）。
func (h *Handler) Register(engine *gin.Engine) {
	group := engine.Group("/api/setup")
	group.GET("/status", h.status)
	group.POST("/check", h.check)
	group.POST("/fix", h.fix)
	group.POST("/database", h.database)
	group.POST("/install", h.install)
}

// StatusStatus 状态查询响应。
type StatusStatus struct {
	Installed bool   `json:"installed"` // 是否已完成安装
	Mode      string `json:"mode"`      // docker / manual
	Version   string `json:"version"`   // 安装向导协议版本（前端兼容用）
}

// status GET /api/setup/status —— 安装状态与模式查询。
func (h *Handler) status(c *gin.Context) {
	resp.OK(c, StatusStatus{
		Installed: Installed(h.dataDir),
		Mode:      Mode(),
		Version:   "1",
	})
}

// check POST /api/setup/check —— 环境依赖检查（返回逐项结果）。
func (h *Handler) check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	resp.OK(c, RunChecks(ctx, h.dataDir, Mode()))
}

// fix POST /api/setup/fix —— 自动配置缺失依赖（建目录/等待数据库就绪）。
func (h *Handler) fix(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()
	if err := RunFix(ctx, h.dataDir, Mode()); err != nil {
		resp.FailFrom(c, err)
		return
	}
	// 修复后立即复查，前端可直接刷新检查项状态
	resp.OK(c, RunChecks(ctx, h.dataDir, Mode()))
}

// DatabaseRequest 数据库验证请求体（裸机模式）。
type DatabaseRequest struct {
	Host     string `json:"host" binding:"required"`
	Port     string `json:"port"`
	User     string `json:"user" binding:"required"`
	Password string `json:"password"`
	Database string `json:"database" binding:"required"`
}

// database POST /api/setup/database —— 验证并暂存数据库连接（裸机模式专用）。
func (h *Handler) database(c *gin.Context) {
	var req DatabaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, http.StatusBadRequest, errs.New(errs.CodeBadRequest, "请完整填写数据库连接信息"))
		return
	}
	cfg := DBConfig{
		Host:     req.Host,
		Port:     req.Port,
		User:     req.User,
		Password: req.Password,
		Database: req.Database,
	}
	if cfg.Port == "" {
		cfg.Port = "5432"
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := pingDatabase(ctx, cfg); err != nil {
		resp.Fail(c, http.StatusBadRequest, errs.New(errs.CodeBadRequest,
			fmt.Sprintf("数据库连接失败：%v", err)))
		return
	}
	if err := StashDBConfig(h.dataDir, cfg); err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, gin.H{
		"host":     cfg.Host,
		"port":     cfg.Port,
		"user":     cfg.User,
		"database": cfg.Database,
		"message":  "数据库连接验证通过",
	})
}

// baseURLFrom 从请求推断站点访问地址（协议取反代头，缺省 http）。
func baseURLFrom(c *gin.Context) string {
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}
	host := c.GetHeader("Host")
	if host == "" {
		host = "localhost:3000"
	}
	return scheme + "://" + host
}

// install POST /api/setup/install —— 执行安装全流程。
func (h *Handler) install(c *gin.Context) {
	if Installed(h.dataDir) {
		resp.Fail(c, http.StatusConflict, errs.New(errs.CodeStateConflict, "站点已安装，如需重装请先删除 data/install.lock"))
		return
	}
	var req InstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.Fail(c, http.StatusBadRequest, errs.New(errs.CodeBadRequest, "安装参数不完整："+err.Error()))
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	result, err := RunInstall(ctx, h.dataDir, Mode(), req, baseURLFrom(c))
	if err != nil {
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, result)

	// Docker 模式：响应送达后进程退出，由编排重启策略拉起正常模式服务
	if result.Restart == "auto" {
		go func() {
			time.Sleep(3 * time.Second)
			fmt.Println("[安装完成] Docker 模式自动重启，切换正常运行模式")
			os.Exit(0)
		}()
	}
}
