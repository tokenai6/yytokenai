package cobo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"XWFrame/internal/dao"
	rewardDao "XWFrame/internal/dao/reward"
	"XWFrame/internal/entity/cobo"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	repo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/service/cobo/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

var errIdempotentRequestConflict = errors.New("idempotent request conflict")

// NodePurchaseService 节点购买服务接口
// 注意：在售节点列表、我的购买记录等复用 venus2 接口
type NodePurchaseService interface {
	// GetBalance 获取用户余额
	GetBalance(ctx context.Context, userID int64, symbol string) (*model.GetBalanceRes, error)

	// BuyNode 购买节点（使用Cobo余额）
	BuyNode(ctx context.Context, req *model.BuyNodeReq) (*model.BuyNodeRes, error)

	// GetNodeTokenGrantRecords 获取YYAI发放记录
	GetNodeTokenGrantRecords(ctx context.Context, req *model.GetNodeTokenGrantRecordsReq) (*model.GetNodeTokenGrantRecordsRes, error)
}

// nodePurchaseService 节点购买服务实现
type nodePurchaseService struct {
	balanceRepo      repo.IBalanceRepository
	nodePurchaseRepo repo.INodePurchaseRepository
	tokenGrantRepo   repo.INodeTokenGrantRepository
	userRepo         repository.IUserRepository
	nodePriceRepo    *repository.NodePriceRepository
	assetRecordDao   rewardDao.IAssetRecordDao
}

// NewNodePurchaseService 创建节点购买服务
func NewNodePurchaseService() NodePurchaseService {
	return &nodePurchaseService{
		balanceRepo:      repo.NewBalanceRepository(),
		nodePurchaseRepo: repo.NewNodePurchaseRepository(),
		tokenGrantRepo:   repo.NewNodeTokenGrantRepository(),
		userRepo:         repository.NewUserRepository(),
		nodePriceRepo:    repository.NewNodePriceRepository(),
		assetRecordDao:   rewardDao.NewAssetRecordDao(),
	}
}

// GetBalance 获取用户余额
func (s *nodePurchaseService) GetBalance(ctx context.Context, userID int64, symbol string) (*model.GetBalanceRes, error) {
	if symbol == "" {
		symbol = "USDT"
	}

	balance, err := s.balanceRepo.GetOrCreate(ctx, userID, symbol)
	if err != nil {
		return nil, gerror.Wrap(err, "获取余额失败")
	}

	return &model.GetBalanceRes{
		Available: balance.AvailableAmount.String(),
		Frozen:    balance.FrozenAmount.String(),
		Symbol:    symbol,
	}, nil
}

