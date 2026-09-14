package file

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/internal/biz/file"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"gorm.io/gorm"
)

type repo struct {
	data *data.Data
}

// NewRepo 创建文件元数据仓储
func NewRepo(d *data.Data) file.Repo { return &repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return file.ErrFileNotFound
	}
	return err
}

func (r *repo) Create(ctx context.Context, f *file.File) error {
	po := model.FileToPO(f)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	// 回写自增主键与时间戳，供应用层返回给前端
	f.ID, f.CreatedAt = po.ID, po.CreatedAt
	return nil
}

func (r *repo) FindByID(ctx context.Context, id uint) (*file.File, error) {
	var po model.FilePO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	return model.FileFromPO(&po), nil
}

func (r *repo) Delete(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.FilePO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return file.ErrFileNotFound
	}
	return nil
}

func (r *repo) List(ctx context.Context, q file.Query, page, pageSize int) ([]*file.File, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.FilePO{})
	if q.Name != "" {
		// 转义用户输入中的 LIKE 通配符，防止 %/_ 改变匹配语义（通配符注入）
		tx = tx.Where("name LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Name)+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.FilePO
	if err := tx.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*file.File, 0, len(pos))
	for i := range pos {
		out = append(out, model.FileFromPO(&pos[i]))
	}
	return out, total, nil
}
