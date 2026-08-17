// internal/plugin/manager.go
// 插件进程管理器（M3.3 核心）：go-plugin 子进程生命周期管理。
// 对齐 docs/architecture.md 6.4：
//   - 启用：拉起子进程 → 握手（AutoMTLS gRPC）→ Info 校验 → Activate → 注册钩子适配器
//   - 崩溃自愈：退避重启 1s→2s→…60s 上限；连续 5 次熔断（crashed，事件回调落库）
//   - 优雅退出：Deactivate（10s 超时）→ Kill；主进程退出时清理全部子进程
//   - 故障隔离：钩子调用超时/失败由 Registry（2s + panic 恢复）兜底，业务零侵入
package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	goplugin "github.com/hashicorp/go-plugin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/pkg/plugin-sdk/proto"
)

// 进程管理常量（对齐架构文档：退避 1s 起、60s 上限、连续 5 次熔断）。
const (
	stateRunning        = "running"  // 进程运行中
	stateStarting       = "starting" // 启动中（E3：握手阶段已移出全局锁，占位防并发启动）
	stateStopped        = "stopped"  // 已停止（主动）
	stateCrashed        = "crashed"  // 熔断（连续崩溃达上限）
	maxRestarts         = 5          // 连续崩溃熔断阈值
	handshakeTimeout    = 15 * time.Second // 握手超时（go-plugin ClientConfig.Timeout）
	deactivateTimeout   = 10 * time.Second // Deactivate 优雅停用超时
	pluginLogDir        = "logs/plugins"   // 插件 stderr 日志目录
)

// backoffDuration 崩溃退避时长（第 n 次：2^(n-1) 秒，上限 60 秒）。
func backoffDuration(attempt int) time.Duration {
	delay := time.Duration(1<<(attempt-1)) * time.Second
	if delay > 60*time.Second {
		return 60 * time.Second
	}
	return delay
}

// ManagerEvents 管理器事件回调（装配方实现：写 plugin_instances 状态）。
type ManagerEvents interface {
	// OnCrashed 插件熔断（连续崩溃达上限，不再自动重启；落库 crashed + last_error）。
	OnCrashed(pluginID string, err string)
	// OnRestarting 崩溃退避重启通知（attempt 为第几次重试；可记录日志）。
	OnRestarting(pluginID string, attempt int)
	// OnRecovered 退避重启成功（插件已恢复运行；应清除退避期间写入的 last_error）。
	OnRecovered(pluginID string)
}

// ManagedPlugin 单个插件进程管理项（受 PluginManager.mu 保护）。
type ManagedPlugin struct {
	pluginID  string        // 插件 ID
	binPath   string        // 二进制路径
	state     string        // running / stopped / crashed
	client    *goplugin.Client // go-plugin 客户端（持有子进程）
	rpc       *pluginClient // gRPC 三服务客户端
	logFile   *os.File      // 插件 stderr 日志文件（进程退出时关闭）
}

// PluginManager 插件进程管理器（连接器类）。
type PluginManager struct {
	mu                 sync.Mutex
	managed            map[string]*ManagedPlugin // pluginID → 管理项
	crashCounts        map[string]int            // pluginID → 连续崩溃次数（跨重启保留）
	store              *BinStore                 // 二进制存储（校验存在）
	registry           *Registry                 // 钩子注册表（注册/注销适配器）
	events             ManagerEvents             // 状态事件回调（写 DB）
	licenseProvider    LicenseProvider           // 许可证查询（M3.5；可空=全部 free）
	configProvider     ConfigProvider            // 配置查询（M3.7；可空=无配置）
	capabilityProvider CapabilityProvider        // 登记能力查询（P2 加固；可空=不交集，仅自报）
	dataProviderFn     func() DataProvider       // 只读数据服务工厂（M3.8；延迟绑定——装配完成后返回非 nil）
	logDir             string                    // 插件日志目录（logs/plugins）
	loggerMu           sync.RWMutex              // 保护 logger（装配后 SetLogger 注入）
	logger             *zap.Logger               // 结构化日志（E4；nil 时退回 stderr 输出）
}

