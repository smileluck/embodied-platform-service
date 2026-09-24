// 告警通知 handler：通知渠道 / 告警规则 / 发送记录（protected）
package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	biznotify "github.com/smilex/smilex-admin-gin/internal/biz/notify"
	notifysvc "github.com/smilex/smilex-admin-gin/internal/service/notify"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

// ---- 通知渠道 ----

func (s *HTTPServer) listNotifyChannels(c *gin.Context) {
	list, err := s.notify.ListChannels(c.Request.Context())
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, list)
}

func (s *HTTPServer) createNotifyChannel(c *gin.Context) {
	var req notifysvc.ChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	ch, err := s.notify.CreateChannel(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, ch)
}

func (s *HTTPServer) getNotifyChannel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	ch, err := s.notify.GetChannel(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, ch)
}

func (s *HTTPServer) updateNotifyChannel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req notifysvc.ChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if _, err := s.notify.UpdateChannel(c.Request.Context(), id, req); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteNotifyChannel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.notify.DeleteChannel(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

// testNotifyChannel 渠道测试：返回发送记录（failed 时 error 字段含原因）
func (s *HTTPServer) testNotifyChannel(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	rec, err := s.notify.TestChannel(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, rec)
}

// ---- 告警规则 ----

func (s *HTTPServer) listNotifyRules(c *gin.Context) {
	list, err := s.notify.ListRules(c.Request.Context())
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, list)
}

func (s *HTTPServer) createNotifyRule(c *gin.Context) {
	var req notifysvc.RuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	r, err := s.notify.CreateRule(c.Request.Context(), req)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, r)
}

func (s *HTTPServer) getNotifyRule(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	r, err := s.notify.GetRule(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, r)
}

func (s *HTTPServer) updateNotifyRule(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req notifysvc.RuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if _, err := s.notify.UpdateRule(c.Request.Context(), id, req); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteNotifyRule(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.notify.DeleteRule(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

// ---- 发送记录 ----

func (s *HTTPServer) listNotifyRecords(c *gin.Context) {
	page, size := s.pageParams(c)
	channelID, _ := strconv.ParseUint(c.Query("channel_id"), 10, 64)
	q := biznotify.RecordQuery{
		ChannelID: uint(channelID),
		Source:    c.Query("source"),
		Status:    c.Query("status"),
	}
	list, pg, err := s.notify.ListRecords(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) clearNotifyRecords(c *gin.Context) {
	if err := s.notify.ClearRecords(c.Request.Context()); err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}
