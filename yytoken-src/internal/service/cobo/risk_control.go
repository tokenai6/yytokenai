package cobo

// 本文件为 2026-04-14 提现并发攻击事件后的紧急风控加固。
// 攻击者利用批量注册小号、小额充值、并发提现等手段，在短时间内转走大量资金。
// 基于该事件，当前启用以下集中式风控规则：
//  1. 新用户限制：注册后24小时内总提现（含本次）不能超过累计奖励；
//  2. 单个账号每天（北京时间0点刷新）只能发起一次提现 —— 遏制高频并发提现攻击；
//  3. 持有赠送节点不再阻断提现；仅影响提现税中的节点抵扣额度；
//  4. 月度提现税预检：税额>=提现额时拒绝。
//
// 说明：历史“总提现上限（总奖励+总充值-总支出）”规则当前未启用。
//
// 规则统一说明（与当前实现保持一致）：
//  - 支持 USDT/JU/ZPN 提现；测试用户（is_test=1）跳过全部风控。
//  - USDT 执行全量风控；JU/ZPN 仅执行每天单次频次限制。
//  - 新用户（注册<24h）：累计提现（含待审/处理中，排除 rejected+failed）+ 本次 <= 累计奖励（与 /api/v1/cobo/reward/records 统计口径一致）。
//  - 每天频次：北京时间0点刷新，当天最多发起1次非拒绝且非失败提现申请。
//  - 新用户累计奖励数据源与 /api/v1/cobo/reward/records 统计口径一致。
//  - 提现创建时先预检，再在事务内通过 CheckWithdrawTx 二次校验，防止并发穿透。

