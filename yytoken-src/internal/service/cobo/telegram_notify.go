package cobo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	entityCobo "XWFrame/internal/entity/cobo"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcron"
	"github.com/shopspring/decimal"
)

type TelegramNotifyService struct {
	enabled              bool
	botToken             string
	chatID               string
	minNotifyAmount      decimal.Decimal
	client               *http.Client
	rejectedNotifyCh     chan struct{}
	insufficientNotifyCh chan struct{}
}

type telegramTopicNotify struct {
	Symbol          string
	MessageThreadID int64
}

var (
	telegramNotifyOnce sync.Once
	telegramNotifySvc  *TelegramNotifyService
	dailyNotifyOnce    sync.Once
)

func GetTelegramNotifyService(ctx context.Context) *TelegramNotifyService {
	telegramNotifyOnce.Do(func() {
		minAmount := decimal.NewFromInt(5)
		if v := g.Cfg().MustGet(ctx, "telegram.min_notify_amount"); v != nil {
			if parsed, err := decimal.NewFromString(v.String()); err == nil && parsed.GreaterThanOrEqual(decimal.Zero) {
				minAmount = parsed
			}
		}
		telegramNotifySvc = &TelegramNotifyService{
			enabled:              g.Cfg().MustGet(ctx, "telegram.enabled").Bool(),
			botToken:             g.Cfg().MustGet(ctx, "telegram.bot_token").String(),
			chatID:               g.Cfg().MustGet(ctx, "telegram.chat_id").String(),
			minNotifyAmount:      minAmount,
			client:               &http.Client{Timeout: 8 * time.Second},
			rejectedNotifyCh:     make(chan struct{}, 32),
			insufficientNotifyCh: make(chan struct{}, 32),
		}
	})
	return telegramNotifySvc
}

func StartDailyStatisticsNotifyJob(ctx context.Context) {
	dailyNotifyOnce.Do(func() {
		_, err := gcron.AddSingleton(ctx, "0 0 0 * * *", func(ctx context.Context) {
			svc := GetTelegramNotifyService(ctx)
			if !svc.IsEnabled() {
				return
			}

			loc, _ := time.LoadLocation("Asia/Shanghai")
			yesterday := time.Now().In(loc).AddDate(0, 0, -1)
			ok, err := svc.acquireDailyStatsLock(ctx, yesterday)
			if err != nil {
				g.Log().Warningf(ctx, "[TelegramNotify] 获取每日统计锁失败: %v", err)
			}
			if !ok {
				return
			}
			if err := svc.NotifyDailyDepositWithdrawStats(ctx, yesterday); err != nil {
				g.Log().Warningf(ctx, "[TelegramNotify] 发送每日统计失败: %v", err)
			}
		})
		if err != nil {
			g.Log().Warningf(ctx, "[TelegramNotify] 注册每日统计任务失败: %v", err)
		}
	})
}

func (s *TelegramNotifyService) IsEnabled() bool {
	return s != nil && s.enabled && s.botToken != "" && s.chatID != ""
}

func (s *TelegramNotifyService) shouldNotifyAmount(amount decimal.Decimal, symbol string) bool {
	minAmount := decimal.NewFromInt(1)
	return amount.GreaterThanOrEqual(minAmount)
}

func (s *TelegramNotifyService) shouldNotifyWithdrawStatus(amount decimal.Decimal, status int, symbol string) bool {
	if status == entityCobo.WithdrawStatusFailed || status == entityCobo.WithdrawStatusRejected {
		return true
	}
	return s.shouldNotifyAmount(amount, symbol)
}

func (s *TelegramNotifyService) NotifyWithdrawRequest(ctx context.Context, withdraw *entityCobo.WithdrawEntity) {
	if withdraw == nil {
		return
	}
	if !s.shouldNotifyAmount(withdraw.Amount, withdraw.Symbol) {
		return
	}
	// 尝试删除该订单之前发送的通知
	s.deletePreviousWithdrawMessage(ctx, withdraw.OrderNo)

	// 获取用户钱包地址和团队信息
	walletAddress, teamName := getUserWalletAndTeam(ctx, withdraw.UserID)
	// 获取用户可提现余额、累计提现和累计充值
	availableBalance := getUserAvailableBalance(ctx, withdraw.UserID, withdraw.Symbol)
	totalWithdraw := getUserTotalWithdraw(ctx, withdraw.UserID, withdraw.Symbol)
	totalRecharge := getUserTotalRecharge(ctx, withdraw.UserID, withdraw.Symbol)
	message := fmt.Sprintf(
		"<b>提现申请</b>\n订单号: <code>%s</code>\n用户ID: <b>%d</b>\n对应地址: <code>%s</code>\n所属团队: <b>%s</b>\n币种: <b>%s</b>\n申请金额: <b>%s</b>\n到账金额: <b>%s</b>\n接收地址: <code>%s</code>\n状态: <b>%s</b>\n可提现余额: <b>%s %s</b>\n累计充值: <b>%s %s</b>\n累计提现: <b>%s %s</b>",
		htmlEscape(withdraw.OrderNo),
		withdraw.UserID,
		htmlEscape(walletAddress),
		htmlEscape(teamName),
		htmlEscape(withdraw.Symbol),
		htmlEscape(withdraw.Amount.String()),
		htmlEscape(withdraw.ActualAmount.String()),
		htmlEscape(withdraw.ToAddress),
		htmlEscape(withdrawStatusText(withdraw.Status)),
		htmlEscape(availableBalance.String()),
		htmlEscape(withdraw.Symbol),
		htmlEscape(totalRecharge.String()),
		htmlEscape(withdraw.Symbol),
		htmlEscape(totalWithdraw.String()),
		htmlEscape(withdraw.Symbol),
	)
	if withdraw.Status == entityCobo.WithdrawStatusPending {
		message += "\n\n请审核: @today_win @TokenAiYY"
	}
	messageID, err := s.sendMessageBySymbol(ctx, message, withdraw.Symbol)
	if err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送提现申请通知失败: order_no=%s err=%v", withdraw.OrderNo, err)
		return
	}
	s.saveWithdrawMessageID(ctx, withdraw.OrderNo, messageID)
}

