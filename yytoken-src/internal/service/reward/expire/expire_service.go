package expire

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	rewardRepo "XWFrame/internal/repository/reward"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// IExpireService 出局检查服务接口
type IExpireService interface {
	// CheckAndHandleExpiredUsers 检查并处理出局用户
	CheckAndHandleExpiredUsers(ctx context.Context, date time.Time) error
}

// expireService 出局检查服务实现
type expireService struct {
	expireRepo rewardRepo.IUserExpireRepository
}

// NewExpireService 创建出局检查服务实例
func NewExpireService() IExpireService {
	return &expireService{
		expireRepo: rewardRepo.NewUserExpireRepository(),
	}
}

// CheckAndHandleExpiredUsers 清理所有出局用户的数据（业务逻辑层）
func (s *expireService) CheckAndHandleExpiredUsers(ctx context.Context, date time.Time) error {
	g.Log().Infof(ctx, "[出局后处理] 开始清理出局用户数据: date=%s", date.Format(consts.TimeFormatDate))

	// 1. 查询所有已出局的用户
	expiredUsers, err := s.expireRepo.GetExpiredUsers(ctx)
	if err != nil {
		return gerror.Wrap(err, "查询出局用户失败")
	}

	if len(expiredUsers) == 0 {
		g.Log().Info(ctx, "[出局后处理] 无出局用户需要清理")
		return nil
	}

	g.Log().Infof(ctx, "[出局后处理] 发现%d个出局用户需要清理", len(expiredUsers))

	successCount := 0
	failedCount := 0

	// 2. 遍历清理每个出局用户
	for _, quota := range expiredUsers {
		err = s.cleanupExpiredUser(ctx, quota.UserID, date)
		if err != nil {
			g.Log().Errorf(ctx, "[出局后处理] 清理失败: userID=%d, err=%v", quota.UserID, err)
			failedCount++
			continue
		}
		successCount++
	}

	g.Log().Infof(ctx, "[出局后处理] 清理完成: 成功=%d, 失败=%d", successCount, failedCount)
	return nil
}

