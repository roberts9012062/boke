// internal/service/auth.go
// 认证业务逻辑：注册 / 登录 / 登出 / 刷新（事务边界与业务规则归属处）。
//
// 规则（需求 3.1 + 架构文档 9.3）：
//   - 注册：邮箱唯一、用户名自动生成（邮箱前缀 + 随机后缀）、密码 bcrypt 哈希
//   - 登录：邮箱或用户名 + 密码；失败提示「邮箱或密码不正确」；登录限流 5 次/分/账号
//   - 登出：refresh token 加入黑名单（Redis）
//   - 刷新：refresh token 校验 + 黑名单检查后签发新令牌对
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/roberts9012062/boke/internal/auth"
	"github.com/roberts9012062/boke/internal/casbin"
	"github.com/roberts9012062/boke/internal/mail"
	"github.com/roberts9012062/boke/internal/media"
	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/redis"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 用户状态常量（对应 users.status）。
const (
	userStatusActive = "active"
	userStatusBanned = "banned"
)

// AuthService 认证服务（连接器类，聚合所需依赖）。
type AuthService struct {
	users       *repository.UserRepo
	audit       *repository.AuditRepo
	settings    *repository.SettingRepo // 站点设置（注册开关 allow_register，站点设置页即时生效）
	jwt         *auth.Manager
	enforcer    *casbin.Enforcer
	limiter     *redis.RateLimiter
	resets      *auth.ResetManager // 密码重置令牌（M2 找回密码）
	mailer      *mail.Sender       // 邮件发送（M2；未配置 SMTP 降级日志）
	media       *media.Store       // 媒体存储（注销账号时清理物理文件）
	siteBaseURL string             // 站点地址（生成重置链接）
}

// NewAuthService 创建认证服务。
func NewAuthService(
	users *repository.UserRepo,
	audit *repository.AuditRepo,
	settings *repository.SettingRepo,
	jwt *auth.Manager,
	enforcer *casbin.Enforcer,
	limiter *redis.RateLimiter,
	resets *auth.ResetManager,
	mailer *mail.Sender,
	mediaStore *media.Store,
	siteBaseURL string,
) *AuthService {
	return &AuthService{users: users, audit: audit, settings: settings, jwt: jwt, enforcer: enforcer, limiter: limiter, resets: resets, mailer: mailer, media: mediaStore, siteBaseURL: siteBaseURL}
}

// Register 注册新用户（注册成功即视为登录，直接签发令牌对）。
// 返回：令牌对；邮箱已存在等业务错误。
func (s *AuthService) Register(ctx context.Context, req model.RegisterReq, ip string, ua string) (*model.TokenPair, error) {
	// ---------- 参数校验（服务端二次校验，需求 3.1） ----------
	nickname := strings.TrimSpace(req.Nickname)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if nickname == "" || len([]rune(nickname)) > 20 {
		return nil, errs.New(errs.CodeBadRequest, "昵称需为 1-20 个字符")
	}
	if !validEmail(email) {
		return nil, errs.New(errs.CodeBadRequest, "邮箱格式不正确")
	}
	if !validPassword(req.Password) {
		return nil, errs.New(errs.CodeBadRequest, "密码至少 8 位，且包含字母与数字")
	}

	// ---------- 注册开关（站点设置 allow_register=false 时关闭注册） ----------
	if !s.registerOpen(ctx) {
		return nil, errs.New(errs.CodeForbidden, "当前站点已关闭注册")
	}

	// ---------- 邮箱唯一性 ----------
	taken, err := s.users.IsEmailTaken(ctx, email)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, errs.New(errs.CodeConflict, "该邮箱已注册，请直接登录")
	}

	// ---------- 生成用户名（邮箱前缀 + 6 位随机后缀，保证唯一） ----------
	username, err := s.uniqueUsername(ctx, email)
	if err != nil {
		return nil, err
	}

	// ---------- 密码哈希（bcrypt，成本默认） ----------
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码哈希失败：%w", err)
	}

	// ---------- 写入用户 ----------
	userID, err := s.users.Create(ctx, model.User{
		Email:        email,
		Username:     username,
		PasswordHash: string(hash),
		Nickname:     nickname,
		Status:       userStatusActive,
	})
	if err != nil {
		return nil, fmt.Errorf("创建用户失败：%w", err)
	}

	// ---------- 审计：注册 ----------
	s.writeAudit(ctx, userID, "register", "user", userID, ip, ua)

	// ---------- 签发令牌对（注册即登录；新用户密码版本为 1） ----------
	return s.issueTokenPair(ctx, userID, username, 1)
}

