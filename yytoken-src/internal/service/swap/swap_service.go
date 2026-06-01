package swap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/blockchain"
	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	repo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/service/swap/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

var errSwapIdempotentConflict = errors.New("swap idempotent conflict")

// ISwapService 兑换服务接口
type ISwapService interface {
	Quote(ctx context.Context, pair, side, amount string) (*model.SwapQuote, error)
	SwapYYToUSDT(ctx context.Context, req *model.SwapYYToUSDTReq) (*model.SwapYYToUSDTRes, error)
	GetSwapRecords(ctx context.Context, req *model.GetSwapRecordsReq) (*model.GetSwapRecordsRes, error)
	GetYYSwapFee(ctx context.Context) decimal.Decimal
}

type swapService struct {
	balanceRepo   repo.IBalanceRepository
	configDao     dao.IConfigDao
	stockPriceDao dao.IStockPriceDao
	swapRecordDao dao.ISwapRecordDao
	userRepo      repository.IUserRepository
}

var swapServiceInstance *swapService

func Svc() ISwapService {
	if swapServiceInstance == nil {
		swapServiceInstance = &swapService{
			balanceRepo:   repo.NewBalanceRepository(),
			configDao:     dao.NewConfigDao(),
			stockPriceDao: dao.NewStockPriceDao(),
			swapRecordDao: dao.NewSwapRecordDao(),
			userRepo:      repository.NewUserRepository(),
		}
	}
	return swapServiceInstance
}

func (s *swapService) Quote(ctx context.Context, pair, side, amount string) (*model.SwapQuote, error) {
	inputAmount, _ := decimal.NewFromString(amount)

	return &model.SwapQuote{
		AmountOut: utils.FormatDecimal(inputAmount.Mul(decimal.NewFromFloat(0.98))),
		Price:     "56.4000",
		Slippage:  "0.01",
		Fee:       utils.FormatDecimal(inputAmount.Mul(decimal.NewFromFloat(0.01))),
	}, nil
}

const swapLockTTLSeconds = 30

