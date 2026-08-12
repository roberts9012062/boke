// src/lib/api.ts
// API 请求封装：统一携带凭证、401 静默刷新一次、解析统一响应、错误码抛错
// （开发流程文档第 8 章约定）。
//
// 说明：
//   - 所有请求走相对路径 /api/v1/...（next.config.ts 已配置代理到后端 :8080）
//   - token 由 AuthProvider 通过 setTokenProvider 注入（避免循环依赖）
//   - 401 时自动用 refresh_token 静默刷新一次后重试原请求
//   - 调用方通过 try/catch 捕获 ApiError 展示错误提示
import type {
  AdminComment,
  AdminPost,
  AdminPostDetail,
  ApiResponse,
  AuthTokens,
  CommentDTO,
  ConversationDTO,
  CreatePostReq,
  GuestIdentity,
  MessageDTO,
  NotificationDTO,
  PageResult,
  PostDetail,
  PostReactionState,
  PostSummary,
  SearchResult,
  TopicDTO,
  UploadResult,
  UserProfile,
  UserRelationDTO,
} from "@/types/api";

// ApiError 请求失败异常（携带错误码与提示文案，前端 Toast 直接展示）。
export class ApiError extends Error {
  code: number; // 后端错误码（errs 段位）
  constructor(code: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.code = code;
  }
}

// 维护中错误码（后端 errs.CodeMaintenance=6004，M2 全站维护开关）。
const MAINTENANCE_CODE = 6004;

// ---------- 凭证存取（由 AuthProvider 注入实现） ----------

// TokenProvider 凭证存取接口（AuthProvider 实现，解耦 storage 细节）。
export interface TokenProvider {
  getAccessToken: () => string; // 读取 access token
  getRefreshToken: () => string; // 读取 refresh token
  refreshTokens: () => Promise<boolean>; // 静默刷新（成功返回 true）
}

// tokenProvider 当前凭证提供者（单例引用，AuthProvider 挂载时注入）。
let tokenProvider: TokenProvider | null = null;

// setTokenProvider 注入凭证提供者（AuthProvider 挂载时调用）。
export function setTokenProvider(provider: TokenProvider | null): void {
  tokenProvider = provider;
}

// ---------- 核心请求 ----------

// request 核心请求方法：携带凭证、解析统一响应、401 静默刷新一次。
// 返回：业务数据 data（成功时）。
async function request<T>(path: string, options: RequestInit = {}, retried = false): Promise<T> {
  // 拼接 API 前缀（相对路径，由 Next 代理转发）
  const url = path.startsWith("/api/") ? path : `/api/v1${path}`;

  // 统一请求头：JSON 内容 + Bearer 凭证
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers as Record<string, string> | undefined),
  };
  const accessToken = tokenProvider?.getAccessToken() ?? "";
  if (accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }

  const response = await fetch(url, { ...options, headers });

  // 401：未登录或 token 过期 → 静默刷新一次后重试
  if (response.status === 401 && !retried && tokenProvider) {
    const refreshed = await tokenProvider.refreshTokens();
    if (refreshed) {
      return request<T>(path, options, true);
    }
    // 刷新失败：抛出未登录错误（AuthProvider 同步清理状态）
    throw new ApiError(1001, "请先登录");
  }

  // 解析统一响应体（后端约定 {code,message,data,request_id}）
  const body = (await response.json()) as ApiResponse<T>;

  // 成功：返回业务数据
  if (body.code === 0) {
    return body.data;
  }

  // 维护中（M2）：跳转维护页（避免在维护页内再次跳转造成循环；
  // 后台页面不跳转——维护期间后台保持可用，仅前端页面重定向）
  if (
    body.code === MAINTENANCE_CODE &&
    typeof window !== "undefined" &&
    !window.location.pathname.startsWith("/maintenance") &&
    !window.location.pathname.startsWith("/admin")
  ) {
    window.location.href = "/maintenance";
  }

  // 失败：抛出 ApiError（携带错误码与后端提示文案）
  throw new ApiError(body.code, body.message);
}

// ---------- 导出便捷方法（函数式，无全局状态） ----------

// GET 请求。
export function get<T>(path: string): Promise<T> {
  return request<T>(path, { method: "GET" });
}

// POST 请求（JSON body）。
export function post<T>(path: string, data?: unknown): Promise<T> {
  return request<T>(path, {
    method: "POST",
    body: data === undefined ? undefined : JSON.stringify(data),
  });
}

// PUT 请求（JSON body）。
export function put<T>(path: string, data?: unknown): Promise<T> {
  return request<T>(path, {
    method: "PUT",
    body: data === undefined ? undefined : JSON.stringify(data),
  });
}

