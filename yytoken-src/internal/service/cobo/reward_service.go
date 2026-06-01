package cobo

import (
	"context"
	"time"

	"XWFrame/internal/frame/consts"
	"XWFrame/internal/service/cobo/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// RewardService 奖励服务接口
type RewardService interface {
	// GetRewardRecords 获取用户奖励记录
	GetRewardRecords(ctx context.Context, req *model.GetRewardRecordsReq) (*model.GetRewardRecordsRes, error)
	// GetTodayRewardAmount 获取用户今日奖励总金额（口径与 GetRewardRecords 一致）
	GetTodayRewardAmount(ctx context.Context, userID int64, dayStart, dayEnd time.Time) (decimal.Decimal, error)
	// GetTotalRewardAmount 获取用户累计奖励总金额（口径与 GetRewardRecords 一致，不限时间）
	GetTotalRewardAmount(ctx context.Context, userID int64) (decimal.Decimal, error)
}

// rewardService 奖励服务实现
type rewardService struct{}

// NewRewardService 创建奖励服务
func NewRewardService() RewardService {
	return &rewardService{}
}

// GetRewardRecords 获取用户奖励记录
func (s *rewardService) GetRewardRecords(ctx context.Context, req *model.GetRewardRecordsReq) (*model.GetRewardRecordsRes, error) {
	locale := consts.LocaleFromCtx(ctx)

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}

	// 非支持类型直接返回空
	if req.RewardType != "" &&
		req.RewardType != model.RewardTypeDirect &&
		req.RewardType != model.RewardTypeIndirect &&
		req.RewardType != model.RewardTypeTeam &&
		req.RewardType != model.RewardTypeLeadership &&
		req.RewardType != model.RewardTypeLeadershipDiff &&
		req.RewardType != model.RewardTypeLeadershipWeight &&
		req.RewardType != model.RewardTypeMatchReward &&
		req.RewardType != model.RewardTypeUSStock {
		return &model.GetRewardRecordsRes{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    0,
			Pages:    0,
			List:     []*model.RewardRecord{},
		}, nil
	}

	var total int64
	switch req.RewardType {
	case model.RewardTypeDirect:
		nodeCount, err := g.DB().Ctx(ctx).Model("cobo_node_purchase").Where("direct_reward_user_id = ? AND direct_reward_amount > 0 AND is_gift = 0", req.UserID).Count()
		if err != nil {
			return nil, err
		}
		stakeCount, err := g.DB().Ctx(ctx).Model("cobo_balance_change_log").
			Where("user_id = ?", req.UserID).
			Where("change_type = ?", consts.ChangeTypeStakingV2ReferralDirect).
			Count()
		if err != nil {
			return nil, err
		}
		total = int64(nodeCount + stakeCount)
	case model.RewardTypeIndirect:
		count, err := g.DB().Ctx(ctx).Model("cobo_balance_change_log").
			Where("user_id = ?", req.UserID).
			Where("change_type = ?", consts.ChangeTypeStakingV2ReferralIndirect).
			Count()
		if err != nil {
			return nil, err
		}
		total = int64(count)
	case model.RewardTypeTeam:
		count, err := g.DB().Ctx(ctx).Model("group_match_team_reward_distribution").
			Where("user_id = ?", req.UserID).
			Where("COALESCE(granted_amount, 0) > 0").
			Count()
		if err != nil {
			return nil, err
		}
		total = int64(count)
	case model.RewardTypeLeadership:
		diffCount, err := g.DB().Ctx(ctx).Model("group_purchase_leadership_reward_detail").
			Where("user_id = ?", req.UserID).
			Where("COALESCE(granted_amount, 0) > 0").
			Count()
		if err != nil {
			return nil, err
		}
		weightCount, err := g.DB().Ctx(ctx).Model("group_purchase_leadership_weight_reward_detail").
			Where("user_id = ?", req.UserID).
			Where("COALESCE(granted_amount, 0) > 0").
			Count()
		if err != nil {
			return nil, err
		}
		total = int64(diffCount + weightCount)
	case model.RewardTypeLeadershipDiff:
		diffCount, err := g.DB().Ctx(ctx).Model("group_purchase_leadership_reward_detail").
			Where("user_id = ?", req.UserID).
			Where("COALESCE(granted_amount, 0) > 0").
			Count()
		if err != nil {
			return nil, err
		}
		total = int64(diffCount)
	case model.RewardTypeLeadershipWeight:
		weightCount, err := g.DB().Ctx(ctx).Model("group_purchase_leadership_weight_reward_detail").
			Where("user_id = ?", req.UserID).
			Where("COALESCE(granted_amount, 0) > 0").
			Count()
		if err != nil {
			return nil, err
		}
		total = int64(weightCount)
	case model.RewardTypeMatchReward:
		countVal, err := g.DB().GetValue(ctx, `
			SELECT COUNT(*)
			FROM (
				SELECT o.session_id
				FROM group_match_order o
				WHERE o.user_id = ?
				  AND o.is_winner = true
				GROUP BY o.session_id
				HAVING COALESCE(SUM(o.reward_amount), 0) > 0
			) t
		`, req.UserID)
		if err != nil {
			return nil, err
		}
		total = countVal.Int64()
	case model.RewardTypeUSStock:
		count, err := g.DB().Ctx(ctx).Model("us_stock_reward_settlement").
			Where("user_id = ?", req.UserID).
			Where("amount > 0").
			Count()
		if err != nil {
			return nil, err
		}
		total = int64(count)
	default:
		nodeCount, err := g.DB().Ctx(ctx).Model("cobo_node_purchase").Where("direct_reward_user_id = ? AND direct_reward_amount > 0 AND is_gift = 0", req.UserID).Count()
		if err != nil {
			return nil, err
		}
		stakeDirectCount, err := g.DB().Ctx(ctx).Model("cobo_balance_change_log").
			Where("user_id = ?", req.UserID).
			Where("change_type = ?", consts.ChangeTypeStakingV2ReferralDirect).
			Count()
		if err != nil {
			return nil, err
		}
		indirectCount, err := g.DB().Ctx(ctx).Model("cobo_balance_change_log").
			Where("user_id = ?", req.UserID).
			Where("change_type = ?", consts.ChangeTypeStakingV2ReferralIndirect).
			Count()
		if err != nil {
			return nil, err
		}
		teamCount, err := g.DB().Ctx(ctx).Model("group_match_team_reward_distribution").
			Where("user_id = ?", req.UserID).
			Where("COALESCE(granted_amount, 0) > 0").
			Count()
		if err != nil {
			return nil, err
		}
		diffCount, err := g.DB().Ctx(ctx).Model("group_purchase_leadership_reward_detail").
			Where("user_id = ?", req.UserID).
			Where("COALESCE(granted_amount, 0) > 0").
			Count()
		if err != nil {
			return nil, err
		}
		weightCount, err := g.DB().Ctx(ctx).Model("group_purchase_leadership_weight_reward_detail").
			Where("user_id = ?", req.UserID).
			Where("COALESCE(granted_amount, 0) > 0").
			Count()
		if err != nil {
			return nil, err
		}
		matchCountVal, err := g.DB().GetValue(ctx, `
			SELECT COUNT(*)
			FROM (
				SELECT o.session_id
				FROM group_match_order o
				WHERE o.user_id = ?
				  AND o.is_winner = true
				GROUP BY o.session_id
				HAVING COALESCE(SUM(o.reward_amount), 0) > 0
			) t
		`, req.UserID)
		if err != nil {
			return nil, err
		}
		usStockCount, err := g.DB().Ctx(ctx).Model("us_stock_reward_settlement").
			Where("user_id = ?", req.UserID).
			Where("amount > 0").
			Count()
		if err != nil {
			return nil, err
		}
		total = int64(nodeCount + stakeDirectCount + indirectCount + teamCount + diffCount + weightCount + int(matchCountVal.Int64()) + usStockCount)
	}

	offset := (req.Page - 1) * req.PageSize

	query := `
SELECT x.id,
       x.reward_type,
       x.amount,
       x.symbol,
       x.source_user_id,
       x.source_wallet_address,
       x.purchase_package_no,
       x.purchase_amount,
       x.reward_rate,
       x.created_at
FROM (
    SELECT p.id AS id,
           'direct' AS reward_type,
           p.direct_reward_amount::text AS amount,
           'USDT' AS symbol,
           p.user_id AS source_user_id,
           COALESCE(u.wallet_address, '') AS source_wallet_address,
           p.package_no AS purchase_package_no,
           p.amount::text AS purchase_amount,
           '0.10' AS reward_rate,
           p.created_at AS created_at
    FROM cobo_node_purchase p
    LEFT JOIN user_info u ON u.id = p.user_id
    WHERE p.direct_reward_user_id = ?
      AND p.direct_reward_amount > 0
      AND p.is_gift = 0

    UNION ALL

    SELECT l.id AS id,
           'direct' AS reward_type,
           l.amount::text AS amount,
           l.symbol AS symbol,
           0 AS source_user_id,
           '' AS source_wallet_address,
           COALESCE(l.related_order_no, '') AS purchase_package_no,
           '' AS purchase_amount,
           '0.08' AS reward_rate,
           l.created_at AS created_at
    FROM cobo_balance_change_log l
    WHERE l.user_id = ?
      AND l.change_type = 'staking_v2_referral_direct'

    UNION ALL

    SELECT l.id AS id,
           'indirect' AS reward_type,
           l.amount::text AS amount,
           l.symbol AS symbol,
           0 AS source_user_id,
           '' AS source_wallet_address,
           COALESCE(l.related_order_no, '') AS purchase_package_no,
           '' AS purchase_amount,
           '0.04' AS reward_rate,
           l.created_at AS created_at
    FROM cobo_balance_change_log l
    WHERE l.user_id = ?
      AND l.change_type = 'staking_v2_referral_indirect'

    UNION ALL

    SELECT d.id AS id,
           'team' AS reward_type,
           COALESCE(d.granted_amount, 0)::text AS amount,
           'USDT' AS symbol,
           0 AS source_user_id,
           '' AS source_wallet_address,
           '' AS purchase_package_no,
           '' AS purchase_amount,
           '' AS reward_rate,
           d.created_at AS created_at
    FROM group_match_team_reward_distribution d
    WHERE d.user_id = ?
      AND COALESCE(d.granted_amount, 0) > 0

    UNION ALL

    SELECT d.id AS id,
           'leadership_diff' AS reward_type,
           COALESCE(d.granted_amount, 0)::text AS amount,
           'USDT' AS symbol,
           0 AS source_user_id,
           '' AS source_wallet_address,
           d.batch_id AS purchase_package_no,
           '' AS purchase_amount,
           d.diff_rate::text AS reward_rate,
           d.created_at AS created_at
    FROM group_purchase_leadership_reward_detail d
    WHERE d.user_id = ?
      AND COALESCE(d.granted_amount, 0) > 0

    UNION ALL

    SELECT d.id AS id,
           'leadership_weight' AS reward_type,
           COALESCE(d.granted_amount, 0)::text AS amount,
           'USDT' AS symbol,
           0 AS source_user_id,
           '' AS source_wallet_address,
           d.batch_id AS purchase_package_no,
           '' AS purchase_amount,
           '' AS reward_rate,
           d.created_at AS created_at
    FROM group_purchase_leadership_weight_reward_detail d
    WHERE d.user_id = ?
      AND COALESCE(d.granted_amount, 0) > 0

    UNION ALL

    SELECT o.session_id AS id,
           'match_reward' AS reward_type,
           COALESCE(SUM(o.reward_amount), 0)::text AS amount,
           'USDT' AS symbol,
           0 AS source_user_id,
           '' AS source_wallet_address,
           'SESSION-' || o.session_id::text AS purchase_package_no,
           '' AS purchase_amount,
           '0.02' AS reward_rate,
           MAX(o.created_at) AS created_at
    FROM group_match_order o
    WHERE o.user_id = ?
      AND o.is_winner = true
    GROUP BY o.session_id
    HAVING COALESCE(SUM(o.reward_amount), 0) > 0

    UNION ALL

    SELECT s.id AS id,
           'us_stock' AS reward_type,
           s.amount::text AS amount,
           'USDT' AS symbol,
           0 AS source_user_id,
           '' AS source_wallet_address,
           TO_CHAR(s.pool_date, 'YYYY-MM-DD') AS purchase_package_no,
           '' AS purchase_amount,
           s.weight_percent::text AS reward_rate,
           s.created_at AS created_at
    FROM us_stock_reward_settlement s
    WHERE s.user_id = ?
      AND s.amount > 0
) x
`

	switch req.RewardType {
	case model.RewardTypeDirect:
		query = `
SELECT p.id AS id,
       'direct' AS reward_type,
       p.direct_reward_amount::text AS amount,
       'USDT' AS symbol,
       p.user_id AS source_user_id,
       COALESCE(u.wallet_address, '') AS source_wallet_address,
       p.package_no AS purchase_package_no,
       p.amount::text AS purchase_amount,
       '0.10' AS reward_rate,
       p.created_at AS created_at
FROM cobo_node_purchase p
LEFT JOIN user_info u ON u.id = p.user_id
WHERE p.direct_reward_user_id = ?
  AND p.direct_reward_amount > 0
  AND p.is_gift = 0

UNION ALL

SELECT l.id AS id,
       'direct' AS reward_type,
       l.amount::text AS amount,
       l.symbol AS symbol,
       0 AS source_user_id,
       '' AS source_wallet_address,
       COALESCE(l.related_order_no, '') AS purchase_package_no,
       '' AS purchase_amount,
       '0.08' AS reward_rate,
       l.created_at AS created_at
FROM cobo_balance_change_log l
WHERE l.user_id = ?
  AND l.change_type = 'staking_v2_referral_direct'
`
	case model.RewardTypeIndirect:
		query = `
SELECT l.id,
       'indirect' AS reward_type,
       l.amount::text AS amount,
       l.symbol AS symbol,
       0 AS source_user_id,
       '' AS source_wallet_address,
       COALESCE(l.related_order_no, '') AS purchase_package_no,
       '' AS purchase_amount,
       '0.04' AS reward_rate,
       l.created_at AS created_at
FROM cobo_balance_change_log l
WHERE l.user_id = ?
  AND l.change_type = 'staking_v2_referral_indirect'
`
	case model.RewardTypeTeam:
		query = `
SELECT d.id,
       'team' AS reward_type,
       COALESCE(d.granted_amount, 0)::text AS amount,
       'USDT' AS symbol,
       0 AS source_user_id,
       '' AS source_wallet_address,
       '' AS purchase_package_no,
       '' AS purchase_amount,
       '' AS reward_rate,
       d.created_at AS created_at
FROM group_match_team_reward_distribution d
WHERE d.user_id = ?
  AND COALESCE(d.granted_amount, 0) > 0
`
	case model.RewardTypeLeadership:
		query = `
SELECT d.id,
       'leadership_diff' AS reward_type,
       COALESCE(d.granted_amount, 0)::text AS amount,
       'USDT' AS symbol,
       0 AS source_user_id,
       '' AS source_wallet_address,
       d.batch_id AS purchase_package_no,
       '' AS purchase_amount,
       d.diff_rate::text AS reward_rate,
       d.created_at AS created_at
FROM group_purchase_leadership_reward_detail d
WHERE d.user_id = ?
  AND COALESCE(d.granted_amount, 0) > 0

UNION ALL

SELECT d.id,
       'leadership_weight' AS reward_type,
       COALESCE(d.granted_amount, 0)::text AS amount,
       'USDT' AS symbol,
       0 AS source_user_id,
       '' AS source_wallet_address,
       d.batch_id AS purchase_package_no,
       '' AS purchase_amount,
       '' AS reward_rate,
       d.created_at AS created_at
FROM group_purchase_leadership_weight_reward_detail d
WHERE d.user_id = ?
  AND COALESCE(d.granted_amount, 0) > 0

`
	case model.RewardTypeLeadershipDiff:
		query = `
SELECT d.id,
       'leadership_diff' AS reward_type,
       COALESCE(d.granted_amount, 0)::text AS amount,
       'USDT' AS symbol,
       0 AS source_user_id,
       '' AS source_wallet_address,
       d.batch_id AS purchase_package_no,
       '' AS purchase_amount,
       d.diff_rate::text AS reward_rate,
       d.created_at AS created_at
FROM group_purchase_leadership_reward_detail d
WHERE d.user_id = ?
  AND COALESCE(d.granted_amount, 0) > 0
`
	case model.RewardTypeLeadershipWeight:
		query = `
SELECT d.id,
       'leadership_weight' AS reward_type,
       COALESCE(d.granted_amount, 0)::text AS amount,
       'USDT' AS symbol,
       0 AS source_user_id,
       '' AS source_wallet_address,
       d.batch_id AS purchase_package_no,
       '' AS purchase_amount,
       '' AS reward_rate,
       d.created_at AS created_at
FROM group_purchase_leadership_weight_reward_detail d
WHERE d.user_id = ?
  AND COALESCE(d.granted_amount, 0) > 0
`
	case model.RewardTypeMatchReward:
		query = `
SELECT o.session_id AS id,
       'match_reward' AS reward_type,
       COALESCE(SUM(o.reward_amount), 0)::text AS amount,
       'USDT' AS symbol,
       0 AS source_user_id,
       '' AS source_wallet_address,
       'SESSION-' || o.session_id::text AS purchase_package_no,
       '' AS purchase_amount,
       '0.02' AS reward_rate,
       MAX(o.created_at) AS created_at
FROM group_match_order o
WHERE o.user_id = ?
  AND o.is_winner = true
GROUP BY o.session_id
HAVING COALESCE(SUM(o.reward_amount), 0) > 0
`
	case model.RewardTypeUSStock:
		query = `
SELECT s.id,
       'us_stock' AS reward_type,
       s.amount::text AS amount,
       'USDT' AS symbol,
       0 AS source_user_id,
       '' AS source_wallet_address,
       TO_CHAR(s.pool_date, 'YYYY-MM-DD') AS purchase_package_no,
       '' AS purchase_amount,
       s.weight_percent::text AS reward_rate,
       s.created_at AS created_at
FROM us_stock_reward_settlement s
WHERE s.user_id = ?
  AND s.amount > 0
`
	}

	query = query + " ORDER BY created_at DESC LIMIT ? OFFSET ?"

	queryArgs := []any{req.UserID, req.UserID, req.UserID, req.UserID, req.UserID, req.UserID, req.UserID, req.UserID}
	switch req.RewardType {
	case model.RewardTypeDirect:
		queryArgs = []any{req.UserID, req.UserID}
	case model.RewardTypeIndirect, model.RewardTypeTeam:
		queryArgs = []any{req.UserID}
	case model.RewardTypeLeadership:
		queryArgs = []any{req.UserID, req.UserID}
	case model.RewardTypeLeadershipDiff, model.RewardTypeLeadershipWeight:
		queryArgs = []any{req.UserID}
	case model.RewardTypeMatchReward, model.RewardTypeUSStock:
		queryArgs = []any{req.UserID}
	}
	queryArgs = append(queryArgs, req.PageSize, offset)

	rows, err := g.DB().GetAll(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}

	// 计算总页数
	pages := int(total) / req.PageSize
	if int(total)%req.PageSize > 0 {
		pages++
	}

	records := make([]*model.RewardRecord, 0, len(rows))
	for _, row := range rows {
		rewardType := row["reward_type"].String()
		records = append(records, &model.RewardRecord{
			ID:                  row["id"].Int64(),
			RewardType:          rewardType,
			RewardTypeText:      s.getRewardTypeText(rewardType, locale),
			Amount:              row["amount"].String(),
			Symbol:              row["symbol"].String(),
			SourceUserID:        row["source_user_id"].Int64(),
			SourceWalletAddress: row["source_wallet_address"].String(),
			PurchasePackageNo:   row["purchase_package_no"].String(),
			PurchaseAmount:      row["purchase_amount"].String(),
			RewardRate:          row["reward_rate"].String(),
			CreatedAt:           utils.DBTimestampToUnix(row["created_at"].GTime().Time),
		})
	}

	return &model.GetRewardRecordsRes{
		Page:     req.Page,
		PageSize: req.PageSize,
		Total:    total,
		Pages:    pages,
		List:     records,
	}, nil
}

