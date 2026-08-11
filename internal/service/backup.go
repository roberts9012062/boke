// internal/service/backup.go
// 备份导出业务（M4-报表，设计稿《备份导出》#237/#244）：
// 备份类型（全站数据/媒体库）+ 范围（内容/用户/媒体）+ 保留天数 + 导出格式（JSON/CSV/ZIP）。
//
// 实现：应用级导出（标准库 encoding/json / encoding/csv / archive/zip，零外部依赖）；
//       pg_dump 后置（本机无 PostgreSQL 客户端，db_dump 字段留空，差异记录）。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/repository"
	"github.com/roberts9012062/boke/pkg/errs"
)

// 备份类型（设计稿：全站数据 / 媒体库）。
const (
	BackupTypeAll   = "all"   // 全站数据（内容+用户+媒体 按范围）
	BackupTypeMedia = "media" // 媒体库（data/media 目录打包）
)

// 导出格式（设计稿：JSON / CSV / ZIP）。
const (
	BackupFormatJSON = "json" // 单 JSON 文件
	BackupFormatCSV  = "csv"  // 每表 CSV，打包 ZIP
	BackupFormatZIP  = "zip"  // 数据 JSON + manifest，打包 ZIP
)

// 备份文件目录名（相对 dataDir）。
const backupDirName = "backup"

// BackupInput 创建备份输入（设计稿表单字段）。
type BackupInput struct {
	BackupType   string   `json:"backup_type"`   // all / media
	Scope        []string `json:"scope"`         // content/users/media（媒体库忽略）
	Format       string   `json:"format"`        // json/csv/zip（媒体库锁定 zip）
	RetentionDays int      `json:"retention_days"` // 保留天数（1-365，默认 30）
}

// BackupDTO 备份记录 DTO（后台列表）。
type BackupDTO struct {
	ID        int64  `json:"id"`         // 记录 ID
	Type      string `json:"type"`       // 备份类型
	Status    string `json:"status"`     // success / failed
	FileName  string `json:"file_name"`  // 文件名（下载展示）
	FileSize  int64  `json:"file_size"`  // 大小（字节）
	CreatedAt string `json:"created_at"` // 备份时间
}

// BackupService 备份服务（连接器类）。
type BackupService struct {
	backup  *repository.BackupRepo // 备份记录
	dataDir string                 // 本地数据目录（备份文件存 dataDir/backup/）
	logger  *zap.Logger            // 日志（失败留痕）
}

// NewBackupService 创建备份服务。
func NewBackupService(backup *repository.BackupRepo, dataDir string, logger *zap.Logger) *BackupService {
	return &BackupService{backup: backup, dataDir: dataDir, logger: logger}
}

// backupDir 备份文件目录（纯函数：dataDir/backup）。
func (s *BackupService) backupDir() string {
	return filepath.Join(s.dataDir, backupDirName)
}

// CreateBackup 创建备份（生成文件 → 落库 → 过期清理）。
// 说明：失败时落库 failed 记录（file_path 为空），便于后台排查。
func (s *BackupService) CreateBackup(ctx context.Context, input BackupInput) (*BackupDTO, error) {
	// 校验输入（媒体库锁定 zip 格式，范围忽略）
	if err := validateBackupInput(&input); err != nil {
		return nil, err
	}

	// 执行备份（生成文件；失败落 failed 记录后返回错误）
	fileName, fileSize, err := s.generateBackup(ctx, input)
	if err != nil {
		_, _ = s.backup.Create(ctx, repository.BackupRecord{
			Type: repository.BackupManual, Status: repository.BackupFailed,
		})
		s.logger.Error("创建备份失败", zap.Error(err))
		return nil, errs.New(errs.CodeInternal, "备份失败："+err.Error())
	}

	// 落库成功记录
	id, err := s.backup.Create(ctx, repository.BackupRecord{
		Type: repository.BackupManual, Status: repository.BackupSuccess,
		FilePath: filepath.Join(s.backupDir(), fileName), FileSize: fileSize,
	})
	if err != nil {
		return nil, err
	}

	// 过期清理（保留天数内新备份不受影响；清理失败不阻断主流程）
	s.cleanupExpired(ctx, input.RetentionDays)

	return &BackupDTO{ID: id, Type: input.BackupType, Status: repository.BackupSuccess,
		FileName: fileName, FileSize: fileSize, CreatedAt: time.Now().Format(time.RFC3339)}, nil
}