// BuyNode 购买节点
func (s *nodePurchaseService) BuyNode(ctx context.Context, req *model.BuyNodeReq) (*model.BuyNodeRes, error) {
	locale := consts.LocaleFromCtx(ctx)
	if req != nil {
		rawLocale := strings.TrimSpace(req.Locale)
		if rawLocale != "" {
			if len(rawLocale) > 20 {
				return nil, gerror.New("locale must not exceed 20 characters")
			}
			locale = rawLocale
		}
	}
	locale = consts.NormalizeLanguage(locale)
	if locale == "" {
		locale = consts.LanguageDefault
	}

	requestID := strings.TrimSpace(req.RequestID)
	if len(requestID) > 64 {
		return nil, gerror.New("request_id must not exceed 64 characters")
	}

	g.Log().Infof(ctx, "[CoboBuyNode] 开始购买: user_id=%d, node_type=%d, request_id=%s", req.UserID, req.NodeType, requestID)

	// 获取分布式锁，防止并发购买导致余额超扣
	lock, lockErr := s.acquireUserBuyNodeLock(ctx, req.UserID)
	if lockErr != nil {
		return nil, gerror.Wrap(lockErr, "获取购买锁失败")
	}
	if lock == nil {
		return nil, gerror.New("购买请求过于频繁，请稍后重试")
	}
	defer s.releaseUserBuyNodeLock(ctx, lock)

	// 1. 获取节点类型价格配置（优先使用数据库配置）
	priceConfig, err := s.nodePriceRepo.GetByNodeType(ctx, req.NodeType)
	if err != nil {
		return nil, gerror.Wrap(err, "获取节点价格配置失败")
	}

	typeConfig := model.GetNodeTypeConfig(req.NodeType)
	if typeConfig == nil {
		return nil, gerror.New("不支持的节点类型")
	}

	amount := decimal.NewFromFloat(typeConfig.Amount)
	if priceConfig != nil {
		amount = priceConfig.Price
	}
	powerValue := amount.Mul(decimal.NewFromFloat(typeConfig.PowerMultiplier))

	// 2. 检查余额
	balance, err := s.balanceRepo.GetOrCreate(ctx, req.UserID, "USDT")
	if err != nil {
		return nil, gerror.Wrap(err, "获取余额失败")
	}

	if !balance.HasEnoughBalance(amount) {
		return nil, gerror.New("余额不足")
	}

	if requestID != "" {
		existing, err := s.nodePurchaseRepo.GetByUserAndRequestID(ctx, req.UserID, requestID)
		if err != nil {
			return nil, gerror.Wrap(err, "查询幂等记录失败")
		}
		if existing != nil {
			g.Log().Infof(ctx, "[CoboBuyNode] 命中幂等请求: user_id=%d, request_id=%s, package_no=%s", req.UserID, requestID, existing.PackageNo)
			return s.buildBuyNodeRes(ctx, existing), nil
		}
	}

	// 3. 获取用户信息（用于直推奖励）
	user, err := s.userRepo.GetUserById(ctx, req.UserID)
	if err != nil {
		return nil, gerror.Wrap(err, "获取用户信息失败")
	}
	if user == nil {
		return nil, gerror.New("用户不存在")
	}

	// 4. 开启事务执行购买
	var purchase *cobo.NodePurchaseEntity
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		g.Log().Infof(ctx, "[CoboBuyNode] 事务开始: user_id=%d, node_type=%d, request_id=%s", req.UserID, req.NodeType, requestID)

		// 4.1 创建购买记录
		now := time.Now()
		purchase = &cobo.NodePurchaseEntity{
			UserID:      req.UserID,
			PackageNo:   fmt.Sprintf("PKG%s", utils.GenerateSnowflakeId()),
			RequestID:   requestID,
			NodeType:    req.NodeType,
			Amount:      amount,
			PowerValue:  powerValue,
			StakeAmount: amount,
			Status:      cobo.NodeStatusRunning,
			StartTime:   now,
			Locale:      locale,
		}

		if err := s.nodePurchaseRepo.Create(ctx, tx, purchase); err != nil {
			if requestID != "" && strings.Contains(strings.ToLower(err.Error()), "request_id") {
				g.Log().Infof(ctx, "[CoboBuyNode] 并发幂等冲突，回滚并读取已有记录: user_id=%d, request_id=%s", req.UserID, requestID)
				return errIdempotentRequestConflict
			}
			return gerror.Wrap(err, "创建购买记录失败")
		}

		// 4.2 扣除余额
		balanceBefore := decimal.Zero
		balanceRow, _ := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, "USDT")
		if balanceRow != nil {
			balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
		}
		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, req.UserID, "USDT", amount.Neg(), decimal.Zero); err != nil {
			return gerror.Wrap(err, "扣除余额失败")
		}
		if err := writeBalanceChangeLogTx(ctx, tx, req.UserID, "USDT", consts.ChangeTypeNodePurchase, amount.Neg(), balanceBefore, balanceBefore.Sub(amount), purchase.PackageNo, purchase.ID, "node purchase deduct", consts.OperatorTypeSystem); err != nil {
			return gerror.Wrap(err, "写入节点购买扣款审计日志失败")
		}

		// 4.3 创建YYAI发放记录并入账（按实时价格换算）
		if err := s.createPendingYYGrantTx(ctx, tx, user.WalletAddress, purchase); err != nil {
			g.Log().Warningf(ctx, "[Cobo] 创建YYAI发放记录失败: %v", err)
		}

		// 4.4 创建Triple(三倍券)待发放记录（按节点档位赠送不同比例）
		if err := s.createPendingTripleGrantTx(ctx, tx, user.WalletAddress, purchase); err != nil {
			g.Log().Warningf(ctx, "[Cobo] 创建Triple发放记录失败: %v", err)
		}

		err := s.increaseNodeSoldShares(ctx, tx, req.NodeType)
		if err != nil {
			return gerror.Wrap(err, "更新节点份额失败")
		}

		g.Log().Infof(ctx, "[CoboBuyNode] 事务完成: user_id=%d, node_type=%d, package_no=%s, request_id=%s", req.UserID, req.NodeType, purchase.PackageNo, requestID)

		return nil
	})

	if err != nil {
		if errors.Is(err, errIdempotentRequestConflict) {
			existing, getErr := s.nodePurchaseRepo.GetByUserAndRequestID(ctx, req.UserID, requestID)
			if getErr != nil {
				return nil, gerror.Wrap(getErr, "读取幂等记录失败")
			}
			if existing == nil {
				return nil, gerror.New("幂等请求记录不存在")
			}
			g.Log().Infof(ctx, "[CoboBuyNode] 幂等返回已有记录: user_id=%d, request_id=%s, package_no=%s", req.UserID, requestID, existing.PackageNo)
			return s.buildBuyNodeRes(ctx, existing), nil
		}

		g.Log().Errorf(ctx, "[CoboBuyNode] 购买失败: user_id=%d, node_type=%d, request_id=%s, err=%v", req.UserID, req.NodeType, requestID, err)
		return nil, err
	}
	if purchase == nil {
		return nil, gerror.New("购买结果异常")
	}

	// 事务提交后发放推荐奖励（在事务外执行，确保额度计算能看到最新的购买记录）
	if err := s.grantReferralRewardsAfterTx(ctx, req.UserID, user.ParentWalletAddress, amount, purchase); err != nil {
		g.Log().Warningf(ctx, "[Cobo] 事务外发放推荐奖励失败: %v", err)
		// 不影响购买流程，记录日志即可
	}

	res := s.buildBuyNodeRes(ctx, purchase)
	g.Log().Infof(ctx, "[CoboBuyNode] 购买成功: user_id=%d, node_type=%d, package_no=%s, request_id=%s, balance_after=%s", req.UserID, req.NodeType, purchase.PackageNo, requestID, res.BalanceAfter)

	return res, nil
}

