package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/igh-silkroad/api"
	"github.com/yourusername/igh-silkroad/internal/service"
)

// GradeHandler 质检记录查询处理器（Center 侧，只读）
type GradeHandler struct {
	gradeService *service.GradeService
}

// NewGradeHandler 创建质检记录查询处理器
func NewGradeHandler(gradeService *service.GradeService) *GradeHandler {
	return &GradeHandler{gradeService: gradeService}
}

// ListGrades godoc
// @Summary 查询质检记录
// @Tags grades
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param lot_id query string false "批次ID（按其下所有丝锭过滤）"
// @Param bobbin_id query string false "丝锭ID"
// @Param is_qualified query string false "合格状态：true / false / unspecified"
// @Success 200 {object} api.Response{data=api.PageResponse{list=[]service.GradeResponse}}
// @Router /v1/grades [get]
func (h *GradeHandler) ListGrades(c *gin.Context) {
	page, pageSize := api.ParsePagination(
		c.DefaultQuery("page", "1"),
		c.DefaultQuery("page_size", "20"),
	)

	filter := service.GradeListFilter{
		LotID:       c.Query("lot_id"),
		BobbinID:    c.Query("bobbin_id"),
		IsQualified: c.Query("is_qualified"),
	}

	grades, total, err := h.gradeService.ListGrades(c.Request.Context(), page, pageSize, filter)
	if err != nil {
		// is_qualified 取值非法属参数错误，其余归为服务端错误。
		if svcErr, ok := err.(*service.ServiceError); ok {
			c.JSON(http.StatusBadRequest, api.Error(svcErr.Code, svcErr.Message))
			return
		}
		c.JSON(http.StatusInternalServerError, api.Error(api.CodeServerError, err.Error()))
		return
	}

	c.JSON(http.StatusOK, api.PageSuccess(grades, page, pageSize, total))
}
