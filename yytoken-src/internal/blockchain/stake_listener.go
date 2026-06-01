package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	rewardRepo "XWFrame/internal/repository/reward"
	"XWFrame/pkg/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/shopspring/decimal"
)

// StakeEventListener 质押事件监听器（支持普通质押和组合质押）
type StakeEventListener struct {
	ctx                     context.Context
	db                      gdb.DB
	cache                   *gcache.Cache
	rpcClient               *RpcClient
	config                  *EventListenerConfig
	contractAddress         common.Address // 普通质押合约地址
	eventID                 common.Hash    // 普通质押事件ID
	combinationContractAddr common.Address // 组合质押合约地址
	combinationEventID      common.Hash    // 组合质押事件ID
}

// NewStakeEventListener 创建质押事件监听器（支持普通质押和组合质押）
func NewStakeEventListener(ctx context.Context, db gdb.DB, cache *gcache.Cache, rpcClient *RpcClient, config *EventListenerConfig) *StakeEventListener {
	return &StakeEventListener{
		ctx:                     ctx,
		db:                      db,
		cache:                   cache,
		rpcClient:               rpcClient,
		config:                  config,
		contractAddress:         common.HexToAddress(config.ContractStaking),
		eventID:                 common.HexToHash(consts.BlockchainEventIDStake),
		combinationContractAddr: common.HexToAddress(config.ContractCombinationStaking),
		combinationEventID:      common.HexToHash(consts.BlockchainEventIDCombinationStake),
	}
}

// Listen 方法已废弃，由统一扫块器接管
// 事件处理逻辑见 processStakeEvent 方法

// processStakeEvent 处理单个质押事件（支持普通质押和组合质押）
func (l *StakeEventListener) processStakeEvent(ctx context.Context, event types.Log) error {
	// 判断事件类型：组合质押还是普通质押
	isCombinationStaking := event.Address == l.combinationContractAddr &&
		len(event.Topics) > 0 && event.Topics[0] == l.combinationEventID

	if isCombinationStaking {
		return l.processCombinationStakeEvent(ctx, event)
	}

	// 处理普通质押事件
	return l.processNormalStakeEvent(ctx, event)
}

// processNormalStakeEvent 处理普通质押事件
func (l *StakeEventListener) processNormalStakeEvent(ctx context.Context, event types.Log) error {
	// 解析事件数据
	user := common.BytesToAddress(event.Topics[1].Bytes())
	userAddress := strings.ToLower(user.Hex())

	// 解析data字段（链上精度：1e18）
	data := event.Data
	if len(data) < 128 {
		return fmt.Errorf("事件数据长度不足: %d", len(data))
	}

	usdtAmountRaw := new(big.Int).SetBytes(data[0:32]) // 链上值
	stakeType := new(big.Int).SetBytes(data[32:64]).Uint64()
	rexPriceRaw := new(big.Int).SetBytes(data[64:96]) // 链上值
	timestamp := new(big.Int).SetBytes(data[96:128]).Uint64()

	// ⚠️ 金额精度转换（除以1e18）
	usdtAmount := l.convertFromWei(usdtAmountRaw) // 转换为实际金额
	rexPrice := l.convertFromWei(rexPriceRaw)     // 转换为实际价格

	// 通过钱包地址查询系统用户ID
	userID, err := l.getUserIDByAddress(ctx, userAddress)
	if err != nil {
		return fmt.Errorf("查询用户ID失败: address=%s, error=%v", userAddress, err)
	}

	glog.Infof(ctx, "[质押监听] 处理普通质押事件: user=%s, user_id=%d, amount=%s USDT, type=%d, tx=%s",
		userAddress, userID, usdtAmount.String(), stakeType, event.TxHash.Hex())

	// 检查是否已处理（防止重复）
	exists, err := l.checkIfProcessed(ctx, event.TxHash.Hex(), event.BlockNumber)
	if err != nil {
		return fmt.Errorf("检查重复失败: %v", err)
	}
	if exists {
		glog.Warningf(ctx, "[质押监听] 事件已处理，跳过: tx=%s, index=%d", event.TxHash.Hex(), event.Index)
		return nil
	}

	// 开启事务处理
	return l.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 创建质押包记录
		return l.createStakingPackage(ctx, tx, &StakeEventData{
			UserID:         userID,
			WalletAddress:  userAddress,
			StakeAmount:    usdtAmount,
			StakeType:      int(stakeType),
			REXPrice:       rexPrice,
			TxHash:         event.TxHash.Hex(),
			BlockNumber:    event.BlockNumber,
			EventIndex:     int(event.Index),
			BlockTimestamp: time.Unix(int64(timestamp), 0),
			// 普通质押：组合质押字段设置
			StakeUsdt:     usdtAmount,   // 普通质押：等于总金额
			TokenAmount:   decimal.Zero, // 普通质押：无Token
			TokenContract: "",           // 普通质押：无合约地址
		})
	})
}

