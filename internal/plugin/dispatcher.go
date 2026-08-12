// internal/plugin/dispatcher.go
// 钩子调度器（M3.2 扩展框架核心）：插件按钩子注册/注销处理器，业务服务只依赖 Dispatcher 接口。
//
// 解耦与兼容性设计：
//   - 业务 service 依赖 Dispatcher 接口（构造器注入），不感知具体插件——插件增删零侵入核心
//   - 同步钩子串行执行：任一插件返回拒绝（OK=false）即阻断核心（对齐敏感词拦截先例）
//   - 插件故障隔离：handler panic / 超时（2s）→ 跳过该插件继续执行，记 last_error（不拖垮核心）
//   - 异步钩子：独立 goroutine 执行，失败静默 + 错误回调
//   - 接口即 go-plugin 进程隔离的边界（M3.2b 替换为进程外实现，业务代码不变）
package plugin

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// 同步钩子执行超时（文档约定：2 秒）。
const syncHookTimeout = 2 * time.Second

// Dispatcher 钩子调度器接口（业务服务依赖；插件实现解耦边界）。
type Dispatcher interface {
	// Dispatch 触发钩子：同步钩子串行执行（拒绝即阻断）；异步钩子后台执行。
	// 返回：同步钩子的聚合结果（首个拒绝）；异步钩子返回 OK 放行。
	Dispatch(ctx context.Context, hook string, ev Event) Result
	// Register 注册钩子处理器（插件安装/启用时调用）。
	Register(hook string, handler Handler)
	// Unregister 注销钩子处理器（插件禁用/卸载时调用）。
	Unregister(hook string, handler Handler)
}

// OnError 钩子执行错误回调（写 plugin_instances.last_error，由装配方注入）。
type OnError func(hook string, err error)

// registeredHandler 已注册处理器（带可选唯一标识）。
// 说明：旧式 Register/Unregister 以函数指针匹配（builtin 顶层函数可用）；
//       进程外插件适配器是闭包（同一函数字面量共享 code 指针，%p 无法区分），
//       必须用 RegisterWithID/UnregisterWithID 按 id 精确匹配。
type registeredHandler struct {
	handler Handler // 处理器
	id      string  // 唯一标识（空=旧式，按函数指针匹配）
}

// Registry 内存钩子注册表（Dispatcher 的进程内实现）。
type Registry struct {
	mu       sync.RWMutex                    // 保护 handlers
	handlers map[string][]registeredHandler  // hook → 处理器（按注册顺序执行）
	onError  OnError                         // 错误回调（可空）
	timeout  time.Duration                   // 同步钩子执行超时（默认 2s）
}

// NewRegistry 创建钩子注册表（同步钩子超时 2 秒）。
// 参数：onError 错误回调（插件执行异常时记录，可传 nil）。
func NewRegistry(onError OnError) *Registry {
	return NewRegistryWithTimeout(onError, syncHookTimeout)
}

// NewRegistryWithTimeout 创建钩子注册表（自定义超时，测试用）。
func NewRegistryWithTimeout(onError OnError, timeout time.Duration) *Registry {
	return &Registry{handlers: make(map[string][]registeredHandler), onError: onError, timeout: timeout}
}

// Register 注册钩子处理器（旧式：按函数指针去重，builtin 顶层函数适用）。
func (r *Registry) Register(hook string, handler Handler) {
	r.register(hook, handler, "")
}

// RegisterWithID 注册带唯一标识的处理器（进程外插件适配器用；同 id 幂等）。
// 参数：id 插件级唯一标识（如 "pluginID/hook"），注销时按 id 精确移除。
func (r *Registry) RegisterWithID(hook string, id string, handler Handler) {
	r.register(hook, handler, id)
}

