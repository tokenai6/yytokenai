package dao

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/frame/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IUserDao 用户数据访问接口（只包含基础增删改查）
type IUserDao interface {
	// 基础CRUD操作
	Create(ctx context.Context, user *entity.UserEntity) error
	GetById(ctx context.Context, id int64) (*entity.UserEntity, error)
	UpdateById(ctx context.Context, id int64, data map[string]interface{}) error
	DeleteById(ctx context.Context, id int64) error

	// 批量更新操作
	UpdateParentWalletAddressByOldWalletAddress(ctx context.Context, oldWalletAddress, newWalletAddress string) error

	// 基础查询操作
	GetByWalletAddress(ctx context.Context, walletAddress string) (*entity.UserEntity, error)
	ExistsByWalletAddress(ctx context.Context, walletAddress string) (bool, error)
	ExistsByInviteCode(ctx context.Context, inviteCode string) (bool, error)

	// 基础列表查询
	GetList(ctx context.Context, req *GetUserListReq) (*GetUserListRes, error)

	// 获取直推用户（分页，通过上级钱包地址查询，日期维度过滤，按当日拼团业绩降序）
	GetDirectReferralsByWalletAddress(ctx context.Context, walletAddress string, page, pageSize int, startDate, endDate string) ([]*entity.UserEntity, int, error)

	// 获取直推人数（通过用户ID查询）
	GetDirectCountByUserID(ctx context.Context, userID int64) (int, error)

	// 获取30层推荐关系（基于钱包地址）
	Get30LayerReferralsByWalletAddress(ctx context.Context, userID int64) (map[int][]int64, error)
	// 获取所有激活用户
	GetActiveUsers(ctx context.Context) ([]*entity.UserEntity, error)

	// 获取团队总人数
	GetTeamTotalCount(ctx context.Context, userID int64) (int, error)
	// 获取直推总人数
	GetDirectReferralCount(ctx context.Context, userID int64) (int, error)
	// 用户业绩查询
	GetSubUserPerformance(ctx context.Context, inviteCode string) (string, error)
	GetUsersPurchaseAmount(ctx context.Context, userIds []int64) (map[int64]int64, error)
	GetUserPurchaseAmount(ctx context.Context, userID int64) (int64, error)

	// 用户关系查询
	GetByInviteCode(ctx context.Context, inviteCode string) (*entity.UserEntity, error)
	// GetDirectDescendants 获取直接下级用户列表（兼容 parent_invite_code / parent_wallet_address 两种关系字段）
	GetDirectDescendants(ctx context.Context, parentInviteCode, parentWalletAddress string) ([]*entity.UserEntity, error)

	// 顶点账号查询
	GetVertexUser(ctx context.Context) (*entity.UserEntity, error)

	// GetDataByIds 根据ID列表获取用户列表
	GetDataByIds(ctx context.Context, ids []int64) ([]*entity.UserEntity, error)

	// GetServiceCenterUsers 获取服务中心用户列表
	GetServiceCenterUsers(ctx context.Context) ([]*entity.UserEntity, error)

	// GetAllDescendantIDs 获取用户所有下级用户ID（不包含当前用户）
	GetAllDescendantIDs(ctx context.Context, userID int64) ([]int64, error)

	// UpdateTeamCanWithdraw 更新团队成员的can_withdraw字段（无限代，包括用户自己）
	UpdateTeamCanWithdraw(ctx context.Context, userID int64, withdrawType string) error

	// UpdateUserCanWithdraw 直接更新单个用户的can_withdraw字段
	UpdateUserCanWithdraw(ctx context.Context, userID int64, canWithdraw bool) error

	// UpdateTeamStakeRate 更新团队成员的质押收益率
	UpdateTeamStakeRate(ctx context.Context, userID int64, stakeRate float64) error

	// UpdateUserNodeExempt 更新用户赠送节点业绩豁免状态
	UpdateUserNodeExempt(ctx context.Context, userID int64, nodeExempt bool) error

	// UpdateTeamTeamId 批量更新团队成员的 team_id（无限代，包括用户自己）
	UpdateTeamTeamId(ctx context.Context, userID int64, teamId *int64) error

	// UpdateDescendantsTeamIdExcludingLeaderSubtrees 更新用户下级（不含用户自己）的 team_id，排除团队长及其子树
	UpdateDescendantsTeamIdExcludingLeaderSubtrees(ctx context.Context, userID int64, teamId *int64) error

	// GetAncestorsByUserIDs 批量获取用户的所有祖先（包含当前用户及其父、祖等）
	GetAncestorsByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]int64, error)

	// GetUserIDsByTeamId 根据团队ID获取用户ID列表
	GetUserIDsByTeamId(ctx context.Context, teamId int64) ([]int64, error)
}

// GetUserListReq 获取用户列表请求
type GetUserListReq struct {
	model.PageReq
	UserID           int64  `json:"user_id"`
	WalletAddress    string `json:"wallet_address"`
	DepositAddress   string `json:"deposit_address"`
	ParentUserID     int64  `json:"parent_user_id"`
	ParentInviteCode string `json:"parent_invite_code"`
	ParentAddress    string `json:"parent_address"`
	LeaderLevel      string `json:"leader_level"`
	AssetSymbol      string `json:"asset_symbol"`
	CanWithdraw      int    `json:"can_withdraw"`
	NodeExempt       int    `json:"node_exempt"`
	AdjustedOnly     int    `json:"adjusted_only"`
	CreatedAtStart   string `json:"created_at_start"`
	CreatedAtEnd     string `json:"created_at_end"`
	Status           int    `json:"status"`
}

