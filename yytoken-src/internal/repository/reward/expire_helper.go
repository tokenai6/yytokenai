package reward

import (
	"context"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// CheckAndHandleUserExpire 检查并处理用户出局（统一方法，所有奖励发放都调用）
//
// 参数:
//   - ctx: 上下文
//   - tx: 数据库事务（必须传入，确保原子性）
//   - userID: 用户ID
//   - quota: 用户额度（已锁定，从GetForUpdate获取，必须是最新数据）
//   - date: 结算日期
//
// 注意:
//   - quota 必须是通过 GetForUpdate 获取的（已加悲观锁，避免并发问题）
//   - 必须在事务中调用
//   - 幂等性：重复调用不会出错（使用 WHERE status = 1）
//
// 返回:
//   - error: 处理失败时返回错误
func CheckAndHandleUserExpire(
	ctx context.Context,
	tx gdb.TX,
	userID int64,
	quota *rewardEntity.UserQuotaEntity, // 已锁定的用户额度
	date time.Time,
) error {
	// 仅保留“静态完成出局”口径：released_static >= max_static_release
	// 旧逻辑：remaining_quota<=0 会触发“额度耗尽出局(status=3)”；该路径已取消。
	isUserStaticComplete := quota.ReleasedStatic.GreaterThanOrEqual(quota.MaxStaticRelease)
	if !isUserStaticComplete {
		return nil
	}

	finalStatus := 2
	statusDesc := "静态完成出局"

	// 3. ⚠️ 更新该用户的所有运行中的算力包状态（关键！）
	// 使用 WHERE status = 1 确保只更新运行中的包（幂等性）
	result, err := tx.Model("staking_package").Ctx(ctx).
		Where("user_id", userID).
		Where("status", 1).
		Data(g.Map{
			"status":          finalStatus,
			"actual_end_time": date,
			"updated_at":      time.Now(),
		}).
		Update()
	if err != nil {
		return gerror.Wrap(err, "更新用户所有算力包状态失败")
	}

	affectedRows, _ := result.RowsAffected()
	g.Log().Infof(ctx, "[出局处理] 更新算力包状态: userID=%d, status=%d(%s), 影响行数=%d",
		userID, finalStatus, statusDesc, affectedRows)

	return nil
}
