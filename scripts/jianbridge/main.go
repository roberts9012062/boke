// jian 桥接程序：模拟 OpenPencil 的 `jian render` CLI，
// 通过 Pen（OpenPencil 桌面版）的 MCP 通道把 .op/.pen 文档渲染为 PNG。
// 用法: jian render <input.op> --out <out.png> [--width W] [--height H] [--scale S]
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Pen 桌面版安装路径（与 MCP server 同目录）
const (
	penExe    = `O:\package\Pen\Pen.exe`
	mcpServer = `O:\package\Pen\resources\app.asar.unpacked\out\mcp-server-windows-x64.exe`
)

// 轮询上限：等待 Pen 打开文档
const (
	pollInterval = 600 * time.Millisecond
	pollTimeout  = 120 * time.Second
)

var activeEditorRe = regexp.MustCompile("Currently active canvas editor: `([^`]+)`")

type mcpResponse struct {
	ID     json.RawMessage `json:"id"`
	Result *struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// 调用一次 MCP 工具（每次新起 mcp-server 进程，stdio 一次性会话）
func mcpCall(tool string, args map[string]any) (string, error) {
	must := func(v any, err error) any {
		if err != nil {
			panic(err)
		}
		return v
	}
	_ = must
	initReq := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "dsh-jian-bridge", "version": "1.0"},
		},
	}
	notif := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	callReq := map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	}
	var buf bytes.Buffer
	for _, r := range []any{initReq, notif, callReq} {
		enc, err := json.Marshal(r)
		if err != nil {
			return "", fmt.Errorf("marshal request: %w", err)
		}
		buf.Write(enc)
		buf.WriteByte('\n')
	}

	cmd := exec.Command(mcpServer, "-app", "desktop")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start mcp-server: %w", err)
	}
	if _, err := stdin.Write(buf.Bytes()); err != nil {
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("write request: %w", err)
	}
	_ = stdin.Close()

	var result string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var resp mcpResponse
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}
		var idInt int
		_ = json.Unmarshal(resp.ID, &idInt)
		if idInt != 3 {
			continue
		}
		if resp.Error != nil {
			return "", fmt.Errorf("mcp %s: %s", tool, resp.Error.Message)
		}
		if resp.Result != nil && len(resp.Result.Content) > 0 {
			result = resp.Result.Content[0].Text
		}
		break
	}
	_ = cmd.Wait()
	if result == "" && stderr.Len() > 0 {
		return "", fmt.Errorf("mcp %s failed: %s", tool, strings.TrimSpace(stderr.String()))
	}
	return result, nil
}

// 从 get_app_state 文本中提取当前激活的编辑器文档路径
func activeEditorPath(stateText string) string {
	m := activeEditorRe.FindStringSubmatch(stateText)
	if m == nil {
		return ""
	}
	return m[1]
}