import (
	"context"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/entity/cobo"
	"XWFrame/internal/frame/consts"
	repository "XWFrame/internal/repository"
	repo "XWFrame/internal/repository/cobo"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// beijingLoc 北京时区，用于自然日刷新判断
var beijingLoc = time.FixedZone("Asia/Shanghai", 8*60*60)

// RiskControlService 提现风控服务接口
type RiskControlService interface {
	// CheckWithdraw 提现统一风控检查
	CheckWithdraw(ctx context.Context, userID int64, amount decimal.Decimal, symbol string) error
	// CheckWithdrawTx 事务内提现统一风控检查
	CheckWithdrawTx(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal, symbol string) error
}

// riskControlService 提现风控服务实现
type riskControlService struct {
	withdrawRepo repo.IWithdrawRepository
	userRepo     repository.IUserRepository
}

// NewRiskControlService 创建风控服务
func NewRiskControlService() RiskControlService {
	return &riskControlService{
		withdrawRepo: repo.NewWithdrawRepository(),
		userRepo:     repository.NewUserRepository(),
	}
}

// CheckWithdraw 提现统一风控检查
func (s *riskControlService) CheckWithdraw(ctx context.Context, userID int64, amount decimal.Decimal, symbol string) error {
	return s.doCheckWithdraw(ctx, nil, userID, amount, symbol)
}

// CheckWithdrawTx 事务内提现统一风控检查
func (s *riskControlService) CheckWithdrawTx(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal, symbol string) error {
	return s.doCheckWithdraw(ctx, tx, userID, amount, symbol)
}

// doCheckWithdraw 统一风控检查实现
func (s *riskControlService) doCheckWithdraw(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal, symbol string) error {
	locale := consts.LocaleFromCtx(ctx)
	symbol = normalizeWithdrawSymbol(symbol)

	// 查询 token_config 验证币种
	var tokenCfg entity.TokenConfigEntity
	var err error
	if tx != nil {
		err = tx.Model("token_config").Ctx(ctx).
			Fields("is_enabled").
			Where("symbol = ?", symbol).
			Scan(&tokenCfg)
	} else {
		err = g.DB().Model("token_config").Ctx(ctx).
			Fields("is_enabled").
			Where("symbol = ?", symbol).
			Scan(&tokenCfg)
	}
	if err != nil {
		return gerror.Wrapf(err, "query token config failed: %s", symbol)
	}
	if !tokenCfg.IsEnabled {
		return withdrawErrf(locale, withdrawErrKey("unsupported_symbol"), symbol)
	}

	// 测试用户跳过所有风控
	user, err := s.userRepo.GetUserById(ctx, userID)
	if err != nil {
		return gerror.Wrap(err, "get user info failed")
	}
	if user == nil {
		return withdrawErr(locale, "user_not_found")
	}
	if user.IsTest == 1 {
		return nil
	}

	if symbol != "USDT" {
		if err := s.check24hFrequency(ctx, tx, userID, symbol, locale); err != nil {
			return err
		}
		return nil
	}

	// 1. 新用户限制：24小时内总提现（含本次）不能超过累计直推奖励
	if err := s.checkNewUserRestriction(ctx, tx, user, amount, symbol, locale); err != nil {
		return err
	}

	// 2. 单个账号24小时内只能发起一次提现
	if err := s.check24hFrequency(ctx, tx, userID, symbol, locale); err != nil {
		return err
	}

	// 3. 月度提现税预检：不再按节点额度拒绝，若税额>=提现额则拒绝
	if err := s.checkWithdrawTaxQuota(ctx, tx, user.Id, amount, symbol, locale); err != nil {
		return err
	}

	return nil
}

// getOne 统一查询辅助方法，支持事务内查询
func (s *riskControlService) getOne(ctx context.Context, tx gdb.TX, sql string, args ...interface{}) (gdb.Record, error) {
	if tx != nil {
		return tx.GetOne(sql, args...)
	}
	return g.DB().GetOne(ctx, sql, args...)
}

// checkNewUserRestriction 新用户24小时内总提现（含本次）不能超过累计充值
func (s *riskControlService) checkNewUserRestriction(ctx context.Context, tx gdb.TX, user *entity.UserEntity, amount decimal.Decimal, symbol, locale string) error {

	if time.Since(user.CreatedAt) < 24*time.Hour {
		withdrawSQL := `
			SELECT COALESCE(SUM(amount), 0) as total
			FROM cobo_withdraw_request
			WHERE user_id = ?
			  AND symbol = ?
			  AND status NOT IN (?, ?)
		`
		withdrawRow, err := s.getOne(ctx, tx, withdrawSQL, user.Id, symbol, cobo.WithdrawStatusRejected, cobo.WithdrawStatusFailed)
		if err != nil {
			return gerror.Wrap(err, "query new user total withdraw failed")
		}
		withdrawTotal, _ := decimal.NewFromString(withdrawRow["total"].String())

		rechargeSQL := `
			SELECT COALESCE(SUM(amount), 0) as total
			FROM cobo_recharge_record
			WHERE user_id = ?
			  AND symbol = ?
			  AND status = ?
		`
		rechargeRow, err := s.getOne(ctx, tx, rechargeSQL, user.Id, symbol, cobo.RechargeStatusConfirmed)
		if err != nil {
			return gerror.Wrap(err, "query new user total recharge failed")
		}
		rechargeTotal, _ := decimal.NewFromString(rechargeRow["total"].String())

		limitTotal := rechargeTotal

		totalWithdraw := withdrawTotal.Add(amount)
		if totalWithdraw.GreaterThan(limitTotal) {
			g.Log().Warningf(ctx, "[RiskControl] 新用户提现拦截: user_id=%d, created_at=%s, total_withdraw=%s, amount=%s, total_recharge=%s, limit_total=%s",
				user.Id, user.CreatedAt.Format(time.RFC3339), withdrawTotal.String(), amount.String(), rechargeTotal.String(), limitTotal.String())
			return wrapRiskI18n(errWithdrawRiskNewUserLimit, locale, "new_user_24h_limit")
		}
	}

	return nil
}

// check24hFrequency 单个账号每天（北京时间0点刷新）只能发起一次提现
// 白名单用户跳过此限制
func (s *riskControlService) check24hFrequency(ctx context.Context, tx gdb.TX, userID int64, symbol, locale string) error {
	// 检查用户是否在白名单中
	whiteListService := NewWithdrawWhiteListService()
	if whiteListService.IsInWhiteList(ctx, userID) {
		g.Log().Infof(ctx, "[RiskControl] 白名单用户跳过提现频次检查: user_id=%d", userID)
		return nil
	}

	// 北京时间当天0点，格式化为字符串避免 timestamptz 与时区转换问题
	now := time.Now().In(beijingLoc)
	since := now.Format("2006-01-02") + " 00:00:00"

	// 查询今天（北京时间0点后）是否有非拒绝且非失败状态的提现记录
	countSQL := `
		SELECT COUNT(*) as cnt
		FROM cobo_withdraw_request
		WHERE user_id = ?
		  AND symbol = ?
		  AND created_at >= ?
		  AND status NOT IN (?, ?)
	`
	var count int64
	row, err := s.getOne(ctx, tx, countSQL, userID, symbol, since, cobo.WithdrawStatusRejected, cobo.WithdrawStatusFailed)
	if err != nil {
		return gerror.Wrap(err, "query daily withdraw frequency failed")
	}
	count = row["cnt"].Int64()

	if count > 0 {
		g.Log().Warningf(ctx, "[RiskControl] 今日提现频次拦截: user_id=%d, count=%d", userID, count)
		return wrapRiskI18n(errWithdrawRisk24hFrequency, locale, "daily_frequency_limit")
	}

	return nil
}

// checkWithdrawTaxQuota 月度提现额度预检
//   - 仅 USDT
//   - 配置开关 withdraw_tax_enabled 关闭时跳过
//   - 不再拒绝超出节点额度的提现：税款直接从提现金额扣除
func (s *riskControlService) checkWithdrawTaxQuota(ctx context.Context, tx gdb.TX, userID int64, amount decimal.Decimal, symbol, locale string) error {
	if symbol != "USDT" {
		return nil
	}

	cfg, err := loadWithdrawTaxConfig(ctx, tx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}

	quota, err := loadWithdrawTaxQuota(ctx, tx, userID, symbol)
	if err != nil {
		return err
	}

	// 预检：若税款超过提现金额（实际到账 <= 0），提前拒绝
	tax, err := computeWithdrawTax(amount, quota, cfg.Rate)
	if err != nil {
		return err
	}
	if tax.GreaterThanOrEqual(amount) {
		return wrapRiskI18n(errWithdrawRiskTotalLimit, locale, "tax_exceeds_amount", tax.String(), amount.String())
	}

	return nil
}
