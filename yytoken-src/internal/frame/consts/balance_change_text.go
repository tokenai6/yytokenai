package consts

import "strings"

// balanceChangeTexts: 资金变动类型在各支持语言下的描述文案
// key: change_type 常量值 -> locale -> 文案
// 新增 ChangeType 时必须在此补全 5 种语言文案（zh-CN / zh-TW / en-US / ja-JP / ko-KR）
var balanceChangeTexts = map[string]map[string]string{
	// 充值
	ChangeTypeRecharge: {
		LanguageChinese:            "{symbol} 充值",
		LanguageTraditionalChinese: "{symbol} 充值",
		LanguageEnglish:            "{symbol} Deposit",
		LanguageJapanese:           "{symbol} 入金",
		LanguageKorean:             "{symbol} 충전",
	},
	ChangeTypeRechargeRefund: {
		LanguageChinese:            "充值退款",
		LanguageTraditionalChinese: "充值退款",
		LanguageEnglish:            "Deposit Refund",
		LanguageJapanese:           "入金返金",
		LanguageKorean:             "충전 환불",
	},

	// 提现
	ChangeTypeWithdrawFreeze: {
		LanguageChinese:            "提现冻结",
		LanguageTraditionalChinese: "提現凍結",
		LanguageEnglish:            "Withdraw Lock",
		LanguageJapanese:           "出金凍結",
		LanguageKorean:             "출금 동결",
	},
	ChangeTypeWithdraw: {
		LanguageChinese:            "提现扣减",
		LanguageTraditionalChinese: "提現扣減",
		LanguageEnglish:            "Withdraw Debit",
		LanguageJapanese:           "出金引き落とし",
		LanguageKorean:             "출금 차감",
	},
	ChangeTypeWithdrawUnfreeze: {
		LanguageChinese:            "提现解冻",
		LanguageTraditionalChinese: "提現解凍",
		LanguageEnglish:            "Withdraw Unlock",
		LanguageJapanese:           "出金解除",
		LanguageKorean:             "출금 해제",
	},
	ChangeTypeWithdrawSuccess: {
		LanguageChinese:            "提现成功",
		LanguageTraditionalChinese: "提現成功",
		LanguageEnglish:            "Withdraw Sent",
		LanguageJapanese:           "出金成功",
		LanguageKorean:             "출금 성공",
	},
	ChangeTypeWithdrawFailRefund: {
		LanguageChinese:            "提现失败退款",
		LanguageTraditionalChinese: "提現失敗退款",
		LanguageEnglish:            "Withdraw Refund",
		LanguageJapanese:           "出金失敗による返金",
		LanguageKorean:             "출금 실패 환불",
	},

	// 提现税款
	ChangeTypeWithdrawTaxPay: {
		LanguageChinese:            "提现税款扣除",
		LanguageTraditionalChinese: "提現稅款扣除",
		LanguageEnglish:            "Tax Debit",
		LanguageJapanese:           "出金税引き落とし",
		LanguageKorean:             "출금 세금 차감",
	},
	ChangeTypeWithdrawTaxReceive: {
		LanguageChinese:            "提现税款入账",
		LanguageTraditionalChinese: "提現稅款入賬",
		LanguageEnglish:            "Tax Received",
		LanguageJapanese:           "出金税受取",
		LanguageKorean:             "출금 세금 입금",
	},
	ChangeTypeWithdrawTaxRefundPay: {
		LanguageChinese:            "提现税款退还（扣回）",
		LanguageTraditionalChinese: "提現稅款退還（扣回）",
		LanguageEnglish:            "Tax Refund (Debit)",
		LanguageJapanese:           "出金税返金（差引）",
		LanguageKorean:             "출금 세금 환불 (차감)",
	},
	ChangeTypeWithdrawTaxRefundReceive: {
		LanguageChinese:            "提现税款退还",
		LanguageTraditionalChinese: "提現稅款退還",
		LanguageEnglish:            "Tax Refund",
		LanguageJapanese:           "出金税返金",
		LanguageKorean:             "출금 세금 환불",
	},
	ChangeTypeWithdrawTaxYYAI: {
		LanguageChinese:            "提现税款兑换YYAI",
		LanguageTraditionalChinese: "提現稅款兌換YYAI",
		LanguageEnglish:            "Tax Convert YYAI",
		LanguageJapanese:           "出金税YYAI換算",
		LanguageKorean:             "출금 세금 YYAI 전환",
	},
	ChangeTypeWithdrawTaxYYAIRollback: {
		LanguageChinese:            "提现税款YYAI回滚",
		LanguageTraditionalChinese: "提現稅款YYAI回滾",
		LanguageEnglish:            "Tax YYAI Rollback",
		LanguageJapanese:           "出金税YYAIロールバック",
		LanguageKorean:             "출금 세금 YYAI 롤백",
	},
	ChangeTypeUSStockRewardPool: {
		LanguageChinese:            "US Stock Reward 每日结算",
		LanguageTraditionalChinese: "US Stock Reward 每日結算",
		LanguageEnglish:            "US Stock Reward",
		LanguageJapanese:           "US Stock Reward 日次決済",
		LanguageKorean:             "US Stock Reward 일일 정산",
	},

	// 兑换
	ChangeTypeExchangeIn: {
		LanguageChinese:            "兑换到账",
		LanguageTraditionalChinese: "兌換到賬",
		LanguageEnglish:            "Swap In",
		LanguageJapanese:           "スワップによる入金",
		LanguageKorean:             "스왑 입금",
	},
	ChangeTypeExchangeOut: {
		LanguageChinese:            "兑换扣减",
		LanguageTraditionalChinese: "兌換扣減",
		LanguageEnglish:            "Swap Out",
		LanguageJapanese:           "スワップによる減算",
		LanguageKorean:             "스왑 차감",
	},
	ChangeTypeExchangeFreeze: {
		LanguageChinese:            "兑换冻结",
		LanguageTraditionalChinese: "兌換凍結",
		LanguageEnglish:            "Swap Lock",
		LanguageJapanese:           "スワップ凍結",
		LanguageKorean:             "스왑 동결",
	},
	ChangeTypeExchangeUnfreeze: {
		LanguageChinese:            "兑换解冻",
		LanguageTraditionalChinese: "兌換解凍",
		LanguageEnglish:            "Swap Unlock",
		LanguageJapanese:           "スワップ解除",
		LanguageKorean:             "스왑 해제",
	},

	// 奖励 / 分红
	ChangeTypeReward: {
		LanguageChinese:            "奖励",
		LanguageTraditionalChinese: "獎勵",
		LanguageEnglish:            "Reward",
		LanguageJapanese:           "報酬",
		LanguageKorean:             "보상",
	},
	ChangeTypeBonus: {
		LanguageChinese:            "分红",
		LanguageTraditionalChinese: "分紅",
		LanguageEnglish:            "Dividend",
		LanguageJapanese:           "配当",
		LanguageKorean:             "배당",
	},
	ChangeTypeRewardSettlement: {
		LanguageChinese:            "奖励结算",
		LanguageTraditionalChinese: "獎勵結算",
		LanguageEnglish:            "Reward Settle",
		LanguageJapanese:           "報酬決済",
		LanguageKorean:             "보상 정산",
	},
	ChangeTypeRewardReduced: {
		LanguageChinese:            "奖励削减",
		LanguageTraditionalChinese: "獎勵削減",
		LanguageEnglish:            "Reward Cut",
		LanguageJapanese:           "報酬減額",
		LanguageKorean:             "보상 차감",
	},

	// 节点购买
	ChangeTypeNodePurchase: {
		LanguageChinese:            "节点购买",
		LanguageTraditionalChinese: "節點購買",
		LanguageEnglish:            "Buy Node",
		LanguageJapanese:           "ノード購入",
		LanguageKorean:             "노드 구매",
	},
	ChangeTypeNodePurchaseDirectReward: {
		LanguageChinese:            "节点直推奖励",
		LanguageTraditionalChinese: "節點直推獎勵",
		LanguageEnglish:            "Node Direct Reward",
		LanguageJapanese:           "ノード直接紹介報酬",
		LanguageKorean:             "노드 직접 추천 보상",
	},

	// 拼团
	ChangeTypeGroupMatchJoinUSDT: {
		LanguageChinese:            "拼团参与（USDT）",
		LanguageTraditionalChinese: "拼團參與（USDT）",
		LanguageEnglish:            "Match Join",
		LanguageJapanese:           "マッチ参加（USDT）",
		LanguageKorean:             "매치 참가 (USDT)",
	},
	ChangeTypeGroupMatchJoinTicket: {
		LanguageChinese:            "拼团参与（票券）",
		LanguageTraditionalChinese: "拼團參與（票券）",
		LanguageEnglish:            "Match Join (Ticket)",
		LanguageJapanese:           "マッチ参加（チケット）",
		LanguageKorean:             "매치 참가 (티켓)",
	},
	ChangeTypeGroupMatchSettleWinner: {
		LanguageChinese:            "拼团中奖结算",
		LanguageTraditionalChinese: "拼團中獎結算",
		LanguageEnglish:            "Win Settle",
		LanguageJapanese:           "マッチ当選決済",
		LanguageKorean:             "매치 당첨 정산",
	},
	ChangeTypeGroupMatchSettleTicketRefund: {
		LanguageChinese:            "拼团门票退还",
		LanguageTraditionalChinese: "拼團門票退還",
		LanguageEnglish:            "Ticket Refund",
		LanguageJapanese:           "マッチチケット返金",
		LanguageKorean:             "매치 티켓 환불",
	},
	ChangeTypeGroupMatchSettleTicketBurnToVtx: {
		LanguageChinese:            "拼团门票销毁",
		LanguageTraditionalChinese: "拼團門票銷毀",
		LanguageEnglish:            "Ticket Burn",
		LanguageJapanese:           "マッチチケットバーン",
		LanguageKorean:             "매치 티켓 소각",
	},
	ChangeTypeGroupMatchSettleFlowRefund: {
		LanguageChinese:            "拼团流团退款",
		LanguageTraditionalChinese: "拼團流團退款",
		LanguageEnglish:            "Flow Refund",
		LanguageJapanese:           "マッチ枠返金",
		LanguageKorean:             "매치 위치 환불",
	},
	ChangeTypeGroupMatchLoserCompRelease: {
		LanguageChinese:            "拼团败者补偿释放",
		LanguageTraditionalChinese: "拼團敗者補償釋放",
		LanguageEnglish:            "Void Free",
		LanguageJapanese:           "マッチ敗者補償リリース",
		LanguageKorean:             "매치 패자 보상 해제",
	},
	ChangeTypeGroupMatchLeadershipReward: {
		LanguageChinese:            "拼团领导池奖励",
		LanguageTraditionalChinese: "拼團領導池獎勵",
		LanguageEnglish:            "Leader Reward",
		LanguageJapanese:           "マッチリーダー報酬",
		LanguageKorean:             "매치 리더십 풀 보상",
	},
	ChangeTypeGroupMatchLeadershipWeightReward: {
		LanguageChinese:            "拼团领导池权重奖励",
		LanguageTraditionalChinese: "拼團領導池權重獎勵",
		LanguageEnglish:            "Leader Weight Reward",
		LanguageJapanese:           "マッチリーダー加重報酬",
		LanguageKorean:             "매치 리더십 가중치 보상",
	},
	ChangeTypeGroupMatchTeamReward: {
		LanguageChinese:            "拼团团队奖励",
		LanguageTraditionalChinese: "拼團團隊獎勵",
		LanguageEnglish:            "Team Reward",
		LanguageJapanese:           "マッチチーム報酬",
		LanguageKorean:             "매치 팀 보상",
	},

	// 三倍券购买（原质押）
	ChangeTypeStakingV2Stake: {
		LanguageChinese:            "三倍券购买",
		LanguageTraditionalChinese: "三倍券购买",
		LanguageEnglish:            "Buy Triple",
		LanguageJapanese:           "トリプル券購入",
		LanguageKorean:             "트리플 바우처 구매",
	},
	ChangeTypeStakingV2ReferralDirect: {
		LanguageChinese:            "三倍券直推奖励",
		LanguageTraditionalChinese: "三倍券直推奖励",
		LanguageEnglish:            "Triple Direct Reward",
		LanguageJapanese:           "トリプル券直接紹介報酬",
		LanguageKorean:             "트리플 바우처 직접 추천 보상",
	},
	ChangeTypeStakingV2ReferralIndirect: {
		LanguageChinese:            "三倍券间推奖励",
		LanguageTraditionalChinese: "三倍券间推奖励",
		LanguageEnglish:            "Triple Indirect Reward",
		LanguageJapanese:           "トリプル券間接紹介報酬",
		LanguageKorean:             "트리플 바우처 간접 추천 보상",
	},
	ChangeTypeStakingV2ReferralBurnToVertex: {
		LanguageChinese:            "三倍券间推销毁",
		LanguageTraditionalChinese: "三倍券间推销毁",
		LanguageEnglish:            "Triple Referral Burn",
		LanguageJapanese:           "トリプル券紹介バーン",
		LanguageKorean:             "트리플 바우처 추천 소각",
	},
	ChangeTypeStakingV2LeaderReward: {
		LanguageChinese:            "三倍券领导奖",
		LanguageTraditionalChinese: "三倍券领导奖",
		LanguageEnglish:            "Triple Leader Reward",
		LanguageJapanese:           "トリプル券リーダー報酬",
		LanguageKorean:             "트리플 바우처 리더 보상",
	},
	ChangeTypeTripleToStakeV2: {
		LanguageChinese:            "三倍券转质押",
		LanguageTraditionalChinese: "三倍券轉質押",
		LanguageEnglish:            "Triple to Stake",
		LanguageJapanese:           "トリプル券をステーキングに変換",
		LanguageKorean:             "트리플 바우처 스테이킹 전환",
	},
	ChangeTypeYYAIToBalance: {
		LanguageChinese:            "YYAI转余额",
		LanguageTraditionalChinese: "YYAI转余额",
		LanguageEnglish:            "YYAI Swap In",
		LanguageJapanese:           "YYAI残高振替",
		LanguageKorean:             "YYAI 잔고 전환",
	},

	// 手续费 / 销毁
	ChangeTypeFee: {
		LanguageChinese:            "手续费",
		LanguageTraditionalChinese: "手續費",
		LanguageEnglish:            "Fee",
		LanguageJapanese:           "手数料",
		LanguageKorean:             "수수료",
	},
	ChangeTypeBurn: {
		LanguageChinese:            "销毁",
		LanguageTraditionalChinese: "銷毀",
		LanguageEnglish:            "Burn",
		LanguageJapanese:           "バーン",
		LanguageKorean:             "소각",
	},

	// 转账
	ChangeTypeTransferIn: {
		LanguageChinese:            "内转-转入",
		LanguageTraditionalChinese: "內轉-轉入",
		LanguageEnglish:            "Transfer In",
		LanguageJapanese:           "内部送金（受取）",
		LanguageKorean:             "내부 송금 (입금)",
	},
	ChangeTypeTransferOut: {
		LanguageChinese:            "内转-转出",
		LanguageTraditionalChinese: "內轉-轉出",
		LanguageEnglish:            "Transfer Out",
		LanguageJapanese:           "内部送金（送付）",
		LanguageKorean:             "내부 송금 (출금)",
	},

	// 调整
	ChangeTypeAdjust: {
		LanguageChinese:            "管理员调整",
		LanguageTraditionalChinese: "管理員調整",
		LanguageEnglish:            "Admin Adjust",
		LanguageJapanese:           "管理者調整",
		LanguageKorean:             "관리자 조정",
	},
	ChangeTypeSystemAdjust: {
		LanguageChinese:            "系统调整",
		LanguageTraditionalChinese: "系統調整",
		LanguageEnglish:            "System Adjust",
		LanguageJapanese:           "システム調整",
		LanguageKorean:             "시스템 조정",
	},

	// 会议报销
	ChangeTypeMeetingReimbursementFirst: {
		LanguageChinese:            "会议报销第一批",
		LanguageTraditionalChinese: "會議報銷第一批",
		LanguageEnglish:            "Meeting Reimburse 1st",
		LanguageJapanese:           "会議経費第1回",
		LanguageKorean:             "회의 비용 첫 번째 지급",
	},
	ChangeTypeMeetingReimbursementSecond: {
		LanguageChinese:            "会议报销第二批",
		LanguageTraditionalChinese: "會議報銷第二批",
		LanguageEnglish:            "Meeting Reimburse 2nd",
		LanguageJapanese:           "会議経費第2回",
		LanguageKorean:             "회의 비용 두 번째 지급",
	},
	ChangeTypeTableShareReward: {
		LanguageChinese:            "饭桌分享会奖励",
		LanguageTraditionalChinese: "飯桌分享會獎勵",
		LanguageEnglish:            "Table Share",
		LanguageJapanese:           "テーブルシェア報酬",
		LanguageKorean:             "테이블 공유 보상",
	},
	ChangeTypeOfficeApplySubsidy: {
		LanguageChinese:            "工作室申请补贴",
		LanguageTraditionalChinese: "工作室申請補貼",
		LanguageEnglish:            "Office Subsidy",
		LanguageJapanese:           "オフィス申請補助",
		LanguageKorean:             "오피스 신청 보조금",
	},
}