func (s *TelegramNotifyService) NotifyLargeWithdrawAutoApproved(ctx context.Context, withdraw *entityCobo.WithdrawEntity) {
	if withdraw == nil || !s.IsEnabled() {
		return
	}
	if strings.ToUpper(strings.TrimSpace(withdraw.Symbol)) != "USDT" {
		return
	}
	threadID := s.getTelegramTopicID(ctx, "telegram_topic_large_withdraw")
	if threadID <= 0 {
		g.Log().Warningf(ctx, "[TelegramNotify] 大额提现 topic 未配置: key=telegram_topic_large_withdraw")
		return
	}

	walletAddress, teamName := getUserWalletAndTeam(ctx, withdraw.UserID)
	availableBalance := getUserAvailableBalance(ctx, withdraw.UserID, withdraw.Symbol)
	totalWithdraw := getUserTotalWithdraw(ctx, withdraw.UserID, withdraw.Symbol)
	totalRecharge := getUserTotalRecharge(ctx, withdraw.UserID, withdraw.Symbol)
	message := fmt.Sprintf(
		"<b>大额提现自动处理</b>\n订单号: <code>%s</code>\n用户ID: <b>%d</b>\n对应地址: <code>%s</code>\n所属团队: <b>%s</b>\n币种: <b>%s</b>\n申请金额: <b>%s</b>\n到账金额: <b>%s</b>\n接收地址: <code>%s</code>\n状态: <b>%s</b>\n可提现余额: <b>%s %s</b>\n累计充值: <b>%s %s</b>\n累计提现: <b>%s %s</b>",
		htmlEscape(withdraw.OrderNo),
		withdraw.UserID,
		htmlEscape(walletAddress),
		htmlEscape(teamName),
		htmlEscape(withdraw.Symbol),
		htmlEscape(withdraw.Amount.String()),
		htmlEscape(withdraw.ActualAmount.String()),
		htmlEscape(withdraw.ToAddress),
		htmlEscape(withdrawStatusText(withdraw.Status)),
		htmlEscape(availableBalance.String()),
		htmlEscape(withdraw.Symbol),
		htmlEscape(totalRecharge.String()),
		htmlEscape(withdraw.Symbol),
		htmlEscape(totalWithdraw.String()),
		htmlEscape(withdraw.Symbol),
	)
	message += "\n\n请关注: @today_win @TokenAiYY"
	if _, err := s.sendMessageToTopic(ctx, message, threadID); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送大额提现通知失败: order_no=%s err=%v", withdraw.OrderNo, err)
	}
}

