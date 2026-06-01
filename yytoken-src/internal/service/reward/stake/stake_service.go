package stake

import (
	"context"
	"database/sql"
	"errors"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	rewardRepo "XWFrame/internal/repository/reward"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// IStakeService 质押统计服务接口
type IStakeService interface {
	// CalculateDailyStakeStatistics 计算每日质押统计
	CalculateDailyStakeStatistics(ctx context.Context, date time.Time) error
}

// stakeService 质押统计服务实现
type stakeService struct {
	stakeStatsRepo rewardRepo.IStakeStatisticsRepository
}

// NewStakeService 创建质押统计服务实例
func NewStakeService() IStakeService {
	return &stakeService{
		stakeStatsRepo: rewardRepo.NewStakeStatisticsRepository(),
	}
}

// CalculateDailyStakeStatistics 计算每日质押统计（业务逻辑层）
func (s *stakeService) CalculateDailyStakeStatistics(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[质押统计] 开始统计每日质押数据: date=%s", date.Format(consts.TimeFormatDate))

	// 幂等性检查：在事务开始前先查询是否已统计过（业务逻辑）
	existingStats, err := s.stakeStatsRepo.GetByDate(ctx, date)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return gerror.Wrap(err, "检查是否已统计失败")
	}
	if existingStats != nil {
		g.Log().Infof(ctx, "[质押统计] 数据已存在，跳过统计: date=%s", date.Format(consts.TimeFormatDate))
		return nil
	}

	// 查询该日期已有的用户统计，用于幂等检查
	existingUserStats, err := s.stakeStatsRepo.GetUserStatsByDate(ctx, date)
	if err != nil {
		return gerror.Wrap(err, "查询已有用户统计失败")
	}

	// 构建已统计用户的 Map（业务逻辑）
	existingUserMap := make(map[int64]bool)
	for _, stats := range existingUserStats {
		existingUserMap[stats.UserID] = true
	}
	g.Log().Infof(ctx, "[质押统计] 已存在用户统计数: %d", len(existingUserMap))

	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 1. 统计全网数据
		globalStats, err := s.stakeStatsRepo.CalculateGlobalStats(ctx, date)
		if err != nil {
			return gerror.Wrap(err, "统计全网数据失败")
		}

		// 2. 保存全网统计
		if err := s.stakeStatsRepo.CreateDailyStats(ctx, tx, globalStats); err != nil {
			return gerror.Wrap(err, "保存全网统计失败")
		}

		g.Log().Infof(ctx, "[质押统计] 全网统计: 新增质押=%s, 笔数=%d, 用户数=%d",
			globalStats.TotalNewStakeAmount.String(),
			globalStats.TotalNewStakeCount,
			globalStats.TotalNewUsers)

		// 3. 统计每个用户数据
		userStats, err := s.stakeStatsRepo.CalculateUserStats(ctx, date)
		if err != nil {
			return gerror.Wrap(err, "统计用户数据失败")
		}

		// 4. 过滤已统计的用户（幂等性检查，业务逻辑）
		newUserStats := make([]*rewardEntity.UserDailyStakeStatisticsEntity, 0, len(userStats))
		skipCount := 0
		for _, stats := range userStats {
			if existingUserMap[stats.UserID] {
				skipCount++
				continue
			}
			newUserStats = append(newUserStats, stats)
		}

		if skipCount > 0 {
			g.Log().Infof(ctx, "[质押统计] 跳过已统计用户数: %d", skipCount)
		}

		// 5. 批量保存新用户统计
		if len(newUserStats) > 0 {
			if err := s.stakeStatsRepo.BatchCreateUserStats(ctx, tx, newUserStats); err != nil {
				return gerror.Wrap(err, "批量保存用户统计失败")
			}
			g.Log().Infof(ctx, "[质押统计] 用户统计: 共%d个用户有新增质押（新增%d条记录）", len(userStats), len(newUserStats))
		} else {
			g.Log().Infof(ctx, "[质押统计] 无新用户需要统计")
		}

		return nil
	})
}
