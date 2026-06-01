package node

import (
	"XWFrame/internal/entity"
	coboEntity "XWFrame/internal/entity/cobo"
	"XWFrame/internal/frame/config"
	"XWFrame/internal/repository"
	coboModel "XWFrame/internal/service/cobo/model"
	"XWFrame/pkg/utils"
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// INodeService 节点服务接口
type INodeService interface {
	// GetNodeInfo 查询节点信息（过滤敏感字段）
	GetNodeInfo(ctx context.Context) (*GetNodeInfoRes, error)

	// ProcessCoboWebhook 处理Cobo回调
	ProcessCoboWebhook(ctx context.Context, webhookData map[string]interface{}) error

	// GetUserPurchaseRecords 查询用户购买记录
	GetUserPurchaseRecords(ctx context.Context, userID int64) (*GetUserPurchaseRecordsRes, error)

	// GetReferralPurchaseStats 查询推荐用户购买统计
	GetReferralPurchaseStats(ctx context.Context, userID int64) (*GetReferralPurchaseStatsRes, error)

	// IncrementNodeShares 节点份额增长（业务逻辑）
	IncrementNodeShares(ctx context.Context) error
}

// nodeService 节点服务实现
type nodeService struct {
	nodeRepo           *repository.NodeRepository
	nodePriceRepo      *repository.NodePriceRepository
	purchaseRepo       *repository.NodePurchaseRepository
	depositAddressRepo *repository.UserDepositAddressRepository
	userRepo           repository.IUserRepository
}

// NewNodeService 创建节点服务实例
func NewNodeService() INodeService {
	return &nodeService{
		nodeRepo:           repository.NewNodeRepository(),
		nodePriceRepo:      repository.NewNodePriceRepository(),
		purchaseRepo:       repository.NewNodePurchaseRepository(),
		depositAddressRepo: repository.NewUserDepositAddressRepository(),
		userRepo:           repository.NewUserRepository(),
	}
}

// GetNodeInfo 查询所有节点信息（过滤敏感字段）
func (s *nodeService) GetNodeInfo(ctx context.Context) (*GetNodeInfoRes, error) {
	// 获取所有节点
	nodeEntities, err := s.nodeRepo.GetAllNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询节点信息失败: %v", err)
	}

	if len(nodeEntities) == 0 {
		return &GetNodeInfoRes{Nodes: []NodeInfo{}}, nil
	}

	priceConfigs, err := s.nodePriceRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询节点价格配置失败: %v", err)
	}

	priceByLevel := make(map[string]string, len(priceConfigs))
	for _, cfg := range priceConfigs {
		priceByLevel[strings.ToUpper(cfg.NodeLevel)] = cfg.Price.String()
	}

	// 转换为C端模型（过滤敏感字段）
	var nodes []NodeInfo
	for _, nodeEntity := range nodeEntities {
		nodeType := 0
		if cfg := coboModel.GetNodeTypeConfigByLevel(nodeEntity.NodeLevel); cfg != nil {
			nodeType = cfg.Type
		}

		price := priceByLevel[strings.ToUpper(nodeEntity.NodeLevel)]
		if price == "" {
			if cfg := coboModel.GetNodeTypeConfigByLevel(nodeEntity.NodeLevel); cfg != nil {
				price = strconv.FormatFloat(cfg.Amount, 'f', -1, 64)
			}
		}

		nodeInfo := NodeInfo{
			Id:              nodeEntity.Id,
			NodeType:        nodeType,
			NodeLevel:       nodeEntity.NodeLevel,
			Price:           price,
			Status:          nodeEntity.Status,
			PowerMultiplier: nodeEntity.PowerMultiplier,
			TotalShares:     nodeEntity.TotalShares,
			SoldShares:      nodeEntity.SoldShares,
			CreatedAt:       nodeEntity.CreatedAt,
			UpdatedAt:       nodeEntity.UpdatedAt,
		}
		nodes = append(nodes, nodeInfo)
	}

	sort.SliceStable(nodes, func(i, j int) bool {
		pi, _ := strconv.ParseFloat(nodes[i].Price, 64)
		pj, _ := strconv.ParseFloat(nodes[j].Price, 64)
		if pi == pj {
			return nodes[i].Id < nodes[j].Id
		}
		return pi < pj
	})

	return &GetNodeInfoRes{Nodes: nodes}, nil
}

