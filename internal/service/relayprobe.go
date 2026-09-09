// 中继站申请质询的探测应答（协议 §4.12，v1.5）：
// 申请时签发一次性 nonce（5 分钟内存缓存），中继站回调 /api/v1/relay/probe 核对回显——
// 证明发起申请的程序实时控制着本站域名，且本站确实运行 boke。
package service

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// nonceTTL 质询随机数有效期（中继站同步回调在申请请求的 8s 探测窗口内，5 分钟余量足够）。
const nonceTTL = 5 * time.Minute

// nonceCapacity 缓存上限（防公开端点刷爆内存；超出时顺带清理过期项后仍超则拒绝签发）。
const nonceCapacity = 128

// nonceEntry 缓存条目（签发时间用于 TTL 清理）。
type nonceEntry struct {
	value     string
	issuedAt  time.Time
}

// ProbeNonceStore 质询随机数内存缓存（签发 / 核对 / 过期清理）。
type ProbeNonceStore struct {
	mu      sync.Mutex
	entries map[string]nonceEntry
}

// NewProbeNonceStore 构造质询随机数缓存。
func NewProbeNonceStore() *ProbeNonceStore {
	return &ProbeNonceStore{entries: make(map[string]nonceEntry)}
}

// Issue 签发一次性随机数（32 字节 → base64url 43 字符，满足协议 32~128 字符要求）。
// 签发前顺带清理过期条目；容量占满时返回空串（调用方将其随请求发出，中继站会以格式校验拒绝）。
func (s *ProbeNonceStore) Issue() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	value := base64.RawURLEncoding.EncodeToString(buf)
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, e := range s.entries {
		if now.Sub(e.issuedAt) > nonceTTL {
			delete(s.entries, k)
		}
	}
	if len(s.entries) >= nonceCapacity {
		return ""
	}
	s.entries[value] = nonceEntry{value: value, issuedAt: now}
	return value
}

// Verify 核对随机数是否为本站签发且未过期（核对即消费——一次性语义，防重放）。
func (s *ProbeNonceStore) Verify(nonce string) bool {
	if len(nonce) < 32 || len(nonce) > 128 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[nonce]
	if !ok {
		return false
	}
	delete(s.entries, nonce)
	return time.Since(e.issuedAt) <= nonceTTL
}

// ProbeAnswer 中继站质询应答（GET /api/v1/relay/probe?nonce=）：
// nonce 有效 → 回 boke 指纹与版本（{boke:true, version, nonce 原样回显}）；无效 → 不通过。
func (s *RelayService) ProbeAnswer(nonce string) (bool, string, string) {
	if !s.nonces.Verify(nonce) {
		return false, "", ""
	}
	return true, s.version, nonce
}
