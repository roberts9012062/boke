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
