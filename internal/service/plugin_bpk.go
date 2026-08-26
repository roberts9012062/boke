// internal/service/plugin_bpk.go
// .bpk 安装（M3.4）：本地上传安装 + GitHub Release 资产下载安装。
// 对齐 docs/architecture.md 6.5.5：按平台匹配资产 → 下载 → 双重 SHA-256 校验
// （清单声明 + 包内 checksums 逐文件）→ 安全解包 → 实例注册 → 激活。
package service

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"
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

// InstallFromBPK 本地上传 .bpk 安装/升级（我的插件页「本地安装」；?upgrade=1 本地升级验证通道）。
// 参数：content .bpk 内容；repoURL 来源说明（可空）；upgrade 升级模式（跳过已安装冲突）。
// 流程：临时落盘 → 安全解包（checksums 校验 + zip-slip 防护）→ 二进制就位 → 实例注册 → 激活。
func (s *PluginService) InstallFromBPK(ctx context.Context, content []byte, repoURL string, upgrade bool) error {
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
	// 本地上传无市场条目可绑定（expectedID 空）；能力校验在解包后按包内 manifest 声明执行
	return s.installFromFile(ctx, tmpPath, repoURL, upgrade, "")
}

// installFromRelease 从 GitHub Release 下载安装（清单 assets 声明 + 当前平台匹配）。
// 流程：最新 Release → 资产名匹配 → 下载 → 清单声明 SHA-256 比对（第一重）→ 解包注册（第二重包内校验）。
func (s *PluginService) installFromRelease(ctx context.Context, info *PluginInfo, upgrade bool) error {
	if s.gh == nil {
		return errs.New(errs.CodePluginDownload, "GitHub 客户端未配置")
	}
	if s.store == nil {
		return errs.New(errs.CodePluginVerify, "插件存储未配置")
	}
	s.applyGHProxy(ctx) // 刷新代理设置（与商城拉取共用同一代理，国内网络安装加速）
	owner, repo, ok := parseRepoURL(info.RepoURL)
	if !ok {
		return errs.New(errs.CodePluginDownload, "插件来源仓库格式不正确")
	}
	// 版本钉扎拉取（P1 加固）：优先按清单声明版本拉对应 tag 的 Release——
	// 防清单过期时 latest 已被替换（或投毒）绕过清单审核；指定 tag 不存在回退 latest
	release, err := s.fetchPinnedRelease(ctx, owner, repo, info.Version)
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
	// 身份绑定（P0 加固）：下载包的 manifest ID 必须与管理员点击安装的市场条目一致
	return s.installFromFile(ctx, tmpPath, info.RepoURL, upgrade, info.ID)
}