type UserListSummary struct {
	TotalRewardUSDT     string `json:"total_reward_usdt"`
	WithdrawnRewardUSDT string `json:"withdrawn_reward_usdt"`
	RemainingRewardUSDT string `json:"remaining_reward_usdt"`
	StakeTotal          string `json:"stake_total"`
	NodePurchaseTotal   string `json:"node_purchase_total"`
	GroupRechargeUSDT   string `json:"group_recharge_usdt"`
	TicketCount         string `json:"ticket_count"`
	GiftTriple          string `json:"gift_triple"`
	SZPN                string `json:"szpn"`
	JU                  string `json:"ju"`
	YY                  string `json:"yy"`
	YYAI                string `json:"yyai"`
}

// GetUserListRes 获取用户列表响应
type GetUserListRes struct {
	model.PageRes
	List    []*entity.UserEntity `json:"list"`
	Summary UserListSummary      `json:"summary"`
}

// userDao 用户数据访问实现
type userDao struct {
	db gdb.DB
}

// NewUserDao 创建用户数据访问实例
func NewUserDao() IUserDao {
	return &userDao{
		db: db.GetDB(),
	}
}

// Create 创建用户
func (d *userDao) Create(ctx context.Context, user *entity.UserEntity) error {
	result, err := d.db.Model("user_info").FieldsEx("id", "created_at", "updated_at").Data(user).InsertAndGetId()
	if err != nil {
		return err
	}
	fmt.Printf("注册的用户id = %d \n", result)
	user.Id = result
	return nil
}

// GetById 根据ID获取用户
func (d *userDao) GetById(ctx context.Context, id int64) (*entity.UserEntity, error) {
	var user entity.UserEntity
	err := d.db.Model("user_info").Where("id", id).Scan(&user)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if user.Id == 0 {
		return nil, nil
	}
	return &user, nil
}

// GetByWalletAddress 根据钱包地址获取用户
func (d *userDao) GetByWalletAddress(ctx context.Context, walletAddress string) (*entity.UserEntity, error) {
	var user entity.UserEntity
	addr := strings.ToLower(strings.TrimSpace(walletAddress))
	err := d.db.Model("user_info").Ctx(ctx).
		Where("LOWER(wallet_address) = ?", addr).
		Scan(&user)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if user.Id == 0 {
		return nil, nil
	}
	return &user, nil
}

// ExistsByWalletAddress 检查钱包地址是否存在
func (d *userDao) ExistsByWalletAddress(ctx context.Context, walletAddress string) (bool, error) {
	addr := strings.ToLower(strings.TrimSpace(walletAddress))
	count, err := d.db.Model("user_info").Ctx(ctx).
		Where("LOWER(wallet_address) = ?", addr).
		Count()
	return count > 0, err
}

// ExistsByInviteCode 检查邀请码是否存在
func (d *userDao) ExistsByInviteCode(ctx context.Context, inviteCode string) (bool, error) {
	count, err := d.db.Model("user_info").Where("invite_code", inviteCode).Count()
	return count > 0, err
}

// UpdateById 根据ID更新用户
func (d *userDao) UpdateById(ctx context.Context, id int64, data map[string]interface{}) error {
	data["updated_at"] = time.Now()
	_, err := d.db.Model("user_info").Where("id", id).Data(data).Update()
	return err
}

// UpdateParentWalletAddressByOldWalletAddress 根据旧的父钱包地址批量更新父钱包地址
func (d *userDao) UpdateParentWalletAddressByOldWalletAddress(ctx context.Context, oldWalletAddress, newWalletAddress string) error {
	_, err := d.db.Model("user_info").
		Where("parent_wallet_address", oldWalletAddress).
		Data(map[string]interface{}{
			"parent_wallet_address": newWalletAddress,
			"updated_at":            time.Now(),
		}).
		Update()
	return err
}

// DeleteById 根据ID删除用户
func (d *userDao) DeleteById(ctx context.Context, id int64) error {
	_, err := d.db.Model("user_info").Where("id", id).Delete()
	return err
}