// SwapYYToUSDT 将YY兑换为USDT
func (s *swapService) SwapYYToUSDT(ctx context.Context, req *model.SwapYYToUSDTReq) (*model.SwapYYToUSDTRes, error) {
	if req.UserID <= 0 {
		return nil, gerror.New("invalid user id")
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return nil, gerror.New("invalid amount")
	}
	minSwapAmount := s.getMinYYSwapAmount(ctx)
	if amount.LessThan(minSwapAmount) {
		return nil, gerror.Newf("amount must be at least %s YY", minSwapAmount.String())
	}

	requestID := strings.TrimSpace(req.RequestID)
	if len(requestID) > 64 {
		return nil, gerror.New("request_id too long")
	}

	// 检查用户是否允许兑换
	user, err := s.userRepo.GetUserById(ctx, req.UserID)
	if err != nil {
		return nil, gerror.Wrap(err, "get user failed")
	}
	if user != nil && !user.CanSwap {
		return nil, gerror.New("swap not allowed")
	}

	// 1. 幂等检查
	if requestID != "" {
		existing, err := s.swapRecordDao.GetByUserAndRequestID(ctx, req.UserID, requestID)
		if err != nil {
			return nil, gerror.Wrap(err, "query swap record failed")
		}
		if existing != nil {
			g.Log().Infof(ctx, "[SwapYYToUSDT] idempotent hit: user_id=%d, request_id=%s", req.UserID, requestID)
			return s.buildSwapRes(ctx, existing)
		}
	}

	// 2. 获取YY最新价格
	priceEntity, err := s.stockPriceDao.GetLatestBySymbol(ctx, "YY")
	if err != nil {
		return nil, gerror.Wrap(err, "get YY price failed")
	}
	if priceEntity == nil || priceEntity.Price.LessThanOrEqual(decimal.Zero) {
		return nil, gerror.New("YY price unavailable")
	}

	// 3. 计算USDT数量，并扣除手续费
	usdtAmount := amount.Mul(priceEntity.Price)
	fee := s.GetYYSwapFee(ctx)
	if fee.GreaterThanOrEqual(usdtAmount) {
		return nil, gerror.New("swap amount too small to cover fee")
	}
	usdtAmountAfterFee := usdtAmount.Sub(fee)

	// 4. 获取手续费收款地址
	var feeRecipientUserID int64
	if fee.GreaterThan(decimal.Zero) {
		feeAddrConfig, _ := s.configDao.GetByKeyName(ctx, consts.YYSwapFeeAddrConfigKey)
		if feeAddrConfig != nil && strings.TrimSpace(feeAddrConfig.KeyValue) != "" {
			feeAddr := strings.ToLower(strings.TrimSpace(feeAddrConfig.KeyValue))
			feeUser, _ := s.userRepo.GetUserByWalletAddress(ctx, feeAddr)
			if feeUser != nil {
				feeRecipientUserID = feeUser.Id
			}
		}
	}

	// 5. 检查YY余额
	balance, err := s.balanceRepo.GetByUserID(ctx, req.UserID, "YY")
	if err != nil {
		return nil, gerror.Wrap(err, "get balance failed")
	}
	if balance == nil || !balance.HasEnoughBalance(amount) {
		return nil, gerror.New("insufficient YY balance")
	}

	// 5. 获取Redis分布式锁
	lockKey := fmt.Sprintf("swap:yy_to_usdt:lock:%d", req.UserID)
	lockToken := utils.GenerateSnowflakeId()
	locked, err := s.acquireRedisLock(ctx, lockKey, lockToken, swapLockTTLSeconds)
	if err != nil {
		return nil, gerror.Wrap(err, "acquire lock failed")
	}
	if !locked {
		return nil, gerror.New("operation in progress, please try again later")
	}
	defer s.releaseRedisLock(ctx, lockKey, lockToken)

	// 6. 事务执行兑换（使用FOR UPDATE锁定余额）
	var record *entity.SwapRecordEntity
	var yyAfterAvail decimal.Decimal
	var usdtAfterAvail decimal.Decimal
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 5.1 创建兑换记录
		record = &entity.SwapRecordEntity{
			UserID:     req.UserID,
			FromSymbol: "YY",
			ToSymbol:   "USDT",
			FromAmount: amount,
			ToAmount:   usdtAmountAfterFee,
			Fee:        fee,
			Price:      priceEntity.Price,
			RequestID:  requestID,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := s.swapRecordDao.Create(ctx, tx, record); err != nil {
			if requestID != "" && strings.Contains(strings.ToLower(err.Error()), "swap_record_user_id_request_id_key") {
				return errSwapIdempotentConflict
			}
			return gerror.Wrap(err, "create swap record failed")
		}

		// 5.2 扣除YY余额
		yyBefore, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, "YY")
		if err != nil {
			return gerror.Wrap(err, "get YY balance for update failed")
		}
		yyBeforeAvail := decimal.Zero
		if yyBefore != nil {
			yyBeforeAvail = yyBefore.AvailableAmount
		}
		if yyBeforeAvail.LessThan(amount) {
			return gerror.New("insufficient YY balance")
		}
		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, req.UserID, "YY", amount.Neg(), decimal.Zero); err != nil {
			return gerror.Wrap(err, "deduct YY balance failed")
		}
		yyAfter, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, "YY")
		if err != nil {
			return gerror.Wrap(err, "get YY balance after update failed")
		}
		yyAfterAvail = decimal.Zero
		if yyAfter != nil {
			yyAfterAvail = yyAfter.AvailableAmount
		}

		// 5.3 增加USDT余额
		usdtBefore, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, "USDT")
		if err != nil {
			return gerror.Wrap(err, "get USDT balance for update failed")
		}
		usdtBeforeAvail := decimal.Zero
		if usdtBefore != nil {
			usdtBeforeAvail = usdtBefore.AvailableAmount
		}
		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, req.UserID, "USDT", usdtAmountAfterFee, decimal.Zero); err != nil {
			return gerror.Wrap(err, "add USDT balance failed")
		}
		usdtAfter, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, "USDT")
		if err != nil {
			return gerror.Wrap(err, "get USDT balance after update failed")
		}
		usdtAfterAvail = decimal.Zero
		if usdtAfter != nil {
			usdtAfterAvail = usdtAfter.AvailableAmount
		}

		// 5.4 写入balance change log
		orderNo := fmt.Sprintf("SWAP-%d-%s", record.Id, utils.GenerateSnowflakeId())
		if err := s.createBalanceChangeLogTx(ctx, tx, req.UserID, "YY", consts.ChangeTypeExchangeOut, amount.Neg(), yyBeforeAvail, yyAfterAvail, orderNo, record.Id, "swap YY to USDT"); err != nil {
			return gerror.Wrap(err, "write YY exchange_out log failed")
		}
		if err := s.createBalanceChangeLogTx(ctx, tx, req.UserID, "USDT", consts.ChangeTypeExchangeIn, usdtAmountAfterFee, usdtBeforeAvail, usdtAfterAvail, orderNo, record.Id, "swap YY to USDT"); err != nil {
			return gerror.Wrap(err, "write USDT exchange_in log failed")
		}

		// 5.5 手续费转入收款账户
		if fee.GreaterThan(decimal.Zero) && feeRecipientUserID > 0 {
			feeUsdtBefore, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, feeRecipientUserID, "USDT")
			if err != nil {
				return gerror.Wrap(err, "get fee recipient USDT balance for update failed")
			}
			feeUsdtBeforeAvail := decimal.Zero
			if feeUsdtBefore != nil {
				feeUsdtBeforeAvail = feeUsdtBefore.AvailableAmount
			}
			if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, feeRecipientUserID, "USDT", fee, decimal.Zero); err != nil {
				return gerror.Wrap(err, "add fee USDT to recipient failed")
			}
			feeUsdtAfter, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, feeRecipientUserID, "USDT")
			if err != nil {
				return gerror.Wrap(err, "get fee recipient USDT balance after update failed")
			}
			feeUsdtAfterAvail := decimal.Zero
			if feeUsdtAfter != nil {
				feeUsdtAfterAvail = feeUsdtAfter.AvailableAmount
			}
			if err := s.createBalanceChangeLogTx(ctx, tx, feeRecipientUserID, "USDT", consts.ChangeTypeFee, fee, feeUsdtBeforeAvail, feeUsdtAfterAvail, orderNo, record.Id, "swap YY to USDT fee"); err != nil {
				return gerror.Wrap(err, "write fee recipient balance change log failed")
			}
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, errSwapIdempotentConflict) {
			existing, getErr := s.swapRecordDao.GetByUserAndRequestID(ctx, req.UserID, requestID)
			if getErr != nil {
				return nil, gerror.Wrap(getErr, "read idempotent record failed")
			}
			if existing == nil {
				return nil, gerror.New("idempotent record not found")
			}
			return s.buildSwapRes(ctx, existing)
		}
		return nil, err
	}

	// 异步 burn YY（链上销毁对应数量）
	go func(recordID int64, burnAmount decimal.Decimal) {
		burnCtx := context.Background()
		burnSvc, err := blockchain.NewTokenStoreBurn()
		if err != nil {
			g.Log().Errorf(burnCtx, "[Swap] NewTokenStoreBurn failed: user_id=%d, err=%v", req.UserID, err)
			return
		}
		chainAmount := burnAmount.Shift(consts.BlockchainDefaultDecimals).BigInt()
		txHash, err := burnSvc.BurnYY(burnCtx, chainAmount)
		if err != nil {
			g.Log().Errorf(burnCtx, "[Swap] burnYY failed: user_id=%d, amount=%s, err=%v", req.UserID, burnAmount.String(), err)
			return
		}
		g.Log().Infof(burnCtx, "[Swap] burnYY success: user_id=%d, amount=%s, txHash=%s", req.UserID, burnAmount.String(), txHash)
		if updateErr := s.swapRecordDao.UpdateBurnTxHash(burnCtx, recordID, txHash); updateErr != nil {
			g.Log().Warningf(burnCtx, "[Swap] update burn_tx_hash failed: record_id=%d, txHash=%s, err=%v", recordID, txHash, updateErr)
		}
	}(record.Id, amount)

	return s.buildSwapResWithSnapshot(record, yyAfterAvail, usdtAfterAvail), nil
}

