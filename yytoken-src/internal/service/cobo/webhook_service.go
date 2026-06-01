package cobo

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"XWFrame/internal/entity/cobo"
	"XWFrame/internal/frame/config"
	"XWFrame/internal/frame/consts"
	coboRepo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/service/cobo/model"
	externalCobo "XWFrame/pkg/external/cobo"
	"XWFrame/pkg/utils"

	coboWaas2 "github.com/CoboGlobal/cobo-waas2-go-sdk/cobo_waas2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// WebhookService Webhook服务接口
type WebhookService interface {
	// HandleRecharge 处理充值回调
	HandleRecharge(ctx context.Context, payload *model.WebhookPayload) error

	// HandleWithdraw 处理提现回调
	HandleWithdraw(ctx context.Context, payload *model.WebhookPayload) error

	// VerifySignature 验证Webhook签名
	VerifySignature(ctx context.Context, body []byte, signature string, timestamp string) error

	// ConfirmPendingRecharges 检查待确认充值并确认入账
	ConfirmPendingRecharges(ctx context.Context, limit int) error
}

// webhookService Webhook服务实现
type webhookService struct {
	balanceRepo     coboRepo.IBalanceRepository
	rechargeRepo    coboRepo.IRechargeRepository
	withdrawRepo    coboRepo.IWithdrawRepository
	withdrawService WithdrawService
	coboConfig      *config.CoboConfig
}

// NewWebhookService 创建Webhook服务
func NewWebhookService() WebhookService {
	return &webhookService{
		balanceRepo:     coboRepo.NewBalanceRepository(),
		rechargeRepo:    coboRepo.NewRechargeRepository(),
		withdrawRepo:    coboRepo.NewWithdrawRepository(),
		withdrawService: NewWithdrawService(),
	}
}

// HandleRecharge 处理充值回调
func (s *webhookService) HandleRecharge(ctx context.Context, payload *model.WebhookPayload) error {
	status := strings.ToLower(payload.Status)
	if status != "confirmed" && status != "confirming" && status != "completed" && status != "succeeded" {
		g.Log().Infof(ctx, "[Cobo Webhook] 非确认状态，忽略: status=%s", payload.Status)
		return nil
	}

	// 1. webhook_id 幂等检查
	existing, err := s.rechargeRepo.GetByWebhookID(ctx, payload.ID)
	if err != nil {
		return gerror.Wrap(err, "查询充值记录失败")
	}
	if existing != nil {
		g.Log().Infof(ctx, "[Cobo Webhook] 重复通知，已处理过: webhook_id=%s", payload.ID)
		return nil
	}

	// 2. 查找用户（通过Cobo地址）
	coboAddress := strings.ToLower(payload.Data.ToAddress)
	userID, err := s.getUserIDByCoboAddress(ctx, coboAddress)
	if err != nil {
		return gerror.Wrap(err, "查找用户失败")
	}
	if userID == 0 {
		g.Log().Warningf(ctx, "[Cobo Webhook] 未找到对应用户: cobo_address=%s", coboAddress)
		return nil
	}

	// 3. 解析金额
	amount, err := decimal.NewFromString(payload.Data.Amount)
	if err != nil {
		return gerror.Wrap(err, "解析金额失败")
	}

	// 4. 充币白名单校验：通过 token_config.cobo_asset_code 匹配
	allowedSymbol, err := s.checkDepositWhitelist(ctx, payload.Data.Asset)
	if err != nil {
		return gerror.Wrap(err, "查询充币白名单失败")
	}

	// 5. 构建记录并原子创建（正常路径和拒绝路径共用同一套冲突处理）
	webhookData, _ := json.Marshal(payload)
	var recharge *cobo.RechargeEntity
	if allowedSymbol == "" {
		recharge = &cobo.RechargeEntity{
			UserID:        userID,
			CoboAddress:   coboAddress,
			TxHash:        payload.Data.TxHash,
			FromAddress:   strings.ToLower(payload.Data.FromAddress),
			Symbol:        payload.Data.Asset,
			Amount:        amount,
			Status:        cobo.RechargeStatusRejected,
			Confirmations: payload.Data.Confirmations,
			WebhookID:     payload.ID,
			WebhookData:   string(webhookData),
		}
	} else {
		recharge = &cobo.RechargeEntity{
			UserID:        userID,
			CoboAddress:   coboAddress,
			TxHash:        payload.Data.TxHash,
			FromAddress:   strings.ToLower(payload.Data.FromAddress),
			Symbol:        allowedSymbol,
			Amount:        amount,
			Status:        cobo.RechargeStatusPending,
			Confirmations: payload.Data.Confirmations,
			WebhookID:     payload.ID,
			WebhookData:   string(webhookData),
		}
	}

	if err := s.rechargeRepo.Create(ctx, recharge); err != nil {
		if isDuplicateKeyError(err) && payload.Data.TxHash != "" {
			return s.handleDuplicateTxHash(ctx, payload, recharge.Symbol)
		}
		return gerror.Wrap(err, "保存充值记录失败")
	}

	// 6. 白名单拒绝：真正写入 rejected 后才发通知，避免并发冲突导致假警报
	if allowedSymbol == "" {
		g.Log().Warningf(ctx,
			"[Cobo Webhook] 拒绝非白名单充币: asset=%s, tx_hash=%s, user_id=%d, amount=%s",
			payload.Data.Asset, payload.Data.TxHash, userID, amount.String(),
		)
		go func(uid int64, amt decimal.Decimal, asset, hash string) {
			notifyCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			GetTelegramNotifyService(notifyCtx).NotifyRechargeRejected(notifyCtx, uid, amt, asset, hash)
		}(userID, amount, payload.Data.Asset, payload.Data.TxHash)
		return nil
	}

	// 7. 检查确认数，达到则入账
	cfg, err := s.getConfig(ctx)
	if err != nil {
		return err
	}

	if recharge.ID > 0 && payload.Data.Confirmations >= cfg.Recharge.MinConfirmations {
		if err := s.confirmRecharge(ctx, recharge.ID, recharge.UserID, recharge.Symbol, recharge.Amount, payload.Data.TxHash, payload.Data.Confirmations); err != nil {
			return gerror.Wrap(err, "确认充值失败")
		}
	}

	g.Log().Infof(ctx, "[Cobo Webhook] 充值处理完成: user_id=%d, amount=%s, confirmations=%d/%d",
		userID, amount.String(), payload.Data.Confirmations, cfg.Recharge.MinConfirmations)

	return nil
}