// GetList 获取用户列表
func (d *userDao) GetList(ctx context.Context, req *GetUserListReq) (*GetUserListRes, error) {
	query := d.db.Model("user_info")

	// 添加查询条件
	if req.UserID > 0 {
		query = query.Where("id", req.UserID)
	}
	if req.WalletAddress != "" {
		pattern := utils.BuildLikePattern(strings.ToLower(req.WalletAddress))
		query = query.Where("(LOWER(wallet_address) LIKE ? OR EXISTS (SELECT 1 FROM user_deposit_address uda WHERE uda.user_id = user_info.id AND uda.is_valid = true AND LOWER(uda.address) LIKE ?))", pattern, pattern)
	}
	if req.DepositAddress != "" {
		pattern := utils.BuildLikePattern(strings.ToLower(req.DepositAddress))
		query = query.Where("EXISTS (SELECT 1 FROM user_deposit_address uda WHERE uda.user_id = user_info.id AND uda.is_valid = true AND LOWER(uda.address) LIKE ?)", pattern)
	}
	if req.ParentUserID > 0 {
		var parent struct {
			WalletAddress string `json:"wallet_address"`
		}
		if err := d.db.Model("user_info").Ctx(ctx).Fields("wallet_address").Where("id", req.ParentUserID).Scan(&parent); err != nil {
			return nil, err
		}
		if parent.WalletAddress == "" {
			return &GetUserListRes{PageRes: model.PageRes{Page: req.Page, PageSize: req.PageSize, Total: 0, Pages: 0}, List: []*entity.UserEntity{}}, nil
		}
		query = query.Where("LOWER(parent_wallet_address) = ?", strings.ToLower(parent.WalletAddress))
	}
	if req.ParentInviteCode != "" {
		query = query.Where("parent_invite_code", req.ParentInviteCode)
	}
	if req.ParentAddress != "" {
		pattern := utils.BuildLikePattern(strings.ToLower(req.ParentAddress))
		query = query.Where("LOWER(parent_wallet_address) LIKE ?", pattern)
	}
	if req.CanWithdraw == 1 {
		query = query.Where("can_withdraw", true)
	} else if req.CanWithdraw == 0 {
		query = query.Where("can_withdraw", false)
	}
	if req.NodeExempt == 1 {
		query = query.Where("node_exempt", true)
	} else if req.NodeExempt == 0 {
		query = query.Where("node_exempt", false)
	}
	if req.AdjustedOnly == 1 {
		query = query.Where(`EXISTS (
			SELECT 1 FROM user_vip_adjustment uva
			WHERE uva.user_id = user_info.id
			  AND uva.expire_time > CURRENT_TIMESTAMP
		)`)
	}
	if req.CreatedAtStart != "" {
		query = query.Where("created_at >= ?", req.CreatedAtStart)
	}
	if req.CreatedAtEnd != "" {
		query = query.Where("created_at <= ?", req.CreatedAtEnd)
	}
	if req.LeaderLevel != "" {
		query = query.Where(`EXISTS (
			SELECT 1 FROM group_purchase_leadership_reward_detail gl
			WHERE gl.user_id = user_info.id AND gl.level_key = ?
		)`, req.LeaderLevel)
	}
	if req.AssetSymbol != "" {
		query = query.Where(`EXISTS (
			SELECT 1 FROM account_balance ab
			WHERE ab.user_id = user_info.id AND UPPER(ab.symbol) = UPPER(?) AND ab.total_amount > 0
		)`, req.AssetSymbol)
	}
	if req.Status != 0 {
		query = query.Where("status", req.Status)
	}

	// 获取总数
	total, err := query.Count()
	if err != nil {
		return nil, err
	}

	// 分页查询
	var users []*entity.UserEntity
	err = query.Page(req.Page, req.PageSize).Order("id DESC").Scan(&users)
	if err != nil {
		return nil, err
	}

	// 计算总页数
	pages := (total + req.PageSize - 1) / req.PageSize

	summary := d.getUserListSummary(ctx, query)

	return &GetUserListRes{
		PageRes: model.PageRes{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    total,
			Pages:    pages,
		},
		List:    users,
		Summary: summary,
	}, nil
}

func (d *userDao) getUserListSummary(ctx context.Context, query *gdb.Model) UserListSummary {
	var rows []struct {
		Id int64 `json:"id"`
	}
	if err := query.Clone().Fields("id").Scan(&rows); err != nil || len(rows) == 0 {
		return UserListSummary{}
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Id)
	}
	getSum := func(model, field string, where string, args ...interface{}) string {
		m := d.db.Model(model).Ctx(ctx).WhereIn("user_id", ids)
		if where != "" {
			m = m.Where(where, args...)
		}
		val, err := m.Fields("COALESCE(SUM(" + field + "), 0)").Value()
		if err != nil || val == nil {
			return "0"
		}
		return val.String()
	}
	totalReward := d.getUserListTotalRewardUSDT(ctx, ids)
	withdrawTotal := getSum("cobo_withdraw_request", "amount", "symbol='USDT' AND status=4")
	remainingQuota := getSum("user_quota", "remaining_quota", "")
	servicePkg := getSum("staking_v2_order", "amount", "")
	nodePurchaseTotal := getSum("cobo_node_purchase", "amount", "is_gift = 0")
	groupRecharge := getSum("apg_player", "payment_amount::numeric", "UPPER(payment_token)='USDT'")
	ticketCount := getSum("account_balance", "total_amount", "UPPER(symbol) IN ('TICKET','TICKETS')")
	giftTriple := getSum("cobo_node_token_grant", "grant_amount::numeric", "token_type='Triple'")
	szpn := getSum("account_balance", "total_amount", "UPPER(symbol)='SZPN'")
	ju := getSum("account_balance", "total_amount", "UPPER(symbol)='JU'")
	yy := getSum("account_balance", "total_amount", "UPPER(symbol)='YY'")
	yyai := getSum("account_balance", "total_amount", "UPPER(symbol)='YYAI'")

	return UserListSummary{
		TotalRewardUSDT:     totalReward,
		WithdrawnRewardUSDT: withdrawTotal,
		RemainingRewardUSDT: remainingQuota,
		StakeTotal:          servicePkg,
		NodePurchaseTotal:   nodePurchaseTotal,
		GroupRechargeUSDT:   groupRecharge,
		TicketCount:         ticketCount,
		GiftTriple:          giftTriple,
		SZPN:                szpn,
		JU:                  ju,
		YY:                  yy,
		YYAI:                yyai,
	}
}

