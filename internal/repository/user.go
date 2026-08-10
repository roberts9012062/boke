// internal/repository/user.go
// 用户数据访问层（pgx 原生 SQL + 结构化扫描）。
// 仅供 service 层调用（分层单向依赖：handler → service → repository）。
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/roberts9012062/boke/internal/model"
)

// ErrNotFound 记录不存在（内部错误，由 service 层转为业务错误码）。
var ErrNotFound = errors.New("记录不存在")

// UserRepo 用户数据访问（连接器类，允许结构体承载连接）。
type UserRepo struct {
	pool *pgxpool.Pool
}

// NewUserRepo 创建用户仓库。
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// scanUser 将查询行扫描为 User 实体（复用，避免重复编写扫描代码）。
// 无记录时返回 ErrNotFound（统一错误语义）。
func scanUser(row pgx.Row) (model.User, error) {
	var u model.User
	err := row.Scan(
		&u.ID, &u.Email, &u.Username, &u.PasswordHash,
		&u.Nickname, &u.AvatarURL, &u.Bio, &u.Status,
		&u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return model.User{}, wrapNotFound(err)
	}
	return u, nil
}

// userColumns 用户查询列清单（插入/查询共用，保证列序一致）。
const userColumns = `id, email, username, password_hash, nickname, avatar_url, bio, status, last_login_at, created_at, updated_at`

// Create 创建用户（返回新用户 ID）。
func (r *UserRepo) Create(ctx context.Context, u model.User) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, username, password_hash, nickname, avatar_url, bio, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		u.Email, u.Username, u.PasswordHash, u.Nickname, u.AvatarURL, u.Bio, u.Status,
	).Scan(&id)
	return id, err
}

// FindByID 按 ID 查询用户。
func (r *UserRepo) FindByID(ctx context.Context, id int64) (model.User, error) {
	return scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

// FindByEmail 按邮箱查询用户（注册唯一性校验/登录用）。
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (model.User, error) {
	return scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1`, email))
}

// FindByUsername 按用户名查询用户（登录用：邮箱或用户名）。
func (r *UserRepo) FindByUsername(ctx context.Context, username string) (model.User, error) {
	return scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE username = $1`, username))
}

// FindByAccount 按邮箱或用户名查询用户（登录统一入口）。
func (r *UserRepo) FindByAccount(ctx context.Context, account string) (model.User, error) {
	return scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE email = $1 OR username = $1`, account))
}

// UpdateLastLogin 更新最后登录时间。
func (r *UserRepo) UpdateLastLogin(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	return err
}

// IsEmailTaken 判断邮箱是否已被占用。
func (r *UserRepo) IsEmailTaken(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	return exists, err
}

// IsUsernameTaken 判断用户名是否已被占用。
func (r *UserRepo) IsUsernameTaken(ctx context.Context, username string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists)
	return exists, err
}

// Search 用户检索（昵称/账号名模糊匹配，搜索页用户 Tab）。
func (r *UserRepo) Search(ctx context.Context, keyword string, limit int) ([]model.User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+userColumns+` FROM users
		WHERE username ILIKE '%' || $1 || '%' OR nickname ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		LIMIT $2`, keyword, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]model.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// UpdateProfile 更新资料（昵称/简介；avatar 单独接口）。
func (r *UserRepo) UpdateProfile(ctx context.Context, id int64, nickname string, bio string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE users SET nickname = $2, bio = $3, updated_at = now() WHERE id = $1`,
		id, nickname, bio)
	return err
}

// UpdatePassword 更新密码哈希（M2 找回密码）。
func (r *UserRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, passwordHash)
	return err
}

// UpdateAvatar 更新头像地址。
func (r *UserRepo) UpdateAvatar(ctx context.Context, id int64, avatarURL string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET avatar_url = $2, updated_at = now() WHERE id = $1`, id, avatarURL)
	return err
}

// CountPosts 统计用户帖子数（主页统计用）。
func (r *UserRepo) CountPosts(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM posts WHERE author_id = $1 AND status = 'published'`, userID).Scan(&count)
	return count, err
}

// CountLikes 统计用户获赞数（posts.like_count 求和，主页统计用）。
func (r *UserRepo) CountLikes(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(sum(like_count), 0) FROM posts WHERE author_id = $1`, userID).Scan(&count)
	return count, err
}

// CountTopics 统计用户话题数（参与的话题数，主页统计用）。
func (r *UserRepo) CountTopics(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx, `
		SELECT count(DISTINCT t.id)
		FROM post_tags pt
		JOIN tags t ON t.id = pt.tag_id
		JOIN posts p ON p.id = pt.post_id
		WHERE p.author_id = $1 AND p.status = 'published'`, userID).Scan(&count)
	return count, err
}

// CountFollowers 统计用户粉丝数（user_relations type=follow，谁关注了我，M1.7）。
func (r *UserRepo) CountFollowers(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM user_relations WHERE target_id = $1 AND type = 'follow'`, userID).Scan(&count)
	return count, err
}

// CountFollowing 统计用户关注数（user_relations type=follow，我关注了谁，M1.7）。
func (r *UserRepo) CountFollowing(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM user_relations WHERE user_id = $1 AND type = 'follow'`, userID).Scan(&count)
	return count, err
}

// CountViews 统计用户帖子浏览总量（posts.view_count 求和，设计稿个人主页「浏览」统计）。
func (r *UserRepo) CountViews(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(sum(view_count), 0) FROM posts WHERE author_id = $1 AND status = 'published'`, userID).Scan(&count)
	return count, err
}

// wrapNotFound 将 pgx 无记录错误转为 ErrNotFound（统一错误语义）。
func wrapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: 目标记录不存在", ErrNotFound)
	}
	return err
}
