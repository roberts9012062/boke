// cmd/bp/main.go
// .bpk 打包工具（M3.4）：插件作者将构建产物打包为 .bpk 安装包（zip 封装）。
//
// 用法：
//   go run ./cmd/bp pack \
//     -plugin yueyan-plugin.json \   # 插件清单（仓库根目录；读取 id/name/description/author/sdk）
//     -bin plugin.bin \              # 插件二进制（按平台构建产物）
//     -os windows -arch amd64 \      # 目标平台（GOOS/GOARCH 小写）
//     -version 0.1.0 \               # 版本（缺省取清单 version）
//     -out dist/                     # 输出目录（缺省当前目录）
//   # 输出：dist/{id}-{version}-{os}-{arch}.bpk
//
// 包结构：manifest.json + plugin.bin + [pubkey.pem] + [frontend/] + checksums.json
// 说明：与 docs/plugin-dev-guide.md 第 10 章 yueyan-bp 工具对齐；发布流程见开发手册。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/roberts9012062/boke/internal/plugin/bpkg"
)

// pluginManifest 插件仓库根目录清单（yueyan-plugin.json，作者侧；仅打包所需字段）。
type pluginManifest struct {
	ID          string `json:"id"`                    // 插件 ID
	Name        string `json:"name"`                  // 名称
	Version     string `json:"version"`               // 版本
	Description string `json:"description"`           // 描述
	SDK         string `json:"sdk"`                   // 兼容 SDK 范围
	Author      struct {
		Name string `json:"name"` // 作者名
	} `json:"author"`
}

// pack 打包子命令（单平台 .bpk；-pubkey 付费插件公钥入包，M3.5；-frontend 前端扩展资产入包，M3.6）。
func pack(pluginPath string, binPath string, pubkeyPath string, frontendDir string, goos string, goarch string, version string, outDir string) error {
	// 读取插件清单（id/name/version/author/description/sdk）
	raw, err := os.ReadFile(pluginPath)
	if err != nil {
		return fmt.Errorf("读取清单失败：%w", err)
	}
	var pm pluginManifest
	if err := json.Unmarshal(raw, &pm); err != nil {
		return fmt.Errorf("清单解析失败：%w", err)
	}
	if pm.ID == "" || pm.Name == "" {
		return fmt.Errorf("清单缺少必填字段（id/name）")
	}
	if version == "" {
		version = pm.Version
	}
	if version == "" {
		return fmt.Errorf("未指定版本（-version 或清单 version）")
	}

	// 读取插件二进制
	bin, err := os.ReadFile(binPath)
	if err != nil {
		return fmt.Errorf("读取插件二进制失败：%w", err)
	}
	files := map[string][]byte{bpkg.PluginBinName: bin}

	// 付费插件公钥入包（安装时登记，激活验签用；M3.5）
	if pubkeyPath != "" {
		pubkey, err := os.ReadFile(pubkeyPath)
		if err != nil {
			return fmt.Errorf("读取公钥失败：%w", err)
		}
		files[bpkg.PubkeyName] = pubkey
	}

	// 前端扩展资产入包（frontend/*，M3.6：扩展点声明 + ESM 模块 + 样式）
	if frontendDir != "" {
		if err := collectDir(files, frontendDir, "frontend"); err != nil {
			return err
		}
	}

	// 打包
	content, err := bpkg.Pack(bpkg.Manifest{
		ID: pm.ID, Name: pm.Name, Version: version,
		Author: pm.Author.Name, Description: pm.Description, SDK: pm.SDK,
	}, files)
	if err != nil {
		return err
	}

	// 输出 {id}-{version}-{os}-{arch}.bpk
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败：%w", err)
	}
	outPath := filepath.Join(outDir, fmt.Sprintf("%s-%s-%s-%s.bpk", pm.ID, version, goos, goarch))
	if err := os.WriteFile(outPath, content, 0o644); err != nil {
		return fmt.Errorf("写入安装包失败：%w", err)
	}
	extra := "免费插件"
	if pubkeyPath != "" {
		extra = "含许可证公钥"
	}
	if frontendDir != "" {
		extra += " + 前端扩展"
	}
	fmt.Printf("[成功] 打包完成：%s（%d 字节，%s）\n", outPath, len(content), extra)
	return nil
}

// collectDir 递归收集目录文件到包条目（prefix 为包内前缀，如 "frontend"）。
func collectDir(files map[string][]byte, dir string, prefix string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("读取前端资产目录失败：%w", err)
	}
	for _, entry := range entries {
		rel := filepath.Join(prefix, entry.Name())
		full := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := collectDir(files, full, rel); err != nil {
				return err
			}
			continue
		}
		content, err := os.ReadFile(full)
		if err != nil {
			return fmt.Errorf("读取前端资产失败（%s）：%w", full, err)
		}
		files[filepath.ToSlash(rel)] = content
	}
	return nil
}

func main() {
	// 子命令剥离（flag 包遇首个非 flag 参数即停止解析，需先取出 pack）
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "pack" {
		args = args[1:]
	}
	fs := flag.NewFlagSet("bp", flag.ExitOnError)
	pluginPath := fs.String("plugin", "yueyan-plugin.json", "插件清单路径（仓库根目录）")
	binPath := fs.String("bin", "plugin.bin", "插件二进制路径")
	pubkeyPath := fs.String("pubkey", "", "许可证公钥路径（付费插件必带，M3.5）")
	frontendDir := fs.String("frontend", "", "前端扩展资产目录（frontend/，M3.6）")
	goos := fs.String("os", runtime.GOOS, "目标平台 OS（linux/darwin/windows）")
	goarch := fs.String("arch", runtime.GOARCH, "目标平台架构（amd64/arm64）")
	version := fs.String("version", "", "版本号（缺省取清单 version）")
	outDir := fs.String("out", ".", "输出目录")
	if err := fs.Parse(args); err != nil {
		os.Exit(1)
	}
	if err := pack(*pluginPath, *binPath, *pubkeyPath, *frontendDir, *goos, *goarch, *version, *outDir); err != nil {
		fmt.Printf("[失败] %v\n", err)
		os.Exit(1)
	}
}
