package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"XWFrame/internal/dao"
	apgDao "XWFrame/internal/dao/apg"
	teamDao "XWFrame/internal/dao/team"
	vnsDao "XWFrame/internal/dao/vns"
	"XWFrame/internal/entity"
	coboEntity "XWFrame/internal/entity/cobo"
	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/cache"
	"XWFrame/internal/frame/config"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/frame/model"
	"XWFrame/internal/repository"
	rewardRepo "XWFrame/internal/repository/reward"
	balanceModel "XWFrame/internal/service/balance/model"
	coboService "XWFrame/internal/service/cobo"
	coboModel "XWFrame/internal/service/cobo/model"
	vipAdjustmentSvc "XWFrame/internal/service/reward/vip_adjustment"
	"XWFrame/internal/service/teamstats"
	"XWFrame/pkg/external/cobo"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/shopspring/decimal"
)

var cnLocation, _ = time.LoadLocation("Asia/Shanghai")

var coboSupportedSymbols = map[string]struct{}{
	"USDT": {},
	"JU":   {},
	"ZPN":  {},
	"YY":   {},
}

const defaultUserPassword = "123456"

func isSixDigitNumericPassword(password string) bool {
	if len(password) != 6 {
		return false
	}
	for _, c := range password {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// IUserService 用户服务接口（仅对外暴露的API）
type IUserService interface {
	// WalletLogin 钱包登录（如果用户不存在则自动注册）
	WalletLogin(ctx context.Context, req *WalletLoginReq) (*WalletLoginRes, error)

	// GetUserProfile 获取用户信息
	GetUserProfile(ctx context.Context, userId int64) (*GetUserProfileRes, error)

	// SetPassword 设置用户密码
	SetPassword(ctx context.Context, userId int64, req *SetPasswordReq) (*SetPasswordRes, error)

	// VerifyPassword 验证用户密码
	VerifyPassword(ctx context.Context, userId int64, req *VerifyPasswordReq) (*VerifyPasswordRes, error)

	// GetDepositAddress 获取用户充值地址（不存在则创建）
	GetDepositAddress(ctx context.Context, userId int64) (*GetDepositAddressRes, error)

	// GetSubUserPerformance 获取伞下用户业绩
	GetSubUserPerformance(ctx context.Context, userId int64) (*GetSubUserPerformanceRes, error)

	// UpdateUserParentInviteCode 修改用户上级邀请码
	UpdateUserParentInviteCode(ctx context.Context, req *UpdateUserParentInviteCodeReq) (*UpdateUserParentInviteCodeRes, error)

	// GetUserDescendants 根据钱包地址查询用户下级(最多5级)
	GetUserDescendants(ctx context.Context, req *GetUserDescendantsReq) (*GetUserDescendantsRes, error)

	// GetUserList 获取用户列表（管理端）
	GetUserList(ctx context.Context, req *GetUserListReq) (*GetUserListRes, error)

	// GetPerformanceStats 业绩统计
	GetPerformanceStats(ctx context.Context, req *GetPerformanceStatsReq) (*GetPerformanceStatsRes, error)

	// GetPerformanceHistory 历史业绩
	GetPerformanceHistory(ctx context.Context, req *GetPerformanceHistoryReq) (*GetPerformanceHistoryRes, error)

	// GetYesterdayWeightedRanking 昨日直推排行
	GetYesterdayWeightedRanking(ctx context.Context) (*GetYesterdayWeightedRankingRes, error)

	// GetReleaseUSDTDetails 查询释放USDT明细信息
	GetReleaseUSDTDetails(ctx context.Context, req *GetReleaseUSDTDetailsReq) (*GetReleaseUSDTDetailsRes, error)

	// GetReleaseUSDTDetail 获取USDT明细详情
	GetReleaseUSDTDetail(ctx context.Context, req *GetReleaseUSDTDetailReq) (*GetReleaseUSDTDetailRes, error)

	// GetNodeInfoList 分页显示节点信息
	GetNodeInfoList(ctx context.Context, req *GetNodeInfoListReq) (*GetNodeInfoListRes, error)

	// GetWithdrawRecords 查询提现记录
	GetWithdrawRecords(ctx context.Context, req *GetWithdrawRecordsReq) (*GetWithdrawRecordsRes, error)

	// GetRechargeRecords 查询充值记录
	GetRechargeRecords(ctx context.Context, req *GetRechargeRecordsReq) (*GetRechargeRecordsRes, error)

	// GetUserDetail 获取用户详情
	GetUserDetail(ctx context.Context, req *GetUserDetailReq) (*GetUserDetailRes, error)

	// GetUserWithdrawQuota 获取用户提现额度
	GetUserWithdrawQuota(ctx context.Context, req *GetUserWithdrawQuotaReq) (*GetUserWithdrawQuotaRes, error)

	// ResetUserWithdrawQuota 重置用户提现额度
	ResetUserWithdrawQuota(ctx context.Context, req *ResetUserWithdrawQuotaReq) (*ResetUserWithdrawQuotaRes, error)

	// AdminResetUserPassword 管理员重置用户密码
	AdminResetUserPassword(ctx context.Context, req *AdminResetUserPasswordReq) (*AdminResetUserPasswordRes, error)

	// GetUserStakingRecords 获取用户质押记录
	GetUserStakingRecords(ctx context.Context, req *GetUserStakingRecordsReq) (*GetUserStakingRecordsRes, error)

	// GetUserRewardRecords 获取用户奖励明细
	GetUserRewardRecords(ctx context.Context, req *GetUserRewardRecordsReq) (*GetUserRewardRecordsRes, error)

	// GetUserRewardAgg 获取用户各项奖励汇总
	GetUserRewardAgg(ctx context.Context, req *GetUserRewardAggReq) (*GetUserRewardAggRes, error)

	// GetUserBalanceChangeLogs 获取用户资金明细（cobo_balance_change_log）
	GetUserBalanceChangeLogs(ctx context.Context, req *GetUserBalanceChangeLogsReq) (*GetUserBalanceChangeLogsRes, error)

	// GetUserInviteRecords 获取用户邀请明细
	GetUserInviteRecords(ctx context.Context, req *GetUserInviteRecordsReq) (*GetUserInviteRecordsRes, error)

	// GetUserMintRecords 获取用户拼团记录
	GetUserMintRecords(ctx context.Context, req *GetUserMintRecordsReq) (*GetUserMintRecordsRes, error)

	// GetUserNodePurchaseRecords 获取用户节点购买记录
	GetUserNodePurchaseRecords(ctx context.Context, req *GetUserNodePurchaseRecordsReq) (*GetUserNodePurchaseRecordsRes, error)

	// SetServiceCenter 设置服务中心
	SetServiceCenter(ctx context.Context, req *SetServiceCenterReq) (*SetServiceCenterRes, error)

	// GetServiceCenterUsers 获取服务中心用户列表
	GetServiceCenterUsers(ctx context.Context) (*GetServiceCenterUsersRes, error)

	// CancelServiceCenter 取消服务中心
	CancelServiceCenter(ctx context.Context, req *CancelServiceCenterReq) (*CancelServiceCenterRes, error)

	// SetServiceCenterRate 设置服务中心点位比例
	SetServiceCenterRate(ctx context.Context, req *SetServiceCenterRateReq) (*SetServiceCenterRateRes, error)

	// UpdateUserWalletAddress 修改用户地址
	UpdateUserWalletAddress(ctx context.Context, req *UpdateUserWalletAddressReq) error

	// ReplaceWalletAddress 后台替换钱包地址（跨表事务，异步执行）
	ReplaceWalletAddress(ctx context.Context, req *ReplaceWalletAddressReq) error

	// GetAddressUpdateLogs 获取用户的地址更换日志
	GetAddressUpdateLogs(ctx context.Context, req *GetAddressUpdateLogsReq) (*GetAddressUpdateLogsRes, error)

	// UpdateTeamCanWithdraw 更新团队成员的提现权限
	UpdateTeamCanWithdraw(ctx context.Context, req *UpdateTeamCanWithdrawReq) error

	// UpdateTeamStakeRate 更新团队成员的质押收益率
	UpdateTeamStakeRate(ctx context.Context, req *UpdateTeamStakeRateReq) error

	// UpdateUserNodeExempt 更新用户赠送节点业绩豁免状态
	UpdateUserNodeExempt(ctx context.Context, req *UpdateUserNodeExemptReq) error

	// GetUserRealtimeStats 获取用户实时数据
	GetUserRealtimeStats(ctx context.Context, req *GetUserRealtimeStatsReq) (*GetUserRealtimeStatsRes, error)

	// GetUserRecordCounts 获取用户记录数量统计
	GetUserRecordCounts(ctx context.Context, req *GetUserRecordCountsReq) (*GetUserRecordCountsRes, error)
	// GetUserYYReleaseRecords 获取用户YY释放记录
	GetUserYYReleaseRecords(ctx context.Context, req *GetUserYYReleaseRecordsReq) (*GetUserYYReleaseRecordsRes, error)
	// GetUserLoserCompensations 获取用户补偿订单
	GetUserLoserCompensations(ctx context.Context, req *GetUserLoserCompensationsReq) (*GetUserLoserCompensationsRes, error)
	// GetUserLoserCompReleaseLogs 获取用户补偿释放日志
	GetUserLoserCompReleaseLogs(ctx context.Context, req *GetUserLoserCompReleaseLogsReq) (*GetUserLoserCompReleaseLogsRes, error)
	// SetUseFullPerf 设置团队奖是否使用完整团队业绩（不扣除大区业绩）
	SetUseFullPerf(ctx context.Context, req *SetUseFullPerfReq) (*SetUseFullPerfRes, error)
}

// userService 用户服务实现
type userService struct {
	userRepo            repository.IUserRepository
	passwordRepo        repository.IUserPasswordRepository
	depositAddressRepo  *repository.UserDepositAddressRepository
	perfRepo            rewardRepo.IUserPerformanceRepository
	quotaRepo           rewardRepo.IQuotaRepository
	assetRecordRepo     rewardRepo.IAssetRecordRepository
	stakingPackageRepo  rewardRepo.IStakingPackageRepository
	vipLevelRepo        rewardRepo.IUserVipLevelRepository
	vipAdjustmentSvc    vipAdjustmentSvc.IVipAdjustmentService
	teamDao             teamDao.ITeamDao
	playerDao           apgDao.IPlayerDao
	balanceChangeLogDao dao.IBalanceChangeLogDao
	vnsHomeDataDao      vnsDao.IUserVnsHomeDataDao
	teamStatsSvc        teamstats.ITeamStatsService
}

// NewUserService 创建用户服务实例
func NewUserService() IUserService {
	return &userService{
		userRepo:            repository.NewUserRepository(),
		passwordRepo:        repository.NewUserPasswordRepository(),
		depositAddressRepo:  repository.NewUserDepositAddressRepository(),
		perfRepo:            rewardRepo.NewUserPerformanceRepository(),
		quotaRepo:           rewardRepo.NewQuotaRepository(),
		assetRecordRepo:     rewardRepo.NewAssetRecordRepository(),
		stakingPackageRepo:  rewardRepo.NewStakingPackageRepository(),
		vipLevelRepo:        rewardRepo.NewUserVipLevelRepository(),
		vipAdjustmentSvc:    vipAdjustmentSvc.NewVipAdjustmentService(),
		teamDao:             teamDao.NewTeamDao(),
		playerDao:           apgDao.Player,
		balanceChangeLogDao: dao.NewBalanceChangeLogDao(),
		vnsHomeDataDao:      vnsDao.NewUserVnsHomeDataDao(),
		teamStatsSvc:        teamstats.NewTeamStatsService(),
	}
}

// WalletLogin 钱包登录（如果用户不存在则自动注册）
func (s *userService) WalletLogin(ctx context.Context, req *WalletLoginReq) (*WalletLoginRes, error) {

	walletAddress := strings.ToLower(req.WalletAddress)

	// 1. 验证钱包地址格式
	if !utils.IsValidEthereumAddress(walletAddress) {
		return nil, errors.New("无效的钱包地址格式")
	}

	// 2. 防重放：校验时间窗口与一次性nonce
	// 允许的时间偏移：5分钟
	/*if time.Since(time.Unix(req.Timestamp, 0)) > 5*time.Minute || time.Until(time.Unix(req.Timestamp, 0)) > 5*time.Minute {
		return nil, errors.New("请求已过期或时间异常")
	}*/

	// 注意：不引入服务端重放Key存储，避免高频登录导致缓存压力

	// 3. 验证钱包签名（绑定 message 内容包含 nonce 与 timestamp）
	// 建议前端 message 固定格式：Login to XWFrame\naddr=<address>\nnonce=<nonce>\nts=<timestamp>
	valid, err := utils.VerifyWalletSignature(walletAddress, req.Message, req.Signature)
	if err != nil {
		return nil, fmt.Errorf("签名验证失败: %v", err)
	}
	if !valid {
		return nil, errors.New("钱包签名验证失败")
	}

	// 4. 检查用户是否存在
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil {
		if err.Error() != "sql: no rows in result set" {
			return nil, fmt.Errorf("查询用户失败: %v", err)
		}
	}

	var isNewUser bool
	var userInfo *UserInfo

	if userEntity == nil {
		// 用户不存在：业务要求必须提供邀请码才能注册
		// 但如果是系统第一个用户（ID=1，钱包地址固定），则不需要邀请码
		const firstUserWalletAddress = "0x7392043a467f5911d6475d92e1b42b9064df23c7"
		isFirstUser := strings.EqualFold(walletAddress, firstUserWalletAddress)
		if req.InviteCode == "" && !isFirstUser {
			return nil, fmt.Errorf("%d", consts.ErrInviteRequired)
		}

		var parentUser *entity.UserEntity
		var err error
		if req.InviteCode != "" {
			// 验证邀请码有效性并获取上级用户信息
			parentUser, err = s.validateInviteCode(ctx, req.InviteCode)
			if err != nil {
				return nil, fmt.Errorf("验证邀请码失败: %v", err)
			}
			if parentUser == nil {
				return nil, errors.New("邀请码不存在")
			}
		}

		// 自动注册
		var parentWalletAddress string
		var parentUserId int64
		var parentTeamId *int64
		if parentUser != nil {
			parentWalletAddress = parentUser.WalletAddress
			parentUserId = parentUser.Id
			parentTeamId = parentUser.TeamId
		}
		userInfo, err = s.createNewUser(ctx, walletAddress, req.InviteCode, parentWalletAddress, parentUserId, parentTeamId)
		if err != nil {
			return nil, fmt.Errorf("自动注册失败: %v", err)
		}
		isNewUser = true
	} else {
		// 用户存在，检查是否需要绑定邀请码
		if req.InviteCode != "" && userEntity.ParentInviteCode == "" {
			// 验证邀请码是否存在并获取上级用户信息（现在验证8-16位随机数邀请码）
			parentUser, err := s.validateInviteCode(ctx, req.InviteCode)
			if err != nil {
				return nil, fmt.Errorf("验证邀请码失败: %v", err)
			}
			if parentUser == nil {
				return nil, errors.New("邀请码不存在")
			}

			// 更新用户的邀请码和上级钱包地址（现在存储8-16位随机数邀请码）
			parentLeaderWallet := ""
			if leader, e := s.getOrFallbackParentLeaderWalletAddress(ctx, parentUser.Id, parentUser.WalletAddress); e == nil {
				parentLeaderWallet = leader
			}
			// 获取直接上级的团队信息（如果上级是团队长，使用其团队）
			bindTeamId, bindTeamName := s.getParentTeamInfo(ctx, parentUser)
			if bindTeamId == 0 && parentUser.TeamId != nil {
				bindTeamId = *parentUser.TeamId
			}

			err = s.updateUserWithLeaderWalletAddressCompatibility(ctx, userEntity.Id, map[string]interface{}{
				"parent_invite_code":    req.InviteCode,
				"parent_wallet_address": parentUser.WalletAddress,
				"team_id":               bindTeamId,
				"team_name":             bindTeamName,
				"leader_wallet_address": parentLeaderWallet,
			})
			if err != nil {
				return nil, fmt.Errorf("更新推荐关系失败: %v", err)
			}
			userEntity.ParentInviteCode = req.InviteCode
			userEntity.ParentWalletAddress = parentUser.WalletAddress
		}

		// 更新最后登录时间
		err = s.userRepo.UpdateLastLoginAt(ctx, userEntity.Id)
		if err != nil {
			return nil, fmt.Errorf("更新登录时间失败: %v", err)
		}

		userInfo = s.convertEntityToModel(userEntity)
		isNewUser = false
	}

	// 5. 生成JWT Token
	token, err := s.generateUserToken(ctx, userInfo)
	if err != nil {
		return nil, fmt.Errorf("生成Token失败: %v", err)
	}

	// 6. 生成 API 签名密钥（与 token 绑定，支持多设备）
	apiSecret, err := s.generateApiSecret(ctx, userInfo.Id, token)
	if err != nil {
		return nil, gerror.Newf("generate api_secret failed: %v", err)
	}

	return &WalletLoginRes{
		Token:     token,
		ApiSecret: apiSecret,
		User:      userInfo,
		IsNewUser: isNewUser,
	}, nil
}

// WalletVerify 钱包验证
// 已移除 WalletVerify 对外方法，校验内聚于 WalletLogin

// GetUserProfile 获取用户信息
func (s *userService) GetUserProfile(ctx context.Context, userId int64) (*GetUserProfileRes, error) {
	userEntity, err := s.userRepo.GetUserById(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %v", err)
	}
	if userEntity == nil {
		return nil, errors.New("用户不存在")
	}

	userInfo := s.convertEntityToModel(userEntity)

	// 获取是否设置过密码
	hasPassword, err := s.passwordRepo.HasPassword(ctx, userId)
	if err != nil {
		g.Log().Warningf(ctx, "[GetUserProfile] 查询密码设置失败: userID=%d, err=%v", userId, err)
	}
	userInfo.HasSetPassword = hasPassword

	// 获取团队统计信息（复用 teamstats，基于用户关系实时计算，不依赖 staking_package）
	teamMemberCount := 0
	directCount := 0
	if agg, err := s.teamStatsSvc.GetOverviewAggByInviteCode(ctx, userEntity.InviteCode); err == nil {
		teamMemberCount = agg.TeamTotalUserCount
		directCount = agg.DirectCount
	} else {
		g.Log().Warningf(ctx, "[GetUserProfile] 获取团队人数失败: userID=%d, err=%v", userId, err)
	}

	// 获取赠送节点及格情况与提现激活信息
	giftNodeInfo, isGiftUser, activateAmount := s.getGiftNodeInfo(ctx, userId)

	// 仅按 Staking V2 口径判断是否已出局：有出局记录且无运行中记录
	isExited := false
	activeCount, err := g.DB().Model("staking_v2_order").Ctx(ctx).
		Where("user_id = ? AND status = ?", userId, consts.StakingV2StatusActive).
		Count()
	if err != nil {
		g.Log().Warningf(ctx, "[GetUserProfile] 查询StakingV2运行中记录失败: userID=%d, err=%v", userId, err)
	} else if activeCount == 0 {
		exitedCount, exitedErr := g.DB().Model("staking_v2_order").Ctx(ctx).
			Where("user_id = ? AND status = ?", userId, consts.StakingV2StatusCapped).
			Count()
		if exitedErr != nil {
			g.Log().Warningf(ctx, "[GetUserProfile] 查询StakingV2出局记录失败: userID=%d, err=%v", userId, exitedErr)
		} else {
			isExited = exitedCount > 0
		}
	}

	return &GetUserProfileRes{
		User:            userInfo,
		TeamMemberCount: teamMemberCount,
		DirectCount:     directCount,
		IsExited:        isExited,
		IsGiftUser:      isGiftUser,
		ActivateAmount:  activateAmount,
		GiftNodeInfo:    giftNodeInfo,
	}, nil
}

// SetPassword 设置用户密码
func (s *userService) SetPassword(ctx context.Context, userId int64, req *SetPasswordReq) (*SetPasswordRes, error) {
	hasPassword, err := s.passwordRepo.HasPassword(ctx, userId)
	if err != nil {
		return nil, gerror.Wrap(err, "check password status failed")
	}
	if hasPassword {
		if err := repository.VerifyUserPassword(ctx, s.passwordRepo, userId, req.OldPassword); err != nil {
			if err.Error() == "password is required" {
				return nil, gerror.New("old password is required")
			}
			return nil, err
		}
		if req.OldPassword == req.Password {
			return nil, gerror.New("new password must be different from old password")
		}
	}

	if !isSixDigitNumericPassword(req.Password) {
		return nil, gerror.New("password must be exactly 6 digits")
	}

	// 加密密码
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return nil, gerror.Wrap(err, "hash password failed")
	}

	// 保存密码
	if err := s.passwordRepo.SetPassword(ctx, userId, hashedPassword); err != nil {
		return nil, gerror.Wrap(err, "save password failed")
	}

	return &SetPasswordRes{Success: true}, nil
}

// VerifyPassword 验证用户密码
func (s *userService) VerifyPassword(ctx context.Context, userId int64, req *VerifyPasswordReq) (*VerifyPasswordRes, error) {
	// 获取密码
	passwordEntity, err := s.passwordRepo.GetPassword(ctx, userId)
	if err != nil {
		return nil, gerror.Wrap(err, "get password failed")
	}
	if passwordEntity == nil {
		return nil, gerror.New("password not set")
	}

	// 验证密码
	valid := utils.VerifyPassword(req.Password, passwordEntity.PasswordHash)
	return &VerifyPasswordRes{Valid: valid}, nil
}

// getGiftNodeInfo 获取用户赠送节点及格情况
func (s *userService) getGiftNodeInfo(ctx context.Context, userId int64) (*GiftNodeInfo, bool, string) {
	activateAmount := "0.00"

	selfPurchaseCount, err := g.DB().Model("cobo_node_purchase").Ctx(ctx).
		Where("user_id = ?", userId).
		Where("is_gift = ?", 0).
		Count()
	if err != nil {
		g.Log().Warningf(ctx, "[GetUserProfile] 查询自购节点数量失败: userID=%d, err=%v", userId, err)
		selfPurchaseCount = 1
	}
	hasSelfPurchase := selfPurchaseCount > 0

	// 查询用户的赠送节点
	var giftPurchases []struct {
		ID                  int64
		Amount              float64
		GiftPerformancePass bool
	}

	err = g.DB().Model("cobo_node_purchase").Ctx(ctx).
		Fields("id, amount, gift_performance_pass").
		Where("user_id = ?", userId).
		Where("is_gift = ?", 1).
		Scan(&giftPurchases)

	if err != nil || len(giftPurchases) == 0 {
		if err != nil {
			g.Log().Warningf(ctx, "[GetUserProfile] 查询赠送节点失败: userID=%d, err=%v", userId, err)
		}
		return nil, false, activateAmount
	}

	isGiftUser := !hasSelfPurchase

	// 计算赠送总金额
	var totalGiftAmount float64
	allPassed := true
	for _, p := range giftPurchases {
		totalGiftAmount += p.Amount
		if !p.GiftPerformancePass {
			allPassed = false
		}
	}

	// 获取用户团队业绩（从 cobo_performance）
	var perfRow struct {
		TeamPerformance float64 `json:"team_performance"`
	}
	if err := g.DB().Model("cobo_performance").Ctx(ctx).
		Fields("team_performance").
		Where("user_id = ?", userId).
		Scan(&perfRow); err != nil {
		g.Log().Warningf(ctx, "[GetUserProfile] 查询团队业绩失败: userID=%d, err=%v", userId, err)
	}
	teamPerformance := perfRow.TeamPerformance

	requiredPerformance := totalGiftAmount * 10
	if isGiftUser {
		remaining := requiredPerformance - teamPerformance
		if remaining < 0 {
			remaining = 0
		}
		activateAmount = fmt.Sprintf("%.2f", remaining)
	}

	return &GiftNodeInfo{
		HasGiftNode:         true,
		GiftNodePass:        allPassed,
		GiftAmount:          fmt.Sprintf("%.2f", totalGiftAmount),
		TeamPerformance:     fmt.Sprintf("%.2f", teamPerformance),
		RequiredPerformance: fmt.Sprintf("%.2f", requiredPerformance),
	}, isGiftUser, activateAmount
}

// UpdateUserProfile 更新用户信息
// 已移除 UpdateUserProfile 对外方法，DApp最小化不提供资料修改

// createNewUser 创建新用户（私有方法）
func (s *userService) createNewUser(ctx context.Context, walletAddress, parentInviteCode, parentWalletAddress string, parentUserId int64, parentTeamId *int64) (*UserInfo, error) {
	// 生成唯一的10位邀请码（大写字母+数字）
	var inviteCode string
	var err error
	maxRetries := 10 // 最多重试10次

	for i := 0; i < maxRetries; i++ {
		inviteCode = utils.GenerateInviteCode()

		// 检查邀请码是否已存在
		exists, checkErr := s.userRepo.CheckInviteCodeExists(ctx, inviteCode)
		if checkErr != nil {
			return nil, fmt.Errorf("检查邀请码唯一性失败: %v", checkErr)
		}

		if !exists {
			break // 邀请码不存在，可以使用
		}

		// 如果邀请码已存在且是最后一次重试，返回错误
		if i == maxRetries-1 {
			return nil, fmt.Errorf("生成唯一邀请码失败，已重试%d次", maxRetries)
		}
	}

	// 获取父级团队名称
	var teamName string
	if parentTeamId != nil && *parentTeamId > 0 {
		// 从数据库查询团队名称
		teams, _ := s.teamDao.GetByIds(ctx, []int64{*parentTeamId})
		if team, ok := teams[*parentTeamId]; ok {
			teamName = team.Name
		}
	}

	// 创建用户实体
	userEntity := &entity.UserEntity{
		WalletAddress:       walletAddress,
		ParentInviteCode:    parentInviteCode,
		ParentWalletAddress: parentWalletAddress,
		InviteCode:          inviteCode,
		TeamId:              parentTeamId,
		TeamName:            teamName,
		CanWithdraw:         true,
		CanSwap:             true,
		CanTransfer:         true,
		Status:              consts.UserStatusActive,
		IsVertex:            false,
		LastLoginAt:         &time.Time{},
	}

	// 保存到数据库
	err = s.userRepo.CreateUser(ctx, userEntity)
	if err != nil {
		return nil, err
	}

	// 绑定关系时同步 leader_wallet_address（若列存在则写入；若已移除则忽略）
	if leader, e := s.getOrFallbackParentLeaderWalletAddress(ctx, parentUserId, parentWalletAddress); e == nil && leader != "" {
		_ = s.updateUserWithLeaderWalletAddressCompatibility(ctx, userEntity.Id, map[string]interface{}{
			"leader_wallet_address": leader,
		})
	}

	// 更新最后登录时间
	err = s.userRepo.UpdateLastLoginAt(ctx, userEntity.Id)
	if err != nil {
		return nil, err
	}

	// 重新从数据库查询完整的用户信息
	userEntity, err = s.userRepo.GetUserById(ctx, userEntity.Id)
	if err != nil {
		return nil, fmt.Errorf("查询新用户信息失败: %v", err)
	}

	// 新用户注册后，异步预创建充值地址（user_deposit_address），避免后续业务接口再触发创建
	go func(newUserID int64) {
		bgCtx := context.Background()
		if addr, err := s.depositAddressRepo.GetUserDepositAddress(bgCtx, newUserID); err == nil && addr != nil {
			return
		}
		if _, err := s.createDepositAddress(bgCtx, newUserID); err != nil {
			g.Log().Warningf(bgCtx, "[用户注册] 异步创建充值地址失败: userID=%d, err=%v", newUserID, err)
		}
	}(userEntity.Id)

	return s.convertEntityToModel(userEntity), nil
}

// convertEntityToModel 将实体转换为模型（私有方法）
func (s *userService) convertEntityToModel(entity *entity.UserEntity) *UserInfo {
	userInfo := &UserInfo{
		Id:                  entity.Id,
		WalletAddress:       entity.WalletAddress,
		ParentInviteCode:    entity.ParentInviteCode,
		ParentWalletAddress: entity.ParentWalletAddress,
		InviteCode:          entity.InviteCode,
		CanWithdraw:         entity.CanWithdraw,
		Status:              entity.Status,
		CreatedAt:           entity.CreatedAt,
		UpdatedAt:           entity.UpdatedAt,
	}

	if entity.LastLoginAt != nil {
		userInfo.LastLoginAt = *entity.LastLoginAt
	}

	return userInfo
}

// generateUserToken 生成用户Token（私有方法）
//
// 多设备：每个 token 用 TokenHash 作为后缀写入独立的 Redis key
// （user:token:{user_id}:{token_hash}），互不覆盖。
func (s *userService) generateUserToken(ctx context.Context, userInfo *UserInfo) (string, error) {
	// 获取配置中的过期时间
	cfg := config.GetConfig()
	expire, err := cfg.Get(ctx, "jwt.expire")
	if err != nil {
		return "", err
	}

	// 获取配置中的JWT密钥
	secret, err := cfg.Get(ctx, "jwt.secret")
	if err != nil {
		return "", err
	}

	// 计算过期时间
	expireDuration := time.Duration(expire.Int()) * time.Second
	expireTime := time.Now().Add(expireDuration)

	claims := map[string]interface{}{
		"user_id":        userInfo.Id,
		"wallet_address": userInfo.WalletAddress,
		"exp":            expireTime.Unix(),
	}

	// 生成JWT token
	token, err := utils.GenerateJWTToken(secret.String(), claims)
	if err != nil {
		return "", err
	}

	// 将 token 存储到独立的 session key，支持多设备同时在线
	redisCache := cache.GetRedisCache()
	cacheKey := cache.CacheKey{}.User().TokenSession(userInfo.Id, utils.TokenHash(token))
	err = redisCache.Set(ctx, cacheKey, token, expireDuration)
	if err != nil {
		return "", fmt.Errorf("存储token到缓存失败: %v", err)
	}

	return token, nil
}

// generateApiSecret 生成用户 API 签名密钥并存储到 Redis
//
// 多设备：api_secret 与 token 一一绑定，存入 user:api_secret:{user_id}:{token_hash}
func (s *userService) generateApiSecret(ctx context.Context, userId int64, token string) (string, error) {
	cfg := config.GetConfig()
	expire, err := cfg.Get(ctx, "jwt.expire")
	if err != nil {
		expire = g.NewVar(86400)
	}
	expireDuration := time.Duration(expire.Int()) * time.Second

	// 如果配置中指定了 api_secret 有效期，使用配置值
	apiSecretExpire, err := cfg.Get(ctx, "apiSign.secretExpire")
	if err == nil && apiSecretExpire.Int() > 0 {
		expireDuration = time.Duration(apiSecretExpire.Int()) * time.Second
	}

	secret := utils.GenerateRandomString(32)
	key := cache.CacheKey{}.User().ApiSecretSession(userId, utils.TokenHash(token))
	redisCache := cache.GetRedisCache()
	err = redisCache.Set(ctx, key, secret, expireDuration)
	if err != nil {
		return "", fmt.Errorf("store api_secret failed: %v", err)
	}
	return secret, nil
}

// validateInviteCode 验证邀请码是否存在并返回用户信息（现在验证8-16位随机数邀请码）
func (s *userService) validateInviteCode(ctx context.Context, inviteCode string) (*entity.UserEntity, error) {
	// 验证邀请码格式（8-16位大写字母+数字）
	if !utils.IsValidInviteCode(inviteCode) {
		return nil, nil
	}

	// 根据邀请码获取用户信息
	userEntity, err := s.userRepo.GetUserByInviteCode(ctx, inviteCode)
	if err != nil {
		return nil, fmt.Errorf("查询邀请码用户失败: %v", err)
	}

	return userEntity, nil
}

// GetDepositAddress 获取用户充值地址（不存在则创建）
func (s *userService) GetDepositAddress(ctx context.Context, userId int64) (*GetDepositAddressRes, error) {
	// 1. 查询用户是否已有有效的充值地址
	addressEntity, err := s.depositAddressRepo.GetUserDepositAddress(ctx, userId)
	if err != nil {
		return nil, gerror.Wrapf(err, "查询用户充值地址失败")
	}

	// 2. 如果已有地址，直接返回
	if addressEntity != nil {
		return &GetDepositAddressRes{
			Address: &DepositAddressInfo{
				Address:   addressEntity.Address,
				ChainID:   addressEntity.ChainID,
				IsValid:   addressEntity.IsValid,
				CreatedAt: addressEntity.CreatedAt,
			},
		}, nil
	}

	// 3. 如果没有地址，创建新地址
	newAddress, err := s.createDepositAddress(ctx, userId)
	if err != nil {
		return nil, gerror.Wrapf(err, "创建充值地址失败")
	}

	return &GetDepositAddressRes{
		Address: newAddress,
	}, nil
}

// createDepositAddress 创建充值地址（私有方法）
func (s *userService) createDepositAddress(ctx context.Context, userId int64) (*DepositAddressInfo, error) {
	// 获取Cobo配置
	coboConfig, err := config.GetCoboConfig(ctx)
	if err != nil {
		return nil, gerror.Wrapf(err, "获取Cobo配置失败")
	}

	// 创建Cobo客户端
	coboClient, err := cobo.NewCoboClient(&cobo.Config{
		APISecret: coboConfig.APISecret,
		Env:       coboConfig.Env,
		Timeout:   coboConfig.Timeout,
	})
	if err != nil {
		return nil, gerror.Wrapf(err, "创建Cobo客户端失败")
	}

	// 调用Cobo API创建地址
	addresses, err := coboClient.CreateAddress(ctx, &cobo.CreateAddressRequest{
		WalletID: coboConfig.WalletID,
		ChainID:  "BSC_BNB", // 固定使用BSC链
		Count:    1,
	})
	if err != nil {
		return nil, gerror.Wrapf(err, "调用Cobo API创建地址失败")
	}

	if len(addresses) == 0 {
		return nil, gerror.New("Cobo返回地址为空")
	}

	// 保存地址到数据库
	addressEntity, err := s.depositAddressRepo.CreateDepositAddress(ctx, userId, "BSC_BNB", addresses[0].Address)
	if err != nil {
		return nil, gerror.Wrapf(err, "保存充值地址到数据库失败")
	}

	return &DepositAddressInfo{
		Address:   addressEntity.Address,
		ChainID:   addressEntity.ChainID,
		IsValid:   addressEntity.IsValid,
		CreatedAt: addressEntity.CreatedAt,
	}, nil
}

// GetSubUserPerformance 获取伞下用户业绩
func (s *userService) GetSubUserPerformance(ctx context.Context, userId int64) (*GetSubUserPerformanceRes, error) {
	// 1. 获取用户信息
	userEntity, err := s.userRepo.GetUserById(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %v", err)
	}
	if userEntity == nil {
		return nil, errors.New("用户不存在")
	}

	// 2. 调用仓储层获取伞下用户业绩
	totalAmount, err := s.userRepo.GetSubUserPerformance(ctx, userEntity.InviteCode)
	if err != nil {
		return nil, fmt.Errorf("查询伞下用户业绩失败: %v", err)
	}

	return &GetSubUserPerformanceRes{
		TotalAmount: totalAmount,
	}, nil
}

// UpdateUserParentInviteCode 修改用户上级邀请码
func (s *userService) UpdateUserParentInviteCode(ctx context.Context, req *UpdateUserParentInviteCodeReq) (*UpdateUserParentInviteCodeRes, error) {
	newWalletAddress := strings.ToLower(req.NewWalletAddress)

	// 1. 校验钱包地址格式
	if !utils.IsValidEthereumAddress(newWalletAddress) {
		return &UpdateUserParentInviteCodeRes{
			Success: false,
			Message: "新钱包地址格式不正确",
		}, nil
	}

	// 2. 根据新钱包地址查询用户信息
	newParentUser, err := s.userRepo.GetUserByWalletAddress(ctx, newWalletAddress)

	if err != nil || newParentUser == nil {
		return &UpdateUserParentInviteCodeRes{
			Success: false,
			Message: "新钱包地址对应的用户不存在",
		}, nil
	}

	// 3. 根据目标用户ID查询用户信息
	targetUser, err := s.userRepo.GetUserById(ctx, req.TargetUserId)

	if err != nil || targetUser == nil {
		return &UpdateUserParentInviteCodeRes{
			Success: false,
			Message: "目标用户不存在",
		}, nil
	}

	// 4. 检查是否与当前上级邀请码相同
	if targetUser.ParentInviteCode == newParentUser.InviteCode {
		return &UpdateUserParentInviteCodeRes{
			Success: false,
			Message: "新钱包地址对应的用户与当前上级用户相同，无需修改",
		}, nil
	}

	// 5. 检查关系死循环
	// 检查目标用户是否在新上级的上级链中（向上递归检查）
	hasCircularRelation, err := s.checkCircularRelation(ctx, targetUser.Id, newParentUser.Id)
	if err != nil {
		return nil, fmt.Errorf("检查关系循环失败: %v", err)
	}
	if hasCircularRelation {
		return &UpdateUserParentInviteCodeRes{
			Success: false,
			Message: "不能设置上级用户为下级，会造成关系死循环",
		}, nil
	}

	// 6. 检查 branch 分支限制
	// 假设A的当前上级是B，需要修改上级为C
	// 先查询A的每层上级，一直找到 branch 字段有值的
	targetUserBranch, foundTargetBranch, err := s.findFirstBranchInParentAncestors(ctx, targetUser.Id)
	if err != nil {
		return nil, fmt.Errorf("查询目标用户分支信息失败: %v", err)
	}

	if foundTargetBranch {
		// 如果找到了A的上级有 branch 值，需要检查C或其上级的 branch 是否与之相等
		// 对于C，应该从C本身开始查找（包括C本身及其上级）
		// 初始化已访问用户ID列表，防止死循环
		newParentBranch, foundNewParentBranch, err := s.findFirstBranchInParentAncestors(ctx, newParentUser.Id)
		if err != nil {
			return nil, fmt.Errorf("查询新上级用户分支信息失败: %v", err)
		}

		// 如果C的所有上级 branch 都没值，无法修改
		if !foundNewParentBranch {
			return &UpdateUserParentInviteCodeRes{
				Success: false,
				Message: fmt.Sprintf("不能修改上级，目标用户属于分支[%s]，但新上级及其所有上级都没有分支值", targetUserBranch),
			}, nil
		}

		// 如果C或其上级有 branch 值，必须与A的上级branch值相等才能修改
		if targetUserBranch != newParentBranch {
			return &UpdateUserParentInviteCodeRes{
				Success: false,
				Message: fmt.Sprintf("不能跨分支修改上级，目标用户属于分支[%s]，新上级属于分支[%s]", targetUserBranch, newParentBranch),
			}, nil
		}
	}
	// 如果A的上级 branch 都没有值，可以直接修改

	// 7. 执行更新操作
	parentLeaderWallet, err := s.getOrFallbackParentLeaderWalletAddress(ctx, newParentUser.Id, newParentUser.WalletAddress)
	if err != nil {
		return nil, fmt.Errorf("查询新上级leader_wallet_address失败: %v", err)
	}
	// 获取直接上级的团队信息（如果上级是团队长，使用其团队）
	bindTeamId, bindTeamName := s.getParentTeamInfo(ctx, newParentUser)
	if bindTeamId == 0 && newParentUser.TeamId != nil {
		bindTeamId = *newParentUser.TeamId
	}
	err = s.updateUserWithLeaderWalletAddressCompatibility(ctx, req.TargetUserId, map[string]interface{}{
		"parent_invite_code":    newParentUser.InviteCode,
		"parent_wallet_address": newParentUser.WalletAddress,
		"team_id":               bindTeamId,
		"team_name":             bindTeamName,
		"leader_wallet_address": parentLeaderWallet,
	})
	if err != nil {
		return nil, fmt.Errorf("更新用户上级邀请码失败: %v", err)
	}

	// 8. 递归更新目标用户下级的 team_id，排除团队长及其子树（保持就近原则）
	err = s.userRepo.UpdateDescendantsTeamIdExcludingLeaderSubtrees(ctx, targetUser.Id, &bindTeamId)
	if err != nil {
		return nil, fmt.Errorf("更新团队ID失败: %v", err)
	}

	return &UpdateUserParentInviteCodeRes{
		Success: true,
		Message: "修改用户上级邀请码成功",
	}, nil
}

// checkCircularRelation 检查关系死循环（私有方法）
// 检查targetUserId是否在newParentUserId的上级链中（向上递归）
// 如果targetUserId在newParentUserId的上级链中，则会造成死循环
func (s *userService) checkCircularRelation(ctx context.Context, targetUserId, newParentUserId int64) (bool, error) {
	// 从newParentUserId开始，向上递归查询所有上级
	// 如果targetUserId在newParentUserId的上级链中，则会造成死循环
	return s.isAncestor(ctx, targetUserId, newParentUserId)
}

// isAncestor 检查targetUserId是否是currentUserId的上级（向上递归检查）
// 性能优化：向上递归只需查询 O(h) 次，h为树的高度，通常 ≤ 10层
// 而向下递归可能需要查询 O(n) 次，n为所有下级用户数量
func (s *userService) isAncestor(ctx context.Context, targetUserId, currentUserId int64) (bool, error) {
	// 如果两个ID相同，说明会造成死循环
	if targetUserId == currentUserId {
		return true, nil
	}

	// 获取currentUserId的用户信息
	currentUser, err := s.userRepo.GetUserById(ctx, currentUserId)
	if err != nil {
		return false, fmt.Errorf("查询用户信息失败: %v", err)
	}

	// 如果没有上级，说明已经到根节点，不会造成死循环
	if currentUser == nil || currentUser.ParentInviteCode == "" {
		return false, nil
	}

	// 获取上级用户
	parentUser, err := s.userRepo.GetUserByInviteCode(ctx, currentUser.ParentInviteCode)
	if err != nil || parentUser == nil {
		// 上级用户不存在，不会造成死循环
		return false, nil
	}

	// 递归向上检查
	return s.isAncestor(ctx, targetUserId, parentUser.Id)
}

func (s *userService) updateUserWithLeaderWalletAddressCompatibility(ctx context.Context, userID int64, data map[string]interface{}) error {
	if userID <= 0 {
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	if err := s.userRepo.UpdateUser(ctx, userID, data); err == nil {
		return nil
	} else if isPgUndefinedColumn(err, "leader_wallet_address") {
		// 兼容：部分环境已移除 user_info.leader_wallet_address
		delete(data, "leader_wallet_address")
		if len(data) == 0 {
			return nil
		}
		return s.userRepo.UpdateUser(ctx, userID, data)
	} else {
		return err
	}
}

func (s *userService) getOrFallbackParentLeaderWalletAddress(ctx context.Context, parentUserID int64, parentWalletAddress string) (string, error) {
	parentWalletLower := strings.ToLower(strings.TrimSpace(parentWalletAddress))
	if parentUserID <= 0 && parentWalletLower == "" {
		return "", nil
	}

	// 优先读父用户的 leader_wallet_address（如果列存在）
	type row struct {
		LeaderWalletAddress string `json:"leader_wallet_address"`
		WalletAddress       string `json:"wallet_address"`
	}
	var r row
	err := db.GetDB().Ctx(ctx).Model("user_info").
		Fields("COALESCE(leader_wallet_address, '') as leader_wallet_address, wallet_address").
		Where("id = ?", parentUserID).
		Limit(1).
		Scan(&r)
	if err == nil {
		leader := strings.ToLower(strings.TrimSpace(r.LeaderWalletAddress))
		if leader != "" {
			return leader, nil
		}
		w := strings.ToLower(strings.TrimSpace(r.WalletAddress))
		if w != "" {
			return w, nil
		}
		if parentWalletLower != "" {
			return parentWalletLower, nil
		}
		return "", nil
	}
	if isPgUndefinedColumn(err, "leader_wallet_address") {
		// 列不存在：用父钱包地址兜底（保证业务继续）
		if parentWalletLower != "" {
			return parentWalletLower, nil
		}
		// 尝试仅查 wallet_address
		var rr struct {
			WalletAddress string `json:"wallet_address"`
		}
		if e := db.GetDB().Ctx(ctx).Model("user_info").
			Fields("wallet_address").
			Where("id = ?", parentUserID).
			Limit(1).
			Scan(&rr); e == nil {
			return strings.ToLower(strings.TrimSpace(rr.WalletAddress)), nil
		}
		return "", nil
	}
	return "", err
}

func isPgUndefinedColumn(err error, column string) bool {
	if err == nil || column == "" {
		return false
	}
	msg := strings.ToLower(err.Error())
	col := strings.ToLower(column)
	// 兼容 PostgreSQL 常见报错：column "xxx" does not exist / undefined column
	return strings.Contains(msg, col) && (strings.Contains(msg, "does not exist") || strings.Contains(msg, "undefined column"))
}

// getParentTeamInfo 获取直接上级的团队信息
// 如果直接上级是团队长，返回其作为 Leader 的团队 ID 和名称
// 如果不是团队长，返回 parentUser.TeamId 和 ""
func (s *userService) getParentTeamInfo(ctx context.Context, parentUser *entity.UserEntity) (teamId int64, teamName string) {
	teamDao := teamDao.NewTeamDao()

	// 检查直接上级是否是某个团队的 Leader
	if team, err := teamDao.GetByLeaderAddress(ctx, parentUser.WalletAddress); err == nil && team != nil {
		return team.Id, team.Name
	}

	// 不是团队长，返回 parentUser 的 team_id
	if parentUser.TeamId != nil {
		return *parentUser.TeamId, ""
	}
	return 0, ""
}

// findFirstBranchInParentAncestors 向上递归查找第一个有 branch 值的上级（从用户的上级开始查找）
// 返回: (branch值, 是否找到, 错误)
// 说明：从用户的上级开始向上查找，不包括用户本身
func (s *userService) findFirstBranchInParentAncestors(ctx context.Context, userId int64) (string, bool, error) {
	// 获取当前用户信息
	currentUser, err := s.userRepo.GetUserById(ctx, userId)
	if err != nil {
		return "", false, fmt.Errorf("查询用户信息失败: %v", err)
	}

	// 如果用户不存在，返回未找到
	if currentUser == nil {
		return "", false, nil
	}

	// 如果没有上级，说明已经到根节点，没有找到 branch
	if currentUser.ParentInviteCode == "" {
		return "", false, nil
	}

	// 获取上级用户（从上级开始查找）
	parentUser, err := s.userRepo.GetUserByInviteCode(ctx, currentUser.ParentInviteCode)
	if err != nil {
		return "", false, fmt.Errorf("查询上级用户失败: %v", err)
	}
	if parentUser == nil {
		return "", false, nil
	}

	// 检查上级用户是否有 branch 值（非空字符串）
	if parentUser.Branch != "" {
		return parentUser.Branch, true, nil
	}

	// 如果上级没有 branch，继续向上查找（使用从本身开始的方法）
	// 初始化已访问用户ID列表，包含当前用户，防止死循环（如果上级链中又回到当前用户）
	visitedUserIDs := []int64{userId}
	return s.findFirstBranchInAncestors(ctx, parentUser.Id, visitedUserIDs)
}

// findFirstBranchInAncestors 向上递归查找第一个有 branch 值的上级（从用户本身开始查找）
// 返回: (branch值, 是否找到, 错误)
// 说明：从用户本身开始向上查找，包括用户本身及其上级
func (s *userService) findFirstBranchInAncestors(ctx context.Context, userId int64, visitedUserIDs []int64) (string, bool, error) {
	// 检查是否已访问过该用户，防止死循环
	for _, visitedID := range visitedUserIDs {
		if visitedID == userId {
			// 发现重复，返回 "", true, nil，让业务流程继续往下走
			return "", true, nil
		}
	}

	// 将当前用户ID添加到已访问列表
	visitedUserIDs = append(visitedUserIDs, userId)

	// 获取当前用户信息
	fmt.Println("userId: ", userId)
	currentUser, err := s.userRepo.GetUserById(ctx, userId)
	if err != nil {
		return "", false, fmt.Errorf("查询用户信息失败: %v", err)
	}

	// 如果用户不存在，返回未找到
	if currentUser == nil {
		return "", false, nil
	}

	// 检查当前用户是否有 branch 值（非空字符串）
	if currentUser.Branch != "" {
		return currentUser.Branch, true, nil
	}

	// 如果没有上级，说明已经到根节点，没有找到 branch
	if currentUser.ParentInviteCode == "" {
		return "", false, nil
	}

	// 获取上级用户
	parentUser, err := s.userRepo.GetUserByInviteCode(ctx, currentUser.ParentInviteCode)
	if err != nil {
		return "", false, fmt.Errorf("查询上级用户失败: %v", err)
	}
	if parentUser == nil {
		return "", false, nil
	}

	// 递归向上查找，传递已访问用户ID列表
	return s.findFirstBranchInAncestors(ctx, parentUser.Id, visitedUserIDs)
}

// GetUserDescendants 根据钱包地址查询用户下级(最多5级)
func (s *userService) GetUserDescendants(ctx context.Context, req *GetUserDescendantsReq) (*GetUserDescendantsRes, error) {
	var userEntity *entity.UserEntity
	var err error

	// 1. 处理钱包地址参数
	if req.WalletAddress == "" {
		// 钱包地址为空：从配置读取顶级账号并聚合其下级
		raw := g.Cfg().MustGet(ctx, "blockchain.vns.super_leader_address", "").String()
		if raw == "" {
			return &GetUserDescendantsRes{Descendants: []UserDescendantInfo{}}, nil
		}

		uniq := make(map[string]struct{}, 8)
		for _, p := range strings.Split(raw, ",") {
			addr := strings.ToLower(strings.TrimSpace(p))
			if addr == "" || !utils.IsValidEthereumAddress(addr) {
				continue
			}
			uniq[addr] = struct{}{}
		}
		if len(uniq) == 0 {
			return &GetUserDescendantsRes{Descendants: []UserDescendantInfo{}}, nil
		}

		all := make([]UserDescendantInfo, 0, 128)
		for addr := range uniq {
			u, e := s.userRepo.GetUserByWalletAddress(ctx, addr)
			if e != nil || u == nil {
				continue
			}
			ds, e := s.getDescendantsRecursively(ctx, u.InviteCode, u.WalletAddress, 1, 4)
			if e != nil {
				return nil, fmt.Errorf("查询下级用户失败: %v", e)
			}
			all = append(all, ds...)
		}

		return &GetUserDescendantsRes{Descendants: all}, nil
	} else {
		walletAddress := strings.ToLower(req.WalletAddress)

		// 2. 验证钱包地址格式
		if !utils.IsValidEthereumAddress(walletAddress) {
			return nil, errors.New("钱包地址格式不正确")
		}

		// 3. 根据钱包地址查询用户是否存在
		userEntity, err = s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
		if err != nil {
			return nil, fmt.Errorf("查询用户失败: %v", err)
		}
		if userEntity == nil {
			// 用户不存在，返回空数据
			return &GetUserDescendantsRes{
				Descendants: []UserDescendantInfo{},
			}, nil
		}
	}

	// 4. 递归查询下级用户(最多4级)
	descendants, err := s.getDescendantsRecursively(ctx, userEntity.InviteCode, userEntity.WalletAddress, 1, 4)
	if err != nil {
		return nil, fmt.Errorf("查询下级用户失败: %v", err)
	}

	return &GetUserDescendantsRes{
		Descendants: descendants,
	}, nil
}

// getDescendantsRecursively 递归查询下级用户并组装树形结构
func (s *userService) getDescendantsRecursively(ctx context.Context, parentInviteCode, parentWalletAddress string, currentLevel, maxLevel int) ([]UserDescendantInfo, error) {
	// 如果超过最大层级，停止递归
	if currentLevel > maxLevel {
		return []UserDescendantInfo{}, nil
	}

	// 查询当前层级的直接下级
	directDescendants, err := s.userRepo.GetDirectDescendants(ctx, parentInviteCode, parentWalletAddress)
	if err != nil {
		return nil, fmt.Errorf("查询第%d层下级失败: %v", currentLevel, err)
	}

	var treeDescendants []UserDescendantInfo

	// 处理当前层级的用户
	for _, descendant := range directDescendants {
		// 递归查询下一层级
		childDescendants, err := s.getDescendantsRecursively(ctx, descendant.InviteCode, descendant.WalletAddress, currentLevel+1, maxLevel)
		if err != nil {
			return nil, err
		}

		descendantInfo := UserDescendantInfo{
			Id:                  descendant.Id,
			WalletAddress:       descendant.WalletAddress,
			ParentWalletAddress: descendant.ParentWalletAddress,
			InviteCode:          descendant.InviteCode,
			Level:               currentLevel,
			Child:               childDescendants,
		}
		treeDescendants = append(treeDescendants, descendantInfo)
	}

	return treeDescendants, nil
}

// GetUserList 获取用户列表（管理端）
func (s *userService) GetUserList(ctx context.Context, req *GetUserListReq) (*GetUserListRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	repoReq := &repository.GetUserListReq{
		UserID:           req.UserID,
		WalletAddress:    walletAddress,
		DepositAddress:   strings.ToLower(req.DepositAddress),
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
	repoReq.Page = req.Page
	repoReq.PageSize = req.PageSize

	repoRes, err := s.userRepo.GetUserList(ctx, repoReq)
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %v", err)
	}

	// 批量获取用户当前VIP等级
	userIDs := make([]int64, 0, len(repoRes.List))
	teamIDs := make([]int64, 0, len(repoRes.List))
	for _, user := range repoRes.List {
		userIDs = append(userIDs, user.Id)
		if user.TeamId != nil {
			teamIDs = append(teamIDs, *user.TeamId)
		}
	}

	// 批量获取团队信息
	teamMap := make(map[int64]string)
	if len(teamIDs) > 0 {
		teams, err := s.teamDao.GetByIds(ctx, teamIDs)
		if err != nil {
			g.Log().Warningf(ctx, "批量获取团队信息失败: %v", err)
		} else {
			for id, t := range teams {
				teamMap[id] = t.Name
			}
		}
	}

	// Read current level from user_info.vip_level.
	userIDToLevelMap := make(map[int64]int, len(userIDs))
	for _, user := range repoRes.List {
		userIDToLevelMap[user.Id] = user.VipLevel
	}
	effectiveVIPLevels := userIDToLevelMap

	adjustedLevelMap := make(map[int64]int, len(userIDs))
	if len(userIDs) > 0 {
		adjustmentRepo := rewardRepo.NewUserVipAdjustmentRepository()
		adjustments, adjErr := adjustmentRepo.BatchGetActiveAdjustments(ctx, userIDs, time.Now())
		if adjErr != nil {
			g.Log().Warningf(ctx, "批量查询用户等级调整记录失败: %v", adjErr)
		} else {
			for userID, adj := range adjustments {
				if adj == nil {
					continue
				}
				adjustedLevelMap[userID] = adj.AdjustedVipLevel
			}
		}
	}

	// Staking V1(staking_package) 已废弃，后台列表不再查询旧表日收益率
	stakeRateMap := make(map[int64]string)

	// 批量获取用户 Cobo 充值地址
	depositAddressMap := make(map[int64]string, len(userIDs))
	if len(userIDs) > 0 {
		var depositRows []struct {
			UserID  int64  `json:"user_id"`
			Address string `json:"address"`
		}
		if depErr := g.DB().Model("user_deposit_address").Ctx(ctx).
			Fields("user_id, address").
			Where("user_id IN (?) AND is_valid = ?", userIDs, true).
			Scan(&depositRows); depErr != nil {
			g.Log().Warningf(ctx, "批量查询用户充值地址失败: %v", depErr)
		} else {
			for _, row := range depositRows {
				depositAddressMap[row.UserID] = row.Address
			}
		}
	}

	var userList []AdminUserListItem
	for _, user := range repoRes.List {
		effectiveLevel := 0
		if level, ok := effectiveVIPLevels[user.Id]; ok {
			effectiveLevel = level
		}

		leader := ""
		if user.TeamId != nil {
			if name, ok := teamMap[*user.TeamId]; ok {
				leader = name
			}
		}

		stakeRate := ""
		if rate, ok := stakeRateMap[user.Id]; ok {
			stakeRate = rate
		}

		getDecimalBySQL := func(sql string, args ...interface{}) decimal.Decimal {
			record, queryErr := g.DB().GetOne(ctx, sql, args...)
			if queryErr != nil || record == nil {
				return decimal.Zero
			}
			v := strings.TrimSpace(record["total"].String())
			if v == "" {
				return decimal.Zero
			}
			d, parseErr := decimal.NewFromString(v)
			if parseErr != nil {
				return decimal.Zero
			}
			return d
		}

		totalRewardUSDT := getDecimalBySQL(`
			SELECT COALESCE(SUM(t.amount), 0) AS total
			FROM (
				SELECT COALESCE(SUM(p.direct_reward_amount), 0) AS amount
				FROM cobo_node_purchase p
				WHERE p.direct_reward_user_id = ?
				  AND p.direct_reward_amount > 0
				  AND p.is_gift = 0

				UNION ALL

				SELECT COALESCE(SUM(l.amount), 0) AS amount
				FROM cobo_balance_change_log l
				WHERE l.user_id = ?
				  AND l.change_type = 'staking_v2_referral_direct'

				UNION ALL

				SELECT COALESCE(SUM(l.amount), 0) AS amount
				FROM cobo_balance_change_log l
				WHERE l.user_id = ?
				  AND l.change_type = 'staking_v2_referral_indirect'

				UNION ALL

				SELECT COALESCE(SUM(d.granted_amount), 0) AS amount
				FROM group_match_team_reward_distribution d
				WHERE d.user_id = ?
				  AND COALESCE(d.granted_amount, 0) > 0

				UNION ALL

				SELECT COALESCE(SUM(d.granted_amount), 0) AS amount
				FROM group_purchase_leadership_reward_detail d
				WHERE d.user_id = ?
				  AND COALESCE(d.granted_amount, 0) > 0

				UNION ALL

				SELECT COALESCE(SUM(d.granted_amount), 0) AS amount
				FROM group_purchase_leadership_weight_reward_detail d
				WHERE d.user_id = ?
				  AND COALESCE(d.granted_amount, 0) > 0

				UNION ALL

				SELECT COALESCE(SUM(x.session_reward), 0) AS amount
				FROM (
					SELECT COALESCE(SUM(o.reward_amount), 0) AS session_reward
					FROM group_match_order o
					WHERE o.user_id = ?
					  AND o.is_winner = true
					GROUP BY o.session_id
					HAVING COALESCE(SUM(o.reward_amount), 0) > 0
				) x
			) t
		`, user.Id, user.Id, user.Id, user.Id, user.Id, user.Id, user.Id)

		withdrawnRewardUSDT := getDecimalBySQL(`
			SELECT COALESCE(SUM(amount), 0) AS total
			FROM cobo_withdraw_request
			WHERE user_id = ? AND symbol = 'USDT' AND status = 4
		`, user.Id)

		remainingRewardUSDT := getDecimalBySQL(`
			SELECT COALESCE(remaining_quota, 0) AS total
			FROM user_quota
			WHERE user_id = ?
		`, user.Id)

		stakeTotal := getDecimalBySQL(`
			SELECT COALESCE(SUM(amount), 0) AS total
			FROM staking_v2_order
			WHERE user_id = ?
		`, user.Id)

		nodePurchaseTotal := getDecimalBySQL(`
			SELECT COALESCE(SUM(amount), 0) AS total
			FROM cobo_node_purchase
			WHERE user_id = ? AND is_gift = 0
		`, user.Id)

		groupRechargeUSDT := getDecimalBySQL(`
			SELECT COALESCE(SUM(payment_amount::numeric), 0) AS total
			FROM apg_player
			WHERE user_id = ? AND UPPER(payment_token) = 'USDT'
		`, user.Id)

		ticketCount := getDecimalBySQL(`
			SELECT COALESCE(SUM(total_amount), 0) AS total
			FROM account_balance
			WHERE user_id = ? AND UPPER(symbol) IN ('TICKET', 'TICKETS')
		`, user.Id)

		giftTriple := getDecimalBySQL(`
			SELECT COALESCE(SUM(grant_amount::numeric), 0) AS total
			FROM cobo_node_token_grant
			WHERE user_id = ? AND token_type = 'Triple'
		`, user.Id)

		szpn := getDecimalBySQL(`
			SELECT COALESCE(SUM(total_amount), 0) AS total
			FROM account_balance
			WHERE user_id = ? AND UPPER(symbol) = 'SZPN'
		`, user.Id)

		ju := getDecimalBySQL(`
			SELECT COALESCE(SUM(total_amount), 0) AS total
			FROM account_balance
			WHERE user_id = ? AND UPPER(symbol) = 'JU'
		`, user.Id)

		yy := getDecimalBySQL(`
			SELECT COALESCE(SUM(total_amount), 0) AS total
			FROM account_balance
			WHERE user_id = ? AND UPPER(symbol) = 'YY'
		`, user.Id)

		yyai := getDecimalBySQL(`
			SELECT COALESCE(SUM(total_amount), 0) AS total
			FROM account_balance
			WHERE user_id = ? AND UPPER(symbol) = 'YYAI'
		`, user.Id)

		item := AdminUserListItem{
			Id:                  user.Id,
			WalletAddress:       user.WalletAddress,
			DepositAddress:      depositAddressMap[user.Id],
			ParentInviteCode:    user.ParentInviteCode,
			ParentWalletAddress: user.ParentWalletAddress,
			InviteCode:          user.InviteCode,
			Status:              user.Status,
			IsVertex:            user.IsVertex,
			IsServiceCenter:     user.IsServiceCenter,
			VipLevel:            effectiveLevel,
			CanWithdraw:         user.CanWithdraw,
			Leader:              leader,
			StakeRate:           stakeRate,
			TotalRewardUSDT:     totalRewardUSDT.StringFixed(2),
			WithdrawnRewardUSDT: withdrawnRewardUSDT.StringFixed(2),
			RemainingRewardUSDT: remainingRewardUSDT.StringFixed(2),
			StakeTotal:          stakeTotal.StringFixed(2),
			NodePurchaseTotal:   nodePurchaseTotal.StringFixed(2),
			GroupRechargeUSDT:   groupRechargeUSDT.StringFixed(2),
			TicketCount:         ticketCount.StringFixed(2),
			GiftTriple:          giftTriple.StringFixed(2),
			SZPN:                szpn.StringFixed(6),
			JU:                  ju.StringFixed(6),
			YY:                  yy.StringFixed(6),
			YYAI:                yyai.StringFixed(6),
			UseFullPerf:         user.UseFullPerf,
			NodeExempt:          user.NodeExempt,
			CreatedAt:           user.CreatedAt,
			UpdatedAt:           user.UpdatedAt,
		}
		if user.LastLoginAt != nil {
			item.LastLoginAt = *user.LastLoginAt
		}
		if adjustedLevel, ok := adjustedLevelMap[user.Id]; ok {
			level := adjustedLevel
			item.AdjustedVipLevel = &level
		}
		userList = append(userList, item)
	}

	return &GetUserListRes{
		Total:    repoRes.Total,
		Page:     repoRes.Page,
		PageSize: repoRes.PageSize,
		List:     userList,
		Summary: UserListSummary{
			TotalRewardUSDT:     repoRes.Summary.TotalRewardUSDT,
			WithdrawnRewardUSDT: repoRes.Summary.WithdrawnRewardUSDT,
			RemainingRewardUSDT: repoRes.Summary.RemainingRewardUSDT,
			StakeTotal:          repoRes.Summary.StakeTotal,
			NodePurchaseTotal:   repoRes.Summary.NodePurchaseTotal,
			GroupRechargeUSDT:   repoRes.Summary.GroupRechargeUSDT,
			TicketCount:         repoRes.Summary.TicketCount,
			GiftTriple:          repoRes.Summary.GiftTriple,
			SZPN:                repoRes.Summary.SZPN,
			JU:                  repoRes.Summary.JU,
			YY:                  repoRes.Summary.YY,
			YYAI:                repoRes.Summary.YYAI,
		},
	}, nil
}

// GetPerformanceStats 业绩统计（主数据来自 cobo_performance 缓存表）
func (s *userService) GetPerformanceStats(ctx context.Context, req *GetPerformanceStatsReq) (*GetPerformanceStatsRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	parentWalletAddress := strings.ToLower(req.ParentWalletAddress)

	// 构建基础查询：user_info 为主表，LEFT JOIN cobo_performance
	dbModel := g.DB().Model("user_info u").
		LeftJoin("cobo_performance cp", "cp.user_id = u.id")

	if walletAddress != "" {
		dbModel = dbModel.Where("LOWER(u.wallet_address) = ?", walletAddress)
	}
	if parentWalletAddress != "" {
		dbModel = dbModel.Where("LOWER(u.parent_wallet_address) = ?", parentWalletAddress)
	}

	// 查总数
	total, err := dbModel.Clone().Count()
	if err != nil {
		return nil, fmt.Errorf("查询总数失败: %v", err)
	}
	if total == 0 {
		return &GetPerformanceStatsRes{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    0,
			Pages:    0,
			List:     []PerformanceStatsItem{},
		}, nil
	}

	// 分页查询
	var rows []struct {
		UserID              int64           `json:"id"`
		WalletAddress       string          `json:"wallet_address"`
		ParentWalletAddress string          `json:"parent_wallet_address"`
		InviteCode          string          `json:"invite_code"`
		TeamPerformance     decimal.Decimal `json:"team_performance"`
		TeamTotalCount      int             `json:"team_total_count"`
		DirectCount         int             `json:"direct_count"`
	}
	offset := (req.Page - 1) * req.PageSize
	err = dbModel.Fields("u.id, u.wallet_address, u.parent_wallet_address, u.invite_code, COALESCE(cp.team_performance, 0) as team_performance, COALESCE(cp.team_total_count, 0) as team_total_count, COALESCE(cp.direct_count, 0) as direct_count").
		Order("COALESCE(cp.team_performance, 0) DESC, u.id DESC").
		Limit(req.PageSize).Offset(offset).
		Scan(&rows)
	if err != nil {
		return nil, fmt.Errorf("查询业绩数据失败: %v", err)
	}

	// 并发计算大区/小区（cobo_performance 未缓存该维度）
	type districtInfo struct {
		maxDistrictCount       int
		minDistrictCount       int
		minDistrictPerformance int
	}
	districtMap := make(map[int]districtInfo, len(rows))

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	var mu sync.Mutex

	for i, row := range rows {
		wg.Add(1)
		go func(idx int, inviteCode string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			agg, err := s.teamStatsSvc.GetOverviewAggByInviteCode(ctx, inviteCode)
			if err != nil {
				g.Log().Warningf(ctx, "[GetPerformanceStats] 获取大区小区数据失败: inviteCode=%s, err=%v", inviteCode, err)
				return
			}

			mu.Lock()
			districtMap[idx] = districtInfo{
				maxDistrictCount:       agg.BigTeamCount,
				minDistrictCount:       agg.SmallTeamSum,
				minDistrictPerformance: int(agg.SmallTeamPerformance.IntPart()),
			}
			mu.Unlock()
		}(i, row.InviteCode)
	}
	wg.Wait()

	// 组装结果
	var itemList []PerformanceStatsItem
	for i, row := range rows {
		di, ok := districtMap[i]
		maxDistrictCount := 0
		minDistrictCount := 0
		minDistrictPerformance := 0
		if ok {
			maxDistrictCount = di.maxDistrictCount
			minDistrictCount = di.minDistrictCount
			minDistrictPerformance = di.minDistrictPerformance
		}

		item := PerformanceStatsItem{
			WalletAddress:          row.WalletAddress,
			ParentWalletAddress:    row.ParentWalletAddress,
			TeamTotalPerformance:   int(row.TeamPerformance.IntPart()),
			TeamMemberCount:        row.TeamTotalCount,
			DirectMemberCount:      row.DirectCount,
			MaxDistrictCount:       maxDistrictCount,
			MinDistrictCount:       minDistrictCount,
			MinDistrictPerformance: minDistrictPerformance,
		}
		itemList = append(itemList, item)
	}

	pages := (total + req.PageSize - 1) / req.PageSize
	if pages == 0 {
		pages = 1
	}

	return &GetPerformanceStatsRes{
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
		Pages:    pages,
		List:     itemList,
	}, nil
}

// GetPerformanceHistory 历史业绩
func (s *userService) GetPerformanceHistory(ctx context.Context, req *GetPerformanceHistoryReq) (*GetPerformanceHistoryRes, error) {
	// 1. 转换钱包地址为小写（用于过滤）
	walletAddress := strings.ToLower(req.WalletAddress)

	// 2. 构建筛选用的 userIDs
	var filterUserIDs []int64

	// 2.1 按用户地址精确匹配
	if walletAddress != "" {
		userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
		if err != nil {
			return &GetPerformanceHistoryRes{
				Page:     req.Page,
				PageSize: req.PageSize,
				Total:    0,
				Pages:    0,
				List:     []PerformanceHistoryItem{},
			}, nil
		}
		if userEntity == nil {
			return &GetPerformanceHistoryRes{Page: req.Page, PageSize: req.PageSize, Total: 0, Pages: 0, List: []PerformanceHistoryItem{}}, nil
		}
		filterUserIDs = append(filterUserIDs, userEntity.Id)
	}

	// 3. 获取业绩记录（按日期范围和user_ids过滤，分页）
	repoRes, err := s.perfRepo.GetByDateRange(ctx, &rewardRepo.GetPerformanceHistoryPageReq{
		PageReq: model.PageReq{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		Date:    req.Date,
		UserIDs: filterUserIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("获取用户业绩失败: %v", err)
	}

	if repoRes == nil || len(repoRes.List) == 0 {
		page := req.Page
		if repoRes != nil && repoRes.Page > 0 {
			page = repoRes.Page
		}
		pageSize := req.PageSize
		if repoRes != nil && repoRes.PageSize > 0 {
			pageSize = repoRes.PageSize
		}
		total := 0
		pages := 0
		if repoRes != nil {
			total = repoRes.Total
			pages = repoRes.Pages
		}
		return &GetPerformanceHistoryRes{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
			Pages:    pages,
			List:     []PerformanceHistoryItem{},
		}, nil
	}

	performances := repoRes.List

	// 4. 提取所有用户ID
	userIDs := make([]int64, 0, len(performances))
	userRecords := make([]rewardRepo.UserRecordTime, 0, len(performances))
	for _, perf := range performances {
		userIDs = append(userIDs, perf.UserID)
		userRecords = append(userRecords, rewardRepo.UserRecordTime{
			UserID:     perf.UserID,
			RecordTime: perf.RecordTime,
		})
	}

	// 5. 批量获取用户信息
	users, err := s.userRepo.GetDataByIds(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", err)
	}

	// 构建用户映射
	userMap := make(map[int64]*entity.UserEntity)
	for _, user := range users {
		userMap[user.Id] = user
	}

	// 批量获取用户当前VIP等级
	vipLevelMap := make(map[int64]*rewardEntity.UserVipLevelEntity)
	if len(userIDs) > 0 {
		vipLevels, err := s.vipLevelRepo.GetCurrentVipLevelsByUserIDs(ctx, userIDs)
		if err != nil {
			return nil, fmt.Errorf("获取用户VIP等级失败: %v", err)
		}
		for userID, vip := range vipLevels {
			if vip == nil {
				continue
			}
			vipLevelMap[userID] = vip
		}
	}

	// 6. 批量查询当日释放（静态释放）
	dailyReleaseMap := make(map[string]decimal.Decimal)
	if len(userIDs) > 0 {
		releaseMap, err := s.assetRecordRepo.GetBatchUserSumByBusinessAndDateRange(ctx, userRecords, consts.AssetBusinessTypeRewardStatic)
		if err != nil {
			return nil, fmt.Errorf("查询当日释放失败: %v", err)
		}
		dailyReleaseMap = releaseMap
	}

	// 7. 组装数据列表
	var itemList []PerformanceHistoryItem
	for _, perf := range performances {
		user, ok := userMap[perf.UserID]
		if !ok {
			continue
		}

		// 根据请求参数过滤
		if req.WalletAddress != "" && strings.ToLower(user.WalletAddress) != walletAddress {
			continue
		}

		key := fmt.Sprintf("%d:%s", perf.UserID, perf.RecordTime.Format("2006-01-02 15:04:05"))
		dailyRelease := decimal.Zero
		if release, ok := dailyReleaseMap[key]; ok {
			dailyRelease = release
		}

		// 从metadata中读取当时实际生效的VIP等级
		vipLevel := perf.VipLevel // 默认使用计算等级
		if perf.Metadata != "" {
			var metadata rewardEntity.PerformanceMetadata
			if err := json.Unmarshal([]byte(perf.Metadata), &metadata); err == nil {
				if metadata.EffectiveVipLevel > 0 {
					vipLevel = metadata.EffectiveVipLevel
				}
			} else {
				g.Log().Warningf(ctx, "解析业绩记录metadata失败: userID=%d, err=%v", perf.UserID, err)
			}
		}

		item := PerformanceHistoryItem{
			WalletAddress:                  user.WalletAddress,
			TeamMemberCount:                perf.TeamMemberCount,
			PersonalPerformance:            perf.PersonalPerformance.String(),
			TeamPerformance:                perf.TeamPerformance.String(),
			TeamCombinationPerformance:     perf.TeamCombinationPerformance.String(),
			PersonalCombinationPerformance: perf.PersonalCombinationPerformance.String(),
			DistrictPerformance:            perf.DistrictPerformance.String(),
			VipLevel:                       vipLevel,
			DailyRelease:                   dailyRelease.String(),
			RecordTime:                     perf.RecordTime.Format(consts.TimeFormatDateTime),
		}

		itemList = append(itemList, item)
	}

	// 8. 分页处理
	return &GetPerformanceHistoryRes{
		Page:     repoRes.Page,
		PageSize: repoRes.PageSize,
		Total:    repoRes.Total,
		Pages:    repoRes.Pages,
		List:     itemList,
	}, nil
}

// GetYesterdayWeightedRanking 昨日直推排行
func (s *userService) GetYesterdayWeightedRanking(ctx context.Context) (*GetYesterdayWeightedRankingRes, error) {
	// 1. 获取最新记录日期
	latestRecordTime, err := s.assetRecordRepo.GetLatestRecordDate(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取最新记录日期失败: %v", err)
	}

	if latestRecordTime.IsZero() {
		return &GetYesterdayWeightedRankingRes{
			Total: YesterdayWeightedRankingTotal{
				WeightedSum: "0",
				TotalReward: "0",
			},
			List: []YesterdayWeightedRankingItem{},
		}, nil
	}

	latestRecordDateStr := latestRecordTime.Format("2006-01-02 15:04:05")

	// 2. 查询昨日加权奖励排行榜（前50）
	records, err := s.assetRecordRepo.GetYesterdayWeightedRewardRanking(ctx, latestRecordDateStr, 50)
	if err != nil {
		return nil, fmt.Errorf("查询昨日加权奖励排行榜失败: %v", err)
	}

	if len(records) == 0 {
		return &GetYesterdayWeightedRankingRes{
			Total: YesterdayWeightedRankingTotal{
				WeightedSum: "0",
				TotalReward: "0",
			},
			List: []YesterdayWeightedRankingItem{},
		}, nil
	}

	// 3. 提取所有用户ID
	userIDs := make([]int64, 0, len(records))
	for _, record := range records {
		userIDs = append(userIDs, record.UserID)
	}

	// 4. 批量查询用户信息
	users, err := s.userRepo.GetDataByIds(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", err)
	}

	// 构建用户映射
	userMap := make(map[int64]*entity.UserEntity)
	for _, user := range users {
		userMap[user.Id] = user
	}

	// 5. 计算总奖励和加权总和
	totalReward := decimal.Zero
	totalWeightedSum := decimal.Zero // 加权总和（所有用户的 new_direct_performance 总和）

	// 6. 解析 metadata 并组装列表数据
	var itemList []YesterdayWeightedRankingItem
	for _, record := range records {
		user, ok := userMap[record.UserID]
		if !ok {
			continue
		}

		// 解析 metadata 获取 new_direct_performance
		newDirectPerformance := decimal.Zero
		if record.Metadata != "" {
			var metadata rewardEntity.WeightedRewardMetadata
			if err := json.Unmarshal([]byte(record.Metadata), &metadata); err == nil {
				newDirectPerformance = metadata.NewDirectPerformance
			}
		}

		// 累计总奖励
		totalReward = totalReward.Add(record.Amount)
		// 累计加权总和（new_direct_performance总和）
		totalWeightedSum = totalWeightedSum.Add(newDirectPerformance)

		itemList = append(itemList, YesterdayWeightedRankingItem{
			WalletAddress: user.WalletAddress,
			NewAmount:     newDirectPerformance.String(),
			RewardAmount:  record.Amount.String(),
		})
	}

	// 7. 计算占比（每个用户的奖励金额 / 总奖励 * 100）
	for i := range itemList {
		if totalReward.GreaterThan(decimal.Zero) {
			itemReward, _ := decimal.NewFromString(itemList[i].RewardAmount)
			percentage := itemReward.DivRound(totalReward, consts.TokenPrecision).Mul(decimal.NewFromInt(100))
			itemList[i].Percentage = utils.FormatDecimal(percentage) + "%"
		} else {
			itemList[i].Percentage = "0.00%"
		}
	}

	return &GetYesterdayWeightedRankingRes{
		Total: YesterdayWeightedRankingTotal{
			WeightedSum: totalWeightedSum.String(),
			TotalReward: totalReward.String(),
		},
		List: itemList,
	}, nil
}

// getBusinessTypeName 获取业务类型的中文名称
func getBusinessTypeName(businessType string) string {
	typeMap := map[string]string{
		consts.AssetBusinessTypeRewardStatic:        "静态释放",
		consts.AssetBusinessTypeRewardNode:          "节点分红",
		consts.AssetBusinessTypeRewardReferral:      "层级奖",
		consts.AssetBusinessTypeRewardDistrict:      "VIP奖励",
		consts.AssetBusinessTypeRewardWeighted:      "直推排行奖",
		consts.AssetBusinessTypeRewardReduced:       "削减扣除",
		consts.AssetBusinessTypeRewardServiceCenter: "服务中心奖励",
	}
	if name, ok := typeMap[businessType]; ok {
		return name
	}
	return businessType
}

// getSettlementStatus 获取结算状态的中文说明
func getSettlementStatus(status int) string {
	switch status {
	case consts.AssetRecordStatusPending:
		return "待结算"
	case consts.AssetRecordStatusSettled:
		return "已结算"
	default:
		return "未知"
	}
}

// GetReleaseUSDTDetails 查询释放USDT明细信息
func (s *userService) GetReleaseUSDTDetails(ctx context.Context, req *GetReleaseUSDTDetailsReq) (*GetReleaseUSDTDetailsRes, error) {
	// 1. 处理用户地址（转小写并查询user_id）
	var userID int64
	if req.WalletAddress != "" {
		walletAddress := strings.ToLower(req.WalletAddress)
		userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
		if err != nil {
			return &GetReleaseUSDTDetailsRes{
				Page:     req.Page,
				PageSize: req.PageSize,
				Total:    0,
				Pages:    0,
				List:     []ReleaseUSDTDetailsItem{},
			}, nil
		}
		if userEntity == nil {
			return &GetReleaseUSDTDetailsRes{
				Page:     req.Page,
				PageSize: req.PageSize,
				Total:    0,
				Pages:    0,
				List:     []ReleaseUSDTDetailsItem{},
			}, nil
		}
		userID = userEntity.Id
	}

	// 2. 定义需要查询的业务类型
	businessTypes := []string{
		consts.AssetBusinessTypeRewardStatic,
		consts.AssetBusinessTypeRewardNode,
		consts.AssetBusinessTypeRewardServiceCenter,
		consts.AssetBusinessTypeRewardReferral,
		consts.AssetBusinessTypeRewardDistrict,
		consts.AssetBusinessTypeRewardWeighted,
		consts.AssetBusinessTypeRewardReduced,
	}

	selectedBusinessType := strings.TrimSpace(req.BusinessType)
	if selectedBusinessType != "" {
		businessTypes = []string{selectedBusinessType}
	}

	// 3. 查询 asset_record 数据
	records, total, err := s.assetRecordRepo.GetReleaseUSDTDetails(ctx, userID, req.Date, businessTypes, selectedBusinessType, req.Page, req.PageSize)
	if err != nil {
		return nil, fmt.Errorf("查询释放USDT明细失败: %v", err)
	}

	if len(records) == 0 {
		return &GetReleaseUSDTDetailsRes{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    0,
			Pages:    0,
			List:     []ReleaseUSDTDetailsItem{},
		}, nil
	}

	// 4. 提取所有用户ID
	userIDs := make([]int64, 0, len(records))
	for _, record := range records {
		userIDs = append(userIDs, record.UserID)
	}

	// 5. 批量查询用户信息
	users, err := s.userRepo.GetDataByIds(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", err)
	}

	// 构建用户映射
	userMap := make(map[int64]*entity.UserEntity)
	for _, user := range users {
		userMap[user.Id] = user
	}

	// 6. 组装返回数据
	var itemList []ReleaseUSDTDetailsItem
	for _, record := range records {
		user, ok := userMap[record.UserID]
		if !ok {
			continue
		}

		item := ReleaseUSDTDetailsItem{
			Id:               record.Id,
			WalletAddress:    user.WalletAddress,
			BusinessType:     record.BusinessType,
			BusinessTypeDesc: getBusinessTypeName(record.BusinessType),
			ReleaseTime:      record.RecordTime.Format("2006-01-02 15:04:05"),
			ReleaseAmount:    record.Amount.String(),
			IsSettled:        getSettlementStatus(record.Status),
		}

		itemList = append(itemList, item)
	}

	return &GetReleaseUSDTDetailsRes{
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
		Pages:    int(math.Ceil(float64(total) / float64(req.PageSize))),
		List:     itemList,
	}, nil
}

// GetReleaseUSDTDetail 获取USDT明细详情
func (s *userService) GetReleaseUSDTDetail(ctx context.Context, req *GetReleaseUSDTDetailReq) (*GetReleaseUSDTDetailRes, error) {
	// 1. 根据ID查询asset_record表数据
	record, err := s.assetRecordRepo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("查询资金记录失败: %v", err)
	}
	if record == nil {
		return nil, errors.New("记录不存在")
	}

	// 2. 根据user_id查询用户信息
	userEntity, err := s.userRepo.GetUserById(ctx, record.UserID)
	if err != nil {
		return nil, fmt.Errorf("查询用户信息失败: %v", err)
	}
	if userEntity == nil {
		return nil, errors.New("用户不存在")
	}

	// 3. 增强Metadata信息（根据业务类型添加额外字段）
	enrichedMetadata, err := s.enrichMetadataWithUserInfo(ctx, record.BusinessType, record.Metadata)
	if err != nil {
		// 如果增强失败，使用原始Metadata，记录警告日志
		g.Log().Warningf(ctx, "增强Metadata信息失败: %v，使用原始Metadata", err)
		enrichedMetadata = record.Metadata
	}

	// 4. 组装返回数据
	res := &GetReleaseUSDTDetailRes{
		Id:               record.Id,
		WalletAddress:    userEntity.WalletAddress,
		BusinessType:     record.BusinessType,
		BusinessTypeDesc: getBusinessTypeName(record.BusinessType),
		ReleaseTime:      record.RecordTime.Format("2006-01-02 15:04:05"),
		ReleaseAmount:    record.Amount.String(),
		IsSettled:        getSettlementStatus(record.Status),
		Metadata:         enrichedMetadata,
	}

	return res, nil
}

// GetNodeInfoList 分页显示节点信息
func (s *userService) GetNodeInfoList(ctx context.Context, req *GetNodeInfoListReq) (*GetNodeInfoListRes, error) {
	// 设置默认分页参数
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	var userID int64

	// 如果提供了 wallet_address，先查询用户信息
	if req.WalletAddress != "" {
		walletAddress := strings.ToLower(req.WalletAddress)
		userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
		if err != nil {
			// 如果查询出错，返回空数据
			return &GetNodeInfoListRes{
				Page:     req.Page,
				PageSize: req.PageSize,
				Total:    0,
				Pages:    0,
				List:     []NodeInfoItem{},
			}, nil
		}

		// 如果用户不存在，返回空数据
		if userEntity == nil {
			return &GetNodeInfoListRes{
				Page:     req.Page,
				PageSize: req.PageSize,
				Total:    0,
				Pages:    0,
				List:     []NodeInfoItem{},
			}, nil
		}

		userID = userEntity.Id
	}

	// 构建查询请求（不指定状态，查询所有状态的质押包，但只查询 stake_type in (2, 4) 的数据）
	repoReq := &rewardRepo.GetStakingPackageListReq{
		PageReq: model.PageReq{
			Page:     req.Page,
			PageSize: req.PageSize,
		},
		UserID:     userID,
		Status:     0,           // 0 表示不限制状态，查询所有
		StakeTypes: []int{2, 4}, // 只查询质押类型为 2 和 4 的数据
	}

	// 调用仓储层查询
	repoRes, err := s.stakingPackageRepo.GetPagedList(ctx, repoReq)
	if err != nil {
		return nil, fmt.Errorf("查询节点信息失败: %v", err)
	}

	// 计算总页数
	pages := 0
	if repoRes.Total > 0 && req.PageSize > 0 {
		pages = int(math.Ceil(float64(repoRes.Total) / float64(req.PageSize)))
	}

	// 提取所有用户ID
	userIDs := make([]int64, 0, len(repoRes.List))
	for _, pkg := range repoRes.List {
		userIDs = append(userIDs, pkg.UserID)
	}

	// 批量查询用户信息
	userMap := make(map[int64]*entity.UserEntity)
	if len(userIDs) > 0 {
		users, err := s.userRepo.GetDataByIds(ctx, userIDs)
		if err != nil {
			return nil, fmt.Errorf("获取用户信息失败: %v", err)
		}
		for _, user := range users {
			userMap[user.Id] = user
		}
	}

	// 转换为响应格式
	var itemList []NodeInfoItem
	for _, pkg := range repoRes.List {
		walletAddress := ""
		if user, ok := userMap[pkg.UserID]; ok {
			walletAddress = user.WalletAddress
		}

		item := NodeInfoItem{
			WalletAddress:   walletAddress,
			StakeAmount:     pkg.StakeAmount.StringFixed(2),
			PowerValue:      pkg.PowerValue.StringFixed(2),
			PowerMultiplier: pkg.PowerMultiplier.StringFixed(2),
			TotalQuota:      pkg.TotalQuota.StringFixed(2),
			StakeType:       pkg.StakeType,
			StakeTypeDesc:   getStakeTypeDesc(pkg.StakeType),
		}
		itemList = append(itemList, item)
	}

	return &GetNodeInfoListRes{
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    repoRes.Total,
		Pages:    pages,
		List:     itemList,
	}, nil
}

func getStakeTypeDesc(stakeType int) string {
	switch stakeType {
	case consts.StakeTypeLPStaking:
		return "LP质押"
	case consts.StakeTypeNodePurchase:
		return "购买节点"
	case consts.StakeTypeExternalImport:
		return "导入质押"
	case consts.StakeTypeExternalNode:
		return "导入节点"
	case consts.StakeTypeCombinationStaking:
		return "组合质押"
	case consts.StakeTypeMintGame:
		return "Mint游戏质押"
	default:
		return "未知"
	}
}

// getNodeTypeDesc 节点类型中文描述
func getNodeTypeDesc(nodeType int) string {
	switch nodeType {
	case 1:
		return "节点1 (1000 USDT)"
	case 2:
		return "节点2 (5000 USDT)"
	case 3:
		return "节点3 (10000 USDT)"
	case 4:
		return "节点4 (30000 USDT)"
	case 5:
		return "节点5 (100000 USDT)"
	default:
		return fmt.Sprintf("节点%d", nodeType)
	}
}

// getCoboNodeStatusDesc cobo_node_purchase 状态中文描述
func getCoboNodeStatusDesc(status int) string {
	switch status {
	case coboEntity.NodeStatusRunning:
		return "运行中"
	case coboEntity.NodeStatusStaticExpired:
		return "静态完成"
	case coboEntity.NodeStatusQuotaExpired:
		return "额度耗尽"
	case coboEntity.NodeStatusForceExpired:
		return "强制出局"
	default:
		return "未知"
	}
}

// getWithdrawStatusDesc 提现状态中文
func getWithdrawStatusDesc(status string) string {
	switch status {
	case consts.WithdrawStatusPendingSignature:
		return "待签名"
	case consts.WithdrawStatusProcessing:
		return "处理中"
	case consts.WithdrawStatusPendingCompletion:
		return "待完成"
	case consts.WithdrawStatusSuccess:
		return "成功"
	case consts.WithdrawStatusFailed:
		return "失败"
	case consts.WithdrawStatusTimeout:
		return "超时"
	case consts.WithdrawStatusCancelled:
		return "已取消"
	default:
		return status
	}
}

func parseCoboWithdrawStatusFilter(status string) []int {
	if status == "" {
		return nil
	}

	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "pending_audit", "pending":
		return []int{0}
	case "approved":
		return []int{1}
	case "cancelled", "rejected":
		return []int{2}
	case "processing", "pending_signature", "pending_completion":
		return []int{3}
	case "success", "completed":
		return []int{4}
	case "failed":
		return []int{5}
	}

	if v, err := strconv.Atoi(s); err == nil && v >= 0 && v <= 5 {
		return []int{v}
	}

	return nil
}

func getCoboWithdrawStatus(status int) (string, string) {
	switch status {
	case 0:
		return "pending_audit", "待审核"
	case 1:
		return "approved", "已审核"
	case 2:
		return "cancelled", "已取消"
	case 3:
		return "processing", "处理中"
	case 4:
		return "success", "成功"
	case 5:
		return "failed", "失败"
	default:
		return strconv.Itoa(status), "未知"
	}
}

func parseCoboRechargeStatusFilter(status string) []int {
	if status == "" {
		return nil
	}

	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "pending":
		return []int{0}
	case "success", "confirmed":
		return []int{1}
	case "failed", "cancelled":
		return []int{2}
	}

	if v, err := strconv.Atoi(s); err == nil && v >= 0 && v <= 2 {
		return []int{v}
	}

	return nil
}

func getCoboRechargeStatus(status int) (string, string) {
	switch status {
	case 0:
		return "pending", "待确认"
	case 1:
		return "success", "成功"
	case 2:
		return "failed", "失败"
	default:
		return strconv.Itoa(status), "未知"
	}
}

func normalizeCoboSymbol(symbol string) (string, bool) {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if s == "" {
		return "", true
	}
	_, ok := coboSupportedSymbols[s]
	return s, ok
}

func (s *userService) getCoboWithdrawRecords(ctx context.Context, req *GetWithdrawRecordsReq, userID int64) (*GetWithdrawRecordsRes, error) {
	type coboWithdrawRecord struct {
		Id                 int64       `json:"id"`
		UserId             int64       `json:"user_id"`
		Symbol             string      `json:"symbol"`
		Amount             string      `json:"amount"`
		TaxAmount          string      `json:"tax_amount"`
		TaxDeductionAmount string      `json:"tax_deduction_amount"`
		TaxYyaiUsdtAmount  string      `json:"tax_yyai_usdt_amount"`
		TaxYyaiAmount      string      `json:"tax_yyai_amount"`
		ActualAmount       string      `json:"actual_amount"`
		TxHash             string      `json:"tx_hash"`
		Status             int         `json:"status"`
		CreatedAt          *gtime.Time `json:"created_at"`
	}

	query := g.DB().Model("cobo_withdraw_request w").Ctx(ctx)
	if userID > 0 {
		query = query.Where("w.user_id", userID)
	}
	if req.MinAmount != "" {
		query = query.Where("w.amount >= ?", req.MinAmount)
	}
	if req.MaxAmount != "" {
		query = query.Where("w.amount <= ?", req.MaxAmount)
	}
	symbol, ok := normalizeCoboSymbol(req.Symbol)
	if !ok {
		return nil, fmt.Errorf("不支持的symbol: %s，仅支持 USDT/JU/ZPN/YY", req.Symbol)
	}
	if symbol != "" {
		query = query.Where("w.symbol", symbol)
	}
	if req.TaxOnly {
		query = query.WhereExists(g.DB().Model("us_stock_reward_pool p").Where("p.withdraw_id = w.id"))
	}
	statuses := parseCoboWithdrawStatusFilter(req.Status)
	if req.Status != "" && len(statuses) == 0 {
		return &GetWithdrawRecordsRes{Page: req.Page, PageSize: req.PageSize, Total: 0, Pages: 0, List: []WithdrawRecordItem{}}, nil
	}
	if len(statuses) > 0 {
		query = query.WhereIn("w.status", statuses)
	}

	total, err := query.Count()
	if err != nil {
		return nil, fmt.Errorf("查询Cobo提现记录总数失败: %v", err)
	}
	if total == 0 {
		return &GetWithdrawRecordsRes{Page: req.Page, PageSize: req.PageSize, Total: 0, Pages: 0, List: []WithdrawRecordItem{}}, nil
	}

	var records []coboWithdrawRecord
	err = query.Order("w.created_at DESC").Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize).Scan(&records)
	if err != nil {
		return nil, fmt.Errorf("查询Cobo提现记录失败: %v", err)
	}

	userIDs := make([]int64, 0, len(records))
	for _, rec := range records {
		userIDs = append(userIDs, rec.UserId)
	}
	users, err := s.userRepo.GetDataByIds(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", err)
	}
	userMap := make(map[int64]*entity.UserEntity, len(users))
	for _, u := range users {
		userMap[u.Id] = u
	}

	items := make([]WithdrawRecordItem, 0, len(records))
	for _, rec := range records {
		wallet := ""
		if u, ok := userMap[rec.UserId]; ok {
			wallet = u.WalletAddress
		}
		statusKey, statusDesc := getCoboWithdrawStatus(rec.Status)
		items = append(items, WithdrawRecordItem{
			Id:                 rec.Id,
			WalletAddress:      wallet,
			Symbol:             rec.Symbol,
			Amount:             rec.Amount,
			TaxAmount:          rec.TaxAmount,
			TaxDeductionAmount: rec.TaxDeductionAmount,
			TaxYyaiUsdtAmount:  rec.TaxYyaiUsdtAmount,
			TaxYyaiAmount:      rec.TaxYyaiAmount,
			ActualAmount:       rec.ActualAmount,
			USDTAmount:         rec.Amount,
			APGAmount:          "",
			APGPrice:           "",
			TxHash:             rec.TxHash,
			Status:             statusKey,
			StatusDesc:         statusDesc,
			WithdrawTime:       rec.CreatedAt,
		})
	}

	pages := int(math.Ceil(float64(total) / float64(req.PageSize)))
	return &GetWithdrawRecordsRes{
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
		Pages:    pages,
		List:     items,
	}, nil
}

