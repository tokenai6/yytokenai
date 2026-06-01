package db

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

var DB gdb.DB

// Init 初始化数据库
func Init(ctx context.Context) error {
	if g.Cfg().MustGet(ctx, "database").IsNil() {
		g.Log().Info(ctx, "数据库未配置，跳过初始化")
		return nil
	}

	// 获取数据库实例
	DB = g.DB()

	g.Log().Info(ctx, "数据库连接初始化完成")
	return nil
}

// GetDB 获取数据库实例
func GetDB() gdb.DB {
	return DB
}
