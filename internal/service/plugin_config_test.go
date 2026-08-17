// internal/service/plugin_config_test.go
// 生效配置合并单元测试（B3 分层叠加）：默认回退/实例覆盖/显式空串/未声明丢弃/schema 全覆盖。
package service

import (
	"reflect"
	"testing"
)

// testSchema 测试 schema（三种形态：有默认值/无默认值/select 型）。
var testSchema = []PluginSettingField{
	{Key: "greeting", Label: "问候语", Type: "text", Default: "你好"},
	{Key: "switch_on", Label: "开关", Type: "switch", Default: "true"},
	{Key: "note", Label: "备注", Type: "text"}, // 无默认值
}

func TestMergeConfigDefaultsFallback(t *testing.T) {
	// 空实例配置：全部回退默认值；无默认的 key 为空串（契约稳定）
	got := MergeConfigDefaults(testSchema, map[string]string{})
	want := map[string]string{"greeting": "你好", "switch_on": "true", "note": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("空实例配置应回退默认值，got %v want %v", got, want)
	}
}

func TestMergeConfigDefaultsOverride(t *testing.T) {
	// 实例值存在即覆盖默认
	got := MergeConfigDefaults(testSchema, map[string]string{"greeting": "欢迎光临"})
	if got["greeting"] != "欢迎光临" || got["switch_on"] != "true" {
		t.Fatalf("实例值应覆盖默认值，got %v", got)
	}
}

func TestMergeConfigDefaultsEmptyStringOverride(t *testing.T) {
	// 显式空串 = 用户显式清空：保持空而非回退默认（语义可预测）
	got := MergeConfigDefaults(testSchema, map[string]string{"greeting": ""})
	if got["greeting"] != "" {
		t.Fatalf("显式空串应保持空（不回退默认），got %v", got)
	}
}

func TestMergeConfigDefaultsDropsUndeclared(t *testing.T) {
	// schema 外的 key 一律丢弃（防注入，与 SetConfig 过滤语义一致）
	got := MergeConfigDefaults(testSchema, map[string]string{"evil_key": "x", "greeting": "hi"})
	if _, ok := got["evil_key"]; ok {
		t.Fatalf("未声明 key 应丢弃，got %v", got)
	}
}

func TestMergeConfigDefaultsNilValues(t *testing.T) {
	// nil 实例配置（未保存过）：等价空 map
	got := MergeConfigDefaults(testSchema, nil)
	if got["greeting"] != "你好" {
		t.Fatalf("nil 实例配置应回退默认值，got %v", got)
	}
}

func TestMergeConfigDefaultsEmptySchema(t *testing.T) {
	// 空 schema：结果为空 map（无声明即无配置）
	got := MergeConfigDefaults(nil, map[string]string{"greeting": "hi"})
	if len(got) != 0 {
		t.Fatalf("空 schema 应返回空 map，got %v", got)
	}
}