// handleDuplicateTxHash tx_hash 重复时统一处理（按 symbol + tx_hash 精确定位后更新确认数或触发入账）
func (s *webhookService) handleDuplicateTxHash(ctx context.Context, payload *model.WebhookPayload, symbol string) error {
	existingByTx, err := s.rechargeRepo.GetBySymbolAndTxHash(ctx, symbol, payload.Data.TxHash)
	if err != nil {
		return gerror.Wrap(err, "冲突后查询充值记录失败")
	}
	if existingByTx == nil {
		g.Log().Warningf(ctx, "[Cobo Webhook] tx_hash 冲突但查不到记录: symbol=%s, tx_hash=%s", symbol, payload.Data.TxHash)
		return nil
	}

	cfg, err := s.getConfig(ctx)
	if err != nil {
		return err
	}

	if payload.Data.Confirmations > existingByTx.Confirmations {
		if existingByTx.IsConfirmed() {
			if err := s.rechargeRepo.UpdateStatus(ctx, existingByTx.ID, cobo.RechargeStatusConfirmed, payload.Data.Confirmations); err != nil {
				return gerror.Wrap(err, "更新已确认充值确认数失败")
			}
		} else if payload.Data.Confirmations >= cfg.Recharge.MinConfirmations {
			if err := s.confirmRecharge(ctx, existingByTx.ID, existingByTx.UserID, existingByTx.Symbol, existingByTx.Amount, payload.Data.TxHash, payload.Data.Confirmations); err != nil {
				return gerror.Wrap(err, "重复通知确认充值失败")
			}
		} else {
			if err := s.rechargeRepo.UpdateStatus(ctx, existingByTx.ID, cobo.RechargeStatusPending, payload.Data.Confirmations); err != nil {
				return gerror.Wrap(err, "更新充值确认数失败")
			}
		}
	}

	g.Log().Infof(ctx, "[Cobo Webhook] 同交易重复通知，忽略入账: tx_hash=%s, webhook_id=%s", payload.Data.TxHash, payload.ID)
	return nil
}

