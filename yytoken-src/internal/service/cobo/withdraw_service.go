package cobo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"XWFrame/internal/blockchain"
	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/entity/cobo"
	"XWFrame/internal/frame/config"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	repo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/service/cobo/model"
	externalCobo "XWFrame/pkg/external/cobo"
	"XWFrame/pkg/utils"

	coboWaas2 "github.com/CoboGlobal/cobo-waas2-go-sdk/cobo_waas2"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// WithdrawService 提现服务接口
type WithdrawService interface {
	// CreateWithdraw 创建提现申请
	CreateWithdraw(ctx context.Context, req *model.CreateWithdrawReq) (*model.CreateWithdrawRes, error)

	// GetWithdrawConfig 获取提现配置
	GetWithdrawConfig(ctx context.Context) (*model.GetWithdrawConfigRes, error)

	// GetWithdrawList 获取用户提现列表
	GetWithdrawList(ctx context.Context, req *model.GetWithdrawListReq) (*model.GetWithdrawListRes, error)

	// AuditWithdraw 审核提现（后台）
	AuditWithdraw(ctx context.Context, req *model.AuditWithdrawReq) error

	// GetPendingList 获取待审核列表（后台）
	GetPendingList(ctx context.Context, page, pageSize int) (*model.GetWithdrawListRes, error)

	// ProcessWithdraw 处理提现（调用Cobo API）
	ProcessWithdraw(ctx context.Context, id int64) error

	// HandleHotwalletWithdrawCallback 处理 hotwallet 通道提现终态回调
	HandleHotwalletWithdrawCallback(ctx context.Context, req *model.SZPNWithdrawCallbackReq) (*model.SZPNWithdrawCallbackRes, error)

	// DeductWithdrawTaxOnSuccess 提现成功时扣税（pool + YYAI），返回扣税后余额快照（用于后续扣冻结）
	DeductWithdrawTaxOnSuccess(ctx context.Context, tx gdb.TX, withdraw *cobo.WithdrawEntity, balanceBefore decimal.Decimal) (decimal.Decimal, error)
}

// withdrawService 提现服务实现
type withdrawService struct {
	withdrawRepo         repo.IWithdrawRepository
	balanceRepo          repo.IBalanceRepository
	userRepo             repository.IUserRepository
	passwordRepo         repository.IUserPasswordRepository
	withdrawWhiteListRepo repository.IWithdrawWhiteListRepository
	riskControl          RiskControlService
	swapRecordDao        dao.ISwapRecordDao
}

const withdrawUserLockTTLSeconds = 10

// isHotwalletWithdrawSymbol 判断该币种是否走 hotwallet/monitor 提现路径
// 规则：USDT/BOX/JU 走 Cobo，YY/YYAI/TRIPLE 禁止提现（创建时拦截），其余按 token_config.detect_by_hot_wallet 判定
func (s *withdrawService) isHotwalletWithdrawSymbol(ctx context.Context, symbol string) bool {
	symbol = normalizeWithdrawSymbol(symbol)
	switch symbol {
	case "USDT", "BOX", "JU", "YY", "YYAI", "TRIPLE":
		return false
	}

	var token entity.TokenConfigEntity
	err := g.DB().Model("token_config").Ctx(ctx).
		Fields("contract_address", "detect_by_hot_wallet").
		Where("symbol = ?", symbol).
		Scan(&token)
	if err != nil {
		g.Log().Warningf(ctx, "[Cobo] query token_config failed in withdraw routing: symbol=%s err=%v", symbol, err)
		return false
	}
	return token.DetectByHotWallet && strings.TrimSpace(token.ContractAddress) != ""
}

// NewWithdrawService 创建提现服务
func NewWithdrawService() WithdrawService {
	return &withdrawService{
		withdrawRepo:         repo.NewWithdrawRepository(),
		balanceRepo:          repo.NewBalanceRepository(),
		userRepo:             repository.NewUserRepository(),
		passwordRepo:         repository.NewUserPasswordRepository(),
		withdrawWhiteListRepo: repository.NewWithdrawWhiteListRepository(),
		riskControl:          NewRiskControlService(),
		swapRecordDao:        dao.NewSwapRecordDao(),
	}
}

