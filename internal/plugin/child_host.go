// internal/plugin/child_host.go
// 插件子进程宿主（自研轻量进程桥，取代 hashicorp/go-plugin）：
//   拉起子进程（注入 cookie/token 环境变量）→ 读 stdout 握手行 → 建立 core/stream
//   两条通道 → 可选启动数据服务监听 → 进程生命周期（退出检测/Kill）。
// 通道鉴权分两级（最小权限）：
//   - env token：core/stream 通道（宿主 → 插件方向；防其他本地进程偷连插件端口）
//   - dataToken：数据服务回连（插件 → 宿主方向；仅随 Activate 下发给授权 data.read
//     的插件——未授权插件无从得知，扫到端口也无法通过鉴权）
package plugin

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/roberts9012062/boke/pkg/plugin-sdk/process"
)

// killGrace 优雅退出等待（stdin 关闭信号 → 强杀前的宽限）。
const killGrace = 3 * time.Second

// childProc 单个插件子进程（连接器类：进程句柄 + 三条通道）。
type childProc struct {
	cmd          *exec.Cmd    // 子进程
	stdin        io.WriteCloser // stdin 管道（关闭即优雅退出信号）
	core         *rpc.Client  // Core 服务客户端
	streamConn   net.Conn     // 流式钩子通道（宿主 → 插件单向写）
	dataListener net.Listener // 数据服务监听（nil=不提供）
	dataToken    string       // 数据服务凭据（随 Activate 下发给授权插件）
	exited       chan struct{} // 进程退出信号（cmd.Wait 后关闭）
	killOnce     sync.Once    // Kill 幂等
}

// startChild 拉起插件子进程并完成握手与通道建立。
// 参数：binPath 插件二进制路径；stderr 插件日志输出（logs/plugins/{id}.log）；
//      dataProvider 数据服务（nil=不启动数据监听）。
func startChild(binPath string, stderr io.Writer, dataProvider DataProvider) (*childProc, error) {
	token, err := newToken()
	if err != nil {
		return nil, fmt.Errorf("连接凭据生成失败：%w", err)
	}
	cmd := exec.Command(binPath)
	cmd.Env = append(os.Environ(),
		process.EnvCookie+"="+process.CookieValue,
		process.EnvToken+"="+token,
	)
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout 管道创建失败：%w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin 管道创建失败：%w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("插件进程启动失败：%w", err)
	}

	p := &childProc{cmd: cmd, stdin: stdin, exited: make(chan struct{})}
	// 进程退出监听（崩溃检测：Exited()；资源回收）
	go func() {
		_ = cmd.Wait()
		close(p.exited)
	}()

	// 握手行读取（超时保护；失败即清理子进程）
	addr, err := readHandshake(stdout)
	if err != nil {
		p.Kill()
		return nil, fmt.Errorf("插件握手失败：%w", err)
	}
	// stdout 排空（插件后续误写 stdout 不阻塞其退出）
	go func() { _, _ = io.Copy(io.Discard, stdout) }()

	// core 通道（主 net/rpc 服务）
	coreConn, err := dialChannel(addr, token, process.ChannelCore)
	if err != nil {
		p.Kill()
		return nil, fmt.Errorf("core 通道建立失败：%w", err)
	}
	p.core = rpc.NewClient(coreConn)

	// stream 通道（异步钩子单向流）
	p.streamConn, err = dialChannel(addr, token, process.ChannelStream)
	if err != nil {
		p.Kill()
		return nil, fmt.Errorf("stream 通道建立失败：%w", err)
	}

	// 数据服务监听（可选；凭据独立生成，仅下发授权插件）
	if dataProvider != nil {
		p.dataToken, err = newToken()
		if err != nil {
			p.Kill()
			return nil, fmt.Errorf("数据服务凭据生成失败：%w", err)
		}
		p.dataListener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			p.Kill()
			return nil, fmt.Errorf("数据服务监听失败：%w", err)
		}
		go dataAcceptLoop(p.dataListener, p.dataToken, dataProvider)
	}
	return p, nil
}

// readHandshake 读取并解析子进程 stdout 握手行（带超时）。
func readHandshake(stdout io.Reader) (string, error) {
	type result struct {
		line string
		err  error
	}
	reader := bufio.NewReader(stdout)
	done := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		done <- result{line: line, err: err}
	}()
	select {
	case res := <-done:
		if res.err != nil {
			return "", fmt.Errorf("握手行读取失败：%w", res.err)
		}
		return process.ParseHandshakeLine(res.line)
	case <-time.After(process.HandshakeTimeout):
		return "", fmt.Errorf("握手超时（%s 内未收到握手行）", process.HandshakeTimeout)
	}
}

// dialChannel 拨号插件端口并发送通道头鉴权。
func dialChannel(addr string, token string, channel string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if err := process.WriteChannelHeader(conn, token, channel); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("通道头发送失败：%w", err)
	}
	return conn, nil
}

// dataAcceptLoop 数据服务接受循环：验 dataToken 后以 net/rpc 服务（注册名 Data）。
func dataAcceptLoop(listener net.Listener, dataToken string, provider DataProvider) {
	rpcServer := rpc.NewServer()
	if err := rpcServer.RegisterName(process.DataServiceName, &dataRPCServer{provider: provider}); err != nil {
		_ = listener.Close()
		return
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			return // listener 关闭（进程终止清理）
		}
		go func(c net.Conn) {
			defer func() { _ = c.Close() }()
			channel, reader, err := process.ReadChannelHeader(c, dataToken)
			if err != nil || channel != process.ChannelData {
				return // 凭据不符/通道不符：直接断开
			}
			rpcServer.ServeConn(&process.BufferedConn{Reader: reader, Conn: c})
		}(conn)
	}
}

// DataAddr 数据服务监听地址（未启动返回空串——Activate 时按授权决定是否下发）。
func (p *childProc) DataAddr() string {
	if p.dataListener == nil {
		return ""
	}
	return p.dataListener.Addr().String()
}

// DataToken 数据服务凭据（与 DataAddr 成对下发）。
func (p *childProc) DataToken() string {
	return p.dataToken
}

// CoreClient 返回 Core 服务客户端（nil=进程尚未完成握手）。
func (p *childProc) CoreClient() *rpc.Client {
	return p.core
}

// StreamConn 返回流式钩子通道连接。
func (p *childProc) StreamConn() net.Conn {
	return p.streamConn
}

// Exited 子进程是否已退出（watchExit 轮询）。
func (p *childProc) Exited() bool {
	select {
	case <-p.exited:
		return true
	default:
		return false
	}
}

// Kill 终止子进程（幂等）：stdin 关闭优雅信号 → 宽限 → 强杀；同步清理通道资源。
func (p *childProc) Kill() {
	p.killOnce.Do(func() {
		_ = p.stdin.Close() // 优雅退出信号（插件读到 EOF 自行退出）
		select {
		case <-p.exited:
		case <-time.After(killGrace):
			_ = p.cmd.Process.Kill() // 宽限超时强杀
			<-p.exited               // 等 Wait 回收完成
		}
		if p.dataListener != nil {
			_ = p.dataListener.Close()
		}
		if p.streamConn != nil {
			_ = p.streamConn.Close()
		}
		if p.core != nil {
			_ = p.core.Close()
		}
	})
}

// newToken 生成 32 字节随机十六进制凭据。
func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