// BalanceChangeText 返回指定 change_type 在 locale 下的本地化描述
// 命中规则：
//  1. 精确匹配 changeType + locale
//  2. 回落到 LanguageDefault
//  3. 回落到 LanguageEnglish
//  4. 仍未命中时返回 changeType 原始字符串（保底，便于排查未翻译类型）
//
// 若文案中包含 {symbol} 占位符且未提供 symbol，默认替换为 "USDT"（向后兼容）
func BalanceChangeText(changeType, locale string) string {
	return BalanceChangeTextWithSymbol(changeType, locale, "USDT")
}

// BalanceChangeTextWithSymbol 返回指定 change_type 在 locale 下的本地化描述，支持 {symbol} 占位符替换
func BalanceChangeTextWithSymbol(changeType, locale, symbol string) string {
	m, ok := balanceChangeTexts[changeType]
	if !ok {
		return changeType
	}
	var text string
	if v, ok := m[locale]; ok && v != "" {
		text = v
	} else if v, ok := m[LanguageDefault]; ok && v != "" {
		text = v
	} else if v, ok := m[LanguageEnglish]; ok && v != "" {
		text = v
	} else {
		return changeType
	}
	if symbol != "" {
		text = strings.ReplaceAll(text, "{symbol}", symbol)
	}
	return text
}

// HasBalanceChangeText 判断指定 change_type 是否在翻译表中（用于单元测试或健康检查）
func HasBalanceChangeText(changeType string) bool {
	_, ok := balanceChangeTexts[changeType]
	return ok
}
