package blockchain

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strings"
	"time"

	"XWFrame/internal/frame/consts"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// MarketCapController 合约ABI定义
const stabilizeABI = `[
	{
		"inputs": [],
		"name": "stabilizeREX",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "stabilizeAPG",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": false, "internalType": "uint256", "name": "usdtAmount", "type": "uint256"},
			{"indexed": false, "internalType": "uint256", "name": "rexBurned", "type": "uint256"}
		],
		"name": "REXStabilized",
		"type": "event"
	},
	{
		"anonymous": false,
		"inputs": [
			{"indexed": false, "internalType": "uint256", "name": "rexMinted", "type": "uint256"},
			{"indexed": false, "internalType": "uint256", "name": "apgBurned", "type": "uint256"}
		],
		"name": "APGStabilized",
		"type": "event"
	}
]`

type TransactionReceiptInfo struct {
	Success     bool
	BlockNumber uint64
	GasUsed     uint64
	GasPrice    uint64
}

type ITransactionSender interface {
	CallStabilizeAPG(ctx context.Context) (string, error)
	CallStabilizeREX(ctx context.Context) (string, error)
	WaitForTransactionReceipt(ctx context.Context, txHash string) (bool, error)
	WaitForTransactionReceiptWithDetails(ctx context.Context, txHash string) (*TransactionReceiptInfo, error)
}

type transactionSender struct {
	rpcEndpoints      []string
	privateKeyAPG     *ecdsa.PrivateKey  // 用于 stabilizeAPG
	privateKeyREX     *ecdsa.PrivateKey  // 用于 stabilizeREX
	chainID           *big.Int
	rpcTimeout        time.Duration
}

func (s *transactionSender) getBscscanURL(txHash string) string {
	chainID := s.chainID.Int64()
	if chainID == 56 {
		return "https://bscscan.com/tx/" + txHash
	}
	return "https://testnet.bscscan.com/tx/" + txHash
}

func NewTransactionSender() ITransactionSender {
	ctx := context.Background()

	// 1. 获取RPC节点列表
	rpcEndpoints := g.Cfg().MustGet(ctx, "blockchain.rpc_endpoints").Strings()
	if len(rpcEndpoints) == 0 {
		g.Log().Fatalf(ctx, "RPC节点未配置")
	}

	// 2. 加载 REX 私钥 (用于 stabilizeREX)
	privateKeyREXHex := g.Cfg().MustGet(ctx, "blockchain.apg.trade_private_key").String()
	if privateKeyREXHex == "" {
		g.Log().Fatalf(ctx, "REX私钥未配置")
	}
	privateKeyREX, err := crypto.HexToECDSA(privateKeyREXHex)
	if err != nil {
		g.Log().Fatalf(ctx, "加载REX私钥失败: %v", err)
	}
	publicKeyREX := privateKeyREX.Public().(*ecdsa.PublicKey)
	addressREX := crypto.PubkeyToAddress(*publicKeyREX)
	g.Log().Infof(ctx, "[交易发送器] REX私钥地址: %s", addressREX.Hex())

	// 3. 加载 APG 私钥 (用于 stabilizeAPG)
	privateKeyAPGHex := g.Cfg().MustGet(ctx, "blockchain.apg.apg_private_key").String()
	if privateKeyAPGHex == "" {
		g.Log().Fatalf(ctx, "APG私钥未配置")
	}
	privateKeyAPG, err := crypto.HexToECDSA(privateKeyAPGHex)
	if err != nil {
		g.Log().Fatalf(ctx, "加载APG私钥失败: %v", err)
	}
	publicKeyAPG := privateKeyAPG.Public().(*ecdsa.PublicKey)
	addressAPG := crypto.PubkeyToAddress(*publicKeyAPG)
	g.Log().Infof(ctx, "[交易发送器] APG私钥地址: %s", addressAPG.Hex())

	// 4. 获取链ID
	chainIDInt := g.Cfg().MustGet(ctx, "blockchain.network.chain_id").Int64()
	chainID := big.NewInt(chainIDInt)

	// 5. 获取RPC超时配置
	rpcTimeoutSec := g.Cfg().MustGet(ctx, "blockchain.timeouts.rpc_call", consts.BlockchainDefaultRPCTimeout).Int()
	rpcTimeout := time.Duration(rpcTimeoutSec) * time.Second

	g.Log().Infof(ctx, "[交易发送器] 初始化完成，ChainID: %d, RPC超时: %v", chainIDInt, rpcTimeout)

	return &transactionSender{
		rpcEndpoints:  rpcEndpoints,
		privateKeyAPG: privateKeyAPG,
		privateKeyREX: privateKeyREX,
		chainID:       chainID,
		rpcTimeout:    rpcTimeout,
	}
}

