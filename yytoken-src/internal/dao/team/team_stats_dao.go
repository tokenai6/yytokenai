package team

import (
	"context"

	"XWFrame/internal/entity/team"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/shopspring/decimal"
)

type Team24hStats struct {
	TeamId         int64           `json:"team_id"`
	StakeAmount    decimal.Decimal `json:"stake_amount"`
	WithdrawAmount decimal.Decimal `json:"withdraw_amount"`
}

type TeamPerformanceStats struct {
	TeamPerformance     decimal.Decimal `json:"team_performance"`
	PersonalPerformance decimal.Decimal `json:"personal_performance"`
	DirectCount         int             `json:"direct_count"`
	TeamTotalCount      int             `json:"team_total_count"`
}

type ITeamStatsDao interface {
	Upsert(ctx context.Context, tx gdb.TX, stats *team.TeamStatsEntity) error
	GetByTeamId(ctx context.Context, teamId int64) (*team.TeamStatsEntity, error)
	GetAllStats(ctx context.Context) ([]*team.TeamStatsEntity, error)
	GetStakeByTeamIdHours(ctx context.Context, teamId int64, hours int) (decimal.Decimal, int, error)
	GetWithdrawByTeamIdHours(ctx context.Context, teamId int64, hours int) (decimal.Decimal, int, error)
	GetStakeAndWithdraw24hByTeamIds(ctx context.Context, teamIds []int64) (map[int64]*Team24hStats, error)
	GetMemberCountByTeamId(ctx context.Context, teamId int64) (int, error)
	GetChildTeamIds(ctx context.Context, parentId int64) ([]int64, error)
	GetStakeDetailsByTeamIdHours(ctx context.Context, teamId int64, hours, page, pageSize int) ([]*team.StakeDetailItem, int, error)
	GetWithdrawDetailsByTeamIdHours(ctx context.Context, teamId int64, hours, page, pageSize int) ([]*team.WithdrawDetailItem, int, error)
	GetStakeDailyByTeamId(ctx context.Context, teamId int64, days int) ([]*team.DailyTrendItem, error)
	GetWithdrawDailyByTeamId(ctx context.Context, teamId int64, days int) ([]*team.DailyTrendItem, error)
	GetStakeSummaryByTeamId(ctx context.Context, teamId int64, days int) (decimal.Decimal, error)
	GetWithdrawSummaryByTeamId(ctx context.Context, teamId int64, days int) (decimal.Decimal, error)
	GetPerformanceByTeamId(ctx context.Context, teamId int64) (*TeamPerformanceStats, error)
}

type teamStatsDao struct {
	db gdb.DB
}

const teamTreeCTE = `
	WITH RECURSIVE root_user AS (
		SELECT u.id, u.invite_code
		FROM team t
		JOIN user_info u ON LOWER(u.wallet_address) = LOWER(t.leader_wallet_address)
		WHERE t.id = ?
		LIMIT 1
	),
	team_tree AS (
		SELECT ru.id, ru.invite_code
		FROM root_user ru
		UNION ALL
		SELECT c.id, c.invite_code
		FROM user_info c
		INNER JOIN team_tree tt ON c.parent_invite_code = tt.invite_code
	)
`

func NewTeamStatsDao() ITeamStatsDao {
	return &teamStatsDao{db: db.GetDB()}
}

func (d *teamStatsDao) Upsert(ctx context.Context, tx gdb.TX, stats *team.TeamStatsEntity) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		count, err := tx.Model("team_stats").Where("team_id", stats.TeamID).Count()
		if err != nil {
			return err
		}

		data := map[string]interface{}{
			"team_id":             stats.TeamID,
			"stake_amount_48h":    stats.StakeAmount48h,
			"withdraw_amount_48h": stats.WithdrawAmount48h,
			"stake_count_48h":     stats.StakeCount48h,
			"withdraw_count_48h":  stats.WithdrawCount48h,
			"total_member_count":  stats.TotalMemberCount,
		}

		if count > 0 {
			_, err = tx.Model("team_stats").Where("team_id", stats.TeamID).Data(data).Update()
		} else {
			_, err = tx.Model("team_stats").Data(data).Insert()
		}
		return err
	})
}

func (d *teamStatsDao) GetByTeamId(ctx context.Context, teamId int64) (*team.TeamStatsEntity, error) {
	var entity team.TeamStatsEntity
	err := d.db.Ctx(ctx).Model("team_stats").Where("team_id", teamId).Scan(&entity)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	if entity.Id == 0 {
		return nil, nil
	}
	return &entity, nil
}