// CreateWithdraw 创建提现申请
func (s *withdrawService) CreateWithdraw(ctx context.Context, req *model.CreateWithdrawReq) (*model.CreateWithdrawRes, error) {
	locale := consts.LocaleFromCtx(ctx)

	// 验证密码（如果用户已设置密码则必填）
	if err := repository.VerifyUserPassword(ctx, s.passwordRepo, req.UserID, req.Password); err != nil {
		return nil, err
	}

	// 1. 参数校验
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return nil, withdrawErr(locale, "amount_must_positive")
	}
	req.Symbol = normalizeWithdrawSymbol(req.Symbol)

	// 查询 token_config 验证币种是否支持提现
	var tokenCfg entity.TokenConfigEntity
	if err := g.DB().Model("token_config").Ctx(ctx).
		Fields("is_enabled").
		Where("symbol = ?", req.Symbol).
		Scan(&tokenCfg); err != nil {
		return nil, gerror.Wrapf(err, "query token config failed: %s", req.Symbol)
	}
	if !tokenCfg.IsEnabled {
		return nil, withdrawErr(locale, "unsupported_symbol")
	}

	// 明确禁止提现的币种
	switch req.Symbol {
	case "YY", "YYAI", "TRIPLE":
		return nil, withdrawErrf(locale, withdrawErrKey("symbol_withdraw_disabled"), req.Symbol)
	}

	// 测试服务器禁止提现
	if utils.IsTestServer() {
		return nil, withdrawErr(locale, "test_server_withdraw_disabled")
	}

	locked, lockErr := s.acquireUserWithdrawLock(ctx, req.UserID)
	if lockErr != nil {
		return nil, gerror.Wrap(lockErr, "acquire withdraw lock failed")
	}
	if !locked {
		return nil, withdrawErr(locale, "withdraw_rate_limited")
	}
	defer s.releaseUserWithdrawLock(ctx, req.UserID)

	enabled, err := s.isWithdrawEnabled(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[Cobo] read withdraw switch failed, use default enabled: err=%v", err)
	}
	if !enabled {
		return nil, withdrawErr(locale, "withdraw_disabled")
	}

	// 2. 获取配置
	cfg, err := config.GetCoboConfig(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "get cobo config failed")
	}

	feeRate := decimal.NewFromFloat(cfg.Withdraw.FeeRate)
	// hotwallet/monitor 通道提现不收取平台手续费。
	if s.isHotwalletWithdrawSymbol(ctx, req.Symbol) {
		feeRate = decimal.Zero
	}
	feeAmount := req.Amount.Mul(feeRate)
	actualAmount := req.Amount.Sub(feeAmount)
	g.Log().Infof(ctx, "[Cobo] withdraw config snapshot: user_id=%d, amount=%s, fee_rate=%s, auto_approve_threshold=%v, min_amount=%v, max_amount=%v",
		req.UserID, req.Amount.String(), feeRate.String(), cfg.Withdraw.AutoApproveThreshold, cfg.Withdraw.RiskRules.MinAmount, cfg.Withdraw.RiskRules.MaxAmount)
	if actualAmount.LessThanOrEqual(decimal.Zero) {
		return nil, withdrawErr(locale, "amount_too_small_after_fee")
	}

	// 2.1 计算提现目标地址（优先使用请求中的 receive_address）
	toAddress := strings.ToLower(strings.TrimSpace(req.ReceiveAddress))
	user, err := s.userRepo.GetUserById(ctx, req.UserID)
	if err != nil {
		return nil, gerror.Wrap(err, "get user info failed")
	}
	if user == nil {
		return nil, withdrawErr(locale, "user_not_found")
	}
	if !user.CanWithdraw {
		return nil, withdrawErr(locale, "withdraw_disabled_for_user")
	}
	userWalletAddress := strings.ToLower(strings.TrimSpace(user.WalletAddress))
	if userWalletAddress == "" {
		return nil, withdrawErr(locale, "wallet_address_empty")
	}
	if toAddress == "" {
		toAddress = userWalletAddress
	}
	if toAddress == "" {
		return nil, withdrawErr(locale, "wallet_address_empty")
	}
	if !utils.IsValidEthereumAddress(toAddress) {
		return nil, withdrawErr(locale, "invalid_receive_address")
	}
	if toAddress != userWalletAddress {
		// 检查用户是否在白名单中
		whiteListRecord, wlErr := s.withdrawWhiteListRepo.GetByUserID(ctx, req.UserID)
		if wlErr != nil {
			g.Log().Warningf(ctx, "[Cobo] query withdraw white list failed: user_id=%d, err=%v", req.UserID, wlErr)
		}
		if whiteListRecord == nil {
			g.Log().Warningf(ctx, "[Cobo] withdraw address mismatch detected but no longer auto-disabling: user_id=%d, wallet=%s, to=%s", req.UserID, userWalletAddress, toAddress)
			alertCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			GetTelegramNotifyService(alertCtx).NotifyWithdrawAddressMismatch(alertCtx, req.UserID, req.Symbol, userWalletAddress, toAddress)
			return nil, withdrawErr(locale, "receive_address_not_match_wallet")
		}
		g.Log().Infof(ctx, "[Cobo] withdraw address mismatch allowed by white list: user_id=%d, wallet=%s, to=%s", req.UserID, userWalletAddress, toAddress)
	}

	// 3. 余额预检（避免余额不足的用户触发无意义的风控拦截和通知）
	balance, err := s.balanceRepo.GetByUserID(ctx, req.UserID, req.Symbol)
	if err != nil {
		return nil, gerror.Wrap(err, "get balance failed")
	}
	if balance == nil || balance.AvailableAmount.LessThan(req.Amount) {
		available := decimal.Zero
		if balance != nil {
			available = balance.AvailableAmount
		}
		if req.Amount.Sub(available).Abs().GreaterThanOrEqual(decimal.NewFromFloat(0.1)) {
			GetTelegramNotifyService(ctx).NotifyWithdrawBalanceInsufficientAsync(req.UserID, req.Amount, req.Symbol, toAddress, available)
		}
		return nil, withdrawErr(locale, "balance_insufficient")
	}

	// 4. 风控检查（预检，快速失败）
	if err := s.riskCheck(ctx, req, cfg); err != nil {
		GetTelegramNotifyService(ctx).NotifyWithdrawRejectedAsync(req.UserID, req.Amount, req.Symbol, toAddress, err.Error())
		return nil, err
	}

	autoApproved := req.Symbol != "USDT" || s.shouldAutoApprove(req.Amount, cfg)
	g.Log().Infof(ctx, "[Cobo] withdraw audit decision: user_id=%d, symbol=%s, amount=%s, auto_approved=%t", req.UserID, req.Symbol, req.Amount.String(), autoApproved)

	fakeTestWithdraw := user.IsTest == 1 && readFakeTestWithdrawEnabled(ctx)

	// 5. 创建提现申请（事务）
	orderNo := fmt.Sprintf("WD%s", utils.GenerateSnowflakeId())
	var withdraw *cobo.WithdrawEntity
	var totalTax = decimal.Zero
	var taxAmount = decimal.Zero
	var taxDeductionAmount = decimal.Zero
	var taxYyaiUsdtAmount = decimal.Zero
	var taxYyaiAmount = decimal.Zero
	var recordFeeAmount = feeAmount
	var recordActualAmount = actualAmount

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 5.1 加锁查询余额并检查
		balance, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, req.Symbol)
		if err != nil {
			return gerror.Wrap(err, "get balance failed")
		}
		if balance == nil || balance.AvailableAmount.LessThan(req.Amount) {
			return errWithdrawBalanceInsufficient
		}

		// 5.2 事务内风控二次检查（防止并发穿透）
		if err := s.riskControl.CheckWithdrawTx(ctx, tx, req.UserID, req.Amount, req.Symbol); err != nil {
			return err
		}

		// 5.3 冻结余额
		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, req.UserID, req.Symbol, req.Amount.Neg(), req.Amount); err != nil {
			return gerror.Wrap(err, "freeze balance failed")
		}
		balanceBefore := balance.AvailableAmount.Add(balance.FrozenAmount)
		if err := writeBalanceChangeLogTx(ctx, tx, req.UserID, req.Symbol, consts.ChangeTypeWithdrawFreeze, req.Amount.Neg(), balanceBefore, balanceBefore, orderNo, 0, "withdraw create freeze", consts.OperatorTypeSystem); err != nil {
			return gerror.Wrap(err, "write withdraw freeze audit log failed")
		}

		// 5.3.1 计算手续费/税款（仅 USDT；按当月累计充值/提现/节点免税额度）
		taxAmount = decimal.Zero
		taxDeductionAmount = decimal.Zero
		taxYyaiUsdtAmount = decimal.Zero
		taxYyaiAmount = decimal.Zero
		totalTax = decimal.Zero
		recordFeeAmount = feeAmount
		if req.Symbol == "USDT" {
			quota, err := loadWithdrawTaxQuota(ctx, tx, req.UserID, req.Symbol)
			if err != nil {
				return err
			}

			// USDT 提现规则：手续费不单独收取，仅按提现税规则扣减（超充值额度部分按税率）
			recordFeeAmount = decimal.Zero

			taxCfg, err := loadWithdrawTaxConfig(ctx, tx)
			if err != nil {
				return err
			}
			if taxCfg.Enabled {
				tax, deductionUsed, err := computeWithdrawTaxDetail(req.Amount, quota, taxCfg.Rate)
				if err != nil {
					return err
				}
				totalWithdrawAfter := quota.TotalWithdraw.Add(req.Amount)
				excess := totalWithdrawAfter.Sub(quota.TotalRecharge)
				if excess.LessThan(decimal.Zero) {
					excess = decimal.Zero
				}
				if excess.GreaterThan(req.Amount) {
					excess = req.Amount
				}
				totalTax = tax
				actualTax, yyaiTax := splitWithdrawTax(totalTax, taxCfg)
				if actualTax.GreaterThan(decimal.Zero) {
					// 税款在提现成功时才沉淀，创建时仅记录
					taxAmount = actualTax
				}
				if yyaiTax.GreaterThan(decimal.Zero) {
					// 税款在提现成功时才兑换 YYAI，创建时仅记录
					taxYyaiUsdtAmount = yyaiTax
				}
				if deductionUsed.GreaterThan(decimal.Zero) {
					taxDeductionAmount = deductionUsed
					month := time.Now().In(cnLocation).Format("2006-01")
					_, uErr := tx.Exec(`
						INSERT INTO user_withdraw_quota (user_id, month, withdraw_offset, extra_quota, tax_deduction_used, remark, created_at, updated_at)
						VALUES (?, ?, 0, 0, ?, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
						ON CONFLICT (user_id, month)
						DO UPDATE SET
							tax_deduction_used = user_withdraw_quota.tax_deduction_used + EXCLUDED.tax_deduction_used,
							updated_at = CURRENT_TIMESTAMP
					`, req.UserID, month, deductionUsed)
					if uErr != nil {
						return gerror.Wrapf(uErr, "update tax deduction used failed: user_id=%d, month=%s", req.UserID, month)
					}
				}
			}
		}
		if s.isHotwalletWithdrawSymbol(ctx, req.Symbol) {
			// hotwallet/monitor 通道不收平台手续费。
			recordFeeAmount = decimal.Zero
		}

		// 5.3.2 实际到账金额扣除手续费和税款
		recordActualAmount = req.Amount.Sub(recordFeeAmount).Sub(taxAmount).Sub(taxYyaiUsdtAmount)
		if recordActualAmount.LessThanOrEqual(decimal.Zero) {
			return withdrawErr(locale, "amount_after_fee_tax_too_small")
		}

		// 5.4 创建提现记录
		now := time.Now()
		status := cobo.WithdrawStatusPending
		var auditAt *time.Time
		auditRemark := ""
		if autoApproved {
			status = cobo.WithdrawStatusProcessing
			auditAt = &now
			if req.Symbol != "USDT" {
				auditRemark = "auto approved (non-USDT)"
			} else if decimal.NewFromFloat(cfg.Withdraw.RiskRules.MaxAmount).GreaterThan(decimal.Zero) && req.Amount.GreaterThan(decimal.NewFromFloat(cfg.Withdraw.RiskRules.MaxAmount)) {
				auditRemark = "auto approved above max amount"
			} else {
				auditRemark = "auto approved by threshold"
			}
		}

		withdraw = &cobo.WithdrawEntity{
			UserID:             req.UserID,
			OrderNo:            orderNo,
			Symbol:             req.Symbol,
			Amount:             req.Amount,
			FeeRate:            feeRate,
			FeeAmount:          recordFeeAmount,
			ActualAmount:       recordActualAmount,
			TaxAmount:          taxAmount,
			TaxDeductionAmount: taxDeductionAmount,
			TaxYyaiUsdtAmount:  taxYyaiUsdtAmount,
			TaxYyaiAmount:      taxYyaiAmount,
			TaxRecipient:       "",
			ToAddress:          toAddress,
			Chain:              req.Chain,
			Status:             status,
			AutoApproved:       autoApproved,
			AuditAt:            auditAt,
			AuditRemark:        auditRemark,
			CreatedAt:          now,
		}

		if err := s.withdrawRepo.Create(ctx, tx, withdraw); err != nil {
			return gerror.Wrap(err, "create withdraw record failed")
		}

		// 测试用户 + system_config.fake_test_withdraw=true：直接标记为成功，跳过实际 Cobo 提现
		if fakeTestWithdraw {
			if err := s.withdrawRepo.UpdateStatusTx(ctx, tx, withdraw.ID, cobo.WithdrawStatusSuccess); err != nil {
				return gerror.Wrap(err, "test mode: update withdraw status failed")
			}
			withdraw.Status = cobo.WithdrawStatusSuccess

			balanceRow, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, req.Symbol)
			if err != nil {
				return gerror.Wrap(err, "test mode: lock balance failed")
			}
			beforeTotal := decimal.Zero
			if balanceRow != nil {
				beforeTotal = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
			}
			// 创建事务已把税款从冻结划走，剩余冻结仅为 Amount - TaxAmount - TaxYyaiUsdtAmount
			frozenDeduct := req.Amount.Sub(taxAmount).Sub(taxYyaiUsdtAmount)
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, req.UserID, req.Symbol, decimal.Zero, frozenDeduct.Neg()); err != nil {
				return gerror.Wrap(err, "test mode: deduct frozen balance failed")
			}
			afterTotal := beforeTotal.Sub(frozenDeduct)
			if err := writeBalanceChangeLogTx(ctx, tx, req.UserID, req.Symbol, consts.ChangeTypeWithdrawSuccess, frozenDeduct.Neg(), beforeTotal, afterTotal, orderNo, withdraw.ID, "test mode withdraw success", consts.OperatorTypeSystem); err != nil {
				return gerror.Wrap(err, "test mode: write balance change log failed")
			}
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, errWithdrawBalanceInsufficient) ||
			errors.Is(err, errWithdrawRisk24hFrequency) ||
			errors.Is(err, errWithdrawRiskTotalLimit) ||
			errors.Is(err, errWithdrawRiskNewUserLimit) ||
			errors.Is(err, errWithdrawRiskGiftNodeUnqualified) {
			GetTelegramNotifyService(ctx).NotifyWithdrawRejectedAsync(req.UserID, req.Amount, req.Symbol, toAddress, err.Error())
		}
		return nil, err
	}

	go func(w *cobo.WithdrawEntity) {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		GetTelegramNotifyService(notifyCtx).NotifyWithdrawRequest(notifyCtx, w)
	}(withdraw)
	if autoApproved && s.isLargeUSDTWithdraw(req.Symbol, req.Amount, cfg) {
		go func(w *cobo.WithdrawEntity) {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			GetTelegramNotifyService(notifyCtx).NotifyLargeWithdrawAutoApproved(notifyCtx, w)
		}(withdraw)
	}

	// 6. 自动审核通过时异步触发处理（测试模式已在事务内直接成功，无需后续处理）
	if autoApproved && !fakeTestWithdraw {
		go func() {
			if err := s.ProcessWithdraw(context.Background(), withdraw.ID); err != nil {
				g.Log().Warningf(context.Background(), "[Cobo] auto-process withdraw failed: %v", err)
			}
		}()
	}

	return &model.CreateWithdrawRes{
		OrderNo:            orderNo,
		Status:             withdraw.Status,
		Message:            consts.LocalizedText(withdrawErrMsgs["withdraw_submitted"], locale),
		Amount:             req.Amount.String(),
		FeeAmount:          recordFeeAmount.String(),
		TaxAmount:          taxAmount.String(),
		TaxDeductionAmount: taxDeductionAmount.String(),
		TaxYyaiUsdtAmount:  taxYyaiUsdtAmount.String(),
		TaxYyaiAmount:      taxYyaiAmount.String(),
		ActualAmount:       recordActualAmount.String(),
	}, nil
}

