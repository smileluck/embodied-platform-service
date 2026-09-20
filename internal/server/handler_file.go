package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	bizfile "github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/server/middleware"
	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/response"
	"io"
)

// ---- 文件 ----

func (s *HTTPServer) listFiles(c *gin.Context) {
	page, size := pageParams(c)
	files, pg, err := s.file.List(c.Request.Context(), bizfile.Query{Name: c.Query("name")}, page, size)
	if err != nil {
		response.FailI18n(c, http.StatusInternalServerError, response.CodeErr, err)
		return
	}
	response.OK(c, listResult{List: files, Page: pg})
}

func (s *HTTPServer) uploadFile(c *gin.Context) {
	sub := middleware.Subject(c)
	fh, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, i18n.T(c.Request.Context(), "file.no_file"))
		return
	}
	if max := s.cfg.Storage.MaxSizeMB << 20; max > 0 && fh.Size > max {
		response.BadRequest(c, i18n.T(c.Request.Context(), "file.too_large", s.cfg.Storage.MaxSizeMB))
		return
	}
	src, err := fh.Open()
	if err != nil {
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
		return
	}
	defer src.Close()
	vo, err := s.file.Upload(c.Request.Context(), fh.Filename, src, fh.Size, sub.UserID, sub.Username)
	if err != nil {
		s.fileErr(c, err)
		return
	}
	response.OK(c, vo)
}

func (s *HTTPServer) downloadFile(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	d, err := s.file.ResolveDownload(c.Request.Context(), id)
	if err != nil {
		s.fileErr(c, err)
		return
	}
	// 平台存储：鉴权通过后 302 到短时效预签名 URL（云后端原生预签名 / local 后端网关代理 URL）
	if d.URL != "" {
		c.Redirect(http.StatusFound, d.URL)
		return
	}
	// 历史本地驱动：后端代理流式输出
	defer d.Body.Close()
	disposition := "attachment"
	if d.Inline && c.Query("download") == "" {
		disposition = "inline"
	}
	c.Header("Content-Type", d.ContentType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", contentDisposition(disposition, d.File.Name))
	if d.File.Size > 0 {
		c.Header("Content-Length", strconv.FormatInt(d.File.Size, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, d.Body)
}

func (s *HTTPServer) deleteFile(c *gin.Context) {
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := s.file.Delete(c.Request.Context(), id); err != nil {
		s.fileErr(c, err)
		return
	}
	response.OK(c, nil)
}

// fileErr 文件操作错误映射：不存在 404，入参类 400，存储后端未配置 503，
// 平台存储对象已清理（platform 404）→ 404「文件已失效」，其余 500
func (s *HTTPServer) fileErr(c *gin.Context, err error) {
	switch {
	case isErr(err, bizfile.ErrFileNotFound):
		response.FailI18n(c, http.StatusNotFound, response.CodeErr, err)
	case isErr(err, bizfile.ErrFileTooLarge):
		// 上限参数按当前配置渲染（biz 层只返回哨兵错误）
		response.BadRequest(c, i18n.T(c.Request.Context(), "file.too_large", s.cfg.Storage.MaxSizeMB))
	case isErr(err, bizfile.ErrFileTypeDenied):
		response.FailI18n(c, http.StatusBadRequest, response.CodeErr, err)
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
