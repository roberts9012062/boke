// internal/plugin/service_registry.go
// ctx 服务注册表（B2 Cordis 对标）：命名空间键 → 服务实例的进程内容器。
// 对齐 dsh「Context 是服务的容器」——消费方按键查找服务而非导入具体实现；
// 与钩子 Registry 并列为插件系统两大进程内基础设施。
//
// 三角色约定（capability seam，见 seam_music.go 样例）：
//   - 服务定义：接口类型 + 键构造函数（Go interface 所在包）
//   - 提供方：内置实现或插件适配器（Register 注册；注册即副作用，可逆）
//   - 消费方：LookupService[T] 按键查找，不依赖具体实现包
//
// 可逆注册：同键多次注册按 id 区分（一插件可注册多个键）；UnregisterAll
// 按注册方 id 清理全部贡献（插件停用/卸载时调用——卸载即回滚）。
package plugin

import "sync"

// ServiceRegistry 服务注册表（连接器类；键 → 服务实例，泛型查找保类型安全）。
type ServiceRegistry struct {
	mu        sync.RWMutex              // 保护 entries
	entries   map[string][]serviceEntry // 键 → 服务列表（同键多提供方并存，查找取首个）
}

// serviceEntry 单条服务注册（提供方 id + 服务实例）。
type serviceEntry struct {
	id      string // 注册方标识（插件 ID 或 "builtin"；注销/清理按此匹配）
	service any    // 服务实例（消费方经 LookupService[T] 断言为具体接口）
}

// NewServiceRegistry 创建空服务注册表。
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{entries: make(map[string][]serviceEntry)}
}

// Register 注册服务（同 id 同键幂等——重复注册覆盖旧实例；不同 id 同键并存，查找取首个）。
// 参数：key 服务键（命名空间形式如 "music.netease"）；id 注册方标识（插件 ID）；svc 服务实例。
func (r *ServiceRegistry) Register(key string, id string, svc any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.entries[key]
	for i, e := range items {
		if e.id == id {
			items[i].service = svc // 同 id 重注册：覆盖（插件重启刷新适配器）
			return
		}
	}
	r.entries[key] = append(items, serviceEntry{id: id, service: svc})
}

// Unregister 注销单个服务条目（按键 + 注册方 id 精确匹配）。
func (r *ServiceRegistry) Unregister(key string, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.entries[key]
	for i, e := range items {
		if e.id == id {
			r.entries[key] = append(items[:i], items[i+1:]...)
			if len(r.entries[key]) == 0 {
				delete(r.entries, key)
			}
			return
		}
	}
}

// UnregisterAll 按注册方 id 清理其贡献的全部服务（插件停用/卸载时调用——注册可逆的统一出口）。
func (r *ServiceRegistry) UnregisterAll(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, items := range r.entries {
		kept := make([]serviceEntry, 0, len(items))
		for _, e := range items {
			if e.id != id {
				kept = append(kept, e)
			}
		}
		if len(kept) == 0 {
			delete(r.entries, key)
		} else {
			r.entries[key] = kept
		}
	}
}

// LookupService 按键查找服务并断言为目标接口类型（泛型；未注册或类型不符返回 false）。
// 说明：取该键下首个注册条目（同键多提供方时按注册先后，内置通常先于插件注册）。
func LookupService[T any](r *ServiceRegistry, key string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := r.entries[key]
	if len(items) == 0 {
		var zero T
		return zero, false
	}
	svc, ok := items[0].service.(T)
	return svc, ok
}
