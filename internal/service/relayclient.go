// 中继站出站订阅客户端（B-2'，M0 轮询模式）：
// manager 监视配置变化启停 worker；worker 握手 → head（backfill 回填）→ 30s 轮询 → 写缓存 → TTL 清理。
// 正确性来源是 seq + 补拉（ADR-4），轮询只是 M0 的传输形态，M1 换 WS 不改本层语义。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/model"
	"github.com/roberts9012062/boke/internal/repository"
)

// relayPollInterval 轮询间隔（M0：30s 页面级实时性）。
const relayPollInterval = 30 * time.Second

// RelayClientManager 订阅任务管理器：随配置启停 worker（配置保存后 ≤5s 生效）。
type RelayClientManager struct {
	svc  *RelayService
	relay *repository.RelayRepo
	log  *zap.Logger
}

// NewRelayClientManager 构造管理器并启动监视循环。
func NewRelayClientManager(svc *RelayService, relay *repository.RelayRepo, log *zap.Logger) *RelayClientManager {
	m := &RelayClientManager{svc: svc, relay: relay, log: log}
	go m.watch(context.Background())
	return m
}

// watch 每 5 秒检查配置版本：updated_at 变化即按当前 enabled 状态启停 worker。
func (m *RelayClientManager) watch(ctx context.Context) {
	var lastUpdated time.Time
	var runningCancel context.CancelFunc
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if runningCancel != nil {
				runningCancel()
			}
			return
		case <-ticker.C:
			rc, err := m.relay.Config(ctx)
			if err != nil {
				continue
			}
			needRestart := !rc.UpdatedAt.Equal(lastUpdated) && !lastUpdated.IsZero() || lastUpdated.IsZero()
			if lastUpdated.IsZero() {
				needRestart = true // 进程启动首轮：按当前配置决定是否拉起
			}
			if !needRestart {
				continue
			}
			if runningCancel != nil {
				runningCancel()
				runningCancel = nil
			}
			if rc.Enabled {
				var workerCtx context.Context
				workerCtx, runningCancel = context.WithCancel(ctx)
				go m.runWorker(workerCtx, rc)
				m.log.Info("大世界订阅已启动", zap.String("relay", rc.URL))
			} else {
				m.log.Info("大世界订阅未启用")
			}
			lastUpdated = rc.UpdatedAt
		}
	}
}

// runWorker 单个订阅工作循环：握手缓存元信息 → 增量轮询 → 每小时清缓存。
func (m *RelayClientManager) runWorker(ctx context.Context, rc model.RelayConfig) {
	if err := m.handshakeOnce(ctx, rc); err != nil {
		m.log.Warn("大世界握手失败（下轮重试）", zap.Error(err))
	}
	m.sweepOnce(ctx)

	poll := time.NewTicker(relayPollInterval)
	sweep := time.NewTicker(time.Hour)
	defer poll.Stop()
	defer sweep.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
			if err := m.pollOnce(ctx); err != nil {
				m.log.Warn("大世界轮询失败（下轮重试）", zap.Error(err))
			}
		case <-sweep.C:
			m.sweepOnce(ctx)
		}
	}
}

// handshakeOnce 握手并把元信息快照落库（配置页回显与发布预检的数据源）。
func (m *RelayClientManager) handshakeOnce(ctx context.Context, rc model.RelayConfig) error {
	name, avatar := m.svc.siteBrief()
	reqBody := map[string]any{
		"proto_ver": 1, "mode": rc.Mode, "base_url": m.svc.baseURL(),
		"site_name": name, "avatar": avatar,
	}
	var resp model.RelayHandshakeResp
	if err := m.svc.postJSON(ctx, rc.URL+"/api/v1/handshake", rc.SiteKey, reqBody, &resp); err != nil {
		return err
	}
	metaJSON, err := json.Marshal(resp.Meta)
	if err != nil {
		return err
	}
	return m.relay.SaveMeta(ctx, string(metaJSON))
}

// pollOnce 一轮增量拉取：head 对齐（首轮回填）→ poll → 信封落地 → 游标推进。
func (m *RelayClientManager) pollOnce(ctx context.Context) error {
	rc, err := m.relay.Config(ctx)
	if err != nil {
		return err
	}
	if !rc.Enabled {
		return nil
	}
	// 首次（游标为 0）：从 backfill_seq 起拉，回填最近 100 条作首屏底料（协议 §4.6）
	if rc.LastSeq == 0 {
		var head struct {
			LatestSeq   int64 `json:"latest_seq"`
			BackfillSeq int64 `json:"backfill_seq"`
		}
		if err := m.svc.getJSON(ctx, rc.URL+"/api/v1/stream/head", rc.SiteKey, &head); err != nil {
			return err
		}
		if head.BackfillSeq > 0 {
			if err := m.relay.SaveCursor(ctx, head.BackfillSeq); err != nil {
				return err
			}
			rc.LastSeq = head.BackfillSeq
		}
	}
	var out struct {
		Envelopes []model.RelayEnvelope `json:"envelopes"`
		LatestSeq int64                 `json:"latest_seq"`
	}
	url := fmt.Sprintf("%s/api/v1/stream/poll?after_seq=%d&limit=200", rc.URL, rc.LastSeq)
	if err := m.svc.getJSON(ctx, url, rc.SiteKey, &out); err != nil {
		if strings.Contains(err.Error(), "SEQ_TOO_OLD") {
			// 游标落后超过保留窗口：全量重置（清缓存重新回填）
			_ = m.relay.ResetCursor(ctx)
			return fmt.Errorf("游标过旧已重置: %w", err)
		}
		return err
	}
	for _, env := range out.Envelopes {
		if err := m.applyEnvelope(ctx, env, rc.LocalRetentionDays); err != nil {
			m.log.Warn("信封处理失败", zap.String("type", env.Type), zap.Error(err))
		}
	}
	if out.LatestSeq > rc.LastSeq {
		return m.relay.SaveCursor(ctx, out.LatestSeq)
	}
	return nil
}

// applyEnvelope 单信封落地：publish/update 入缓存、delete 删缓存、config.update 刷元信息。
func (m *RelayClientManager) applyEnvelope(ctx context.Context, env model.RelayEnvelope, retentionDays int) error {
	switch env.Type {
	case model.RelayEventPublish, model.RelayEventUpdate:
		var payload model.RelayContentPayload
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			return err
		}
		return m.relay.UpsertCache(ctx, model.RelayCacheItem{
			ContentID:   payload.ContentID,
			Payload:     payload,
			PublishedAt: time.Unix(payload.PublishedAt, 0),
		}, retentionDays)
	case model.RelayEventDelete:
		var data model.RelayDeleteData
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return err
		}
		return m.relay.DeleteCache(ctx, data.ContentID)
	case model.RelayEventConfigUpdate:
		var data struct {
			Meta model.RelayHandshakeMeta `json:"meta"`
		}
		if err := json.Unmarshal(env.Data, &data); err != nil {
			return err
		}
		metaJSON, err := json.Marshal(data.Meta)
		if err != nil {
			return err
		}
		return m.relay.SaveMeta(ctx, string(metaJSON))
	}
	return nil // comment.* 等后续里程碑事件：M0 忽略
}

// sweepExpired 本地缓存 TTL 清理（定时 + 惰性双保险的定时半边）。
func (m *RelayClientManager) sweepOnce(ctx context.Context) {
	if n, err := m.relay.SweepExpired(ctx); err != nil {
		m.log.Warn("大世界缓存清理失败", zap.Error(err))
	} else if n > 0 {
		m.log.Info("大世界缓存清理", zap.Int64("deleted", n))
	}
}
