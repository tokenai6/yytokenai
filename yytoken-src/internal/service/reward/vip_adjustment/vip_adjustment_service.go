package vip_adjustment

import (
	"context"
	"strings"
	"time"

	rewardEntity "XWFrame/internal/entity/reward"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/repository"
	rewardRepo "XWFrame/internal/repository/reward"
	"XWFrame/internal/service/reward/vip_adjustment/model"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// IVipAdjustmentService VIP调整服务接口
type IVipAdjustmentService interface {
	// CreateAdjustment 创建VIP调整记录
	CreateAdjustment(ctx context.Context, req *model.CreateAdjustmentReq) (*model.CreateAdjustmentRes, error)

	// GetAdjustmentList 获取调整记录列表
	GetAdjustmentList(ctx context.Context, req *model.GetAdjustmentListReq) (*model.GetAdjustmentListRes, error)

	// CancelAdjustment 取消调整记录
	CancelAdjustment(ctx context.Context, req *model.CancelAdjustmentReq) error

	// GetEffectiveVIPLevel 获取用户有效VIP等级（统一获取函数）
	// 业务逻辑：如果存在有效调整记录且调整后等级 > 计算等级，返回调整后等级
	// 如果用户真实VIP等级 > 调整后等级，返回真实等级（调整不能降低等级）
	GetEffectiveVIPLevel(ctx context.Context, userID int64, calculatedLevel int) (int, error)

	// BatchGetEffectiveVIPLevels 批量获取用户有效VIP等级
	BatchGetEffectiveVIPLevels(ctx context.Context, userIDToLevelMap map[int64]int) (map[int64]int, error)
}

// vipAdjustmentService VIP调整服务实现
type vipAdjustmentService struct {
	adjustmentRepo rewardRepo.IUserVipAdjustmentRepository
	userRepo       repository.IUserRepository
	adminRepo      repository.IAdminRepository
}

// NewVipAdjustmentService 创建VIP调整服务实例
func NewVipAdjustmentService() IVipAdjustmentService {
	return &vipAdjustmentService{
		adjustmentRepo: rewardRepo.NewUserVipAdjustmentRepository(),
		userRepo:       repository.NewUserRepository(),
		adminRepo:      repository.NewAdminRepository(),
	}
}

// CreateAdjustment 创建VIP调整记录
func (s *vipAdjustmentService) CreateAdjustment(ctx context.Context, req *model.CreateAdjustmentReq) (*model.CreateAdjustmentRes, error) {
	// 1. 获取用户当前等级（从 user_info.vip_level 读取）
	user, err := s.userRepo.GetUserById(ctx, req.UserID)
	if err != nil {
		return nil, gerror.Wrap(err, "查询用户信息失败")
	}
	if user == nil {
		return nil, gerror.New("用户不存在")
	}

	originalLevel := user.VipLevel

	// 2. 查询是否已有有效调整记录（如果没有记录，这是正常情况，不报错）
	now := time.Now()
	existingAdjustment, err := s.adjustmentRepo.GetActiveAdjustment(ctx, req.UserID, now)
	if err != nil {
		// 查询失败时记录日志但不报错，继续执行
		g.Log().Warningf(ctx, "查询用户VIP调整记录失败，继续创建: userID=%d, err=%v", req.UserID, err)
	}

	// 3. 创建调整记录
	adjustTime := time.Now()
	expireTime := adjustTime.Add(30 * 24 * time.Hour) // 30天后过期

	adjustment := &rewardEntity.UserVipAdjustmentEntity{
		UserID:           req.UserID,
		AdjustedVipLevel: req.AdjustedVipLevel,
		OriginalVipLevel: originalLevel,
		AdjustTime:       adjustTime,
		ExpireTime:       expireTime,
		OperatorID:       req.OperatorID,
		Remark:           req.Remark,
	}

	err = db.GetDB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 如果已有生效中的调整记录，先在事务内将其失效，再创建新记录
		if existingAdjustment != nil {
			if err := s.adjustmentRepo.CancelAdjustment(ctx, tx, existingAdjustment.Id, now); err != nil {
				return err
			}
		}

		// 创建调整记录
		if err := s.adjustmentRepo.CreateAdjustment(ctx, tx, adjustment); err != nil {
			return err
		}

		if _, err := tx.Model("user_info").Ctx(ctx).
			Where("id", req.UserID).
			Data(map[string]interface{}{"vip_level": req.AdjustedVipLevel}).
			Update(); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, gerror.Wrap(err, "创建调整记录失败")
	}

	return &model.CreateAdjustmentRes{
		ID:               adjustment.Id,
		UserID:           adjustment.UserID,
		AdjustedVipLevel: adjustment.AdjustedVipLevel,
		OriginalVipLevel: adjustment.OriginalVipLevel,
		AdjustTime:       adjustment.AdjustTime,
		ExpireTime:       adjustment.ExpireTime,
	}, nil
}

