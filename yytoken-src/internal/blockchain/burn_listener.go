package blockchain

import (
	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/shopspring/decimal"
)

// BurnEventListener Swap销毁事件监听器
type BurnEventListener struct {
	ctx             context.Context
	db              gdb.DB
	cache           *gcache.Cache
	rpcClient       *RpcClient
	config          *EventListenerConfig
	contractAddress common.Address
	eventID         common.Hash
	burnRepository  repository.IBurnRepository
	snapshotDao     *dao.ApgBurnSnapshotDao
}

// NewBurnEventListener 创建Swap销毁事件监听器
func NewBurnEventListener(ctx context.Context, db gdb.DB, cache *gcache.Cache, rpcClient *RpcClient, config *EventListenerConfig) *BurnEventListener {
	return &BurnEventListener{
		ctx:             ctx,
		db:              db,
		cache:           cache,
		rpcClient:       rpcClient,
		config:          config,
		contractAddress: common.HexToAddress(config.ContractAPG),
		eventID:         common.HexToHash(consts.BlockchainEventIDBurn),
		burnRepository:  repository.NewBurnRepository(),
		snapshotDao:     dao.NewApgBurnSnapshotDao(),
	}
}

// processBurnEvent 处理单个Swap销毁事件
func (l *BurnEventListener) processBurnEvent(ctx context.Context, event types.Log) error {
	// 解析事件
	from := common.BytesToAddress(event.Topics[1].Bytes())
	fromAddress := strings.ToLower(from.Hex())

	// 解析data字段（所有金额都是链上精度）
	data := event.Data
	if len(data) < 96 {
		return fmt.Errorf("事件数据长度不足: %d", len(data))
	}

	burnAmountRaw := new(big.Int).SetBytes(data[0:32])       // 销毁数量（Wei）
	marketingAmountRaw := new(big.Int).SetBytes(data[32:64]) // 市场奖励（Wei）
	operationAmountRaw := new(big.Int).SetBytes(data[64:96]) // 运营奖励（Wei）

	// ⚠️ 金额精度转换（所有金额都需除以1e18）
	burnAmount := l.convertFromWei(burnAmountRaw)
	marketingAmount := l.convertFromWei(marketingAmountRaw)
	operationAmount := l.convertFromWei(operationAmountRaw)

	glog.Infof(ctx, "[Swap销毁监听] 销毁事件: from=%s, burn=%s APG, tx=%s",
		fromAddress, burnAmount.String(), event.TxHash.Hex())

	// 检查是否已处理
	exists, err := l.burnRepository.ExistsBurnRecordByTxHash(ctx, event.TxHash.Hex(), int(event.Index))
	if err != nil {
		return fmt.Errorf("检查重复失败: %v", err)
	}
	if exists {
		glog.Warningf(ctx, "[Swap销毁监听] 事件已处理，跳过: tx=%s, index=%d", event.TxHash.Hex(), event.Index)
		return nil
	}

	// 获取区块时间戳
	blockTimestamp, err := l.getBlockTimestamp(ctx, event.BlockNumber)
	if err != nil {
		glog.Warningf(ctx, "[Swap销毁监听] 获取区块时间失败，使用当前时间: %v", err)
		blockTimestamp = time.Now()
	}

	// 创建销毁记录
	record := &entity.ApgBurnRecord{
		FromAddress:     fromAddress,
		BurnAmount:      burnAmount,
		MarketingAmount: marketingAmount,
		OperationAmount: operationAmount,
		TxHash:          event.TxHash.Hex(),
		BlockNumber:     int64(event.BlockNumber),
		EventIndex:      int(event.Index),
		BlockTimestamp:  blockTimestamp,
		CreatedAt:       time.Now(),
	}

	// 插入记录
	if err := l.burnRepository.InsertBurnRecord(ctx, record); err != nil {
		return fmt.Errorf("插入销毁记录失败: %v", err)
	}

	// 销毁操作不需要创建assetRecord记录
	// 销毁记录仅存储在apg_burn_records表中，用于节点分红计算

	// 更新累计销毁总量到Redis缓存
	l.updateTotalBurnCache(ctx, burnAmount)

	return nil
}

// convertFromWei 金额精度转换
func (l *BurnEventListener) convertFromWei(weiValue *big.Int) decimal.Decimal {
	return decimal.NewFromBigInt(weiValue, -int32(l.config.Decimals))
}

// updateTotalBurnCache 更新累计销毁总量缓存
func (l *BurnEventListener) updateTotalBurnCache(ctx context.Context, newBurn decimal.Decimal) {
	// 从缓存获取当前累计值
	value, err := l.cache.Get(ctx, consts.BlockchainCacheKeySwapBurnTotal)
	currentTotal := decimal.Zero
	if err == nil && value != nil {
		currentTotal, _ = decimal.NewFromString(value.String())
	}

	// 累加新的销毁量
	newTotal := currentTotal.Add(newBurn)

	// 更新缓存
	err = l.cache.Set(ctx, consts.BlockchainCacheKeySwapBurnTotal, newTotal.String(), 0)
	if err != nil {
		glog.Errorf(ctx, "[Swap销毁监听] 更新累计销毁缓存失败: %v", err)
	}
}

// getBlockTimestamp 获取区块时间戳
func (l *BurnEventListener) getBlockTimestamp(ctx context.Context, blockNumber uint64) (time.Time, error) {
	blockNum := new(big.Int).SetUint64(blockNumber)
	block, err := l.rpcClient.BlockByNumber(ctx, blockNum)
	if err != nil {
		return time.Time{}, fmt.Errorf("获取区块信息失败: %v", err)
	}

	// 区块时间戳是 Unix 时间戳（秒）
	return time.Unix(int64(block.Time()), 0), nil
}

// 以下方法已废弃，由统一扫块器处理
// getLastProcessedBlock 和 saveLastProcessedBlock 已由 UnifiedBlockScanner 接管