var rewardTypeTexts = map[string]map[string]string{
	model.RewardTypeDirect: {
		consts.LanguageChinese:            "直推奖励",
		consts.LanguageTraditionalChinese: "直推奖励",
		consts.LanguageEnglish:            "Direct Reward",
		consts.LanguageJapanese:           "直推報酬",
		consts.LanguageKorean:             "직접 추천 보상",
	},
	model.RewardTypeIndirect: {
		consts.LanguageChinese:            "间推奖励",
		consts.LanguageTraditionalChinese: "間推獎勵",
		consts.LanguageEnglish:            "Indirect Reward",
		consts.LanguageJapanese:           "間推報酬",
		consts.LanguageKorean:             "간접 추천 보상",
	},
	model.RewardTypeTeam: {
		consts.LanguageChinese:            "团队奖励",
		consts.LanguageTraditionalChinese: "團隊獎勵",
		consts.LanguageEnglish:            "Team Reward",
		consts.LanguageJapanese:           "チーム報酬",
		consts.LanguageKorean:             "팀 보상",
	},
	model.RewardTypeLeadership: {
		consts.LanguageChinese:            "领导奖励",
		consts.LanguageTraditionalChinese: "領導獎勵",
		consts.LanguageEnglish:            "Leadership Reward",
		consts.LanguageJapanese:           "リーダー報酬",
		consts.LanguageKorean:             "리더십 보상",
	},
	model.RewardTypeLeadershipDiff: {
		consts.LanguageChinese:            "等级奖励",
		consts.LanguageTraditionalChinese: "等級獎勵",
		consts.LanguageEnglish:            "Level Reward",
		consts.LanguageJapanese:           "等級報酬",
		consts.LanguageKorean:             "등급 보상",
	},
	model.RewardTypeLeadershipWeight: {
		consts.LanguageChinese:            "等级分红",
		consts.LanguageTraditionalChinese: "等級分紅",
		consts.LanguageEnglish:            "Level Dividend",
		consts.LanguageJapanese:           "等級配当",
		consts.LanguageKorean:             "등급 배당",
	},
	model.RewardTypeMatchReward: {
		consts.LanguageChinese:            "拼团收益",
		consts.LanguageTraditionalChinese: "拼團收益",
		consts.LanguageEnglish:            "Match Reward",
		consts.LanguageJapanese:           "グループ購入収益",
		consts.LanguageKorean:             "그룹 매치 수익",
	},
	model.RewardTypeUSStock: {
		consts.LanguageChinese:            "股权收益",
		consts.LanguageTraditionalChinese: "股權收益",
		consts.LanguageEnglish:            "Equity Reward",
		consts.LanguageJapanese:           "株式報酬",
		consts.LanguageKorean:             "주식 보상",
	},
}