// Login 登录（邮箱或用户名 + 密码）。
// 返回：令牌对；账号不存在/密码错误统一提示「邮箱或密码不正确」。
func (s *AuthService) Login(ctx context.Context, req model.LoginReq, ip string, ua string) (*model.TokenPair, error) {
	account := strings.TrimSpace(req.Account)
	if account == "" || req.Password == "" {
		return nil, errs.New(errs.CodeBadRequest, "请输入邮箱与密码")
	}

	// ---------- 登录限流（5 次/分/账号） ----------
	if !s.limiter.AllowLogin(ctx, account) {
		return nil, errs.ErrRateLimit
	}

	// ---------- 查询用户（邮箱或用户名） ----------
	user, err := s.users.FindByAccount(ctx, account)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errs.New(errs.CodeUnauthorized, "邮箱或密码不正确")
		}
		return nil, err
	}

	// ---------- 密码校验 ----------
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errs.New(errs.CodeUnauthorized, "邮箱或密码不正确")
	}

	// ---------- 用户状态检查（封禁用户禁止登录） ----------
	if user.Status == userStatusBanned {
		return nil, errs.New(errs.CodeForbidden, "账号已被封禁，请联系管理员")
	}

	// ---------- 更新最后登录时间 ----------
	if err := s.users.UpdateLastLogin(ctx, user.ID); err != nil {
		return nil, err
	}

	// ---------- 审计：登录 ----------
	s.writeAudit(ctx, user.ID, "login", "user", user.ID, ip, ua)

	// ---------- 签发令牌对 ----------
	return s.issueTokenPair(ctx, user.ID, user.Username, user.PasswordVersion)
}

// Logout 登出（撤销 refresh token，加入黑名单）。
// 参数：tokenID refresh 令牌 ID（登出时由 handler 传入）。
func (s *AuthService) Logout(ctx context.Context, tokenID string, userID int64, ip string, ua string) error {
	// 撤销 refresh token（黑名单持有其剩余有效期）
	s.limiter.RevokeToken(ctx, tokenID, auth.RefreshTTL)
	// 审计：登出
	s.writeAudit(ctx, userID, "logout", "user", userID, ip, ua)
	return nil
}

// Refresh 刷新令牌对（refresh token 校验 + 黑名单检查 + 密码版本号校验）。
// 返回：新令牌对；refresh 无效、已撤销或密码已重置（版本号不匹配）返回未登录错误。
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*model.TokenPair, error) {
	claims, err := s.jwt.ParseRefresh(refreshToken)
	if err != nil {
		return nil, errs.ErrUnauthorized
	}
	// 黑名单检查（登出后撤销）
	if s.limiter.IsTokenRevoked(ctx, claims.TokenID) {
		return nil, errs.ErrUnauthorized
	}
	// 查询用户（确保仍有效）
	user, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errs.ErrUnauthorized
		}
		return nil, err
	}
	// 密码版本号校验（P1：找回密码后旧 refresh 立即失效，实现全局会话退出）
	if claims.PasswordVersion != user.PasswordVersion {
		return nil, errs.ErrUnauthorized
	}
	// 签发新令牌对
	return s.issueTokenPair(ctx, user.ID, user.Username, user.PasswordVersion)
}

// RequestPasswordReset 请求密码重置（M2 找回密码）。
// 说明：无论邮箱是否存在都返回成功（防邮箱枚举）；SMTP 未配置时重置链接写入日志（开发模式）。
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errs.New(errs.CodeBadRequest, "请输入注册邮箱")
	}
	// 生成重置令牌（60 秒重发限制）
	token, err := s.resets.Issue(email)
	if err != nil {
		return errs.New(errs.CodeBadRequest, err.Error())
	}
	// 重置链接（设计稿：链接 30 分钟内有效）
	resetURL := s.siteBaseURL + "/reset-password?token=" + token
	subject := "月言 · 重置你的密码"
	body := "你好，\n\n我们收到了你的密码重置请求。\n请在 30 分钟内打开以下链接完成验证：\n" + resetURL + "\n\n如果你没有发起该请求，请忽略此邮件。\n—— 月言"
	// 发送（失败静默，避免暴露邮箱存在性）
	_ = s.mailer.Send(email, subject, body)
	return nil
}