func (d *userDao) getUserListTotalRewardUSDT(ctx context.Context, userIDs []int64) string {
	if len(userIDs) == 0 {
		return "0"
	}

	getSumByUserField := func(table, userField, amountField, where string, args ...interface{}) decimal.Decimal {
		m := d.db.Model(table).Ctx(ctx).WhereIn(userField, userIDs)
		if where != "" {
			m = m.Where(where, args...)
		}
		val, err := m.Fields("COALESCE(SUM(" + amountField + "), 0)").Value()
		if err != nil || val == nil {
			return decimal.Zero
		}
		d, parseErr := decimal.NewFromString(strings.TrimSpace(val.String()))
		if parseErr != nil {
			return decimal.Zero
		}
		return d
	}

	total := decimal.Zero
	total = total.Add(getSumByUserField("cobo_node_purchase", "direct_reward_user_id", "direct_reward_amount", "direct_reward_amount > 0 AND is_gift = 0"))
	total = total.Add(getSumByUserField("cobo_balance_change_log", "user_id", "amount", "change_type = 'staking_v2_referral_direct'"))
	total = total.Add(getSumByUserField("cobo_balance_change_log", "user_id", "amount", "change_type = 'staking_v2_referral_indirect'"))
	total = total.Add(getSumByUserField("group_match_team_reward_distribution", "user_id", "granted_amount", "COALESCE(granted_amount, 0) > 0"))
	total = total.Add(getSumByUserField("group_purchase_leadership_reward_detail", "user_id", "granted_amount", "COALESCE(granted_amount, 0) > 0"))
	total = total.Add(getSumByUserField("group_purchase_leadership_weight_reward_detail", "user_id", "granted_amount", "COALESCE(granted_amount, 0) > 0"))

	matchVal, err := d.db.GetValue(ctx, `
		SELECT COALESCE(SUM(t.session_reward), 0)
		FROM (
			SELECT o.user_id, o.session_id, COALESCE(SUM(o.reward_amount), 0) AS session_reward
			FROM group_match_order o
			WHERE o.user_id IN (?)
			  AND o.is_winner = true
			GROUP BY o.user_id, o.session_id
			HAVING COALESCE(SUM(o.reward_amount), 0) > 0
		) t
	`, userIDs)
	if err == nil && matchVal != nil {
		if dVal, parseErr := decimal.NewFromString(strings.TrimSpace(matchVal.String())); parseErr == nil {
			total = total.Add(dVal)
		}
	}

	return total.String()
}