// DELETE 请求。
export function del<T>(path: string): Promise<T> {
  return request<T>(path, { method: "DELETE" });
}

// ---------- 认证便捷方法（M1.2） ----------

// 登录（返回令牌对，由 AuthProvider 持久化）。
export function apiLogin(account: string, password: string): Promise<AuthTokens> {
  return post<AuthTokens>("/auth/login", { account, password });
}

// 注册（注册即登录，返回令牌对）。
export function apiRegister(nickname: string, email: string, password: string): Promise<AuthTokens> {
  return post<AuthTokens>("/auth/register", { nickname, email, password });
}

// 刷新令牌对（静默刷新）。
export function apiRefresh(refreshToken: string): Promise<AuthTokens> {
  return post<AuthTokens>("/auth/refresh", { refresh_token: refreshToken });
}

// 登出（撤销 refresh token）。
export function apiLogout(refreshToken: string): Promise<void> {
  return post<void>("/auth/logout", { refresh_token: refreshToken });
}

// 请求密码重置（M2 找回密码；SMTP 未配置时后端降级日志输出链接）。
export function apiForgotPassword(email: string): Promise<void> {
  return post<void>("/auth/forgot-password", { email });
}

// 重置密码（token 从邮件链接 URL 带入）。
export function apiResetPassword(token: string, newPassword: string): Promise<void> {
  return post<void>("/auth/reset-password", { token, new_password: newPassword });
}

// 当前用户资料。
export function apiMe(): Promise<UserProfile> {
  return get<UserProfile>("/me");
}

// 修改密码（账号安全页：校验当前密码 → 更新，其他设备旧会话失效）。
export function apiChangePassword(currentPassword: string, newPassword: string): Promise<void> {
  return put<void>("/me/password", { current_password: currentPassword, new_password: newPassword });
}

// ---------- 帖子方法（M1.3） ----------

// 时间线列表（type 过滤：全部/图/音/影；分页）。
export function apiTimeline(
  params: { type?: string; page?: number; page_size?: number } = {},
): Promise<PageResult<PostSummary>> {
  const query = new URLSearchParams();
  if (params.type) query.set("type", params.type);
  query.set("page", String(params.page ?? 1));
  query.set("page_size", String(params.page_size ?? 20));
  return get<PageResult<PostSummary>>(`/posts?${query.toString()}`);
}

// 帖子详情。
// 帖子详情（P1 浏览埋点：匿名访客带 guest_token 用于访客区分与同日去重）。
export function apiPostDetail(postId: number, guestToken?: string): Promise<PostDetail> {
  const query = guestToken ? `?guest_token=${encodeURIComponent(guestToken)}` : "";
  return get<PostDetail>(`/posts/${postId}${query}`);
}

// 发帖/存草稿。
export function apiCreatePost(req: CreatePostReq): Promise<{ id: number }> {
  return post<{ id: number }>("/posts", req);
}

// 发布草稿。
// 更新帖子（走查纠偏：编辑已发布帖子 / 草稿继续编辑；后端 PUT /posts/:id，类型不可变）。
export function apiUpdatePost(
  postId: number,
  req: { title?: string; content?: string; tags?: string[]; media_ids?: number[]; visibility?: string },
): Promise<{ id: number }> {
  return put<{ id: number }>(`/posts/${postId}`, req);
}

export function apiPublishPost(postId: number): Promise<{ id: number }> {
  return post<{ id: number }>(`/posts/${postId}/publish`);
}

// 草稿箱。
export function apiDrafts(): Promise<PostSummary[]> {
  return get<PostSummary[]>("/me/drafts");
}

// 删除帖子。
export function apiDeletePost(postId: number): Promise<void> {
  return del<void>(`/posts/${postId}`);
}

// ---------- 媒体方法（M1.3） ----------

// 上传媒体（multipart；file 为选中的文件对象）。
export async function apiUploadMedia(file: File): Promise<UploadResult> {
  const form = new FormData();
  form.append("file", file);
  const url = "/api/v1/media";
  // 携带 Bearer 凭证（与 request 一致，但上传不能设 Content-Type 头）
  const headers: Record<string, string> = {};
  const accessToken = tokenProvider?.getAccessToken() ?? "";
  if (accessToken) {
    headers.Authorization = `Bearer ${accessToken}`;
  }
  const response = await fetch(url, { method: "POST", headers, body: form });
  const body = (await response.json()) as ApiResponse<UploadResult>;
  if (body.code === 0) {
    return body.data;
  }
  throw new ApiError(body.code, body.message);
}

