// 中继站发布选择与分类/标签校验（从 relay.go 拆出；方案见 discuss/relay-publish-choice-and-world-filter.md）：
// 发帖时「是否发送大世界 + 分类 + 标签」的选项聚合与合法性校验（校验钩子注入 PostService，避免反向依赖）。
package service

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/pkg/errs"
)


// relayTagsFor 中继站标签来源：用户发帖时选择的标签优先；
// 未选择时回退帖子自身标签（保持既有默认行为）。
func (s *RelayService) relayTagsFor(ctx context.Context, post model.Post) []string {
	if len(post.RelayTags) > 0 {
		return post.RelayTags
	}
	return s.postTags(ctx, post.ID)
}

// handshakeCategories 从握手元信息快照提取中继站分类列表（快照缺失/无分类返回空）。
func handshakeCategories(rc model.RelayConfig) []string {
	if rc.RelayMetaJSON == nil || *rc.RelayMetaJSON == "" {
		return nil
	}
	var meta model.RelayHandshakeMeta
	if err := json.Unmarshal([]byte(*rc.RelayMetaJSON), &meta); err != nil {
		return nil
	}
	return meta.Categories
}

// allowedCategories 发帖可选分类：握手快照非空以远端为权威（中继站只接受它认识的分类），
// 快照缺失时回退本地规范表分类。
func allowedCategories(rc model.RelayConfig) []string {
	if cats := handshakeCategories(rc); len(cats) > 0 {
		return cats
	}
	return model.RelayTaxonomyCategories()
}

// RelayPublishOptions 发帖表单的中继站发布选项（GET /relay/publish-options）。
type RelayPublishOptions struct {
	Enabled         bool                `json:"enabled"`          // 中继站是否启用（未启用时前端隐藏选择区）
	Categories      []string            `json:"categories"`       // 可选分类（有序）
	Taxonomy        map[string][]string `json:"taxonomy"`         // 分类 → 规范标签（本地规范表）
	DefaultCategory string              `json:"default_category"` // 预选分类（后台意向分类）
}

// PublishOptions 输出发帖表单所需的分类/标签规范与启用状态（登录接口）。
func (s *RelayService) PublishOptions(ctx context.Context) (RelayPublishOptions, error) {
	rc, err := s.relay.Config(ctx)
	if err != nil {
		return RelayPublishOptions{}, err
	}
	return RelayPublishOptions{
		Enabled:         rc.Enabled,
		Categories:      allowedCategories(rc),
		Taxonomy:        model.RelayTaxonomy(),
		DefaultCategory: rc.DefaultCategory,
	}, nil
}

// Taxonomy 输出分类/标签规范（公开接口：大世界筛选用；enabled/默认分类对游客无意义，置空）。
func (s *RelayService) Taxonomy(ctx context.Context) (RelayPublishOptions, error) {
	rc, err := s.relay.Config(ctx)
	if err != nil {
		return RelayPublishOptions{}, err
	}
	return RelayPublishOptions{
		Enabled:         false,
		Categories:      allowedCategories(rc),
		Taxonomy:        model.RelayTaxonomy(),
		DefaultCategory: "",
	}, nil
}

// relayMaxTags 中继站标签数量上限（协议 §3.2 tags ≤5）。
const relayMaxTags = 5

// relayMaxTagLen 单个中继站标签长度上限（字符）。
const relayMaxTagLen = 20

// ValidatePublishChoice 校验发帖时的中继站发布选择（勾选发送时：分类必填且在允许范围、
// 标签按分类规范表校验；远端自定义分类不在规范表内时仅校验数量与长度）。
func (s *RelayService) ValidatePublishChoice(ctx context.Context, input model.RelayPublishInput) error {
	if !input.Publish {
		return nil // 未勾选公开发送：无需校验
	}
	rc, err := s.relay.Config(ctx)
	if err != nil {
		return err
	}
	if !rc.Enabled || rc.SiteKey == "" {
		return errs.New(errs.CodeValidation, "站长未开启中继站（大世界），暂不能公开发送")
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		return errs.New(errs.CodeValidation, "公开发送到中继站须选择一个分类")
	}
	allowed := false
	for _, c := range allowedCategories(rc) {
		if c == category {
			allowed = true
			break
		}
	}
	if !allowed {
		return errs.New(errs.CodeValidation, "分类「"+category+"」不在中继站允许的分类范围内")
	}
	if len(input.Tags) > relayMaxTags {
		return errs.New(errs.CodeValidation, "中继站标签最多选择 5 个")
	}
	inTaxonomy := model.RelayTaxonomyTags(category) != nil
	for _, tag := range input.Tags {
		if utf8.RuneCountInString(tag) > relayMaxTagLen {
			return errs.New(errs.CodeValidation, "中继站标签「"+tag+"」超过 20 字")
		}
		if inTaxonomy && !model.RelayTaxonomyHasTag(category, tag) {
			return errs.New(errs.CodeValidation,
				"标签「"+tag+"」不是分类「"+category+"」的规范标签，请从列表中选择")
		}
	}
	return nil
}