// createPendingYYGrantTx 创建YYAI发放记录并实时入账（根据实时价格计算YYAI数量）
func (s *nodePurchaseService) createPendingYYGrantTx(ctx context.Context, tx gdb.TX, wallet string, purchase *cobo.NodePurchaseEntity) error {
	if purchase == nil || purchase.ID == 0 {
		return nil
	}

	// 100U 体验节点不发放YYAI
	if purchase.NodeType == 6 {
		return nil
	}

	// 获取最新YYAI价格
	stockPriceDao := dao.NewStockPriceDao()
	latestPrice, err := stockPriceDao.GetLatestBySymbol(ctx, "YYAI")
	if err != nil {
		g.Log().Warningf(ctx, "[Cobo] 获取YYAI最新价格失败: %v", err)
		// 价格获取失败时，仍创建记录但数量为0，后续可人工处理
		latestPrice = nil
	}

	var yyaiPrice decimal.Decimal
	var yyaiAmount decimal.Decimal
	usdtAmount := purchase.Amount

	if latestPrice != nil && latestPrice.Price.GreaterThan(decimal.Zero) {
		yyaiPrice = latestPrice.Price
		// 计算YYAI数量: USDT金额 / YYAI价格(USD)
		yyaiAmount = usdtAmount.Div(yyaiPrice)
		g.Log().Infof(ctx, "[Cobo] 计算YYAI发放数量: usdt=%s, yyai_price=%s, yyai_amount=%s",
			usdtAmount.String(), yyaiPrice.String(), yyaiAmount.String())
	} else {
		g.Log().Warningf(ctx, "[Cobo] YYAI价格无效，无法计算发放数量: purchase_id=%d", purchase.ID)
		yyaiPrice = decimal.Zero
		yyaiAmount = decimal.Zero
	}

	grant := &cobo.NodeTokenGrantEntity{
		PurchaseID:  purchase.ID,
		UserID:      purchase.UserID,
		Wallet:      wallet,
		TokenType:   cobo.TokenTypeYYAI,
		TokenSymbol: "YYAI",
		TokenChain:  "BSC",
		TokenAddr:   "",
		GrantAmount: yyaiAmount.String(), // 实际YYAI数量
		Status:      cobo.NodeTokenGrantStatusSent,
		// YYAI价格相关字段
		YyaiPrice:  yyaiPrice,
		UsdtAmount: usdtAmount,
		YyaiAmount: yyaiAmount,
	}

	if err := s.tokenGrantRepo.CreateTx(ctx, tx, grant); err != nil {
		return err
	}

	if !yyaiAmount.GreaterThan(decimal.Zero) {
		return nil
	}

	yyaiBefore := decimal.Zero
	yyaiBalance, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, purchase.UserID, "YYAI")
	if err != nil {
		return gerror.Wrap(err, "查询YYAI余额失败")
	}
	if yyaiBalance != nil {
		yyaiBefore = yyaiBalance.AvailableAmount.Add(yyaiBalance.FrozenAmount)
	}

	if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, purchase.UserID, "YYAI", yyaiAmount, decimal.Zero); err != nil {
		return gerror.Wrap(err, "更新YYAI余额失败")
	}

	yyaiAfter := yyaiBefore.Add(yyaiAmount)
	if err := writeBalanceChangeLogTx(ctx, tx, purchase.UserID, "YYAI", consts.ChangeTypeYYAIToBalance, yyaiAmount, yyaiBefore, yyaiAfter, purchase.PackageNo, purchase.ID, "node purchase yyai grant to balance", consts.OperatorTypeSystem); err != nil {
		return gerror.Wrap(err, "写入YYAI发放审计日志失败")
	}

	return nil
}

