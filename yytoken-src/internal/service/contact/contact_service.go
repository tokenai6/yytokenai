package contact

import (
	"context"

	"XWFrame/internal/dao"
	"XWFrame/internal/service/contact/model"
)

type IContactService interface {
	GetList(ctx context.Context) ([]*model.ContactInfoItem, error)
}

type contactService struct {
	contactInfoDao dao.IContactInfoDao
}

func NewContactService() IContactService {
	return &contactService{contactInfoDao: dao.NewContactInfoDao()}
}

func (s *contactService) GetList(ctx context.Context) ([]*model.ContactInfoItem, error) {
	rows, err := s.contactInfoDao.GetEnabledList(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]*model.ContactInfoItem, 0, len(rows))
	for _, row := range rows {
		list = append(list, &model.ContactInfoItem{
			Title:   row.Title,
			Url:     row.Url,
			IconUrl: row.IconUrl,
		})
	}
	return list, nil
}
