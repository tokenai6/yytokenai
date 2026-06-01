package cache

import (
	"context"
	"time"

	"XWFrame/internal/frame/cache"

	"github.com/gogf/gf/v2/frame/g"
)

// CacheExample 缓存使用示例
type CacheExample struct{}

// NewCacheExample 创建缓存示例
func NewCacheExample() *CacheExample {
	return &CacheExample{}
}

// MemoryCacheExample 内存缓存使用示例
func (e *CacheExample) MemoryCacheExample(ctx context.Context) {
	// 方式1：使用全局内存缓存
	memoryCache := cache.GetMemoryCache()

	// 设置缓存
	err := memoryCache.Set(ctx, "user:1", "张三", 10*time.Minute)
	if err != nil {
		g.Log().Error(ctx, "设置内存缓存失败:", err)
		return
	}

	// 获取缓存
	value, err := memoryCache.Get(ctx, "user:1")
	if err != nil {
		g.Log().Error(ctx, "获取内存缓存失败:", err)
		return
	}
	g.Log().Info(ctx, "内存缓存值:", value)

	// 检查是否存在
	exists, _ := memoryCache.Contains(ctx, "user:1")
	if exists {
		g.Log().Info(ctx, "缓存存在")
	}

	// 删除缓存
	_, err = memoryCache.Remove(ctx, "user:1")
	if err != nil {
		g.Log().Error(ctx, "删除内存缓存失败:", err)
	}
}

// RedisCacheExample Redis缓存使用示例
func (e *CacheExample) RedisCacheExample(ctx context.Context) {
	// 方式1：使用全局Redis缓存
	redisCache := cache.GetRedisCache()

	// 设置缓存
	err := redisCache.Set(ctx, "session:abc123", "user_data", 30*time.Minute)
	if err != nil {
		g.Log().Error(ctx, "设置Redis缓存失败:", err)
		return
	}

	// 获取缓存
	value, err := redisCache.Get(ctx, "session:abc123")
	if err != nil {
		g.Log().Error(ctx, "获取Redis缓存失败:", err)
		return
	}
	g.Log().Info(ctx, "Redis缓存值:", value)

	// 使用函数设置缓存（如果不存在）
	result, err := redisCache.SetIfNotExistFunc(ctx, "counter", func(ctx context.Context) (interface{}, error) {
		return 1, nil
	}, 1*time.Hour)
	if err != nil {
		g.Log().Error(ctx, "设置Redis缓存失败:", err)
		return
	}
	g.Log().Info(ctx, "计数器值:", result)
}

// LocalMemoryCacheExample 本地内存缓存使用示例
func (e *CacheExample) LocalMemoryCacheExample(ctx context.Context) {
	// 方式2：在各个模块中直接使用 gcache.New()
	localCache := cache.NewMemoryCache()

	// 设置缓存
	err := localCache.Set(ctx, "temp:data", "临时数据", 5*time.Minute)
	if err != nil {
		g.Log().Error(ctx, "设置本地缓存失败:", err)
		return
	}

	// 获取缓存
	value, err := localCache.Get(ctx, "temp:data")
	if err != nil {
		g.Log().Error(ctx, "获取本地缓存失败:", err)
		return
	}
	g.Log().Info(ctx, "本地缓存值:", value)

	// 获取缓存大小
	size, _ := localCache.Size(ctx)
	g.Log().Info(ctx, "本地缓存大小:", size)
}

// StructCacheExample 结构体缓存示例
func (e *CacheExample) StructCacheExample(ctx context.Context) {
	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	redisCache := cache.GetRedisCache()

	// 设置结构体缓存
	user := &User{
		ID:   1,
		Name: "李四",
		Age:  25,
	}

	err := redisCache.Set(ctx, "user:struct:1", user, 1*time.Hour)
	if err != nil {
		g.Log().Error(ctx, "设置结构体缓存失败:", err)
		return
	}

	// 获取结构体缓存
	value, err := redisCache.Get(ctx, "user:struct:1")
	if err != nil {
		g.Log().Error(ctx, "获取结构体缓存失败:", err)
		return
	}

	// 使用Struct方法将gvar.Var转换为结构体
	var cachedUser User
	err = value.Struct(&cachedUser)
	if err != nil {
		g.Log().Error(ctx, "解析缓存数据失败:", err)
		return
	}
	g.Log().Info(ctx, "缓存的用户:", cachedUser)
}

// CacheWithLockExample 带锁的缓存示例
func (e *CacheExample) CacheWithLockExample(ctx context.Context) {
	redisCache := cache.GetRedisCache()

	// 使用带锁的函数设置缓存
	result, err := redisCache.SetIfNotExistFuncLock(ctx, "expensive:data", func(ctx context.Context) (interface{}, error) {
		// 模拟耗时操作
		time.Sleep(100 * time.Millisecond)
		return "expensive_result", nil
	}, 1*time.Hour)

	if err != nil {
		g.Log().Error(ctx, "设置带锁缓存失败:", err)
		return
	}

	g.Log().Info(ctx, "带锁缓存结果:", result)
}

// CacheStatisticsExample 缓存统计示例
func (e *CacheExample) CacheStatisticsExample(ctx context.Context) {
	memoryCache := cache.GetMemoryCache()
	redisCache := cache.GetRedisCache()

	// 内存缓存统计
	memorySize, _ := memoryCache.Size(ctx)
	memoryKeys, _ := memoryCache.Keys(ctx)
	g.Log().Info(ctx, "内存缓存统计 - 大小:", memorySize, "键数量:", len(memoryKeys))

	// Redis缓存统计
	redisSize, _ := redisCache.Size(ctx)
	redisKeys, _ := redisCache.Keys(ctx)
	g.Log().Info(ctx, "Redis缓存统计 - 大小:", redisSize, "键数量:", len(redisKeys))
}
