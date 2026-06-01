package cobo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity/cobo"
	"XWFrame/internal/repository"
	repo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/service/cobo/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// AdminNodeGiftService 后台赠送节点服务接口
type AdminNodeGiftService interface {
	// GiftNode 赠送节点给用户
	GiftNode(ctx context.Context, req *GiftNodeReq) (*GiftNodeRes, error)
	// ListGiftNodes 分页查询赠送节点记录
	ListGiftNodes(ctx context.Context, req *ListGiftNodesReq) (*ListGiftNodesRes, error)
	// DeleteGiftNode 硬删除赠送节点记录
	DeleteGiftNode(ctx context.Context, req *DeleteGiftNodeReq) error
}

// GiftNodeReq 赠送节点请求
type GiftNodeReq struct {
	AdminID          int64   `json:"admin_id"`           // 操作人ID
	UserID           int64   `json:"user_id"`            // 目标用户ID
	WalletAddress    string  `json:"wallet_address"`     // 目标用户钱包地址（与 user_id 二选一）
	NodeType         int     `json:"node_type"`          // 节点类型 1-4,6
	Remark           string  `json:"remark"`             // 赠送备注
	EnableExempt     bool    `json:"enable_exempt"`      // 开通提现业绩豁免
	TripleCouponRate float64 `json:"triple_coupon_rate"` // 手动三倍券比例，单位百分比（0-100）
}

// GiftNodeRes 赠送节点响应
type GiftNodeRes struct {
	PackageNo  string `json:"package_no"`  // 包号
	NodeType   int    `json:"node_type"`   // 节点类型
	Amount     string `json:"amount"`      // 节点金额
	PowerValue string `json:"power_value"` // 算力值
	CreatedAt  int64  `json:"created_at"`  // 创建时间
}