// connectRPC 连接RPC节点(支持多节点容错)
func (s *transactionSender) connectRPC(ctx context.Context) (*ethclient.Client, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.rpcTimeout)
	defer cancel()

	var lastErr error
	for i, endpoint := range s.rpcEndpoints {
		client, err := ethclient.DialContext(timeoutCtx, endpoint)
		if err != nil {
			g.Log().Warningf(ctx, "[交易发送] RPC节点 #%d (%s) 连接失败: %v", i+1, endpoint, err)
			lastErr = err
			continue
		}

		// 测试连接
		if _, err := client.ChainID(timeoutCtx); err != nil {
			g.Log().Warningf(ctx, "[交易发送] RPC节点 #%d (%s) 测试失败: %v", i+1, endpoint, err)
			client.Close()
			lastErr = err
			continue
		}

		g.Log().Infof(ctx, "[交易发送] 成功连接到RPC节点 #%d: %s", i+1, endpoint)
		return client, nil
	}

	return nil, gerror.Wrapf(lastErr, "所有RPC节点均不可用")
}

// CallStabilizeREX 调用REX价格稳定方法
func (s *transactionSender) CallStabilizeREX(ctx context.Context) (string, error) {
	return s.callStabilizeMethod(ctx, "stabilizeREX")
}

// CallStabilizeAPG 调用APG价格稳定方法
func (s *transactionSender) CallStabilizeAPG(ctx context.Context) (string, error) {
	return s.callStabilizeMethod(ctx, "stabilizeAPG")
}

// callStabilizeMethod 调用价格稳定方法的通用实现
func (s *transactionSender) callStabilizeMethod(ctx context.Context, methodName string) (string, error) {
	g.Log().Infof(ctx, "[交易发送] 准备调用 %s()...", methodName)

	// 1. 根据方法名选择私钥
	var privateKey *ecdsa.PrivateKey
	if methodName == "stabilizeREX" {
		privateKey = s.privateKeyREX
		g.Log().Infof(ctx, "[交易发送] 使用 REX 私钥")
	} else {
		privateKey = s.privateKeyAPG
		g.Log().Infof(ctx, "[交易发送] 使用 APG 私钥")
	}

	// 2. 连接RPC节点
	client, err := s.connectRPC(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	// 3. 获取合约地址
	contractAddr := g.Cfg().MustGet(ctx, "blockchain.contracts.market_cap_controller").String()
	if contractAddr == "" {
		return "", gerror.New("价格合约地址未配置")
	}

	contractAddress := common.HexToAddress(contractAddr)
	g.Log().Infof(ctx, "[交易发送] 合约地址: %s", contractAddr)

	// 4. 解析ABI并编码方法调用
	parsedABI, err := abi.JSON(strings.NewReader(stabilizeABI))
	if err != nil {
		return "", gerror.Wrapf(err, "解析ABI失败")
	}

	callData, err := parsedABI.Pack(methodName)
	if err != nil {
		return "", gerror.Wrapf(err, "编码%s失败", methodName)
	}

	// 5. 获取发送者地址
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	fromAddress := crypto.PubkeyToAddress(*publicKey)
	g.Log().Infof(ctx, "[交易发送] 发送者地址: %s", fromAddress.Hex())

	// 6. 创建带超时的上下文用于RPC调用
	rpcCtx, cancel := context.WithTimeout(ctx, s.rpcTimeout)
	defer cancel()

	// 7. 获取nonce
	nonce, err := client.PendingNonceAt(rpcCtx, fromAddress)
	if err != nil {
		return "", gerror.Wrapf(err, "获取nonce失败")
	}

	// 8. 获取gas价格
	gasPrice, err := client.SuggestGasPrice(rpcCtx)
	if err != nil {
		return "", gerror.Wrapf(err, "获取gas价格失败")
	}

	if methodName == "stabilizeREX" {
		configuredGasPrice := g.Cfg().MustGet(ctx, "blockchain.stabilize_rex_gas_price_gwei", 0).Float64()
		if configuredGasPrice > 0 {
			gasPrice = new(big.Int).SetUint64(uint64(configuredGasPrice * 1e9))
			g.Log().Infof(ctx, "[交易发送] stabilizeREX 使用配置的 GasPrice: %.2f Gwei", configuredGasPrice)
		}
	}

	// 9. 估算Gas限额
	gasLimit := uint64(500000)

	g.Log().Infof(ctx, "[交易发送] Nonce: %d, GasPrice: %s Wei, GasLimit: %d",
		nonce, gasPrice.String(), gasLimit)

	// 10. 构建交易
	tx := types.NewTransaction(
		nonce,
		contractAddress,
		big.NewInt(0),
		gasLimit,
		gasPrice,
		callData,
	)

	// 11. 签名交易
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(s.chainID), privateKey)
	if err != nil {
		return "", gerror.Wrapf(err, "签名交易失败")
	}

	// 12. 发送交易
	err = client.SendTransaction(rpcCtx, signedTx)

	// 13. 获取交易哈希
	txHash := signedTx.Hash().Hex()

	if err != nil {
		// "already known" 表示交易已在pending池,视为成功
		if strings.Contains(err.Error(), "already known") {
			g.Log().Warningf(ctx, "[交易发送] ⚠️ 交易已在pending池: %s", txHash)
			g.Log().Infof(ctx, "[交易发送] 浏览器: %s", s.getBscscanURL(txHash))
			return txHash, nil
		}
		return "", gerror.Wrapf(err, "发送交易失败")
	}

	g.Log().Infof(ctx, "[交易发送] ✅ 交易发送成功")
	g.Log().Infof(ctx, "[交易发送] TxHash: %s", txHash)
	g.Log().Infof(ctx, "[交易发送] 浏览器: %s", s.getBscscanURL(txHash))

	return txHash, nil
}

