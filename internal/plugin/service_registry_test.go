// internal/plugin/service_registry_test.go
// seam 服务注册表单元测试：注册/查找/幂等覆盖/按键注销/按 id 全清理/类型不符。
package plugin

import (
	"testing"
)

// fakeGreeter 测试用 seam 服务接口（模拟 MusicSource 角色）。
type fakeGreeter interface {
	Greet() string
}

// greeterImpl 接口实现（同一类型不同实例，验证幂等覆盖语义）。
type greeterImpl struct{ msg string }

func (g greeterImpl) Greet() string { return g.msg }

// fakeCounter 类型不符的实现（LookupService 泛型断言应失败）。
type fakeCounter struct{ n int }

func TestServiceRegistryRegisterLookup(t *testing.T) {
	reg := NewServiceRegistry()
	reg.Register("demo.greeter", "plugin-a", greeterImpl{msg: "hello"})
	svc, ok := LookupService[fakeGreeter](reg, "demo.greeter")
	if !ok || svc.Greet() != "hello" {
		t.Fatalf("应查回注册的服务，实际 ok=%v svc=%v", ok, svc)
	}
}

func TestServiceRegistryLookupMiss(t *testing.T) {
	reg := NewServiceRegistry()
	if _, ok := LookupService[fakeGreeter](reg, "demo.missing"); ok {
		t.Fatal("未注册键应查找失败")
	}
}

func TestServiceRegistryTypeMismatch(t *testing.T) {
	reg := NewServiceRegistry()
	reg.Register("demo.counter", "plugin-a", fakeCounter{n: 1})
	if _, ok := LookupService[fakeGreeter](reg, "demo.counter"); ok {
		t.Fatal("类型不符应查找失败（泛型断言保护）")
	}
}

func TestServiceRegistryIdempotentOverwrite(t *testing.T) {
	reg := NewServiceRegistry()
	reg.Register("demo.greeter", "plugin-a", greeterImpl{msg: "v1"})
	reg.Register("demo.greeter", "plugin-a", greeterImpl{msg: "v2"}) // 同 id 重注册：覆盖
	svc, ok := LookupService[fakeGreeter](reg, "demo.greeter")
	if !ok || svc.Greet() != "v2" {
		t.Fatalf("同 id 重注册应覆盖旧实例，实际 %v", svc)
	}
}

func TestServiceRegistryUnregister(t *testing.T) {
	reg := NewServiceRegistry()
	reg.Register("demo.greeter", "plugin-a", greeterImpl{msg: "a"})
	reg.Unregister("demo.greeter", "plugin-a")
	if _, ok := LookupService[fakeGreeter](reg, "demo.greeter"); ok {
		t.Fatal("注销后应查找失败")
	}
	// 注销后键完全移除（内部空切片不残留——重复注册/注销循环无脏状态）
	reg.Register("demo.greeter", "plugin-b", greeterImpl{msg: "b"})
	svc, ok := LookupService[fakeGreeter](reg, "demo.greeter")
	if !ok || svc.Greet() != "b" {
		t.Fatal("注销后重新注册应生效")
	}
}

func TestServiceRegistryUnregisterAll(t *testing.T) {
	reg := NewServiceRegistry()
	// 同一插件贡献多个键（模拟音乐插件注册多个 seam 服务）
	reg.Register("music.netease", "netease-plugin", greeterImpl{msg: "netease"})
	reg.Register("music.qq", "netease-plugin", greeterImpl{msg: "qq"})
	reg.Register("music.kw", "other-plugin", greeterImpl{msg: "kw"})
	// 插件停用：按 id 清理其全部贡献，不影响其他注册方
	reg.UnregisterAll("netease-plugin")
	if _, ok := LookupService[fakeGreeter](reg, "music.netease"); ok {
		t.Fatal("UnregisterAll 应清理该插件贡献的 music.netease")
	}
	if _, ok := LookupService[fakeGreeter](reg, "music.qq"); ok {
		t.Fatal("UnregisterAll 应清理该插件贡献的 music.qq")
	}
	if svc, ok := LookupService[fakeGreeter](reg, "music.kw"); !ok || svc.Greet() != "kw" {
		t.Fatal("其他注册方的服务不应受 UnregisterAll 影响")
	}
}
