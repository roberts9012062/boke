// relayprobe_test.go 申请质询随机数与探测应答的单元测试（v1.5）。
package service

import (
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestProbeNonceIssueAndVerify 签发 → 核对通过 → 一次性语义（再核对失败）。
func TestProbeNonceIssueAndVerify(t *testing.T) {
	s := NewProbeNonceStore()
	n := s.Issue()
	if len(n) < 32 || len(n) > 128 {
		t.Fatalf("nonce 长度应满足协议 32~128，实际 %d", len(n))
	}
	if !s.Verify(n) {
		t.Fatal("签发后首次核对应通过")
	}
	if s.Verify(n) {
		t.Fatal("nonce 应为一次性：第二次核对必须失败（防重放）")
	}
}

// TestProbeNonceVerifyUnknown 未知 / 长度非法的 nonce 一律拒绝。
func TestProbeNonceVerifyUnknown(t *testing.T) {
	s := NewProbeNonceStore()
	if s.Verify("") || s.Verify("short") || s.Verify(strings.Repeat("x", 200)) {
		t.Fatal("未知或长度非法的 nonce 不应通过")
	}
}

// TestProbeNonceTTL 过期条目核对失败。
func TestProbeNonceTTL(t *testing.T) {
	s := NewProbeNonceStore()
	n := s.Issue()
	s.entries[n] = nonceEntry{value: n, issuedAt: time.Now().Add(-nonceTTL - time.Second)}
	if s.Verify(n) {
		t.Fatal("过期 nonce 不应通过")
	}
}

// TestProbeAnswer 应答分支：有效 nonce 回版本与回显；无效拒绝。
func TestProbeAnswer(t *testing.T) {
	svc := &RelayService{nonces: NewProbeNonceStore(), version: "v1.5.10", log: zap.NewNop()}
	n := svc.nonces.Issue()
	ok, version, echo := svc.ProbeAnswer(n)
	if !ok || version != "v1.5.10" || echo != n {
		t.Fatalf("有效 nonce 应答异常：ok=%v version=%s", ok, version)
	}
	// 已消费（一次性）+ 未知值：均拒绝
	if ok, _, _ = svc.ProbeAnswer(n); ok {
		t.Fatal("同一 nonce 二次探测应拒绝")
	}
	if ok, _, _ = svc.ProbeAnswer("unknown-nonce-value-1234567890abcdefghij"); ok {
		t.Fatal("未知 nonce 应拒绝")
	}
}
