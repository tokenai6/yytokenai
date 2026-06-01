package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"XWFrame/internal/frame/consts"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/glog"
)

// ApgMintBlockScanner APG Mint 专用扫块器
// 独立扫描 MintGroup 合约事件，与主扫块器并行运行
type ApgMintBlockScanner struct {
	ctx             context.Context
	cache           *gcache.Cache
	rpcClient       *RpcClient
	config          *EventListenerConfig
	handlers        []EventHandler
	contractAddress common.Address
	cacheKey        string // Redis 缓存 key
}

// NewApgMintBlockScanner 创建 APG Mint 专用扫块器
func NewApgMintBlockScanner(
	ctx context.Context,
	cache *gcache.Cache,
	rpcClient *RpcClient,
	config *EventListenerConfig,
) *ApgMintBlockScanner {
	return NewApgMintBlockScannerWithCacheKey(ctx, cache, rpcClient, config, consts.BlockchainCacheKeyApgMintLastBlock)
}

// NewApgMintBlockScannerWithCacheKey 创建 APG Mint 专用扫块器（自定义缓存 key）
func NewApgMintBlockScannerWithCacheKey(
	ctx context.Context,
	cache *gcache.Cache,
	rpcClient *RpcClient,
	config *EventListenerConfig,
	cacheKey string,
) *ApgMintBlockScanner {
	contractAddress := common.HexToAddress(config.ContractMintGroup)

	scanner := &ApgMintBlockScanner{
		ctx:             ctx,
		cache:           cache,
		rpcClient:       rpcClient,
		config:          config,
		handlers:        make([]EventHandler, 0),
		contractAddress: contractAddress,
		cacheKey:        cacheKey,
	}

	glog.Infof(ctx, "[APG Mint扫块器] 初始化 - 合约地址: %s, 缓存Key: %s", contractAddress.Hex(), cacheKey)

	return scanner
}

// RegisterHandler 注册事件处理器
func (s *ApgMintBlockScanner) RegisterHandler(handler EventHandler) {
	s.handlers = append(s.handlers, handler)
	glog.Infof(s.ctx, "[APG Mint扫块器] 注册事件处理器: %s", handler.GetName())
}

// Scan 执行区块扫描（简化策略：一次查询所有新区块）
func (s *ApgMintBlockScanner) Scan(ctx context.Context) error {
	if len(s.handlers) == 0 {
		glog.Warning(ctx, "[APG Mint扫块器] 没有注册任何事件处理器")
		return nil
	}

	lastBlock := s.getLastProcessedBlock(ctx)

	latestBlock, err := s.rpcClient.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("获取最新区块失败: %v", err)
	}
	latestBlock = latestBlock - uint64(s.config.ConfirmBlocks)

	if lastBlock >= latestBlock {
		return nil
	}

	fromBlock := lastBlock + 1
	blockCount := latestBlock - fromBlock + 1

	// 收集所有事件ID
	eventIDs := make([]common.Hash, 0, len(s.handlers))
	for _, handler := range s.handlers {
		eventIDs = append(eventIDs, handler.GetEventID())
	}

	glog.Infof(ctx, "[APG Mint扫块器] 扫描: %d -> %d (%d 块)", fromBlock, latestBlock, blockCount)

	// 一次性查询所有新区块
	events, err := s.rpcClient.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(latestBlock)),
		Addresses: []common.Address{s.contractAddress},
		Topics:    [][]common.Hash{eventIDs},
	})

	if err != nil {
		if isContextCanceledError(err) {
			glog.Infof(ctx, "[APG Mint扫块器] 上下文已取消")
			return nil
		}

		if isPrunedError(err) {
			glog.Warningf(ctx, "[APG Mint扫块器] 历史区块已被修剪，重置到最新")
			newStartBlock := latestBlock - uint64(s.config.ConfirmBlocks)
			s.saveLastProcessedBlock(ctx, newStartBlock)
			return nil
		}

		return fmt.Errorf("查询事件失败 [%d-%d]: %v", fromBlock, latestBlock, err)
	}

	if len(events) > 0 {
		glog.Infof(ctx, "[APG Mint扫块器] 发现 %d 个事件", len(events))
	}

	// 分发事件
	eventCount := make(map[string]int)
	for _, event := range events {
		for _, handler := range s.handlers {
			if len(event.Topics) > 0 && event.Topics[0] == handler.GetEventID() {
				if err := handler.ProcessEvent(ctx, event); err != nil {
					glog.Errorf(ctx, "[APG Mint扫块器] %s 处理失败 (tx: %s): %v",
						handler.GetName(), event.TxHash.Hex(), err)
					continue
				}
				eventCount[handler.GetName()]++
				break
			}
		}
	}

	for name, count := range eventCount {
		if count > 0 {
			glog.Infof(ctx, "[APG Mint扫块器] %s 处理 %d 个事件", name, count)
		}
	}

	s.saveLastProcessedBlock(ctx, latestBlock)
	return nil
}

// getLastProcessedBlock 获取上次处理的区块号（独立进度）
func (s *ApgMintBlockScanner) getLastProcessedBlock(ctx context.Context) uint64 {
	value, err := s.cache.Get(ctx, s.cacheKey)
	if err == nil && value != nil {
		if blockNum, err := strconv.ParseUint(value.String(), 10, 64); err == nil {
			return blockNum
		}
	}

	// 首次启动，从最新区块往前 100 个开始（参考 sell_and_stake 策略）
	latestBlock, err := s.rpcClient.BlockNumber(ctx)
	if err != nil {
		glog.Errorf(ctx, "[APG Mint扫块器] 获取最新区块失败: %v", err)
		return 0
	}

	var startBlock uint64
	if latestBlock > 100 {
		startBlock = latestBlock - 100
	} else {
		startBlock = 0
	}

	glog.Infof(ctx, "[APG Mint扫块器] 首次启动，从区块 %d 开始 (最新: %d)", startBlock, latestBlock)
	return startBlock
}

// saveLastProcessedBlock 保存处理进度（独立进度）
func (s *ApgMintBlockScanner) saveLastProcessedBlock(ctx context.Context, blockNum uint64) {
	if err := s.cache.Set(ctx, s.cacheKey, blockNum, 0); err != nil {
		glog.Errorf(ctx, "[APG Mint扫块器] 保存进度失败: %v", err)
	}
}