// GetWithdrawRecords 查询提现记录
func (s *userService) GetWithdrawRecords(ctx context.Context, req *GetWithdrawRecordsReq) (*GetWithdrawRecordsRes, error) {
	// 1. 可选钱包地址过滤
	var userID int64
	if req.WalletAddress != "" {
		addr := strings.ToLower(req.WalletAddress)
		user, err := s.userRepo.GetUserByWalletAddress(ctx, addr)
		if err != nil || user == nil {
			return &GetWithdrawRecordsRes{Page: req.Page, PageSize: req.PageSize, Total: 0, Pages: 0, List: []WithdrawRecordItem{}}, nil
		}
		userID = user.Id
	}

	return s.getCoboWithdrawRecords(ctx, req, userID)
}

// GetRechargeRecords 查询充值记录
func (s *userService) GetRechargeRecords(ctx context.Context, req *GetRechargeRecordsReq) (*GetRechargeRecordsRes, error) {
	var userID int64
	if req.WalletAddress != "" {
		addr := strings.ToLower(req.WalletAddress)
		user, err := s.userRepo.GetUserByWalletAddress(ctx, addr)
		if err != nil || user == nil {
			return &GetRechargeRecordsRes{Page: req.Page, PageSize: req.PageSize, Total: 0, Pages: 0, List: []RechargeRecordItem{}}, nil
		}
		userID = user.Id
	}

	type coboRechargeRecord struct {
		Id            int64       `json:"id"`
		UserId        int64       `json:"user_id"`
		Symbol        string      `json:"symbol"`
		Amount        string      `json:"amount"`
		TxHash        string      `json:"tx_hash"`
		Status        int         `json:"status"`
		Confirmations int         `json:"confirmations"`
		CreatedAt     *gtime.Time `json:"created_at"`
	}

	query := g.DB().Model("cobo_recharge_record").Ctx(ctx)
	if userID > 0 {
		query = query.Where("user_id", userID)
	}
	if req.TxHash != "" {
		pattern := utils.BuildLikePattern(strings.ToLower(req.TxHash))
		if pattern != "" {
			query = query.Where("LOWER(tx_hash) LIKE ?", pattern)
		}
	}
	if req.MinAmount != "" {
		query = query.Where("amount >= ?", req.MinAmount)
	}
	if req.MaxAmount != "" {
		query = query.Where("amount <= ?", req.MaxAmount)
	}
	symbol, ok := normalizeCoboSymbol(req.Symbol)
	if !ok {
		return nil, fmt.Errorf("不支持的symbol: %s，仅支持 USDT/JU/ZPN/YY", req.Symbol)
	}
	if symbol != "" {
		query = query.Where("symbol", symbol)
	}
	statuses := parseCoboRechargeStatusFilter(req.Status)
	if req.Status != "" && len(statuses) == 0 {
		return &GetRechargeRecordsRes{Page: req.Page, PageSize: req.PageSize, Total: 0, Pages: 0, List: []RechargeRecordItem{}}, nil
	}
	if len(statuses) > 0 {
		query = query.WhereIn("status", statuses)
	}

	total, err := query.Count()
	if err != nil {
		return nil, fmt.Errorf("查询充值记录总数失败: %v", err)
	}
	if total == 0 {
		return &GetRechargeRecordsRes{Page: req.Page, PageSize: req.PageSize, Total: 0, Pages: 0, List: []RechargeRecordItem{}}, nil
	}

	var records []coboRechargeRecord
	err = query.Order("created_at DESC").Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize).Scan(&records)
	if err != nil {
		return nil, fmt.Errorf("查询充值记录失败: %v", err)
	}

	userIDs := make([]int64, 0, len(records))
	for _, rec := range records {
		userIDs = append(userIDs, rec.UserId)
	}
	users, err := s.userRepo.GetDataByIds(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", err)
	}
	userMap := make(map[int64]*entity.UserEntity, len(users))
	for _, u := range users {
		userMap[u.Id] = u
	}

	items := make([]RechargeRecordItem, 0, len(records))
	for _, rec := range records {
		wallet := ""
		if u, ok := userMap[rec.UserId]; ok {
			wallet = u.WalletAddress
		}
		statusKey, statusDesc := getCoboRechargeStatus(rec.Status)
		items = append(items, RechargeRecordItem{
			Id:            rec.Id,
			WalletAddress: wallet,
			Symbol:        rec.Symbol,
			Amount:        rec.Amount,
			TxHash:        rec.TxHash,
			Status:        statusKey,
			StatusDesc:    statusDesc,
			Confirmations: rec.Confirmations,
			RechargeTime:  rec.CreatedAt,
		})
	}

	pages := int(math.Ceil(float64(total) / float64(req.PageSize)))
	return &GetRechargeRecordsRes{
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
		Pages:    pages,
		List:     items,
	}, nil
}

