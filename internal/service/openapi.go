// internal/service/openapi.go
// 接口开放业务层：生成 Key（校验接口目录、生成随机 Key、计算过期时间）与凭证管理。
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 开放接口限制常量。
const (
	openAPIKeyNameMaxLen = 100 // 备注名上限（字符）
	openAPIKeyPrefix     = "oa_" // Key 前缀（便于识别来源）
	openAPIKeyRandomLen  = 32   // 随机部分字节数（hex 编码后 64 字符）
)

// OpenAPIService 接口开放服务（连接器类）。
type OpenAPIService struct {
	keys *repository.OpenAPIKeyRepo // 凭证数据访问
}

// NewOpenAPIService 创建接口开放服务。
func NewOpenAPIService(keys *repository.OpenAPIKeyRepo) *OpenAPIService {
	return &OpenAPIService{keys: keys}
}

// generateAPIKey 生成随机 Key：oa_ 前缀 + 32 字节随机数的 hex（64 字符；纯函数）。
func generateAPIKey() (string, error) {
	buf := make([]byte, openAPIKeyRandomLen)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成随机 Key 失败：%w", err)
	}
	return openAPIKeyPrefix + hex.EncodeToString(buf), nil
}

// normalizeEndpoints 校验并去重接口标识清单（纯函数；创建与权限设置共用）。
// 规则：至少一个、全部在目录内（防绕过目录授权未开放接口）、保持原顺序去重。
func normalizeEndpoints(endpoints []string) ([]string, error) {
	if len(endpoints) == 0 {
		return nil, errs.New(errs.CodeBadRequest, "请至少选择一个接口")
	}
	valid := model.CatalogEndpoints()
	seen := make(map[string]bool, len(endpoints))
	for _, ep := range endpoints {
		if !valid[ep] {
			return nil, errs.New(errs.CodeBadRequest, "包含未开放的接口："+ep)
		}
		seen[ep] = true
	}
	out := make([]string, 0, len(seen))
	for _, ep := range endpoints {
		if seen[ep] {
			out = append(out, ep)
			seen[ep] = false
		}
	}
	return out, nil
}

// CreateKey 生成凭证（endpoints 须在目录内且 ≥1；expire_days 正整数或空=永久；
// ownerUserID 为生成者用户 ID，作为 Key 的绑定归属供 /open/me 返回资料，0=不绑定）。
// 返回：创建后的完整凭证（含明文 Key）。
func (s *OpenAPIService) CreateKey(ctx context.Context, req model.CreateOpenAPIKeyReq, ownerUserID int64) (*model.OpenAPIKey, error) {
	// ---------- 参数校验 ----------
	name := strings.TrimSpace(req.Name)
	if utf8.RuneCountInString(name) > openAPIKeyNameMaxLen {
		return nil, errs.New(errs.CodeBadRequest, "备注名不能超过 100 字")
	}
	endpoints, err := normalizeEndpoints(req.Endpoints)
	if err != nil {
		return nil, err
	}
	// 过期时间：正整数 = N 天后过期；nil/0 = 永久
	var expiresAt *time.Time
	if req.ExpireDays != nil && *req.ExpireDays > 0 {
		t := time.Now().AddDate(0, 0, *req.ExpireDays)
		expiresAt = &t
	}

	// ---------- 生成 Key 并落库 ----------
	key, err := generateAPIKey()
	if err != nil {
		return nil, errs.New(errs.CodeInternal, "生成 Key 失败，请稍后重试")
	}
	record := model.OpenAPIKey{
		Name:      name,
		Key:       key,
		Endpoints: endpoints,
		UserID:    ownerUserID,
		ExpiresAt: expiresAt,
	}
	id, err := s.keys.Create(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("保存凭证失败：%w", err)
	}
	record.ID = id
	return &record, nil
}

// ListKeys 凭证列表（按创建时间倒序）。
func (s *OpenAPIService) ListKeys(ctx context.Context) ([]model.OpenAPIKey, error) {
	return s.keys.List(ctx)
}

// DeleteKey 删除凭证（不存在返回 404 语义）。
func (s *OpenAPIService) DeleteKey(ctx context.Context, id int64) error {
	found, err := s.keys.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("删除凭证失败：%w", err)
	}
	if !found {
		return errs.ErrNotFound
	}
	return nil
}

// UpdateKeyEndpoints 更新凭证的授权接口清单（后台「权限设置」：增/减可调用的接口）。
// 校验与创建同规（目录内、≥1、去重）；返回更新后的完整凭证。
func (s *OpenAPIService) UpdateKeyEndpoints(ctx context.Context, id int64, endpoints []string) (*model.OpenAPIKey, error) {
	normalized, err := normalizeEndpoints(endpoints)
	if err != nil {
		return nil, err
	}
	record, found, err := s.keys.UpdateEndpoints(ctx, id, normalized)
	if err != nil {
		return nil, fmt.Errorf("更新凭证权限失败：%w", err)
	}
	if !found {
		return nil, errs.ErrNotFound
	}
	return &record, nil
}
