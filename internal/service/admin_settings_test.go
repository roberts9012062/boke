// internal/service/admin_settings_test.go
// 头部导航配置单元测试：ValidateNavLinks 保存前校验 + ParseNavLinks 前台解析。
package service

import "testing"

// TestValidateNavLinks 覆盖导航配置的合法/非法样例（两级结构/数量/label/URL 协议白名单）。
func TestValidateNavLinks(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"空串等于清空配置", "", false},
		{"合法单级配置", `[{"label":"首页","url":"/","new_tab":false},{"label":"GitHub","url":"https://github.com","new_tab":true}]`, false},
		{"合法两级配置", `[{"label":"首页","url":"/"},{"label":"更多","url":"","children":[{"label":"关于","url":"/pages/about"},{"label":"外链","url":"https://example.com","new_tab":true}]}]`, false},
		{"一级纯分组URL可空", `[{"label":"分组","url":"","children":[{"label":"子项","url":"/topics"}]}]`, false},
		{"非法 JSON", `{label:首页}`, true},
		{"JSON 对象非数组", `{"label":"首页","url":"/"}`, true},
		{"label 为空", `[{"label":"","url":"/"}]`, true},
		{"一级 url 为空且无二级允许", `[{"label":"孤立项","url":""}]`, false},
		{"二级 url 为空拒绝", `[{"label":"组","children":[{"label":"子","url":""}]}]`, true},
		{"三级嵌套拒绝", `[{"label":"组","children":[{"label":"子","url":"/x","children":[{"label":"孙","url":"/y"}]}]}]`, true},
		{"二级危险协议拒绝", `[{"label":"组","children":[{"label":"子","url":"javascript:alert(1)"}]}]`, true},
		{"一级危险协议拒绝", `[{"label":"X","url":"javascript:alert(1)"}]`, true},
		{"相对协议拒绝", `[{"label":"X","url":"//evil.com"}]`, true},
		{"http 外链允许", `[{"label":"文档","url":"http://docs.example.com"}]`, false},
		{"站内路径允许", `[{"label":"话题","url":"/topics"}]`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateNavLinks(c.raw)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidateNavLinks(%q) 错误=%v，期望错误=%v", c.raw, err, c.wantErr)
			}
		})
	}

	// 数量上限：一级 21 项拒绝
	t.Run("一级数量上限", func(t *testing.T) {
		raw := `[`
		for i := 0; i < maxNavLinks+1; i++ {
			if i > 0 {
				raw += ","
			}
			raw += `{"label":"项` + string(rune('A'+i)) + `","url":"/"}`
		}
		raw += `]`
		if err := ValidateNavLinks(raw); err == nil {
			t.Fatal("超过 20 个一级项应返回校验错误")
		}
	})

	// 数量上限：单项 11 个二级拒绝
	t.Run("二级数量上限", func(t *testing.T) {
		raw := `[{"label":"组","children":[`
		for i := 0; i < maxNavChildren+1; i++ {
			if i > 0 {
				raw += ","
			}
			raw += `{"label":"子` + string(rune('A'+i)) + `","url":"/x"}`
		}
		raw += `]}]`
		if err := ValidateNavLinks(raw); err == nil {
			t.Fatal("超过 10 个二级项应返回校验错误")
		}
	})
}

// TestParseNavLinks 前台解析：合法文本解析为数组，空串/非法文本返回 nil（前端回退默认导航）。
func TestParseNavLinks(t *testing.T) {
	links := ParseNavLinks(`[{"label":"首页","url":"/","new_tab":false}]`)
	if len(links) != 1 || links[0].Label != "首页" || links[0].URL != "/" {
		t.Fatalf("合法配置解析不符：%+v", links)
	}
	if ParseNavLinks("") != nil {
		t.Fatal("空串应返回 nil")
	}
	if ParseNavLinks("not-json") != nil {
		t.Fatal("非法 JSON 应返回 nil")
	}
	// 空数组（清空配置落库形态）：解析为空列表，前端按未配置回退默认导航
	if links := ParseNavLinks("[]"); len(links) != 0 {
		t.Fatalf("空数组应解析为空列表，得到 %+v", links)
	}
}
