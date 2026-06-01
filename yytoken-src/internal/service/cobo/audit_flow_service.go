package cobo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"XWFrame/internal/frame/config"
	externalCobo "XWFrame/pkg/external/cobo"

	coboWaas2 "github.com/CoboGlobal/cobo-waas2-go-sdk/cobo_waas2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

const (
	auditCoboDefaultTokenIDs = "BSC_USDT"
	auditCoboDefaultLimit    = int32(50)
	auditCoboLookbackMillis  = int64(24 * time.Hour / time.Millisecond)
)

type AuditCoboFlowSyncOptions struct {
	WalletID string
	TokenIDs string
	Limit    int32
	MaxPages int
}

type AuditCoboFlowSyncResult struct {
	WalletID          string
	TokenIDs          string
	DepositProcessed  int
	WithdrawProcessed int
	DepositPages      int
	WithdrawPages     int
}

type auditCoboFlowRecord struct {
	TransactionID    string
	CoboID           string
	RequestID        string
	WalletID         string
	ChainID          string
	TokenID          string
	Status           string
	TxHash           string
	Amount           decimal.Decimal
	FromAddress      string
	ToAddress        string
	SourceType       string
	DestinationType  string
	ConfirmedNum     int32
	CreatedTimestamp int64
	UpdatedTimestamp int64
	RawJSON          string
}

func SyncAuditCoboFlow(ctx context.Context, opts AuditCoboFlowSyncOptions) (*AuditCoboFlowSyncResult, error) {
	coboCfg, err := config.GetCoboConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("读取 Cobo 配置失败: %w", err)
	}
	if strings.TrimSpace(coboCfg.APISecret) == "" {
		return nil, fmt.Errorf("Cobo API Secret 未配置")
	}

	walletID := strings.TrimSpace(opts.WalletID)
	if walletID == "" {
		walletID = strings.TrimSpace(coboCfg.WalletID)
	}
	if walletID == "" {
		return nil, fmt.Errorf("Cobo wallet_id 未配置")
	}

	tokenIDs := strings.TrimSpace(opts.TokenIDs)
	if tokenIDs == "" {
		tokenIDs = auditCoboDefaultTokenIDs
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = auditCoboDefaultLimit
	}
	if limit > auditCoboDefaultLimit {
		limit = auditCoboDefaultLimit
	}
	g.Log().Infof(ctx, "[Cobo审计流水同步] config env=%s wallet_id=%s token_ids=%s limit=%d timeout=%d api_secret=%s",
		coboCfg.Env,
		walletID,
		tokenIDs,
		limit,
		coboCfg.Timeout,
		maskedSecretFingerprint(coboCfg.APISecret),
	)

	timeout := coboCfg.Timeout
	if timeout <= 0 {
		timeout = 60
	}
	client, err := externalCobo.NewCoboClient(&externalCobo.Config{
		APISecret: coboCfg.APISecret,
		Env:       coboCfg.Env,
		Timeout:   timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 Cobo 客户端失败: %w", err)
	}

	result := &AuditCoboFlowSyncResult{WalletID: walletID, TokenIDs: tokenIDs}
	depositProcessed, depositPages, err := syncAuditCoboFlowType(ctx, client, walletID, tokenIDs, "Deposit", "audit_deposit", limit, opts.MaxPages)
	if err != nil {
		return nil, err
	}
	withdrawProcessed, withdrawPages, err := syncAuditCoboFlowType(ctx, client, walletID, tokenIDs, "Withdrawal", "audit_withdraw", limit, opts.MaxPages)
	if err != nil {
		return nil, err
	}

	result.DepositProcessed = depositProcessed
	result.DepositPages = depositPages
	result.WithdrawProcessed = withdrawProcessed
	result.WithdrawPages = withdrawPages
	return result, nil
}