// SetServiceCenter 设置服务中心
func (s *userService) SetServiceCenter(ctx context.Context, req *SetServiceCenterReq) (*SetServiceCenterRes, error) {
	// 校验用户ID
	if req.Id <= 0 {
		return nil, fmt.Errorf("用户ID不能为空")
	}

	// 查询用户是否存在
	userEntity, _ := s.userRepo.GetUserById(ctx, req.Id)
	if userEntity == nil || userEntity.Id == 0 {
		return nil, fmt.Errorf("用户不存在")
	}

	// 判断用户是否已经是服务中心
	if userEntity.IsServiceCenter {
		return nil, fmt.Errorf("用户已经是服务中心")
	}

	// 更新用户为服务中心
	err := s.userRepo.UpdateUser(ctx, req.Id, map[string]interface{}{
		"is_service_center": true,
	})
	if err != nil {
		return nil, fmt.Errorf("设置服务中心失败: %v", err)
	}

	return &SetServiceCenterRes{
		IsServiceCenter: true,
	}, nil
}

// GetServiceCenterUsers 获取服务中心用户列表
func (s *userService) GetServiceCenterUsers(ctx context.Context) (*GetServiceCenterUsersRes, error) {
	// 查询服务中心用户列表
	users, err := s.userRepo.GetServiceCenterUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询服务中心用户列表失败: %v", err)
	}

	// 组装返回数据
	var list []ServiceCenterUserItem
	for _, user := range users {
		list = append(list, ServiceCenterUserItem{
			Id:                user.Id,
			WalletAddress:     user.WalletAddress,
			ServiceCenterRate: user.ServiceCenterRate.String(),
		})
	}

	return &GetServiceCenterUsersRes{
		List: list,
	}, nil
}

