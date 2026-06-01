package group_match

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/repository"
	coboRepo "XWFrame/internal/repository/cobo"
	"XWFrame/internal/service/group_match/model"
	"XWFrame/internal/service/shared"
	"XWFrame/internal/service/staking_v2"
	"XWFrame/pkg/utils"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	groupMatchChangeTypeUSDT   = consts.ChangeTypeGroupMatchJoinUSDT
	groupMatchChangeTypeTicket = consts.ChangeTypeGroupMatchJoinTicket
)

var errJoinMatchIdempotentConflict = errors.New("join match idempotent conflict")

var joinErrMsgs = map[string]map[string]string{
	"invalid_param": {
		"zh-CN": "参数无效",
		"zh-TW": "參數無效",
		"en-US": "invalid parameter",
		"ja-JP": "無効なパラメータです",
		"ko-KR": "유효하지 않은 파라미터입니다",
	},
	"request_id_too_long": {
		"zh-CN": "request_id 不能超过64个字符",
		"zh-TW": "request_id 不能超過64個字元",
		"en-US": "request_id must not exceed 64 characters",
		"ja-JP": "request_id は64文字を超えてはいけません",
		"ko-KR": "request_id 는 64자를 초과할 수 없습니다",
	},
	"count_must_one_with_request_id": {
		"zh-CN": "使用 request_id 时 count 必须为1",
		"zh-TW": "使用 request_id 時 count 必須為1",
		"en-US": "count must be 1 when request_id is provided",
		"ja-JP": "request_id を使用する場合、count は1である必要があります",
		"ko-KR": "request_id 를 사용할 때 count 는 1이어야 합니다",
	},
	"count_too_small": {
		"zh-CN": "count 必须为正整数",
		"zh-TW": "count 必須為正整數",
		"en-US": "count must be a positive integer",
		"ja-JP": "count は正の整数である必要があります",
		"ko-KR": "count 는 양의 정수여야 합니다",
	},
	"count_invalid": {
		"zh-CN": "count 必须为正整数，且不超过 %d",
		"zh-TW": "count 必須為正整數，且不超過 %d",
		"en-US": "count must be a positive integer, and no greater than %d",
		"ja-JP": "count は正の整数で、%d を超えてはいけません",
		"ko-KR": "count 는 양의 정수이며 %d 를 초과할 수 없습니다",
	},
	"locale_too_long": {
		"zh-CN": "字段不可超过20",
		"zh-TW": "字段不可超過20",
		"en-US": "Field must not exceed 20",
		"ja-JP": "フィールドは20文字以内",
		"ko-KR": "필드는 20자 이하여야 합니다",
	},
	"ticket_symbol_required": {
		"zh-CN": "ticket_symbol 不能为空",
		"zh-TW": "ticket_symbol 不能為空",
		"en-US": "ticket_symbol is required",
		"ja-JP": "ticket_symbol は必須です",
		"ko-KR": "ticket_symbol 은 필수입니다",
	},
	"no_active_session": {
		"zh-CN": "暂未开放喔",
		"zh-TW": "暫未開放喔",
		"en-US": "Not available yet",
		"ja-JP": "まだ開放されていません",
		"ko-KR": "아직 오픈되지 않았습니다",
	},
	"session_not_found": {
		"zh-CN": "场次不存在",
		"zh-TW": "場次不存在",
		"en-US": "session not found",
		"ja-JP": "セッションが見つかりません",
		"ko-KR": "세션을 찾을 수 없습니다",
	},
	"session_not_active": {
		"zh-CN": "场次未激活",
		"zh-TW": "場次未啟用",
		"en-US": "session is not active",
		"ja-JP": "セッションは有効化されていません",
		"ko-KR": "세션이 활성화되어 있지 않습니다",
	},
	"session_not_started": {
		"zh-CN": "即将开始喔",
		"zh-TW": "即將開始喔",
		"en-US": "Starting soon",
		"ja-JP": "まもなく開始します",
		"ko-KR": "곧 시작됩니다",
	},
	"session_ended": {
		"zh-CN": "场次已结束",
		"zh-TW": "場次已結束",
		"en-US": "session has ended",
		"ja-JP": "セッションは終了しました",
		"ko-KR": "세션이 종료되었습니다",
	},
	"session_unavailable": {
		"zh-CN": "当前不可用喔",
		"zh-TW": "當前不可用喔",
		"en-US": "Currently unavailable",
		"ja-JP": "現在利用不可",
		"ko-KR": "현재 사용 불가",
	},
	"staking_required": {
		"zh-CN": "先买三倍卷噢",
		"zh-TW": "先買三倍卷噢",
		"en-US": "Buy Triple voucher first",
		"ja-JP": "先にトリプル券を購入してください",
		"ko-KR": "먼저 트리플 바우처를 구매하세요",
	},
	"unsupported_token": {
		"zh-CN": "Gas不支持",
		"zh-TW": "Gas不支援",
		"en-US": "Gas not supported",
		"ja-JP": "Gasは非対応",
		"ko-KR": "Gas는 지원되지 않습니다",
	},
	"price_unavailable": {
		"zh-CN": "Gas不可用",
		"zh-TW": "Gas不可用",
		"en-US": "Gas unavailable",
		"ja-JP": "Gasは利用不可",
		"ko-KR": "Gas를 사용할 수 없습니다",
	},
	"exceeds_max_orders": {
		"zh-CN": "单场上限50单喔",
		"zh-TW": "單場上限50單喔",
		"en-US": "Max 50 orders per session",
		"ja-JP": "1セッション最大50注文",
		"ko-KR": "세션당 최대 50주문",
	},
	"insufficient_usdt": {
		"zh-CN": "U额不足喔",
		"zh-TW": "U額不足喔",
		"en-US": "Insufficient USDT",
		"ja-JP": "USDT不足",
		"ko-KR": "USDT 부족",
	},
	"insufficient_ticket": {
		"zh-CN": "Gas不足喔",
		"zh-TW": "Gas不足喔",
		"en-US": "Insufficient Gas",
		"ja-JP": "Gas不足",
		"ko-KR": "Gas 부족",
	},
	"request_id_conflict": {
		"zh-CN": "request_id 冲突，请更换后重试",
		"zh-TW": "request_id 衝突，請更換後重試",
		"en-US": "request_id conflict, please retry with a different one",
		"ja-JP": "request_id が競合しています。別の値で再試行してください",
		"ko-KR": "request_id 가 충돌했습니다. 다른 값으로 다시 시도해 주세요",
	},
}

func joinErr(locale, key string) error {
	locale = consts.NormalizeLanguage(locale)
	if locale == "" {
		locale = consts.LanguageDefault
	}
	m := joinErrMsgs[key]
	msg := consts.LocalizedText(m, locale)
	if msg == "" {
		return gerror.New(key)
	}
	return gerror.New(msg)
}

type joinErrKey string

func joinErrf(locale string, key joinErrKey, args ...interface{}) error {
	locale = consts.NormalizeLanguage(locale)
	if locale == "" {
		locale = consts.LanguageDefault
	}
	m := joinErrMsgs[string(key)]
	msg := consts.LocalizedText(m, locale)
	if msg == "" {
		return gerror.Newf(string(key), args...)
	}
	return gerror.Newf(msg, args...)
}

type IGroupMatchService interface {
	GetCurrentSession(ctx context.Context) (*model.CurrentSessionRes, error)
	GetSupportedTicketSymbols(ctx context.Context) []string
	GetSessionTimeRanges(ctx context.Context) (*model.SessionTimeRangesRes, error)
	GetSessionList(ctx context.Context, req *model.GetSessionListReq) (*model.GetSessionListRes, error)
	GetHistoryStats(ctx context.Context, req *model.GetHistoryStatsReq) (*model.GetHistoryStatsRes, error)
	GetLatestJoins(ctx context.Context, req *model.GetLatestJoinsReq) (*model.GetLatestJoinsRes, error)
	GetTopJoiners(ctx context.Context, req *model.GetTopJoinersReq) (*model.GetTopJoinersRes, error)
	GetSessionDetail(ctx context.Context, req *model.GetSessionDetailReq) (*model.GetSessionDetailRes, error)
	GetSessionLoserDetail(ctx context.Context, req *model.GetSessionLoserDetailReq) (*model.GetSessionLoserDetailRes, error)
	JoinGroup(ctx context.Context, req *model.JoinGroupReq) (*model.JoinGroupRes, error)
	GetMyOrders(ctx context.Context, req *model.GetMyOrdersReq) (*model.GetMyOrdersRes, error)
	GetParticipationStats(ctx context.Context, req *model.GetParticipationStatsReq) (*model.GetParticipationStatsRes, error)
	GetCurrentParticipationStats(ctx context.Context, req *model.GetCurrentParticipationStatsReq) (*model.GetCurrentParticipationStatsRes, error)
	GetBalanceChanges(ctx context.Context, req *model.GetBalanceChangesReq) (*model.GetBalanceChangesRes, error)
	GetLeadershipLevelsConfig(ctx context.Context) (*model.LeadershipLevelsConfigRes, error)
	ActivatePendingSessions(ctx context.Context) (int64, error)
	SettleDueSessions(ctx context.Context, limit int) (int, error)
	SettleSessionByID(ctx context.Context, sessionID int64) error
	UnsettleSessionByID(ctx context.Context, sessionID int64) error
	ClearSessionByID(ctx context.Context, sessionID int64) error
	ReleaseLoserCompensations(ctx context.Context, limit int) (int, error)
	ReleaseLoserCompensationsSince(ctx context.Context, limit int, minNextReleaseAt time.Time) (int, error)
	ReleaseLoserCompensationsBefore(ctx context.Context, limit int, maxNextReleaseAt time.Time) (int, error)
	CreateDailySessions(ctx context.Context) (int64, error)
	DistributeLeadershipRewards(ctx context.Context, bizDate time.Time) (int, error)
	RefreshGroupPerformanceCache(ctx context.Context) error
	ResolveCurrentSessionID(ctx context.Context) (int64, error)
	GetUserLeadershipLevel(ctx context.Context, userID int64, bizDate time.Time) (*model.UserLeadershipLevel, error)
	GetUserLeadershipRewardPreview(ctx context.Context, userID int64, bizDate time.Time) (*model.UserLeadershipRewardPreview, error)
	DistributeTeamReward(ctx context.Context, sessionID int64) error
	RedistributeTeamReward(ctx context.Context, sessionID int64, forceRefreshPerformance bool, allowZeroPerformance bool) error
	GetTodayReward(ctx context.Context, userID int64, dateStr string) (*model.GetTodayRewardRes, error)
	GetLoserOutputStats(ctx context.Context, userID int64) (*model.GetLoserOutputStatsRes, error)
	GetLoserReleaseLog(ctx context.Context, req *model.GetLoserReleaseLogReq) (*model.GetLoserReleaseLogRes, error)
	GetSessionFakeOrderCount(ctx context.Context, sessionID int64) (int, error)
	GetAutoSimulationConfig(ctx context.Context) AutoSimulationConfig
}

