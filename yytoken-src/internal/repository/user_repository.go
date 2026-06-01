package repository

import (
	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/model"
	"context"
	"time"
)

// IUserRepository 用户仓储接口（业务逻辑层）
type IUserRepository interface {
	// 用户管理
	CreateUser(ctx context.Context, user *entity.UserEntity) error
	GetUserById(ctx context.Context, id int64) (*entity.UserEntity, error)
	GetUserByWalletAddress(ctx context.Context, walletAddress string) (*entity.UserEntity, error)
	UpdateUser(ctx context.Context, id int64, data map[string]interface{}) error
	UpdateLastLoginAt(ctx context.Context, id int64) error
	DeleteUser(ctx context.Context, id int64) error

	// 用户查询
	CheckWalletAddressExists(ctx context.Context, walletAddress string) (bool, error)
	CheckInviteCodeExists(ctx context.Context, inviteCode string) (bool, error)
	GetUserList(ctx context.Context, req *GetUserListReq) (*GetUserListRes, error)
	GetReferralUsers(ctx context.Context, parentUserId int64) ([]*entity.UserEntity, error)

	// 用户业绩查询
	GetSubUserPerformance(ctx context.Context, inviteCode string) (string, error)

	// 用户关系查询
	GetUserByInviteCode(ctx context.Context, inviteCode string) (*entity.UserEntity, error)
	// GetDirectDescendants 获取直接下级用户列表（兼容 parent_invite_code / parent_wallet_address 两种关系字段）
	GetDirectDescendants(ctx context.Context, parentInviteCode, parentWalletAddress string) ([]*entity.UserEntity, error)

	// 顶点账号查询
	GetVertexUser(ctx context.Context) (*entity.UserEntity, error)

	// GetDataByIds 根据ID列表获取用户列表
	GetDataByIds(ctx context.Context, ids []int64) ([]*entity.UserEntity, error)

	// GetServiceCenterUsers 获取服务中心用户列表
	GetServiceCenterUsers(ctx context.Context) ([]*entity.UserEntity, error)

	// UpdateParentWalletAddressByOldWalletAddress 根据旧的父钱包地址批量更新父钱包地址
	UpdateParentWalletAddressByOldWalletAddress(ctx context.Context, oldWalletAddress, newWalletAddress string) error

	// GetAllDescendantIDs 获取用户所有下级用户ID（不包含当前用户）
	GetAllDescendantIDs(ctx context.Context, userID int64) ([]int64, error)

	// GetUserPurchaseAmount 查询单个用户购买总金额
	GetUserPurchaseAmount(ctx context.Context, userID int64) (int64, error)

	// UpdateTeamCanWithdraw 更新团队成员的提现权限
	UpdateTeamCanWithdraw(ctx context.Context, userID int64, withdrawType string) error

	// UpdateUserCanWithdraw 直接更新单个用户的提现权限
	UpdateUserCanWithdraw(ctx context.Context, userID int64, canWithdraw bool) error

	// UpdateTeamStakeRate 更新团队成员的质押收益率
	UpdateTeamStakeRate(ctx context.Context, userID int64, stakeRate float64) error

	// UpdateUserNodeExempt 更新用户赠送节点业绩豁免状态
	UpdateUserNodeExempt(ctx context.Context, userID int64, nodeExempt bool) error

	// UpdateTeamTeamId 批量更新团队成员的 team_id（无限代，包括用户自己）
	UpdateTeamTeamId(ctx context.Context, userID int64, teamId *int64) error

	// UpdateDescendantsTeamIdExcludingLeaderSubtrees 更新用户下级（不含用户自己）的 team_id，排除团队长及其子树
	UpdateDescendantsTeamIdExcludingLeaderSubtrees(ctx context.Context, userID int64, teamId *int64) error

	// GetUsersByParentAddress 根据上级钱包地址获取用户列表
	GetUsersByParentAddress(ctx context.Context, parentAddress string) ([]*entity.UserEntity, error)

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
	List                []*entity.UserEntity `json:"list"`
	UserPurchaseAmounts map[int64]int64      `json:"user_purchase_amounts"` // 用户购买总金额映射
	Summary             UserListSummary      `json:"summary"`
}

// userRepository 用户仓储实现
type userRepository struct {
	userDao dao.IUserDao
}

// NewUserRepository 创建用户仓储实例
func NewUserRepository() IUserRepository {
	return &userRepository{
		userDao: dao.NewUserDao(),
	}
}

// CreateUser 创建用户
func (r *userRepository) CreateUser(ctx context.Context, user *entity.UserEntity) error {
	return r.userDao.Create(ctx, user)
}

// GetUserById 根据ID获取用户
func (r *userRepository) GetUserById(ctx context.Context, id int64) (*entity.UserEntity, error) {
	return r.userDao.GetById(ctx, id)
}

