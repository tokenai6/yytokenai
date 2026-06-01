package group_match

import (
	"context"
	"math"

	"XWFrame/internal/frame/consts"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// InsertFakeGroupOrderBatch 为指定场次插入一批假订单。
// batchSize 为本次新增单量，由调用方根据剩余目标与剩余时间均匀计算。
func InsertFakeGroupOrderBatch(ctx context.Context, sessionID int64, batchSize int) error {
	if batchSize <= 0 {
		return nil
	}
	_, err := g.DB().Model("fake_group_order").Ctx(ctx).Insert(g.Map{
		"session_id": sessionID,
		"new_order":  batchSize,
	})
	if err != nil {
		return err
	}
	return nil
}

// ResolveBatchSize 根据剩余目标量、剩余秒数和调度周期（秒）计算本次应插入的单量。
// ResolveTargetBySessionName 根据场次名称从 system_config 读取假订单目标总量，读不到时用默认值。
func ResolveTargetBySessionName(ctx context.Context, sessionName string) int {
	keyMap := map[string]string{
		"Morning":   "group_match_sim_target_morning",
		"Afternoon": "group_match_sim_target_afternoon",
		"Evening":   "group_match_sim_target_evening",
	}
	defaultMap := map[string]int{
		"Morning":   80000,
		"Afternoon": 90000,
		"Evening":   100000,
	}
	key, ok := keyMap[sessionName]
	if !ok {
		return 0
	}
	fallback := defaultMap[sessionName]
	val, err := g.DB().Model("system_config").Ctx(ctx).
		Fields("value").
		Where("key", key).
		Value()
	if err != nil {
		g.Log().Warningf(ctx, "ResolveTargetBySessionName read config failed: key=%s err=%v, fallback=%d", key, err, fallback)
		return fallback
	}
	if val.IsEmpty() {
		return fallback
	}
	d, err := decimal.NewFromString(val.String())
	if err != nil {
		g.Log().Warningf(ctx, "ResolveTargetBySessionName parse config failed: key=%s value=%s err=%v, fallback=%d", key, val.String(), err, fallback)
		return fallback
	}
	v := int(d.IntPart())
	if v <= 0 {
		return fallback
	}
	return v
}

func ResolveBatchSize(remainingTarget, remainingSec, intervalSec int) int {
	if remainingTarget <= 0 || remainingSec <= 0 || intervalSec <= 0 {
		return 0
	}
	batch := int(math.Ceil(float64(remainingTarget) / float64(remainingSec) * float64(intervalSec)))
	if batch > remainingTarget {
		batch = remainingTarget
	}
	if batch <= 0 {
		return 0
	}
	return batch
}

// IsSessionActive 查询数据库确认场次是否仍为 Active 状态。
func IsSessionActive(ctx context.Context, sessionID int64) (bool, error) {
	value, err := g.DB().Model("group_match_session").Ctx(ctx).
		Fields("status").
		Where("id", sessionID).
		Value()
	if err != nil {
		return false, err
	}
	status := value.Int()
	return status == consts.GroupMatchSessionStatusActive, nil
}