// createPendingTripleGrantTx 创建Triple(三倍券)待发放记录
// 三倍券按节点档位赠送不同比例，固定价格 1 USDT
// 普通购买和赠送节点都发放 Triple（仅赠送节点不产生佣金）
func (s *nodePurchaseService) createPendingTripleGrantTx(ctx context.Context, tx gdb.TX, wallet string, purchase *cobo.NodePurchaseEntity) error {
	if purchase == nil || purchase.ID == 0 {
		return nil
	}

	// 100U 体验节点不发放三倍券
	if purchase.NodeType == 6 {
		return nil
	}

	// 获取该节点档位的赠送比例（按购买时间计算）
	rate := cobo.GetTripleCouponRate(purchase.NodeType, time.Now())

	usdtAmount := purchase.Amount
	// 计算Triple数量: USDT金额 * 赠送比例 / Triple价格(1 USDT)
	// 即: Triple数量 = USDT金额 * 赠送比例
	tripleAmount := usdtAmount.Mul(decimal.NewFromFloat(rate))

	g.Log().Infof(ctx, "[Cobo] 计算Triple发放数量: usdt=%s, rate=%.0f%%, triple_amount=%s",
		usdtAmount.String(), rate*100, tripleAmount.String())

	if tripleAmount.IsZero() {
		return nil
	}

	grant := &cobo.NodeTokenGrantEntity{
		PurchaseID:  purchase.ID,
		UserID:      purchase.UserID,
		Wallet:      wallet,
		TokenType:   cobo.TokenTypeTriple, // 标记为三倍券
		TokenSymbol: "Triple",
		TokenChain:  "BSC",
		TokenAddr:   "",
		GrantAmount: tripleAmount.String(),
		Status:      cobo.NodeTokenGrantStatusPending,
		// 复用YYAI价格相关字段记录原始信息
		UsdtAmount: usdtAmount,
		YyaiAmount: tripleAmount,
	}

	return s.tokenGrantRepo.CreateTx(ctx, tx, grant)
}

