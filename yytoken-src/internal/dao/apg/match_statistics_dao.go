package apg

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
)

type IMatchStatisticsDao interface {
	GetByMatchId(ctx context.Context, matchId int64) ([]*entity.ApgMatchStatistics, error)
	BatchCreate(ctx context.Context, statistics []*entity.ApgMatchStatistics) error
	DeleteByMatchId(ctx context.Context, matchId int64) (int64, error)
}

type matchStatisticsDao struct{}

var MatchStatistics IMatchStatisticsDao = &matchStatisticsDao{}

func (d *matchStatisticsDao) GetByMatchId(ctx context.Context, matchId int64) ([]*entity.ApgMatchStatistics, error) {
	var statistics []*entity.ApgMatchStatistics
	err := db.GetDB().Model("apg_mint_match_statistics").
		Where("match_id", matchId).
		Scan(&statistics)
	return statistics, err
}

func (d *matchStatisticsDao) BatchCreate(ctx context.Context, statistics []*entity.ApgMatchStatistics) error {
	if len(statistics) == 0 {
		return nil
	}
	_, err := db.GetDB().Model("apg_mint_match_statistics").
		Data(statistics).
		FieldsEx("id").
		Insert()
	return err
}

func (d *matchStatisticsDao) DeleteByMatchId(ctx context.Context, matchId int64) (int64, error) {
	result, err := db.GetDB().Model("apg_mint_match_statistics").
		Where("match_id", matchId).
		Delete()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
