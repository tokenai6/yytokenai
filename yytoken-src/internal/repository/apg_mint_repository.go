package repository

import (
	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"context"
	"fmt"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
)

// ApgMintRepository APG提取记录仓储
type ApgMintRepository struct {
	dao *dao.ApgMintRecordDao
}

// NewApgMintRepository 创建APG提取记录仓储
func NewApgMintRepository() *ApgMintRepository {
	return &ApgMintRepository{
		dao: dao.NewApgMintRecordDao(),
	}
}

// CreateMintRecord 创建APG提取记录
func (r *ApgMintRepository) CreateMintRecord(ctx context.Context, record *entity.ApgMintRecord) error {
	err := r.dao.Insert(ctx, record)
	if err != nil {
		return fmt.Errorf("创建APG提取记录失败: %v", err)
	}

	g.Log().Infof(ctx, "创建APG提取记录成功: 用户 %d, 金额 %s, 交易哈希 %s",
		record.UserId, record.Amount.String(), record.TxHash)
	return nil
}

// CreateMintRecordWithTx 在事务中创建APG提取记录
func (r *ApgMintRepository) CreateMintRecordWithTx(ctx context.Context, tx gdb.TX, record *entity.ApgMintRecord) error {
	err := r.dao.InsertWithTx(ctx, tx, record)
	if err != nil {
		return fmt.Errorf("创建APG提取记录失败: %v", err)
	}

	g.Log().Infof(ctx, "创建APG提取记录成功: 用户 %d, 金额 %s, 交易哈希 %s",
		record.UserId, record.Amount.String(), record.TxHash)
	return nil
}

// CheckEventProcessed 检查事件是否已处理
func (r *ApgMintRepository) CheckEventProcessed(ctx context.Context, txHash string, eventIndex int) (bool, error) {
	exists, err := r.dao.ExistsByTxHash(ctx, txHash, eventIndex)
	if err != nil {
		return false, fmt.Errorf("检查事件处理状态失败: %v", err)
	}
	return exists, nil
}

// GetMintRecordByTxHash 根据交易哈希获取提取记录
func (r *ApgMintRepository) GetMintRecordByTxHash(ctx context.Context, txHash string, eventIndex int) (*entity.ApgMintRecord, error) {
	record, err := r.dao.GetByTxHash(ctx, txHash, eventIndex)
	if err != nil {
		return nil, fmt.Errorf("查询APG提取记录失败: %v", err)
	}
	return record, nil
}

// GetUserMintRecords 获取用户APG提取记录
func (r *ApgMintRepository) GetUserMintRecords(ctx context.Context, userID int64, limit int) ([]*entity.ApgMintRecord, error) {
	records, err := r.dao.GetByUserId(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("查询用户APG提取记录失败: %v", err)
	}
	return records, nil
}