// ListGiftNodesReq 赠送节点记录分页请求
type ListGiftNodesReq struct {
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
	UserID        int64  `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
}

// GiftNodeRecordItem 赠送节点记录
type GiftNodeRecordItem struct {
	ID                 int64   `json:"id"`
	UserID             int64   `json:"user_id"`
	WalletAddress      string  `json:"wallet_address"`
	PackageNo          string  `json:"package_no"`
	NodeType           int     `json:"node_type"`
	Amount             string  `json:"amount"`
	PowerValue         string  `json:"power_value"`
	TripleCouponAmount string  `json:"triple_coupon_amount"`
	TripleCouponRate   float64 `json:"triple_coupon_rate"`
	GiftBy             int64   `json:"gift_by"`
	GiftRemark         string  `json:"gift_remark"`
	CreatedAt          string  `json:"created_at"`
}

// ListGiftNodesRes 赠送节点记录分页响应
type ListGiftNodesRes struct {
	List     []GiftNodeRecordItem `json:"list"`
	Total    int                  `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// DeleteGiftNodeReq 删除赠送节点请求
type DeleteGiftNodeReq struct {
	ID int64 `json:"id"`
}

// adminNodeGiftService 后台赠送节点服务实现
type adminNodeGiftService struct {
	nodePurchaseRepo repo.INodePurchaseRepository
	userRepo         repository.IUserRepository
	nodePriceRepo    *repository.NodePriceRepository
	tokenGrantRepo   repo.INodeTokenGrantRepository
}

// NewAdminNodeGiftService 创建后台赠送节点服务
func NewAdminNodeGiftService() AdminNodeGiftService {
	return &adminNodeGiftService{
		nodePurchaseRepo: repo.NewNodePurchaseRepository(),
		userRepo:         repository.NewUserRepository(),
		nodePriceRepo:    repository.NewNodePriceRepository(),
		tokenGrantRepo:   repo.NewNodeTokenGrantRepository(),
	}
}

// GiftNode 赠送节点给用户（不产生佣金）
func (s *adminNodeGiftService) GiftNode(ctx context.Context, req *GiftNodeReq) (*GiftNodeRes, error) {
	g.Log().Infof(ctx, "[AdminGiftNode] 开始赠送: admin_id=%d, user_id=%d, node_type=%d", req.AdminID, req.UserID, req.NodeType)
	if req.TripleCouponRate < 0 || req.TripleCouponRate > 100 {
		return nil, gerror.New("invalid triple_coupon_rate: must be between 0 and 100")
	}

	// 1. 验证并获取用户
	var walletAddress string
	if req.UserID > 0 {
		user, err := s.userRepo.GetUserById(ctx, req.UserID)
		if err != nil {
			return nil, gerror.Wrap(err, "获取用户信息失败")
		}
		if user == nil {
			return nil, gerror.New("用户不存在")
		}
		walletAddress = user.WalletAddress
	} else if req.WalletAddress != "" {
		walletAddress = strings.ToLower(strings.TrimSpace(req.WalletAddress))
		user, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
		if err != nil {
			return nil, gerror.Wrap(err, "获取用户信息失败")
		}
		if user == nil {
			return nil, gerror.New("用户不存在")
		}
		req.UserID = user.Id
	} else {
		return nil, gerror.New("请提供 user_id 或 wallet_address")
	}

	// 2. 获取节点配置
	typeConfig := model.GetNodeTypeConfig(req.NodeType)
	if typeConfig == nil {
		return nil, gerror.New("不支持的节点类型")
	}

	priceConfig, err := s.nodePriceRepo.GetByNodeType(ctx, req.NodeType)
	if err != nil {
		return nil, gerror.Wrap(err, "获取节点价格配置失败")
	}

	amount := decimal.NewFromFloat(typeConfig.Amount)
	if priceConfig != nil {
		amount = priceConfig.Price
	}
	powerValue := amount.Mul(decimal.NewFromFloat(typeConfig.PowerMultiplier))

	// 3. 创建赠送记录（不产生佣金）
	now := time.Now()
	purchase := &cobo.NodePurchaseEntity{
		UserID:      req.UserID,
		PackageNo:   fmt.Sprintf("GIFT%s", generateGiftPackageNo()),
		RequestID:   "", // 赠送不需要 request_id
		NodeType:    req.NodeType,
		Amount:      amount,
		PowerValue:  powerValue,
		StakeAmount: decimal.Zero, // 赠送节点不质押
		Status:      cobo.NodeStatusRunning,
		StartTime:   now,
		IsGift:      1,           // 标记为赠送
		GiftBy:      req.AdminID, // 记录操作人
		GiftRemark:  req.Remark,
	}

	// 4. 开启事务
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 4.1 创建购买记录
		if err := s.nodePurchaseRepo.Create(ctx, tx, purchase); err != nil {
			return gerror.Wrap(err, "创建赠送记录失败")
		}

		// 4.2 更新节点份额（但不扣减余额，不发放奖励）
		if err := s.increaseNodeSoldShares(ctx, tx, req.NodeType); err != nil {
			return gerror.Wrap(err, "更新节点份额失败")
		}

		// 4.3 开通提现业绩豁免
		if req.EnableExempt {
			if _, err := tx.Model("user_info").Ctx(ctx).
				Where("id = ?", req.UserID).
				Where("node_exempt = ?", false).
				Data(g.Map{
					"node_exempt": true,
					"updated_at":  time.Now(),
				}).Update(); err != nil {
				g.Log().Warningf(ctx, "[AdminGiftNode] 开通业绩豁免失败: user_id=%d, err=%v", req.UserID, err)
				// 不影响赠送流程
			} else {
				g.Log().Infof(ctx, "[AdminGiftNode] 已开通业绩豁免: user_id=%d", req.UserID)
			}
		}

		// 4.4 创建YYAI待发放记录（赠送节点也发放YYAI）
		if err := s.createPendingYYGrantTx(ctx, tx, walletAddress, purchase); err != nil {
			g.Log().Warningf(ctx, "[AdminGiftNode] 创建YYAI发放记录失败: %v", err)
			// 不影响赠送流程，记录日志即可
		}

		// 4.4 创建Triple待发放记录（赠送节点也发放Triple）
		if err := s.createPendingTripleGrantTx(ctx, tx, walletAddress, purchase, req.TripleCouponRate); err != nil {
			g.Log().Warningf(ctx, "[AdminGiftNode] 创建Triple发放记录失败: %v", err)
			// 不影响赠送流程，记录日志即可
		}

		g.Log().Infof(ctx, "[AdminGiftNode] 事务完成: admin_id=%d, user_id=%d, package_no=%s",
			req.AdminID, req.UserID, purchase.PackageNo)

		return nil
	})

	if err != nil {
		g.Log().Errorf(ctx, "[AdminGiftNode] 赠送失败: admin_id=%d, user_id=%d, err=%v", req.AdminID, req.UserID, err)
		return nil, err
	}

	g.Log().Infof(ctx, "[AdminGiftNode] 赠送成功: admin_id=%d, user_id=%d, package_no=%s",
		req.AdminID, req.UserID, purchase.PackageNo)

	return &GiftNodeRes{
		PackageNo:  purchase.PackageNo,
		NodeType:   purchase.NodeType,
		Amount:     purchase.Amount.String(),
		PowerValue: purchase.PowerValue.String(),
		CreatedAt:  now.Unix(),
	}, nil
}

