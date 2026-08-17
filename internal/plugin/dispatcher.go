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
	"sort"
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
	// Register 注册钩子处理器（插件安装/启用时调用；优先级 0）。
	Register(hook string, handler Handler)
	// Unregister 注销钩子处理器（插件禁用/卸载时调用；按函数指针匹配）。
	Unregister(hook string, handler Handler)
	// RegisterWithID 注册带唯一标识的处理器（进程外插件适配器用；同 id 幂等；优先级 0）。
	RegisterWithID(hook string, id string, handler Handler)
	// RegisterRanked 注册带优先级的处理器（D3：小值先执行；同 id 幂等）。
	RegisterRanked(hook string, id string, priority int, handler Handler)
	// UnregisterWithID 按唯一标识注销处理器（与 RegisterWithID/RegisterRanked 对称）。
	UnregisterWithID(hook string, id string)
}

// OnError 钩子执行错误回调（写 plugin_instances.last_error，由装配方注入）。
type OnError func(hook string, err error)

// registeredHandler 已注册处理器（带可选唯一标识与优先级）。
// 说明：旧式 Register/Unregister 以函数指针匹配（builtin 顶层函数可用）；
//       进程外插件适配器是闭包（同一函数字面量共享 code 指针，%p 无法区分），
//       必须用 RegisterWithID/UnregisterWithID 按 id 精确匹配。
// 优先级（D3）：小值先执行（同值按注册顺序——稳定排序）；内置注册表可声明
// Priority（HookRegistration.Priority），进程外适配器暂为 0（proto HookInfo 无该字段，
// 后续契约扩展时透传 sdk.Hook.Priority）。
type registeredHandler struct {
	handler  Handler // 处理器
	id       string // 唯一标识（空=旧式，按函数指针匹配）
	priority int    // 执行优先级（小先执行）
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

// Register 注册钩子处理器（旧式：按函数指针去重，builtin 顶层函数适用；优先级 0）。
func (r *Registry) Register(hook string, handler Handler) {
	r.register(hook, handler, "", 0)
}

// RegisterWithID 注册带唯一标识的处理器（进程外插件适配器用；同 id 幂等；优先级 0）。
// 参数：id 插件级唯一标识（如 "pluginID/hook"），注销时按 id 精确移除。
func (r *Registry) RegisterWithID(hook string, id string, handler Handler) {
	r.register(hook, handler, id, 0)
}

// RegisterRanked 注册带优先级的处理器（D3：小值先执行；同值按注册顺序）。
// 参数：id 唯一标识；priority 执行优先级（内置注册表 HookRegistration.Priority）。
func (r *Registry) RegisterRanked(hook string, id string, priority int, handler Handler) {
	r.register(hook, handler, id, priority)
}

// register 内部注册（id 非空按 id 去重；id 空按函数指针去重；插入后按优先级稳定排序）。
func (r *Registry) register(hook string, handler Handler, id string, priority int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, h := range r.handlers[hook] {
		if (id != "" && h.id == id) || (id == "" && h.id == "" && sameHandler(h.handler, handler)) {
			return // 已注册（幂等）
		}
	}
	r.handlers[hook] = append(r.handlers[hook], registeredHandler{handler: handler, id: id, priority: priority})
	// D3：优先级排序（稳定——同优先级保持注册先后；每次注册后重排代价可忽略：处理器数量个位数）
	rank(r.handlers[hook])
}

// rank 就地按优先级稳定排序（纯辅助：registeredHandler 需要 sort 接口适配）。
func rank(items []registeredHandler) {
	sort.SliceStable(items, func(i int, j int) bool { return items[i].priority < items[j].priority })
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

// Dispatch 触发钩子（见接口注释；B1 起按钩子分发模式三路分发）。
func (r *Registry) Dispatch(ctx context.Context, hook string, ev Event) Result {
	// 快照处理器列表（避免执行中注册表变更竞争）
	r.mu.RLock()
	registered := append([]registeredHandler(nil), r.handlers[hook]...)
	r.mu.RUnlock()
	handlers := make([]Handler, 0, len(registered))
	for _, h := range registered {
		handlers = append(handlers, h.handler)
	}

	mode, _ := HookMode(hook)
	switch mode {
	case ModeEmit:
		// 异步钩子：后台执行，失败静默（记录错误）
		go r.dispatchAsync(hook, ev, handlers)
		return Result{OK: true}
	case ModeWaterfall:
		return r.dispatchWaterfall(ctx, hook, ev, handlers)
	default:
		// serial（含未知钩子名兜底）：串行执行，任一拒绝即阻断
		return r.dispatchSerial(ctx, hook, ev, handlers)
	}
}

// dispatchSerial 串行拦截执行：任一拒绝即短路返回；Modify 取最后一个非空（拦截钩子无改写语义，防御性保留）。
func (r *Registry) dispatchSerial(ctx context.Context, hook string, ev Event, handlers []Handler) Result {
	var lastModify any
	for _, handler := range handlers {
		result := r.dispatchOne(ctx, hook, ev, handler)
		if !result.OK {
			return result
		}
		if result.Modify != nil {
			lastModify = result.Modify
		}
	}
	return Result{OK: true, Modify: lastModify}
}

// dispatchWaterfall 链式改写管道（Cordis waterfall 的 Go 化落地）：
// 维护 currentPayload——每个处理器收到上游改写后的载荷，其 Modify 更新管道值；
// 任一拒绝即短路（拦截语义不变）；全程无改写时 Modify 为 nil（对齐旧扁平模型，
// 单处理器行为完全等价——零兼容成本）。
// 说明：Cordis 洋葱模型要求监听器持有 next()（可在下游返回后包装结果），
// 我们的 Handler 签名扁平（进程外插件经 gRPC 也只能扁平往返），故落地为管道语义——
// 已解决核心问题：多改写者基于彼此的结果组合，而非互相覆盖（见 discuss/插件架构B路线-实施计划.md）。
func (r *Registry) dispatchWaterfall(ctx context.Context, hook string, ev Event, handlers []Handler) Result {
	current := ev.Payload
	modified := false
	for _, handler := range handlers {
		step := ev // 逐处理器事件副本（Payload 换为管道当前值，其余字段不变）
		step.Payload = current
		result := r.dispatchOne(ctx, hook, step, handler)
		if !result.OK {
			return result // 拒绝短路（拦截语义与 serial 一致）
		}
		if result.Modify != nil {
			current = result.Modify // 上游改写作为下游输入（链式组合）
			modified = true
		}
	}
	if modified {
		return Result{OK: true, Modify: current}
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
