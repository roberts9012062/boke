// internal/update/version.go
// 站点版本管理：当前版本读取（data/app-version.txt，更新代理写入）与语义化版本比较。
//
// 说明：当前部署版本由宿主机更新代理（scripts/update-agent.sh）在构建部署时写入
// data/app-version.txt（git tag 或 dev-<短SHA>）；文件缺失视为开发版（不提示更新）。
package update

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// VersionPath 当前版本文件路径（相对数据目录）。
func VersionPath(dataDir string) string {
	return dataDir + string(os.PathSeparator) + "app-version.txt"
}

// CurrentVersion 读取当前部署版本（文件缺失/为空返回 "dev"）。
func CurrentVersion(dataDir string) string {
	raw, err := os.ReadFile(VersionPath(dataDir))
	if err != nil {
		return "dev"
	}
	v := strings.TrimSpace(string(raw))
	if v == "" {
		return "dev"
	}
	return v
}

// parseSemver 解析 "v1.2.3" 为三段数字（纯函数；非 semver 格式返回 false）。
func parseSemver(tag string) ([3]int, bool) {
	tag = strings.TrimPrefix(strings.TrimSpace(tag), "v")
	parts := strings.Split(tag, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(strings.TrimSuffix(part, "-dev"))
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// IsSemver 判断 tag 是否为 "vX.Y.Z" 语义化版本（触发更新时的入参校验用）。
func IsSemver(tag string) bool {
	_, ok := parseSemver(tag)
	return ok
}

// IsNewer 判断 target 是否比 current 更新（纯函数）。
// 规则：双方均为 semver 时按段比较；current 非 semver（如 dev）时视为已是最新。
func IsNewer(target string, current string) bool {
	tv, okT := parseSemver(target)
	cv, okC := parseSemver(current)
	if !okT || !okC {
		return false
	}
	for i := 0; i < 3; i++ {
		if tv[i] != cv[i] {
			return tv[i] > cv[i]
		}
	}
	return false
}

// FormatVersion 展示格式化（补 v 前缀；纯函数）。
func FormatVersion(v string) string {
	if v == "" || v == "dev" {
		return "开发版"
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return fmt.Sprintf("v%s", v)
}