// processCombinationStakeEvent 处理组合质押事件
func (l *StakeEventListener) processCombinationStakeEvent(ctx context.Context, event types.Log) error {
	// 解析 indexed 参数（Topics）
	if len(event.Topics) < 4 {
		return fmt.Errorf("组合质押事件Topics长度不足: %d，期望至少4个", len(event.Topics))
	}

	user := common.BytesToAddress(event.Topics[1].Bytes())
	token := common.BytesToAddress(event.Topics[3].Bytes())

	userAddress := strings.ToLower(user.Hex())
	tokenAddress := strings.ToLower(token.Hex())

	// 解析 data 字段
	data := event.Data
	if len(data) < 256 {
		return fmt.Errorf("组合质押事件数据长度不足: %d，期望至少256字节", len(data))
	}

	// 解析 data 字段中的参数
	tokenNameOffset := new(big.Int).SetBytes(data[0:32]).Uint64()
	usdtAmountRaw := new(big.Int).SetBytes(data[32:64])       // USDT+代币的总金额
	actualUsdtAmountRaw := new(big.Int).SetBytes(data[64:96]) // 实际质押的USDT数量
	tokenAmountRaw := new(big.Int).SetBytes(data[96:128])     // 实际质押的代币数量
	rexPriceRaw := new(big.Int).SetBytes(data[160:192])
	timestampRaw := new(big.Int).SetBytes(data[192:224])

	// 解析 tokenName (string)
	var tokenName string
	if tokenNameOffset > 0 && int(tokenNameOffset) < len(data) {
		if int(tokenNameOffset)+32 <= len(data) {
			length := new(big.Int).SetBytes(data[tokenNameOffset : tokenNameOffset+32]).Uint64()
			start := tokenNameOffset + 32
			end := start + length
			if end <= uint64(len(data)) {
				tokenNameBytes := data[start:end]
				tokenName = strings.TrimRight(string(tokenNameBytes), "\x00")
				tokenName = strings.TrimSpace(tokenName)
			}
		}
	}

	// ⚠️ 金额精度转换（除以1e18）
	usdtAmount := l.convertFromWei(usdtAmountRaw)             // USDT+代币的总金额
	actualUsdtAmount := l.convertFromWei(actualUsdtAmountRaw) // 实际质押的USDT数量
	tokenAmount := l.convertFromWei(tokenAmountRaw)           // 实际质押的代币数量
	rexPrice := l.convertFromWei(rexPriceRaw)

	// 时间戳转换
	timestamp := time.Unix(timestampRaw.Int64(), 0)

	// 从映射表中获取币种简称
	tokenSymbol := tokenName

	// 通过钱包地址查询系统用户ID
	userID, err := l.getUserIDByAddress(ctx, userAddress)
	if err != nil {
		return fmt.Errorf("查询用户ID失败: address=%s, error=%v", userAddress, err)
	}

	glog.Infof(ctx, "[质押监听] 处理组合质押事件: user=%s, user_id=%d, usdt=%s, token=%s(%s), token_address=%s, tx=%s",
		userAddress, userID, actualUsdtAmount.String(), tokenAmount.String(), tokenSymbol, tokenAddress, event.TxHash.Hex())

	// 检查是否已处理（防止重复）
	exists, err := l.checkIfProcessed(ctx, event.TxHash.Hex(), event.BlockNumber)
	if err != nil {
		return fmt.Errorf("检查重复失败: %v", err)
	}
	if exists {
		glog.Warningf(ctx, "[质押监听] 组合质押事件已处理，跳过: tx=%s, index=%d", event.TxHash.Hex(), event.Index)
		return nil
	}

	// 开启事务处理
	return l.db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 创建质押包记录（组合质押）
		return l.createStakingPackage(ctx, tx, &StakeEventData{
			UserID:         userID,
			WalletAddress:  userAddress,
			StakeAmount:    usdtAmount, // 组合质押：USDT+代币的总金额
			StakeType:      consts.StakeTypeCombinationStaking,
			REXPrice:       rexPrice,
			TxHash:         event.TxHash.Hex(),
			BlockNumber:    event.BlockNumber,
			EventIndex:     int(event.Index),
			BlockTimestamp: timestamp,
			// 组合质押特有字段
			StakeUsdt:     actualUsdtAmount, // 实际质押的USDT数量
			TokenAmount:   tokenAmount,      // 实际质押的代币数量
			TokenContract: tokenAddress,     // Token合约地址
			TokenSymbol:   tokenSymbol,      // Token简称
		})
	})
}