// validateBackupInput 校验备份输入（媒体库强制 zip；范围去重去非法值）。
func validateBackupInput(input *BackupInput) error {
	if input.BackupType != BackupTypeAll && input.BackupType != BackupTypeMedia {
		return errs.New(errs.CodeBadRequest, "备份类型仅支持 all（全站数据）/ media（媒体库）")
	}
	if input.RetentionDays < 1 || input.RetentionDays > 365 {
		return errs.New(errs.CodeBadRequest, "保留天数需为 1-365 天")
	}
	if input.BackupType == BackupTypeMedia {
		input.Format = BackupFormatZIP // 媒体库固定 ZIP
		return nil
	}
	if input.Format != BackupFormatJSON && input.Format != BackupFormatCSV && input.Format != BackupFormatZIP {
		return errs.New(errs.CodeBadRequest, "导出格式仅支持 json / csv / zip")
	}
	// 范围规范化（去重 + 仅合法值；为空视为默认全选）
	valid := map[string]bool{repository.ScopeContent: true, repository.ScopeUsers: true, repository.ScopeMedia: true}
	seen := make(map[string]bool)
	scopes := make([]string, 0, len(input.Scope))
	for _, s := range input.Scope {
		if valid[s] && !seen[s] {
			seen[s] = true
			scopes = append(scopes, s)
		}
	}
	if len(scopes) == 0 {
		scopes = []string{repository.ScopeContent, repository.ScopeUsers, repository.ScopeMedia}
	}
	input.Scope = scopes
	return nil
}

// generateBackup 生成备份文件（返回：文件名、大小）。
func (s *BackupService) generateBackup(ctx context.Context, input BackupInput) (string, int64, error) {
	if err := os.MkdirAll(s.backupDir(), 0o755); err != nil {
		return "", 0, err
	}
	stamp := time.Now().Format("20060102-150405")

	// 媒体库备份：打包 data/media 目录
	if input.BackupType == BackupTypeMedia {
		fileName := fmt.Sprintf("backup_media_%s.zip", stamp)
		if err := zipDir(s.mediaDir(), filepath.Join(s.backupDir(), fileName)); err != nil {
			return "", 0, err
		}
		size, err := fileSize(filepath.Join(s.backupDir(), fileName))
		return fileName, size, err
	}

	// 全站数据：按格式生成（CSV 多表打包为 zip，扩展名用 .zip）
	ext := input.Format
	if input.Format == BackupFormatCSV {
		ext = "zip"
	}
	fileName := fmt.Sprintf("backup_all_%s.%s", stamp, ext)
	path := filepath.Join(s.backupDir(), fileName)
	switch input.Format {
	case BackupFormatJSON:
		if err := writeJSONBackup(ctx, s, input, path); err != nil {
			return "", 0, err
		}
	case BackupFormatCSV:
		if err := writeCSVBackup(ctx, s, input, path); err != nil {
			return "", 0, err
		}
	case BackupFormatZIP:
		if err := writeZIPBackup(ctx, s, input, path); err != nil {
			return "", 0, err
		}
	}
	size, err := fileSize(path)
	return fileName, size, err
}

// mediaDir 媒体目录（data/media）。
func (s *BackupService) mediaDir() string {
	return filepath.Join(s.dataDir, "media")
}

// exportData 读取按范围导出的全站数据（表名 → 行）。
func (s *BackupService) exportData(ctx context.Context, input BackupInput) (map[string][]map[string]any, error) {
	return s.backup.ExportAllData(ctx, input.Scope)
}