// GetDirectReferralsByWalletAddress 获取直推用户（分页，通过上级钱包地址查询，日期维度过滤，按最新拼团业绩降序）
func (d *userDao) GetDirectReferralsByWalletAddress(ctx context.Context, walletAddress string, page, pageSize int, startDate, endDate string) ([]*entity.UserEntity, int, error) {
	// 先查询钱包地址对应的邀请码
	var parentUser entity.UserEntity
	err := d.db.Model("user_info").Ctx(ctx).
		Where("wallet_address", walletAddress).
		Scan(&parentUser)
	if err != nil {
		return nil, 0, err
	}
	if parentUser.Id == 0 {
		return []*entity.UserEntity{}, 0, nil
	}

	// count
	countSQL := `
		SELECT COUNT(*) FROM user_info u
		WHERE u.parent_invite_code = ?
	`
	countArgs := []interface{}{parentUser.InviteCode}
	if startDate != "" {
		countSQL += " AND DATE(u.created_at) >= ?"
		countArgs = append(countArgs, startDate)
	}
	if endDate != "" {
		countSQL += " AND DATE(u.created_at) <= ?"
		countArgs = append(countArgs, endDate)
	}

	totalVal, err := d.db.GetValue(ctx, countSQL, countArgs...)
	if err != nil {
		return nil, 0, err
	}
	total := totalVal.Int()

	// 分页查询，按 personal + team(group_performance最新快照) + node(cobo_performance) + triple(staking_v2_performance) 总和降序
	dataSQL := `
		SELECT u.*
		FROM user_info u
		LEFT JOIN (
			SELECT user_id,
				COALESCE(SUM(personal_group_amount), 0) AS personal_group_amount,
				COALESCE(SUM(team_group_amount), 0) AS team_group_amount
			FROM group_performance
			GROUP BY user_id
		) gp ON gp.user_id = u.id
		LEFT JOIN cobo_performance cp ON cp.user_id = u.id
		LEFT JOIN staking_v2_performance svp ON svp.user_id = u.id
		WHERE u.parent_invite_code = ?
	`
	dataArgs := []interface{}{parentUser.InviteCode}
	if startDate != "" {
		dataSQL += " AND DATE(u.created_at) >= ?"
		dataArgs = append(dataArgs, startDate)
	}
	if endDate != "" {
		dataSQL += " AND DATE(u.created_at) <= ?"
		dataArgs = append(dataArgs, endDate)
	}
	dataSQL += `
		ORDER BY COALESCE(gp.personal_group_amount, 0) + COALESCE(gp.team_group_amount, 0) + COALESCE(cp.team_performance, 0) + COALESCE(svp.team_performance, 0) DESC
		LIMIT ? OFFSET ?
	`
	dataArgs = append(dataArgs, pageSize, (page-1)*pageSize)

	var users []*entity.UserEntity
	err = d.db.Ctx(ctx).Raw(dataSQL, dataArgs...).Scan(&users)
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetDirectCountByUserID 获取直推人数（通过用户ID查询）
func (d *userDao) GetDirectCountByUserID(ctx context.Context, userID int64) (int, error) {
	if userID <= 0 {
		return 0, nil
	}
	// 查询用户邀请码
	var parent struct {
		InviteCode string `json:"invite_code"`
	}
	if err := d.db.Model("user_info").Ctx(ctx).
		Fields("invite_code").
		Where("id", userID).
		Scan(&parent); err != nil {
		return 0, err
	}
	if parent.InviteCode == "" {
		return 0, nil
	}
	// 统计直推人数
	count, err := d.db.Model("user_info").Ctx(ctx).
		Where("parent_invite_code", parent.InviteCode).
		Count()
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Get30LayerReferralsByWalletAddress 获取最多30层推荐关系（根据直推数量动态控制层级）
func (d *userDao) Get30LayerReferralsByWalletAddress(ctx context.Context, userID int64) (map[int][]int64, error) {
	layerMap := make(map[int][]int64)

	// 查询用户邀请码
	var parent struct {
		InviteCode string `json:"invite_code"`
	}
	if err := d.db.Model("user_info").Ctx(ctx).
		Fields("invite_code").
		Where("id", userID).
		Scan(&parent); err != nil {
		return nil, err
	}
	if parent.InviteCode == "" {
		return layerMap, nil
	}

	// 查询第一层直推用户
	var directUsers []struct {
		ID int64 `json:"id"`
	}
	if err := d.db.Model("user_info").Ctx(ctx).
		Fields("id").
		Where("parent_invite_code", parent.InviteCode).
		Where("status", 1).
		OrderAsc("id").
		Scan(&directUsers); err != nil {
		return nil, err
	}

	if len(directUsers) == 0 {
		return layerMap, nil
	}

	levelOne := make([]int64, 0, len(directUsers))
	for _, user := range directUsers {
		levelOne = append(levelOne, user.ID)
	}
	layerMap[1] = levelOne

	// 只有一个直推用户时，只返回第一层
	if len(directUsers) == 1 {
		return layerMap, nil
	}

	// 根据直推人数动态控制最大层级（不超过30层）
	maxDepth := len(directUsers)
	if maxDepth > 30 {
		maxDepth = 30
	}

	sql := `
	WITH RECURSIVE referral_tree AS (
		SELECT 
			u.id,
			u.invite_code,
			u.parent_invite_code,
			1 AS level
		FROM user_info u
		WHERE u.parent_invite_code = ?
		  AND u.status = 1
		
		UNION ALL
		
		SELECT 
			u.id,
			u.invite_code,
			u.parent_invite_code,
			rt.level + 1
		FROM user_info u
		INNER JOIN referral_tree rt ON u.parent_invite_code = rt.invite_code
		WHERE rt.level < ?
		  AND u.status = 1
	)
	SELECT level, id FROM referral_tree ORDER BY level, id;
	`

	var results []struct {
		Level int   `json:"level"`
		ID    int64 `json:"id"`
	}
	if err := d.db.Ctx(ctx).Raw(sql, parent.InviteCode, maxDepth).Scan(&results); err != nil {
		return nil, err
	}

	layerMap = make(map[int][]int64)
	for _, result := range results {
		layerMap[result.Level] = append(layerMap[result.Level], result.ID)
	}

	return layerMap, nil
}

// GetActiveUsers 获取所有激活用户
func (d *userDao) GetActiveUsers(ctx context.Context) ([]*entity.UserEntity, error) {
	var users []*entity.UserEntity
	err := d.db.Model("user_info").Ctx(ctx).
		Where("status", 1).
		Scan(&users)
	if err != nil {
		return nil, err
	}
	return users, nil
}

// GetTeamTotalCount 获取团队总人数
func (d *userDao) GetTeamTotalCount(ctx context.Context, userID int64) (int, error) {
	// 使用递归CTE查询团队总人数
	sql := `
	WITH RECURSIVE team_tree AS (
		-- 第1层：直推下级
		SELECT 
			u.id,
			u.invite_code,
			u.parent_invite_code,
			1 as level
		FROM user_info u
		WHERE u.parent_invite_code = (
			SELECT invite_code FROM user_info WHERE id = $1
		)
		AND u.status = 1
		
		UNION ALL
		
		-- 第2-10层：递归查询（限制深度避免性能问题）
		SELECT 
			u.id,
			u.invite_code,
			u.parent_invite_code,
			tt.level + 1
		FROM user_info u
		INNER JOIN team_tree tt ON u.parent_invite_code = tt.invite_code
		WHERE u.status = 1
			AND tt.level < 10
	)
	SELECT COUNT(*) as count FROM team_tree;
	`

	var results []struct {
		Count int `json:"count"`
	}
	err := d.db.Ctx(ctx).Raw(sql, userID).Scan(&results)
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		return 0, nil
	}
	return results[0].Count, nil
}

// GetDirectReferralCount 获取直推总人数
func (d *userDao) GetDirectReferralCount(ctx context.Context, userID int64) (int, error) {
	var parent struct {
		InviteCode string `json:"invite_code"`
	}

	if err := d.db.Model("user_info").Ctx(ctx).
		Fields("invite_code").
		Where("id", userID).
		Scan(&parent); err != nil {
		return 0, err
	}

	if parent.InviteCode == "" {
		return 0, nil
	}

	return d.db.Model("user_info").Ctx(ctx).
		Where("parent_invite_code", parent.InviteCode).
		Where("status", 1).
		Count()
}

// GetSubUserPerformance 获取伞下用户业绩
func (d *userDao) GetSubUserPerformance(ctx context.Context, inviteCode string) (string, error) {
	// 使用递归CTE查询伞下用户业绩
	sql := `
		WITH RECURSIVE user_hierarchy AS (
			SELECT 
				id,
				invite_code,
				parent_invite_code,
				1 as level
			FROM user_info 
			WHERE parent_invite_code = ?
			UNION ALL
			SELECT 
				ui.id,
				ui.invite_code,
				ui.parent_invite_code,
				uh.level + 1 as level
			FROM user_info ui
			INNER JOIN user_hierarchy uh ON ui.parent_invite_code = uh.invite_code
		)
		SELECT 
			COALESCE(FLOOR(SUM(CAST(npr.amount AS NUMERIC))), 0) AS total_amount
		FROM cobo_node_purchase npr
		INNER JOIN user_hierarchy uh ON npr.user_id = uh.id
		WHERE npr.is_gift = 0
	`

	var totalAmount string
	value, err := d.db.GetValue(ctx, sql, inviteCode)
	if err != nil {
		return "0", err
	}

	if value.IsNil() {
		return "0", nil
	}

	totalAmount = value.String()
	return totalAmount, nil
}

// GetByInviteCode 根据邀请码获取用户
func (d *userDao) GetByInviteCode(ctx context.Context, inviteCode string) (*entity.UserEntity, error) {
	var user entity.UserEntity
	err := d.db.Model("user_info").Where("invite_code", inviteCode).Scan(&user)
	if err != nil {
		return nil, err
	}
	if user.Id == 0 {
		return nil, nil
	}
	return &user, nil
}

// GetDirectDescendants 获取直接下级用户列表（兼容 parent_invite_code / parent_wallet_address 两种关系字段）
func (d *userDao) GetDirectDescendants(ctx context.Context, parentInviteCode, parentWalletAddress string) ([]*entity.UserEntity, error) {
	var users []*entity.UserEntity
	parentWalletAddress = strings.ToLower(strings.TrimSpace(parentWalletAddress))

	model := d.db.Model("user_info").Ctx(ctx)
	if parentInviteCode != "" && parentWalletAddress != "" {
		model = model.Where("parent_invite_code = ? OR LOWER(parent_wallet_address) = ?", parentInviteCode, parentWalletAddress)
	} else if parentInviteCode != "" {
		model = model.Where("parent_invite_code", parentInviteCode)
	} else if parentWalletAddress != "" {
		model = model.Where("LOWER(parent_wallet_address) = ?", parentWalletAddress)
	} else {
		return []*entity.UserEntity{}, nil
	}

	if err := model.Scan(&users); err != nil {
		return nil, err
	}
	return users, nil
}

// GetVertexUser 获取顶点账号（is_vertex为true的用户）
func (d *userDao) GetVertexUser(ctx context.Context) (*entity.UserEntity, error) {
	var user entity.UserEntity
	err := d.db.Model("user_info").Ctx(ctx).
		Where("is_vertex", true).
		Limit(1).
		Scan(&user)
	if err != nil {
		// 兼容：Scan 可能返回 "sql: no rows in result set"
		if strings.Contains(err.Error(), "no rows in result set") {
			return nil, nil
		}
		return nil, err
	}
	if user.Id == 0 {
		return nil, nil
	}
	return &user, nil
}

// GetServiceCenterUsers 获取服务中心用户列表
func (d *userDao) GetServiceCenterUsers(ctx context.Context) ([]*entity.UserEntity, error) {
	var users []*entity.UserEntity
	err := d.db.Model("user_info").Ctx(ctx).
		Where("is_service_center", true).
		Fields("id", "wallet_address", "service_center_rate").
		Order("id ASC").
		Scan(&users)
	if err != nil {
		return nil, err
	}
	return users, nil
}

// GetAllDescendantIDs 获取用户所有下级用户ID（不包含当前用户）
func (d *userDao) GetAllDescendantIDs(ctx context.Context, userID int64) ([]int64, error) {
	if userID <= 0 {
		return []int64{}, nil
	}

	// 查询用户邀请码
	var parent struct {
		InviteCode string `json:"invite_code"`
	}
	if err := d.db.Model("user_info").Ctx(ctx).
		Fields("invite_code").
		Where("id", userID).
		Scan(&parent); err != nil {
		return nil, err
	}
	if parent.InviteCode == "" {
		return []int64{}, nil
	}

	sql := `
WITH RECURSIVE user_tree AS (
	SELECT 
		u.id,
		u.invite_code
	FROM user_info u
	WHERE u.parent_invite_code = ?

	UNION ALL

	SELECT 
		ui.id,
		ui.invite_code
	FROM user_info ui
	INNER JOIN user_tree ut ON ui.parent_invite_code = ut.invite_code
)
SELECT id FROM user_tree;
`

	var results []struct {
		ID int64 `json:"id"`
	}
	if err := d.db.Ctx(ctx).Raw(sql, parent.InviteCode).Scan(&results); err != nil {
		return nil, err
	}

	descendantIDs := make([]int64, 0, len(results))
	for _, result := range results {
		if result.ID == 0 {
			continue
		}
		descendantIDs = append(descendantIDs, result.ID)
	}

	return descendantIDs, nil
}

// GetUsersPurchaseAmount 批量查询用户购买总金额
func (d *userDao) GetUsersPurchaseAmount(ctx context.Context, userIds []int64) (map[int64]int64, error) {
	result := make(map[int64]int64)

	// 如果用户ID列表为空，直接返回空映射
	if len(userIds) == 0 {
		return result, nil
	}

	// 使用 GoFrame Model 方法查询用户购买总金额
	type PurchaseAmount struct {
		UserId      int64 `json:"user_id"`
		TotalAmount int64 `json:"total_amount"`
	}

	var amounts []PurchaseAmount
	err := d.db.Model("cobo_node_purchase").
		Fields("user_id, COALESCE(FLOOR(SUM(CAST(amount AS NUMERIC))), 0) as total_amount").
		WhereIn("user_id", userIds).
		Where("is_gift", 0).
		Group("user_id").
		Scan(&amounts)

	if err != nil {
		return nil, err
	}

	// 将结果转换为map
	for _, item := range amounts {
		result[item.UserId] = item.TotalAmount
	}

	return result, nil
}

// GetDataByIds 根据ID列表获取用户列表
func (d *userDao) GetDataByIds(ctx context.Context, ids []int64) ([]*entity.UserEntity, error) {
	if len(ids) == 0 {
		return []*entity.UserEntity{}, nil
	}

	const batchSize = 30000
	var (
		users  []*entity.UserEntity
		result = make([]*entity.UserEntity, 0, len(ids))
	)

	for start := 0; start < len(ids); start += batchSize {
		end := start + batchSize
		if end > len(ids) {
			end = len(ids)
		}

		users = users[:0]
		if err := d.db.Model("user_info").Ctx(ctx).WhereIn("id", ids[start:end]).Scan(&users); err != nil {
			return nil, err
		}
		if len(users) > 0 {
			result = append(result, users...)
		}
	}

	return result, nil
}

// GetUserPurchaseAmount 查询单个用户购买总金额
func (d *userDao) GetUserPurchaseAmount(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, nil
	}

	var amount struct {
		TotalAmount int64 `json:"total_amount"`
	}

	err := d.db.Model("cobo_node_purchase").
		Fields("COALESCE(FLOOR(SUM(CAST(amount AS NUMERIC))), 0) as total_amount").
		Where("user_id", userID).
		Where("is_gift", 0).
		Scan(&amount)
	if err != nil {
		return 0, err
	}

	return amount.TotalAmount, nil
}

