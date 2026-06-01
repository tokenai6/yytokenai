package cobo

import (
	"context"
	"fmt"

	"XWFrame/internal/frame/consts"
	"XWFrame/internal/service/cobo/model"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// FlowService 流水查询服务接口
type FlowService interface {
	// GetFlowRecords 获取用户流水记录
	GetFlowRecords(ctx context.Context, req *model.GetFlowRecordsReq) (*model.GetFlowRecordsRes, error)
}

type flowService struct{}

// NewFlowService 创建流水服务
func NewFlowService() FlowService {
	return &flowService{}
}

// GetFlowRecords 获取用户流水记录
// 提现部分基于 cobo_withdraw_request（一笔提现一条记录），其余基于 cobo_balance_change_log
func (s *flowService) GetFlowRecords(ctx context.Context, req *model.GetFlowRecordsReq) (*model.GetFlowRecordsRes, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	locale := consts.LocaleFromCtx(ctx)

	// 非支持类型直接返回空，避免拼接出非法 SQL
	if req.FlowType != "" &&
		req.FlowType != model.FlowTypeRecharge &&
		req.FlowType != model.FlowTypeWithdraw &&
		req.FlowType != model.FlowTypeReward &&
		req.FlowType != model.FlowTypeRefund &&
		req.FlowType != model.FlowTypePayout &&
		req.FlowType != model.FlowTypeCompensation &&
		req.FlowType != model.FlowTypeNodePurchase &&
		req.FlowType != model.FlowTypeMatchReward &&
		req.FlowType != model.FlowTypeMatchRefund &&
		req.FlowType != model.FlowTypeDirectReward &&
		req.FlowType != model.FlowTypeMatchJoin &&
		req.FlowType != model.FlowTypeGroupTeamReward &&
		req.FlowType != model.FlowTypeSwap &&
		req.FlowType != model.FlowTypeTransfer {
		return &model.GetFlowRecordsRes{
			Page:     page,
			PageSize: pageSize,
			Total:    0,
			Pages:    0,
			List:     []*model.FlowRecordItem{},
		}, nil
	}

	query, args := s.buildFlowQuery(req)

	// 查询总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) AS cnt FROM (%s) AS total", query)
	totalRow, err := g.DB().GetOne(ctx, countQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("查询流水总数失败: %w", err)
	}
	total := totalRow["cnt"].Int64()

	// 分页查询数据（外层包一层确保 ORDER BY/LIMIT 作用于 UNION ALL 后的整体）
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf("SELECT * FROM (%s) AS unified ORDER BY created_at DESC LIMIT %d OFFSET %d", query, pageSize, offset)

	rows, err := g.DB().GetAll(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("查询流水记录失败: %w", err)
	}

	// 转换为响应格式
	list := make([]*model.FlowRecordItem, 0, len(rows))
	for _, row := range rows {
		flowType := row["flow_type"].String()
		changeType := row["change_type"].String()
		status := row["status"].Int()
		item := &model.FlowRecordItem{
			ID:         row["flow_id"].String(),
			Type:       flowType,
			TypeText:   s.getFlowTypeText(flowType, locale),
			Amount:     s.normalizeDecimal(row["amount"].String()),
			Symbol:     row["symbol"].String(),
			Status:     status,
			StatusText: s.getStatusText(flowType, changeType, status, locale),
			TxHash:     row["tx_hash"].String(),
			CreatedAt:  utils.DBTimestampToUnix(row["created_at"].GTime().Time),
		}

		// 根据类型填充特有字段
		switch item.Type {
		case model.FlowTypeRecharge:
			item.Confirmations = row["confirmations"].Int()
		case model.FlowTypeWithdraw:
			item.FeeRate = s.normalizeDecimal(row["fee_rate"].String())
			item.FeeAmount = s.normalizeDecimal(row["fee_amount"].String())
			item.ActualAmount = s.normalizeDecimal(row["actual_amount"].String())
			item.ToAddress = row["to_address"].String()
		case model.FlowTypeReward, model.FlowTypeGroupTeamReward:
			item.RewardType = row["reward_type"].String()
			item.SourceWalletAddress = row["source_wallet_address"].String()
			item.PurchaseAmount = s.normalizeDecimal(row["purchase_amount"].String())
		}

		// 描述：提现按 status 取多语言；其他按 change_type 走 consts 翻译，再回退到 remark
		item.Description = s.getDescription(flowType, changeType, status, locale)
		if item.Description == "" {
			item.Description = row["remark"].String()
		}

		list = append(list, item)
	}

	pages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		pages++
	}

	return &model.GetFlowRecordsRes{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		Pages:    pages,
		List:     list,
	}, nil
}