// AutoSimulationConfig 自动模拟调度参数,来自 system_config 表。
type AutoSimulationConfig struct {
	MinTarget   int
	MaxTarget   int
	DurationSec int
}

type groupMatchService struct {
	balanceRepo     coboRepo.IBalanceRepository
	stockPriceDao   dao.IStockPriceDao
	tokenConfigRepo repository.ITokenConfigRepository
}

func NewGroupMatchService() IGroupMatchService {
	return &groupMatchService{
		balanceRepo:     coboRepo.NewBalanceRepository(),
		stockPriceDao:   dao.NewStockPriceDao(),
		tokenConfigRepo: repository.NewTokenConfigRepository(),
	}
}

func (s *groupMatchService) GetSessionFakeOrderCount(ctx context.Context, sessionID int64) (int, error) {
	val, err := g.DB().Model("fake_group_order").Ctx(ctx).
		Fields("COALESCE(SUM(new_order), 0)").
		Where("session_id", sessionID).
		Value()
	if err != nil {
		return 0, gerror.Wrap(err, "query fake order count failed")
	}
	return val.Int(), nil
}

func (s *groupMatchService) GetAutoSimulationConfig(ctx context.Context) AutoSimulationConfig {
	minTarget := s.mustReadIntConfig(ctx, "group_match_auto_sim_min_target", 35000)
	maxTarget := s.mustReadIntConfig(ctx, "group_match_auto_sim_max_target", 41000)
	durationSec := s.mustReadIntConfig(ctx, "group_match_auto_sim_duration_sec", 4*60*60)
	if minTarget <= 0 {
		minTarget = 35000
	}
	if maxTarget < minTarget {
		maxTarget = minTarget
	}
	if durationSec <= 0 {
		durationSec = 4 * 60 * 60
	}
	return AutoSimulationConfig{MinTarget: minTarget, MaxTarget: maxTarget, DurationSec: durationSec}
}

func (s *groupMatchService) SettleSessionByID(ctx context.Context, sessionID int64) error {
	_, err := s.settleSingleSession(ctx, sessionID, true)
	return err
}