// ---------- 评论方法（M1.4） ----------

// 评论列表（楼中楼结构）。
export function apiComments(postId: number): Promise<CommentDTO[]> {
  return get<CommentDTO[]>(`/posts/${postId}/comments`);
}

// 发表评论（登录自动带 token；访客传 guest_token）。
export function apiCreateComment(
  postId: number,
  content: string,
  guestToken: string,
): Promise<{ id: number }> {
  return post<{ id: number }>(`/posts/${postId}/comments`, { content, guest_token: guestToken });
}

// 回复评论（@用户名 前缀由调用方生成）。
export function apiReplyComment(
  commentId: number,
  content: string,
  guestToken: string,
): Promise<{ id: number }> {
  return post<{ id: number }>(`/comments/${commentId}/reply`, { content, guest_token: guestToken });
}

// 评论点赞。
export function apiLikeComment(commentId: number): Promise<{ like_count: number }> {
  return post<{ like_count: number }>(`/comments/${commentId}/like`);
}

// 删除评论。
export function apiDeleteComment(commentId: number): Promise<void> {
  return del<void>(`/comments/${commentId}`);
}

// ---------- 互动方法（M1.4） ----------

// 匿名身份签发（访客评论前调用）。
export function apiGuestIdentity(nickname: string): Promise<GuestIdentity> {
  return post<GuestIdentity>("/guest-identity", { nickname });
}

// 帖子互动状态。
export function apiPostState(postId: number): Promise<PostReactionState> {
  return get<PostReactionState>(`/posts/${postId}/state`);
}

// 帖子点赞 / 取消。
export function apiLikePost(postId: number): Promise<{ like_count: number }> {
  return post<{ like_count: number }>(`/posts/${postId}/like`);
}
export function apiUnlikePost(postId: number): Promise<{ like_count: number }> {
  return del<{ like_count: number }>(`/posts/${postId}/like`);
}

// 帖子收藏 / 取消。
export function apiFavoritePost(postId: number): Promise<{ favorite_count: number }> {
  return post<{ favorite_count: number }>(`/posts/${postId}/favorite`);
}
export function apiUnfavoritePost(postId: number): Promise<{ favorite_count: number }> {
  return del<{ favorite_count: number }>(`/posts/${postId}/favorite`);
}

// ---------- 话题/搜索/通知/关注（M1.5） ----------

// 话题列表。
export function apiTopics(): Promise<TopicDTO[]> {
  return get<TopicDTO[]>("/topics");
}

// 话题详情。
export function apiTopicDetail(name: string): Promise<TopicDTO> {
  return get<TopicDTO>(`/topics/${encodeURIComponent(name)}`);
}

// 话题帖子流（sort=latest 最新 / hot 热门）。
export function apiTopicPosts(name: string, sort = "latest", page = 1): Promise<PageResult<PostSummary>> {
  return get<PageResult<PostSummary>>(
    `/topics/${encodeURIComponent(name)}/posts?sort=${sort}&page=${page}&page_size=20`,
  );
}

// 关注/取消话题。
export function apiFollowTopic(name: string): Promise<void> {
  return post<void>(`/topics/${encodeURIComponent(name)}/follow`);
}
export function apiUnfollowTopic(name: string): Promise<void> {
  return del<void>(`/topics/${encodeURIComponent(name)}/follow`);
}

// 搜索（q 关键词）。
export function apiSearch(q: string): Promise<SearchResult> {
  return get<SearchResult>(`/search?q=${encodeURIComponent(q)}`);
}

// 通知列表（type 过滤）。
export function apiNotifications(type = ""): Promise<PageResult<NotificationDTO>> {
  const query = type ? `?type=${type}` : "";
  return get<PageResult<NotificationDTO>>(`/notifications${query}`);
}

// 通知未读数（角标轮询）。
export function apiUnreadCount(): Promise<{ unread: number }> {
  return get<{ unread: number }>("/notifications/unread-count");
}

// 全部已读。
export function apiMarkAllRead(): Promise<void> {
  return put<void>("/notifications/read-all");
}

// 单条已读。
export function apiMarkRead(id: number): Promise<void> {
  return put<void>(`/notifications/${id}/read`);
}

// 关注/取消用户。
export function apiFollowUser(userId: number): Promise<void> {
  return put<void>(`/users/${userId}/follow`);
}
export function apiUnfollowUser(userId: number): Promise<void> {
  return del<void>(`/users/${userId}/follow`);
}

