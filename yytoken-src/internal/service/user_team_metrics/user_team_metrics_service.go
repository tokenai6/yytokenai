package user_team_metrics

import (
	"context"
	"math"
	"strings"
	"time"

	"XWFrame/internal/entity"
	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/db"
	"XWFrame/internal/repository"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/shopspring/decimal"
)

// IUserTeamMetricsService 用户团队指标服务接口
type IUserTeamMetricsService interface {
	// Create 创建用户团队指标
	Create(ctx context.Context, req *CreateMetricsReq) (*CreateMetricsRes, error)

	// GetList 获取用户团队指标列表（分页）
	GetList(ctx context.Context, req *GetMetricsListReq) (*GetMetricsListRes, error)

	// GetById 根据ID获取用户团队指标详情
	GetById(ctx context.Context, id int64) (*GetMetricsDetailRes, error)

	// DeleteById 根据ID删除用户团队指标
	DeleteById(ctx context.Context, id int64) (*DeleteMetricsRes, error)

	//处理并更新所有用户团队指标
	HandleAndUpdateAllMetrics(ctx context.Context) error
}

// userTeamMetricsService 用户团队指标服务实现
type userTeamMetricsService struct {
	metricsRepo repository.IUserTeamMetricsRepository
	userRepo    repository.IUserRepository
}

// NewUserTeamMetricsService 创建用户团队指标服务实例
func NewUserTeamMetricsService() IUserTeamMetricsService {
	return &userTeamMetricsService{
		metricsRepo: repository.NewUserTeamMetricsRepository(),
		userRepo:    repository.NewUserRepository(),
	}
}

// Create 创建用户团队指标
func (s *userTeamMetricsService) Create(ctx context.Context, req *CreateMetricsReq) (*CreateMetricsRes, error) {
	// 地址统一转小写
	walletAddress := strings.ToLower(req.Address)

	// 检查地址是否已存在
	existing, err := s.metricsRepo.GetByAddress(ctx, walletAddress)
	g.Log().Info(ctx, "existing", existing)
	g.Log().Info(ctx, "err", err)
	if err != nil {
		return nil, gerror.Wrap(err, "查询地址失败")
	}
	if existing != nil {
		return nil, gerror.New("该地址已存在")
	}

	// 根据地址查询用户ID
	userEntity, err := s.userRepo.GetUserByWalletAddress(ctx, walletAddress)
	if err != nil {
		return nil, gerror.Wrap(err, "查询用户失败")
	}
	if userEntity == nil {
		return nil, gerror.New("该地址对应的用户不存在")
	}

	// 获取当前日期（只取日期部分，时间设为00:00:00）Date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	//获取一条已存在的数据别人的，插入NextRatioChangeDate
	metricsList, _, err := s.metricsRepo.GetList(ctx, 1, 1, "")
	if err != nil {
		return nil, gerror.Wrap(err, "查询用户团队指标列表失败")
	}
	if len(metricsList) == 0 {
		return nil, gerror.New("用户团队指标列表为空")
	}
	// 加载本地时区
	loc, _ := time.LoadLocation(consts.TimezoneUTC8)
	// 确保时间对象使用本地时区
	var nextRatioChangeDate *time.Time
	var cycleEndDate *time.Time
	if metricsList[0].CycleEndDate != nil {
		localTime := metricsList[0].CycleEndDate.In(loc)
		cycleEndDate = &localTime
		nextRatioChangeDate = &localTime
	}
	// 创建实体，其他字段设为0，地址统一转小写存储
	metrics := &entity.UserTeamMetricsEntity{
		UserId:                  userEntity.Id,
		Address:                 walletAddress,
		LastCycleTeamRelease:    decimal.Zero,
		LastCycleTeamNew:        decimal.Zero,
		CurrentCycleTeamRelease: decimal.Zero,
		CurrentCycleTeamNew:     decimal.Zero,
		TeamReleaseRatio:        decimal.Zero,
		NextRatioChangeDate:     nextRatioChangeDate,
		CycleEndDate:            cycleEndDate,
	}

	// 保存到数据库
	err = s.metricsRepo.Create(ctx, metrics)
	if err != nil {
		return nil, gerror.Wrap(err, "创建用户团队指标失败")
	}

	return &CreateMetricsRes{
		Id: metrics.Id,
	}, nil
}

