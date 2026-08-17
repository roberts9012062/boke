// internal/plugin/bpkg/manifest.go
// .bpk 包内清单（M3.4）：manifest.json 定义与校验。
// 对齐 docs/architecture.md 6.5.6 + docs/plugin-dev-guide.md 10.1：
//   包内 manifest.json 与插件仓库 yueyan-plugin.json、代码 Info() 三处 id/version 必须一致。
package bpkg

import (
	"encoding/json"
	"fmt"
)

// Manifest 包内清单（.bpk 内 manifest.json）。
type Manifest struct {
	ID           string   `json:"id"`                    // 插件 ID（唯一，与安装实例一致）
	Name         string   `json:"name"`                  // 插件名称
	Version      string   `json:"version"`               // 版本号（与包文件名/Release tag 一致）
	Author       string   `json:"author,omitempty"`      // 作者
	Description  string   `json:"description,omitempty"` // 一句话描述
	SDK          string   `json:"sdk,omitempty"`         // 兼容 SDK 版本范围（如 ">=1.0.0"；空=不限制）
	Capabilities []string `json:"capabilities,omitempty"` // 能力声明（P0 加固：上传通道校验 + 安装落库供运行时门控取交集）
}

// ParseManifest 解析包内 manifest.json。
func ParseManifest(raw []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("manifest.json 解析失败：%w", err)
	}
	if m.ID == "" || m.Name == "" || m.Version == "" {
		return nil, fmt.Errorf("manifest.json 缺少必填字段（id/name/version）")
	}
	return &m, nil
}
