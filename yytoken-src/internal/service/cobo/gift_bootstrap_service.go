package cobo

import (
	"context"
	"fmt"
	"strings"
	"time"

	teamDao "XWFrame/internal/dao/team"
	"XWFrame/internal/entity"
	teamEntity "XWFrame/internal/entity/team"
	"XWFrame/internal/repository"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/frame/g"
)

const giftDefaultParentID int64 = 1
const giftDefaultTeamID int64 = 13

type GiftBootstrapReq struct {
	WalletAddress  string
	NodeType       int
	AdminID        int64
	Remark         string
	TeamName       string
	Force          bool
	EnableExempt   bool
}

type GiftBootstrapRes struct {
	UserID          int64
	WalletAddress   string
	PackageNo       string
	NodeType        int
	Amount          string
	PowerValue      string
	UserCreated     bool
	TeamCreated     bool
	TeamSyncedCount int
	ExemptUpdated   bool
}

type GiftBootstrapService interface {
	EnsureAndGift(ctx context.Context, req *GiftBootstrapReq) (*GiftBootstrapRes, error)
}

type giftBootstrapService struct{}

func NewGiftBootstrapService() GiftBootstrapService {
	return &giftBootstrapService{}
}

func (s *giftBootstrapService) EnsureAndGift(ctx context.Context, req *GiftBootstrapReq) (*GiftBootstrapRes, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	walletAddress := normalizeGiftWallet(req.WalletAddress)
	if walletAddress == "" {
		return nil, fmt.Errorf("wallet_address is empty")
	}

	user, userCreated, err := s.ensureUserExists(ctx, walletAddress)
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

	teamCreated, err := s.ensureTeamExists(ctx, walletAddress, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("确保团队存在失败: %w", err)
	}

	fixedCount, err := s.syncTeamIDByNearestRuleForSubtree(ctx, walletAddress)
	if err != nil {
		return nil, fmt.Errorf("同步用户及下级 team_id 失败: %w", err)
	}

	if !req.Force {
		exists, err := g.DB().Model("cobo_node_purchase").Ctx(ctx).
			Where("user_id = ? AND is_gift = 1", user.Id).
			One()
		if err != nil {
			return nil, fmt.Errorf("检查已有赠送记录失败: %w", err)
		}
		if exists != nil {
			return nil, fmt.Errorf("用户 %d 已存在赠送节点记录，如需重复赠送请使用 --force", user.Id)
		}
	}

	giftRes, err := NewAdminNodeGiftService().GiftNode(ctx, &GiftNodeReq{
		AdminID:  req.AdminID,
		UserID:   user.Id,
		NodeType: req.NodeType,
		Remark:   req.Remark,
	})
	if err != nil {
		return nil, fmt.Errorf("赠送节点服务调用失败: %w", err)
	}

	return &GiftBootstrapRes{
		UserID:          user.Id,
		WalletAddress:   user.WalletAddress,
		PackageNo:       giftRes.PackageNo,
		NodeType:        giftRes.NodeType,
		Amount:          giftRes.Amount,
		PowerValue:      giftRes.PowerValue,
		UserCreated:     userCreated,
		TeamCreated:     teamCreated,
		TeamSyncedCount: fixedCount,
		ExemptUpdated:   exemptUpdated,
	}, nil
}

