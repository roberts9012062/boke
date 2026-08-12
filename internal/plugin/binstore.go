// internal/plugin/binstore.go
// 插件二进制存储（M3.3）：data/plugins/{id}/ 目录管理。
// 说明（M3.4）：目录同时作为 .bpk 解包落点（Dir）与临时文件区（TempPath）；
//       pluginID 白名单校验防路径注入（与 backup safePath 同思路）。
package plugin

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

// 插件数据子目录（相对数据目录；bin 存 {dataDir}/plugins/{id}/）。
const pluginDataRoot = "plugins"

// 插件 ID 白名单（小写字母/数字/中划线；防止清单恶意 id 做路径注入）。
var pluginIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,99}$`)

// binSuffix 插件二进制后缀（Windows .exe，其余平台空）。
var binSuffix = func() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}()

// BinStore 插件二进制存取（连接器类）。
type BinStore struct {
	root string // 根目录（绝对路径）
}

// NewBinStore 创建二进制存储（dataDir 为数据目录，如 "data"）。
func NewBinStore(dataDir string) *BinStore {
	return &BinStore{root: filepath.Join(dataDir, pluginDataRoot)}
}

// BinPath 返回插件二进制路径（校验 ID 合法；不存在不报错）。
func (s *BinStore) BinPath(pluginID string) string {
	if !pluginIDPattern.MatchString(pluginID) {
		return ""
	}
	return filepath.Join(s.root, pluginID, "plugin"+binSuffix)
}

// Dir 返回插件目录（.bpk 解包落点；ID 不合法返回空串）。
func (s *BinStore) Dir(pluginID string) string {
	if !pluginIDPattern.MatchString(pluginID) {
		return ""
	}
	return filepath.Join(s.root, pluginID)
}

// TempPath 生成临时文件路径（data/plugins/tmp/{random}；安装包暂存/下载用，自动建目录）。
func (s *BinStore) TempPath() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	dir := filepath.Join(s.root, "tmp")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, hex.EncodeToString(buf))
}

// Exists 插件二进制是否存在（启用前置校验）。
func (s *BinStore) Exists(pluginID string) bool {
	path := s.BinPath(pluginID)
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