// GetNodeTokenGrantRecords 获取YYAI发放记录
func (s *nodePurchaseService) GetNodeTokenGrantRecords(ctx context.Context, req *model.GetNodeTokenGrantRecordsReq) (*model.GetNodeTokenGrantRecordsRes, error) {
	if req == nil {
		return &model.GetNodeTokenGrantRecordsRes{List: []*model.NodeTokenGrantRecordItem{}}, nil
	}

	rows, total, err := s.tokenGrantRepo.GetByUserID(ctx, req.UserID, req.TokenType, req.Page, req.PageSize)
	if err != nil {
		return nil, gerror.Wrap(err, "查询YYAI发放记录失败")
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	pages := 0
	if total > 0 {
		pages = int((total + int64(pageSize) - 1) / int64(pageSize))
	}

	list := make([]*model.NodeTokenGrantRecordItem, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		list = append(list, &model.NodeTokenGrantRecordItem{
			ID:          row.ID,
			PurchaseID:  row.PurchaseID,
			TokenType:   row.TokenType,
			TokenSymbol: row.TokenSymbol,
			TokenChain:  row.TokenChain,
			GrantAmount: row.GrantAmount,
			Status:      row.Status,
			CreatedAt:   row.CreatedAt.Unix(),
			// YYAI价格相关字段
			YyaiPrice:  row.YyaiPrice.String(),
			UsdtAmount: row.UsdtAmount.String(),
			YyaiAmount: row.YyaiAmount.String(),
		})
	}

	return &model.GetNodeTokenGrantRecordsRes{
		Page:     page,
		PageSize: pageSize,
		Total:    int(total),
		Pages:    pages,
		List:     list,
	}, nil
}

// increaseNodeSoldShares 更新 node_info 的 sold_shares，售罄时更新状态
func (s *nodePurchaseService) increaseNodeSoldShares(ctx context.Context, tx gdb.TX, nodeType int) error {
	level := fmt.Sprintf("NODE%d", nodeType)

	list, err := tx.Model("node_info").
		Ctx(ctx).
		Where("node_level = ?", level).
		Where("status = ?", 2).
		Order("id ASC").
		Limit(1).
		All()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return gerror.Newf("节点类型%d当前未在售", nodeType)
	}

	record := list[0]

	id := record["id"].Int64()
	result, err := tx.Model("node_info").
		Ctx(ctx).
		Where("id = ?", id).
		Where("total_shares <= 0 OR sold_shares < total_shares").
		Data(g.Map{
			"sold_shares": gdb.Raw("sold_shares + 1"),
			"status":      gdb.Raw("CASE WHEN total_shares > 0 AND sold_shares + 1 >= total_shares THEN 3 ELSE status END"),
			"updated_at":  time.Now(),
		}).
		Update()
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		g.Log().Warningf(ctx, "[CoboBuyNode] 更新节点份额失败(可能售罄): node_id=%d", id)
		return gerror.New("节点已售罄")
	}

	return nil
}

func (s *nodePurchaseService) buildBuyNodeRes(ctx context.Context, purchase *cobo.NodePurchaseEntity) *model.BuyNodeRes {
	balanceAfter, _ := s.balanceRepo.GetByUserID(ctx, purchase.UserID, "USDT")
	balanceAfterStr := "0"
	if balanceAfter != nil {
		balanceAfterStr = balanceAfter.AvailableAmount.String()
	}

	return &model.BuyNodeRes{
		PackageNo:    purchase.PackageNo,
		NodeType:     purchase.NodeType,
		Amount:       purchase.Amount.String(),
		PowerValue:   purchase.PowerValue.String(),
		BalanceAfter: balanceAfterStr,
		CreatedAt:    utils.DBTimestampToUnix(purchase.CreatedAt),
	}
}

