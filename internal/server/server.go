// internal/server/server.go
// HTTP 服务装配与生命周期管理：zap 日志、数据库/Redis 连接、依赖注入、优雅退出。
//
// 约定（AGENTS.md 规则）：
//   - 日志通过 zap + lumberjack 输出到 logs/ 目录（按大小滚动）
//   - 启动/停止由 scripts/dev-server.sh 与 scripts/stop-all.sh 控制
//   - 依赖注入集中在本文件（router 不直接构造业务对象）
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/roberts9012062/boke/internal/auth"
	"github.com/roberts9012062/boke/internal/casbin"
	"github.com/roberts9012062/boke/internal/config"
	"github.com/roberts9012062/boke/internal/ghclient"
	"github.com/roberts9012062/boke/internal/handler"
	"github.com/roberts9012062/boke/internal/mail"
	"github.com/roberts9012062/boke/internal/media"
	"github.com/roberts9012062/boke/internal/plugin"
	"github.com/roberts9012062/boke/internal/redis"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/internal/router"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/contract"
)

// NewLogger 创建 zap 日志器：控制台（开发可读）+ 文件（logs/server.log，按大小滚动）。
// 返回：日志器；文件输出不可用时返回错误。
func NewLogger(logDir string) (*zap.Logger, error) {
	// 确保日志目录存在
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败：%w", err)
	}

	// 文件输出：滚动策略（单文件 50MB，保留 30 份）
	fileWriter := &lumberjack.Logger{
		Filename:   fmt.Sprintf("%s/server.log", logDir),
		MaxSize:    50,
		MaxBackups: 30,
		MaxAge:     30,
		Compress:   true,
	}

	// 控制台输出（开发环境便于直接观察）
	consoleWriter := zapcore.Lock(os.Stdout)

	// 统一编码配置：ISO8601 时间 + 请求级别字段
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	// 文件用 JSON 编码（便于后续检索），控制台用彩色控制台编码
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(fileWriter),
		zapcore.DebugLevel,
	)
	consoleCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		consoleWriter,
		zapcore.DebugLevel,
	)

	return zap.New(zapcore.NewTee(fileCore, consoleCore), zap.AddCaller()), nil
}

// connectDatabase 建立数据库连接池（pgxpool，并发请求共享）。
// 说明：M1.5 前使用单连接（pgx.Conn），页面并发请求时报 conn busy；
//       改为连接池后并发安全（历史故障：internal/repository 全部迁移 pgxpool）。
func connectDatabase(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, connString)
}