// GetAdjustmentList 获取调整记录列表
func (s *vipAdjustmentService) GetAdjustmentList(ctx context.Context, req *model.GetAdjustmentListReq) (*model.GetAdjustmentListRes, error) {
	// 设置默认分页参数
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 如果传了钱包地址，先通过钱包地址查询用户ID
	userID := req.UserID
	if userID == 0 && req.WalletAddress != "" {
		walletAddress := strings.ToLower(strings.TrimSpace(req.WalletAddress))
		if walletAddress != "" {
			user, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
			if err != nil {
				g.Log().Warningf(ctx, "通过钱包地址查询用户失败: wallet_address=%s, err=%v", walletAddress, err)
				// 用户不存在，返回空列表
				return &model.GetAdjustmentListRes{
					Page:     page,
					PageSize: pageSize,
					Total:    0,
					Pages:    1,
					List:     []model.AdjustmentItem{},
				}, nil
			}
			if user == nil {
				// 用户不存在，返回空列表
				return &model.GetAdjustmentListRes{
					Page:     page,
					PageSize: pageSize,
					Total:    0,
					Pages:    1,
					List:     []model.AdjustmentItem{},
				}, nil
			}
			userID = user.Id
		}
	}

	adjustments, total, err := s.adjustmentRepo.GetAdjustmentList(ctx, userID, page, pageSize)
	if err != nil {
		return nil, gerror.Wrap(err, "获取调整记录列表失败")
	}

	// 批量获取用户信息（钱包地址）
	userIDs := make([]int64, 0, len(adjustments))
	for _, adj := range adjustments {
		userIDs = append(userIDs, adj.UserID)
	}
	userMap := make(map[int64]string) // userID -> walletAddress
	if len(userIDs) > 0 {
		users, err := s.userRepo.GetDataByIds(ctx, userIDs)
		if err != nil {
			g.Log().Warningf(ctx, "批量获取用户信息失败: err=%v", err)
		} else {
			for _, user := range users {
				if user != nil {
					userMap[user.Id] = user.WalletAddress
				}
			}
		}
	}

	// 批量获取管理员信息（操作员用户名）
	operatorIDs := make([]int64, 0, len(adjustments))
	for _, adj := range adjustments {
		if adj.OperatorID > 0 {
			operatorIDs = append(operatorIDs, adj.OperatorID)
		}
	}
	adminMap := make(map[int64]string) // operatorID -> username
	if len(operatorIDs) > 0 {
		admins, err := s.adminRepo.GetDataByIds(ctx, operatorIDs)
		if err != nil {
			g.Log().Warningf(ctx, "批量获取管理员信息失败: err=%v", err)
		} else {
			for _, admin := range admins {
				if admin != nil {
					adminMap[admin.Id] = admin.Username
				}
			}
		}
	}

	now := time.Now()
	var list []model.AdjustmentItem
	for _, adj := range adjustments {
		isValid := adj.IsValid(now)
		walletAddress := userMap[adj.UserID]
		operatorName := adminMap[adj.OperatorID]
		list = append(list, model.AdjustmentItem{
			ID:               adj.Id,
			UserID:           adj.UserID,
			WalletAddress:    walletAddress,
			AdjustedVipLevel: adj.AdjustedVipLevel,
			OriginalVipLevel: adj.OriginalVipLevel,
			AdjustTime:       adj.AdjustTime,
			ExpireTime:       adj.ExpireTime,
			OperatorID:       adj.OperatorID,
			OperatorName:     operatorName,
			Remark:           adj.Remark,
			IsValid:          isValid,
		})
	}

	pages := (total + pageSize - 1) / pageSize
	if pages == 0 {
		pages = 1
	}

	return &model.GetAdjustmentListRes{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		Pages:    pages,
		List:     list,
	}, nil
}

