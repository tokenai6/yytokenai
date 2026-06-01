package perm

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/repository"
	roleModel "XWFrame/internal/service/role/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// IMenuFunctionService 菜单/功能服务接口
type IMenuFunctionService interface {
	// Create 创建菜单/功能
	Create(ctx context.Context, req *roleModel.CreateMenuFunctionReq) (int64, error)

	// Update 修改菜单/功能
	Update(ctx context.Context, id int64, resourceName, pageUrl, route, functionKey, icon string, sort int) error

	// Delete 批量删除（软删除）
	Delete(ctx context.Context, ids []int64) error

	// GetList 获取菜单/功能树形列表
	GetList(ctx context.Context) (*roleModel.GetMenuFunctionListRes, error)

	// GetMenuFunctionByAdminId 根据管理员ID获取菜单功能树
	GetMenuFunctionByAdminId(ctx context.Context, adminId int64) (*roleModel.GetMenuFunctionListRes, error)
}

type menuFunctionService struct {
	repo repository.IAdminMenuFunctionRepository
}

// NewMenuFunctionService 创建服务
func NewMenuFunctionService() IMenuFunctionService {
	return &menuFunctionService{repo: repository.NewAdminMenuFunctionRepository()}
}

// Create 创建菜单/功能
func (s *menuFunctionService) Create(ctx context.Context, req *roleModel.CreateMenuFunctionReq) (int64, error) {
	if req.ResourceName == "" {
		return 0, gerror.New("资源名称不能为空")
	}
	if req.IsMenuFunction == consts.MenuTypeMenu && req.PageUrl == "" {
		return 0, gerror.New("页面路由不能为空")
	}
	if req.IsMenuFunction == consts.MenuTypeFunction && req.Route == "" {
		return 0, gerror.New("API路由不能为空")
	}

	if req.IsMenuFunction == consts.MenuTypeFunction && req.FunctionKey == "" {
		return 0, gerror.New("功能键名不能为空")
	}

	// 功能必须指定父级
	if req.IsMenuFunction == consts.MenuTypeFunction && req.ParentId == 0 {
		return 0, gerror.New("功能的父级不能为空")
	}

	if req.Sort == 0 {
		req.Sort = 99
	}
	if req.Sort < 0 || req.Sort > 32767 {
		return 0, gerror.New("排序号超出范围")
	}

	// 唯一性校验（排除ID=0，限定同一父级）
	if v, _ := s.repo.GetByResourceName(ctx, req.ResourceName, 0, req.ParentId); v != nil && v.Id > 0 {
		return 0, gerror.New("资源名称已存在")
	}
	if req.Route != "" {
		if v, _ := s.repo.GetByRoute(ctx, req.Route, 0); v != nil && v.Id > 0 {
			return 0, gerror.New("API路由已存在")
		}
	}
	if req.FunctionKey != "" {
		if v, _ := s.repo.GetByFunctionKey(ctx, req.FunctionKey, 0); v != nil && v.Id > 0 {
			return 0, gerror.New("功能键名已存在")
		}
	}

	// 父级校验
	if req.ParentId > 0 {
		parent, err := s.repo.GetById(ctx, req.ParentId)
		if err != nil {
			return 0, err
		}
		if parent == nil || parent.Id == 0 {
			return 0, gerror.New("父级不存在")
		}
		if parent.IsMenuFunction != consts.MenuTypeMenu {
			return 0, gerror.New("功能不能成为父级")
		}
	}

	entityData := &entity.AdminMenuFunctionEntity{
		ParentId:       req.ParentId,
		ResourceName:   req.ResourceName,
		PageUrl:        req.PageUrl,
		Route:          req.Route,
		FunctionKey:    req.FunctionKey,
		IsMenuFunction: req.IsMenuFunction,
		Icon:           req.Icon,
		Sort:           req.Sort,
	}
	return s.repo.Create(ctx, entityData)
}

