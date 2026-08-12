// internal/router/router.go
// 路由注册：装配前台 /api/v1 与后台 /api/v1/admin 路由组（架构文档 11.1）。
// 依赖由 server 装配后注入，M1.3 阶段接入帖子/媒体路由。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/auth"
	"github.com/roberts9012062/boke/internal/casbin"
	"github.com/roberts9012062/boke/internal/config"
	"github.com/roberts9012062/boke/internal/handler"
	"github.com/roberts9012062/boke/internal/middleware"
	"github.com/roberts9012062/boke/pkg/resp"
)

// Handlers 业务控制器集合（由 server 装配后传入，避免 router 直接依赖构造细节）。
type Handlers struct {
	Auth     *handler.AuthHandler     // 认证控制器
	User     *handler.UserHandler     // 用户控制器
	Post     *handler.PostHandler     // 帖子控制器
	Media    *handler.MediaHandler    // 媒体控制器
	Comment  *handler.CommentHandler  // 评论控制器
	Reaction *handler.ReactionHandler // 互动控制器（点赞/收藏/匿名身份）
	Social   *handler.SocialHandler   // 社交控制器（话题/搜索/通知/关注）
	Admin    *handler.AdminHandler    // 后台控制器（M1.6）
	Site     *handler.SiteHandler     // 站点信息控制器（M1.7：meta 从 settings 读取）
	Message  *handler.MessageHandler  // 私信控制器（M2）
	Moderation *handler.ModerationHandler // 内容治理控制器（M2 举报/敏感词/封禁）
	Plugin   *handler.PluginHandler   // 插件控制器（M3.1 商城/管理）
	Seo      *handler.SeoHandler      // SEO 控制器（M4）
	Ai       *handler.AiHandler       // AI 控制器（M4：供应商/任务/用量/内置场景）
	Report   *handler.ReportHandler   // 数据报表控制器（M4-报表）
	Backup   *handler.BackupHandler   // 备份导出控制器（M4-报表）
	Role     *handler.RoleHandler     // 角色权限控制器（M5）
}

// Register 注册全部路由并返回 Gin 引擎。
// 参数：cfg 运行配置；logger 请求/恢复日志输出器；handlers 业务控制器；jwtMgr JWT 管理器；
//       enforcer 权限执行器（M5 RBAC 后台路由按资源域挂 RequirePermission）。
func Register(cfg config.Config, logger *zap.Logger, handlers Handlers, jwtMgr *auth.Manager, enforcer *casbin.Enforcer) *gin.Engine {
	// 使用 gin.New（不启用 Gin 默认日志，统一走 zap）
	engine := gin.New()

	// ---------- 全局中间件链：恢复 → 请求ID → CORS → 日志 ----------
	engine.Use(middleware.Recovery(logger))
	engine.Use(middleware.RequestID())
	engine.Use(middleware.CORS(cfg.CORSOrigin))
	engine.Use(middleware.RequestLogger(logger))

	// ---------- 基础路由 ----------
	// 健康检查（部署探活 / 验收脚本使用）
	engine.GET("/healthz", func(c *gin.Context) {
		resp.OK(c, gin.H{"status": "ok"})
	})
	// 媒体静态服务：/media/202608/xxx.jpg（data/media 目录）
	engine.StaticFS("/media", http.Dir(cfg.DataDir+"/media"))
	// 插件前端资源静态服务（M3.6：/plugin-assets/{id}/frontend/*，公开——页面渲染时无需登录；
	// 资源安装时已 checksums 全量校验；独立前缀避免与 /api/v1/plugins/:id 通配冲突）
	engine.GET("/plugin-assets/:id/*filepath", handlers.Plugin.Asset)
	// SEO 公开端点（M4）：sitemap.xml / robots.txt
	engine.GET("/sitemap.xml", handlers.Seo.Sitemap)
	engine.GET("/robots.txt", handlers.Seo.Robots)

	// ---------- API v1 路由组 ----------
	api := engine.Group("/api/v1")
	// 全站维护中间件（M2）：维护开关开启时拦截前台 API，放行后台/认证/meta
	api.Use(middleware.Maintenance(handlers.Site.MaintenanceMode))
	// 前台插件扩展清单（M3.6：公开——页面槽位加载插件扩展；独立前缀避免与 /plugins/:id 参数段冲突）
	api.GET("/plugin-extensions", handlers.Plugin.Extensions)
	registerV1(api, handlers, jwtMgr, enforcer)

	return engine
}