// ResetPassword 校验重置令牌并更新密码（M2 找回密码）。
func (s *AuthService) ResetPassword(ctx context.Context, token string, newPassword string) error {
	// 校验并消费令牌（30 分钟有效）
	email, err := s.resets.Consume(token)
	if err != nil {
		return errs.New(errs.CodeBadRequest, err.Error())
	}
	// 密码强度校验（与注册一致：≥8 位含字母数字）
	if !validPassword(newPassword) {
		return errs.New(errs.CodeBadRequest, "密码至少 8 位，且包含字母与数字")
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		return errs.New(errs.CodeBadRequest, "账号不存在")
	}
	// bcrypt 更新密码
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.users.UpdatePassword(ctx, user.ID, string(hash))
}

// ChangePassword 修改密码（账号安全页，需校验当前密码）。
// 参数：userID 当前用户；currentPassword 当前密码；newPassword 新密码（≥8 位含字母数字）。
// 说明：更新后密码版本自增（复用 P1 机制，其他设备旧会话立即失效）。
func (s *AuthService) ChangePassword(ctx context.Context, userID int64, currentPassword string, newPassword string) error {	if currentPassword == "" || newPassword == "" {
		return errs.New(errs.CodeBadRequest, "请填写当前密码与新密码")
	}
	if !validPassword(newPassword) {
		return errs.New(errs.CodeBadRequest, "密码至少 8 位，且包含字母与数字")
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return errs.ErrNotFound
	}
	// 校验当前密码（错误统一提示，不泄露账号信息）
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return errs.New(errs.CodeBadRequest, "当前密码不正确")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.users.UpdatePassword(ctx, userID, string(hash))
}

// DeactivateAccount 注销账号（需求 3.9）：删除用户与其全部数据（帖子/评论/关注/私信等
// 经外键级联清理），清理媒体物理文件与 casbin 内存分组，不可恢复。
// 保护：superadmin 禁止自助注销（防删光管理员导致系统锁死，先由其他管理员降级）。
func (s *AuthService) DeactivateAccount(ctx context.Context, userID int64, ip string, ua string) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return errs.ErrNotFound
	}
	if s.enforcer.GetRole(user.Username) == casbin.RoleSuperAdmin {
		return errs.New(errs.CodeStateConflict, "站长账号不可自助注销，请先移交或联系其他管理员")
	}
	// 审计（删除前落库，留痕注销动作）
	s.writeAudit(ctx, userID, "deactivate", "user", userID, ip, ua)
	// 删除用户行与级联数据，返回媒体存储键（物理文件需单独清理）
	storageKeys, err := s.users.DeleteCascade(ctx, userID)
	if err != nil {
		return err
	}
	// 清理 casbin 内存分组（重启前不残留已注销用户的角色映射）
	_ = s.enforcer.RemoveRole(user.Username)
	// 清理媒体物理文件（失败静默——磁盘残留不影响数据一致性，后续可人工清理）
	for _, key := range storageKeys {
		_ = s.media.Remove(key)
	}
	return nil
}

