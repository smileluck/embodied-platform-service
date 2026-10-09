package tenantrole

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	biztenantuser "github.com/smilex/smilex-admin-gin/internal/biz/tenantuser"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newTestRepo 每个用例独享命名的共享内存 sqlite（沿用 notice repo 测试模式）
var testDBSeq int64

func newTestRepo(t *testing.T) *Repo {
	t.Helper()
	name := fmt.Sprintf("tenantrole_%d", atomic.AddInt64(&testDBSeq, 1))
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.TenantRolePO{}, &model.TenantRolePermPO{}, &model.TenantUserRoleBindPO{},
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

// 角色删除：仍有用户绑定时拒绝（ErrRoleInUse）；解除后可删，
// 墓碑改写 code 释放 (tenant_id, code) 槽位，删后同码可重建
func TestRoleDelete_GuardAndTombstone(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()

	role := &biztenantuser.TenantRole{TenantID: 71, Name: "运维", Code: "ops", PermCodes: []string{"device:list"}}
	if err := r.Create(ctx, role); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceUserRoles(ctx, 9001, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, role.ID); !errors.Is(err, biztenantuser.ErrRoleInUse) {
		t.Fatalf("仍有绑定时应拒绝删除, got %v", err)
	}
	if err := r.ReplaceUserRoles(ctx, 9001, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, role.ID); err != nil {
		t.Fatal(err)
	}
	var softDeleted model.TenantRolePO
	if err := r.data.DB.Unscoped().First(&softDeleted, role.ID).Error; err != nil {
		t.Fatal(err)
	}
	if softDeleted.Code == "ops" {
		t.Fatalf("软删行应墓碑化 code: %+v", softDeleted)
	}
	role2 := &biztenantuser.TenantRole{TenantID: 71, Name: "运维2", Code: "ops"}
	if err := r.Create(ctx, role2); err != nil {
		t.Fatalf("删后重建同 (tenant_id, code) 应成功: %v", err)
	}
}

// 同租户同 code 唯一冲突映射 ErrDuplicateRoleCode；不同租户同 code 允许
func TestRoleCreate_DuplicateCode(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	if err := r.Create(ctx, &biztenantuser.TenantRole{TenantID: 71, Code: "ops"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Create(ctx, &biztenantuser.TenantRole{TenantID: 71, Code: "ops"}); !errors.Is(err, biztenantuser.ErrDuplicateRoleCode) {
		t.Fatalf("同租户同码应 409, got %v", err)
	}
	if err := r.Create(ctx, &biztenantuser.TenantRole{TenantID: 72, Code: "ops"}); err != nil {
		t.Fatalf("不同租户同码应允许, got %v", err)
	}
}

// 权限点/绑定替换语义：全量替换（先删后插），残留不阻碍重赋
func TestReplaceSemantics(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	role := &biztenantuser.TenantRole{TenantID: 71, Code: "ops", PermCodes: []string{"device:list", "alarm:list"}}
	if err := r.Create(ctx, role); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.PermCodes) != 2 {
		t.Fatalf("创建应落数权限点, got %v", got.PermCodes)
	}
	// 更新替换为单项
	got.PermCodes = []string{"device:list"}
	if err := r.Update(ctx, got); err != nil {
		t.Fatal(err)
	}
	got2, _ := r.Get(ctx, role.ID)
	if len(got2.PermCodes) != 1 || got2.PermCodes[0] != "device:list" {
		t.Fatalf("权限点应全量替换, got %v", got2.PermCodes)
	}
	// 绑定替换：[role] → [] → 重新绑定不因复合主键残留失败
	if err := r.ReplaceUserRoles(ctx, 7, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceUserRoles(ctx, 7, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceUserRoles(ctx, 7, []uint{role.ID}); err != nil {
		t.Fatal(err)
	}
	m, err := r.UserRoleIDs(ctx, []uint{7})
	if err != nil || len(m[7]) != 1 || m[7][0] != role.ID {
		t.Fatalf("绑定应替换后保留最新, got %v err %v", m, err)
	}
}

// ResolvePerms：binds → 本租户角色 → 权限码去重；跨租户角色/他租户绑定不生效
func TestResolvePerms(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	r1 := &biztenantuser.TenantRole{TenantID: 71, Code: "ops", PermCodes: []string{"device:list"}}
	r2 := &biztenantuser.TenantRole{TenantID: 72, Code: "ops2", PermCodes: []string{"alarm:list"}}
	for _, role := range []*biztenantuser.TenantRole{r1, r2} {
		if err := r.Create(ctx, role); err != nil {
			t.Fatal(err)
		}
	}
	// user 7 绑定本租户 r1 与他租户 r2：仅 r1 权限生效
	if err := r.ReplaceUserRoles(ctx, 7, []uint{r1.ID, r2.ID}); err != nil {
		t.Fatal(err)
	}
	perms, err := r.ResolvePerms(ctx, 7, 71)
	if err != nil {
		t.Fatal(err)
	}
	if len(perms) != 1 || perms[0] != "device:list" {
		t.Fatalf("跨租户角色不应计入, got %v", perms)
	}
	// 同码去重：多角色同权限点只出一次
	r3 := &biztenantuser.TenantRole{TenantID: 71, Code: "ops3", PermCodes: []string{"device:list"}}
	if err := r.Create(ctx, r3); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceUserRoles(ctx, 7, []uint{r1.ID, r3.ID}); err != nil {
		t.Fatal(err)
	}
	perms, _ = r.ResolvePerms(ctx, 7, 71)
	if len(perms) != 1 {
		t.Fatalf("权限码应去重, got %v", perms)
	}
	// 无绑定用户返回空集（default deny）
	perms, _ = r.ResolvePerms(ctx, 8, 71)
	if len(perms) != 0 {
		t.Fatalf("无绑定应空集, got %v", perms)
	}
}

// NormalizePerms（本地注册表目录口径）：目录外拒绝、去重排序、上限
func TestNormalizePerms(t *testing.T) {
	got, err := biztenantuser.NormalizePerms([]string{"device:update", "device:list", "device:list", " ", ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "device:list" || got[1] != "device:update" {
		t.Fatalf("应去重并字典序排序, got %v", got)
	}
	if _, err := biztenantuser.NormalizePerms([]string{"device:hack"}); !errors.Is(err, biztenantuser.ErrInvalidPerm) {
		t.Fatalf("目录外权限点应拒绝, got %v", err)
	}
	if !biztenantuser.PermAllowed([]string{"device:list"}, "device:list") {
		t.Fatal("精确命中应放行")
	}
	if biztenantuser.PermAllowed([]string{"device:*"}, "device:list") {
		t.Fatal("租户权限点不支持通配形态")
	}
}
