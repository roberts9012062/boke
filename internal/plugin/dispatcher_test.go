// internal/plugin/dispatcher_test.go
// 钩子调度器单元测试：拒绝阻断 / panic 恢复 / 超时隔离 / 异步不阻塞。
package plugin

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// 测试用同步可拦截钩子（真实钩子名，确保走同步调度路径）。
const testHook = HookCommentBeforeSave

func TestRegistryDispatchReject(t *testing.T) {
	// 拒绝处理器：返回 OK=false → Dispatch 返回拒绝（阻断核心）
	reg := NewRegistry(nil)
	reg.Register(testHook, func(ctx context.Context, ev Event) (Result, error) {
		return Result{OK: false, Reason: "插件拒绝"}, nil
	})
	res := reg.Dispatch(context.Background(), testHook, Event{})
	if res.OK {
		t.Fatal("期望拒绝，实际放行")
	}
	if res.Reason != "插件拒绝" {
		t.Fatalf("拒绝原因不符：%s", res.Reason)
	}
}

func TestRegistryPanicRecover(t *testing.T) {
	// panic 处理器：调度器恢复 → 放行（故障隔离）+ 错误回调
	var errCount atomic.Int64
	reg := NewRegistry(func(hook string, err error) {
		errCount.Add(1)
	})
	reg.Register(testHook, func(ctx context.Context, ev Event) (Result, error) {
		panic("插件崩溃")
	})
	res := reg.Dispatch(context.Background(), testHook, Event{})
	if !res.OK {
		t.Fatal("插件 panic 应放行（故障隔离），实际被阻断")
	}
	if errCount.Load() != 1 {
		t.Fatalf("错误回调应触发 1 次，实际 %d", errCount.Load())
	}
}

func TestRegistryTimeout(t *testing.T) {
	// 超时处理器：阻塞超过超时 → 放行 + 错误回调（不拖垮核心）
	var errCount atomic.Int64
	reg := NewRegistryWithTimeout(func(hook string, err error) {
		errCount.Add(1)
	}, 100*time.Millisecond)
	reg.Register(testHook, func(ctx context.Context, ev Event) (Result, error) {
		time.Sleep(500 * time.Millisecond) // 远超 100ms 超时
		return Result{OK: false, Reason: "迟到拒绝"}, nil
	})
	start := time.Now()
	res := reg.Dispatch(context.Background(), testHook, Event{})
	elapsed := time.Since(start)
	if !res.OK {
		t.Fatal("超时应放行（故障隔离），实际被阻断")
	}
	if elapsed > 400*time.Millisecond {
		t.Fatalf("超时隔离失败：耗时 %v（应约 100ms）", elapsed)
	}
	if errCount.Load() != 1 {
		t.Fatalf("超时错误回调应触发 1 次，实际 %d", errCount.Load())
	}
}

func TestRegistryHandlerErrorSkip(t *testing.T) {
	// 处理器返回 error：跳过该插件（放行）+ 错误回调；后续处理器继续执行
	var errCount atomic.Int64
	var secondRan atomic.Bool
	reg := NewRegistry(func(hook string, err error) {
		errCount.Add(1)
	})
	reg.Register(testHook, func(ctx context.Context, ev Event) (Result, error) {
		return Result{}, errors.New("插件内部错误")
	})
	reg.Register(testHook, func(ctx context.Context, ev Event) (Result, error) {
		secondRan.Store(true)
		return Result{OK: true}, nil
	})
	res := reg.Dispatch(context.Background(), testHook, Event{})
	if !res.OK {
		t.Fatal("插件错误应跳过（放行），实际被阻断")
	}
	if !secondRan.Load() {
		t.Fatal("后续处理器应继续执行")
	}
	if errCount.Load() != 1 {
		t.Fatalf("错误回调应触发 1 次，实际 %d", errCount.Load())
	}
}

func TestRegistryAsyncNotBlock(t *testing.T) {
	// 异步钩子：Dispatch 立即返回（后台执行，不阻塞调用方）
	reg := NewRegistry(nil)
	reg.Register(HookNotificationSend, func(ctx context.Context, ev Event) (Result, error) {
		time.Sleep(200 * time.Millisecond)
		return Result{OK: true}, nil
	})
	start := time.Now()
	res := reg.Dispatch(context.Background(), HookNotificationSend, Event{})
	if !res.OK {
		t.Fatal("异步钩子应放行")
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("异步钩子不应阻塞调用方")
	}
}

func TestRegistryUnregister(t *testing.T) {
	// 注销后处理器不再执行
	reg := NewRegistry(nil)
	handler := func(ctx context.Context, ev Event) (Result, error) {
		return Result{OK: false, Reason: "不应执行"}, nil
	}
	reg.Register(testHook, handler)
	reg.Unregister(testHook, handler)
	res := reg.Dispatch(context.Background(), testHook, Event{})
	if !res.OK {
		t.Fatal("注销后处理器不应执行")
	}
}