func (s *groupMatchService) GetCurrentSession(ctx context.Context) (*model.CurrentSessionRes, error) {
	type sessionRow struct {
		ID          int64     `json:"id"`
		SessionDate time.Time `json:"session_date"`
		SessionName string    `json:"session_name"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
		Status      int       `json:"status"`
	}

	var row sessionRow
	err := g.DB().Model("group_match_session").Ctx(ctx).
		Where("status = ?", consts.GroupMatchSessionStatusActive).
		Where("start_time <= NOW()").
		Where("end_time > NOW()").
		OrderAsc("start_time").
		Limit(1).
		Scan(&row)
	if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
		return nil, gerror.Wrap(err, "query current session failed")
	}
	if row.ID == 0 {
		return nil, nil
	}

	return &model.CurrentSessionRes{
		ID:          row.ID,
		SessionDate: row.SessionDate.Format("2006-01-02"),
		SessionName: row.SessionName,
		StartTime:   row.StartTime.Format("2006-01-02 15:04:05"),
		EndTime:     row.EndTime.Format("2006-01-02 15:04:05"),
		Status:      row.Status,
	}, nil
}

func (s *groupMatchService) GetSessionTimeRanges(ctx context.Context) (*model.SessionTimeRangesRes, error) {
	cfgVal := s.mustReadStringConfig(ctx, "group_match_session_times", "")
	if cfgVal == "" {
		return nil, gerror.New("group_match_session_times config not found")
	}

	var cfg sessionTimeConfig
	if err := gjson.DecodeTo(cfgVal, &cfg); err != nil {
		return nil, gerror.Wrap(err, "parse group_match_session_times config failed")
	}
	if len(cfg.Sessions) == 0 {
		return nil, gerror.New("group_match_session_times config is empty")
	}

	order := map[string]int{"morning": 1, "afternoon": 2, "evening": 3}
	type sortItem struct {
		item  *model.SessionTimeRangeItem
		index int
	}
	items := make([]sortItem, 0, len(cfg.Sessions))

	for _, se := range cfg.Sessions {
		normalized := normalizeSessionType(se.Name)
		if normalized == "" {
			continue
		}
		items = append(items, sortItem{
			item: &model.SessionTimeRangeItem{
				SessionName: se.Name,
				StartTime:   normalizeClock(se.Start),
				EndTime:     normalizeClock(se.End),
			},
			index: order[normalized],
		})
	}

	if len(items) == 0 {
		return nil, gerror.New("group_match_session_times has no valid session config")
	}

	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].index > items[j].index {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	list := make([]*model.SessionTimeRangeItem, 0, len(items))
	for _, it := range items {
		list = append(list, it.item)
	}

	return &model.SessionTimeRangesRes{Timezone: "Asia/Shanghai", List: list}, nil
}

func normalizeClock(v string) string {
	v = strings.TrimSpace(v)
	if len(v) == 5 {
		return v + ":00"
	}
	return v
}

func (s *groupMatchService) JoinGroup(ctx context.Context, req *model.JoinGroupReq) (*model.JoinGroupRes, error) {
	locale := consts.LocaleFromCtx(ctx)
	if req != nil {
		if strings.TrimSpace(req.Locale) != "" {
			if len(strings.TrimSpace(req.Locale)) > 20 {
				return nil, joinErr(locale, "locale_too_long")
			}
			locale = req.Locale
		}
	}
	locale = consts.NormalizeLanguage(locale)
	if locale == "" {
		locale = consts.LanguageDefault
	}

	if req == nil || req.UserID <= 0 {
		return nil, joinErr(locale, "invalid_param")
	}
	if req.Count == 0 {
		req.Count = 1
	}
	if req.Count < 1 {
		return nil, joinErr(locale, "count_too_small")
	}

	requestID := strings.TrimSpace(req.RequestID)
	if len(requestID) > 64 {
		return nil, joinErr(locale, "request_id_too_long")
	}
	if requestID != "" && req.Count > 1 {
		return nil, joinErr(locale, "count_must_one_with_request_id")
	}

	ticketSymbol := strings.ToUpper(strings.TrimSpace(req.TicketSymbol))
	if ticketSymbol == "" {
		return nil, joinErr(locale, "ticket_symbol_required")
	}

	sessionID, err := s.resolveJoinSession(ctx, req.SessionID, locale)
	if err != nil {
		return nil, err
	}

	if requestID != "" {
		existing, err := s.getJoinGroupByRequestID(ctx, req.UserID, sessionID, requestID)
		if err != nil {
			return nil, gerror.Wrap(err, "query idempotent record failed")
		}
		if existing != nil {
			return existing, nil
		}
	}

	if ok, err := staking_v2.NewStakingV2Service().HasActiveStake(ctx, req.UserID); err != nil {
		return nil, err
	} else if !ok {
		return nil, joinErr(locale, "staking_required")
	}

	singleAmount := s.mustReadDecimalConfig(ctx, "group_match_single_amount", "100")
	ticketUSDTValue := s.mustReadDecimalConfig(ctx, "group_match_ticket_amount", "5")
	maxOrders := s.mustReadIntConfig(ctx, "group_match_max_orders_per_session", 50)

	if req.Count > maxOrders {
		return nil, joinErrf(locale, joinErrKey("count_invalid"), maxOrders)
	}

	ticketTokens, err := s.tokenConfigRepo.GetTicketTokens(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "query ticket tokens failed")
	}
	if !containsTicketToken(ticketTokens, ticketSymbol) {
		return nil, joinErr(locale, "unsupported_token")
	}

	ticketPrice, err := s.getTicketPrice(ctx, ticketSymbol)
	if err != nil {
		return nil, joinErr(locale, "price_unavailable")
	}
	ticketAmountPerOrder := ticketUSDTValue.Div(ticketPrice)
	totalUSDT := singleAmount.Mul(decimal.NewFromInt(int64(req.Count)))
	totalTicket := ticketAmountPerOrder.Mul(decimal.NewFromInt(int64(req.Count)))

	orders := make([]*model.JoinOrderItem, 0, req.Count)
	var usdtLeft decimal.Decimal
	var ticketLeft decimal.Decimal

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// 提前获取 USDT 余额行锁（FOR UPDATE），同一用户并发请求会串行化，避免 currentCount 并发竞态
		usdtBefore, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, "USDT")
		if err != nil {
			return gerror.Wrap(err, "query USDT balance failed")
		}
		if usdtBefore == nil || usdtBefore.AvailableAmount.LessThan(totalUSDT) {
			usdtAvail := decimal.Zero
			if usdtBefore != nil {
				usdtAvail = usdtBefore.AvailableAmount
			}
			short := totalUSDT.Sub(usdtAvail).StringFixedBank(2)
			return joinErrf(locale, joinErrKey("insufficient_usdt"), totalUSDT.StringFixedBank(2), short)
		}

		currentCount, err := tx.Model("group_match_order").Ctx(ctx).
			Where("session_id = ? AND user_id = ?", sessionID, req.UserID).
			Count()
		if err != nil {
			return gerror.Wrap(err, "query order count failed")
		}
		if currentCount+req.Count > maxOrders {
			return joinErr(locale, "exceeds_max_orders")
		}

		if requestID != "" {
			dupCount, err := tx.Model("group_match_order").Ctx(ctx).
				Where("user_id = ? AND session_id = ? AND request_id = ?", req.UserID, sessionID, requestID).
				Count()
			if err != nil {
				return gerror.Wrap(err, "query idempotent record failed")
			}
			if dupCount > 0 {
				return errJoinMatchIdempotentConflict
			}
		}

		ticketBefore, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, ticketSymbol)
		if err != nil {
			return gerror.Wrap(err, "query ticket balance failed")
		}
		if ticketBefore == nil || ticketBefore.AvailableAmount.LessThan(totalTicket) {
			ticketAvail := decimal.Zero
			if ticketBefore != nil {
				ticketAvail = ticketBefore.AvailableAmount
			}
			short := totalTicket.Sub(ticketAvail).StringFixedBank(2)
			return joinErrf(locale, joinErrKey("insufficient_ticket"), ticketSymbol, totalTicket.StringFixedBank(2), ticketSymbol, short, ticketSymbol)
		}

		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, req.UserID, "USDT", totalUSDT.Neg(), decimal.Zero); err != nil {
			return gerror.Wrap(err, "deduct USDT failed")
		}
		if err := s.balanceRepo.UpdateBalanceTx(ctx, tx, req.UserID, ticketSymbol, totalTicket.Neg(), decimal.Zero); err != nil {
			return gerror.Wrap(err, "deduct ticket token failed")
		}

		usdtAfter, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, "USDT")
		if err != nil {
			return gerror.Wrap(err, "query USDT balance failed")
		}
		ticketAfter, err := s.balanceRepo.GetByUserIDForUpdate(ctx, tx, req.UserID, ticketSymbol)
		if err != nil {
			return gerror.Wrap(err, "query ticket balance failed")
		}

		relatedOrderNo := requestID
		if relatedOrderNo == "" {
			relatedOrderNo = "GMB-" + utils.GenerateSnowflakeId()
		}
		if err := s.createCoboBalanceChangeLogTx(ctx, tx, req.UserID, "USDT", groupMatchChangeTypeUSDT, totalUSDT.Neg(), usdtBefore.AvailableAmount, usdtAfter.AvailableAmount, relatedOrderNo, sessionID); err != nil {
			return err
		}
		if err := s.createCoboBalanceChangeLogTx(ctx, tx, req.UserID, ticketSymbol, groupMatchChangeTypeTicket, totalTicket.Neg(), ticketBefore.AvailableAmount, ticketAfter.AvailableAmount, relatedOrderNo, sessionID); err != nil {
			return err
		}

		usdtLeft = usdtAfter.AvailableAmount
		ticketLeft = ticketAfter.AvailableAmount

		for i := 0; i < req.Count; i++ {
			orderNo := "GM-" + utils.GenerateSnowflakeId()
			_, err := tx.Model("group_match_order").Ctx(ctx).Data(g.Map{
				"session_id":        sessionID,
				"user_id":           req.UserID,
				"request_id":        requestID,
				"order_no":          orderNo,
				"amount":            singleAmount,
				"ticket_symbol":     ticketSymbol,
				"ticket_amount":     ticketAmountPerOrder,
				"ticket_usdt_value": ticketUSDTValue,
				"status":            consts.GroupMatchOrderStatusPending,
				"locale":            locale,
				"created_at":        time.Now(),
				"updated_at":        time.Now(),
			}).Insert()
			if err != nil {
				if requestID != "" && strings.Contains(strings.ToLower(err.Error()), "idx_gm_order_user_session_request_id") {
					return errJoinMatchIdempotentConflict
				}
				return gerror.Wrap(err, "create group match order failed")
			}
			orders = append(orders, &model.JoinOrderItem{OrderNo: orderNo, Amount: singleAmount.String(), TicketSymbol: ticketSymbol, TicketAmount: ticketAmountPerOrder.String(), Status: consts.GroupMatchOrderStatusPending})
		}

		_, err = tx.Exec("UPDATE group_match_session SET total_orders = total_orders + ? WHERE id = ?", req.Count, sessionID)
		return err
	})
	if err != nil {
		if errors.Is(err, errJoinMatchIdempotentConflict) {
			existing, queryErr := s.getJoinGroupByRequestID(ctx, req.UserID, sessionID, requestID)
			if queryErr != nil {
				return nil, queryErr
			}
			if existing == nil {
				return nil, joinErr(locale, "request_id_conflict")
			}
			return existing, nil
		}
		return nil, err
	}

	s.markGroupPerformanceDirty(ctx, sessionID)
	s.asyncRefreshGroupPerformance(ctx, sessionID)

	return &model.JoinGroupRes{SessionID: sessionID, Orders: orders, TotalUSDT: totalUSDT.String(), TotalTicket: totalTicket.String(), UsdtLeft: usdtLeft.String(), TicketLeft: ticketLeft.String(), TicketSymbol: ticketSymbol, TicketPrice: ticketPrice.String(), TicketUSDTVal: ticketUSDTValue.String()}, nil
}

func (s *groupMatchService) GetSessionList(ctx context.Context, req *model.GetSessionListReq) (*model.GetSessionListRes, error) {
	if req == nil {
		req = &model.GetSessionListReq{}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	m := g.DB().Model("group_match_session").Ctx(ctx)
	total, err := m.Count()
	if err != nil {
		return nil, gerror.Wrap(err, "query session total count failed")
	}

	type row struct {
		ID                 int64     `json:"id"`
		SessionDate        time.Time `json:"session_date"`
		SessionName        string    `json:"session_name"`
		StartTime          time.Time `json:"start_time"`
		EndTime            time.Time `json:"end_time"`
		Status             int       `json:"status"`
		TotalOrders        int       `json:"total_orders"`
		TotalGroups        int       `json:"total_groups"`
		TotalFlowUserCount int       `json:"total_flow_user_count"`
	}
	var rows []*row
	if err := m.OrderDesc("id").Page(req.Page, req.PageSize).Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "query session list failed")
	}

	list := make([]*model.SessionItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, &model.SessionItem{
			SessionID:          r.ID,
			SessionDate:        r.SessionDate.Format("2006-01-02"),
			SessionName:        r.SessionName,
			StartTime:          r.StartTime.Format("2006-01-02 15:04:05"),
			EndTime:            r.EndTime.Format("2006-01-02 15:04:05"),
			Status:             r.Status,
			TotalOrders:        r.TotalOrders,
			TotalGroups:        r.TotalGroups,
			TotalFlowUserCount: r.TotalFlowUserCount,
		})
	}

	return &model.GetSessionListRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (s *groupMatchService) GetHistoryStats(ctx context.Context, req *model.GetHistoryStatsReq) (*model.GetHistoryStatsRes, error) {
	if req == nil {
		req = &model.GetHistoryStatsReq{}
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	type sessionRow struct {
		ID          int64     `json:"id"`
		SessionDate time.Time `json:"session_date"`
		SessionName string    `json:"session_name"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
		Status      int       `json:"status"`
		TotalOrders int       `json:"total_orders"`
	}

	m := g.DB().Model("group_match_session").Ctx(ctx).
		Where("status IN (?)", g.Slice{consts.GroupMatchSessionStatusSettling, consts.GroupMatchSessionStatusCompleted})
	total, err := m.Count()
	if err != nil {
		return nil, gerror.Wrap(err, "query session total count failed")
	}

	var sessions []*sessionRow
	if err := m.OrderDesc("id").Page(req.Page, req.PageSize).Scan(&sessions); err != nil {
		return nil, gerror.Wrap(err, "query session list failed")
	}

	fakeMap := make(map[int64]int, len(sessions))
	if len(sessions) > 0 {
		sessionIDs := make([]int64, 0, len(sessions))
		for _, se := range sessions {
			sessionIDs = append(sessionIDs, se.ID)
		}
		type fakeRow struct {
			SessionID int64 `json:"session_id"`
			TotalFake int   `json:"total_fake"`
		}
		var fakeRows []*fakeRow
		if err := g.DB().Model("fake_group_order").Ctx(ctx).
			Fields("session_id, COALESCE(SUM(new_order), 0) AS total_fake").
			Where("session_id", sessionIDs).
			Group("session_id").
			Scan(&fakeRows); err != nil {
			return nil, gerror.Wrap(err, "query session fake order stats failed")
		}
		for _, fr := range fakeRows {
			fakeMap[fr.SessionID] = fr.TotalFake
		}
	}

	list := make([]*model.HistoryStatsItem, 0, len(sessions))
	for _, se := range sessions {
		flowOutCount, err := g.DB().Model("group_match_order").Ctx(ctx).
			Where("session_id = ? AND status = ?", se.ID, consts.GroupMatchOrderStatusFlowOut).
			Count()
		if err != nil {
			return nil, gerror.Wrap(err, "query session flow out count failed")
		}

		totalOrders := (se.TotalOrders + fakeMap[se.ID]) / 10 * 10
		loserCount := totalOrders / 10
		winnerCount := totalOrders - loserCount
		list = append(list, &model.HistoryStatsItem{
			SessionID:    se.ID,
			SessionDate:  se.SessionDate.Format("2006-01-02"),
			SessionName:  se.SessionName,
			StartTime:    se.StartTime.Format("2006-01-02 15:04:05"),
			EndTime:      se.EndTime.Format("2006-01-02 15:04:05"),
			TotalOrders:  totalOrders,
			WinnerCount:  winnerCount,
			LoserCount:   loserCount,
			FlowOutCount: flowOutCount,
		})
	}

	return &model.GetHistoryStatsRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (s *groupMatchService) GetLatestJoins(ctx context.Context, req *model.GetLatestJoinsReq) (*model.GetLatestJoinsRes, error) {
	type sessionRow struct {
		ID          int64     `json:"id"`
		SessionName string    `json:"session_name"`
		StartTime   time.Time `json:"start_time"`
		EndTime     time.Time `json:"end_time"`
		Status      int       `json:"status"`
	}
	var se sessionRow
	m := g.DB().Model("group_match_session").Ctx(ctx).
		Where("status = ?", consts.GroupMatchSessionStatusActive).
		Where("start_time <= NOW()").
		Where("end_time > NOW()").
		OrderAsc("start_time")
	if err := m.Limit(1).Scan(&se); err != nil && !strings.Contains(err.Error(), "no rows in result set") {
		return nil, gerror.Wrap(err, "query current session failed")
	}
	if se.ID == 0 {
		return &model.GetLatestJoinsRes{List: make([]*model.LatestJoinItem, 0)}, nil
	}

	type orderRow struct {
		UserID        int64           `json:"user_id"`
		WalletAddress string          `json:"wallet_address"`
		Amount        decimal.Decimal `json:"amount"`
		TicketSymbol  string          `json:"ticket_symbol"`
		TicketAmount  decimal.Decimal `json:"ticket_amount"`
		CreatedAt     time.Time       `json:"created_at"`
		Count         int             `json:"count"`
	}
	var rows []*orderRow
	if err := g.DB().Ctx(ctx).Raw(`
		WITH latest AS (
			SELECT DISTINCT ON (o.user_id)
				o.id,
				o.user_id,
				o.ticket_symbol,
				o.created_at
			FROM group_match_order o
			WHERE o.session_id = ?
			ORDER BY o.user_id, o.id DESC
		), agg AS (
			SELECT
				user_id,
				COUNT(*) AS user_order_count,
				COALESCE(SUM(amount), 0) AS total_amount,
				COALESCE(SUM(ticket_amount), 0) AS total_ticket_amount
			FROM group_match_order
			WHERE session_id = ?
			GROUP BY user_id
		)
		SELECT
			l.user_id,
			COALESCE(u.wallet_address, '') AS wallet_address,
			a.total_amount AS amount,
			l.ticket_symbol,
			a.total_ticket_amount AS ticket_amount,
			l.created_at,
			COALESCE(a.user_order_count, 0) AS count
		FROM latest l
		JOIN agg a ON a.user_id = l.user_id
		LEFT JOIN user_info u ON u.id = l.user_id
		ORDER BY l.id DESC
		LIMIT 10
	`, se.ID, se.ID).Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "query latest join records failed")
	}

	list := make([]*model.LatestJoinItem, 0, len(rows))
	for _, r := range rows {
		addr := r.WalletAddress
		if len(addr) > 6 {
			addr = addr[len(addr)-6:]
		}
		list = append(list, &model.LatestJoinItem{
			UserID:       r.UserID,
			Address:      addr,
			Amount:       r.Amount.String(),
			TicketSymbol: r.TicketSymbol,
			TicketAmount: r.TicketAmount.String(),
			Count:        r.Count,
			CreatedAt:    r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &model.GetLatestJoinsRes{SessionID: se.ID, SessionName: se.SessionName, List: list}, nil
}

func (s *groupMatchService) GetTopJoiners(ctx context.Context, req *model.GetTopJoinersReq) (*model.GetTopJoinersRes, error) {
	top := 10
	if req != nil && req.Top > 0 {
		top = req.Top
	}
	if top > 100 {
		top = 100
	}

	bizDate := dateOnlyInChina(time.Now()).AddDate(0, 0, -1)
	windowStart, windowEnd, err := shared.ResolveSessionDayWindow(ctx, bizDate)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve group match business window failed")
	}
	if windowStart.IsZero() || windowEnd.IsZero() || !windowEnd.After(windowStart) {
		windowStart = bizDate
		windowEnd = bizDate.AddDate(0, 0, 1)
	}

	type rankRow struct {
		Wallet string `json:"wallet"`
		Count  int    `json:"count"`
	}
	var rows []*rankRow
	if err = g.DB().Ctx(ctx).Raw(`
		WITH base_users AS (
			SELECT u.id, u.wallet_address, u.invite_code, u.parent_invite_code
			FROM user_info u
		),
		direct_team_orders AS (
			SELECT r.id AS root_user_id, r.wallet_address, COUNT(*) AS cnt
			FROM base_users r
			JOIN base_users c ON c.parent_invite_code = r.invite_code
			JOIN group_match_order o ON o.user_id = c.id AND o.status <> ?
			JOIN group_match_session s ON s.id = o.session_id
			WHERE s.start_time >= ? AND s.start_time < ?
			GROUP BY r.id, r.wallet_address
		)
		SELECT
			COALESCE(wallet_address, '') AS wallet,
			cnt AS count
		FROM direct_team_orders
		ORDER BY count DESC, root_user_id ASC
		LIMIT ?
	`, consts.GroupMatchOrderStatusFlowOut, windowStart, windowEnd, top).Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "query top joiners failed")
	}

	list := make([]*model.TopJoinerItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, &model.TopJoinerItem{
			Wallet: r.Wallet,
			Count:  r.Count,
		})
	}
	return &model.GetTopJoinersRes{BizDate: bizDate.Format("2006-01-02"), List: list}, nil
}

