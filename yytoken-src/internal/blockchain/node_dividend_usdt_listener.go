package blockchain

import (
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

// NodeDividendUsdtListener 节点分红USDT转账事件监听器
type NodeDividendUsdtListener struct {
	ctx                   context.Context
	db                    gdb.DB
	cache                 *gcache.Cache
	rpcClient             *RpcClient
	config                *EventListenerConfig
	contractAddress       common.Address // USDT合约地址
	eventID               common.Hash    // Transfer事件ID
	treasuryAddress       common.Address // 国库合约地址
	nodeDividendAddress   common.Address // 节点收税地址
	transferRepo          repository.INodeDividendUsdtTransferRepository
}

// NewNodeDividendUsdtListener 创建节点分红USDT转账事件监听器
func NewNodeDividendUsdtListener(
	ctx context.Context,
	db gdb.DB,
	cache *gcache.Cache,
	rpcClient *RpcClient,
	config *EventListenerConfig,
) *NodeDividendUsdtListener {
	listener := &NodeDividendUsdtListener{
		ctx:                 ctx,
		db:                  db,
		cache:               cache,
		rpcClient:           rpcClient,
		config:              config,
		contractAddress:     common.HexToAddress(config.ContractUSDT),
		eventID:             common.HexToHash(consts.BlockchainEventIDTransfer),
		treasuryAddress:     common.HexToAddress(config.TreasuryAddress),
		nodeDividendAddress: common.HexToAddress(config.NodeDividendAddress),
		transferRepo:        repository.NewNodeDividendUsdtTransferRepository(),
	}

	glog.Infof(ctx, "[节点分红USDT监听] 初始化完成 - 国库地址: %s, 节点收税地址: %s",
		strings.ToLower(listener.treasuryAddress.Hex()),
		strings.ToLower(listener.nodeDividendAddress.Hex()))

	return listener
}

// processTransferEvent 处理单个Transfer事件
func (l *NodeDividendUsdtListener) processTransferEvent(ctx context.Context, event types.Log) error {
	if len(event.Topics) < 3 {
		return fmt.Errorf("事件Topics长度不足: %d", len(event.Topics))
	}

	from := common.BytesToAddress(event.Topics[1].Bytes())
	to := common.BytesToAddress(event.Topics[2].Bytes())

	fromAddress := strings.ToLower(from.Hex())
	toAddress := strings.ToLower(to.Hex())
	treasuryAddr := strings.ToLower(l.treasuryAddress.Hex())
	nodeDividendAddr := strings.ToLower(l.nodeDividendAddress.Hex())

	// 筛选：只处理国库转给节点收税地址的转账
	if fromAddress != treasuryAddr || toAddress != nodeDividendAddr {
		return nil
	}

	if len(event.Data) < 32 {
		return fmt.Errorf("事件数据长度不足: %d", len(event.Data))
	}

	amountRaw := new(big.Int).SetBytes(event.Data[0:32])
	amount := l.convertFromWei(amountRaw)

	glog.Infof(ctx, "[节点分红USDT监听] ✅ 国库转账: from=%s, to=%s, amount=%s USDT, tx=%s",
		fromAddress, toAddress, amount.String(), event.TxHash.Hex())

	exists, err := l.transferRepo.ExistsTransferRecordByTxHash(ctx, event.TxHash.Hex(), int(event.Index))
	if err != nil {
		return fmt.Errorf("检查重复失败: %v", err)
	}
	if exists {
		glog.Warningf(ctx, "[节点分红USDT监听] 事件已处理，跳过: tx=%s, index=%d", event.TxHash.Hex(), event.Index)
		return nil
	}

	blockTimestamp, err := l.getBlockTimestamp(ctx, event.BlockNumber)
	if err != nil {
		return fmt.Errorf("获取区块时间戳失败: %v", err)
	}

	record := &entity.NodeDividendUsdtTransfer{
		FromAddress:    fromAddress,
		ToAddress:      toAddress,
		Amount:         amount,
		TxHash:         event.TxHash.Hex(),
		BlockNumber:    int64(event.BlockNumber),
		EventIndex:     int(event.Index),
		BlockTimestamp: blockTimestamp,
		CreatedAt:      time.Now(),
	}

	if err := l.transferRepo.InsertTransferRecord(ctx, record); err != nil {
		return fmt.Errorf("插入转账记录失败: %v", err)
	}

	glog.Infof(ctx, "[节点分红USDT监听] ✅ 转账记录已保存: amount=%s USDT, block=%d",
		amount.String(), event.BlockNumber)

	return nil
}

// convertFromWei 金额精度转换（USDT也是18位精度）
func (l *NodeDividendUsdtListener) convertFromWei(weiValue *big.Int) decimal.Decimal {
	return decimal.NewFromBigInt(weiValue, -int32(l.config.Decimals))
}

// getBlockTimestamp 获取区块时间戳
func (l *NodeDividendUsdtListener) getBlockTimestamp(ctx context.Context, blockNumber uint64) (time.Time, error) {
	blockNum := new(big.Int).SetUint64(blockNumber)
	block, err := l.rpcClient.BlockByNumber(ctx, blockNum)
	if err != nil {
		return time.Time{}, fmt.Errorf("获取区块信息失败: %v", err)
	}

	return time.Unix(int64(block.Time()), 0), nil
}
