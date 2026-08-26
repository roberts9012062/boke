// internal/plugin/manager_start.go
// 插件启动流程（从 manager.go 拆出，行数硬性指标）。
// E3 并发改造：启动拆为三阶段——
//   阶段 1（锁内，微秒级）：幂等/熔断检查 + starting 占位（防并发启动）；
//   阶段 2（锁外，最长 15s）：二进制校验 → 拉起握手 → Info → Activate → 配置下发；
//   阶段 3（锁内，微秒级）：占位仍属本启动时提交 running 管理项 + 注册钩子 + 崩溃监听。
// 效果：一个慢插件握手不再阻塞其他插件的 Stop/Call/PushConfig/IsRunning（此前全程持锁）。
package plugin

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"

	"github.com/roberts9012062/boke/pkg/plugin-sdk/contract"
)

// startSession 一次启动会话（阶段 2 的产物：子进程客户端与资源句柄）。
type startSession struct {
	proc    *childProc    // 子进程（进程桥：进程句柄 + 三条通道）
	rpc     *pluginClient // Core 服务客户端封装
	logFile *os.File      // 插件 stderr 日志文件（进程退出时关闭）
}

// cleanup 释放会话资源（阶段 2 失败或阶段 3 被抢占时调用）。
func (s *startSession) cleanup() {
	if s.proc != nil {
		s.proc.Kill()
	}
	if s.logFile != nil {
		_ = s.logFile.Close()
	}
}

// Start 启用插件：拉起子进程（已 running 幂等返回；starting 中返回错误防并发启动）。
func (m *PluginManager) Start(ctx context.Context, pluginID string) error {
	// ---------- 阶段 1：锁内快速检查和占位 ----------
	m.mu.Lock()
	if mp, ok := m.managed[pluginID]; ok {
		switch mp.state {
		case stateRunning:
			m.mu.Unlock()
			return nil // 已在运行
		case stateStarting:
			m.mu.Unlock()
			return fmt.Errorf("插件「%s」正在启动中，请稍候", pluginID)
		case stateCrashed:
			m.mu.Unlock()
			return fmt.Errorf("插件「%s」已熔断（连续崩溃），请手动重新启用", pluginID)
		}
		delete(m.managed, pluginID) // stopped：清理旧管理项重建
	}
	// 熔断后手动启用：重置崩溃计数（崩溃-重启-再崩溃视为连续，Start 成功不清零）
	if m.crashCounts[pluginID] >= maxRestarts {
		m.crashCounts[pluginID] = 0
	}
	// starting 占位（Stop/并发 Start 在阶段 1 即识别并避让）
	m.managed[pluginID] = &ManagedPlugin{pluginID: pluginID, state: stateStarting}
	m.mu.Unlock()

	// ---------- 阶段 2：锁外慢速启动（握手最长 15s，不阻塞其他插件管理） ----------
	session, err := m.launch(ctx, pluginID)
	if err != nil {
		// 失败：清理资源 + 移除占位（仅当占位仍是 starting，未被外部变更）
		session.cleanup()
		m.mu.Lock()
		if mp, ok := m.managed[pluginID]; ok && mp.state == stateStarting {
			delete(m.managed, pluginID)
		}
		m.mu.Unlock()
		return err
	}

	// ---------- 阶段 3：锁内提交 ----------
	m.mu.Lock()
	cur, ok := m.managed[pluginID]
	if !ok || cur.state != stateStarting {
		// 占位已被外部移除（理论上的 Stop 冲突窗口）：丢弃本次启动结果
		m.mu.Unlock()
		session.cleanup()
		return fmt.Errorf("插件「%s」启动会话已失效（管理项被并发变更）", pluginID)
	}
	mp := &ManagedPlugin{
		pluginID: pluginID, binPath: m.store.BinPath(pluginID),
		state: stateRunning, client: session.proc, rpc: session.rpc, logFile: session.logFile,
	}
	m.managed[pluginID] = mp
	m.mu.Unlock()

	// 钩子适配器注册（锁外：Registry 自带锁，避免锁嵌套）
	m.registerAdapters(pluginID, session.rpc)
	// 配置下发（running 提交后：schema 聚合的进程上报分支此时可用）
	m.pushInitialConfig(ctx, pluginID, session.rpc)
	go m.watchExit(mp)
	m.logInfo("插件启动完成", zap.String("plugin", pluginID))
	return nil
}

