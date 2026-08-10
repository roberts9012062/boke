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

	"github.com/yueyan/boke/internal/auth"
	"github.com/yueyan/boke/internal/casbin"
	"github.com/yueyan/boke/internal/config"
	"github.com/yueyan/boke/internal/handler"
	"github.com/yueyan/boke/internal/mail"
	"github.com/yueyan/boke/internal/media"
	"github.com/yueyan/boke/internal/redis"
	"github.com/yueyan/boke/internal/repository"
	"github.com/yueyan/boke/internal/router"
	"github.com/yueyan/boke/internal/service"
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
func buildHandlers(ctx context.Context, cfg config.Config, logger *zap.Logger) (router.Handlers, *auth.Manager, func(), error) {
	// ---------- 数据库连接 ----------
	conn, err := connectDatabase(ctx, cfg.DB.ConnString())
	if err != nil {
		return router.Handlers{}, nil, nil, fmt.Errorf("连接数据库失败：%w", err)
	}
	// 退出时关闭数据库连接池
	cleanup := func() {
		conn.Close()
	}

	// ---------- Redis 连接（不可用时降级，不阻断启动） ----------
	redisClient := redis.NewClient(ctx, redis.Config{Host: cfg.Redis.Host, Port: cfg.Redis.Port, DB: cfg.Redis.DB})
	if redisClient == nil {
		logger.Warn("Redis 不可用，登录限流与令牌黑名单降级放行")
	}

	// ---------- 核心组件装配 ----------
	jwtMgr := auth.NewManager(cfg.JWTSecret)     // JWT 管理器
	enforcer, err := casbin.NewEnforcer()        // 角色权限执行器
	if err != nil {
		cleanup()
		return router.Handlers{}, nil, nil, fmt.Errorf("初始化权限执行器失败：%w", err)
	}
	// 媒体存储（data/media 目录）
	mediaStore, err := media.NewStore(cfg.DataDir + "/media")
	if err != nil {
		cleanup()
		return router.Handlers{}, nil, nil, err
	}

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
	reportRepo := repository.NewReportRepo(conn)      // 举报工单（M2 内容治理）
	sensitiveRepo := repository.NewSensitiveRepo(conn) // 敏感词（M2）
	banRepo := repository.NewBanRepo(conn)             // 封禁记录（M2）

	// ---------- 业务层 ----------
	limiter := redis.NewRateLimiter(redisClient)
	guestMgr := auth.NewGuestManager() // 匿名身份管理器（内存）
	resetMgr := auth.NewResetManager()                 // 密码重置令牌（M2）
	mailer := mail.NewSender(cfg.Mail, logger)         // 邮件发送（M2；未配置降级日志）
	authSvc := service.NewAuthService(userRepo, auditRepo, jwtMgr, enforcer, limiter, resetMgr, mailer, cfg.SiteBaseURL)
	// 内容治理服务（M2：举报/敏感词/封禁；先建供发帖/评论拦截注入）
	moderationSvc := service.NewModerationService(reportRepo, sensitiveRepo, banRepo, userRepo, postRepo, commentRepo)
	// 启动时加载敏感词表（后台变更后自动刷新）
	_ = moderationSvc.ReloadForbidden(ctx)
	postSvc := service.NewPostService(postRepo, tagRepo, mediaRepo, userRepo, mediaStore, moderationSvc, relationRepo)
	notifySvc := service.NewNotificationService(notificationRepo, userRepo)
	commentSvc := service.NewCommentService(commentRepo, reactionRepo, userRepo, guestMgr, postRepo, notifySvc, moderationSvc)
	reactionSvc := service.NewReactionService(reactionRepo, postRepo, notifySvc)
	followSvc := service.NewFollowService(relationRepo, userRepo, postRepo, postSvc, notifySvc)
	topicSvc := service.NewTopicService(tagRepo, postRepo, postSvc)
	searchSvc := service.NewSearchService(postRepo, tagRepo, userRepo, postSvc)
	adminSvc := service.NewAdminService(adminRepo, postRepo, commentRepo, settingRepo, enforcer, banRepo)
	siteSvc := service.NewSiteService(settingRepo) // 站点信息（meta 从 settings 实时读取，M1.7）
	messageSvc := service.NewMessageService(messageRepo, userRepo, notifySvc) // 私信（M2）

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
	}
	return handlers, jwtMgr, cleanup, nil
}

// Run 构建并启动 HTTP 服务，阻塞直至收到退出信号或发生致命错误。
// 参数：cfg 运行配置；logger 日志器。
// 返回：错误信息（正常退出返回 nil）。
func Run(cfg config.Config, logger *zap.Logger) error {
	// 装配依赖（数据库连接等）
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	handlers, jwtMgr, cleanup, err := buildHandlers(ctx, cfg, logger)
	cancel()
	if err != nil {
		return err
	}
	// 服务退出时关闭连接
	defer cleanup()

	// 构建 Gin 引擎（路由 + 中间件链）
	engine := router.Register(cfg, logger, handlers, jwtMgr)

	// HTTP 服务：读写超时保护，防止慢请求拖垮
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      engine,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
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