// confirmRecharge 确认充值（增加用户余额）
func (s *webhookService) confirmRecharge(ctx context.Context, rechargeID, userID int64, symbol string, amount decimal.Decimal, txHash string, confirmations int) error {
	now := time.Now()

	// 原子确认：仅允许 pending -> confirmed 一次，避免并发重复入账
	result, err := g.DB().Model("cobo_recharge_record").
		Ctx(ctx).
		Data(g.Map{
			"status":        cobo.RechargeStatusConfirmed,
			"confirmations": confirmations,
			"confirmed_at":  now,
			"updated_at":    now,
		}).
		Where("id = ? AND status = ?", rechargeID, cobo.RechargeStatusPending).
		Update()
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		// 已确认则仅更新确认数，不重复入账
		_, err = g.DB().Model("cobo_recharge_record").
			Ctx(ctx).
			Data(g.Map{
				"confirmations": confirmations,
				"updated_at":    now,
			}).
			Where("id = ? AND status = ? AND confirmations < ?", rechargeID, cobo.RechargeStatusConfirmed, confirmations).
			Update()
		if err != nil {
			return err
		}

		g.Log().Infof(ctx, "[Cobo] 充值已确认，跳过重复入账: recharge_id=%d", rechargeID)
		return nil
	}

	// 增加余额
	balanceBefore := decimal.Zero
	balanceRow, _ := s.balanceRepo.GetByUserID(ctx, userID, symbol)
	if balanceRow != nil {
		balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
	}
	if err = s.balanceRepo.UpdateBalance(ctx, userID, symbol, amount, decimal.Zero); err != nil {
		return err
	}
	balanceAfter := balanceBefore.Add(amount)
	if err = writeBalanceChangeLog(ctx, userID, symbol, consts.ChangeTypeRecharge, amount, balanceBefore, balanceAfter, txHash, rechargeID, "Cobo recharge confirmed", consts.OperatorTypeSystem); err != nil {
		return gerror.Wrap(err, "写入充值审计日志失败")
	}

	g.Log().Infof(ctx, "[Cobo] 充值已确认并入账: user_id=%d, amount=%s", userID, amount.String())

	go func(uid int64, amt decimal.Decimal, sym, hash string) {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		totalVar, err := g.DB().Model("cobo_recharge_record").Ctx(notifyCtx).
			Fields("COALESCE(SUM(amount), 0)").
			Where("user_id = ? AND symbol = ? AND status = ?", uid, sym, cobo.RechargeStatusConfirmed).
			Value()
		totalAmount := decimal.Zero
		if err == nil && totalVar != nil {
			totalAmount, _ = decimal.NewFromString(totalVar.String())
		}

		GetTelegramNotifyService(notifyCtx).NotifyRechargeConfirmed(notifyCtx, uid, amt, totalAmount, sym, hash)
	}(userID, amount, symbol, txHash)

	return nil
}