// Update 修改菜单/功能
func (s *menuFunctionService) Update(ctx context.Context, id int64, resourceName, pageUrl, route, functionKey, icon string, sort int) error {
	if id <= 0 {
		return gerror.New("ID不能为空")
	}
	// 读取当前数据
	current, err := s.repo.GetById(ctx, id)
	if err != nil {
		return err
	}
	if current == nil || current.Id == 0 {
		return gerror.New("找不到数据")
	}

	// 通用校验
	if resourceName == "" {
		return gerror.New("资源名称不能为空")
	}
	if sort == 0 {
		sort = 99
	}
	if sort < 0 || sort > 32767 {
		return gerror.New("排序号超出范围")
	}

	// 名称唯一（同父级，排除自己）
	if v, _ := s.repo.GetByResourceName(ctx, resourceName, id, current.ParentId); v != nil && v.Id > 0 {
		return gerror.New("资源名称已存在")
	}

	// 如果提供了 route，检查唯一性（排除自己）
	if route != "" {
		if v, _ := s.repo.GetByRoute(ctx, route, id); v != nil && v.Id > 0 {
			return gerror.New("API路由已存在")
		}
	}

	// 如果提供了 functionKey，检查唯一性（排除自己）
	if functionKey != "" {
		if v, _ := s.repo.GetByFunctionKey(ctx, functionKey, id); v != nil && v.Id > 0 {
			return gerror.New("功能键名已存在")
		}
	}

	// 统一使用通用更新方法，允许更新所有字段
	return s.repo.UpdateById(ctx, id, resourceName, pageUrl, route, functionKey, icon, sort)
}

// Delete 批量删除（软删除）
func (s *menuFunctionService) Delete(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return gerror.New("没有数据可删除")
	}
	// 查询存在的数据
	list, _ := s.repo.GetDataByIds(ctx, ids)
	if len(list) == 0 {
		return gerror.New("没有数据可删除")
	}
	return db.WithTx(ctx, nil, func(ctx context.Context, tx gdb.TX) error {
		return s.repo.SoftDeleteByIds(ctx, tx, ids)
	})
}

// buildTreeItem 构建树形项（递归）
func (s *menuFunctionService) buildTreeItem(ctx context.Context, entity *entity.AdminMenuFunctionEntity) (*roleModel.MenuFunctionTreeItem, error) {
	item := &roleModel.MenuFunctionTreeItem{
		Id:             entity.Id,
		ParentId:       entity.ParentId,
		ResourceName:   entity.ResourceName,
		PageUrl:        entity.PageUrl,
		Route:          entity.Route,
		FunctionKey:    entity.FunctionKey,
		Icon:           entity.Icon,
		Sort:           entity.Sort,
		IsMenuFunction: entity.IsMenuFunction,
		Items:          []*roleModel.MenuFunctionTreeItem{},
	}

	// 如果是菜单，递归查询子项
	if entity.IsMenuFunction == consts.MenuTypeMenu {
		children, err := s.repo.GetByParentIds(ctx, []int64{entity.Id})
		if err != nil {
			return nil, err
		}
		// 递归构建子项
		for _, child := range children {
			childItem, err := s.buildTreeItem(ctx, child)
			if err != nil {
				return nil, err
			}
			item.Items = append(item.Items, childItem)
		}
	}

	return item, nil
}

// GetList 获取菜单/功能树形列表
func (s *menuFunctionService) GetList(ctx context.Context) (*roleModel.GetMenuFunctionListRes, error) {
	// 查询一级菜单
	topMenus, err := s.repo.GetTopLevelMenus(ctx)
	if err != nil {
		return nil, err
	}

	// 构建树形结构
	list := make([]*roleModel.MenuFunctionTreeItem, 0, len(topMenus))
	for _, menu := range topMenus {
		item, err := s.buildTreeItem(ctx, menu)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}

	return &roleModel.GetMenuFunctionListRes{
		List: list,
	}, nil
}