// cleanupExpiredUser 清理出局用户的数据（业务逻辑）
func (s *expireService) cleanupExpiredUser(ctx context.Context, userID int64, date time.Time) error {
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 1. 将当前统计字段重置为0（不影响剩余额度/已用额度/历史字段）
		if err := s.expireRepo.ResetCurrentStakeToZero(ctx, tx, userID); err != nil {
			return gerror.Wrap(err, "重置用户当前统计字段失败")
		}

		// 2. 查询用户所有已出局的算力包
		expiredPackages, err := s.expireRepo.GetExpiredPackagesByUserID(ctx, tx, userID)
		if err != nil {
			return gerror.Wrap(err, "查询用户已出局算力包失败")
		}

		if len(expiredPackages) == 0 {
			g.Log().Warningf(ctx, "[出局后处理] 用户无已出局的算力包: userID=%d", userID)
			return nil
		}

		// 3. 累计需要扣减的当前额度（业务逻辑）
		var totalStakeAmountToDeduct = decimal.Zero
		var totalQuotaToDeduct = decimal.Zero
		var maxStaticReleaseToDeduct = decimal.Zero

		for _, pkg := range expiredPackages {
			totalStakeAmountToDeduct = totalStakeAmountToDeduct.Add(pkg.StakeAmount)
			totalQuotaToDeduct = totalQuotaToDeduct.Add(pkg.TotalQuota)
			maxStaticReleaseToDeduct = maxStaticReleaseToDeduct.Add(pkg.MaxStaticRelease)
		}

		g.Log().Infof(ctx, "[出局后处理] 用户需扣减: userID=%d, 质押=%s, 额度=%s, 静态上限=%s, 包数=%d",
			userID,
			totalStakeAmountToDeduct.String(),
			totalQuotaToDeduct.String(),
			maxStaticReleaseToDeduct.String(),
			len(expiredPackages))

		// 4. 清零业绩（记录清零前快照）
		var metadataStr string
		if perfBefore, _ := s.expireRepo.GetPerformanceByUserAndDate(ctx, userID, date); perfBefore != nil {
			snapshot := map[string]string{
				"personal_performance":     perfBefore.PersonalPerformance.String(),
				"new_personal_performance": perfBefore.NewPersonalPerformance.String(),
				"team_performance":         perfBefore.TeamPerformance.String(),
				"district_performance":     perfBefore.DistrictPerformance.String(),
				"max_district_performance": perfBefore.MaxDistrictPerformance.String(),
				"new_direct_performance":   perfBefore.NewDirectPerformance.String(),
				"new_team_performance":     perfBefore.NewTeamPerformance.String(),
				"new_district_performance": perfBefore.NewDistrictPerformance.String(),
				"total_power_value":        perfBefore.TotalPowerValue.String(),
				"vip_level":                fmt.Sprintf("%d", perfBefore.VipLevel),
			}
			if b, e := json.Marshal(snapshot); e == nil {
				metadataStr = string(b)
			}
		}

		remarkPerf := fmt.Sprintf("出局清零业绩，时间=%s", date.Format(consts.TimeFormatDate))
		if err := s.expireRepo.UpdatePerformanceToZeroWithAudit(ctx, tx, userID, date, remarkPerf, metadataStr); err != nil {
			g.Log().Warningf(ctx, "[出局检查] 清零业绩失败: userID=%d, err=%v", userID, err)
		}

		// 5. 降级VIP等级为0（业务逻辑）
		offset0 := 0
		vipRecords, err := s.expireRepo.GetUserVIPByOffset(ctx, userID, &offset0)
		if err != nil {
			g.Log().Warningf(ctx, "[出局检查] 获取当前VIP失败: userID=%d, err=%v", userID, err)
		}

		var currentVIP *rewardEntity.UserVipLevelEntity
		if len(vipRecords) > 0 {
			currentVIP = vipRecords[0]
		}

		if currentVIP != nil {
			// 存在VIP记录，直接更新为VIP0
			prevLevel := currentVIP.VipLevel
			currentVIP.VipLevel = 0
			currentVIP.DistrictPerformance = decimal.Zero
			currentVIP.RewardRate = decimal.Zero
			currentVIP.RecordTime = date
			currentVIP.PrevVipLevel = &prevLevel

			if err := s.expireRepo.UpdateVIP(ctx, tx, currentVIP); err != nil {
				g.Log().Warningf(ctx, "[出局检查] 更新VIP为VIP0失败: userID=%d, err=%v", userID, err)
			} else {
				g.Log().Infof(ctx, "[出局检查] VIP降级: userID=%d, %d -> 0", userID, prevLevel)
			}
		} else {
			// 不存在VIP记录，创建新的VIP0记录
			newVIP := &rewardEntity.UserVipLevelEntity{
				UserID:              userID,
				VipLevel:            0,
				DistrictPerformance: decimal.Zero,
				RewardRate:          decimal.Zero,
				RecordTime:          date,
				PrevVipLevel:        nil,
				IsCurrent:           true,
			}
			if err := s.expireRepo.CreateVIP(ctx, tx, newVIP); err != nil {
				g.Log().Warningf(ctx, "[出局检查] 创建VIP0记录失败: userID=%d, err=%v", userID, err)
			} else {
				g.Log().Infof(ctx, "[出局检查] 创建VIP0记录: userID=%d", userID)
			}
		}

		// 6. 节点权益已整合到 staking_package 中，不需要单独处理

		g.Log().Infof(ctx, "[出局后处理] 用户清理完成: userID=%d, 扣减质押=%s, 扣减额度=%s",
			userID, totalStakeAmountToDeduct.String(), totalQuotaToDeduct.String())

		return nil
	})
}