func (s *withdrawService) isWithdrawEnabled(ctx context.Context) (bool, error) {
	return readWithdrawEnabledConfig(ctx, "system_config")
}

func readWithdrawEnabledConfig(ctx context.Context, table string) (bool, error) {
	value, err := g.DB().Model(table).Ctx(ctx).
		Fields("value").
		Where("key = ?", "enable_withdraw").
		Value()
	if err != nil {
		return true, err
	}
	if value == nil || value.IsNil() {
		return true, nil
	}

	raw := strings.ToLower(strings.TrimSpace(value.String()))
	if raw == "" {
		return true, nil
	}

	switch raw {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		g.Log().Warningf(ctx, "[Cobo] invalid withdraw switch value, use default enabled: key=enable_withdraw value=%s", raw)
		return true, nil
	}
}

// readFakeTestWithdrawEnabled 读取 system_config.fake_test_withdraw，默认 false
func readFakeTestWithdrawEnabled(ctx context.Context) bool {
	value, err := g.DB().Model("system_config").Ctx(ctx).
		Fields("value").
		Where("key = ?", "fake_test_withdraw").
		Value()
	if err != nil {
		g.Log().Warningf(ctx, "[Cobo] read fake_test_withdraw config failed, default disabled: err=%v", err)
		return false
	}
	if value == nil || value.IsNil() {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(value.String())) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// GetWithdrawConfig 获取提现配置
func (s *withdrawService) GetWithdrawConfig(ctx context.Context) (*model.GetWithdrawConfigRes, error) {
	configMap, err := s.getSymbolWithdrawConfigMap(ctx)
	if err != nil {
		return nil, err
	}
	res := model.GetWithdrawConfigRes(configMap)
	return &res, nil
}

// GetWithdrawList 获取用户提现列表
func (s *withdrawService) GetWithdrawList(ctx context.Context, req *model.GetWithdrawListReq) (*model.GetWithdrawListRes, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	entities, total, err := s.withdrawRepo.GetByUserID(ctx, req.UserID, req.Symbol, page, pageSize)
	if err != nil {
		return nil, gerror.Wrap(err, "get withdraw list failed")
	}

	list := make([]*model.WithdrawItem, 0, len(entities))
	for _, e := range entities {
		if e == nil {
			continue
		}
		list = append(list, &model.WithdrawItem{
			OrderNo:            e.OrderNo,
			Symbol:             e.Symbol,
			Amount:             e.Amount.String(),
			FeeAmount:          e.FeeAmount.String(),
			TaxAmount:          e.TaxAmount.String(),
			TaxDeductionAmount: e.TaxDeductionAmount.String(),
			ActualAmount:       e.ActualAmount.String(),
			ToAddress:          e.ToAddress,
			Status:             e.Status,
			StatusText:         model.WithdrawStatusText(e.Status),
			TxHash:             e.TxHash,
			CreatedAt:          utils.DBTimestampToUnix(e.CreatedAt),
		})
	}

	pages := total / int64(pageSize)
	if total%int64(pageSize) > 0 {
		pages++
	}

	return &model.GetWithdrawListRes{
		Page:     page,
		PageSize: pageSize,
		Total:    int(total),
		Pages:    int(pages),
		List:     list,
	}, nil
}

// AuditWithdraw 审核提现
func (s *withdrawService) AuditWithdraw(ctx context.Context, req *model.AuditWithdrawReq) error {
	locale := consts.LocaleFromCtx(ctx)
	if req.Status != cobo.WithdrawStatusApproved && req.Status != cobo.WithdrawStatusRejected {
		return withdrawErr(locale, "audit_status_invalid")
	}

	withdraw, err := s.withdrawRepo.GetByID(ctx, req.ID)
	if err != nil {
		return gerror.Wrap(err, "query withdraw record failed")
	}
	if withdraw == nil {
		return withdrawErr(locale, "withdraw_record_not_found")
	}
	if withdraw.Status != cobo.WithdrawStatusPending {
		return withdrawErr(locale, "only_pending_can_audit")
	}
	oldStatus := withdraw.Status

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		updated, err := s.withdrawRepo.AuditTx(ctx, tx, req.ID, req.Status, req.AdminID, req.Remark)
		if err != nil {
			return gerror.Wrap(err, "audit withdraw failed")
		}
		if !updated {
			return withdrawErr(locale, "only_pending_can_audit")
		}

		// 审核通过时，先更新为 processing 状态，防止并发重复处理
		if req.Status == cobo.WithdrawStatusApproved {
			if err := s.withdrawRepo.UpdateStatusTx(ctx, tx, req.ID, cobo.WithdrawStatusProcessing); err != nil {
				return gerror.Wrap(err, "update processing status failed")
			}
		}

		if req.Status == cobo.WithdrawStatusRejected {
			balanceBefore := decimal.Zero
			balanceRow, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, withdraw.UserID, withdraw.Symbol)
			if err != nil {
				return gerror.Wrap(err, "query balance failed")
			}
			if balanceRow != nil {
				balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
			}
			// 税款在成功时才扣减，拒绝时全部解冻
			refundAmount := withdraw.Amount
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, withdraw.UserID, withdraw.Symbol, refundAmount, refundAmount.Neg()); err != nil {
				return gerror.Wrap(err, "unfreeze balance failed")
			}
			if err := writeBalanceChangeLogTx(ctx, tx, withdraw.UserID, withdraw.Symbol, consts.ChangeTypeWithdrawFailRefund, refundAmount, balanceBefore, balanceBefore, withdraw.OrderNo, withdraw.ID, "withdraw rejected refund", consts.OperatorTypeSystem); err != nil {
				return gerror.Wrap(err, "write withdraw rejected refund audit log failed")
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	newStatus := req.Status
	if req.Status == cobo.WithdrawStatusApproved {
		newStatus = cobo.WithdrawStatusProcessing
	}
	withdraw.Status = newStatus
	withdraw.AuditRemark = req.Remark
	GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, withdraw, oldStatus, req.Remark)

	// 如果审核通过，异步处理提现
	if req.Status == cobo.WithdrawStatusApproved {
		go func() {
			if err := s.ProcessWithdraw(context.Background(), req.ID); err != nil {
				g.Log().Errorf(context.Background(), "[Cobo] process withdraw failed: id=%d, err=%v", req.ID, err)
			}
		}()
	}

	return nil
}

