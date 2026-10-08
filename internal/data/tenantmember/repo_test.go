package tenantmember

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	biztenantmember "github.com/smilex/smilex-admin-gin/internal/biz/tenantmember"
	"github.com/smilex/smilex-admin-gin/internal/data"
	"github.com/smilex/smilex-admin-gin/internal/data/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newTestRepo 每个用例独享命名的共享内存 sqlite（同 notice 仓储测试范式）
var testDBSeq int64

func newTestRepo(t *testing.T) biztenantmember.Repo {
	t.Helper()
	name := fmt.Sprintf("tenantmember_%d", atomic.AddInt64(&testDBSeq, 1))
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.TenantUserRolePO{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return NewRepo(&data.Data{DB: db})
}

// TestRepo_SetRoleUpsert 同键覆盖不冲突（OnConflict 更新 role 而非报唯一冲突）
func TestRepo_SetRoleUpsert(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	if err := r.SetRole(ctx, 1, 10, biztenantmember.RoleMember); err != nil {
		t.Fatal(err)
	}
	if err := r.SetRole(ctx, 1, 10, biztenantmember.RoleTenantAdmin); err != nil {
		t.Fatalf("同键再写应覆盖而非唯一冲突: %v", err)
	}
	role, err := r.RoleOf(ctx, 1, 10)
	if err != nil || role != biztenantmember.RoleTenantAdmin {
		t.Errorf("覆盖后应读到 tenant_admin, got %s err=%v", role, err)
	}
	// 无绑定返回空串（biz 层归一为 member）
	role, err = r.RoleOf(ctx, 2, 10)
	if err != nil || role != "" {
		t.Errorf("无绑定应返回空串, got %q err=%v", role, err)
	}
}

// TestRepo_RolesOfAndAdminIDs 批量读取与管理员过滤（含跨租户隔离）
func TestRepo_RolesOfAndAdminIDs(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	_ = r.SetRole(ctx, 1, 10, biztenantmember.RoleTenantAdmin)
	_ = r.SetRole(ctx, 3, 10, biztenantmember.RoleMember)
	_ = r.SetRole(ctx, 1, 20, biztenantmember.RoleMember) // 同用户他租户

	m, err := r.RolesOf(ctx, []uint{1, 2, 3}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 || m[1] != biztenantmember.RoleTenantAdmin || m[3] != biztenantmember.RoleMember {
		t.Errorf("RolesOf 批量读取错误: %v", m)
	}
	admins, err := r.TenantAdminIDs(ctx, 10)
	if err != nil || len(admins) != 1 || admins[0] != 1 {
		t.Errorf("TenantAdminIDs 应为 [1], got %v err=%v", admins, err)
	}
}

// TestRepo_DeleteAndPurge 单条解除与按租户清空
func TestRepo_DeleteAndPurge(t *testing.T) {
	r := newTestRepo(t)
	ctx := context.Background()
	_ = r.SetRole(ctx, 1, 10, biztenantmember.RoleTenantAdmin)
	_ = r.SetRole(ctx, 3, 10, biztenantmember.RoleMember)
	_ = r.SetRole(ctx, 5, 20, biztenantmember.RoleTenantAdmin)

	if err := r.Delete(ctx, 3, 10); err != nil {
		t.Fatal(err)
	}
	if role, _ := r.RoleOf(ctx, 3, 10); role != "" {
		t.Error("解除后应无绑定")
	}
	if err := r.DeleteByTenant(ctx, 10); err != nil {
		t.Fatal(err)
	}
	if role, _ := r.RoleOf(ctx, 1, 10); role != "" {
		t.Error("按租户清空后应无绑定")
	}
	// 其他租户不受影响
	if role, _ := r.RoleOf(ctx, 5, 20); role != biztenantmember.RoleTenantAdmin {
		t.Error("他租户绑定不应被清空")
	}
}
