package cobo

// 提现税收规则（2026-04-28 落地，2026-05-05 调整）
//  规则定义见 docs/match/withdraw-tax.md
//  - 免税额度：累计充值（终身累计），不清空
//  - 累计提现：终身累计（不含本次，排除 rejected/failed）
//  - 节点抵扣额度：每月 1 号 0 点（北京时间）按月刷新（tax_deduction_used 按月清零）
//  - 无节点用户：本次提现使 W>T（累计提现>累计充值）的差额按 30% 内转给税收接收方
//  - 已购节点用户：超出累计充值的部分按 30% 计税，税款可用节点抵扣额度抵扣；抵扣额度用完后全额缴税
//  - 仅 USDT 适用，JU/ZPN 不参与
//  - 测试用户（is_test=1）跳过

import (
	"context"
	"strings"
	"time"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity/cobo"
	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

var cnLocation, _ = time.LoadLocation("Asia/Shanghai")

const (
	// configKeyEnableTax 历史兼容开关：初代版本用于固定 10% 提现手续费（已废弃）。
	// 现网仅保留为总开关兼容字段：存在时优先于 withdraw_tax_enabled。
	configKeyEnableTax                 = "enable_tax"
	configKeyWithdrawTaxRate            = "withdraw_tax_rate"
	configKeyWithdrawTaxEnabled         = "withdraw_tax_enabled"
	configKeyWithdrawTaxActualRate      = "withdraw_tax_actual_rate"
	defaultWithdrawTaxRateStr           = "0.30"
	defaultWithdrawTaxActualRateStr     = "0.023"
	withdrawTaxChangeTypePay            = consts.ChangeTypeWithdrawTaxPay
	withdrawTaxChangeTypeReceive        = consts.ChangeTypeWithdrawTaxReceive
)

// withdrawTaxConfig 提现税收配置快照
type withdrawTaxConfig struct {
	Enabled    bool
	Rate       decimal.Decimal
	ActualRate decimal.Decimal // 实际转入 pool 的比例（默认 0.023）
}

// withdrawTaxQuota 提现税收额度统计
type withdrawTaxQuota struct {
	TotalRecharge    decimal.Decimal // T：累计充值（已确认，终身累计）
	TotalWithdraw    decimal.Decimal // W：累计提现（不含本次，排除 rejected/failed，终身累计）
	NodeQuota        decimal.Decimal // N：节点抵扣额度总额（无节点为 0）
	TaxDeductionUsed decimal.Decimal // D：本月已使用的税款抵扣额度
}

// loadWithdrawTaxConfig 读取提现税收配置（事务/非事务）
func loadWithdrawTaxConfig(ctx context.Context, tx gdb.TX) (*withdrawTaxConfig, error) {
	cfg := &withdrawTaxConfig{
		Enabled:    true,
		Rate:       decimal.RequireFromString(defaultWithdrawTaxRateStr),
		ActualRate: decimal.RequireFromString(defaultWithdrawTaxActualRateStr),
	}
	hasEnableTax := false

	rows, err := getRecords(ctx, tx, `SELECT key, value FROM system_config WHERE key IN (?, ?, ?, ?)`,
		configKeyEnableTax, configKeyWithdrawTaxEnabled, configKeyWithdrawTaxRate, configKeyWithdrawTaxActualRate)
	if err != nil {
		return nil, gerror.Wrap(err, "读取提现税收配置失败")
	}

	for _, row := range rows {
		key := row["key"].String()
		val := strings.TrimSpace(row["value"].String())
		switch key {
		case configKeyEnableTax:
			hasEnableTax = true
			cfg.Enabled = parseBoolFlag(val, true)
		case configKeyWithdrawTaxEnabled:
			if !hasEnableTax {
				cfg.Enabled = parseBoolFlag(val, true)
			}
		case configKeyWithdrawTaxRate:
			if val != "" {
				if v, e := decimal.NewFromString(val); e == nil && v.GreaterThanOrEqual(decimal.Zero) {
					cfg.Rate = v
				}
			}
		case configKeyWithdrawTaxActualRate:
			if val != "" {
				if v, e := decimal.NewFromString(val); e == nil && v.GreaterThanOrEqual(decimal.Zero) {
					cfg.ActualRate = v
				}
			}
		}
	}

	if !cfg.Enabled {
		return cfg, nil
	}
	if cfg.Rate.LessThanOrEqual(decimal.Zero) {
		return nil, wrapRiskError(errWithdrawTaxConfigMissing, "withdraw tax rate must be greater than 0: key=%s value=%s", configKeyWithdrawTaxRate, cfg.Rate.String())
	}
	if cfg.ActualRate.LessThan(decimal.Zero) || cfg.ActualRate.GreaterThan(cfg.Rate) {
		return nil, wrapRiskError(errWithdrawTaxConfigMissing,
			"withdraw tax actual rate must be between 0 and withdraw tax rate: actual_key=%s actual=%s rate_key=%s rate=%s",
			configKeyWithdrawTaxActualRate, cfg.ActualRate.String(), configKeyWithdrawTaxRate, cfg.Rate.String())
	}

	return cfg, nil
}

// loadWithdrawTaxQuota 查询提现税收额度统计（事务/非事务）
//  - T（累计充值）和 W（累计提现）为终身累计，不按月重置
//  - W 不含本次，由调用方加上当前申请额
//  - 支持管理员通过 user_withdraw_quota 表调整额度（offset/extra_quota）
//  - D（tax_deduction_used）按月统计，每月 1 号清零
func loadWithdrawTaxQuota(ctx context.Context, tx gdb.TX, userID int64, symbol string) (*withdrawTaxQuota, error) {
	q := &withdrawTaxQuota{
		TotalRecharge:    decimal.Zero,
		TotalWithdraw:    decimal.Zero,
		NodeQuota:        decimal.Zero,
		TaxDeductionUsed: decimal.Zero,
	}

	rechargeSQL := `
		SELECT COALESCE(SUM(amount), 0)::numeric(20,8) AS total
		FROM cobo_recharge_record
		WHERE user_id = ?
		  AND symbol = ?
		  AND status = ?
	`
	rRow, err := getOneRecord(ctx, tx, rechargeSQL, userID, symbol, cobo.RechargeStatusConfirmed)
	if err != nil {
		return nil, gerror.Wrap(err, "查询累计充值失败")
	}
	if rRow != nil {
		q.TotalRecharge, _ = decimal.NewFromString(rRow["total"].String())
	}

	withdrawSQL := `
		SELECT COALESCE(SUM(amount), 0)::numeric(20,8) AS total
		FROM cobo_withdraw_request
		WHERE user_id = ?
		  AND symbol = ?
		  AND status NOT IN (?, ?)
	`
	wRow, err := getOneRecord(ctx, tx, withdrawSQL, userID, symbol, cobo.WithdrawStatusRejected, cobo.WithdrawStatusFailed)
	if err != nil {
		return nil, gerror.Wrap(err, "查询累计提现失败")
	}
	if wRow != nil {
		q.TotalWithdraw, _ = decimal.NewFromString(wRow["total"].String())
	}

	nRow, err := getOneRecord(ctx, tx, `
		SELECT
			COALESCE(SUM(CASE WHEN p.is_gift = 0 THEN p.amount * n.power_multiplier ELSE 0 END), 0)::numeric(28,8) AS real_total,
			COALESCE(SUM(CASE WHEN p.is_gift = 1 THEN p.amount * n.power_multiplier ELSE 0 END), 0)::numeric(28,8) AS gift_total,
			COALESCE(SUM(CASE WHEN p.is_gift = 1 THEN p.amount ELSE 0 END), 0)::numeric(28,8) AS gift_amount_total
		FROM cobo_node_purchase p
		JOIN node_info n ON n.node_type = p.node_type
		WHERE p.user_id = ?
	`, userID)
	if err != nil {
		return nil, gerror.Wrap(err, "查询节点免税额度失败")
	}
	if nRow != nil {
		realQuota, _ := decimal.NewFromString(nRow["real_total"].String())
		giftQuota, _ := decimal.NewFromString(nRow["gift_total"].String())
		giftAmountTotal, _ := decimal.NewFromString(nRow["gift_amount_total"].String())

		// 查询用户是否豁免赠送节点业绩限制
		nodeExempt := false
		exemptRow, exemptErr := getOneRecord(ctx, tx, `SELECT COALESCE(node_exempt, false) AS node_exempt FROM user_info WHERE id = ? LIMIT 1`, userID)
		if exemptErr == nil && exemptRow != nil {
			nodeExempt = exemptRow["node_exempt"].Bool()
		}

		giftQualified := false
		if nodeExempt {
			giftQualified = true
		} else {
			var perfErr error
			giftQualified, _, _, _, perfErr = checkGiftNodePerformanceQualified(ctx, tx, userID, giftAmountTotal)
			if perfErr != nil {
				return nil, perfErr
			}
		}

		q.NodeQuota = realQuota
		if giftQualified {
			q.NodeQuota = q.NodeQuota.Add(giftQuota)
		}
	}

	// 加载管理员额度调整及本月已抵扣额度
	month := time.Now().In(cnLocation).Format("2006-01")
	quotaDao := dao.NewUserWithdrawQuotaDao()
	quotaRecord, err := quotaDao.GetByUserIDAndMonth(ctx, userID, month)
	if err != nil {
		g.Log().Warningf(ctx, "[WithdrawTax] 查询用户额度调整记录失败: user_id=%d, month=%s, err=%v", userID, month, err)
	}
	if quotaRecord != nil {
		// 应用 withdraw_offset：抵消已提现金额
		q.TotalWithdraw = q.TotalWithdraw.Sub(quotaRecord.WithdrawOffset)
		if q.TotalWithdraw.LessThan(decimal.Zero) {
			q.TotalWithdraw = decimal.Zero
		}
		// 应用 extra_quota：增加充值额度
		q.TotalRecharge = q.TotalRecharge.Add(quotaRecord.ExtraQuota)
		// 加载本月已使用的税款抵扣额度
		q.TaxDeductionUsed = quotaRecord.TaxDeductionUsed
	}

	return q, nil
}

// checkGiftNodePerformanceQualified 判断赠送节点额度是否可用于税抵扣
// 规则：
//   - 无赠送节点：直接视为达标（不影响额度）
//   - 有赠送节点：伞下节点真实购买业绩 + 伞下用户质押业绩 > 赠送节点总额 * 10
func checkGiftNodePerformanceQualified(ctx context.Context, tx gdb.TX, userID int64, giftAmountTotal decimal.Decimal) (qualified bool, nodePerf, groupPerf, required decimal.Decimal, err error) {
	nodePerf = decimal.Zero
	groupPerf = decimal.Zero
	required = giftAmountTotal.Mul(decimal.NewFromInt(10))

	if giftAmountTotal.LessThanOrEqual(decimal.Zero) {
		return true, nodePerf, groupPerf, required, nil
	}

	perfRow, err := getOneRecord(ctx, tx, `
		SELECT
			COALESCE(cp.team_performance, 0)::numeric(28,8) AS node_performance,
			COALESCE(sv2.team_performance, 0)::numeric(28,8) AS group_performance
		FROM user_info u
		LEFT JOIN cobo_performance cp ON cp.user_id = u.id
		LEFT JOIN staking_v2_performance sv2 ON sv2.user_id = u.id
		WHERE u.id = ?
		LIMIT 1
	`, userID)
	if err != nil {
		return false, nodePerf, groupPerf, required, gerror.Wrap(err, "查询赠送节点业绩考核数据失败")
	}
	if perfRow != nil {
		nodePerf, _ = decimal.NewFromString(perfRow["node_performance"].String())
		groupPerf, _ = decimal.NewFromString(perfRow["group_performance"].String())
	}

	qualified = nodePerf.Add(groupPerf).GreaterThanOrEqual(required)
	return qualified, nodePerf, groupPerf, required, nil
}

// computeWithdrawTax 根据规则计算本次提现的税款：返回 (tax, err)
//  规则：
//   case 1: W ≤ T → tax=0
//   case 2: W > T → 对超出充值部分按税率计税，再用节点抵扣额度抵扣税款
//       excess = MIN(amt, W-T)
//       tax    = excess × rate
//       deduction = MIN(tax, N-D)  （节点额度 N 减去本月已抵扣 D）
//       actualTax = tax - deduction
func computeWithdrawTax(amount decimal.Decimal, q *withdrawTaxQuota, rate decimal.Decimal) (decimal.Decimal, error) {
	totalWithdraw := q.TotalWithdraw.Add(amount)

	// case 1: 累计提现不超过累计充值，不收税
	if totalWithdraw.LessThanOrEqual(q.TotalRecharge) {
		return decimal.Zero, nil
	}

	// case 2: 本次提现中超出充值的部分需要计税
	excess := totalWithdraw.Sub(q.TotalRecharge)
	if excess.GreaterThan(amount) {
		excess = amount
	}

	tax := excess.Mul(rate).Round(8)
	if tax.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, nil
	}

	// 用节点抵扣额度抵扣税款
	remainingQuota := q.NodeQuota.Sub(q.TaxDeductionUsed)
	if remainingQuota.GreaterThanOrEqual(tax) {
		// 抵扣额度充足，实际缴税为 0
		return decimal.Zero, nil
	}
	if remainingQuota.GreaterThan(decimal.Zero) {
		// 部分抵扣
		return tax.Sub(remainingQuota), nil
	}
	// 抵扣额度已用完，全额缴税
	return tax, nil
}

