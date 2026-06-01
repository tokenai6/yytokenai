package dao

import (
	"context"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

// AnnouncementDao 公告DAO
type AnnouncementDao struct{}

// NewAnnouncementDao 创建公告DAO实例
func NewAnnouncementDao() *AnnouncementDao {
	return &AnnouncementDao{}
}

// Create 创建公告
func (d *AnnouncementDao) Create(ctx context.Context, tx gdb.TX, entity *entity.AnnouncementEntity) (int64, error) {
	// 排除自增ID字段，让数据库自动生成
	result, err := db.GetDB().Model("announcement").TX(tx).FieldsEx("id").Data(entity).Insert()
	if err != nil {
		return 0, gerror.Wrap(err, "创建公告失败")
	}
	lastInsertId, err := result.LastInsertId()
	if err != nil {
		return 0, gerror.Wrap(err, "获取插入ID失败")
	}
	return lastInsertId, nil
}

// GetById 根据ID获取公告
func (d *AnnouncementDao) GetById(ctx context.Context, id int64) (*entity.AnnouncementEntity, error) {
	var announcement entity.AnnouncementEntity
	err := db.GetDB().Model("announcement").Where("id = ? AND deleted_at IS NULL", id).Scan(&announcement)
	if err != nil {
		return nil, gerror.Wrap(err, "获取公告失败")
	}
	if announcement.Id == 0 {
		return nil, nil
	}
	return &announcement, nil
}

// GetList 获取公告列表
func (d *AnnouncementDao) GetList(ctx context.Context, titleKey string, status int, page, pageSize int) ([]*entity.AnnouncementEntity, int, error) {
	model := db.GetDB().Model("announcement").Where("deleted_at IS NULL")

	if titleKey != "" {
		model = model.Where("title_key LIKE ?", "%"+titleKey+"%")
	}

	// 状态筛选：-1 表示全部；0/1 时按状态过滤
	if status == 0 || status == 1 {
		model = model.Where("status = ?", status)
	}

	// 获取总数
	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "获取公告总数失败")
	}

	// 获取列表
	var list []*entity.AnnouncementEntity
	err = model.Order("priority DESC, publish_time DESC NULLS LAST, created_at DESC").Page(page, pageSize).Scan(&list)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "获取公告列表失败")
	}

	return list, total, nil
}

// GetActiveList 获取启用的公告列表（按优先级和时间排序）
func (d *AnnouncementDao) GetActiveList(ctx context.Context, language string, page, pageSize int) ([]*entity.AnnouncementEntity, int, error) {
	model := db.GetDB().Model("announcement").
		Where("deleted_at IS NULL").
		Where("status = ?", 1)

	// 获取总数
	total, err := model.Count()
	if err != nil {
		return nil, 0, gerror.Wrap(err, "获取启用公告总数失败")
	}

	// 获取列表
	var list []*entity.AnnouncementEntity
	err = model.Order("priority DESC, publish_time DESC NULLS LAST, created_at DESC").Page(page, pageSize).Scan(&list)
	if err != nil {
		return nil, 0, gerror.Wrap(err, "获取启用公告列表失败")
	}

	return list, total, nil
}

// GetActiveTopList 获取启用的公告前N条（用于首页轮询）
func (d *AnnouncementDao) GetActiveTopList(ctx context.Context, language string, limit int) ([]*entity.AnnouncementEntity, error) {
	var list []*entity.AnnouncementEntity
	err := db.GetDB().Model("announcement").
		Where("deleted_at IS NULL").
		Where("status = ?", 1).
		Order("priority DESC, publish_time DESC NULLS LAST, created_at DESC").
		Limit(limit).
		Scan(&list)
	if err != nil {
		return nil, gerror.Wrap(err, "获取启用公告前N条失败")
	}

	return list, nil
}

// UpdateById 根据ID更新公告
func (d *AnnouncementDao) UpdateById(ctx context.Context, tx gdb.TX, id int64, titleKey string, status, priority int, publishTime interface{}, updatePublish bool) error {
	data := map[string]interface{}{
		"title_key":  titleKey,
		"status":     status,
		"priority":   priority,
		"updated_at": time.Now(),
	}
	if updatePublish {
		data["publish_time"] = publishTime
	}

	_, err := db.GetDB().Model("announcement").TX(tx).
		Where("id = ?", id).
		Data(data).Update()
	if err != nil {
		return gerror.Wrap(err, "更新公告失败")
	}
	return nil
}

// DeleteById 根据ID删除公告（软删除）
func (d *AnnouncementDao) DeleteById(ctx context.Context, tx gdb.TX, id int64) error {
	_, err := db.GetDB().Model("announcement").TX(tx).
		Where("id = ?", id).
		Data(map[string]interface{}{
			"deleted_at": time.Now(),
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "删除公告失败")
	}
	return nil
}
