package db

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// TxHandler 事务处理函数类型
type TxHandler func(ctx context.Context, tx gdb.TX) error

// WithTx 事务管理器
// 如果tx不为nil，直接使用传入的事务执行handler
// 如果tx为nil，创建新事务并在handler执行后自动提交或回滚
func WithTx(ctx context.Context, tx gdb.TX, handler TxHandler) error {
	// 如果传入了事务，直接使用
	if tx != nil {
		return handler(ctx, tx)
	}

	// 创建新事务
	return g.DB().Transaction(ctx, func(ctx context.Context, newTx gdb.TX) error {
		return handler(ctx, newTx)
	})
}

// GetOrCreateDB 获取数据库操作对象
// 如果tx不为nil，返回事务对象
// 如果tx为nil，返回普通数据库对象
func GetOrCreateDB(ctx context.Context, tx gdb.TX) gdb.DB {
	if tx != nil {
		return tx.GetDB()
	}
	return g.DB().Ctx(ctx)
}

// MustBeginTx 确保在事务中执行
// 如果tx为nil会panic，用于强制要求在事务中执行的场景
func MustBeginTx(tx gdb.TX) {
	if tx == nil {
		panic("transaction is required but not provided")
	}
}