// buildFlowQuery 根据 FlowType 构造 UNION ALL 查询
// 提现部分基于 cobo_withdraw_request；充值部分基于 cobo_recharge_record；其余基于 cobo_balance_change_log
func (s *flowService) buildFlowQuery(req *model.GetFlowRecordsReq) (string, []interface{}) {
	withdrawSubQuery := `
		SELECT
			'withdraw:' || w.id::text AS flow_id,
			'' AS change_type,
			'withdraw' AS flow_type,
			(- w.amount)::text AS amount,
			w.symbol,
			w.status::int AS status,
			COALESCE(w.tx_hash, '') AS tx_hash,
			0 AS confirmations,
			COALESCE(w.fee_rate::text, '') AS fee_rate,
			COALESCE(w.fee_amount::text, '') AS fee_amount,
			COALESCE(w.actual_amount::text, '') AS actual_amount,
			COALESCE(w.to_address, '') AS to_address,
			'' AS reward_type,
			'' AS source_wallet_address,
			'' AS purchase_amount,
			'' AS remark,
			w.created_at
		FROM cobo_withdraw_request w
		WHERE w.user_id = ?
			AND w.symbol = 'USDT'
			AND w.status <> 2
	`

	rechargeSubQuery := `
		SELECT
			'recharge:' || r.id::text AS flow_id,
			'' AS change_type,
			'recharge' AS flow_type,
			r.amount::text AS amount,
			r.symbol,
			r.status::int AS status,
			COALESCE(r.tx_hash, '') AS tx_hash,
			COALESCE(r.confirmations, 0) AS confirmations,
			'' AS fee_rate,
			'' AS fee_amount,
			'' AS actual_amount,
			'' AS to_address,
			'' AS reward_type,
			'' AS source_wallet_address,
			'' AS purchase_amount,
			'' AS remark,
			r.created_at
		FROM cobo_recharge_record r
		WHERE r.user_id = ?
			AND r.symbol = 'USDT'
	`

	logSubQuery := `
		SELECT
			CASE
				WHEN l.change_type = 'node_purchase' THEN 'node_purchase:' || l.id::text
				WHEN l.change_type IN ('group_match_settle_flow_refund', 'group_match_settle_ticket_refund') THEN 'match_refund:' || l.id::text
				WHEN l.change_type = 'group_match_settle_winner' THEN 'match_reward:' || l.id::text
				WHEN l.change_type = 'group_match_loser_comp_release' THEN 'compensation:' || l.id::text
				WHEN l.change_type IN ('group_match_join_usdt', 'group_match_join_ticket') THEN 'match_join:' || l.id::text
				WHEN l.change_type = 'node_purchase_direct_reward' THEN 'direct_reward:' || l.id::text
				WHEN l.change_type = 'staking_v2_referral_direct' THEN 'direct_reward:' || l.id::text
				WHEN l.change_type = 'group_match_team_reward' THEN 'group_team_reward:' || l.id::text
				WHEN l.change_type IN ('exchange_out', 'exchange_in') THEN 'swap:' || l.id::text
				WHEN l.change_type IN ('transfer_out', 'transfer_in') THEN 'transfer:' || l.id::text
				ELSE 'reward:' || l.id::text
			END AS flow_id,
			l.change_type,
			CASE
				WHEN l.change_type = 'node_purchase' THEN 'node_purchase'
				WHEN l.change_type IN ('group_match_settle_flow_refund', 'group_match_settle_ticket_refund') THEN 'match_refund'
				WHEN l.change_type = 'group_match_settle_winner' THEN 'match_reward'
				WHEN l.change_type = 'group_match_loser_comp_release' THEN 'compensation'
				WHEN l.change_type IN ('group_match_join_usdt', 'group_match_join_ticket') THEN 'match_join'
				WHEN l.change_type = 'node_purchase_direct_reward' THEN 'direct_reward'
				WHEN l.change_type = 'staking_v2_referral_direct' THEN 'direct_reward'
				WHEN l.change_type = 'group_match_team_reward' THEN 'group_team_reward'
				WHEN l.change_type IN ('exchange_out', 'exchange_in') THEN 'swap'
				WHEN l.change_type IN ('transfer_out', 'transfer_in') THEN 'transfer'
				ELSE 'reward'
			END AS flow_type,
			l.amount::text AS amount,
			l.symbol,
			CASE
				WHEN l.change_type = 'node_purchase' THEN 4
				ELSE 1
			END AS status,
			'' AS tx_hash,
			0 AS confirmations,
			'' AS fee_rate,
			'' AS fee_amount,
			'' AS actual_amount,
			'' AS to_address,
			CASE
				WHEN l.change_type = 'node_purchase_direct_reward' THEN 'direct'
				WHEN l.change_type = 'staking_v2_referral_burn_to_vertex' THEN 'burn_to_vertex'
				ELSE ''
			END AS reward_type,
			CASE
				WHEN l.change_type = 'node_purchase_direct_reward' THEN COALESCE(src.wallet_address, '')
				ELSE ''
			END AS source_wallet_address,
			CASE
				WHEN l.change_type = 'node_purchase_direct_reward' THEN COALESCE(np.amount::text, '')
				ELSE ''
			END AS purchase_amount,
			COALESCE(l.remark, '') AS remark,
			l.created_at
		FROM cobo_balance_change_log l
		LEFT JOIN cobo_node_purchase np
			ON np.id = l.related_id
			AND l.change_type = 'node_purchase_direct_reward'
		LEFT JOIN user_info src
			ON src.id = np.user_id
		WHERE l.user_id = ?
			AND l.symbol = 'USDT'
			AND l.change_type NOT IN (
				'recharge',
				'withdraw_freeze', 'withdraw_success', 'withdraw_fail_refund',
				'withdraw_tax_pay', 'withdraw_tax_receive',
				'withdraw_tax_refund_pay', 'withdraw_tax_refund_receive'
			)
	`

	args := []interface{}{}

	switch req.FlowType {
	case "":
		args = append(args, req.UserID, req.UserID, req.UserID)
		return "(" + withdrawSubQuery + ") UNION ALL (" + rechargeSubQuery + ") UNION ALL (" + logSubQuery + ")", args

	case model.FlowTypeWithdraw:
		args = append(args, req.UserID)
		return withdrawSubQuery, args

	case model.FlowTypeRecharge:
		args = append(args, req.UserID)
		return rechargeSubQuery, args

	default:
		filter := ""
		switch req.FlowType {
		case model.FlowTypeNodePurchase:
			filter = " AND l.change_type = 'node_purchase'"
		case model.FlowTypeReward:
			filter = " AND l.change_type NOT IN ('node_purchase', 'group_match_settle_flow_refund', 'group_match_settle_ticket_refund', 'group_match_settle_winner', 'group_match_loser_comp_release', 'group_match_join_usdt', 'group_match_join_ticket', 'node_purchase_direct_reward', 'staking_v2_referral_direct', 'group_match_team_reward') AND l.amount > 0"
		case model.FlowTypeRefund, model.FlowTypeMatchRefund:
			filter = " AND l.change_type IN ('group_match_settle_flow_refund', 'group_match_settle_ticket_refund')"
		case model.FlowTypePayout, model.FlowTypeMatchReward:
			filter = " AND l.change_type = 'group_match_settle_winner'"
		case model.FlowTypeCompensation:
			filter = " AND l.change_type = 'group_match_loser_comp_release'"
		case model.FlowTypeDirectReward:
			filter = " AND l.change_type IN ('node_purchase_direct_reward', 'staking_v2_referral_direct')"
		case model.FlowTypeMatchJoin:
			filter = " AND l.change_type IN ('group_match_join_usdt', 'group_match_join_ticket')"
		case model.FlowTypeGroupTeamReward:
			filter = " AND l.change_type = 'group_match_team_reward'"
		case model.FlowTypeSwap:
			filter = " AND l.change_type IN ('exchange_out', 'exchange_in')"
		case model.FlowTypeTransfer:
			filter = " AND l.change_type IN ('transfer_out', 'transfer_in')"
		}
		args = append(args, req.UserID)
		return logSubQuery + filter, args
	}
}

