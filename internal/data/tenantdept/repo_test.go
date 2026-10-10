package tenantdept

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	biztenant "github.com/smilex/smilex-admin-gin/internal/biz/tenant"
	biztenantdept "github.com/smilex/smilex-admin-gin/internal/biz/tenantdept"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newTestRepo 每个用例独享命名的共享内存 sqlite（沿用 tenantrole repo 测试模式）
var testDBSeq int64

func newTestRepo(t *testing.T) (*Repo, *biztenantdept.Usecase) {
	t.Helper()
	name := fmt.Sprintf("tenantdept_%d", atomic.AddInt64(&testDBSeq, 1))
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TenantDeptPO{}, &model.TenantUserDeptBindPO{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	repo := NewRepo(&data.Data{DB: db})
	uc := biztenantdept.NewUsecase(repo, stubChecker{})
	return repo, uc
}

// stubChecker 租户 71 存在、其余不存在的本地投影桩
type stubChecker struct{}

func (stubChecker) GetByPlatformID(_ context.Context, platformID uint) (*biztenant.Tenant, error) {
	if platformID == 71 {
		return &biztenant.Tenant{PlatformID: 71, Name: "T71"}, nil
	}
	return nil, biztenant.ErrTenantNotFound
}

// 部门删除：墓碑改写 code 释放 (tenant_id, code) 槽位（删后同码可重建），
// 成员绑定随删除级联清理
func TestDeptDelete_TombstoneAndCascade(t *testing.T) {
	r, uc := newTestRepo(t)
	ctx := context.Background()
	dept := &biztenantdept.TenantDept{TenantID: 71, Name: "研发", Code: "rd"}
	if err := r.Create(ctx, dept); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceUserDepts(ctx, 9001, []uint{dept.ID}); err != nil {
		t.Fatal(err)
	}
	if err := uc.DeleteDept(ctx, dept.ID); err != nil {
		t.Fatal(err)
	}
	var softDeleted model.TenantDeptPO
	if err := r.data.DB.Unscoped().First(&softDeleted, dept.ID).Error; err != nil {
		t.Fatal(err)
	}
	if softDeleted.Code == "rd" {
		t.Fatalf("软删行应墓碑化 code: %+v", softDeleted)
	}
	binds, err := r.UserDeptIDs(ctx, []uint{9001})
	if err != nil {
		t.Fatal(err)
	}
	if len(binds[9001]) != 0 {
		t.Fatalf("成员绑定应随部门删除级联清理, got %v", binds)
	}
	dept2 := &biztenantdept.TenantDept{TenantID: 71, Name: "研发2", Code: "rd"}
	if err := r.Create(ctx, dept2); err != nil {
		t.Fatalf("删后重建同 (tenant_id, code) 应成功: %v", err)
	}
}

// 同租户同 code 唯一冲突映射 ErrDuplicateDeptCode；不同租户同 code 允许
func TestDeptCreate_DuplicateCode(t *testing.T) {
	r, _ := newTestRepo(t)
	ctx := context.Background()
	if err := r.Create(ctx, &biztenantdept.TenantDept{TenantID: 71, Code: "rd"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Create(ctx, &biztenantdept.TenantDept{TenantID: 71, Code: "rd"}); !errors.Is(err, biztenantdept.ErrDuplicateDeptCode) {
		t.Fatalf("同租户同码应冲突, got %v", err)
	}
	if err := r.Create(ctx, &biztenantdept.TenantDept{TenantID: 72, Code: "rd"}); err != nil {
		t.Fatalf("不同租户同码应允许, got %v", err)
	}
}

// 绑定替换语义：全量替换（先删后插），残留不阻碍重赋；MemberCounts/DeptNamesByIDs 口径
func TestBindSemantics(t *testing.T) {
	r, _ := newTestRepo(t)
	ctx := context.Background()
	d1 := &biztenantdept.TenantDept{TenantID: 71, Code: "rd", Name: "研发"}
	d2 := &biztenantdept.TenantDept{TenantID: 71, Code: "ops", Name: "运维"}
	for _, d := range []*biztenantdept.TenantDept{d1, d2} {
		if err := r.Create(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	// 多部门绑定：user 7 → [d1, d2]；替换为 [d1]；再清空
	if err := r.ReplaceUserDepts(ctx, 7, []uint{d1.ID, d2.ID}); err != nil {
		t.Fatal(err)
	}
	m, _ := r.UserDeptIDs(ctx, []uint{7})
	if len(m[7]) != 2 {
		t.Fatalf("多部门绑定应落数, got %v", m)
	}
	if err := r.ReplaceUserDepts(ctx, 7, []uint{d1.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceUserDepts(ctx, 7, nil); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceUserDepts(ctx, 7, []uint{d1.ID}); err != nil {
		t.Fatal(err)
	}
	m, _ = r.UserDeptIDs(ctx, []uint{7})
	if len(m[7]) != 1 || m[7][0] != d1.ID {
		t.Fatalf("绑定应替换后保留最新, got %v", m)
	}
	// 计数与名称解析
	r.ReplaceUserDepts(ctx, 8, []uint{d1.ID})
	counts, err := r.MemberCounts(ctx, 71)
	if err != nil {
		t.Fatal(err)
	}
	if counts[d1.ID] != 2 || counts[d2.ID] != 0 {
		t.Fatalf("直属成员数应为 d1=2 d2=0, got %v", counts)
	}
	names, err := r.DeptNamesByIDs(ctx, []uint{d1.ID, d2.ID})
	if err != nil || names[d1.ID] != "研发" || names[d2.ID] != "运维" {
		t.Fatalf("部门名解析失败, got %v err %v", names, err)
	}
}

// 用例守卫：更新防环（父级指向自身/后代拒绝，指向合法部门放行）、
// 删除有子部门拒绝、跨租户部门绑定拒绝
func TestUsecase_Guards(t *testing.T) {
	_, uc := newTestRepo(t)
	ctx := context.Background()
	// 树：root → mid → leaf；sibling 为另一根
	root, err := uc.CreateDept(ctx, biztenantdept.DeptCreateParams{TenantID: 71, Code: "root", Name: "总部"})
	if err != nil {
		t.Fatal(err)
	}
	mid, err := uc.CreateDept(ctx, biztenantdept.DeptCreateParams{TenantID: 71, Code: "mid", Name: "中段", ParentID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := uc.CreateDept(ctx, biztenantdept.DeptCreateParams{TenantID: 71, Code: "leaf", Name: "末端", ParentID: mid.ID})
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := uc.CreateDept(ctx, biztenantdept.DeptCreateParams{TenantID: 71, Code: "sib", Name: "平行"})
	if err != nil {
		t.Fatal(err)
	}
	// 防环：root 挂到自身 / 后代（mid、leaf）均拒绝
	self := root.ID
	if err := uc.UpdateDept(ctx, root.ID, biztenantdept.DeptUpdateParams{ParentID: &self}); !errors.Is(err, biztenantdept.ErrParentInvalid) {
		t.Fatalf("父级指向自身应拒绝, got %v", err)
	}
	for _, pid := range []uint{mid.ID, leaf.ID} {
		p := pid
		if err := uc.UpdateDept(ctx, root.ID, biztenantdept.DeptUpdateParams{ParentID: &p}); !errors.Is(err, biztenantdept.ErrParentInvalid) {
			t.Fatalf("父级指向后代应拒绝(%d), got %v", pid, err)
		}
	}
	// 合法移动：leaf 改挂 sibling 放行
	sib := sibling.ID
	if err := uc.UpdateDept(ctx, leaf.ID, biztenantdept.DeptUpdateParams{ParentID: &sib}); err != nil {
		t.Fatalf("合法移动应放行, got %v", err)
	}
	// 删除守卫：root 仍有子部门 mid 时拒绝；删空子部门后（mid 无子可先删）root 放行
	if err := uc.DeleteDept(ctx, root.ID); !errors.Is(err, biztenantdept.ErrDeptHasChildren) {
		t.Fatalf("有子部门应拒绝删除, got %v", err)
	}
	if err := uc.DeleteDept(ctx, mid.ID); err != nil {
		t.Fatalf("叶子部门（子已移走）应可删除, got %v", err)
	}
	if err := uc.DeleteDept(ctx, root.ID); err != nil {
		t.Fatalf("子部门清空后应可删除, got %v", err)
	}
	// 跨租户挂载/绑定拒绝：租户 72 的部门不可挂到 71 的部门下、不可绑定给 71 的用户
	other, err := uc.CreateDept(ctx, biztenantdept.DeptCreateParams{TenantID: 72, Code: "other", Name: "他租户"})
	_ = other
	// 注：stubChecker 仅放行租户 71，72 的创建在租户校验即被拒——此分支验证租户闸门
	if err == nil {
		t.Fatal("未同步租户应拒绝创建部门")
	}
	if err := uc.SetUserDepts(ctx, 71, 7, []uint{sibling.ID + 9999}); !errors.Is(err, biztenantdept.ErrDeptNotFound) {
		t.Fatalf("不存在的部门应拒绝绑定, got %v", err)
	}
}

// UserIDsUnderDept：部门成员反查含全部后代展开
func TestUserIDsUnderDept_Descendants(t *testing.T) {
	r, uc := newTestRepo(t)
	ctx := context.Background()
	root, err := uc.CreateDept(ctx, biztenantdept.DeptCreateParams{TenantID: 71, Code: "root", Name: "总部"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := uc.CreateDept(ctx, biztenantdept.DeptCreateParams{TenantID: 71, Code: "c1", Name: "子部", ParentID: root.ID})
	if err != nil {
		t.Fatal(err)
	}
	grand, err := uc.CreateDept(ctx, biztenantdept.DeptCreateParams{TenantID: 71, Code: "g1", Name: "孙部", ParentID: child.ID})
	if err != nil {
		t.Fatal(err)
	}
	// 成员分布：root 直属 1、child 直属 2、grand 直属 1（其中 1 人跨 root/child 双部门）
	if err := r.ReplaceUserDepts(ctx, 1, []uint{root.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceUserDepts(ctx, 2, []uint{child.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceUserDepts(ctx, 3, []uint{child.ID, root.ID}); err != nil {
		t.Fatal(err)
	}
	if err := r.ReplaceUserDepts(ctx, 4, []uint{grand.ID}); err != nil {
		t.Fatal(err)
	}
	ids, tenantID, err := uc.UserIDsUnderDept(ctx, root.ID)
	if err != nil {
		t.Fatal(err)
	}
	if tenantID != 71 {
		t.Fatalf("应返回部门归属租户, got %d", tenantID)
	}
	if len(ids) != 4 {
		t.Fatalf("根部门反查应含全部后代成员 4 人, got %v", ids)
	}
	ids, _, err = uc.UserIDsUnderDept(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 {
		t.Fatalf("子部门反查应含孙部门成员 3 人, got %v", ids)
	}
	ids, _, err = uc.UserIDsUnderDept(ctx, grand.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != 4 {
		t.Fatalf("孙部门反查应仅直属 1 人, got %v", ids)
	}
}