// launch 慢速启动流程（锁外执行）：二进制校验 → 拉起子进程握手 → Info 校验 →
// 能力门控 → Activate。失败返回错误（session 尽量填充以便清理）。
// 流式钩子通道随进程握手一并建立（child_host 拨号 stream 通道；断连时 sendStream
// 自动降级 ExecuteHook，无需启动阶段单独建流）。
// 配置下发在 Start 阶段 3 之后（pushInitialConfig）——launch 期间插件尚在 starting
// 状态，service 层 schema 聚合（进程 Info 上报优先）取不到，会合并出空配置。
func (m *PluginManager) launch(ctx context.Context, pluginID string) (*startSession, error) {
	session := &startSession{}
	// 二进制校验（M3.3 本地预置；Release 下载安装 M3.4 后置）
	binPath := m.store.BinPath(pluginID)
	if binPath == "" {
		return session, fmt.Errorf("插件 ID 不合法")
	}
	if !m.store.Exists(pluginID) {
		return session, fmt.Errorf("插件「%s」二进制不可用（期望路径 %s）", pluginID, binPath)
	}

	// 插件 stderr → 日志文件（stdout 用于握手行，不可占用）
	logFile, err := m.openLogFile(pluginID)
	if err != nil {
		return session, fmt.Errorf("打开插件日志失败：%w", err)
	}
	session.logFile = logFile

	// 拉起子进程并建立通道（core/stream + 可选数据服务监听）
	proc, err := startChild(binPath, logFile, m.dataProviderFn())
	if err != nil {
		return session, fmt.Errorf("插件「%s」%w", pluginID, err)
	}
	session.proc = proc
	session.rpc = newPluginClient(proc)

	// Info 一致性校验（声明的插件 ID 必须与实例一致）
	var info contract.PluginInfo
	if err := session.rpc.callCore(ctx, "Info", &contract.Empty{}, &info); err != nil {
		return session, fmt.Errorf("插件「%s」Info 调用失败：%w", pluginID, err)
	}
	if info.ID != pluginID {
		return session, fmt.Errorf("插件二进制声明 ID「%s」与实例「%s」不一致", info.ID, pluginID)
	}
	// 能力门控（P2 加固）：以「安装登记能力 ∩ 二进制自报能力」判定——
	// 恶意二进制自报 data.read 不再直接得手（安装登记中未声明的扩展能力一律不授权）；
	// provider 未配置（旧装配/单测）时退化为仅自报（保持原行为）
	allowedCaps := info.Capabilities
	if m.capabilityProvider != nil {
		registered, err := m.capabilityProvider(ctx, pluginID)
		if err == nil {
			allowedCaps = intersectStrings(registered, allowedCaps)
		}
	}
	// 仅声明 data.read 的插件获得数据服务地址与凭据（未声明一律不下发）
	dataAddr, dataToken := proc.DataAddr(), proc.DataToken()
	if dataAddr != "" && !stringListContains(allowedCaps, "data.read") {
		dataAddr, dataToken = "", ""
	}

	// Activate（携带许可证信息 + 数据服务凭据：provider 查询，无记录/失败按 free demo 兜底）
	license := contract.LicenseInfo{Edition: "free"}
	if m.licenseProvider != nil {
		if li, err := m.licenseProvider(ctx, pluginID); err == nil && li != nil {
			license = *li
		}
	}
	var status contract.Status
	if err := session.rpc.callCore(ctx, "Activate", &contract.ActivateRequest{
		License:   license,
		DataAddr:  dataAddr,  // 能力门控后的数据服务地址（空=未授权）
		DataToken: dataToken, // 数据服务凭据（与地址成对；空=未授权）
	}, &status); err != nil || !status.OK {
		reason := "未知原因"
		if err == nil && status.Error != "" {
			reason = status.Error
		}
		return session, fmt.Errorf("插件「%s」激活失败：%s", pluginID, reason)
	}
	return session, nil
}

// pushInitialConfig 启动收尾配置下发（Start 阶段 3 提交 running 后调用：
// 此时 service 层 schema 聚合的进程上报分支可用，合并结果完整；失败仅告警
// 不阻断——插件可用 schema 默认值，保存配置时会经 PushConfig 补推）。
func (m *PluginManager) pushInitialConfig(ctx context.Context, pluginID string, rpc *pluginClient) {
	if m.configProvider == nil {
		return
	}
	values, err := m.configProvider(ctx, pluginID)
	if err != nil {
		m.logWarn("插件配置查询失败（启动阶段）", zap.String("plugin", pluginID), zap.Error(err))
		return
	}
	var status contract.Status
	if err := rpc.callCore(ctx, "SetConfig", &contract.ConfigInfo{Values: values}, &status); err != nil {
		m.logWarn("插件配置下发失败（启动阶段）", zap.String("plugin", pluginID), zap.Error(err))
	}
}

// SetLogger 注入结构化日志（E4：装配后调用；未注入时 logInfo/logWarn 退回 stderr）。
func (m *PluginManager) SetLogger(logger *zap.Logger) {
	m.loggerMu.Lock()
	m.logger = logger
	m.loggerMu.Unlock()
}

// logInfo 信息级日志（logger 未注入时退回 stderr，保持进程日志可追溯）。
func (m *PluginManager) logInfo(msg string, fields ...zap.Field) {
	m.loggerMu.RLock()
	logger := m.logger
	m.loggerMu.RUnlock()
	if logger != nil {
		logger.Info(msg, fields...)
		return
	}
	fmt.Fprintln(os.Stderr, "[plugin-mgr] "+msg)
}

// logWarn 告警级日志（同 logInfo 退化策略）。
func (m *PluginManager) logWarn(msg string, fields ...zap.Field) {
	m.loggerMu.RLock()
	logger := m.logger
	m.loggerMu.RUnlock()
	if logger != nil {
		logger.Warn(msg, fields...)
		return
	}
	fmt.Fprintln(os.Stderr, "[plugin-mgr] "+msg)
}
