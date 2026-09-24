// Package notice 通知公告仓储 GORM 实现
package notice

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/smilex/smilex-admin-gin/internal/biz/notice"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"github.com/smilex/smilex-admin-gin/pkg/security"
)

type repo struct {
	data *data.Data
}

func NewRepo(d *data.Data) notice.Repo { return &repo{data: d} }

func mapErr(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return notice.ErrNotFound
	}
	return err
}

// deliveredCondition 送达条件：全体广播 OR 命中定向目标（角色经 platform_user_roles 展开、用户直接命中；userID 为平台用户 ID）
func deliveredCondition(userID uint) (string, []interface{}) {
	return "(notices.scope = ? OR notices.id IN (SELECT notice_id FROM notice_targets WHERE " +
			"(target_type = ? AND target_id IN (SELECT role_id FROM platform_user_roles WHERE platform_user_id = ?)) " +
			"OR (target_type = ? AND target_id = ?)))",
		[]interface{}{string(notice.ScopeAll), notice.TargetRole, userID, notice.TargetUser, userID}
}

// insertTargets 写入定向目标（scope=all 或无目标时跳过）
func insertTargets(tx *gorm.DB, noticeID uint, n *notice.Notice) error {
	if n.Scope == notice.ScopeAll {
		return nil
	}
	rows := make([]model.NoticeTargetPO, 0, len(n.RoleIDs)+len(n.UserIDs))
	for _, id := range n.RoleIDs {
		rows = append(rows, model.NoticeTargetPO{NoticeID: noticeID, TargetType: notice.TargetRole, TargetID: id})
	}
	for _, id := range n.UserIDs {
		rows = append(rows, model.NoticeTargetPO{NoticeID: noticeID, TargetType: notice.TargetUser, TargetID: id})
	}
	if len(rows) == 0 {
		return nil
	}
	return tx.Create(&rows).Error
}

// loadTargets 批量回填定向目标（管理端回显用，避免逐条查询）
func (r *repo) loadTargets(ctx context.Context, list []*notice.Notice) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(list))
	for _, n := range list {
		if n.Scope != notice.ScopeAll {
			ids = append(ids, n.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var targets []model.NoticeTargetPO
	if err := r.data.DB.WithContext(ctx).Where("notice_id IN ?", ids).Find(&targets).Error; err != nil {
		return err
	}
	byNotice := make(map[uint]*notice.Notice, len(list))
	for _, n := range list {
		byNotice[n.ID] = n
	}
	for _, t := range targets {
		n := byNotice[t.NoticeID]
		if n == nil {
			continue
		}
		if t.TargetType == notice.TargetRole {
			n.RoleIDs = append(n.RoleIDs, t.TargetID)
		} else {
			n.UserIDs = append(n.UserIDs, t.TargetID)
		}
	}
	// 名称回填（目标可能已被删除，名称与 ID 逐位对齐，缺失记 id#n）
	roleIDs, userIDs := make([]uint, 0), make([]uint, 0)
	for _, n := range list {
		roleIDs = append(roleIDs, n.RoleIDs...)
		userIDs = append(userIDs, n.UserIDs...)
	}
	roleNames := r.roleNames(ctx, roleIDs)
	userNames := r.userNames(ctx, userIDs)
	for _, n := range list {
		for _, id := range n.RoleIDs {
			if name := roleNames[id]; name != "" {
				n.RoleNames = append(n.RoleNames, name)
			} else {
				n.RoleNames = append(n.RoleNames, fmt.Sprintf("id#%d", id))
			}
		}
		for _, id := range n.UserIDs {
			if name := userNames[id]; name != "" {
				n.UserNames = append(n.UserNames, name)
			} else {
				n.UserNames = append(n.UserNames, fmt.Sprintf("id#%d", id))
			}
		}
	}
	return nil
}

// roleNames 批量查角色名
func (r *repo) roleNames(ctx context.Context, ids []uint) map[uint]string {
	out := make(map[uint]string, len(ids))
	if len(ids) == 0 {
		return out
	}
	var rows []model.RolePO
	if err := r.data.DB.WithContext(ctx).Select("id, name").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out
}

// userNames 批量查用户展示名「昵称(用户名)」
func (r *repo) userNames(ctx context.Context, ids []uint) map[uint]string {
	out := make(map[uint]string, len(ids))
	if len(ids) == 0 {
		return out
	}
	var rows []model.PlatformUserPO
	if err := r.data.DB.WithContext(ctx).Select("platform_user_id, username, nickname").Where("platform_user_id IN ?", ids).Find(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		if row.Nickname == "" {
			out[row.PlatformUserID] = row.Username
		} else {
			out[row.ID] = row.Nickname + "(" + row.Username + ")"
		}
	}
	return out
}

func (r *repo) Create(ctx context.Context, n *notice.Notice) error {
	po := model.NoticeToPO(n)
	err := r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(po).Error; err != nil {
			return err
		}
		return insertTargets(tx, po.ID, n)
	})
	if err != nil {
		return err
	}
	n.ID, n.CreatedAt, n.UpdatedAt = po.ID, po.CreatedAt, po.UpdatedAt
	return nil
}

func (r *repo) Update(ctx context.Context, n *notice.Notice) error {
	return r.data.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.NoticePO{}).Where("id = ?", n.ID).
			Updates(map[string]interface{}{
				"title": n.Title, "content": n.Content, "level": string(n.Level), "scope": string(n.Scope),
				"publish_at": n.PublishAt, "expire_at": n.ExpireAt,
			}).Error; err != nil {
			return err
		}
		// 范围/目标整体替换（保留原范围时 Find 已回填目标，重写等价）
		if err := tx.Where("notice_id = ?", n.ID).Delete(&model.NoticeTargetPO{}).Error; err != nil {
			return err
		}
		return insertTargets(tx, n.ID, n)
	})
}