func (s *TelegramNotifyService) NotifyWithdrawStatusChanged(ctx context.Context, withdraw *entityCobo.WithdrawEntity, oldStatus int, reason string) {
	if withdraw == nil || oldStatus == withdraw.Status {
		return
	}
	if !s.shouldNotifyWithdrawStatus(withdraw.Amount, withdraw.Status, withdraw.Symbol) {
		return
	}

	// 删除该订单之前发送的通知，避免同一订单多条消息堆积
	s.deletePreviousWithdrawMessage(ctx, withdraw.OrderNo)

	// 获取用户钱包地址和团队信息
	walletAddress, teamName := getUserWalletAndTeam(ctx, withdraw.UserID)
	// 获取用户可提现余额、累计提现和累计充值
	availableBalance := getUserAvailableBalance(ctx, withdraw.UserID, withdraw.Symbol)
	totalWithdraw := getUserTotalWithdraw(ctx, withdraw.UserID, withdraw.Symbol)
	totalRecharge := getUserTotalRecharge(ctx, withdraw.UserID, withdraw.Symbol)

	title := "<b>提现状态变更</b>"
	statusText := fmt.Sprintf("<b>%s -> %s</b>", htmlEscape(withdrawStatusText(oldStatus)), htmlEscape(withdrawStatusText(withdraw.Status)))
	if withdraw.Status == entityCobo.WithdrawStatusRejected {
		title = "🚨 <b>提现状态变更 - 已拒绝</b> 🚨"
		statusText = fmt.Sprintf("<b>%s -> ❌ %s</b>", htmlEscape(withdrawStatusText(oldStatus)), htmlEscape(withdrawStatusText(withdraw.Status)))
	}
	message := fmt.Sprintf(
		"%s\n订单号: <code>%s</code>\n用户ID: <b>%d</b>\n对应地址: <code>%s</code>\n所属团队: <b>%s</b>\n金额: <b>%s %s</b>\n状态: %s\n接收地址: <code>%s</code>\n可提现余额: <b>%s %s</b>\n累计充值: <b>%s %s</b>\n累计提现: <b>%s %s</b>",
		title,
		htmlEscape(withdraw.OrderNo),
		withdraw.UserID,
		htmlEscape(walletAddress),
		htmlEscape(teamName),
		htmlEscape(withdraw.Amount.String()),
		htmlEscape(withdraw.Symbol),
		statusText,
		htmlEscape(withdraw.ToAddress),
		htmlEscape(availableBalance.String()),
		htmlEscape(withdraw.Symbol),
		htmlEscape(totalRecharge.String()),
		htmlEscape(withdraw.Symbol),
		htmlEscape(totalWithdraw.String()),
		htmlEscape(withdraw.Symbol),
	)
	if withdraw.TxHash != "" {
		message += fmt.Sprintf("\nTxHash: <code>%s</code>", htmlEscape(withdraw.TxHash))
	}
	if reason != "" {
		message += fmt.Sprintf("\n备注: %s", htmlEscape(reason))
	}
	if withdraw.Status == entityCobo.WithdrawStatusPending {
		message += "\n\n请审核: @today_win @TokenAiYY"
	}

	messageID, err := s.sendMessageBySymbol(ctx, message, withdraw.Symbol)
	if err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送提现状态通知失败: order_no=%s err=%v", withdraw.OrderNo, err)
		return
	}
	s.saveWithdrawMessageID(ctx, withdraw.OrderNo, messageID)
}

func (s *TelegramNotifyService) NotifyWithdrawRejected(ctx context.Context, userID int64, amount decimal.Decimal, symbol, toAddress, reason string) {
	if !s.IsEnabled() {
		return
	}

	walletAddress, teamName := getUserWalletAndTeam(ctx, userID)
	availableBalance := getUserAvailableBalance(ctx, userID, symbol)
	totalWithdraw := getUserTotalWithdraw(ctx, userID, symbol)
	totalRecharge := getUserTotalRecharge(ctx, userID, symbol)

	message := fmt.Sprintf(
		"🚨 <b>提现被拒绝</b> 🚨\n⛔ 用户ID: <b>%d</b>\n⛔ 对应地址: <code>%s</code>\n⛔ 所属团队: <b>%s</b>\n⛔ 币种: <b>%s</b>\n⛔ 申请金额: <b>%s</b>\n⛔ 接收地址: <code>%s</code>\n⛔ 拒绝原因: <b>%s</b>\n可提现余额: <b>%s %s</b>\n累计充值: <b>%s %s</b>\n累计提现: <b>%s %s</b>",
		userID,
		htmlEscape(walletAddress),
		htmlEscape(teamName),
		htmlEscape(symbol),
		htmlEscape(amount.String()),
		htmlEscape(toAddress),
		htmlEscape(reason),
		htmlEscape(availableBalance.String()),
		htmlEscape(symbol),
		htmlEscape(totalRecharge.String()),
		htmlEscape(symbol),
		htmlEscape(totalWithdraw.String()),
		htmlEscape(symbol),
	)

	if _, err := s.sendMessageBySymbol(ctx, message, symbol); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送提现拒绝通知失败: user_id=%d err=%v", userID, err)
	}
}

func (s *TelegramNotifyService) NotifyWithdrawBalanceInsufficient(ctx context.Context, userID int64, amount decimal.Decimal, symbol, toAddress string, availableBalance decimal.Decimal) {
	if !s.IsEnabled() {
		return
	}

	walletAddress, teamName := getUserWalletAndTeam(ctx, userID)
	totalWithdraw := getUserTotalWithdraw(ctx, userID, symbol)
	totalRecharge := getUserTotalRecharge(ctx, userID, symbol)

	message := fmt.Sprintf(
		"⚠️ <b>提现余额不足 - 可能存在异常</b> ⚠️\n⛔ 用户ID: <b>%d</b>\n⛔ 对应地址: <code>%s</code>\n⛔ 所属团队: <b>%s</b>\n⛔ 币种: <b>%s</b>\n⛔ 申请金额: <b>%s</b>\n⛔ 接收地址: <code>%s</code>\n⛔ 可用余额: <b>%s %s</b>\n⛔ 累计充值: <b>%s %s</b>\n⛔ 累计提现: <b>%s %s</b>\n\n<b>注意：该用户可用余额不足但尝试发起提现，可能存在被盗号或攻击行为，请关注！</b>",
		userID,
		htmlEscape(walletAddress),
		htmlEscape(teamName),
		htmlEscape(symbol),
		htmlEscape(amount.String()),
		htmlEscape(toAddress),
		htmlEscape(availableBalance.String()),
		htmlEscape(symbol),
		htmlEscape(totalRecharge.String()),
		htmlEscape(symbol),
		htmlEscape(totalWithdraw.String()),
		htmlEscape(symbol),
	)

	if _, err := s.sendMessageBySymbol(ctx, message, symbol); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送余额不足警告通知失败: user_id=%d err=%v", userID, err)
	}
}