// NewPluginManager 创建进程管理器。
// 参数：store 二进制存储；registry 钩子注册表；events 状态回调（可空）；logDir 插件日志目录；
//      licenseProvider 许可证查询（M3.5，可空=全部 demo）；configProvider 配置查询（M3.7，可空=无配置）；
//      capabilityProvider 登记能力查询（P2 加固，可空=能力门控退化为仅二进制自报）；
//      dataProviderFn 只读数据服务工厂（M3.8，每次拉起插件时调用取当前值；返回 nil=插件无数据访问能力）。
func NewPluginManager(store *BinStore, registry *Registry, events ManagerEvents, logDir string, licenseProvider LicenseProvider, configProvider ConfigProvider, capabilityProvider CapabilityProvider, dataProviderFn func() DataProvider) *PluginManager {
	return &PluginManager{
		managed:            make(map[string]*ManagedPlugin),
		crashCounts:        make(map[string]int),
		store:              store,
		registry:           registry,
		events:             events,
		licenseProvider:    licenseProvider,
		configProvider:     configProvider,
		capabilityProvider: capabilityProvider,
		dataProviderFn:     dataProviderFn,
		logDir:             logDir,
	}
}


// Stop 停用插件：Deactivate（超时保护）→ Kill 进程（已停止幂等返回）。
func (m *PluginManager) Stop(pluginID string) error {
	m.mu.Lock()
	mp, ok := m.managed[pluginID]
	if !ok || mp.state != stateRunning {
		starting := ok && mp.state == stateStarting
		m.mu.Unlock()
		if starting {
			return fmt.Errorf("插件「%s」正在启动中，请稍后停用", pluginID)
		}
		return nil // 未运行
	}
	mp.state = stateStopped // 防崩溃自愈误重启
	client, rpc, logFile := mp.client, mp.rpc, mp.logFile
	delete(m.managed, pluginID)
	m.mu.Unlock()

	// 注销钩子适配器（精确按插件 ID）
	m.unregisterAdapters(pluginID)

	// 优雅停用（超时保护，失败不阻塞杀进程）
	ctx, cancel := context.WithTimeout(context.Background(), deactivateTimeout)
	if _, err := rpc.info.Deactivate(ctx, &proto.Empty{}); err != nil {
		_ = err // 停用失败不阻断（记录可后续优化）
	}
	cancel()
	client.Kill()
	if logFile != nil {
		_ = logFile.Close()
	}
	return nil
}

// IsRunning 插件进程是否在运行。
func (m *PluginManager) IsRunning(pluginID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	mp, ok := m.managed[pluginID]
	return ok && mp.state == stateRunning
}

// Shutdown 停用全部插件进程（主进程退出时调用）。
func (m *PluginManager) Shutdown() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.managed))
	for id, mp := range m.managed {
		// starting 项跳过：会话尚不可安全终止，子进程依赖 go-plugin stdin 关闭机制随主进程退出
		if mp.state == stateRunning {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Stop(id)
	}
}

