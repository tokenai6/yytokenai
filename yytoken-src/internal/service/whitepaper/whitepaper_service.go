package whitepaper

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/service/whitepaper/model"
)

type IWhitepaperService interface {
	GetList(ctx context.Context) ([]*model.WhitepaperItem, error)
}

type whitepaperService struct {
	whitepaperDao dao.IWhitepaperDao
}

func NewWhitepaperService() IWhitepaperService {
	return &whitepaperService{whitepaperDao: dao.NewWhitepaperDao()}
}

func (s *whitepaperService) GetList(ctx context.Context) ([]*model.WhitepaperItem, error) {
	rows, err := s.whitepaperDao.GetEnabledList(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]*model.WhitepaperItem, 0, len(rows))
	for _, row := range rows {
		list = append(list, &model.WhitepaperItem{
			Language: row.Language,
			Url:      row.Url,
		})
	}
	return list, nil
}
