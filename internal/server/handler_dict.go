// 数据字典 handler：类型 CRUD + 项 CRUD；按编码取项挂在 basic 组（登录即可消费）。
package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	bizdict "github.com/smilex/smilex-admin-gin/internal/biz/dict"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	dictsvc "github.com/smilex/smilex-admin-gin/internal/service/dict"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// dictErr 字典错误映射（未注册 i18n 的走原错误文本）
func (s *HTTPServer) dictErr(c *gin.Context, err error) {
	response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
}

// ---- 字典类型 ----

func (s *HTTPServer) listDictTypes(c *gin.Context) {
	page, size := pageParams(c)
	q := bizdict.Query{Name: c.Query("name"), Code: c.Query("code")}
	list, pg, err := s.dict.ListTypes(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createDictType(c *gin.Context) {
	var req dictsvc.TypeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	t, err := s.dict.CreateType(c.Request.Context(), req)
	if err != nil {
		s.dictErr(c, err)
		return
	}
	response.OK(c, t)
}

func (s *HTTPServer) getDictType(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	t, err := s.dict.GetType(c.Request.Context(), id)
	if err != nil {
		s.dictErr(c, err)
		return
	}
	response.OK(c, t)
}

func (s *HTTPServer) updateDictType(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req dictsvc.TypeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.dict.UpdateType(c.Request.Context(), id, req); err != nil {
		s.dictErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteDictType(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.dict.DeleteType(c.Request.Context(), id); err != nil {
		s.dictErr(c, err)
		return
	}
	response.OK(c, nil)
}

// ---- 字典项 ----

func (s *HTTPServer) listDictItems(c *gin.Context) {
	typeID, err := strconv.ParseUint(c.Param("typeID"), 10, 64)
	if err != nil || typeID == 0 {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	page, size := pageParams(c)
	list, pg, err := s.dict.ListItems(c.Request.Context(), uint(typeID), page, size)
	if err != nil {
		s.dictErr(c, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createDictItem(c *gin.Context) {
	var req dictsvc.ItemCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	i, err := s.dict.CreateItem(c.Request.Context(), req)
	if err != nil {
		s.dictErr(c, err)
		return
	}
	response.OK(c, i)
}

func (s *HTTPServer) updateDictItem(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req dictsvc.ItemUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.dict.UpdateItem(c.Request.Context(), id, req); err != nil {
		s.dictErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteDictItem(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.dict.DeleteItem(c.Request.Context(), id); err != nil {
		s.dictErr(c, err)
		return
	}
	response.OK(c, nil)
}

// listDictItemsByCode 消费入口：按类型编码取启用项（basic 组，登录即可）
func (s *HTTPServer) listDictItemsByCode(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	_ = middleware.Subject(c) // 仅确认已认证（basic 组已保证）
	list, err := s.dict.ItemsByCode(c.Request.Context(), code)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, list)
}
