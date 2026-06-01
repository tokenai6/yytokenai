package cobo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/frame/config"
	"XWFrame/internal/repository"
	externalCobo "XWFrame/pkg/external/cobo"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// DirectTransferService 直接调用 Cobo 转账到用户钱包的服务
// 不经过平台余额系统，审批通过后直接链上转账
type DirectTransferService interface {
	EnsureTransferOrderTx(ctx context.Context, tx gdb.TX, req *EnsureTransferOrderReq) (*DirectTransferOrder, error)
	SubmitTransferOrder(ctx context.Context, bizType string, bizID int64, phase string) (*DirectTransferOrder, error)
	// TransferUSDT 直接转账 USDT 到用户钱包地址
	// userID: 收款用户ID
	// amount: 转账金额
	// orderNo: 业务订单号（用于日志和追踪）
	// description: 转账描述
	// 返回 Cobo request_id 和错误
	TransferUSDT(ctx context.Context, userID int64, amount decimal.Decimal, orderNo, description string) (string, error)
}

const (
	DirectTransferStatusPending    = "pending"
	DirectTransferStatusSubmitting = "submitting"
	DirectTransferStatusSubmitted  = "submitted"
	DirectTransferStatusSuccess    = "success"
	DirectTransferStatusFailed     = "failed"
)

type EnsureTransferOrderReq struct {
	BizType     string
	BizID       int64
	Phase       string
	UserID      int64
	Symbol      string
	Amount      decimal.Decimal
	Description string
}

type DirectTransferOrder struct {
	ID           int64           `orm:"id"`
	BizType      string          `orm:"biz_type"`
	BizID        int64           `orm:"biz_id"`
	Phase        string          `orm:"phase"`
	UserID       int64           `orm:"user_id"`
	Symbol       string          `orm:"symbol"`
	Amount       decimal.Decimal `orm:"amount"`
	RequestID    string          `orm:"request_id"`
	CoboTxID     string          `orm:"cobo_tx_id"`
	TxHash       string          `orm:"tx_hash"`
	CoboStatus   string          `orm:"cobo_status"`
	Status       string          `orm:"status"`
	ErrorMessage string          `orm:"error_message"`
	Description  string          `orm:"description"`
}

type directTransferService struct {
	userRepo repository.IUserRepository
}

// NewDirectTransferService 创建直接转账服务
func NewDirectTransferService() DirectTransferService {
	return &directTransferService{
		userRepo: repository.NewUserRepository(),
	}
}

func (s *directTransferService) EnsureTransferOrderTx(ctx context.Context, tx gdb.TX, req *EnsureTransferOrderReq) (*DirectTransferOrder, error) {
	if req == nil {
		return nil, fmt.Errorf("transfer order request is nil")
	}
	if strings.TrimSpace(req.BizType) == "" || req.BizID <= 0 || strings.TrimSpace(req.Phase) == "" || req.UserID <= 0 || !req.Amount.GreaterThan(decimal.Zero) {
		return nil, fmt.Errorf("invalid transfer order request")
	}
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	if symbol == "" {
		symbol = "USDT"
	}
	bizType := strings.TrimSpace(req.BizType)
	phase := strings.TrimSpace(req.Phase)
	requestID := fmt.Sprintf("DT-%s-%d-%s", bizType, req.BizID, phase)

	rows, err := tx.GetAll(`
		INSERT INTO direct_transfer_order
		(biz_type, biz_id, phase, user_id, symbol, amount, request_id, status, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		ON CONFLICT (biz_type, biz_id, phase) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			symbol = EXCLUDED.symbol,
			amount = EXCLUDED.amount,
			description = EXCLUDED.description,
			updated_at = NOW()
		RETURNING id, biz_type, biz_id, phase, user_id, symbol, amount, request_id, COALESCE(cobo_tx_id, '') AS cobo_tx_id, COALESCE(tx_hash, '') AS tx_hash, status, COALESCE(error_message, '') AS error_message, COALESCE(description, '') AS description
	`, bizType, req.BizID, phase, req.UserID, symbol, req.Amount, requestID, DirectTransferStatusPending, strings.TrimSpace(req.Description))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("create transfer order returned no rows")
	}
	var order DirectTransferOrder
	if err := rows[0].Struct(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (s *directTransferService) SubmitTransferOrder(ctx context.Context, bizType string, bizID int64, phase string) (*DirectTransferOrder, error) {
	var order DirectTransferOrder
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if err := tx.Ctx(ctx).Raw(`
			SELECT id, biz_type, biz_id, phase, user_id, symbol, amount, request_id, COALESCE(cobo_tx_id, '') AS cobo_tx_id, COALESCE(tx_hash, '') AS tx_hash, status, COALESCE(error_message, '') AS error_message, COALESCE(description, '') AS description
			FROM direct_transfer_order
			WHERE biz_type = ? AND biz_id = ? AND phase = ?
			FOR UPDATE
		`, strings.TrimSpace(bizType), bizID, strings.TrimSpace(phase)).Scan(&order); err != nil {
			return err
		}
		if order.ID == 0 {
			return fmt.Errorf("transfer order not found: %s %d %s", bizType, bizID, phase)
		}
		if order.Status == DirectTransferStatusSubmitted || order.Status == DirectTransferStatusSuccess {
			return nil
		}
		if order.Status == DirectTransferStatusSubmitting {
			return fmt.Errorf("transfer order is submitting: %s", order.RequestID)
		}
		_, err := tx.Model("direct_transfer_order").Ctx(ctx).Where("id = ?", order.ID).Data(g.Map{"status": DirectTransferStatusSubmitting, "error_message": "", "updated_at": time.Now()}).Update()
		return err
	})
	if err != nil {
		return nil, err
	}
	if order.Status == DirectTransferStatusSubmitted || order.Status == DirectTransferStatusSuccess {
		return &order, nil
	}

	coboTxID, status, transferErr := s.submitUSDT(ctx, order.UserID, order.Amount, order.RequestID, order.Description)
	update := g.Map{"updated_at": time.Now()}
	if transferErr != nil {
		update["status"] = DirectTransferStatusFailed
		update["error_message"] = transferErr.Error()
		_, _ = g.DB().Model("direct_transfer_order").Ctx(ctx).Where("id = ?", order.ID).Data(update).Update()
		return &order, transferErr
	}
	update["status"] = DirectTransferStatusSubmitted
	update["cobo_tx_id"] = coboTxID
	update["cobo_status"] = status
	update["error_message"] = ""
	_, err = g.DB().Model("direct_transfer_order").Ctx(ctx).Where("id = ?", order.ID).Data(update).Update()
	if err != nil {
		return nil, err
	}
	order.Status = DirectTransferStatusSubmitted
	order.CoboTxID = coboTxID
	return &order, nil
}

