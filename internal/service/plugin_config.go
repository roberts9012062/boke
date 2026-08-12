// internal/service/plugin_config.go
// 插件设置服务（M3.7 设置功能端到端）：
//   - schema 聚合：进程 Info 上报优先、市场清单兜底（旧插件兼容）
//   - 配置存储：plugin_instances.config JSONB（表已预留，不再污染全局 settings 表）
//   - 配置下发：保存后 PushConfig 即时生效；进程重启由 manager Start 激活后自动下发
package service

import (
	"context"
	"errors"

	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/proto"
)

// PluginDetailDTO 插件详情（设置页数据源：实例 + 聚合 schema + 已存配置）。
type PluginDetailDTO struct {
	ID             int64                       `json:"id"`               // 实例 ID
	PluginID       string                      `json:"plugin_id"`        // 插件 ID
	Name           string                      `json:"name"`             // 名称
	Version        string                      `json:"version"`          // 版本
	State          string                      `json:"state"`            // 状态
	SettingsSchema []PluginSettingField        `json:"settings_schema,omitempty"` // 设置项 schema（聚合结果）
	Config         map[string]string           `json:"config,omitempty"` // 已存配置（键值对）
}

// PluginConfigProvider 插件配置查询回调（manager 启动激活时调用：查 DB config 下发）。
func (s *PluginService) PluginConfigProvider(ctx context.Context, pluginID string) (map[string]string, error) {
	inst, err := s.plugs.FindByPluginID(ctx, pluginID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil // 未安装：按无配置处理
		}
		return nil, err
	}
	values, err := s.plugs.GetConfig(ctx, inst.ID)
	if err != nil {
		return nil, err
	}
	return values, nil
}

// Detail 插件详情（设置页数据源）：进程 Info 上报 schema 优先，市场清单兜底；附已存配置。
func (s *PluginService) Detail(ctx context.Context, instanceID int64) (*PluginDetailDTO, error) {
	inst, err := s.plugs.FindByID(ctx, instanceID)
	if err != nil {
		return nil, errs.ErrNotFound
	}
	dto := &PluginDetailDTO{
		ID: inst.ID, PluginID: inst.PluginID, Name: inst.Name,
		Version: inst.Version, State: inst.State,
	}
	// schema：进程上报优先（运行中拉 Info；失败/空静默走兜底）
	if s.manager != nil {
		if info, err := s.manager.PluginInfo(inst.PluginID); err == nil && len(info.GetSettings()) > 0 {
			dto.SettingsSchema = settingFieldsFromProto(info.GetSettings())
		}
	}
	// 兜底：市场清单声明（旧插件无 Info 上报时仍可配置）
	if len(dto.SettingsSchema) == 0 {
		if manifest, err := s.fetchManifest(ctx, ""); err == nil {
			for _, p := range manifest.Plugins {
				if p.ID == inst.PluginID {
					dto.SettingsSchema = p.SettingsSchema
					break
				}
			}
		}
	}
	// 已存配置（回显）
	values, err := s.plugs.GetConfig(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	dto.Config = values
	return dto, nil
}

// GetConfig 读取插件配置（设置页回显；不存在返回空 map）。
func (s *PluginService) GetConfig(ctx context.Context, instanceID int64) (map[string]string, error) {
	if _, err := s.plugs.FindByID(ctx, instanceID); err != nil {
		return nil, errs.ErrNotFound
	}
	values, err := s.plugs.GetConfig(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return values, nil
}

// SetConfig 保存插件配置：按 schema 声明的 key 过滤（未声明键丢弃）→ 落库 config JSONB
// → 进程运行则推送（即时生效）；进程未运行静默（重启时 Start 激活后自动下发）。
// 返回：实际保存的过滤后配置。
func (s *PluginService) SetConfig(ctx context.Context, instanceID int64, values map[string]string) (map[string]string, error) {
	// 聚合 schema（复用详情逻辑：进程 Info 优先、清单兜底）
	detail, err := s.Detail(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	// 仅保留 schema 声明的 key（防止任意键注入配置表）
	allowed := make(map[string]bool, len(detail.SettingsSchema))
	for _, f := range detail.SettingsSchema {
		allowed[f.Key] = true
	}
	filtered := make(map[string]string, len(values))
	for k, v := range values {
		if allowed[k] {
			filtered[k] = v
		}
	}
	// 落库（整体覆盖；schema 为空则清空配置）
	if err := s.plugs.SetConfig(ctx, instanceID, filtered); err != nil {
		return nil, err
	}
	// 运行中推送（失败不阻断——已落库，重启后 Start 激活时生效）
	if s.manager != nil {
		_ = s.manager.PushConfig(detail.PluginID, filtered)
	}
	return filtered, nil
}

// settingFieldsFromProto proto 设置项 → service 设置项（契约字段对齐转换）。
func settingFieldsFromProto(fields []*proto.SettingField) []PluginSettingField {
	out := make([]PluginSettingField, 0, len(fields))
	for _, f := range fields {
		out = append(out, PluginSettingField{
			Key: f.GetKey(), Label: f.GetLabel(), Type: f.GetType(),
			Default: f.GetDefault(), Options: f.GetOptions(),
		})
	}
	return out
}