// computeWithdrawTaxDetail 计算税款及本次实际抵扣额度
// 返回 (actualTax, deductionUsed, err)
func computeWithdrawTaxDetail(amount decimal.Decimal, q *withdrawTaxQuota, rate decimal.Decimal) (decimal.Decimal, decimal.Decimal, error) {
	tax, err := computeWithdrawTax(amount, q, rate)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}

	totalWithdraw := q.TotalWithdraw.Add(amount)
	if totalWithdraw.LessThanOrEqual(q.TotalRecharge) {
		return decimal.Zero, decimal.Zero, nil
	}

	excess := totalWithdraw.Sub(q.TotalRecharge)
	if excess.GreaterThan(amount) {
		excess = amount
	}

	taxBeforeDeduction := excess.Mul(rate).Round(8)
	if taxBeforeDeduction.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero, nil
	}

	deductionUsed := taxBeforeDeduction.Sub(tax)
	if deductionUsed.LessThan(decimal.Zero) {
		deductionUsed = decimal.Zero
	}
	remainingQuota := q.NodeQuota.Sub(q.TaxDeductionUsed)
	if deductionUsed.GreaterThan(remainingQuota) {
		deductionUsed = remainingQuota
	}
	if deductionUsed.LessThan(decimal.Zero) {
		deductionUsed = decimal.Zero
	}

	return tax, deductionUsed, nil
}