// GetPendingList 获取待审核列表
func (s *withdrawService) GetPendingList(ctx context.Context, page, pageSize int) (*model.GetWithdrawListRes, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	entities, total, err := s.withdrawRepo.GetPendingList(ctx, page, pageSize)
	if err != nil {
		return nil, gerror.Wrap(err, "get pending list failed")
	}

	list := make([]*model.WithdrawItem, 0, len(entities))
	for _, e := range entities {
		if e == nil {
			continue
		}
		list = append(list, &model.WithdrawItem{
			OrderNo:            e.OrderNo,
			Symbol:             e.Symbol,
			Amount:             e.Amount.String(),
			FeeAmount:          e.FeeAmount.String(),
			TaxAmount:          e.TaxAmount.String(),
			TaxDeductionAmount: e.TaxDeductionAmount.String(),
			TaxYyaiUsdtAmount:  e.TaxYyaiUsdtAmount.String(),
			TaxYyaiAmount:      e.TaxYyaiAmount.String(),
			ActualAmount:       e.ActualAmount.String(),
			ToAddress:          e.ToAddress,
			Status:             e.Status,
			StatusText:         model.WithdrawStatusText(e.Status),
			TxHash:             e.TxHash,
			CreatedAt:          utils.DBTimestampToUnix(e.CreatedAt),
		})
	}

	pages := total / int64(pageSize)
	if total%int64(pageSize) > 0 {
		pages++
	}

	return &model.GetWithdrawListRes{
		Page:     page,
		PageSize: pageSize,
		Total:    int(total),
		Pages:    int(pages),
		List:     list,
	}, nil
}

