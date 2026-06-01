package cache

import (
	"context"
	"time"

	"XWFrame/internal/frame/cache"

	"github.com/gogf/gf/v2/os/gcache"
)

const (
	tokenPriceCachePrefix = "apg:mint:token:price:"
	tokenPriceCacheTTL    = 1 * time.Minute
)

type TokenPriceData struct {
	Price     string `json:"price"`
	UpdatedAt int64  `json:"updated_at"`
}

type ITokenPriceCache interface {
	Get(ctx context.Context, contractAddress string) (*TokenPriceData, bool)
	Set(ctx context.Context, contractAddress string, price string) error
	GetAll(ctx context.Context, addresses []string) map[string]*TokenPriceData
	SetBatch(ctx context.Context, prices map[string]string) error
}

type tokenPriceCache struct {
	redis *gcache.Cache
}

func NewTokenPriceCache() ITokenPriceCache {
	return &tokenPriceCache{
		redis: cache.GetRedisCache(),
	}
}

func (c *tokenPriceCache) buildKey(contractAddress string) string {
	return tokenPriceCachePrefix + contractAddress
}

func (c *tokenPriceCache) Get(ctx context.Context, contractAddress string) (*TokenPriceData, bool) {
	val, err := c.redis.Get(ctx, c.buildKey(contractAddress))
	if err != nil || val.IsNil() || val.IsEmpty() {
		return nil, false
	}
	var data TokenPriceData
	if err := val.Struct(&data); err != nil {
		return nil, false
	}
	return &data, true
}

func (c *tokenPriceCache) Set(ctx context.Context, contractAddress string, price string) error {
	data := &TokenPriceData{
		Price:     price,
		UpdatedAt: time.Now().Unix(),
	}
	return c.redis.Set(ctx, c.buildKey(contractAddress), data, tokenPriceCacheTTL)
}

func (c *tokenPriceCache) GetAll(ctx context.Context, addresses []string) map[string]*TokenPriceData {
	result := make(map[string]*TokenPriceData)
	for _, addr := range addresses {
		if data, ok := c.Get(ctx, addr); ok {
			result[addr] = data
		}
	}
	return result
}

func (c *tokenPriceCache) SetBatch(ctx context.Context, prices map[string]string) error {
	for addr, price := range prices {
		if err := c.Set(ctx, addr, price); err != nil {
			return err
		}
	}
	return nil
}
