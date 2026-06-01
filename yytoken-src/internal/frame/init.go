package frame

import (
	"XWFrame/internal/frame/cache"
	"XWFrame/internal/frame/config"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"
	"XWFrame/pkg/external/cobo"
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
)

var GlobalCoboClient *cobo.CoboClient

// Init 初始化框架
func Init(ctx context.Context) error {
	// 0. 初始化时区（最优先）
	if err := initTimezone(ctx); err != nil {
		return err
	}

	// 1. 初始化配置
	if err := config.Init(ctx); err != nil {
		return err
	}

	// 2. 初始化数据库
	if err := db.Init(ctx); err != nil {
		return err
	}

	// 3. 初始化缓存
	if err := cache.InitMemoryCache(ctx); err != nil {
		return err
	}
	if err := cache.InitRedisCache(ctx); err != nil {
		return err
	}

	// 5. 初始化Cobo客户端
	if err := initCoboClient(ctx); err != nil {
		return err
	}

	// 6. 检查 mint_group 合约同步
	if err := checkMintGroupContractSync(ctx); err != nil {
		g.Log().Warningf(ctx, "mint_group合约同步检查失败: %v", err)
	}

	g.Log().Info(ctx, "框架初始化完成")
	return nil
}

// initCoboClient 初始化Cobo客户端
func initCoboClient(ctx context.Context) error {
	coboConfig, err := config.GetCoboConfig(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "获取Cobo配置失败，跳过Cobo客户端初始化: %v", err)
		return nil
	}

	// API密钥为空时跳过初始化（用于venus-cobo等不需要Cobo API的场景）
	if coboConfig.APISecret == "" {
		g.Log().Warning(ctx, "Cobo API密钥未配置，跳过Cobo客户端初始化")
		return nil
	}

	client, err := cobo.NewCoboClient(&cobo.Config{
		APISecret: coboConfig.APISecret,
		Env:       coboConfig.Env,
		Timeout:   coboConfig.Timeout,
	})
	if err != nil {
		g.Log().Warningf(ctx, "Cobo配置无效，跳过Cobo客户端初始化: %v", err)
		return nil
	}

	GlobalCoboClient = client
	g.Log().Info(ctx, "Cobo客户端初始化完成")
	return nil
}

// GetCoboClient 获取全局Cobo客户端
func GetCoboClient() *cobo.CoboClient {
	return GlobalCoboClient
}

// initTimezone 初始化全局时区
func initTimezone(ctx context.Context) error {
	// 加载时区
	location, err := time.LoadLocation(consts.TimezoneUTC8)
	if err != nil {
		g.Log().Errorf(ctx, "加载时区失败: %v", err)
		return err
	}

	// 设置GoFrame时间组件的默认时区
	gtime.SetTimeZone(consts.TimezoneUTC8)

	// 设置Go标准库的默认时区
	time.Local = location

	g.Log().Infof(ctx, "时区初始化完成: %s", consts.TimezoneUTC8)
	return nil
}

// checkMintGroupContractSync 检查配置文件中的 mint_group 合约是否与数据库活跃合约一致
// 如果不一致，自动触发 RefreshToken 同步
func checkMintGroupContractSync(ctx context.Context) error {
	configContract := g.Cfg().MustGet(ctx, "blockchain.contracts.mint_group").String()
	if configContract == "" {
		g.Log().Debug(ctx, "[MintGroupSync] mint_group 合约未配置，跳过同步检查")
		return nil
	}

	var dbActiveContract *struct {
		ContractAddress string `json:"contract_address"`
	}
	err := db.GetDB().Model("apg_group_contract").
		Where("is_active", true).
		Limit(1).
		Scan(&dbActiveContract)
	if err != nil {
		return err
	}

	dbContractAddr := ""
	if dbActiveContract != nil {
		dbContractAddr = dbActiveContract.ContractAddress
	}

	needSync := dbContractAddr == "" || !strings.EqualFold(dbContractAddr, configContract)

	if !needSync {
		g.Log().Debugf(ctx, "[MintGroupSync] 合约地址一致 (%s)，无需同步", configContract)
		return nil
	}

	g.Log().Infof(ctx, "[MintGroupSync] 检测到合约变更: DB=%s -> Config=%s, 开始同步...",
		dbContractAddr, configContract)

	go func() {
		syncCtx := context.Background()
		if mintGroupSyncCallback != nil {
			if err := mintGroupSyncCallback(syncCtx); err != nil {
				g.Log().Errorf(syncCtx, "[MintGroupSync] 同步失败: %v", err)
			} else {
				g.Log().Info(syncCtx, "[MintGroupSync] 同步完成")
			}
		}
	}()

	return nil
}

// mintGroupSyncCallback 用于注册 mint_group 合约同步回调，避免循环依赖
var mintGroupSyncCallback func(ctx context.Context) error

// RegisterMintGroupSyncCallback 注册同步回调函数
func RegisterMintGroupSyncCallback(callback func(ctx context.Context) error) {
	mintGroupSyncCallback = callback
}