// writeJSONBackup 全站数据 → 单 JSON 文件。
func writeJSONBackup(ctx context.Context, s *BackupService, input BackupInput, path string) error {
	data, err := s.exportData(ctx, input)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

// writeCSVBackup 全站数据 → 每表 CSV → 打包 ZIP。
func writeCSVBackup(ctx context.Context, s *BackupService, input BackupInput, path string) error {
	data, err := s.exportData(ctx, input)
	if err != nil {
		return err
	}
	return zipCSVFiles(path, data)
}

// writeZIPBackup 全站数据 → 数据 JSON + manifest.json → 打包 ZIP。
func writeZIPBackup(ctx context.Context, s *BackupService, input BackupInput, path string) error {
	data, err := s.exportData(ctx, input)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	manifest, err := json.MarshalIndent(map[string]any{
		"app":        "yueyan-blog",
		"exported_at": time.Now().Format(time.RFC3339),
		"scopes":     input.Scope,
	}, "", "  ")
	if err != nil {
		return err
	}
	return zipFiles(path, map[string][]byte{
		"data.json":   payload,
		"manifest.json": manifest,
	})
}

// ---------- 列表 / 下载 / 删除 ----------

// List 备份记录列表（DTO 组装）。
func (s *BackupService) List(ctx context.Context) ([]BackupDTO, error) {
	records, err := s.backup.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]BackupDTO, 0, len(records))
	for _, r := range records {
		items = append(items, BackupDTO{
			ID: r.ID, Type: r.Type, Status: r.Status,
			FileName: filepath.Base(r.FilePath), FileSize: r.FileSize,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
		})
	}
	return items, nil
}

// Download 备份文件下载信息（校验记录存在 + 路径安全）。
// 返回：文件绝对路径、文件名、大小。
func (s *BackupService) Download(ctx context.Context, id int64) (string, string, int64, error) {
	record, found, err := s.backup.FindByID(ctx, id)
	if err != nil {
		return "", "", 0, err
	}
	if !found {
		return "", "", 0, errs.ErrNotFound
	}
	if record.Status != repository.BackupSuccess || record.FilePath == "" {
		return "", "", 0, errs.New(errs.CodeStateConflict, "该备份无可用文件")
	}
	path, err := s.safePath(record.FilePath)
	if err != nil {
		return "", "", 0, err
	}
	if _, err := os.Stat(path); err != nil {
		return "", "", 0, errs.New(errs.CodeNotFound, "备份文件已丢失")
	}
	return path, filepath.Base(path), record.FileSize, nil
}

// Delete 删除备份（文件 + 记录）。
func (s *BackupService) Delete(ctx context.Context, id int64) error {
	record, found, err := s.backup.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return errs.ErrNotFound
	}
	// 清理磁盘文件（路径安全校验；失败记录日志不阻断记录删除）
	if record.FilePath != "" {
		if path, err := s.safePath(record.FilePath); err == nil {
			_ = os.Remove(path)
		} else {
			s.logger.Warn("删除备份文件路径校验失败", zap.Int64("id", id), zap.Error(err))
		}
	}
	return s.backup.Delete(ctx, id)
}

// safePath 路径安全校验（纯函数：filePath 必须以备份目录为前缀，防越权读取/删除）。
func (s *BackupService) safePath(filePath string) (string, error) {
	abs := filepath.Clean(filePath)
	dir := filepath.Clean(s.backupDir())
	if !strings.HasPrefix(abs, dir+string(os.PathSeparator)) {
		return "", errors.New("备份文件路径越权")
	}
	return abs, nil
}

// cleanupExpired 过期清理（删除同类型早于保留天数的备份：文件 + 记录）。
// 说明：仅清理「本次备份类型」的过期记录；清理失败静默（观测性操作）。
func (s *BackupService) cleanupExpired(ctx context.Context, retentionDays int) {
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	paths, err := s.backup.DeleteOlderThan(ctx, repository.BackupManual, cutoff)
	if err != nil {
		s.logger.Warn("备份过期清理查询失败", zap.Error(err))
		return
	}
	for _, p := range paths {
		if path, err := s.safePath(p); err == nil {
			_ = os.Remove(path)
		}
	}
}

