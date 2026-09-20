// 权限点 path 通配匹配规则测试：* 单段、** 跨段，锚定首尾
package permission

import "testing"

func TestPermissionMatch(t *testing.T) {
	cases := []struct {
		name   string
		perm   Permission
		method string
		path   string
		want   bool
	}{
		// * 仅匹配单个路径段：粗粒度点不得越级命中子资源（越权防线）
		{"单段命中", Permission{Type: TypeButton, Method: "PUT", Path: "/api/v1/users/*"}, "PUT", "/api/v1/users/123", true},
		{"不越级命中子资源", Permission{Type: TypeButton, Method: "PUT", Path: "/api/v1/users/*"}, "PUT", "/api/v1/users/123/password", false},
		{"不越级命中分配角色", Permission{Type: TypeButton, Method: "PUT", Path: "/api/v1/users/*"}, "PUT", "/api/v1/users/123/roles", false},
		{"删除不跨段", Permission{Type: TypeButton, Method: "DELETE", Path: "/api/v1/users/*"}, "DELETE", "/api/v1/users/123/sessions", false},
		{"子资源独立点命中", Permission{Type: TypeButton, Method: "PUT", Path: "/api/v1/users/*/password"}, "PUT", "/api/v1/users/123/password", true},
		{"中间*不匹配缺段", Permission{Type: TypeButton, Method: "PUT", Path: "/api/v1/users/*/password"}, "PUT", "/api/v1/users/password", false},
		// ** 跨段通配
		{"**命中详情", Permission{Type: TypeButton, Method: "GET", Path: "/api/v1/files/**"}, "GET", "/api/v1/files/9", true},
		{"**命中深层子资源", Permission{Type: TypeButton, Method: "GET", Path: "/api/v1/files/**"}, "GET", "/api/v1/files/9/raw", true},
		{"**不越根", Permission{Type: TypeButton, Method: "GET", Path: "/api/v1/files/**"}, "GET", "/api/v1/files", false},
		// method 语义
		{"method 不符", Permission{Type: TypeButton, Method: "GET", Path: "/api/v1/users/*"}, "PUT", "/api/v1/users/1", false},
		{"method 通配", Permission{Type: TypeButton, Method: "*", Path: "/api/v1/exports/*"}, "DELETE", "/api/v1/exports/3", true},
		// menu / 未绑定接口的按钮不参与
		{"menu 不命中", Permission{Type: TypeMenu, Method: "GET", Path: "/api/v1/users"}, "GET", "/api/v1/users", false},
		{"无 path 不命中", Permission{Type: TypeButton, Method: "GET"}, "GET", "/api/v1/users", false},
		// 点号等元字符按字面匹配（glob 转义）
		{"点号字面匹配", Permission{Type: TypeButton, Method: "PUT", Path: "/api/v1/sys-configs/*"}, "PUT", "/api/v1/sys-configs/a.b", true},
		{"点号不吞段", Permission{Type: TypeButton, Method: "PUT", Path: "/api/v1/sys-configs/*"}, "PUT", "/api/v1/sys-configs/ab/cd", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.perm.Match(tc.method, tc.path); got != tc.want {
				t.Fatalf("Match(%s,%s) with pattern %q = %v, want %v", tc.method, tc.path, tc.perm.Path, got, tc.want)
			}
		})
	}
}