// buildHandlers 依赖注入：创建数据库/Redis 连接与全部业务控制器。
func buildHandlers(ctx context.Context, cfg config.Config, logger *zap.Logger) (router.Handlers, *auth.Manager, *casbin.Enforcer, func(), error) {
	// ---------- 数据库连接 ----------
	conn, err := connectDatabase(ctx, cfg.DB.ConnString())
	if err != nil {
		return router.Handlers{}, nil, nil, nil, fmt.Errorf("连接数据库失败：%w", err)
	}
	// 退出时关闭数据库连接池（插件子进程先停，防止孤儿进程）
	var pluginManager *plugin.PluginManager
	cleanup := func() {
		// 早退路径（casbin/媒体存储初始化失败）时 pluginManager 尚未赋值，防 nil 解引用 panic
		if pluginManager != nil {
			pluginManager.Shutdown()
		}
		conn.Close()
	}

	// ---------- Redis 连接（不可用时降级，不阻断启动） ----------
	redisClient := redis.NewClient(ctx, redis.Config{Host: cfg.Redis.Host, Port: cfg.Redis.Port, DB: cfg.Redis.DB})
	if redisClient == nil {
		logger.Warn("Redis 不可用，登录限流与令牌黑名单降级放行")
	}

	// ---------- 核心组件装配 ----------
	jwtMgr := auth.NewManager(cfg.JWTSecret)     // JWT 管理器
	enforcer, err := casbin.NewEnforcer()        // 角色权限执行器（M5 五级 RBAC）
	if err != nil {
		cleanup()
		return router.Handlers{}, nil, nil, nil, fmt.Errorf("初始化权限执行器失败：%w", err)
	}
	// 媒体存储（data/media 目录）
	mediaStore, err := media.NewStore(cfg.DataDir + "/media")
	if err != nil {
		cleanup()
		return router.Handlers{}, nil, nil, nil, err
	}

	// ---------- GitHub 客户端（M3.1 插件商城清单源） ----------
	ghClient := ghclient.NewClient(cfg.GitHubToken)

	// ---------- 数据访问层 ----------
	userRepo := repository.NewUserRepo(conn)
	auditRepo := repository.NewAuditRepo(conn)
	postRepo := repository.NewPostRepo(conn)
	tagRepo := repository.NewTagRepo(conn)
	mediaRepo := repository.NewMediaRepo(conn)
	commentRepo := repository.NewCommentRepo(conn)
	reactionRepo := repository.NewReactionRepo(conn)
	relationRepo := repository.NewRelationRepo(conn)
	notificationRepo := repository.NewNotificationRepo(conn)
	adminRepo := repository.NewAdminRepo(conn)
	settingRepo := repository.NewSettingRepo(conn)
	messageRepo := repository.NewMessageRepo(conn) // 私信会话/消息（M2）
	pluginRepo := repository.NewPluginRepo(conn)    // 插件实例（M3.1）
	reportRepo := repository.NewReportRepo(conn)      // 举报工单（M2 内容治理）
	sensitiveRepo := repository.NewSensitiveRepo(conn) // 敏感词（M2）
	banRepo := repository.NewBanRepo(conn)             // 封禁记录（M2）
	aiProviderRepo := repository.NewAiProviderRepo(conn) // AI 供应商（M4）
	aiTaskRepo := repository.NewAiTaskRepo(conn)         // AI 任务（M4）
	aiUsageRepo := repository.NewAiUsageRepo(conn)       // AI 用量（M4）
	seoRepo := repository.NewSeoRepo(conn)               // SEO 元数据（M4：摘要落库）
	pageRepo := repository.NewPageRepo(conn)             // 自定义页面（导航自定义数据源）
	backupRepo := repository.NewBackupRepo(conn)         // 备份记录（M4-报表）
	licenseRepo := repository.NewLicenseRepo(conn)       // 插件许可证（M3.5）
	orderRepo := repository.NewPluginOrderRepo(conn)     // 插件购买订单（M3.9 支付渠道）
	openAPIKeyRepo := repository.NewOpenAPIKeyRepo(conn) // 接口开放凭证（/open 网关鉴权）

	// ---------- 业务层 ----------
	limiter := redis.NewRateLimiter(redisClient)
	guestMgr := auth.NewGuestManager(redisClient) // 匿名身份管理器（M2：Redis 优先 + 内存兜底）
	resetMgr := auth.NewResetManager()                 // 密码重置令牌（M2）
	mailer := mail.NewSender(cfg.Mail, logger)         // 邮件发送（M2；未配置降级日志）
	authSvc := service.NewAuthService(userRepo, auditRepo, settingRepo, jwtMgr, enforcer, limiter, resetMgr, mailer, mediaStore, cfg.SiteBaseURL)
	// ---------- 插件钩子调度器（M3.2 扩展框架：注册表 + 故障隔离；错误记录到日志） ----------
	hookDispatcher := plugin.NewRegistry(func(hook string, err error) {
		logger.Warn("插件钩子执行异常", zap.String("hook", hook), zap.Error(err))
	})
	// ---------- 插件进程管理器（M3.3：go-plugin 进程外化；崩溃熔断事件落库） ----------
	binStore := plugin.NewBinStore(cfg.DataDir) // 插件二进制存储（进程拉起 + .bpk 解包共用）
	// 许可证查询回调（M3.5）：延迟绑定——pluginSvc 在 manager 之后创建，用包装闭包捕获变量
	var licenseProvider plugin.LicenseProvider
	// 配置查询回调（M3.7）：同上延迟绑定（启动激活时下发插件配置）
	var configProvider plugin.ConfigProvider
	// 只读数据服务（M3.8）：同上延迟绑定（插件经 broker 查询脱敏数据）
	var dataProvider plugin.DataProvider
	pluginManager = plugin.NewPluginManager(
		binStore,
		hookDispatcher,
		service.NewPluginManagerEvents(pluginRepo),
		"logs/plugins",
		func(ctx context.Context, pluginID string) (*contract.LicenseInfo, error) {
			if licenseProvider == nil {
				return nil, nil // 未绑定：按 free（demo）处理
			}
			return licenseProvider(ctx, pluginID)
		},
		func(ctx context.Context, pluginID string) (map[string]string, error) {
			if configProvider == nil {
				return nil, nil // 未绑定：按无配置处理
			}
			return configProvider(ctx, pluginID)
		},
		func(ctx context.Context, pluginID string) ([]string, error) {
			// 登记能力查询（P2 加固）：pluginRepo 此处已就绪，无需延迟绑定；
			// 门控 = 安装登记能力 ∩ 二进制自报能力（见 manager.Start）
			inst, err := pluginRepo.FindByPluginID(ctx, pluginID)
			if err != nil {
				return nil, err
			}
			return inst.Capabilities, nil
		},
		func() plugin.DataProvider {
			return dataProvider // 延迟闭包：装配完成前返回 nil（不注册数据服务）
		},
	)
	// E4：进程管理器接入结构化日志（此前 fmt.Fprintf(os.Stderr)，不可检索不可分级）
	pluginManager.SetLogger(logger)
	// 内容治理服务（M2：举报/敏感词/封禁；先建供发帖/评论拦截注入）
	moderationSvc := service.NewModerationService(reportRepo, sensitiveRepo, banRepo, userRepo, postRepo, commentRepo)
	// 启动时加载敏感词表（后台变更后自动刷新）
	_ = moderationSvc.ReloadForbidden(ctx)
	// 媒体存储 seam 延迟绑定（pluginSvc 在 postSvc 之后构造——闭包调用时才解析）
	var mediaStorageLookup func() (plugin.MediaStorage, bool)
	postSvc := service.NewPostService(postRepo, tagRepo, mediaRepo, userRepo, mediaStore, moderationSvc, relationRepo, hookDispatcher, seoRepo,
		func() (plugin.MediaStorage, bool) {
			if mediaStorageLookup == nil {
				return nil, false
			}
			return mediaStorageLookup()
		})
	notifySvc := service.NewNotificationService(notificationRepo, userRepo, hookDispatcher)
	// AI 服务（M4：供应商/任务/用量 + 三内置场景；先建供评论预审注入）
	aiSvc := service.NewAiService(aiProviderRepo, aiTaskRepo, aiUsageRepo, seoRepo, postRepo, commentRepo, reportRepo, mediaStore, mediaRepo, settingRepo, cfg.AIKeySecret, hookDispatcher)
	commentSvc := service.NewCommentService(commentRepo, reactionRepo, userRepo, settingRepo, guestMgr, postRepo, notifySvc, moderationSvc, hookDispatcher, aiSvc)
	reactionSvc := service.NewReactionService(reactionRepo, postRepo, notifySvc)
	followSvc := service.NewFollowService(relationRepo, userRepo, postRepo, postSvc, notifySvc)
	topicSvc := service.NewTopicService(tagRepo, postRepo, postSvc)
	searchSvc := service.NewSearchService(postRepo, tagRepo, userRepo, postSvc, hookDispatcher)
	adminSvc := service.NewAdminService(adminRepo, postRepo, commentRepo, settingRepo, enforcer, banRepo, userRepo, postSvc, mediaRepo, tagRepo, mediaStore, hookDispatcher, auditRepo, reportRepo)
	siteSvc := service.NewSiteService(settingRepo, userRepo) // 站点信息（meta 从 settings 实时读取，M1.7；附带站长摘要）
	messageSvc := service.NewMessageService(messageRepo, userRepo, notifySvc) // 私信（M2）
	pluginSvc := service.NewPluginService(ghClient, pluginRepo, settingRepo, hookDispatcher, pluginManager, binStore, licenseRepo, orderRepo, cfg.AIKeySecret)
	licenseProvider = pluginSvc.LicenseInfoProvider // M3.5：许可证查询回调绑定（延迟闭包生效）
	configProvider = pluginSvc.PluginConfigProvider // M3.7：配置查询回调绑定（启动激活时下发）
	mediaStorageLookup = pluginSvc.MediaStorageSeam() // 图床 seam 绑定（图床插件运行时上传直达外部存储）
	dataProvider = service.NewPluginDataProvider(userRepo, postRepo, settingRepo, aiSvc, openAPIKeyRepo) // M3.8：只读数据服务（M4.1：+AI 辅助；+开放接口 Key 读取）
	seoSvc := service.NewSeoService(seoRepo, postRepo, "http://localhost:"+cfg.ServerPort)
	// 数据报表服务（M4-报表：统计聚合 + 趋势 CSV；复用后台聚合数据源）
	reportSvc := service.NewReportService(repository.NewAdminRepo(conn), reportRepo)
	// 备份导出服务（M4-报表：应用级 JSON/CSV/ZIP 导出 + 媒体库打包）
	backupSvc := service.NewBackupService(backupRepo, cfg.DataDir, logger)
	// 角色权限服务（M5：矩阵查询 + 权限域编辑持久化；audit 复用）
	roleSvc := service.NewRoleService(enforcer, adminRepo, settingRepo, auditRepo)
	// QQ 音乐解析服务（M7：songmid→songid，发帖内嵌播放器；无数据库依赖）
	musicSvc := service.NewQQMusicService()
	// 自定义页面服务（后台创建独立页面，前台 /pages/{slug} 访问）
	pageSvc := service.NewPageService(pageRepo)
	// 接口开放服务（外部 API Key 生成与管理）
	openAPISvc := service.NewOpenAPIService(openAPIKeyRepo)
	// GitHub OAuth 服务（M3.5：连接 GitHub 拉取私有/加速清单；凭证未配置时入口隐藏）
	oauthSvc := service.NewOAuthService(cfg.GitHubOAuthClientID, cfg.GitHubOAuthSecret, cfg.AIKeySecret, cfg.GitHubToken, settingRepo, ghClient)
	oauthSvc.RestoreToken(ctx) // 启动恢复 OAuth token（有则优先于 .env 静态 token）
	// 启动同步已启用插件（M3.2 钩子 + M3.3 进程外插件拉起子进程，重启恢复）。
	// 后台执行（修复历史 bug）：此前复用装配 ctx（Run 传入 buildHandlers 的 10s 超时）同步执行——
	// 初始化稍慢（DB 首连/OAuth 恢复等）轮到插件恢复时 ctx 已近过期，握手被静默取消，
	// DB 保持 running 而进程未拉起（状态假象，插件 API 全 500）且时序敏感、时好时坏。
	// 插件握手最长达分钟级，本就不应阻塞 HTTP 就绪：独立 ctx 后台恢复，失败仅告警。
	go func() {
		syncCtx, syncCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer syncCancel()
		if err := pluginSvc.SyncActivePlugins(syncCtx); err != nil {
			logger.Warn("插件启动同步失败", zap.Error(err))
		}
	}()

	// ---------- 角色策略加载（M5：自定义矩阵恢复 → 用户角色全量同步） ----------
	// 先恢复后台编辑过的权限矩阵（settings.role_permissions），再同步用户角色分组
	if custom, err := roleSvc.CustomMatrix(ctx); err == nil && len(custom) > 0 {
		if err := enforcer.InitPolicies(custom); err != nil {
			logger.Warn("自定义权限矩阵恢复失败，使用默认矩阵", zap.Error(err))
		}
	}
	if err := syncRoles(ctx, enforcer, userRepo, logger); err != nil {
		logger.Warn("角色策略加载失败，使用兜底策略", zap.Error(err))
	}

	// ---------- 控制器层 ----------
	handlers := router.Handlers{
		Auth:     handler.NewAuthHandler(authSvc, jwtMgr),
		User:     handler.NewUserHandler(authSvc),
		Post:     handler.NewPostHandler(postSvc, logger),
		Media:    handler.NewMediaHandler(postSvc),
		Comment:  handler.NewCommentHandler(commentSvc),
		Reaction: handler.NewReactionHandler(reactionSvc, guestMgr),
		Social: handler.NewSocialHandler(topicSvc, searchSvc, notifySvc, followSvc, logger),
		Admin:  handler.NewAdminHandler(adminSvc),
		Site:   handler.NewSiteHandler(siteSvc),
		Message: handler.NewMessageHandler(messageSvc, logger),
		Moderation: handler.NewModerationHandler(moderationSvc, logger),
		Plugin:     handler.NewPluginHandler(pluginSvc, oauthSvc),
		PluginConfig: handler.NewPluginConfigHandler(pluginSvc),
		PluginOrder: handler.NewPluginOrderHandler(pluginSvc),
		Seo:        handler.NewSeoHandler(seoSvc),
		Ai:         handler.NewAiHandler(aiSvc, logger),
		Report:     handler.NewReportHandler(reportSvc, logger),
		Backup:     handler.NewBackupHandler(backupSvc, logger),
		Role:       handler.NewRoleHandler(roleSvc),
		Music:      handler.NewMusicHandler(musicSvc, pluginSvc),
		Video:      handler.NewVideoHandler(pluginSvc),
		TTS:        handler.NewTTSHandler(pluginSvc),
		Stats:      handler.NewStatsHandler(pluginSvc),
		Page:       handler.NewPageHandler(pageSvc, logger),
		OpenAPI:     handler.NewOpenAPIHandler(openAPISvc, aiSvc, authSvc, logger),
		OpenAPIKeys: openAPIKeyRepo,
		Update:     handler.NewUpdateHandler(ghClient, cfg.DataDir),
	}
	return handlers, jwtMgr, enforcer, cleanup, nil
}