// ConfirmPendingRecharges 检查待确认充值并确认入账
func (s *webhookService) ConfirmPendingRecharges(ctx context.Context, limit int) error {
	if limit <= 0 {
		limit = 100
	}

	pendingRecords, err := s.rechargeRepo.GetPendingWithTxHash(ctx, limit)
	if err != nil {
		return gerror.Wrap(err, "查询待确认充值记录失败")
	}
	if len(pendingRecords) == 0 {
		return nil
	}

	cfg, err := s.getConfig(ctx)
	if err != nil {
		return err
	}

	if cfg.APISecret == "" {
		g.Log().Warning(ctx, "[Cobo Recharge Checker] API密钥未配置，跳过待确认充值检查")
		return nil
	}

	coboClient, err := externalCobo.NewCoboClient(&externalCobo.Config{
		APISecret: cfg.APISecret,
		Env:       cfg.Env,
		Timeout:   cfg.Timeout,
	})
	if err != nil {
		return gerror.Wrap(err, "创建Cobo客户端失败")
	}

	confirmedCount := 0
	failedCount := 0
	updatedCount := 0

	for _, record := range pendingRecords {
		if record == nil || strings.TrimSpace(record.TxHash) == "" {
			continue
		}

		txInfo, err := coboClient.GetTransactionByHash(ctx, record.TxHash)
		if err != nil {
			g.Log().Warningf(ctx, "[Cobo Recharge Checker] 查询交易失败: recharge_id=%d, tx_hash=%s, err=%v", record.ID, record.TxHash, err)
			continue
		}
		if txInfo == nil {
			continue
		}

		confirmations := record.Confirmations
		if txInfo.ConfirmedNum != nil {
			confirmations = int(*txInfo.ConfirmedNum)
		}

		switch txInfo.Status {
		case coboWaas2.TRANSACTIONSTATUS_FAILED, coboWaas2.TRANSACTIONSTATUS_REJECTED:
			if err := s.rechargeRepo.UpdateStatus(ctx, record.ID, cobo.RechargeStatusFailed, confirmations); err != nil {
				g.Log().Warningf(ctx, "[Cobo Recharge Checker] 更新充值失败状态失败: recharge_id=%d, err=%v", record.ID, err)
				continue
			}
			failedCount++
			continue
		}

		if confirmations >= cfg.Recharge.MinConfirmations {
			if err := s.confirmRecharge(ctx, record.ID, record.UserID, record.Symbol, record.Amount, record.TxHash, confirmations); err != nil {
				g.Log().Warningf(ctx, "[Cobo Recharge Checker] 确认充值失败: recharge_id=%d, err=%v", record.ID, err)
				continue
			}
			confirmedCount++
			continue
		}

		if confirmations > record.Confirmations {
			if err := s.rechargeRepo.UpdateStatus(ctx, record.ID, cobo.RechargeStatusPending, confirmations); err != nil {
				g.Log().Warningf(ctx, "[Cobo Recharge Checker] 更新确认数失败: recharge_id=%d, err=%v", record.ID, err)
				continue
			}
			updatedCount++
		}
	}

	if confirmedCount > 0 || failedCount > 0 || updatedCount > 0 {
		g.Log().Infof(ctx,
			"[Cobo Recharge Checker] 执行完成: scanned=%d, confirmed=%d, failed=%d, updated=%d",
			len(pendingRecords), confirmedCount, failedCount, updatedCount,
		)
	}

	return nil
}

// getUserIDByCoboAddress 根据Cobo地址获取用户ID
func (s *webhookService) getUserIDByCoboAddress(ctx context.Context, coboAddress string) (int64, error) {
	// 查询 user_deposit_address 表
	var result struct {
		UserID int64 `json:"user_id"`
	}
	err := g.DB().Model("user_deposit_address").
		Ctx(ctx).
		Where("LOWER(address) = ?", coboAddress).
		Scan(&result)
	if err != nil {
		return 0, err
	}
	return result.UserID, nil
}

// VerifySignature 验证Webhook签名（Cobo使用ED25519签名）
func (s *webhookService) VerifySignature(ctx context.Context, body []byte, signature string, timestamp string) error {
	// 获取配置
	cfg, err := s.getConfig(ctx)
	if err != nil {
		return gerror.Wrap(err, "获取Cobo配置失败")
	}

	// 获取公钥
	var publicKeyHex string
	if cfg.Env == "prod" || cfg.Env == "production" {
		publicKeyHex = cfg.PubKeyProd
	} else {
		publicKeyHex = cfg.PubKeyDev
	}

	if publicKeyHex == "" {
		return gerror.Newf("Cobo %s 环境公钥未配置", cfg.Env)
	}

	publicKey, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return gerror.Wrap(err, "解码公钥失败")
	}

	// 构造验证消息: body|timestamp
	message := string(body) + "|" + timestamp

	// 验证签名
	if !utils.VerifyED25519Signature(publicKey, signature, message) {
		return gerror.New("签名验证失败")
	}

	return nil
}

// getConfig 获取配置
func (s *webhookService) getConfig(ctx context.Context) (*config.CoboConfig, error) {
	if s.coboConfig == nil {
		cfg, err := config.GetCoboConfig(ctx)
		if err != nil {
			return nil, err
		}
		s.coboConfig = cfg
	}
	return s.coboConfig, nil
}

// normalizeSymbol 标准化币种符号
func normalizeSymbol(asset string) string {
	// 处理 BSC_USDT -> USDT
	parts := strings.Split(asset, "_")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return asset
}

