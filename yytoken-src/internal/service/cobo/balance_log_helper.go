package cobo

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// writeBalanceChangeLog 写入余额变更审计日志（非事务）
func writeBalanceChangeLog(ctx context.Context, userID int64, symbol, changeType string, amount, beforeBalance, afterBalance decimal.Decimal, relatedOrderNo string, relatedID int64, remark, operatorType string) error {
	return writeBalanceChangeLogTx(ctx, nil, userID, symbol, changeType, amount, beforeBalance, afterBalance, relatedOrderNo, relatedID, remark, operatorType)
}

// writeBalanceChangeLogTx 写入余额变更审计日志（事务/非事务）
func writeBalanceChangeLogTx(ctx context.Context, tx gdb.TX, userID int64, symbol, changeType string, amount, beforeBalance, afterBalance decimal.Decimal, relatedOrderNo string, relatedID int64, remark, operatorType string) error {
	if operatorType == "" {
		operatorType = "system"
	}
	sql := `
		INSERT INTO cobo_balance_change_log
		(user_id, symbol, change_type, amount, before_balance, after_balance, related_order_no, related_id, remark, operator_type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	var err error
	if tx != nil {
		_, err = tx.Exec(sql, userID, symbol, changeType, amount, beforeBalance, afterBalance, relatedOrderNo, relatedID, remark, operatorType, time.Now())
	} else {
		_, err = g.DB().Exec(ctx, sql, userID, symbol, changeType, amount, beforeBalance, afterBalance, relatedOrderNo, relatedID, remark, operatorType, time.Now())
	}
	if err != nil {
		g.Log().Errorf(ctx, "[BalanceLog] 写入余额变更日志失败: user_id=%d, change_type=%s, err=%v", userID, changeType, err)
	}
	return err
}
