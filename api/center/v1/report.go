package v1

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// ReportHandler 报表处理器
type ReportHandler struct {
	reportService *service.ReportService
}

// NewReportHandler 创建报表处理器
func NewReportHandler(reportService *service.ReportService) *ReportHandler {
	return &ReportHandler{
		reportService: reportService,
	}
}

// QualityDistribution godoc
// @Summary 质量分布
// @Tags reports
// @Produce json
// @Param start query string false "区间起点（RFC3339）"
// @Param end query string false "区间终点（RFC3339）"
// @Param top query int false "缺陷原因 Top N" default(10)
// @Success 200 {object} api.Response{data=service.QualityDistributionResponse}
// @Failure 400 {object} api.ErrorResponse
// @Router /v1/reports/quality-distribution [get]
func (h *ReportHandler) QualityDistribution(c *gin.Context) {
	start, end, err := parseTimeRange(c.Query("start"), c.Query("end"), time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	top, err := parseTopN(c.Query("top"))
	if err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	resp, err := h.reportService.QualityDistribution(c.Request.Context(), start, end, top)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// MachineOperator godoc
// @Summary 机台与操作员
// @Tags reports
// @Produce json
// @Param start query string false "区间起点（RFC3339）"
// @Param end query string false "区间终点（RFC3339）"
// @Success 200 {object} api.Response{data=service.MachineOperatorResponse}
// @Failure 400 {object} api.ErrorResponse
// @Router /v1/reports/machine-operator [get]
func (h *ReportHandler) MachineOperator(c *gin.Context) {
	start, end, err := parseTimeRange(c.Query("start"), c.Query("end"), time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, api.Error(api.CodeInvalidParams, err.Error()))
		return
	}

	resp, err := h.reportService.MachineOperator(c.Request.Context(), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.Success(resp))
}

// 报表端点的区间与数量约束。
const (
	// reportDefaultWindow 是缺省统计窗口：spec §5.2「缺省为最近 7 天」。
	reportDefaultWindow = 7 * 24 * time.Hour

	// reportMaxWindow 是允许的最大区间跨度。
	//
	// 这不是 spec 的要求，而是本实现加的防护：两个报表端点都是 GROUP BY
	// 全表聚合，bobbin_grade 长期运行后会达到百万级，无上限的区间等于
	// 提供一个 DoS 入口。366 天足够覆盖日终、月度与年度三种真实用法。
	reportMaxWindow = 366 * 24 * time.Hour

	// reportDefaultTopN 与 reportMaxTopN 约束缺陷原因 Top N 的行数。
	// 上限与 api.ParsePagination 的 pageSize 上限同理，属防御性约束。
	reportDefaultTopN = 10
	reportMaxTopN     = 50
)

var (
	errEndNotAfterStart = errors.New("end 必须晚于 start")
	errRangeTooLong     = errors.New("区间跨度不得超过 366 天")
)

// parseTimeRange 解析报表端点的 start / end 参数，返回左闭右开区间。
//
// 缺省规则（spec 2026-10-12 §5.2）：
//   - 都不传 → [now-7d, now)
//   - 只传 start → [start, now)
//   - 只传 end → [end-7d, end)
//
// now 由调用方传入而非内部取 time.Now()，否则缺省窗口无法在测试中稳定断言。
// 返回值一律转为 UTC：数据库存储的是 UTC，本地日界换算不在本端点职责内。
func parseTimeRange(startStr, endStr string, now time.Time) (time.Time, time.Time, error) {
	var (
		start time.Time
		end   time.Time
	)

	if endStr != "" {
		parsed, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("end 参数格式非法（需 RFC3339）: %w", err)
		}
		end = parsed.UTC()
	} else {
		end = now.UTC()
	}

	if startStr != "" {
		parsed, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("start 参数格式非法（需 RFC3339）: %w", err)
		}
		start = parsed.UTC()
	} else {
		start = end.Add(-reportDefaultWindow)
	}

	if !end.After(start) {
		return time.Time{}, time.Time{}, errEndNotAfterStart
	}
	if end.Sub(start) > reportMaxWindow {
		return time.Time{}, time.Time{}, errRangeTooLong
	}

	return start, end, nil
}

// parseTopN 解析缺陷原因 Top N 的行数。
//
// 越界一律报错而非静默截断：静默修正会让前端以为拿到了自己请求的数量，
// 排查报表与预期不符时无从发现。
func parseTopN(raw string) (int, error) {
	if raw == "" {
		return reportDefaultTopN, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("top 参数必须为整数: %w", err)
	}
	if n < 1 || n > reportMaxTopN {
		return 0, fmt.Errorf("top 参数必须在 1 到 %d 之间（得到 %d）", reportMaxTopN, n)
	}

	return n, nil
}