// ProcessWithdraw 处理提现（调用Cobo API）
func (s *withdrawService) ProcessWithdraw(ctx context.Context, id int64) error {
	locale := consts.LocaleFromCtx(ctx)
	withdraw, err := s.withdrawRepo.GetByID(ctx, id)
	if err != nil {
		return gerror.Wrap(err, "query withdraw record failed")
	}
	if withdraw == nil {
		return withdrawErr(locale, "withdraw_record_not_found")
	}

	if withdraw.Status == cobo.WithdrawStatusSuccess || withdraw.Status == cobo.WithdrawStatusFailed || withdraw.Status == cobo.WithdrawStatusRejected {
		return nil
	}

	if withdraw.Status != cobo.WithdrawStatusApproved && withdraw.Status != cobo.WithdrawStatusProcessing {
		return withdrawErr(locale, "process_status_invalid")
	}
	currentStatus := withdraw.Status

	// 更新为处理中状态
	if withdraw.Status == cobo.WithdrawStatusApproved {
		if err := s.withdrawRepo.UpdateStatus(ctx, id, cobo.WithdrawStatusProcessing); err != nil {
			return gerror.Wrap(err, "update withdraw status failed")
		}
		withdraw.Status = cobo.WithdrawStatusProcessing
		GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, withdraw, currentStatus, "")
		currentStatus = cobo.WithdrawStatusProcessing
	}

	failAndUnfreeze := func(requestID string, stage string, cause error) error {
		reason := ""
		if cause != nil {
			reason = cause.Error()
		}
		if len(reason) > 500 {
			reason = reason[:500]
		}

		failRespData, _ := json.Marshal(map[string]string{
			"stage": stage,
			"error": reason,
		})
		if updateErr := s.withdrawRepo.UpdateCoboInfo(ctx, id, requestID, string(failRespData), ""); updateErr != nil {
			g.Log().Errorf(ctx, "[Cobo] update fail status failed: id=%d, stage=%s, err=%v", id, stage, updateErr)
		}

		if balanceErr := s.unfreezeBalanceWithLog(ctx, withdraw.UserID, withdraw.Symbol, withdraw.Amount, withdraw.OrderNo, withdraw.ID, "withdraw failed refund"); balanceErr != nil {
			g.Log().Errorf(ctx, "[Cobo] unfreeze balance failed: id=%d, user_id=%d, stage=%s, err=%v", id, withdraw.UserID, stage, balanceErr)
		}

		if statusErr := s.withdrawRepo.UpdateStatus(ctx, id, cobo.WithdrawStatusFailed); statusErr != nil {
			g.Log().Errorf(ctx, "[Cobo] update withdraw status to failed failed: id=%d, stage=%s, err=%v", id, stage, statusErr)
		} else {
			failedWithdraw := *withdraw
			failedWithdraw.Status = cobo.WithdrawStatusFailed
			GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, &failedWithdraw, currentStatus, stage)
		}

		g.Log().Errorf(ctx, "[Cobo] pre-submit failed, marked failed and unfrozen: id=%d, stage=%s, err=%v", id, stage, cause)
		if cause != nil {
			return gerror.Wrapf(cause, "pre-submit failed(%s)", stage)
		}
		return gerror.Newf("pre-submit failed(%s)", stage)
	}

	// YY/YYAI 提现走 TokenStoreBurn 合约
	if withdraw.Symbol == "YY" || withdraw.Symbol == "YYAI" {
		return s.processContractWithdraw(ctx, withdraw, currentStatus)
	}

	// hotwallet/monitor 通道提现不走 Cobo，由外部 monitor 执行。
	// 这里保留提现单为 processing，等待后续通道处理与终态回写。
	if s.isHotwalletWithdrawSymbol(ctx, withdraw.Symbol) {
		requestID := fmt.Sprintf("HW-%s-%d-%d", strings.ToUpper(withdraw.Symbol), id, time.Now().Unix())
		if err := s.submitHotwalletWithdraw(ctx, withdraw, requestID); err != nil {
			return failAndUnfreeze(requestID, "hotwallet_submit", err)
		}

		pendingRespData, _ := json.Marshal(map[string]string{
			"stage":      "hotwallet_external_pending",
			"status":     "waiting_external_processor",
			"request_id": requestID,
		})
		if err := s.withdrawRepo.UpdateCoboInfo(ctx, id, requestID, string(pendingRespData), ""); err != nil {
			g.Log().Warningf(ctx, "[Cobo] update hotwallet pending info failed: id=%d, err=%v", id, err)
		}
		g.Log().Infof(ctx, "[Cobo] hotwallet withdraw queued for external processor: id=%d, order_no=%s, symbol=%s", id, withdraw.OrderNo, withdraw.Symbol)
		return nil
	}

	// 获取Cobo配置
	cfg, err := config.GetCoboConfig(ctx)
	if err != nil {
		return failAndUnfreeze(fmt.Sprintf("LOCAL-%d", id), "get_config", gerror.Wrap(err, "get cobo config failed"))
	}

	// 检查API密钥是否配置
	transferWalletID := strings.TrimSpace(cfg.Withdraw.WalletID)
	if transferWalletID == "" {
		transferWalletID = strings.TrimSpace(cfg.WalletID)
	}
	if cfg.APISecret == "" || transferWalletID == "" {
		return failAndUnfreeze(
			fmt.Sprintf("LOCAL-%d", id),
			"missing_credentials",
			gerror.New("api secret or wallet id not configured"),
		)
	}

	// 创建Cobo客户端
	coboClient, err := externalCobo.NewCoboClient(&externalCobo.Config{
		APISecret: cfg.APISecret,
		Env:       cfg.Env,
		Timeout:   cfg.Timeout,
	})
	if err != nil {
		return failAndUnfreeze(fmt.Sprintf("LOCAL-%d", id), "new_client", gerror.Wrap(err, "create cobo client failed"))
	}

	// 构建Token ID（格式: Chain_Symbol，如 BSC_USDT）
	tokenID := buildWithdrawTokenID(withdraw.Chain, withdraw.Symbol)

	// 根据钱包子类型决定转账来源
	walletInfo, err := coboClient.GetWalletById(ctx, transferWalletID)
	if err != nil {
		return failAndUnfreeze(fmt.Sprintf("LOCAL-%d", id), "get_wallet", gerror.Wrap(err, "get cobo wallet info failed"))
	}

	walletSubtype := ""
	if walletInfo != nil {
		if actual := walletInfo.GetActualInstance(); actual != nil {
			if custodial, ok := actual.(*coboWaas2.CustodialWalletInfo); ok {
				walletSubtype = strings.ToLower(string(custodial.GetWalletSubtype()))
			}
		}
	}

	sourceAddress := ""
	if walletSubtype == "web3" {
		if sweepResp, e := coboClient.ListWalletSweepToAddresses(ctx, transferWalletID); e == nil && sweepResp != nil {
			for _, item := range sweepResp.GetData() {
				chainID := strings.ToUpper(strings.TrimSpace(item.GetChainId()))
				if chainID == "" {
					continue
				}
				if strings.HasPrefix(chainID, strings.ToUpper(withdraw.Chain)+"_") {
					sourceAddress = strings.ToLower(strings.TrimSpace(item.GetAddress()))
					break
				}
			}
			if sourceAddress == "" && len(sweepResp.GetData()) > 0 {
				sourceAddress = strings.ToLower(strings.TrimSpace(sweepResp.GetData()[0].GetAddress()))
			}
		}

		if sourceAddress == "" {
			var sourceAddressEntity struct {
				Address string `json:"address"`
			}
			_ = g.DB().Model("user_deposit_address").
				Ctx(ctx).
				Where("user_id = ? AND is_valid = ?", withdraw.UserID, true).
				OrderDesc("id").
				Scan(&sourceAddressEntity)
			sourceAddress = strings.ToLower(strings.TrimSpace(sourceAddressEntity.Address))
		}
		if sourceAddress == "" {
			return failAndUnfreeze(
				fmt.Sprintf("LOCAL-%d", id),
				"missing_source_address",
				gerror.New("web3 wallet withdraw missing source_address"),
			)
		}
	}

	// 发起转账请求
	requestID := fmt.Sprintf("WD-%d-%d", withdraw.ID, time.Now().Unix())
	transferAmount := withdraw.ActualAmount
	if transferAmount.LessThanOrEqual(decimal.Zero) {
		transferAmount = withdraw.Amount
	}

	transferReq := &externalCobo.TransferRequest{
		RequestID:     requestID,
		WalletID:      transferWalletID,
		SourceAddress: sourceAddress,
		TokenID:       tokenID,
		ToAddress:     withdraw.ToAddress,
		Amount:        transferAmount.String(),
		Description:   fmt.Sprintf("Withdraw order: %s", withdraw.OrderNo),
	}
	if walletSubtype == "web3" && cfg.Withdraw.GasPriceWei != "" {
		transferReq.GasPriceWei = cfg.Withdraw.GasPriceWei
		transferReq.GasTokenID = cfg.Withdraw.GasTokenID
	}

	g.Log().Infof(ctx, "[Cobo] submit withdraw transfer: id=%d, order_no=%s, token=%s, amount=%s, to=%s, wallet_id=%s, wallet_subtype=%s",
		id, withdraw.OrderNo, tokenID, transferReq.Amount, withdraw.ToAddress, transferWalletID, walletSubtype)

	// 预写入 cobo_request_id，防止 Cobo 在 API 返回前推送 webhook 时查不到记录
	if err := s.withdrawRepo.UpdateCoboRequestID(ctx, id, requestID); err != nil {
		g.Log().Warningf(ctx, "[Cobo] pre-write cobo_request_id failed: id=%d, err=%v", id, err)
	}

	transferResp, err := coboClient.Transfer(ctx, transferReq)
	if err != nil {
		// 转账失败，更新状态为失败并解冻余额
		g.Log().Errorf(ctx, "[Cobo] transfer failed: id=%d, err=%v", id, err)

		failReason := err.Error()
		if len(failReason) > 500 {
			failReason = failReason[:500]
		}

		// 更新提现记录为失败（JSONB 需写入合法 JSON）
		failRespData, _ := json.Marshal(map[string]string{"error": failReason})
		if updateErr := s.withdrawRepo.UpdateCoboInfo(ctx, id, requestID, string(failRespData), ""); updateErr != nil {
			g.Log().Errorf(ctx, "[Cobo] update fail status failed: id=%d, err=%v", id, updateErr)
		}

		// 解冻用户余额
		if balanceErr := s.unfreezeBalanceWithLog(ctx, withdraw.UserID, withdraw.Symbol, withdraw.Amount, withdraw.OrderNo, withdraw.ID, "withdraw failed refund"); balanceErr != nil {
			g.Log().Errorf(ctx, "[Cobo] unfreeze balance failed: id=%d, user_id=%d, err=%v", id, withdraw.UserID, balanceErr)
		}

		// 更新状态为失败
		if statusErr := s.withdrawRepo.UpdateStatus(ctx, id, cobo.WithdrawStatusFailed); statusErr != nil {
			g.Log().Errorf(ctx, "[Cobo] update withdraw status to failed failed: id=%d, err=%v", id, statusErr)
		} else {
			failedWithdraw := *withdraw
			failedWithdraw.Status = cobo.WithdrawStatusFailed
			GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, &failedWithdraw, currentStatus, "submit transfer failed")
		}

		return gerror.Wrap(err, "submit transfer failed")
	}

	// 转账成功，更新Cobo返回信息
	respData, _ := json.Marshal(map[string]interface{}{
		"transaction_id": transferResp.TransactionID,
		"status":         transferResp.Status,
		"request_id":     requestID,
	})

	if err := s.withdrawRepo.UpdateCoboInfo(ctx, id, requestID, string(respData), ""); err != nil {
		g.Log().Warningf(ctx, "[Cobo] update cobo response info failed: id=%d, err=%v", id, err)
	}

	g.Log().Infof(ctx, "[Cobo] withdraw transfer submitted: id=%d, order_no=%s, cobo_tx_id=%s, status=%s",
		id, withdraw.OrderNo, transferResp.TransactionID, transferResp.Status)

	return nil
}

func (s *withdrawService) submitHotwalletWithdraw(ctx context.Context, withdraw *cobo.WithdrawEntity, requestID string) error {
	endpointVar, err := g.DB().Model("system_config").Ctx(ctx).
		Fields("value").Where("key = ?", consts.SZPNMonitorWithdrawEndpointConfigKey).Value()
	if err != nil {
		return gerror.Wrap(err, "read hotwallet monitor withdraw endpoint failed")
	}
	endpoint := strings.TrimSpace(endpointVar.String())
	if endpoint == "" {
		return gerror.New("hotwallet monitor withdraw endpoint not configured")
	}

	secretVar, err := g.DB().Model("system_config").Ctx(ctx).
		Fields("value").Where("key = ?", consts.SZPNMonitorCallbackSecretConfigKey).Value()
	if err != nil {
		return gerror.Wrap(err, "read hotwallet monitor callback secret failed")
	}
	secret := strings.TrimSpace(secretVar.String())
	if secret == "" {
		return gerror.New("hotwallet monitor callback secret not configured")
	}

	payload := &model.SZPNWithdrawSubmitReq{
		WithdrawID: withdraw.ID,
		OrderNo:    withdraw.OrderNo,
		RequestID:  requestID,
		Symbol:     withdraw.Symbol,
		ToAddress:  strings.ToLower(strings.TrimSpace(withdraw.ToAddress)),
		Amount:     withdraw.ActualAmount.String(),
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return gerror.Wrap(err, "build hotwallet withdraw submit request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", secret)

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return gerror.Wrap(err, "call hotwallet monitor withdraw endpoint failed")
	}
	defer resp.Body.Close()
	bodyResp, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return gerror.Newf("hotwallet monitor withdraw endpoint status invalid: %d", resp.StatusCode)
	}

	var parsed struct {
		Code int                          `json:"code"`
		Data model.SZPNWithdrawSubmitRes `json:"data"`
		Msg  string                       `json:"msg"`
	}
	if err := json.Unmarshal(bodyResp, &parsed); err != nil {
		return gerror.Wrap(err, "parse hotwallet monitor withdraw response failed")
	}
	if parsed.Code != 0 {
		return gerror.Newf("hotwallet monitor withdraw business failed: code=%d msg=%s", parsed.Code, parsed.Msg)
	}
	if !parsed.Data.Accepted {
		return gerror.Newf("hotwallet monitor withdraw rejected: %s", parsed.Data.Reason)
	}
	return nil
}