// HandleWithdraw 处理提现回调
func (s *webhookService) HandleWithdraw(ctx context.Context, payload *model.WebhookPayload) error {
	webhookResponse := buildWithdrawWebhookResponse(payload)

	// 1. Webhook ID幂等检查
	if payload.ID != "" {
		existing, err := s.withdrawRepo.GetByWebhookID(ctx, payload.ID)
		if err != nil {
			return gerror.Wrap(err, "查询提现webhook记录失败")
		}
		if existing != nil {
			g.Log().Infof(ctx, "[Cobo Webhook] 提现重复通知，已处理过: webhook_id=%s", payload.ID)
			return nil
		}
	}

	// 2. 获取请求ID（Cobo使用 request_id 关联我们的提现记录）
	requestID := payload.Data.RequestID
	if requestID == "" {
		g.Log().Warningf(ctx, "[Cobo Webhook] 提现回调缺少 request_id")
		return ErrWebhookDeny
	}

	// 3. 查找提现记录
	withdraw, err := s.withdrawRepo.GetByCoboRequestID(ctx, requestID)
	if err != nil {
		return gerror.Wrap(err, "查询提现记录失败")
	}
	if withdraw == nil {
		// fallback: direct_transfer_order 的 request_id 以 DT- 开头
		if strings.HasPrefix(requestID, "DT-") {
			return s.handleDirectTransferWebhook(ctx, payload, webhookResponse)
		}
		g.Log().Warningf(ctx, "[Cobo Webhook] 未找到对应提现记录，回调拒绝: request_id=%s", requestID)
		return ErrWebhookDeny
	}

	// 4. 状态幂等检查：如果已经是最终状态，仅更新webhook_id
	if withdraw.Status == cobo.WithdrawStatusSuccess || withdraw.Status == cobo.WithdrawStatusFailed || withdraw.Status == cobo.WithdrawStatusRejected {
		g.Log().Infof(ctx, "[Cobo Webhook] 提现已处理完成，忽略: request_id=%s, status=%d", requestID, withdraw.Status)
		updateData := g.Map{
			"updated_at":    time.Now(),
			"cobo_response": webhookResponse,
		}
		if payload.ID != "" {
			updateData["webhook_id"] = payload.ID
		}
		if _, err := g.DB().Model("cobo_withdraw_request").Ctx(ctx).Data(updateData).Where("id", withdraw.ID).Update(); err != nil {
			g.Log().Warningf(ctx, "[Cobo Webhook] 更新终态调试信息失败: id=%d, err=%v", withdraw.ID, err)
		}
		return nil
	}

	g.Log().Infof(ctx, "[Cobo Webhook] 处理提现回调: request_id=%s, tx_hash=%s, status=%s",
		requestID, payload.Data.TxHash, payload.Status)

	// 5. 根据状态处理
	switch strings.ToLower(payload.Status) {
	case "completed", "success", "succeeded":
		statusChanged := false
		err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			updateData := g.Map{
				"tx_hash":       payload.Data.TxHash,
				"status":        cobo.WithdrawStatusSuccess,
				"updated_at":    time.Now(),
				"cobo_response": webhookResponse,
			}
			if payload.ID != "" {
				updateData["webhook_id"] = payload.ID
			}

			result, err := tx.Model("cobo_withdraw_request").
				Ctx(ctx).
				Data(updateData).
				Where("id = ? AND status IN (?, ?, ?)", withdraw.ID, cobo.WithdrawStatusPending, cobo.WithdrawStatusApproved, cobo.WithdrawStatusProcessing).
				Update()
			if err != nil {
				return gerror.Wrap(err, "更新提现成功状态失败")
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				g.Log().Infof(ctx, "[Cobo Webhook] 提现成功重复通知，跳过资金处理: id=%d", withdraw.ID)
				return nil
			}
			statusChanged = true

			balanceBefore := decimal.Zero
			balanceRow, _ := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, withdraw.UserID, withdraw.Symbol)
			if balanceRow != nil {
				balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
			}

			// 提现成功时扣税（pool + YYAI）
			balanceBefore, taxErr := s.withdrawService.DeductWithdrawTaxOnSuccess(ctx, tx, withdraw, balanceBefore)
			if taxErr != nil {
				g.Log().Errorf(ctx, "[Cobo Webhook] 扣税失败: withdraw_id=%d, user_id=%d, err=%v", withdraw.ID, withdraw.UserID, taxErr)
				return gerror.Wrap(taxErr, "扣税失败")
			}

			// 扣除剩余冻结金额（税款已在 DeductWithdrawTaxOnSuccess 中从 frozen 扣减）
			frozenDeduct := withdraw.Amount.Sub(withdraw.TaxAmount).Sub(withdraw.TaxYyaiUsdtAmount)
			if frozenDeduct.LessThan(decimal.Zero) {
				return gerror.New("withdraw frozen deduct amount is negative")
			}
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, withdraw.UserID, withdraw.Symbol, decimal.Zero, frozenDeduct.Neg()); err != nil {
				g.Log().Errorf(ctx, "[Cobo Webhook] 扣除冻结余额失败: withdraw_id=%d, user_id=%d, err=%v", withdraw.ID, withdraw.UserID, err)
				return gerror.Wrap(err, "扣除冻结余额失败")
			}
			balanceAfter := balanceBefore.Sub(frozenDeduct)
			if err := writeBalanceChangeLogTx(ctx, tx, withdraw.UserID, withdraw.Symbol, consts.ChangeTypeWithdrawSuccess, frozenDeduct.Neg(), balanceBefore, balanceAfter, withdraw.OrderNo, withdraw.ID, "withdraw success deduct frozen", consts.OperatorTypeSystem); err != nil {
				return gerror.Wrap(err, "写入提现成功审计日志失败")
			}

			return nil
		})
		if err != nil {
			return err
		}
		if statusChanged {
			updatedWithdraw := *withdraw
			updatedWithdraw.Status = cobo.WithdrawStatusSuccess
			updatedWithdraw.TxHash = payload.Data.TxHash
			GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, &updatedWithdraw, withdraw.Status, "")
		}

		g.Log().Infof(ctx, "[Cobo Webhook] 提现成功: id=%d, tx_hash=%s", withdraw.ID, payload.Data.TxHash)

	case "failed", "rejected":
		statusChanged := false
		err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			updateData := g.Map{
				"tx_hash":       payload.Data.TxHash,
				"status":        cobo.WithdrawStatusFailed,
				"updated_at":    time.Now(),
				"cobo_response": webhookResponse,
			}
			if payload.ID != "" {
				updateData["webhook_id"] = payload.ID
			}

			result, err := tx.Model("cobo_withdraw_request").
				Ctx(ctx).
				Data(updateData).
				Where("id = ? AND status IN (?, ?, ?)", withdraw.ID, cobo.WithdrawStatusPending, cobo.WithdrawStatusApproved, cobo.WithdrawStatusProcessing).
				Update()
			if err != nil {
				return gerror.Wrap(err, "更新提现失败状态失败")
			}

			affected, _ := result.RowsAffected()
			if affected == 0 {
				g.Log().Infof(ctx, "[Cobo Webhook] 提现失败重复通知，跳过资金处理: id=%d", withdraw.ID)
				return nil
			}
			statusChanged = true

			balanceBefore := decimal.Zero
			balanceRow, _ := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, withdraw.UserID, withdraw.Symbol)
			if balanceRow != nil {
				balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
			}
			// 税款在成功时才扣减，失败时全部解冻
			refundAmount := withdraw.Amount
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, withdraw.UserID, withdraw.Symbol, refundAmount, refundAmount.Neg()); err != nil {
				g.Log().Errorf(ctx, "[Cobo Webhook] 解冻余额失败: withdraw_id=%d, user_id=%d, err=%v", withdraw.ID, withdraw.UserID, err)
				return gerror.Wrap(err, "解冻余额失败")
			}
			if err := writeBalanceChangeLogTx(ctx, tx, withdraw.UserID, withdraw.Symbol, consts.ChangeTypeWithdrawFailRefund, refundAmount, balanceBefore, balanceBefore, withdraw.OrderNo, withdraw.ID, "withdraw fail refund", consts.OperatorTypeSystem); err != nil {
				return gerror.Wrap(err, "写入提现失败退款审计日志失败")
			}

			return nil
		})
		if err != nil {
			return err
		}
		if statusChanged {
			updatedWithdraw := *withdraw
			updatedWithdraw.Status = cobo.WithdrawStatusFailed
			updatedWithdraw.TxHash = payload.Data.TxHash
			GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, &updatedWithdraw, withdraw.Status, payload.Status)
		}

		g.Log().Infof(ctx, "[Cobo Webhook] 提现失败，余额已解冻: id=%d, amount=%s", withdraw.ID, withdraw.Amount.String())

	case "broadcasting", "confirming", "pending", "pendingauthorization", "submitted", "pendingscreening", "pendingsignature":
		// 处理中状态，更新交易哈希（如果有）和webhook_id
		updateData := g.Map{
			"status":        cobo.WithdrawStatusProcessing,
			"updated_at":    time.Now(),
			"cobo_response": webhookResponse,
		}
		if payload.Data.TxHash != "" {
			updateData["tx_hash"] = payload.Data.TxHash
		}
		if payload.ID != "" {
			updateData["webhook_id"] = payload.ID
		}
		result, err := g.DB().Model("cobo_withdraw_request").
			Ctx(ctx).
			Data(updateData).
			Where("id = ? AND status IN (?, ?, ?)", withdraw.ID, cobo.WithdrawStatusPending, cobo.WithdrawStatusApproved, cobo.WithdrawStatusProcessing).
			Update()
		if err != nil {
			g.Log().Warningf(ctx, "[Cobo Webhook] 更新交易哈希失败: id=%d, err=%v", withdraw.ID, err)
		}
		affected := int64(0)
		if result != nil {
			affected, _ = result.RowsAffected()
		}
		if affected > 0 && withdraw.Status != cobo.WithdrawStatusProcessing {
			updatedWithdraw := *withdraw
			updatedWithdraw.Status = cobo.WithdrawStatusProcessing
			updatedWithdraw.TxHash = payload.Data.TxHash
			GetTelegramNotifyService(ctx).NotifyWithdrawStatusChanged(ctx, &updatedWithdraw, withdraw.Status, payload.Status)
		}
		g.Log().Infof(ctx, "[Cobo Webhook] 提现处理中: id=%d, status=%s", withdraw.ID, payload.Status)

	default:
		g.Log().Infof(ctx, "[Cobo Webhook] 未知状态，忽略: id=%d, status=%s", withdraw.ID, payload.Status)
	}

	return nil
}