// UpdateTeamCanWithdraw 更新团队成员的can_withdraw字段（无限代）
func (d *userDao) UpdateTeamCanWithdraw(ctx context.Context, userID int64, withdrawType string) error {
	if userID <= 0 {
		return nil
	}

	// 查询用户邀请码
	var parent struct {
		InviteCode string `json:"invite_code"`
	}
	//查新当前用户是否可以提现
	var user entity.UserEntity
	if err := d.db.Model("user_info").Ctx(ctx).
		Where("id", userID).
		Scan(&user); err != nil {
		return err
	}
	if user.WalletAddress == "" {
		return nil
	}
	var canWithdraw = !user.CanWithdraw
	if withdrawType == "personal" {
		//禁止当前用户的个人提现
		if _, err := d.db.Model("user_info").Ctx(ctx).
			Where("id", userID).
			Data(map[string]interface{}{
				"can_withdraw": canWithdraw,
			}).
			Update(); err != nil {
			return err
		}
		return nil
	}
	parent.InviteCode = user.InviteCode
	if parent.InviteCode == "" {
		return nil
	}

	// 使用递归CTE查询所有团队成员ID，然后批量更新（包括用户自己）
	sql := `
	WITH RECURSIVE user_tree AS (
		SELECT 
			u.id,
			u.invite_code
		FROM user_info u
		WHERE u.parent_invite_code = $1
		
		UNION ALL
		
		SELECT 
			ui.id,
			ui.invite_code
		FROM user_info ui
		INNER JOIN user_tree ut ON ui.parent_invite_code = ut.invite_code
	)
	UPDATE user_info 
	SET can_withdraw = $2, updated_at = $3
	WHERE id IN (SELECT id FROM user_tree) OR id = $4;
	`

	_, err := d.db.Exec(ctx, sql, parent.InviteCode, canWithdraw, time.Now(), userID)
	return err
}