// StakeEventData 质押事件数据（支持普通质押和组合质押）
type StakeEventData struct {
	UserID        int64
	WalletAddress string
	StakeAmount   decimal.Decimal // 质押总金额（普通质押=USDT金额，组合质押=USDT+Token总金额）
	StakeType     int             // 质押类型（业务类型）
	// DailyYieldRateOverride 可选：覆盖默认日释放率（默认 consts.StakeDailyYieldRate）
	// 说明：用于特殊质押（节点购买）需要日释放率=0的场景。
	DailyYieldRateOverride *decimal.Decimal
	REXPrice               decimal.Decimal
	TxHash                 string
	BlockNumber            uint64
	EventIndex             int
	BlockTimestamp         time.Time
	// 组合质押特有字段（普通质押时：StakeUsdt=StakeAmount，TokenAmount=0）
	StakeUsdt     decimal.Decimal // 实际质押的USDT数量
	TokenAmount   decimal.Decimal // 实际质押的Token数量
	TokenContract string          // Token合约地址
	TokenSymbol   string          // Token简称
}

// createStakingPackage 创建质押包记录
func (l *StakeEventListener) createStakingPackage(ctx context.Context, tx gdb.TX, data *StakeEventData) error {
	// 1. 生成包编号（使用雪花ID）
	packageNo := fmt.Sprintf("PKG%s", utils.GenerateSnowflakeId())

	// 2. 计算算力值和额度（根据需求文档：默认两倍算力和两倍额度）
	stakeAmount := data.StakeAmount
	powerMultiplier := decimal.NewFromFloat(consts.StakePowerMultiplier) // 2.0
	quotaMultiplier := decimal.NewFromFloat(consts.StakeQuotaMultiplier) // 2.0
	dailyYieldRate := decimal.NewFromFloat(consts.StakeDailyYieldRate)   // 0.01
	if data.DailyYieldRateOverride != nil {
		dailyYieldRate = *data.DailyYieldRateOverride
	}

	powerValue := stakeAmount.Mul(powerMultiplier)       // 质押金额 × 2
	totalQuota := stakeAmount.Mul(quotaMultiplier)       // 质押金额 × 2（动静两倍出局）
	maxStaticRelease := stakeAmount.Mul(quotaMultiplier) // 最大静态释放 = 质押金额 × 2（如日释放率=0则置为0）

	// 计算理论总释放天数 = 最大静态释放 / (质押金额 × 日收益率)
	// 例如：质押金额×2 / (质押金额×1%) = 2 / 0.01 = 200天
	dailyRelease := stakeAmount.Mul(dailyYieldRate) // 每日静态释放 = 质押金额 × 日收益率
	totalDays := decimal.Zero
	var expectedEndTime *time.Time
	if dailyYieldRate.LessThanOrEqual(decimal.Zero) || dailyRelease.LessThanOrEqual(decimal.Zero) {
		// 日释放率为0：不产生静态释放，避免除零
		maxStaticRelease = decimal.Zero
		totalDays = decimal.Zero
		expectedEndTime = nil
	} else {
		totalDays = maxStaticRelease.DivRound(dailyRelease, consts.TokenPrecision) // 总天数 = 最大静态释放 / 每日释放
		totalDaysInt := int(totalDays.IntPart())
		end := data.BlockTimestamp.AddDate(0, 0, totalDaysInt)
		expectedEndTime = &end
	}

	// 3. 创建质押包实体
	// 将链上类型转换为业务类型，并保留链上原值到 stake_type_chain
	businessStakeType := l.convertChainTypeToBusinessType(data.StakeType)

	stakingPackage := &rewardEntity.StakingPackageEntity{
		UserID:           data.UserID,
		PackageNo:        packageNo,
		StakeAmount:      stakeAmount,
		PowerValue:       powerValue,
		StakeType:        int(data.StakeType), // 业务类型
		StakeTypeChain:   int(data.StakeType),
		PowerMultiplier:  powerMultiplier,
		DailyYieldRate:   dailyYieldRate,
		TotalQuota:       totalQuota,
		MaxStaticRelease: maxStaticRelease,
		ReleasedStatic:   decimal.Zero,
		DaysElapsed:      decimal.Zero, // 刚创建时，已释放天数为0
		RemainingDays:    totalDays,
		TotalDays:        totalDays,
		StartTime:        data.BlockTimestamp,
		Status:           consts.StakingStatusForceExpired,
		TxHash:           data.TxHash,
		BlockNumber:      int64(data.BlockNumber),
		ExpectedEndTime:  expectedEndTime,
		// 组合质押字段
		// 普通质押：stake_usdt = stake_amount, stake_token = 0, token_contract = ""
		// 组合质押：stake_usdt = 实际USDT数量, stake_token = Token数量, token_contract = Token合约地址
		StakeUsdt:     data.StakeUsdt,     // 质押USDT数量（普通质押时等于StakeAmount）
		StakeToken:    data.TokenAmount,   // 质押Token数量（组合质押时才有值）
		TokenContract: data.TokenContract, // Token合约地址（组合质押时才有值）
		TokenSymbol:   data.TokenSymbol,   // Token简称（组合质押时才有值）
	}

	// 4. 插入质押包记录（通过Repository调用DAO）
	stakingRepo := rewardRepo.NewStakingPackageRepository()
	err := stakingRepo.Create(ctx, tx, stakingPackage)
	if err != nil {
		return fmt.Errorf("创建质押包失败: %v", err)
	}

	glog.Infof(ctx, "[质押创建] 用户 %d 创建质押包 %s，金额 %s USDT，算力 %s，额度 %s",
		data.UserID, packageNo, stakeAmount.String(), powerValue.String(), totalQuota.String())

	// 5. 更新用户额度（增加额度）
	// 传入最大静态释放金额（已在 maxStaticRelease 中计算好）
	quotaRepo := rewardRepo.NewQuotaRepository()
	err = quotaRepo.AddQuota(ctx, tx, data.UserID, stakeAmount, powerValue, totalQuota, maxStaticRelease, packageNo, stakingPackage.Id)
	if err != nil {
		return fmt.Errorf("更新用户额度失败: %v", err)
	}

	// 6. 创建资金记录（AssetRecord，走Repository）

	// 构建链上元数据
	stakeMetadata := map[string]interface{}{
		"chain_info": map[string]interface{}{
			"tx_hash":      data.TxHash,
			"block_number": data.BlockNumber,
			"event_index":  data.EventIndex,
			"timestamp":    data.BlockTimestamp.Unix(),
		},
		"stake_info": map[string]interface{}{
			"stake_type":         data.StakeType,                     // 链上类型值
			"stake_type_name":    l.getStakeTypeName(data.StakeType), // 链上类型名称
			"business_type":      businessStakeType,                  // 业务类型值
			"business_type_name": "LP质押",                             // 业务类型名称（统一为LP质押）
			"usdt_amount":        stakeAmount.String(),
			"rex_price":          data.REXPrice.String(),
			"package_no":         packageNo,
			"power_value":        powerValue.String(),
			"total_quota":        totalQuota.String(),
		},
		"user_info": map[string]interface{}{
			"user_id":        data.UserID,
			"wallet_address": data.WalletAddress,
		},
	}

	// 将元数据转换为JSON字符串
	metadataJSON, err := json.Marshal(stakeMetadata)
	if err != nil {
		glog.Warningf(ctx, "[质押监听] 序列化元数据失败: %v", err)
		metadataJSON = []byte("{}")
	}

	assetRecord := &rewardEntity.AssetRecordEntity{
		UserID:    data.UserID,
		AssetType: consts.AssetTypeAPGUserBalance, // 使用APG余额科目
		// record_time 在 reward_init.sql 里有唯一约束 (user_id, record_time, business_type)。
		// 链上时间戳通常是“秒级”，同一秒内可能出现多笔 stake 事件，直接用 BlockTimestamp 会触发唯一键冲突。
		// 这里把 block_number + event_index 编到纳秒部分，保证同一秒内唯一且可幂等重放。
		RecordTime:   time.Unix(data.BlockTimestamp.Unix(), int64((data.BlockNumber%1_000_000)*1000+uint64(data.EventIndex%1000))),
		Amount:       stakeAmount,
		FlowType:     consts.AssetFlowTypeExpense, // 支出（质押消耗USDT）
		Status:       consts.AssetRecordStatusSettled,
		BusinessType: consts.AssetBusinessTypeStake,
		BusinessID:   stakingPackage.Id,
		Remark:       fmt.Sprintf("质押购买算力包 %s，金额 %s USDT", packageNo, stakeAmount.String()),
		Metadata:     string(metadataJSON), // 存储详细的链上元数据
	}

	assetRepo := rewardRepo.NewAssetRecordRepository()
	err = assetRepo.Create(ctx, tx, assetRecord)
	if err != nil {
		return fmt.Errorf("创建资金记录失败: %v", err)
	}

	glog.Infof(ctx, "[质押成功] 用户 %d 质押完成，包编号 %s，交易哈希 %s", data.UserID, packageNo, data.TxHash)

	return nil
}

