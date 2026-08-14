// op-host-web-server 模拟器：模拟 OpenPencil 托管 Web 宿主的 CLI/HTTP 契约，
// 实际渲染与文档操作全部转发给 Pen（OpenPencil 桌面版）的 MCP 通道。
// 用法: op-host-web-server --serve-web --managed --port 0 --file <path> --allow-origin <origin>
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	penExe    = `O:\package\Pen\Pen.exe`
	mcpServer = `O:\package\Pen\resources\app.asar.unpacked\out\mcp-server-windows-x64.exe`
	// Pen 自动保存的备份目录（%USERPROFILE%\.pencil\backup）
	pencilBackupDir = `C:\Users\boss\.pencil\backup`
)

// ---------- 全局状态 ----------

type hostState struct {
	mu         sync.Mutex
	filePath   string         // --file 参数（用户 .op）
	tempPen    string         // Pen 打开的副本路径
	backupKey  string         // 备份文件完整路径（按 fileURI sha1 定位）
	version    int            // 文档版本（每次写操作 +1）
	docJSON    string         // 权威文档 JSON（写操作后刷新）
	initialDoc map[string]any // 启动时的文档（文档级字段来源）
	penReady   atomic.Bool    // Pen 是否已就绪（后台预热完成）
	token      string         // handshake token
}

// ---------- 工具函数 ----------

func randToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// 计算 Pen 备份文件路径：sha1("file:///" + 正斜杠路径)
func backupPathFor(winPath string) string {
	uri := "file:///" + strings.ReplaceAll(winPath, "\\", "/")
	sum := sha1.Sum([]byte(uri))
	return filepath.Join(pencilBackupDir, hex.EncodeToString(sum[:]))
}

// ---------- Pen MCP 客户端（stdio 一次性调用） ----------

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

// 调用一次 Pen MCP 工具，返回 text 内容
func penMcpCall(tool string, args map[string]any) (string, error) {
	initReq := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "dsh-openpencil-host", "version": "1.0"},
		},
	}
	callReq := map[string]any{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	}
	var buf bytes.Buffer
	for _, r := range []any{initReq, `{"jsonrpc":"2.0","method":"notifications/initialized"}`, callReq} {
		switch v := r.(type) {
		case string:
			buf.WriteString(v)
		default:
			enc, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			buf.Write(enc)
		}
		buf.WriteByte('\n')
	}

	cmd := exec.Command(mcpServer, "-app", "desktop")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	_, _ = stdin.Write(buf.Bytes())
	_ = stdin.Close()

	type mcpOutcome struct {
		result string
		err    error
	}
	outcomeCh := make(chan mcpOutcome, 1)
	go func() {
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
				outcomeCh <- mcpOutcome{"", fmt.Errorf("pen %s: %s", tool, resp.Error.Message)}
				return
			}
			if resp.Result != nil && len(resp.Result.Content) > 0 {
				result = resp.Result.Content[0].Text
			}
			outcomeCh <- mcpOutcome{result, nil}
			return
		}
		if result == "" && stderr.Len() > 0 {
			outcomeCh <- mcpOutcome{"", fmt.Errorf("pen %s failed: %s", tool, strings.TrimSpace(stderr.String()))}
			return
		}
		outcomeCh <- mcpOutcome{result, nil}
	}()
	select {
	case oc := <-outcomeCh:
		_ = cmd.Wait()
		return oc.result, oc.err
	case <-time.After(20 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return "", fmt.Errorf("pen %s timed out", tool)
	}
}

var activeEditorRe = regexp.MustCompile("Currently active canvas editor: `([^`]+)`")

// 检查 Pen 进程是否在运行
func penRunning() bool {
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq Pen.exe", "/NH").Output()
	return err == nil && strings.Contains(string(out), "Pen.exe")
}

