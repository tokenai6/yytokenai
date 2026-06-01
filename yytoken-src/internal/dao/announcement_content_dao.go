package dao

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// AnnouncementContentDao 公告内容DAO
type AnnouncementContentDao struct{}

// NewAnnouncementContentDao 创建公告内容DAO实例
func NewAnnouncementContentDao() *AnnouncementContentDao {
	return &AnnouncementContentDao{}
}

// Create 创建公告内容
func (d *AnnouncementContentDao) Create(ctx context.Context, tx gdb.TX, entity *entity.AnnouncementContentEntity) (int64, error) {
	// 排除自增ID字段，让数据库自动生成
	result, err := db.GetDB().Model("announcement_content").TX(tx).FieldsEx("id").Data(entity).Insert()
	if err != nil {
		return 0, gerror.Wrap(err, "创建公告内容失败")
	}
	lastInsertId, err := result.LastInsertId()
	if err != nil {
		return 0, gerror.Wrap(err, "获取插入ID失败")
	}
	return lastInsertId, nil
}

// GetByAnnouncementIdAndLanguage 根据公告ID和语言获取内容
func (d *AnnouncementContentDao) GetByAnnouncementIdAndLanguage(ctx context.Context, announcementId int64, language string) (*entity.AnnouncementContentEntity, error) {
	var content entity.AnnouncementContentEntity
	err := db.GetDB().Model("announcement_content").
		Where("announcement_id = ? AND language = ?", announcementId, language).
		Scan(&content)
	if err != nil {
		return nil, gerror.Wrap(err, "获取公告内容失败")
	}
	if content.Id == 0 {
		return nil, nil
	}
	return &content, nil
}

// GetAllByAnnouncementId 获取指定公告ID的所有语言内容
func (d *AnnouncementContentDao) GetAllByAnnouncementId(ctx context.Context, announcementId int64) ([]*entity.AnnouncementContentEntity, error) {
	var list []*entity.AnnouncementContentEntity
	err := db.GetDB().Model("announcement_content").
		Where("announcement_id = ?", announcementId).
		Order("language ASC").
		Scan(&list)
	if err != nil {
		return nil, gerror.Wrap(err, "获取公告所有语言内容失败")
	}
	return list, nil
}

// GetAllByAnnouncementIds 批量获取多个公告ID的所有语言内容
func (d *AnnouncementContentDao) GetAllByAnnouncementIds(ctx context.Context, announcementIds []int64) ([]*entity.AnnouncementContentEntity, error) {
	if len(announcementIds) == 0 {
		return []*entity.AnnouncementContentEntity{}, nil
	}
	var list []*entity.AnnouncementContentEntity
	err := db.GetDB().Model("announcement_content").
		Where("announcement_id IN (?)", announcementIds).
		Order("announcement_id ASC, language ASC").
		Scan(&list)
	if err != nil {
		return nil, gerror.Wrap(err, "批量获取公告所有语言内容失败")
	}
	return list, nil
}

// GetByAnnouncementIdsAndLanguage 根据公告ID列表和语言获取内容
func (d *AnnouncementContentDao) GetByAnnouncementIdsAndLanguage(ctx context.Context, announcementIds []int64, language string) ([]*entity.AnnouncementContentEntity, error) {
	var list []*entity.AnnouncementContentEntity
	err := db.GetDB().Model("announcement_content").
		Where("announcement_id IN (?) AND language = ?", announcementIds, language).
		Scan(&list)
	if err != nil {
		return nil, gerror.Wrap(err, "获取公告内容列表失败")
	}
	return list, nil
}

// GetByAnnouncementIdsAndLanguageWithDefault 根据公告ID列表和语言获取内容，如果语言不存在则返回默认语言
func (d *AnnouncementContentDao) GetByAnnouncementIdsAndLanguageWithDefault(ctx context.Context, announcementIds []int64, language string) ([]*entity.AnnouncementContentEntity, error) {
	// 先尝试获取指定语言的内容
	list, err := d.GetByAnnouncementIdsAndLanguage(ctx, announcementIds, language)
	if err != nil {
		return nil, err
	}

	// 如果找到了内容，直接返回
	if len(list) > 0 {
		return list, nil
	}

	// 如果没找到内容，尝试获取默认语言（中文）的内容
	return d.GetByAnnouncementIdsAndLanguage(ctx, announcementIds, "zh-CN")
}

// UpdateByAnnouncementIdAndLanguage 根据公告ID和语言更新内容
func (d *AnnouncementContentDao) UpdateByAnnouncementIdAndLanguage(ctx context.Context, tx gdb.TX, announcementId int64, language, title, content string) error {
	_, err := db.GetDB().Model("announcement_content").TX(tx).
		Where("announcement_id = ? AND language = ?", announcementId, language).
		Data(map[string]interface{}{
			"title":      title,
			"content":    content,
			"updated_at": "NOW()",
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "更新公告内容失败")
	}
	return nil
}

// DeleteByAnnouncementId 根据公告ID删除所有内容
func (d *AnnouncementContentDao) DeleteByAnnouncementId(ctx context.Context, tx gdb.TX, announcementId int64) error {
	_, err := db.GetDB().Model("announcement_content").TX(tx).
		Where("announcement_id = ?", announcementId).
		Delete()
	if err != nil {
		return gerror.Wrap(err, "删除公告内容失败")
	}
	return nil
}

// BatchCreate 批量创建公告内容
func (d *AnnouncementContentDao) BatchCreate(ctx context.Context, tx gdb.TX, contents []*entity.AnnouncementContentEntity) error {
	if len(contents) == 0 {
		return nil
	}

	// 排除自增ID字段，让数据库自动生成
	_, err := db.GetDB().Model("announcement_content").TX(tx).FieldsEx("id").Data(contents).Insert()
	if err != nil {
		return gerror.Wrap(err, "批量创建公告内容失败")
	}
	return nil
}
