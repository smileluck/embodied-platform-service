package server

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	bizblacklist "github.com/smilex/smilex-admin-gin/internal/biz/blacklist"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	blacklistsvc "github.com/smilex/smilex-admin-gin/internal/service/blacklist"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	"strings"
)

// ---- IP 黑名单 ----

func (s *HTTPServer) listIPBlacklist(c *gin.Context) {
	page, size := s.pageParams(c)
	list, pg, err := s.blacklist.List(c.Request.Context(), bizblacklist.Query{IP: c.Query("ip")}, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: list, Page: pg})
}

func (s *HTTPServer) createIPBlacklist(c *gin.Context) {
	sub := middleware.Subject(c)
	var req blacklistsvc.CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "common.invalid_params"))
		return
	}
	// 防呆：禁止封禁当前操作者自己的 IP（归一化后比较，避免自我锁定）
	if ip := net.ParseIP(strings.TrimSpace(req.IP)); ip != nil && ip.String() == c.ClientIP() {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, bizblacklist.ErrSelfBan)
		return
	}
	vo, err := s.blacklist.Create(c.Request.Context(), req, sub.UserID, sub.Username)
	if err != nil {
		if isErr(err, bizblacklist.ErrIPExists) {
			response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
			return
		}
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) deleteIPBlacklist(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.blacklist.Delete(c.Request.Context(), id); err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	response.OK(c, nil)
}