func (s *TelegramNotifyService) NotifyWithdrawBalanceInsufficientAsync(userID int64, amount decimal.Decimal, symbol, toAddress string, availableBalance decimal.Decimal) {
	if !s.IsEnabled() {
		return
	}

	select {
	case s.insufficientNotifyCh <- struct{}{}:
	default:
		g.Log().Warningf(context.Background(), "[TelegramNotify] 提现余额不足通知队列已满，跳过通知: user_id=%d", userID)
		return
	}

	go func() {
		defer func() {
			<-s.insufficientNotifyCh
		}()

		notifyCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		allowed, err := s.acquireWithdrawInsufficientNotifyLock(notifyCtx, userID)
		if err != nil {
			g.Log().Warningf(notifyCtx, "[TelegramNotify] 提现余额不足通知限流检查失败: user_id=%d err=%v", userID, err)
		}
		if !allowed {
			return
		}

		s.NotifyWithdrawBalanceInsufficient(notifyCtx, userID, amount, symbol, toAddress, availableBalance)
	}()
}

func (s *TelegramNotifyService) NotifyWithdrawRejectedAsync(userID int64, amount decimal.Decimal, symbol, toAddress, reason string) {
	if !s.IsEnabled() {
		return
	}

	select {
	case s.rejectedNotifyCh <- struct{}{}:
	default:
		g.Log().Warningf(context.Background(), "[TelegramNotify] 提现拒绝通知队列已满，跳过通知: user_id=%d", userID)
		return
	}

	go func() {
		defer func() {
			<-s.rejectedNotifyCh
		}()

		notifyCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		allowed, err := s.acquireWithdrawRejectedNotifyLock(notifyCtx, userID)
		if err != nil {
			g.Log().Warningf(notifyCtx, "[TelegramNotify] 提现拒绝通知限流检查失败: user_id=%d err=%v", userID, err)
		}
		if !allowed {
			return
		}

		s.NotifyWithdrawRejected(notifyCtx, userID, amount, symbol, toAddress, reason)
	}()
}

func (s *TelegramNotifyService) NotifyWithdrawAddressMismatch(ctx context.Context, userID int64, symbol, walletAddress, receiveAddress string) {
	if !s.IsEnabled() {
		return
	}

	_, teamName := getUserWalletAndTeam(ctx, userID)
	message := fmt.Sprintf(
		"🚨🚨🚨 <b>紧急告警：发现黑客疑似通过 API 窃取资金</b> 🚨🚨🚨\n"+
			"👤 用户ID: <b>%d</b>\n"+
			"💰 币种: <b>%s</b>\n"+
			"👥 所属团队: <b>%s</b>\n"+
			"🏠 用户钱包地址: <code>%s</code>\n"+
			"🎯 提现接收地址: <code>%s</code>\n"+
			"🔍 地址对比: <b>不一致（已自动禁用该用户提现权限）</b>\n\n"+
			"<b>请立即核查该用户账户与提现行为。</b>",
		userID,
		htmlEscape(symbol),
		htmlEscape(teamName),
		htmlEscape(walletAddress),
		htmlEscape(receiveAddress),
	)

	if _, err := s.sendMessageBySymbol(ctx, message, symbol); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送提现地址不一致紧急告警失败: user_id=%d err=%v", userID, err)
	}
}

func (s *TelegramNotifyService) NotifyRechargeConfirmed(ctx context.Context, userID int64, amount, totalAmount decimal.Decimal, symbol, txHash string) {
	s.NotifyRechargeConfirmedWithSource(ctx, userID, amount, totalAmount, symbol, txHash, "Cobo")
}

func (s *TelegramNotifyService) NotifyRechargeRejected(ctx context.Context, userID int64, amount decimal.Decimal, asset, txHash string) {
	if !s.IsEnabled() {
		return
	}

	walletAddress, teamName := getUserWalletAndTeam(ctx, userID)
	message := fmt.Sprintf(
		"🚨🚨🚨 <b>充币被拒绝：非白名单合约</b> 🚨🚨🚨\n"+
			"👤 用户ID: <b>%d</b>\n"+
			"🏠 钱包: <code>%s</code>\n"+
			"👥 团队: <b>%s</b>\n"+
			"💰 Cobo Asset: <b>%s</b>\n"+
			"💵 金额: <b>%s</b>",
		userID,
		htmlEscape(walletAddress),
		htmlEscape(teamName),
		htmlEscape(asset),
		htmlEscape(amount.String()),
	)
	if txHash != "" {
		if strings.HasPrefix(asset, "BSC_") {
			bscScanURL := fmt.Sprintf("https://bscscan.com/tx/%s", txHash)
			message += fmt.Sprintf("\n🔗 TxHash: <a href=\"%s\">%s</a>", bscScanURL, htmlEscape(txHash))
		} else {
			message += fmt.Sprintf("\n🔗 TxHash: <code>%s</code>", htmlEscape(txHash))
		}
	}

	if _, err := s.sendMessage(ctx, message); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送充币拒绝告警失败: user_id=%d err=%v", userID, err)
	}
}

