package entity

import (
	"time"

	"github.com/shopspring/decimal"
)

// ApgMintRecord APG提取记录实体
type ApgMintRecord struct {
	Id             int64           `json:"id" orm:"id,primary"`                   // 主键ID
	UserId         int64           `json:"user_id" orm:"user_id"`                 // 用户ID
	WalletAddress  string          `json:"wallet_address" orm:"wallet_address"`   // 钱包地址
	Amount         decimal.Decimal `json:"amount" orm:"amount"`                   // APG数量
	Nonce          int64           `json:"nonce" orm:"nonce"`                     // 链上nonce（防重放）
	SignerAddress  string          `json:"signer_address" orm:"signer_address"`   // 签名者地址
	TxHash         string          `json:"tx_hash" orm:"tx_hash"`                 // 交易哈希
	BlockNumber    int64           `json:"block_number" orm:"block_number"`       // 区块号
	EventIndex     int             `json:"event_index" orm:"event_index"`         // 事件索引
	BlockTimestamp time.Time       `json:"block_timestamp" orm:"block_timestamp"` // 区块时间
	CreatedAt      time.Time       `json:"created_at" orm:"created_at"`           // 创建时间
}