// CancelServiceCenter 取消服务中心
func (s *userService) CancelServiceCenter(ctx context.Context, req *CancelServiceCenterReq) (*CancelServiceCenterRes, error) {
	// 校验用户ID
	if req.Id <= 0 {
		return nil, fmt.Errorf("用户ID不能为空")
	}

	// 查询用户是否存在
	userEntity, _ := s.userRepo.GetUserById(ctx, req.Id)
	if userEntity == nil || userEntity.Id == 0 {
		return nil, fmt.Errorf("用户不存在")
	}

	// 判断用户是否不是服务中心
	if !userEntity.IsServiceCenter {
		return nil, fmt.Errorf("用户不是服务中心")
	}

	// 更新用户取消服务中心
	err := s.userRepo.UpdateUser(ctx, req.Id, map[string]interface{}{
		"is_service_center": false,
	})
	if err != nil {
		return nil, fmt.Errorf("取消服务中心失败: %v", err)
	}

	return &CancelServiceCenterRes{
		IsServiceCenter: false,
	}, nil
}

// getAllParentServiceCenters 获取所有上级服务中心用户（私有方法）
func (s *userService) getAllParentServiceCenters(ctx context.Context, userId int64) ([]*entity.UserEntity, error) {
	var serviceCenters []*entity.UserEntity

	currentUser, err := s.userRepo.GetUserById(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("查询用户信息失败: %v", err)
	}
	if currentUser == nil {
		return serviceCenters, nil
	}

	// 向上递归查询所有上级
	for {
		// 如果没有上级，停止递归
		if currentUser.ParentInviteCode == "" {
			break
		}

		// 获取上级用户
		parentUser, err := s.userRepo.GetUserByInviteCode(ctx, currentUser.ParentInviteCode)
		if err != nil {
			return nil, fmt.Errorf("查询上级用户失败: %v", err)
		}
		if parentUser == nil {
			break
		}

		// 如果是服务中心，添加到列表中
		if parentUser.IsServiceCenter {
			serviceCenters = append(serviceCenters, parentUser)
		}

		// 继续向上查询
		currentUser = parentUser
	}

	return serviceCenters, nil
}

