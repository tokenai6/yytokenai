package cache

import "github.com/gogf/gf/v2/util/gconv"

// CacheKey 缓存Key规则定义
type CacheKey struct{}

// User 用户相关缓存Key
func (CacheKey) User() UserCacheKey {
	return UserCacheKey{}
}

// Home 首页缓存Key
func (CacheKey) Home() HomeCacheKey {
	return HomeCacheKey{}
}

// Team 团队缓存Key
func (CacheKey) Team() TeamCacheKey {
	return TeamCacheKey{}
}

type HomeCacheKey struct{}

type TeamCacheKey struct{}

// Overview 团队概览缓存Key
func (TeamCacheKey) Overview(userId int64) string {
	return "team:overview:" + gconv.String(userId)
}

// GlobalDistrictStats 全网小区统计缓存Key
func (TeamCacheKey) GlobalDistrictStats() string {
	return "team:overview:global_district_stats"
}

// Stats 首页统计缓存Key
func (HomeCacheKey) Stats() string {
	return "home:stats"
}

// StatsRelease 首页统计释放类数据缓存Key
func (HomeCacheKey) StatsRelease() string {
	return "home:stats:release"
}

// StatsTokenStaking 首页统计代币质押情况缓存Key
func (HomeCacheKey) StatsTokenStaking() string {
	return "home:stats:token_staking"
}

type UserCacheKey struct{}

// Info 用户信息缓存Key
func (UserCacheKey) Info(userId int64) string {
	return "user:info:" + gconv.String(userId)
}

// Token 用户Token缓存Key
//
// Deprecated: 已被 TokenSession 取代以支持多设备在线。保留是为了清理旧缓存。
func (UserCacheKey) Token(userId int64) string {
	return "user:token:" + gconv.String(userId)
}

// TokenSession 用户单个设备 token 的独立 session Key
// 每个 token 一个 key，支持同一用户多设备同时在线
func (UserCacheKey) TokenSession(userId int64, tokenHash string) string {
	return "user:token:" + gconv.String(userId) + ":" + tokenHash
}

// TokenSessionPrefix 用户所有设备 token session 的扫描前缀
func (UserCacheKey) TokenSessionPrefix(userId int64) string {
	return "user:token:" + gconv.String(userId) + ":"
}

// ApiSecret 用户API签名密钥缓存Key
//
// Deprecated: 已被 ApiSecretSession 取代以支持多设备在线。保留是为了清理旧缓存。
func (UserCacheKey) ApiSecret(userId int64) string {
	return "user:api_secret:" + gconv.String(userId)
}

// ApiSecretSession 用户单个设备 api_secret 的独立 session Key
// 与 TokenSession 一一对应，按 token hash 区分设备
func (UserCacheKey) ApiSecretSession(userId int64, tokenHash string) string {
	return "user:api_secret:" + gconv.String(userId) + ":" + tokenHash
}

// Wallet 用户钱包缓存Key
func (UserCacheKey) Wallet(userId int64) string {
	return "user:wallet:" + gconv.String(userId)
}

// Admin 管理员相关缓存Key
func (CacheKey) Admin() AdminCacheKey {
	return AdminCacheKey{}
}

type AdminCacheKey struct{}

// Info 管理员信息缓存Key
func (AdminCacheKey) Info(adminId int64) string {
	return "admin:info:" + gconv.String(adminId)
}

// Token 管理员Token缓存Key
func (AdminCacheKey) Token(adminId int64) string {
	return "admin:token:" + gconv.String(adminId)
}

// Permission 管理员权限缓存Key
func (AdminCacheKey) Permission(adminId int64) string {
	return "admin:permission:" + gconv.String(adminId)
}

// Wallet 钱包相关缓存Key
func (CacheKey) Wallet() WalletCacheKey {
	return WalletCacheKey{}
}

type WalletCacheKey struct{}

// Nonce 钱包Nonce缓存Key
func (WalletCacheKey) Nonce(address string) string {
	return "wallet:nonce:" + address
}

// Balance 钱包余额缓存Key
func (WalletCacheKey) Balance(address string) string {
	return "wallet:balance:" + address
}

// RateLimit 限流相关缓存Key
func (CacheKey) RateLimit() RateLimitCacheKey {
	return RateLimitCacheKey{}
}

type RateLimitCacheKey struct{}

// Login 登录限流Key
func (RateLimitCacheKey) Login(ip string) string {
	return "ratelimit:login:" + ip
}

// API API限流Key
func (RateLimitCacheKey) API(userId int64, endpoint string) string {
	return "ratelimit:api:" + gconv.String(userId) + ":" + endpoint
}

// Balance 资金相关缓存Key
func (CacheKey) Balance() BalanceCacheKey {
	return BalanceCacheKey{}
}

type BalanceCacheKey struct{}

// UserBalance 用户余额缓存Key
func (BalanceCacheKey) UserBalance(userId, assetId int64) string {
	return "balance:" + gconv.String(userId) + ":" + gconv.String(assetId)
}

// ExchangeRate 兑换汇率缓存Key
func (BalanceCacheKey) ExchangeRate(fromToken, toToken string) string {
	return "exchange:rate:" + fromToken + ":" + toToken
}

// WithdrawLimit 提现限制缓存Key
func (BalanceCacheKey) WithdrawLimit(userId int64, date string) string {
	return "withdraw:limit:" + gconv.String(userId) + ":" + date
}

// System 系统相关缓存Key
func (CacheKey) System() SystemCacheKey {
	return SystemCacheKey{}
}

type SystemCacheKey struct{}

// Maintenance 系统维护模式缓存Key
func (SystemCacheKey) Maintenance() string {
	return "system:maintenance"
}

// Performance 业绩相关缓存Key
func (CacheKey) Performance() PerformanceCacheKey {
	return PerformanceCacheKey{}
}

type PerformanceCacheKey struct{}

// RealtimeTeamDistrict 用户实时团队/小区业绩缓存Key
func (PerformanceCacheKey) RealtimeTeamDistrict(userId int64) string {
	return "performance:realtime:team_district:" + gconv.String(userId)
}

// RealtimeStats 用户实时统计缓存Key
func (PerformanceCacheKey) RealtimeStats(userId int64) string {
	return "performance:realtime:stats:" + gconv.String(userId)
}
