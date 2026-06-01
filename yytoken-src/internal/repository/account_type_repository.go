package repository

import (
	"context"
	"fmt"
	"sync"

	"XWFrame/internal/dao"
	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/frame/g"
)

// IAccountTypeRepository 账户类型仓储接口
type IAccountTypeRepository interface {
	// GetIDByType 根据 type 获取账户类型ID（自动缓存）
	GetIDByType(ctx context.Context, accountType string) (int64, error)
	// MustGetIDByType 根据 type 获取账户类型ID（如果不存在则 panic）
	MustGetIDByType(ctx context.Context, accountType string) int64
	// GetAPGUserBalanceID 获取APG用户余额账户类型ID
	GetAPGUserBalanceID(ctx context.Context) int64

	GetVSUserBalanceID(ctx context.Context) (int64, string)

	GetUSDTUserBalanceID(ctx context.Context) (int64, string)
}

// accountTypeRepository 账户类型仓储实现
type accountTypeRepository struct {
	mu             sync.RWMutex
	cache          map[string]int64 // type -> id 的映射（按需加载缓存）
	accountTypeDao dao.IAccountTypeDao
}

var (
	accountTypeRepo     IAccountTypeRepository
	accountTypeRepoOnce sync.Once
)

// NewAccountTypeRepository 创建账户类型仓储实例（单例）
func NewAccountTypeRepository() IAccountTypeRepository {
	accountTypeRepoOnce.Do(func() {
		accountTypeRepo = &accountTypeRepository{
			cache:          make(map[string]int64),
			accountTypeDao: dao.NewAccountTypeDao(),
		}
	})
	return accountTypeRepo
}

// loadByType 按需加载单个账户类型（内部方法）
func (r *accountTypeRepository) loadByType(ctx context.Context, accountType string) (int64, error) {
	// 使用写锁，因为需要修改缓存
	r.mu.Lock()
	defer r.mu.Unlock()

	// 双重检查：获取写锁后再次检查缓存（避免并发重复查询）
	if id, exists := r.cache[accountType]; exists {
		return id, nil
	}

	// 从数据库查询所有 APG 账户类型（一次性加载，避免多次数据库查询）
	accountTypes, err := r.accountTypeDao.GetAllBySymbol(ctx, "APG")
	if err != nil {
		return 0, fmt.Errorf("查询账户类型失败: %w", err)
	}

	if len(accountTypes) == 0 {
		return 0, fmt.Errorf("未找到任何账户类型数据，请先执行数据库初始化脚本")
	}

	// 将所有结果都缓存起来
	for _, at := range accountTypes {
		r.cache[at.Type] = at.Id
	}

	g.Log().Infof(ctx, "✅ 账户类型按需加载成功，共 %d 条记录", len(r.cache))

	// 再次查找目标类型
	if id, exists := r.cache[accountType]; exists {
		return id, nil
	}

	return 0, fmt.Errorf("未找到账户类型: %s", accountType)
}

// GetIDByType 根据 type 获取账户类型ID（优先从缓存读取，缓存未命中时查询数据库并缓存）
func (r *accountTypeRepository) GetIDByType(ctx context.Context, accountType string) (int64, error) {
	// 先用读锁快速检查缓存
	r.mu.RLock()
	id, exists := r.cache[accountType]
	r.mu.RUnlock()

	if exists {
		// 缓存命中，直接返回
		return id, nil
	}

	// 缓存未命中，按需加载
	return r.loadByType(ctx, accountType)
}

// MustGetIDByType 根据 type 获取账户类型ID（如果不存在则 panic）
func (r *accountTypeRepository) MustGetIDByType(ctx context.Context, accountType string) int64 {
	id, err := r.GetIDByType(ctx, accountType)
	if err != nil {
		panic(err)
	}
	return id
}

// GetAPGUserBalanceID 获取APG用户余额账户类型ID
// 优先从缓存读取，缓存未命中时自动查询数据库并缓存
func (r *accountTypeRepository) GetAPGUserBalanceID(ctx context.Context) int64 {
	return r.MustGetIDByType(ctx, consts.AssetTypeAPGUserBalance)
}

// GetVSUserBalanceID 获取VS用户余额账户类型ID
func (r *accountTypeRepository) GetVSUserBalanceID(ctx context.Context) (int64, string) {
	return 74, consts.AssetTypeVSUserBalance
}

// GetUSDTUserBalanceID 获取USDT用户余额账户类型ID
func (r *accountTypeRepository) GetUSDTUserBalanceID(ctx context.Context) (int64, string) {
	return 75, consts.AssetTypeUSDTUserBalance
}
