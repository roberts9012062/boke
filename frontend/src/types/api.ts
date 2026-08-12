// src/types/api.ts
// API 类型定义（与后端 DTO 同步——注释标注「与后端 dto 同步」）
//
// 统一响应结构（架构文档 11.2）：{ code, message, data, request_id }
// 说明：M1.1 阶段先定义骨架类型（meta/分页/主题），业务类型随里程碑扩充。
// 与后端 dto 同步。
export interface ApiResponse<T> {
  code: number; // 错误码（0 = 成功）
  message: string; // 提示文案
  data: T; // 业务数据
  request_id: string; // 请求 ID（贯穿日志）
}

// 站点元信息（GET /api/v1/meta）
// 与后端 dto 同步。
export interface SiteMeta {
  site_name: string; // 站点名称
  site_description: string; // 站点描述
  default_theme: ThemeName; // 服务端默认主题
  maintenance_mode: "on" | "off"; // 维护开关（M2：前端拦截判定）
}

// 分页数据结构（架构文档 11.4）
// 与后端 dto 同步。
export interface PageResult<T> {
  page: number; // 当前页码（从 1 起）
  page_size: number; // 每页条数
  total: number; // 总条数
  items: T[]; // 当前页数据
}

// ---------- 主题 ----------

// 主题名称（设计稿双主题：冷月 / 薄雾）
export type ThemeName = "cool-moon" | "mist";

// ---------- 用户（骨架） ----------

// 用户概要（列表/卡片场景）
// 与后端 dto 同步。
export interface UserSummary {
  id: number; // 用户 ID
  username: string; // 账号名
  nickname: string; // 昵称
  avatar_url: string; // 头像地址
  bio: string; // 个人简介
}

// ---------- 认证（M1.2） ----------

// 用户资料（GET /api/v1/me 与 GET /api/v1/users/:id）
// 与后端 dto 同步。
export interface UserProfile {
  id: number; // 用户 ID
  email: string; // 邮箱（本人完整值，他人脱敏）
  username: string; // 用户名（@账号）
  nickname: string; // 昵称
  avatar_url: string; // 头像地址
  bio: string; // 个人简介
  role: "superadmin" | "editor" | "author" | "visitor" | "restricted"; // 角色（M5 五级 RBAC）
  status: string; // 用户状态
  post_count: number; // 帖子数
  like_count: number; // 获赞数
  topic_count: number; // 话题数
  view_count: number; // 浏览数（帖子浏览量求和，设计稿主页统计）
  follower_count: number; // 粉丝数（M1.7）
  following_count: number; // 关注数（M1.7）
  created_at: string; // 注册时间（ISO8601）
}

// 令牌对（POST /api/v1/auth/register|login|refresh）
// 与后端 dto 同步。
export interface AuthTokens {
  access_token: string; // 访问令牌（15 分钟）
  refresh_token: string; // 刷新令牌（7 天）
  expires_in: number; // access 有效期（秒）
}

// 注册请求（昵称 + 邮箱 + 密码）
export interface RegisterReq {
  nickname: string; // 昵称（1-20 字符）
  email: string; // 邮箱
  password: string; // 密码（≥8 位含字母数字）
}

// 登录请求（邮箱或用户名 + 密码）
export interface LoginReq {
  account: string; // 邮箱或用户名
  password: string; // 密码
}

// ---------- 帖子（M1.3） ----------

// 帖子内容类型（设计稿四形态；video M2 启用）
export type PostContentType = "text" | "image" | "audio" | "video";

// 标签信息
export interface TagDTO {
  name: string; // 标签名（含 # 前缀）
  slug: string; // URL 别名
}

// 作者信息
export interface AuthorDTO {
  id: number; // 用户 ID
  username: string; // 账号名
  nickname: string; // 昵称
  avatar_url: string; // 头像地址
}

// 媒体信息
export interface MediaDTO {
  id: number; // 媒体 ID
  type: "image" | "audio" | "video"; // 类型（video M2）
  url: string; // 访问地址
  mime_type: string; // MIME 类型
  size_bytes: number; // 文件大小
  width: number; // 宽（图片/视频）
  height: number; // 高（图片/视频）
}

