// 中继站（大世界）对接的领域类型：配置、缓存条目、协议信封与负载。
// 契约以 Relay Station docs/02-协议规范.md v1.2 为准。
package model

import (
	"encoding/json"
	"time"
)

// 中继站信封类型常量（协议 §3.1）。
const (
	RelayEventPublish       = "content.publish"
	RelayEventUpdate        = "content.update"
	RelayEventDelete        = "content.delete"
	RelayEventCommentCreate = "comment.create"
	RelayEventCommentDelete = "comment.delete"
	RelayEventConfigUpdate  = "config.update"
)

// RelayConfig 中继站对接配置（单行表 relay_config）。
type RelayConfig struct {
	Enabled            bool       `json:"enabled"`              // 「大世界」开关
	URL                string     `json:"url"`                  // 中继站基础 URL
	SiteKey            string     `json:"site_key"`             // 站点 key
	Mode               string     `json:"mode"`                 // public / bridged
	DefaultCategory    string     `json:"default_category"`     // 发布默认分类
	LocalRetentionDays int        `json:"local_retention_days"` // 本地缓存天数（1~30）
	RelayMetaJSON      *string    `json:"relay_meta_json"`      // 握手元信息快照
	LastSeq            int64      `json:"last_seq"`             // 订阅游标
	UpdatedAt          time.Time  `json:"updated_at"`
}

// RelayHandshakeMeta 握手元信息（handshake 响应缓存）。
type RelayHandshakeMeta struct {
	Name          string   `json:"name"`
	RulesMD       string   `json:"rules_md"`
	MaxSites      int      `json:"max_sites"`
	SiteCount     int      `json:"site_count"`
	RetentionDays int      `json:"retention_days"`
	Categories    []string `json:"categories"`
}

// RelayHandshakeResp handshake 响应（连接测试回显）。
type RelayHandshakeResp struct {
	ProtoVer   int                `json:"proto_ver"`
	Meta       RelayHandshakeMeta `json:"meta"`
	Quota      RelayQuota         `json:"quota"`
	ServerTime int64              `json:"server_time"`
}

// RelayApplyResp POST /apply 响应（协议 v1.3：自助申请自动领取 key）。
type RelayApplyResp struct {
	SiteID     int64    `json:"site_id"`
	SiteKey    string   `json:"site_key"`
	RelayName  string   `json:"relay_name"`
	MaxSites   int      `json:"max_sites"`
	SiteCount  int      `json:"site_count"`
	Categories []string `json:"categories"`
}

// RelayQuota 握手响应的配额段。
type RelayQuota struct {
	DailyMoments  int              `json:"daily_moments"`
	DailyArticles int              `json:"daily_articles"`
	Media         RelayMediaQuota  `json:"media"`
}

// RelayMediaQuota 媒体配额。
type RelayMediaQuota struct {
	PerItemBytes int64    `json:"per_item_bytes"`
	DailyItems   int      `json:"daily_items"`
	DailyBytes   int64    `json:"daily_bytes"`
	Formats      []string `json:"formats"`
}

// RelayEnvelope 中继站分发信封（WS 帧与轮询响应共用，协议 §3）。
type RelayEnvelope struct {
	ProtoVer int             `json:"proto_ver"`
	Seq      int64           `json:"seq"`
	Type     string          `json:"type"`
	TS       int64           `json:"ts"`
	Data     json.RawMessage `json:"data"`
}

// RelayContentPayload content.publish / update 的 data（协议 §3.2）。
type RelayContentPayload struct {
	ContentID   string              `json:"content_id"`
	Site        RelaySiteBrief      `json:"site"`
	Kind        string              `json:"kind"` // moment / article
	Category    string              `json:"category"`
	Tags        []string            `json:"tags"`
	PublishedAt int64               `json:"published_at"`
	Moment      *RelayMomentPayload `json:"moment,omitempty"`
	Article     *RelayArticlePayload `json:"article,omitempty"`
}

// RelaySiteBrief 来源站概要。
type RelaySiteBrief struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Avatar string `json:"avatar"`
	Mode   string `json:"mode"`
}

// RelayMomentPayload 说说负载（图片为绝对 URL；视频音频仅 URL）。
type RelayMomentPayload struct {
	Text   string   `json:"text"`
	Images []string `json:"images"`
	Videos []string `json:"videos,omitempty"`
	Audios []string `json:"audios,omitempty"`
}

// RelayArticlePayload 文章摘要负载（全文不进信封）。
type RelayArticlePayload struct {
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	Cover     string `json:"cover"`
	OriginURL string `json:"origin_url"`
	ReadURL   string `json:"read_url,omitempty"`
}

// RelayDeleteData content.delete 的 data。
type RelayDeleteData struct {
	ContentID string `json:"content_id"`
	Reason    string `json:"reason"`
}

// RelayCacheItem 大世界缓存条目（前台卡片视图）。
type RelayCacheItem struct {
	ContentID   string          `json:"content_id"`
	Payload     RelayContentPayload `json:"payload"`
	PublishedAt time.Time       `json:"published_at"`
}