func (s *withdrawService) HandleHotwalletWithdrawCallback(ctx context.Context, req *model.SZPNWithdrawCallbackReq) (*model.SZPNWithdrawCallbackRes, error) {
	requestID := strings.TrimSpace(req.RequestID)
	orderNo := strings.TrimSpace(req.OrderNo)
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if requestID == "" || orderNo == "" || status == "" {
		return &model.SZPNWithdrawCallbackRes{Accepted: false, Processed: false, Reason: "invalid request"}, nil
	}

	withdraw, err := s.withdrawRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, gerror.Wrap(err, "query withdraw record failed")
	}
	if withdraw == nil || !s.isHotwalletWithdrawSymbol(ctx, withdraw.Symbol) {
		return &model.SZPNWithdrawCallbackRes{Accepted: true, Processed: false, Reason: "withdraw not found"}, nil
	}
	if withdraw.Status == cobo.WithdrawStatusSuccess || withdraw.Status == cobo.WithdrawStatusFailed || withdraw.Status == cobo.WithdrawStatusRejected {
		return &model.SZPNWithdrawCallbackRes{Accepted: true, Processed: true}, nil
	}

	switch status {
	case "completed", "success", "succeeded":
		statusChanged := false
		err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			result, err := tx.Model("cobo_withdraw_request").Ctx(ctx).
				Data(g.Map{"status": cobo.WithdrawStatusSuccess, "tx_hash": strings.TrimSpace(req.TxHash), "updated_at": time.Now()}).
				Where("id = ? AND status IN (?, ?, ?)", withdraw.ID, cobo.WithdrawStatusPending, cobo.WithdrawStatusApproved, cobo.WithdrawStatusProcessing).
				Update()
			if err != nil {
				return err
			}
			affected, _ := result.RowsAffected()
			if affected == 0 {
				return nil
			}
			statusChanged = true

			balanceBefore := decimal.Zero
			balanceRow, _ := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, withdraw.UserID, withdraw.Symbol)
			if balanceRow != nil {
				balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
			}

			// 提现成功时扣税（pool + YYAI）
			if err := s.applyWithdrawTaxPoolTx(ctx, tx, withdraw.UserID, withdraw.TaxAmount, balanceBefore, withdraw.OrderNo, withdraw.ID, ""); err != nil {
				return err
			}
			balanceBefore = balanceBefore.Sub(withdraw.TaxAmount)
			if _, err := s.applyWithdrawTaxYYAITx(ctx, tx, withdraw.UserID, withdraw.TaxYyaiUsdtAmount, withdraw.OrderNo); err != nil {
				return err
			}
			balanceBefore = balanceBefore.Sub(withdraw.TaxYyaiUsdtAmount)

			// 扣除剩余冻结金额（税款已在成功分支中从 frozen 扣减）
			frozenDeduct := withdraw.Amount.Sub(withdraw.TaxAmount).Sub(withdraw.TaxYyaiUsdtAmount)
			if frozenDeduct.LessThan(decimal.Zero) {
				return gerror.New("withdraw frozen deduct amount is negative")
			}
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, withdraw.UserID, withdraw.Symbol, decimal.Zero, frozenDeduct.Neg()); err != nil {
				return err
			}
			balanceAfter := balanceBefore.Sub(frozenDeduct)
			return writeBalanceChangeLogTx(ctx, tx, withdraw.UserID, withdraw.Symbol, consts.ChangeTypeWithdrawSuccess, frozenDeduct.Neg(), balanceBefore, balanceAfter, withdraw.OrderNo, withdraw.ID, "szpn withdraw success deduct frozen", consts.OperatorTypeSystem)
		})
		if err != nil {
			return nil, gerror.Wrap(err, "handle szpn withdraw success callback failed")
		}
		if statusChanged {
			updatedWithdraw := *withdraw
			updatedWithdraw.Status = cobo.WithdrawStatusSuccess
			updatedWithdraw.TxHash = strings.TrimSpace(req.TxHash)
			GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, &updatedWithdraw, withdraw.Status, "")
		}
	case "failed", "rejected":
		statusChanged := false
		err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			result, err := tx.Model("cobo_withdraw_request").Ctx(ctx).
				Data(g.Map{"status": cobo.WithdrawStatusFailed, "tx_hash": strings.TrimSpace(req.TxHash), "updated_at": time.Now()}).
				Where("id = ? AND status IN (?, ?, ?)", withdraw.ID, cobo.WithdrawStatusPending, cobo.WithdrawStatusApproved, cobo.WithdrawStatusProcessing).
				Update()
			if err != nil {
				return err
			}
			affected, _ := result.RowsAffected()
			if affected == 0 {
				return nil
			}
			statusChanged = true

			balanceBefore := decimal.Zero
			balanceRow, _ := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, withdraw.UserID, withdraw.Symbol)
			if balanceRow != nil {
				balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
			}
			refundAmount := withdraw.Amount
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, withdraw.UserID, withdraw.Symbol, refundAmount, refundAmount.Neg()); err != nil {
				return err
			}
			if err := writeBalanceChangeLogTx(ctx, tx, withdraw.UserID, withdraw.Symbol, consts.ChangeTypeWithdrawFailRefund, refundAmount, balanceBefore, balanceBefore, withdraw.OrderNo, withdraw.ID, "szpn withdraw fail refund", consts.OperatorTypeSystem); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return nil, gerror.Wrap(err, "handle szpn withdraw failed callback failed")
		}
		if statusChanged {
			updatedWithdraw := *withdraw
			updatedWithdraw.Status = cobo.WithdrawStatusFailed
			updatedWithdraw.TxHash = strings.TrimSpace(req.TxHash)
			GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, &updatedWithdraw, withdraw.Status, req.Status)
		}
	default:
		updateData := g.Map{
			"status":     cobo.WithdrawStatusProcessing,
			"updated_at": time.Now(),
		}
		if txHash := strings.TrimSpace(req.TxHash); txHash != "" {
			updateData["tx_hash"] = txHash
		}
		_, err := g.DB().Model("cobo_withdraw_request").Ctx(ctx).
			Data(updateData).
			Where("id = ? AND status IN (?, ?, ?)", withdraw.ID, cobo.WithdrawStatusPending, cobo.WithdrawStatusApproved, cobo.WithdrawStatusProcessing).
			Update()
		if err != nil {
			return nil, gerror.Wrap(err, "update szpn withdraw processing status failed")
		}
	}

	respData, _ := json.Marshal(map[string]string{"stage": "szpn_callback", "request_id": requestID, "status": status, "reason": strings.TrimSpace(req.Reason)})
	_ = s.withdrawRepo.UpdateCoboInfo(ctx, withdraw.ID, requestID, string(respData), strings.TrimSpace(req.TxHash))
	return &model.SZPNWithdrawCallbackRes{Accepted: true, Processed: true}, nil
}

// processContractWithdraw 处理 YY/YYAI 合约提现（TokenStoreBurn）
func (s *withdrawService) processContractWithdraw(ctx context.Context, withdraw *cobo.WithdrawEntity, currentStatus int) error {
	g.Log().Infof(ctx, "[Cobo] processing contract withdraw: id=%d, symbol=%s, amount=%s, to=%s", withdraw.ID, withdraw.Symbol, withdraw.Amount.String(), withdraw.ToAddress)

	burnSvc, err := blockchain.NewTokenStoreBurn()
	if err != nil {
		g.Log().Errorf(ctx, "[Cobo] NewTokenStoreBurn failed: id=%d, err=%v", withdraw.ID, err)
		return s.failContractWithdraw(ctx, withdraw, currentStatus, "new_token_store_burn", err)
	}

	toAddr := common.HexToAddress(withdraw.ToAddress)
	chainAmount := withdraw.Amount.Shift(consts.BlockchainDefaultDecimals).BigInt()

	var txHash string
	if withdraw.Symbol == "YY" {
		txHash, err = burnSvc.WithdrawYY(ctx, toAddr, chainAmount)
	} else {
		txHash, err = burnSvc.WithdrawYYAI(ctx, toAddr, chainAmount)
	}
	if err != nil {
		g.Log().Errorf(ctx, "[Cobo] contract withdraw failed: id=%d, symbol=%s, err=%v", withdraw.ID, withdraw.Symbol, err)
		return s.failContractWithdraw(ctx, withdraw, currentStatus, "contract_withdraw", err)
	}

	g.Log().Infof(ctx, "[Cobo] contract withdraw success: id=%d, symbol=%s, txHash=%s", withdraw.ID, withdraw.Symbol, txHash)

	// 扣除冻结余额
	balanceBefore := decimal.Zero
	balanceRow, err := s.balanceRepo.GetByUserID(ctx, withdraw.UserID, withdraw.Symbol)
	if err != nil {
		g.Log().Warningf(ctx, "[Cobo] get balance before deduct failed: id=%d, err=%v", withdraw.ID, err)
	} else if balanceRow != nil {
		balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
	}
	if err := s.balanceRepo.UpdateBalance(ctx, withdraw.UserID, withdraw.Symbol, decimal.Zero, withdraw.Amount.Neg()); err != nil {
		g.Log().Errorf(ctx, "[Cobo] deduct frozen balance failed: id=%d, err=%v", withdraw.ID, err)
		return s.failContractWithdraw(ctx, withdraw, currentStatus, "deduct_frozen_balance", err)
	}

	balanceAfter := balanceBefore.Sub(withdraw.Amount)
	if err := writeBalanceChangeLog(ctx, withdraw.UserID, withdraw.Symbol, consts.ChangeTypeWithdraw, withdraw.Amount.Neg(), balanceBefore, balanceAfter, withdraw.OrderNo, withdraw.ID, "contract withdraw success", consts.OperatorTypeSystem); err != nil {
		g.Log().Warningf(ctx, "[Cobo] write balance change log failed: id=%d, err=%v", withdraw.ID, err)
	}

	// 更新记录
	if err := s.withdrawRepo.UpdateTxHashAndStatus(ctx, withdraw.ID, txHash, cobo.WithdrawStatusSuccess); err != nil {
		g.Log().Errorf(ctx, "[Cobo] update tx_hash and status failed: id=%d, err=%v", withdraw.ID, err)
		return err
	}

	successWithdraw := *withdraw
	successWithdraw.Status = cobo.WithdrawStatusSuccess
	successWithdraw.TxHash = txHash
	GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, &successWithdraw, currentStatus, "contract withdraw success")
	return nil
}

