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
	"github.com/roberts9012062/boke/internal/pluginshared"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/resp"
)

// Handlers 业务控制器集合（由 server 装配后传入，避免 router 直接依赖构造细节）。
type Handlers struct {
	Auth     *handler.AuthHandler     // 认证控制器
	User     *handler.UserHandler     // 用户控制器
	Post     *handler.PostHandler     // 帖子控制器
	Page     *handler.PageHandler     // 自定义页面控制器（自定义页面 + 导航自定义）
	Media    *handler.MediaHandler    // 媒体控制器
	Comment  *handler.CommentHandler  // 评论控制器
	Reaction *handler.ReactionHandler // 互动控制器（点赞/收藏/匿名身份）
	Social   *handler.SocialHandler   // 社交控制器（话题/搜索/通知/关注）
	Admin    *handler.AdminHandler    // 后台控制器（M1.6）
	Site     *handler.SiteHandler     // 站点信息控制器（M1.7：meta 从 settings 读取）
	Message  *handler.MessageHandler  // 私信控制器（M2）
	Moderation *handler.ModerationHandler // 内容治理控制器（M2 举报/敏感词/封禁）
	Plugin   *handler.PluginHandler   // 插件控制器（M3.1 商城/管理）
	PluginConfig *handler.PluginConfigHandler // 插件设置控制器（M3.7 设置端到端）
	PluginOrder *handler.PluginOrderHandler // 插件订单控制器（M3.9 支付渠道）
	Seo      *handler.SeoHandler      // SEO 控制器（M4）
	Ai       *handler.AiHandler       // AI 控制器（M4：供应商/任务/用量/内置场景）
	Report   *handler.ReportHandler   // 数据报表控制器（M4-报表）
	Backup   *handler.BackupHandler   // 备份导出控制器（M4-报表）
	Role     *handler.RoleHandler     // 角色权限控制器（M5）
	Music    *handler.MusicHandler    // 音乐解析控制器（M7：QQ songmid→songid）
	Nav      *handler.NavBridgeHandler // 精品导航桥接控制器（nav-links 插件访客/开放网关通道）
	PluginOpenGateway *handler.PluginOpenGatewayHandler // 插件开放网关泛化转发（声明式开放端点）
	PluginOpenCatalog *service.PluginOpenCatalog        // 插件开放目录聚合器（网关鉴权索引 + 目录合并）
	Video    *handler.VideoHandler    // B站视频桥接控制器（bilibili-video 插件游客通道）
	TTS      *handler.TTSHandler      // 朗读桥接控制器（tts-reader 插件游客通道）
	Stats    *handler.StatsHandler    // 统计桥接控制器（stats-pro 插件访客上报通道）
	OpenAPI     *handler.OpenAPIHandler       // 接口开放控制器（目录与凭证管理）
	OpenAPIKeys *repository.OpenAPIKeyRepo   // 开放网关鉴权依赖（ApiKeyAuth 中间件查询凭证用）
	Update    *handler.UpdateHandler   // 站点更新控制器（版本检查/触发/进度）
	Relay     *handler.RelayHandler    // 中继站控制器（大世界：后台配置 + 前台列表）
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
	// 安装状态查询（正常模式恒 installed=true）：前端 middleware 探测全站引导用；
	// 未安装时由安装模式服务（internal/server/setup_mode.go）提供同名路由与完整向导接口
	engine.GET("/api/setup/status", func(c *gin.Context) {
		resp.OK(c, gin.H{"installed": true, "mode": "manual", "version": "1"})
	})
	// 媒体静态服务：/media/202608/xxx.jpg（data/media 目录）
	engine.StaticFS("/media", http.Dir(cfg.DataDir+"/media"))
	// 插件前端资源静态服务（M3.6：/plugin-assets/{id}/frontend/*，公开——页面渲染时无需登录；
	// 资源安装时已 checksums 全量校验；独立前缀避免与 /api/v1/plugins/:id 通配冲突）
	engine.GET("/plugin-assets/:id/*filepath", handlers.Plugin.Asset)
	// 插件前端共享 SDK（E2 去重：escapeHtml/试播控制器/页面骨架，同源 ESM 公开分发）
	engine.GET("/plugin-sdk/shared.js", pluginshared.SharedJS)
	// SEO 公开端点（M4）：sitemap.xml / robots.txt
	engine.GET("/sitemap.xml", handlers.Seo.Sitemap)
	engine.GET("/robots.txt", handlers.Seo.Robots)
	// URL 别名短链（M4.1 插件通道）：/p/{alias} → 302 /posts/{id}
	engine.GET("/p/:alias", handlers.Seo.ResolveAlias)

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
	// ---------- 插件钩子：api.middleware（M3.9 同步拦截写请求；无插件订阅时空跑开销极小） ----------
	authed.Use(func(c *gin.Context) {
		// 仅拦截写请求（GET/HEAD 只读放行——内容渲染等读取场景不阻断）
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			c.Next()
			return
		}
		if handlers.Plugin != nil {
			if !handlers.Plugin.HookAPI(c) {
				c.Abort() // 插件拒绝：已由 HookAPI 写入响应
				return
			}
		}
		c.Next()
	})
	authed.POST("/auth/logout", handlers.Auth.Logout) // 登出（撤销 refresh）
	authed.GET("/me", handlers.Auth.Me)               // 当前用户资料
	authed.PUT("/me/password", handlers.Auth.ChangePassword) // 修改密码（账号安全页）
	authed.POST("/me/deactivate", handlers.Auth.Deactivate)  // 注销账号（需求 3.9，永久删除）
	// 插件 iframe 沙箱短期令牌（M3.6 后置：1 小时，插件直接调用代理 API）
	authed.POST("/plugin-sandbox-token", handlers.Auth.SandboxToken)

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
	authed.POST("/media", middleware.RequireNotRestricted(), handlers.Media.Upload) // 媒体上传（multipart；M5：受限访客 403）

	// ---------- 音乐解析（M7：QQ 音乐 songmid→songid，发帖内嵌播放器用） ----------
	authed.GET("/music/qq-resolve", handlers.Music.ResolveQQ)
	// 通用音乐源桥接（E7 可插拔：provider 由清单 music_provider 声明动态发现；公开）
	api.GET("/music/:provider/url", handlers.Music.ProviderURL)
	api.GET("/music/:provider/bgm", handlers.Music.ProviderBGM)
	// 网易云播放地址公开代理（M7 插件：访客无需登录即可播放，挂公开组）
	api.GET("/music/netease-url", handlers.Music.NeteaseURL)
	// QQ 音乐播放地址公开代理（M8 插件：访客无需登录即可播放，挂公开组）
	api.GET("/music/qq-url", handlers.Music.QqURL)
	api.GET("/music/qq-bgm", handlers.Music.QqBGM) // 首页背景音乐（公开：开关+歌单歌曲）

	// ---------- B站视频公开桥接（bilibili-video 插件：游客播放/扫码通道） ----------
	// 插件代理 API 需登录，而帖内视频要求匿名可播——公开组 + System 身份直达插件
	api.POST("/video/bilibili/resolve", handlers.Video.Resolve)             // 地址解析（清晰度档位）
	api.POST("/video/bilibili/url", handlers.Video.URL)                     // 播放地址（guest_token 优先）
	api.POST("/video/bilibili/qr-init", handlers.Video.QrInit)              // 游客扫码初始化
	api.POST("/video/bilibili/guest-qr-check", handlers.Video.GuestQrCheck) // 游客扫码轮询（签发 guest_token）
	api.POST("/video/bilibili/guest-status", handlers.Video.GuestStatus)    // 游客 token 有效性
	api.GET("/video/bilibili/stream", handlers.Video.Stream)               // 视频流代理（同源加载 + Range 透传）

	// ---------- 精品导航公开桥接（nav-links 插件：前台访客数据通道） ----------
	// 插件代理 API 需登录，而前台导航页要求访客可浏览——公开组 + System 身份直达插件
	api.GET("/nav/links", handlers.Nav.PublicLinks) // 导航页数据（含内嵌图标；30s 浏览器缓存；仅开放条目）
	// 私有导航门禁（v1.3.14）：meta/unlock 公开；数据端点挂 OptionalAuth 识别管理员登录态
	api.GET("/nav/private/meta", handlers.Nav.PrivateMeta)                                   // 门禁元数据（模式/密码已设/文案/条数）
	api.POST("/nav/private/unlock", handlers.Nav.PrivateUnlock)                              // 密码解锁（7 天 token）
	api.GET("/nav/private/links", middleware.OptionalAuth(jwtMgr), handlers.Nav.PrivateLinks) // 私有数据（管理员或 token 放行；no-store）
	api.GET("/video/bilibili/image", handlers.Video.Image)                 // 图床代理（封面/头像防盗链）

	// ---------- TTS 朗读公开桥接（tts-reader 插件：访客朗读通道） ----------
	// 插件代理 API 需登录，而朗读要求匿名可听——公开组 + System 身份直达插件；
	// POST + JSON body（宿主代理丢弃 query，SDK 端点精确匹配，id 经 body 传递）
	api.POST("/tts", handlers.TTS.Synthesize)       // 合成（{text, voice?, rate?}）→ {id}
	api.POST("/tts/audio", handlers.TTS.Audio)      // 取音频（{id}）→ audio/mpeg 字节

	// ---------- 站点统计公开桥接（stats-pro 插件：访客浏览上报） ----------
	// 上报以 System 身份直达插件（访客无需登录）；查看统计走插件代理 API（登录态）
	api.POST("/stats/hit", handlers.Stats.Hit) // 浏览上报（{post_id?, visitor_id?}）→ {counted}

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

	// ---------- 大世界（中继站聚合流，公开：读本地缓存） ----------
	api.GET("/relay/status", handlers.Relay.WorldStatus)
	api.GET("/relay/contents", handlers.Relay.ListWorld)

	// ---------- 自定义页面（公开：仅已发布页面，草稿视同不存在） ----------
	api.GET("/pages/:slug", handlers.Page.GetBySlug)

	// ---------- 开放接口网关（外部应用凭 X-Api-Key 调用；复用公开 handler，匿名视角） ----------
	// 鉴权：组级 ApiKeyAuth 中间件按「Method + 路由模板」反查目录得到接口标识，
	//       校验 Key 绑定的 endpoints 包含该标识后放行（未授权 403 / 无效或过期 401）。
	//       插件声明的开放端点（泛化通配路由）按「Method + 实际路径」二级索引匹配。
	// 变更约束：增删路由须同步 model.OpenAPICatalog() 目录数据（插件条目除外——随插件清单自动登记）。
	openGroup := api.Group("/open")
	openGroup.Use(middleware.ApiKeyAuth(handlers.OpenAPIKeys, handlers.PluginOpenCatalog.RouteIndex))
	openGroup.GET("/me", handlers.OpenAPI.Me)                          // 我的资料（me.profile，凭 Key 返回绑定用户）
	openGroup.POST("/posts", handlers.OpenAPI.CreatePost)              // 发布文章（posts.create，凭 Key 绑定用户发帖）
	openGroup.GET("/posts", handlers.Post.List)                        // 帖子列表（posts.list）
	openGroup.GET("/posts/:id", handlers.Post.Get)                     // 帖子详情（posts.detail）
	openGroup.GET("/posts/:id/comments", handlers.Comment.List)        // 帖子评论（posts.comments）
	openGroup.GET("/topics", handlers.Social.ListTopics)               // 话题列表（topics.list）
	openGroup.GET("/topics/:name/posts", handlers.Social.ListTopicPosts) // 话题帖子（topics.posts）
	openGroup.GET("/search", handlers.Social.Search)                   // 搜索（search）
	openGroup.GET("/users/:id", handlers.User.GetUser)                 // 用户主页（users.profile）
	openGroup.GET("/users/:id/posts", handlers.Social.UserPosts)       // 用户帖子（users.posts）
	openGroup.GET("/pages/:slug", handlers.Page.GetBySlug)             // 自定义页面（pages.detail）
	openGroup.GET("/meta", handlers.Site.GetMeta)                      // 站点信息（site.meta）
	openGroup.GET("/ai/models", handlers.OpenAPI.GatewayAIModels)      // AI 模型列表（ai.models）
	openGroup.POST("/ai/chat", handlers.OpenAPI.GatewayAIChat)         // AI 对话（ai.chat）
	openGroup.POST("/ai/assist", handlers.OpenAPI.GatewayAIAssist)     // AI 辅助（ai.assist：扩写/润色/配图/配乐/识图）
	openGroup.POST("/ai/search", handlers.OpenAPI.GatewayAISearch)     // 联网搜索（ai.search：SearXNG 聚合检索）
	openGroup.POST("/media/transfer", handlers.OpenAPI.MediaTransfer)  // 图片转存（media.transfer：外链图落站点媒体库）
	openGroup.GET("/nav/links", handlers.Nav.OpenList)   // 导航列表（navlinks.list：精品导航插件收藏站点同步）
	openGroup.POST("/nav/links", handlers.Nav.OpenSave)  // 导航同步写入（navlinks.save：插件书签批量同步到精品导航）
	openGroup.GET("/nav/private/links", handlers.Nav.OpenPrivateList)        // 私有导航数据（navlinks.private.list：私有条目同步）
	openGroup.GET("/nav/private/config", handlers.Nav.OpenPrivateConfigGet)  // 私有访问设置读取（navlinks.private.config）
	openGroup.POST("/nav/private/config", handlers.Nav.OpenPrivateConfigSave) // 私有访问设置写入（navlinks.private.save：模式/密码/文案）
	openGroup.POST("/media", handlers.OpenAPI.MediaUpload)             // 媒体上传（media.upload：本地图凭 Key 落站点媒体库）
	openGroup.POST("/ai/chat/stream", handlers.OpenAPI.GatewayAIChatStream) // AI 流式对话（ai.chat.stream：SSE 透传）
	// 插件声明式开放端点泛化转发（open_endpoints 声明的接口经此直达插件进程；
	// 白名单精确匹配 + System 身份——插件发版即可上新开放接口，主程序免发版）
	openGroup.Any("/plugins/:id/*path", handlers.PluginOpenGateway.Gateway)

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

		// 站点更新（设置域：版本检查/触发更新/进度轮询，后台左下角更新徽标）
		upd := adminGroup.Group("/update", perm(casbin.DomainSettings))
		upd.GET("/check", handlers.Update.Check)
		upd.POST("/start", handlers.Update.Start)
		upd.GET("/status", handlers.Update.Status)

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
	// 自定义页面域（后台创建独立页面 + 头部导航自定义数据源）
	pages := adminGroup.Group("/pages", perm(casbin.DomainPages))
	pages.GET("", handlers.Page.AdminList)
	pages.GET("/:id", handlers.Page.AdminGet)     // 编辑回显（含正文）
	pages.POST("", handlers.Page.AdminCreate)     // 创建
	pages.PUT("/:id", handlers.Page.AdminUpdate)  // 更新
		pages.DELETE("/:id", handlers.Page.AdminDelete) // 删除
		// 接口开放域（外部 API Key 管理：目录 + 凭证增删查）
		openAPI := adminGroup.Group("/open-api", perm(casbin.DomainOpenAPI))
		openAPI.GET("/catalog", handlers.OpenAPI.Catalog)   // 开放接口目录（页面多选数据源）
		openAPI.GET("/keys", handlers.OpenAPI.ListKeys)     // 凭证列表
		openAPI.POST("/keys", handlers.OpenAPI.CreateKey)   // 生成凭证（多选接口 + 过期天数）
		openAPI.DELETE("/keys/:id", handlers.OpenAPI.DeleteKey) // 删除凭证
		openAPI.PUT("/keys/:id/endpoints", handlers.OpenAPI.UpdateKeyEndpoints) // 权限设置（增/减授权接口）
		// 站点设置域
		settings := adminGroup.Group("/settings", perm(casbin.DomainSettings))
		settings.GET("", handlers.Admin.GetSettings)
		settings.PUT("", handlers.Admin.SaveSettings)
		// 中继站对接域（大世界）：配置回显 / 自助申请 / 连接测试 / 保存即重启订阅
		relay := adminGroup.Group("/relay", perm(casbin.DomainRelay))
		relay.GET("", handlers.Relay.GetConfig)
		relay.POST("/apply", handlers.Relay.Apply)
		relay.GET("/claim", handlers.Relay.Claim)
		relay.POST("/test", handlers.Relay.TestConnection)
		relay.PUT("", handlers.Relay.SaveConfig)
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
		plugins.GET("/market/:id/readme", handlers.Plugin.Readme) // 插件介绍 README（M5 文件夹结构；?source= 自定义仓库）
		plugins.GET("", handlers.Plugin.ListInstalled)      // 我的插件
		plugins.POST("/install", handlers.Plugin.Install)   // 安装 {plugin_id}
		plugins.POST("/upload", handlers.Plugin.Upload)     // 本地上传 .bpk 安装（?upgrade=1 升级）
		plugins.PUT("/:id/state", handlers.Plugin.SetState) // 启用/禁用
		plugins.DELETE("/:id", handlers.Plugin.Uninstall)   // 卸载
		plugins.POST("/:id/upgrade", handlers.Plugin.Upgrade) // 一键升级（M3.6 后置）
		plugins.GET("/:id/license", handlers.Plugin.LicenseStatus)    // 许可证状态（M3.5）
		plugins.POST("/:id/license", handlers.Plugin.ActivateLicense) // 激活许可证（M3.5）
		plugins.GET("/:id/backups/:file/download", handlers.Plugin.DownloadBackup) // 插件备份文件下载（流式直出）
		// 插件设置（M3.7：详情/配置读写；Gin 静态段优先，与 /market、/:id/* 子路径不冲突）
		plugins.GET("/:id", handlers.PluginConfig.Detail)             // 详情（设置页数据源）
		plugins.GET("/:id/config", handlers.PluginConfig.GetConfig)   // 读取配置
		plugins.PUT("/:id/config", handlers.PluginConfig.SaveConfig)  // 保存配置（推送即时生效）
		// 插件购买（M3.9 支付渠道：独立前缀避免与 /:id 参数段冲突）
		plugins.PUT("/issuer-key", handlers.PluginOrder.SetIssuerKey)               // 配置签发私钥
		plugins.POST("/:id/orders", handlers.PluginOrder.CreateOrder)               // 创建订单
		adminGroup.POST("/plugin-orders/:orderId/pay", perm(casbin.DomainPlugins), handlers.PluginOrder.PayOrder) // 支付签发
		// 可更新检查（独立前缀——/plugins/updates 与 /:id 参数段冲突）
		adminGroup.GET("/plugin-updates", perm(casbin.DomainPlugins), handlers.Plugin.Updates)
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
		ai.POST("/providers/fetch-models", handlers.Ai.FetchModels)
		ai.GET("/tasks", handlers.Ai.ListTasks)
		ai.PUT("/tasks/:name", handlers.Ai.UpdateTask)
		ai.POST("/tasks/:name/toggle", handlers.Ai.ToggleTask)
		ai.GET("/usage", handlers.Ai.UsageStats)
		ai.POST("/generate", handlers.Ai.Generate)             // 统一非流式生成
		ai.POST("/generate/stream", handlers.Ai.GenerateStream) // 统一流式生成（SSE）
		ai.POST("/embedding", handlers.Ai.Embedding)           // 统一向量嵌入
		ai.POST("/gen/summary", handlers.Ai.GenSummary)        // ?post_id= 生成摘要（seo_meta.summary）
		ai.POST("/gen/tags", handlers.Ai.GenTags)              // ?post_id= 生成标签建议
		ai.POST("/gen/reply", handlers.Ai.GenReply)            // ?post_id=&action= 智能回复助手
		ai.POST("/gen/seo-advice", handlers.Ai.GenSeoAdvice)   // ?post_id= SEO 建议
		ai.POST("/review/comments", handlers.Ai.ReviewComments) // 批量 AI 审核评论
		ai.POST("/assist", handlers.Ai.Assist)                 // 发帖 AI 辅助（扩写/润色/配图/配乐/识图）
		ai.GET("/search-config", handlers.Ai.GetSearchConfig)  // 联网搜索配置（SearXNG 地址）
		ai.PUT("/search-config", handlers.Ai.SaveSearchConfig) // 保存联网搜索配置
		ai.POST("/search-test", handlers.Ai.SearchTest)        // 联网搜索实测
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
