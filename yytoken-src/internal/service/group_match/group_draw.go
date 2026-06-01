package group_match

import (
	"context"
	"crypto/rand"
	"math/big"
	"sync"

	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

type settlementPlanKind int

const (
	planKindIndependent settlementPlanKind = 1
	planKindMixed       settlementPlanKind = 2
	planKindFlow        settlementPlanKind = 3
)

type settlementPlan struct {
	Kind       settlementPlanKind
	Orders     []*settlementOrderRow
	LoserIndex int
}

type planQueue struct {
	mu     sync.Mutex
	plans  []*settlementPlan
	cursor int
}

func newPlanQueue(plans []*settlementPlan) *planQueue {
	return &planQueue{plans: plans}
}

func (q *planQueue) take(n int) []*settlementPlan {
	if n <= 0 {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cursor >= len(q.plans) {
		return nil
	}
	end := q.cursor + n
	end = min(end, len(q.plans))
	batch := q.plans[q.cursor:end]
	q.cursor = end
	return batch
}

func secureRandInt(max int) (int, error) {
	if max <= 0 {
		return 0, gerror.New("invalid max value")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func secureShuffle(orders []*settlementOrderRow) error {
	for i := len(orders) - 1; i > 0; i-- {
		j, err := secureRandInt(i + 1)
		if err != nil {
			return err
		}
		orders[i], orders[j] = orders[j], orders[i]
	}
	return nil
}

func secureShuffleInt64(values []int64) error {
	for i := len(values) - 1; i > 0; i-- {
		j, err := secureRandInt(i + 1)
		if err != nil {
			return err
		}
		values[i], values[j] = values[j], values[i]
	}
	return nil
}

// buildSettlementPlan 全局预分桶：
//  1. 按 user_id 聚合所有 pending 订单
//  2. 每个用户先切 floor(n/10) 个 independent 组（每组必输 1 单）
//  3. 余单进入 scatter pool，随机打散后每 10 条组成 mixed 组，尾部不足 10 条作为 flow 组退款
//  4. mixed 组在这里预先分配 loser（写入 LoserIndex），并强制单用户总 lose 上限为 ceil(n/10)
//     - 例如: 11-19 单最多 lose 2，21-29 单最多 lose 3，31-39 单最多 lose 4，41-49 单最多 lose 5
//     - 分配策略采用“按组-用户二分匹配”，避免出现某组没有可用 loser 的死局
//  5. 整体 plan 列表再次打散，避免 worker 总是先扎堆某个用户
func (s *groupMatchService) buildSettlementPlan(ctx context.Context, sessionID int64) ([]*settlementPlan, error) {
	orders := make([]*settlementOrderRow, 0)
	err := g.DB().Ctx(ctx).Raw(`
		SELECT id, user_id, order_no, amount, ticket_symbol, ticket_amount, ticket_usdt_value, created_at, status, group_id
		FROM group_match_order
		WHERE session_id = ? AND status = ?
		ORDER BY id ASC
	`, sessionID, consts.GroupMatchOrderStatusPending).Scan(&orders)
	if err != nil {
		return nil, gerror.Wrap(err, "load session pending orders failed")
	}
	if len(orders) == 0 {
		return nil, nil
	}

	userOrders := make(map[int64][]*settlementOrderRow, 64)
	userTotalOrderCount := make(map[int64]int, 64)
	userSeq := make([]int64, 0, 64)
	for _, o := range orders {
		if _, ok := userOrders[o.UserID]; !ok {
			userSeq = append(userSeq, o.UserID)
		}
		userOrders[o.UserID] = append(userOrders[o.UserID], o)
		userTotalOrderCount[o.UserID]++
	}

	plans := make([]*settlementPlan, 0, len(orders)/10+1)
	scatter := make([]*settlementOrderRow, 0)
	independentUserCount := 0
	independentGroupCount := 0
	for _, uid := range userSeq {
		list := userOrders[uid]
		if len(list)%10 == 0 {
			independentUserCount++
			for i := 0; i < len(list); i += 10 {
				plans = append(plans, &settlementPlan{
					Kind:       planKindIndependent,
					Orders:     list[i : i+10],
					LoserIndex: -1,
				})
				independentGroupCount++
			}
			continue
		}
		// 非 10 整数倍：整数部分进 independent
		independentCount := len(list) / 10
		if independentCount > 0 {
			independentUserCount++
			for i := 0; i < independentCount; i++ {
				plans = append(plans, &settlementPlan{
					Kind:       planKindIndependent,
					Orders:     list[i*10 : (i+1)*10],
					LoserIndex: -1,
				})
				independentGroupCount++
			}
		}
		// 余单进入 scatter pool
		scatter = append(scatter, list[independentCount*10:]...)
	}

	if err := secureShuffle(scatter); err != nil {
		return nil, gerror.Wrap(err, "shuffle scatter pool failed")
	}

	mixedPlans := make([]*settlementPlan, 0, len(scatter)/10)
	mixedGroupCount := 0
	flowOrderCount := 0
	for i := 0; i+10 <= len(scatter); i += 10 {
		mixedPlans = append(mixedPlans, &settlementPlan{
			Kind:       planKindMixed,
			Orders:     scatter[i : i+10],
			LoserIndex: -1,
		})
		mixedGroupCount++
	}

	// mixed 阶段每个非整数倍用户最多再 lose 1 次（cap=ceil(n/10), used=floor(n/10)）
	userMixedQuota := make(map[int64]int, len(userTotalOrderCount))
	for uid, total := range userTotalOrderCount {
		userMixedQuota[uid] = (total+9)/10 - total/10
	}

	mixedCandidates := make([][]int64, len(mixedPlans))
	for gi, mixedPlan := range mixedPlans {
		seen := make(map[int64]struct{}, len(mixedPlan.Orders))
		for _, order := range mixedPlan.Orders {
			if userMixedQuota[order.UserID] <= 0 {
				continue
			}
			if _, ok := seen[order.UserID]; ok {
				continue
			}
			seen[order.UserID] = struct{}{}
			mixedCandidates[gi] = append(mixedCandidates[gi], order.UserID)
		}
		if len(mixedCandidates[gi]) == 0 {
			return nil, gerror.Newf("mixed group has no eligible loser candidate: session_id=%d group_index=%d", sessionID, gi)
		}
		if err := secureShuffleInt64(mixedCandidates[gi]); err != nil {
			return nil, gerror.Wrap(err, "shuffle mixed candidates failed")
		}
	}

	userAssignedGroup := make(map[int64]int, len(userTotalOrderCount))
	groupAssignedUser := make([]int64, len(mixedPlans))
	var tryMatchGroup func(groupIdx int, seenUser map[int64]bool) bool
	tryMatchGroup = func(groupIdx int, seenUser map[int64]bool) bool {
		for _, uid := range mixedCandidates[groupIdx] {
			if seenUser[uid] {
				continue
			}
			seenUser[uid] = true
			prevGroup, occupied := userAssignedGroup[uid]
			if !occupied || tryMatchGroup(prevGroup, seenUser) {
				userAssignedGroup[uid] = groupIdx
				groupAssignedUser[groupIdx] = uid
				return true
			}
		}
		return false
	}

	for gi := range mixedPlans {
		if !tryMatchGroup(gi, make(map[int64]bool, len(userTotalOrderCount))) {
			return nil, gerror.Newf("mixed loser assignment failed under lose cap: session_id=%d group_index=%d", sessionID, gi)
		}
	}

	for gi, mixedPlan := range mixedPlans {
		loserUserID := groupAssignedUser[gi]
		if loserUserID == 0 {
			return nil, gerror.Newf("mixed loser assignment missing: session_id=%d group_index=%d", sessionID, gi)
		}
		candidateIndexes := make([]int, 0, len(mixedPlan.Orders))
		for idx, order := range mixedPlan.Orders {
			if order.UserID == loserUserID {
				candidateIndexes = append(candidateIndexes, idx)
			}
		}
		if len(candidateIndexes) == 0 {
			return nil, gerror.Newf("mixed loser user not in group orders: session_id=%d group_index=%d user_id=%d", sessionID, gi, loserUserID)
		}
		pick, err := secureRandInt(len(candidateIndexes))
		if err != nil {
			return nil, gerror.Wrap(err, "pick mixed loser order index failed")
		}
		mixedPlan.LoserIndex = candidateIndexes[pick]
	}
	plans = append(plans, mixedPlans...)

	tail := len(scatter) % 10
	if tail > 0 {
		plans = append(plans, &settlementPlan{
			Kind:       planKindFlow,
			Orders:     scatter[len(scatter)-tail:],
			LoserIndex: -1,
		})
		flowOrderCount = tail
	}

	for i := len(plans) - 1; i > 0; i-- {
		j, err := secureRandInt(i + 1)
		if err != nil {
			return nil, gerror.Wrap(err, "shuffle plan list failed")
		}
		plans[i], plans[j] = plans[j], plans[i]
	}

	g.Log().Infof(ctx, "[GroupMatch] settlement plan: session_id=%d total_orders=%d independent_users=%d independent_groups=%d mixed_groups=%d flow_orders=%d",
		sessionID, len(orders), independentUserCount, independentGroupCount, mixedGroupCount, flowOrderCount)

	return plans, nil
}