func syncAuditCoboFlowType(ctx context.Context, client *externalCobo.CoboClient, walletID, tokenIDs, txType, table string, limit int32, maxPages int) (int, int, error) {
	after := ""
	processed := 0
	pages := 0
	minCreated, err := getAuditCoboFlowMinCreatedTimestamp(ctx, table)
	if err != nil {
		return 0, 0, err
	}

	for {
		if maxPages > 0 && pages >= maxPages {
			break
		}

		resp, err := client.ListTransactions(ctx, &externalCobo.ListTransactionsRequest{
			WalletIDs:           walletID,
			Types:               txType,
			TokenIDs:            tokenIDs,
			Limit:               limit,
			After:               after,
			MinCreatedTimestamp: minCreated,
		})
		if err != nil {
			return processed, pages, fmt.Errorf("同步 Cobo %s 失败: %w", txType, err)
		}
		pages++

		data := resp.GetData()
		if len(data) == 0 {
			break
		}

		for _, tx := range data {
			record := buildAuditCoboFlowRecord(tx)
			if record.TransactionID == "" || !strings.Contains(strings.ToUpper(record.TokenID), "USDT") {
				continue
			}
			if err := upsertAuditCoboFlowRecord(ctx, table, record); err != nil {
				return processed, pages, err
			}
			processed++
		}

		if resp.Pagination == nil {
			break
		}
		nextAfter := strings.TrimSpace(resp.Pagination.After)
		if nextAfter == "" || nextAfter == after {
			break
		}
		after = nextAfter
	}

	return processed, pages, nil
}

func getAuditCoboFlowMinCreatedTimestamp(ctx context.Context, table string) (int64, error) {
	val, err := g.DB().Ctx(ctx).Model(table).Fields("COALESCE(MAX(created_timestamp), 0)").Value()
	if err != nil {
		return 0, fmt.Errorf("查询 %s 最大 created_timestamp 失败: %w", table, err)
	}
	maxCreated := val.Int64()
	if maxCreated <= auditCoboLookbackMillis {
		return 0, nil
	}
	return maxCreated - auditCoboLookbackMillis, nil
}

func buildAuditCoboFlowRecord(tx coboWaas2.Transaction) auditCoboFlowRecord {
	raw, _ := json.Marshal(tx)
	record := auditCoboFlowRecord{
		TransactionID:    strings.TrimSpace(tx.GetTransactionId()),
		CoboID:           strings.TrimSpace(tx.GetCoboId()),
		RequestID:        strings.TrimSpace(tx.GetRequestId()),
		WalletID:         strings.TrimSpace(tx.GetWalletId()),
		ChainID:          strings.TrimSpace(tx.GetChainId()),
		TokenID:          strings.ToUpper(strings.TrimSpace(tx.GetTokenId())),
		Status:           string(tx.GetStatus()),
		TxHash:           strings.TrimSpace(tx.GetTransactionHash()),
		ConfirmedNum:     tx.GetConfirmedNum(),
		CreatedTimestamp: tx.CreatedTimestamp,
		UpdatedTimestamp: tx.UpdatedTimestamp,
		RawJSON:          string(raw),
	}
	record.SourceType, record.FromAddress = auditCoboSource(tx.GetSource())
	record.DestinationType, record.ToAddress, record.Amount = auditCoboDestination(tx.GetDestination())
	return record
}

func auditCoboSource(source coboWaas2.TransactionSource) (string, string) {
	if source.TransactionDepositFromAddressSource != nil {
		return string(source.TransactionDepositFromAddressSource.GetSourceType()), strings.ToLower(strings.Join(source.TransactionDepositFromAddressSource.GetAddresses(), ","))
	}
	if source.TransactionCustodialAssetWalletSource != nil {
		return string(source.TransactionCustodialAssetWalletSource.GetSourceType()), source.TransactionCustodialAssetWalletSource.GetWalletId()
	}
	if source.TransactionCustodialWeb3WalletSource != nil {
		return string(source.TransactionCustodialWeb3WalletSource.GetSourceType()), source.TransactionCustodialWeb3WalletSource.GetWalletId()
	}
	if source.TransactionDepositFromWalletSource != nil {
		return string(source.TransactionDepositFromWalletSource.GetSourceType()), source.TransactionDepositFromWalletSource.GetWalletId()
	}
	if source.TransactionDepositFromLoopSource != nil {
		return string(source.TransactionDepositFromLoopSource.GetSourceType()), ""
	}
	if source.TransactionExchangeWalletSource != nil {
		return string(source.TransactionExchangeWalletSource.GetSourceType()), source.TransactionExchangeWalletSource.GetWalletId()
	}
	if source.TransactionMPCWalletSource != nil {
		return string(source.TransactionMPCWalletSource.GetSourceType()), source.TransactionMPCWalletSource.GetWalletId()
	}
	if source.TransactionSmartContractSafeWalletSource != nil {
		return string(source.TransactionSmartContractSafeWalletSource.GetSourceType()), source.TransactionSmartContractSafeWalletSource.GetWalletId()
	}
	return "", ""
}