// watchExit 子进程退出监听（轮询 Exited，500ms 粒度）：主动停止忽略；崩溃则退避重启。
func (m *PluginManager) watchExit(mp *ManagedPlugin) {
	// 轮询进程退出（go-plugin v1.8 无 ExitChan，Exited 为进程状态检查）
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		if mp.client.Exited() {
			break
		}
	}

	m.mu.Lock()
	// 已被替换/移除/主动停止 → 忽略
	if m.managed[mp.pluginID] != mp || mp.state != stateRunning {
		m.logInfo("插件退出被忽略（管理项已变更）", zap.String("plugin", mp.pluginID), zap.String("state", mp.state))
		m.mu.Unlock()
		return
	}
	// 崩溃：先移除管理项（重建由 Start 负责），计数跨重启保留
	delete(m.managed, mp.pluginID)
	m.crashCounts[mp.pluginID]++
	attempt := m.crashCounts[mp.pluginID]
	m.logWarn("插件进程退出", zap.String("plugin", mp.pluginID), zap.Int("crash_count", attempt), zap.Int("max_restarts", maxRestarts))
	m.mu.Unlock()

	if mp.logFile != nil {
		_ = mp.logFile.Close()
	}

	// 熔断：连续崩溃达上限，不再自动重启
	if attempt >= maxRestarts {
		m.reportCrash(mp.pluginID, fmt.Sprintf("插件进程连续崩溃 %d 次，已熔断停止自动重启", attempt))
		return
	}

	// 退避重启（异步，避免阻塞其他插件管理）
	delay := backoffDuration(attempt)
	m.logWarn("插件退避重启", zap.String("plugin", mp.pluginID), zap.Int("attempt", attempt), zap.Duration("delay", delay))
	m.reportRestarting(mp.pluginID, attempt, delay)
	time.Sleep(delay)
	if err := m.Start(context.Background(), mp.pluginID); err != nil {
		m.logWarn("插件重启失败", zap.String("plugin", mp.pluginID), zap.Error(err))
		m.reportCrash(mp.pluginID, err.Error())
		return
	}
	// 自愈成功：通知清除退避期间写入的 last_error（后台不再显示「退避重启中」）
	m.logInfo("插件退避重启成功", zap.String("plugin", mp.pluginID))
	m.reportRecovered(mp.pluginID)
}

// registerAdapters 注册进程外钩子适配器（Info 声明的钩子 → Registry，按插件 ID 幂等）。
func (m *PluginManager) registerAdapters(pluginID string, rpc *pluginClient) {
	if m.registry == nil {
		return
	}
	// 幂等：先注销旧适配器（同插件重复启用时）
	m.unregisterAdapters(pluginID)
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	defer cancel()
	info, err := rpc.info.Info(ctx, &proto.Empty{})
	if err != nil {
		return // 钩子声明拉取失败：进程已运行但钩子不生效（记录由调用方兜底）
	}
	for _, hookName := range info.Hooks {
		if !IsHookRegistered(hookName) {
			continue // 未知钩子名跳过（契约外扩展不注册）
		}
		m.registry.RegisterWithID(hookName, adapterID(pluginID, hookName), rpc.adapterHandler(hookName))
	}
}

// unregisterAdapters 注销插件全部钩子适配器（按插件 ID 精确移除）。
func (m *PluginManager) unregisterAdapters(pluginID string) {
	if m.registry == nil {
		return
	}
	for _, hookName := range allHookNames {
		m.registry.UnregisterWithID(hookName, adapterID(pluginID, hookName))
	}
}

// adapterID 适配器唯一标识（插件 ID + 钩子名）。
func adapterID(pluginID string, hookName string) string {
	return pluginID + "/" + hookName
}

// openLogFile 打开插件日志文件（logs/plugins/{id}.log；失败返回空文件不阻断）。
func (m *PluginManager) openLogFile(pluginID string) (*os.File, error) {
	if m.logDir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(m.logDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(m.logDir, pluginID+".log")
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// reportCrash 熔断回调（写 DB crashed + last_error）。
func (m *PluginManager) reportCrash(pluginID string, errMsg string) {
	if m.events != nil {
		m.events.OnCrashed(pluginID, errMsg)
	}
}

// reportRestarting 重启通知回调。
func (m *PluginManager) reportRestarting(pluginID string, attempt int, delay time.Duration) {
	if m.events != nil {
		m.events.OnRestarting(pluginID, attempt)
	}
	_ = delay // 退避时长已 sleep，仅日志用途
}

// reportRecovered 自愈成功回调转发（清 last_error）。
func (m *PluginManager) reportRecovered(pluginID string) {
	if m.events != nil {
		m.events.OnRecovered(pluginID)
	}
}
