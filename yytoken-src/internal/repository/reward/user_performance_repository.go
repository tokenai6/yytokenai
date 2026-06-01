package reward

import (
	"context"
	"math"
	"time"

	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/model"
)

// IUserPerformanceRepository 用户业绩仓储接口
//
// Deprecated: 该接口已废弃，团队业绩统一使用 cobo_node_purchase 表实时计算
// 请使用 internal/repository/cobo IPerformanceRepository
// 保留该接口仅用于兼容历史代码
type IUserPerformanceRepository interface {
	// GetUserRecordsByOffset 按offset获取用户业绩记录
	GetUserRecordsByOffset(ctx context.Context, userID int64, offset *int) ([]*rewardEntity.UserPerformanceEntity, error)

	// GetByUserAndDate 根据用户ID和日期获取业绩
	GetByUserAndDate(ctx context.Context, userID int64, date time.Time) (*rewardEntity.UserPerformanceEntity, error)

	// GetLatestPerformancePage 分页获取业绩记录（按id倒序，可按user_ids过滤）
	GetLatestPerformancePage(ctx context.Context, req *GetLatestPerformancePageReq) (*GetLatestPerformancePageRes, error)

	// GetLatestRecordDate 获取业绩最新记录日期
	GetLatestRecordDate(ctx context.Context) (time.Time, error)

	// GetByDateRange 按日期范围和user_ids分页查询业绩记录（按id倒序）
	GetByDateRange(ctx context.Context, req *GetPerformanceHistoryPageReq) (*GetPerformanceHistoryPageRes, error)
}

// GetLatestPerformancePageReq 分页查询请求
type GetLatestPerformancePageReq struct {
	model.PageReq
	UserIDs    []int64
	RecordDate string
}

// GetLatestPerformancePageRes 分页查询响应
type GetLatestPerformancePageRes struct {
	model.PageRes
	List []*rewardEntity.UserPerformanceEntity
}

// GetPerformanceHistoryPageReq 历史业绩分页查询请求
type GetPerformanceHistoryPageReq struct {
	model.PageReq
	Date    string
	UserIDs []int64
}

// GetPerformanceHistoryPageRes 历史业绩分页查询响应
type GetPerformanceHistoryPageRes struct {
	model.PageRes
	List []*rewardEntity.UserPerformanceEntity
}

// userPerformanceRepository 用户业绩仓储实现
type userPerformanceRepository struct {
	performanceDao rewardDao.IUserPerformanceDao
}

// NewUserPerformanceRepository 创建用户业绩仓储实例
func NewUserPerformanceRepository() IUserPerformanceRepository {
	return &userPerformanceRepository{
		performanceDao: rewardDao.NewUserPerformanceDao(),
	}
}

// GetUserRecordsByOffset 按offset获取用户业绩记录
func (r *userPerformanceRepository) GetUserRecordsByOffset(ctx context.Context, userID int64, offset *int) ([]*rewardEntity.UserPerformanceEntity, error) {
	return r.performanceDao.GetUserRecordsByOffset(ctx, userID, offset)
}

// GetByUserAndDate 根据用户ID和日期获取业绩
func (r *userPerformanceRepository) GetByUserAndDate(ctx context.Context, userID int64, date time.Time) (*rewardEntity.UserPerformanceEntity, error) {
	return r.performanceDao.GetByUserAndDate(ctx, userID, date)
}

// GetLatestPerformancePage 分页获取业绩记录（按id倒序，可按user_ids过滤）
func (r *userPerformanceRepository) GetLatestPerformancePage(ctx context.Context, req *GetLatestPerformancePageReq) (*GetLatestPerformancePageRes, error) {
	if req == nil {
		return &GetLatestPerformancePageRes{
			PageRes: model.PageRes{},
			List:    []*rewardEntity.UserPerformanceEntity{},
		}, nil
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	list, total, err := r.performanceDao.GetLatestPerformancePage(ctx, page, pageSize, req.UserIDs, req.RecordDate)
	if err != nil {
		return nil, err
	}

	pages := 0
	if total > 0 {
		pages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	return &GetLatestPerformancePageRes{
		PageRes: model.PageRes{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
			Pages:    pages,
		},
		List: list,
	}, nil
}

// GetLatestRecordDate 获取业绩最新记录日期
func (r *userPerformanceRepository) GetLatestRecordDate(ctx context.Context) (time.Time, error) {
	return r.performanceDao.GetLatestRecordDate(ctx)
}

// GetByDateRange 按日期范围和user_ids分页查询业绩记录（按id倒序）
func (r *userPerformanceRepository) GetByDateRange(ctx context.Context, req *GetPerformanceHistoryPageReq) (*GetPerformanceHistoryPageRes, error) {
	if req == nil {
		return &GetPerformanceHistoryPageRes{
			PageRes: model.PageRes{},
			List:    []*rewardEntity.UserPerformanceEntity{},
		}, nil
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	list, total, err := r.performanceDao.GetByDateRange(ctx, req.Date, req.UserIDs, page, pageSize)
	if err != nil {
		return nil, err
	}

	pages := 0
	if total > 0 {
		pages = int(math.Ceil(float64(total) / float64(pageSize)))
	}

	return &GetPerformanceHistoryPageRes{
		PageRes: model.PageRes{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
			Pages:    pages,
		},
		List: list,
	}, nil
}
