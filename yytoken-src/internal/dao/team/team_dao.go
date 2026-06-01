package team

import (
	"context"
	"fmt"
	"strings"

	"XWFrame/internal/entity/team"
	"XWFrame/internal/frame/db"

	"github.com/gogf/gf/v2/database/gdb"
)

type ITeamDao interface {
	Create(ctx context.Context, tx gdb.TX, entity *team.TeamEntity) error
	Update(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error
	Delete(ctx context.Context, tx gdb.TX, id int64) error
	GetById(ctx context.Context, id int64) (*team.TeamEntity, error)
	GetByIds(ctx context.Context, ids []int64) (map[int64]*team.TeamEntity, error)
	GetByLeaderAddress(ctx context.Context, address string) (*team.TeamEntity, error)
	GetList(ctx context.Context, name string, status int, page, pageSize int) ([]*team.TeamEntity, int, error)
	GetAll(ctx context.Context) ([]*team.TeamEntity, error)
	GetTopLevelTeams(ctx context.Context) ([]*team.TeamEntity, error)
	GetChildrenByParentId(ctx context.Context, parentId int64) ([]*team.TeamEntity, error)
	GetPRDirectChildTeams(ctx context.Context) ([]*team.TeamEntity, error)
}

type teamDao struct {
	db gdb.DB
}

var prTargetLeaderAddresses = []string{
	"0x239fd41551f83f91aa2500be987a6a7f12994b1d",
	"0x52681178a52491ff51e1ba416448fda2f3300588",
	"0xda23d3ee389999ac10a4161f2a531dc1ffd12b72",
	"0x73f0ed351e91bc649ac675ca2cda40af396a1e24",
}

func NewTeamDao() ITeamDao {
	return &teamDao{db: db.GetDB()}
}

func (d *teamDao) Create(ctx context.Context, tx gdb.TX, entity *team.TeamEntity) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("team").FieldsEx("id").Data(entity).Insert()
		return err
	})
}

func (d *teamDao) Update(ctx context.Context, tx gdb.TX, id int64, data map[string]interface{}) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("team").Where("id", id).Data(data).Update()
		return err
	})
}

func (d *teamDao) Delete(ctx context.Context, tx gdb.TX, id int64) error {
	return db.WithTx(ctx, tx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("team").Where("id", id).Delete()
		return err
	})
}

func (d *teamDao) GetById(ctx context.Context, id int64) (*team.TeamEntity, error) {
	var entity team.TeamEntity
	err := d.db.Ctx(ctx).Model("team").Where("id", id).Scan(&entity)
	if err != nil {
		return nil, err
	}
	if entity.Id == 0 {
		return nil, nil
	}
	return &entity, nil
}

func (d *teamDao) GetByIds(ctx context.Context, ids []int64) (map[int64]*team.TeamEntity, error) {
	if len(ids) == 0 {
		return make(map[int64]*team.TeamEntity), nil
	}
	var list []*team.TeamEntity
	err := d.db.Ctx(ctx).Model("team").WhereIn("id", ids).Scan(&list)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*team.TeamEntity, len(list))
	for _, t := range list {
		result[t.Id] = t
	}
	return result, nil
}

func (d *teamDao) GetByLeaderAddress(ctx context.Context, address string) (*team.TeamEntity, error) {
	var entity team.TeamEntity
	// 领导人地址可能存在大小写差异（校验和地址），这里做不区分大小写匹配
	err := d.db.Ctx(ctx).Model("team").
		Where("LOWER(leader_wallet_address) = LOWER(?)", address).
		Scan(&entity)
	if err != nil {
		return nil, err
	}
	if entity.Id == 0 {
		return nil, nil
	}
	return &entity, nil
}

func (d *teamDao) GetList(ctx context.Context, name string, status int, page, pageSize int) ([]*team.TeamEntity, int, error) {
	var list []*team.TeamEntity
	query := d.db.Ctx(ctx).Model("team")

	if name != "" {
		query = query.WhereLike("name", "%"+name+"%")
	}
	if status >= 0 {
		query = query.Where("status", status)
	}

	total, err := query.Count()
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("id ASC").Limit((page-1)*pageSize, pageSize).Scan(&list)
	return list, total, err
}

func (d *teamDao) GetAll(ctx context.Context) ([]*team.TeamEntity, error) {
	var list []*team.TeamEntity
	err := d.db.Ctx(ctx).Model("team").Where("status", 1).Order("id ASC").Scan(&list)
	return list, err
}

func (d *teamDao) GetTopLevelTeams(ctx context.Context) ([]*team.TeamEntity, error) {
	var list []*team.TeamEntity
	err := d.db.Ctx(ctx).Model("team").Where("status", 1).Order("id ASC").Scan(&list)
	return list, err
}

func (d *teamDao) GetChildrenByParentId(ctx context.Context, parentId int64) ([]*team.TeamEntity, error) {
	var list []*team.TeamEntity
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
		),
		child_team_ids AS (
			SELECT DISTINCT team_id
			FROM direct_users
			WHERE team_id IS NOT NULL AND team_id <> ?
		)
		SELECT t.*
		FROM team t
		JOIN child_team_ids c ON c.team_id = t.id
		WHERE t.status = 1
		ORDER BY t.id ASC
	`, parentId, parentId).Scan(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (d *teamDao) GetPRDirectChildTeams(ctx context.Context) ([]*team.TeamEntity, error) {
	quoted := make([]string, 0, len(prTargetLeaderAddresses))
	for _, addr := range prTargetLeaderAddresses {
		quoted = append(quoted, fmt.Sprintf("'%s'", strings.ToLower(addr)))
	}

	var list []*team.TeamEntity
	err := d.db.Ctx(ctx).Raw(fmt.Sprintf(`
		WITH target_leaders AS (
			SELECT invite_code
			FROM user_info
			WHERE status = 1
			  AND LOWER(wallet_address) IN (%s)
		)
		SELECT DISTINCT t.*
		FROM user_info u
		JOIN target_leaders tl ON u.parent_invite_code = tl.invite_code
		JOIN team t ON LOWER(t.leader_wallet_address) = LOWER(u.wallet_address)
		WHERE u.status = 1
		  AND COALESCE(u.is_test, 0) = 0
		  AND t.status = 1
		  AND t.name <> '首码'
		  AND t.name NOT LIKE 'R-%%'
		ORDER BY t.id ASC
	`, strings.Join(quoted, ","))).Scan(&list)
	if err != nil {
		return nil, err
	}
	return list, nil
}
