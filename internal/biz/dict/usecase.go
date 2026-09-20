package dict

import (
	"context"
	"errors"
	"strings"

	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// Usecase 数据字典领域用例
type Usecase struct {
	repo Repo
}

func NewUsecase(repo Repo) *Usecase { return &Usecase{repo: repo} }

// TypeInput 类型写入参数（更新时空字符串=保持原值）
type TypeInput struct {
	Name   string
	Code   string
	Remark string
	Status Status
}

func (uc *Usecase) CreateType(ctx context.Context, in TypeInput) (*DictType, error) {
	in.Code = strings.TrimSpace(in.Code)
	if _, err := uc.repo.FindTypeByCode(ctx, in.Code); err == nil {
		return nil, ErrTypeCodeExists
	} else if !errors.Is(err, ErrTypeNotFound) {
		return nil, err
	}
	t := &DictType{Name: in.Name, Code: in.Code, Remark: in.Remark, Status: in.Status}
	if err := uc.repo.CreateType(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (uc *Usecase) UpdateType(ctx context.Context, id uint, in TypeInput) error {
	t, err := uc.repo.FindTypeByID(ctx, id)
	if err != nil {
		return err
	}
	if in.Code = strings.TrimSpace(in.Code); in.Code == "" {
		in.Code = t.Code
	}
	if in.Code != t.Code {
		if _, err := uc.repo.FindTypeByCode(ctx, in.Code); err == nil {
			return ErrTypeCodeExists
		} else if !errors.Is(err, ErrTypeNotFound) {
			return err
		}
		t.Code = in.Code
	}
	if in.Name != "" {
		t.Name = in.Name
	}
	if in.Remark != "" {
		t.Remark = in.Remark
	}
	t.Status = in.Status
	return uc.repo.UpdateType(ctx, t)
}

func (uc *Usecase) DeleteType(ctx context.Context, id uint) error {
	if n, err := uc.repo.CountItemsByType(ctx, id); err != nil {
		return err
	} else if n > 0 {
		return ErrTypeHasItems
	}
	return uc.repo.DeleteType(ctx, id)
}

func (uc *Usecase) GetType(ctx context.Context, id uint) (*DictType, error) {
	return uc.repo.FindTypeByID(ctx, id)
}

func (uc *Usecase) ListTypes(ctx context.Context, q Query, page, pageSize int) ([]*DictType, pagination.Page, error) {
	list, total, err := uc.repo.ListTypes(ctx, q, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// ItemInput 字典项写入参数
type ItemInput struct {
	TypeID uint
	Label  string
	Value  string
	Sort   int
	Remark string
	Status Status
}

func (uc *Usecase) CreateItem(ctx context.Context, in ItemInput) (*DictItem, error) {
	if _, err := uc.repo.FindTypeByID(ctx, in.TypeID); err != nil {
		return nil, err
	}
	if err := uc.checkItemDup(ctx, in.TypeID, in.Label, in.Value, 0); err != nil {
		return nil, err
	}
	i := &DictItem{
		TypeID: in.TypeID, Label: in.Label, Value: in.Value,
		Sort: in.Sort, Remark: in.Remark, Status: in.Status,
	}
	if err := uc.repo.CreateItem(ctx, i); err != nil {
		return nil, err
	}
	return i, nil
}

func (uc *Usecase) UpdateItem(ctx context.Context, id uint, in ItemInput) error {
	i, err := uc.repo.FindItemByID(ctx, id)
	if err != nil {
		return err
	}
	if in.TypeID != 0 && in.TypeID != i.TypeID {
		if _, err := uc.repo.FindTypeByID(ctx, in.TypeID); err != nil {
			return err
		}
		i.TypeID = in.TypeID
	}
	if err := uc.checkItemDup(ctx, i.TypeID, in.Label, in.Value, id); err != nil {
		return err
	}
	i.Label = in.Label
	i.Value = in.Value
	i.Sort = in.Sort
	if in.Remark != "" {
		i.Remark = in.Remark
	}
	i.Status = in.Status
	return uc.repo.UpdateItem(ctx, i)
}

func (uc *Usecase) DeleteItem(ctx context.Context, id uint) error {
	return uc.repo.DeleteItem(ctx, id)
}

func (uc *Usecase) GetItem(ctx context.Context, id uint) (*DictItem, error) {
	return uc.repo.FindItemByID(ctx, id)
}

// ListItems 管理端分页（含禁用项）
func (uc *Usecase) ListItems(ctx context.Context, typeID uint, page, pageSize int) ([]*DictItem, pagination.Page, error) {
	if _, err := uc.repo.FindTypeByID(ctx, typeID); err != nil {
		return nil, pagination.Page{}, err
	}
	list, total, err := uc.repo.ListItems(ctx, typeID, page, pageSize)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	return list, pagination.Page{Page: page, PageSize: pageSize, Total: total}, nil
}

// ItemsByCode 消费入口：按类型编码取启用项（sort 升序）
func (uc *Usecase) ItemsByCode(ctx context.Context, code string) ([]*DictItem, error) {
	return uc.repo.ListEnabledItemsByCode(ctx, strings.TrimSpace(code))
}

// checkItemDup 同类型下 label/value 查重
func (uc *Usecase) checkItemDup(ctx context.Context, typeID uint, label, value string, excludeID uint) error {
	dup, err := uc.repo.FindItemByLabelOrValue(ctx, typeID, label, value, excludeID)
	if err != nil {
		return err
	}
	if dup != nil {
		return ErrItemExists
	}
	return nil
}
