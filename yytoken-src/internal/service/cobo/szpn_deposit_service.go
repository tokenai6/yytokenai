package cobo

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	entityCobo "XWFrame/internal/entity/cobo"
	"XWFrame/internal/frame/consts"
	coboRepo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/repository"
	"XWFrame/internal/service/cobo/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// SZPNDepositService SZPN充值回调服务
type SZPNDepositService interface {
	HandleDepositCallback(ctx context.Context, req *model.SZPNDepositCallbackReq) (*model.SZPNDepositCallbackRes, error)
	VerifyCallbackSecret(ctx context.Context, secret string) error
}

type szpnDepositService struct {
	configRepo      repository.IConfigRepository
	userRepo        repository.IUserRepository
	balanceRepo     coboRepo.IBalanceRepository
	tokenConfigRepo repository.ITokenConfigRepository
}

// NewSZPNDepositService 创建服务
func NewSZPNDepositService() SZPNDepositService {
	return &szpnDepositService{
		configRepo:      repository.NewConfigRepository(),
		userRepo:        repository.NewUserRepository(),
		balanceRepo:     coboRepo.NewBalanceRepository(),
		tokenConfigRepo: repository.NewTokenConfigRepository(),
	}
}

func (s *szpnDepositService) VerifyCallbackSecret(ctx context.Context, secret string) error {
	configEntity, err := s.configRepo.GetByKeyName(ctx, consts.SZPNMonitorCallbackSecretConfigKey)
	if err != nil {
		return gerror.Wrap(err, "读取SZPN回调密钥配置失败")
	}
	if configEntity == nil || strings.TrimSpace(configEntity.KeyValue) == "" {
		return gerror.New("SZPN回调密钥未配置")
	}
	if strings.TrimSpace(secret) != strings.TrimSpace(configEntity.KeyValue) {
		return gerror.New("回调鉴权失败")
	}
	return nil
}

func (s *szpnDepositService) HandleDepositCallback(ctx context.Context, req *model.SZPNDepositCallbackReq) (*model.SZPNDepositCallbackRes, error) {
	if req == nil {
		return nil, gerror.New("请求不能为空")
	}

	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		symbol = "SZPN"
	}
	// 校验 symbol 是否在 token_config 中且已启用
	tokenCfg, err := s.tokenConfigRepo.GetBySymbol(ctx, symbol)
	if err != nil {
		return nil, gerror.Wrap(err, "查询代币配置失败")
	}
	if tokenCfg == nil {
		return &model.SZPNDepositCallbackRes{Accepted: false, Processed: false, Reason: "unsupported symbol"}, nil
	}

	toAddress := strings.ToLower(strings.TrimSpace(req.ToAddress))
	fromAddress := strings.ToLower(strings.TrimSpace(req.FromAddress))
	txHash := strings.ToLower(strings.TrimSpace(req.TxHash))
	if !utils.IsValidEthereumAddress(toAddress) || !utils.IsValidEthereumAddress(fromAddress) || txHash == "" {
		return nil, gerror.New("回调参数不合法")
	}

	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil || !amount.IsPositive() {
		return nil, gerror.New("amount不合法")
	}

	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, fromAddress)
	if err != nil {
		return nil, gerror.Wrap(err, "查询充值用户失败")
	}
	if userEntity == nil {
		return &model.SZPNDepositCallbackRes{Accepted: true, Processed: false, Reason: "unmatched from_address"}, nil
	}

	payloadJSON, _ := json.Marshal(req)
	now := time.Now()

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		count, err := tx.Model("cobo_recharge_record").Ctx(ctx).
			Where("symbol = ? AND tx_hash = ? AND log_index = ?", symbol, txHash, req.LogIndex).
			Count()
		if err != nil {
			return err
		}
		if count > 0 {
			return nil
		}

		recharge := g.Map{
			"user_id":        userEntity.Id,
			"cobo_address":   toAddress,
			"tx_hash":        txHash,
			"from_address":   fromAddress,
			"symbol":         symbol,
			"amount":         amount,
			"status":         1,
			"confirmations":  req.Confirmations,
			"webhook_id":     strings.TrimSpace(req.EventID),
			"webhook_data":   string(payloadJSON),
			"chain_id":       strings.TrimSpace(req.ChainID),
			"log_index":      req.LogIndex,
			"confirmed_at":   now,
			"created_at":     now,
			"updated_at":     now,
		}
		result, err := tx.Model("cobo_recharge_record").Ctx(ctx).Data(recharge).Insert()
		if err != nil {
			if isDuplicateKeyError(err) {
				return nil
			}
			return err
		}

		rechargeID, _ := result.LastInsertId()

		balance, err := s.balanceRepo.GetByUserID(ctx, userEntity.Id, symbol)
		if err != nil {
			return err
		}
		beforeTotal := decimal.Zero
		if balance != nil {
			beforeTotal = balance.AvailableAmount.Add(balance.FrozenAmount)
		}

		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, userEntity.Id, symbol, amount, decimal.Zero); err != nil {
			return err
		}
		afterTotal := beforeTotal.Add(amount)

		if err := writeBalanceChangeLogTx(ctx, tx, userEntity.Id, symbol, consts.ChangeTypeRecharge, amount, beforeTotal, afterTotal, txHash, rechargeID, symbol+" monitor deposit confirmed", consts.OperatorTypeSystem); err != nil {
			return gerror.Wrap(err, "写入充值审计日志失败")
		}

		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "处理SZPN充值回调失败")
	}

	go func(uid int64, amt decimal.Decimal, sym, hash string) {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		totalVar, qErr := g.DB().Model("cobo_recharge_record").Ctx(notifyCtx).
			Fields("COALESCE(SUM(amount), 0)").
			Where("user_id = ? AND symbol = ? AND status = ?", uid, sym, entityCobo.RechargeStatusConfirmed).
			Value()
		totalAmount := decimal.Zero
		if qErr == nil && totalVar != nil {
			totalAmount, _ = decimal.NewFromString(totalVar.String())
		}

		source := "内部渠道"
		if sym == "USDT" {
			source = "内部渠道(USDT)"
		}
		GetTelegramNotifyService(notifyCtx).NotifyRechargeConfirmedWithSource(notifyCtx, uid, amt, totalAmount, sym, hash, source)
	}(userEntity.Id, amount, symbol, txHash)

	return &model.SZPNDepositCallbackRes{Accepted: true, Processed: true}, nil
}