func (s *giftBootstrapService) ensureUserExists(ctx context.Context, walletAddress string) (*entity.UserEntity, bool, error) {
	userRepo := repository.NewUserRepository()

	user, err := userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			user = nil
		} else {
			return nil, false, fmt.Errorf("查询用户失败: %w", err)
		}
	}

	if user != nil {
		if user.ParentWalletAddress != "" && user.ParentWalletAddress != "0x00" {
			parentUser, err := userRepo.GetUserByWalletAddress(ctx, user.ParentWalletAddress)
			if err == nil && parentUser != nil {
				var bindTeamID int64
				bindTeamName := ""

				if parentUser.TeamId != nil {
					bindTeamID = *parentUser.TeamId
					bindTeamName = parentUser.TeamName
				}

				teamRepo := teamDao.NewTeamDao()
				if parentTeam, teamErr := teamRepo.GetByLeaderAddress(ctx, parentUser.WalletAddress); teamErr == nil && parentTeam != nil {
					bindTeamID = parentTeam.Id
					bindTeamName = parentTeam.Name
				}

				if bindTeamID > 0 && (user.TeamId == nil || *user.TeamId != bindTeamID) {
					updateData := map[string]interface{}{
						"team_id":    bindTeamID,
						"team_name":  bindTeamName,
						"updated_at": time.Now(),
					}
					if err := userRepo.UpdateUser(ctx, user.Id, updateData); err != nil {
						return nil, false, fmt.Errorf("更新用户 team_id 失败: %w", err)
					}

					user, err = userRepo.GetUserByWalletAddress(ctx, walletAddress)
					if err != nil {
						return nil, false, fmt.Errorf("查询更新后的用户失败: %w", err)
					}
				}
			}
		} else {
			parentUser, err := userRepo.GetUserById(ctx, giftDefaultParentID)
			if err == nil && parentUser != nil {
				updateData := map[string]interface{}{
					"parent_wallet_address": parentUser.WalletAddress,
					"team_id":               giftDefaultTeamID,
					"team_name":             "顶号",
					"updated_at":            time.Now(),
				}
				if err := userRepo.UpdateUser(ctx, user.Id, updateData); err != nil {
					return nil, false, fmt.Errorf("更新用户团队信息失败: %w", err)
				}

				user, err = userRepo.GetUserByWalletAddress(ctx, walletAddress)
				if err != nil {
					return nil, false, fmt.Errorf("查询更新后的用户失败: %w", err)
				}
			}
		}

		return user, false, nil
	}

	parentUser, err := userRepo.GetUserById(ctx, giftDefaultParentID)
	if err != nil {
		return nil, false, fmt.Errorf("获取默认上级用户失败: %w", err)
	}
	if parentUser == nil {
		return nil, false, fmt.Errorf("默认上级用户(ID=%d)不存在", giftDefaultParentID)
	}

	inviteCode, err := s.generateUniqueInviteCode(ctx, userRepo)
	if err != nil {
		return nil, false, fmt.Errorf("生成邀请码失败: %w", err)
	}

	teamID := giftDefaultTeamID
	newUser := &entity.UserEntity{
		WalletAddress:       walletAddress,
		ParentWalletAddress: parentUser.WalletAddress,
		InviteCode:          inviteCode,
		TeamId:              &teamID,
		TeamName:            "顶号",
		Status:              1,
		CanWithdraw:         true,
	}

	if err := userRepo.CreateUser(ctx, newUser); err != nil {
		return nil, false, fmt.Errorf("创建用户失败: %w", err)
	}

	user, err = userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil {
		return nil, false, fmt.Errorf("查询新创建用户失败: %w", err)
	}

	return user, true, nil
}

func (s *giftBootstrapService) ensureTeamExists(ctx context.Context, walletAddress string, customTeamName string) (bool, error) {
	teamRepo := teamDao.NewTeamDao()

	existingTeam, err := teamRepo.GetByLeaderAddress(ctx, walletAddress)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			existingTeam = nil
		} else {
			return false, fmt.Errorf("查询团队失败: %w", err)
		}
	}

	if existingTeam != nil {
		return false, nil
	}

	finalTeamName := strings.TrimSpace(customTeamName)
	if finalTeamName == "" {
		finalTeamName = strings.ToUpper(walletAddress[len(walletAddress)-6:])
	}

	newTeam := &teamEntity.TeamEntity{
		Name:                finalTeamName,
		LeaderWalletAddress: walletAddress,
		Status:              1,
	}

	if err := teamRepo.Create(ctx, nil, newTeam); err != nil {
		return false, fmt.Errorf("创建团队失败: %w", err)
	}

	return true, nil
}