// 启动 Pen 打开指定文档（--disable-gpu 加速初始化；无效代理阻断 updater）
func spawnPenFor(penFile string) {
	penCmd := exec.Command(penExe, "--disable-gpu", "--file", penFile)
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

// 杀掉所有 Pen 进程
func killPen() {
	_ = exec.Command("taskkill", "/IM", "Pen.exe", "/F").Run()
	time.Sleep(1500 * time.Millisecond)
}

// 等待 Pen 打开 tempPen（轮询 get_app_state；进程消失时自动重启）
func waitPenReady(st *hostState, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	lastRespawn := time.Now()
	want := "/" + strings.ReplaceAll(st.tempPen, "\\", "/")
	for time.Now().Before(deadline) {
		if !penRunning() {
			if time.Since(lastRespawn) > 5*time.Second {
				fmt.Fprintln(os.Stderr, "op-host: Pen not running, spawning")
				spawnPenFor(st.tempPen)
				lastRespawn = time.Now()
			}
			time.Sleep(700 * time.Millisecond)
			continue
		}
		state, err := penMcpCall("get_app_state", map[string]any{
			"include_schema":              false,
			"include_canvas_design":       false,
			"include_scripts_and_shaders": false,
			"include_browser":             false,
		})
		if err == nil {
			active := ""
			if m := activeEditorRe.FindStringSubmatch(state); m != nil {
				active = m[1]
			}
			if active != "" && strings.EqualFold(active, want) {
				return nil
			}
		}
		time.Sleep(700 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for Pen to open %s", st.tempPen)
}

// ---------- 写操作（batch_design / update_node） ----------

// 从 execute 响应中提取 "## Print output" 之后的最后一个 Print 内容
func extractPrintOutput(text string) string {
	idx := strings.Index(text, "## Print output")
	if idx < 0 {
		return ""
	}
	rest := text[idx+len("## Print output"):]
	rest = strings.TrimSpace(rest)
	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return line
	}
	return ""
}

// 组装权威文档：初始文档的文档级字段 + Get 返回的顶层节点
func assembleDocument(st *hostState, initialDoc map[string]any, nodesJSON string) (string, error) {
	var nodes []map[string]any
	if err := json.Unmarshal([]byte(nodesJSON), &nodes); err != nil {
		return "", fmt.Errorf("parse Get output: %w", err)
	}
	doc := map[string]any{}
	for k, v := range initialDoc {
		doc[k] = v
	}
	doc["children"] = nodes
	encoded, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// 读取文档第一个顶层节点 id（可能为空字符串）
func firstTopLevelID(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc struct {
		Children []struct {
			ID string `json:"id"`
		} `json:"children"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	for _, c := range doc.Children {
		if c.ID != "" {
			return c.ID
		}
	}
	return ""
}

var topLevelNodesRe = regexp.MustCompile("Top-level nodes: `([^`]+)`")

// 校验 Pen 内存文档与目标文件一致（第一个顶层节点 id 相同；
// 空文档两者都应为空）。不一致说明 Pen 窗口还停留在旧文档，需要重启。
func documentMatches(st *hostState) bool {
	target := firstTopLevelID(st.tempPen)
	state, err := penMcpCall("get_app_state", map[string]any{
		"include_schema":              false,
		"include_canvas_design":       false,
		"include_scripts_and_shaders": false,
		"include_browser":             false,
	})
	if err != nil {
		return false
	}
	first := ""
	if m := topLevelNodesRe.FindStringSubmatch(state); m != nil {
		first = m[1]
	} else if strings.Contains(state, "The document is empty") {
		first = ""
	}
	return first == target
}

// 快速检查 active editor 是否为 temp.pen
func activeMatches(st *hostState) bool {
	state, err := penMcpCall("get_app_state", map[string]any{
		"include_schema":              false,
		"include_canvas_design":       false,
		"include_scripts_and_shaders": false,
		"include_browser":             false,
	})
	if err != nil {
		return false
	}
	active := ""
	if m := activeEditorRe.FindStringSubmatch(state); m != nil {
		active = m[1]
	}
	want := "/" + strings.ReplaceAll(st.tempPen, "\\", "/")
	return active != "" && strings.EqualFold(active, want)
}

// 确保 Pen 就绪（后台预热已完成的情况下走快速路径）
func ensureActiveEditor(st *hostState) error {
	if !st.penReady.Load() {
		return waitPenReady(st, 18*time.Second)
	}
	// active 漂移：重新聚焦 temp.pen
	if !activeMatches(st) {
		fmt.Fprintln(os.Stderr, "op-host: active editor drift, refocusing")
		spawnPenFor(st.tempPen)
		return waitPenReady(st, 15*time.Second)
	}
	return nil
}

func applyWrite(st *hostState, program string) (string, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	// 确保 Pen 已就绪且 active editor 是 temp.pen
	if err := ensureActiveEditor(st); err != nil {
		return "", err
	}
	// 在程序尾部追加文档导出：仅收集顶层节点（ctx.depth === 0），
	// 直接读取内存中的最新文档（不依赖备份节流）
	fullProgram := program + "\nPrint(JSON.stringify(Get((n, ctx) => ctx.depth === 0 ? n : undefined)))"
	text, err := penMcpCall("execute", map[string]any{
		"filePath": st.tempPen,
		"input":    fullProgram,
	})
	if err != nil {
		return "", err
	}
	// 解析 Get 输出并组装权威文档
	nodesJSON := extractPrintOutput(text)
	if nodesJSON != "" {
		if assembled, err := assembleDocument(st, st.initialDoc, nodesJSON); err == nil {
			st.docJSON = assembled
		}
	}
	// 自动保存：把权威文档写回用户的 .op 文件
	if st.docJSON != "" {
		_ = os.WriteFile(st.filePath, []byte(st.docJSON), 0o644)
	}
	st.version++
	return text, nil
}

// ---------- batch_design 翻译器（openpencil 语法 → Pen 语法） ----------

var opMap = map[string]string{
	"I": "Insert", "U": "Update", "D": "Delete", "M": "Move",
	"C": "Copy", "R": "Replace", "V": "SetVariables", "P": "Print", "G": "Generate",
}

// 词法级翻译：跳过字符串字面量，仅替换代码位置的标识符
func translateProgram(src string) string {
	var b strings.Builder
	n := len(src)
	i := 0
	for i < n {
		c := src[i]
		// 字符串字面量：原样复制
		if c == '"' || c == '\'' || c == '`' {
			j := i + 1
			for j < n {
				if src[j] == '\\' {
					j += 2
					continue
				}
				if src[j] == c {
					j++
					break
				}
				j++
			}
			b.WriteString(src[i:j])
			i = j
			continue
		}
		// 标识符
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
			j := i
			for j < n {
				ch := src[j]
				if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
					j++
				} else {
					break
				}
			}
			word := src[i:j]
			// 跳过空白，看是否紧跟 "("
			k := j
			for k < n && (src[k] == ' ' || src[k] == '\t' || src[k] == '\r' || src[k] == '\n') {
				k++
			}
			if k < n && src[k] == '(' {
				rep, ok := opMap[word]
				if ok {
					b.WriteString(rep)
					b.WriteString(src[j : k+1])
					// I(null, → Insert("document",
					if word == "I" {
						m := k + 1
						for m < n && (src[m] == ' ' || src[m] == '\t' || src[m] == '\r' || src[m] == '\n') {
							m++
						}
						if strings.HasPrefix(src[m:], "null") {
							after := m + 4
							if after >= n || src[after] == ',' || src[after] == ')' || src[after] == ' ' || src[after] == '\t' {
								b.WriteString(`"document"`)
								i = after
								continue
							}
						}
					}
					i = k + 1
					continue
				}
			}
			b.WriteString(src[i:j])
			i = j
			continue
		}
		b.WriteByte(c)
		i++
	}
	return b.String()
}

// ---------- get_selection：解析 Pen get_app_state 的选中节点 ----------

var selectedNodeRe = regexp.MustCompile("`([^`]+)`\\s*\\(([^)]+)\\)(?:\\s*:\\s*([^,`]+))?")

func selectionFromState(stateText string) map[string]any {
	line := ""
	for _, l := range strings.Split(stateText, "\n") {
		if strings.Contains(l, "Selected nodes:") {
			line = strings.TrimPrefix(l, "- ")
			break
		}
	}
	selected := []string{}
	nodes := []map[string]any{}
	if !strings.Contains(line, "No nodes are selected") {
		for _, m := range selectedNodeRe.FindAllStringSubmatch(line, -1) {
			id := strings.TrimSpace(m[1])
			selected = append(selected, id)
			node := map[string]any{"id": id}
			if m[2] != "" {
				node["type"] = strings.TrimSpace(m[2])
			}
			if m[3] != "" {
				node["name"] = strings.TrimSpace(m[3])
			}
			nodes = append(nodes, node)
		}
	}
	return map[string]any{
		"selectedIds":  selected,
		"nodes":        nodes,
		"activePageId": "",
	}
}

// ---------- HTTP / MCP 端点 ----------

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func checkToken(r *http.Request, st *hostState) bool {
	auth := r.Header.Get("authorization")
	xop := r.Header.Get("x-openpencil-token")
	return strings.TrimPrefix(auth, "Bearer ") == st.token || xop == st.token
}

func handleMCP(w http.ResponseWriter, r *http.Request, st *hostState) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]any{"jsonrpc": "2.0", "error": map[string]any{"code": -32600, "message": "method not allowed"}})
		return
	}
	var req struct {
		ID     any `json:"id"`
		Method string `json:"method"`
		Params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		} `json:"params"`
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, 200, map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32700, "message": "parse error"}})
		return
	}
	respond := func(result any, errMsg string) {
		if errMsg != "" {
			writeJSON(w, 200, map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32603, "message": errMsg}})
			return
		}
		writeJSON(w, 200, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}
	textResult := func(text string, isErr bool) {
		content := []map[string]any{{"type": "text", "text": text}}
		respond(map[string]any{"content": content, "isError": isErr}, "")
	}

	switch req.Params.Name {
	case "batch_design":
		operations, _ := req.Params.Arguments["operations"].(string)
		if operations == "" {
			textResult("batch_design: operations must not be empty", true)
			return
		}
		program := translateProgram(operations)
		text, err := applyWrite(st, program)
		if err != nil {
			textResult(fmt.Sprintf("## Failure during operation execution\n\n%s", err), true)
			return
		}
		textResult(text, false)
	case "update_node":
		nodeID, _ := req.Params.Arguments["nodeId"].(string)
		data, _ := req.Params.Arguments["data"].(map[string]any)
		if nodeID == "" || data == nil {
			textResult("update_node: nodeId and data are required", true)
			return
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			textResult("update_node: cannot encode data", true)
			return
		}
		program := fmt.Sprintf("Update(%s,%s)", jsonString(nodeID), string(encoded))
		text, err := applyWrite(st, program)
		if err != nil {
			textResult(fmt.Sprintf("## Failure during operation execution\n\n%s", err), true)
			return
		}
		textResult(text, false)
	case "get_selection":
		state, err := penMcpCall("get_app_state", map[string]any{
			"include_schema":              false,
			"include_canvas_design":       false,
			"include_scripts_and_shaders": false,
			"include_browser":             false,
		})
		if err != nil {
			textResult(fmt.Sprintf("get_selection failed: %s", err), true)
			return
		}
		sel := selectionFromState(state)
		encoded, _ := json.Marshal(sel)
		textResult(string(encoded), false)
	default:
		textResult(fmt.Sprintf("unknown tool: %s", req.Params.Name), true)
	}
}

