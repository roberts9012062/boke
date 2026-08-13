// internal/repository/ai.go
// AI 数据访问（M4）：供应商（ai_providers）/ 任务路由（ai_tasks）/ 用量统计（ai_usage）。
// 说明：三张表同属「AI 域」，合并于一个文件（每层文件数 ≤8 约束）。
package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AiProvider AI 供应商实体（ai_providers 表）。
type AiProvider struct {
	ID             int64     // 供应商 ID
	Name           string    // 供应商名称：deepseek / qwen / kimi / glm / openai
	BaseURL        string    // OpenAI 兼容接口地址
	APIKeyEncrypted string   // API Key（AES 加密后存储）
	Models         []string  // 可用模型列表（JSONB）
	Enabled        bool      // 是否启用
	Priority       int       // 路由优先级（小先选）
	PriceInput     float64   // 输入单价（元/百万 token）
	PriceOutput    float64   // 输出单价（元/百万 token）
	CreatedAt      time.Time // 创建时间
	UpdatedAt      time.Time // 更新时间
}

// AiTask AI 任务路由实体（ai_tasks 表，task_name 唯一）。
type AiTask struct {
	ID             int64     // 任务 ID
	TaskName       string    // 任务名：post.summary / post.tags / comment.review
	ProviderID     *int64    // 绑定的供应商（NULL = 按 priority 自动路由）
	Model          string    // 模型名（空 = 用供应商默认模型）
	PromptTemplate string    // 提示词模板（{title}/{content} 占位符，后台可编辑）
	MaxTokens      int       // 最大输出 token
	Enabled        bool      // 是否启用
	ProviderName   string    // 供应商名（List 时 JOIN 回填，未绑定为空）
	UpdatedAt      time.Time // 更新时间
}

// AiUsage AI 用量实体（ai_usage 表，调用成功后写入）。
type AiUsage struct {
	ID         int64     // 记录 ID
	TaskName   string    // 任务名
	ProviderID int64     // 供应商 ID
	TokensIn   int64     // 输入 token
	TokensOut  int64     // 输出 token
	Cost       float64   // 费用（按供应商单价折算；MVP 记 0）
	CreatedAt  time.Time // 调用时间
}

// AiProviderRepo AI 供应商数据访问（连接器类）。
type AiProviderRepo struct {
	pool *pgxpool.Pool
}

// NewAiProviderRepo 创建供应商仓库。
func NewAiProviderRepo(pool *pgxpool.Pool) *AiProviderRepo {
	return &AiProviderRepo{pool: pool}
}

// ListAll 全部供应商（后台列表，按优先级排序）。
func (r *AiProviderRepo) ListAll(ctx context.Context) ([]AiProvider, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, base_url, api_key_encrypted, models, enabled, priority,
		       price_input, price_output, created_at, updated_at
		FROM ai_providers ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AiProvider, 0)
	for rows.Next() {
		var p AiProvider
		var models []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.APIKeyEncrypted, &models, &p.Enabled, &p.Priority, &p.PriceInput, &p.PriceOutput, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(models, &p.Models) // 模型列表解析失败降级为空（不影响主流程）
		items = append(items, p)
	}
	return items, rows.Err()
}

// ListEnabled 已启用供应商（任务自动路由候选）。
func (r *AiProviderRepo) ListEnabled(ctx context.Context) ([]AiProvider, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, base_url, api_key_encrypted, models, enabled, priority,
		       price_input, price_output
		FROM ai_providers WHERE enabled = TRUE ORDER BY priority, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AiProvider, 0)
	for rows.Next() {
		var p AiProvider
		var models []byte
		if err := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.APIKeyEncrypted, &models, &p.Enabled, &p.Priority, &p.PriceInput, &p.PriceOutput); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(models, &p.Models)
		items = append(items, p)
	}
	return items, rows.Err()
}

