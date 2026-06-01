package dao

import (
	"XWFrame/internal/entity"
	"XWFrame/internal/frame/db"
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

// INodeDividendUsdtTransferDao 节点分红USDT转账记录数据访问接口
type INodeDividendUsdtTransferDao interface {
	Insert(ctx context.Context, record *entity.NodeDividendUsdtTransfer) error
	InsertWithTx(ctx context.Context, tx gdb.TX, record *entity.NodeDividendUsdtTransfer) error

	GetByTxHash(ctx context.Context, txHash string, eventIndex int) (*entity.NodeDividendUsdtTransfer, error)
	ExistsByTxHash(ctx context.Context, txHash string, eventIndex int) (bool, error)

	SumAmountByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error)
	GetByDateRange(ctx context.Context, startTime, endTime time.Time) ([]*entity.NodeDividendUsdtTransfer, error)
	CountByDateRange(ctx context.Context, startTime, endTime time.Time) (int64, error)
}

// NodeDividendUsdtTransferDao 节点分红USDT转账记录DAO
type NodeDividendUsdtTransferDao struct {
	table string
	db    gdb.DB
}

// NewNodeDividendUsdtTransferDao 创建节点分红USDT转账记录DAO
func NewNodeDividendUsdtTransferDao() *NodeDividendUsdtTransferDao {
	return &NodeDividendUsdtTransferDao{
		table: "node_dividend_usdt_transfer",
		db:    db.GetDB(),
	}
}

// Insert 插入记录
func (d *NodeDividendUsdtTransferDao) Insert(ctx context.Context, record *entity.NodeDividendUsdtTransfer) error {
	_, err := d.db.Model(d.table).Ctx(ctx).FieldsEx("id").Insert(record)
	return err
}

// InsertWithTx 在事务中插入记录
func (d *NodeDividendUsdtTransferDao) InsertWithTx(ctx context.Context, tx gdb.TX, record *entity.NodeDividendUsdtTransfer) error {
	_, err := tx.Model(d.table).Ctx(ctx).FieldsEx("id").Insert(record)
	return err
}

// GetByTxHash 根据交易哈希和事件索引查询记录
func (d *NodeDividendUsdtTransferDao) GetByTxHash(ctx context.Context, txHash string, eventIndex int) (*entity.NodeDividendUsdtTransfer, error) {
	var record entity.NodeDividendUsdtTransfer
	err := d.db.Model(d.table).Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Where("event_index = ?", eventIndex).
		Scan(&record)

	if err != nil {
		return nil, err
	}

	return &record, nil
}

// ExistsByTxHash 检查交易是否已处理
func (d *NodeDividendUsdtTransferDao) ExistsByTxHash(ctx context.Context, txHash string, eventIndex int) (bool, error) {
	count, err := d.db.Model(d.table).Ctx(ctx).
		Where("tx_hash = ?", txHash).
		Where("event_index = ?", eventIndex).
		Count()

	return count > 0, err
}

// SumAmountByDateRange 统计时间范围内的转账总额
func (d *NodeDividendUsdtTransferDao) SumAmountByDateRange(ctx context.Context, startTime, endTime time.Time) (decimal.Decimal, error) {
	var result struct {
		Total decimal.Decimal `json:"total"`
	}

	err := d.db.Model(d.table).Ctx(ctx).
		Fields("COALESCE(SUM(amount), 0) as total").
		Where("block_timestamp >= ?", startTime).
		Where("block_timestamp < ?", endTime).
		Scan(&result)

	if err != nil {
		return decimal.Zero, err
	}

	return result.Total, nil
}

// GetByDateRange 获取时间范围内的转账记录
func (d *NodeDividendUsdtTransferDao) GetByDateRange(ctx context.Context, startTime, endTime time.Time) ([]*entity.NodeDividendUsdtTransfer, error) {
	var records []*entity.NodeDividendUsdtTransfer
	err := d.db.Model(d.table).Ctx(ctx).
		Where("block_timestamp >= ?", startTime).
		Where("block_timestamp < ?", endTime).
		Order("block_timestamp ASC").
		Scan(&records)

	return records, err
}

// CountByDateRange 统计时间范围内的转账记录数量
func (d *NodeDividendUsdtTransferDao) CountByDateRange(ctx context.Context, startTime, endTime time.Time) (int64, error) {
	count, err := d.db.Model(d.table).Ctx(ctx).
		Where("block_timestamp >= ?", startTime).
		Where("block_timestamp < ?", endTime).
		Count()

	return int64(count), err
}