// 帖子摘要（时间线列表项）
// 与后端 dto 同步。
export interface PostSummary {
  id: number; // 帖子 ID
  title: string; // 标题
  summary: string; // 摘要
  content_type: PostContentType; // 类型
  visibility: "public" | "followers" | "private"; // 可见性（followers=仅关注者 M2）
  author: AuthorDTO; // 作者
  tags: TagDTO[]; // 标签
  media: MediaDTO[]; // 媒体预览
  like_count: number; // 点赞数
  comment_count: number; // 评论数
  view_count: number; // 浏览量
  favorite_count: number; // 收藏数（M1.7：列表聚合）
  favorited_at?: string; // 收藏时间（仅「我的收藏」列表返回，M1.7）
  published_at: string; // 发布时间（ISO8601）
}

// 后台内容管理帖子（PostSummary + 状态）
// 与后端 dto 同步。
export interface AdminPost extends PostSummary {
  status: string; // draft/published/taken_down/deleted
  updated_at: string; // 更新时间
}

// 后台编辑帖子详情（设计稿《后台编辑·文字/图片/音频/视频》四画板）
// 与后端 dto 同步。
export interface AdminPostDetail {
  id: number; // 帖子 ID
  title: string; // 标题
  content: string; // 正文（文字帖/图说/说明）
  content_type: PostContentType; // 内容类型
  status: string; // draft/published/taken_down
  visibility: "public" | "followers" | "private"; // 可见性
  cover_url: string; // 封面图（视频帖独立封面）
  tags: string[]; // 标签名（不带 #）
  media: MediaDTO[]; // 媒体列表
  view_count: number; // 浏览量（互动数据·览）
  like_count: number; // 点赞数（互动数据·赞）
  comment_count: number; // 评论数（互动数据·评）
  author: AuthorDTO; // 作者（发布信息）
  created_at: string; // 创建时间（发布信息·创建）
  updated_at: string; // 更新时间（发布信息·更新）
  published_at: string; // 发布时间（空串=未发布）
}

// 帖子详情（详情页）
// 与后端 dto 同步。
export interface PostDetail extends PostSummary {
  content: string; // 完整正文
  is_author: boolean; // 是否作者本人
  can_view: boolean; // 是否有权查看
  seo?: PostSeoOutput; // SEO 输出（M4.1 插件通道：robots 收录策略等）
}

// 发帖/存草稿请求
export interface CreatePostReq {
  content_type: PostContentType; // 类型
  title?: string; // 标题（可选）
  content: string; // 正文（≤2000 字）
  tags: string[]; // 标签
  media_ids: number[]; // 媒体 ID
  visibility: "public" | "followers" | "private"; // 可见性（followers=仅关注者 M2）
  status: "draft" | "published"; // draft 草稿 / published 发布
  seo?: PostSeoInput; // SEO 输入（M4.1 插件通道：发帖 SEO 面板提交）
}

// PostSeoInput 发帖/编辑时 SEO 输入（插件面板渲染，值随请求提交）。
export interface PostSeoInput {
  seo_title: string; // SEO 标题（默认用正文摘要）
  seo_description: string; // SEO 描述
  url_alias: string; // URL 别名（/p/{alias} 短链）
  robots: string; // 收录策略（index,follow 等；空=跟随全局）
}

// PostSeoOutput 详情页 SEO 输出（robots 收录策略/自定义标题描述）。
export interface PostSeoOutput {
  title: string; // SEO 标题（空=用默认）
  description: string; // SEO 描述
  url_alias: string; // URL 别名（编辑回填）
  robots: string; // 收录策略
}

// 媒体上传结果
export interface UploadResult {
  id: number; // 媒体 ID
  type: "image" | "audio" | "video"; // 类型
  url: string; // 访问地址
  mime_type: string; // MIME 类型
  size_bytes: number; // 文件大小
}

// ---------- 评论（M1.4） ----------