func (d *teamStatsDao) GetAllStats(ctx context.Context) ([]*team.TeamStatsEntity, error) {
	var list []*team.TeamStatsEntity
	err := d.db.Ctx(ctx).Model("team_stats").Scan(&list)
	return list, err
}

func (d *teamStatsDao) GetStakeByTeamIdHours(ctx context.Context, teamId int64, hours int) (decimal.Decimal, int, error) {
	sql := teamTreeCTE + `
		SELECT
			COALESCE(SUM(crr.amount), 0) as total_amount,
			COUNT(crr.id) as total_count
		FROM cobo_recharge_record crr
		JOIN team_tree tt ON crr.user_id = tt.id
		AND crr.status = 1
	`
	args := []interface{}{teamId}
	if hours > 0 {
		sql = teamTreeCTE + `
		SELECT
			COALESCE(SUM(crr.amount), 0) as total_amount,
			COUNT(crr.id) as total_count
		FROM cobo_recharge_record crr
		JOIN team_tree tt ON crr.user_id = tt.id
		AND crr.created_at >= NOW() - INTERVAL '1 hour' * ?
		AND crr.status = 1
	`
		args = []interface{}{teamId, hours}
	}

	record, err := d.db.Ctx(ctx).Raw(sql, args...).One()

	if err != nil {
		return decimal.Zero, 0, err
	}
	if record.IsEmpty() {
		return decimal.Zero, 0, nil
	}

	amount, _ := decimal.NewFromString(record["total_amount"].String())
	count := record["total_count"].Int()
	return amount, count, nil
}

func (d *teamStatsDao) GetWithdrawByTeamIdHours(ctx context.Context, teamId int64, hours int) (decimal.Decimal, int, error) {
	sql := teamTreeCTE + `
		SELECT
			COALESCE(SUM(cwr.amount), 0) as total_amount,
			COUNT(cwr.id) as total_count
		FROM cobo_withdraw_request cwr
		JOIN team_tree tt ON cwr.user_id = tt.id
		AND cwr.status NOT IN (2, 5)
	`
	args := []interface{}{teamId}
	if hours > 0 {
		sql = teamTreeCTE + `
		SELECT
			COALESCE(SUM(cwr.amount), 0) as total_amount,
			COUNT(cwr.id) as total_count
		FROM cobo_withdraw_request cwr
		JOIN team_tree tt ON cwr.user_id = tt.id
		AND cwr.created_at >= NOW() - INTERVAL '1 hour' * ?
		AND cwr.status NOT IN (2, 5)
	`
		args = []interface{}{teamId, hours}
	}

	record, err := d.db.Ctx(ctx).Raw(sql, args...).One()

	if err != nil {
		return decimal.Zero, 0, err
	}
	if record.IsEmpty() {
		return decimal.Zero, 0, nil
	}

	amount, _ := decimal.NewFromString(record["total_amount"].String())
	count := record["total_count"].Int()
	return amount, count, nil
}

func (d *teamStatsDao) GetStakeAndWithdraw24hByTeamIds(ctx context.Context, teamIds []int64) (map[int64]*Team24hStats, error) {
	if len(teamIds) == 0 {
		return make(map[int64]*Team24hStats), nil
	}

	result := make(map[int64]*Team24hStats)
	for _, id := range teamIds {
		result[id] = &Team24hStats{TeamId: id}
	}

	for _, teamId := range teamIds {
		stakeAmount, _, err := d.GetStakeByTeamIdHours(ctx, teamId, 24)
		if err != nil {
			return nil, err
		}
		withdrawAmount, _, err := d.GetWithdrawByTeamIdHours(ctx, teamId, 24)
		if err != nil {
			return nil, err
		}
		result[teamId].StakeAmount = stakeAmount
		result[teamId].WithdrawAmount = withdrawAmount
	}

	return result, nil
}

func (d *teamStatsDao) GetMemberCountByTeamId(ctx context.Context, teamId int64) (int, error) {
	record, err := d.db.Ctx(ctx).Raw(teamTreeCTE+`
		SELECT COUNT(*) as cnt
		FROM team_tree
	`, teamId).One()
	if err != nil {
		return 0, err
	}
	if record.IsEmpty() {
		return 0, nil
	}
	return record["cnt"].Int(), nil
}

