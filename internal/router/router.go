// internal/router/router.go
// 路由注册：装配前台 /api/v1 与后台 /api/v1/admin 路由组（架构文档 11.1）。
// 依赖由 server 装配后注入，M1.3 阶段接入帖子/媒体路由。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/auth"
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
}

// Register 注册全部路由并返回 Gin 引擎。
// 参数：cfg 运行配置；logger 请求/恢复日志输出器；handlers 业务控制器；jwtMgr JWT 管理器。
func Register(cfg config.Config, logger *zap.Logger, handlers Handlers, jwtMgr *auth.Manager) *gin.Engine {
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
	// SEO 公开端点（M4）：sitemap.xml / robots.txt
	engine.GET("/sitemap.xml", handlers.Seo.Sitemap)
	engine.GET("/robots.txt", handlers.Seo.Robots)

	// ---------- API v1 路由组 ----------
	api := engine.Group("/api/v1")
	// 全站维护中间件（M2）：维护开关开启时拦截前台 API，放行后台/认证/meta
	api.Use(middleware.Maintenance(handlers.Site.MaintenanceMode))
	registerV1(api, handlers, jwtMgr)

	return engine
}

// registerV1 注册 /api/v1 下各业务模块路由。
func registerV1(api *gin.RouterGroup, handlers Handlers, jwtMgr *auth.Manager) {
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

	// ---------- 用户模块（M1.2 基础） ----------
	api.GET("/users/:id", handlers.User.GetUser) // 用户公开资料

	// ---------- 帖子模块（M1.3） ----------
	// 列表/详情挂可选鉴权：登录用户可看自己的私密帖（viewerID 识别）
	api.GET("/posts", middleware.OptionalAuth(jwtMgr), handlers.Post.List) // 时间线
	api.GET("/posts/:id", middleware.OptionalAuth(jwtMgr), handlers.Post.Get) // 帖子详情

	// 需要登录的帖子操作
	authed.POST("/posts", handlers.Post.Create)       // 发帖/存草稿
	authed.PUT("/posts/:id", handlers.Post.Update)    // 更新帖子
	authed.POST("/posts/:id/publish", handlers.Post.Publish) // 发布草稿
	authed.DELETE("/posts/:id", handlers.Post.Delete) // 删除帖子
	authed.GET("/me/drafts", handlers.Post.ListDrafts) // 草稿箱

	// ---------- 媒体模块（M1.3） ----------
	authed.POST("/media", handlers.Media.Upload) // 媒体上传（multipart）

	// ---------- 评论模块（M1.4） ----------
	// 评论接口开放（登录/匿名均可）：挂可选鉴权识别登录用户身份
	api.GET("/posts/:id/comments", middleware.OptionalAuth(jwtMgr), handlers.Comment.List)     // 评论列表
	api.POST("/posts/:id/comments", middleware.OptionalAuth(jwtMgr), handlers.Comment.Create)  // 发表评论
	api.POST("/comments/:id/reply", middleware.OptionalAuth(jwtMgr), handlers.Comment.Reply)   // 回复评论
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
		authed.POST("/conversations/:id/messages", handlers.Message.SendMessage)  // 发送消息（{content}）

		// ---------- 内容治理（M2）：前台举报 ----------
		authed.POST("/reports", handlers.Moderation.SubmitReport) // 提交举报（帖子/评论/用户）

	// ---------- 后台路由组（M1.6；admin 角色校验） ----------
	adminGroup := api.Group("/admin")
	adminGroup.Use(middleware.RequireAuth(jwtMgr), middleware.RequireAdmin())
	adminGroup.GET("/ping", func(c *gin.Context) { // 后台健康探活
		resp.OK(c, gin.H{"role": middleware.GetRole(c), "uid": middleware.GetUserID(c)})
	})
	// 仪表盘
	adminGroup.GET("/dashboard", handlers.Admin.Dashboard)
	// 内容管理
	adminGroup.GET("/posts", handlers.Admin.ListPosts)
	adminGroup.GET("/posts/:id", handlers.Admin.GetPost)      // 后台编辑详情（M2）
	adminGroup.PUT("/posts/:id", handlers.Admin.UpdatePost)   // 后台编辑保存（M2）
	adminGroup.PUT("/posts/:id/status", handlers.Admin.SetPostStatus)
	adminGroup.DELETE("/posts/:id", handlers.Admin.DeletePost)
	// 评论管理
	adminGroup.GET("/comments", handlers.Admin.ListComments)
	adminGroup.GET("/comments/stats", handlers.Admin.CommentStats) // 统计条（M2）
	adminGroup.PUT("/comments/:id/status", handlers.Admin.SetCommentStatus) // 隐藏/恢复（M2）
	adminGroup.DELETE("/comments/:id", handlers.Admin.DeleteComment)
	// 用户管理
	adminGroup.GET("/users", handlers.Admin.ListUsers)
	adminGroup.PUT("/users/:id/status", handlers.Admin.SetUserStatus)
	adminGroup.PUT("/users/:id/role", handlers.Admin.SetUserRole) // 角色调整（M2）
	// 媒体库（M2.9）
	adminGroup.GET("/media", handlers.Admin.ListMedia)
	adminGroup.GET("/media/stats", handlers.Admin.MediaStats)
	adminGroup.DELETE("/media/:id", handlers.Admin.DeleteMedia)
	// 标签分类（M2.9）
	adminGroup.GET("/tags", handlers.Admin.ListTags)
	adminGroup.GET("/tags/stats", handlers.Admin.TagStats)
	adminGroup.PUT("/tags/:id", handlers.Admin.RenameTag)
	adminGroup.POST("/tags/:id/merge", handlers.Admin.MergeTag)
	adminGroup.DELETE("/tags/:id", handlers.Admin.DeleteTag)
	// 站点设置
	adminGroup.GET("/settings", handlers.Admin.GetSettings)
	adminGroup.PUT("/settings", handlers.Admin.SaveSettings)
	// 内容治理（M2）：举报工单 / 敏感词 / 封禁记录
	adminGroup.GET("/reports/stats", handlers.Moderation.ReportStats)
	adminGroup.GET("/reports", handlers.Moderation.ListReports)
	adminGroup.PUT("/reports/:id/status", handlers.Moderation.SetReportStatus)
	adminGroup.POST("/reports/:id/verdict", handlers.Moderation.VerdictReport) // M4：AI 工单放行/删除
	adminGroup.GET("/sensitive-words/stats", handlers.Moderation.SensitiveStats)
	adminGroup.GET("/sensitive-words", handlers.Moderation.ListSensitiveWords)
	adminGroup.POST("/sensitive-words", handlers.Moderation.AddSensitiveWord)
	adminGroup.POST("/sensitive-words/batch", handlers.Moderation.AddSensitiveWords) // 设置页逗号分隔批量（M2 设计稿纠偏）
	adminGroup.DELETE("/sensitive-words/:word", handlers.Moderation.DeleteSensitiveWord)
	adminGroup.GET("/bans", handlers.Moderation.ListBans)
	adminGroup.GET("/users/stats", handlers.Admin.UserStats)
	// 插件（M3.1：GitHub 仓库清单驱动商城 + 安装管理）
	adminGroup.GET("/plugins/market", handlers.Plugin.Market)     // 商城清单（?source= 自定义仓库）
	adminGroup.GET("/plugins", handlers.Plugin.ListInstalled)     // 我的插件
	adminGroup.POST("/plugins/install", handlers.Plugin.Install)  // 安装 {plugin_id}
	adminGroup.PUT("/plugins/:id/state", handlers.Plugin.SetState) // 启用/禁用
	adminGroup.DELETE("/plugins/:id", handlers.Plugin.Uninstall)   // 卸载
	// SEO（M4）：设置/元数据/健康度/批量修复/SERP
	adminGroup.GET("/seo/settings", handlers.Seo.GetSettings)
	adminGroup.PUT("/seo/settings", handlers.Seo.SaveSettings)
	adminGroup.GET("/seo/meta/:postId", handlers.Seo.GetMeta)
	adminGroup.PUT("/seo/meta/:postId", handlers.Seo.SaveMeta)
	adminGroup.GET("/seo/health", handlers.Seo.Health)
	adminGroup.POST("/seo/health/scan", handlers.Seo.ScanHealth)
	adminGroup.POST("/seo/batch-fix", handlers.Seo.BatchFix)
	adminGroup.GET("/seo/serp-preview", handlers.Seo.SerpPreview)
	// AI（M4）：供应商 / 任务配置 / 用量 / 内置场景（摘要/标签/评论审核）
	adminGroup.GET("/ai/providers", handlers.Ai.ListProviders)
	adminGroup.POST("/ai/providers", handlers.Ai.CreateProvider)
	adminGroup.PUT("/ai/providers/:id", handlers.Ai.UpdateProvider)
	adminGroup.DELETE("/ai/providers/:id", handlers.Ai.DeleteProvider)
	adminGroup.POST("/ai/providers/:id/test", handlers.Ai.TestProvider)
	adminGroup.GET("/ai/tasks", handlers.Ai.ListTasks)
	adminGroup.PUT("/ai/tasks/:name", handlers.Ai.UpdateTask)
	adminGroup.POST("/ai/tasks/:name/toggle", handlers.Ai.ToggleTask)
	adminGroup.GET("/ai/usage", handlers.Ai.UsageStats)
	adminGroup.POST("/ai/gen/summary", handlers.Ai.GenSummary)  // ?post_id= 生成摘要（seo_meta.summary）
	adminGroup.POST("/ai/gen/tags", handlers.Ai.GenTags)        // ?post_id= 生成标签建议
	adminGroup.POST("/ai/review/comments", handlers.Ai.ReviewComments) // 批量 AI 审核评论
	// 数据报表（M4-报表，设计稿《数据报表》）：overview 聚合 + 趋势 CSV 导出
	adminGroup.GET("/reports/overview", handlers.Report.Overview) // ?days=7|30
	adminGroup.GET("/reports/export.csv", handlers.Report.ExportCSV) // ?days= 附件下载
	// 备份导出（M4-报表，设计稿《备份导出》）：记录/创建/下载/删除
	adminGroup.GET("/backups", handlers.Backup.List)
	adminGroup.POST("/backups", handlers.Backup.Create)
	adminGroup.GET("/backups/:id/download", handlers.Backup.Download)
	adminGroup.DELETE("/backups/:id", handlers.Backup.Delete)
}