// ListGiftNodes 分页查询赠送节点记录
func (s *adminNodeGiftService) ListGiftNodes(ctx context.Context, req *ListGiftNodesReq) (*ListGiftNodesRes, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	baseWhere := func(m *gdb.Model) {
		m.Where("p.is_gift = ?", 1)
		if req.UserID > 0 {
			m.Where("p.user_id = ?", req.UserID)
		}
		if strings.TrimSpace(req.WalletAddress) != "" {
			walletAddress := strings.ToLower(strings.TrimSpace(req.WalletAddress))
			m.Where("u.wallet_address = ?", walletAddress)
		}
	}

	mCount := g.DB().Ctx(ctx).Model("cobo_node_purchase p").
		LeftJoin("user_info u", "u.id = p.user_id")
	baseWhere(mCount)

	total, err := mCount.Count()
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query gift node records total")
	}

	var records []struct {
		ID                 int64           `orm:"id"`
		UserID             int64           `orm:"user_id"`
		WalletAddress      string          `orm:"wallet_address"`
		PackageNo          string          `orm:"package_no"`
		NodeType           int             `orm:"node_type"`
		Amount             decimal.Decimal `orm:"amount"`
		PowerValue         decimal.Decimal `orm:"power_value"`
		TripleCouponAmount decimal.Decimal `orm:"triple_coupon_amount"`
		TripleCouponRate   decimal.Decimal `orm:"triple_coupon_rate"`
		GiftBy             int64           `orm:"gift_by"`
		GiftRemark         string          `orm:"gift_remark"`
		CreatedAt          *time.Time      `orm:"created_at"`
	}

	mData := g.DB().Ctx(ctx).Model("cobo_node_purchase p").
		LeftJoin("user_info u", "u.id = p.user_id").
		LeftJoin("cobo_node_token_grant tg", fmt.Sprintf("tg.purchase_id = p.id AND tg.token_type = '%s'", cobo.TokenTypeTriple))
	baseWhere(mData)

	err = mData.Fields(
		"p.id",
		"p.user_id",
		"COALESCE(u.wallet_address, '') AS wallet_address",
		"p.package_no",
		"p.node_type",
		"p.amount",
		"p.power_value",
		"COALESCE(tg.grant_amount::numeric, 0) AS triple_coupon_amount",
		"CASE WHEN p.amount > 0 THEN ROUND((COALESCE(tg.grant_amount::numeric, 0) / p.amount) * 100, 4) ELSE 0 END AS triple_coupon_rate",
		"p.gift_by",
		"p.gift_remark",
		"p.created_at",
	).
		OrderDesc("p.created_at").
		Page(req.Page, req.PageSize).
		Scan(&records)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to query gift node records")
	}

	items := make([]GiftNodeRecordItem, 0, len(records))
	for _, record := range records {
		createdAt := ""
		if record.CreatedAt != nil {
			createdAt = record.CreatedAt.Format(time.RFC3339)
		}
		items = append(items, GiftNodeRecordItem{
			ID:                 record.ID,
			UserID:             record.UserID,
			WalletAddress:      record.WalletAddress,
			PackageNo:          record.PackageNo,
			NodeType:           record.NodeType,
			Amount:             record.Amount.StringFixed(2),
			PowerValue:         record.PowerValue.StringFixed(2),
			TripleCouponAmount: record.TripleCouponAmount.String(),
			TripleCouponRate:   record.TripleCouponRate.InexactFloat64(),
			GiftBy:             record.GiftBy,
			GiftRemark:         record.GiftRemark,
			CreatedAt:          createdAt,
		})
	}

	return &ListGiftNodesRes{
		List:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// DeleteGiftNode 硬删除赠送节点记录
func (s *adminNodeGiftService) DeleteGiftNode(ctx context.Context, req *DeleteGiftNodeReq) error {
	if req.ID <= 0 {
		return gerror.New("invalid id")
	}

	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var purchase *cobo.NodePurchaseEntity
		if err := tx.Model("cobo_node_purchase").Ctx(ctx).Where("id = ?", req.ID).Scan(&purchase); err != nil {
			return gerror.Wrap(err, "failed to query gift purchase record")
		}
		if purchase == nil || purchase.ID == 0 {
			return gerror.New("gift node record not found")
		}
		if purchase.IsGift != 1 {
			return gerror.New("record is not a gift node")
		}

		grantSentCount, err := tx.Model("cobo_node_token_grant").Ctx(ctx).
			Where("purchase_id = ?", purchase.ID).
			Where("status = ?", cobo.NodeTokenGrantStatusSent).
			Count()
		if err != nil {
			return gerror.Wrap(err, "failed to check token grant status")
		}
		if grantSentCount > 0 {
			return gerror.New("cannot delete: token grant already sent")
		}

		if _, err = tx.Model("cobo_node_token_grant").Ctx(ctx).Where("purchase_id = ?", purchase.ID).Delete(); err != nil {
			return gerror.Wrap(err, "failed to delete token grants")
		}

		if _, err = tx.Model("cobo_node_purchase").Ctx(ctx).Where("id = ?", purchase.ID).Delete(); err != nil {
			return gerror.Wrap(err, "failed to delete gift purchase record")
		}

		level := fmt.Sprintf("NODE%d", purchase.NodeType)
		list, err := tx.Model("node_info").Ctx(ctx).
			Where("UPPER(node_level) = ?", level).
			Where("sold_shares > 0").
			Order("id DESC").
			Limit(1).
			All()
		if err != nil {
			return gerror.Wrap(err, "failed to query node info for sold shares rollback")
		}
		if len(list) > 0 {
			nodeInfoID := list[0]["id"].Int64()
			if _, err = tx.Model("node_info").Ctx(ctx).
				Where("id = ?", nodeInfoID).
				Where("sold_shares > 0").
				Data(g.Map{
					"sold_shares": gdb.Raw("sold_shares - 1"),
					"status":      gdb.Raw("CASE WHEN status = 3 THEN 2 ELSE status END"),
					"updated_at":  time.Now(),
				}).
				Update(); err != nil {
				return gerror.Wrap(err, "failed to rollback node sold shares")
			}
		}

		return nil
	})
}