func (s *groupMatchService) GetSessionDetail(ctx context.Context, req *model.GetSessionDetailReq) (*model.GetSessionDetailRes, error) {
	if req == nil || req.SessionID <= 0 {
		return nil, gerror.New("invalid session_id parameter")
	}

	type sessionRow struct {
		ID                 int64     `json:"id"`
		SessionDate        time.Time `json:"session_date"`
		SessionName        string    `json:"session_name"`
		StartTime          time.Time `json:"start_time"`
		EndTime            time.Time `json:"end_time"`
		Status             int       `json:"status"`
		TotalOrders        int       `json:"total_orders"`
		TotalGroups        int       `json:"total_groups"`
		TotalFlowUserCount int       `json:"total_flow_user_count"`
	}
	var session sessionRow
	if err := g.DB().Model("group_match_session").Ctx(ctx).Where("id", req.SessionID).Limit(1).Scan(&session); err != nil {
		return nil, gerror.Wrap(err, "query session detail failed")
	}
	if session.ID == 0 {
		return nil, gerror.New("session not found")
	}

	type userRow struct {
		UserID        int64           `json:"user_id"`
		WalletAddress string          `json:"wallet_address"`
		OrderCount    int             `json:"order_count"`
		GroupCount    int             `json:"group_count"`
		TotalAmount   decimal.Decimal `json:"total_amount"`
		TotalReward   decimal.Decimal `json:"total_reward"`
		TotalRefund   decimal.Decimal `json:"total_refund"`
		WinCount      int             `json:"win_count"`
		LoseCount     int             `json:"lose_count"`
	}
	var rows []*userRow
	if err := g.DB().Ctx(ctx).Raw(`
		SELECT
			o.user_id,
			u.wallet_address,
			COUNT(*) AS order_count,
			COUNT(DISTINCT o.group_id) AS group_count,
			COALESCE(SUM(o.amount), 0) AS total_amount,
			COALESCE(SUM(o.reward_amount), 0) AS total_reward,
			COALESCE(SUM(o.refund_amount), 0) AS total_refund,
			COALESCE(SUM(CASE WHEN o.is_winner = true THEN 1 ELSE 0 END), 0) AS win_count,
			COALESCE(SUM(CASE WHEN o.is_winner = false AND o.status IN (?, ?) THEN 1 ELSE 0 END), 0) AS lose_count
		FROM group_match_order o
		LEFT JOIN user_info u ON u.id = o.user_id
		WHERE o.session_id = ?
		GROUP BY o.user_id, u.wallet_address
		ORDER BY total_amount DESC
	`, consts.GroupMatchOrderStatusLoser, consts.GroupMatchOrderStatusFlowOut, req.SessionID).Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "query user aggregated data failed")
	}

	users := make([]*model.UserSessionItem, 0, len(rows))
	for _, r := range rows {
		users = append(users, &model.UserSessionItem{
			UserID:        r.UserID,
			WalletAddress: strings.ToLower(strings.TrimSpace(r.WalletAddress)),
			OrderCount:    r.OrderCount,
			GroupCount:    r.GroupCount,
			TotalAmount:   r.TotalAmount.String(),
			TotalReward:   r.TotalReward.String(),
			TotalRefund:   r.TotalRefund.String(),
			WinCount:      r.WinCount,
			LoseCount:     r.LoseCount,
		})
	}

	return &model.GetSessionDetailRes{
		Session: &model.SessionItem{
			SessionID:          session.ID,
			SessionDate:        session.SessionDate.Format("2006-01-02"),
			SessionName:        session.SessionName,
			StartTime:          session.StartTime.Format("2006-01-02 15:04:05"),
			EndTime:            session.EndTime.Format("2006-01-02 15:04:05"),
			Status:             session.Status,
			TotalOrders:        session.TotalOrders,
			TotalGroups:        session.TotalGroups,
			TotalFlowUserCount: session.TotalFlowUserCount,
		},
		Users: users,
	}, nil
}

