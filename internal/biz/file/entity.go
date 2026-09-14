package file

import "time"

// File 文件元数据（json tag 与前端 FileInfo 类型字段对齐）
type File struct {
	ID           uint      `json:"id"`
	Driver       string    `json:"driver"`      // 落库时的存储后端（local | oss | cos | tos | minio）
	ObjectKey    string    `json:"-"`           // 对象 key，服务端内部使用，不下发
	Name         string    `json:"name"`        // 原始文件名
	Ext          string    `json:"ext"`         // 扩展名（小写，不含点）
	Size         int64     `json:"size"`        // 字节数
	ContentType  string    `json:"content_type"`
	UploaderID   uint      `json:"uploader_id"`
	UploaderName string    `json:"uploader_name"`
	CreatedAt    time.Time `json:"created_at"`
}
