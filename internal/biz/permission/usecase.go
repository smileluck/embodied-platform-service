package permission

import (
	"context"
	"errors"
	"strings"

	"github.com/smilex/smilex-admin-gin/pkg/i18n"
	"github.com/smilex/smilex-admin-gin/pkg/pagination"
)

// 父子层级规则错误（dir → menu → button 三级模型校验用，供 i18n 注册表 errors.Is 匹配）
var (
	// ErrDirTopLevelOnly 目录只能挂在顶级
	ErrDirTopLevelOnly = errors.New("目录只能挂在顶级")
	// ErrMenuParentNotDir 菜单的父级必须是目录
	ErrMenuParentNotDir = errors.New("菜单的父级必须是目录")
	// ErrButtonParentNotMenu 权限点的父级必须是菜单
	ErrButtonParentNotMenu = errors.New("权限点的父级必须是菜单")
	// ErrParentIsSelf 父级不能是自身
	ErrParentIsSelf = errors.New("父级不能是自身")
	// ErrParentIsDescendant 父级不能是自身的子级
	ErrParentIsDescendant = errors.New("父级不能是自身的子级")
	// ErrWildcardLocked 超管通配权限禁止删除
	ErrWildcardLocked = errors.New("超管通配权限禁止删除")
)

// Usecase 权限领域用例
type Usecase struct {
	repo Repo
}

func NewUsecase(repo Repo) *Usecase { return &Usecase{repo: repo} }

