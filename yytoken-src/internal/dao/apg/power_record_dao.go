package apg

import (
	"context"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

type IPowerRecordDao interface {
	Create(ctx context.Context, record *entity.ApgPowerRecord) error
	BatchCreate(ctx context.Context, tx gdb.TX, records []*entity.ApgPowerRecord) error
	GetUserRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPowerRecord, int, error)
	GetUserTotalPower(ctx context.Context, userId int64) (map[string]string, error)
	DeleteByMatchId(ctx context.Context, matchId int64) (int64, error)
}

type powerRecordDao struct{}

var PowerRecord IPowerRecordDao = &powerRecordDao{}

func (d *powerRecordDao) Create(ctx context.Context, record *entity.ApgPowerRecord) error {
	_, err := db.GetDB().Model("apg_mint_power_record").Insert(map[string]interface{}{
		"user_id":         record.UserId,
		"source_type":     record.SourceType,
		"source_id":       record.SourceId,
		"purchase_token":  record.PurchaseToken,
		"purchase_amount": record.PurchaseAmount,
		"power_amount":    record.PowerAmount,
		"conversion_rate": record.ConversionRate,
		"notes":           record.Notes,
		"created_at":      record.CreatedAt,
	})
	return err
}

func (d *powerRecordDao) BatchCreate(ctx context.Context, tx gdb.TX, records []*entity.ApgPowerRecord) error {
	if len(records) == 0 {
		return nil
	}
	_, err := tx.Model("apg_mint_power_record").OmitEmptyData().Insert(records)
	return err
}

func (d *powerRecordDao) GetUserRecords(ctx context.Context, userId int64, page, pageSize int) ([]*entity.ApgPowerRecord, int, error) {
	model := db.GetDB().Model("apg_mint_power_record").Where("user_id", userId)

	total, err := model.Count()
	if err != nil {
		return nil, 0, err
	}

	var records []*entity.ApgPowerRecord
	err = model.Order("created_at DESC").
		Page(page, pageSize).
		Scan(&records)

	return records, total, err
}

func (d *powerRecordDao) GetUserTotalPower(ctx context.Context, userId int64) (map[string]string, error) {
	var result map[string]string
	err := db.GetDB().Model("apg_mint_power_record").
		Where("user_id", userId).
		Fields(
			"purchase_token",
			"SUM(power_amount) as total_power",
		).
		Group("purchase_token").
		Scan(&result)
	return result, err
}

func (d *powerRecordDao) DeleteByMatchId(ctx context.Context, matchId int64) (int64, error) {
	result, err := db.GetDB().Model("apg_mint_power_record").
		Where("source_type", "mint_game").
		Where("source_id IN (SELECT id FROM apg_mint_player WHERE match_id = ?)", matchId).
		Delete()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
