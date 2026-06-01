package cache

import (
	"XWFrame/internal/frame/cache"
	"context"
	"time"

	"github.com/gogf/gf/v2/os/gcache"
)

// UserInfo 用户信息结构体
type UserInfo struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Wallet   string `json:"wallet"`
	Status   int    `json:"status"`
}

// IUserCache 用户缓存接口
type IUserCache interface {
	SetToken(ctx context.Context, key string, token string, expiration time.Duration) error
	GetToken(ctx context.Context, key string) (string, error)
	DelToken(ctx context.Context, key string) error
	SetUserInfo(ctx context.Context, key string, userInfo *UserInfo, expiration time.Duration) error
	GetUserInfo(ctx context.Context, key string) (*UserInfo, error)
	DelUserInfo(ctx context.Context, key string) error
}

// userCache 用户缓存实现
type userCache struct {
	redisCache *gcache.Cache
}

// NewUserCache 创建用户缓存实例
func NewUserCache() IUserCache {
	return &userCache{
		redisCache: cache.GetRedisCache(),
	}
}

// SetToken 设置token缓存（使用Redis）
func (c *userCache) SetToken(ctx context.Context, key string, token string, expiration time.Duration) error {
	return c.redisCache.Set(ctx, key, token, expiration)
}

// GetToken 获取token缓存（使用Redis）
func (c *userCache) GetToken(ctx context.Context, key string) (string, error) {
	value, err := c.redisCache.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return value.String(), nil
}

// DelToken 删除token缓存（使用Redis）
func (c *userCache) DelToken(ctx context.Context, key string) error {
	_, err := c.redisCache.Remove(ctx, key)
	return err
}

// SetUserInfo 设置用户信息缓存（使用Redis）
func (c *userCache) SetUserInfo(ctx context.Context, key string, userInfo *UserInfo, expiration time.Duration) error {
	return c.redisCache.Set(ctx, key, userInfo, expiration)
}

// GetUserInfo 获取用户信息缓存（使用Redis）
func (c *userCache) GetUserInfo(ctx context.Context, key string) (*UserInfo, error) {
	value, err := c.redisCache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var userInfo UserInfo
	err = value.Struct(&userInfo)
	if err != nil {
		return nil, err
	}
	return &userInfo, nil
}

// DelUserInfo 删除用户信息缓存（使用Redis）
func (c *userCache) DelUserInfo(ctx context.Context, key string) error {
	_, err := c.redisCache.Remove(ctx, key)
	return err
}