func (s *groupMatchService) GetSessionLoserDetail(ctx context.Context, req *model.GetSessionLoserDetailReq) (*model.GetSessionLoserDetailRes, error) {
	if req == nil || req.SessionID <= 0 {
		return nil, gerror.New("invalid session_id parameter")
	}

	count, err := g.DB().Model("group_match_session").Ctx(ctx).Where("id", req.SessionID).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "query session failed")
	}
	if count == 0 {
		return nil, gerror.New("session not found")
	}

	type loserRow struct {
		WalletAddress string    `json:"wallet_address"`
		LoseCount     int       `json:"lose_count"`
		CreatedAt     time.Time `json:"created_at"`
	}
	var rows []*loserRow
	if err := g.DB().Ctx(ctx).Raw(`
		SELECT
			COALESCE(u.wallet_address, '') AS wallet_address,
			COUNT(*) AS lose_count,
			MAX(o.created_at) AS created_at
		FROM group_match_order o
		LEFT JOIN user_info u ON u.id = o.user_id
		WHERE o.session_id = ? AND o.status = ?
		GROUP BY o.user_id, u.wallet_address
		ORDER BY lose_count DESC
	`, req.SessionID, consts.GroupMatchOrderStatusLoser).Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "query loser data failed")
	}

	list := make([]*model.LoserDetailItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, &model.LoserDetailItem{
			LoserAddress: strings.ToLower(strings.TrimSpace(r.WalletAddress)),
			LoseCount:    r.LoseCount,
			CreatedAt:    r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &model.GetSessionLoserDetailRes{List: list}, nil
}

func (s *groupMatchService) GetMyOrders(ctx context.Context, req *model.GetMyOrdersReq) (*model.GetMyOrdersRes, error) {
	if req == nil || req.UserID <= 0 {
		return nil, gerror.New("invalid parameter")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	m := g.DB().Model("group_match_order o").Ctx(ctx).
		LeftJoin("group_match_session s", "s.id = o.session_id").
		Where("o.user_id", req.UserID).
		Where("s.status IN (?)", g.Slice{
			consts.GroupMatchSessionStatusActive,
			consts.GroupMatchSessionStatusSettling,
			consts.GroupMatchSessionStatusCompleted,
		})
	if req.SessionID > 0 {
		m = m.Where("o.session_id", req.SessionID)
	}
	if req.Status > 0 {
		m = m.Where("o.status", req.Status)
	}

	total, err := m.Clone().Fields("COUNT(DISTINCT session_id)").Value()
	if err != nil {
		return nil, gerror.Wrap(err, "query session total count failed")
	}
	totalSessions := total.Int()

	type row struct {
		ID            int64           `json:"id"`
		SessionID     int64           `json:"session_id"`
		SessionStatus int             `json:"session_status"`
		OrderNo       string          `json:"order_no"`
		Amount        decimal.Decimal `json:"amount"`
		TicketSymbol  string          `json:"ticket_symbol"`
		TicketAmount  decimal.Decimal `json:"ticket_amount"`
		TicketUSDTVal decimal.Decimal `json:"ticket_usdt_value"`
		Status        int             `json:"status"`
		GroupID       int64           `json:"group_id"`
		IsWinner      *bool           `json:"is_winner"`
		RewardAmount  decimal.Decimal `json:"reward_amount"`
		RefundAmount  decimal.Decimal `json:"refund_amount"`
		CreatedAt     time.Time       `json:"created_at"`
	}
	var rows []*row
	if err := m.Clone().Fields("o.*, s.status AS session_status").OrderDesc("o.id").Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "query orders failed")
	}

	type sessionAgg struct {
		ID            int64
		SessionID     int64
		SessionStatus int
		OrderNo       string
		Amount        decimal.Decimal
		TicketSymbol  string
		TicketAmount  decimal.Decimal
		TicketUSDTVal decimal.Decimal
		TotalOrderCnt int
		Status        int
		GroupID       int64
		RewardAmount  decimal.Decimal
		RefundAmount  decimal.Decimal
		CreatedAt     time.Time
		WinnerCount   int
		LoserCount    int
	}

	aggMap := make(map[int64]*sessionAgg, len(rows))
	orderSessionIDs := make([]int64, 0, len(rows))
	for _, r := range rows {
		agg, ok := aggMap[r.SessionID]
		if !ok {
			agg = &sessionAgg{
				ID:            r.ID,
				SessionID:     r.SessionID,
				SessionStatus: r.SessionStatus,
				OrderNo:       r.OrderNo,
				Amount:        decimal.Zero,
				TicketSymbol:  r.TicketSymbol,
				TicketAmount:  decimal.Zero,
				TicketUSDTVal: r.TicketUSDTVal,
				Status:        r.Status,
				GroupID:       r.GroupID,
				RewardAmount:  decimal.Zero,
				RefundAmount:  decimal.Zero,
				CreatedAt:     r.CreatedAt,
			}
			aggMap[r.SessionID] = agg
			orderSessionIDs = append(orderSessionIDs, r.SessionID)
		}

		if r.ID > agg.ID {
			agg.ID = r.ID
			agg.OrderNo = r.OrderNo
			agg.Status = r.Status
			agg.GroupID = r.GroupID
			agg.CreatedAt = r.CreatedAt
			if r.TicketUSDTVal.GreaterThan(decimal.Zero) {
				agg.TicketUSDTVal = r.TicketUSDTVal
			}
		}

		agg.Amount = agg.Amount.Add(r.Amount)
		agg.TicketAmount = agg.TicketAmount.Add(r.TicketAmount)
		agg.RewardAmount = agg.RewardAmount.Add(r.RewardAmount)
		agg.RefundAmount = agg.RefundAmount.Add(r.RefundAmount)
		agg.TotalOrderCnt++

		if (r.IsWinner != nil && *r.IsWinner) || r.Status == consts.GroupMatchOrderStatusWinner {
			agg.WinnerCount++
		} else if (r.IsWinner != nil && !*r.IsWinner) || r.Status == consts.GroupMatchOrderStatusLoser {
			agg.LoserCount++
		}
	}

	sessionIDs := make([]int64, 0, len(orderSessionIDs))
	seen := make(map[int64]struct{}, len(orderSessionIDs))
	for _, sid := range orderSessionIDs {
		if _, ok := seen[sid]; ok {
			continue
		}
		seen[sid] = struct{}{}
		sessionIDs = append(sessionIDs, sid)
	}

	start := (req.Page - 1) * req.PageSize
	if start >= len(sessionIDs) {
		return &model.GetMyOrdersRes{List: []*model.MyOrderItem{}, Total: totalSessions, Page: req.Page, PageSize: req.PageSize}, nil
	}
	end := start + req.PageSize
	if end > len(sessionIDs) {
		end = len(sessionIDs)
	}

	list := make([]*model.MyOrderItem, 0, end-start)
	for _, sid := range sessionIDs[start:end] {
		agg := aggMap[sid]
		if agg == nil {
			continue
		}
		list = append(list, &model.MyOrderItem{
			SessionID:      agg.SessionID,
			Status:         agg.SessionStatus,
			TotalOrderCnt:  agg.TotalOrderCnt,
			WinnerOrderCnt: agg.WinnerCount,
		})
	}

	return &model.GetMyOrdersRes{List: list, Total: totalSessions, Page: req.Page, PageSize: req.PageSize}, nil
}

func (s *groupMatchService) resolveJoinSession(ctx context.Context, sessionID int64, locale string) (int64, error) {
	type sessionRow struct {
		ID        int64     `json:"id"`
		Status    int       `json:"status"`
		StartTime time.Time `json:"start_time"`
		EndTime   time.Time `json:"end_time"`
	}
	var row sessionRow
	m := g.DB().Model("group_match_session").Ctx(ctx).
		Where("status = ?", consts.GroupMatchSessionStatusActive).
		Where("start_time <= NOW()").
		Where("end_time > NOW()")
	if sessionID > 0 {
		m = m.Where("id", sessionID)
	} else {
		m = m.OrderAsc("start_time")
	}
	if err := m.Limit(1).Scan(&row); err != nil && !strings.Contains(err.Error(), "no rows in result set") {
		return 0, gerror.Wrap(err, "query session failed")
	}
	if row.ID == 0 {
		if sessionID <= 0 {
			return 0, joinErr(locale, "no_active_session")
		}
		// 传了 session_id 但查不到，需要细化原因
		var exists sessionRow
		if err := g.DB().Model("group_match_session").Ctx(ctx).Where("id", sessionID).Limit(1).Scan(&exists); err != nil && !strings.Contains(err.Error(), "no rows in result set") {
			return 0, gerror.Wrap(err, "query session failed")
		}
		if exists.ID == 0 {
			return 0, joinErr(locale, "session_not_found")
		}
		if exists.Status != consts.GroupMatchSessionStatusActive {
			return 0, joinErr(locale, "session_not_active")
		}
		now := time.Now()
		if now.Before(exists.StartTime) {
			return 0, joinErr(locale, "session_not_started")
		}
		if !now.Before(exists.EndTime) {
			return 0, joinErr(locale, "session_ended")
		}
		return 0, joinErr(locale, "session_unavailable")
	}
	return row.ID, nil
}

func (s *groupMatchService) getJoinGroupByRequestID(ctx context.Context, userID, sessionID int64, requestID string) (*model.JoinGroupRes, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil, nil
	}

	type row struct {
		OrderNo       string          `json:"order_no"`
		Amount        decimal.Decimal `json:"amount"`
		TicketSymbol  string          `json:"ticket_symbol"`
		TicketAmount  decimal.Decimal `json:"ticket_amount"`
		TicketUSDTVal decimal.Decimal `json:"ticket_usdt_value"`
		Status        int             `json:"status"`
	}
	var rows []*row
	if err := g.DB().Model("group_match_order").Ctx(ctx).
		Where("user_id = ? AND session_id = ? AND request_id = ? AND request_id <> ''", userID, sessionID, requestID).
		OrderAsc("id").
		Scan(&rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	ticketSymbol := rows[0].TicketSymbol
	totalUSDT := decimal.Zero
	totalTicket := decimal.Zero
	ticketUSDTVal := decimal.Zero
	orders := make([]*model.JoinOrderItem, 0, len(rows))
	for _, r := range rows {
		totalUSDT = totalUSDT.Add(r.Amount)
		totalTicket = totalTicket.Add(r.TicketAmount)
		ticketUSDTVal = r.TicketUSDTVal
		orders = append(orders, &model.JoinOrderItem{OrderNo: r.OrderNo, Amount: r.Amount.String(), TicketSymbol: r.TicketSymbol, TicketAmount: r.TicketAmount.String(), Status: r.Status})
	}
	ticketPrice := decimal.Zero
	if rows[0].TicketAmount.GreaterThan(decimal.Zero) {
		ticketPrice = rows[0].TicketUSDTVal.Div(rows[0].TicketAmount)
	}
	if !ticketPrice.GreaterThan(decimal.Zero) {
		price, err := s.getTicketPrice(ctx, ticketSymbol)
		if err != nil {
			return nil, err
		}
		ticketPrice = price
	}
	usdtBal, err := s.balanceRepo.GetOrCreate(ctx, userID, "USDT")
	if err != nil {
		return nil, err
	}
	ticketBal, err := s.balanceRepo.GetOrCreate(ctx, userID, ticketSymbol)
	if err != nil {
		return nil, err
	}
	return &model.JoinGroupRes{SessionID: sessionID, Orders: orders, TotalUSDT: totalUSDT.String(), TotalTicket: totalTicket.String(), UsdtLeft: usdtBal.AvailableAmount.String(), TicketLeft: ticketBal.AvailableAmount.String(), TicketSymbol: ticketSymbol, TicketPrice: ticketPrice.String(), TicketUSDTVal: ticketUSDTVal.String()}, nil
}

