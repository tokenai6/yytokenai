package transfer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	repo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/service/transfer/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

var errTransferIdempotentConflict = errors.New("transfer idempotent conflict")

const transferLockTTLSeconds = 30

var allowedTransferSymbols = map[string]struct{}{
	"YY": {},
}

// ITransferService 内部转账服务接口
type ITransferService interface {
	InternalTransfer(ctx context.Context, req *model.InternalTransferReq) (*model.InternalTransferRes, error)
	GetTransferRecords(ctx context.Context, req *model.GetTransferRecordsReq) (*model.GetTransferRecordsRes, error)
}

type transferService struct {
	balanceRepo       repo.IBalanceRepository
	transferRecordDao dao.ITransferRecordDao
	userRepo          repository.IUserRepository
	passwordRepo     repository.IUserPasswordRepository
}

var transferServiceInstance *transferService

func Svc() ITransferService {
	if transferServiceInstance == nil {
		transferServiceInstance = &transferService{
			balanceRepo:       repo.NewBalanceRepository(),
			transferRecordDao: dao.NewTransferRecordDao(),
			userRepo:          repository.NewUserRepository(),
			passwordRepo:     repository.NewUserPasswordRepository(),
		}
	}
	return transferServiceInstance
}

// InternalTransfer 执行内部转账
func (s *transferService) InternalTransfer(ctx context.Context, req *model.InternalTransferReq) (*model.InternalTransferRes, error) {
	// 验证密码（如果用户已设置密码则必填）
	if err := repository.VerifyUserPassword(ctx, s.passwordRepo, req.FromUserID, req.Password); err != nil {
		return nil, err
	}

	if req.FromUserID <= 0 {
		return nil, gerror.New("invalid from user id")
	}

	toWalletAddress := strings.ToLower(strings.TrimSpace(req.ToWalletAddress))
	if toWalletAddress == "" {
		return nil, gerror.New("to_wallet_address is required")
	}

	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		return nil, gerror.New("symbol is required")
	}
	if _, ok := allowedTransferSymbols[symbol]; !ok {
		return nil, gerror.New("unsupported symbol")
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || amount.LessThanOrEqual(decimal.Zero) {
		return nil, gerror.New("invalid amount")
	}

	requestID := strings.TrimSpace(req.RequestID)
	if len(requestID) > 64 {
		return nil, gerror.New("request_id too long")
	}
	if requestID == "" {
		requestID = utils.GenerateSnowflakeId()
	}

	// 1. 禁止向自己转账
	fromUser, err := s.userRepo.GetUserById(ctx, req.FromUserID)
	if err != nil {
		return nil, gerror.Wrap(err, "get from user failed")
	}
	if fromUser == nil {
		return nil, gerror.New("from user not found")
	}
	if !fromUser.CanTransfer {
		return nil, gerror.New("transfer not allowed")
	}
	if strings.ToLower(fromUser.WalletAddress) == toWalletAddress {
		return nil, gerror.New("cannot transfer to yourself")
	}

	// 2. 查找目标用户
	toUser, err := s.userRepo.GetUserByWalletAddress(ctx, toWalletAddress)
	if err != nil {
		return nil, gerror.Wrap(err, "get to user failed")
	}
	if toUser == nil {
		return nil, gerror.New("target user not found")
	}

	// 3. 幂等检查
	if requestID != "" {
		existing, err := s.transferRecordDao.GetByFromUserAndRequestID(ctx, req.FromUserID, requestID)
		if err != nil {
			return nil, gerror.Wrap(err, "query transfer record failed")
		}
		if existing != nil {
			g.Log().Infof(ctx, "[InternalTransfer] idempotent hit: from_user_id=%d, request_id=%s", req.FromUserID, requestID)
			return s.buildTransferRes(ctx, existing)
		}
	}

	// 4. 检查转出方余额
	balance, err := s.balanceRepo.GetByUserID(ctx, req.FromUserID, symbol)
	if err != nil {
		return nil, gerror.Wrap(err, "get balance failed")
	}
	if balance == nil || !balance.HasEnoughBalance(amount) {
		return nil, gerror.New("insufficient balance")
	}

	// 5. 获取Redis分布式锁
	lockKey := fmt.Sprintf("transfer:internal:lock:%d", req.FromUserID)
	lockToken := utils.GenerateSnowflakeId()
	locked, err := s.acquireRedisLock(ctx, lockKey, lockToken, transferLockTTLSeconds)
	if err != nil {
		return nil, gerror.Wrap(err, "acquire lock failed")
	}
	if !locked {
		return nil, gerror.New("operation in progress, please try again later")
	}
	defer s.releaseRedisLock(ctx, lockKey, lockToken)

	// 6. 事务执行转账（使用FOR UPDATE锁定余额）
	var record *entity.InternalTransferRecordEntity
	var fromAfterAvail decimal.Decimal
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 5.1 创建转账记录
		record = &entity.InternalTransferRecordEntity{
			FromUserID: req.FromUserID,
			ToUserID:   toUser.Id,
			Symbol:     symbol,
			Amount:     amount,
			RequestID:  requestID,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		if err := s.transferRecordDao.Create(ctx, tx, record); err != nil {
			if requestID != "" && strings.Contains(strings.ToLower(err.Error()), "internal_transfer_record_from_user_id_request_id_key") {
				return errTransferIdempotentConflict
			}
			return gerror.Wrap(err, "create transfer record failed")
		}

		// 5.2 扣除转出方余额
		fromBefore, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.FromUserID, symbol)
		if err != nil {
			return gerror.Wrap(err, "get from balance for update failed")
		}
		fromBeforeAvail := decimal.Zero
		if fromBefore != nil {
			fromBeforeAvail = fromBefore.AvailableAmount
		}
		if fromBeforeAvail.LessThan(amount) {
			return gerror.New("insufficient balance")
		}
		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, req.FromUserID, symbol, amount.Neg(), decimal.Zero); err != nil {
			return gerror.Wrap(err, "deduct from balance failed")
		}
		fromAfter, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.FromUserID, symbol)
		if err != nil {
			return gerror.Wrap(err, "get from balance after update failed")
		}
		fromAfterAvail = decimal.Zero
		if fromAfter != nil {
			fromAfterAvail = fromAfter.AvailableAmount
		}

		// 5.3 增加接收方余额
		toBefore, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, toUser.Id, symbol)
		if err != nil {
			return gerror.Wrap(err, "get to balance for update failed")
		}
		toBeforeAvail := decimal.Zero
		if toBefore != nil {
			toBeforeAvail = toBefore.AvailableAmount
		}
		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, toUser.Id, symbol, amount, decimal.Zero); err != nil {
			return gerror.Wrap(err, "add to balance failed")
		}
		toAfter, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, toUser.Id, symbol)
		if err != nil {
			return gerror.Wrap(err, "get to balance after update failed")
		}
		toAfterAvail := decimal.Zero
		if toAfter != nil {
			toAfterAvail = toAfter.AvailableAmount
		}

		// 5.4 写入balance change log
		orderNo := fmt.Sprintf("TRF-%d-%s", record.Id, utils.GenerateSnowflakeId())
		if err := s.createBalanceChangeLogTx(ctx, tx, req.FromUserID, symbol, consts.ChangeTypeTransferOut, amount.Neg(), fromBeforeAvail, fromAfterAvail, orderNo, record.Id, fmt.Sprintf("internal transfer to %s", toWalletAddress)); err != nil {
			return gerror.Wrap(err, "write transfer_out log failed")
		}
		if err := s.createBalanceChangeLogTx(ctx, tx, toUser.Id, symbol, consts.ChangeTypeTransferIn, amount, toBeforeAvail, toAfterAvail, orderNo, record.Id, fmt.Sprintf("internal transfer from %s", fromUser.WalletAddress)); err != nil {
			return gerror.Wrap(err, "write transfer_in log failed")
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, errTransferIdempotentConflict) {
			existing, getErr := s.transferRecordDao.GetByFromUserAndRequestID(ctx, req.FromUserID, requestID)
			if getErr != nil {
				return nil, gerror.Wrap(getErr, "read idempotent record failed")
			}
			if existing == nil {
				return nil, gerror.New("idempotent record not found")
			}
			return s.buildTransferRes(ctx, existing)
		}
		return nil, err
	}

	return s.buildTransferResWithBalance(ctx, record, fromAfterAvail)
}