func (s *TelegramNotifyService) NotifyRechargeConfirmedWithSource(ctx context.Context, userID int64, amount, totalAmount decimal.Decimal, symbol, txHash, source string) {
	if !s.IsEnabled() {
		return
	}
	if !s.shouldNotifyAmount(amount, symbol) {
		return
	}
	if strings.TrimSpace(source) == "" {
		source = "Cobo"
	}

	// 获取用户钱包地址和团队信息
	walletAddress, teamName := getUserWalletAndTeam(ctx, userID)
	// 获取用户可提现余额和累计提现
	availableBalance := getUserAvailableBalance(ctx, userID, symbol)
	totalWithdraw := getUserTotalWithdraw(ctx, userID, symbol)

	message := fmt.Sprintf(
		"<b>用户充值到账</b>\n渠道: <b>%s</b>\n用户ID: <b>%d</b>\n对应地址: <code>%s</code>\n所属团队: <b>%s</b>\n币种: <b>%s</b>\n到账金额: <b>%s</b>\n累计充值: <b>%s</b>\n可提现余额: <b>%s %s</b>\n累计提现: <b>%s %s</b>",
		htmlEscape(source),
		userID,
		htmlEscape(walletAddress),
		htmlEscape(teamName),
		htmlEscape(symbol),
		htmlEscape(amount.String()),
		htmlEscape(totalAmount.String()),
		htmlEscape(availableBalance.String()),
		htmlEscape(symbol),
		htmlEscape(totalWithdraw.String()),
		htmlEscape(symbol),
	)
	if txHash != "" {
		bscScanURL := fmt.Sprintf("https://bscscan.com/tx/%s", txHash)
		message += fmt.Sprintf("\nTxHash: <a href=\"%s\">%s</a>", bscScanURL, htmlEscape(txHash))
	}
	if _, err := s.sendMessageBySymbol(ctx, message, symbol); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送充值到账通知失败: user_id=%d err=%v", userID, err)
	}
}

func (s *TelegramNotifyService) NotifyDailyDepositWithdrawStats(ctx context.Context, day time.Time) error {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	start := time.Date(day.In(loc).Year(), day.In(loc).Month(), day.In(loc).Day(), 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)

	rechargeCount, err := g.DB().Model("cobo_recharge_record").Ctx(ctx).
		Where("created_at >= ? AND created_at < ?", start, end).
		Where("status = ?", entityCobo.RechargeStatusConfirmed).
		Count()
	if err != nil {
		return err
	}
	rechargeAmountVar, err := g.DB().Model("cobo_recharge_record").Ctx(ctx).
		Fields("COALESCE(SUM(amount), 0)").
		Where("created_at >= ? AND created_at < ?", start, end).
		Where("status = ?", entityCobo.RechargeStatusConfirmed).
		Value()
	if err != nil {
		return err
	}

	withdrawCount, err := g.DB().Model("cobo_withdraw_request").Ctx(ctx).
		Where("created_at >= ? AND created_at < ?", start, end).
		Count()
	if err != nil {
		return err
	}
	withdrawAmountVar, err := g.DB().Model("cobo_withdraw_request").Ctx(ctx).
		Fields("COALESCE(SUM(amount), 0)").
		Where("created_at >= ? AND created_at < ?", start, end).
		Value()
	if err != nil {
		return err
	}

	withdrawSuccessCount, err := g.DB().Model("cobo_withdraw_request").Ctx(ctx).
		Where("created_at >= ? AND created_at < ?", start, end).
		Where("status = ?", entityCobo.WithdrawStatusSuccess).
		Count()
	if err != nil {
		return err
	}
	withdrawFailedCount, err := g.DB().Model("cobo_withdraw_request").Ctx(ctx).
		Where("created_at >= ? AND created_at < ?", start, end).
		Where("status IN (?)", []int{entityCobo.WithdrawStatusFailed, entityCobo.WithdrawStatusRejected}).
		Count()
	if err != nil {
		return err
	}

	message := fmt.Sprintf(
		"<b>每日充提统计</b>\n日期: <b>%s</b>\n\n充值(已确认): <b>%d</b> 笔 / <b>%s USDT</b>\n提现(总申请): <b>%d</b> 笔 / <b>%s USDT</b>\n提现成功: <b>%d</b> 笔\n提现失败/拒绝: <b>%d</b> 笔",
		htmlEscape(start.Format("2006-01-02")),
		rechargeCount,
		htmlEscape(rechargeAmountVar.String()),
		withdrawCount,
		htmlEscape(withdrawAmountVar.String()),
		withdrawSuccessCount,
		withdrawFailedCount,
	)

	_, err = s.sendMessage(ctx, message)
	return err
}

func (s *TelegramNotifyService) NotifyGroupMatchSettlementStarted(ctx context.Context, sessionID int64, sessionName string, totalOrders, totalPlans int) {
	if !s.IsEnabled() {
		return
	}
	message := fmt.Sprintf(
		"<b>拼团结算开始</b>\n#%d %s | 订单: %d | 组: %d",
		sessionID,
		htmlEscape(sessionName),
		totalOrders,
		totalPlans,
	)
	if _, err := s.sendMessage(ctx, message); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送拼团结算开始通知失败: session_id=%d err=%v", sessionID, err)
	}
}

