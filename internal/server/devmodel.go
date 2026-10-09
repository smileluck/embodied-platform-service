// 型号管理 handlers（纯代理平台管理面 /api/v1/device-models + 物模型只读选择器）
package server

import (
	"strconv"

	"github.com/gin-gonic/gin"
	devmodelsvc "github.com/smilex/smilex-admin-gin/internal/service/devmodel"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

func (s *HTTPServer) listDeviceModels(c *gin.Context) {
	page, size := s.pageParams(c)
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

// ---- 物模型读面扩展 + 写面（2026-10-09；错误一律 platformErr 透传）----

func (s *HTTPServer) getThingModelNode(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	n, err := s.devmodel.TMNode(c.Request.Context(), id)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, n)
}

func (s *HTTPServer) createThingModelNode(c *gin.Context) {
	var req devmodelsvc.TMNodeCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	n, err := s.devmodel.CreateTMNode(c.Request.Context(), req)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, n)
}

func (s *HTTPServer) updateThingModelNode(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req devmodelsvc.TMNodeUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.devmodel.UpdateTMNode(c.Request.Context(), id, req); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteThingModelNode(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.devmodel.DeleteTMNode(c.Request.Context(), id); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) createThingModelDraft(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	v, err := s.devmodel.CreateTMDraft(c.Request.Context(), id)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, v)
}

func (s *HTTPServer) updateThingModelDraft(c *gin.Context) {
	vid, ok := mappingIDParam(c, "vid")
	if !ok {
		return
	}
	var req devmodelsvc.TMSchemaUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.devmodel.UpdateTMDraft(c.Request.Context(), vid, req); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) publishThingModelVersion(c *gin.Context) {
	vid, ok := mappingIDParam(c, "vid")
	if !ok {
		return
	}
	v, err := s.devmodel.PublishTMVersion(c.Request.Context(), vid)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, v)
}

// rollbackThingModelVersion 回滚到指定已发布版本（body 可省略，默认仅创建回滚草稿不直接发布）
func (s *HTTPServer) rollbackThingModelVersion(c *gin.Context) {
	vid, ok := mappingIDParam(c, "vid")
	if !ok {
		return
	}
	var req devmodelsvc.TMRollbackRequest
	_ = c.ShouldBindJSON(&req)
	v, err := s.devmodel.RollbackTMVersion(c.Request.Context(), vid, req)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, v)
}

func (s *HTTPServer) deleteThingModelVersion(c *gin.Context) {
	vid, ok := mappingIDParam(c, "vid")
	if !ok {
		return
	}
	if err := s.devmodel.DeleteTMVersion(c.Request.Context(), vid); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}

// resolveThingModel 合并解析节点完整 Schema（?version_id= 可选 pin 版本）
func (s *HTTPServer) resolveThingModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	pinned, _ := strconv.ParseUint(c.Query("version_id"), 10, 64)
	res, err := s.devmodel.ResolveTM(c.Request.Context(), id, uint(pinned))
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, res)
}

func (s *HTTPServer) getThingModelInheritanceStatus(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	st, err := s.devmodel.TMInheritanceStatus(c.Request.Context(), id)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, st)
}

// ---- 数据映射（纯代理平台开放面 /data-mappings；错误一律 platformErr 透传）----

// mappingIDParam 解析指定名称的 uint 路径参数（vid/mid；id 走公共 idParam）
func mappingIDParam(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return 0, false
	}
	return uint(id), true
}

func (s *HTTPServer) listMappingDefs(c *gin.Context) {
	page, size := s.pageParams(c)
	var req devmodelsvc.MappingListRequest
	_ = c.ShouldBindQuery(&req)
	defs, pg, err := s.mapping.List(c.Request.Context(), req, page, size)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, listResult{List: defs, Page: pg})
}

func (s *HTTPServer) createMappingDef(c *gin.Context) {
	var req devmodelsvc.MappingCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	res, err := s.mapping.Create(c.Request.Context(), req)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, res)
}

func (s *HTTPServer) getMappingDef(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	def, err := s.mapping.Get(c.Request.Context(), id)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, def)
}

func (s *HTTPServer) deleteMappingDef(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.mapping.Delete(c.Request.Context(), id); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}

// getPublishedMapping 型号当前生效映射版本（?model_id=；无已发布版本返回 null）
func (s *HTTPServer) getPublishedMapping(c *gin.Context) {
	modelID, err := strconv.ParseUint(c.Query("model_id"), 10, 64)
	if err != nil || modelID == 0 {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	v, err := s.mapping.PublishedForModel(c.Request.Context(), uint(modelID))
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, v)
}

func (s *HTTPServer) listMappingVersions(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	vs, err := s.mapping.Versions(c.Request.Context(), id)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, listResult{List: vs})
}

func (s *HTTPServer) createMappingDraft(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req devmodelsvc.MappingDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	v, err := s.mapping.CreateDraft(c.Request.Context(), id, req)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, v)
}

func (s *HTTPServer) getMappingVersion(c *gin.Context) {
	vid, ok := mappingIDParam(c, "vid")
	if !ok {
		return
	}
	v, err := s.mapping.GetVersion(c.Request.Context(), vid)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, v)
}

func (s *HTTPServer) updateMappingDraft(c *gin.Context) {
	vid, ok := mappingIDParam(c, "vid")
	if !ok {
		return
	}
	var req devmodelsvc.MappingDraftUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.mapping.UpdateDraft(c.Request.Context(), vid, req); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) publishMapping(c *gin.Context) {
	vid, ok := mappingIDParam(c, "vid")
	if !ok {
		return
	}
	v, err := s.mapping.Publish(c.Request.Context(), vid)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, v)
}

// rollbackMapping 回滚到指定已发布版本（body 可省略，默认仅创建回滚草稿不直接发布）
func (s *HTTPServer) rollbackMapping(c *gin.Context) {
	vid, ok := mappingIDParam(c, "vid")
	if !ok {
		return
	}
	var req devmodelsvc.MappingRollbackRequest
	_ = c.ShouldBindJSON(&req)
	v, err := s.mapping.Rollback(c.Request.Context(), vid, req)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, v)
}

func (s *HTTPServer) deleteMappingVersion(c *gin.Context) {
	vid, ok := mappingIDParam(c, "vid")
	if !ok {
		return
	}
	if err := s.mapping.DeleteVersion(c.Request.Context(), vid); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) listMappingBoundModels(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	models, err := s.mapping.BoundModels(c.Request.Context(), id)
	if err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, listResult{List: models})
}

func (s *HTTPServer) bindMappingModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req devmodelsvc.MappingBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.mapping.BindModel(c.Request.Context(), id, req); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) unbindMappingModel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	mid, ok := mappingIDParam(c, "mid")
	if !ok {
		return
	}
	if err := s.mapping.UnbindModel(c.Request.Context(), id, mid); err != nil {
		s.platformErr(c, err)
		return
	}
	response.OK(c, nil)
}
