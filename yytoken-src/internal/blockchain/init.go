package blockchain

import (
	"XWFrame/internal/frame/cache"
	"XWFrame/internal/frame/db"
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
)

var globalEventListener *EventListenerService

// Init 初始化区块链事件监听服务
func Init(ctx context.Context) error {
	// 检查是否启用
	if !g.Cfg().MustGet(ctx, "blockchain.enabled", false).Bool() {
		glog.Info(ctx, "区块链事件监听服务未启用（blockchain.enabled=false）")
		return nil
	}

	glog.Info(ctx, "正在初始化区块链事件监听服务...")

	// 获取数据库和缓存实例
	database := db.GetDB()
	cacheInstance := cache.GetRedisCache()

	// 创建事件监听服务
	eventListener, err := NewEventListenerService(ctx, database, cacheInstance)
	if err != nil {
		return err
	}

	// 启动事件监听服务
	if err := eventListener.Start(); err != nil {
		return err
	}

	// 保存全局实例
	globalEventListener = eventListener

	glog.Info(ctx, "✅ 区块链事件监听服务已启动")

	// 注册优雅退出信号处理
	go handleShutdown(ctx, eventListener)

	return nil
}

// handleShutdown 处理优雅退出
func handleShutdown(ctx context.Context, eventListener *EventListenerService) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待退出信号
	sig := <-sigChan
	glog.Infof(ctx, "收到退出信号: %v，正在关闭区块链事件监听服务...", sig)

	if eventListener != nil {
		// 立即停止事件监听服务
		eventListener.Stop()
		glog.Info(ctx, "✅ 区块链事件监听服务已停止")
	}

	// 强制退出程序
	os.Exit(0)
}

// Stop 停止区块链事件监听服务
func Stop() {
	if globalEventListener != nil {
		globalEventListener.Stop()
	}
}

// GetEventListener 获取全局事件监听服务实例
func GetEventListener() *EventListenerService {
	return globalEventListener
}