// ProcessCoboWebhook 处理Cobo回调
func (s *nodeService) ProcessCoboWebhook(ctx context.Context, webhookData map[string]interface{}) error {
	// 1. 验证事件类型和状态
	eventType, ok := webhookData["type"].(string)
	if !ok || eventType != "wallets.transaction.succeeded" {
		g.Log().Info(ctx, "Cobo回调事件类型不匹配，跳过处理")
		return nil
	}

	// 获取data字段
	data, ok := webhookData["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("cobo回调数据格式错误")
	}

	// 验证交易类型
	transactionType, ok := data["type"].(string)
	if !ok || transactionType != "Deposit" {
		g.Log().Info(ctx, "Cobo回调非充值事件，跳过处理")
		return nil
	}

	// 验证交易状态
	status, ok := data["status"].(string)
	if !ok || status != "Completed" {
		g.Log().Info(ctx, "Cobo回调交易未完成，跳过处理")
		return nil
	}

	// 验证代币类型
	tokenID, ok := data["token_id"].(string)
	if !ok {
		return fmt.Errorf("cobo回调缺少token_id")
	}

	// 检查代币是否在支持列表中
	coboConfig, err := config.GetCoboConfig(ctx)
	if err != nil {
		return fmt.Errorf("获取Cobo配置失败: %v", err)
	}

	tokenSupported := false
	for _, supportedToken := range coboConfig.SupportedTokens {
		if tokenID == supportedToken {
			tokenSupported = true
			break
		}
	}

	if !tokenSupported {
		g.Log().Infof(ctx, "Cobo回调不支持的代币类型，跳过处理: %s", tokenID)
		return nil
	}

	// 2. 获取事件ID，检查是否已处理
	eventID, ok := webhookData["event_id"].(string)
	if !ok || eventID == "" {
		g.Log().Info(ctx, "Cobo回调事件ID为空，跳过处理")
		return nil
	}

	processed, err := s.purchaseRepo.CheckEventProcessed(ctx, eventID)
	if err != nil {
		return fmt.Errorf("检查事件处理状态失败: %v", err)
	}
	if processed {
		g.Log().Infof(ctx, "Cobo回调事件已处理，跳过: %s", eventID)
		return nil
	}

	// 3. 解析交易信息
	destination, ok := data["destination"].(map[string]interface{})
	if !ok {
		g.Log().Info(ctx, "Cobo回调缺少destination，跳过处理")
		return nil
	}

	toAddress, ok := destination["address"].(string)
	if !ok {
		g.Log().Info(ctx, "Cobo回调缺少目的地址，跳过处理")
		return nil
	}

	amount, ok := destination["amount"].(string)
	if !ok {
		return fmt.Errorf("解析金额失败: %v", err)
	}

	// 验证金额是否大于等于1
	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return fmt.Errorf("解析金额失败: %v", err)
	}
	if amountFloat < 1 {
		// 金额小于1，记录日志但不入账，返回成功给Cobo
		g.Log().Infof(ctx, "充值金额小于1，跳过处理: address=%s, amount=%s", toAddress, amount)
		return nil
	}

	// 4. 查询充值地址对应的用户
	depositAddress, err := s.depositAddressRepo.GetByAddress(ctx, toAddress)
	if err != nil {
		return fmt.Errorf("查询充值地址失败: %v", err)
	}
	if depositAddress == nil {
		g.Log().Infof(ctx, "Cobo回调目的地址非本系统地址，跳过: %s", toAddress)
		return nil
	}

	// 5. 解析来源地址
	source, ok := data["source"].(map[string]interface{})
	if !ok {
		g.Log().Info(ctx, "Cobo回调缺少source，跳过处理")
		return nil
	}

	fromAddresses, ok := source["addresses"].([]interface{})
	if !ok || len(fromAddresses) == 0 {
		g.Log().Info(ctx, "Cobo回调缺少来源地址，跳过处理")
		return nil
	}

	fromAddress, ok := fromAddresses[0].(string)
	if !ok {
		g.Log().Info(ctx, "Cobo回调来源地址格式错误，跳过处理")
		return nil
	}

	// 6. 获取当前在售节点信息
	nodeEntity, err := s.nodeRepo.GetOnSaleNode(ctx)
	if err != nil {
		return fmt.Errorf("查询在售节点失败: %v", err)
	}

	// 7. 计算算力值（金额取整数部分 * 算力倍数）
	// amountFloat 已经在上面验证过了，直接使用

	amountInt := int64(math.Floor(amountFloat)) // 取整数部分
	var powerValue int64
	var nodeLevel string
	var powerMultiplier float64

	if nodeEntity != nil {
		powerMultiplier = nodeEntity.PowerMultiplier
		powerValue = int64(float64(amountInt) * powerMultiplier)
		nodeLevel = nodeEntity.NodeLevel
	}

	// 8. 解析其他交易信息
	transactionHash, ok := data["transaction_hash"].(string)
	if !ok {
		return fmt.Errorf("缺少交易哈希")
	}

	transactionID, ok := data["transaction_id"].(string)
	if !ok {
		return fmt.Errorf("缺少交易ID")
	}

	blockInfo, ok := data["block_info"].(map[string]interface{})
	var blockNumber int64
	if ok {
		if bn, exists := blockInfo["block_number"]; exists {
			if bnFloat, ok := bn.(float64); ok {
				blockNumber = int64(bnFloat)
			}
		}
	}

	// 9. 解析交易时间
	updatedTimestamp, ok := data["updated_timestamp"].(float64)
	if !ok {
		return fmt.Errorf("缺少更新时间")
	}
	transactionTime := time.Unix(int64(updatedTimestamp)/1000, 0)

	// 10. 创建购买记录
	purchaseRecord := &entity.NodePurchaseRecordEntity{
		UserID:          depositAddress.UserID,
		UserAddress:     toAddress,
		FromAddress:     fromAddress,
		Amount:          amount,
		NodeLevel:       nodeLevel,
		PowerMultiplier: powerMultiplier,
		PowerValue:      powerValue,
		TransactionHash: transactionHash,
		TransactionTime: transactionTime,
		EventID:         eventID,
		TransactionID:   transactionID,
		BlockNumber:     blockNumber,
	}

	err = s.purchaseRepo.CreatePurchaseRecord(ctx, purchaseRecord)
	if err != nil {
		return fmt.Errorf("创建购买记录失败: %v", err)
	}

	return nil
}

