package staking_v2

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	framedb "XWFrame/internal/frame/db"
	"XWFrame/internal/service/staking_v2/model"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

func TestCreateStake_RejectsTooLongRequestID(t *testing.T) {
	svc := NewStakingV2Service()
	_, err := svc.CreateStake(context.Background(), &model.CreateStakeReq{
		UserID:    1,
		Amount:    "100",
		RequestID: "12345678901234567890123456789012345678901234567890123456789012345",
	})
	if err == nil {
		t.Fatal("expected request_id length error")
	}
}

func TestCreateStake_IdempotentAndAuditLog(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_DB_TEST") != "1" {
		t.Skip("set RUN_INTEGRATION_DB_TEST=1 to run")
	}

	ctx := context.Background()
	if err := framedb.Init(ctx); err != nil {
		t.Fatalf("init framedb failed: %v", err)
	}

	if err := ensureStakingV2TestSchema(ctx); err != nil {
		t.Fatalf("ensure schema failed: %v", err)
	}

	uniq := time.Now().UnixNano()
	wallet := fmt.Sprintf("0x%040x", uniq)
	inviteCode := fmt.Sprintf("s2_%d", uniq)
	requestID := fmt.Sprintf("stk-it-%d", uniq)

	cleanup := func(userID int64) {
		_, _ = g.DB().Exec(ctx, "DELETE FROM cobo_balance_change_log WHERE user_id = ? AND related_order_no = ?", userID, requestID)
		_, _ = g.DB().Exec(ctx, "DELETE FROM staking_v2_order WHERE user_id = ?", userID)
		_, _ = g.DB().Exec(ctx, "DELETE FROM staking_v2_user_stats WHERE user_id = ?", userID)
		_, _ = g.DB().Exec(ctx, "DELETE FROM cobo_balance WHERE user_id = ?", userID)
		_, _ = g.DB().Exec(ctx, "DELETE FROM user_info WHERE id = ?", userID)
	}

	userID, err := g.DB().Model("user_info").Ctx(ctx).Data(g.Map{
		"wallet_address": wallet,
		"invite_code":    inviteCode,
	}).InsertAndGetId()
	if err != nil {
		t.Fatalf("insert user failed: %v", err)
	}
	defer cleanup(userID)

	_, err = g.DB().Exec(ctx, "INSERT INTO cobo_balance (user_id, symbol, available_amount, frozen_amount, created_at, updated_at) VALUES (?, 'USDT', 1000, 0, NOW(), NOW())", userID)
	if err != nil {
		t.Fatalf("insert balance failed: %v", err)
	}

	_, err = g.DB().Exec(ctx, "INSERT INTO stock_price (symbol, price, price_time, created_at) VALUES ('YYAI', 0.2, NOW(), NOW())")
	if err != nil {
		t.Fatalf("insert stock price failed: %v", err)
	}

	svc := NewStakingV2Service()
	first, err := svc.CreateStake(ctx, &model.CreateStakeReq{
		UserID:    userID,
		Amount:    "100",
		RequestID: requestID,
	})
	if err != nil {
		t.Fatalf("first CreateStake failed: %v", err)
	}

	second, err := svc.CreateStake(ctx, &model.CreateStakeReq{
		UserID:    userID,
		Amount:    "100",
		RequestID: requestID,
	})
	if err != nil {
		t.Fatalf("second CreateStake failed: %v", err)
	}

	if first.OrderID != second.OrderID {
		t.Fatalf("idempotent order mismatch: first=%d second=%d", first.OrderID, second.OrderID)
	}

	orderCount, err := g.DB().Model("staking_v2_order").Ctx(ctx).Where("user_id = ?", userID).Count()
	if err != nil {
		t.Fatalf("query order count failed: %v", err)
	}
	if orderCount != 1 {
		t.Fatalf("unexpected order count: got=%d want=1", orderCount)
	}

	balanceValue, err := g.DB().Model("cobo_balance").Ctx(ctx).
		Where("user_id = ? AND symbol = 'USDT'", userID).
		Value("available_amount")
	if err != nil {
		t.Fatalf("query balance failed: %v", err)
	}
	balanceAfter, err := decimal.NewFromString(balanceValue.String())
	if err != nil {
		t.Fatalf("parse balance failed: %v", err)
	}
	if !balanceAfter.Equal(decimal.NewFromInt(900)) {
		t.Fatalf("unexpected balance after stake: got=%s want=900", balanceAfter.String())
	}

	logCount, err := g.DB().Model("cobo_balance_change_log").Ctx(ctx).
		Where("user_id = ? AND related_order_no = ?", userID, requestID).
		Count()
	if err != nil {
		t.Fatalf("query balance log count failed: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("unexpected balance log count: got=%d want=1", logCount)
	}
}

func ensureStakingV2TestSchema(ctx context.Context) error {
	_, err := g.DB().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS staking_v2_order (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			request_id VARCHAR(64) NOT NULL DEFAULT '',
			amount DECIMAL(36,18) NOT NULL,
			yyai_amount DECIMAL(36,18) NOT NULL DEFAULT 0,
			yyai_price DECIMAL(36,18) NOT NULL DEFAULT 0,
			source_type SMALLINT NOT NULL DEFAULT 1,
			status SMALLINT NOT NULL DEFAULT 1,
			total_reward DECIMAL(36,18) NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_staking_v2_order_user_request_id
			ON staking_v2_order(user_id, request_id)
			WHERE request_id <> '';

		CREATE TABLE IF NOT EXISTS staking_v2_user_stats (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL UNIQUE,
			total_stake_amount DECIMAL(36,18) NOT NULL DEFAULT 0,
			total_reward_earned DECIMAL(36,18) NOT NULL DEFAULT 0,
			reward_limit DECIMAL(36,18) NOT NULL DEFAULT 0,
			is_capped BOOLEAN NOT NULL DEFAULT FALSE,
			capped_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS cobo_balance_change_log (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			symbol VARCHAR(20) NOT NULL,
			change_type VARCHAR(50) NOT NULL,
			amount DECIMAL(36,18) NOT NULL,
			before_balance DECIMAL(36,18) NOT NULL DEFAULT 0,
			after_balance DECIMAL(36,18) NOT NULL DEFAULT 0,
			related_order_no VARCHAR(64) NOT NULL DEFAULT '',
			related_id BIGINT NOT NULL DEFAULT 0,
			remark VARCHAR(255) NOT NULL DEFAULT '',
			operator_type VARCHAR(20) NOT NULL DEFAULT 'system',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}