// flowTypeTexts 流水类型在各语言下的文案
// key: flow_type（机器码），value: locale -> 文案
var flowTypeTexts = map[string]map[string]string{
	model.FlowTypeRecharge: {
		consts.LanguageChinese:            "充值",
		consts.LanguageTraditionalChinese: "充值",
		consts.LanguageEnglish:            "Recharge",
		consts.LanguageJapanese:           "入金",
		consts.LanguageKorean:             "충전",
	},
	model.FlowTypeWithdraw: {
		consts.LanguageChinese:            "提现",
		consts.LanguageTraditionalChinese: "提現",
		consts.LanguageEnglish:            "Withdrawal",
		consts.LanguageJapanese:           "出金",
		consts.LanguageKorean:             "출금",
	},
	model.FlowTypeReward: {
		consts.LanguageChinese:            "奖励",
		consts.LanguageTraditionalChinese: "獎勵",
		consts.LanguageEnglish:            "Reward",
		consts.LanguageJapanese:           "報酬",
		consts.LanguageKorean:             "보상",
	},
	model.FlowTypeRefund: {
		consts.LanguageChinese:            "退款",
		consts.LanguageTraditionalChinese: "退款",
		consts.LanguageEnglish:            "Refund",
		consts.LanguageJapanese:           "返金",
		consts.LanguageKorean:             "환불",
	},
	model.FlowTypePayout: {
		consts.LanguageChinese:            "中奖派发",
		consts.LanguageTraditionalChinese: "中獎派發",
		consts.LanguageEnglish:            "Winnings Payout",
		consts.LanguageJapanese:           "当選払い出し",
		consts.LanguageKorean:             "당첨금 지급",
	},
	model.FlowTypeCompensation: {
		consts.LanguageChinese:            "补偿释放",
		consts.LanguageTraditionalChinese: "補償釋放",
		consts.LanguageEnglish:            "Compensation Release",
		consts.LanguageJapanese:           "補償リリース",
		consts.LanguageKorean:             "보상 해제",
	},
	model.FlowTypeNodePurchase: {
		consts.LanguageChinese:            "节点购买",
		consts.LanguageTraditionalChinese: "節點購買",
		consts.LanguageEnglish:            "Node Purchase",
		consts.LanguageJapanese:           "ノード購入",
		consts.LanguageKorean:             "노드 구매",
	},
	model.FlowTypeMatchReward: {
		consts.LanguageChinese:            "拼团奖励",
		consts.LanguageTraditionalChinese: "拼團獎勵",
		consts.LanguageEnglish:            "Match Reward",
		consts.LanguageJapanese:           "マッチ報酬",
		consts.LanguageKorean:             "매치 보상",
	},
	model.FlowTypeMatchRefund: {
		consts.LanguageChinese:            "拼团退款",
		consts.LanguageTraditionalChinese: "拼團退款",
		consts.LanguageEnglish:            "Match Refund",
		consts.LanguageJapanese:           "マッチ返金",
		consts.LanguageKorean:             "매치 환불",
	},
	model.FlowTypeDirectReward: {
		consts.LanguageChinese:            "直推奖励",
		consts.LanguageTraditionalChinese: "直推獎勵",
		consts.LanguageEnglish:            "Direct Reward",
		consts.LanguageJapanese:           "直接報酬",
		consts.LanguageKorean:             "직접 보상",
	},
	model.FlowTypeMatchJoin: {
		consts.LanguageChinese:            "拼团参与",
		consts.LanguageTraditionalChinese: "拼團參與",
		consts.LanguageEnglish:            "Match Join",
		consts.LanguageJapanese:           "マッチ参加",
		consts.LanguageKorean:             "매치 참가",
	},
	model.FlowTypeGroupTeamReward: {
		consts.LanguageChinese:            "团队奖励",
		consts.LanguageTraditionalChinese: "團隊獎勵",
		consts.LanguageEnglish:            "Team Reward",
		consts.LanguageJapanese:           "チーム報酬",
		consts.LanguageKorean:             "팀 보상",
	},
	model.FlowTypeSwap: {
		consts.LanguageChinese:            "兑换",
		consts.LanguageTraditionalChinese: "兌換",
		consts.LanguageEnglish:            "Swap",
		consts.LanguageJapanese:           "スワップ",
		consts.LanguageKorean:             "스왑",
	},
	model.FlowTypeTransfer: {
		consts.LanguageChinese:            "转账",
		consts.LanguageTraditionalChinese: "轉賬",
		consts.LanguageEnglish:            "Transfer",
		consts.LanguageJapanese:           "送金",
		consts.LanguageKorean:             "송금",
	},
}