// GetUserPurchaseRecords 查询用户购买记录
func (s *nodeService) GetUserPurchaseRecords(ctx context.Context, userID int64) (*GetUserPurchaseRecordsRes, error) {
	var records []*coboEntity.NodePurchaseEntity
	err := g.DB().Model("cobo_node_purchase").
		Ctx(ctx).
		Where("user_id = ?", userID).
		Order("start_time DESC, created_at DESC").
		Scan(&records)
	if err != nil {
		return nil, fmt.Errorf("查询用户购买记录失败: %v", err)
	}

	var recordInfos []*PurchaseRecordInfo
	for _, record := range records {
		powerMultiplier := 2.0
		if cfg := coboModel.GetNodeTypeConfig(record.NodeType); cfg != nil {
			powerMultiplier = cfg.PowerMultiplier
		}

		transactionTime := record.StartTime
		if transactionTime.IsZero() {
			transactionTime = record.CreatedAt
		}

		transactionHash := record.PackageNo
		if transactionHash == "" {
			transactionHash = fmt.Sprintf("cobo-%d", record.ID)
		}

		recordInfos = append(recordInfos, &PurchaseRecordInfo{
			TransactionTime: utils.DBTimestampToUnix(transactionTime),
			NodeType:        record.NodeType,
			Amount:          record.Amount.String(),
			PowerMultiplier: powerMultiplier,
			PowerValue:      record.PowerValue.IntPart(),
			TransactionHash: transactionHash,
			IsGift:          record.IsGift,
			GiftRemark:      record.GiftRemark,
		})
	}

	return &GetUserPurchaseRecordsRes{Records: recordInfos}, nil
}