func (uc *Usecase) Create(ctx context.Context, name, code string, t Type, method, path string, parentID uint, icon string, sort int) (*Permission, error) {
	if code != "" {
		if _, err := uc.repo.FindByCode(ctx, code); err == nil {
			return nil, ErrDuplicateCode
		}
	}
	if err := uc.validateParentType(ctx, t, parentID); err != nil {
		return nil, err
	}
	p := &Permission{Name: name, Code: code, Type: t, Method: method, Path: path, ParentID: parentID, Icon: icon, Sort: sort}
	if err := uc.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// validateParentType 按节点类型校验父级层级（dir → menu → button 三级模型）：
// dir 只能挂在顶级；menu 父级必须是 dir；button 父级必须是 menu。menu/button 允许顶级
// （顶级页面菜单、超管通配权限点等场景）。
func (uc *Usecase) validateParentType(ctx context.Context, t Type, parentID uint) error {
	if t == TypeDir {
		if parentID != 0 {
			return ErrDirTopLevelOnly
		}
		return nil
	}
	if parentID == 0 {
		return nil
	}
	parent, err := uc.repo.FindByID(ctx, parentID)
	if err != nil {
		return err
	}
	if t == TypeMenu && parent.Type != TypeDir {
		return ErrMenuParentNotDir
	}
	if t == TypeButton && parent.Type != TypeMenu {
		return ErrButtonParentNotMenu
	}
	return nil
}

// Update 更新权限（空字符串字段不更新；icon 为指针，传空字符串可清空图标；parentID 为指针，传 0 可挪到顶级）
func (uc *Usecase) Update(ctx context.Context, id uint, name, method, path string, icon *string, sort int, parentID *uint) error {
	p, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if name != "" {
		p.Name = name
	}
	if method != "" {
		p.Method = method
	}
	if path != "" {
		p.Path = path
	}
	if icon != nil {
		p.Icon = *icon
	}
	p.Sort = sort
	if parentID != nil {
		if err := uc.validateParent(ctx, id, *parentID); err != nil {
			return err
		}
		if err := uc.validateParentType(ctx, p.Type, *parentID); err != nil {
			return err
		}
		p.ParentID = *parentID
	}
	return uc.repo.Update(ctx, p)
}

// validateParent 校验新父级合法性：不能是自身，也不能位于自身的后代链上（防环）
func (uc *Usecase) validateParent(ctx context.Context, id, parentID uint) error {
	if parentID == 0 {
		return nil
	}
	if parentID == id {
		return ErrParentIsSelf
	}
	cur := parentID
	for cur != 0 {
		if cur == id {
			return ErrParentIsDescendant
		}
		p, err := uc.repo.FindByID(ctx, cur)
		if err != nil {
			return err
		}
		cur = p.ParentID
	}
	return nil
}

// Delete 删除权限：超管通配权限（id=1）禁止删除；存在子级时须先删子级
func (uc *Usecase) Delete(ctx context.Context, id uint) error {
	if id == 1 {
		return ErrWildcardLocked
	}
	n, err := uc.repo.CountByParentID(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrHasChildren
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *Usecase) Get(ctx context.Context, id uint) (*Permission, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *Usecase) List(ctx context.Context, q Query, page, pageSize int) ([]*Permission, pagination.Page, error) {
	ps, total, err := uc.repo.List(ctx, q, page, pageSize)
	if pageSize <= 0 { // 全量返回时分页元信息按单页全量填充
		page, pageSize = 1, int(total)
	}
	return ps, pagination.Page{Page: page, PageSize: pageSize, Total: total}, err
}

// UserMenuTree 当前用户可见的菜单树（含 dir 目录分组与 menu 菜单页面）
func (uc *Usecase) UserMenuTree(ctx context.Context, userID uint) ([]*MenuNode, error) {
	ps, err := uc.repo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	menus := make([]*Permission, 0, len(ps))
	for _, p := range ps {
		if p.Type == TypeMenu || p.Type == TypeDir {
			menus = append(menus, p)
		}
	}
	tree := BuildMenuTree(menus, 0)
	localizeMenuNames(ctx, tree)
	return tree, nil
}

// localizeMenuNames 按请求 locale 用语言包（menu.<code>）覆盖菜单名；
// 语言包未命中（T 返回 key 本身）时保留库中原名。SearchUserMenus 基于本树构建，
// 其 Parents 父级链随之一并本地化
func localizeMenuNames(ctx context.Context, nodes []*MenuNode) {
	for _, n := range nodes {
		key := "menu." + n.Code
		if v := i18n.T(ctx, key); v != key {
			n.Name = v
		}
		if len(n.Children) > 0 {
			localizeMenuNames(ctx, n.Children)
		}
	}
}

// MenuHit 菜单搜索命中项（顶栏命令面板用；Parents 为父级链提示，不含自身）
// Dir 标记目录（含子菜单，无对应路由，前端渲染为灰色不可选中的分组标题）；
// Depth 为节点在菜单树中的层级（根为 0），前端据此缩进展示树形结构
type MenuHit struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Icon    string `json:"icon"`
	Parents string `json:"parents"`
	Dir     bool   `json:"dir"`
	Depth   int    `json:"depth"`
}

// SearchUserMenus 当前用户可见菜单中按关键词模糊搜索（名称/路径，不区分大小写），上限 limit 条。
// 空关键词返回空。命中目录（含子菜单）时，该目录本身与其全部子孙一并纳入结果，
// 便于输入"系统"这类目录词时看到目录下的全部菜单。
func (uc *Usecase) SearchUserMenus(ctx context.Context, userID uint, kw string, limit int) ([]*MenuHit, error) {
	if kw == "" {
		return nil, nil
	}
	tree, err := uc.UserMenuTree(ctx, userID)
	if err != nil {
		return nil, err
	}
	lkwl := strings.ToLower(kw)
	var out []*MenuHit
	// forceIn：祖先已命中，整棵子树纳入；depth：当前节点层级（根为 0）
	var walk func(nodes []*MenuNode, parents string, depth int, forceIn bool)
	walk = func(nodes []*MenuNode, parents string, depth int, forceIn bool) {
		for _, n := range nodes {
			if limit > 0 && len(out) >= limit {
				return
			}
			selfHit := strings.Contains(strings.ToLower(n.Name), lkwl) || strings.Contains(strings.ToLower(n.Path), lkwl)
			if forceIn || selfHit {
				out = append(out, &MenuHit{
					Name: n.Name, Path: n.Path, Icon: n.Icon,
					// dir 类型必为目录；len(Children)>0 兜底未迁移的旧数据（有子级的 menu）
					Parents: parents, Dir: n.Type == TypeDir || len(n.Children) > 0, Depth: depth,
				})
			}
			if len(n.Children) > 0 {
				trail := n.Name
				if parents != "" {
					trail = parents + " / " + trail
				}
				walk(n.Children, trail, depth+1, forceIn || selfHit)
			}
		}
	}
	walk(tree, "", 0, false)
	return out, nil
}