func (s *TelegramNotifyService) NotifyGroupMatchSettlementCompleted(ctx context.Context, sessionID int64, sessionName string, totalOrders, totalGroups, totalFlow int, elapsed time.Duration) {
	if !s.IsEnabled() {
		return
	}
	message := fmt.Sprintf(
		"<b>拼团结算完成</b>\n#%d %s | 订单: %d | 成团: %d | 流单: %d | %s",
		sessionID,
		htmlEscape(sessionName),
		totalOrders,
		totalGroups,
		totalFlow,
		htmlEscape(elapsed.Truncate(time.Second).String()),
	)
	if _, err := s.sendMessage(ctx, message); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送拼团结算完成通知失败: session_id=%d err=%v", sessionID, err)
	}
}

func (s *TelegramNotifyService) NotifyGroupMatchLeadershipRewardDistributed(ctx context.Context, sessionID int64, sessionName string, count int, elapsed time.Duration) {
	if !s.IsEnabled() {
		return
	}
	message := fmt.Sprintf(
		"<b>拼团领导奖完成</b>\n#%d %s | 人数: %d | %s",
		sessionID,
		htmlEscape(sessionName),
		count,
		htmlEscape(elapsed.Truncate(time.Second).String()),
	)
	if _, err := s.sendMessage(ctx, message); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送拼团领导奖通知失败: session_id=%d err=%v", sessionID, err)
	}
}

func (s *TelegramNotifyService) NotifyGroupMatchStakingV2LeaderRewardDistributed(ctx context.Context, sessionID int64, sessionName string, count int, elapsed time.Duration) {
	if !s.IsEnabled() {
		return
	}
	message := fmt.Sprintf(
		"<b>StakingV2领导奖完成</b>\n#%d %s | 人数: %d | %s",
		sessionID,
		htmlEscape(sessionName),
		count,
		htmlEscape(elapsed.Truncate(time.Second).String()),
	)
	if _, err := s.sendMessage(ctx, message); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送StakingV2领导奖通知失败: session_id=%d err=%v", sessionID, err)
	}
}

func (s *TelegramNotifyService) NotifyGroupMatchLoserCompReleaseCompleted(ctx context.Context, sessionID int64, sessionName string, totalReleased int, elapsed time.Duration) {
	if !s.IsEnabled() {
		return
	}
	message := fmt.Sprintf(
		"<b>拼团YY释放完成</b>\n#%d %s | 笔数: %d | %s",
		sessionID,
		htmlEscape(sessionName),
		totalReleased,
		htmlEscape(elapsed.Truncate(time.Second).String()),
	)
	if _, err := s.sendMessage(ctx, message); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 发送拼团YY释放通知失败: session_id=%d err=%v", sessionID, err)
	}
}