func (d *teamStatsDao) GetChildTeamIds(ctx context.Context, parentId int64) ([]int64, error) {
	type idRow struct {
		Id int64 `json:"id"`
	}
	var rows []idRow
	err := d.db.Ctx(ctx).Raw(`
		WITH RECURSIVE root_user AS (
			SELECT u.id, u.invite_code
			FROM team t
			JOIN user_info u ON LOWER(u.wallet_address) = LOWER(t.leader_wallet_address)
			WHERE t.id = ?
			LIMIT 1
		),
		direct_users AS (
			SELECT u.team_id
			FROM user_info u
			JOIN root_user ru ON u.parent_invite_code = ru.invite_code
		)
		SELECT DISTINCT team_id AS id
		FROM direct_users
		WHERE team_id IS NOT NULL AND team_id <> ?
		ORDER BY team_id ASC
	`, parentId, parentId).Scan(&rows)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Id)
	}
	return ids, nil
}

func (d *teamStatsDao) GetStakeDetailsByTeamIdHours(ctx context.Context, teamId int64, hours, page, pageSize int) ([]*team.StakeDetailItem, int, error) {
	countSql := teamTreeCTE + `
		SELECT COUNT(*) as cnt
		FROM cobo_recharge_record crr
		JOIN team_tree tt ON crr.user_id = tt.id
		AND crr.status = 1
	`
	countArgs := []interface{}{teamId}
	if hours > 0 {
		countSql = teamTreeCTE + `
		SELECT COUNT(*) as cnt
		FROM cobo_recharge_record crr
		JOIN team_tree tt ON crr.user_id = tt.id
		AND crr.created_at >= NOW() - INTERVAL '1 hour' * ?
		AND crr.status = 1
	`
		countArgs = []interface{}{teamId, hours}
	}
	countRecord, err := d.db.Ctx(ctx).Raw(countSql, countArgs...).One()
	if err != nil {
		return nil, 0, err
	}
	total := countRecord["cnt"].Int()

	offset := (page - 1) * pageSize
	var list []*team.StakeDetailItem
	listSql := teamTreeCTE + `
		SELECT
			crr.id,
			crr.user_id,
			ui.wallet_address,
			crr.amount as stake_amount,
			crr.amount as stake_usdt,
			0 as stake_type,
			crr.tx_hash,
			TO_CHAR(crr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD HH24:MI:SS') as created_at
		FROM cobo_recharge_record crr
		JOIN user_info ui ON crr.user_id = ui.id
		JOIN team_tree tt ON crr.user_id = tt.id
		WHERE 1=1
		AND crr.status = 1
		ORDER BY crr.created_at DESC
		LIMIT ? OFFSET ?
	`
	listArgs := []interface{}{teamId, pageSize, offset}
	if hours > 0 {
		listSql = teamTreeCTE + `
		SELECT
			crr.id,
			crr.user_id,
			ui.wallet_address,
			crr.amount as stake_amount,
			crr.amount as stake_usdt,
			0 as stake_type,
			crr.tx_hash,
			TO_CHAR(crr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD HH24:MI:SS') as created_at
		FROM cobo_recharge_record crr
		JOIN user_info ui ON crr.user_id = ui.id
		JOIN team_tree tt ON crr.user_id = tt.id
		WHERE 1=1
		AND crr.created_at >= NOW() - INTERVAL '1 hour' * ?
		AND crr.status = 1
		ORDER BY crr.created_at DESC
		LIMIT ? OFFSET ?
	`
		listArgs = []interface{}{teamId, hours, pageSize, offset}
	}
	err = d.db.Ctx(ctx).Raw(listSql, listArgs...).Scan(&list)

	return list, total, err
}