// 粉丝/关注列表。
export function apiFollowers(userId: number): Promise<PageResult<UserRelationDTO>> {
  return get<PageResult<UserRelationDTO>>(`/users/${userId}/followers`);
}
export function apiFollowing(userId: number): Promise<PageResult<UserRelationDTO>> {
  return get<PageResult<UserRelationDTO>>(`/users/${userId}/following`);
}

// 我的收藏。
export function apiFavorites(): Promise<PageResult<PostSummary>> {
  return get<PageResult<PostSummary>>("/me/favorites");
}

// 用户赞过的帖子。
export function apiLikedPosts(userId: number): Promise<PageResult<PostSummary>> {
  return get<PageResult<PostSummary>>(`/users/${userId}/liked`);
}

// 用户主页帖子流（type 过滤：空=全部 / image / audio）。
export function apiUserPosts(userId: number, type = ""): Promise<PageResult<PostSummary>> {
  const query = type ? `?type=${type}` : "";
  return get<PageResult<PostSummary>>(`/users/${userId}/posts${query}`);
}

// 编辑资料。
export function apiUpdateProfile(nickname: string, bio: string): Promise<void> {
  return put<void>("/me/profile", { nickname, bio });
}

// 更新头像（M1.7：先上传媒体拿 url，再写入用户头像）。
export function apiUpdateAvatar(avatarUrl: string): Promise<{ avatar_url: string }> {
  return put<{ avatar_url: string }>("/me/avatar", { avatar_url: avatarUrl });
}

// 关注流（首页 feed=following）。
export function apiFollowingFeed(page = 1): Promise<PageResult<PostSummary>> {
  return get<PageResult<PostSummary>>(`/posts?feed=following&page=${page}&page_size=20`);
}

// ---------- 后台管理（M1.6） ----------

// 趋势点（近 7 日每日互动，后端 dto 同步）。
export interface TrendPoint {
  date: string; // 日期（MM-DD）
  posts: number; // 当日新帖
  likes: number; // 当日获赞
  comments: number; // 当日评论
}

// 仪表盘聚合数据（与后端 dto 同步）。
export interface DashboardData {
  views_7d: number; // 近 7 日浏览
  views_trend: number; // 环比 %
  likes_7d: number;
  likes_trend: number;
  comments_7d: number;
  comments_trend: number;
  posts_7d: number;
  posts_trend: number;
  type_counts: Record<string, number>; // 内容分布
  trend_series: TrendPoint[]; // 近 7 日互动趋势（M1.7 新增）
  activities: { kind: string; id: number; actor: string; content: string; created_at: string }[];
  pending: { comments: number; reports: number; sensitive: number }; // 待处理块（走查纠偏补）
}

// 仪表盘聚合。
export function apiDashboard(): Promise<DashboardData> {
  return get<DashboardData>("/admin/dashboard");
}

// 内容管理列表（type/status/q 筛选）。
export function apiAdminPosts(params: {
  type?: string;
  status?: string;
  q?: string;
  page?: number;
}): Promise<PageResult<AdminPost>> {
  const query = new URLSearchParams();
  if (params.type) query.set("type", params.type);
  if (params.status) query.set("status", params.status);
  if (params.q) query.set("q", params.q);
  query.set("page", String(params.page ?? 1));
  return get<PageResult<AdminPost>>(`/admin/posts?${query.toString()}`);
}

// 内容上下架。
export function apiAdminSetPostStatus(postId: number, status: string): Promise<void> {
  return put<void>(`/admin/posts/${postId}/status`, { status });
}

// 内容删除。
export function apiAdminDeletePost(postId: number): Promise<void> {
  return del<void>(`/admin/posts/${postId}`);
}

// 后台编辑详情（设计稿《后台编辑》四画板：编辑/发布信息/互动数据/操作）。
export function apiAdminPostDetail(postId: number): Promise<AdminPostDetail> {
  return get<AdminPostDetail>(`/admin/posts/${postId}`);
}

// 后台编辑保存（M2：status draft=保存草稿 / published=更新发布；cover_url 视频帖换封面）。
export function apiAdminUpdatePost(
  postId: number,
  req: {
    title: string;
    content: string;
    tags: string[];
    media_ids: number[];
    visibility: string;
    cover_url?: string;
    status: "draft" | "published";
  },
): Promise<{ id: number }> {
  return put<{ id: number }>(`/admin/posts/${postId}`, req);
}

// 评论管理列表。
export function apiAdminComments(params: { status?: string; q?: string; page?: number }): Promise<PageResult<AdminComment>> {
  const query = new URLSearchParams();
  if (params.status) query.set("status", params.status);
  if (params.q) query.set("q", params.q);
  query.set("page", String(params.page ?? 1));
  return get<PageResult<AdminComment>>(`/admin/comments?${query.toString()}`);
}