// GetReferralPurchaseStats 查询推荐用户购买统计
func (s *nodeService) GetReferralPurchaseStats(ctx context.Context, userID int64) (*GetReferralPurchaseStatsRes, error) {
	// 1. 查询推荐用户列表
	referralUsers, err := s.userRepo.GetReferralUsers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("查询推荐用户失败: %v", err)
	}

	if len(referralUsers) == 0 {
		return &GetReferralPurchaseStatsRes{Users: []*ReferralUserStats{}}, nil
	}

	// 2. 提取用户ID列表
	var userIDs []int64
	for _, user := range referralUsers {
		userIDs = append(userIDs, user.Id)
	}

	// 3. 查询推荐用户的购买记录
	var purchaseRecords []*coboEntity.NodePurchaseEntity
	err = g.DB().Model("cobo_node_purchase").
		Ctx(ctx).
		WhereIn("user_id", userIDs).
		Scan(&purchaseRecords)
	if err != nil {
		return nil, fmt.Errorf("查询推荐用户购买记录失败: %v", err)
	}

	// 4. 按用户分组统计算力
	userPowerMap := make(map[int64]int64)
	for _, record := range purchaseRecords {
		userPowerMap[record.UserID] += record.PowerValue.IntPart()
	}

	// 5. 构建返回数据
	var userStats []*ReferralUserStats
	for _, user := range referralUsers {
		totalPower := userPowerMap[user.Id]
		userStats = append(userStats, &ReferralUserStats{
			UserAddress:  user.WalletAddress,
			RegisterTime: user.CreatedAt,
			TotalPower:   totalPower,
		})
	}

	return &GetReferralPurchaseStatsRes{Users: userStats}, nil
}

// IncrementNodeShares 节点份额增长（业务逻辑）
func (s *nodeService) IncrementNodeShares(ctx context.Context) error {
	// 1. 查询在售节点
	nodeEntity, err := s.nodeRepo.GetOnSaleNode(ctx)
	if err != nil {
		return fmt.Errorf("查询在售节点失败: %v", err)
	}

	if nodeEntity == nil {
		g.Log().Info(ctx, "[NodeService] 当前没有在售节点，跳过执行")
		return nil
	}

	// 2. 检查剩余份额 > 最大增长
	remainingShares := nodeEntity.TotalShares - nodeEntity.SoldShares
	if remainingShares <= nodeEntity.MaxGrowth {
		g.Log().Infof(ctx, "[NodeService] 节点 %d 剩余份额 %d 小于等于最大增长 %d，跳过执行",
			nodeEntity.Id, remainingShares, nodeEntity.MaxGrowth)
		return nil
	}

	// 3. 生成随机增长值（在最小-最大之间，取最接近的100倍数）
	randomValue := math.Floor(float64(nodeEntity.MinGrowth) + float64(nodeEntity.MaxGrowth-nodeEntity.MinGrowth+1)*rand.Float64())
	increment := int64(randomValue/100) * 100 // 取最接近的100倍数

	// 4. 更新已销售份额
	newSoldShares := nodeEntity.SoldShares + increment
	err = s.nodeRepo.IncreaseSoldShares(ctx, nodeEntity.Id, newSoldShares)
	if err != nil {
		return fmt.Errorf("更新已销售份额失败: %v", err)
	}

	g.Log().Infof(ctx, "[NodeService] 节点 %d 份额增长完成，增长量: %d，新已销售份额: %d",
		nodeEntity.Id, increment, newSoldShares)

	// 5. 检查是否需要更新节点状态为售罄
	if newSoldShares >= nodeEntity.TotalShares {
		// 更新状态为售罄（状态3）
		err = s.nodeRepo.UpdateNodeStatus(ctx, nodeEntity.Id, 3)
		if err != nil {
			return fmt.Errorf("更新节点状态失败: %v", err)
		}
		g.Log().Infof(ctx, "[NodeService] 节点 %d 已售罄，状态更新为售罄", nodeEntity.Id)
	}

	return nil
}