func (s *TelegramNotifyService) sendMessage(ctx context.Context, message string) (int64, error) {
	if !s.IsEnabled() {
		return 0, nil
	}

	bodyMap := map[string]interface{}{
		"chat_id":                  s.chatID,
		"text":                     message,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("telegram http status=%d body=%s", resp.StatusCode, string(raw))
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(raw, &respMap); err != nil {
		return 0, fmt.Errorf("解析telegram响应失败: %w", err)
	}
	ok, _ := respMap["ok"].(bool)
	if !ok {
		return 0, fmt.Errorf("telegram返回失败: %s", string(raw))
	}

	if result, ok := respMap["result"].(map[string]interface{}); ok {
		if msgID, ok := result["message_id"].(float64); ok {
			return int64(msgID), nil
		}
	}

	return 0, nil
}

func (s *TelegramNotifyService) sendMessageBySymbol(ctx context.Context, message, symbol string) (int64, error) {
	topic := s.getTelegramTopicNotify(ctx, symbol)
	if topic == nil || topic.MessageThreadID <= 0 {
		return s.sendMessage(ctx, message)
	}
	return s.sendMessageToTopic(ctx, message, topic.MessageThreadID)
}

func (s *TelegramNotifyService) sendMessageToTopic(ctx context.Context, message string, messageThreadID int64) (int64, error) {
	if !s.IsEnabled() {
		return 0, nil
	}

	bodyMap := map[string]interface{}{
		"chat_id":                  s.chatID,
		"text":                     message,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
		"message_thread_id":        messageThreadID,
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return 0, fmt.Errorf("telegram http status=%d body=%s", resp.StatusCode, string(raw))
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(raw, &respMap); err != nil {
		return 0, fmt.Errorf("解析telegram响应失败: %w", err)
	}
	ok, _ := respMap["ok"].(bool)
	if !ok {
		return 0, fmt.Errorf("telegram返回失败: %s", string(raw))
	}

	if result, ok := respMap["result"].(map[string]interface{}); ok {
		if msgID, ok := result["message_id"].(float64); ok {
			return int64(msgID), nil
		}
	}

	return 0, nil
}

// getTelegramTopicNotify 根据币种读取 topic 路由配置。
// 仅提供路由信息，不改变现有发送行为。
func (s *TelegramNotifyService) getTelegramTopicNotify(ctx context.Context, symbol string) *telegramTopicNotify {
	if strings.TrimSpace(symbol) == "" {
		return nil
	}

	normalized := strings.ToLower(strings.TrimSpace(symbol))
	key := fmt.Sprintf("telegram_topic_%s", normalized)
	threadID := s.getTelegramTopicID(ctx, key)
	if threadID <= 0 {
		return nil
	}

	return &telegramTopicNotify{
		Symbol:          strings.ToUpper(normalized),
		MessageThreadID: threadID,
	}
}

func (s *TelegramNotifyService) getTelegramTopicID(ctx context.Context, key string) int64 {
	value, err := g.DB().Model("system_config").Ctx(ctx).
		Fields("value").
		Where("key = ?", key).
		Value()
	if err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 查询 topic 配置失败: key=%s err=%v", key, err)
		return 0
	}
	if value == nil {
		return 0
	}

	threadID := value.Int64()
	if threadID <= 0 {
		return 0
	}
	return threadID
}

func (s *TelegramNotifyService) deleteMessage(ctx context.Context, chatID string, messageID int64) error {
	if !s.IsEnabled() || messageID <= 0 {
		return nil
	}

	bodyMap := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	url := fmt.Sprintf("https://api.telegram.org/bot%s/deleteMessage", s.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("telegram deleteMessage http status=%d body=%s", resp.StatusCode, string(raw))
	}

	var respMap map[string]interface{}
	if err := json.Unmarshal(raw, &respMap); err != nil {
		return fmt.Errorf("解析telegram deleteMessage响应失败: %w", err)
	}
	ok, _ := respMap["ok"].(bool)
	if !ok {
		return fmt.Errorf("telegram deleteMessage返回失败: %s", string(raw))
	}

	return nil
}

func (s *TelegramNotifyService) saveWithdrawMessageID(ctx context.Context, orderNo string, messageID int64) {
	if messageID <= 0 || orderNo == "" {
		return
	}
	key := fmt.Sprintf("cobo:telegram:msg:%s", orderNo)
	if err := g.Redis().SetEX(ctx, key, messageID, 604800); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 保存提现消息ID失败: order_no=%s err=%v", orderNo, err)
	}
}

func (s *TelegramNotifyService) deleteWithdrawMessageID(ctx context.Context, orderNo string) {
	if orderNo == "" {
		return
	}
	key := fmt.Sprintf("cobo:telegram:msg:%s", orderNo)
	if _, err := g.Redis().Del(ctx, key); err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 删除提现消息ID失败: order_no=%s err=%v", orderNo, err)
	}
}

func (s *TelegramNotifyService) getWithdrawMessageID(ctx context.Context, orderNo string) int64 {
	if orderNo == "" {
		return 0
	}
	key := fmt.Sprintf("cobo:telegram:msg:%s", orderNo)
	val, err := g.Redis().Get(ctx, key)
	if err != nil || val == nil {
		return 0
	}
	return val.Int64()
}

func (s *TelegramNotifyService) deletePreviousWithdrawMessage(ctx context.Context, orderNo string) {
	if orderNo == "" {
		return
	}
	messageID := s.getWithdrawMessageID(ctx, orderNo)
	if messageID <= 0 {
		return
	}
	if err := s.deleteMessage(ctx, s.chatID, messageID); err != nil {
		if strings.Contains(err.Error(), "message to delete not found") {
			s.deleteWithdrawMessageID(ctx, orderNo)
		}
		g.Log().Warningf(ctx, "[TelegramNotify] 删除旧提现通知失败: order_no=%s message_id=%d err=%v", orderNo, messageID, err)
	} else {
		s.deleteWithdrawMessageID(ctx, orderNo)
		g.Log().Infof(ctx, "[TelegramNotify] 已删除旧提现通知: order_no=%s message_id=%d", orderNo, messageID)
	}
}

func withdrawStatusText(status int) string {
	switch status {
	case entityCobo.WithdrawStatusPending:
		return "待审核"
	case entityCobo.WithdrawStatusApproved:
		return "审核通过"
	case entityCobo.WithdrawStatusRejected:
		return "审核拒绝"
	case entityCobo.WithdrawStatusProcessing:
		return "处理中"
	case entityCobo.WithdrawStatusSuccess:
		return "成功"
	case entityCobo.WithdrawStatusFailed:
		return "失败"
	default:
		return fmt.Sprintf("未知(%d)", status)
	}
}

func htmlEscape(v string) string {
	return html.EscapeString(v)
}

func (s *TelegramNotifyService) acquireDailyStatsLock(ctx context.Context, day time.Time) (bool, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	key := fmt.Sprintf("cobo:telegram:daily_stats:%s", day.In(loc).Format("2006-01-02"))

	result, err := g.Redis().Do(ctx, "SET", key, "1", "NX", "EX", 172800)
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}
	return result.String() == "OK", nil
}