// 评论统计（设计稿后台评论统计条：全部/今日新增/已屏蔽）。
export function apiAdminCommentStats(): Promise<{ total: number; today: number; hidden: number }> {
  return get<{ total: number; today: number; hidden: number }>("/admin/comments/stats");
}

// 评论隐藏/恢复（M2：visible ↔ hidden，前台列表仅展示 visible）。
export function apiAdminSetCommentStatus(commentId: number, status: string): Promise<void> {
  return put<void>(`/admin/comments/${commentId}/status`, { status });
}

// 评论删除。
export function apiAdminDeleteComment(commentId: number): Promise<void> {
  return del<void>(`/admin/comments/${commentId}`);
}

// 用户管理列表。
export function apiAdminUsers(params: { q?: string; page?: number }): Promise<PageResult<UserProfile>> {
  const query = new URLSearchParams();
  if (params.q) query.set("q", params.q);
  query.set("page", String(params.page ?? 1));
  return get<PageResult<UserProfile>>(`/admin/users?${query.toString()}`);
}

// 用户封禁/解封（M2：封禁支持原因与期限，写 ban_records）。
export function apiAdminSetUserStatus(userId: number, status: string, reason?: string, until?: string): Promise<void> {
  return put<void>("/admin/users/" + userId + "/status", { status, reason, until });
}

// 角色调整（M2→M5：五级角色 superadmin/editor/author/visitor/restricted；
// 落库 + casbin 即时生效，需该用户重新登录）。
export function apiAdminSetUserRole(userId: number, role: string): Promise<void> {
  return put<void>(`/admin/users/${userId}/role`, { role });
}

// 角色矩阵行（M5，设计稿《后台角色》表格行）。
export interface RoleMatrixItem {
  role: string; // 角色标识
  count: number; // 人数
  permissions: string[]; // 权限域列表
  status: string; // enabled / restricted
  builtin: boolean; // 系统内置
}

// 角色矩阵（设计稿统计条 + 表格）。
export interface RoleMatrix {
  roles: RoleMatrixItem[];
  role_count: number; // 角色数
  total: number; // 全部用户数
}

// 角色矩阵查询。
export function apiAdminRoles(): Promise<RoleMatrix> {
  return get<RoleMatrix>("/admin/roles");
}

// 更新角色权限域（superadmin 不可编辑；settings 持久化 + 即时生效）。
export function apiUpdateRolePermissions(role: string, permissions: string[]): Promise<void> {
  return put<void>(`/admin/roles/${role}/permissions`, { permissions });
}

// 站点设置读取/保存。
export function apiAdminSettings(): Promise<Record<string, string>> {
  return get<Record<string, string>>("/admin/settings");
}
export function apiAdminSaveSettings(updates: Record<string, string>): Promise<void> {
  return put<void>("/admin/settings", updates);
}

// ---------- 媒体库 / 标签分类（M2.9） ----------

// 后台媒体行（设计稿：文件/类型/大小/引用/上传/操作）。
export interface AdminMediaItem {
  id: number; // 媒体 ID
  type: string; // image/audio/video
  url: string; // 访问地址
  mime_type: string; // MIME
  size_bytes: number; // 大小
  width: number; // 宽
  height: number; // 高
  file_name: string; // 文件名
  ref_count: number; // 引用数
  created_at: string; // 上传时间
}

// 媒体统计条（设计稿：全部文件/图片/音频/视频）。
export function apiAdminMediaStats(): Promise<{ total: number; image: number; audio: number; video: number }> {
  return get<{ total: number; image: number; audio: number; video: number }>("/admin/media/stats");
}

// 媒体列表（type/q 筛选）。
export function apiAdminMedia(params: { type?: string; q?: string; page?: number }): Promise<PageResult<AdminMediaItem>> {
  const query = new URLSearchParams();
  if (params.type) query.set("type", params.type);
  if (params.q) query.set("q", params.q);
  query.set("page", String(params.page ?? 1));
  return get<PageResult<AdminMediaItem>>(`/admin/media?${query.toString()}`);
}

// 删除媒体（解除帖子引用 + 删记录 + 删磁盘文件）。
export function apiAdminDeleteMedia(mediaId: number): Promise<void> {
  return del<void>(`/admin/media/${mediaId}`);
}