// SetServiceCenterRate 设置服务中心点位比例
func (s *userService) SetServiceCenterRate(ctx context.Context, req *SetServiceCenterRateReq) (*SetServiceCenterRateRes, error) {
	// 校验点位比例范围 0-100
	if req.ServiceCenterRate < 0 || req.ServiceCenterRate > 100 {
		return nil, fmt.Errorf("服务中心点位比例必须在0-100之间")
	}

	// 校验用户ID
	if req.Id <= 0 {
		return nil, fmt.Errorf("用户ID不能为空")
	}

	// 查询用户是否存在
	userEntity, _ := s.userRepo.GetUserById(ctx, req.Id)
	if userEntity == nil || userEntity.Id == 0 {
		return nil, fmt.Errorf("用户不存在")
	}

	// 查询所有上级服务中心
	parentServiceCenters, err := s.getAllParentServiceCenters(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	// 检查请求的点位比例是否大于任何上级的点位比例
	requestRate := decimal.NewFromFloat(req.ServiceCenterRate)
	for _, parent := range parentServiceCenters {
		if requestRate.GreaterThan(parent.ServiceCenterRate) {
			return nil, fmt.Errorf("服务中心点位比例不能大于上级服务中心的点位比例")
		}
	}

	// 更新用户的服务中心点位比例
	err = s.userRepo.UpdateUser(ctx, req.Id, map[string]interface{}{
		"service_center_rate": req.ServiceCenterRate,
	})
	if err != nil {
		return nil, fmt.Errorf("设置服务中心点位比例失败: %v", err)
	}

	return &SetServiceCenterRateRes{}, nil
}

// UpdateUserWalletAddress 修改用户地址
func (s *userService) UpdateUserWalletAddress(ctx context.Context, req *UpdateUserWalletAddressReq) error {
	newWalletAddress := strings.ToLower(req.NewWalletAddress)

	// 1. 校验钱包地址格式
	if !utils.IsValidEthereumAddress(newWalletAddress) {
		return fmt.Errorf("新钱包地址格式不正确")
	}

	// 2. 根据 id 查询用户数据
	userEntity, err := s.userRepo.GetUserById(ctx, req.Id)
	if err != nil {
		return fmt.Errorf("查询用户失败: %v", err)
	}
	if userEntity == nil {
		return fmt.Errorf("用户不存在")
	}

	// 3. 根据 new_wallet_address 新地址查询数据
	exists, err := s.userRepo.CheckWalletAddressExists(ctx, newWalletAddress)
	if err != nil {
		return fmt.Errorf("检查新地址是否存在失败: %v", err)
	}
	if exists {
		return fmt.Errorf("该地址已存在")
	}

	// 保存旧的钱包地址
	oldWalletAddress := userEntity.WalletAddress

	// 4. 将这个 id 用户的 wallet_address 值更新为 new_wallet_address
	err = s.userRepo.UpdateUser(ctx, req.Id, map[string]interface{}{
		"wallet_address": newWalletAddress,
	})
	if err != nil {
		return fmt.Errorf("更新用户钱包地址失败: %v", err)
	}

	// 5. 更新 parent_wallet_address = 这个id原先的wallet_address的数据，parent_wallet_address值更新为new_wallet_address
	err = s.userRepo.UpdateParentWalletAddressByOldWalletAddress(ctx, oldWalletAddress, newWalletAddress)
	if err != nil {
		return fmt.Errorf("批量更新下级用户的父钱包地址失败: %v", err)
	}

	return nil
}

// enrichMetadataWithUserInfo 根据业务类型增强Metadata信息（私有方法）
func (s *userService) enrichMetadataWithUserInfo(ctx context.Context, businessType, metadata string) (string, error) {
	// 如果Metadata为空，直接返回
	if metadata == "" {
		return metadata, nil
	}

	// 根据业务类型进行不同的处理
	switch businessType {
	case consts.AssetBusinessTypeRewardDistrict:
		return s.enrichDistrictRewardMetadata(ctx, metadata)
	case consts.AssetBusinessTypeRewardReferral:
		return s.enrichReferralRewardMetadata(ctx, metadata)
	case consts.AssetBusinessTypeRewardServiceCenter:
		return s.enrichServiceCenterRewardMetadata(ctx, metadata)
	default:
		// 其他业务类型暂不处理，直接返回原始Metadata
		return metadata, nil
	}
}

// enrichDistrictRewardMetadata 增强小区奖励Metadata信息（私有方法）
// 在branch_details中为每个分支添加branch_root_user_wallet_address字段
func (s *userService) enrichDistrictRewardMetadata(ctx context.Context, metadata string) (string, error) {
	// 解析Metadata为结构体
	var districtMeta rewardEntity.DistrictRewardMetadata
	if err := json.Unmarshal([]byte(metadata), &districtMeta); err != nil {
		return "", fmt.Errorf("解析小区奖励Metadata失败: %v", err)
	}

	// 如果branch_details为空，直接返回原始Metadata
	if len(districtMeta.BranchDetails) == 0 {
		return metadata, nil
	}

	// 提取所有branch_root_user_id
	userIDs := make([]int64, 0, len(districtMeta.BranchDetails))
	userIDSet := make(map[int64]struct{}) // 用于去重
	for _, branch := range districtMeta.BranchDetails {
		if branch.BranchRootUserID > 0 {
			if _, exists := userIDSet[branch.BranchRootUserID]; !exists {
				userIDs = append(userIDs, branch.BranchRootUserID)
				userIDSet[branch.BranchRootUserID] = struct{}{}
			}
		}
	}

	// 如果没有有效的用户ID，直接返回原始Metadata
	if len(userIDs) == 0 {
		return metadata, nil
	}

	// 批量查询用户信息
	users, err := s.userRepo.GetDataByIds(ctx, userIDs)
	if err != nil {
		return "", fmt.Errorf("批量查询用户信息失败: %v", err)
	}

	// 构建用户ID到钱包地址的映射
	userIDToWalletMap := make(map[int64]string)
	for _, user := range users {
		if user != nil {
			userIDToWalletMap[user.Id] = user.WalletAddress
		}
	}

	// 将结构体转换为map，以便动态添加字段
	var metadataMap map[string]interface{}
	if err := json.Unmarshal([]byte(metadata), &metadataMap); err != nil {
		return "", fmt.Errorf("将Metadata转换为map失败: %v", err)
	}

	// 处理branch_details数组
	branchDetailsInterface, ok := metadataMap["branch_details"]
	if !ok {
		return metadata, nil
	}

	branchDetailsArray, ok := branchDetailsInterface.([]interface{})
	if !ok {
		return metadata, nil
	}

	// 为每个branch_detail添加wallet_address字段
	for i, branchDetailInterface := range branchDetailsArray {
		branchDetailMap, ok := branchDetailInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// 获取branch_root_user_id
		branchRootUserIDFloat, ok := branchDetailMap["branch_root_user_id"].(float64)
		if !ok {
			continue
		}
		branchRootUserID := int64(branchRootUserIDFloat)

		// 查找对应的钱包地址
		if walletAddress, exists := userIDToWalletMap[branchRootUserID]; exists {
			branchDetailMap["branch_root_user_wallet_address"] = walletAddress
			branchDetailsArray[i] = branchDetailMap
		}
	}

	// 更新metadataMap中的branch_details
	metadataMap["branch_details"] = branchDetailsArray

	// 重新序列化为JSON
	enrichedMetadataBytes, err := json.Marshal(metadataMap)
	if err != nil {
		return "", fmt.Errorf("序列化增强后的Metadata失败: %v", err)
	}

	return string(enrichedMetadataBytes), nil
}

// enrichReferralRewardMetadata 增强推荐奖励Metadata信息（私有方法）
// 在static_rewards中为每个用户记录添加user_wallet_address字段
func (s *userService) enrichReferralRewardMetadata(ctx context.Context, metadata string) (string, error) {
	// 将Metadata转换为map，以便动态处理
	var metadataMap map[string]interface{}
	if err := json.Unmarshal([]byte(metadata), &metadataMap); err != nil {
		return "", fmt.Errorf("解析推荐奖励Metadata失败: %v", err)
	}

	// 获取static_rewards字段
	staticRewardsInterface, ok := metadataMap["static_rewards"]
	if !ok {
		// 如果没有static_rewards字段，直接返回原始Metadata
		return metadata, nil
	}

	// static_rewards应该是一个对象（map），键是层级（字符串），值是数组
	staticRewardsMap, ok := staticRewardsInterface.(map[string]interface{})
	if !ok {
		// 如果格式不正确，直接返回原始Metadata
		return metadata, nil
	}

	// 提取所有user_id（去重）
	userIDs := make([]int64, 0)
	userIDSet := make(map[int64]struct{}) // 用于去重

	// 遍历所有层级
	for _, layerArrayInterface := range staticRewardsMap {
		layerArray, ok := layerArrayInterface.([]interface{})
		if !ok {
			continue
		}

		// 遍历该层级的所有用户记录
		for _, userRecordInterface := range layerArray {
			userRecordMap, ok := userRecordInterface.(map[string]interface{})
			if !ok {
				continue
			}

			// 获取user_id
			userIDFloat, ok := userRecordMap["user_id"].(float64)
			if !ok {
				continue
			}
			userID := int64(userIDFloat)

			// 添加到集合中（去重）
			if userID > 0 {
				if _, exists := userIDSet[userID]; !exists {
					userIDs = append(userIDs, userID)
					userIDSet[userID] = struct{}{}
				}
			}
		}
	}

	// 如果没有有效的用户ID，直接返回原始Metadata
	if len(userIDs) == 0 {
		return metadata, nil
	}

	// 批量查询用户信息
	users, err := s.userRepo.GetDataByIds(ctx, userIDs)
	if err != nil {
		return "", fmt.Errorf("批量查询用户信息失败: %v", err)
	}

	// 构建用户ID到钱包地址的映射
	userIDToWalletMap := make(map[int64]string)
	for _, user := range users {
		if user != nil {
			userIDToWalletMap[user.Id] = user.WalletAddress
		}
	}

	// 为每个层级的每个用户记录添加user_wallet_address字段
	for layerKey, layerArrayInterface := range staticRewardsMap {
		layerArray, ok := layerArrayInterface.([]interface{})
		if !ok {
			continue
		}

		// 遍历该层级的所有用户记录，添加钱包地址
		for i, userRecordInterface := range layerArray {
			userRecordMap, ok := userRecordInterface.(map[string]interface{})
			if !ok {
				continue
			}

			// 获取user_id
			userIDFloat, ok := userRecordMap["user_id"].(float64)
			if !ok {
				continue
			}
			userID := int64(userIDFloat)

			// 查找对应的钱包地址并添加
			if walletAddress, exists := userIDToWalletMap[userID]; exists {
				userRecordMap["user_wallet_address"] = walletAddress
				layerArray[i] = userRecordMap
			}
		}

		// 更新该层级的数据
		staticRewardsMap[layerKey] = layerArray
	}

	// 更新metadataMap中的static_rewards
	metadataMap["static_rewards"] = staticRewardsMap

	// 重新序列化为JSON
	enrichedMetadataBytes, err := json.Marshal(metadataMap)
	if err != nil {
		return "", fmt.Errorf("序列化增强后的Metadata失败: %v", err)
	}

	return string(enrichedMetadataBytes), nil
}

// enrichServiceCenterRewardMetadata 增强服务中心奖励Metadata信息（私有方法）
// 在allocated_details中为每个分配记录添加child_user_wallet_address字段
func (s *userService) enrichServiceCenterRewardMetadata(ctx context.Context, metadata string) (string, error) {
	// 将Metadata转换为map，以便动态处理
	var metadataMap map[string]interface{}
	if err := json.Unmarshal([]byte(metadata), &metadataMap); err != nil {
		return "", fmt.Errorf("解析服务中心奖励Metadata失败: %v", err)
	}

	// 获取allocated_details字段
	allocatedDetailsInterface, ok := metadataMap["allocated_details"]
	if !ok {
		// 如果没有allocated_details字段，直接返回原始Metadata
		return metadata, nil
	}

	// allocated_details应该是一个数组
	allocatedDetailsArray, ok := allocatedDetailsInterface.([]interface{})
	if !ok {
		// 如果格式不正确，直接返回原始Metadata
		return metadata, nil
	}

	// 如果数组为空，直接返回原始Metadata
	if len(allocatedDetailsArray) == 0 {
		return metadata, nil
	}

	// 提取所有child_user_id（去重）
	userIDs := make([]int64, 0)
	userIDSet := make(map[int64]struct{}) // 用于去重

	// 遍历allocated_details数组
	for _, detailInterface := range allocatedDetailsArray {
		detailMap, ok := detailInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// 获取child_user_id
		childUserIDFloat, ok := detailMap["child_user_id"].(float64)
		if !ok {
			continue
		}
		childUserID := int64(childUserIDFloat)

		// 添加到集合中（去重）
		if childUserID > 0 {
			if _, exists := userIDSet[childUserID]; !exists {
				userIDs = append(userIDs, childUserID)
				userIDSet[childUserID] = struct{}{}
			}
		}
	}

	// 如果没有有效的用户ID，直接返回原始Metadata
	if len(userIDs) == 0 {
		return metadata, nil
	}

	// 批量查询用户信息
	users, err := s.userRepo.GetDataByIds(ctx, userIDs)
	if err != nil {
		return "", fmt.Errorf("批量查询用户信息失败: %v", err)
	}

	// 构建用户ID到钱包地址的映射
	userIDToWalletMap := make(map[int64]string)
	for _, user := range users {
		if user != nil {
			userIDToWalletMap[user.Id] = user.WalletAddress
		}
	}

	// 为每个allocated_detail添加child_user_wallet_address字段
	for i, detailInterface := range allocatedDetailsArray {
		detailMap, ok := detailInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// 获取child_user_id
		childUserIDFloat, ok := detailMap["child_user_id"].(float64)
		if !ok {
			continue
		}
		childUserID := int64(childUserIDFloat)

		// 查找对应的钱包地址并添加
		if walletAddress, exists := userIDToWalletMap[childUserID]; exists {
			detailMap["child_user_wallet_address"] = walletAddress
			allocatedDetailsArray[i] = detailMap
		}
	}

	// 更新metadataMap中的allocated_details
	metadataMap["allocated_details"] = allocatedDetailsArray

	// 重新序列化为JSON
	enrichedMetadataBytes, err := json.Marshal(metadataMap)
	if err != nil {
		return "", fmt.Errorf("序列化增强后的Metadata失败: %v", err)
	}

	return string(enrichedMetadataBytes), nil
}

// GetUserRealtimeStats 获取用户实时数据
func (s *userService) GetUserRealtimeStats(ctx context.Context, req *GetUserRealtimeStatsReq) (*GetUserRealtimeStatsRes, error) {
	if req == nil || req.Id <= 0 {
		return nil, fmt.Errorf("用户ID不能为空")
	}

	redisCache := cache.GetRedisCache()
	cacheKey := cache.CacheKey{}.Performance().RealtimeStats(req.Id)

	if redisCache != nil {
		if cachedValue, err := redisCache.Get(ctx, cacheKey); err == nil && cachedValue != nil && !cachedValue.IsNil() {
			var cachedRes GetUserRealtimeStatsRes
			if err = cachedValue.Struct(&cachedRes); err == nil {
				return &cachedRes, nil
			}
		}
	}

	userEntity, err := s.userRepo.GetUserById(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %v", err)
	}
	if userEntity == nil {
		return nil, errors.New("用户不存在")
	}

	userPurchaseAmount, err := s.userRepo.GetUserPurchaseAmount(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("查询用户购买总金额失败: %v", err)
	}
	userPurchaseAmountStr := decimal.NewFromInt(userPurchaseAmount).String()

	descendantIDs, err := s.userRepo.GetAllDescendantIDs(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("获取用户下级失败: %v", err)
	}

	if len(descendantIDs) == 0 {
		result := &GetUserRealtimeStatsRes{
			UserPurchaseAmount:         userPurchaseAmountStr,
			NetworkWithdrawTotal:       "0",
			NetworkRealtimePerformance: "0",
		}
		if redisCache != nil {
			_ = redisCache.Set(ctx, cacheKey, result, 10*time.Minute)
		}
		return result, nil
	}

	realtimePerformance, err := s.stakingPackageRepo.SumStakeAmountByUserIDs(ctx, descendantIDs)
	if err != nil {
		return nil, fmt.Errorf("统计网体实时业绩失败: %v", err)
	}

	result := &GetUserRealtimeStatsRes{
		UserPurchaseAmount:         userPurchaseAmountStr,
		NetworkWithdrawTotal:       "0",
		NetworkRealtimePerformance: realtimePerformance.String(),
	}

	if redisCache != nil {
		_ = redisCache.Set(ctx, cacheKey, result, 10*time.Minute)
	}

	return result, nil
}

// UpdateTeamCanWithdraw 更新团队成员的提现权限
func (s *userService) UpdateTeamCanWithdraw(ctx context.Context, req *UpdateTeamCanWithdrawReq) error {
	if req == nil || req.Id <= 0 {
		return fmt.Errorf("用户ID不能为空")
	}

	// 如果直接传了 can_withdraw，则只更新单个用户
	if req.CanWithdraw != nil {
		err := s.userRepo.UpdateUserCanWithdraw(ctx, req.Id, *req.CanWithdraw)
		if err != nil {
			return fmt.Errorf("更新用户提现权限失败: %v", err)
		}
		return nil
	}

	if req.WithdrawType != "team" && req.WithdrawType != "personal" {
		req.WithdrawType = "personal"
	}

	err := s.userRepo.UpdateTeamCanWithdraw(ctx, req.Id, req.WithdrawType)
	if err != nil {
		return fmt.Errorf("更新团队成员的提现权限失败: %v", err)
	}

	return nil
}

// UpdateTeamStakeRate 更新团队成员的质押收益率
func (s *userService) UpdateTeamStakeRate(ctx context.Context, req *UpdateTeamStakeRateReq) error {
	if req == nil || req.Id <= 0 {
		return fmt.Errorf("用户ID不能为空")
	}
	if req.StakeRate < 0 || req.StakeRate > 1 {
		return fmt.Errorf("质押收益率必须在0-1之间,0.01表示1%%")
	}

	err := s.userRepo.UpdateTeamStakeRate(ctx, req.Id, req.StakeRate)
	if err != nil {
		return fmt.Errorf("更新团队成员的质押收益率失败: %v", err)
	}
	return nil
}

// UpdateUserNodeExempt 更新用户赠送节点业绩豁免状态
func (s *userService) UpdateUserNodeExempt(ctx context.Context, req *UpdateUserNodeExemptReq) error {
	if req == nil || req.Id <= 0 {
		return fmt.Errorf("用户ID不能为空")
	}

	err := s.userRepo.UpdateUserNodeExempt(ctx, req.Id, req.NodeExempt)
	if err != nil {
		return fmt.Errorf("更新用户豁免状态失败: %v", err)
	}
	return nil
}

// SetUseFullPerf 设置团队奖是否使用完整团队业绩（不扣除大区业绩）
func (s *userService) SetUseFullPerf(ctx context.Context, req *SetUseFullPerfReq) (*SetUseFullPerfRes, error) {
	if req == nil || req.Id <= 0 {
		return nil, fmt.Errorf("user id is required")
	}

	err := s.userRepo.UpdateUser(ctx, req.Id, map[string]interface{}{
		"use_full_perf": req.UseFullPerf,
	})
	if err != nil {
		return nil, fmt.Errorf("update team reward use full performance failed: %v", err)
	}

	return &SetUseFullPerfRes{
		UseFullPerf: req.UseFullPerf,
	}, nil
}

// GetUserDetail 获取用户详情
func (s *userService) GetUserDetail(ctx context.Context, req *GetUserDetailReq) (*GetUserDetailRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return nil, fmt.Errorf("用户不存在")
	}

	vipLevel := userEntity.VipLevel

	// 获取团队名称
	leader := ""
	if userEntity.TeamId != nil && *userEntity.TeamId > 0 {
		teams, _ := s.teamDao.GetByIds(ctx, []int64{*userEntity.TeamId})
		if team, ok := teams[*userEntity.TeamId]; ok {
			leader = team.Name
		}
	}

	// 获取总质押金额（从 staking_v2_order 统计）
	var totalStaking decimal.Decimal
	stakeRow, err := g.DB().GetOne(ctx, "SELECT COALESCE(SUM(amount), 0) AS total FROM staking_v2_order WHERE user_id = ?", userEntity.Id)
	if err == nil && stakeRow != nil {
		totalStaking, _ = decimal.NewFromString(stakeRow["total"].String())
	}

	// Staking V1(staking_package) 已废弃，详情页不再查询旧表日收益率
	stakeRate := ""

	// 获取总提现金额（从 cobo_withdraw_request 统计，排除 rejected/failed）
	var totalWithdraw decimal.Decimal
	withdrawRow, err := g.DB().GetOne(ctx, `
		SELECT COALESCE(SUM(amount), 0) AS total
		FROM cobo_withdraw_request
		WHERE user_id = ? AND status NOT IN (?, ?)
	`, userEntity.Id, coboEntity.WithdrawStatusRejected, coboEntity.WithdrawStatusFailed)
	if err == nil && withdrawRow != nil {
		totalWithdraw, _ = decimal.NewFromString(withdrawRow["total"].String())
	}

	// 获取总拒绝次数和金额（从 cobo_withdraw_request 统计）
	var totalRejectCount int
	var totalRejectAmount decimal.Decimal
	rejectRow, err := g.DB().GetOne(ctx, `
		SELECT COALESCE(COUNT(*), 0) AS cnt, COALESCE(SUM(amount), 0) AS total
		FROM cobo_withdraw_request
		WHERE user_id = ? AND status = ?
	`, userEntity.Id, coboEntity.WithdrawStatusRejected)
	if err == nil && rejectRow != nil {
		totalRejectCount = rejectRow["cnt"].Int()
		totalRejectAmount, _ = decimal.NewFromString(rejectRow["total"].String())
	}

	// 获取总奖励金额
	totalReward, _ := s.assetRecordRepo.GetUserTotalIncome(ctx, userEntity.Id, 1)

	// 身份判断
	identity := "普通用户"
	if userEntity.IsServiceCenter {
		identity = "服务中心"
	}

	// 获取用户所有币种余额
	var balances []UserBalanceInfo
	balanceRows, err := g.DB().GetAll(ctx, `
		SELECT symbol, available_amount, frozen_amount
		FROM cobo_balance
		WHERE user_id = ?
		ORDER BY symbol
	`, userEntity.Id)
	if err == nil && balanceRows != nil {
		for _, row := range balanceRows {
			available, _ := decimal.NewFromString(row["available_amount"].String())
			frozen, _ := decimal.NewFromString(row["frozen_amount"].String())
			if available.IsZero() && frozen.IsZero() {
				continue
			}
			balances = append(balances, UserBalanceInfo{
				Symbol:          row["symbol"].String(),
				AvailableAmount: available.StringFixed(8),
				FrozenAmount:    frozen.StringFixed(8),
			})
		}
	}

	// 大小区团队业绩（复用 Staking V2 口径）
	var bigTeamPerf, smallTeamPerf decimal.Decimal
	if agg, err := s.teamStatsSvc.GetOverviewAggByInviteCode(ctx, userEntity.InviteCode); err == nil && agg != nil {
		bigTeamPerf = agg.BigTeamPerformance
		smallTeamPerf = agg.SmallTeamPerformance
	}

	return &GetUserDetailRes{
		Id:                   userEntity.Id,
		WalletAddress:        userEntity.WalletAddress,
		CreatedAt:            userEntity.CreatedAt.Format("2006-01-02 15:04:05"),
		InviteCode:           userEntity.InviteCode,
		Identity:             identity,
		VipLevel:             vipLevel,
		CanWithdraw:          userEntity.CanWithdraw,
		UseFullPerf:          userEntity.UseFullPerf,
		Leader:               leader,
		TotalStaking:         totalStaking.StringFixed(2),
		TotalWithdraw:        totalWithdraw.StringFixed(2),
		TotalReward:          totalReward.StringFixed(2),
		StakeRate:            stakeRate,
		TotalRejectCount:     totalRejectCount,
		TotalRejectAmount:    totalRejectAmount.StringFixed(2),
		BigTeamPerformance:   bigTeamPerf.StringFixed(2),
		SmallTeamPerformance: smallTeamPerf.StringFixed(2),
		Balances:             balances,
	}, nil
}