func (s *rewardService) getRewardTypeText(rewardType, locale string) string {
	if m, ok := rewardTypeTexts[rewardType]; ok {
		if v := consts.LocalizedText(m, locale); v != "" {
			return v
		}
	}
	return rewardType
}

// GetTodayRewardAmount 获取用户今日奖励总金额（口径与 /api/v1/cobo/reward/records 一致）
func (s *rewardService) GetTodayRewardAmount(ctx context.Context, userID int64, dayStart, dayEnd time.Time) (decimal.Decimal, error) {
	return s.getRewardAmount(ctx, userID, &dayStart, &dayEnd)
}

// GetTotalRewardAmount 获取用户累计奖励总金额（口径与 /api/v1/cobo/reward/records 一致，不限时间）
func (s *rewardService) GetTotalRewardAmount(ctx context.Context, userID int64) (decimal.Decimal, error) {
	return s.getRewardAmount(ctx, userID, nil, nil)
}

func (s *rewardService) getRewardAmount(ctx context.Context, userID int64, dayStart, dayEnd *time.Time) (decimal.Decimal, error) {
	var query string
	args := make([]any, 0, 24)

	if dayStart != nil && dayEnd != nil {
		query = `
SELECT COALESCE(SUM(amount), 0) AS total FROM (
    SELECT direct_reward_amount AS amount FROM cobo_node_purchase
    WHERE direct_reward_user_id = ? AND direct_reward_amount > 0 AND is_gift = 0
      AND created_at >= ? AND created_at < ?

    UNION ALL

    SELECT amount FROM cobo_balance_change_log
    WHERE user_id = ? AND change_type = ?
      AND created_at >= ? AND created_at < ?

    UNION ALL

    SELECT amount FROM cobo_balance_change_log
    WHERE user_id = ? AND change_type = ?
      AND created_at >= ? AND created_at < ?

    UNION ALL

    SELECT granted_amount FROM group_match_team_reward_distribution
    WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
      AND created_at >= ? AND created_at < ?

    UNION ALL

    SELECT granted_amount FROM group_purchase_leadership_reward_detail
    WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
      AND created_at >= ? AND created_at < ?

    UNION ALL

    SELECT granted_amount FROM group_purchase_leadership_weight_reward_detail
    WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0
      AND created_at >= ? AND created_at < ?

    UNION ALL

    SELECT SUM(reward_amount) AS amount FROM group_match_order
    WHERE user_id = ? AND is_winner = true
      AND created_at >= ? AND created_at < ?
    GROUP BY session_id
    HAVING COALESCE(SUM(reward_amount), 0) > 0

    UNION ALL

    SELECT amount FROM us_stock_reward_settlement
    WHERE user_id = ? AND amount > 0
      AND created_at >= ? AND created_at < ?
) t
`
		args = append(args,
			userID, *dayStart, *dayEnd,
			userID, consts.ChangeTypeStakingV2ReferralDirect, *dayStart, *dayEnd,
			userID, consts.ChangeTypeStakingV2ReferralIndirect, *dayStart, *dayEnd,
			userID, *dayStart, *dayEnd,
			userID, *dayStart, *dayEnd,
			userID, *dayStart, *dayEnd,
			userID, *dayStart, *dayEnd,
			userID, *dayStart, *dayEnd,
		)
	} else {
		query = `
SELECT COALESCE(SUM(amount), 0) AS total FROM (
    SELECT direct_reward_amount AS amount FROM cobo_node_purchase
    WHERE direct_reward_user_id = ? AND direct_reward_amount > 0 AND is_gift = 0

    UNION ALL

    SELECT amount FROM cobo_balance_change_log
    WHERE user_id = ? AND change_type = ?

    UNION ALL

    SELECT amount FROM cobo_balance_change_log
    WHERE user_id = ? AND change_type = ?

    UNION ALL

    SELECT granted_amount FROM group_match_team_reward_distribution
    WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0

    UNION ALL

    SELECT granted_amount FROM group_purchase_leadership_reward_detail
    WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0

    UNION ALL

    SELECT granted_amount FROM group_purchase_leadership_weight_reward_detail
    WHERE user_id = ? AND COALESCE(granted_amount, 0) > 0

    UNION ALL

    SELECT SUM(reward_amount) AS amount FROM group_match_order
    WHERE user_id = ? AND is_winner = true
    GROUP BY session_id
    HAVING COALESCE(SUM(reward_amount), 0) > 0

    UNION ALL

    SELECT amount FROM us_stock_reward_settlement
    WHERE user_id = ? AND amount > 0
) t
`
		args = append(args,
			userID,
			userID, consts.ChangeTypeStakingV2ReferralDirect,
			userID, consts.ChangeTypeStakingV2ReferralIndirect,
			userID,
			userID,
			userID,
			userID,
			userID,
		)
	}

	val, err := g.DB().GetValue(ctx, query, args...)
	if err != nil {
		return decimal.Zero, err
	}
	result, parseErr := decimal.NewFromString(val.String())
	if parseErr != nil {
		return decimal.Zero, parseErr
	}
	return result, nil
}
