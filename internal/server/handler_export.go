package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	bizexport "github.com/smilex/smilex-admin-gin/internal/biz/export"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	"io"
	"strings"
)

// ---- 异步导出 ----

// submitExport 提交导出任务：原始查询条件（query，剔除分页参数）快照进记录，
// worker 按同一套条件分批拉数，保证导出结果与列表页所见一致
func (s *HTTPServer) submitExport(c *gin.Context, biz string) {
	sub := middleware.Subject(c)
	params := c.Request.URL.Query()
	params.Del("page")
	params.Del("page_size")
	vo, err := s.export.Submit(c.Request.Context(), biz, params, sub.UserID, sub.Username)
	if err != nil {
		switch {
		case isErr(err, bizexport.ErrQueueFull):
			response.FailI18n(c, http.StatusTooManyRequests, response.CodeErr, err)
		case isErr(err, bizexport.ErrUnsupportedBiz):
			response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		default:
			response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		}
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) listExports(c *gin.Context) {
	sub := middleware.Subject(c)
	if c.Query("recent") == "1" {
		vos, err := s.export.Recent(c.Request.Context(), sub.UserID, 5)
		if err != nil {
			response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
			return
		}
		response.OK(c, vos)
		return
	}
	page, size := s.pageParams(c)
	vos, pg, err := s.export.List(c.Request.Context(), sub.UserID, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: vos, Page: pg})
}

func (s *HTTPServer) downloadExport(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sub := middleware.Subject(c)
	d, err := s.export.ResolveDownload(c.Request.Context(), id, sub.UserID)
	if err != nil {
		s.exportErr(c, err)
		return
	}
	// 平台存储：鉴权通过后 302 到短时效预签名 URL
	if d.URL != "" {
		c.Redirect(http.StatusFound, d.URL)
		return
	}
	// 历史本地驱动：后端代理流式输出（强制 attachment + nosniff，CSV 不内联渲染）
	defer d.Body.Close()
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", contentDisposition("attachment", d.Record.Name))
	if d.Record.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(d.Record.Size, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, d.Body)
}

func (s *HTTPServer) deleteExport(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	sub := middleware.Subject(c)
	if err := s.export.Delete(c.Request.Context(), id, sub.UserID); err != nil {
		s.exportErr(c, err)
		return
	}
	response.OK(c, nil)
}

// exportErr 导出操作错误映射：不存在 404，越权 403，未完成 409，队列满 429，存储后端未配置 503，
// 平台存储对象已清理（platform 404）→ 404「文件已失效」，其余 500
func (s *HTTPServer) exportErr(c *gin.Context, err error) {
	switch {
	case isErr(err, bizexport.ErrNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, bizexport.ErrNotOwner):
		response.FailI18n(c, http.StatusForbidden, response.CodeForbidden, err)
	case isErr(err, bizexport.ErrNotReady):
		response.FailI18n(c, http.StatusConflict, response.CodeErr, err)
	case isErr(err, bizexport.ErrQueueFull):
		response.FailI18n(c, http.StatusTooManyRequests, response.CodeErr, err)
	case isErr(err, bizfile.ErrDriverUnavailable):
		response.FailI18n(c, http.StatusServiceUnavailable, response.CodeErr, err)
	default:
		var perr *platformError
		if errorsAs(err, &perr) && perr.HTTPStatus == http.StatusNotFound {
			response.FailI18n(c, http.StatusNotFound, response.CodeErr, bizfile.ErrFileGone)
			return
		}
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
	}
}

// contentDisposition 生成 Content-Disposition 头：ASCII 回退名 + RFC 5987 UTF-8 编码名

func contentDisposition(disposition, filename string) string {
	fallback := strings.Map(func(r rune) rune {
		if r < 32 || r > 126 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, filename)
	return fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, fallback, url.PathEscape(filename))
}

// parseUnixParam 解析 unix 秒级时间戳查询参数（空/非法返回 false 表示不限）
func parseUnixParam(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(n, 0), true
}
