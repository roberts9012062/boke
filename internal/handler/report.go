// internal/handler/report.go
// 数据报表控制器（M4-报表，设计稿《数据报表》#235/#242）：
// overview 聚合 + 趋势 CSV 导出（文件流，不走统一 JSON 响应）。
package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/roberts9012062/boke/internal/service"
	"github.com/roberts9012062/boke/pkg/resp"
)

// ReportHandler 数据报表控制器（连接器类）。
type ReportHandler struct {
	report *service.ReportService // 报表服务
	logger *zap.Logger            // 错误日志（5xx 留痕）
}

// NewReportHandler 创建报表控制器。
func NewReportHandler(report *service.ReportService, logger *zap.Logger) *ReportHandler {
	return &ReportHandler{report: report, logger: logger}
}

// Overview 报表页聚合（GET /api/v1/admin/reports/overview?days=7|30）。
func (h *ReportHandler) Overview(c *gin.Context) {
	days := parseDays(c.Query("days"))
	data, err := h.report.Overview(c.Request.Context(), days)
	if err != nil {
		h.logger.Error("报表聚合失败", zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	resp.OK(c, data)
}

// ExportCSV 导出趋势 CSV（GET /api/v1/admin/reports/export.csv?days=；
// 附件下载：Content-Disposition attachment，不走统一 JSON 包装）。
func (h *ReportHandler) ExportCSV(c *gin.Context) {
	days := parseDays(c.Query("days"))
	content, err := h.report.ExportTrendCSV(c.Request.Context(), days)
	if err != nil {
		h.logger.Error("报表 CSV 导出失败", zap.Error(err))
		resp.FailFrom(c, err)
		return
	}
	// 附件响应（文件名带天数，便于区分 7/30 日视图）
	filename := "trend-" + strconv.Itoa(days) + "d.csv"
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Data(200, "text/csv; charset=utf-8", content)
}

// parseDays 解析天数参数（非法/缺省回退 30）。
func parseDays(raw string) int {
	days, err := strconv.Atoi(raw)
	if err != nil || (days != 7 && days != 30) {
		return 30
	}
	return days
}