// getUserIDByAddress 通过钱包地址查询用户ID
func (l *StakeEventListener) getUserIDByAddress(ctx context.Context, address string) (int64, error) {
	var userID int64

	// 查询用户表（地址统一转小写匹配）
	result, err := l.db.Model("user_info").Ctx(ctx).
		Where("LOWER(wallet_address) = ?", strings.ToLower(address)).
		Fields("id").
		Value()

	if err != nil {
		return 0, fmt.Errorf("查询用户失败: %v", err)
	}

	if result == nil {
		return 0, fmt.Errorf("未找到用户: %s", address)
	}

	// 转换结果类型
	if result.IsNil() {
		return 0, fmt.Errorf("未找到用户: %s", address)
	}

	// 尝试直接获取int64值
	if id := result.Int64(); id != 0 {
		userID = id
	} else {
		// 如果int64为0，尝试从字符串转换
		if idStr := result.String(); idStr != "" {
			if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
				userID = id
			} else {
				return 0, fmt.Errorf("用户ID格式错误: %v", result)
			}
		} else {
			return 0, fmt.Errorf("用户ID为空: %v", result)
		}
	}

	if userID == 0 {
		return 0, fmt.Errorf("未找到用户: %s", address)
	}

	return userID, nil
}

// checkIfProcessed 检查事件是否已处理（基于最新表结构）
// 说明：staking_package 表保留了 tx_hash 与 block_number 字段，不再使用 chain_event_index。
// 因此以 (tx_hash, block_number) 作为去重判断依据。
func (l *StakeEventListener) checkIfProcessed(ctx context.Context, txHash string, blockNumber uint64) (bool, error) {
	count, err := l.db.Model("staking_package").Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Where("block_number = ?", blockNumber).
		Count()
	return count > 0, err
}

