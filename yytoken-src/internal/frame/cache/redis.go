package cache

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
)

var GlobalRedisCache *gcache.Cache

// InitRedisCache 初始化Redis缓存
func InitRedisCache(ctx context.Context) error {
	// 创建Redis缓存实例
	cache := gcache.New()

	// 设置Redis适配器
	redisAdapter := gcache.NewAdapterRedis(g.Redis())
	cache.SetAdapter(redisAdapter)

	GlobalRedisCache = cache

	g.Log().Info(ctx, "Redis缓存初始化完成")
	return nil
}

// GetRedisCache 获取全局Redis缓存实例
func GetRedisCache() *gcache.Cache {
	return GlobalRedisCache
}

// NewRedisCache 创建新的Redis缓存实例
func NewRedisCache() *gcache.Cache {
	cache := gcache.New()
	redisAdapter := gcache.NewAdapterRedis(g.Redis())
	cache.SetAdapter(redisAdapter)

	return cache
}
