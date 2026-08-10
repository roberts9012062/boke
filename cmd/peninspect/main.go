// cmd/peninspect/main.go
// boke.pen 设计文件检查工具：列出全部画板（页面）清单，或输出指定画板的全部文本内容。
//
// 用途：在无法直接查看设计稿图片时，从 pen.dev 源文件还原页面结构与文案。
// 用法：
//   go run ./cmd/peninspect              # 仅列出画板清单
//   go run ./cmd/peninspect 首页         # 输出名称包含「首页」的画板全部文本
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// node 设计文件节点（仅保留关心的字段，其余忽略）。
type node struct {
	Type     string          `json:"type"`
	Name     string          `json:"name"`
	Content  string          `json:"content"`
	Width    json.RawMessage `json:"width"`
	Height   json.RawMessage `json:"height"`
	Children []node          `json:"children"`
}

// fileDoc 设计文件根结构。
type fileDoc struct {
	Version  string `json:"version"`
	Children []node `json:"children"`
}

// renderDimension 渲染尺寸字段（可能是数字或 fill_container 等字符串）。
func renderDimension(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "-"
	}
	var num float64
	if err := json.Unmarshal(raw, &num); err == nil {
		return fmt.Sprintf("%.0f", num)
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	return string(raw)
}

// walkTexts 递归收集节点内全部文本内容。
func walkTexts(n node, out *[]string) {
	if n.Type == "text" && n.Content != "" {
		*out = append(*out, n.Content)
	}
	for _, child := range n.Children {
		walkTexts(child, out)
	}
}

// printArtboards 打印全部画板清单（名称与尺寸）。
func printArtboards(doc fileDoc) {
	fmt.Printf("设计文件版本：%s，共 %d 个画板：\n", doc.Version, len(doc.Children))
	for i, frame := range doc.Children {
		if frame.Type != "frame" {
			continue
		}
		dim := fmt.Sprintf("%s x %s", renderDimension(frame.Width), renderDimension(frame.Height))
		fmt.Printf("  %2d. %s（%s）\n", i+1, frame.Name, dim)
	}
}

// printArtboardTexts 输出名称包含关键字的所有画板的全部文本。
func printArtboardTexts(doc fileDoc, keyword string) {
	for _, frame := range doc.Children {
		if !strings.Contains(frame.Name, keyword) {
			continue
		}
		fmt.Printf("\n=== 画板：%s（%s x %s）===\n",
			frame.Name, renderDimension(frame.Width), renderDimension(frame.Height))
		texts := make([]string, 0)
		walkTexts(frame, &texts)
		for _, t := range texts {
			fmt.Println("  •", t)
		}
	}
}

func main() {
	// 读取设计文件
	raw, err := os.ReadFile("boke.pen")
	if err != nil {
		fmt.Println("[失败] 读取 boke.pen 失败：", err)
		os.Exit(1)
	}

	var doc fileDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		fmt.Println("[失败] 解析 boke.pen 失败：", err)
		os.Exit(1)
	}

	// 无参数：仅列画板清单；有参数：输出匹配画板的文本
	if len(os.Args) > 1 {
		printArtboardTexts(doc, os.Args[1])
	} else {
		printArtboards(doc)
	}
}
