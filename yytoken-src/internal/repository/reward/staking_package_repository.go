package reward

import (
	rewardDao "XWFrame/internal/dao/reward"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/model"
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IStakingPackageRepository 质押包仓储接口
type IStakingPackageRepository interface {
	// GetPagedList 分页获取质押包列表
	GetPagedList(ctx context.Context, req *GetStakingPackageListReq) (*GetStakingPackageListRes, error)

	// GetUserStakingStats 获取用户质押统计数据
	GetUserStakingStats(ctx context.Context, userID int64) (*rewardDao.UserStakingStats, error)

	// GetByUserIDs 根据用户ID批量查询质押包
	GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.StakingPackageEntity, error)

	// GetByUserID 根据用户ID查询所有质押包
	GetByUserID(ctx context.Context, userID int64) ([]*rewardEntity.StakingPackageEntity, error)

	// Create 创建质押包（可在事务中调用）
	Create(ctx context.Context, tx gdb.TX, pkg *rewardEntity.StakingPackageEntity) error

	// GetByTxHash 根据交易哈希查询质押包（用于幂等）
	GetByTxHash(ctx context.Context, txHash string) (*rewardEntity.StakingPackageEntity, error)

	// SumStakeAmountByUserIDs 根据用户ID列表统计质押金额
	SumStakeAmountByUserIDs(ctx context.Context, userIDs []int64) (decimal.Decimal, error)

	// SumStakeAmountByUserIDsWithStatus 根据用户ID列表统计质押金额（仅统计运行中的质押包，status=1）
	SumStakeAmountByUserIDsWithStatus(ctx context.Context, userIDs []int64) (decimal.Decimal, error)

	// SumStakeAmountByUserIDsWithPowerMultiplier 根据用户ID列表统计质押金额（只统计 power_multiplier=1 的数据，必须有 user_id）
	SumStakeAmountByUserIDsWithPowerMultiplier(ctx context.Context, userIDs []int64) (decimal.Decimal, error)

	// GetStakeAmountMapByUserIDsWithPowerMultiplier 批量查询用户 stake_amount 汇总（按 user_id 分组，只统计 power_multiplier=1）
	GetStakeAmountMapByUserIDsWithPowerMultiplier(ctx context.Context, userIDs []int64) (map[int64]decimal.Decimal, error)

	// GetByPackageNo 根据包编号查询质押包
	GetByPackageNo(ctx context.Context, packageNo string) (*rewardEntity.StakingPackageEntity, error)

	// GetByID 根据ID查询质押包
	GetByID(ctx context.Context, packageID int64) (*rewardEntity.StakingPackageEntity, error)

	// UpdateStakingAdjustment 更新质押包调整信息（类型、倍数、额度等）
	UpdateStakingAdjustment(ctx context.Context, tx gdb.TX, packageID int64, updateData map[string]interface{}) error

	// CheckStakeEventProcessed 检查质押事件是否已处理
	CheckStakeEventProcessed(ctx context.Context, txHash string, blockNumber uint64) (bool, error)
}

// GetStakingPackageListReq 分页获取质押包列表请求
type GetStakingPackageListReq struct {
	model.PageReq
	UserID        int64  `json:"user_id"`        // 用户ID（必填）
	Status        int    `json:"status"`         // 状态：1-运行中，2-历史质押（status in 2,3,4）
	StakeTypes    []int  `json:"stake_types"`    // 质押类型列表（可选，如 [2, 4]）
	TokenContract string `json:"token_contract"` // 质押合约地址
}

// GetStakingPackageListRes 分页获取质押包列表响应
type GetStakingPackageListRes struct {
	model.PageRes
	List []*rewardEntity.StakingPackageEntity `json:"list"`
}

// stakingPackageRepository 质押包仓储实现
type stakingPackageRepository struct {
	stakingPackageDao rewardDao.IStakingPackageDao
}

// NewStakingPackageRepository 创建质押包仓储实例
func NewStakingPackageRepository() IStakingPackageRepository {
	return &stakingPackageRepository{
		stakingPackageDao: rewardDao.NewStakingPackageDao(),
	}
}

// GetPagedList 分页获取质押包列表
func (r *stakingPackageRepository) GetPagedList(ctx context.Context, req *GetStakingPackageListReq) (*GetStakingPackageListRes, error) {
	list, total, err := r.stakingPackageDao.GetPagedList(ctx, &rewardDao.GetStakingPackageListReq{
		UserID:        req.UserID,
		Status:        req.Status,
		StakeTypes:    req.StakeTypes,
		Page:          req.Page,
		PageSize:      req.PageSize,
		TokenContract: req.TokenContract,
	})
	if err != nil {
		return nil, err
	}

	// Repository 层只返回原始数据和总数，不计算总页数（业务逻辑应在 Service 层）
	return &GetStakingPackageListRes{
		PageRes: model.PageRes{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    total,
			Pages:    0, // Service 层会计算
		},
		List: list,
	}, nil
}