// 读取 .op/.pen JSON 文档，返回第一个顶层节点 id
func firstTopLevelID(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var doc struct {
		Children []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"children"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("parse document: %w", err)
	}
	for _, c := range doc.Children {
		if c.ID != "" {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("document has no top-level nodes")
}

// 从 PNG 头部读取宽高
func pngSize(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	head := make([]byte, 24)
	if _, err := io.ReadFull(f, head); err != nil {
		return 0, 0, err
	}
	if !bytes.Equal(head[:8], []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		return 0, 0, fmt.Errorf("not a PNG")
	}
	return int(binary.BigEndian.Uint32(head[16:20])), int(binary.BigEndian.Uint32(head[20:24])), nil
}

// 杀掉所有 Pen 进程（transport 异常/多实例冲突时重启用）
func killPen() {
	_ = exec.Command("taskkill", "/IM", "Pen.exe", "/F").Run()
	time.Sleep(1500 * time.Millisecond)
}

// 检查 Pen 进程是否在运行
func penRunning() bool {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq Pen.exe", "/NH").Output()
	return err == nil && strings.Contains(string(out), "Pen.exe")
}

// 统计 Pen 进程数量（多实例 = 单实例锁失效，需要清理）
func penCount() int {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq Pen.exe", "/NH").Output()
	if err != nil {
		return 0
	}
	return strings.Count(string(out), "Pen.exe")
}

// 轮询等待 Pen 打开指定文档；进程消失或多实例冲突时自动修复
func waitForDocument(tempPen string, nodeID string) error {
	deadline := time.Now().Add(pollTimeout)
	lastRespawn := time.Now()
	consecutiveFailures := 0
	for time.Now().Before(deadline) {
		// 进程消失 → 重启
		if !penRunning() {
			if time.Since(lastRespawn) > 5*time.Second {
				fmt.Fprintln(os.Stderr, "jian: Pen not running, spawning")
				spawnPenFor(tempPen)
				lastRespawn = time.Now()
			}
			time.Sleep(pollInterval)
			continue
		}
		// 多实例（单实例锁失效/错误窗口）→ 清理后重启
		if penCount() > 1 && time.Since(lastRespawn) > 8*time.Second {
			fmt.Fprintln(os.Stderr, "jian: multiple Pen instances, restarting cleanly")
			killPen()
			spawnPenFor(tempPen)
			lastRespawn = time.Now()
			continue
		}
		state, err := mcpCall("get_app_state", map[string]any{
			"include_schema":              false,
			"include_canvas_design":       false,
			"include_scripts_and_shaders": false,
			"include_browser":             false,
		})
		if err != nil {
			consecutiveFailures++
			time.Sleep(pollInterval)
			continue
		}
		consecutiveFailures = 0
		active := activeEditorPath(state)
		want := "/" + strings.ReplaceAll(tempPen, "\\", "/")
		if active != "" && strings.EqualFold(active, want) && strings.Contains(state, "Top-level nodes") {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("timed out waiting for Pen to open %s", tempPen)
}

// 启动 Pen 打开指定文档（已运行则 second-instance 开新窗口）
// 附加优化：--disable-gpu 加速 renderer 初始化；无效代理阻断 updater 下载
func spawnPenFor(tempPen string) {
	penCmd := exec.Command(penExe, "--disable-gpu", "--file", tempPen)
	penCmd.Stdout = io.Discard
	penCmd.Stderr = io.Discard
	penCmd.Env = append(os.Environ(),
		"HTTPS_PROXY=http://127.0.0.1:9",
		"HTTP_PROXY=http://127.0.0.1:9",
		"NO_PROXY=",
	)
	if err := penCmd.Start(); err == nil {
		_ = penCmd.Process.Release()
	}
}

// 执行一次渲染
func runRender(input string, out string, scale int) error {
	// 强制清理残留 Pen 实例（锁冲突/Error 窗口会导致 active 错乱），
	// 确保 spawn 的是唯一干净实例
	killPen()

	nodeID, err := firstTopLevelID(input)
	if err != nil {
		return err
	}

	// 准备临时 .pen（固定路径复用，避免 Pen 窗口无限累积）
	tempDir := filepath.Join(os.TempDir(), "dsh-openpencil-bridge")
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return err
	}
	tempPen := filepath.Join(tempDir, "preview.pen")
	raw, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	if err := os.WriteFile(tempPen, raw, 0o600); err != nil {
		return err
	}

	// 让 Pen 打开文档（已在运行则 second-instance 开新窗口，否则启动 Pen）
	spawnPenFor(tempPen)

	if err := waitForDocument(tempPen, nodeID); err != nil {
		return err
	}

	// 通过 export_nodes 导出顶层节点 PNG
	result, err := mcpCall("export_nodes", map[string]any{
		"filePath":  tempPen,
		"nodeIds":   []string{nodeID},
		"outputDir": tempDir,
		"format":    "png",
		"scale":     scale,
	})
	if err != nil {
		return err
	}
	exported := parseExportedPath(result)
	if exported == "" {
		return fmt.Errorf("export_nodes produced no file: %s", result)
	}
	if err := copyFile(exported, out); err != nil {
		return err
	}
	w, h, err := pngSize(out)
	if err != nil {
		return err
	}
	fmt.Printf("rendered (%dx%d physical)\n", w, h)
	return nil
}

// 从 export_nodes 响应中提取导出文件路径
func parseExportedPath(text string) string {
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Exported") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(line), ".png") {
			return line
		}
	}
	return ""
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func main() {
	args := os.Args[1:]
	if len(args) < 2 || args[0] != "render" {
		fmt.Fprintln(os.Stderr, "usage: jian render <input> --out <output> [--width N] [--height N] [--scale N]")
		os.Exit(2)
	}
	input := args[1]
	out := ""
	scale := 1
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--out":
			if i+1 < len(args) {
				out = args[i+1]
				i++
			}
		case "--scale":
			if i+1 < len(args) {
				if v, err := strconv.ParseFloat(args[i+1], 64); err == nil {
					scale = int(v)
					if scale < 1 {
						scale = 1
					}
				}
				i++
			}
		case "--width", "--height":
			// 视口参数仅精确渲染器语义，这里忽略
			if i+1 < len(args) {
				i++
			}
		}
	}
	if out == "" {
		fmt.Fprintln(os.Stderr, "jian render: --out is required")
		os.Exit(2)
	}
	if err := runRender(input, out, scale); err != nil {
		fmt.Fprintln(os.Stderr, "jian render:", err)
		os.Exit(1)
	}
}
