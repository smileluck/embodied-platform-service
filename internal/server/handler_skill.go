// 技能 handler：多文件技能包 CRUD（protected）
package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	bizskill "github.com/smilex/smilex-admin-gin/internal/biz/skill"
	skillsvc "github.com/smilex/smilex-admin-gin/internal/service/skill"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

func (s *HTTPServer) listSkills(c *gin.Context) {
	page, size := s.pageParams(c)
	q := bizskill.Query{Kw: c.Query("kw")}
	if st := c.Query("status"); st != "" {
		v, err := strconv.Atoi(st)
		if err != nil || (v != 0 && v != 1) {
			response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
			return
		}
		q.Status = &v
	}
	list, pg, err := s.skill.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createSkill(c *gin.Context) {
	var req skillsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	sk, err := s.skill.Create(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, sk)
}

func (s *HTTPServer) getSkill(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sk, err := s.skill.Get(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, sk)
}

func (s *HTTPServer) updateSkill(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req skillsvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.skill.Update(c.Request.Context(), id, req); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteSkill(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.skill.Delete(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}