func jsonString(s string) string {
	enc, _ := json.Marshal(s)
	return string(enc)
}

// ---------- 主流程 ----------

func main() {
	args := os.Args[1:]
	var filePath string
	port := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file":
			if i+1 < len(args) {
				filePath = args[i+1]
				i++
			}
		case "--port":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &port)
				i++
			}
		case "--serve-web", "--managed", "--allow-origin":
			if args[i] == "--allow-origin" && i+1 < len(args) {
				i++
			}
		}
	}
	if filePath == "" {
		fmt.Fprintln(os.Stderr, "op-host-web-server: --file is required")
		os.Exit(1)
	}

	// 强制清理残留 Pen 实例（锁冲突/Error 窗口会导致 active 错乱），
	// 确保 spawn 的是唯一干净实例
	killPen()

	// 准备 temp.pen 副本（固定路径复用，避免 Pen 窗口无限累积）
	tempDir := filepath.Join(os.TempDir(), "dsh-openpencil-host")
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "temp dir:", err)
		os.Exit(1)
	}
	tempPen := filepath.Join(tempDir, "preview.pen")
	raw, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read file:", err)
		os.Exit(1)
	}
	if err := os.WriteFile(tempPen, raw, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "write temp:", err)
		os.Exit(1)
	}

	st := &hostState{
		filePath:  filePath,
		tempPen:   tempPen,
		backupKey: backupPathFor(tempPen),
		version:   1,
		docJSON:   string(raw),
		token:     randToken(16),
	}
	var initialDoc map[string]any
	if err := json.Unmarshal(raw, &initialDoc); err != nil {
		fmt.Fprintln(os.Stderr, "parse file:", err)
		os.Exit(1)
	}
	st.initialDoc = initialDoc

	// 让 Pen 打开文档（已运行则 second-instance 开新窗口，否则启动 Pen）
	spawnPenFor(tempPen)
	// 后台预热：等待 Pen 就绪并修复文档一致性（host 在 batch_design 前完成）
	go func() {
		for i := 0; i < 3; i++ {
			if err := waitPenReady(st, 90*time.Second); err != nil {
				continue
			}
			if documentMatches(st) {
				break
			}
			fmt.Fprintln(os.Stderr, "op-host: document mismatch, restarting Pen cleanly")
			killPen()
			spawnPenFor(st.tempPen)
		}
		st.penReady.Store(true)
	}()

	// 监听随机端口
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<!doctype html><html><head><meta charset=\"utf-8\"><title>OpenPencil Host (Pen bridge)</title></head><body style=\"font-family:system-ui;padding:24px;background:#F5F7FA\"><h2>OpenPencil 托管编辑器（Pen 桥接）</h2><p>当前环境通过 Pen（OpenPencil 桌面版）引擎提供渲染与编辑能力。</p><p>画布编辑请使用 Pen 应用；Agent 可通过 openpencil_create / openpencil_edit 直接修改本文档。</p></body></html>"))
		case "/pkg/op_host_web.js":
			w.Header().Set("Content-Type", "application/javascript")
			_, _ = w.Write([]byte("// op_host_web.js (Pen bridge placeholder)\n"))
		case "/api/mcp/version":
			if !checkToken(r, st) {
				writeJSON(w, 401, map[string]any{"error": "unauthorized"})
				return
			}
			st.mu.Lock()
			v := st.version
			st.mu.Unlock()
			writeJSON(w, 200, map[string]any{"version": v})
		case "/api/mcp/document":
			if !checkToken(r, st) {
				writeJSON(w, 401, map[string]any{"error": "unauthorized"})
				return
			}
			if r.Method == http.MethodGet {
				st.mu.Lock()
				doc := st.docJSON
				v := st.version
				st.mu.Unlock()
				var parsed any
				if err := json.Unmarshal([]byte(doc), &parsed); err != nil {
					writeJSON(w, 500, map[string]any{"error": "invalid document"})
					return
				}
				writeJSON(w, 200, map[string]any{"document": parsed, "version": v})
				return
			}
			if r.Method == http.MethodPost {
				// 恢复文档：更新缓存并写回磁盘
				var body struct {
					Document json.RawMessage `json:"document"`
				}
				raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<22))
				if err := json.Unmarshal(raw, &body); err != nil || len(body.Document) == 0 {
					writeJSON(w, 400, map[string]any{"error": "invalid restore payload"})
					return
				}
				st.mu.Lock()
				st.docJSON = string(body.Document)
				_ = os.WriteFile(st.filePath, body.Document, 0o644)
				st.version++
				v := st.version
				st.mu.Unlock()
				writeJSON(w, 200, map[string]any{"ok": true, "version": v})
				return
			}
			writeJSON(w, 405, map[string]any{"error": "method not allowed"})
		case "/mcp":
			handleMCP(w, r, st)
		default:
			http.NotFound(w, r)
		}
	})

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()

	// stdout 输出 handshake（插件等待的第一行）
	hs, _ := json.Marshal(map[string]any{
		"ok":      true,
		"port":    actualPort,
		"token":   st.token,
		"version": 1,
	})
	fmt.Println(string(hs))
	// 保持进程存活
	select {}
}