func (s *swapService) getMinYYSwapAmount(ctx context.Context) decimal.Decimal {
	minAmount, err := decimal.NewFromString(consts.YYSwapMinTradeAmountDefault)
	if err != nil || !minAmount.GreaterThan(decimal.Zero) {
		minAmount = decimal.NewFromInt(15)
	}

	configEntity, err := s.configDao.GetByKeyName(ctx, consts.YYSwapMinTradeAmountConfigKey)
	if err != nil || configEntity == nil {
		return minAmount
	}

	configAmount, parseErr := decimal.NewFromString(strings.TrimSpace(configEntity.KeyValue))
	if parseErr != nil || !configAmount.GreaterThan(decimal.Zero) {
		return minAmount
	}

	return configAmount
}

func (s *swapService) GetYYSwapFee(ctx context.Context) decimal.Decimal {
	fee, err := decimal.NewFromString(consts.YYSwapFeeUSDTDefault)
	if err != nil || fee.LessThan(decimal.Zero) {
		fee = decimal.NewFromInt(1)
	}

	configEntity, err := s.configDao.GetByKeyName(ctx, consts.YYSwapFeeUSDTConfigKey)
	if err != nil || configEntity == nil {
		return fee
	}

	configFee, parseErr := decimal.NewFromString(strings.TrimSpace(configEntity.KeyValue))
	if parseErr != nil || configFee.LessThan(decimal.Zero) {
		return fee
	}

	return configFee
}

