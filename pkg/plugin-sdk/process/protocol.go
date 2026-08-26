// pkg/plugin-sdk/process/protocol.go
// 插件进程桥协议（宿主 internal/plugin 与插件子进程共用的最小契约）：
// 取代 hashicorp/go-plugin——该库核心文件无条件依赖 gRPC，插件二进制被传递性
// 链入 grpc/protobuf 全家桶（+10MB）；本协议仅用标准库（net/rpc + gob + TCP），
// 插件体积回归 Go runtime 基线。
//
// 协议流程：
//   1. 宿主 exec 插件子进程，注入环境变量：COOKIE（魔数防误启动）+ TOKEN（连接凭据）
//   2. 子进程 listen 127.0.0.1 随机端口，stdout 打印一行握手（本包 BuildHandshakeLine）
//   3. 宿主解析握手行，与子进程建立两条 TCP 连接，每条连接首行发送 "TOKEN 通道名"
//      （core = 主 net/rpc 服务；stream = 异步钩子单向流），子进程验 token 后分发
//   4. 数据服务为反向通道：宿主 listen，Activate 请求携带地址，插件 Dial 并同样
//      以 token 首行鉴权后建立 net/rpc 客户端
//   5. 宿主关闭子进程 stdin 作为退出信号（先 Deactivate RPC 再关），Kill 兜底
//
// 安全：仅监听回环地址 + 每连接首行 token 鉴权（token 仅经环境变量与进程间
// RPC 传递，其他本地进程无从得知）；进程级隔离与故障语义同前。
package process

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

// 协议常量（宿主与插件两侧单一事实源；不兼容变更时同步升 BridgeVersion）。
const (
	bridgeName    = "YUEYAN-PLUGIN"     // 握手行前缀（行首标识）
	bridgeVersion = 1                   // 进程桥协议版本
	contractVer   = 4                   // 插件契约版本（v3=gRPC 已废弃；见 contract 包）
	EnvCookie     = "YUEYAN_PLUGIN_COOKIE" // 魔数环境变量（防误启动：直接运行插件即刻退出）
	EnvToken      = "YUEYAN_PLUGIN_TOKEN"  // 连接凭据环境变量（TCP 首行鉴权）
	CookieValue   = "yueyan-blog-plugin-v1" // 魔数取值（与宿主约定）
	listenAddr    = "127.0.0.1:0"       // 子进程监听地址（回环随机端口）

	// HandshakeTimeout 握手超时（子进程启动 → stdout 握手行的等待上限）。
	HandshakeTimeout = 15 * time.Second
)

// 通道名（连接首行声明；决定接受方的连接分发去向）。
const (
	ChannelCore   = "core"   // 主 net/rpc 服务（生命周期/钩子/自定义 API；宿主 → 插件）
	ChannelStream = "stream" // 异步钩子单向流（宿主 → 插件，gob 持续编码）
	ChannelData   = "data"   // 数据服务 net/rpc（插件 → 宿主回连，Activate 携带地址）
)

// net/rpc 服务注册名（宿主与插件两侧调用的路径约定："Core.XXX" / "Data.XXX"）。
const (
	CoreServiceName  = "Core" // 主服务（插件进程内提供）
	DataServiceName  = "Data" // 数据服务（宿主进程内提供）
)

// NewListener 创建插件侧监听（回环随机端口；插件进程标准入口）。
func NewListener() (net.Listener, error) {
	return net.Listen("tcp", listenAddr)
}

// BuildHandshakeLine 构造握手行（子进程 stdout 输出；addr 为 listener 实际地址）。
// 格式：YUEYAN-PLUGIN <桥版本> <契约版本> tcp <addr>
func BuildHandshakeLine(addr string) string {
	return fmt.Sprintf("%s %d %d tcp %s", bridgeName, bridgeVersion, contractVer, addr)
}

// ParseHandshakeLine 解析握手行（宿主侧；返回子进程监听地址）。
// 校验前缀、桥版本与契约版本——不匹配报清晰错误（旧插件二进制在此即被拒绝）。
func ParseHandshakeLine(line string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 5 || fields[0] != bridgeName || fields[3] != "tcp" {
		return "", fmt.Errorf("握手行格式非法：%q", line)
	}
	var bv, cv int
	if _, err := fmt.Sscanf(fields[1], "%d", &bv); err != nil {
		return "", fmt.Errorf("桥版本解析失败：%w", err)
	}
	if _, err := fmt.Sscanf(fields[2], "%d", &cv); err != nil {
		return "", fmt.Errorf("契约版本解析失败：%w", err)
	}
	if bv != bridgeVersion {
		return "", fmt.Errorf("插件桥协议版本不兼容（插件=%d，宿主=%d，请重新编译安装插件）", bv, bridgeVersion)
	}
	if cv != contractVer {
		return "", fmt.Errorf("插件契约版本不兼容（插件=v%d，宿主=v%d，请重新编译安装插件）", cv, contractVer)
	}
	return fields[4], nil
}

// WriteChannelHeader 连接建立后发送首行鉴权（"TOKEN 通道名\n"；发起方调用）。
func WriteChannelHeader(conn net.Conn, token string, channel string) error {
	header := token + " " + channel + "\n"
	_, err := conn.Write([]byte(header))
	return err
}

// ReadChannelHeader 接收并校验连接首行（接受方调用；token 不符即断开）。
// 返回声明的通道名与包装了预读缓冲的读取器——bufio 预读的字节必须经返回的
// reader 继续消费（直接读原连接会丢失预读数据）。
func ReadChannelHeader(conn net.Conn, token string) (string, *bufio.Reader, error) {
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", nil, fmt.Errorf("通道头读取失败：%w", err)
	}
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) != 2 || fields[0] != token {
		return "", nil, fmt.Errorf("通道鉴权失败（token 不符）")
	}
	if fields[1] != ChannelCore && fields[1] != ChannelStream && fields[1] != ChannelData {
		return "", nil, fmt.Errorf("未知通道名：%q", fields[1])
	}
	return fields[1], reader, nil
}

// BufferedConn 带预读缓冲的连接（ReadChannelHeader 之后继续使用连接的标准形态：
// 读走 reader，写与关闭透传原连接）。
type BufferedConn struct {
	Reader *bufio.Reader // 预读缓冲（通道头之后的数据从这里继续读）
	Conn   net.Conn      // 原连接（写/关闭/地址）
}

// Read 从预读缓冲读取（缓冲耗尽后回源连接）。
func (c *BufferedConn) Read(p []byte) (int, error) { return c.Reader.Read(p) }

// Write 透传原连接。
func (c *BufferedConn) Write(p []byte) (int, error) { return c.Conn.Write(p) }

// Close 关闭原连接。
func (c *BufferedConn) Close() error { return c.Conn.Close() }