// GetUserWithdrawQuota 获取用户提现额度
func (s *userService) GetUserWithdrawQuota(ctx context.Context, req *GetUserWithdrawQuotaReq) (*GetUserWithdrawQuotaRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return nil, fmt.Errorf("user not found")
	}

	month := time.Now().In(cnLocation).Format("2006-01")

	// 累计充值（终身累计）
	var totalRecharge decimal.Decimal
	rRow, err := g.DB().GetOne(ctx, `
		SELECT COALESCE(SUM(amount), 0)::numeric(20,8) AS total
		FROM cobo_recharge_record
		WHERE user_id = ? AND symbol = 'USDT' AND status = 1
	`, userEntity.Id)
	if err == nil && rRow != nil {
		totalRecharge, _ = decimal.NewFromString(rRow["total"].String())
	}

	// 累计提现（终身累计，不含本次，排除 rejected/failed）
	var totalWithdraw decimal.Decimal
	wRow, err := g.DB().GetOne(ctx, `
		SELECT COALESCE(SUM(amount), 0)::numeric(20,8) AS total
		FROM cobo_withdraw_request
		WHERE user_id = ? AND symbol = 'USDT'
		  AND status NOT IN (?, ?)
	`, userEntity.Id, coboEntity.WithdrawStatusRejected, coboEntity.WithdrawStatusFailed)
	if err != nil {
		g.Log().Warningf(ctx, "[Admin] 查询累计提现失败: user_id=%d, err=%v", userEntity.Id, err)
	} else if wRow != nil {
		totalWithdraw, _ = decimal.NewFromString(wRow["total"].String())
	}

	// 节点免税额度（与提现扣税逻辑保持一致）
	// - 真实购买节点额度始终计入
	// - 赠送节点额度仅当 (伞下节点真实购买业绩 + 伞下用户质押业绩) > 赠送节点总额*10 时计入
	var nodeQuota decimal.Decimal
	nRow, err := g.DB().GetOne(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN p.is_gift = 0 THEN p.amount * n.power_multiplier ELSE 0 END), 0)::numeric(28,8) AS real_total,
			COALESCE(SUM(CASE WHEN p.is_gift = 1 THEN p.amount * n.power_multiplier ELSE 0 END), 0)::numeric(28,8) AS gift_total,
			COALESCE(SUM(CASE WHEN p.is_gift = 1 THEN p.amount ELSE 0 END), 0)::numeric(28,8) AS gift_amount_total
		FROM cobo_node_purchase p
		JOIN node_info n ON n.node_type = p.node_type
		WHERE p.user_id = ?
	`, userEntity.Id)
	if err == nil && nRow != nil {
		realQuota, _ := decimal.NewFromString(nRow["real_total"].String())
		giftQuota, _ := decimal.NewFromString(nRow["gift_total"].String())
		giftAmountTotal, _ := decimal.NewFromString(nRow["gift_amount_total"].String())

		nodeQuota = realQuota
		if giftAmountTotal.GreaterThan(decimal.Zero) {
			perfRow, perfErr := g.DB().GetOne(ctx, `
				SELECT
					COALESCE(cp.team_performance, 0)::numeric(28,8) AS node_performance,
					COALESCE(sv2.team_performance, 0)::numeric(28,8) AS group_performance
				FROM user_info u
				LEFT JOIN cobo_performance cp ON cp.user_id = u.id
				LEFT JOIN staking_v2_performance sv2 ON sv2.user_id = u.id
				WHERE u.id = ?
				LIMIT 1
			`, userEntity.Id)
			if perfErr != nil {
				g.Log().Warningf(ctx, "[Admin] 查询赠送节点业绩考核数据失败: user_id=%d, err=%v", userEntity.Id, perfErr)
			} else {
				nodePerf := decimal.Zero
				groupPerf := decimal.Zero
				if perfRow != nil {
					nodePerf, _ = decimal.NewFromString(perfRow["node_performance"].String())
					groupPerf, _ = decimal.NewFromString(perfRow["group_performance"].String())
				}
				required := giftAmountTotal.Mul(decimal.NewFromInt(10))
				if nodePerf.Add(groupPerf).GreaterThan(required) {
					nodeQuota = nodeQuota.Add(giftQuota)
				}
			}
		} else {
			nodeQuota = nodeQuota.Add(giftQuota)
		}
	}

	// 管理员调整（tax_deduction_used 按月，其余用于终身额度调整）
	quotaDao := dao.NewUserWithdrawQuotaDao()
	quotaRecord, _ := quotaDao.GetByUserIDAndMonth(ctx, userEntity.Id, month)

	withdrawOffset := decimal.Zero
	extraQuota := decimal.Zero
	taxDeductionUsed := decimal.Zero
	if quotaRecord != nil {
		withdrawOffset = quotaRecord.WithdrawOffset
		extraQuota = quotaRecord.ExtraQuota
		taxDeductionUsed = quotaRecord.TaxDeductionUsed
	}

	// extra_quota 用于提升税计算中的免费提现阈值（TotalRecharge），
	// 不计入节点抵扣池展示口径，避免与实际扣税逻辑不一致。
	remainingTaxDeductionQuota := nodeQuota.Sub(taxDeductionUsed)
	if remainingTaxDeductionQuota.LessThan(decimal.Zero) {
		remainingTaxDeductionQuota = decimal.Zero
	}

	effectiveWithdraw := totalWithdraw.Sub(withdrawOffset)
	if effectiveWithdraw.LessThan(decimal.Zero) {
		effectiveWithdraw = decimal.Zero
	}

	return &GetUserWithdrawQuotaRes{
		Quota: &UserWithdrawQuotaInfo{
			Month:                      month,
			TotalRecharge:              totalRecharge.StringFixed(2),
			TotalWithdraw:              totalWithdraw.StringFixed(2),
			WithdrawOffset:             withdrawOffset.StringFixed(2),
			EffectiveWithdraw:          effectiveWithdraw.StringFixed(2),
			NodeQuota:                  nodeQuota.StringFixed(2),
			TaxDeductionUsed:           taxDeductionUsed.StringFixed(2),
			RemainingTaxDeductionQuota: remainingTaxDeductionQuota.StringFixed(2),
			ExtraQuota:                 extraQuota.StringFixed(2),
		},
	}, nil
}

// ResetUserWithdrawQuota 重置用户提现额度
func (s *userService) ResetUserWithdrawQuota(ctx context.Context, req *ResetUserWithdrawQuotaReq) (*ResetUserWithdrawQuotaRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return nil, fmt.Errorf("user not found")
	}

	month := time.Now().In(cnLocation).Format("2006-01")

	// 查询累计提现（终身累计）
	var totalWithdraw decimal.Decimal
	wRow, err := g.DB().GetOne(ctx, `
		SELECT COALESCE(SUM(amount), 0)::numeric(20,8) AS total
		FROM cobo_withdraw_request
		WHERE user_id = ? AND symbol = 'USDT'
		  AND status NOT IN (?, ?)
	`, userEntity.Id, coboEntity.WithdrawStatusRejected, coboEntity.WithdrawStatusFailed)
	if err != nil {
		g.Log().Warningf(ctx, "[Admin] 查询累计提现失败(重置额度): user_id=%d, err=%v", userEntity.Id, err)
	} else if wRow != nil {
		totalWithdraw, _ = decimal.NewFromString(wRow["total"].String())
	}

	// 解析参数
	withdrawOffset := totalWithdraw
	if req.WithdrawOffset != "" {
		v, err := decimal.NewFromString(req.WithdrawOffset)
		if err != nil {
			return nil, gerror.New("invalid withdraw_offset")
		}
		if v.LessThan(decimal.Zero) {
			return nil, gerror.New("withdraw_offset must be greater than or equal to 0")
		}
		withdrawOffset = v
	}

	extraQuota := decimal.Zero
	if req.ExtraQuota != "" {
		v, err := decimal.NewFromString(req.ExtraQuota)
		if err != nil {
			return nil, gerror.New("invalid extra_quota")
		}
		if v.LessThan(decimal.Zero) {
			return nil, gerror.New("extra_quota must be greater than or equal to 0")
		}
		extraQuota = v
	}

	quotaDao := dao.NewUserWithdrawQuotaDao()
	record, err := quotaDao.GetByUserIDAndMonth(ctx, userEntity.Id, month)
	if err != nil {
		return nil, fmt.Errorf("query quota record failed: %v", err)
	}

	if record == nil {
		err = quotaDao.Create(ctx, nil, &entity.UserWithdrawQuotaEntity{
			UserID:         userEntity.Id,
			Month:          month,
			WithdrawOffset: withdrawOffset,
			ExtraQuota:     extraQuota,
			Remark:         req.Remark,
		})
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			updateData := map[string]interface{}{
				"withdraw_offset": withdrawOffset,
				"extra_quota":     extraQuota,
				"remark":          req.Remark,
				"updated_at":      time.Now(),
			}
			err = quotaDao.Update(ctx, nil, userEntity.Id, month, updateData)
		}
	} else {
		updateData := map[string]interface{}{
			"withdraw_offset": withdrawOffset,
			"extra_quota":     extraQuota,
			"remark":          req.Remark,
			"updated_at":      time.Now(),
		}
		err = quotaDao.Update(ctx, nil, userEntity.Id, month, updateData)
	}

	if err != nil {
		return nil, fmt.Errorf("update quota record failed: %v", err)
	}

	g.Log().Infof(ctx, "[Admin] 重置用户提现额度: user_id=%d, month=%s, withdraw_offset=%s, extra_quota=%s, remark=%s",
		userEntity.Id, month, withdrawOffset.String(), extraQuota.String(), req.Remark)

	return &ResetUserWithdrawQuotaRes{Success: true}, nil
}

// AdminResetUserPassword 管理员重置用户密码
func (s *userService) AdminResetUserPassword(ctx context.Context, req *AdminResetUserPasswordReq) (*AdminResetUserPasswordRes, error) {
	walletAddress := strings.ToLower(strings.TrimSpace(req.WalletAddress))
	if walletAddress == "" {
		return nil, gerror.New("wallet address is required")
	}
	if !utils.IsValidEthereumAddress(walletAddress) {
		return nil, gerror.New("invalid wallet address format")
	}

	password := strings.TrimSpace(req.Password)
	if password == "" {
		password = defaultUserPassword
	}
	if !isSixDigitNumericPassword(password) {
		return nil, gerror.New("password must be exactly 6 digits")
	}

	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil {
		return nil, gerror.Wrap(err, "query user failed")
	}
	if userEntity == nil {
		return nil, gerror.New("user not found")
	}

	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return nil, gerror.Wrap(err, "hash password failed")
	}

	if err := s.passwordRepo.SetPassword(ctx, userEntity.Id, hashedPassword); err != nil {
		return nil, gerror.Wrap(err, "save password failed")
	}

	return &AdminResetUserPasswordRes{Success: true}, nil
}

// GetUserStakingRecords 获取用户质押记录（基于 staking_v2_order）
func (s *userService) GetUserStakingRecords(ctx context.Context, req *GetUserStakingRecordsReq) (*GetUserStakingRecordsRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return &GetUserStakingRecordsRes{List: []UserStakingRecordItem{}, Total: 0, Page: req.Page, PageSize: req.PageSize}, nil
	}

	total, err := db.GetDB().Ctx(ctx).Model("staking_v2_order").
		Where("user_id", userEntity.Id).
		Count()
	if err != nil {
		return nil, fmt.Errorf("查询质押记录总数失败: %v", err)
	}

	type stakingRow struct {
		Id          int64       `orm:"id"`
		RequestId   string      `orm:"request_id"`
		Amount      string      `orm:"amount"`
		YYAIAmount  string      `orm:"yyai_amount"`
		SourceType  int         `orm:"source_type"`
		Status      int         `orm:"status"`
		TotalReward string      `orm:"total_reward"`
		CreatedAt   *gtime.Time `orm:"created_at"`
	}
	var rows []stakingRow
	err = db.GetDB().Ctx(ctx).Model("staking_v2_order").
		Where("user_id", userEntity.Id).
		Fields("id, request_id, amount, yyai_amount, source_type, status, total_reward, created_at").
		OrderDesc("id").
		Page(req.Page, req.PageSize).
		Scan(&rows)
	if err != nil {
		return nil, fmt.Errorf("查询质押记录失败: %v", err)
	}

	sourceTypeDesc := map[int]string{
		consts.StakingV2SourceTypeManualStake: "手动质押",
		consts.StakingV2SourceTypeTripleFill:  "Triple补充",
	}
	statusDesc := map[int]string{
		consts.StakingV2StatusActive: "运行中",
		consts.StakingV2StatusCapped: "已封顶",
	}

	items := make([]UserStakingRecordItem, 0, len(rows))
	for _, r := range rows {
		stakeTime := ""
		if r.CreatedAt != nil && !r.CreatedAt.IsZero() {
			stakeTime = r.CreatedAt.Layout("2006-01-02 15:04:05")
		}
		packageNo := r.RequestId
		if packageNo == "" {
			packageNo = fmt.Sprintf("STAKEV2-%d", r.Id)
		}
		sourceType := strconv.Itoa(r.SourceType)
		desc := sourceTypeDesc[r.SourceType]

		amountDec, amountErr := decimal.NewFromString(r.Amount)
		if amountErr != nil {
			amountDec = decimal.Zero
		}
		totalRewardDec, rewardErr := decimal.NewFromString(r.TotalReward)
		if rewardErr != nil {
			totalRewardDec = decimal.Zero
		}
		rewardLimit := amountDec.Mul(decimal.NewFromInt(4))
		restQuota := rewardLimit.Sub(totalRewardDec)
		if restQuota.LessThan(decimal.Zero) {
			restQuota = decimal.Zero
		}

		items = append(items, UserStakingRecordItem{
			Id:             r.Id,
			PackageNo:      packageNo,
			StakeTime:      stakeTime,
			Amount:         r.Amount,
			TotalReward:    totalRewardDec.String(),
			RewardLimit:    rewardLimit.String(),
			RestQuota:      restQuota.String(),
			SourceType:     sourceType,
			SourceTypeDesc: desc,
			StakeType:      sourceType,
			StakeTypeDesc:  desc,
			Status:         r.Status,
			StatusDesc:     statusDesc[r.Status],
			IsGift:         0,
			IsGiftDesc:     "",
		})
	}

	return &GetUserStakingRecordsRes{
		List:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetUserNodePurchaseRecords 获取用户节点购买记录
func (s *userService) GetUserNodePurchaseRecords(ctx context.Context, req *GetUserNodePurchaseRecordsReq) (*GetUserNodePurchaseRecordsRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return &GetUserNodePurchaseRecordsRes{List: []UserNodePurchaseRecordItem{}, Total: 0, Page: req.Page, PageSize: req.PageSize}, nil
	}

	total, err := db.GetDB().Ctx(ctx).Model("cobo_node_purchase").
		Where("user_id", userEntity.Id).
		Count()
	if err != nil {
		return nil, fmt.Errorf("查询节点购买记录总数失败: %v", err)
	}

	var records []*coboEntity.NodePurchaseEntity
	err = db.GetDB().Ctx(ctx).Model("cobo_node_purchase").
		Where("user_id", userEntity.Id).
		OrderDesc("created_at").
		Page(req.Page, req.PageSize).
		Scan(&records)
	if err != nil {
		return nil, fmt.Errorf("查询节点购买记录失败: %v", err)
	}

	items := make([]UserNodePurchaseRecordItem, 0, len(records))
	for _, record := range records {
		nodeTypeDesc := getNodeTypeDesc(record.NodeType)
		statusDesc := getCoboNodeStatusDesc(record.Status)
		isGiftDesc := "正常购买"
		if record.IsGift == 1 {
			isGiftDesc = "后台赠送"
		}

		items = append(items, UserNodePurchaseRecordItem{
			Id:                 record.ID,
			PackageNo:          record.PackageNo,
			NodeType:           record.NodeType,
			NodeTypeDesc:       nodeTypeDesc,
			Amount:             record.Amount.StringFixed(2),
			PowerValue:         record.PowerValue.StringFixed(2),
			Status:             record.Status,
			StatusDesc:         statusDesc,
			IsGift:             record.IsGift,
			IsGiftDesc:         isGiftDesc,
			DirectRewardAmount: record.DirectRewardAmount.StringFixed(2),
			CreatedAt:          record.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &GetUserNodePurchaseRecordsRes{
		List:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetUserRewardRecords 获取用户奖励明细
func (s *userService) GetUserRewardRecords(ctx context.Context, req *GetUserRewardRecordsReq) (*GetUserRewardRecordsRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return &GetUserRewardRecordsRes{List: []UserRewardRecordItem{}, Total: 0, Page: req.Page, PageSize: req.PageSize}, nil
	}
	rewardType := req.RewardType
	if rewardType == "" {
		switch req.BusinessType {
		case consts.AssetBusinessTypeMintReferral:
			rewardType = coboModel.RewardTypeIndirect
		case consts.AssetBusinessTypeMintWinner:
			rewardType = coboModel.RewardTypeMatchReward
		default:
			rewardType = ""
		}
	}

	rewardRes, err := coboService.NewRewardService().GetRewardRecords(ctx, &coboModel.GetRewardRecordsReq{
		UserID:     userEntity.Id,
		RewardType: rewardType,
		Page:       req.Page,
		PageSize:   req.PageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("query reward records failed: %v", err)
	}

	items := make([]UserRewardRecordItem, 0, len(rewardRes.List))
	for _, rec := range rewardRes.List {
		items = append(items, UserRewardRecordItem{
			Id:                  rec.ID,
			RewardType:          rec.RewardType,
			RewardTypeText:      rec.RewardTypeText,
			Amount:              rec.Amount,
			Symbol:              rec.Symbol,
			SourceUserID:        rec.SourceUserID,
			SourceWalletAddress: rec.SourceWalletAddress,
			PurchasePackageNo:   rec.PurchasePackageNo,
			PurchaseAmount:      rec.PurchaseAmount,
			RewardRate:          rec.RewardRate,
			CreatedAt:           rec.CreatedAt,
		})
	}

	return &GetUserRewardRecordsRes{
		List:     items,
		Total:    int(rewardRes.Total),
		Page:     rewardRes.Page,
		PageSize: rewardRes.PageSize,
	}, nil
}

// changeTypeDescMap 资金明细 change_type 中文描述
var changeTypeDescMap = map[string]string{
	"recharge":                                 "充值",
	"withdraw_freeze":                          "提现冻结",
	"withdraw_success":                         "提现成功",
	"withdraw_fail_refund":                     "提现失败退款",
	"node_purchase":                            "节点购买",
	"node_purchase_direct_reward":              "节点直推奖励",
	"group_match_join_usdt":                    "拼团参与-本金",
	"group_match_join_ticket":                  "拼团参与-门票",
	"group_match_settle_winner":                "拼团中奖结算",
	"group_match_settle_ticket_refund":         "拼团门票退还",
	"group_match_settle_ticket_burn_to_vertex": "拼团门票销毁",
	"group_match_settle_flow_refund":           "拼团流团退款",
	"group_match_loser_comp_release":           "拼团败者补偿释放",
	"group_match_leadership_reward":            "拼团领导池奖励",
	"staking_v2_stake":                         "购买三倍券",
	"staking_v2_referral_direct":               "三倍券直推奖",
	"staking_v2_referral_indirect":             "三倍券间推奖",
	"staking_v2_referral_burn_to_vertex":       "三倍券推荐销毁",
	"transfer_in":                              "内转-转入",
	"transfer_out":                             "内转-转出",
	"exchange_in":                              "兑换-入账",
	"exchange_out":                             "兑换-出账",
	"system_adjust":                            "系统调整",
}

// GetUserBalanceChangeLogs 获取用户资金明细（直接查 cobo_balance_change_log）
func (s *userService) GetUserBalanceChangeLogs(ctx context.Context, req *GetUserBalanceChangeLogsReq) (*GetUserBalanceChangeLogsRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return &GetUserBalanceChangeLogsRes{List: []UserBalanceChangeLogItem{}, Total: 0, Page: req.Page, PageSize: req.PageSize}, nil
	}

	parseTime := func(s string) (string, error) {
		if s == "" {
			return "", nil
		}
		layouts := []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02"}
		for _, l := range layouts {
			if t, err := time.ParseInLocation(l, s, cnLocation); err == nil {
				return t.Format("2006-01-02 15:04:05"), nil
			}
		}
		return "", fmt.Errorf("invalid time format: %s", s)
	}

	startTime, err := parseTime(req.StartTime)
	if err != nil {
		return nil, gerror.Newf("Invalid start_time: %s", req.StartTime)
	}
	endTime, err := parseTime(req.EndTime)
	if err != nil {
		return nil, gerror.Newf("Invalid end_time: %s", req.EndTime)
	}

	query := db.GetDB().Ctx(ctx).Model("cobo_balance_change_log").Where("user_id", userEntity.Id)
	if req.Symbol != "" {
		query = query.Where("symbol", strings.ToUpper(req.Symbol))
	}
	if startTime != "" {
		query = query.Where("created_at >= ?", startTime)
	}
	if endTime != "" {
		query = query.Where("created_at <= ?", endTime)
	}

	total, err := query.Count()
	if err != nil {
		return nil, fmt.Errorf("查询资金明细总数失败: %v", err)
	}

	type logRow struct {
		Id             int64       `orm:"id"`
		Symbol         string      `orm:"symbol"`
		ChangeType     string      `orm:"change_type"`
		Amount         string      `orm:"amount"`
		BeforeBalance  string      `orm:"before_balance"`
		AfterBalance   string      `orm:"after_balance"`
		RelatedOrderNo string      `orm:"related_order_no"`
		Remark         string      `orm:"remark"`
		CreatedAt      *gtime.Time `orm:"created_at"`
	}
	var rows []logRow
	if err := query.
		Fields("id, symbol, change_type, amount, before_balance, after_balance, related_order_no, remark, created_at").
		Order("id DESC").
		Limit((req.Page-1)*req.PageSize, req.PageSize).
		Scan(&rows); err != nil {
		return nil, fmt.Errorf("查询资金明细列表失败: %v", err)
	}

	items := make([]UserBalanceChangeLogItem, 0, len(rows))
	for _, r := range rows {
		timeStr := ""
		if r.CreatedAt != nil && !r.CreatedAt.IsZero() {
			timeStr = r.CreatedAt.Format("Y-m-d H:i:s")
		}
		desc, ok := changeTypeDescMap[r.ChangeType]
		if !ok || desc == "" {
			desc = r.ChangeType
		}
		items = append(items, UserBalanceChangeLogItem{
			Id:             r.Id,
			CreatedAt:      timeStr,
			ChangeType:     r.ChangeType,
			ChangeTypeDesc: desc,
			Symbol:         r.Symbol,
			Amount:         r.Amount,
			BeforeBalance:  r.BeforeBalance,
			AfterBalance:   r.AfterBalance,
			RelatedOrderNo: r.RelatedOrderNo,
			Remark:         r.Remark,
		})
	}

	return &GetUserBalanceChangeLogsRes{
		List:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetUserInviteRecords 获取用户邀请明细
func (s *userService) GetUserInviteRecords(ctx context.Context, req *GetUserInviteRecordsReq) (*GetUserInviteRecordsRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return &GetUserInviteRecordsRes{List: []UserInviteRecordItem{}, Total: 0, Page: req.Page, PageSize: req.PageSize}, nil
	}

	descendants, err := s.userRepo.GetDirectDescendants(ctx, userEntity.InviteCode, userEntity.WalletAddress)
	if err != nil {
		return nil, fmt.Errorf("查询邀请明细失败: %v", err)
	}

	total := len(descendants)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	pagedDescendants := descendants[start:end]

	// 批量获取质押金额和VIP等级
	userIDs := make([]int64, 0, len(pagedDescendants))
	for _, u := range pagedDescendants {
		userIDs = append(userIDs, u.Id)
	}

	stakingMap := make(map[int64]string)
	vipMap := make(map[int64]int)
	if len(userIDs) > 0 {
		for _, uid := range userIDs {
			stats, _ := s.stakingPackageRepo.GetUserStakingStats(ctx, uid)
			if stats != nil {
				stakingMap[uid] = stats.TotalStakeAmount.StringFixed(2)
			}
		}
		vipLevelMap, _ := s.vipLevelRepo.GetCurrentVipLevelsByUserIDs(ctx, userIDs)
		for uid, vipEntity := range vipLevelMap {
			vipMap[uid] = vipEntity.VipLevel
		}
	}

	items := make([]UserInviteRecordItem, 0, len(pagedDescendants))
	for _, u := range pagedDescendants {
		stakeAmount := stakingMap[u.Id]
		if stakeAmount == "" {
			stakeAmount = "0.00"
		}
		items = append(items, UserInviteRecordItem{
			Id:            u.Id,
			WalletAddress: u.WalletAddress,
			InviteCode:    u.InviteCode,
			CreatedAt:     u.CreatedAt.Format("2006-01-02 15:04:05"),
			StakeAmount:   stakeAmount,
			VipLevel:      vipMap[u.Id],
		})
	}

	return &GetUserInviteRecordsRes{
		List:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

// GetUserMintRecords 获取用户拼团记录
func (s *userService) GetUserMintRecords(ctx context.Context, req *GetUserMintRecordsReq) (*GetUserMintRecordsRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)

	players, total, err := s.playerDao.GetUserRecordsByWalletAddress(ctx, walletAddress, req.Page, req.PageSize)
	if err != nil {
		return nil, fmt.Errorf("查询拼团记录失败: %v", err)
	}

	isWinnerDescMap := map[int]string{
		0: "待开奖",
		1: "中奖",
		2: "未中奖",
		3: "未拼成",
	}

	items := make([]UserMintRecordItem, 0, len(players))
	for _, p := range players {
		joinTime := ""
		if p.CreatedAt != nil {
			joinTime = p.CreatedAt.Format("2006-01-02 15:04:05")
		}

		paymentAmount := p.PaymentAmount
		if d, err := decimal.NewFromString(p.PaymentAmount); err == nil {
			paymentAmount = d.String()
		}
		winnerRewardAmount := p.WinnerRewardAmount
		if d, err := decimal.NewFromString(p.WinnerRewardAmount); err == nil {
			winnerRewardAmount = d.String()
		}

		items = append(items, UserMintRecordItem{
			Id:                 p.Id,
			MatchId:            p.MatchId,
			JoinTime:           joinTime,
			PaymentToken:       p.PaymentToken,
			PaymentAmount:      paymentAmount,
			IsWinner:           p.IsWinner,
			IsWinnerDesc:       isWinnerDescMap[p.IsWinner],
			WinnerRewardAmount: winnerRewardAmount,
			GroupId:            p.GroupId,
		})
	}

	return &GetUserMintRecordsRes{
		List:     items,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *userService) GetUserRecordCounts(ctx context.Context, req *GetUserRecordCountsReq) (*GetUserRecordCountsRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)

	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return &GetUserRecordCountsRes{}, nil
	}

	stakingCount, _ := db.GetDB().Ctx(ctx).Model("staking_v2_order").Where("user_id", userEntity.Id).Count()
	nodePurchaseCount, _ := db.GetDB().Ctx(ctx).Model("cobo_node_purchase").Where("user_id", userEntity.Id).Count()

	withdrawCount, _ := db.GetDB().Ctx(ctx).Model("cobo_withdraw_request").Where("user_id", userEntity.Id).Count()

	inviteCount := 0
	inviteRes, err := s.userRepo.GetUserList(ctx, &repository.GetUserListReq{
		PageReq:       model.PageReq{Page: 1, PageSize: 1},
		ParentAddress: walletAddress,
	})
	if err == nil && inviteRes != nil {
		inviteCount = inviteRes.Total
	}

	mintCount, _ := db.GetDB().Ctx(ctx).Model("group_match_order").Where("user_id", userEntity.Id).Count()
	coboRewardCount := s.getCoboRewardCount(ctx, userEntity.Id)

	// rewardCount: 还原旧口径（asset_record 奖励记录 + balance_change_log 中铸币奖励）
	rewardCount := 0
	_, assetTotal, err := s.assetRecordRepo.GetPagedByUserID(ctx, userEntity.Id, "", "", 1, 1)
	if err == nil {
		rewardCount += assetTotal
	}
	mintLogs, _, err := s.balanceChangeLogDao.GetUserLogs(ctx, &balanceModel.QueryBalanceLogsReq{
		UserID:   userEntity.Id,
		Page:     1,
		PageSize: 999999,
	})
	if err == nil {
		for _, log := range mintLogs {
			if log.ChangeType == consts.AssetBusinessTypeMintWinner || log.ChangeType == consts.AssetBusinessTypeMintReferral {
				rewardCount++
			}
		}
	}

	// balanceLogCount: 资金明细（cobo_balance_change_log）总数
	balanceLogCount, _ := db.GetDB().Ctx(ctx).Model("cobo_balance_change_log").Where("user_id", userEntity.Id).Count()

	transferCount, _ := db.GetDB().Ctx(ctx).Model("internal_transfer_record").Where("from_user_id = ? OR to_user_id = ?", userEntity.Id, userEntity.Id).Count()
	swapCount, _ := db.GetDB().Ctx(ctx).Model("swap_record").Where("user_id", userEntity.Id).Count()
	rechargeCount, _ := db.GetDB().Ctx(ctx).Model("cobo_recharge_record").Where("user_id", userEntity.Id).Count()
	yyReleaseCount, _ := db.GetDB().Ctx(ctx).Model("group_match_loser_comp_release_log").Where("user_id", userEntity.Id).Count()
	loserCompensationCount, _ := db.GetDB().Ctx(ctx).Model("group_match_loser_compensation").Where("user_id", userEntity.Id).Count()

	return &GetUserRecordCountsRes{
		StakingCount:           stakingCount,
		WithdrawCount:          withdrawCount,
		InviteCount:            inviteCount,
		MintCount:              mintCount,
		RewardCount:            rewardCount,
		CoboRewardCount:        coboRewardCount,
		BalanceLogCount:        balanceLogCount,
		NodePurchaseCount:      nodePurchaseCount,
		TransferCount:          transferCount,
		SwapCount:              swapCount,
		RechargeCount:          rechargeCount,
		YYReleaseCount:         yyReleaseCount,
		LoserCompensationCount: loserCompensationCount,
	}, nil
}

func (s *userService) GetUserYYReleaseRecords(ctx context.Context, req *GetUserYYReleaseRecordsReq) (*GetUserYYReleaseRecordsRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return &GetUserYYReleaseRecordsRes{List: []YYReleaseRecordItem{}, Total: 0, Page: req.Page, PageSize: req.PageSize}, nil
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	total, err := db.GetDB().Ctx(ctx).Model("group_match_loser_comp_release_log").Where("user_id = ?", userEntity.Id).Count()
	if err != nil {
		return nil, err
	}

	type row struct {
		ID               int64           `json:"id"`
		CompensationID   int64           `json:"compensation_id"`
		OrderID          int64           `json:"order_id"`
		ReleaseDate      time.Time       `json:"release_date"`
		ReleaseAmount    decimal.Decimal `json:"release_amount"`
		ReleaseUSDTValue decimal.Decimal `json:"release_usdt_value"`
		CreatedAt        time.Time       `json:"created_at"`
	}

	var rows []*row
	err = db.GetDB().Ctx(ctx).Model("group_match_loser_comp_release_log").
		Where("user_id = ?", userEntity.Id).
		OrderDesc("created_at").OrderDesc("id").
		Page(page, pageSize).
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	list := make([]YYReleaseRecordItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, YYReleaseRecordItem{
			ID:               r.ID,
			CompensationID:   r.CompensationID,
			OrderID:          r.OrderID,
			ReleaseDate:      r.ReleaseDate.Format("2006-01-02"),
			ReleaseAmount:    r.ReleaseAmount.Round(8).String(),
			ReleaseUSDTValue: r.ReleaseUSDTValue.Round(2).String(),
			CreatedAt:        r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &GetUserYYReleaseRecordsRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *userService) GetUserLoserCompensations(ctx context.Context, req *GetUserLoserCompensationsReq) (*GetUserLoserCompensationsRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return &GetUserLoserCompensationsRes{List: []LoserCompensationItem{}, Total: 0, Page: req.Page, PageSize: req.PageSize}, nil
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	total, err := db.GetDB().Ctx(ctx).Model("group_match_loser_compensation").Where("user_id = ?", userEntity.Id).Count()
	if err != nil {
		return nil, err
	}

	type row struct {
		ID             int64           `json:"id"`
		OrderID        int64           `json:"order_id"`
		TokenSymbol    string          `json:"token_symbol"`
		USDTValue      decimal.Decimal `json:"usdt_value"`
		ReleaseDays    int             `json:"release_days"`
		ReleasedAmount decimal.Decimal `json:"released_amount"`
		PendingAmount  decimal.Decimal `json:"pending_amount"`
		Status         int             `json:"status"`
		StartDate      time.Time       `json:"start_date"`
		CreatedAt      time.Time       `json:"created_at"`
	}

	var rows []*row
	err = db.GetDB().Ctx(ctx).Raw(`
		SELECT
			c.id,
			c.order_id,
			c.token_symbol,
			c.usdt_value,
			c.release_days,
			c.released_amount,
			c.pending_amount,
			c.status,
			c.start_date,
			c.created_at
		FROM group_match_loser_compensation c
		WHERE c.user_id = ?
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT ? OFFSET ?
	`, userEntity.Id, pageSize, (page-1)*pageSize).Scan(&rows)
	if err != nil {
		return nil, err
	}

	list := make([]LoserCompensationItem, 0, len(rows))
	for _, r := range rows {
		releaseLogCount, _ := db.GetDB().Ctx(ctx).Model("group_match_loser_comp_release_log").Where("compensation_id = ?", r.ID).Count()
		list = append(list, LoserCompensationItem{
			ID:              r.ID,
			OrderID:         r.OrderID,
			TokenSymbol:     r.TokenSymbol,
			USDTValue:       r.USDTValue.Round(2).String(),
			ReleaseDays:     r.ReleaseDays,
			ReleasedAmount:  r.ReleasedAmount.Round(8).String(),
			PendingAmount:   r.PendingAmount.Round(8).String(),
			Status:          r.Status,
			StartDate:       r.StartDate.Format("2006-01-02"),
			CreatedAt:       r.CreatedAt.Format("2006-01-02 15:04:05"),
			ReleaseLogCount: releaseLogCount,
		})
	}

	return &GetUserLoserCompensationsRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *userService) GetUserLoserCompReleaseLogs(ctx context.Context, req *GetUserLoserCompReleaseLogsReq) (*GetUserLoserCompReleaseLogsRes, error) {
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	total, err := db.GetDB().Ctx(ctx).Model("group_match_loser_comp_release_log").Where("compensation_id = ?", req.CompensationID).Count()
	if err != nil {
		return nil, err
	}

	type row struct {
		ID               int64           `json:"id"`
		CompensationID   int64           `json:"compensation_id"`
		OrderID          int64           `json:"order_id"`
		ReleaseDate      time.Time       `json:"release_date"`
		ReleaseAmount    decimal.Decimal `json:"release_amount"`
		ReleaseUSDTValue decimal.Decimal `json:"release_usdt_value"`
		CreatedAt        time.Time       `json:"created_at"`
	}

	var rows []*row
	err = db.GetDB().Ctx(ctx).Model("group_match_loser_comp_release_log").
		Where("compensation_id = ?", req.CompensationID).
		OrderDesc("created_at").OrderDesc("id").
		Page(page, pageSize).
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	list := make([]LoserCompReleaseLogItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, LoserCompReleaseLogItem{
			ID:               r.ID,
			CompensationID:   r.CompensationID,
			OrderID:          r.OrderID,
			ReleaseDate:      r.ReleaseDate.Format("2006-01-02"),
			ReleaseAmount:    r.ReleaseAmount.Round(8).String(),
			ReleaseUSDTValue: r.ReleaseUSDTValue.Round(2).String(),
			CreatedAt:        r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &GetUserLoserCompReleaseLogsRes{
		List:     list,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *userService) getCoboRewardCount(ctx context.Context, userID int64) int {
	nodeCount, _ := db.GetDB().Ctx(ctx).Model("cobo_node_purchase").Where("direct_reward_user_id = ? AND direct_reward_amount > 0 AND is_gift = 0", userID).Count()
	directCount, _ := db.GetDB().Ctx(ctx).Model("cobo_balance_change_log").Where("user_id = ? AND change_type = ?", userID, consts.ChangeTypeStakingV2ReferralDirect).Count()
	indirectCount, _ := db.GetDB().Ctx(ctx).Model("cobo_balance_change_log").Where("user_id = ? AND change_type = ?", userID, consts.ChangeTypeStakingV2ReferralIndirect).Count()
	teamCount, _ := db.GetDB().Ctx(ctx).Model("group_match_team_reward_distribution").Where("user_id = ? AND COALESCE(granted_amount, 0) > 0", userID).Count()
	diffCount, _ := db.GetDB().Ctx(ctx).Model("group_purchase_leadership_reward_detail").Where("user_id = ? AND COALESCE(granted_amount, 0) > 0", userID).Count()
	weightCount, _ := db.GetDB().Ctx(ctx).Model("group_purchase_leadership_weight_reward_detail").Where("user_id = ? AND COALESCE(granted_amount, 0) > 0", userID).Count()
	matchCountVal, _ := db.GetDB().GetValue(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT o.session_id
			FROM group_match_order o
			WHERE o.user_id = ?
			  AND o.is_winner = true
			GROUP BY o.session_id
			HAVING COALESCE(SUM(o.reward_amount), 0) > 0
		) t
	`, userID)

	return nodeCount + directCount + indirectCount + teamCount + diffCount + weightCount + matchCountVal.Int()
}