// 后台标签行（设计稿：标签/分类/文章/热度/更新/操作）。
export interface AdminTagItem {
  id: number; // 标签 ID
  name: string; // 标签名
  slug: string; // URL 别名
  description: string; // 描述
  category: string; // 分类（情绪/栏目/体裁/临时）
  post_count: number; // 文章数
  created_at: string; // 创建时间
}

// 标签统计条（设计稿：全部/热门/本周新建/未使用）。
export function apiAdminTagStats(): Promise<{ total: number; hot: number; week_new: number; unused: number }> {
  return get<{ total: number; hot: number; week_new: number; unused: number }>("/admin/tags/stats");
}

// 标签列表（q 搜索）。
export function apiAdminTags(params: { q?: string; page?: number }): Promise<PageResult<AdminTagItem>> {
  const query = new URLSearchParams();
  if (params.q) query.set("q", params.q);
  query.set("page", String(params.page ?? 1));
  return get<PageResult<AdminTagItem>>(`/admin/tags?${query.toString()}`);
}

// 重命名标签（name + slug + category 同步）。
export function apiAdminRenameTag(tagId: number, name: string, slug: string, category: string): Promise<void> {
  return put<void>(`/admin/tags/${tagId}`, { name, slug, category });
}

// 合并标签（src → target，src 删除、帖子关联转移）。
export function apiAdminMergeTag(tagId: number, targetId: number): Promise<void> {
  return post<void>(`/admin/tags/${tagId}/merge`, { target_id: targetId });
}

// 删除标签（解除帖子关联）。
export function apiAdminDeleteTag(tagId: number): Promise<void> {
  return del<void>(`/admin/tags/${tagId}`);
}

// ---------- 插件商城/管理（M3.1，GitHub 仓库清单驱动） ----------

// 插件侧栏入口声明（前端数据驱动扩展）。
export interface PluginNav {
  href: string; // 后台路径
  label: string; // 菜单名
  icon: string; // 图标 key（nav-icons 注册表）
}

// 插件设置项（schema 驱动通用设置页）。
export interface PluginSettingField {
  key: string; // 设置键（存 settings：plugin_{id}_{key}）
  label: string; // 标签
  type: string; // text / switch / select
  default: string; // 默认值
  options?: string[]; // select 选项
}

// 插件信息（清单项，与后端 dto 同步，含兼容性契约字段）。
export interface PluginInfo {
  id: string; // 插件 ID
  name: string; // 名称
  version: string; // 版本
  category: string; // 类别：seo/security/performance/analytics/writing/ops/enhancement
  price: number; // 价格（0=免费）
  installs: number; // 安装量
  official: boolean; // 官方标签
  description: string; // 描述
  capabilities: string[]; // 能力清单
  repo_url: string; // 来源仓库
  core_version?: string; // 兼容核心版本（如 >=0.1.0）
  requires?: string[]; // 依赖插件
  conflicts?: string[]; // 冲突插件
  nav?: PluginNav; // 侧栏入口声明
  settings_schema?: PluginSettingField[]; // 设置项 schema
}

// 商城插件（清单 + 已安装状态）。
export interface MarketPlugin extends PluginInfo {
  installed: boolean; // 是否已安装
  state: string; // 已安装状态
  instance_id: number; // 实例 ID
}

// 已安装插件（我的插件页）。
export interface InstalledPlugin {
  id: number; // 实例 ID
  plugin_id: string; // 插件 ID
  name: string; // 名称
  version: string; // 版本
  repo_url: string; // 来源
  state: string; // running/disabled/installed/crashed（M3.3 进程外）
  last_error?: string; // 最近错误（M3.3 崩溃/缺失提示）
  created_at: string; // 安装时间
  nav?: PluginNav; // 侧栏入口声明（动态扩展）
  settings_schema?: PluginSettingField[]; // 设置项 schema
}

// 插件商城清单（source 为空 = settings 默认源；返回清单 + 插件列表）。
export function apiPluginMarket(source = ""): Promise<{
  source: string;
  name: string;
  description: string;
  items: MarketPlugin[];
}> {
  const query = source ? `?source=${encodeURIComponent(source)}` : "";
  return get<{ source: string; name: string; description: string; items: MarketPlugin[] }>(
    `/admin/plugins/market${query}`,
  );
}

// 我的插件列表。
export function apiInstalledPlugins(): Promise<{ items: InstalledPlugin[] }> {
  return get<{ items: InstalledPlugin[] }>("/admin/plugins");
}

// 安装插件（从清单拉取信息落库）。
export function apiInstallPlugin(pluginId: string): Promise<void> {
  return post<void>("/admin/plugins/install", { plugin_id: pluginId });
}

