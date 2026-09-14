package file

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/conf"
	"github.com/smilex/smilex-admin-gin/pkg/logger"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
	"go.uber.org/zap"
)

// Usecase 文件管理领域用例
type Usecase struct {
	repo    Repo
	storage *StorageManager
	cfg     *conf.Bootstrap
}

func NewUsecase(repo Repo, storage *StorageManager, c *conf.Bootstrap) *Usecase {
	return &Usecase{repo: repo, storage: storage, cfg: c}
}

// defaultDenyExts 内置禁止上传的扩展名（可执行/脚本类，防 webshell）
var defaultDenyExts = map[string]bool{
	"exe": true, "sh": true, "bat": true, "cmd": true, "com": true, "msi": true,
	"scr": true, "ps1": true, "dll": true, "so": true, "jar": true,
	"jsp": true, "jspx": true, "php": true, "asp": true, "aspx": true,
}

// inlineTypes 允许浏览器内联预览的 Content-Type 白名单；
// 其余类型（含 svg/html 等可携带脚本的类型）下载一律强制 attachment
var inlineTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "image/bmp": true,
	"application/pdf": true, "text/plain": true, "video/mp4": true, "audio/mpeg": true,
}

// IsInlineAllowed 该 Content-Type 是否允许内联预览
func IsInlineAllowed(contentType string) bool {
	ct, _, _ := strings.Cut(contentType, ";")
	return inlineTypes[strings.TrimSpace(ct)]
}

func (uc *Usecase) denyExts() map[string]bool {
	if len(uc.cfg.Storage.DenyExts) == 0 {
		return defaultDenyExts
	}
	m := make(map[string]bool, len(uc.cfg.Storage.DenyExts))
	for _, e := range uc.cfg.Storage.DenyExts {
		m[strings.ToLower(strings.TrimPrefix(strings.TrimSpace(e), "."))] = true
	}
	return m
}

// Upload 校验大小/类型后写入当前存储后端并落库（落库失败尽力删除已上传对象）
func (uc *Usecase) Upload(ctx context.Context, name string, r io.Reader, size int64, uploaderID uint, uploaderName string) (*File, error) {
	if max := uc.cfg.Storage.MaxSizeMB << 20; max > 0 && size > max {
		// 上限参数由传输层按配置渲染（见 HTTPServer.fileErr），这里只返回哨兵错误
		return nil, ErrFileTooLarge
	}
	name = path.Base(strings.ReplaceAll(name, "\\", "/")) // 防 Windows 路径文件名
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	if uc.denyExts()[ext] {
		return nil, ErrFileTypeDenied
	}
	// 嗅探 Content-Type（前 512 字节），不轻信客户端声明
	head := make([]byte, 512)
	n, _ := io.ReadFull(r, head)
	head = head[:n]
	contentType := http.DetectContentType(head)
	r = io.MultiReader(bytes.NewReader(head), r)

	f := &File{
		Driver:       uc.storage.Current().Driver(),
		ObjectKey:    genObjectKey(ext),
		Name:         name,
		Ext:          ext,
		Size:         size,
		ContentType:  contentType,
		UploaderID:   uploaderID,
		UploaderName: uploaderName,
	}
	if err := uc.storage.Current().Put(ctx, f.ObjectKey, r, size, contentType); err != nil {
		return nil, err
	}
	if err := uc.repo.Create(ctx, f); err != nil {
		if derr := uc.storage.Current().Delete(context.Background(), f.ObjectKey); derr != nil {
			logger.Warn("file metadata save failed, orphan object cleanup failed",
				zap.String("key", f.ObjectKey), zap.Error(derr))
		}
		return nil, err
	}
	return f, nil
}

// Download 下载/预览解析结果：云存储返回预签名 URL（URL 非空），本地存储返回内容流（Body 非空）
type Download struct {
	File        *File
	URL         string        // 预签名下载地址（云存储；鉴权通过后签发，短时效）
	Body        io.ReadCloser // 本地存储代理内容流
	ContentType string
	Inline      bool // 是否允许内联预览（否则强制 attachment）
}

// ResolveDownload 按文件记录落库时的 driver 解析存储后端：
// 云存储签发预签名 URL，本地存储打开内容流 —— 后端升级后旧文件仍走原后端读取
func (uc *Usecase) ResolveDownload(ctx context.Context, id uint) (*Download, error) {
	f, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	backend, err := uc.storage.For(f.Driver)
	if err != nil {
		return nil, err
	}
	d := &Download{File: f, ContentType: f.ContentType, Inline: IsInlineAllowed(f.ContentType)}
	expire := time.Duration(uc.cfg.Storage.SignExpireMinutes) * time.Minute
	if expire <= 0 {
		expire = 15 * time.Minute
	}
	url, err := backend.PresignGet(ctx, f.ObjectKey, expire)
	switch {
	case err == nil:
		d.URL = url
		return d, nil
	case errors.Is(err, ErrPresignUnsupported):
		body, err := backend.Get(ctx, f.ObjectKey)
		if err != nil {
			return nil, err
		}
		d.Body = body
		return d, nil
	default:
		return nil, err
	}
}

// Delete 删除元数据并尽力删除存储对象（对象删除失败仅告警，不阻塞）
func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	f, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	if backend, err := uc.storage.For(f.Driver); err == nil {
		if derr := backend.Delete(ctx, f.ObjectKey); derr != nil {
			logger.Warn("file object delete failed", zap.String("driver", f.Driver),
				zap.String("key", f.ObjectKey), zap.Error(derr))
		}
	} else {
		logger.Warn("file object backend unavailable, object left behind",
			zap.String("driver", f.Driver), zap.String("key", f.ObjectKey))
	}
	return nil
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*File, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*File, pagination.Page, error) {
	files, total, err := uc.repo.List(ctx, q, page, pageSize)
	return files, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// genObjectKey 服务端生成对象 key：yyyy/mm/<随机串>.<ext>，杜绝用户控制存储路径
func genObjectKey(ext string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	key := time.Now().Format("2006/01") + "/" + hex.EncodeToString(b)
	if ext != "" {
		key += "." + ext
	}
	return key
}