// CancelAdjustment 取消调整记录
func (s *vipAdjustmentService) CancelAdjustment(ctx context.Context, req *model.CancelAdjustmentReq) error {
	// 1. 验证记录是否存在
	adjustment, err := s.adjustmentRepo.GetAdjustmentByID(ctx, req.ID)
	if err != nil {
		return gerror.Wrap(err, "查询调整记录失败")
	}

	if adjustment == nil {
		return gerror.New("调整记录不存在")
	}

	// 2. 检查是否已过期（已取消的记录过期时间会被设置为当前时间之前）
	now := time.Now()
	if !adjustment.IsValid(now) {
		return gerror.New("该调整记录已过期或已取消")
	}

	// 3. 取消调整记录（将过期时间设置为当前时间）
	err = db.GetDB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 取消调整记录
		if err := s.adjustmentRepo.CancelAdjustment(ctx, tx, req.ID, now); err != nil {
			return err
		}

		if _, err := tx.Model("user_info").Ctx(ctx).
			Where("id", adjustment.UserID).
			Data(map[string]interface{}{"vip_level": adjustment.OriginalVipLevel}).
			Update(); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return gerror.Wrap(err, "取消调整记录失败")
	}

	return nil
}

// GetEffectiveVIPLevel 获取用户有效VIP等级（统一获取函数）
// 业务逻辑：
// 1. 查询用户有效调整记录
// 2. 如果存在有效调整记录且调整后等级 > 计算等级，使用调整后等级
// 3. 如果用户真实VIP等级 > 调整后等级，使用真实等级（调整不能降低等级）
// 注意：如果查询调整记录失败或不存在，返回计算等级，不报错
func (s *vipAdjustmentService) GetEffectiveVIPLevel(ctx context.Context, userID int64, calculatedLevel int) (int, error) {
	now := time.Now()

	// 1. 查询有效调整记录（如果查询失败，记录日志但不报错，使用计算等级）
	adjustment, err := s.adjustmentRepo.GetActiveAdjustment(ctx, userID, now)
	if err != nil {
		g.Log().Warningf(ctx, "查询用户VIP调整记录失败，使用计算等级: userID=%d, err=%v", userID, err)
		return calculatedLevel, nil
	}

	// 2. 如果没有调整记录，直接返回计算等级
	if adjustment == nil {
		return calculatedLevel, nil
	}

	// 3. 如果调整后等级 <= 计算等级，使用计算等级（真实等级更高）
	if adjustment.AdjustedVipLevel <= calculatedLevel {
		return calculatedLevel, nil
	}

	// 4. 使用调整后等级
	return adjustment.AdjustedVipLevel, nil
}

// BatchGetEffectiveVIPLevels 批量获取用户有效VIP等级
func (s *vipAdjustmentService) BatchGetEffectiveVIPLevels(ctx context.Context, userIDToLevelMap map[int64]int) (map[int64]int, error) {
	if len(userIDToLevelMap) == 0 {
		return make(map[int64]int), nil
	}

	now := time.Now()

	// 1. 批量获取有效调整记录
	userIDs := make([]int64, 0, len(userIDToLevelMap))
	for userID := range userIDToLevelMap {
		userIDs = append(userIDs, userID)
	}

	adjustments, err := s.adjustmentRepo.BatchGetActiveAdjustments(ctx, userIDs, now)
	if err != nil {
		// 批量查询失败时，记录日志但不报错，所有用户都使用计算等级
		g.Log().Warningf(ctx, "批量查询VIP调整记录失败，所有用户使用计算等级: err=%v", err)
		// 返回所有用户都使用计算等级的结果
		result := make(map[int64]int)
		for userID, calculatedLevel := range userIDToLevelMap {
			result[userID] = calculatedLevel
		}
		return result, nil
	}

	// 2. 构建结果映射
	result := make(map[int64]int)
	for userID, calculatedLevel := range userIDToLevelMap {
		adjustment, hasAdjustment := adjustments[userID]

		// 如果没有调整记录，使用计算等级
		if !hasAdjustment || adjustment == nil {
			result[userID] = calculatedLevel
			continue
		}

		// 如果调整后等级 <= 计算等级，使用计算等级（真实等级更高）
		if adjustment.AdjustedVipLevel <= calculatedLevel {
			result[userID] = calculatedLevel
			continue
		}

		// 使用调整后等级
		result[userID] = adjustment.AdjustedVipLevel
	}

	return result, nil
}