// GetUserStakingStats 获取用户质押统计数据
func (r *stakingPackageRepository) GetUserStakingStats(ctx context.Context, userID int64) (*rewardDao.UserStakingStats, error) {
	return r.stakingPackageDao.GetUserStakingStats(ctx, userID)
}

// GetByUserIDs 根据用户ID批量查询质押包
func (r *stakingPackageRepository) GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]*rewardEntity.StakingPackageEntity, error) {
	return r.stakingPackageDao.GetByUserIDs(ctx, userIDs)
}

// GetByUserID 根据用户ID查询所有质押包
func (r *stakingPackageRepository) GetByUserID(ctx context.Context, userID int64) ([]*rewardEntity.StakingPackageEntity, error) {
	return r.stakingPackageDao.GetByUserID(ctx, userID)
}

// Create 创建质押包（可在事务中调用）
func (r *stakingPackageRepository) Create(ctx context.Context, tx gdb.TX, pkg *rewardEntity.StakingPackageEntity) error {
	return r.stakingPackageDao.Create(ctx, tx, pkg)
}

// GetByTxHash 根据交易哈希查询质押包（用于幂等）
func (r *stakingPackageRepository) GetByTxHash(ctx context.Context, txHash string) (*rewardEntity.StakingPackageEntity, error) {
	return r.stakingPackageDao.GetByTxHash(ctx, txHash)
}

// GetByPackageNo 根据包编号查询质押包
func (r *stakingPackageRepository) GetByPackageNo(ctx context.Context, packageNo string) (*rewardEntity.StakingPackageEntity, error) {
	return r.stakingPackageDao.GetByPackageNo(ctx, packageNo)
}

// SumStakeAmountByUserIDs 根据用户ID列表统计质押金额
func (r *stakingPackageRepository) SumStakeAmountByUserIDs(ctx context.Context, userIDs []int64) (decimal.Decimal, error) {
	return r.stakingPackageDao.SumStakeAmountByUserIDs(ctx, userIDs)
}

// SumStakeAmountByUserIDsWithStatus 根据用户ID列表统计质押金额（仅统计运行中的质押包，status=1）
func (r *stakingPackageRepository) SumStakeAmountByUserIDsWithStatus(ctx context.Context, userIDs []int64) (decimal.Decimal, error) {
	return r.stakingPackageDao.SumStakeAmountByUserIDsWithStatus(ctx, userIDs)
}

// SumStakeAmountByUserIDsWithPowerMultiplier 根据用户ID列表统计质押金额（只统计 power_multiplier=1 的数据，必须有 user_id）
func (r *stakingPackageRepository) SumStakeAmountByUserIDsWithPowerMultiplier(ctx context.Context, userIDs []int64) (decimal.Decimal, error) {
	return r.stakingPackageDao.SumStakeAmountByUserIDsWithPowerMultiplier(ctx, userIDs)
}

func (r *stakingPackageRepository) GetStakeAmountMapByUserIDsWithPowerMultiplier(ctx context.Context, userIDs []int64) (map[int64]decimal.Decimal, error) {
	return r.stakingPackageDao.GetStakeAmountMapByUserIDsWithPowerMultiplier(ctx, userIDs)
}

// GetByID 根据ID查询质押包
func (r *stakingPackageRepository) GetByID(ctx context.Context, packageID int64) (*rewardEntity.StakingPackageEntity, error) {
	return r.stakingPackageDao.GetByID(ctx, packageID)
}

// UpdateStakingAdjustment 更新质押包调整信息（类型、倍数、额度等）
func (r *stakingPackageRepository) UpdateStakingAdjustment(ctx context.Context, tx gdb.TX, packageID int64, updateData map[string]interface{}) error {
	return r.stakingPackageDao.UpdateStakingAdjustment(ctx, tx, packageID, updateData)
}

// CheckStakeEventProcessed 检查质押事件是否已处理
func (r *stakingPackageRepository) CheckStakeEventProcessed(ctx context.Context, txHash string, blockNumber uint64) (bool, error) {
	return r.stakingPackageDao.CheckStakeEventProcessed(ctx, txHash, blockNumber)
}
