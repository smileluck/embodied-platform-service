// 定时任务 handler
package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	bizjob "github.com/smilex/smilex-admin-gin/internal/biz/job"
	jobsvc "github.com/smilex/smilex-admin-gin/internal/service/job"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

func (s *HTTPServer) listJobs(c *gin.Context) {
	page, size := s.pageParams(c)
	list, pg, err := s.job.List(c.Request.Context(), bizjob.Query{Name: c.Query("name")}, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) listJobHandlers(c *gin.Context) {
	response.OK(c, s.job.Handlers())
}

func (s *HTTPServer) createJob(c *gin.Context) {
	var req jobsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	j, err := s.job.Create(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, j)
}

func (s *HTTPServer) getJob(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	j, err := s.job.Get(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, j)
}

func (s *HTTPServer) updateJob(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req jobsvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.job.Update(c.Request.Context(), id, req); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) setJobStatus(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req jobsvc.StatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.job.SetStatus(c.Request.Context(), id, req.Status); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteJob(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.job.Delete(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) runJobOnce(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.job.RunOnce(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) listJobLogs(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	page, size := s.pageParams(c)
	list, pg, err := s.job.ListLogs(c.Request.Context(), id, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}
