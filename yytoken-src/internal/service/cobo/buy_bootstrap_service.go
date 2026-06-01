package cobo

import (
	"context"
	"fmt"
	"time"

	"XWFrame/internal/dao"
	rewardDao "XWFrame/internal/dao/reward"
	"XWFrame/internal/entity"
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

type BuyBootstrapReq struct {
	WalletAddress string
	NodeType      int
	AdminID       int64
	Remark        string
	TeamName      string
	EnableExempt  bool
	DeductUSDT    bool
}

type BuyBootstrapRes struct {
	UserID             int64
	WalletAddress      string
	PackageNo          string
	NodeType           int
	Amount             string
	PowerValue         string
	UserCreated        bool
	TeamCreated        bool
	TeamSyncedCount    int
	ExemptUpdated      bool
	DirectRewardUserID int64
	DirectRewardAmount string
	YYAICreated        bool
	TripleCreated      bool
}

type BuyBootstrapService interface {
	EnsureAndBuy(ctx context.Context, req *BuyBootstrapReq) (*BuyBootstrapRes, error)
}

type buyBootstrapService struct{}

func NewBuyBootstrapService() BuyBootstrapService {
	return &buyBootstrapService{}
}

func (s *buyBootstrapService) EnsureAndBuy(ctx context.Context, req *BuyBootstrapReq) (*BuyBootstrapRes, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	walletAddress := normalizeGiftWallet(req.WalletAddress)
	if walletAddress == "" {
		return nil, fmt.Errorf("wallet_address is empty")
	}

	// 复用 giftBootstrapService 的用户/团队创建逻辑
	giftSvc := &giftBootstrapService{}
	user, userCreated, err := giftSvc.ensureUserExists(ctx, walletAddress)
	if err != nil {
		return nil, fmt.Errorf("确保用户存在失败: %w", err)
	}

	exemptUpdated := false
	if req.EnableExempt && !user.NodeExempt {
		userRepo := repository.NewUserRepository()
		if err := userRepo.UpdateUser(ctx, user.Id, map[string]interface{}{
			"node_exempt": true,
			"updated_at":  time.Now(),
		}); err != nil {
			return nil, fmt.Errorf("更新用户业绩豁免状态失败: %w", err)
		}
		user.NodeExempt = true
		exemptUpdated = true
	}

	teamCreated, err := giftSvc.ensureTeamExists(ctx, walletAddress, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("确保团队存在失败: %w", err)
	}

	fixedCount, err := giftSvc.syncTeamIDByNearestRuleForSubtree(ctx, walletAddress)
	if err != nil {
		return nil, fmt.Errorf("同步用户及下级 team_id 失败: %w", err)
	}

	// 获取节点配置
	typeConfig := model.GetNodeTypeConfig(req.NodeType)
	if typeConfig == nil {
		return nil, fmt.Errorf("不支持的节点类型: %d", req.NodeType)
	}

	nodePriceRepo := repository.NewNodePriceRepository()
	priceConfig, err := nodePriceRepo.GetByNodeType(ctx, req.NodeType)
	if err != nil {
		return nil, fmt.Errorf("获取节点价格配置失败: %w", err)
	}

	amount := decimal.NewFromFloat(typeConfig.Amount)
	if priceConfig != nil {
		amount = priceConfig.Price
	}
	powerValue := amount.Mul(decimal.NewFromFloat(typeConfig.PowerMultiplier))

	// 执行购买（不扣款，发放直推奖励）
	buyRes, err := s.buyNode(ctx, user, req.NodeType, amount, powerValue, req.AdminID, req.Remark, req.DeductUSDT)
	if err != nil {
		return nil, fmt.Errorf("执行购买失败: %w", err)
	}

	return &BuyBootstrapRes{
		UserID:             user.Id,
		WalletAddress:      user.WalletAddress,
		PackageNo:          buyRes.PackageNo,
		NodeType:           req.NodeType,
		Amount:             amount.String(),
		PowerValue:         powerValue.String(),
		UserCreated:        userCreated,
		TeamCreated:        teamCreated,
		TeamSyncedCount:    fixedCount,
		ExemptUpdated:      exemptUpdated,
		DirectRewardUserID: buyRes.DirectRewardUserID,
		DirectRewardAmount: buyRes.DirectRewardAmount,
		YYAICreated:        buyRes.YYAICreated,
		TripleCreated:      buyRes.TripleCreated,
	}, nil
}

type buyNodeResult struct {
	PackageNo          string
	DirectRewardUserID int64
	DirectRewardAmount string
	YYAICreated        bool
	TripleCreated      bool
}

func (s *buyBootstrapService) buyNode(ctx context.Context, user *entity.UserEntity, nodeType int, amount, powerValue decimal.Decimal, adminID int64, remark string, deductUSDT bool) (*buyNodeResult, error) {
	now := time.Now()
	purchase := &cobo.NodePurchaseEntity{
		UserID:      user.Id,
		PackageNo:   fmt.Sprintf("BUYADMIN%s", utils.GenerateSnowflakeId()),
		RequestID:   "",
		NodeType:    nodeType,
		Amount:      amount,
		PowerValue:  powerValue,
		StakeAmount: amount,
		Status:      cobo.NodeStatusRunning,
		StartTime:   now,
		IsGift:      0,
		GiftBy:      0,
		GiftRemark:  remark,
	}

	nodePurchaseRepo := repo.NewNodePurchaseRepository()
	balanceRepo := repo.NewBalanceRepository()
	userRepo := repository.NewUserRepository()
	assetRecordDao := rewardDao.NewAssetRecordDao()

	var directRewardUserID int64
	var directRewardAmount decimal.Decimal
	var yyaiCreated bool
	var tripleCreated bool

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 1. 创建购买记录
		if err := nodePurchaseRepo.Create(ctx, tx, purchase); err != nil {
			return gerror.Wrap(err, "创建购买记录失败")
		}

		// 2. 扣除用户 USDT 余额
		if deductUSDT {
			balanceRow, err := balanceRepo.GetByUserIDForUpdate(ctx, tx, user.Id, "USDT")
			if err != nil {
				return gerror.Wrap(err, "查询用户 USDT 余额失败")
			}
			balanceBefore := decimal.Zero
			if balanceRow != nil {
				balanceBefore = balanceRow.AvailableAmount
			}
			if balanceBefore.LessThan(amount) {
				return gerror.Newf("用户 USDT 余额不足: available=%s, required=%s", balanceBefore.String(), amount.String())
			}
			if err := balanceRepo.UpdateBalanceTx(ctx, tx, user.Id, "USDT", amount.Neg(), decimal.Zero); err != nil {
				return gerror.Wrap(err, "扣除用户 USDT 余额失败")
			}
			balanceAfter := balanceBefore.Sub(amount)
			if err := writeBalanceChangeLogTx(ctx, tx, user.Id, "USDT", consts.ChangeTypeNodePurchase, amount.Neg(), balanceBefore, balanceAfter, purchase.PackageNo, purchase.ID, "admin buy node deduct usdt", consts.OperatorTypeSystem); err != nil {
				return gerror.Wrap(err, "写入 USDT 扣除审计日志失败")
			}
			g.Log().Infof(ctx, "[AdminBuyNode] 扣除用户 USDT 余额: user=%d, amount=%s", user.Id, amount.String())
		}

		// 3. 发放直推奖励（不检查 is_gift）
		if user.ParentWalletAddress != "" && user.ParentWalletAddress != "0x00" {
			parentUser, err := userRepo.GetUserByWalletAddress(ctx, user.ParentWalletAddress)
			if err != nil {
				return gerror.Wrap(err, "查询上级用户失败")
			}
			if parentUser != nil {
				rewardRate := decimal.NewFromFloat(0.10)
				rewardAmount := amount.Mul(rewardRate)
				if rewardAmount.GreaterThan(decimal.Zero) {
					// 增加上级余额
					balanceBefore := decimal.Zero
					balanceRow, _ := balanceRepo.GetByUserIDForUpdate(ctx, tx, parentUser.Id, "USDT")
					if balanceRow != nil {
						balanceBefore = balanceRow.AvailableAmount.Add(balanceRow.FrozenAmount)
					}
					if err := balanceRepo.UpdateBalanceTx(ctx, tx, parentUser.Id, "USDT", rewardAmount, decimal.Zero); err != nil {
						return gerror.Wrap(err, "发放直推奖励失败")
					}
					if err := writeBalanceChangeLogTx(ctx, tx, parentUser.Id, "USDT", consts.ChangeTypeNodePurchaseDirectReward, rewardAmount, balanceBefore, balanceBefore.Add(rewardAmount), purchase.PackageNo, purchase.ID, "admin buy node direct reward", consts.OperatorTypeSystem); err != nil {
						return gerror.Wrap(err, "写入直推奖励审计日志失败")
					}

					// 更新购买记录直推奖励信息
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
						Remark:       "后台模拟购买直推奖励",
						Metadata:     "{}",
					}
					if err := assetRecordDao.Create(ctx, tx, record); err != nil {
						return gerror.Wrap(err, "创建直推奖励资产记录失败")
					}

					directRewardUserID = parentUser.Id
					directRewardAmount = rewardAmount

					g.Log().Infof(ctx, "[AdminBuyNode] 发放直推奖励: buyer=%d, parent=%d, amount=%s",
						user.Id, parentUser.Id, rewardAmount.String())
				}
			}
		}

		// 4. 创建YYAI发放记录
		if created, err := s.createPendingYYGrantTx(ctx, tx, user.WalletAddress, purchase); err != nil {
			g.Log().Warningf(ctx, "[AdminBuyNode] 创建YYAI发放记录失败: %v", err)
		} else {
			yyaiCreated = created
		}

		// 5. 创建Triple待发放记录
		if created, err := s.createPendingTripleGrantTx(ctx, tx, user.WalletAddress, purchase); err != nil {
			g.Log().Warningf(ctx, "[AdminBuyNode] 创建Triple发放记录失败: %v", err)
		} else {
			tripleCreated = created
		}

		// 6. 更新节点份额
		if err := s.increaseNodeSoldShares(ctx, tx, nodeType); err != nil {
			return gerror.Wrap(err, "更新节点份额失败")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &buyNodeResult{
		PackageNo:          purchase.PackageNo,
		DirectRewardUserID: directRewardUserID,
		DirectRewardAmount: directRewardAmount.String(),
		YYAICreated:        yyaiCreated,
		TripleCreated:      tripleCreated,
	}, nil
}

// createPendingYYGrantTx 创建YYAI发放记录并实时入账，返回是否创建了记录
func (s *buyBootstrapService) createPendingYYGrantTx(ctx context.Context, tx gdb.TX, wallet string, purchase *cobo.NodePurchaseEntity) (bool, error) {
	if purchase == nil || purchase.ID == 0 {
		return false, nil
	}
	if purchase.NodeType == 6 {
		return false, nil
	}

	stockPriceDao := dao.NewStockPriceDao()
	latestPrice, err := stockPriceDao.GetLatestBySymbol(ctx, "YYAI")
	if err != nil {
		g.Log().Warningf(ctx, "[AdminBuyNode] 获取YYAI最新价格失败: %v", err)
		latestPrice = nil
	}

	var yyaiPrice decimal.Decimal
	var yyaiAmount decimal.Decimal
	usdtAmount := purchase.Amount

	if latestPrice != nil && latestPrice.Price.GreaterThan(decimal.Zero) {
		yyaiPrice = latestPrice.Price
		yyaiAmount = usdtAmount.Div(yyaiPrice)
	} else {
		g.Log().Warningf(ctx, "[AdminBuyNode] YYAI价格无效: purchase_id=%d", purchase.ID)
		yyaiPrice = decimal.Zero
		yyaiAmount = decimal.Zero
	}

	tokenGrantRepo := repo.NewNodeTokenGrantRepository()
	balanceRepo := repo.NewBalanceRepository()

	grant := &cobo.NodeTokenGrantEntity{
		PurchaseID:  purchase.ID,
		UserID:      purchase.UserID,
		Wallet:      wallet,
		TokenType:   cobo.TokenTypeYYAI,
		TokenSymbol: "YYAI",
		TokenChain:  "BSC",
		TokenAddr:   "",
		GrantAmount: yyaiAmount.String(),
		Status:      cobo.NodeTokenGrantStatusSent,
		YyaiPrice:   yyaiPrice,
		UsdtAmount:  usdtAmount,
		YyaiAmount:  yyaiAmount,
	}

	if err := tokenGrantRepo.CreateTx(ctx, tx, grant); err != nil {
		return false, err
	}

	if !yyaiAmount.GreaterThan(decimal.Zero) {
		return true, nil
	}

	yyaiBefore := decimal.Zero
	yyaiBalance, err := balanceRepo.GetByUserIDForUpdate(ctx, tx, purchase.UserID, "YYAI")
	if err != nil {
		return true, gerror.Wrap(err, "查询YYAI余额失败")
	}
	if yyaiBalance != nil {
		yyaiBefore = yyaiBalance.AvailableAmount.Add(yyaiBalance.FrozenAmount)
	}
	if err := balanceRepo.UpdateBalanceTx(ctx, tx, purchase.UserID, "YYAI", yyaiAmount, decimal.Zero); err != nil {
		return true, gerror.Wrap(err, "发放YYAI到余额失败")
	}
	yyaiAfter := yyaiBefore.Add(yyaiAmount)
	if err := writeBalanceChangeLogTx(ctx, tx, purchase.UserID, "YYAI", consts.ChangeTypeYYAIToBalance, yyaiAmount, yyaiBefore, yyaiAfter, purchase.PackageNo, purchase.ID, "admin buy node yyai grant to balance", consts.OperatorTypeSystem); err != nil {
		return true, gerror.Wrap(err, "写入YYAI发放审计日志失败")
	}

	return true, nil
}

// createPendingTripleGrantTx 创建Triple待发放记录，返回是否创建了记录
func (s *buyBootstrapService) createPendingTripleGrantTx(ctx context.Context, tx gdb.TX, wallet string, purchase *cobo.NodePurchaseEntity) (bool, error) {
	if purchase == nil || purchase.ID == 0 {
		return false, nil
	}
	if purchase.NodeType == 6 {
		return false, nil
	}

	rate := cobo.GetTripleCouponRate(purchase.NodeType, time.Now())
	usdtAmount := purchase.Amount
	tripleAmount := usdtAmount.Mul(decimal.NewFromFloat(rate))

	if tripleAmount.IsZero() {
		return false, nil
	}

	tokenGrantRepo := repo.NewNodeTokenGrantRepository()

	grant := &cobo.NodeTokenGrantEntity{
		PurchaseID:  purchase.ID,
		UserID:      purchase.UserID,
		Wallet:      wallet,
		TokenType:   cobo.TokenTypeTriple,
		TokenSymbol: "Triple",
		TokenChain:  "BSC",
		TokenAddr:   "",
		GrantAmount: tripleAmount.String(),
		Status:      cobo.NodeTokenGrantStatusPending,
		UsdtAmount:  usdtAmount,
		YyaiAmount:  tripleAmount,
	}

	if err := tokenGrantRepo.CreateTx(ctx, tx, grant); err != nil {
		return false, err
	}
	return true, nil
}

// increaseNodeSoldShares 更新 node_info 的 sold_shares
func (s *buyBootstrapService) increaseNodeSoldShares(ctx context.Context, tx gdb.TX, nodeType int) error {
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
		return gerror.New("节点已售罄")
	}

	return nil
}