// parseBoolFlag 将系统配置字符串解析为布尔值
func parseBoolFlag(raw string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

// getOneRecord 查询单行记录（事务/非事务）
func getOneRecord(ctx context.Context, tx gdb.TX, sql string, args ...interface{}) (gdb.Record, error) {
	if tx != nil {
		return tx.GetOne(sql, args...)
	}
	return g.DB().GetOne(ctx, sql, args...)
}

// getRecords 查询多行记录（事务/非事务）
func getRecords(ctx context.Context, tx gdb.TX, sql string, args ...interface{}) (gdb.Result, error) {
	if tx != nil {
		return tx.GetAll(sql, args...)
	}
	return g.DB().GetAll(ctx, sql, args...)
}

// getYYAIPrice 查询 YYAI 最新价格（USD）
func getYYAIPrice(ctx context.Context, tx gdb.TX) (decimal.Decimal, error) {
	row, err := getOneRecord(ctx, tx, `SELECT price FROM stock_price WHERE symbol = 'YYAI' ORDER BY price_time DESC LIMIT 1`)
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "查询 YYAI 价格失败")
	}
	if row == nil || row["price"].String() == "" {
		return decimal.Zero, gerror.New("YYAI 价格不存在")
	}
	price, err := decimal.NewFromString(row["price"].String())
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "解析 YYAI 价格失败")
	}
	if price.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, gerror.New("YYAI 价格无效")
	}
	return price, nil
}

