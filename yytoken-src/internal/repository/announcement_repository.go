package repository

import (
	"context"
	"time"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// IAnnouncementRepository 公告仓储接口
type IAnnouncementRepository interface {
	// Create 创建公告
	Create(ctx context.Context, req *CreateAnnouncementReq) (int64, error)

	// GetById 根据ID获取公告
	GetById(ctx context.Context, id int64) (*entity.AnnouncementEntity, error)

	// GetContentsByAnnouncementId 获取公告ID下的所有语言内容
	GetContentsByAnnouncementId(ctx context.Context, id int64) ([]*entity.AnnouncementContentEntity, error)

	// GetAllContentsByAnnouncementIds 批量获取多个公告ID的所有语言内容
	GetAllContentsByAnnouncementIds(ctx context.Context, announcementIds []int64) ([]*entity.AnnouncementContentEntity, error)

	// GetList 获取公告列表
	GetList(ctx context.Context, req *GetAnnouncementListReq) ([]*entity.AnnouncementEntity, int, error)

	// GetActiveList 获取启用的公告列表
	GetActiveList(ctx context.Context, language string, page, pageSize int) ([]*AnnouncementWithContent, int, error)

	// GetActiveTopList 获取启用的公告前N条
	GetActiveTopList(ctx context.Context, language string, limit int) ([]*AnnouncementWithContent, error)

	// GetActiveDetail 获取启用的公告详情
	GetActiveDetail(ctx context.Context, id int64, language string) (*AnnouncementWithContent, error)

	// UpdateById 根据ID更新公告
	UpdateById(ctx context.Context, req *UpdateAnnouncementReq) error

	// DeleteById 根据ID删除公告
	DeleteById(ctx context.Context, id int64) error
}

// announcementRepository 公告仓储实现
type announcementRepository struct {
	announcementDao        *dao.AnnouncementDao
	announcementContentDao *dao.AnnouncementContentDao
}

// NewAnnouncementRepository 创建公告仓储实例
func NewAnnouncementRepository() IAnnouncementRepository {
	return &announcementRepository{
		announcementDao:        dao.NewAnnouncementDao(),
		announcementContentDao: dao.NewAnnouncementContentDao(),
	}
}

// CreateAnnouncementReq 创建公告请求
type CreateAnnouncementReq struct {
	TitleKey string                   `json:"title_key"`
	Status   int                      `json:"status"`
	Priority int                      `json:"priority"`
	Contents []AnnouncementContentReq `json:"contents"`
}

// AnnouncementContentReq 公告内容请求
type AnnouncementContentReq struct {
	Language string `json:"language"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

// GetAnnouncementListReq 获取公告列表请求
type GetAnnouncementListReq struct {
	TitleKey string `json:"title_key"`
	Status   int    `json:"status"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// UpdateAnnouncementReq 更新公告请求
type UpdateAnnouncementReq struct {
	Id       int64                    `json:"id"`
	TitleKey string                   `json:"title_key"`
	Status   int                      `json:"status"`
	Priority int                      `json:"priority"`
	Contents []AnnouncementContentReq `json:"contents"`
}

// AnnouncementWithContent 带内容的公告
type AnnouncementWithContent struct {
	*entity.AnnouncementEntity
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Create 创建公告
func (r *announcementRepository) Create(ctx context.Context, req *CreateAnnouncementReq) (int64, error) {
	var announcementId int64
	err := db.WithTx(ctx, nil, func(ctx context.Context, tx gdb.TX) error {
		var publishTime *time.Time
		if req.Status == 1 {
			now := time.Now()
			publishTime = &now
		}

		// 创建公告主记录
		announcement := &entity.AnnouncementEntity{
			TitleKey:    req.TitleKey,
			Status:      req.Status,
			Priority:    req.Priority,
			PublishTime: publishTime,
		}

		var err error
		announcementId, err = r.announcementDao.Create(ctx, tx, announcement)
		if err != nil {
			return err
		}

		// 创建公告内容
		if len(req.Contents) > 0 {
			contents := make([]*entity.AnnouncementContentEntity, 0, len(req.Contents))
			for _, content := range req.Contents {
				contents = append(contents, &entity.AnnouncementContentEntity{
					AnnouncementId: announcementId,
					Language:       content.Language,
					Title:          content.Title,
					Content:        content.Content,
				})
			}
			err = r.announcementContentDao.BatchCreate(ctx, tx, contents)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return 0, err
	}
	return announcementId, nil
}

// GetById 根据ID获取公告
func (r *announcementRepository) GetById(ctx context.Context, id int64) (*entity.AnnouncementEntity, error) {
	return r.announcementDao.GetById(ctx, id)
}

// GetContentsByAnnouncementId 获取公告ID下的所有语言内容
func (r *announcementRepository) GetContentsByAnnouncementId(ctx context.Context, id int64) ([]*entity.AnnouncementContentEntity, error) {
	return r.announcementContentDao.GetAllByAnnouncementId(ctx, id)
}

// GetAllContentsByAnnouncementIds 批量获取多个公告ID的所有语言内容
func (r *announcementRepository) GetAllContentsByAnnouncementIds(ctx context.Context, announcementIds []int64) ([]*entity.AnnouncementContentEntity, error) {
	return r.announcementContentDao.GetAllByAnnouncementIds(ctx, announcementIds)
}

// GetList 获取公告列表
func (r *announcementRepository) GetList(ctx context.Context, req *GetAnnouncementListReq) ([]*entity.AnnouncementEntity, int, error) {
	return r.announcementDao.GetList(ctx, req.TitleKey, req.Status, req.Page, req.PageSize)
}

// GetActiveList 获取启用的公告列表
func (r *announcementRepository) GetActiveList(ctx context.Context, language string, page, pageSize int) ([]*AnnouncementWithContent, int, error) {
	// 获取启用的公告列表
	announcements, total, err := r.announcementDao.GetActiveList(ctx, language, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	if len(announcements) == 0 {
		return []*AnnouncementWithContent{}, total, nil
	}

	// 获取公告ID列表
	announcementIds := make([]int64, 0, len(announcements))
	for _, announcement := range announcements {
		announcementIds = append(announcementIds, announcement.Id)
	}

	// 获取公告内容
	contents, err := r.announcementContentDao.GetByAnnouncementIdsAndLanguageWithDefault(ctx, announcementIds, language)
	if err != nil {
		return nil, 0, err
	}

	// 创建内容映射
	contentMap := make(map[int64]*entity.AnnouncementContentEntity)
	for _, content := range contents {
		contentMap[content.AnnouncementId] = content
	}

	// 组装结果
	result := make([]*AnnouncementWithContent, 0, len(announcements))
	for _, announcement := range announcements {
		item := &AnnouncementWithContent{
			AnnouncementEntity: announcement,
		}

		if content, exists := contentMap[announcement.Id]; exists {
			item.Title = content.Title
			item.Content = content.Content
		}

		result = append(result, item)
	}

	return result, total, nil
}

// GetActiveTopList 获取启用的公告前N条
func (r *announcementRepository) GetActiveTopList(ctx context.Context, language string, limit int) ([]*AnnouncementWithContent, error) {
	// 获取启用的公告前N条
	announcements, err := r.announcementDao.GetActiveTopList(ctx, language, limit)
	if err != nil {
		return nil, err
	}

	if len(announcements) == 0 {
		return []*AnnouncementWithContent{}, nil
	}

	// 获取公告ID列表
	announcementIds := make([]int64, 0, len(announcements))
	for _, announcement := range announcements {
		announcementIds = append(announcementIds, announcement.Id)
	}

	// 获取公告内容
	contents, err := r.announcementContentDao.GetByAnnouncementIdsAndLanguageWithDefault(ctx, announcementIds, language)
	if err != nil {
		return nil, err
	}

	// 创建内容映射
	contentMap := make(map[int64]*entity.AnnouncementContentEntity)
	for _, content := range contents {
		contentMap[content.AnnouncementId] = content
	}

	// 组装结果
	result := make([]*AnnouncementWithContent, 0, len(announcements))
	for _, announcement := range announcements {
		item := &AnnouncementWithContent{
			AnnouncementEntity: announcement,
		}

		if content, exists := contentMap[announcement.Id]; exists {
			item.Title = content.Title
			item.Content = content.Content
		}

		result = append(result, item)
	}

	return result, nil
}

// UpdateById 根据ID更新公告
func (r *announcementRepository) UpdateById(ctx context.Context, req *UpdateAnnouncementReq) error {
	current, err := r.announcementDao.GetById(ctx, req.Id)
	if err != nil {
		return err
	}
	if current == nil {
		return gerror.New("公告不存在")
	}

	var publishTime interface{}
	updatePublishTime := false
	if current.Status != req.Status {
		updatePublishTime = true
		if req.Status == 1 {
			publishTime = time.Now()
		} else {
			publishTime = nil
		}
	}

	return db.WithTx(ctx, nil, func(ctx context.Context, tx gdb.TX) error {
		// 更新公告主记录
		if err := r.announcementDao.UpdateById(ctx, tx, req.Id, req.TitleKey, req.Status, req.Priority, publishTime, updatePublishTime); err != nil {
			return err
		}

		// 删除原有内容
		if err := r.announcementContentDao.DeleteByAnnouncementId(ctx, tx, req.Id); err != nil {
			return err
		}

		// 创建新内容
		if len(req.Contents) > 0 {
			contents := make([]*entity.AnnouncementContentEntity, 0, len(req.Contents))
			for _, content := range req.Contents {
				contents = append(contents, &entity.AnnouncementContentEntity{
					AnnouncementId: req.Id,
					Language:       content.Language,
					Title:          content.Title,
					Content:        content.Content,
				})
			}
			if err := r.announcementContentDao.BatchCreate(ctx, tx, contents); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetActiveDetail 获取启用的公告详情
func (r *announcementRepository) GetActiveDetail(ctx context.Context, id int64, language string) (*AnnouncementWithContent, error) {
	// 获取公告主记录
	announcement, err := r.announcementDao.GetById(ctx, id)
	if err != nil {
		return nil, err
	}

	if announcement == nil || announcement.Status != 1 {
		return nil, nil // 公告不存在或已禁用
	}

	// 获取指定语言的内容
	content, err := r.announcementContentDao.GetByAnnouncementIdAndLanguage(ctx, id, language)
	if err != nil {
		return nil, err
	}

	if content == nil {
		// 如果指定语言不存在，尝试获取默认语言
		content, err = r.announcementContentDao.GetByAnnouncementIdAndLanguage(ctx, id, "zh-CN")
		if err != nil {
			return nil, err
		}
		if content == nil {
			return nil, nil // 没有找到任何语言的内容
		}
	}

	return &AnnouncementWithContent{
		AnnouncementEntity: announcement,
		Title:              content.Title,
		Content:            content.Content,
	}, nil
}

// DeleteById 根据ID删除公告
func (r *announcementRepository) DeleteById(ctx context.Context, id int64) error {
	return db.WithTx(ctx, nil, func(ctx context.Context, tx gdb.TX) error {
		// 删除公告内容
		err := r.announcementContentDao.DeleteByAnnouncementId(ctx, tx, id)
		if err != nil {
			return err
		}

		// 删除公告主记录
		return r.announcementDao.DeleteById(ctx, tx, id)
	})
}
