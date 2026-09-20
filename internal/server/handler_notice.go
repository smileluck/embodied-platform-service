// 通知公告 handler：管理端 CRUD（protected）+ 消费端（basic：生效列表/未读数/已读上报）
package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	biznotice "github.com/smilex/smilex-admin-gin/internal/biz/notice"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	noticesvc "github.com/smilex/smilex-admin-gin/internal/service/notice"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
)

func (s *HTTPServer) listNotices(c *gin.Context) {
	page, size := s.pageParams(c)
	q := biznotice.Query{Title: c.Query("title"), Level: c.Query("level"), Status: c.Query("status")}
	list, pg, err := s.notice.List(c.Request.Context(), q, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createNotice(c *gin.Context) {
	var req noticesvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	sub := middleware.Subject(c)
	n, err := s.notice.Create(c.Request.Context(), req, sub.UserID, sub.Username)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, n)
}

func (s *HTTPServer) getNotice(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	n, err := s.notice.Get(c.Request.Context(), id)
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, n)
}

func (s *HTTPServer) updateNotice(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	var req noticesvc.UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	if err := s.notice.Update(c.Request.Context(), id, req); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

func (s *HTTPServer) deleteNotice(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.notice.Delete(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}

// ---- 消费端 ----

func (s *HTTPServer) listActiveNotices(c *gin.Context) {
	sub := middleware.Subject(c)
	list, err := s.notice.ListActive(c.Request.Context(), sub.UserID)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, list)
}

func (s *HTTPServer) unreadNoticeCount(c *gin.Context) {
	sub := middleware.Subject(c)
	n, err := s.notice.CountUnread(c.Request.Context(), sub.UserID)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, gin.H{"count": n})
}

func (s *HTTPServer) readNotice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	sub := middleware.Subject(c)
	if err := s.notice.MarkRead(c.Request.Context(), sub.UserID, uint(id)); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}