// UpdateUserCanWithdraw 直接更新单个用户的can_withdraw字段
func (d *userDao) UpdateUserCanWithdraw(ctx context.Context, userID int64, canWithdraw bool) error {
	if userID <= 0 {
		return nil
	}
	_, err := d.db.Model("user_info").Ctx(ctx).
		Where("id", userID).
		Data(map[string]interface{}{
			"can_withdraw": canWithdraw,
		}).
		Update()
	return err
}

// UpdateTeamStakeRate 更新团队成员的质押收益率
func (d *userDao) UpdateTeamStakeRate(ctx context.Context, userID int64, stakeRate float64) error {
	if userID <= 0 {
		return nil
	}
	//需要更新当前用户和团队的质押收益率，表user_info和staking_package.daily_yield_rate
	// 查询用户邀请码
	var user entity.UserEntity
	if err := d.db.Model("user_info").Ctx(ctx).
		Where("id", userID).
		Scan(&user); err != nil {
		return err
	}
	if user.WalletAddress == "" {
		return nil
	}
	if user.InviteCode == "" {
		return nil
	}

	// 使用递归CTE查询所有团队成员ID，然后批量更新（包括用户自己）
	// 只更新入金时间（start_time）在2025-12-28 00:00:00之前的订单的释放比例
	// 使用参数化查询处理时间，避免时区格式问题
	loc, _ := time.LoadLocation("Asia/Shanghai")
	cutoffTime, _ := time.ParseInLocation("2006-01-02 15:04:05", "2025-12-28 00:00:00", loc)

	sql := `
	WITH RECURSIVE user_tree AS (
		SELECT 
			u.id,
			u.invite_code
		FROM user_info u
		WHERE u.parent_invite_code = $1
	
		UNION ALL
		
		SELECT 
			ui.id,
			ui.invite_code
		FROM user_info ui
		INNER JOIN user_tree ut ON ui.parent_invite_code = ut.invite_code
	)
	UPDATE staking_package 
	SET daily_yield_rate = $2, updated_at = $3
	WHERE (user_id IN (SELECT id FROM user_tree) OR user_id = $4)
		AND start_time < $5;
	`
	_, err := d.db.Exec(ctx, sql, user.InviteCode, stakeRate, time.Now(), userID, cutoffTime)
	return err
}

// UpdateUserNodeExempt 更新用户赠送节点业绩豁免状态
func (d *userDao) UpdateUserNodeExempt(ctx context.Context, userID int64, nodeExempt bool) error {
	if userID <= 0 {
		return nil
	}
	_, err := d.db.Model("user_info").Ctx(ctx).
		Where("id", userID).
		Data(map[string]interface{}{
			"node_exempt": nodeExempt,
			"updated_at":  time.Now(),
		}).
		Update()
	return err
}