// GetUserByWalletAddress 根据钱包地址获取用户
func (r *userRepository) GetUserByWalletAddress(ctx context.Context, walletAddress string) (*entity.UserEntity, error) {
	return r.userDao.GetByWalletAddress(ctx, walletAddress)
}

// UpdateUser 更新用户
func (r *userRepository) UpdateUser(ctx context.Context, id int64, data map[string]interface{}) error {
	return r.userDao.UpdateById(ctx, id, data)
}

// UpdateLastLoginAt 更新最后登录时间
func (r *userRepository) UpdateLastLoginAt(ctx context.Context, id int64) error {
	now := time.Now()
	return r.userDao.UpdateById(ctx, id, map[string]interface{}{
		"last_login_at": now,
		"updated_at":    now,
	})
}

// DeleteUser 删除用户
func (r *userRepository) DeleteUser(ctx context.Context, id int64) error {
	return r.userDao.DeleteById(ctx, id)
}

// CheckWalletAddressExists 检查钱包地址是否存在
func (r *userRepository) CheckWalletAddressExists(ctx context.Context, walletAddress string) (bool, error) {
	return r.userDao.ExistsByWalletAddress(ctx, walletAddress)
}

// CheckInviteCodeExists 检查邀请码是否存在
func (r *userRepository) CheckInviteCodeExists(ctx context.Context, inviteCode string) (bool, error) {
	return r.userDao.ExistsByInviteCode(ctx, inviteCode)
}

// GetUserList 获取用户列表
func (r *userRepository) GetUserList(ctx context.Context, req *GetUserListReq) (*GetUserListRes, error) {
	daoReq := &dao.GetUserListReq{
		PageReq:          req.PageReq,
		UserID:           req.UserID,
		WalletAddress:    req.WalletAddress,
		DepositAddress:   req.DepositAddress,
		ParentUserID:     req.ParentUserID,
		ParentInviteCode: req.ParentInviteCode,
		ParentAddress:    req.ParentAddress,
		LeaderLevel:      req.LeaderLevel,
		AssetSymbol:      req.AssetSymbol,
		CanWithdraw:      req.CanWithdraw,
		NodeExempt:       req.NodeExempt,
		AdjustedOnly:     req.AdjustedOnly,
		CreatedAtStart:   req.CreatedAtStart,
		CreatedAtEnd:     req.CreatedAtEnd,
		Status:           req.Status,
	}

	daoRes, err := r.userDao.GetList(ctx, daoReq)
	if err != nil {
		return nil, err
	}

	// 提取所有用户ID
	var userIds []int64
	for _, user := range daoRes.List {
		userIds = append(userIds, user.Id)
	}

	// 批量查询用户购买总金额
	userPurchaseAmounts := make(map[int64]int64)
	if len(userIds) > 0 {
		userPurchaseAmounts, err = r.userDao.GetUsersPurchaseAmount(ctx, userIds)
		if err != nil {
			return nil, err
		}
	}

	return &GetUserListRes{
		PageRes:             daoRes.PageRes,
		List:                daoRes.List,
		UserPurchaseAmounts: userPurchaseAmounts,
		Summary: UserListSummary{
			TotalRewardUSDT:     daoRes.Summary.TotalRewardUSDT,
			WithdrawnRewardUSDT: daoRes.Summary.WithdrawnRewardUSDT,
			RemainingRewardUSDT: daoRes.Summary.RemainingRewardUSDT,
			StakeTotal:          daoRes.Summary.StakeTotal,
			NodePurchaseTotal:   daoRes.Summary.NodePurchaseTotal,
			GroupRechargeUSDT:   daoRes.Summary.GroupRechargeUSDT,
			TicketCount:         daoRes.Summary.TicketCount,
			GiftTriple:          daoRes.Summary.GiftTriple,
			SZPN:                daoRes.Summary.SZPN,
			JU:                  daoRes.Summary.JU,
			YY:                  daoRes.Summary.YY,
			YYAI:                daoRes.Summary.YYAI,
		},
	}, nil
}