// failContractWithdraw 合约提现失败处理：解冻余额并更新状态
func (s *withdrawService) failContractWithdraw(ctx context.Context, withdraw *cobo.WithdrawEntity, currentStatus int, stage string, cause error) error {
	if balanceErr := s.unfreezeBalanceWithLog(ctx, withdraw.UserID, withdraw.Symbol, withdraw.Amount, withdraw.OrderNo, withdraw.ID, "contract withdraw failed refund"); balanceErr != nil {
		g.Log().Errorf(ctx, "[Cobo] unfreeze balance failed: id=%d, err=%v", withdraw.ID, balanceErr)
	}
	if statusErr := s.withdrawRepo.UpdateStatus(ctx, withdraw.ID, cobo.WithdrawStatusFailed); statusErr != nil {
		g.Log().Errorf(ctx, "[Cobo] update status to failed failed: id=%d, err=%v", withdraw.ID, statusErr)
	}
	failedWithdraw := *withdraw
	failedWithdraw.Status = cobo.WithdrawStatusFailed
	GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, &failedWithdraw, currentStatus, stage)
	if cause != nil {
		return gerror.Wrapf(cause, "contract withdraw failed(%s)", stage)
	}
	return gerror.Newf("contract withdraw failed(%s)", stage)
}

func defaultWithdrawConfigMap() map[string]*model.WithdrawSymbolConfig {
	return map[string]*model.WithdrawSymbolConfig{
		"USDT": {MinAmount: "10"},
		"YY":   {MinAmount: "20"},
		"JU":     {MinAmount: "1"},
		"YYAI":   {MinAmount: "10"},
		"BOX":    {MinAmount: "1"},
		"LABUBU": {MinAmount: "1"},
	}
}

func buildWithdrawTokenID(chain, symbol string) string {
	chain = strings.ToUpper(strings.TrimSpace(chain))
	symbol = strings.ToUpper(strings.TrimSpace(symbol))

	if symbol == "JU" {
		return "JU_JU"
	}

	return fmt.Sprintf("%s_%s", chain, symbol)
}

func (s *withdrawService) getSymbolWithdrawConfigMap(ctx context.Context) (map[string]*model.WithdrawSymbolConfig, error) {
	configMap := defaultWithdrawConfigMap()
	configs, err := repository.NewTokenConfigRepository().GetAllEnabled(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "query token config failed")
	}
	for _, item := range configs {
		if item == nil || strings.TrimSpace(item.Symbol) == "" {
			continue
		}
		symbol := strings.ToUpper(strings.TrimSpace(item.Symbol))
		configMap[symbol] = &model.WithdrawSymbolConfig{MinAmount: decimal.NewFromFloat(item.MinWithdrawAmount).String()}
	}
	return configMap, nil
}

func (s *withdrawService) getSymbolMinWithdrawAmount(ctx context.Context, symbol string) (decimal.Decimal, bool) {
	cfgMap, err := s.getSymbolWithdrawConfigMap(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "[Cobo] load token min withdraw config failed, fallback default: err=%v", err)
		cfgMap = defaultWithdrawConfigMap()
	}
	cfg, ok := cfgMap[strings.ToUpper(strings.TrimSpace(symbol))]
	if !ok || cfg == nil {
		return decimal.Zero, false
	}
	minAmount, err := decimal.NewFromString(cfg.MinAmount)
	if err != nil {
		return decimal.Zero, false
	}
	return minAmount, true
}

// riskCheck 风控检查
func (s *withdrawService) riskCheck(ctx context.Context, req *model.CreateWithdrawReq, cfg *config.CoboConfig) error {
	locale := consts.LocaleFromCtx(ctx)
	rules := cfg.Withdraw.RiskRules

	// 1. 检查金额范围（按配置）
	if rules.MinAmount <= 0 || rules.MaxAmount <= 0 {
		g.Log().Errorf(ctx, "[Cobo] withdraw risk config invalid: min_amount=%v, max_amount=%v", rules.MinAmount, rules.MaxAmount)
		return withdrawErr(locale, "withdraw_config_invalid")
	}
	minAmount := decimal.NewFromFloat(rules.MinAmount)
	if symbolMin, ok := s.getSymbolMinWithdrawAmount(ctx, req.Symbol); ok {
		minAmount = symbolMin
	}
	maxAmount := decimal.NewFromFloat(rules.MaxAmount)
	if maxAmount.LessThan(minAmount) {
		g.Log().Errorf(ctx, "[Cobo] withdraw risk config invalid: min_amount=%s, max_amount=%s", minAmount.String(), maxAmount.String())
		return withdrawErr(locale, "withdraw_config_invalid")
	}
	if req.Amount.LessThan(minAmount) {
		g.Log().Infof(ctx, "[Cobo] withdraw risk rejected (min amount): user_id=%d, amount=%s, min_amount=%s", req.UserID, req.Amount.String(), minAmount.String())
		return withdrawErr(locale, "amount_below_min")
	}
	if req.Amount.GreaterThan(maxAmount) {
		g.Log().Infof(ctx, "[Cobo] withdraw above max amount, auto approve after risk checks: user_id=%d, amount=%s, max_amount=%s", req.UserID, req.Amount.String(), maxAmount.String())
	}

	// 2. 集中式风控检查
	if err := s.riskControl.CheckWithdraw(ctx, req.UserID, req.Amount, req.Symbol); err != nil {
		g.Log().Warningf(ctx, "[Cobo] withdraw risk rejected: user_id=%d, err=%v", req.UserID, err)
		return err
	}

	g.Log().Infof(ctx, "[Cobo] withdraw risk passed: user_id=%d, amount=%s, min_amount=%s, max_amount=%s", req.UserID, req.Amount.String(), minAmount.String(), maxAmount.String())

	return nil
}

// shouldAutoApprove 是否自动审核通过
func (s *withdrawService) shouldAutoApprove(amount decimal.Decimal, cfg *config.CoboConfig) bool {
	threshold := decimal.NewFromFloat(cfg.Withdraw.AutoApproveThreshold)
	if threshold.GreaterThan(decimal.Zero) && amount.LessThanOrEqual(threshold) {
		return true
	}

	maxAmount := decimal.NewFromFloat(cfg.Withdraw.RiskRules.MaxAmount)
	return maxAmount.GreaterThan(decimal.Zero) && amount.GreaterThan(maxAmount)
}

func (s *withdrawService) isLargeUSDTWithdraw(symbol string, amount decimal.Decimal, cfg *config.CoboConfig) bool {
	if normalizeWithdrawSymbol(symbol) != "USDT" {
		return false
	}
	maxAmount := decimal.NewFromFloat(cfg.Withdraw.RiskRules.MaxAmount)
	return maxAmount.GreaterThan(decimal.Zero) && amount.GreaterThan(maxAmount)
}

func (s *withdrawService) acquireUserWithdrawLock(ctx context.Context, userID int64) (bool, error) {
	redisClient := g.Redis()
	if redisClient == nil {
		return false, gerror.New("redis not initialized")
	}

	key := fmt.Sprintf("cobo:withdraw:lock:%d", userID)
	result, err := redisClient.Do(ctx, "SET", key, "1", "NX", "EX", withdrawUserLockTTLSeconds)
	if err != nil {
		return false, err
	}
	if result.IsNil() {
		return false, nil
	}
	return result.String() == "OK", nil
}