// convertFromWei 金额精度转换：链上值（Wei）→ 实际金额
func (l *StakeEventListener) convertFromWei(weiValue *big.Int) decimal.Decimal {
	// 实际金额 = Wei值 / 10^18
	return decimal.NewFromBigInt(weiValue, -int32(l.config.Decimals))
}

// 以下方法已废弃，由统一扫块器处理
// getLastProcessedBlock 和 saveLastProcessedBlock 已由 UnifiedBlockScanner 接管

// convertChainTypeToBusinessType 将链上类型转换为业务类型
// 不管链上是LP质押还是国库质押，本地都归类为LP质押
// 特殊类型（组合质押、Mint游戏质押）保持原值
func (l *StakeEventListener) convertChainTypeToBusinessType(chainType int) int {
	switch chainType {
	case consts.StakeTypeCombinationStaking:
		return consts.StakeTypeCombinationStaking
	case consts.StakeTypeMintGame:
		// Mint游戏质押保持原类型，便于区分业务来源
		return consts.StakeTypeMintGame
	default:
		// 链上类型：0=LP质押, 1=国库质押
		// 本地业务：统一归类为LP质押 (StakeTypeLPStaking = 1)
		return consts.StakeTypeLPStaking
	}
}

// getStakeTypeName 获取质押类型名称
func (l *StakeEventListener) getStakeTypeName(stakeType int) string {
	switch stakeType {
	case consts.StakeTypeChainLPStake:
		return "LP质押"
	case consts.StakeTypeChainTreasuryStake:
		return "国库质押"
	case consts.StakeTypeCombinationStaking:
		return "组合质押"
	case consts.StakeTypeMintGame:
		return "Mint游戏质押"
	default:
		return fmt.Sprintf("未知类型(%d)", stakeType)
	}
}

// min 返回两个uint64的较小值
func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
