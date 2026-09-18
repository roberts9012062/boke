// internal/model/relaytaxonomy_test.go
// 中继站「分类 → 规范标签」注册表纯函数测试：顺序稳定性、副本隔离、标签数量上限。
package model

import "testing"

// TestRelayTaxonomyCategoriesOrder 分类列表有序且与注册表键一致。
func TestRelayTaxonomyCategoriesOrder(t *testing.T) {
	cats := RelayTaxonomyCategories()
	tax := RelayTaxonomy()
	if len(cats) != len(tax) {
		t.Fatalf("分类顺序表与注册表键数不一致：%d vs %d", len(cats), len(tax))
	}
	for _, c := range cats {
		if _, ok := tax[c]; !ok {
			t.Fatalf("分类「%s」在顺序表中但不在注册表中", c)
		}
	}
}

// TestRelayTaxonomyCopyIsolated 副本隔离：改副本不影响注册表。
func TestRelayTaxonomyCopyIsolated(t *testing.T) {
	tax := RelayTaxonomy()
	tax["技术"] = []string{"被篡改"}
	if RelayTaxonomyHasTag("技术", "被篡改") {
		t.Fatal("修改副本影响了注册表（未做深拷贝）")
	}
	if !RelayTaxonomyHasTag("技术", "前端") {
		t.Fatal("注册表标签丢失")
	}
}

// TestRelayTaxonomyTagLimits 每个分类的规范标签数量在 5~12 之间（保证可选性且不过载）。
func TestRelayTaxonomyTagLimits(t *testing.T) {
	for category, tags := range RelayTaxonomy() {
		if len(tags) < 5 {
			t.Errorf("分类「%s」规范标签过少：%d", category, len(tags))
		}
		if len(tags) > 12 {
			t.Errorf("分类「%s」规范标签过多：%d", category, len(tags))
		}
	}
}

// TestRelayTaxonomyUnknownCategory 未知分类：无标签、HasTag 恒 false。
func TestRelayTaxonomyUnknownCategory(t *testing.T) {
	if tags := RelayTaxonomyTags("不存在的分类"); tags != nil {
		t.Fatalf("未知分类应返回 nil，实际 %v", tags)
	}
	if RelayTaxonomyHasTag("不存在的分类", "前端") {
		t.Fatal("未知分类 HasTag 应为 false")
	}
}
