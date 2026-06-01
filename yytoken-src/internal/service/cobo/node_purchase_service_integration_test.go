package cobo

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	framedb "XWFrame/internal/frame/db"
	"XWFrame/internal/service/cobo/model"

	"github.com/gogf/gf/v2/frame/g"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/shopspring/decimal"
)

func TestBuyNode_GrantDirectRewardOnly(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_DB_TEST") != "1" {
		t.Skip("set RUN_INTEGRATION_DB_TEST=1 to run")
	}

	ctx := context.Background()
	if err := framedb.Init(ctx); err != nil {
		t.Fatalf("init framedb failed: %v", err)
	}
	uniq := time.Now().UnixNano()

	buyerWallet := fmt.Sprintf("0x%040x", uniq)
	parentWallet := fmt.Sprintf("0x%040x", uniq+1)
	grandWallet := fmt.Sprintf("0x%040x", uniq+2)

	buyerCode := fmt.Sprintf("B_%d", uniq)
	parentCode := fmt.Sprintf("P_%d", uniq)
	grandCode := fmt.Sprintf("G_%d", uniq)

	cleanup := func() {
		_, _ = g.DB().Exec(ctx, "DELETE FROM cobo_reward_record WHERE reward_type = ? AND purchase_package_no IN (SELECT package_no FROM cobo_node_purchase WHERE request_id = ?)", "indirect", fmt.Sprintf("it-%d", uniq))
		_, _ = g.DB().Exec(ctx, "DELETE FROM cobo_node_purchase WHERE request_id = ?", fmt.Sprintf("it-%d", uniq))
		_, _ = g.DB().Exec(ctx, "DELETE FROM node_price_config WHERE node_type = 1 AND node_level = 'NODE1' AND price = 1000")
		_, _ = g.DB().Exec(ctx, "DELETE FROM node_info WHERE node_level = 'NODE1' AND status = 2 AND power_multiplier = 4 AND total_shares = 1000")
		_, _ = g.DB().Exec(ctx, "DELETE FROM cobo_balance WHERE user_id IN (SELECT id FROM user_info WHERE wallet_address IN (?,?,?))", buyerWallet, parentWallet, grandWallet)
		_, _ = g.DB().Exec(ctx, "DELETE FROM user_info WHERE wallet_address IN (?,?,?)", buyerWallet, parentWallet, grandWallet)
	}
	cleanup()
	defer cleanup()

	grandID, err := g.DB().Model("user_info").Ctx(ctx).Data(g.Map{
		"wallet_address": grandWallet,
		"invite_code":    grandCode,
	}).InsertAndGetId()
	if err != nil {
		t.Fatalf("insert grand user failed: %v", err)
	}

	parentID, err := g.DB().Model("user_info").Ctx(ctx).Data(g.Map{
		"wallet_address":        parentWallet,
		"invite_code":           parentCode,
		"parent_invite_code":    grandCode,
		"parent_wallet_address": grandWallet,
	}).InsertAndGetId()
	if err != nil {
		t.Fatalf("insert parent user failed: %v", err)
	}

	buyerID, err := g.DB().Model("user_info").Ctx(ctx).Data(g.Map{
		"wallet_address":        buyerWallet,
		"invite_code":           buyerCode,
		"parent_invite_code":    parentCode,
		"parent_wallet_address": parentWallet,
	}).InsertAndGetId()
	if err != nil {
		t.Fatalf("insert buyer user failed: %v", err)
	}

	_, err = g.DB().Exec(ctx, "INSERT INTO cobo_balance (user_id, symbol, available_amount, frozen_amount) VALUES (?, 'USDT', 2000, 0)", buyerID)
	if err != nil {
		t.Fatalf("insert buyer balance failed: %v", err)
	}

	_, err = g.DB().Exec(ctx, "INSERT INTO node_price_config (node_type, node_level, price) VALUES (1, 'NODE1', 1000)")
	if err != nil {
		t.Fatalf("insert node_price_config failed: %v", err)
	}

	_, err = g.DB().Exec(ctx, "INSERT INTO node_info (node_level, status, power_multiplier, total_shares, sold_shares) VALUES ('NODE1', 2, 4, 1000, 0)")
	if err != nil {
		t.Fatalf("insert node_info failed: %v", err)
	}

	svc := NewNodePurchaseService()
	requestID := fmt.Sprintf("it-%d", uniq)
	_, err = svc.BuyNode(ctx, &model.BuyNodeReq{
		UserID:    buyerID,
		NodeType:  1,
		RequestID: requestID,
	})
	if err != nil {
		t.Fatalf("buy node failed: %v", err)
	}

	getBal := func(userID int64) decimal.Decimal {
		v, e := g.DB().Model("cobo_balance").Ctx(ctx).Where("user_id = ? AND symbol = 'USDT'", userID).Value("available_amount")
		if e != nil {
			t.Fatalf("query balance failed for user %d: %v", userID, e)
		}
		if v.IsEmpty() {
			return decimal.Zero
		}
		d, e := decimal.NewFromString(v.String())
		if e != nil {
			t.Fatalf("parse balance failed for user %d: %v", userID, e)
		}
		return d
	}

	parentBal := getBal(parentID)
	grandBal := getBal(grandID)

	if !parentBal.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("unexpected direct reward balance, got=%s want=100", parentBal.String())
	}
	if !grandBal.Equal(decimal.Zero) {
		t.Fatalf("unexpected indirect reward balance, got=%s want=0", grandBal.String())
	}

	v, err := g.DB().Model("cobo_reward_record").Ctx(ctx).
		Where("user_id = ?", grandID).
		Where("reward_type = ?", "indirect").
		Where("source_user_id = ?", buyerID).
		Count()
	if err != nil {
		t.Fatalf("query indirect reward count failed: %v", err)
	}
	if v != 0 {
		t.Fatalf("unexpected indirect reward record count, got=%d want=0", v)
	}
}