// syncRoles 从数据库全量加载角色到 casbin（M2 角色调整：users.role 为唯一数据源）。
// 参数：enforcer 权限执行器；users 用户仓库；logger 日志器（加载失败仅告警，不阻断启动）。
func syncRoles(ctx context.Context, enforcer *casbin.Enforcer, users *repository.UserRepo, logger *zap.Logger) error {
	rows, err := users.AllRoles(ctx)
	if err != nil {
		return err
	}
	roles := make([]casbin.UserRole, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, casbin.UserRole{Username: row.Username, Role: row.Role})
	}
	if err := enforcer.SyncRoles(roles); err != nil {
		return err
	}
	logger.Info("角色策略已从数据库加载", zap.Int("admin_count", len(roles)))
	return nil
}

// Run 构建并启动 HTTP 服务，阻塞直至收到退出信号或发生致命错误。
// 参数：cfg 运行配置；logger 日志器。
// 返回：错误信息（正常退出返回 nil）。
func Run(cfg config.Config, logger *zap.Logger) error {
	// 装配依赖（数据库连接等）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	handlers, jwtMgr, enforcer, cleanup, err := buildHandlers(ctx, cfg, logger)
	cancel()
	if err != nil {
		return err
	}
	// 服务退出时关闭连接
	defer cleanup()

	// 构建 Gin 引擎（路由 + 中间件链）
	engine := router.Register(cfg, logger, handlers, jwtMgr, enforcer)

	// HTTP 服务：读写超时保护，防止慢请求拖垮
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      engine,
		ReadTimeout:  30 * time.Second,
		// 写超时放宽：插件升级（Release 下载+替换）与 .bpk 上传是长操作，
		// 30 秒会把进行中的请求掐断为 500（实测：升级必失败）
		WriteTimeout: 180 * time.Second,
	}

	// 启动服务（独立 goroutine，失败时上报）
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("月言博客服务启动", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()

	// 监听退出信号（Ctrl+C / 脚本 kill）
	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serveErr:
		// 服务启动失败：直接返回错误
		return fmt.Errorf("服务启动失败：%w", err)
	case <-stopCtx.Done():
		// 收到退出信号：优雅关闭（超时 10 秒强制退出）
		logger.Info("收到退出信号，正在优雅关闭...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("优雅关闭失败：%w", err)
		}
		logger.Info("服务已退出")
		return nil
	}
}