// GetTransferRecords 查询用户内部转账记录
func (s *transferService) GetTransferRecords(ctx context.Context, req *model.GetTransferRecordsReq) (*model.GetTransferRecordsRes, error) {
	if req.UserID <= 0 {
		return nil, gerror.New("invalid user id")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	direction := strings.ToLower(strings.TrimSpace(req.Direction))
	if direction == "" {
		direction = "all"
	}
	if direction != "in" && direction != "out" && direction != "all" {
		return nil, gerror.New("invalid direction, must be in/out/all")
	}

	records, total, err := s.transferRecordDao.ListByUserID(ctx, req.UserID, direction, page, pageSize)
	if err != nil {
		return nil, gerror.Wrap(err, "query transfer records failed")
	}

	list := make([]model.TransferRecordItem, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}

		var dir string
		var counterpartyUserID int64
		if record.FromUserID == req.UserID {
			dir = "out"
			counterpartyUserID = record.ToUserID
		} else {
			dir = "in"
			counterpartyUserID = record.FromUserID
		}

		counterpartyWallet := ""
		if counterpartyUserID > 0 {
			u, _ := s.userRepo.GetUserById(ctx, counterpartyUserID)
			if u != nil {
				counterpartyWallet = u.WalletAddress
			}
		}

		list = append(list, model.TransferRecordItem{
			TransferID:                record.Id,
			Symbol:                    record.Symbol,
			Amount:                    record.Amount.String(),
			Direction:                 dir,
			CounterpartyWalletAddress: counterpartyWallet,
			CounterpartyUserID:        counterpartyUserID,
			CreatedAt:                 record.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	pages := total / pageSize
	if total%pageSize > 0 {
		pages++
	}

	return &model.GetTransferRecordsRes{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		Pages:    pages,
		List:     list,
	}, nil
}

func (s *transferService) buildTransferRes(ctx context.Context, record *entity.InternalTransferRecordEntity) (*model.InternalTransferRes, error) {
	// 查询转出方最新余额
	balance, err := s.balanceRepo.GetByUserID(ctx, record.FromUserID, record.Symbol)
	if err != nil {
		return nil, gerror.Wrap(err, "get balance failed")
	}

	balanceLeft := decimal.Zero
	if balance != nil {
		balanceLeft = balance.AvailableAmount
	}

	return s.buildTransferResWithBalance(ctx, record, balanceLeft)
}

func (s *transferService) buildTransferResWithBalance(ctx context.Context, record *entity.InternalTransferRecordEntity, balanceLeft decimal.Decimal) (*model.InternalTransferRes, error) {
	toUser, err := s.userRepo.GetUserById(ctx, record.ToUserID)
	if err != nil {
		return nil, gerror.Wrap(err, "get to user failed")
	}

	toWalletAddress := ""
	if toUser != nil {
		toWalletAddress = toUser.WalletAddress
	}

	return &model.InternalTransferRes{
		TransferID:      record.Id,
		Symbol:          record.Symbol,
		Amount:          record.Amount.String(),
		ToWalletAddress: toWalletAddress,
		ToUserID:        record.ToUserID,
		BalanceLeft:     balanceLeft.String(),
		CreatedAt:       record.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (s *transferService) createBalanceChangeLogTx(ctx context.Context, tx gdb.TX, userID int64, symbol, changeType string, amount, beforeBalance, afterBalance decimal.Decimal, relatedOrderNo string, relatedID int64, remark string) error {
	_, err := tx.Exec(`
		INSERT INTO cobo_balance_change_log (user_id, symbol, change_type, amount, before_balance, after_balance, related_order_no, related_id, remark, operator_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, userID, symbol, changeType, amount, beforeBalance, afterBalance, relatedOrderNo, relatedID, remark, consts.OperatorTypeUser, time.Now())
	if err != nil {
		g.Log().Errorf(ctx, "[Transfer] write balance change log failed: user_id=%d, change_type=%s, err=%v", userID, changeType, err)
	}
	return err
}

func (s *transferService) acquireRedisLock(ctx context.Context, key, token string, ttlSeconds int) (bool, error) {
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

func (s *transferService) releaseRedisLock(ctx context.Context, key, token string) {
	redisClient := g.Redis()
	if redisClient == nil {
		return
	}
	_, err := redisClient.Do(ctx, "EVAL", `if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) end return 0`, 1, key, token)
	if err != nil {
		g.Log().Warningf(ctx, "[Transfer] release distributed lock failed: key=%s err=%v", key, err)
	}
}