// UpdateTeamTeamId 批量更新团队成员的 team_id（无限代，包括用户自己）
func (d *userDao) UpdateTeamTeamId(ctx context.Context, userID int64, teamId *int64) error {
	if userID <= 0 {
		return nil
	}

	var user entity.UserEntity
	if err := d.db.Model("user_info").Ctx(ctx).
		Where("id", userID).
		Scan(&user); err != nil {
		return err
	}
	if user.InviteCode == "" {
		return nil
	}

	sql := `
	WITH RECURSIVE user_tree AS (
		SELECT id, invite_code
		FROM user_info
		WHERE parent_invite_code = $1

		UNION ALL

		SELECT ui.id, ui.invite_code
		FROM user_info ui
		INNER JOIN user_tree ut ON ui.parent_invite_code = ut.invite_code
	)
	UPDATE user_info
	SET team_id = $2, updated_at = $3
	WHERE id IN (SELECT id FROM user_tree) OR id = $4;
	`
	_, err := d.db.Exec(ctx, sql, user.InviteCode, teamId, time.Now(), userID)
	return err
}

// UpdateDescendantsTeamIdExcludingLeaderSubtrees 更新用户下级（不含用户自己）的 team_id，排除团队长及其子树
func (d *userDao) UpdateDescendantsTeamIdExcludingLeaderSubtrees(ctx context.Context, userID int64, teamId *int64) error {
	if userID <= 0 {
		return nil
	}

	var user entity.UserEntity
	if err := d.db.Model("user_info").Ctx(ctx).
		Where("id", userID).
		Scan(&user); err != nil {
		return err
	}
	if user.InviteCode == "" {
		return nil
	}

	sql := `
	WITH RECURSIVE descendants AS (
		SELECT id, invite_code, wallet_address
		FROM user_info
		WHERE parent_invite_code = $1

		UNION ALL

		SELECT ui.id, ui.invite_code, ui.wallet_address
		FROM user_info ui
		INNER JOIN descendants d ON ui.parent_invite_code = d.invite_code
	),
	leaders AS (
		SELECT d.id, d.invite_code
		FROM descendants d
		JOIN team t ON t.leader_wallet_address = d.wallet_address
	),
	excluded AS (
		SELECT id, invite_code FROM leaders

		UNION ALL

		SELECT ui.id, ui.invite_code
		FROM user_info ui
		INNER JOIN excluded e ON ui.parent_invite_code = e.invite_code
	)
	UPDATE user_info
	SET team_id = $2, updated_at = $3
	WHERE id IN (SELECT id FROM descendants EXCEPT SELECT id FROM excluded);
	`
	_, err := d.db.Exec(ctx, sql, user.InviteCode, teamId, time.Now())
	return err
}

// GetAncestorsByUserIDs 批量获取用户的所有祖先（包含当前用户及其父、祖等，顺序由近到远）
func (d *userDao) GetAncestorsByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]int64, error) {
	if len(userIDs) == 0 {
		return make(map[int64][]int64), nil
	}

	// 分批处理，避免 PostgreSQL 参数上限
	const batchSize = 1000
	ancestorMap := make(map[int64][]int64)

	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		batch := userIDs[i:end]

		// 构建IN子句的占位符
		placeholders := make([]string, len(batch))
		args := make([]interface{}, len(batch))
		for j, id := range batch {
			placeholders[j] = "?"
			args[j] = id
		}
		inClause := "(" + strings.Join(placeholders, ",") + ")"

		sql := `
		WITH RECURSIVE ancestor_tree AS (
			-- 第1层：初始用户
			SELECT 
				u.id as original_user_id,
				u.id,
				u.parent_invite_code,
				1 as depth,
				ARRAY[u.id]::bigint[] as path
			FROM user_info u
			WHERE u.id IN ` + inClause + `
			
			UNION ALL
			
			-- 递归查询上级（添加深度限制和路径追踪，防止循环）
			SELECT 
				at.original_user_id,
				u.id,
				u.parent_invite_code,
				at.depth + 1,
				at.path || u.id
			FROM user_info u
			INNER JOIN ancestor_tree at ON u.invite_code = at.parent_invite_code
			WHERE at.parent_invite_code IS NOT NULL 
			  AND at.parent_invite_code != ''
			  AND at.depth < 60
			  AND NOT (u.id = ANY(at.path))
		)
		SELECT original_user_id, id, depth FROM ancestor_tree ORDER BY original_user_id, depth;
		`

		var batchResults []struct {
			OriginalUserID int64 `json:"original_user_id"`
			ID             int64 `json:"id"`
			Depth          int   `json:"depth"`
		}
		if err := d.db.Ctx(ctx).Raw(sql, args...).Scan(&batchResults); err != nil {
			return nil, err
		}

		for _, res := range batchResults {
			ancestorMap[res.OriginalUserID] = append(ancestorMap[res.OriginalUserID], res.ID)
		}
	}

	return ancestorMap, nil
}

// GetUserIDsByTeamId 根据团队ID获取用户ID列表
func (d *userDao) GetUserIDsByTeamId(ctx context.Context, teamId int64) ([]int64, error) {
	var userIDs []int64
	err := d.db.Model("user_info").Ctx(ctx).
		Fields("id").
		Where("team_id = ?", teamId).
		Scan(&userIDs)
	if err != nil {
		return nil, err
	}
	return userIDs, nil
}