// statusTexts 状态文案（非提现），key: change_type -> locale -> 文案
var statusTexts = map[string]map[string]string{
	"recharge": {
		consts.LanguageChinese:            "已确认",
		consts.LanguageTraditionalChinese: "已確認",
		consts.LanguageEnglish:            "Confirmed",
		consts.LanguageJapanese:           "確認済み",
		consts.LanguageKorean:             "확인됨",
	},
	"node_purchase": {
		consts.LanguageChinese:            "成功",
		consts.LanguageTraditionalChinese: "成功",
		consts.LanguageEnglish:            "Success",
		consts.LanguageJapanese:           "成功",
		consts.LanguageKorean:             "성공",
	},
}

// rechargeStatusTexts 充值状态文案，按 cobo_recharge_record.status 编码 -> locale -> 文案
// 状态编码：0=待确认 1=已确认 2=已取消/失败
var rechargeStatusTexts = map[int]map[string]string{
	0: {
		consts.LanguageChinese:            "待确认",
		consts.LanguageTraditionalChinese: "待確認",
		consts.LanguageEnglish:            "Pending Confirmation",
		consts.LanguageJapanese:           "確認待ち",
		consts.LanguageKorean:             "확인 대기",
	},
	1: {
		consts.LanguageChinese:            "已确认",
		consts.LanguageTraditionalChinese: "已確認",
		consts.LanguageEnglish:            "Confirmed",
		consts.LanguageJapanese:           "確認済み",
		consts.LanguageKorean:             "확인됨",
	},
	2: {
		consts.LanguageChinese:            "已取消",
		consts.LanguageTraditionalChinese: "已取消",
		consts.LanguageEnglish:            "Cancelled",
		consts.LanguageJapanese:           "キャンセル",
		consts.LanguageKorean:             "취소됨",
	},
	3: {
		consts.LanguageChinese:            "已拒绝",
		consts.LanguageTraditionalChinese: "已拒絕",
		consts.LanguageEnglish:            "Rejected",
		consts.LanguageJapanese:           "拒否",
		consts.LanguageKorean:             "거부됨",
	},
}