// 启用/禁用插件（running / disabled）。
export function apiSetPluginState(instanceId: number, state: string): Promise<void> {
  return put<void>(`/admin/plugins/${instanceId}/state`, { state });
}

// 卸载插件。
export function apiUninstallPlugin(instanceId: number): Promise<void> {
  return del<void>(`/admin/plugins/${instanceId}`);
}

// ---------- SEO（M4） ----------

// 全局 SEO 设置（seo_settings 单行）。
export interface SeoSettings {
  site_name: string; // 站点名称
  site_description: string; // 默认描述
  title_suffix: string; // 标题后缀
  keywords: string; // 默认关键词
  og_title: string; // 默认 OG 标题
  robots_txt: string; // robots.txt
  sitemap_enabled: boolean; // sitemap 开关
}

// 帖子 SEO 元数据（seo_meta）。
export interface SeoMeta {
  post_id: number;
  title: string;
  description: string;
  keywords: string;
  canonical_url: string;
  og_image: string;
  summary: string;
}

// 健康问题项。
export interface SeoHealthIssue {
  code: string;
  message: string;
}

// 健康度汇总（设计稿：四卡片 + 近 7 日趋势 + 问题类型分布 + 优先修复）。
export interface SeoHealthSummary {
  total_posts: number; // 已扫描帖子
  pending_issues: number; // 待修复问题
  avg_score: number; // 综合评分（0-100）
  meta_coverage: number; // 元信息覆盖 %
  indexable: number; // 可收录页面
  noindex: number; // noindex 页面
  trend: { date: string; score: number }[]; // 近 7 日健康分趋势
  distribution: { code: string; label: string; count: number; percent: number }[]; // 问题类型分布
  priorities: { level: string; message: string; hint: string; where: string }[]; // 优先修复
  items: { post_id: number; post_title: string; score: number; issues: SeoHealthIssue[]; checked_at: string }[];
}

// SERP 预览数据。
export interface SerpPreview {
  title: string;
  title_len: number;
  url: string;
  description: string;
  checks: string[];
  warnings: string[];
}

export function apiSeoSettings(): Promise<SeoSettings> {
  return get<SeoSettings>("/admin/seo/settings");
}
export function apiSaveSeoSettings(updates: SeoSettings): Promise<void> {
  return put<void>("/admin/seo/settings", updates);
}
export function apiSeoMeta(postId: number): Promise<SeoMeta> {
  return get<SeoMeta>(`/admin/seo/meta/${postId}`);
}
export function apiSaveSeoMeta(postId: number, meta: Partial<SeoMeta>): Promise<void> {
  return put<void>(`/admin/seo/meta/${postId}`, meta);
}
export function apiSeoHealth(): Promise<SeoHealthSummary> {
  return get<SeoHealthSummary>("/admin/seo/health");
}
export function apiSeoScan(): Promise<SeoHealthSummary> {
  return post<SeoHealthSummary>("/admin/seo/health/scan");
}
export function apiSeoBatchFix(): Promise<{ fixed: number }> {
  return post<{ fixed: number }>("/admin/seo/batch-fix");
}
export function apiSerpPreview(postId: number): Promise<SerpPreview> {
  return get<SerpPreview>(`/admin/seo/serp-preview?post_id=${postId}`);
}

// ---------- 私信/消息（M2） ----------

// 会话列表（filter=all|unread）。
export function apiConversations(filter = ""): Promise<PageResult<ConversationDTO>> {
  const query = filter ? `?filter=${filter}` : "";
  return get<PageResult<ConversationDTO>>(`/conversations${query}`);
}

// 发起/打开会话（对方主页「私信」按钮进入）。
export function apiOpenConversation(userId: number): Promise<ConversationDTO> {
  return post<ConversationDTO>("/conversations", { user_id: userId });
}

// 会话消息列表（打开即已读）。
export function apiMessages(conversationId: number, page = 1): Promise<PageResult<MessageDTO>> {
  return get<PageResult<MessageDTO>>(`/conversations/${conversationId}/messages?page=${page}&page_size=30`);
}

// 发送消息。
export function apiSendMessage(conversationId: number, content: string): Promise<MessageDTO> {
  return post<MessageDTO>(`/conversations/${conversationId}/messages`, { content });
}

// 全部会话未读总数（消息中心角标）。
export function apiMessageUnread(): Promise<{ unread: number }> {
  return get<{ unread: number }>("/conversations/unread-count");
}

// ---------- 内容治理（M2） ----------

