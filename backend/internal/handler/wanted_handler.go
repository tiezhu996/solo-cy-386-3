package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/marketpal/marketpal/internal/dto"
	"github.com/marketpal/marketpal/internal/middleware"
	"github.com/marketpal/marketpal/internal/service"
	"github.com/marketpal/marketpal/internal/util"
)

// WantedHandler 求购需求 HTTP 处理器。
type WantedHandler struct {
	svc *service.WantedService
}

// NewWantedHandler 构造求购需求处理器。
func NewWantedHandler(svc *service.WantedService) *WantedHandler {
	return &WantedHandler{svc: svc}
}

// Create POST /api/v1/wanteds
func (h *WantedHandler) Create(c *gin.Context) {
	var req dto.WantedCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "求购发布失败：参数校验不通过 "+err.Error())
		return
	}
	wanted, err := h.svc.Create(middleware.GetUserID(c), req)
	if err != nil {
		util.AbortWithError(c, err)
		return
	}
	util.OKMessage(c, "求购发布成功", dto.FromWanted(wanted))
}

// Close POST /api/v1/wanteds/:id/close
func (h *WantedHandler) Close(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "求购关闭失败：求购 id 参数非法")
		return
	}
	wanted, err := h.svc.Close(middleware.GetUserID(c), uint(id))
	if err != nil {
		util.AbortWithError(c, err)
		return
	}
	util.OKMessage(c, "求购已关闭", dto.FromWanted(wanted))
}

// Detail GET /api/v1/wanteds/:id
func (h *WantedHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "求购详情失败：求购 id 参数非法")
		return
	}
	wanted, err := h.svc.GetDetail(uint(id))
	if err != nil {
		util.AbortWithError(c, err)
		return
	}
	util.OK(c, dto.FromWanted(wanted))
}

// List GET /api/v1/wanteds
func (h *WantedHandler) List(c *gin.Context) {
	var q dto.WantedQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		util.Fail(c, http.StatusBadRequest, 40000, "求购大厅失败：查询参数不合法 "+err.Error())
		return
	}
	res, err := h.svc.List(q)
	if err != nil {
		util.AbortWithError(c, err)
		return
	}
	util.OK(c, res)
}

// MyWanteds GET /api/v1/wanteds/mine
func (h *WantedHandler) MyWanteds(c *gin.Context) {
	page := parseInt(c.DefaultQuery("page", "1"), 1)
	pageSize := parseInt(c.DefaultQuery("page_size", "10"), 10)
	res, err := h.svc.ListMine(middleware.GetUserID(c), page, pageSize)
	if err != nil {
		util.AbortWithError(c, err)
		return
	}
	util.OK(c, res)
}
