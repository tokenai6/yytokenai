package blockchain

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

const tokenStoreBurnABI = `[{"inputs":[{"internalType":"contract IERC20","name":"tokenYY_","type":"address"},{"internalType":"contract IERC20","name":"tokenYYAI_","type":"address"},{"internalType":"address","name":"initialOwner","type":"address"}],"stateMutability":"nonpayable","type":"constructor"},{"inputs":[{"internalType":"address","name":"owner","type":"address"}],"name":"OwnableInvalidOwner","type":"error"},{"inputs":[{"internalType":"address","name":"account","type":"address"}],"name":"OwnableUnauthorizedAccount","type":"error"},{"inputs":[{"internalType":"address","name":"token","type":"address"}],"name":"SafeERC20FailedOperation","type":"error"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"previousOwner","type":"address"},{"indexed":true,"internalType":"address","name":"newOwner","type":"address"}],"name":"OwnershipTransferred","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"}],"name":"YYAIWithdrawn","type":"event"},{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"}],"name":"YYBurned","type":"event"},{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"to","type":"address"},{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"}],"name":"YYWithdrawn","type":"event"},{"inputs":[],"name":"balanceYY","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"balanceYYAI","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"burnYY","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"owner","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"renounceOwnership","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[],"name":"tokenYY","outputs":[{"internalType":"contract IERC20","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"tokenYYAI","outputs":[{"internalType":"contract IERC20","name":"","type":"address"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"newOwner","type":"address"}],"name":"transferOwnership","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"withdrawYY","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"to","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"}],"name":"withdrawYYAI","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

// ITokenStoreBurn TokenStoreBurn 合约交互接口
type ITokenStoreBurn interface {
	BurnYY(ctx context.Context, amount *big.Int) (string, error)
	WithdrawYY(ctx context.Context, to common.Address, amount *big.Int) (string, error)
	WithdrawYYAI(ctx context.Context, to common.Address, amount *big.Int) (string, error)
	BalanceYY(ctx context.Context) (*big.Int, error)
	BalanceYYAI(ctx context.Context) (*big.Int, error)
	TokenYY(ctx context.Context) (common.Address, error)
	TokenYYAI(ctx context.Context) (common.Address, error)
	Owner(ctx context.Context) (common.Address, error)
}

type tokenStoreBurn struct {
	rpcEndpoints []string
	privateKey   *ecdsa.PrivateKey
	fromAddress  common.Address
	contractAddr common.Address
	chainID      *big.Int
	rpcTimeout   time.Duration
	parsedABI    abi.ABI
}

// NewTokenStoreBurn 创建 TokenStoreBurn 合约交互实例
func NewTokenStoreBurn() (ITokenStoreBurn, error) {
	ctx := context.Background()

	// 1. RPC 节点
	rpcEndpoints := g.Cfg().MustGet(ctx, "blockchain.rpc_endpoints").Strings()
	if len(rpcEndpoints) == 0 {
		return nil, gerror.New("blockchain.rpc_endpoints not configured")
	}

	// 2. 加载私钥（从 admin/address.json）
	privateKeyHex, err := loadAdminPrivateKey(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "load admin private key failed")
	}
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, gerror.Wrap(err, "parse admin private key failed")
	}
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	fromAddress := crypto.PubkeyToAddress(*publicKey)
	g.Log().Infof(ctx, "[TokenStoreBurn] signer address: %s", fromAddress.Hex())

	// 3. 合约地址
	contractAddrStr := g.Cfg().MustGet(ctx, "blockchain.contracts.token_store_burn").String()
	if contractAddrStr == "" {
		return nil, gerror.New("blockchain.contracts.token_store_burn not configured")
	}
	contractAddr := common.HexToAddress(contractAddrStr)
	g.Log().Infof(ctx, "[TokenStoreBurn] contract address: %s", contractAddr.Hex())

	// 4. ChainID
	chainIDInt := g.Cfg().MustGet(ctx, "blockchain.network.chain_id").Int64()
	chainID := big.NewInt(chainIDInt)

	// 5. RPC 超时
	rpcTimeoutSec := g.Cfg().MustGet(ctx, "blockchain.timeouts.rpc_call", 30).Int()
	rpcTimeout := time.Duration(rpcTimeoutSec) * time.Second

	// 6. 解析 ABI
	parsedABI, err := abi.JSON(strings.NewReader(tokenStoreBurnABI))
	if err != nil {
		return nil, gerror.Wrap(err, "parse TokenStoreBurn ABI failed")
	}

	return &tokenStoreBurn{
		rpcEndpoints: rpcEndpoints,
		privateKey:   privateKey,
		fromAddress:  fromAddress,
		contractAddr: contractAddr,
		chainID:      chainID,
		rpcTimeout:   rpcTimeout,
		parsedABI:    parsedABI,
	}, nil
}

// loadAdminPrivateKey 从 admin/address.json 加载私钥
func loadAdminPrivateKey(ctx context.Context) (string, error) {
	path := g.Cfg().MustGet(ctx, "blockchain.token_store_burn.private_key_path", "./admin/address.json").String()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", gerror.Wrapf(err, "read admin address file failed: %s", path)
	}
	var addrFile struct {
		Address    string `json:"address"`
		PrivateKey string `json:"private_key"`
	}
	if err := json.Unmarshal(data, &addrFile); err != nil {
		return "", gerror.Wrap(err, "parse admin address file failed")
	}
	if addrFile.PrivateKey == "" {
		return "", gerror.New("admin address file missing private_key")
	}
	return strings.TrimPrefix(addrFile.PrivateKey, "0x"), nil
}

func (t *tokenStoreBurn) connectRPC(ctx context.Context) (*ethclient.Client, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, t.rpcTimeout)
	defer cancel()

	var lastErr error
	for i, endpoint := range t.rpcEndpoints {
		client, err := ethclient.DialContext(timeoutCtx, endpoint)
		if err != nil {
			g.Log().Warningf(ctx, "[TokenStoreBurn] RPC node #%d (%s) dial failed: %v", i+1, endpoint, err)
			lastErr = err
			continue
		}
		if _, err := client.ChainID(timeoutCtx); err != nil {
			g.Log().Warningf(ctx, "[TokenStoreBurn] RPC node #%d (%s) chainID failed: %v", i+1, endpoint, err)
			client.Close()
			lastErr = err
			continue
		}
		return client, nil
	}
	return nil, gerror.Wrap(lastErr, "all RPC nodes unavailable")
}

// callView 通用 view 调用
func (t *tokenStoreBurn) callView(ctx context.Context, methodName string, result interface{}) error {
	client, err := t.connectRPC(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	callData, err := t.parsedABI.Pack(methodName)
	if err != nil {
		return gerror.Wrapf(err, "pack %s failed", methodName)
	}

	rpcCtx, cancel := context.WithTimeout(ctx, t.rpcTimeout)
	defer cancel()

	data, err := client.CallContract(rpcCtx, ethereum.CallMsg{
		To:   &t.contractAddr,
		Data: callData,
	}, nil)
	if err != nil {
		return gerror.Wrapf(err, "call %s failed", methodName)
	}

	if err := t.parsedABI.UnpackIntoInterface(result, methodName, data); err != nil {
		return gerror.Wrapf(err, "unpack %s result failed", methodName)
	}
	return nil
}

// sendTransaction 通用交易发送
func (t *tokenStoreBurn) sendTransaction(ctx context.Context, methodName string, args ...interface{}) (string, error) {
	g.Log().Infof(ctx, "[TokenStoreBurn] calling %s...", methodName)

	client, err := t.connectRPC(ctx)
	if err != nil {
		return "", err
	}
	defer client.Close()

	callData, err := t.parsedABI.Pack(methodName, args...)
	if err != nil {
		return "", gerror.Wrapf(err, "pack %s failed", methodName)
	}

	rpcCtx, cancel := context.WithTimeout(ctx, t.rpcTimeout)
	defer cancel()

	nonce, err := client.PendingNonceAt(rpcCtx, t.fromAddress)
	if err != nil {
		return "", gerror.Wrap(err, "get nonce failed")
	}

	gasPrice, err := client.SuggestGasPrice(rpcCtx)
	if err != nil {
		return "", gerror.Wrap(err, "get gas price failed")
	}

	gasLimit := uint64(300000)

	g.Log().Infof(ctx, "[TokenStoreBurn] %s nonce=%d gasPrice=%s gasLimit=%d", methodName, nonce, gasPrice.String(), gasLimit)

	tx := types.NewTransaction(nonce, t.contractAddr, big.NewInt(0), gasLimit, gasPrice, callData)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(t.chainID), t.privateKey)
	if err != nil {
		return "", gerror.Wrap(err, "sign tx failed")
	}

	if err := client.SendTransaction(rpcCtx, signedTx); err != nil {
		if strings.Contains(err.Error(), "already known") {
			g.Log().Warningf(ctx, "[TokenStoreBurn] tx already known: %s", signedTx.Hash().Hex())
			return signedTx.Hash().Hex(), nil
		}
		return "", gerror.Wrap(err, "send tx failed")
	}

	txHash := signedTx.Hash().Hex()
	g.Log().Infof(ctx, "[TokenStoreBurn] %s txHash=%s", methodName, txHash)
	return txHash, nil
}

func (t *tokenStoreBurn) BurnYY(ctx context.Context, amount *big.Int) (string, error) {
	return t.sendTransaction(ctx, "burnYY", amount)
}

func (t *tokenStoreBurn) WithdrawYY(ctx context.Context, to common.Address, amount *big.Int) (string, error) {
	return t.sendTransaction(ctx, "withdrawYY", to, amount)
}

func (t *tokenStoreBurn) WithdrawYYAI(ctx context.Context, to common.Address, amount *big.Int) (string, error) {
	return t.sendTransaction(ctx, "withdrawYYAI", to, amount)
}

func (t *tokenStoreBurn) BalanceYY(ctx context.Context) (*big.Int, error) {
	var result *big.Int
	if err := t.callView(ctx, "balanceYY", &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (t *tokenStoreBurn) BalanceYYAI(ctx context.Context) (*big.Int, error) {
	var result *big.Int
	if err := t.callView(ctx, "balanceYYAI", &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (t *tokenStoreBurn) TokenYY(ctx context.Context) (common.Address, error) {
	var result common.Address
	if err := t.callView(ctx, "tokenYY", &result); err != nil {
		return common.Address{}, err
	}
	return result, nil
}

func (t *tokenStoreBurn) TokenYYAI(ctx context.Context) (common.Address, error) {
	var result common.Address
	if err := t.callView(ctx, "tokenYYAI", &result); err != nil {
		return common.Address{}, err
	}
	return result, nil
}

func (t *tokenStoreBurn) Owner(ctx context.Context) (common.Address, error) {
	var result common.Address
	if err := t.callView(ctx, "owner", &result); err != nil {
		return common.Address{}, err
	}
	return result, nil
}
