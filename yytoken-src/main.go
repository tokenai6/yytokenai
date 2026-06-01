package main

import (
	"context"

	"XWFrame/api"
	"XWFrame/cmd/sync_usdt"
	"XWFrame/cmd/sync_usdt_api"
	"XWFrame/cmd/task"
	"XWFrame/internal/frame"
	"XWFrame/internal/frame/config"
	"XWFrame/internal/frame/swagger"
	"XWFrame/internal/scheduler"
	"XWFrame/internal/service/apg"
	coboSvc "XWFrame/internal/service/cobo"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcmd"
	"github.com/gogf/gf/v2/os/gctx"

	// 导入PostgreSQL驱动
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	// 导入Redis驱动
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	// 导入任务处理器(触发init注册)
	_ "XWFrame/internal/scheduler/handlers"
)

// startServer 启动HTTP服务器
func startServer(ctx context.Context, parser *gcmd.Parser) error {
	// 支持 -c / --config 指定配置文件
	if parser != nil {
		if opt := parser.GetOpt("c"); !opt.IsEmpty() {
			config.CustomConfigFile = opt.String()
		} else if opt := parser.GetOpt("config"); !opt.IsEmpty() {
			config.CustomConfigFile = opt.String()
		}
	}

	// 注册 mint_group 合约同步回调
	frame.RegisterMintGroupSyncCallback(func(ctx context.Context) error {
		return apg.NewTokenService().RefreshFromContract(ctx)
	})

	// 初始化框架
	if err := frame.Init(ctx); err != nil {
		g.Log().Fatal(ctx, "框架初始化失败:", err)
	}

	// 初始化任务调度器
	if err := scheduler.Init(ctx); err != nil {
		g.Log().Errorf(ctx, "任务调度器初始化失败: %v", err)
	}

	// 启动 Cobo 每日充提 Telegram 统计通知（北京时间 00:00）
	coboSvc.StartDailyStatisticsNotifyJob(ctx)

	// 区块链事件监听已停用（当前仅保留 Cobo 相关能力）

	// 获取HTTP服务器实例
	s := g.Server()

	// 初始化Swagger配置
	swagger.InitSwagger(s)

	// 注册所有路由（C端 + Admin端）
	api.RegisterRoutes(s)

	// 优雅关闭(服务器关闭时停止调度器和事件监听)
	defer func() {
		g.Log().Info(ctx, "正在执行优雅关闭...")

		// 区块链事件监听已停用

		// 停止任务调度器
		if err := scheduler.Stop(); err != nil {
			g.Log().Errorf(ctx, "停止调度器失败: %v", err)
		}

		g.Log().Info(ctx, "✅ 优雅关闭完成")
	}()

	// 启动服务器
	s.Run()
	return nil
}

func main() {
	// 创建根命令
	cmd := &gcmd.Command{
		Name:        "main",
		Brief:       "XWFrame DApp 应用",
		Description: "区块链DApp后台系统",
		Func: func(ctx context.Context, parser *gcmd.Parser) (err error) {
			// 默认启动服务器
			return startServer(ctx, parser)
		},
	}

	// 添加任务命令
	cmd.AddCommand(task.Task)

	// 添加USDT同步命令
	cmd.AddCommand(sync_usdt.SyncUsdtCommand)
	cmd.AddCommand(sync_usdt_api.SyncUsdtApiCommand)

	// 运行命令
	cmd.Run(gctx.New())
}