// WaitForTransactionReceipt 等待交易确认并检查执行状态
func (s *transactionSender) WaitForTransactionReceipt(ctx context.Context, txHash string) (bool, error) {
	maxRetries := 30
	retryInterval := 2 * time.Second

	g.Log().Infof(ctx, "[交易确认] 开始等待交易确认: %s", txHash)

	client, err := s.connectRPC(ctx)
	if err != nil {
		return false, err
	}
	defer client.Close()

	txHashCommon := common.HexToHash(txHash)

	for i := 0; i < maxRetries; i++ {
		rpcCtx, cancel := context.WithTimeout(ctx, s.rpcTimeout)
		receipt, err := client.TransactionReceipt(rpcCtx, txHashCommon)
		cancel()

		if err == nil {
			if receipt.Status == 1 {
				g.Log().Infof(ctx, "[交易确认] ✅ 交易执行成功 (区块: %d, GasUsed: %d)",
					receipt.BlockNumber.Uint64(), receipt.GasUsed)
				return true, nil
			} else {
				g.Log().Errorf(ctx, "[交易确认] ❌ 交易执行失败 (区块: %d, Status: %d)",
					receipt.BlockNumber.Uint64(), receipt.Status)
				return false, gerror.Newf("交易执行失败: %s", s.getBscscanURL(txHash))
			}
		}

		if i < maxRetries-1 {
			g.Log().Debugf(ctx, "[交易确认] 等待中... (%d/%d)", i+1, maxRetries)
			time.Sleep(retryInterval)
		}
	}

	g.Log().Warningf(ctx, "[交易确认] ⏱️ 超时未确认 (已等待 %v)", time.Duration(maxRetries)*retryInterval)
	return false, gerror.Newf("交易确认超时: %s", s.getBscscanURL(txHash))
}

func (s *transactionSender) WaitForTransactionReceiptWithDetails(ctx context.Context, txHash string) (*TransactionReceiptInfo, error) {
	maxRetries := 30
	retryInterval := 2 * time.Second

	g.Log().Infof(ctx, "[交易确认] 开始等待交易确认: %s", txHash)

	client, err := s.connectRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	txHashCommon := common.HexToHash(txHash)

	for i := 0; i < maxRetries; i++ {
		rpcCtx, cancel := context.WithTimeout(ctx, s.rpcTimeout)
		receipt, err := client.TransactionReceipt(rpcCtx, txHashCommon)
		cancel()

		if err == nil {
			info := &TransactionReceiptInfo{
				Success:     receipt.Status == 1,
				BlockNumber: receipt.BlockNumber.Uint64(),
				GasUsed:     receipt.GasUsed,
				GasPrice:    0,
			}

			if receipt.Status == 1 {
				g.Log().Infof(ctx, "[交易确认] ✅ 交易执行成功 (区块: %d, GasUsed: %d)",
					receipt.BlockNumber.Uint64(), receipt.GasUsed)

				tx, _, err := client.TransactionByHash(rpcCtx, txHashCommon)
				if err == nil && tx != nil {
					info.GasPrice = tx.GasPrice().Uint64()
				}

				return info, nil
			} else {
				g.Log().Errorf(ctx, "[交易确认] ❌ 交易执行失败 (区块: %d, Status: %d)",
					receipt.BlockNumber.Uint64(), receipt.Status)
				return info, gerror.Newf("交易执行失败: %s", s.getBscscanURL(txHash))
			}
		}

		if i < maxRetries-1 {
			g.Log().Debugf(ctx, "[交易确认] 等待中... (%d/%d)", i+1, maxRetries)
			time.Sleep(retryInterval)
		}
	}

	g.Log().Warningf(ctx, "[交易确认] ⏱️ 超时未确认 (已等待 %v)", time.Duration(maxRetries)*retryInterval)
	return nil, gerror.Newf("交易确认超时: %s", s.getBscscanURL(txHash))
}