// ReplaceWalletAddress 后台替换钱包地址
// 同步检查：旧地址是否存在、新地址是否已存在；检查通过后启动后台 goroutine 执行跨表事务替换
func (s *userService) ReplaceWalletAddress(ctx context.Context, req *ReplaceWalletAddressReq) error {
	old := strings.ToLower(strings.TrimSpace(req.OldAddress))
	new := strings.ToLower(strings.TrimSpace(req.NewAddress))

	if old == "" || new == "" {
		return fmt.Errorf("旧地址和新地址不能为空")
	}
	if old == new {
		return fmt.Errorf("新旧地址不能相同")
	}

	// 1. 查询旧地址用户
	oldUser, err := s.userRepo.GetUserByWalletAddress(ctx, old)
	if err != nil {
		return fmt.Errorf("查询旧地址失败: %w", err)
	}
	if oldUser == nil {
		return fmt.Errorf("旧地址不存在: %s", old)
	}

	// 2. 检查新地址是否已存在（已存在则直接拒绝）
	exists, err := s.userRepo.CheckWalletAddressExists(ctx, new)
	if err != nil {
		return fmt.Errorf("检查新地址失败: %w", err)
	}
	if exists {
		return fmt.Errorf("新地址已存在，请先处理冲突账号")
	}

	// 3. 查询直推下级
	var referrals []struct {
		ID            int64  `json:"id"`
		WalletAddress string `json:"wallet_address"`
	}
	err = g.DB().Ctx(ctx).Model("user_info").
		Fields("id, wallet_address").
		Where("parent_wallet_address = ?", old).
		Scan(&referrals)
	if err != nil {
		return fmt.Errorf("查询直推下级失败: %w", err)
	}

	// 4. 查询是否为团队长
	var team struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	_ = g.DB().Ctx(ctx).Model("team").
		Fields("id, name").
		Where("leader_wallet_address = ?", old).
		Scan(&team)

	// 5. 启动后台 goroutine 执行替换（HTTP 请求已可返回）
	go s.doReplaceWalletAddress(context.Background(), old, new, oldUser, len(referrals), team.ID > 0, req.AllTables, req.Operator, req.Remark)

	return nil
}