// grantReferralRewards 发放推荐奖励（仅直推10%）
func (s *nodePurchaseService) grantReferralRewards(ctx context.Context, tx gdb.TX, buyerID int64, parentWallet string, buyAmount decimal.Decimal, purchase *cobo.NodePurchaseEntity) error {
	// 赠送节点不产生佣金
	if purchase != nil && purchase.IsGift == 1 {
		g.Log().Infof(ctx, "[Cobo] 赠送节点跳过推荐奖励: buyer=%d, package_no=%s", buyerID, purchase.PackageNo)
		return nil
	}

	if parentWallet == "" {
		return nil
	}

	// 查找上级用户
	parentUser, err := s.userRepo.GetUserByWalletAddress(ctx, parentWallet)
	if err != nil {
		return err
	}
	if parentUser == nil {
		return nil
	}

	// 计算奖励（10%的购买金额）
	rewardRate := decimal.NewFromFloat(0.10)
	rewardAmount := buyAmount.Mul(rewardRate)

	if rewardAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}

	// 发放全额奖励到上级余额
	balanceBefore := decimal.Zero
	balanceRow, _ := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, parentUser.Id, "USDT")
	if balanceRow != nil {
		balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
	}
	if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, parentUser.Id, "USDT", rewardAmount, decimal.Zero); err != nil {
		return gerror.Wrap(err, "发放直推奖励失败")
	}
	if err := writeBalanceChangeLogTx(ctx, tx, parentUser.Id, "USDT", consts.ChangeTypeNodePurchaseDirectReward, rewardAmount, balanceBefore, balanceBefore.Add(rewardAmount), purchase.PackageNo, purchase.ID, "node purchase direct reward", consts.OperatorTypeSystem); err != nil {
		return gerror.Wrap(err, "写入直推奖励审计日志失败")
	}

	// 更新购买记录的直推奖励信息
	purchase.DirectRewardAmount = rewardAmount
	purchase.DirectRewardUserID = parentUser.Id

	g.Log().Infof(ctx, "[Cobo] 发放直推奖励: buyer=%d, parent=%d, amount=%s",
		buyerID, parentUser.Id, rewardAmount.String())

	return nil
}

// grantReferralRewardsAfterTx 事务提交后发放推荐奖励（非事务版本）
func (s *nodePurchaseService) grantReferralRewardsAfterTx(ctx context.Context, buyerID int64, parentWallet string, buyAmount decimal.Decimal, purchase *cobo.NodePurchaseEntity) error {
	// 赠送节点不产生佣金
	if purchase != nil && purchase.IsGift == 1 {
		g.Log().Infof(ctx, "[Cobo] 赠送节点跳过推荐奖励: buyer=%d, package_no=%s", buyerID, purchase.PackageNo)
		return nil
	}

	if parentWallet == "" {
		return nil
	}

	// 查找上级用户
	parentUser, err := s.userRepo.GetUserByWalletAddress(ctx, parentWallet)
	if err != nil {
		return err
	}
	if parentUser == nil {
		return nil
	}

	// 计算奖励（10%的购买金额）
	rewardRate := decimal.NewFromFloat(0.10)
	rewardAmount := buyAmount.Mul(rewardRate)

	if rewardAmount.LessThanOrEqual(decimal.Zero) {
		return nil
	}

	// 开启独立事务发放全额奖励
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		balanceBefore := decimal.Zero
		balanceRow, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, parentUser.Id, "USDT")
		if err != nil {
			return gerror.Wrap(err, "查询上级余额失败")
		}
		if balanceRow != nil {
			balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
		}

		// 发放奖励到上级余额
		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, parentUser.Id, "USDT", rewardAmount, decimal.Zero); err != nil {
			return gerror.Wrap(err, "发放直推奖励失败")
		}
		if err := writeBalanceChangeLogTx(ctx, tx, parentUser.Id, "USDT", consts.ChangeTypeNodePurchaseDirectReward, rewardAmount, balanceBefore, balanceBefore.Add(rewardAmount), purchase.PackageNo, purchase.ID, "node purchase direct reward", consts.OperatorTypeSystem); err != nil {
			return gerror.Wrap(err, "写入直推奖励审计日志失败")
		}

		// 更新购买记录的直推奖励信息
		_, err = tx.Model("cobo_node_purchase").Ctx(ctx).
			Data(g.Map{
				"direct_reward_amount":  rewardAmount,
				"direct_reward_user_id": parentUser.Id,
				"updated_at":            time.Now(),
			}).
			Where("id = ?", purchase.ID).
			Update()
		if err != nil {
			return gerror.Wrap(err, "更新购买记录奖励信息失败")
		}

		// 创建资产记录
		record := &rewardEntity.AssetRecordEntity{
			UserID:       parentUser.Id,
			AssetType:    "USDT",
			RecordTime:   time.Now(),
			Amount:       rewardAmount,
			FlowType:     consts.AssetFlowTypeIncome,
			Status:       consts.AssetRecordStatusSettled,
			BusinessType: consts.AssetBusinessTypeRewardReferral,
			BusinessID:   purchase.ID,
			Remark:       "节点购买直推奖励",
			Metadata:     "{}",
		}
		if err := s.assetRecordDao.Create(ctx, tx, record); err != nil {
			return gerror.Wrap(err, "创建直推奖励资产记录失败")
		}

		return nil
	})

	if err != nil {
		return err
	}

	g.Log().Infof(ctx, "[Cobo] 事务外发放直推奖励成功: buyer=%d, parent=%d, amount=%s",
		buyerID, parentUser.Id, rewardAmount.String())

	return nil
}

