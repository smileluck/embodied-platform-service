// 型号管理 handlers（纯代理平台管理面 /api/v1/device-models + 物模型只读选择器）
package server

import (
	"github.com/gin-gonic/gin"
	devmodelsvc "github.com/smilex/smilex-admin-gin/internal/service/devmodel"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

func (s *HTTPServer) listDeviceModels(c *gin.Context) {
	page, size := pageParams(c)
	var req devmodelsvc.ListRequest
	_ = c.ShouldBindQuery(&req)
	models, pg, err := s.devmodel.List(c.Request.Context(), req, page, size)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, listResult{List: models, Page: pg})
}

func (s *HTTPServer) createDeviceModel(c *gin.Context) {
	var req devmodelsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	m, err := s.devmodel.Create(c.Request.Context(), req)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, m)
}

func (s *HTTPServer) getDeviceModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	m, err := s.devmodel.Get(c.Request.Context(), id)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, m)
}

func (s *HTTPServer) updateDeviceModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req devmodelsvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.devmodel.Update(c.Request.Context(), id, req); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteDeviceModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.devmodel.Delete(c.Request.Context(), id); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}

// 物模型只读选择器（型号创建表单数据源）

func (s *HTTPServer) listThingModelNodes(c *gin.Context) {
	var req devmodelsvc.TMNodesRequest
	_ = c.ShouldBindQuery(&req)
	nodes, err := s.devmodel.TMNodes(c.Request.Context(), req)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nodes)
}

func (s *HTTPServer) listThingModelVersions(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vs, err := s.devmodel.TMPublishedVersions(c.Request.Context(), id)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, vs)
}