// splitWithdrawTax 将总税款拆分为实际税收和 YYAI 兑换两部分
//  规则：按 ActualRate / Rate 比例拆分
//   actualTax = totalTax * (ActualRate / Rate)
//   yyaiTax   = totalTax - actualTax
func splitWithdrawTax(totalTax decimal.Decimal, cfg *withdrawTaxConfig) (actualTax, yyaiTax decimal.Decimal) {
	if totalTax.LessThanOrEqual(decimal.Zero) || cfg.Rate.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero
	}
	actualTax = totalTax.Mul(cfg.ActualRate).Div(cfg.Rate).Round(8)
	if actualTax.GreaterThan(totalTax) {
		actualTax = totalTax
	}
	yyaiTax = totalTax.Sub(actualTax).Round(8)
	if yyaiTax.LessThan(decimal.Zero) {
		yyaiTax = decimal.Zero
	}
	return actualTax, yyaiTax
}

func getUserIDByWalletAddress(ctx context.Context, tx gdb.TX, address string) (int64, error) {
	addr := strings.ToLower(strings.TrimSpace(address))
	if addr == "" {
		return 0, nil
	}
	row, err := getOneRecord(ctx, tx, `SELECT id FROM user_info WHERE LOWER(wallet_address) = ? LIMIT 1`, addr)
	if err != nil {
		return 0, err
	}
	if row == nil {
		return 0, nil
	}
	return row["id"].Int64(), nil
}