// GetProfile 查询用户资料（/me 与公开主页共用）。
// 参数：self 是否本人（本人含邮箱完整值，他人隐藏邮箱）。
func (s *AuthService) GetProfile(ctx context.Context, userID int64, self bool) (*model.UserProfile, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errs.ErrNotFound
		}
		return nil, err
	}

	profile := user.ToProfile()
	// 角色可见性：本人返回完整角色；公开资料仅保留 superadmin（设计稿主页「站长」徽章有意公开），
	// 其余运营角色（editor/author 等）对外隐藏，避免身份泄露与定向攻击面
	role := s.enforcer.GetRole(user.Username)
	if self {
		profile.Role = role
	} else if role == casbin.RoleSuperAdmin {
		profile.Role = role
	}
	// 非本人：邮箱脱敏（仅显示首字母 + *** + 域名）
	if !self {
		profile.Email = maskEmail(user.Email)
	}
	// 统计：帖子/获赞/话题（主页显示）
	profile.PostCount, err = s.users.CountPosts(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.LikeCount, err = s.users.CountLikes(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.TopicCount, err = s.users.CountTopics(ctx, userID)
	if err != nil {
		return nil, err
	}
	// 统计：浏览（设计稿个人主页「帖子/获赞/浏览」）
	profile.ViewCount, err = s.users.CountViews(ctx, userID)
	if err != nil {
		return nil, err
	}
	// 统计：粉丝/关注（粉丝/关注列表页入口展示）
	profile.FollowerCount, err = s.users.CountFollowers(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile.FollowingCount, err = s.users.CountFollowing(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// ---------- 内部辅助（纯函数） ----------

// issueTokenPair 签发令牌对（注册/登录/刷新共用）。
// 参数：userID 用户 ID；username 账号（casbin 查角色）；passwordVersion 密码版本号（写入 JWT claims）。
func (s *AuthService) issueTokenPair(ctx context.Context, userID int64, username string, passwordVersion int) (*model.TokenPair, error) {
	role := s.enforcer.GetRole(username)
	// 生成 refresh 令牌 ID（随机，撤销用）
	refreshID, err := randomTokenID()
	if err != nil {
		return nil, err
	}
	access, refresh, err := s.jwt.GenerateTokenPair(userID, role, passwordVersion, refreshID)
	if err != nil {
		return nil, err
	}
	return &model.TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    auth.ExpiresInSeconds(),
	}, nil
}

// uniqueUsername 生成唯一用户名：邮箱前缀 + 6 位随机后缀。
// 冲突时重试（最多 3 次）。
func (s *AuthService) uniqueUsername(ctx context.Context, email string) (string, error) {
	// 邮箱前缀：@ 前部分，仅保留字母数字（其余转 _）
	prefix := strings.Split(email, "@")[0]
	prefix = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, prefix)
	if prefix == "" {
		prefix = "user"
	}
	// 截断保留 24 字符（用户名上限 50，留后缀空间）
	if len(prefix) > 24 {
		prefix = prefix[:24]
	}

	for range 3 {
		suffix, err := randomTokenID()
		if err != nil {
			return "", err
		}
		candidate := fmt.Sprintf("%s_%s", prefix, suffix[:6])
		taken, err := s.users.IsUsernameTaken(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", errs.New(errs.CodeConflict, "用户名生成失败，请更换邮箱重试")
}

// registerOpen 注册开关是否开启（settings.allow_register；值为 "false" 时关闭）。
// 缺失或读取失败默认开启（与站点信息读取失败回退默认值一致，保证服务可用性）。
func (s *AuthService) registerOpen(ctx context.Context) bool {
	value, ok, err := s.settings.Get(ctx, "allow_register")
	if err != nil || !ok {
		return true
	}
	return value != "false"
}

// writeAudit 写入审计日志（失败仅记录，不影响主流程）。
func (s *AuthService) writeAudit(ctx context.Context, actorID int64, action string, resourceType string, resourceID int64, ip string, ua string) {
	if err := s.audit.Insert(ctx, repository.AuditEntry{
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		IP:           ip,
		UserAgent:    ua,
	}); err != nil {
		// 审计失败不阻断业务（日志由调用方记录）
		_ = err
	}
}

// randomTokenID 生成随机令牌 ID（24 位 hex）。
func randomTokenID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("随机数生成失败：%w", err)
	}
	return hex.EncodeToString(buf), nil
}

// validEmail 简单邮箱格式校验（含 @ 且点号分段合理）。
func validEmail(email string) bool {
	at := strings.IndexByte(email, '@')
	if at <= 0 || at == len(email)-1 {
		return false
	}
	domain := email[at+1:]
	return strings.Contains(domain, ".") && !strings.Contains(domain, "..")
}

// validPassword 密码强度校验：≥8 位且含字母与数字。
func validPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, r := range password {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			hasLetter = true
		}
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

// maskEmail 邮箱脱敏：首字符 + *** + @域名（他人主页显示）。
func maskEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 1 {
		return "***" + email[at:]
	}
	return email[:1] + "***" + email[at:]
}
