package apg

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gtime"
)

type IMatchDao interface {
	Create(ctx context.Context, match *entity.ApgMatch) error
	GetById(ctx context.Context, id int64) (*entity.ApgMatch, error)
	GetByDateRoundPool(ctx context.Context, matchDate string, round, poolType int) (*entity.ApgMatch, error)
	GetUnfinishedMatch(ctx context.Context, matchDate string, round, poolType int) (*entity.ApgMatch, error)
	GetCurrentActiveMatch(ctx context.Context, poolType int, now *gtime.Time) (*entity.ApgMatch, error)
	GetNextMatch(ctx context.Context, poolType int, now *gtime.Time) (*entity.ApgMatch, error)
	LockMatch(ctx context.Context, matchId int64) (int64, error)
	UpdateById(ctx context.Context, id int64, data gdb.Map) error
	IncrementPlayerCount(ctx context.Context, matchId int64, count int) error
	GetMatchList(ctx context.Context, poolType int, startDate, endDate string, page, pageSize int) ([]*entity.ApgMatch, int, error)
	GetAdminMatchList(ctx context.Context, matchDate string, poolType, isFinished, page, pageSize int) ([]*entity.ApgMatch, int, error)
}

type matchDao struct{}

var Match IMatchDao = &matchDao{}

func (d *matchDao) Create(ctx context.Context, match *entity.ApgMatch) error {
	id, err := db.GetDB().Model("apg_mint_match").OmitEmpty().InsertAndGetId(match)
	if err != nil {
		return err
	}
	match.Id = id
	return nil
}

func (d *matchDao) GetById(ctx context.Context, id int64) (*entity.ApgMatch, error) {
	var match *entity.ApgMatch
	err := db.GetDB().Model("apg_mint_match").Where("id", id).Scan(&match)
	if err != nil {
		return nil, err
	}
	return match, nil
}

func (d *matchDao) GetByDateRoundPool(ctx context.Context, matchDate string, round, poolType int) (*entity.ApgMatch, error) {
	var match *entity.ApgMatch
	err := db.GetDB().Model("apg_mint_match").
		Where("match_date", matchDate).
		Where("round", round).
		Where("pool_type", poolType).
		Scan(&match)
	if err != nil {
		return nil, err
	}
	return match, nil
}

func (d *matchDao) GetUnfinishedMatch(ctx context.Context, matchDate string, round, poolType int) (*entity.ApgMatch, error) {
	var match *entity.ApgMatch
	err := db.GetDB().Model("apg_mint_match").
		Where("match_date", matchDate).
		Where("round", round).
		Where("pool_type", poolType).
		Where("is_finished", 0).
		Scan(&match)
	if err != nil {
		return nil, err
	}
	return match, nil
}

// GetCurrentActiveMatch 查询当前时间落在 start_time 和 finish_time 之间的场次（包括已完成的场次，用于处理迟到事件）
func (d *matchDao) GetCurrentActiveMatch(ctx context.Context, poolType int, now *gtime.Time) (*entity.ApgMatch, error) {
	var match *entity.ApgMatch
	err := db.GetDB().Model("apg_mint_match").
		Where("pool_type", poolType).
		Where("start_time <= ?", now).
		Where("finish_time > ?", now).
		Order("start_time ASC").
		Limit(1).
		Scan(&match)
	if err != nil {
		return nil, err
	}
	return match, nil
}

// GetNextMatch 查询下一个即将开始的场次（start_time > now）
func (d *matchDao) GetNextMatch(ctx context.Context, poolType int, now *gtime.Time) (*entity.ApgMatch, error) {
	var match *entity.ApgMatch
	err := db.GetDB().Model("apg_mint_match").
		Where("pool_type", poolType).
		Where("is_finished", 0).
		Where("start_time > ?", now).
		Order("start_time ASC").
		Limit(1).
		Scan(&match)
	if err != nil {
		return nil, err
	}
	return match, nil
}

func (d *matchDao) LockMatch(ctx context.Context, matchId int64) (int64, error) {
	result, err := db.GetDB().Model("apg_mint_match").
		Where("id", matchId).
		Where("is_finished", 0).
		Update(gdb.Map{"is_finished": 1})
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func (d *matchDao) UpdateById(ctx context.Context, id int64, data gdb.Map) error {
	data["updated_at"] = gtime.Now()
	_, err := db.GetDB().Model("apg_mint_match").Where("id", id).Update(data)
	return err
}

func (d *matchDao) IncrementPlayerCount(ctx context.Context, matchId int64, count int) error {
	_, err := db.GetDB().Model("apg_mint_match").
		Where("id", matchId).
		Increment("player_count", count)
	return err
}

func (d *matchDao) GetMatchList(ctx context.Context, poolType int, startDate, endDate string, page, pageSize int) ([]*entity.ApgMatch, int, error) {
	model := db.GetDB().Model("apg_mint_match")

	// 历史场次只返回已完成的场次，排除未来和正在进行的场次
	model = model.Where("is_finished", 1)

	if poolType > 0 {
		model = model.Where("pool_type", poolType)
	}
	if startDate != "" {
		model = model.Where("match_date >= ?", startDate)
	}
	if endDate != "" {
		model = model.Where("match_date <= ?", endDate)
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var matches []*entity.ApgMatch
	err = model.Order("match_date DESC, round DESC").
		Page(page, pageSize).
		Scan(&matches)
	if err != nil {
		return nil, 0, err
	}

	if len(matches) > 0 {
		matchIds := make([]int64, len(matches))
		for i, m := range matches {
			matchIds[i] = m.Id
		}

		type CountResult struct {
			MatchId int64 `json:"match_id"`
			Count   int   `json:"count"`
		}
		var counts []CountResult
		err = db.GetDB().Model("apg_mint_player").
			Fields("match_id", "COUNT(*) as count").
			Where("match_id IN(?)", matchIds).
			Where("join_status", 0).
			Group("match_id").
			Scan(&counts)
		if err != nil {
			return nil, 0, err
		}

		countMap := make(map[int64]int)
		for _, c := range counts {
			countMap[c.MatchId] = c.Count
		}
		for _, m := range matches {
			m.PlayerCount = countMap[m.Id]
		}
	}

	return matches, total, err
}

func (d *matchDao) GetAdminMatchList(ctx context.Context, matchDate string, poolType, isFinished, page, pageSize int) ([]*entity.ApgMatch, int, error) {
	model := db.GetDB().Model("apg_mint_match")

	if matchDate != "" {
		model = model.Where("match_date", matchDate)
	}
	if poolType > 0 {
		model = model.Where("pool_type", poolType)
	}
	if isFinished >= 0 {
		model = model.Where("is_finished", isFinished)
	}

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var matches []*entity.ApgMatch
	err = model.Order("id DESC").
		Page(page, pageSize).
		Scan(&matches)

	return matches, total, err
}
