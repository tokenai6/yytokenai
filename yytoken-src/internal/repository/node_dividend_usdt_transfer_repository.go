package repository

import (
	"XWFrame/internal/dao"
	"XWFrame/internal/entity"
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// INodeDividendUsdtTransferRepository 节点分红USDT转账记录仓储接口
type INodeDividendUsdtTransferRepository interface {
	InsertTransferRecord(ctx context.Context, record *entity.NodeDividendUsdtTransfer) error
	InsertTransferRecordWithTx(ctx context.Context, tx gdb.TX, record *entity.NodeDividendUsdtTransfer) error

	GetTransferRecordByTxHash(ctx context.Context, txHash string, eventIndex int) (*entity.NodeDividendUsdtTransfer, error)
	ExistsTransferRecordByTxHash(ctx context.Context, txHash string, eventIndex int) (bool, error)

	SumTransferAmountByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error)
	GetTransferRecordsByDateRange(ctx context.Context, startTime, endTime time.Time) ([]*entity.NodeDividendUsdtTransfer, error)
	CountTransferRecordsByDateRange(ctx context.Context, startTime, endTime time.Time) (int64, error)
}

// nodeDividendUsdtTransferRepository 节点分红USDT转账记录仓储实现
type nodeDividendUsdtTransferRepository struct {
	transferDao dao.INodeDividendUsdtTransferDao
}

// NewNodeDividendUsdtTransferRepository 创建节点分红USDT转账记录仓储实例
func NewNodeDividendUsdtTransferRepository() INodeDividendUsdtTransferRepository {
	return &nodeDividendUsdtTransferRepository{
		transferDao: dao.NewNodeDividendUsdtTransferDao(),
	}
}

// InsertTransferRecord 插入转账记录
func (r *nodeDividendUsdtTransferRepository) InsertTransferRecord(ctx context.Context, record *entity.NodeDividendUsdtTransfer) error {
	return r.transferDao.Insert(ctx, record)
}

// InsertTransferRecordWithTx 在事务中插入转账记录
func (r *nodeDividendUsdtTransferRepository) InsertTransferRecordWithTx(ctx context.Context, tx gdb.TX, record *entity.NodeDividendUsdtTransfer) error {
	return r.transferDao.InsertWithTx(ctx, tx, record)
}

// GetTransferRecordByTxHash 根据交易哈希和事件索引查询记录
func (r *nodeDividendUsdtTransferRepository) GetTransferRecordByTxHash(ctx context.Context, txHash string, eventIndex int) (*entity.NodeDividendUsdtTransfer, error) {
	return r.transferDao.GetByTxHash(ctx, txHash, eventIndex)
}

// ExistsTransferRecordByTxHash 检查交易是否已处理
func (r *nodeDividendUsdtTransferRepository) ExistsTransferRecordByTxHash(ctx context.Context, txHash string, eventIndex int) (bool, error) {
	return r.transferDao.ExistsByTxHash(ctx, txHash, eventIndex)
}

// SumTransferAmountByDateRange 统计时间范围内的转账总额
func (r *nodeDividendUsdtTransferRepository) SumTransferAmountByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error) {
	return r.transferDao.SumAmountByDateRange(ctx, startTime, endTime)
}

// GetTransferRecordsByDateRange 获取时间范围内的转账记录
func (r *nodeDividendUsdtTransferRepository) GetTransferRecordsByDateRange(ctx context.Context, startTime, endTime time.Time) ([]*entity.NodeDividendUsdtTransfer, error) {
	return r.transferDao.GetByDateRange(ctx, startTime, endTime)
}

// CountTransferRecordsByDateRange 统计时间范围内的转账记录数量
func (r *nodeDividendUsdtTransferRepository) CountTransferRecordsByDateRange(ctx context.Context, startTime, endTime time.Time) (int64, error) {
	return r.transferDao.CountByDateRange(ctx, startTime, endTime)
}
