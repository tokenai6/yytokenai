package cache

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
)

var GlobalMemoryCache *gcache.Cache

// InitMemoryCache 初始化内存缓存
func InitMemoryCache(ctx context.Context) error {
	// 创建全局内存缓存实例
	GlobalMemoryCache = gcache.New()

	g.Log().Info(ctx, "内存缓存初始化完成")
	return nil
}

// GetMemoryCache 获取全局内存缓存实例
func GetMemoryCache() *gcache.Cache {
	return GlobalMemoryCache
}

// NewMemoryCache 创建新的内存缓存实例
func NewMemoryCache() *gcache.Cache {
	return gcache.New()
}