// withdrawStatusTexts 提现状态文案，按 cobo_withdraw_request.status 编码 -> locale -> 文案
// 状态编码：0=待审核 1=审核通过 3=处理中 4=成功 5=失败（2=已拒绝 在 SQL 已过滤）
var withdrawStatusTexts = map[int]map[string]string{
	0: {
		consts.LanguageChinese:            "审核中",
		consts.LanguageTraditionalChinese: "審核中",
		consts.LanguageEnglish:            "Pending Review",
		consts.LanguageJapanese:           "審査中",
		consts.LanguageKorean:             "심사 중",
	},
	1: {
		consts.LanguageChinese:            "处理中",
		consts.LanguageTraditionalChinese: "處理中",
		consts.LanguageEnglish:            "Processing",
		consts.LanguageJapanese:           "処理中",
		consts.LanguageKorean:             "처리 중",
	},
	3: {
		consts.LanguageChinese:            "处理中",
		consts.LanguageTraditionalChinese: "處理中",
		consts.LanguageEnglish:            "Processing",
		consts.LanguageJapanese:           "処理中",
		consts.LanguageKorean:             "처리 중",
	},
	4: {
		consts.LanguageChinese:            "成功",
		consts.LanguageTraditionalChinese: "成功",
		consts.LanguageEnglish:            "Success",
		consts.LanguageJapanese:           "成功",
		consts.LanguageKorean:             "성공",
	},
	5: {
		consts.LanguageChinese:            "失败",
		consts.LanguageTraditionalChinese: "失敗",
		consts.LanguageEnglish:            "Failed",
		consts.LanguageJapanese:           "失敗",
		consts.LanguageKorean:             "실패",
	},
}