func (s *giftBootstrapService) syncTeamIDByNearestRuleForSubtree(ctx context.Context, walletAddress string) (int, error) {
	users, err := s.loadAllUsersForTeamSync(ctx)
	if err != nil {
		return 0, fmt.Errorf("加载用户失败: %w", err)
	}

	teams, err := s.loadAllTeamsForTeamSync(ctx)
	if err != nil {
		return 0, fmt.Errorf("加载团队失败: %w", err)
	}

	userMap := make(map[string]*giftUserTeamInfo, len(users))
	childrenMap := make(map[string][]*giftUserTeamInfo)
	for i := range users {
		u := &users[i]
		wallet := normalizeGiftWallet(u.WalletAddress)
		parent := normalizeGiftWallet(u.ParentWalletAddress)
		userMap[wallet] = u
		childrenMap[parent] = append(childrenMap[parent], u)
	}

	leaderMap := make(map[string]giftTeamInfo, len(teams))
	for _, t := range teams {
		leaderMap[normalizeGiftWallet(t.LeaderWalletAddress)] = t
	}

	targetWallet := normalizeGiftWallet(walletAddress)
	if _, ok := userMap[targetWallet]; !ok {
		return 0, fmt.Errorf("目标用户不存在: %s", walletAddress)
	}

	affected := collectGiftSubtreeUsers(targetWallet, userMap, childrenMap)
	if len(affected) == 0 {
		return 0, nil
	}

	userRepo := repository.NewUserRepository()
	memo := make(map[int64]*giftTeamAssignment)
	fixCount := 0

	for _, u := range affected {
		expected := calculateGiftExpectedTeamID(u, userMap, leaderMap, memo, make(map[int64]bool))
		if sameGiftTeamID(u.TeamID, expected.TeamID) {
			continue
		}

		updateData := map[string]interface{}{
			"team_id":    expected.TeamID,
			"updated_at": time.Now(),
		}
		if expected.TeamName != "" {
			updateData["team_name"] = expected.TeamName
		}

		if err := userRepo.UpdateUser(ctx, u.ID, updateData); err != nil {
			return 0, fmt.Errorf("更新用户 %d team_id 失败: %w", u.ID, err)
		}

		fixCount++
		u.TeamID = cloneGiftInt64Ptr(expected.TeamID)
		if expected.TeamName != "" {
			u.TeamName = expected.TeamName
		}
	}

	return fixCount, nil
}

type giftUserTeamInfo struct {
	ID                  int64  `json:"id"`
	WalletAddress       string `json:"wallet_address"`
	ParentWalletAddress string `json:"parent_wallet_address"`
	TeamID              *int64 `json:"team_id"`
	TeamName            string `json:"team_name"`
}

type giftTeamAssignment struct {
	TeamID   *int64
	TeamName string
	Reason   string
}

type giftTeamInfo struct {
	ID                  int64  `json:"id"`
	Name                string `json:"name"`
	LeaderWalletAddress string `json:"leader_wallet_address"`
}

func (s *giftBootstrapService) loadAllUsersForTeamSync(ctx context.Context) ([]giftUserTeamInfo, error) {
	var users []giftUserTeamInfo
	err := g.DB().Model("user_info").Ctx(ctx).
		Fields("id, wallet_address, parent_wallet_address, team_id, team_name").
		Scan(&users)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (s *giftBootstrapService) loadAllTeamsForTeamSync(ctx context.Context) ([]giftTeamInfo, error) {
	var teams []giftTeamInfo
	err := g.DB().Model("team").Ctx(ctx).
		Fields("id, name, leader_wallet_address").
		Scan(&teams)
	if err != nil {
		return nil, err
	}
	return teams, nil
}

func collectGiftSubtreeUsers(targetWallet string, userMap map[string]*giftUserTeamInfo, childrenMap map[string][]*giftUserTeamInfo) []*giftUserTeamInfo {
	queue := []string{targetWallet}
	seen := make(map[string]bool)
	result := make([]*giftUserTeamInfo, 0)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if seen[current] {
			continue
		}
		seen[current] = true

		if u, ok := userMap[current]; ok {
			result = append(result, u)
		}

		for _, child := range childrenMap[current] {
			if child == nil {
				continue
			}
			queue = append(queue, normalizeGiftWallet(child.WalletAddress))
		}
	}

	return result
}