const buyNodeUserLockTTLSeconds = 30

const buyNodeUserLockRenewIntervalSeconds = 10

const releaseUserBuyNodeLockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0
`

const renewUserBuyNodeLockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("EXPIRE", KEYS[1], ARGV[2])
end
return 0
`

type userBuyNodeLock struct {
	key   string
	token string
	stop  chan struct{}
	done  chan struct{}
	once  sync.Once
}

func (s *nodePurchaseService) acquireUserBuyNodeLock(ctx context.Context, userID int64) (*userBuyNodeLock, error) {
	redisClient := g.Redis()
	if redisClient == nil {
		return nil, gerror.New("redis not initialized")
	}

	key := fmt.Sprintf("cobo:node:buy:lock:%d", userID)
	token := utils.GenerateSnowflakeId()
	result, err := redisClient.Do(ctx, "SET", key, token, "NX", "EX", buyNodeUserLockTTLSeconds)
	if err != nil {
		return nil, err
	}
	if result.IsNil() {
		return nil, nil
	}
	if result.String() != "OK" {
		return nil, gerror.Newf("unexpected lock response: %s", result.String())
	}

	lock := &userBuyNodeLock{
		key:   key,
		token: token,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}

	go s.renewUserBuyNodeLock(lock)

	return lock, nil
}

func (s *nodePurchaseService) renewUserBuyNodeLock(lock *userBuyNodeLock) {
	defer close(lock.done)

	ticker := time.NewTicker(time.Duration(buyNodeUserLockRenewIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-lock.stop:
			return
		case <-ticker.C:
			ok, err := s.tryRenewUserBuyNodeLock(lock)
			if err != nil {
				g.Log().Warningf(context.Background(), "[Cobo] 续期购买锁失败: key=%s, err=%v", lock.key, err)
				continue
			}
			if !ok {
				g.Log().Warningf(context.Background(), "[Cobo] 购买锁续期终止: key=%s", lock.key)
				return
			}
		}
	}
}

func (s *nodePurchaseService) tryRenewUserBuyNodeLock(lock *userBuyNodeLock) (bool, error) {
	redisClient := g.Redis()
	if redisClient == nil {
		return false, gerror.New("redis not initialized")
	}

	result, err := redisClient.Do(
		context.Background(),
		"EVAL",
		renewUserBuyNodeLockScript,
		1,
		lock.key,
		lock.token,
		buyNodeUserLockTTLSeconds,
	)
	if err != nil {
		return false, err
	}
	if result.IsNil() {
		return false, nil
	}

	n := result.Int()

	return n == 1, nil
}

func (s *nodePurchaseService) releaseUserBuyNodeLock(ctx context.Context, lock *userBuyNodeLock) {
	if lock == nil {
		return
	}

	lock.once.Do(func() {
		close(lock.stop)
		<-lock.done

		redisClient := g.Redis()
		if redisClient == nil {
			return
		}

		result, err := redisClient.Do(
			ctx,
			"EVAL",
			releaseUserBuyNodeLockScript,
			1,
			lock.key,
			lock.token,
		)
		if err != nil {
			g.Log().Warningf(ctx, "[Cobo] 释放购买锁失败: key=%s, err=%v", lock.key, err)
			return
		}
		if result.IsNil() {
			return
		}

		n := result.Int()
		if n != 1 {
			g.Log().Warningf(ctx, "[Cobo] 购买锁已过期或不归属当前请求: key=%s", lock.key)
		}
	})
}