func (s *TelegramNotifyService) acquireWithdrawRejectedNotifyLock(ctx context.Context, userID int64) (bool, error) {
	redisClient := g.Redis()
	if redisClient == nil {
		return true, nil
	}

	key := fmt.Sprintf("cobo:telegram:withdraw_rejected:%d", userID)
	result, err := redisClient.Do(ctx, "SET", key, "1", "NX", "EX", 60)
	if err != nil {
		return true, err
	}
	if result.IsNil() {
		return false, nil
	}
	return result.String() == "OK", nil
}

func (s *TelegramNotifyService) acquireWithdrawInsufficientNotifyLock(ctx context.Context, userID int64) (bool, error) {
	redisClient := g.Redis()
	if redisClient == nil {
		return true, nil
	}

	key := fmt.Sprintf("cobo:telegram:withdraw_insufficient:%d", userID)
	result, err := redisClient.Do(ctx, "SET", key, "1", "NX", "EX", 60)
	if err != nil {
		return true, err
	}
	if result.IsNil() {
		return false, nil
	}
	return result.String() == "OK", nil
}

func getUserWalletAddress(ctx context.Context, userID int64) string {
	var result struct {
		WalletAddress string `json:"wallet_address"`
	}
	err := g.DB().Model("user_info").Ctx(ctx).
		Where("id = ?", userID).
		Scan(&result)
	if err != nil {
		return ""
	}
	return result.WalletAddress
}

// getUserAvailableBalance 获取用户可提现余额
func getUserAvailableBalance(ctx context.Context, userID int64, symbol string) decimal.Decimal {
	var result struct {
		AvailableAmount decimal.Decimal `json:"available_amount"`
	}
	err := g.DB().Model("cobo_balance").Ctx(ctx).
		Fields("available_amount").
		Where("user_id = ? AND symbol = ?", userID, symbol).
		Scan(&result)
	if err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 查询用户余额失败 (user_id=%d): %v", userID, err)
		return decimal.Zero
	}
	return result.AvailableAmount
}

// getUserTotalWithdraw 获取用户累计提现金额（排除已拒绝/失败状态，口径与风控一致）
func getUserTotalWithdraw(ctx context.Context, userID int64, symbol string) decimal.Decimal {
	var result struct {
		TotalAmount decimal.Decimal `json:"total_amount"`
	}
	err := g.DB().Model("cobo_withdraw_request").Ctx(ctx).
		Fields("COALESCE(SUM(amount), 0) as total_amount").
		Where("user_id = ? AND symbol = ? AND status NOT IN (?, ?)", userID, symbol, entityCobo.WithdrawStatusRejected, entityCobo.WithdrawStatusFailed).
		Scan(&result)
	if err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 查询用户累计提现失败 (user_id=%d): %v", userID, err)
		return decimal.Zero
	}
	return result.TotalAmount
}

// getUserTotalRecharge 获取用户累计充值金额（仅已确认）
func getUserTotalRecharge(ctx context.Context, userID int64, symbol string) decimal.Decimal {
	var result struct {
		TotalAmount decimal.Decimal `json:"total_amount"`
	}
	err := g.DB().Model("cobo_recharge_record").Ctx(ctx).
		Fields("COALESCE(SUM(amount), 0) as total_amount").
		Where("user_id = ? AND symbol = ? AND status = ?", userID, symbol, entityCobo.RechargeStatusConfirmed).
		Scan(&result)
	if err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 查询用户累计充值失败 (user_id=%d): %v", userID, err)
		return decimal.Zero
	}
	return result.TotalAmount
}

// getUserWalletAndTeam 获取用户钱包地址和所属团队名称
// 规则：优先通过 team_id JOIN team 表获取团队名；若用户同时是其他团队的 leader，则拼接显示
func getUserWalletAndTeam(ctx context.Context, userID int64) (walletAddress, teamName string) {
	record, err := g.DB().Model("user_info ui").Ctx(ctx).
		Fields("ui.wallet_address, ui.team_name, ui.team_id, t.name as team_name_from_table").
		LeftJoin("team t", "ui.team_id = t.id").
		Where("ui.id = ?", userID).
		One()
	if err != nil {
		g.Log().Warningf(ctx, "[TelegramNotify] 查询用户团队信息失败 (user_id=%d): %v", userID, err)
		return "", "未知"
	}
	if record == nil {
		return "", "未知"
	}

	walletAddress = record["wallet_address"].String()

	baseTeamName := ""
	if name := record["team_name_from_table"].String(); name != "" {
		baseTeamName = name
	} else if name := record["team_name"].String(); name != "" && name != "未知" {
		baseTeamName = name
	}
	if baseTeamName == "" {
		baseTeamName = "未知"
	}

	// 若该用户本身也是某个团队的 leader，且团队名与 baseTeamName 不同，则拼接显示
	leaderTeamVar, err := g.DB().Model("team").Ctx(ctx).
		Fields("name").
		Where("LOWER(leader_wallet_address) = LOWER(?)", walletAddress).
		Value()
	if err == nil {
		if leaderTeamName := leaderTeamVar.String(); leaderTeamName != "" && leaderTeamName != baseTeamName {
			return walletAddress, fmt.Sprintf("%s (%s本人)团队", baseTeamName, leaderTeamName)
		}
	}

	return walletAddress, baseTeamName
}