// GetList 获取用户团队指标列表（分页）
func (s *userTeamMetricsService) GetList(ctx context.Context, req *GetMetricsListReq) (*GetMetricsListRes, error) {
	// 参数校验
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	// 查询列表
	metricsList, total, err := s.metricsRepo.GetList(ctx, req.Page, req.PageSize, req.Address)
	if err != nil {
		return nil, gerror.Wrap(err, "查询用户团队指标列表失败")
	}

	// 转换为响应格式
	list := make([]*MetricsItem, 0, len(metricsList))
	// 加载本地时区
	loc, _ := time.LoadLocation(consts.TimezoneUTC8)
	for _, metrics := range metricsList {
		// 转换时区：确保返回的时间是本地时区（Asia/Shanghai）
		var cycleEndDate *time.Time
		if metrics.CycleEndDate != nil {
			localTime := metrics.CycleEndDate.In(loc)
			cycleEndDate = &localTime
		}
		var nextRatioChangeDate *time.Time
		if metrics.NextRatioChangeDate != nil {
			localTime := metrics.NextRatioChangeDate.In(loc)
			nextRatioChangeDate = &localTime
		}

		list = append(list, &MetricsItem{
			Id:      metrics.Id,
			UserId:  metrics.UserId,
			Address: metrics.Address,
			//保留2位小数
			LastCycleTeamRelease:    metrics.LastCycleTeamRelease.Round(2),
			LastCycleTeamNew:        metrics.LastCycleTeamNew.Round(2),
			CurrentCycleTeamRelease: metrics.CurrentCycleTeamRelease.Round(2),
			CurrentCycleTeamNew:     metrics.CurrentCycleTeamNew.Round(2),
			TeamReleaseRatio:        metrics.TeamReleaseRatio.Round(2),
			NextRatioChangeDate:     nextRatioChangeDate,
			UpdatedAt:               metrics.UpdatedAt.In(loc),
			CycleEndDate:            cycleEndDate,
		})
	}

	// 计算总页数
	pages := int(math.Ceil(float64(total) / float64(req.PageSize)))

	return &GetMetricsListRes{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Pages:    pages,
	}, nil
}

// GetById 根据ID获取用户团队指标详情
func (s *userTeamMetricsService) GetById(ctx context.Context, id int64) (*GetMetricsDetailRes, error) {
	if id <= 0 {
		return nil, gerror.New("ID不能为空")
	}

	metrics, err := s.metricsRepo.GetById(ctx, id)
	if err != nil {
		return nil, gerror.Wrap(err, "查询用户团队指标失败")
	}
	if metrics == nil {
		return nil, gerror.New("用户团队指标不存在")
	}

	// 加载本地时区
	loc, _ := time.LoadLocation(consts.TimezoneUTC8)
	// 转换时区：确保返回的时间是本地时区（Asia/Shanghai）
	var cycleEndDate *time.Time
	if metrics.CycleEndDate != nil {
		localTime := metrics.CycleEndDate.In(loc)
		cycleEndDate = &localTime
	}
	var nextRatioChangeDate *time.Time
	if metrics.NextRatioChangeDate != nil {
		localTime := metrics.NextRatioChangeDate.In(loc)
		nextRatioChangeDate = &localTime
	}

	return &GetMetricsDetailRes{
		Id:                      metrics.Id,
		UserId:                  metrics.UserId,
		Address:                 metrics.Address,
		LastCycleTeamRelease:    metrics.LastCycleTeamRelease,
		LastCycleTeamNew:        metrics.LastCycleTeamNew,
		CurrentCycleTeamRelease: metrics.CurrentCycleTeamRelease,
		CurrentCycleTeamNew:     metrics.CurrentCycleTeamNew,
		TeamReleaseRatio:        metrics.TeamReleaseRatio,
		NextRatioChangeDate:     nextRatioChangeDate,
		CycleEndDate:            cycleEndDate,
		UpdatedAt:               metrics.UpdatedAt.In(loc),
	}, nil
}