func (s *swapService) buildSwapRes(ctx context.Context, record *entity.SwapRecordEntity) (*model.SwapYYToUSDTRes, error) {
	// 查询最新余额
	yyBalance, _ := s.balanceRepo.GetByUserID(ctx, record.UserID, "YY")
	usdtBalance, _ := s.balanceRepo.GetByUserID(ctx, record.UserID, "USDT")

	yyLeft := decimal.Zero
	if yyBalance != nil {
		yyLeft = yyBalance.AvailableAmount
	}
	usdtLeft := decimal.Zero
	if usdtBalance != nil {
		usdtLeft = usdtBalance.AvailableAmount
	}

	return s.buildSwapResWithSnapshot(record, yyLeft, usdtLeft), nil
}

func (s *swapService) buildSwapResWithSnapshot(record *entity.SwapRecordEntity, yyLeft, usdtLeft decimal.Decimal) *model.SwapYYToUSDTRes {
	return &model.SwapYYToUSDTRes{
		USDTAmount: record.ToAmount.String(),
		YYPrice:    record.Price.String(),
		Fee:        record.Fee.String(),
		YYLeft:     yyLeft.String(),
		USDTLeft:   usdtLeft.String(),
		CreatedAt:  record.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func (s *swapService) GetSwapRecords(ctx context.Context, req *model.GetSwapRecordsReq) (*model.GetSwapRecordsRes, error) {
	if req.UserID <= 0 {
		return nil, gerror.New("invalid user id")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	m := g.DB().Model("swap_record").Ctx(ctx).Where("user_id", req.UserID)

	total, err := m.Count()
	if err != nil {
		return nil, gerror.Wrap(err, "query swap records total count failed")
	}

	var records []*entity.SwapRecordEntity
	err = m.OrderDesc("created_at").OrderDesc("id").Page(req.Page, req.PageSize).Scan(&records)
	if err != nil {
		return nil, gerror.Wrap(err, "query swap records failed")
	}

	list := make([]*model.SwapRecordItem, 0, len(records))
	for _, r := range records {
		list = append(list, &model.SwapRecordItem{
			ID:         r.Id,
			FromSymbol: r.FromSymbol,
			FromAmount: r.FromAmount.String(),
			ToSymbol:   r.ToSymbol,
			ToAmount:   r.ToAmount.String(),
			Fee:        r.Fee.String(),
			Price:      r.Price.String(),
			BurnTxHash: r.BurnTxHash,
			CreatedAt:  r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &model.GetSwapRecordsRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (s *swapService) createBalanceChangeLogTx(ctx context.Context, tx gdb.TX, userID int64, symbol, changeType string, amount, beforeBalance, afterBalance decimal.Decimal, relatedOrderNo string, relatedID int64, remark string) error {
	_, err := tx.Exec(`
		INSERT INTO cobo_balance_change_log (user_id, symbol, change_type, amount, before_balance, after_balance, related_order_no, related_id, remark, operator_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, symbol, changeType, amount, beforeBalance, afterBalance, relatedOrderNo, relatedID, remark, consts.OperatorTypeUser, time.Now())
	if err != nil {
		g.Log().Errorf(ctx, "[Swap] write balance change log failed: user_id=%d, change_type=%s, err=%v", userID, changeType, err)
	}
	return err
}

func (s *swapService) acquireRedisLock(ctx context.Context, key, token string, ttlSeconds int) (bool, error) {
	redisClient := g.Redis()
	if redisClient == nil {
		return false, gerror.New("redis not initialized")
	}
	result, err := redisClient.Do(ctx, "SET", key, token, "NX", "EX", ttlSeconds)
	if err != nil {
		return false, gerror.Wrap(err, "acquire distributed lock failed")
	}
	if result.IsNil() {
		return false, nil
	}
	return result.String() == "OK", nil
}

func (s *swapService) releaseRedisLock(ctx context.Context, key, token string) {
	redisClient := g.Redis()
	if redisClient == nil {
		return
	}
	_, err := redisClient.Do(ctx, "EVAL", `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end return 0`, 1, key, token)
	if err != nil {
		g.Log().Warningf(ctx, "[Swap] release distributed lock failed: key=%s err=%v", key, err)
	}
}