// 提交举报（帖子/评论/用户；6 预置原因）。
export function apiSubmitReport(
  targetType: "post" | "comment" | "user",
  targetId: number,
  reason: string,
  detail: string,
): Promise<void> {
  return post<void>("/reports", { target_type: targetType, target_id: targetId, reason, detail });
}

// 举报工单（后台审核队列）。
export interface ReportDTO {
  id: number; // 工单 ID
  reporter: string; // 举报人昵称
  target_type: string; // 对象类型
  target_id: number; // 对象 ID
  target_brief: string; // 目标摘要
  reason: string; // 原因
  detail: string; // 补充说明
  status: string; // pending/resolved/rejected
  source: string; // 来源：user 人工举报 / ai AI 审核标记（M4）
  created_at: string; // 提交时间
  cost_seconds?: number; // 处理耗时（秒，已处理工单；P1 审核耗时）
}

// 审核队列统计（设计稿统计条：待处理/高风险/今日已审/平均耗时；M4 补高风险）。
export function apiAdminReportStats(): Promise<{
  pending: number;
  high_risk: number;
  resolved_today: number;
  avg_cost_seconds: number;
}> {
  return get<{ pending: number; high_risk: number; resolved_today: number; avg_cost_seconds: number }>(
    "/admin/reports/stats",
  );
}

// 后台工单列表。
export function apiAdminReports(params: { status?: string; page?: number }): Promise<PageResult<ReportDTO>> {
  const query = new URLSearchParams();
  if (params.status) query.set("status", params.status);
  query.set("page", String(params.page ?? 1));
  return get<PageResult<ReportDTO>>(`/admin/reports?${query.toString()}`);
}

// 处理工单（resolved/rejected）。
export function apiAdminSetReportStatus(reportId: number, status: string): Promise<void> {
  return put<void>(`/admin/reports/${reportId}/status`, { status });
}

// 复核 AI 标记工单（M4：action=allow 放行 / delete 删除，仅 AI 来源工单）。
export function apiAdminVerdictReport(reportId: number, action: "allow" | "delete"): Promise<void> {
  return post<void>(`/admin/reports/${reportId}/verdict`, { action });
}

// 敏感词（后台管理）。
export interface SensitiveWordDTO {
  id: number; // 词 ID
  word: string; // 词内容
  level: string; // forbidden/review
  hit_count: number; // 命中次数（P1 命中统计）
  created_at: string; // 添加时间
}

// 敏感词统计（设计稿统计条：全部/拦截/审核）。
export function apiAdminSensitiveStats(): Promise<{ total: number; forbidden: number; review: number }> {
  return get<{ total: number; forbidden: number; review: number }>("/admin/sensitive-words/stats");
}

export function apiAdminSensitiveWords(params: { q?: string; page?: number }): Promise<PageResult<SensitiveWordDTO>> {
  const query = new URLSearchParams();
  if (params.q) query.set("q", params.q);
  query.set("page", String(params.page ?? 1));
  return get<PageResult<SensitiveWordDTO>>(`/admin/sensitive-words?${query.toString()}`);
}

export function apiAdminAddSensitiveWord(word: string, level: string): Promise<void> {
  return post<void>("/admin/sensitive-words", { word, level });
}

// 批量添加敏感词（后台站点设置「敏感词（逗号分隔）」入口，forbidden 级别）。
export function apiAdminAddSensitiveWords(
  words: string[],
): Promise<{ added: number; skipped: number }> {
  return post<{ added: number; skipped: number }>("/admin/sensitive-words/batch", { words });
}

export function apiAdminDeleteSensitiveWord(word: string): Promise<void> {
  return del<void>(`/admin/sensitive-words/${encodeURIComponent(word)}`);
}

// 封禁记录（后台管理）。
export interface BanRecordDTO {
  id: number; // 记录 ID
  user_id: number; // 被封禁用户
  nickname: string; // 用户昵称
  reason: string; // 原因
  until: string | null; // 解封时间（null=永久）
  created_by: number; // 操作者
  created_at: string; // 封禁时间
}

// 用户统计（设计稿《后台用户》统计条：全部/本周新增/活跃/已禁言）。
export function apiAdminUserStats(): Promise<{
  total: number;
  week_new: number;
  active: number;
  banned: number;
}> {
  return get<{ total: number; week_new: number; active: number; banned: number }>(
    "/admin/users/stats",
  );
}

export function apiAdminBans(params: { page?: number }): Promise<PageResult<BanRecordDTO>> {
  const query = new URLSearchParams();
  query.set("page", String(params.page ?? 1));
  return get<PageResult<BanRecordDTO>>(`/admin/bans?${query.toString()}`);
}
