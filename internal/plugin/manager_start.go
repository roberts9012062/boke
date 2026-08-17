// internal/plugin/manager_start.go
// 插件启动流程（从 manager.go 拆出，行数硬性指标）。
// E3 并发改造：启动拆为三阶段——
//   阶段 1（锁内，微秒级）：幂等/熔断检查 + starting 占位（防并发启动）；
//   阶段 2（锁外，最长 15s）：二进制校验 → 握手 → Info → Activate → 流通道 → 配置下发；
//   阶段 3（锁内，微秒级）：占位仍属本启动时提交 running 管理项 + 注册钩子 + 崩溃监听。
// 效果：一个慢插件握手不再阻塞其他插件的 Stop/Call/PushConfig/IsRunning（此前全程持锁）。
package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/pkg/plugin-sdk/handshake"
	"github.com/roberts9012062/boke/pkg/plugin-sdk/proto"
)

// startSession 一次启动会话（阶段 2 的产物：子进程客户端与资源句柄）。
type startSession struct {
	client  *goplugin.Client // go-plugin 客户端（持有子进程）
	rpc     *pluginClient    // gRPC 三服务客户端
	logFile *os.File         // 插件 stderr 日志文件（进程退出时关闭）
}

// cleanup 释放会话资源（阶段 2 失败或阶段 3 被抢占时调用）。
func (s *startSession) cleanup() {
	if s.client != nil {
		s.client.Kill()
	}
	if s.logFile != nil {
		_ = s.logFile.Close()
	}
}

// Start 启用插件：拉起子进程（已 running 幂等返回；starting 中返回错误防并发启动）。
func (m *PluginManager) Start(ctx context.Context, pluginID string) error {
	// ---------- 阶段 1：锁内快速检查与占位 ----------
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
		state: stateRunning, client: session.client, rpc: session.rpc, logFile: session.logFile,
	}
	m.managed[pluginID] = mp
	m.mu.Unlock()

	// 钩子适配器注册（锁外：Registry 自带锁，避免锁嵌套）
	m.registerAdapters(pluginID, session.rpc)
	go m.watchExit(mp)
	m.logInfo("插件启动完成", zap.String("plugin", pluginID))
	return nil
}

// launch 慢速启动流程（锁外执行）：二进制校验 → 拉起子进程握手 → Info 校验 →
// 能力门控 → Activate → 流式钩子通道 → 配置下发。失败返回错误（session 尽量填充以便清理）。
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

	// 插件 stderr → 日志文件（stdout 由 go-plugin 用于握手，不可占用）
	logFile, err := m.openLogFile(pluginID)
	if err != nil {
		return session, fmt.Errorf("打开插件日志失败：%w", err)
	}
	session.logFile = logFile

	// go-plugin 客户端（AutoMTLS 自动 TLS 加密）
	// 日志双通道（go-plugin v1.8 行为：握手后子进程 os.Stderr 被 stdio 流接管，
	// 客户端经 SyncStderr 写入；握手前的输出走原始管道 Stderr——两者都指向日志文件）
	session.client = goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  handshake.Handshake, // D1 解耦：握手配置单一事实源（handshake 包）
		Plugins:          map[string]goplugin.Plugin{"core": &coreGRPCPlugin{dataProvider: m.dataProviderFn()}},
		Cmd:              exec.Command(binPath),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		AutoMTLS:         true,
		StartTimeout:     handshakeTimeout,
		Stderr:           logFile,
		SyncStderr:       logFile,
		Logger:           hclog.NewNullLogger(),
	})

	// 握手（阻塞至协议就绪；失败清理资源）
	rpcClient, err := session.client.Client()
	if err != nil {
		return session, fmt.Errorf("插件「%s」握手失败：%w", pluginID, err)
	}
	raw, err := rpcClient.Dispense("core")
	if err != nil {
		return session, fmt.Errorf("插件「%s」客户端分发失败：%w", pluginID, err)
	}
	rpc, ok := raw.(*pluginClient)
	if !ok {
		return session, fmt.Errorf("插件「%s」客户端类型错误", pluginID)
	}
	session.rpc = rpc

	// Info 一致性校验（声明的插件 ID 必须与实例一致）
	info, err := rpc.info.Info(ctx, &proto.Empty{})
	if err != nil {
		return session, fmt.Errorf("插件「%s」Info 调用失败：%w", pluginID, err)
	}
	if info.Id != pluginID {
		return session, fmt.Errorf("插件二进制声明 ID「%s」与实例「%s」不一致", info.Id, pluginID)
	}
	// 能力门控（P2 加固）：以「安装登记能力 ∩ 二进制自报能力」判定——
	// 恶意二进制自报 data.read 不再直接得手（安装登记中未声明的扩展能力一律不授权）；
	// provider 未配置（旧装配/单测）时退化为仅自报（保持 M3.8 原行为）
	allowedCaps := info.GetCapabilities()
	if m.capabilityProvider != nil {
		registered, err := m.capabilityProvider(ctx, pluginID)
		if err == nil {
			allowedCaps = intersectStrings(registered, allowedCaps)
		}
	}
	// 仅声明 data.read 的插件获得数据服务 brokerID（未声明一律不下发）
	dataBrokerID := rpc.dataBrokerID
	if dataBrokerID != 0 && !stringListContains(allowedCaps, "data.read") {
		dataBrokerID = 0
	}

	// Activate（携带许可证信息：provider 查询，无记录/失败按 free demo 兜底）
	license := &proto.LicenseInfo{Edition: "free"}
	if m.licenseProvider != nil {
		if li, err := m.licenseProvider(ctx, pluginID); err == nil && li != nil {
			license = li
		}
	}
	if st, err := rpc.info.Activate(ctx, &proto.ActivateRequest{
		License:      license,
		DataBrokerId: dataBrokerID, // M3.8：能力门控后的数据服务 brokerID（0=未授权）
	}); err != nil || !st.Ok {
		reason := "未知原因"
		if err == nil && st.Error != "" {
			reason = st.Error
		}
		return session, fmt.Errorf("插件「%s」激活失败：%s", pluginID, reason)
	}

	// 建立流式钩子通道（M3.9：异步事件经流推送；失败回退 Execute——不阻断启动）
	if err := rpc.openStream(ctx); err != nil {
		m.logWarn("插件流式通道建立失败（回退 Execute）", zap.String("plugin", pluginID), zap.Error(err))
	}

	// 下发配置（M3.7：provider 查询，无记录/失败按空配置；失败仅告警不阻断启动——插件可用默认值）
	if m.configProvider != nil {
		if values, err := m.configProvider(ctx, pluginID); err == nil {
			if _, err := rpc.info.SetConfig(ctx, &proto.ConfigInfo{Values: values}); err != nil {
				m.logWarn("插件配置下发失败（启动阶段）", zap.String("plugin", pluginID), zap.Error(err))
			}
		} else {
			m.logWarn("插件配置查询失败（启动阶段）", zap.String("plugin", pluginID), zap.Error(err))
		}
	}
	return session, nil
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
