// internal/service/plugin_bpk.go
// .bpk 安装（M3.4）：本地上传安装 + GitHub Release 资产下载安装。
// 对齐 docs/architecture.md 6.5.5：按平台匹配资产 → 下载 → 双重 SHA-256 校验
// （清单声明 + 包内 checksums 逐文件）→ 安全解包 → 实例注册 → 激活。
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/roberts9012062/boke/internal/ghclient"
	"github.com/roberts9012062/boke/internal/plugin/bpkg"
	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// .bpk 上传大小上限（50MB；与 handler 侧校验一致）。
const maxBPKSize = 50 << 20

// InstallFromBPK 本地上传 .bpk 安装（我的插件页「本地安装」）。
// 参数：content .bpk 内容；repoURL 来源说明（可空）。
// 流程：临时落盘 → 安全解包（checksums 校验 + zip-slip 防护）→ 二进制就位 → 实例注册 → 激活。
func (s *PluginService) InstallFromBPK(ctx context.Context, content []byte, repoURL string) error {
	if s.store == nil {
		return errs.New(errs.CodePluginVerify, "插件存储未配置")
	}
	if len(content) == 0 {
		return errs.New(errs.CodeBadRequest, "插件包为空")
	}
	if len(content) > maxBPKSize {
		return errs.New(errs.CodeBadRequest, fmt.Sprintf("插件包超过大小上限（%dMB）", maxBPKSize>>20))
	}
	// 临时落盘（校验失败/安装完成即清理）
	tmpPath := s.store.TempPath()
	if err := os.WriteFile(tmpPath, content, 0o644); err != nil {
		return errs.New(errs.CodePluginVerify, "插件包暂存失败")
	}
	defer os.Remove(tmpPath)
	return s.installFromFile(ctx, tmpPath, repoURL)
}

// installFromRelease 从 GitHub Release 下载安装（清单 assets 声明 + 当前平台匹配）。
// 流程：最新 Release → 资产名匹配 → 下载 → 清单声明 SHA-256 比对（第一重）→ 解包注册（第二重包内校验）。
func (s *PluginService) installFromRelease(ctx context.Context, info *PluginInfo) error {
	if s.gh == nil {
		return errs.New(errs.CodePluginDownload, "GitHub 客户端未配置")
	}
	if s.store == nil {
		return errs.New(errs.CodePluginVerify, "插件存储未配置")
	}
	owner, repo, ok := parseRepoURL(info.RepoURL)
	if !ok {
		return errs.New(errs.CodePluginDownload, "插件来源仓库格式不正确")
	}
	// 最新 Release（tag 取版本；无 Release 报错）
	release, err := s.gh.FetchLatestRelease(ctx, owner, repo)
	if err != nil {
		return errs.New(errs.CodePluginDownload, err.Error())
	}
	// 资产名匹配（{id}-{version}-{os}-{arch}.bpk，{version} 取 tag 去 v 前缀）
	expect := assetName(info, release.TagName)
	var asset *ghclient.ReleaseAsset
	for i := range release.Assets {
		if release.Assets[i].Name == expect {
			asset = &release.Assets[i]
			break
		}
	}
	if asset == nil {
		return errs.New(errs.CodePluginDownload, "Release 中未找到资产 "+expect)
	}
	// 下载到临时文件
	tmpPath := s.store.TempPath()
	defer os.Remove(tmpPath)
	if err := s.gh.DownloadAsset(ctx, asset.URL, tmpPath, maxBPKSize); err != nil {
		return errs.New(errs.CodePluginDownload, err.Error())
	}
	// 第一重校验：清单声明 SHA-256（有声明才比对；包内 checksums 为第二重）
	if info.Assets != nil && info.Assets.SHA256 != "" {
		if !strings.EqualFold(fileSHA256(tmpPath), info.Assets.SHA256) {
			return errs.New(errs.CodePluginVerify, "插件包校验失败（SHA-256 与清单声明不符）")
		}
	}
	return s.installFromFile(ctx, tmpPath, info.RepoURL)
}

