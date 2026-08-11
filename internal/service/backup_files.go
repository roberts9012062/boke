// internal/service/backup_files.go
// 备份文件打包辅助（M4-报表，纯函数）：目录/多文件打包 ZIP、行数组转 CSV。
// 说明：从 backup.go 拆分（保持单文件 ≤400 行）；全部纯函数，不修改入参。
package service

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// zipDir 打包目录为 ZIP（空目录生成空 zip；文件递归收集）。
func zipDir(dir string, zipPath string) error {
	files := make(map[string][]byte)
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		files[rel] = content
		return nil
	})
	return zipFiles(zipPath, files)
}

// zipFiles 多文件打包 ZIP（map：相对路径 → 内容）。
func zipFiles(zipPath string, files map[string][]byte) error {
	// 收集文件并排序（zip 条目顺序稳定，可重复构建）
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()
	writer := zip.NewWriter(f)
	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write(files[name]); err != nil {
			return err
		}
	}
	return writer.Close()
}

// zipCSVFiles 全站数据 → 每表 CSV 打包 ZIP。
func zipCSVFiles(zipPath string, data map[string][]map[string]any) error {
	files := make(map[string][]byte)
	for table, rows := range data {
		content, err := marshalCSV(rows)
		if err != nil {
			return err
		}
		files[table+".csv"] = content
	}
	return zipFiles(zipPath, files)
}

// marshalCSV 行数组 → CSV 字节（键序按首行稳定；UTF-8 BOM 便于 Excel）。
func marshalCSV(rows []map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buf)
	// 列序：取首行键（无数据时仅空表）
	keys := make([]string, 0)
	if len(rows) > 0 {
		for k := range rows[0] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
	}
	if err := writer.Write(keys); err != nil {
		return nil, err
	}
	for _, row := range rows {
		line := make([]string, 0, len(keys))
		for _, k := range keys {
			line = append(line, fmt.Sprintf("%v", row[k]))
		}
		if err := writer.Write(line); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// fileSize 文件大小（字节）。
func fileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}