// FindByID 按 ID 查供应商（无记录返回 false）。
func (r *AiProviderRepo) FindByID(ctx context.Context, id int64) (*AiProvider, bool, error) {
	var p AiProvider
	var models []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, base_url, api_key_encrypted, models, enabled, priority,
		       price_input, price_output, created_at, updated_at
		FROM ai_providers WHERE id = $1`, id).Scan(
		&p.ID, &p.Name, &p.BaseURL, &p.APIKeyEncrypted, &models, &p.Enabled, &p.Priority, &p.PriceInput, &p.PriceOutput, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	_ = json.Unmarshal(models, &p.Models)
	return &p, true, nil
}

// Create 新增供应商（返回 ID；api_key 由调用方加密后传入）。
func (r *AiProviderRepo) Create(ctx context.Context, p AiProvider) (int64, error) {
	models, _ := json.Marshal(p.Models)
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO ai_providers (name, base_url, api_key_encrypted, models, enabled, priority, price_input, price_output)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		p.Name, p.BaseURL, p.APIKeyEncrypted, models, p.Enabled, p.Priority, p.PriceInput, p.PriceOutput).Scan(&id)
	return id, err
}

// Update 更新供应商（api_key_encrypted 为空串时保持原值不动，支持「留空不改」）。
func (r *AiProviderRepo) Update(ctx context.Context, p AiProvider) error {
	models, _ := json.Marshal(p.Models)
	// api_key 留空时不覆盖（COALESCE 分支），否则更新为新密文
	_, err := r.pool.Exec(ctx, `
		UPDATE ai_providers SET
			name = $2, base_url = $3,
			api_key_encrypted = CASE WHEN $4::text = '' THEN api_key_encrypted ELSE $4 END,
			models = $5, enabled = $6, priority = $7,
			price_input = $8, price_output = $9, updated_at = now()
		WHERE id = $1`,
		p.ID, p.Name, p.BaseURL, p.APIKeyEncrypted, models, p.Enabled, p.Priority, p.PriceInput, p.PriceOutput)
	return err
}

// Delete 删除供应商（ai_tasks 引用 ON DELETE SET NULL 自动置空）。
func (r *AiProviderRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM ai_providers WHERE id = $1`, id)
	return err
}

// AiTaskRepo AI 任务数据访问（连接器类）。
type AiTaskRepo struct {
	pool *pgxpool.Pool
}

// NewAiTaskRepo 创建任务仓库。
func NewAiTaskRepo(pool *pgxpool.Pool) *AiTaskRepo {
	return &AiTaskRepo{pool: pool}
}

// List 任务列表（JOIN 供应商名，按任务名排序）。
func (r *AiTaskRepo) List(ctx context.Context) ([]AiTask, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.task_name, t.provider_id, t.model, t.prompt_template, t.max_tokens, t.enabled, t.updated_at,
		       COALESCE(p.name, '')
		FROM ai_tasks t LEFT JOIN ai_providers p ON p.id = t.provider_id
		ORDER BY t.task_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AiTask, 0)
	for rows.Next() {
		var t AiTask
		if err := rows.Scan(&t.ID, &t.TaskName, &t.ProviderID, &t.Model, &t.PromptTemplate, &t.MaxTokens, &t.Enabled, &t.UpdatedAt, &t.ProviderName); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// FindByName 按任务名查任务（无记录返回 false）。
func (r *AiTaskRepo) FindByName(ctx context.Context, taskName string) (*AiTask, bool, error) {
	var t AiTask
	err := r.pool.QueryRow(ctx, `
		SELECT id, task_name, provider_id, model, prompt_template, max_tokens, enabled, updated_at
		FROM ai_tasks WHERE task_name = $1`, taskName).Scan(
		&t.ID, &t.TaskName, &t.ProviderID, &t.Model, &t.PromptTemplate, &t.MaxTokens, &t.Enabled, &t.UpdatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &t, true, nil
}

// Update 更新任务配置（模型/提示词/最大 token/绑定供应商）。
func (r *AiTaskRepo) Update(ctx context.Context, t AiTask) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE ai_tasks SET
			provider_id = $2, model = $3, prompt_template = $4, max_tokens = $5, updated_at = now()
		WHERE task_name = $1`,
		t.TaskName, t.ProviderID, t.Model, t.PromptTemplate, t.MaxTokens)
	return err
}

// SetEnabled 启停任务。
func (r *AiTaskRepo) SetEnabled(ctx context.Context, taskName string, enabled bool) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE ai_tasks SET enabled = $2, updated_at = now() WHERE task_name = $1`, taskName, enabled)
	return err
}

