// Package dict 数据字典仓储 GORM 实现
package dict

import (
	"context"
	"errors"

	"github.com/smilex/smilex-admin-gin/internal/biz/dict"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
	"gorm.io/gorm"
)

type repo struct {
	data *data.Data
}

func NewRepo(d *data.Data) dict.Repo { return &repo{data: d} }

func mapTypeErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dict.ErrTypeNotFound
	}
	return err
}

func mapItemErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return dict.ErrItemNotFound
	}
	return err
}

func (r *repo) CreateType(ctx context.Context, t *dict.DictType) error {
	po := model.DictTypeToPO(t)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	t.ID, t.CreatedAt, t.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) UpdateType(ctx context.Context, t *dict.DictType) error {
	return r.data.DB.WithContext(ctx).Model(&model.DictTypePO{}).Where("id = ?", t.ID).
		Updates(map[string]interface{}{
			"name": t.Name, "code": t.Code, "remark": t.Remark, "status": int(t.Status),
		}).Error
}

func (r *repo) DeleteType(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.DictTypePO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return dict.ErrTypeNotFound
	}
	return nil
}

func (r *repo) FindTypeByID(ctx context.Context, id uint) (*dict.DictType, error) {
	var po model.DictTypePO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapTypeErr(err)
	}
	return model.DictTypeFromPO(&po), nil
}

func (r *repo) FindTypeByCode(ctx context.Context, code string) (*dict.DictType, error) {
	var po model.DictTypePO
	if err := r.data.DB.WithContext(ctx).Where("code = ?", code).First(&po).Error; err != nil {
		return nil, mapTypeErr(err)
	}
	return model.DictTypeFromPO(&po), nil
}

func (r *repo) ListTypes(ctx context.Context, q dict.Query, page, pageSize int) ([]*dict.DictType, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.DictTypePO{})
	if q.Name != "" {
		tx = tx.Where("name LIKE ? ESCAPE '/'", security.EscapeLike(q.Name)+"%")
	}
	if q.Code != "" {
		tx = tx.Where("code LIKE ? ESCAPE '/'", security.EscapeLike(q.Code)+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.DictTypePO
	if err := tx.Scopes(paginate(page, pageSize)).
		Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*dict.DictType, 0, len(pos))
	for i := range pos {
		out = append(out, model.DictTypeFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *repo) CountItemsByType(ctx context.Context, typeID uint) (int64, error) {
	var n int64
	err := r.data.DB.WithContext(ctx).Model(&model.DictItemPO{}).
		Where("type_id = ?", typeID).Count(&n).Error
	return n, err
}

func (r *repo) CreateItem(ctx context.Context, i *dict.DictItem) error {
	po := model.DictItemToPO(i)
	if err := r.data.DB.WithContext(ctx).Create(po).Error; err != nil {
		return err
	}
	i.ID, i.CreatedAt, i.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) UpdateItem(ctx context.Context, i *dict.DictItem) error {
	return r.data.DB.WithContext(ctx).Model(&model.DictItemPO{}).Where("id = ?", i.ID).
		Updates(map[string]interface{}{
			"type_id": i.TypeID, "label": i.Label, "value": i.Value,
			"sort": i.Sort, "remark": i.Remark, "status": int(i.Status),
		}).Error
}

func (r *repo) DeleteItem(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.DictItemPO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return dict.ErrItemNotFound
	}
	return nil
}

func (r *repo) FindItemByID(ctx context.Context, id uint) (*dict.DictItem, error) {
	var po model.DictItemPO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapItemErr(err)
	}
	return model.DictItemFromPO(&po), nil
}

// FindItemByLabelOrValue 同类型下按 label/value 查重；未命中返回 (nil, nil)
func (r *repo) FindItemByLabelOrValue(ctx context.Context, typeID uint, label, value string, excludeID uint) (*dict.DictItem, error) {
	var po model.DictItemPO
	tx := r.data.DB.WithContext(ctx).Where("type_id = ? AND (label = ? OR value = ?)", typeID, label, value)
	if excludeID > 0 {
		tx = tx.Where("id <> ?", excludeID)
	}
	err := tx.First(&po).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return model.DictItemFromPO(&po), nil
}

func (r *repo) ListItems(ctx context.Context, typeID uint, page, pageSize int) ([]*dict.DictItem, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.DictItemPO{}).Where("type_id = ?", typeID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.DictItemPO
	if err := tx.Scopes(paginate(page, pageSize)).
		Order("sort ASC, id ASC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*dict.DictItem, 0, len(pos))
	for i := range pos {
		out = append(out, model.DictItemFromPO(&pos[i]))
	}
	return out, total, nil
}

func (r *repo) ListEnabledItemsByCode(ctx context.Context, code string) ([]*dict.DictItem, error) {
	var pos []model.DictItemPO
	err := r.data.DB.WithContext(ctx).
		Joins("JOIN dict_types ON dict_types.id = dict_items.type_id AND dict_types.code = ? AND dict_types.status = 1", code).
		Where("dict_items.status = 1").
		Order("dict_items.sort ASC, dict_items.id ASC").
		Find(&pos).Error
	if err != nil {
		return nil, err
	}
	out := make([]*dict.DictItem, 0, len(pos))
	for i := range pos {
		out = append(out, model.DictItemFromPO(&pos[i]))
	}
	return out, nil
}

// paginate pageSize<=0 表示全量（引用数据整表拉取）；否则按页取
func paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if pageSize > 0 {
			return db.Offset((page - 1) * pageSize).Limit(pageSize)
		}
		return db
	}
}