// handleDirectTransferWebhook 处理 direct_transfer_order 的 Cobo 回调
func (s *webhookService) handleDirectTransferWebhook(ctx context.Context, payload *model.WebhookPayload, webhookResponse string) error {
	requestID := payload.Data.RequestID
	status := strings.ToLower(payload.Status)
	incomingSuccess := status == "completed" || status == "success" || status == "succeeded"
	incomingFailure := status == "failed" || status == "rejected"

	// 查找 direct_transfer_order
	var order DirectTransferOrder
	err := g.DB().Model("direct_transfer_order").Ctx(ctx).Where("request_id = ?", requestID).Scan(&order)
	if err != nil {
		return gerror.Wrap(err, "查询 direct_transfer_order 失败")
	}
	if order.ID == 0 {
		g.Log().Warningf(ctx, "[Cobo Webhook] 未找到对应 direct_transfer_order，回调拒绝: request_id=%s", requestID)
		return ErrWebhookDeny
	}

	// 已经是终态，忽略；本地提交失败但 Cobo 后续终态回调允许补正。
	if order.Status == DirectTransferStatusSuccess || (order.Status == DirectTransferStatusFailed && !((incomingSuccess || incomingFailure) && strings.TrimSpace(order.CoboStatus) == "")) {
		g.Log().Infof(ctx, "[Cobo Webhook] direct_transfer 已处理完成，忽略: request_id=%s, status=%s", requestID, order.Status)
		return nil
	}

	g.Log().Infof(ctx, "[Cobo Webhook] 处理 direct_transfer 回调: request_id=%s, tx_hash=%s, status=%s",
		requestID, payload.Data.TxHash, payload.Status)

	switch status {
	case "completed", "success", "succeeded":
		result, err := g.DB().Model("direct_transfer_order").Ctx(ctx).Data(g.Map{
			"status":      DirectTransferStatusSuccess,
			"cobo_status": payload.Status,
			"tx_hash":     payload.Data.TxHash,
			"updated_at":  time.Now(),
		}).Where("id = ? AND (status IN (?, ?, ?) OR (status = ? AND cobo_status = ''))", order.ID, DirectTransferStatusPending, DirectTransferStatusSubmitting, DirectTransferStatusSubmitted, DirectTransferStatusFailed).Update()
		if err != nil {
			return gerror.Wrap(err, "更新 direct_transfer 成功状态失败")
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			g.Log().Infof(ctx, "[Cobo Webhook] direct_transfer 成功重复通知，跳过状态更新: id=%d", order.ID)
			return nil
		}
		g.Log().Infof(ctx, "[Cobo Webhook] direct_transfer 成功: id=%d, tx_hash=%s", order.ID, payload.Data.TxHash)

	case "failed", "rejected":
		result, err := g.DB().Model("direct_transfer_order").Ctx(ctx).Data(g.Map{
			"status":      DirectTransferStatusFailed,
			"cobo_status": payload.Status,
			"tx_hash":     payload.Data.TxHash,
			"updated_at":  time.Now(),
		}).Where("id = ? AND (status IN (?, ?, ?) OR (status = ? AND cobo_status = ''))", order.ID, DirectTransferStatusPending, DirectTransferStatusSubmitting, DirectTransferStatusSubmitted, DirectTransferStatusFailed).Update()
		if err != nil {
			return gerror.Wrap(err, "更新 direct_transfer 失败状态失败")
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			g.Log().Infof(ctx, "[Cobo Webhook] direct_transfer 失败重复通知，跳过状态更新: id=%d", order.ID)
			return nil
		}
		g.Log().Infof(ctx, "[Cobo Webhook] direct_transfer 失败: id=%d, status=%s", order.ID, payload.Status)

	case "broadcasting", "confirming", "pending", "pendingauthorization", "submitted", "pendingscreening", "pendingsignature":
		updateData := g.Map{
			"cobo_status": payload.Status,
			"updated_at":  time.Now(),
		}
		if strings.TrimSpace(payload.Data.TxHash) != "" {
			updateData["tx_hash"] = payload.Data.TxHash
		}
		result, err := g.DB().Model("direct_transfer_order").Ctx(ctx).Data(updateData).Where("id = ? AND status IN (?, ?, ?)", order.ID, DirectTransferStatusPending, DirectTransferStatusSubmitting, DirectTransferStatusSubmitted).Update()
		if err != nil {
			g.Log().Warningf(ctx, "[Cobo Webhook] 更新 direct_transfer 处理中状态失败: id=%d, err=%v", order.ID, err)
			return nil
		}
		affected, _ := result.RowsAffected()
		if affected == 0 {
			g.Log().Infof(ctx, "[Cobo Webhook] direct_transfer 处理中重复通知，跳过状态更新: id=%d", order.ID)
			return nil
		}
		g.Log().Infof(ctx, "[Cobo Webhook] direct_transfer 处理中: id=%d, status=%s", order.ID, payload.Status)

	default:
		g.Log().Infof(ctx, "[Cobo Webhook] direct_transfer 未知状态，忽略: id=%d, status=%s", order.ID, payload.Status)
	}

	return nil
}

func buildWithdrawWebhookResponse(payload *model.WebhookPayload) string {
	if payload == nil {
		return "{}"
	}

	result, _ := json.Marshal(g.Map{
		"event_id":   payload.ID,
		"event_type": payload.Type,
		"status":     payload.Status,
		"request_id": payload.Data.RequestID,
		"tx_hash":    payload.Data.TxHash,
		"data":       payload.Data,
		"timestamp":  payload.Timestamp,
	})

	return string(result)
}

// isDuplicateKeyError 检查错误是否为数据库重复键/唯一约束冲突
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "duplicate key") ||
		strings.Contains(errStr, "unique constraint") ||
		strings.Contains(errStr, "unique violation")
}

// checkDepositWhitelist 用 Cobo asset 字段查 token_config 充币白名单。
// 命中返回 token_config.symbol，未命中返回 ""。
func (s *webhookService) checkDepositWhitelist(ctx context.Context, asset string) (string, error) {
	if asset == "" {
		return "", nil
	}
	v, err := g.DB().Model("token_config").Ctx(ctx).
		Fields("symbol").
		Where("cobo_asset_code = ? AND is_enabled = ?", asset, true).
		Value()
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	return v.String(), nil
}