// DeleteById 根据ID删除用户团队指标
func (s *userTeamMetricsService) DeleteById(ctx context.Context, id int64) (*DeleteMetricsRes, error) {
	if id <= 0 {
		return nil, gerror.New("ID不能为空")
	}

	// 检查是否存在
	metrics, err := s.metricsRepo.GetById(ctx, id)
	if err != nil {
		return nil, gerror.Wrap(err, "查询用户团队指标失败")
	}
	if metrics == nil {
		return nil, gerror.New("用户团队指标不存在")
	}

	// 删除
	err = s.metricsRepo.DeleteById(ctx, id)
	if err != nil {
		return nil, gerror.Wrap(err, "删除用户团队指标失败")
	}

	return &DeleteMetricsRes{
		Message: "删除成功",
	}, nil
}

// HandleAndUpdateAllMetrics 处理并更新所有用户团队指标
func (s *userTeamMetricsService) HandleAndUpdateAllMetrics(ctx context.Context) error {
	// 加载本地时区（+0800），确保所有时间对象使用正确的时区
	loc, _ := time.LoadLocation(consts.TimezoneUTC8)

	metricsList, err := s.metricsRepo.GetAllMetrics(ctx)
	if err != nil {
		return gerror.Wrap(err, "查询用户团队指标列表失败")
	}
	for _, metrics := range metricsList {
		//从周期结束时间，往前推2天
		cycleStartTime := metrics.CycleEndDate.AddDate(0, 0, -2)
		cycleEndTime := metrics.CycleEndDate.AddDate(0, 0, 0)
		//查询周期内的用户团队新增
		userTeamNew, err := s.metricsRepo.GetUserTeamNewByCycle(ctx, metrics.UserId, cycleStartTime, cycleEndTime)
		//查询团队静态释放

		var teamStaticRelease = metrics.CurrentCycleTeamRelease
		//当前时间小时是0点的时候，处理团队释放
		if time.Now().Hour() == 23 {
			teamStaticRelease, err = s.metricsRepo.GetTeamStaticReleaseByCycle(ctx, metrics.UserId, cycleStartTime, cycleEndTime)
			if err != nil {
				return gerror.Wrap(err, "查询团队静态释放失败")
			}
			//乘以0.01
			teamStaticRelease = teamStaticRelease.Mul(decimal.NewFromFloat(0.01))
		}
		now := time.Now()

		//如果是结算日，则乘以2
		if now.Day() == metrics.CycleEndDate.Day() {
			teamStaticRelease = teamStaticRelease.Mul(decimal.NewFromFloat(2))
		}

		// 查询的是整个周期内的累计新增，直接使用查询结果，不需要累加 CurrentCycleTeamNew
		// 因为 GetUserTeamNewByCycle 已经查询了从 cycleStartTime 到 cycleEndTime 的完整数据
		totalUserTeamNew := userTeamNew
		//累计团队静态释放（同样，查询的是整个周期内的累计释放）
		totalTeamStaticRelease := teamStaticRelease

		//如果hour = 23 并且日期是今天，则更新用户团队指标
		if (now.Hour() == 23) && metrics.CycleEndDate != nil && now.Day() == metrics.CycleEndDate.Day() {

			//判断是否达标，就是周期内的新增是否大于释放，并且计算倍数，不要四舍五入
			var count decimal.Decimal
			if totalTeamStaticRelease.IsZero() {
				// 如果释放为0，新增大于0则倍数设为1，否则为0
				if totalUserTeamNew.GreaterThan(decimal.Zero) {
					count = decimal.NewFromInt32(1)
				} else {
					count = decimal.Zero
				}
			} else {
				// 计算倍数：新增 / 释放，向下取整（不四舍五入）
				count = totalUserTeamNew.Div(totalTeamStaticRelease).Floor()
			}
			addDate := 2 * int(count.IntPart())
			if addDate == 0 {
				addDate = 2
			}
			// 判断是否达标：新增大于释放
			if totalUserTeamNew.GreaterThan(totalTeamStaticRelease) {
				metrics.TeamReleaseRatio = decimal.NewFromFloat(0.01)
				// 达标：设置团队和个人释放比例为0.01
				if !metrics.TeamReleaseRatio.Equal(decimal.NewFromFloat(0.01)) {
					//UpdateTeamStakeRate 更新团队和个人的释放比例为0.01
					err = s.userRepo.UpdateTeamStakeRate(ctx, metrics.UserId, 0.01)
					if err != nil {
						return gerror.Wrap(err, "更新团队和个人的释放比例为0.01失败")
					}
				}
			} else {
				// 不达标：保持释放比例为0.001（已在上面设置为0）
				//并且刷新团队change时间是今天
				if !metrics.TeamReleaseRatio.Equal(decimal.NewFromFloat(0.001)) && metrics.NextRatioChangeDate.Day() == now.Day() {
					metrics.TeamReleaseRatio = decimal.NewFromFloat(0.001)
					//更新团队和个人的释放比例为0.001
					err = s.userRepo.UpdateTeamStakeRate(ctx, metrics.UserId, 0.001)
					if err != nil {
						return gerror.Wrap(err, "更新团队和个人的释放比例为0.001失败")
					}
				}
			}
			g.Log().Info(ctx, "metrics.TeamReleaseRatio", metrics.TeamReleaseRatio)

			// 确保时间对象使用本地时区（+0800）
			nextRatioChangeDate := metrics.CycleEndDate.AddDate(0, 0, addDate).In(loc)
			metrics.NextRatioChangeDate = &nextRatioChangeDate
			metrics.LastCycleTeamRelease = totalTeamStaticRelease
			metrics.LastCycleTeamNew = totalUserTeamNew
			metrics.CurrentCycleTeamRelease = decimal.Zero
			metrics.CurrentCycleTeamNew = decimal.Zero
			//重置周期（确保使用本地时区）
			newCycleEndDate := metrics.CycleEndDate.AddDate(0, 0, 2).In(loc)
			metrics.CycleEndDate = &newCycleEndDate

		} else {
			metrics.CurrentCycleTeamRelease = totalTeamStaticRelease
			metrics.CurrentCycleTeamNew = totalUserTeamNew
		}
		// 确保 UpdatedAt 使用本地时区（+0800）
		metrics.UpdatedAt = time.Now().In(loc)

		err = s.UpdateMetrics(ctx, metrics)
		if err != nil {
			return gerror.Wrap(err, "更新用户团队指标失败")
		}
	}
	return nil
}

// UpdateMetrics 更新用户团队指标
func (s *userTeamMetricsService) UpdateMetrics(ctx context.Context, metrics *entity.UserTeamMetricsEntity) error {
	//开启事务
	tx := db.WithTx(ctx, nil, func(ctx context.Context, tx gdb.TX) error {
		//更新LastCycleTeamRelease
		err := s.metricsRepo.UpdateById(ctx, metrics.Id, map[string]interface{}{
			"last_cycle_team_release":    metrics.LastCycleTeamRelease,
			"last_cycle_team_new":        metrics.LastCycleTeamNew,
			"current_cycle_team_release": metrics.CurrentCycleTeamRelease,
			"current_cycle_team_new":     metrics.CurrentCycleTeamNew,
			"team_release_ratio":         metrics.TeamReleaseRatio,
			"next_ratio_change_date":     metrics.NextRatioChangeDate,
			"cycle_end_date":             metrics.CycleEndDate,
			"updated_at":                 metrics.UpdatedAt,
		})
		if err != nil {
			return gerror.Wrap(err, "更新用户团队指标失败")
		}
		return nil
	})
	return tx
}
