// internal/server/setup_mode.go
// 安装模式服务：未完成安装（无 data/install.lock）时启动的最小 HTTP 服务。
//
// 行为：
//   - 仅挂载 /api/setup/* 安装向导接口与 /api/setup/status 健康查询
//   - 其余全部 /api/* 请求返回 503（站点尚未初始化），前端经 middleware 引导至 /setup
//   - 安装完成后（Docker 模式）安装接口触发进程退出，由 compose 重启进入正常模式
package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/internal/setup"
)

// RunSetupMode 启动安装模式 HTTP 服务（阻塞直至退出信号）。
// 参数：port 监听端口；dataDir 数据目录（安装锁/配置位置）；logger 日志器。
func RunSetupMode(port string, dataDir string, logger *zap.Logger) error {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()

	// 安装阶段放开 CORS（向导页面可能经反代跨域访问）
	engine.Use(middleware.Recovery(logger), middleware.CORS(""))

	// 安装向导路由
	setup.NewHandler(dataDir).Register(engine)

	// 其余 API 一律 503：站点未初始化
	engine.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    503,
				"message": "站点尚未完成安装，请先访问 /setup 完成安装向导",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "boke setup mode"})
	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: engine,
	}

	// 优雅退出（安装接口 os.Exit 场景不经过此处，属预期行为）
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	logger.Info("安装模式启动：站点尚未初始化，请访问前端 /setup 页面完成安装向导",
		zap.String("port", port), zap.String("data_dir", dataDir))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