// registerV1 注册 /api/v1 下各业务模块路由。
func registerV1(api *gin.RouterGroup, handlers Handlers, jwtMgr *auth.Manager, enforcer *casbin.Enforcer) {
	// ---------- 认证模块（M1.2） ----------
	authGroup := api.Group("/auth")
	authGroup.POST("/register", handlers.Auth.Register) // 注册（注册即登录）
	authGroup.POST("/login", handlers.Auth.Login)       // 登录
	authGroup.POST("/refresh", handlers.Auth.Refresh)   // 刷新令牌对
	authGroup.POST("/forgot-password", handlers.Auth.ForgotPassword) // 请求重置（M2）
	authGroup.POST("/reset-password", handlers.Auth.ResetPassword)   // 重置密码（M2）

	// 需要登录的路由组
	authed := api.Group("")
	authed.Use(middleware.RequireAuth(jwtMgr))
	authed.POST("/auth/logout", handlers.Auth.Logout) // 登出（撤销 refresh）
	authed.GET("/me", handlers.Auth.Me)               // 当前用户资料
	authed.PUT("/me/password", handlers.Auth.ChangePassword) // 修改密码（账号安全页）

	// 插件自定义 API 代理（M3.3：/api/plugins/{id}/** 转发子进程，登录用户可用）
	pluginAPI := authed.Group("/plugins/:id")
	pluginAPI.Any("/*path", handlers.Plugin.Call)

	// 前台写操作拦截（M5：受限访客仅可阅读；路由级挂 RequireNotRestricted，
	// 发帖/编辑/评论/回复/私信发送——评论接口在 OptionalAuth 组，匿名按 visitor 放行）

	// ---------- 用户模块（M1.2 基础） ----------
	api.GET("/users/:id", handlers.User.GetUser) // 用户公开资料

	// ---------- 帖子模块（M1.3） ----------
	// 列表/详情挂可选鉴权：登录用户可看自己的私密帖（viewerID 识别）
	api.GET("/posts", middleware.OptionalAuth(jwtMgr), handlers.Post.List) // 时间线
	api.GET("/posts/:id", middleware.OptionalAuth(jwtMgr), handlers.Post.Get) // 帖子详情

	// 需要登录的帖子操作（M5：写操作挂 RequireNotRestricted，受限访客 403）
	authed.POST("/posts", middleware.RequireNotRestricted(), handlers.Post.Create)       // 发帖/存草稿
	authed.PUT("/posts/:id", middleware.RequireNotRestricted(), handlers.Post.Update)    // 更新帖子
	authed.POST("/posts/:id/publish", middleware.RequireNotRestricted(), handlers.Post.Publish) // 发布草稿
	authed.DELETE("/posts/:id", handlers.Post.Delete) // 删除帖子
	authed.GET("/me/drafts", handlers.Post.ListDrafts) // 草稿箱

	// ---------- 媒体模块（M1.3） ----------
	authed.POST("/media", handlers.Media.Upload) // 媒体上传（multipart）

	// ---------- 评论模块（M1.4） ----------
	// 评论接口开放（登录/匿名均可）：挂可选鉴权识别登录用户身份；
	// 发表/回复挂 RequireNotRestricted（M5：受限访客 403，匿名按 visitor 放行）
	api.GET("/posts/:id/comments", middleware.OptionalAuth(jwtMgr), handlers.Comment.List)     // 评论列表
	api.POST("/posts/:id/comments", middleware.OptionalAuth(jwtMgr), middleware.RequireNotRestricted(), handlers.Comment.Create)  // 发表评论
	api.POST("/comments/:id/reply", middleware.OptionalAuth(jwtMgr), middleware.RequireNotRestricted(), handlers.Comment.Reply)   // 回复评论
	authed.POST("/comments/:id/like", handlers.Comment.Like)  // 评论点赞
	authed.DELETE("/comments/:id", handlers.Comment.Delete)   // 删除评论

	// ---------- 互动模块（M1.4） ----------
	api.POST("/guest-identity", handlers.Reaction.GuestIdentity) // 匿名身份签发
	api.GET("/posts/:id/state", handlers.Reaction.PostState)     // 帖子互动状态
	authed.POST("/posts/:id/like", handlers.Reaction.LikePost)         // 点赞
	authed.DELETE("/posts/:id/like", handlers.Reaction.UnlikePost)     // 取消点赞
	authed.POST("/posts/:id/favorite", handlers.Reaction.FavoritePost) // 收藏
	authed.DELETE("/posts/:id/favorite", handlers.Reaction.UnfavoritePost) // 取消收藏

	// ---------- 话题模块（M1.5） ----------
	api.GET("/topics", middleware.OptionalAuth(jwtMgr), handlers.Social.ListTopics)              // 话题列表
	api.GET("/topics/:name", middleware.OptionalAuth(jwtMgr), handlers.Social.GetTopic)          // 话题详情
	api.GET("/topics/:name/posts", middleware.OptionalAuth(jwtMgr), handlers.Social.ListTopicPosts) // 话题帖子流
	authed.POST("/topics/:name/follow", handlers.Social.FollowTopic)     // 关注话题
	authed.DELETE("/topics/:name/follow", handlers.Social.UnfollowTopic) // 取消关注话题

	// ---------- 搜索模块（M1.5） ----------
	api.GET("/search", middleware.OptionalAuth(jwtMgr), handlers.Social.Search) // 搜索（q=关键词）

	// ---------- 通知模块（M1.5） ----------
	authed.GET("/notifications", handlers.Social.ListNotifications)                 // 通知列表（type= 过滤）
	authed.GET("/notifications/unread-count", handlers.Social.CountUnreadNotifications) // 未读数（角标）
	authed.PUT("/notifications/read-all", handlers.Social.MarkAllNotificationsRead) // 全部已读
	authed.PUT("/notifications/:id/read", handlers.Social.MarkNotificationRead)    // 单条已读

	// ---------- 用户关系模块（M1.5） ----------
	authed.PUT("/users/:id/follow", handlers.Social.FollowUser)       // 关注用户
	authed.DELETE("/users/:id/follow", handlers.Social.UnfollowUser)  // 取消关注
	api.GET("/users/:id/followers", middleware.OptionalAuth(jwtMgr), handlers.Social.ListFollowers) // 粉丝
	api.GET("/users/:id/following", middleware.OptionalAuth(jwtMgr), handlers.Social.ListFollowing) // 关注
	api.GET("/users/:id/liked", middleware.OptionalAuth(jwtMgr), handlers.Social.LikedPosts)        // 赞过
	api.GET("/users/:id/posts", middleware.OptionalAuth(jwtMgr), handlers.Social.UserPosts)         // 主页帖子流（type= 过滤）
		authed.PUT("/me/profile", handlers.Social.UpdateProfile)   // 编辑资料
		authed.PUT("/me/avatar", handlers.Social.UpdateAvatar)     // 更新头像（M1.7）
		authed.GET("/me/favorites", handlers.Social.Favorites)     // 我的收藏

		// ---------- 站点元信息（M1.7：改从 settings 表实时读取） ----------
		api.GET("/meta", handlers.Site.GetMeta)

		// ---------- 私信模块（M2） ----------
		authed.GET("/conversations", handlers.Message.ListConversations)          // 会话列表（filter=all|unread）
		authed.POST("/conversations", handlers.Message.OpenConversation)          // 发起/打开会话（{user_id}）
		authed.GET("/conversations/unread-count", handlers.Message.UnreadTotal)   // 未读总数（角标）
		authed.GET("/conversations/:id/messages", handlers.Message.ListMessages)  // 消息列表（打开即已读）
		authed.POST("/conversations/:id/messages", middleware.RequireNotRestricted(), handlers.Message.SendMessage)  // 发送消息（M5：受限访客 403）

		// ---------- 内容治理（M2）：前台举报 ----------
		authed.POST("/reports", handlers.Moderation.SubmitReport) // 提交举报（帖子/评论/用户）

		// ---------- 后台路由组（M5 RBAC：按资源域挂 RequirePermission） ----------
		adminGroup := api.Group("/admin")
		adminGroup.Use(middleware.RequireAuth(jwtMgr))
		// perm 域权限中间件工厂（enforcer 由 server 注入）
		perm := func(domain string) gin.HandlerFunc { return middleware.RequirePermission(enforcer, domain) }

		// 仪表盘域
		dash := adminGroup.Group("", perm(casbin.DomainDashboard))
		dash.GET("/ping", func(c *gin.Context) { // 后台健康探活
			resp.OK(c, gin.H{"role": middleware.GetRole(c), "uid": middleware.GetUserID(c)})
		})
		dash.GET("/dashboard", handlers.Admin.Dashboard)
		// 内容管理域（author 角色仅自己帖子，service 层数据隔离）
		posts := adminGroup.Group("/posts", perm(casbin.DomainPosts))
		posts.GET("", handlers.Admin.ListPosts)
		posts.GET("/:id", handlers.Admin.GetPost)      // 后台编辑详情（M2）
		posts.PUT("/:id", handlers.Admin.UpdatePost)   // 后台编辑保存（M2）
		posts.PUT("/:id/status", handlers.Admin.SetPostStatus)
		posts.DELETE("/:id", handlers.Admin.DeletePost)
		// 评论管理域
		comments := adminGroup.Group("/comments", perm(casbin.DomainComments))
		comments.GET("", handlers.Admin.ListComments)
		comments.GET("/stats", handlers.Admin.CommentStats)              // 统计条（M2）
		comments.PUT("/:id/status", handlers.Admin.SetCommentStatus)     // 隐藏/恢复（M2）
		comments.DELETE("/:id", handlers.Admin.DeleteComment)
		// 用户管理域（含角色调整）
		users := adminGroup.Group("/users", perm(casbin.DomainUsers))
		users.GET("", handlers.Admin.ListUsers)
		users.GET("/stats", handlers.Admin.UserStats)
		users.PUT("/:id/status", handlers.Admin.SetUserStatus)
		users.PUT("/:id/role", handlers.Admin.SetUserRole) // 角色调整（M2→M5 五级）
		// 媒体库域（M2.9）
		media := adminGroup.Group("/media", perm(casbin.DomainMedia))
		media.GET("", handlers.Admin.ListMedia)
		media.GET("/stats", handlers.Admin.MediaStats)
		media.DELETE("/:id", handlers.Admin.DeleteMedia)
		// 标签分类域（M2.9）
		tags := adminGroup.Group("/tags", perm(casbin.DomainTags))
		tags.GET("", handlers.Admin.ListTags)
		tags.GET("/stats", handlers.Admin.TagStats)
		tags.PUT("/:id", handlers.Admin.RenameTag)
		tags.POST("/:id/merge", handlers.Admin.MergeTag)
		tags.DELETE("/:id", handlers.Admin.DeleteTag)
		// 站点设置域
		settings := adminGroup.Group("/settings", perm(casbin.DomainSettings))
		settings.GET("", handlers.Admin.GetSettings)
		settings.PUT("", handlers.Admin.SaveSettings)
		// 角色权限域（M5，设计稿《后台角色》）
		roles := adminGroup.Group("/roles", perm(casbin.DomainRoles))
		roles.GET("", handlers.Role.Matrix)
		roles.PUT("/:role/permissions", handlers.Role.UpdatePermissions)
		// 内容治理域（M2）：举报工单 / 敏感词 / 封禁记录
		reports := adminGroup.Group("/reports", perm(casbin.DomainModeration))
		reports.GET("/stats", handlers.Moderation.ReportStats)
		reports.GET("", handlers.Moderation.ListReports)
		reports.PUT("/:id/status", handlers.Moderation.SetReportStatus)
		reports.POST("/:id/verdict", handlers.Moderation.VerdictReport) // M4：AI 工单放行/删除
		sw := adminGroup.Group("/sensitive-words", perm(casbin.DomainModeration))
		sw.GET("/stats", handlers.Moderation.SensitiveStats)
		sw.GET("", handlers.Moderation.ListSensitiveWords)
		sw.POST("", handlers.Moderation.AddSensitiveWord)
		sw.POST("/batch", handlers.Moderation.AddSensitiveWords) // 设置页逗号分隔批量（M2 设计稿纠偏）
		sw.DELETE("/:word", handlers.Moderation.DeleteSensitiveWord)
		bans := adminGroup.Group("/bans", perm(casbin.DomainModeration))
		bans.GET("", handlers.Moderation.ListBans)
		// 插件域（M3.1：GitHub 仓库清单驱动商城 + 安装管理）
		plugins := adminGroup.Group("/plugins", perm(casbin.DomainPlugins))
		plugins.GET("/market", handlers.Plugin.Market)      // 商城清单（?source= 自定义仓库）
		plugins.GET("", handlers.Plugin.ListInstalled)      // 我的插件
		plugins.POST("/install", handlers.Plugin.Install)   // 安装 {plugin_id}
		plugins.POST("/upload", handlers.Plugin.Upload)     // 本地上传 .bpk 安装（M3.4）
		plugins.PUT("/:id/state", handlers.Plugin.SetState) // 启用/禁用
		plugins.DELETE("/:id", handlers.Plugin.Uninstall)   // 卸载
		plugins.GET("/:id/license", handlers.Plugin.LicenseStatus)    // 许可证状态（M3.5）
		plugins.POST("/:id/license", handlers.Plugin.ActivateLicense) // 激活许可证（M3.5）
		// GitHub OAuth（M3.5：插件市场设置区连接；独立前缀避免与 /plugins/:id 参数段冲突）
		pluginOAuth := adminGroup.Group("/plugin-oauth", perm(casbin.DomainPlugins))
		pluginOAuth.GET("/authorize", handlers.Plugin.OAuthAuthorize)     // 发起连接（返回跳转 URL）
		pluginOAuth.GET("/callback", handlers.Plugin.OAuthCallback)       // 授权回调（?code=）
		pluginOAuth.GET("/status", handlers.Plugin.OAuthStatus)           // 连接状态
		pluginOAuth.POST("/disconnect", handlers.Plugin.OAuthDisconnect)  // 断开连接
		// SEO 域（M4）：设置/元数据/健康度/批量修复/SERP
		seo := adminGroup.Group("/seo", perm(casbin.DomainSeo))
		seo.GET("/settings", handlers.Seo.GetSettings)
		seo.PUT("/settings", handlers.Seo.SaveSettings)
		seo.GET("/meta/:postId", handlers.Seo.GetMeta)
		seo.PUT("/meta/:postId", handlers.Seo.SaveMeta)
		seo.GET("/health", handlers.Seo.Health)
		seo.POST("/health/scan", handlers.Seo.ScanHealth)
		seo.POST("/batch-fix", handlers.Seo.BatchFix)
		seo.GET("/serp-preview", handlers.Seo.SerpPreview)
		// AI 域（M4）：供应商 / 任务配置 / 用量 / 内置场景（摘要/标签/评论审核）
		ai := adminGroup.Group("/ai", perm(casbin.DomainAi))
		ai.GET("/providers", handlers.Ai.ListProviders)
		ai.POST("/providers", handlers.Ai.CreateProvider)
		ai.PUT("/providers/:id", handlers.Ai.UpdateProvider)
		ai.DELETE("/providers/:id", handlers.Ai.DeleteProvider)
		ai.POST("/providers/:id/test", handlers.Ai.TestProvider)
		ai.GET("/tasks", handlers.Ai.ListTasks)
		ai.PUT("/tasks/:name", handlers.Ai.UpdateTask)
		ai.POST("/tasks/:name/toggle", handlers.Ai.ToggleTask)
		ai.GET("/usage", handlers.Ai.UsageStats)
		ai.POST("/gen/summary", handlers.Ai.GenSummary)        // ?post_id= 生成摘要（seo_meta.summary）
		ai.POST("/gen/tags", handlers.Ai.GenTags)              // ?post_id= 生成标签建议
		ai.POST("/review/comments", handlers.Ai.ReviewComments) // 批量 AI 审核评论
		// 数据报表域（M4-报表，设计稿《数据报表》；与举报 /reports 前缀并存，路径不同）
		reportsView := adminGroup.Group("/reports", perm(casbin.DomainReports))
		reportsView.GET("/overview", handlers.Report.Overview)     // ?days=7|30
		reportsView.GET("/export.csv", handlers.Report.ExportCSV)  // ?days= 附件下载
		// 备份导出域（M4-报表，设计稿《备份导出》）：记录/创建/下载/删除
		backups := adminGroup.Group("/backups", perm(casbin.DomainBackups))
		backups.GET("", handlers.Backup.List)
		backups.POST("", handlers.Backup.Create)
		backups.GET("/:id/download", handlers.Backup.Download)
		backups.DELETE("/:id", handlers.Backup.Delete)
	}