func (s *groupMatchService) getTicketPrice(ctx context.Context, symbol string) (decimal.Decimal, error) {
	price, err := s.stockPriceDao.GetLatestBySymbol(ctx, symbol)
	if err != nil {
		return decimal.Zero, gerror.Wrap(err, "query ticket price failed")
	}
	if price == nil || !price.Price.GreaterThan(decimal.Zero) {
		return decimal.Zero, gerror.New("ticket price unavailable, please try again later")
	}
	return price.Price, nil
}

func (s *groupMatchService) createCoboBalanceChangeLogTx(ctx context.Context, tx gdb.TX, userID int64, symbol, changeType string, amount, beforeBalance, afterBalance decimal.Decimal, relatedOrderNo string, relatedID int64) error {
	_, err := tx.Exec(`INSERT INTO cobo_balance_change_log (user_id, symbol, change_type, amount, before_balance, after_balance, related_order_no, related_id, remark, operator_type, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, userID, symbol, changeType, amount, beforeBalance, afterBalance, relatedOrderNo, relatedID, "group_match join", consts.OperatorTypeUser, time.Now())
	return err
}

func (s *groupMatchService) mustReadDecimalConfig(ctx context.Context, key, fallback string) decimal.Decimal {
	val := s.mustReadStringConfig(ctx, key, fallback)
	d, err := decimal.NewFromString(val)
	if err != nil {
		d, _ = decimal.NewFromString(fallback)
	}
	return d
}

func (s *groupMatchService) mustReadIntConfig(ctx context.Context, key string, fallback int) int {
	val := s.mustReadStringConfig(ctx, key, fmt.Sprintf("%d", fallback))
	d, err := decimal.NewFromString(val)
	if err != nil {
		return fallback
	}
	return int(d.IntPart())
}

func (s *groupMatchService) mustReadBoolConfig(ctx context.Context, key string, fallback bool) bool {
	fallbackStr := "false"
	if fallback {
		fallbackStr = "true"
	}
	val := strings.ToLower(strings.TrimSpace(s.mustReadStringConfig(ctx, key, fallbackStr)))
	switch val {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func (s *groupMatchService) mustReadStringConfig(ctx context.Context, key, fallback string) string {
	cfg, err := repository.NewConfigRepository().GetByKeyName(ctx, key)
	if err == nil && cfg != nil && strings.TrimSpace(cfg.KeyValue) != "" {
		return strings.TrimSpace(cfg.KeyValue)
	}
	return fallback
}

func containsTicketToken(tokens []*entity.TokenConfigEntity, symbol string) bool {
	for _, t := range tokens {
		if strings.ToUpper(t.Symbol) == symbol {
			return true
		}
	}
	return false
}

func (s *groupMatchService) GetSupportedTicketSymbols(ctx context.Context) []string {
	ticketTokens, err := s.tokenConfigRepo.GetTicketTokens(ctx)
	if err != nil {
		g.Log().Warningf(ctx, "query ticket tokens failed: %v", err)
		return nil
	}
	list := make([]string, 0, len(ticketTokens))
	for _, t := range ticketTokens {
		if t.Symbol == "" {
			continue
		}
		list = append(list, strings.ToUpper(t.Symbol))
	}
	return list
}

type sessionTimeConfig struct {
	Sessions []struct {
		Name  string `json:"name"`
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"sessions"`
}

func (s *groupMatchService) CreateDailySessions(ctx context.Context) (int64, error) {
	cfgVal := s.mustReadStringConfig(ctx, "group_match_session_times", "")
	if cfgVal == "" {
		return 0, gerror.New("group_match_session_times config not found")
	}

	var cfg sessionTimeConfig
	if err := gjson.DecodeTo(cfgVal, &cfg); err != nil {
		return 0, gerror.Wrap(err, "parse group_match_session_times config failed")
	}
	if len(cfg.Sessions) == 0 {
		return 0, gerror.New("group_match_session_times config is empty")
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	var created int64

	for _, se := range cfg.Sessions {
		startTime, err := time.ParseInLocation("2006-01-02 15:04", today.Format("2006-01-02")+" "+se.Start, time.Local)
		if err != nil {
			return created, gerror.Wrapf(err, "parse start time failed: %s", se.Start)
		}

		endTime, err := time.ParseInLocation("2006-01-02 15:04", today.Format("2006-01-02")+" "+se.End, time.Local)
		if err != nil {
			return created, gerror.Wrapf(err, "parse end time failed: %s", se.End)
		}
		if !endTime.After(startTime) {
			endTime = endTime.Add(24 * time.Hour)
		}

		_, err = g.DB().Exec(ctx, `
			INSERT INTO group_match_session (session_date, session_name, start_time, end_time, status, total_orders, total_groups, total_flow_user_count, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 0, 0, 0, ?, ?)
			ON CONFLICT (session_date, session_name) DO NOTHING
		`, today, se.Name, startTime, endTime, consts.GroupMatchSessionStatusPending, time.Now(), time.Now())
		if err != nil {
			return created, gerror.Wrapf(err, "insert session failed: %s", se.Name)
		}
		created++
	}

	return created, nil
}

func (s *groupMatchService) GetParticipationStats(ctx context.Context, req *model.GetParticipationStatsReq) (*model.GetParticipationStatsRes, error) {
	if req == nil || req.UserID <= 0 {
		return nil, gerror.New("invalid parameter")
	}

	inviteCodeVal, err := g.DB().Ctx(ctx).Raw("SELECT invite_code FROM user_info WHERE id = ?", req.UserID).Value()
	if err != nil {
		return nil, gerror.Wrap(err, "query user invite_code failed")
	}
	inviteCode := inviteCodeVal.String()

	myCount, err := g.DB().Model("group_match_order").Ctx(ctx).Where("user_id", req.UserID).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "query my count failed")
	}

	res := &model.GetParticipationStatsRes{MyCount: int64(myCount)}
	if inviteCode == "" {
		return res, nil
	}

	teamCountVal, err := g.DB().Ctx(ctx).Raw(`
		WITH RECURSIVE downline AS (
			SELECT u.id, u.invite_code FROM user_info u WHERE u.parent_invite_code = ?
			UNION ALL
			SELECT c.id, c.invite_code FROM user_info c INNER JOIN downline d ON c.parent_invite_code = d.invite_code
		)
		SELECT COUNT(*) FROM group_match_order o INNER JOIN downline d ON o.user_id = d.id
	`, inviteCode).Value()
	if err != nil {
		return nil, gerror.Wrap(err, "query team count failed")
	}
	res.TeamCount = teamCountVal.Int64()
	return res, nil
}

func (s *groupMatchService) GetCurrentParticipationStats(ctx context.Context, req *model.GetCurrentParticipationStatsReq) (*model.GetCurrentParticipationStatsRes, error) {
	if req == nil || req.UserID <= 0 {
		return nil, gerror.New("invalid parameter")
	}

	type sessionRow struct {
		ID          int64  `json:"id"`
		SessionName string `json:"session_name"`
	}

	var session sessionRow
	var err error
	if req.SessionID > 0 {
		err = g.DB().Model("group_match_session").Ctx(ctx).
			Where("id", req.SessionID).
			Limit(1).
			Scan(&session)
		if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
			return nil, gerror.Wrap(err, "query target session failed")
		}
	} else {
		err = g.DB().Model("group_match_session").Ctx(ctx).
			Where("status = ?", consts.GroupMatchSessionStatusActive).
			Where("start_time <= NOW()").
			Where("end_time > NOW()").
			OrderAsc("start_time").
			Limit(1).
			Scan(&session)
		if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
			return nil, gerror.Wrap(err, "query current session failed")
		}
		if session.ID == 0 {
			err = g.DB().Model("group_match_session").Ctx(ctx).
				Where("start_time <= NOW()").
				OrderDesc("start_time").
				Limit(1).
				Scan(&session)
			if err != nil && !strings.Contains(err.Error(), "no rows in result set") {
				return nil, gerror.Wrap(err, "query recent session failed")
			}
		}
	}

	res := &model.GetCurrentParticipationStatsRes{}
	if session.ID == 0 {
		return res, nil
	}

	res.SessionType = normalizeSessionType(session.SessionName)

	totalOrderCount, err := g.DB().Model("group_match_order").Ctx(ctx).Where("session_id", session.ID).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "query current session total order count failed")
	}

	fakeOrderCountVal, err := g.DB().Model("fake_group_order").Ctx(ctx).
		Fields("COALESCE(SUM(new_order), 0)").
		Where("session_id", session.ID).
		Value()
	if err != nil {
		return nil, gerror.Wrap(err, "query current session fake order count failed")
	}
	res.TotalOrderCount = int64(totalOrderCount) + fakeOrderCountVal.Int64()

	inviteCodeVal, err := g.DB().Ctx(ctx).Raw("SELECT invite_code FROM user_info WHERE id = ?", req.UserID).Value()
	if err != nil {
		return nil, gerror.Wrap(err, "query user invite_code failed")
	}
	inviteCode := inviteCodeVal.String()
	if inviteCode == "" {
		return res, nil
	}

	teamOrderCountVal, err := g.DB().Ctx(ctx).Raw(`
		WITH RECURSIVE downline AS (
			SELECT u.id, u.invite_code FROM user_info u WHERE u.parent_invite_code = ?
			UNION ALL
			SELECT c.id, c.invite_code FROM user_info c INNER JOIN downline d ON c.parent_invite_code = d.invite_code
		)
		SELECT COUNT(*)
		FROM group_match_order o
		INNER JOIN downline d ON o.user_id = d.id
		WHERE o.session_id = ?
	`, inviteCode, session.ID).Value()
	if err != nil {
		return nil, gerror.Wrap(err, "query current session team order count failed")
	}
	res.TeamOrderCount = teamOrderCountVal.Int64()
	return res, nil
}