func TestRegistryDispatchModifyCollect(t *testing.T) {
	// 同步钩子改写收集（M3.9 修复）：多处理器时收集最后一个非空 Modify；
	// 无 Modify 的处理器不覆盖前序改写。
	reg := NewRegistry(nil)
	reg.Register(testHook, func(ctx context.Context, ev Event) (Result, error) {
		return Result{OK: true, Modify: map[string]any{"content": "第一段"}}, nil
	})
	reg.Register(testHook, func(ctx context.Context, ev Event) (Result, error) {
		return Result{OK: true}, nil // 无 Modify：不覆盖前序
	})
	reg.Register(testHook, func(ctx context.Context, ev Event) (Result, error) {
		return Result{OK: true, Modify: map[string]any{"content": "第二段"}}, nil
	})
	res := reg.Dispatch(context.Background(), testHook, Event{})
	modify, ok := res.Modify.(map[string]any)
	if !ok || modify["content"] != "第二段" {
		t.Fatalf("应收集最后一个非空 Modify，实际 %v", res.Modify)
	}
	// 拒绝仍优先于改写（拦截语义不变）
	reg.Register(testHook, func(ctx context.Context, ev Event) (Result, error) {
		return Result{OK: false, Reason: "拒绝"}, nil
	})
	res = reg.Dispatch(context.Background(), testHook, Event{})
	if res.OK || res.Reason != "拒绝" {
		t.Fatalf("拒绝应阻断（含改写收集），实际 %+v", res)
	}
}

// ---------- B1：waterfall 链式改写管道（分发模式对齐 Cordis 事件目录） ----------

// TestRegistryWaterfallChained 链式改写：下游处理器收到上游改写后的载荷，
// 最终 Modify 为管道末端值（多改写者组合而非覆盖）。
func TestRegistryWaterfallChained(t *testing.T) {
	reg := NewRegistry(nil)
	reg.Register(HookSearchQuery, func(ctx context.Context, ev Event) (Result, error) {
		kw, _ := ev.Payload.(string)
		return Result{OK: true, Modify: kw + "-a"}, nil
	})
	reg.Register(HookSearchQuery, func(ctx context.Context, ev Event) (Result, error) {
		kw, _ := ev.Payload.(string)
		if kw != "原始-a" {
			return Result{}, errors.New("下游应收到上游改写结果，实际：" + kw)
		}
		return Result{OK: true, Modify: kw + "-b"}, nil
	})
	res := reg.Dispatch(context.Background(), HookSearchQuery, Event{Payload: "原始"})
	if !res.OK {
		t.Fatalf("链式改写应放行，实际 %+v", res)
	}
	if res.Modify != "原始-a-b" {
		t.Fatalf("最终 Modify 应为管道末端值「原始-a-b」，实际 %v", res.Modify)
	}
}

// TestRegistryWaterfallRejectShortCircuit 拒绝短路：任一处理器拒绝即返回，
// 后续处理器不执行（拦截语义与 serial 一致）。
func TestRegistryWaterfallRejectShortCircuit(t *testing.T) {
	var downstreamRan atomic.Bool
	reg := NewRegistry(nil)
	reg.Register(HookContentRender, func(ctx context.Context, ev Event) (Result, error) {
		return Result{OK: false, Reason: "内容不合规"}, nil
	})
	reg.Register(HookContentRender, func(ctx context.Context, ev Event) (Result, error) {
		downstreamRan.Store(true)
		return Result{OK: true}, nil
	})
	res := reg.Dispatch(context.Background(), HookContentRender, Event{Payload: "正文"})
	if res.OK || res.Reason != "内容不合规" {
		t.Fatalf("拒绝应短路返回，实际 %+v", res)
	}
	if downstreamRan.Load() {
		t.Fatal("拒绝后下游处理器不应执行")
	}
}

// TestRegistryWaterfallNoModifyNil 全程无改写（或无处理器）时 Modify 为 nil
// （对齐旧扁平模型——调用点类型断言自然跳过）。
func TestRegistryWaterfallNoModifyNil(t *testing.T) {
	reg := NewRegistry(nil)
	reg.Register(HookSearchQuery, func(ctx context.Context, ev Event) (Result, error) {
		return Result{OK: true}, nil // 仅观察不修改
	})
	res := reg.Dispatch(context.Background(), HookSearchQuery, Event{Payload: "关键词"})
	if !res.OK || res.Modify != nil {
		t.Fatalf("无改写应返回 Modify=nil，实际 %+v", res)
	}
	// 无处理器同样 Modify=nil
	res = reg.Dispatch(context.Background(), HookAIBeforeGenerate, Event{Payload: map[string]any{"input": "x"}})
	if !res.OK || res.Modify != nil {
		t.Fatalf("无处理器应返回 Modify=nil，实际 %+v", res)
	}
}

// TestHookModeTable 分发模式目录：11 个钩子的模式标注与同步性派生正确。
func TestHookModeTable(t *testing.T) {
	cases := map[string]struct {
		mode DispatchMode
		sync bool
	}{
		HookPostBeforePublish: {ModeSerial, true},
		HookPostAfterPublish:  {ModeEmit, false},
		HookCommentBeforeSave: {ModeSerial, true},
		HookCommentAfterSave:  {ModeEmit, false},
		HookSearchQuery:       {ModeWaterfall, true},
		HookNotificationSend:  {ModeEmit, false},
		HookAdminPage:         {ModeSerial, true},
		HookContentRender:     {ModeWaterfall, true},
		HookAPIMiddleware:     {ModeSerial, true},
		HookAIBeforeGenerate:  {ModeWaterfall, true},
		HookAIAfterGenerate:   {ModeEmit, false},
	}
	for hook, want := range cases {
		mode, ok := HookMode(hook)
		if !ok || mode != want.mode {
			t.Fatalf("钩子 %s 分发模式应为 %s，实际 %s（ok=%v）", hook, want.mode, mode, ok)
		}
		if IsSyncHook(hook) != want.sync {
			t.Fatalf("钩子 %s 同步性应为 %v", hook, want.sync)
		}
	}
}