func (d *teamStatsDao) GetWithdrawDetailsByTeamIdHours(ctx context.Context, teamId int64, hours, page, pageSize int) ([]*team.WithdrawDetailItem, int, error) {
	countSql := teamTreeCTE + `
		SELECT COUNT(*) as cnt
		FROM cobo_withdraw_request cwr
		JOIN team_tree tt ON cwr.user_id = tt.id
		AND cwr.status NOT IN (2, 5)
	`
	countArgs := []interface{}{teamId}
	if hours > 0 {
		countSql = teamTreeCTE + `
		SELECT COUNT(*) as cnt
		FROM cobo_withdraw_request cwr
		JOIN team_tree tt ON cwr.user_id = tt.id
		AND cwr.created_at >= NOW() - INTERVAL '1 hour' * ?
		AND cwr.status NOT IN (2, 5)
	`
		countArgs = []interface{}{teamId, hours}
	}
	countRecord, err := d.db.Ctx(ctx).Raw(countSql, countArgs...).One()
	if err != nil {
		return nil, 0, err
	}
	total := countRecord["cnt"].Int()

	offset := (page - 1) * pageSize
	var list []*team.WithdrawDetailItem
	listSql := teamTreeCTE + `
		SELECT
			cwr.id,
			cwr.user_id,
			ui.wallet_address,
			cwr.amount,
			cwr.fee_amount as fee,
			cwr.actual_amount,
			cwr.symbol,
			cwr.status::text as status,
			cwr.to_address as withdraw_address,
			cwr.tx_hash,
			TO_CHAR(cwr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD HH24:MI:SS') as created_at
		FROM cobo_withdraw_request cwr
		JOIN user_info ui ON cwr.user_id = ui.id
		JOIN team_tree tt ON cwr.user_id = tt.id
		WHERE 1=1
		AND cwr.status NOT IN (2, 5)
		ORDER BY cwr.created_at DESC
		LIMIT ? OFFSET ?
	`
	listArgs := []interface{}{teamId, pageSize, offset}
	if hours > 0 {
		listSql = teamTreeCTE + `
		SELECT
			cwr.id,
			cwr.user_id,
			ui.wallet_address,
			cwr.amount,
			cwr.fee_amount as fee,
			cwr.actual_amount,
			cwr.symbol,
			cwr.status::text as status,
			cwr.to_address as withdraw_address,
			cwr.tx_hash,
			TO_CHAR(cwr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD HH24:MI:SS') as created_at
		FROM cobo_withdraw_request cwr
		JOIN user_info ui ON cwr.user_id = ui.id
		JOIN team_tree tt ON cwr.user_id = tt.id
		WHERE 1=1
		AND cwr.created_at >= NOW() - INTERVAL '1 hour' * ?
		AND cwr.status NOT IN (2, 5)
		ORDER BY cwr.created_at DESC
		LIMIT ? OFFSET ?
	`
		listArgs = []interface{}{teamId, hours, pageSize, offset}
	}
	err = d.db.Ctx(ctx).Raw(listSql, listArgs...).Scan(&list)

	return list, total, err
}

func (d *teamStatsDao) GetStakeDailyByTeamId(ctx context.Context, teamId int64, days int) ([]*team.DailyTrendItem, error) {
	var list []*team.DailyTrendItem
	var sql string
	var args []interface{}
	if days > 0 {
		sql = teamTreeCTE + `
			SELECT
				TO_CHAR(crr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') as date,
				COALESCE(SUM(crr.amount), 0) as amount
			FROM cobo_recharge_record crr
			JOIN team_tree tt ON crr.user_id = tt.id
			WHERE 1=1
			AND crr.created_at >= (CURRENT_DATE - INTERVAL '1 day' * ?) AT TIME ZONE 'Asia/Shanghai'
			AND crr.status = 1
			GROUP BY TO_CHAR(crr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')
			ORDER BY date ASC`
		args = []interface{}{teamId, days}
	} else {
		sql = teamTreeCTE + `
			SELECT
				TO_CHAR(crr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') as date,
				COALESCE(SUM(crr.amount), 0) as amount
			FROM cobo_recharge_record crr
			JOIN team_tree tt ON crr.user_id = tt.id
			WHERE 1=1
			AND crr.status = 1
			GROUP BY TO_CHAR(crr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')
			ORDER BY date ASC`
		args = []interface{}{teamId}
	}
	err := d.db.Ctx(ctx).Raw(sql, args...).Scan(&list)
	return list, err
}

func (d *teamStatsDao) GetWithdrawDailyByTeamId(ctx context.Context, teamId int64, days int) ([]*team.DailyTrendItem, error) {
	var list []*team.DailyTrendItem
	var sql string
	var args []interface{}
	if days > 0 {
		sql = teamTreeCTE + `
			SELECT
				TO_CHAR(cwr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') as date,
				COALESCE(SUM(cwr.amount), 0) as amount
			FROM cobo_withdraw_request cwr
			JOIN team_tree tt ON cwr.user_id = tt.id
			WHERE 1=1
			AND cwr.created_at >= (CURRENT_DATE - INTERVAL '1 day' * ?) AT TIME ZONE 'Asia/Shanghai'
			AND cwr.status NOT IN (2, 5)
			GROUP BY TO_CHAR(cwr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')
			ORDER BY date ASC`
		args = []interface{}{teamId, days}
	} else {
		sql = teamTreeCTE + `
			SELECT
				TO_CHAR(cwr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') as date,
				COALESCE(SUM(cwr.amount), 0) as amount
			FROM cobo_withdraw_request cwr
			JOIN team_tree tt ON cwr.user_id = tt.id
			WHERE 1=1
			AND cwr.status NOT IN (2, 5)
			GROUP BY TO_CHAR(cwr.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')
			ORDER BY date ASC`
		args = []interface{}{teamId}
	}
	err := d.db.Ctx(ctx).Raw(sql, args...).Scan(&list)
	return list, err
}