func (s *userService) doReplaceWalletAddress(ctx context.Context, old, new string, oldUser *entity.UserEntity, referralCount int, isTeamLeader, allTables bool, operator, remark string) {
	updates := []struct {
		Table    string
		SetCol   string
		WhereCol string
	}{
		{Table: "user_info", SetCol: "wallet_address", WhereCol: "wallet_address"},
		{Table: "user_info", SetCol: "parent_wallet_address", WhereCol: "parent_wallet_address"},
		{Table: "cobo_performance", SetCol: "wallet_address", WhereCol: "wallet_address"},
		{Table: "group_performance", SetCol: "wallet_address", WhereCol: "wallet_address"},
		{Table: "staking_v2_performance", SetCol: "wallet_address", WhereCol: "wallet_address"},
		{Table: "cobo_recharge_record", SetCol: "from_address", WhereCol: "from_address"},
	}

	if isTeamLeader {
		updates = append(updates, struct {
			Table    string
			SetCol   string
			WhereCol string
		}{Table: "team", SetCol: "leader_wallet_address", WhereCol: "leader_wallet_address"})
	}

	if allTables {
		updates = append(updates, []struct {
			Table    string
			SetCol   string
			WhereCol string
		}{
			{Table: "cobo_withdraw_request", SetCol: "to_address", WhereCol: "to_address"},
			{Table: "cobo_reward_record", SetCol: "source_wallet_address", WhereCol: "source_wallet_address"},
			{Table: "org_chart_nodes", SetCol: "wallet_address", WhereCol: "wallet_address"},
			{Table: "user_deposit_address", SetCol: "address", WhereCol: "address"},
			{Table: "staking_v2_reward_record", SetCol: "wallet_address", WhereCol: "wallet_address"},
			{Table: "staking_v2_leader_reward_detail", SetCol: "wallet_address", WhereCol: "wallet_address"},
			{Table: "group_match_loser_compensation", SetCol: "wallet_address", WhereCol: "wallet_address"},
			{Table: "group_match_team_reward_distribution", SetCol: "wallet_address", WhereCol: "wallet_address"},
			{Table: "public_team_distribution", SetCol: "wallet_address", WhereCol: "wallet_address"},
			{Table: "service_center", SetCol: "wallet_address", WhereCol: "wallet_address"},
			{Table: "admin_info", SetCol: "wallet_address", WhereCol: "wallet_address"},
		}...)
	}

	now := time.Now()
	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		for _, u := range updates {
			_, err := tx.Model(u.Table).Ctx(ctx).
				Data(g.Map{
					u.SetCol:     new,
					"updated_at": now,
				}).
				Where(u.WhereCol+" = ?", old).
				Update()
			if err != nil {
				return fmt.Errorf("更新 %s.%s 失败: %w", u.Table, u.SetCol, err)
			}
		}

		_, err := tx.Model("address_update_log").Ctx(ctx).Data(g.Map{
			"old_address":          old,
			"new_address":          new,
			"user_id":              oldUser.Id,
			"team_name":            oldUser.TeamName,
			"remark":               remark,
			"operator":             operator,
			"updated_tables_count": len(updates),
			"referral_count":       referralCount,
			"is_team_leader":       isTeamLeader,
			"created_at":           now,
			"updated_at":           now,
		}).Insert()
		if err != nil {
			return fmt.Errorf("写入操作日志失败: %w", err)
		}
		return nil
	})

	if err != nil {
		g.Log().Errorf(ctx, "后台替换钱包地址失败 old=%s new=%s err=%v", old, new, err)
	} else {
		g.Log().Infof(ctx, "后台替换钱包地址成功 old=%s new=%s tables=%d", old, new, len(updates))
	}
}

// GetAddressUpdateLogs 获取用户的地址更换日志
func (s *userService) GetAddressUpdateLogs(ctx context.Context, req *GetAddressUpdateLogsReq) (*GetAddressUpdateLogsRes, error) {
	walletAddress := strings.ToLower(strings.TrimSpace(req.WalletAddress))
	if walletAddress == "" {
		return nil, fmt.Errorf("钱包地址不能为空")
	}

	var logs []struct {
		ID                 int64     `json:"id"`
		OldAddress         string    `json:"old_address"`
		NewAddress         string    `json:"new_address"`
		UserID             int64     `json:"user_id"`
		TeamName           string    `json:"team_name"`
		Remark             string    `json:"remark"`
		UpdatedTablesCount int       `json:"updated_tables_count"`
		ReferralCount      int       `json:"referral_count"`
		IsTeamLeader       bool      `json:"is_team_leader"`
		Operator           string    `json:"operator"`
		CreatedAt          time.Time `json:"created_at"`
	}

	err := g.DB().Ctx(ctx).Model("address_update_log").
		Where("old_address = ? OR new_address = ?", walletAddress, walletAddress).
		Order("created_at DESC").
		Scan(&logs)
	if err != nil {
		return nil, fmt.Errorf("查询地址更换日志失败: %w", err)
	}

	var items []AddressUpdateLogItem
	for _, l := range logs {
		items = append(items, AddressUpdateLogItem{
			ID:                 l.ID,
			OldAddress:         l.OldAddress,
			NewAddress:         l.NewAddress,
			UserID:             l.UserID,
			TeamName:           l.TeamName,
			Remark:             l.Remark,
			UpdatedTablesCount: l.UpdatedTablesCount,
			ReferralCount:      l.ReferralCount,
			IsTeamLeader:       l.IsTeamLeader,
			Operator:           l.Operator,
			CreatedAt:          l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &GetAddressUpdateLogsRes{List: items}, nil
}

// GetUserRewardAgg 获取用户各项奖励汇总
func (s *userService) GetUserRewardAgg(ctx context.Context, req *GetUserRewardAggReq) (*GetUserRewardAggRes, error) {
	walletAddress := strings.ToLower(req.WalletAddress)
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil || userEntity == nil {
		return &GetUserRewardAggRes{}, nil
	}
	userID := userEntity.Id

	// direct: cobo_node_purchase 直推 + staking_v2 直推
	directAmount, _ := g.DB().GetValue(ctx, `
		SELECT COALESCE(SUM(amount), 0)::text FROM (
			SELECT direct_reward_amount AS amount FROM cobo_node_purchase
			WHERE direct_reward_user_id = ? AND direct_reward_amount > 0 AND is_gift = 0
			UNION ALL
			SELECT amount FROM cobo_balance_change_log
			WHERE user_id = ? AND change_type = ?
		) t
	`, userID, userID, consts.ChangeTypeStakingV2ReferralDirect)
	directCount, _ := g.DB().GetValue(ctx, `
		SELECT COUNT(*) FROM (
			SELECT id FROM cobo_node_purchase WHERE direct_reward_user_id = ? AND direct_reward_amount > 0 AND is_gift = 0
			UNION ALL
			SELECT id FROM cobo_balance_change_log WHERE user_id = ? AND change_type = ?
		) t
	`, userID, userID, consts.ChangeTypeStakingV2ReferralDirect)

	// indirect: staking_v2 间推
	indirectAmount, _ := g.DB().GetValue(ctx, `
		SELECT COALESCE(SUM(amount), 0)::text FROM cobo_balance_change_log
		WHERE user_id = ? AND change_type = ?
	`, userID, consts.ChangeTypeStakingV2ReferralIndirect)
	indirectCount, _ := g.DB().Ctx(ctx).Model("cobo_balance_change_log").
		Where("user_id = ? AND change_type = ?", userID, consts.ChangeTypeStakingV2ReferralIndirect).Count()

	// team: 拼团团队奖
	teamAmount, _ := g.DB().GetValue(ctx, `
		SELECT COALESCE(SUM(granted_amount), 0)::text FROM group_match_team_reward_distribution
		WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
	`, userID)
	teamCount, _ := g.DB().Ctx(ctx).Model("group_match_team_reward_distribution").
		Where("user_id = ? AND COALESCE(granted_amount, 0) > 0", userID).Count()

	// leadership: 领导奖 diff + weight
	leadershipAmount, _ := g.DB().GetValue(ctx, `
		SELECT COALESCE(SUM(amount), 0)::text FROM (
			SELECT granted_amount AS amount FROM group_purchase_leadership_reward_detail
			WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
			UNION ALL
			SELECT granted_amount AS amount FROM group_purchase_leadership_weight_reward_detail
			WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
		) t
	`, userID, userID)
	leadershipCount, _ := g.DB().GetValue(ctx, `
		SELECT COUNT(*) FROM (
			SELECT id FROM group_purchase_leadership_reward_detail WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
			UNION ALL
			SELECT id FROM group_purchase_leadership_weight_reward_detail WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
		) t
	`, userID, userID)

	// match_reward: 拼团赢家奖
	matchAmount, _ := g.DB().GetValue(ctx, `
		SELECT COALESCE(SUM(session_reward), 0)::text FROM (
			SELECT COALESCE(SUM(reward_amount), 0) AS session_reward
			FROM group_match_order
			WHERE user_id = ? AND is_winner = true
			GROUP BY session_id
			HAVING COALESCE(SUM(reward_amount), 0) > 0
		) t
	`, userID)
	matchCount, _ := g.DB().GetValue(ctx, `
		SELECT COUNT(*) FROM (
			SELECT session_id FROM group_match_order
			WHERE user_id = ? AND is_winner = true
			GROUP BY session_id
			HAVING COALESCE(SUM(reward_amount), 0) > 0
		) t
	`, userID)

	// us_stock: 美股奖励
	usStockAmount, _ := g.DB().GetValue(ctx, `
		SELECT COALESCE(SUM(amount), 0)::text FROM us_stock_reward_settlement
		WHERE user_id = ? AND amount > 0
	`, userID)
	usStockCountInt, _ := g.DB().Ctx(ctx).Model("us_stock_reward_settlement").
		Where("user_id = ? AND amount > 0", userID).Count()

	valStr := func(v gdb.Value) string {
		if v == nil {
			return "0"
		}
		s := strings.TrimSpace(v.String())
		if s == "" {
			return "0"
		}
		return s
	}
	valInt64 := func(v gdb.Value) int64 {
		if v == nil {
			return 0
		}
		return v.Int64()
	}

	directAmt := valStr(directAmount)
	indirectAmt := valStr(indirectAmount)
	teamAmt := valStr(teamAmount)
	leadershipAmt := valStr(leadershipAmount)
	matchAmt := valStr(matchAmount)
	usStockAmt := valStr(usStockAmount)

	directCnt := valInt64(directCount)
	indirectCnt := int64(indirectCount)
	teamCnt := int64(teamCount)
	leadershipCnt := valInt64(leadershipCount)
	matchCnt := valInt64(matchCount)
	usStockCnt := int64(usStockCountInt)

	total := decimal.Zero
	total = total.Add(mustDecimal(directAmt))
	total = total.Add(mustDecimal(indirectAmt))
	total = total.Add(mustDecimal(teamAmt))
	total = total.Add(mustDecimal(leadershipAmt))
	total = total.Add(mustDecimal(matchAmt))
	total = total.Add(mustDecimal(usStockAmt))

	return &GetUserRewardAggRes{
		Direct:      RewardAggItem{Amount: directAmt, Count: directCnt},
		Indirect:    RewardAggItem{Amount: indirectAmt, Count: indirectCnt},
		Team:        RewardAggItem{Amount: teamAmt, Count: teamCnt},
		Leadership:  RewardAggItem{Amount: leadershipAmt, Count: leadershipCnt},
		MatchReward: RewardAggItem{Amount: matchAmt, Count: matchCnt},
		USStock:     RewardAggItem{Amount: usStockAmt, Count: usStockCnt},
		Total:       RewardAggItem{Amount: total.String(), Count: directCnt + indirectCnt + teamCnt + leadershipCnt + matchCnt + usStockCnt},
	}, nil
}

func mustDecimal(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}
