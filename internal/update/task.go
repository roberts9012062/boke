// internal/update/task.go
// 更新任务与进度状态文件：后端与宿主机更新代理（scripts/update-agent.sh）的文件通信协议。
//
// 约定（均位于数据目录，容器与宿主机经挂载卷共享）：
//   - update-task.json     后端写入的更新请求（代理处理后删除）
//   - update-status.json   代理写入的执行进度（阶段/百分比/结果），后端读取转发给前端
//   - app-version.txt      代理在部署完成时写入的当前版本
package update

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Task 更新请求（后端 → 代理）。
type Task struct {
	Version   string `json:"version"`    // 目标版本 tag（如 v1.3.0）
	CreatedAt string `json:"created_at"` // 创建时间（RFC3339）
}

// 状态常量（status.json 的 state 字段）。
const (
	StateIdle    = "idle"    // 无任务（文件不存在同义）
	StateRunning = "running" // 更新执行中
	StateDone    = "done"    // 更新完成
	StateFailed  = "failed"  // 更新失败（message 含原因）
)

// Status 更新执行进度（代理 → 后端 → 前端轮询）。
type Status struct {
	State     string `json:"state"`               // idle / running / done / failed
	Stage     string `json:"stage,omitempty"`     // 当前阶段说明（拉取代码/构建镜像/重启服务）
	Percent   int    `json:"percent"`             // 总进度百分比（0-100，阶段节点值）
	Version   string `json:"version,omitempty"`   // 目标版本
	Message   string `json:"message,omitempty"`   // 失败原因或完成说明
	UpdatedAt string `json:"updated_at"`          // 状态刷新时间（RFC3339）
}

// taskPath / statusPath 文件路径（数据目录下）。
func taskPath(dataDir string) string   { return filepath.Join(dataDir, "update-task.json") }
func statusPath(dataDir string) string { return filepath.Join(dataDir, "update-status.json") }

// ensureDir 确保数据目录存在。
func ensureDir(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("创建数据目录失败：%w", err)
	}
	return nil
}

// CreateTask 写入更新请求（已有未完成任务时返回错误，防止重复触发）。
func CreateTask(dataDir string, version string) error {
	if _, err := os.Stat(taskPath(dataDir)); err == nil {
		return fmt.Errorf("已有更新任务在队列中，请等待执行完成")
	}
	if err := ensureDir(dataDir); err != nil {
		return err
	}
	raw, err := json.Marshal(Task{Version: version, CreatedAt: time.Now().Format(time.RFC3339)})
	if err != nil {
		return err
	}
	return os.WriteFile(taskPath(dataDir), raw, 0o600)
}

// ReadTask 读取待处理任务；无任务第二个返回值为 false。
func ReadTask(dataDir string) (Task, bool, error) {
	raw, err := os.ReadFile(taskPath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return Task{}, false, nil
		}
		return Task{}, false, err
	}
	var task Task
	if err := json.Unmarshal(raw, &task); err != nil {
		return Task{}, false, fmt.Errorf("解析更新任务失败：%w", err)
	}
	return task, true, nil
}

// RemoveTask 删除任务文件（代理处理完毕后调用）。
func RemoveTask(dataDir string) error {
	err := os.Remove(taskPath(dataDir))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// WriteStatus 写入执行进度（代理调用；同时落盘失败不阻断流程的场景由调用方取舍）。
func WriteStatus(dataDir string, status Status) error {
	if err := ensureDir(dataDir); err != nil {
		return err
	}
	status.UpdatedAt = time.Now().Format(time.RFC3339)
	raw, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return os.WriteFile(statusPath(dataDir), raw, 0o600)
}

// ReadStatus 读取执行进度；无状态文件返回 idle 空状态。
func ReadStatus(dataDir string) Status {
	raw, err := os.ReadFile(statusPath(dataDir))
	if err != nil {
		return Status{State: StateIdle, Percent: 0}
	}
	var status Status
	if json.Unmarshal(raw, &status) != nil {
		return Status{State: StateIdle, Percent: 0}
	}
	return status
}

// ClearStatus 清理状态文件（新更新开始时重置旧进度用）。
func ClearStatus(dataDir string) {
	_ = os.Remove(statusPath(dataDir))
}