// register 内部注册（id 非空按 id 去重；id 空按函数指针去重）。
func (r *Registry) register(hook string, handler Handler, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, h := range r.handlers[hook] {
		if (id != "" && h.id == id) || (id == "" && h.id == "" && sameHandler(h.handler, handler)) {
			return // 已注册（幂等）
		}
	}
	r.handlers[hook] = append(r.handlers[hook], registeredHandler{handler: handler, id: id})
}

// Unregister 注销钩子处理器（旧式：按函数指针匹配）。
func (r *Registry) Unregister(hook string, handler Handler) {
	r.unregister(hook, handler, "")
}

// UnregisterWithID 按唯一标识注销处理器（进程外插件适配器用）。
func (r *Registry) UnregisterWithID(hook string, id string) {
	r.unregister(hook, nil, id)
}

// unregister 内部注销（id 非空按 id 匹配；id 空按函数指针匹配）。
func (r *Registry) unregister(hook string, handler Handler, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := r.handlers[hook]
	for i, h := range items {
		if (id != "" && h.id == id) || (id == "" && h.id == "" && sameHandler(h.handler, handler)) {
			r.handlers[hook] = append(items[:i], items[i+1:]...)
			return
		}
	}
}

// Dispatch 触发钩子（见接口注释）。
func (r *Registry) Dispatch(ctx context.Context, hook string, ev Event) Result {
	// 快照处理器列表（避免执行中注册表变更竞争）
	r.mu.RLock()
	registered := append([]registeredHandler(nil), r.handlers[hook]...)
	r.mu.RUnlock()
	handlers := make([]Handler, 0, len(registered))
	for _, h := range registered {
		handlers = append(handlers, h.handler)
	}

	// 异步钩子：后台执行，失败静默（记录错误）
	if !IsSyncHook(hook) {
		go r.dispatchAsync(hook, ev, handlers)
		return Result{OK: true}
	}

	// 同步钩子：串行执行，任一拒绝即阻断
	for _, handler := range handlers {
		result := r.dispatchOne(ctx, hook, ev, handler)
		if !result.OK {
			return result
		}
	}
	return Result{OK: true}
}

// dispatchOne 执行单个同步处理器（超时 + panic 恢复 → 故障跳过并记录，不阻断核心）。
// 注意：handler 在独立 goroutine 执行，panic 恢复必须在该 goroutine 内（panic 不跨 goroutine 传播）。
func (r *Registry) dispatchOne(ctx context.Context, hook string, ev Event, handler Handler) (result Result) {
	timeoutCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	done := make(chan Result, 1)
	go func() {
		// panic 恢复：插件故障隔离（记录错误，按放行处理）
		defer func() {
			if p := recover(); p != nil {
				r.report(hook, fmt.Errorf("插件处理器 panic：%v", p))
				done <- Result{OK: true}
			}
		}()
		res, err := handler(timeoutCtx, ev)
		if err != nil {
			r.report(hook, err)
			// 处理器内部错误：跳过该插件（故障隔离），不阻断核心
			done <- Result{OK: true}
			return
		}
		done <- res
	}()

	select {
	case res := <-done:
		return res
	case <-timeoutCtx.Done():
		// 超时：跳过并记录（不阻断核心）
		r.report(hook, fmt.Errorf("插件处理器超时（%s）", r.timeout))
		return Result{OK: true}
	}
}

// dispatchAsync 后台执行异步钩子（失败静默 + 记录）。
func (r *Registry) dispatchAsync(hook string, ev Event, handlers []Handler) {
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	for _, handler := range handlers {
		func() {
			defer func() { _ = recover() }()
			if _, err := handler(ctx, ev); err != nil {
				r.report(hook, err)
			}
		}()
	}
}

// report 错误回调（线程安全由调用方保证）。
func (r *Registry) report(hook string, err error) {
	if r.onError != nil {
		r.onError(hook, err)
	}
}

// sameHandler 判断两个处理器是否同一函数（函数指针比较）。
func sameHandler(a Handler, b Handler) bool {
	return fmt.Sprintf("%p", a) == fmt.Sprintf("%p", b)
}