// AiUsageRepo AI 用量数据访问（连接器类）。
type AiUsageRepo struct {
	pool *pgxpool.Pool
}

// NewAiUsageRepo 创建用量仓库。
func NewAiUsageRepo(pool *pgxpool.Pool) *AiUsageRepo {
	return &AiUsageRepo{pool: pool}
}

// Record 写入一次用量（调用成功后记录 token 与费用）。
func (r *AiUsageRepo) Record(ctx context.Context, u AiUsage) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ai_usage (task_name, provider_id, tokens_in, tokens_out, cost)
		VALUES ($1, $2, $3, $4, $5)`,
		u.TaskName, u.ProviderID, u.TokensIn, u.TokensOut, u.Cost)
	return err
}

// StatsSummary 用量汇总（今日调用数/今日 token/累计调用/累计 token + 费用）。
type AiUsageSummary struct {
	TodayCalls  int64   `json:"today_calls"`  // 今日调用次数
	TodayTokens int64   `json:"today_tokens"` // 今日 token 总量
	TotalCalls  int64   `json:"total_calls"`  // 累计调用次数
	TotalTokens int64   `json:"total_tokens"` // 累计 token 总量
	TodayCost   float64 `json:"today_cost"`   // 今日费用（元）
	TotalCost   float64 `json:"total_cost"`   // 累计费用（元）
}

// Summary 用量汇总（今日 + 累计）。
func (r *AiUsageRepo) Summary(ctx context.Context) (*AiUsageSummary, error) {
	s := &AiUsageSummary{}
	err := r.pool.QueryRow(ctx, `
		SELECT
			count(*) FILTER (WHERE created_at >= current_date),
			COALESCE(sum(tokens_in + tokens_out) FILTER (WHERE created_at >= current_date), 0),
			count(*),
			COALESCE(sum(tokens_in + tokens_out), 0),
			COALESCE(sum(cost) FILTER (WHERE created_at >= current_date), 0),
			COALESCE(sum(cost), 0)
		FROM ai_usage`).Scan(&s.TodayCalls, &s.TodayTokens, &s.TotalCalls, &s.TotalTokens, &s.TodayCost, &s.TotalCost)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// DayStat 单日用量（趋势图表）。
type AiDayStat struct {
	Day    string  `json:"day"`     // 日期（YYYY-MM-DD）
	Calls  int64   `json:"calls"`   // 当日调用次数
	Tokens int64   `json:"tokens"`  // 当日 token 总量
	Cost   float64 `json:"cost"`    // 当日费用（元）
}

// StatsByDay 近 N 日按日聚合（补零到每日，图表直用）。
func (r *AiUsageRepo) StatsByDay(ctx context.Context, days int) ([]AiDayStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT to_char(created_at, 'YYYY-MM-DD') AS day,
		       count(*) AS calls,
		       COALESCE(sum(tokens_in + tokens_out), 0) AS tokens,
		       COALESCE(sum(cost), 0) AS cost
		FROM ai_usage
		WHERE created_at >= current_date - ($1::int - 1)
		GROUP BY day ORDER BY day`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AiDayStat, 0)
	for rows.Next() {
		var d AiDayStat
		if err := rows.Scan(&d.Day, &d.Calls, &d.Tokens, &d.Cost); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	// 补零：近 N 日每天一行（无调用日补 0，图表不中断）
	return fillDayGaps(items, days), nil
}

// fillDayGaps 按日期补零（纯函数：输入有序日期列表，输出连续 N 日）。
func fillDayGaps(items []AiDayStat, days int) []AiDayStat {
	// 建立日期 → 统计映射
	byDay := make(map[string]AiDayStat, len(items))
	for _, d := range items {
		byDay[d.Day] = d
	}
	filled := make([]AiDayStat, 0, days)
	day := time.Now().AddDate(0, 0, -(days - 1))
	for i := 0; i < days; i++ {
		key := day.Format("2006-01-02")
		if d, ok := byDay[key]; ok {
			filled = append(filled, d)
		} else {
			filled = append(filled, AiDayStat{Day: key})
		}
		day = day.AddDate(0, 0, 1)
	}
	return filled
}