// GetMenuFunctionByAdminId 根据管理员ID获取菜单功能树
func (s *menuFunctionService) GetMenuFunctionByAdminId(ctx context.Context, adminId int64) (*roleModel.GetMenuFunctionListRes, error) {
	// 查询管理员信息
	adminRepo := repository.NewAdminRepository()
	admin, _ := adminRepo.GetById(ctx, adminId)

	if admin == nil || admin.Id == 0 {
		return nil, gerror.New("管理员不存在")
	}

	// 如果是超级管理员，直接返回全部菜单
	if admin.Role == "super_admin" {
		return s.GetList(ctx)
	}

	// 根据role_id查询角色与功能映射
	mappingRepo := repository.NewAdminRoleMenuFunctionMappingRepository()
	mappings, _ := mappingRepo.GetDataByRoleId(ctx, admin.RoleId)

	// 组装function_ids
	functionIds := make([]int64, 0, len(mappings))
	for _, mapping := range mappings {
		if mapping != nil {
			functionIds = append(functionIds, mapping.FunctionId)
		}
	}

	// 如果没有功能权限，返回空列表
	if len(functionIds) == 0 {
		return &roleModel.GetMenuFunctionListRes{
			List: []*roleModel.MenuFunctionTreeItem{},
		}, nil
	}

	// 根据function_ids查询功能数据
	functions, err := s.repo.GetDataByIds(ctx, functionIds)
	if err != nil {
		return nil, err
	}

	// 收集所有相关的ID（功能ID和需要查询的父级ID）
	allIds := make(map[int64]bool)
	for _, function := range functions {
		if function != nil {
			allIds[function.Id] = true
			// 递归向上收集所有父级ID
			currentParentId := function.ParentId
			for currentParentId > 0 {
				allIds[currentParentId] = true
				parent, err := s.repo.GetById(ctx, currentParentId)
				if err != nil || parent == nil || parent.Id == 0 {
					break
				}
				currentParentId = parent.ParentId
			}
		}
	}

	// 将map转为slice
	idSlice := make([]int64, 0, len(allIds))
	for id := range allIds {
		idSlice = append(idSlice, id)
	}

	// 查询所有相关的菜单和功能数据
	allEntities, err := s.repo.GetDataByIds(ctx, idSlice)
	if err != nil {
		return nil, err
	}

	// 构建ID到实体的映射
	entityMap := make(map[int64]*entity.AdminMenuFunctionEntity)
	for _, e := range allEntities {
		if e != nil {
			entityMap[e.Id] = e
		}
	}

	// 找出所有一级菜单（parent_id=0）
	topMenus := make([]*entity.AdminMenuFunctionEntity, 0)
	for _, e := range allEntities {
		if e != nil && e.ParentId == 0 && e.IsMenuFunction == consts.MenuTypeMenu {
			topMenus = append(topMenus, e)
		}
	}

	// 构建树形结构
	list := make([]*roleModel.MenuFunctionTreeItem, 0, len(topMenus))
	for _, menu := range topMenus {
		item, err := s.buildTreeItemFromMap(ctx, menu, entityMap)
		if err != nil {
			return nil, err
		}
		list = append(list, item)
	}

	return &roleModel.GetMenuFunctionListRes{
		List: list,
	}, nil
}

// buildTreeItemFromMap 从实体映射构建树形项（递归）
func (s *menuFunctionService) buildTreeItemFromMap(ctx context.Context, menuFunc *entity.AdminMenuFunctionEntity, entityMap map[int64]*entity.AdminMenuFunctionEntity) (*roleModel.MenuFunctionTreeItem, error) {
	item := &roleModel.MenuFunctionTreeItem{
		Id:             menuFunc.Id,
		ParentId:       menuFunc.ParentId,
		ResourceName:   menuFunc.ResourceName,
		PageUrl:        menuFunc.PageUrl,
		Route:          menuFunc.Route,
		FunctionKey:    menuFunc.FunctionKey,
		Icon:           menuFunc.Icon,
		Sort:           menuFunc.Sort,
		IsMenuFunction: menuFunc.IsMenuFunction,
		Items:          []*roleModel.MenuFunctionTreeItem{},
	}

	// 如果是菜单，递归查询子项
	if menuFunc.IsMenuFunction == consts.MenuTypeMenu {
		// 从entityMap中查找所有子项
		children := make([]*entity.AdminMenuFunctionEntity, 0)
		for _, e := range entityMap {
			if e != nil && e.ParentId == menuFunc.Id {
				children = append(children, e)
			}
		}

		// 递归构建子项
		for _, child := range children {
			childItem, err := s.buildTreeItemFromMap(ctx, child, entityMap)
			if err != nil {
				return nil, err
			}
			item.Items = append(item.Items, childItem)
		}
	}

	return item, nil
}
