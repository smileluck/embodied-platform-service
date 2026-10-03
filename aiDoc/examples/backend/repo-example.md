<!-- last-updated: 2026-10-03 -->
# 示例：数据仓储 repo（GORM 实现）

> 目的：展示 `internal/data/<ctx>/repo.go` 的组织标准——PO 转换、错误映射、软删归档、安全分页。

## 核心原则

1. 实现 biz 定义的 Repo 接口，构造 `NewRepo(d *data.Data)`；所有操作 `r.data.DB.WithContext(ctx)`
2. GORM 错误必须映射为哨兵错误（`mapXxxErr` + `isUniqueViolation`），不让库错误穿透到上层
3. 写操作检查 `RowsAffected == 0` 返回 NotFound；模糊查询必须 `security.EscapeLike`；软删表删除配 `ArchiveUniqueColumns`

## 示例（提取自 internal/data/dict/repo.go，节选）

```go
type repo struct {
	data *data.Data
}

func NewRepo(d *data.Data) dict.Repo { return &repo{data: d} }

func mapTypeErr(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return dict.ErrTypeNotFound
	case isUniqueViolation(err):
		return dict.ErrTypeCodeExists
	}
	return err
}

// isUniqueViolation 各数据库唯一约束冲突文案判断：
// MySQL Error 1062、Postgres 23505（duplicate key value）、SQLite UNIQUE constraint failed
func isUniqueViolation(err error) bool { /* strings.Contains 三方言判断 */ }

func (r *repo) UpdateType(ctx context.Context, t *dict.DictType) error {
	res := r.data.DB.WithContext(ctx).Model(&model.DictTypePO{}).Where("id = ?", t.ID).
		Updates(map[string]interface{}{"name": t.Name, "code": t.Code, "status": int(t.Status)})
	if res.Error != nil {
		return mapTypeErr(res.Error)
	}
	if res.RowsAffected == 0 {
		return dict.ErrTypeNotFound // 幂等重复更新不误报成功
	}
	return nil
}

func (r *repo) DeleteType(ctx context.Context, id uint) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&model.DictTypePO{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return dict.ErrTypeNotFound
		}
		// 软删行仍占用 code 唯一索引，归档释放以便同编码重建
		return data.ArchiveUniqueColumns(tx, "dict_types", id, map[string]int{"code": 64})
	})
}

func (r *repo) ListTypes(ctx context.Context, q dict.Query, page, pageSize int) ([]*dict.DictType, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.DictTypePO{})
	if q.Name != "" {
		tx = tx.Where("name LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Name)+"%")
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil { return nil, 0, err }
	var pos []model.DictTypePO
	if err := tx.Scopes(paginate(page, pageSize)).Order("id DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*dict.DictType, 0, len(pos))
	for i := range pos {
		out = append(out, model.DictTypeFromPO(&pos[i]))
	}
	return out, total, nil
}

// paginate pageSize<=0 表示全量（引用数据整表拉取）；否则按页取
func paginate(page, pageSize int) func(db *gorm.DB) *gorm.DB { /* Offset/Limit */ }
```

## 要点

- PO ↔ 领域转换统一走 `internal/data/model` 的 `XxxToPO`/`XxxFromPO`，repo 不手写字段搬运
- 约束冲突靠库层唯一索引兜底（并发安全），biz 层查重只是前置友好提示——两处错误语义一致
- 新表/新列同步 `migrations/{mysql,postgres,sqlite}.sql` 三方言与 `internal/data/data.go:migrateAndSeed`

## 真实参考文件

- `internal/data/dict/repo.go`
- `internal/data/archive_unique.go`（唯一槽位释放工具）
- `internal/data/model/model.go`（PO 与转换函数）
