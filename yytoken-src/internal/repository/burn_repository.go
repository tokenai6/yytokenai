package repository

import (
	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// IBurnRepository 销毁记录仓储接口
type IBurnRepository interface {
	// 插入销毁记录
	InsertBurnRecord(ctx context.Context, record *entity.ApgBurnRecord) error
	InsertBurnRecordWithTx(ctx context.Context, tx gdb.TX, record *entity.ApgBurnRecord) error

	// 查询销毁记录
	GetBurnRecordByTxHash(ctx context.Context, txHash string, eventIndex int) (*entity.ApgBurnRecord, error)
	ExistsBurnRecordByTxHash(ctx context.Context, txHash string, eventIndex int) (bool, error)
	GetBurnRecordsByFromAddress(ctx context.Context, fromAddress string, limit int) ([]*entity.ApgBurnRecord, error)

	// 统计查询
	SumBurnAmountByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error)
	GetBurnRecordsByDateRange(ctx context.Context, startTime, endTime time.Time) ([]*entity.ApgBurnRecord, error)
}

// burnRepository 销毁记录仓储实现
type burnRepository struct {
	burnDao dao.IApgBurnRecordDao
}

// NewBurnRepository 创建销毁记录仓储实例
func NewBurnRepository() IBurnRepository {
	return &burnRepository{
		burnDao: dao.NewApgBurnRecordDao(),
	}
}

// InsertBurnRecord 插入销毁记录
func (r *burnRepository) InsertBurnRecord(ctx context.Context, record *entity.ApgBurnRecord) error {
	return r.burnDao.Insert(ctx, record)
}

// InsertBurnRecordWithTx 在事务中插入销毁记录
func (r *burnRepository) InsertBurnRecordWithTx(ctx context.Context, tx gdb.TX, record *entity.ApgBurnRecord) error {
	return r.burnDao.InsertWithTx(ctx, tx, record)
}

// GetBurnRecordByTxHash 根据交易哈希和事件索引查询记录
func (r *burnRepository) GetBurnRecordByTxHash(ctx context.Context, txHash string, eventIndex int) (*entity.ApgBurnRecord, error) {
	return r.burnDao.GetByTxHash(ctx, txHash, eventIndex)
}

// ExistsBurnRecordByTxHash 检查交易是否已处理
func (r *burnRepository) ExistsBurnRecordByTxHash(ctx context.Context, txHash string, eventIndex int) (bool, error) {
	return r.burnDao.ExistsByTxHash(ctx, txHash, eventIndex)
}

// GetBurnRecordsByFromAddress 根据发起者地址查询记录列表
func (r *burnRepository) GetBurnRecordsByFromAddress(ctx context.Context, fromAddress string, limit int) ([]*entity.ApgBurnRecord, error) {
	return r.burnDao.GetByFromAddress(ctx, fromAddress, limit)
}

// SumBurnAmountByDateRange 统计时间范围内的销毁总量
func (r *burnRepository) SumBurnAmountByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error) {
	return r.burnDao.SumBurnAmountByDateRange(ctx, startTime, endTime)
}

// GetBurnRecordsByDateRange 获取时间范围内的销毁记录
func (r *burnRepository) GetBurnRecordsByDateRange(ctx context.Context, startTime, endTime time.Time) ([]*entity.ApgBurnRecord, error) {
	return r.burnDao.GetByDateRange(ctx, startTime, endTime)
}
