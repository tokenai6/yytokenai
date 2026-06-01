package config

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/repository"
	configModel "XWFrame/internal/service/config/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// IConfigService 系统配置服务接口
type IConfigService interface {
	// GetByKeyName 根据键名获取配置
	GetByKeyName(ctx context.Context, keyName string) (*entity.ConfigEntity, error)

	// GetById 根据ID获取配置详情
	GetById(ctx context.Context, id int64) (*entity.ConfigEntity, error)

	// GetList 获取配置列表（分页）
	GetList(ctx context.Context, req *GetConfigListReq) (*configModel.GetConfigListRes, error)

	// UpdateById 根据ID更新配置
	UpdateById(ctx context.Context, req *UpdateConfigByIdReq) error
}

// configService 系统配置服务实现
type configService struct {
	configRepo repository.IConfigRepository
}

// NewConfigService 创建系统配置服务实例
func NewConfigService() IConfigService {
	return &configService{
		configRepo: repository.NewConfigRepository(),
	}
}

// GetConfigListReq 获取配置列表请求
type GetConfigListReq struct {
	KeyName  string `json:"keyName"`  // 键名（模糊查询）
	Page     int    `json:"page"`     // 页码
	PageSize int    `json:"pageSize"` // 每页数量
}

// UpdateConfigByIdReq 根据ID更新配置请求
type UpdateConfigByIdReq struct {
	Id          int64  `json:"id"`
	KeyValue    string `json:"keyValue"`
	Description string `json:"description"`
}

// GetByKeyName 根据键名获取配置
func (s *configService) GetByKeyName(ctx context.Context, keyName string) (*entity.ConfigEntity, error) {
	if keyName == "" {
		return nil, gerror.New("键名不能为空")
	}
	return s.configRepo.GetByKeyName(ctx, keyName)
}

// GetById 根据ID获取配置详情
func (s *configService) GetById(ctx context.Context, id int64) (*entity.ConfigEntity, error) {
	if id <= 0 {
		return nil, gerror.New("配置ID不能为空")
	}
	return s.configRepo.GetById(ctx, id)
}

// GetList 获取配置列表（分页）
func (s *configService) GetList(ctx context.Context, req *GetConfigListReq) (*configModel.GetConfigListRes, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	list, total, err := s.configRepo.GetList(ctx, req.KeyName, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	// 将 Entity 转换为 ConfigListItem
	items := make([]*configModel.ConfigListItem, 0, len(list))
	for _, entity := range list {
		items = append(items, &configModel.ConfigListItem{
			Id:          entity.Id,
			KeyName:     entity.KeyName,
			KeyValue:    entity.KeyValue,
			Description: entity.Description,
			CreatedAt:   entity.CreatedAt.String(),
			UpdatedAt:   entity.UpdatedAt.String(),
		})
	}

	return &configModel.GetConfigListRes{
		List:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// UpdateById 根据ID更新配置
func (s *configService) UpdateById(ctx context.Context, req *UpdateConfigByIdReq) error {
	if req.Id <= 0 {
		return gerror.New("配置ID不能为空")
	}
	return db.WithTx(ctx, nil, func(ctx context.Context, tx gdb.TX) error {
		return s.configRepo.UpdateById(ctx, tx, req.Id, req.KeyValue, req.Description)
	})
}
