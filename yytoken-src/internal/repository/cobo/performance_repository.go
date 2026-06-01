package cobo

import (
	"context"

	coboDao "XWFrame/internal/dao/cobo"
	coboEntity "XWFrame/internal/entity/cobo"

	"github.com/shopspring/decimal"
)

// IPerformanceRepository cobo_performance 仓储接口
type IPerformanceRepository interface {
	// GetByUserID 根据用户ID获取缓存业绩
	GetByUserID(ctx context.Context, userID int64) (*coboEntity.PerformanceEntity, error)

	// GetByInviteCode 根据邀请码获取缓存业绩
	GetByInviteCode(ctx context.Context, inviteCode string) (*coboEntity.PerformanceEntity, error)

	// GetByWalletAddress 根据钱包地址获取缓存业绩
	GetByWalletAddress(ctx context.Context, walletAddress string) (*coboEntity.PerformanceEntity, error)

	// GetDirectList 获取直推用户的缓存业绩列表
	GetDirectList(ctx context.Context, parentInviteCode string, page, pageSize int) ([]*coboEntity.PerformanceEntity, int, error)

	// Upsert 插入或更新缓存业绩
	Upsert(ctx context.Context, entity *coboEntity.PerformanceEntity) error

	// BatchUpsert 批量插入或更新缓存业绩
	BatchUpsert(ctx context.Context, entities []*coboEntity.PerformanceEntity) error

	// GetLatestPurchaseID 获取 cobo_node_purchase 表的最大ID
	GetLatestPurchaseID(ctx context.Context) (int64, error)

	// GetPurchaseCount 获取 cobo_node_purchase 表的记录数（非赠送）
	GetPurchaseCount(ctx context.Context) (int, error)

	// Count 获取缓存表记录数
	Count(ctx context.Context) (int, error)

	// GetStaleUsers 获取需要更新的用户列表（超过5分钟未更新）
	GetStaleUsers(ctx context.Context, limit int) ([]*coboEntity.PerformanceEntity, error)

	// GetAffectedUsersByPurchaseRange 获取指定ID范围内新购买记录涉及的用户ID（包括购买用户及其祖先）
	GetAffectedUsersByPurchaseRange(ctx context.Context, minID, maxID int64) ([]int64, error)

	// CalculatePersonalPerformance 计算用户个人业绩
	CalculatePersonalPerformance(ctx context.Context, userID int64) (decimal.Decimal, int, int64, error)

	// CalculateTeamPerformance 计算用户团队业绩
	CalculateTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, int, error)

	// CalculateSmallTeamPerformance 计算用户小区业绩
	CalculateSmallTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, error)
}

// performanceRepository cobo_performance 仓储实现
type performanceRepository struct {
	dao coboDao.IPerformanceDao
}

// NewPerformanceRepository 创建仓储实例
func NewPerformanceRepository() IPerformanceRepository {
	return &performanceRepository{
		dao: coboDao.NewPerformanceDao(),
	}
}

// GetByUserID 根据用户ID获取缓存业绩
func (r *performanceRepository) GetByUserID(ctx context.Context, userID int64) (*coboEntity.PerformanceEntity, error) {
	return r.dao.GetByUserID(ctx, userID)
}

// GetByInviteCode 根据邀请码获取缓存业绩
func (r *performanceRepository) GetByInviteCode(ctx context.Context, inviteCode string) (*coboEntity.PerformanceEntity, error) {
	return r.dao.GetByInviteCode(ctx, inviteCode)
}

// GetByWalletAddress 根据钱包地址获取缓存业绩
func (r *performanceRepository) GetByWalletAddress(ctx context.Context, walletAddress string) (*coboEntity.PerformanceEntity, error) {
	return r.dao.GetByWalletAddress(ctx, walletAddress)
}

// GetDirectList 获取直推用户的缓存业绩列表
func (r *performanceRepository) GetDirectList(ctx context.Context, parentInviteCode string, page, pageSize int) ([]*coboEntity.PerformanceEntity, int, error) {
	return r.dao.GetDirectList(ctx, parentInviteCode, page, pageSize)
}

// Upsert 插入或更新缓存业绩
func (r *performanceRepository) Upsert(ctx context.Context, entity *coboEntity.PerformanceEntity) error {
	return r.dao.Upsert(ctx, entity)
}

// BatchUpsert 批量插入或更新缓存业绩
func (r *performanceRepository) BatchUpsert(ctx context.Context, entities []*coboEntity.PerformanceEntity) error {
	return r.dao.BatchUpsert(ctx, entities)
}

// GetLatestPurchaseID 获取 cobo_node_purchase 表的最大ID
func (r *performanceRepository) GetLatestPurchaseID(ctx context.Context) (int64, error) {
	return r.dao.GetLatestPurchaseID(ctx)
}

// GetPurchaseCount 获取 cobo_node_purchase 表的记录数（非赠送）
func (r *performanceRepository) GetPurchaseCount(ctx context.Context) (int, error) {
	return r.dao.GetPurchaseCount(ctx)
}

// Count 获取缓存表记录数
func (r *performanceRepository) Count(ctx context.Context) (int, error) {
	return r.dao.Count(ctx)
}

// GetStaleUsers 获取需要更新的用户列表（超过5分钟未更新）
func (r *performanceRepository) GetStaleUsers(ctx context.Context, limit int) ([]*coboEntity.PerformanceEntity, error) {
	return r.dao.GetStaleUsers(ctx, limit)
}

// GetAffectedUsersByPurchaseRange 获取指定ID范围内新购买记录涉及的用户ID（包括购买用户及其祖先）
func (r *performanceRepository) GetAffectedUsersByPurchaseRange(ctx context.Context, minID, maxID int64) ([]int64, error) {
	return r.dao.GetAffectedUsersByPurchaseRange(ctx, minID, maxID)
}

// CalculatePersonalPerformance 计算用户个人业绩
func (r *performanceRepository) CalculatePersonalPerformance(ctx context.Context, userID int64) (decimal.Decimal, int, int64, error) {
	return r.dao.CalculatePersonalPerformance(ctx, userID)
}

// CalculateTeamPerformance 计算用户团队业绩
func (r *performanceRepository) CalculateTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, int, error) {
	return r.dao.CalculateTeamPerformance(ctx, inviteCode)
}

// CalculateSmallTeamPerformance 计算用户小区业绩
func (r *performanceRepository) CalculateSmallTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, error) {
	return r.dao.CalculateSmallTeamPerformance(ctx, inviteCode)
}
