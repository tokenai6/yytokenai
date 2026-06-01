package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gogf/gf/v2/os/glog"
)

// RpcClient RPC客户端封装（支持多节点容错）
type RpcClient struct {
	endpoints []string            // RPC节点列表
	clients   []*ethclient.Client // 客户端列表
	timeout   time.Duration       // 超时时间
	mu        sync.RWMutex        // 读写锁
	current   int                 // 当前使用的节点索引
}

// NewRpcClient 创建RPC客户端
func NewRpcClient(endpoints []string, timeout time.Duration, requestsPerSecond int, burst int) (*RpcClient, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("RPC节点列表为空")
	}

	client := &RpcClient{
		endpoints: endpoints,
		clients:   make([]*ethclient.Client, len(endpoints)),
		timeout:   timeout,
		current:   0,
	}

	// 初始化所有RPC客户端
	for i, endpoint := range endpoints {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		ethClient, err := ethclient.DialContext(ctx, endpoint)
		cancel()

		if err != nil {
			glog.Warningf(context.Background(), "连接RPC节点失败 [%d] %s: %v", i, endpoint, err)
			continue
		}

		client.clients[i] = ethClient
		glog.Infof(context.Background(), "✅ 连接RPC节点成功 [%d] %s", i, endpoint)
	}

	// 检查是否至少有一个节点可用
	hasAvailable := false
	for _, c := range client.clients {
		if c != nil {
			hasAvailable = true
			break
		}
	}

	if !hasAvailable {
		return nil, fmt.Errorf("所有RPC节点连接失败")
	}

	return client, nil
}

// getClient 获取当前可用的客户端（自动故障切换）
func (c *RpcClient) getClient() (*ethclient.Client, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 从当前节点开始尝试
	for i := 0; i < len(c.clients); i++ {
		idx := (c.current + i) % len(c.clients)
		if c.clients[idx] != nil {
			return c.clients[idx], nil
		}
	}

	return nil, fmt.Errorf("没有可用的RPC节点")
}

// switchToNext 切换到下一个节点
func (c *RpcClient) switchToNext() {
	c.mu.Lock()
	defer c.mu.Unlock()

	oldIdx := c.current
	c.current = (c.current + 1) % len(c.clients)

	glog.Warningf(context.Background(), "切换RPC节点: [%d] %s -> [%d] %s",
		oldIdx, c.endpoints[oldIdx],
		c.current, c.endpoints[c.current])
}

// BlockNumber 获取最新区块号
func (c *RpcClient) BlockNumber(ctx context.Context) (uint64, error) {

	client, err := c.getClient()
	if err != nil {
		return 0, err
	}

	// 带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	blockNum, err := client.BlockNumber(execCtx)
	if err != nil {
		// 失败时切换节点
		c.switchToNext()
		return 0, fmt.Errorf("获取区块号失败: %v", err)
	}

	return blockNum, nil
}

// FilterLogs 查询事件日志
func (c *RpcClient) FilterLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {

	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	// 带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	logs, err := client.FilterLogs(execCtx, query)
	if err != nil {
		// 失败时切换节点
		c.switchToNext()
		return nil, fmt.Errorf("查询事件失败: %v", err)
	}

	return logs, nil
}

// BlockByNumber 获取区块信息
func (c *RpcClient) BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error) {

	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	// 带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	block, err := client.BlockByNumber(execCtx, number)
	if err != nil {
		// 失败时切换节点
		c.switchToNext()
		return nil, fmt.Errorf("获取区块信息失败: %v", err)
	}

	return block, nil
}

// CallContract 调用合约方法（只读）
func (c *RpcClient) CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {

	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	// 带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	result, err := client.CallContract(execCtx, msg, blockNumber)
	if err != nil {
		// 失败时切换节点
		c.switchToNext()
		return nil, fmt.Errorf("调用合约失败: %v", err)
	}

	return result, nil
}

// TransactionReceipt 通过交易hash获取交易回执
func (c *RpcClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, err
	}

	// 带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	receipt, err := client.TransactionReceipt(execCtx, txHash)
	if err != nil {
		// 失败时切换节点
		c.switchToNext()
		return nil, fmt.Errorf("获取交易回执失败: %v", err)
	}

	return receipt, nil
}

// GetLogsByTxHash 通过交易hash获取事件日志
func (c *RpcClient) GetLogsByTxHash(ctx context.Context, txHash string) ([]types.Log, error) {
	// 将字符串hash转换为common.Hash
	hash := common.HexToHash(txHash)

	// 获取交易回执
	receipt, err := c.TransactionReceipt(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("获取交易回执失败: %v", err)
	}

	// 转换日志类型：[]*types.Log -> []types.Log
	logs := make([]types.Log, 0, len(receipt.Logs))
	for _, log := range receipt.Logs {
		if log != nil {
			logs = append(logs, *log)
		}
	}

	return logs, nil
}

// Close 关闭所有客户端连接
func (c *RpcClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, client := range c.clients {
		if client != nil {
			client.Close()
			glog.Infof(context.Background(), "关闭RPC连接 [%d] %s", i, c.endpoints[i])
		}
	}
}
