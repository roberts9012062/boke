// internal/service/plugin_config.go
// 插件设置服务（M3.7 设置功能端到端 + B3 配置分层叠加）：
//   - schema 聚合：进程 Info 上报优先、市场清单兜底（旧插件兼容）
//   - 配置存储：plugin_instances.config JSONB（表已预留，不再污染全局 settings 表）
//   - 生效配置（B3）：schema Default 层 ⊕ 实例配置层——下发/回显/推送统一为合并结果，
//     插件无需自行处理默认值（对齐 Cordis bundle 默认 → patch 叠加思想）
//   - 配置下发：保存后 PushConfig 即时生效；进程重启由 manager Start 激活后自动下发
package service

import (
	"context"
	"errors"

	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/contract"
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
// B3：下发**生效配置**（schema 默认值 ⊕ 实例配置），插件无需自行处理默认值。
func (s *PluginService) PluginConfigProvider(ctx context.Context, pluginID string) (map[string]string, error) {
	inst, err := s.plugs.FindByPluginID(ctx, pluginID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil // 未安装：按无配置处理
		}
		return nil, err
	}
	return s.effectiveConfig(ctx, inst)
}

// PluginIDByInstance 已存在（plugin_runner.go）。此处补充反向：
// InstanceIDByPluginID 按插件 ID 查实例 ID（设置页/详情接口兼容插件 ID 直达——
// nav 动态入口声明的 href 用插件 ID，需解析为实例 ID；不存在返回 ErrNotFound）。
func (s *PluginService) InstanceIDByPluginID(ctx context.Context, pluginID string) (int64, error) {
	inst, err := s.plugs.FindByPluginID(ctx, pluginID)
	if err != nil {
		return 0, errs.ErrNotFound
	}
	return inst.ID, nil
}

// aggregateSchema 聚合插件设置 schema（进程 Info 上报优先，市场清单兜底）。
// 抽取自 Detail（B3：SetConfig/effectiveConfig 复用，消除三处内联重复）。
func (s *PluginService) aggregateSchema(ctx context.Context, pluginID string) []PluginSettingField {
	// 进程上报优先（运行中拉 Info；失败/空静默走兜底）
	if s.manager != nil {
		if info, err := s.manager.PluginInfo(pluginID); err == nil && len(info.Settings) > 0 {
			return settingFieldsFromContract(info.Settings)
		}
	}
	// 兜底：市场清单声明（旧插件无 Info 上报时仍可配置）
	if manifest, err := s.fetchManifest(ctx, ""); err == nil {
		for _, p := range manifest.Plugins {
			if p.ID == pluginID {
				return p.SettingsSchema
			}
		}
	}
	return nil
}

// MergeConfigDefaults 生效配置合并（B3 分层叠加，纯函数）：
// schema Default 层 ⊕ 实例配置层——实例 key **存在即覆盖**（含显式空串，
// 用户显式清空即空），不存在回退 Default；结果覆盖全部 schema key
// （无默认无实例的 key 为空串——插件 Config() 读取契约稳定）。
// 未在 schema 声明的 key 一律丢弃（防任意键注入，与 SetConfig 过滤语义一致）。
func MergeConfigDefaults(schema []PluginSettingField, values map[string]string) map[string]string {
	out := make(map[string]string, len(schema))
	for _, f := range schema {
		if v, ok := values[f.Key]; ok {
			out[f.Key] = v
		} else {
			out[f.Key] = f.Default
		}
	}
	return out
}

// effectiveConfig 计算插件生效配置（实例配置 + 默认值合并；读取侧统一出口）。
func (s *PluginService) effectiveConfig(ctx context.Context, inst repository.PluginInstance) (map[string]string, error) {
	values, err := s.plugs.GetConfig(ctx, inst.ID)
	if err != nil {
		return nil, err
	}
	schema := s.aggregateSchema(ctx, inst.PluginID)
	return MergeConfigDefaults(schema, values), nil
}

// Detail 插件详情（设置页数据源）：聚合 schema + 生效配置（默认值合并后回显）。
func (s *PluginService) Detail(ctx context.Context, instanceID int64) (*PluginDetailDTO, error) {
	inst, err := s.plugs.FindByID(ctx, instanceID)
	if err != nil {
		return nil, errs.ErrNotFound
	}
	dto := &PluginDetailDTO{
		ID: inst.ID, PluginID: inst.PluginID, Name: inst.Name,
		Version: inst.Version, State: inst.State,
		SettingsSchema: s.aggregateSchema(ctx, inst.PluginID),
	}
	// 生效配置（回显 = 默认值 ⊕ 实例值；B3 前仅回显实例值）
	config, err := s.effectiveConfig(ctx, inst)
	if err != nil {
		return nil, err
	}
	dto.Config = config
	return dto, nil
}

// GetConfig 读取插件生效配置（设置页回显；不存在返回空 map）。
func (s *PluginService) GetConfig(ctx context.Context, instanceID int64) (map[string]string, error) {
	inst, err := s.plugs.FindByID(ctx, instanceID)
	if err != nil {
		return nil, errs.ErrNotFound
	}
	return s.effectiveConfig(ctx, inst)
}

// SetConfig 保存插件配置：按 schema 声明的 key 过滤（未声明键丢弃）→ 落库 config JSONB
// → 进程运行则推送**生效配置**（默认值合并；即时生效）；进程未运行静默（重启时 Start 激活后自动下发）。
// 返回：实际生效的配置（合并后）。
func (s *PluginService) SetConfig(ctx context.Context, instanceID int64, values map[string]string) (map[string]string, error) {
	inst, err := s.plugs.FindByID(ctx, instanceID)
	if err != nil {
		return nil, errs.ErrNotFound
	}
	// 仅保留 schema 声明的 key（防止任意键注入配置表）
	schema := s.aggregateSchema(ctx, inst.PluginID)
	allowed := make(map[string]bool, len(schema))
	for _, f := range schema {
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
	// 生效配置（默认值合并；运行中推送失败不阻断——已落库，重启后生效）
	effective := MergeConfigDefaults(schema, filtered)
	if s.manager != nil {
		_ = s.manager.PushConfig(inst.PluginID, effective)
	}
	return effective, nil
}

// settingFieldsFromContract 契约设置项 → service 设置项（字段对齐转换）。
func settingFieldsFromContract(fields []contract.SettingField) []PluginSettingField {
	out := make([]PluginSettingField, 0, len(fields))
	for _, f := range fields {
		out = append(out, PluginSettingField{
			Key: f.Key, Label: f.Label, Type: f.Type,
			Default: f.Default, Options: f.Options,
		})
	}
	return out
}