func calculateGiftExpectedTeamID(
	user *giftUserTeamInfo,
	userMap map[string]*giftUserTeamInfo,
	leaderMap map[string]giftTeamInfo,
	memo map[int64]*giftTeamAssignment,
	visiting map[int64]bool,
) *giftTeamAssignment {
	if assignment, ok := memo[user.ID]; ok {
		return assignment
	}

	if visiting[user.ID] {
		assignment := &giftTeamAssignment{Reason: "检测到上级循环"}
		memo[user.ID] = assignment
		return assignment
	}
	visiting[user.ID] = true
	defer delete(visiting, user.ID)

	if isGiftRootParent(user.ParentWalletAddress) {
		assignment := &giftTeamAssignment{Reason: "根节点"}
		memo[user.ID] = assignment
		return assignment
	}

	parentUser, ok := userMap[normalizeGiftWallet(user.ParentWalletAddress)]
	if !ok {
		assignment := &giftTeamAssignment{Reason: "上级用户不存在"}
		memo[user.ID] = assignment
		return assignment
	}

	if team, ok := leaderMap[normalizeGiftWallet(parentUser.WalletAddress)]; ok {
		teamID := team.ID
		assignment := &giftTeamAssignment{TeamID: &teamID, TeamName: team.Name, Reason: "直接上级是团队长"}
		memo[user.ID] = assignment
		return assignment
	}

	parentAssignment := calculateGiftExpectedTeamID(parentUser, userMap, leaderMap, memo, visiting)
	if parentAssignment.TeamID != nil {
		assignment := &giftTeamAssignment{TeamID: cloneGiftInt64Ptr(parentAssignment.TeamID), TeamName: parentAssignment.TeamName, Reason: "继承直接上级团队"}
		memo[user.ID] = assignment
		return assignment
	}

	assignment := &giftTeamAssignment{Reason: "上级链路无法确定"}
	memo[user.ID] = assignment
	return assignment
}

func normalizeGiftWallet(wallet string) string {
	return strings.ToLower(strings.TrimSpace(wallet))
}

func isGiftRootParent(parentWallet string) bool {
	addr := normalizeGiftWallet(parentWallet)
	if addr == "" {
		return true
	}
	addr = strings.TrimPrefix(addr, "0x")
	if addr == "" {
		return true
	}
	for _, c := range addr {
		if c != '0' {
			return false
		}
	}
	return true
}

func sameGiftTeamID(current *int64, expected *int64) bool {
	if current == nil || expected == nil {
		return current == nil && expected == nil
	}
	return *current == *expected
}

func cloneGiftInt64Ptr(v *int64) *int64 {
	if v == nil {
		return nil
	}
	vv := *v
	return &vv
}

func (s *giftBootstrapService) generateUniqueInviteCode(ctx context.Context, userRepo repository.IUserRepository) (string, error) {
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		inviteCode := utils.GenerateInviteCode()
		exists, err := userRepo.CheckInviteCodeExists(ctx, inviteCode)
		if err != nil {
			return "", fmt.Errorf("检查邀请码唯一性失败: %v", err)
		}
		if !exists {
			return inviteCode, nil
		}
	}
	return "", fmt.Errorf("无法生成唯一邀请码，已重试 %d 次", maxRetries)
}