// withdrawDescriptionTexts 提现描述文案，按 cobo_withdraw_request.status 编码 -> locale -> 文案
var withdrawDescriptionTexts = map[int]map[string]string{
	0: {
		consts.LanguageChinese:            "提现审核中",
		consts.LanguageTraditionalChinese: "提現審核中",
		consts.LanguageEnglish:            "Withdrawal pending review",
		consts.LanguageJapanese:           "出金審査中",
		consts.LanguageKorean:             "출금 심사 중",
	},
	1: {
		consts.LanguageChinese:            "提现处理中",
		consts.LanguageTraditionalChinese: "提現處理中",
		consts.LanguageEnglish:            "Withdrawal processing",
		consts.LanguageJapanese:           "出金処理中",
		consts.LanguageKorean:             "출금 처리 중",
	},
	3: {
		consts.LanguageChinese:            "提现处理中",
		consts.LanguageTraditionalChinese: "提現處理中",
		consts.LanguageEnglish:            "Withdrawal processing",
		consts.LanguageJapanese:           "出金処理中",
		consts.LanguageKorean:             "출금 처리 중",
	},
	4: {
		consts.LanguageChinese:            "提现成功",
		consts.LanguageTraditionalChinese: "提現成功",
		consts.LanguageEnglish:            "Withdrawal successful",
		consts.LanguageJapanese:           "出金成功",
		consts.LanguageKorean:             "출금 성공",
	},
	5: {
		consts.LanguageChinese:            "提现失败",
		consts.LanguageTraditionalChinese: "提現失敗",
		consts.LanguageEnglish:            "Withdrawal failed",
		consts.LanguageJapanese:           "出金失敗",
		consts.LanguageKorean:             "출금 실패",
	},
}

// statusFallback 未命中映射时的兜底状态文案
var statusFallback = map[string]string{
	consts.LanguageChinese:            "已完成",
	consts.LanguageTraditionalChinese: "已完成",
	consts.LanguageEnglish:            "Completed",
	consts.LanguageJapanese:           "完了",
	consts.LanguageKorean:             "완료",
}


// getFlowTypeText 获取流水类型在指定 locale 下的文案
func (s *flowService) getFlowTypeText(flowType, locale string) string {
	if m, ok := flowTypeTexts[flowType]; ok {
		if v := consts.LocalizedText(m, locale); v != "" {
			return v
		}
	}
	return flowType
}

// getStatusText 输出状态文案
// 提现/充值按 status 编码取多语言；其余按 change_type 取，未命中走兜底
func (s *flowService) getStatusText(flowType, changeType string, status int, locale string) string {
	if flowType == model.FlowTypeWithdraw {
		if m, ok := withdrawStatusTexts[status]; ok {
			if v := consts.LocalizedText(m, locale); v != "" {
				return v
			}
		}
		return s.fallbackStatusText(locale)
	}
	if flowType == model.FlowTypeRecharge {
		if m, ok := rechargeStatusTexts[status]; ok {
			if v := consts.LocalizedText(m, locale); v != "" {
				return v
			}
		}
		return s.fallbackStatusText(locale)
	}
	if m, ok := statusTexts[changeType]; ok {
		if v := consts.LocalizedText(m, locale); v != "" {
			return v
		}
	}
	return s.fallbackStatusText(locale)
}

// fallbackStatusText 状态兜底文案
func (s *flowService) fallbackStatusText(locale string) string {
	if v, ok := statusFallback[locale]; ok && v != "" {
		return v
	}
	return statusFallback[consts.LanguageDefault]
}

// getDescription 输出描述文案
// 提现按 status 取多语言；其余按 change_type 走 consts 翻译；未命中返回空（由调用方回退到 remark）
func (s *flowService) getDescription(flowType, changeType string, status int, locale string) string {
	if flowType == model.FlowTypeWithdraw {
		if m, ok := withdrawDescriptionTexts[status]; ok {
			if v := consts.LocalizedText(m, locale); v != "" {
				return v
			}
		}
		return ""
	}
	if !consts.HasBalanceChangeText(changeType) {
		return ""
	}
	return consts.BalanceChangeText(changeType, locale)
}

// normalizeDecimal 标准化金额字符串，去除无意义尾零
func (s *flowService) normalizeDecimal(value string) string {
	if value == "" {
		return ""
	}
	d, err := decimal.NewFromString(value)
	if err != nil {
		return value
	}
	return d.String()
}
