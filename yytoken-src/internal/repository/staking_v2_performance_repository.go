package repository

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"

	"github.com/shopspring/decimal"
)

// IStakingV2PerformanceRepository staking_v2_performance 仓储接口
type IStakingV2PerformanceRepository interface {
	// GetByUserID 根据用户ID获取缓存业绩
	GetByUserID(ctx context.Context, userID int64) (*entity.StakingV2PerformanceEntity, error)

	// GetByInviteCode 根据邀请码获取缓存业绩
	GetByInviteCode(ctx context.Context, inviteCode string) (*entity.StakingV2PerformanceEntity, error)

	// GetByWalletAddress 根据钱包地址获取缓存业绩
	GetByWalletAddress(ctx context.Context, walletAddress string) (*entity.StakingV2PerformanceEntity, error)

	// Upsert 插入或更新缓存业绩
	Upsert(ctx context.Context, entity *entity.StakingV2PerformanceEntity) error

	// BatchUpsert 批量插入或更新缓存业绩
	BatchUpsert(ctx context.Context, entities []*entity.StakingV2PerformanceEntity) error

	// GetLatestOrderID 获取 staking_v2_order 表的最大ID（source_type=1）
	GetLatestOrderID(ctx context.Context) (int64, error)

	// Count 获取缓存表记录数
	Count(ctx context.Context) (int, error)

	// GetStaleUsers 获取需要更新的用户列表（超过5分钟未更新）
	GetStaleUsers(ctx context.Context, limit int) ([]*entity.StakingV2PerformanceEntity, error)

	// GetAffectedUsersByOrderRange 获取指定ID范围内新订单涉及的用户ID（包括购买用户及其祖先）
	GetAffectedUsersByOrderRange(ctx context.Context, minID, maxID int64) ([]int64, error)

	// CalculatePersonalPerformance 计算用户个人业绩
	CalculatePersonalPerformance(ctx context.Context, userID int64) (decimal.Decimal, int, int64, error)

	// CalculateTeamPerformance 计算用户团队业绩
	CalculateTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, error)
}

// stakingV2PerformanceRepository staking_v2_performance 仓储实现
type stakingV2PerformanceRepository struct {
	dao dao.IStakingV2PerformanceDao
}

// NewStakingV2PerformanceRepository 创建仓储实例
func NewStakingV2PerformanceRepository() IStakingV2PerformanceRepository {
	return &stakingV2PerformanceRepository{
		dao: dao.NewStakingV2PerformanceDao(),
	}
}

// GetByUserID 根据用户ID获取缓存业绩
func (r *stakingV2PerformanceRepository) GetByUserID(ctx context.Context, userID int64) (*entity.StakingV2PerformanceEntity, error) {
	return r.dao.GetByUserID(ctx, userID)
}

// GetByInviteCode 根据邀请码获取缓存业绩
func (r *stakingV2PerformanceRepository) GetByInviteCode(ctx context.Context, inviteCode string) (*entity.StakingV2PerformanceEntity, error) {
	return r.dao.GetByInviteCode(ctx, inviteCode)
}

// GetByWalletAddress 根据钱包地址获取缓存业绩
func (r *stakingV2PerformanceRepository) GetByWalletAddress(ctx context.Context, walletAddress string) (*entity.StakingV2PerformanceEntity, error) {
	return r.dao.GetByWalletAddress(ctx, walletAddress)
}

// Upsert 插入或更新缓存业绩
func (r *stakingV2PerformanceRepository) Upsert(ctx context.Context, entity *entity.StakingV2PerformanceEntity) error {
	return r.dao.Upsert(ctx, entity)
}

// BatchUpsert 批量插入或更新缓存业绩
func (r *stakingV2PerformanceRepository) BatchUpsert(ctx context.Context, entities []*entity.StakingV2PerformanceEntity) error {
	return r.dao.BatchUpsert(ctx, entities)
}

// GetLatestOrderID 获取 staking_v2_order 表的最大ID（source_type=1）
func (r *stakingV2PerformanceRepository) GetLatestOrderID(ctx context.Context) (int64, error) {
	return r.dao.GetLatestOrderID(ctx)
}

// Count 获取缓存表记录数
func (r *stakingV2PerformanceRepository) Count(ctx context.Context) (int, error) {
	return r.dao.Count(ctx)
}

// GetStaleUsers 获取需要更新的用户列表（超过5分钟未更新）
func (r *stakingV2PerformanceRepository) GetStaleUsers(ctx context.Context, limit int) ([]*entity.StakingV2PerformanceEntity, error) {
	return r.dao.GetStaleUsers(ctx, limit)
}

// GetAffectedUsersByOrderRange 获取指定ID范围内新订单涉及的用户ID（包括购买用户及其祖先）
func (r *stakingV2PerformanceRepository) GetAffectedUsersByOrderRange(ctx context.Context, minID, maxID int64) ([]int64, error) {
	return r.dao.GetAffectedUsersByOrderRange(ctx, minID, maxID)
}

// CalculatePersonalPerformance 计算用户个人业绩
func (r *stakingV2PerformanceRepository) CalculatePersonalPerformance(ctx context.Context, userID int64) (decimal.Decimal, int, int64, error) {
	return r.dao.CalculatePersonalPerformance(ctx, userID)
}

// CalculateTeamPerformance 计算用户团队业绩
func (r *stakingV2PerformanceRepository) CalculateTeamPerformance(ctx context.Context, inviteCode string) (decimal.Decimal, error) {
	return r.dao.CalculateTeamPerformance(ctx, inviteCode)
}