// installFromFile 解包注册激活（上传与 Release 下载共用）。
// 流程：安全解包到临时目录 → manifest ID 绑定校验 → 目标目录替换 → 二进制就位 → 实例注册 → 激活。
// 参数：expectedID 期望的插件 ID（市场/升级路径传入清单 ID，空=本地上传无绑定）；
//      非空时 manifest.ID 不一致直接拒绝——防投毒包冒充其他插件身份静默替换（配合 ?upgrade=1）。
func (s *PluginService) installFromFile(ctx context.Context, bpkPath string, repoURL string, upgrade bool, expectedID string) error {
	tmpDir := bpkPath + ".d"
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return errs.New(errs.CodePluginVerify, "解包目录创建失败")
	}
	defer os.RemoveAll(tmpDir)

	// 读取并校验包内容（checksums 逐文件 + zip-slip + 大小上限；不落盘）
	manifest, entries, err := bpkg.Read(bpkPath)
	if err != nil {
		return errs.New(errs.CodePluginVerify, err.Error())
	}
	// 身份绑定（P0 加固）：包自报 ID 与期望 ID 不一致 → 拒绝（防身份冒充）
	if expectedID != "" && manifest.ID != expectedID {
		return errs.New(errs.CodePluginVerify,
			"插件包身份不符（包内声明 "+manifest.ID+"，期望 "+expectedID+"），拒绝安装")
	}
	// 上传通道能力校验（P0 加固）：本地上传无市场清单，按包内 manifest 声明校验未知能力
	// （空声明放行——运行时门控与登记能力取交集，未声明即无授权）
	if unknown := unknownCapabilities(manifest.Capabilities); len(unknown) > 0 {
		return errs.New(errs.CodeBadRequest,
			"插件包声明了未知能力："+strings.Join(unknown, "、")+
				"（支持："+strings.Join(knownCapabilitiesList(), "、")+"）")
	}
	// 包签名校验（P1 供应链加固）：配置了信任公钥则必须验签通过，落盘前拦截投毒包
	trusted, err := s.trustedSignKeys(ctx)
	if err != nil {
		return errs.New(errs.CodePluginVerify, "信任公钥配置无效（已拒绝安装）："+err.Error())
	}
	if len(trusted) > 0 {
		if err := bpkg.VerifySignature(entries, trusted); err != nil {
			return errs.New(errs.CodePluginVerify, "插件包签名校验失败："+err.Error())
		}
	}
	// 落盘（校验与验签全部通过后才写入目标目录）
	if err := bpkg.WriteEntries(entries, tmpDir); err != nil {
		return errs.New(errs.CodePluginVerify, err.Error())
	}
	// 插件 ID 合法性（白名单）与重复安装检查
	destDir := s.store.Dir(manifest.ID)
	if destDir == "" {
		return errs.New(errs.CodePluginVerify, "插件 ID 不合法")
	}
	existing, findErr := s.plugs.FindByPluginID(ctx, manifest.ID)
	// 升级模式跳过已安装冲突（先停用旧进程释放二进制占用；直接走记录复用分支替换版本）
	if !upgrade && findErr == nil && existing.State != PluginUninstalled {
		return errs.New(errs.CodeConflict, "插件「"+manifest.Name+"」已安装，请先卸载")
	}
	if upgrade {
		if err := s.deactivate(manifest.ID); err != nil {
			return errs.New(errs.CodeUpstream, err.Error())
		}
	}
	// 升级模式：先备份旧目录中的插件自建数据文件（登录态 state.json / 自有 settings.json
	// 等包内不提供的文件），目录替换后恢复——升级不该清空插件的用户数据
	var dataBackup []dataFileBackup
	if upgrade {
		binName := filepath.Base(s.store.BinPath(manifest.ID))
		dataBackup = backupPluginData(destDir, entries, binName)
	}
	// 目标目录原子替换（清空旧内容 → 移动解包结果）。
	// Windows 上刚 Kill 的插件进程 exe 句柄释放存在异步窗口，RemoveAll 偶发
	// 「文件被占用」——短退避重试消解（仍失败才报错）
	if err := removeAllRetry(destDir, 5); err != nil {
		return errs.New(errs.CodePluginVerify, "清理插件目录失败："+err.Error())
	}
	if err := os.Rename(tmpDir, destDir); err != nil {
		return errs.New(errs.CodePluginVerify, "插件文件就位失败")
	}
	// 二进制重命名：plugin.bin → plugin[.exe]（平台后缀，与 binstore 约定一致）
	if binPath := s.store.BinPath(manifest.ID); binPath != "" {
		_ = os.Rename(filepath.Join(destDir, bpkg.PluginBinName), binPath)
	}
	// 恢复升级前备份的自建数据文件（失败不阻断——数据文件缺失按未配置处理）
	for _, backup := range dataBackup {
		_ = os.Rename(backup.path, filepath.Join(destDir, backup.rel))
	}
	// 付费插件公钥登记（M3.5：包内 pubkey.pem → plugin_instances.pubkey_pem，激活验签用）
	if pubkey, err := os.ReadFile(filepath.Join(destDir, bpkg.PubkeyName)); err == nil && len(pubkey) > 0 {
		if err := s.plugs.SetPubkey(ctx, manifest.ID, string(pubkey)); err != nil {
			_ = err // 公钥登记失败不阻断安装（激活接口会提示未登记）
		}
	}

	// 实例注册（已卸载记录复用；否则新建）→ 激活（拉起进程/注册钩子）
	// capabilities 登记取包内 manifest 声明（P2 加固：运行时门控与二进制自报取交集）
	if findErr == nil {
		if err := s.plugs.Reinstall(ctx, existing.ID, manifest.Name, manifest.Version, repoURL, manifest.Capabilities); err != nil {
			return fmt.Errorf("重新安装插件失败：%w", err)
		}
		s.activateInstalled(ctx, existing.ID, manifest.ID)
		return nil
	}
	if !errors.Is(findErr, repository.ErrNotFound) {
		return findErr
	}
	instanceID, err := s.plugs.Create(ctx, repository.PluginInstance{
		PluginID:     manifest.ID,
		Name:         manifest.Name,
		Version:      manifest.Version,
		RepoURL:      repoURL,
		State:        PluginInstalled,
		Capabilities: manifest.Capabilities,
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

// fetchPinnedRelease 版本钉扎拉取 Release（version 非空优先按 v{version} tag；
// tag 不存在/拉取失败回退 latest，保持可用性——清单未更新时仍可安装最新版）。
func (s *PluginService) fetchPinnedRelease(ctx context.Context, owner string, repo string, version string) (*ghclient.LatestRelease, error) {
	tag := "v" + strings.TrimPrefix(strings.TrimSpace(version), "v")
	if tag != "v" {
		if release, err := s.gh.FetchReleaseByTag(ctx, owner, repo, tag); err == nil {
			return release, nil
		}
	}
	return s.gh.FetchLatestRelease(ctx, owner, repo)
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

// ---------- 升级（架构 6.5.5 第 5 步：检测新 tag → 一键升级） ----------

// pluginPkgPubkeysSetting 包签名信任公钥设置键（后台设置写入；PEM 文本可含多个 PUBLIC KEY 块）。
// 策略：配置后强制验签（无签名/验签失败拒绝安装）；未配置放行无签名包（兼容本地开发）。
const pluginPkgPubkeysSetting = "plugin_pkg_pubkeys"

// trustedSignKeys 解析信任公钥配置（空/未配置返回空列表；配置存在但解析失败返回错误）。
// 说明：解析失败按「已配置但无效」处理——拒绝安装而非降级放行，防坏配置静默关闭验签。
func (s *PluginService) trustedSignKeys(ctx context.Context) ([]ed25519.PublicKey, error) {
	if s.settings == nil {
		return nil, nil
	}
	raw, ok, err := s.settings.Get(ctx, pluginPkgPubkeysSetting)
	if err != nil {
		return nil, err
	}
	if !ok || strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	return bpkg.ParsePublicKeysPEM(raw)
}

// 默认资产名模式（清单未声明 assets 时的兜底）。
const defaultBPKPattern = "{id}-{version}-{os}-{arch}.bpk"

// PluginUpdateDTO 可更新插件项（我的插件页「可更新」徽章）。
type PluginUpdateDTO struct {
	InstanceID    int64  `json:"instance_id"`    // 实例 ID
	PluginID      string `json:"plugin_id"`      // 插件 ID
	Name          string `json:"name"`           // 插件名称
	CurrentVersion string `json:"current_version"` // 当前版本
	LatestVersion string `json:"latest_version"` // 最新版本（Release tag 去 v）
}

// CheckUpdates 检查可更新插件（优先市场清单每插件版本对比；清单不可用回退 Release latest）。
// 说明：发布脚本对主仓库统一发 v{version} tag（tag 空间共享），Release latest 是
// 全局最高版本而非各插件自己的——一刀切对比会让全部旧版本插件集体误报「可更新
// 至 {latest}」（历史 bug：seo 发 v1.2.0 后全站插件都提示 1.2.0）。市场清单
// per-plugin version 才是各自的版本通道（5 分钟缓存），Release 仅作清单缺失的兜底。
func (s *PluginService) CheckUpdates(ctx context.Context) ([]PluginUpdateDTO, error) {
	s.applyGHProxy(ctx) // 刷新代理设置（清单/Release 拉取经代理加速）
	installed, err := s.plugs.ListInstalled(ctx)
	if err != nil {
		return nil, err
	}
	// 市场清单版本表（拉取失败置空——逐插件回退 Release latest）
	manifestVersions := make(map[string]string)
	if manifest, err := s.fetchManifest(ctx, ""); err == nil && manifest != nil {
		for _, info := range manifest.Plugins {
			manifestVersions[info.ID] = strings.TrimSpace(info.Version)
		}
	}
	updates := make([]PluginUpdateDTO, 0)
	for _, inst := range installed {
		// 仅进程外插件（有分发能力）
		if !s.isProcessPlugin(inst.PluginID) {
			continue
		}
		latest := manifestVersions[inst.PluginID]
		if latest == "" {
			// 清单缺失该插件：回退 Release latest（无 Release/失败跳过）
			if inst.RepoURL == "" {
				continue
			}
			owner, repo, ok := parseRepoURL(inst.RepoURL)
			if !ok || s.gh == nil {
				continue
			}
			release, err := s.gh.FetchLatestRelease(ctx, owner, repo)
			if err != nil || len(release.Assets) == 0 {
				continue
			}
			latest = strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
		}
		if versionLess(inst.Version, latest) {
			updates = append(updates, PluginUpdateDTO{
				InstanceID: inst.ID, PluginID: inst.PluginID, Name: inst.Name,
				CurrentVersion: inst.Version, LatestVersion: latest,
			})
		}
	}
	return updates, nil
}

// UpdatePlugin 一键升级插件（解析目标版本 → 停用 → 下载新包 → 校验解包替换 → 更新版本 → 启用）。
// 目标版本取 **GitHub Release latest**（与 CheckUpdates 同源——修复历史矛盾：此前用市场清单
// version 作目标，与「可更新」检查的 Release tag 不一致时出现「提示可更新却报已最新」）。
// 顺序约束（修复历史 bug）：**先解析升级目标再停用**——此前先停用后拉清单，目标解析失败时
// 插件已被停用、恢复再遇网络抖动即陷入崩溃循环（用户点升级后插件崩的根因）。
// 恢复失败落库 crashed（此前吞错，DB 显示 running 而进程实际不在——状态假象）。
func (s *PluginService) UpdatePlugin(ctx context.Context, instanceID int64) error {
	s.applyGHProxy(ctx) // 刷新代理设置（升级拉取走 Release API，同样经代理加速）
	inst, err := s.plugs.FindByID(ctx, instanceID)
	if err != nil {
		return errs.ErrNotFound
	}
	if !s.isProcessPlugin(inst.PluginID) {
		return errs.New(errs.CodeBadRequest, "内置插件无需升级")
	}
	if s.gh == nil {
		return errs.New(errs.CodePluginDownload, "GitHub 客户端未配置，无法在线升级")
	}
	// ---------- 解析升级目标（与 CheckUpdates 同源：市场清单是分发事实源——
	// 目标版本/来源仓库/资产名模式均优先取清单，实例记录值仅兜底；失败快速返回不动运行中插件）。
	// 背景：清单 repo_url 迁移（如 boke → yueyan-plugins）后，历史安装实例的 RepoURL
	// 仍是旧仓库——按旧仓库找新版本 Release 必然「未找到资产」（2026-08 实测）。
	target := ""
	pattern := defaultBPKPattern
	sourceURL := strings.TrimSpace(inst.RepoURL)
	if manifest, err := s.fetchManifest(ctx, ""); err == nil && manifest != nil {
		for i := range manifest.Plugins {
			info := manifest.Plugins[i]
			if info.ID != inst.PluginID {
				continue
			}
			target = strings.TrimSpace(info.Version)
			if strings.TrimSpace(info.RepoURL) != "" {
				sourceURL = strings.TrimSpace(info.RepoURL)
			}
			if info.Assets != nil && info.Assets.Pattern != "" {
				pattern = info.Assets.Pattern
			}
			break
		}
	}
	if sourceURL == "" {
		return errs.New(errs.CodeBadRequest, "插件无来源仓库（清单与实例记录均缺失），请用本地安装 .bpk 升级")
	}
	owner, repo, ok := parseRepoURL(sourceURL)
	if !ok {
		return errs.New(errs.CodePluginDownload, "插件来源仓库格式不正确")
	}
	if target == "" {
		release, err := s.gh.FetchLatestRelease(ctx, owner, repo)
		if err != nil || len(release.Assets) == 0 {
			return errs.New(errs.CodePluginDownload, "未找到可用的升级 Release（网络或仓库无发行版），插件保持运行未受影响")
		}
		target = strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	}
	if !versionLess(inst.Version, target) {
		return errs.New(errs.CodeBadRequest, "已是最新版本 v"+inst.Version)
	}
	// 钉扎拉取目标版本的 Release（v{target} tag；各插件 tag 空间共享，资产名按插件 ID 区分）
	release, err := s.fetchPinnedRelease(ctx, owner, repo, target)
	if err != nil || len(release.Assets) == 0 {
		return errs.New(errs.CodePluginDownload, "未找到 v"+target+" 的升级 Release（网络或仓库无发行版），插件保持运行未受影响")
	}
	info := &PluginInfo{
		ID: inst.PluginID, Name: inst.Name, RepoURL: sourceURL, Version: target,
		Assets: &PluginAssets{Pattern: pattern},
	}

	// ---------- 停用（停进程 + 注销钩子；未运行幂等）——installFromFile 会原子替换二进制目录 ----------
	if err := s.deactivate(inst.PluginID); err != nil {
		return errs.New(errs.CodeUpstream, err.Error())
	}

	// ---------- 下载安装（升级模式：跳过已安装冲突，复用记录更新版本 + 激活） ----------
	if err := s.installFromRelease(ctx, info, true); err != nil {
		// 升级失败：尝试恢复原版本运行（目录未被替换时原版本完好）
		if actErr := s.activate(ctx, inst.PluginID); actErr != nil {
			// 恢复失败落库 crashed（消除「DB 显示 running 而进程不在」的状态假象）
			msg := fmt.Sprintf("升级失败（%s），且恢复原版本失败：%s", err.Error(), actErr.Error())
			_ = s.plugs.SetStateByPluginID(ctx, inst.PluginID, PluginCrashed, msg)
		}
		return err
	}
	return nil
}

// ---------- 升级数据迁移（插件自建文件不随目录替换丢失） ----------

// dataFileBackup 数据文件备份项（临时路径 + 原相对路径）。
type dataFileBackup struct {
	path string // 备份临时文件绝对路径
	rel  string // 插件目录内相对路径（恢复目标）
}

// backupPluginData 备份旧插件目录中的自建数据文件（包内不提供且非二进制的文件）。
// 说明：升级走「整目录替换」，state.json（登录态）/settings.json（自有配置）等
// 插件运行期写入的用户数据不在包内——不迁移会随替换丢失（历史 bug：扫码登录
// 成功后升级，登录态消失）。备份到系统临时目录，由调用方在替换后恢复。
func backupPluginData(dir string, packed map[string][]byte, binName string) []dataFileBackup {
	var backups []dataFileBackup
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if _, isPacked := packed[relSlash]; isPacked {
			return nil // 新包自带：以新包为准
		}
		if rel == binName {
			return nil // 旧二进制：新包重命名就位，不迁移
		}
		// 备份临时文件放插件目录同级（同盘）——恢复用 os.Rename，跨盘必失败
		tmp, cpErr := os.CreateTemp(filepath.Dir(dir), ".plugin-data-*")
		if cpErr != nil {
			return nil
		}
		tmpPath := tmp.Name()
		src, openErr := os.Open(path)
		if openErr != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			return nil
		}
		_, _ = io.Copy(tmp, src)
		_ = src.Close()
		_ = tmp.Close()
		backups = append(backups, dataFileBackup{path: tmpPath, rel: rel})
		return nil
	})
	return backups
}

// removeAllRetry 带短退避重试的目录删除（Windows 进程句柄释放窗口的实用消解）。
func removeAllRetry(dir string, attempts int) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return err
}