// generateGiftPackageNo 生成赠送包号
func generateGiftPackageNo() string {
	return fmt.Sprintf("%d%04d", time.Now().Unix(), time.Now().Nanosecond()%10000)
}

// increaseNodeSoldShares 更新 node_info 的 sold_shares（复用 nodePurchaseService 的方法）
func (s *adminNodeGiftService) increaseNodeSoldShares(ctx context.Context, tx gdb.TX, nodeType int) error {
	level := fmt.Sprintf("NODE%d", nodeType)

	list, err := tx.Model("node_info").
		Ctx(ctx).
		Where("UPPER(node_level) = ?", level).
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

// createPendingYYGrantTx 创建YYAI待发放记录（赠送节点同样发放YYAI）
func (s *adminNodeGiftService) createPendingYYGrantTx(ctx context.Context, tx gdb.TX, wallet string, purchase *cobo.NodePurchaseEntity) error {
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
		g.Log().Warningf(ctx, "[AdminGiftNode] 获取YYAI最新价格失败: %v", err)
		latestPrice = nil
	}

	var yyaiPrice decimal.Decimal
	var yyaiAmount decimal.Decimal
	usdtAmount := purchase.Amount

	if latestPrice != nil && latestPrice.Price.GreaterThan(decimal.Zero) {
		yyaiPrice = latestPrice.Price
		// 计算YYAI数量: USDT金额 / YYAI价格(USD)
		yyaiAmount = usdtAmount.Div(yyaiPrice)
		g.Log().Infof(ctx, "[AdminGiftNode] 计算YYAI发放数量: usdt=%s, yyai_price=%s, yyai_amount=%s",
			usdtAmount.String(), yyaiPrice.String(), yyaiAmount.String())
	} else {
		g.Log().Warningf(ctx, "[AdminGiftNode] YYAI价格无效，无法计算发放数量: purchase_id=%d", purchase.ID)
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
		GrantAmount: yyaiAmount.String(),
		Status:      cobo.NodeTokenGrantStatusPending,
		YyaiPrice:   yyaiPrice,
		UsdtAmount:  usdtAmount,
		YyaiAmount:  yyaiAmount,
	}

	return s.tokenGrantRepo.CreateTx(ctx, tx, grant)
}

// createPendingTripleGrantTx 创建Triple(三倍券)待发放记录（赠送节点同样发放）
func (s *adminNodeGiftService) createPendingTripleGrantTx(ctx context.Context, tx gdb.TX, wallet string, purchase *cobo.NodePurchaseEntity, manualRatePercent float64) error {
	if purchase == nil || purchase.ID == 0 {
		return nil
	}

	// 100U 体验节点不发放Triple
	if purchase.NodeType == 6 {
		return nil
	}

	// 优先使用手动传入比例（百分比），否则走既有档位规则
	rate := cobo.GetTripleCouponRate(purchase.NodeType, time.Now())
	if manualRatePercent > 0 {
		rate = manualRatePercent / 100
	}

	usdtAmount := purchase.Amount
	tripleAmount := usdtAmount.Mul(decimal.NewFromFloat(rate))

	g.Log().Infof(ctx, "[AdminGiftNode] 计算Triple发放数量: usdt=%s, rate=%.0f%%, triple_amount=%s",
		usdtAmount.String(), rate*100, tripleAmount.String())

	if tripleAmount.IsZero() {
		return nil
	}

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

	return s.tokenGrantRepo.CreateTx(ctx, tx, grant)
}