func auditCoboDestination(destination coboWaas2.TransactionDestination) (string, string, decimal.Decimal) {
	if destination.TransactionDepositToAddressDestination != nil {
		dst := destination.TransactionDepositToAddressDestination
		return string(dst.GetDestinationType()), strings.ToLower(strings.TrimSpace(dst.GetAddress())), decimalFromString(dst.GetAmount())
	}
	if destination.TransactionDepositToWalletDestination != nil {
		dst := destination.TransactionDepositToWalletDestination
		return string(dst.GetDestinationType()), dst.GetWalletId(), decimalFromString(dst.GetAmount())
	}
	if destination.TransactionTransferToAddressDestination != nil {
		dst := destination.TransactionTransferToAddressDestination
		if dst.AccountOutput != nil {
			return string(dst.GetDestinationType()), strings.ToLower(strings.TrimSpace(dst.AccountOutput.GetAddress())), decimalFromString(dst.AccountOutput.GetAmount())
		}
		return string(dst.GetDestinationType()), "", decimal.Zero
	}
	if destination.TransactionTransferToWalletDestination != nil {
		dst := destination.TransactionTransferToWalletDestination
		return string(dst.GetDestinationType()), dst.GetWalletId(), decimalFromString(dst.GetAmount())
	}
	if destination.TransactionEvmContractDestination != nil {
		dst := destination.TransactionEvmContractDestination
		return string(dst.GetDestinationType()), strings.ToLower(strings.TrimSpace(dst.GetAddress())), decimal.Zero
	}
	return "", "", decimal.Zero
}

func decimalFromString(s string) decimal.Decimal {
	amount, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil {
		return decimal.Zero
	}
	return amount
}

func maskedSecretFingerprint(secret string) string {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "empty"
	}
	sum := sha256.Sum256([]byte(secret))
	return fmt.Sprintf("len=%d sha256=%s", len(secret), hex.EncodeToString(sum[:])[:12])
}

func upsertAuditCoboFlowRecord(ctx context.Context, table string, record auditCoboFlowRecord) error {
	_, err := g.DB().Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s (
			transaction_id, cobo_id, request_id, wallet_id, chain_id, token_id, status, tx_hash,
			amount, from_address, to_address, source_type, destination_type, confirmed_num,
			created_timestamp, updated_timestamp, raw_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?::jsonb, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (transaction_id) DO UPDATE SET
			cobo_id = EXCLUDED.cobo_id,
			request_id = EXCLUDED.request_id,
			wallet_id = EXCLUDED.wallet_id,
			chain_id = EXCLUDED.chain_id,
			token_id = EXCLUDED.token_id,
			status = EXCLUDED.status,
			tx_hash = EXCLUDED.tx_hash,
			amount = EXCLUDED.amount,
			from_address = EXCLUDED.from_address,
			to_address = EXCLUDED.to_address,
			source_type = EXCLUDED.source_type,
			destination_type = EXCLUDED.destination_type,
			confirmed_num = EXCLUDED.confirmed_num,
			created_timestamp = EXCLUDED.created_timestamp,
			updated_timestamp = EXCLUDED.updated_timestamp,
			raw_json = EXCLUDED.raw_json,
			updated_at = CURRENT_TIMESTAMP
	`, table),
		record.TransactionID,
		record.CoboID,
		record.RequestID,
		record.WalletID,
		record.ChainID,
		record.TokenID,
		record.Status,
		record.TxHash,
		record.Amount,
		record.FromAddress,
		record.ToAddress,
		record.SourceType,
		record.DestinationType,
		record.ConfirmedNum,
		record.CreatedTimestamp,
		record.UpdatedTimestamp,
		record.RawJSON,
	)
	if err != nil {
		return fmt.Errorf("写入 %s 失败 transaction_id=%s: %w", table, record.TransactionID, err)
	}
	return nil
}