func (s *directTransferService) TransferUSDT(ctx context.Context, userID int64, amount decimal.Decimal, orderNo, description string) (string, error) {
	requestID := fmt.Sprintf("DT-%s", strings.TrimSpace(orderNo))
	_, _, err := s.submitUSDT(ctx, userID, amount, requestID, description)
	return requestID, err
}

func (s *directTransferService) submitUSDT(ctx context.Context, userID int64, amount decimal.Decimal, requestID, description string) (string, string, error) {
	if amount.LessThanOrEqual(decimal.Zero) {
		return "", "", fmt.Errorf("transfer amount must be positive")
	}

	// 1. 查询用户钱包地址
	user, err := s.userRepo.GetUserById(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("get user failed: %w", err)
	}
	if user == nil {
		return "", "", fmt.Errorf("user not found: %d", userID)
	}
	toAddress := strings.ToLower(strings.TrimSpace(user.WalletAddress))
	if toAddress == "" {
		return "", "", fmt.Errorf("user wallet address empty: %d", userID)
	}

	// 2. 获取 Cobo 配置
	cfg, err := config.GetCoboConfig(ctx)
	if err != nil {
		return "", "", fmt.Errorf("get cobo config failed: %w", err)
	}

	walletID := strings.TrimSpace(cfg.Withdraw.WalletID)
	if walletID == "" {
		walletID = strings.TrimSpace(cfg.WalletID)
	}
	if cfg.APISecret == "" || walletID == "" {
		return "", "", fmt.Errorf("cobo api secret or wallet id not configured")
	}

	// 3. 创建 Cobo 客户端
	coboClient, err := externalCobo.NewCoboClient(&externalCobo.Config{
		APISecret: cfg.APISecret,
		Env:       cfg.Env,
		Timeout:   cfg.Timeout,
	})
	if err != nil {
		return "", "", fmt.Errorf("create cobo client failed: %w", err)
	}

	// 4. 构建转账请求
	tokenID := "BSC_USDT" // USDT 默认走 BSC 链

	transferReq := &externalCobo.TransferRequest{
		RequestID:   requestID,
		WalletID:    walletID,
		TokenID:     tokenID,
		ToAddress:   toAddress,
		Amount:      amount.String(),
		Description: description,
	}

	g.Log().Infof(ctx, "[DirectTransfer] submitting: user_id=%d, amount=%s, to=%s, request_id=%s",
		userID, amount.String(), toAddress, requestID)

	// 5. 发起转账
	resp, err := coboClient.Transfer(ctx, transferReq)
	if err != nil {
		g.Log().Errorf(ctx, "[DirectTransfer] failed: user_id=%d, amount=%s, order_no=%s, err=%v",
			userID, amount.String(), requestID, err)
		return "", "", fmt.Errorf("cobo transfer failed: %w", err)
	}

	g.Log().Infof(ctx, "[DirectTransfer] success: user_id=%d, amount=%s, request_id=%s, cobo_tx_id=%s, status=%s",
		userID, amount.String(), requestID, resp.TransactionID, resp.Status)

	return resp.TransactionID, resp.Status, nil
}