// 评论作者（登录用户；匿名为 null）
export interface CommentAuthor {
  id: number; // 用户 ID
  username: string; // 账号名
  nickname: string; // 昵称
}

// 评论（顶层含 replies 子回复，楼中楼）
// 与后端 dto 同步。
export interface CommentDTO {
  id: number; // 评论 ID
  content: string; // 评论内容
  author: CommentAuthor | null; // 作者（匿名为 null）
  guest_name: string; // 匿名昵称
  like_count: number; // 点赞数
  created_at: string; // 创建时间（ISO8601）
  is_author: boolean; // 是否本人（删除权限）
  liked: boolean; // 当前用户是否已赞
  reply_count: number; // 回复数
  replies: CommentDTO[]; // 子回复（楼中楼，仅顶层含）
}

// 后台评论管理条目（CommentDTO + 所属帖子/状态）
// 与后端 dto 同步。
export interface AdminComment extends CommentDTO {
  post_id: number; // 所属帖子 ID
  status: string; // visible/hidden/deleted
}

// 匿名身份（POST /guest-identity）
export interface GuestIdentity {
  guest_token: string; // 匿名 token
  guest_name: string; // 匿名昵称
}

// ---------- 互动（M1.4） ----------

// 帖子互动状态（GET /posts/:id/state）
// 与后端 dto 同步。
export interface PostReactionState {
  liked: boolean; // 当前用户是否已赞
  favorited: boolean; // 当前用户是否已收藏
  favorite_count: number; // 收藏数
}

// ---------- 话题/搜索/通知/关注（M1.5） ----------

// 话题信息（列表/详情）
// 与后端 dto 同步。
export interface TopicDTO {
  name: string; // 话题名（含 #）
  slug: string; // URL 别名
  description: string; // 描述
  post_count: number; // 帖子数
  follow_count: number; // 关注数
  browse_count: number; // 浏览数（帖子浏览量求和）
  following: boolean; // 当前用户是否已关注
}

// 通知条目
// 与后端 dto 同步。
export interface NotificationDTO {
  id: number; // 通知 ID
  type: "like" | "comment" | "reply" | "follow" | "system" | "message"; // 类型（message 私信 M2）
  title: string; // 动作文案（赞了你的帖子…）
  content: string; // 内容（帖子摘要）
  link: string; // 跳转链接
  post_id: number; // 相关帖子 ID
  read: boolean; // 是否已读
  created_at: string; // 创建时间
  actor: CommentAuthor | null; // 触发者（系统通知 null）
}

// 用户关系列表项（粉丝/关注）
// 与后端 dto 同步。
export interface UserRelationDTO {
  id: number; // 用户 ID
  username: string; // 账号名
  nickname: string; // 昵称
  avatar_url: string; // 头像
  bio: string; // 简介
  following: boolean; // 当前用户是否已关注
}

// 搜索结果（帖子/话题/用户分组）
// 与后端 dto 同步。
export interface SearchResult {
  posts: PostSummary[]; // 帖子结果
  total: number; // 帖子总数
  topics: TopicDTO[]; // 话题结果
  users: UserRelationDTO[]; // 用户结果
}

// ---------- 私信/消息（M2） ----------

// 会话对方信息
// 与后端 dto 同步。
export interface PeerDTO {
  id: number; // 用户 ID
  username: string; // 账号
  nickname: string; // 昵称
  avatar_url: string; // 头像
  online: boolean; // 在线状态（最后登录 5 分钟内，设计稿「· 在线」）
}

// 私信会话（列表项）
// 与后端 dto 同步。
export interface ConversationDTO {
  id: number; // 会话 ID
  peer: PeerDTO; // 对方信息
  last_message: string; // 最后消息摘要
  last_message_at: string; // 最后消息时间（ISO8601）
  unread: number; // 我的未读数（列表徽标）
}

// 私信消息（气泡）
// 与后端 dto 同步。
export interface MessageDTO {
  id: number; // 消息 ID
  sender_id: number; // 发送者
  content: string; // 内容
  created_at: string; // 发送时间（ISO8601）
  is_mine: boolean; // 是否我发的（气泡左右）
}
