package notice

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smilex/smilex-admin-gin/internal/biz/notice"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newTestRepo 每个用例独享命名的共享内存 sqlite，避免用例间串数据
var testDBSeq int64

func newTestRepo(t *testing.T) notice.Repo {
	t.Helper()
	name := fmt.Sprintf("notice_%d", atomic.AddInt64(&testDBSeq, 1))
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.PlatformUserPO{}, &model.RolePO{}, &model.PlatformUserRolePO{},
		&model.NoticePO{}, &model.NoticeReadPO{}, &model.NoticeTargetPO{},
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return NewRepo(&data.Data{DB: db})
}

func mustCreate(t *testing.T, r notice.Repo, n *notice.Notice) *notice.Notice {
	t.Helper()
	if err := r.Create(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestTargetedDelivery 定向送达：角色/用户范围只对目标可见，未送达不可已读
func TestTargetedDelivery(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	db := r.(*repo).data.DB

	// 用户（平台身份投影；id 均为平台用户 ID）：alice(1,运维角色) / bob(2) / carol(3)
	if err := db.Create(&[]model.PlatformUserPO{
		{PlatformUserID: 1, Username: "alice", Nickname: "爱丽丝", Enabled: true},
		{PlatformUserID: 2, Username: "bob", Nickname: "鲍勃", Enabled: true},
		{PlatformUserID: 3, Username: "carol", Nickname: "卡罗尔", Enabled: true},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&[]model.RolePO{{ID: 10, Name: "运维"}}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.PlatformUserRolePO{PlatformUserID: 1, RoleID: 10}).Error; err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	future := now.Add(time.Hour)
	nAll := mustCreate(t, r, &notice.Notice{Title: "全体公告", Level: notice.LevelInfo, Scope: notice.ScopeAll, PublishAt: now})
	nRole := mustCreate(t, r, &notice.Notice{Title: "运维专属", Level: notice.LevelWarning, Scope: notice.ScopeRoles, RoleIDs: []uint{10}, PublishAt: now})
	nUser := mustCreate(t, r, &notice.Notice{Title: "私信bob", Level: notice.LevelImportant, Scope: notice.ScopeUsers, UserIDs: []uint{2}, PublishAt: now})
	mustCreate(t, r, &notice.Notice{Title: "待发布", Level: notice.LevelInfo, Scope: notice.ScopeAll, PublishAt: future})

	ids := func(list []*notice.Notice) map[uint]bool {
		out := map[uint]bool{}
		for _, n := range list {
			out[n.ID] = true
		}
		return out
	}

	// alice：全体 + 角色；bob：全体 + 私信；carol：仅全体
	seen := ids(mustList(t, r, 1))
	if !seen[nAll.ID] || !seen[nRole.ID] || seen[nUser.ID] {
		t.Fatalf("alice should see all+role notices, got %v", seen)
	}
	seen = ids(mustList(t, r, 2))
	if !seen[nAll.ID] || !seen[nUser.ID] || seen[nRole.ID] {
		t.Fatalf("bob should see all+user notices, got %v", seen)
	}
	seen = ids(mustList(t, r, 3))
	if !seen[nAll.ID] || seen[nRole.ID] || seen[nUser.ID] {
		t.Fatalf("carol should see only broadcast, got %v", seen)
	}

	// 未读数与送达范围一致
	if n, _ := r.CountUnread(ctx, 2); n != 2 {
		t.Fatalf("bob unread want 2, got %d", n)
	}
	if n, _ := r.CountUnread(ctx, 3); n != 1 {
		t.Fatalf("carol unread want 1, got %d", n)
	}

	// 越权已读被拒；目标用户已读幂等并扣减未读
	if err := r.MarkRead(ctx, 3, nUser.ID); !errors.Is(err, notice.ErrNotDelivered) {
		t.Fatalf("carol markread others' notice want ErrNotDelivered, got %v", err)
	}
	if err := r.MarkRead(ctx, 2, nUser.ID); err != nil {
		t.Fatalf("bob markread own notice: %v", err)
	}
	if err := r.MarkRead(ctx, 2, nUser.ID); err != nil {
		t.Fatalf("markread should be idempotent: %v", err)
	}
	if n, _ := r.CountUnread(ctx, 2); n != 1 {
		t.Fatalf("bob unread after read want 1, got %d", n)
	}

	// 管理端回显目标
	got, err := r.Find(ctx, nRole.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.RoleIDs) != 1 || got.RoleIDs[0] != 10 || len(got.UserIDs) != 0 {
		t.Fatalf("role targets echo wrong: %+v", got)
	}

	// 范围整体替换：定向改全体，原目标作废、所有人可见
	got.Scope = notice.ScopeAll
	got.RoleIDs = nil
	if err := r.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	seen = ids(mustList(t, r, 3))
	if !seen[nRole.ID] {
		t.Fatalf("carol should see re-scoped notice, got %v", seen)
	}
	var targets int64
	_ = db.Model(&model.NoticeTargetPO{}).Where("notice_id = ?", nRole.ID).Count(&targets).Error
	if targets != 0 {
		t.Fatalf("stale targets not cleaned: %d", targets)
	}
}

func mustList(t *testing.T, r notice.Repo, userID uint) []*notice.Notice {
	t.Helper()
	list, err := r.ListActive(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// TestUserOptions 用户选项仅含已准入用户并支持模糊搜索
func TestUserOptions(t *testing.T) {
	r := newTestRepo(t)
	db := r.(*repo).data.DB
	if err := db.Create(&[]model.PlatformUserPO{
		{PlatformUserID: 1, Username: "alice", Nickname: "爱丽丝", Enabled: true},
		{PlatformUserID: 2, Username: "bob_disabled", Nickname: "停用者", Enabled: false},
	}).Error; err != nil {
		t.Fatal(err)
	}
	opts, err := r.ListUserOptions(context.Background(), "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) != 1 || opts[0].Username != "alice" {
		t.Fatalf("options should only include admitted users, got %+v", opts)
	}
	opts, err = r.ListUserOptions(context.Background(), "爱丽", 20)
	if err != nil || len(opts) != 1 {
		t.Fatalf("nickname search failed: %v %+v", err, opts)
	}
}
