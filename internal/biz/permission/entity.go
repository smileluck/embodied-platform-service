// Package permission 权限限界上下文 —— 领域层
package permission

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

// Type 权限类型：dir 目录分组（无页面，仅组织菜单层级）/ menu 前端菜单页面 / button 按钮权限点（可选绑定接口参与 RBAC 校验）
type Type string

const (
	TypeDir    Type = "dir"
	TypeMenu   Type = "menu"
	TypeButton Type = "button"
	// TypeAPI 已废弃：接口绑定能力并入 button，存量 api 记录由启动迁移转为 button
	TypeAPI Type = "api"
)

// Permission 权限聚合根（type=menu 时为前端菜单，type=button 时为按钮权限点）
type Permission struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Code      string    `json:"code"`
	Type      Type      `json:"type"`
	Method    string    `json:"method"`
	Path      string    `json:"path"`
	ParentID  uint      `json:"parent_id"`
	Icon      string    `json:"icon"`
	Sort      int       `json:"sort"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Match 判断权限是否能命中请求：button（及存量未迁移的 api）绑定了 method/path 即参与校验。
// path 支持 glob 通配：尾部 * 为前缀匹配（/api/v1/users/*），中间 * 匹配任意路径段（/api/v1/users/*/roles）。
func (p *Permission) Match(method, path string) bool {
	if p.Type == TypeMenu || p.Method == "" || p.Path == "" {
		return false
	}
	if p.Path == "*" {
		return true
	}
	if p.Method != "*" && p.Method != method {
		return false
	}
	// glob -> 正则：转义后把 \* 还原为 .*，锚定首尾
	pattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(p.Path), `\*`, ".*") + "$"
	return regexp.MustCompile(pattern).MatchString(path)
}

// MenuNode 菜单树节点（Type 区分 dir 目录分组 / menu 菜单页面）
type MenuNode struct {
	ID       uint        `json:"id"`
	Name     string      `json:"name"`
	Code     string      `json:"code"`
	Type     Type        `json:"type"`
	Path     string      `json:"path"`
	Icon     string      `json:"icon"`
	Sort     int         `json:"sort"`
	Children []*MenuNode `json:"children"`
}

// BuildMenuTree 将菜单权限列表组装为树（含 dir 目录与 menu 菜单；同级按 sort 升序、id 稳定排序）
func BuildMenuTree(items []*Permission, parentID uint) []*MenuNode {
	var nodes []*MenuNode
	for _, p := range items {
		if (p.Type != TypeDir && p.Type != TypeMenu) || p.ParentID != parentID {
			continue
		}
		node := &MenuNode{ID: p.ID, Name: p.Name, Code: p.Code, Type: p.Type, Path: p.Path, Icon: p.Icon, Sort: p.Sort}
		node.Children = BuildMenuTree(items, p.ID)
		nodes = append(nodes, node)
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Sort != nodes[j].Sort {
			return nodes[i].Sort < nodes[j].Sort
		}
		return nodes[i].ID < nodes[j].ID
	})
	return nodes
}