// GetReferralUsers 获取推荐用户列表
func (r *userRepository) GetReferralUsers(ctx context.Context, parentUserId int64) ([]*entity.UserEntity, error) {
	// 先获取父用户的邀请码
	parentUser, err := r.userDao.GetById(ctx, parentUserId)
	if err != nil {
		return nil, err
	}
	if parentUser == nil {
		return []*entity.UserEntity{}, nil
	}

	// 查询所有使用该邀请码作为邀请人的用户
	req := &dao.GetUserListReq{
		PageReq: model.PageReq{
			Page:     1,
			PageSize: 1000, // 设置一个较大的值，实际项目中可能需要分页
		},

		ParentInviteCode: parentUser.InviteCode,
	}

	res, err := r.userDao.GetList(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.List, nil
}

// GetSubUserPerformance 获取伞下用户业绩
func (r *userRepository) GetSubUserPerformance(ctx context.Context, inviteCode string) (string, error) {
	return r.userDao.GetSubUserPerformance(ctx, inviteCode)
}

// GetUserByInviteCode 根据邀请码获取用户
func (r *userRepository) GetUserByInviteCode(ctx context.Context, inviteCode string) (*entity.UserEntity, error) {
	return r.userDao.GetByInviteCode(ctx, inviteCode)
}

// GetDirectDescendants 获取直接下级用户列表（兼容 parent_invite_code / parent_wallet_address 两种关系字段）
func (r *userRepository) GetDirectDescendants(ctx context.Context, parentInviteCode, parentWalletAddress string) ([]*entity.UserEntity, error) {
	return r.userDao.GetDirectDescendants(ctx, parentInviteCode, parentWalletAddress)
}

// GetVertexUser 获取顶点账号（parent_wallet_address为空的用户）
func (r *userRepository) GetVertexUser(ctx context.Context) (*entity.UserEntity, error) {
	return r.userDao.GetVertexUser(ctx)
}

// GetDataByIds 根据ID列表获取用户列表
func (r *userRepository) GetDataByIds(ctx context.Context, ids []int64) ([]*entity.UserEntity, error) {
	return r.userDao.GetDataByIds(ctx, ids)
}

// GetServiceCenterUsers 获取服务中心用户列表
func (r *userRepository) GetServiceCenterUsers(ctx context.Context) ([]*entity.UserEntity, error) {
	return r.userDao.GetServiceCenterUsers(ctx)
}

// UpdateParentWalletAddressByOldWalletAddress 根据旧的父钱包地址批量更新父钱包地址
func (r *userRepository) UpdateParentWalletAddressByOldWalletAddress(ctx context.Context, oldWalletAddress, newWalletAddress string) error {
	return r.userDao.UpdateParentWalletAddressByOldWalletAddress(ctx, oldWalletAddress, newWalletAddress)
}

// GetAllDescendantIDs 获取用户所有下级用户ID（不包含当前用户）
func (r *userRepository) GetAllDescendantIDs(ctx context.Context, userID int64) ([]int64, error) {
	return r.userDao.GetAllDescendantIDs(ctx, userID)
}

// GetUserPurchaseAmount 查询单个用户购买总金额
func (r *userRepository) GetUserPurchaseAmount(ctx context.Context, userID int64) (int64, error) {
	return r.userDao.GetUserPurchaseAmount(ctx, userID)
}

// UpdateTeamCanWithdraw 更新团队成员的提现权限
func (r *userRepository) UpdateTeamCanWithdraw(ctx context.Context, userID int64, withdrawType string) error {
	return r.userDao.UpdateTeamCanWithdraw(ctx, userID, withdrawType)
}

// UpdateUserCanWithdraw 直接更新单个用户的提现权限
func (r *userRepository) UpdateUserCanWithdraw(ctx context.Context, userID int64, canWithdraw bool) error {
	return r.userDao.UpdateUserCanWithdraw(ctx, userID, canWithdraw)
}

// UpdateTeamStakeRate 更新团队成员的质押收益率
func (r *userRepository) UpdateTeamStakeRate(ctx context.Context, userID int64, stakeRate float64) error {
	return r.userDao.UpdateTeamStakeRate(ctx, userID, stakeRate)
}

// UpdateUserNodeExempt 更新用户赠送节点业绩豁免状态
func (r *userRepository) UpdateUserNodeExempt(ctx context.Context, userID int64, nodeExempt bool) error {
	return r.userDao.UpdateUserNodeExempt(ctx, userID, nodeExempt)
}

// UpdateTeamTeamId 批量更新团队成员的 team_id（无限代，包括用户自己）
func (r *userRepository) UpdateTeamTeamId(ctx context.Context, userID int64, teamId *int64) error {
	return r.userDao.UpdateTeamTeamId(ctx, userID, teamId)
}

// UpdateDescendantsTeamIdExcludingLeaderSubtrees 更新用户下级（不含用户自己）的 team_id，排除团队长及其子树
func (r *userRepository) UpdateDescendantsTeamIdExcludingLeaderSubtrees(ctx context.Context, userID int64, teamId *int64) error {
	return r.userDao.UpdateDescendantsTeamIdExcludingLeaderSubtrees(ctx, userID, teamId)
}

// GetUsersByParentAddress 根据上级钱包地址获取用户列表
func (r *userRepository) GetUsersByParentAddress(ctx context.Context, parentAddress string) ([]*entity.UserEntity, error) {
	res, err := r.GetUserList(ctx, &GetUserListReq{
		PageReq:       model.PageReq{Page: 1, PageSize: 100000},
		ParentAddress: parentAddress,
	})
	if err != nil {
		return nil, err
	}
	return res.List, nil
}

// GetUserIDsByTeamId 根据团队ID获取用户ID列表
func (r *userRepository) GetUserIDsByTeamId(ctx context.Context, teamId int64) ([]int64, error) {
	return r.userDao.GetUserIDsByTeamId(ctx, teamId)
}
