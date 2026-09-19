package file

import (
	"context"
	"errors"
)

// ErrFileNotFound 文件不存在
var ErrFileNotFound = errors.New("文件不存在")

// ErrFileTooLarge 文件超出大小上限
var ErrFileTooLarge = errors.New("文件大小超出限制")

// ErrFileTypeDenied 文件类型被禁止上传
var ErrFileTypeDenied = errors.New("该类型文件禁止上传")

// ErrFileGone 文件已失效（本地元数据在、平台存储对象已被清理/桶被删）
var ErrFileGone = errors.New("文件已失效（存储对象不存在），请重新上传")

// Query 文件列表查询条件
type Query struct {
	Name string // 文件名模糊
}

// Repo 文件元数据仓储接口
type Repo interface {
	Create(ctx context.Context, f *File) error
	FindByID(ctx context.Context, id uint) (*File, error)
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, q Query, page, pageSize int) ([]*File, int64, error)
}