func (d *teamStatsDao) GetStakeSummaryByTeamId(ctx context.Context, teamId int64, days int) (decimal.Decimal, error) {
	var sql string
	var args []interface{}
	if days > 0 {
		sql = teamTreeCTE + `
			SELECT COALESCE(SUM(crr.amount), 0) as amount
			FROM cobo_recharge_record crr
			JOIN team_tree tt ON crr.user_id = tt.id
			WHERE 1=1
			AND crr.created_at >= (CURRENT_DATE - INTERVAL '1 day' * ?) AT TIME ZONE 'Asia/Shanghai'
			AND crr.status = 1`
		args = []interface{}{teamId, days}
	} else {
		sql = teamTreeCTE + `
			SELECT COALESCE(SUM(crr.amount), 0) as amount
			FROM cobo_recharge_record crr
			JOIN team_tree tt ON crr.user_id = tt.id
			WHERE 1=1
			AND crr.status = 1`
		args = []interface{}{teamId}
	}
	record, err := d.db.Ctx(ctx).Raw(sql, args...).One()
	if err != nil {
		return decimal.Zero, err
	}
	amount, _ := decimal.NewFromString(record["amount"].String())
	return amount, nil
}

func (d *teamStatsDao) GetWithdrawSummaryByTeamId(ctx context.Context, teamId int64, days int) (decimal.Decimal, error) {
	var sql string
	var args []interface{}
	if days > 0 {
		sql = teamTreeCTE + `
			SELECT COALESCE(SUM(cwr.amount), 0) as amount
			FROM cobo_withdraw_request cwr
			JOIN team_tree tt ON cwr.user_id = tt.id
			WHERE 1=1
			AND cwr.created_at >= (CURRENT_DATE - INTERVAL '1 day' * ?) AT TIME ZONE 'Asia/Shanghai'
			AND cwr.status NOT IN (2, 5)`
		args = []interface{}{teamId, days}
	} else {
		sql = teamTreeCTE + `
			SELECT COALESCE(SUM(cwr.amount), 0) as amount
			FROM cobo_withdraw_request cwr
			JOIN team_tree tt ON cwr.user_id = tt.id
			WHERE 1=1
			AND cwr.status NOT IN (2, 5)`
		args = []interface{}{teamId}
	}
	record, err := d.db.Ctx(ctx).Raw(sql, args...).One()
	if err != nil {
		return decimal.Zero, err
	}
	amount, _ := decimal.NewFromString(record["amount"].String())
	return amount, nil
}

func (d *teamStatsDao) GetPerformanceByTeamId(ctx context.Context, teamId int64) (*TeamPerformanceStats, error) {
	record, err := d.db.Ctx(ctx).Raw(teamTreeCTE+`
		SELECT
			COALESCE(SUM(p.team_performance), 0) as team_performance,
			COALESCE(SUM(p.personal_performance), 0) as personal_performance,
			COALESCE(SUM(p.direct_count), 0) as direct_count,
			COALESCE(SUM(p.team_total_count), 0) as team_total_count
		FROM team_tree tt
		LEFT JOIN cobo_performance p ON p.user_id = tt.id
	`, teamId).One()
	if err != nil {
		return nil, err
	}
	if record.IsEmpty() {
		return &TeamPerformanceStats{}, nil
	}

	teamPerf, _ := decimal.NewFromString(record["team_performance"].String())
	personalPerf, _ := decimal.NewFromString(record["personal_performance"].String())
	directCount := record["direct_count"].Int()
	teamTotalCount := record["team_total_count"].Int()

	return &TeamPerformanceStats{
		TeamPerformance:     teamPerf,
		PersonalPerformance: personalPerf,
		DirectCount:         directCount,
		TeamTotalCount:      teamTotalCount,
	}, nil
}