func (s *groupMatchService) GetBalanceChanges(ctx context.Context, req *model.GetBalanceChangesReq) (*model.GetBalanceChangesRes, error) {
	if req == nil || req.UserID <= 0 {
		return nil, gerror.New("invalid parameter")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	bizType := strings.ToLower(strings.TrimSpace(req.BizType))
	switch bizType {
	case "winner":
		return s.getBalanceChangesWinner(ctx, req)
	case "loser_release":
		return s.getBalanceChangesLoserRelease(ctx, req)
	case "internal":
		return s.getBalanceChangesInternal(ctx, req)
	default:
		return s.getBalanceChangesAll(ctx, req)
	}
}

func (s *groupMatchService) getBalanceChangesAll(ctx context.Context, req *model.GetBalanceChangesReq) (*model.GetBalanceChangesRes, error) {
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	m := g.DB().Model("cobo_balance_change_log").Ctx(ctx).Where("user_id", req.UserID).
		WhereNotIn("change_type", []string{
			consts.ChangeTypeWithdrawFreeze,
			consts.ChangeTypeWithdrawUnfreeze,
			consts.ChangeTypeExchangeFreeze,
			consts.ChangeTypeExchangeUnfreeze,
		})
	if symbol != "" {
		m = m.Where("symbol", symbol)
	}

	total, err := m.Count()
	if err != nil {
		return nil, gerror.Wrap(err, "query balance change total count failed")
	}

	type row struct {
		ID             int64           `json:"id"`
		Symbol         string          `json:"symbol"`
		ChangeType     string          `json:"change_type"`
		Amount         decimal.Decimal `json:"amount"`
		BeforeBalance  decimal.Decimal `json:"before_balance"`
		AfterBalance   decimal.Decimal `json:"after_balance"`
		RelatedOrderNo string          `json:"related_order_no"`
		RelatedID      int64           `json:"related_id"`
		Remark         string          `json:"remark"`
		CreatedAt      time.Time       `json:"created_at"`
	}

	rows := make([]*row, 0)
	err = m.OrderDesc("created_at").OrderDesc("id").Page(req.Page, req.PageSize).Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "query balance changes failed")
	}

	list := make([]*model.BalanceChangeItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, &model.BalanceChangeItem{
			ID:             r.ID,
			Symbol:         r.Symbol,
			ChangeType:     consts.BalanceChangeTextWithSymbol(r.ChangeType, req.Locale, r.Symbol),
			Amount:         r.Amount.String(),
			BeforeBalance:  r.BeforeBalance.String(),
			AfterBalance:   r.AfterBalance.String(),
			RelatedOrderNo: r.RelatedOrderNo,
			RelatedID:      r.RelatedID,
			Remark:         r.Remark,
			CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &model.GetBalanceChangesRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (s *groupMatchService) getBalanceChangesWinner(ctx context.Context, req *model.GetBalanceChangesReq) (*model.GetBalanceChangesRes, error) {
	totalVal, err := g.DB().Ctx(ctx).Raw(`
		SELECT COUNT(*) FROM group_match_order
		WHERE user_id = ? AND status = ?
	`, req.UserID, consts.GroupMatchOrderStatusWinner).Value()
	if err != nil {
		return nil, gerror.Wrap(err, "query winner total count failed")
	}
	total := totalVal.Int()

	type row struct {
		ID             int64           `json:"id"`
		Symbol         string          `json:"symbol"`
		ChangeType     string          `json:"change_type"`
		Amount         decimal.Decimal `json:"amount"`
		BeforeBalance  decimal.Decimal `json:"before_balance"`
		AfterBalance   decimal.Decimal `json:"after_balance"`
		RelatedOrderNo string          `json:"related_order_no"`
		RelatedID      int64           `json:"related_id"`
		Remark         string          `json:"remark"`
		CreatedAt      time.Time       `json:"created_at"`
	}

	rows := make([]*row, 0)
	err = g.DB().Ctx(ctx).Raw(`
		SELECT
			o.id,
			'USDT' AS symbol,
			? AS change_type,
			o.reward_amount AS amount,
			COALESCE(l.before_balance, 0) AS before_balance,
			COALESCE(l.after_balance, 0) AS after_balance,
			o.order_no AS related_order_no,
			o.session_id AS related_id,
			'group_match winner settlement' AS remark,
			o.updated_at AS created_at
		FROM group_match_order o
		LEFT JOIN cobo_balance_change_log l ON l.related_order_no = o.order_no AND l.change_type = ?
		WHERE o.user_id = ? AND o.status = ?
		ORDER BY o.updated_at DESC, o.id DESC
		LIMIT ? OFFSET ?
	`, consts.ChangeTypeGroupMatchSettleWinner, groupMatchChangeTypeWinnerPayout, req.UserID, consts.GroupMatchOrderStatusWinner, req.PageSize, (req.Page-1)*req.PageSize).Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "query winner balance changes failed")
	}

	list := make([]*model.BalanceChangeItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, &model.BalanceChangeItem{
			ID:             r.ID,
			Symbol:         r.Symbol,
			ChangeType:     consts.BalanceChangeTextWithSymbol(r.ChangeType, req.Locale, r.Symbol),
			Amount:         r.Amount.String(),
			BeforeBalance:  r.BeforeBalance.String(),
			AfterBalance:   r.AfterBalance.String(),
			RelatedOrderNo: r.RelatedOrderNo,
			RelatedID:      r.RelatedID,
			Remark:         r.Remark,
			CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &model.GetBalanceChangesRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (s *groupMatchService) getBalanceChangesLoserRelease(ctx context.Context, req *model.GetBalanceChangesReq) (*model.GetBalanceChangesRes, error) {
	totalVal, err := g.DB().Ctx(ctx).Raw(`
		SELECT COUNT(*) FROM group_match_loser_comp_release_log
		WHERE user_id = ?
	`, req.UserID).Value()
	if err != nil {
		return nil, gerror.Wrap(err, "query loser release total count failed")
	}
	total := totalVal.Int()

	type row struct {
		ID             int64           `json:"id"`
		Symbol         string          `json:"symbol"`
		ChangeType     string          `json:"change_type"`
		Amount         decimal.Decimal `json:"amount"`
		BeforeBalance  decimal.Decimal `json:"before_balance"`
		AfterBalance   decimal.Decimal `json:"after_balance"`
		RelatedOrderNo string          `json:"related_order_no"`
		RelatedID      int64           `json:"related_id"`
		Remark         string          `json:"remark"`
		CreatedAt      time.Time       `json:"created_at"`
	}

	rows := make([]*row, 0)
	err = g.DB().Ctx(ctx).Raw(`
		SELECT
			rl.id,
			'YY' AS symbol,
			'group_match_loser_comp_release' AS change_type,
			rl.release_amount AS amount,
			COALESCE(l.before_balance, 0) AS before_balance,
			COALESCE(l.after_balance, 0) AS after_balance,
			'COMP-' || rl.compensation_id AS related_order_no,
			c.order_id AS related_id,
			'group_match loser compensation release' AS remark,
			rl.created_at AS created_at
		FROM group_match_loser_comp_release_log rl
		JOIN group_match_loser_compensation c ON c.id = rl.compensation_id
		LEFT JOIN cobo_balance_change_log l ON l.related_order_no = ('COMP-' || rl.compensation_id) AND l.change_type = ?
		WHERE rl.user_id = ?
		ORDER BY rl.created_at DESC, rl.id DESC
		LIMIT ? OFFSET ?
	`, groupMatchChangeTypeLoserComp, req.UserID, req.PageSize, (req.Page-1)*req.PageSize).Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "query loser release balance changes failed")
	}

	list := make([]*model.BalanceChangeItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, &model.BalanceChangeItem{
			ID:             r.ID,
			Symbol:         r.Symbol,
			ChangeType:     consts.BalanceChangeTextWithSymbol(r.ChangeType, req.Locale, r.Symbol),
			Amount:         r.Amount.String(),
			BeforeBalance:  r.BeforeBalance.String(),
			AfterBalance:   r.AfterBalance.String(),
			RelatedOrderNo: r.RelatedOrderNo,
			RelatedID:      r.RelatedID,
			Remark:         r.Remark,
			CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &model.GetBalanceChangesRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func (s *groupMatchService) getBalanceChangesInternal(ctx context.Context, req *model.GetBalanceChangesReq) (*model.GetBalanceChangesRes, error) {
	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	m := g.DB().Model("cobo_balance_change_log").Ctx(ctx).Where("user_id", req.UserID).WhereIn("change_type", []string{consts.ChangeTypeTransferIn, consts.ChangeTypeTransferOut})
	if symbol != "" {
		m = m.Where("symbol", symbol)
	}

	total, err := m.Count()
	if err != nil {
		return nil, gerror.Wrap(err, "query internal transfer total count failed")
	}

	type row struct {
		ID             int64           `json:"id"`
		Symbol         string          `json:"symbol"`
		ChangeType     string          `json:"change_type"`
		Amount         decimal.Decimal `json:"amount"`
		BeforeBalance  decimal.Decimal `json:"before_balance"`
		AfterBalance   decimal.Decimal `json:"after_balance"`
		RelatedOrderNo string          `json:"related_order_no"`
		RelatedID      int64           `json:"related_id"`
		Remark         string          `json:"remark"`
		CreatedAt      time.Time       `json:"created_at"`
	}

	rows := make([]*row, 0)
	err = m.OrderDesc("created_at").OrderDesc("id").Page(req.Page, req.PageSize).Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "query internal transfer records failed")
	}

	list := make([]*model.BalanceChangeItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, &model.BalanceChangeItem{
			ID:             r.ID,
			Symbol:         r.Symbol,
			ChangeType:     consts.BalanceChangeTextWithSymbol(r.ChangeType, req.Locale, r.Symbol),
			Amount:         r.Amount.String(),
			BeforeBalance:  r.BeforeBalance.String(),
			AfterBalance:   r.AfterBalance.String(),
			RelatedOrderNo: r.RelatedOrderNo,
			RelatedID:      r.RelatedID,
			Remark:         r.Remark,
			CreatedAt:      r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &model.GetBalanceChangesRes{List: list, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func normalizeSessionType(sessionName string) string {
	name := strings.ToLower(strings.TrimSpace(sessionName))
	switch name {
	case "morning", "早场":
		return "morning"
	case "afternoon", "午场":
		return "afternoon"
	case "evening", "night", "晚场", "夜场":
		return "evening"
	default:
		return ""
	}
}

func (s *groupMatchService) GetTodayReward(ctx context.Context, userID int64, dateStr string) (*model.GetTodayRewardRes, error) {
	if userID <= 0 {
		return nil, gerror.New("invalid parameter")
	}

	queryDate := strings.TrimSpace(dateStr)
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.Local
	}
	if queryDate == "" {
		queryDate = time.Now().In(loc).Format("2006-01-02")
	} else {
		parsedDate, err := time.ParseInLocation("2006-01-02", queryDate, loc)
		if err != nil {
			return nil, gerror.New("invalid date format, expected YYYY-MM-DD")
		}
		queryDate = parsedDate.Format("2006-01-02")
	}

	dailyWinnerReward, err := s.queryDecimal(ctx, `
		SELECT COALESCE(SUM(o.reward_amount), 0) AS amount
		FROM group_match_order o
		JOIN group_match_session s ON s.id = o.session_id
		WHERE o.user_id = ? AND o.status = ?
		  AND s.session_date = ?
	`, userID, consts.GroupMatchOrderStatusWinner, queryDate)
	if err != nil {
		return nil, gerror.Wrap(err, "query winner reward failed")
	}

	dailyTeamReward, err := s.queryDecimal(ctx, `
		SELECT COALESCE(SUM(l.amount), 0) AS amount
		FROM cobo_balance_change_log l
		JOIN group_match_session s ON s.id = l.related_id
		WHERE l.user_id = ? AND l.change_type = ?
		  AND s.session_date = ?
	`, userID, groupMatchChangeTypeTeamReward, queryDate)
	if err != nil {
		return nil, gerror.Wrap(err, "query team reward failed")
	}

	dailyReward := dailyWinnerReward.Add(dailyTeamReward)

	dailyReleasedYY, err := s.queryDecimal(ctx, `
		SELECT COALESCE(SUM(release_amount), 0) AS amount
		FROM group_match_loser_comp_release_log
		WHERE user_id = ? AND release_date = ?
	`, userID, queryDate)
	if err != nil {
		return nil, gerror.Wrap(err, "query released yy failed")
	}

	yyPrice, err := s.getTicketPrice(ctx, "YY")
	if err != nil {
		yyPrice = decimal.Zero
	}
	dailyYYUSDTValue := dailyReleasedYY.Mul(yyPrice)

	totalWinnerReward, err := s.queryDecimal(ctx, `
		SELECT COALESCE(SUM(o.reward_amount), 0) AS amount
		FROM group_match_order o
		JOIN group_match_session s ON s.id = o.session_id
		WHERE o.user_id = ? AND o.status = ?
	`, userID, consts.GroupMatchOrderStatusWinner)
	if err != nil {
		return nil, gerror.Wrap(err, "query total winner reward failed")
	}

	totalTeamReward, err := s.queryDecimal(ctx, `
		SELECT COALESCE(SUM(l.amount), 0) AS amount
		FROM cobo_balance_change_log l
		JOIN group_match_session s ON s.id = l.related_id
		WHERE l.user_id = ? AND l.change_type = ?
	`, userID, groupMatchChangeTypeTeamReward)
	if err != nil {
		return nil, gerror.Wrap(err, "query total team reward failed")
	}

	totalReward := totalWinnerReward.Add(totalTeamReward)

	totalReleasedYY, err := s.queryDecimal(ctx, `
		SELECT COALESCE(SUM(release_amount), 0) AS amount
		FROM group_match_loser_comp_release_log
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, gerror.Wrap(err, "query total released yy failed")
	}
	totalYYUSDTValue := totalReleasedYY.Mul(yyPrice)

	return &model.GetTodayRewardRes{
		Date:                    queryDate,
		SettledRewardUSDT:       dailyReward.StringFixedBank(2),
		ReleaseYYUSDTValue:      dailyYYUSDTValue.StringFixedBank(2),
		TotalSettledRewardUSDT:  totalReward.StringFixedBank(2),
		TotalReleaseYYUSDTValue: totalYYUSDTValue.StringFixedBank(2),
	}, nil
}

func (s *groupMatchService) queryDecimal(ctx context.Context, sql string, args ...interface{}) (decimal.Decimal, error) {
	row, err := g.DB().GetOne(ctx, sql, args...)
	if err != nil {
		return decimal.Zero, err
	}
	if row == nil {
		return decimal.Zero, nil
	}
	v, _ := decimal.NewFromString(row["amount"].String())
	return v, nil
}

func (s *groupMatchService) GetLoserOutputStats(ctx context.Context, userID int64) (*model.GetLoserOutputStatsRes, error) {
	if userID <= 0 {
		return nil, gerror.New("invalid parameter")
	}

	yyPrice, err := s.getTicketPrice(ctx, "YY")
	if err != nil {
		yyPrice = decimal.Zero
	}

	type releasingRow struct {
		USDTValue   decimal.Decimal `json:"usdt_value"`
		ReleaseDays int             `json:"release_days"`
	}
	var releasingRows []*releasingRow
	err = g.DB().Model("group_match_loser_compensation").Ctx(ctx).
		Where("user_id = ? AND status = ?", userID, groupMatchLoserCompStatusReleasing).
		Scan(&releasingRows)
	if err != nil {
		return nil, gerror.Wrap(err, "query releasing compensations failed")
	}

	dailyUSDT := decimal.Zero
	for _, r := range releasingRows {
		if r.ReleaseDays > 0 {
			dailyUSDT = dailyUSDT.Add(r.USDTValue.Div(decimal.NewFromInt(int64(r.ReleaseDays))))
		}
	}

	pendingUSDT, err := s.queryDecimal(ctx, `
		SELECT COALESCE(SUM(pending_amount), 0) AS amount
		FROM group_match_loser_compensation
		WHERE user_id = ? AND status = ?
	`, userID, groupMatchLoserCompStatusReleasing)
	if err != nil {
		return nil, gerror.Wrap(err, "query pending amount failed")
	}

	completedRow, err := g.DB().GetOne(ctx, `
		SELECT COALESCE(SUM(release_amount), 0) AS yy_amount, COALESCE(SUM(release_usdt_value), 0) AS usdt_amount
		FROM group_match_loser_comp_release_log
		WHERE user_id = ?
	`, userID)
	if err != nil {
		return nil, gerror.Wrap(err, "query completed release log failed")
	}
	completedYY, _ := decimal.NewFromString(completedRow["yy_amount"].String())
	completedUSDT, _ := decimal.NewFromString(completedRow["usdt_amount"].String())

	dailyYY := decimal.Zero
	pendingYY := decimal.Zero
	if yyPrice.GreaterThan(decimal.Zero) {
		dailyYY = dailyUSDT.Div(yyPrice)
		pendingYY = pendingUSDT.Div(yyPrice)
	}

	return &model.GetLoserOutputStatsRes{
		DailyOutput: &model.LoserOutputItem{
			Amount:    dailyYY.Round(8).String(),
			USDTValue: dailyUSDT.Round(2).String(),
		},
		PendingOutput: &model.LoserOutputItem{
			Amount:    pendingYY.Round(8).String(),
			USDTValue: pendingUSDT.Round(2).String(),
		},
		CompletedOutput: &model.LoserOutputItem{
			Amount:    completedYY.Round(8).String(),
			USDTValue: completedUSDT.Round(2).String(),
		},
	}, nil
}

func (s *groupMatchService) GetLoserReleaseLog(ctx context.Context, req *model.GetLoserReleaseLogReq) (*model.GetLoserReleaseLogRes, error) {
	if req == nil || req.UserID <= 0 {
		return nil, gerror.New("invalid parameter")
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	totalVal, err := g.DB().Model("group_match_loser_comp_release_log").Ctx(ctx).
		Where("user_id = ?", req.UserID).Count()
	if err != nil {
		return nil, gerror.Wrap(err, "query release log total count failed")
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
	err = g.DB().Model("group_match_loser_comp_release_log").Ctx(ctx).
		Where("user_id = ?", req.UserID).
		OrderDesc("created_at").OrderDesc("id").
		Page(req.Page, req.PageSize).
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "query release log failed")
	}

	list := make([]*model.LoserReleaseLogItem, 0, len(rows))
	for _, r := range rows {
		list = append(list, &model.LoserReleaseLogItem{
			ID:               r.ID,
			CompensationID:   r.CompensationID,
			OrderID:          r.OrderID,
			ReleaseDate:      r.ReleaseDate.Format("2006-01-02"),
			ReleaseAmount:    r.ReleaseAmount.Round(8).String(),
			ReleaseUSDTValue: r.ReleaseUSDTValue.Round(2).String(),
			CreatedAt:        r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return &model.GetLoserReleaseLogRes{
		List:     list,
		Total:    totalVal,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}