// installFromFile 解包注册激活（上传与 Release 下载共用）。
// 流程：安全解包到临时目录 → manifest ID 校验 → 目标目录替换 → 二进制就位 → 实例注册 → 激活。
func (s *PluginService) installFromFile(ctx context.Context, bpkPath string, repoURL string) error {
	tmpDir := bpkPath + ".d"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return errs.New(errs.CodePluginVerify, "解包目录创建失败")
	}
	defer os.RemoveAll(tmpDir)

	// 安全解包（checksums 逐文件校验 + zip-slip 防护；校验失败不落盘）
	manifest, err := bpkg.Unpack(bpkPath, tmpDir)
	if err != nil {
		return errs.New(errs.CodePluginVerify, err.Error())
	}
	// 插件 ID 合法性（白名单）与重复安装检查
	destDir := s.store.Dir(manifest.ID)
	if destDir == "" {
		return errs.New(errs.CodePluginVerify, "插件 ID 不合法")
	}
	existing, findErr := s.plugs.FindByPluginID(ctx, manifest.ID)
	if findErr == nil && existing.State != PluginUninstalled {
		return errs.New(errs.CodeConflict, "插件「"+manifest.Name+"」已安装，请先卸载")
	}
	// 目标目录原子替换（清空旧内容 → 移动解包结果）
	if err := os.RemoveAll(destDir); err != nil {
		return errs.New(errs.CodePluginVerify, "清理插件目录失败")
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		return errs.New(errs.CodePluginVerify, "插件文件就位失败")
	}
	// 二进制重命名：plugin.bin → plugin[.exe]（平台后缀，与 binstore 约定一致）
	if binPath := s.store.BinPath(manifest.ID); binPath != "" {
		_ = os.Rename(filepath.Join(destDir, bpkg.PluginBinName), binPath)
	}

	// 实例注册（已卸载记录复用；否则新建）→ 激活（拉起进程/注册钩子）
	if findErr == nil {
		if err := s.plugs.Reinstall(ctx, existing.ID, manifest.Version, repoURL); err != nil {
			return fmt.Errorf("重新安装插件失败：%w", err)
		}
		s.activateInstalled(ctx, existing.ID, manifest.ID)
		return nil
	}
	if !errors.Is(findErr, repository.ErrNotFound) {
		return findErr
	}
	instanceID, err := s.plugs.Create(ctx, repository.PluginInstance{
		PluginID: manifest.ID,
		Name:     manifest.Name,
		Version:  manifest.Version,
		RepoURL:  repoURL,
		State:    PluginInstalled,
	})
	if err != nil {
		return fmt.Errorf("安装插件失败：%w", err)
	}
	s.activateInstalled(ctx, instanceID, manifest.ID)
	return nil
}

// assetName 按清单资产模式生成目标资产名（{id}/{version}/{os}/{arch} 替换；tag 去 v 前缀）。
func assetName(info *PluginInfo, tag string) string {
	pattern := info.Assets.Pattern
	version := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	pattern = strings.ReplaceAll(pattern, "{id}", info.ID)
	pattern = strings.ReplaceAll(pattern, "{version}", version)
	pattern = strings.ReplaceAll(pattern, "{os}", runtime.GOOS)
	pattern = strings.ReplaceAll(pattern, "{arch}", runtime.GOARCH)
	return pattern
}

// parseRepoURL 解析仓库 URL → owner/repo（支持 https://github.com/owner/repo 形式）。
func parseRepoURL(repoURL string) (string, string, bool) {
	trimmed := strings.TrimSuffix(strings.TrimSpace(repoURL), "/")
	const prefix = "https://github.com/"
	if !strings.HasPrefix(trimmed, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(trimmed, prefix), "/")
	if len(parts) < 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// fileSHA256 计算文件 SHA-256（hex 小写；读失败返回空串）。
func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}