func (s *withdrawService) unfreezeBalanceWithLog(ctx context.Context, userID int64, symbol string, amount decimal.Decimal, orderNo string, relatedID int64, remark string) error {
	balanceBefore := decimal.Zero
	balanceRow, err := s.balanceRepo.GetByUserID(ctx, userID, symbol)
	if err != nil {
		return err
	}
	if balanceRow != nil {
		balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
	}
	if err := s.balanceRepo.UpdateBalance(ctx, userID, symbol, amount, amount.Neg()); err != nil {
		return err
	}
	if err := writeBalanceChangeLog(ctx, userID, symbol, consts.ChangeTypeWithdrawFailRefund, amount, balanceBefore, balanceBefore, orderNo, relatedID, remark, consts.OperatorTypeSystem); err != nil {
		return err
	}
	return nil
}

func (s *withdrawService) releaseUserWithdrawLock(ctx context.Context, userID int64) {
	redisClient := g.Redis()
	if redisClient == nil {
		return
	}

	key := fmt.Sprintf("cobo:withdraw:lock:%d", userID)
	if _, err := redisClient.Do(ctx, "DEL", key); err != nil {
		g.Log().Warningf(ctx, "[Cobo] release withdraw lock failed: user_id=%d, err=%v", userID, err)
	}
}


// applyWithdrawTaxPoolTx 在事务内执行提现税款沉淀到 US Stock Reward Pool
//   - payerUserID：提现申请人；payerBalanceBefore 为冻结操作完成后的总余额
//   - tax：实际沉淀到 pool 的金额（actualTax）
func (s *withdrawService) applyWithdrawTaxPoolTx(ctx context.Context, tx gdb.TX, payerUserID int64, tax decimal.Decimal, payerBalanceBefore decimal.Decimal, orderNo string, withdrawID int64, taxFormulaRemark string) error {
	if tax.LessThanOrEqual(decimal.Zero) {
		return nil
	}

	// 1. 付款方：从冻结余额扣减 tax（available 不变，frozen -= tax）
	if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, payerUserID, "USDT", decimal.Zero, tax.Neg()); err != nil {
		return gerror.Wrap(err, "deduct withdraw tax frozen balance failed")
	}
	payerAfter := payerBalanceBefore.Sub(tax)
	payerRemark := "withdraw tax pool deposit"
	if strings.TrimSpace(taxFormulaRemark) != "" {
		payerRemark = taxFormulaRemark
	}
	if err := writeBalanceChangeLogTx(ctx, tx, payerUserID, "USDT", consts.ChangeTypeWithdrawTaxPay, tax.Neg(), payerBalanceBefore, payerAfter, orderNo, 0, payerRemark, consts.OperatorTypeSystem); err != nil {
		return gerror.Wrap(err, "write withdraw tax payer audit log failed")
	}

	// 2. 沉淀到 pool（逐笔关联提现ID）
	poolDate := time.Now().In(cnLocation).Format("2006-01-02")
	_, err := tx.Exec(`
		INSERT INTO us_stock_reward_pool (withdraw_id, user_id, pool_date, amount, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (withdraw_id)
		DO UPDATE SET
			amount = EXCLUDED.amount,
			pool_date = EXCLUDED.pool_date,
			updated_at = CURRENT_TIMESTAMP
	`, withdrawID, payerUserID, poolDate, tax)
	if err != nil {
		return gerror.Wrap(err, "deposit withdraw tax to pool failed")
	}

	g.Log().Infof(ctx, "[Cobo] withdraw tax pool deposit: order_no=%s, withdraw_id=%d, payer=%d, pool_date=%s, tax=%s",
		orderNo, withdrawID, payerUserID, poolDate, tax.String())
	return nil
}

// DeductWithdrawTaxOnSuccess 提现成功时扣税（pool + YYAI）
func (s *withdrawService) DeductWithdrawTaxOnSuccess(ctx context.Context, tx gdb.TX, withdraw *cobo.WithdrawEntity, balanceBefore decimal.Decimal) (decimal.Decimal, error) {
	if withdraw == nil {
		return balanceBefore, nil
	}

	// 扣 pool 税
	if err := s.applyWithdrawTaxPoolTx(ctx, tx, withdraw.UserID, withdraw.TaxAmount, balanceBefore, withdraw.OrderNo, withdraw.ID, ""); err != nil {
		return balanceBefore, err
	}
	balanceBefore = balanceBefore.Sub(withdraw.TaxAmount)

	// 扣 YYAI 兑换税
	if _, err := s.applyWithdrawTaxYYAITx(ctx, tx, withdraw.UserID, withdraw.TaxYyaiUsdtAmount, withdraw.OrderNo); err != nil {
		return balanceBefore, err
	}
	balanceBefore = balanceBefore.Sub(withdraw.TaxYyaiUsdtAmount)

	return balanceBefore, nil
}

// applyWithdrawTaxYYAITx 在事务内执行税款兑换YYAI：用户 USDT 冻结减少，YYAI 可用增加
func (s *withdrawService) applyWithdrawTaxYYAITx(ctx context.Context, tx gdb.TX, userID int64, yyaiTaxUsdt decimal.Decimal, orderNo string) (decimal.Decimal, error) {
	if yyaiTaxUsdt.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, nil
	}

	yyaiPrice, err := getYYAIPrice(ctx, tx)
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "query YYAI price failed")
	}
	yyaiAmount := yyaiTaxUsdt.Div(yyaiPrice).Round(8)
	if yyaiAmount.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, gerror.New("invalid YYAI swap amount")
	}

	// 1. 用户 USDT 冻结扣减
	usdtBalance, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, userID, "USDT")
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "get user USDT balance failed")
	}
	usdtBefore := decimal.Zero
	if usdtBalance != nil {
		usdtBefore = usdtBalance.AvailableAmount.Add(usdtBalance.FrozenAmount)
	}
	if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, userID, "USDT", decimal.Zero, yyaiTaxUsdt.Neg()); err != nil {
		return decimal.Zero, gerror.Wrap(err, "deduct USDT frozen balance failed")
	}
	usdtAfter := usdtBefore.Sub(yyaiTaxUsdt)
	if err := writeBalanceChangeLogTx(ctx, tx, userID, "USDT", consts.ChangeTypeWithdrawTaxPay, yyaiTaxUsdt.Neg(), usdtBefore, usdtAfter, orderNo, 0, fmt.Sprintf("withdraw tax yyai usdt deduct (price=%s)", yyaiPrice.String()), consts.OperatorTypeSystem); err != nil {
		return decimal.Zero, gerror.Wrap(err, "write USDT deduct audit log failed")
	}

	// 2. 用户 YYAI 可用增加
	yyaiBalance, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, userID, "YYAI")
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "get user YYAI balance failed")
	}
	yyaiBefore := decimal.Zero
	if yyaiBalance != nil {
		yyaiBefore = yyaiBalance.AvailableAmount.Add(yyaiBalance.FrozenAmount)
	}
	if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, userID, "YYAI", yyaiAmount, decimal.Zero); err != nil {
		return decimal.Zero, gerror.Wrap(err, "credit YYAI balance failed")
	}
	yyaiAfter := yyaiBefore.Add(yyaiAmount)
	if err := writeBalanceChangeLogTx(ctx, tx, userID, "YYAI", consts.ChangeTypeWithdrawTaxYYAI, yyaiAmount, yyaiBefore, yyaiAfter, orderNo, 0, fmt.Sprintf("withdraw tax converted to YYAI (price=%s)", yyaiPrice.String()), consts.OperatorTypeSystem); err != nil {
		return decimal.Zero, gerror.Wrap(err, "write YYAI grant audit log failed")
	}

	// 3. 写入 swap_record，让提现税款兑换在兑换记录中可见
	swapRecord := &entity.SwapRecordEntity{
		UserID:     userID,
		FromSymbol: "USDT",
		ToSymbol:   "YYAI",
		FromAmount: yyaiTaxUsdt,
		ToAmount:   yyaiAmount,
		Fee:        decimal.Zero,
		Price:      yyaiPrice,
		RequestID:  orderNo,
	}
	if err := s.swapRecordDao.Create(ctx, tx, swapRecord); err != nil {
		return decimal.Zero, gerror.Wrap(err, "write swap_record failed")
	}

	g.Log().Infof(ctx, "[Cobo] withdraw tax converted to YYAI: order_no=%s, user_id=%d, usdt=%s, yyai=%s, price=%s",
		orderNo, userID, yyaiTaxUsdt.String(), yyaiAmount.String(), yyaiPrice.String())
	return yyaiAmount, nil
}