func (r *repo) Delete(ctx context.Context, id uint) error {
	res := r.data.DB.WithContext(ctx).Delete(&model.NoticePO{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return notice.ErrNotFound
	}
	return nil
}

func (r *repo) Find(ctx context.Context, id uint) (*notice.Notice, error) {
	var po model.NoticePO
	if err := r.data.DB.WithContext(ctx).First(&po, id).Error; err != nil {
		return nil, mapErr(err)
	}
	n := model.NoticeFromPO(&po)
	if err := r.loadTargets(ctx, []*notice.Notice{n}); err != nil {
		return nil, err
	}
	return n, nil
}

func (r *repo) List(ctx context.Context, q notice.Query, page, pageSize int) ([]*notice.Notice, int64, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.NoticePO{})
	if q.Title != "" {
		tx = tx.Where("title LIKE ? ESCAPE '/'", "%"+security.EscapeLike(q.Title)+"%")
	}
	if q.Level != "" {
		tx = tx.Where("level = ?", q.Level)
	}
	now := time.Now()
	switch q.Status {
	case "active":
		tx = tx.Where("publish_at <= ? AND (expire_at IS NULL OR expire_at > ?)", now, now)
	case "expired":
		tx = tx.Where("expire_at IS NOT NULL AND expire_at <= ?", now)
	case "pending":
		tx = tx.Where("publish_at > ?", now)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var pos []model.NoticePO
	if pageSize > 0 { // pageSize<=0 表示全量（不分页）
		tx = tx.Offset((page - 1) * pageSize).Limit(pageSize)
	}
	if err := tx.Order("publish_at DESC").Find(&pos).Error; err != nil {
		return nil, 0, err
	}
	out := make([]*notice.Notice, 0, len(pos))
	for i := range pos {
		out = append(out, model.NoticeFromPO(&pos[i]))
	}
	if err := r.loadTargets(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ListActive 生效中且送达本人的公告 + 本人已读标记（LEFT JOIN notice_reads）
func (r *repo) ListActive(ctx context.Context, userID uint) ([]*notice.Notice, error) {
	now := time.Now()
	cond, args := deliveredCondition(userID)
	var rows []struct {
		model.NoticePO
		ReadID *uint `gorm:"column:read_id"`
	}
	err := r.data.DB.WithContext(ctx).
		Select("notices.*, notice_reads.id AS read_id").
		Joins("LEFT JOIN notice_reads ON notice_reads.notice_id = notices.id AND notice_reads.user_id = ?", userID).
		Where("notices.publish_at <= ? AND (notices.expire_at IS NULL OR notices.expire_at > ?)", now, now).
		Where(cond, args...).
		Order("notices.publish_at DESC").Limit(50).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*notice.Notice, 0, len(rows))
	for i := range rows {
		n := model.NoticeFromPO(&rows[i].NoticePO)
		n.HasRead = rows[i].ReadID != nil
		out = append(out, n)
	}
	return out, nil
}

func (r *repo) CountUnread(ctx context.Context, userID uint) (int64, error) {
	now := time.Now()
	cond, args := deliveredCondition(userID)
	var n int64
	err := r.data.DB.WithContext(ctx).Model(&model.NoticePO{}).
		Where("publish_at <= ? AND (expire_at IS NULL OR expire_at > ?)", now, now).
		Where("id NOT IN (SELECT notice_id FROM notice_reads WHERE user_id = ?)", userID).
		Where(cond, args...).
		Count(&n).Error
	return n, err
}

func (r *repo) MarkRead(ctx context.Context, userID, noticeID uint) error {
	// 送达校验：定向公告未送达本人不可已读（防越权探测/写入）
	cond, args := deliveredCondition(userID)
	var cnt int64
	if err := r.data.DB.WithContext(ctx).Model(&model.NoticePO{}).
		Where("id = ?", noticeID).Where(cond, args...).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		return notice.ErrNotDelivered
	}
	// 幂等：唯一键冲突视为成功
	return r.data.DB.WithContext(ctx).
		Where("user_id = ? AND notice_id = ?", userID, noticeID).
		FirstOrCreate(&model.NoticeReadPO{UserID: userID, NoticeID: noticeID}).Error
}

// ListRoleOptions 送达范围角色选项（全部启用中的角色）
func (r *repo) ListRoleOptions(ctx context.Context) ([]notice.RoleOption, error) {
	var rows []notice.RoleOption
	err := r.data.DB.WithContext(ctx).Model(&model.RolePO{}).
		Select("id, name").Order("id ASC").Limit(100).Find(&rows).Error
	return rows, err
}

// ListUserOptions 送达范围用户选项（已准入用户，按用户名/昵称模糊搜索；平台是唯一身份源，本地取准入投影）
func (r *repo) ListUserOptions(ctx context.Context, kw string, limit int) ([]notice.UserOption, error) {
	tx := r.data.DB.WithContext(ctx).Model(&model.PlatformUserPO{}).
		Select("platform_user_id AS id, username, nickname").Where("enabled = ?", true)
	if kw != "" {
		like := "%" + security.EscapeLike(kw) + "%"
		tx = tx.Where("username LIKE ? ESCAPE '/' OR nickname LIKE ? ESCAPE '/'", like, like)
	}
	var rows []notice.UserOption
	err := tx.Order("id ASC").Limit(limit).Find(&rows).Error
	return rows, err
}
