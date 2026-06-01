package announcement

import (
	"context"
	"time"

	"XWFrame/internal/frame/consts"
	"XWFrame/internal/frame/model"
	"XWFrame/internal/repository"
	announcementModel "XWFrame/internal/service/announcement/model"

	"github.com/gogf/gf/v2/errors/gerror"
)

// IAnnouncementService 公告服务接口
type IAnnouncementService interface {
	// Create 创建公告
	Create(ctx context.Context, req *CreateAnnouncementReq) error

	// GetById 根据ID获取公告详情
	GetById(ctx context.Context, id int64) (*announcementModel.AnnouncementDetailRes, error)

	// GetList 获取公告列表（后台）
	GetList(ctx context.Context, req *GetAnnouncementListReq) (*announcementModel.GetAnnouncementListRes, error)

	// UpdateById 根据ID更新公告
	UpdateById(ctx context.Context, req *UpdateAnnouncementReq) error

	// DeleteById 根据ID删除公告
	DeleteById(ctx context.Context, id int64) error

	// GetActiveList 获取启用的公告列表（前端）
	GetActiveList(ctx context.Context, req *GetActiveAnnouncementListReq) (*announcementModel.GetActiveAnnouncementListRes, error)

	// GetActiveTopList 获取启用的公告前N条（首页轮询）
	GetActiveTopList(ctx context.Context, req *GetActiveTopListReq) (*announcementModel.GetActiveTopListRes, error)

	// GetActiveDetail 获取启用的公告详情（前端）
	GetActiveDetail(ctx context.Context, req *GetActiveDetailReq) (*announcementModel.GetActiveDetailRes, error)

	// GetSupportedLanguages 获取支持的语言列表
	GetSupportedLanguages(ctx context.Context) (*announcementModel.GetSupportedLanguagesRes, error)
}

// announcementService 公告服务实现
type announcementService struct {
	announcementRepo repository.IAnnouncementRepository
}

// NewAnnouncementService 创建公告服务实例
func NewAnnouncementService() IAnnouncementService {
	return &announcementService{
		announcementRepo: repository.NewAnnouncementRepository(),
	}
}

// CreateAnnouncementReq 创建公告请求
type CreateAnnouncementReq struct {
	TitleKey string                   `json:"title_key"`
	Status   int                      `json:"status"`
	Priority int                      `json:"priority"`
	Contents []AnnouncementContentReq `json:"contents"`
}

// AnnouncementContentReq 公告内容请求
type AnnouncementContentReq struct {
	Language string `json:"language"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

// GetAnnouncementListReq 获取公告列表请求（后台）
type GetAnnouncementListReq struct {
	TitleKey string `json:"title_key"`
	Status   int    `json:"status"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// UpdateAnnouncementReq 更新公告请求
type UpdateAnnouncementReq struct {
	Id       int64                    `json:"id"`
	TitleKey string                   `json:"title_key"`
	Status   int                      `json:"status"`
	Priority int                      `json:"priority"`
	Contents []AnnouncementContentReq `json:"contents"`
}

// GetActiveAnnouncementListReq 获取启用公告列表请求（前端）
type GetActiveAnnouncementListReq struct {
	Language string `json:"language"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// GetActiveTopListReq 获取启用公告前N条请求（首页轮询）
type GetActiveTopListReq struct {
	Language string `json:"language"`
}

// GetActiveDetailReq 获取启用公告详情请求（前端）
type GetActiveDetailReq struct {
	Id       int64  `json:"id"`
	Language string `json:"language"`
}

// Create 创建公告
func (s *announcementService) Create(ctx context.Context, req *CreateAnnouncementReq) error {
	supportedContents := filterSupportedContents(req.Contents)
	if len(req.Contents) > 0 && len(supportedContents) == 0 {
		return gerror.New("no supported language content provided")
	}

	// 转换请求格式
	repoReq := &repository.CreateAnnouncementReq{
		TitleKey: req.TitleKey,
		Status:   req.Status,
		Priority: req.Priority,
		Contents: make([]repository.AnnouncementContentReq, 0, len(req.Contents)),
	}

	for _, content := range supportedContents {
		repoReq.Contents = append(repoReq.Contents, repository.AnnouncementContentReq{
			Language: content.Language,
			Title:    content.Title,
			Content:  content.Content,
		})
	}

	_, err := s.announcementRepo.Create(ctx, repoReq)
	return err
}

// GetById 根据ID获取公告详情
func (s *announcementService) GetById(ctx context.Context, id int64) (*announcementModel.AnnouncementDetailRes, error) {
	announcement, err := s.announcementRepo.GetById(ctx, id)
	if err != nil {
		return nil, err
	}

	if announcement == nil {
		return nil, gerror.New("公告不存在")
	}

	// 获取公告的所有语言内容
	contents, err := s.announcementRepo.GetContentsByAnnouncementId(ctx, id)
	if err != nil {
		return nil, err
	}

	contentItems := make([]announcementModel.AnnouncementContentItem, 0, len(contents))
	indexByLanguage := make(map[string]int, len(contents))
	for _, c := range contents {
		normalized := consts.NormalizeLanguage(c.Language)
		if normalized == "" {
			continue
		}
		item := announcementModel.AnnouncementContentItem{
			Language: normalized,
			Title:    c.Title,
			Content:  c.Content,
		}
		if idx, exists := indexByLanguage[normalized]; exists {
			contentItems[idx] = item
			continue
		}
		indexByLanguage[normalized] = len(contentItems)
		contentItems = append(contentItems, item)
	}

	publishStr := formatPublishTime(announcement.PublishTime, announcement.CreatedAt)
	return &announcementModel.AnnouncementDetailRes{
		Id:          announcement.Id,
		TitleKey:    announcement.TitleKey,
		Status:      announcement.Status,
		Priority:    announcement.Priority,
		PublishTime: publishStr,
		CreatedAt:   publishStr,
		UpdatedAt:   announcement.UpdatedAt.Format("2006-01-02 15:04:05"),
		Contents:    contentItems,
	}, nil
}

// GetList 获取公告列表（后台）
func (s *announcementService) GetList(ctx context.Context, req *GetAnnouncementListReq) (*announcementModel.GetAnnouncementListRes, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	repoReq := &repository.GetAnnouncementListReq{
		TitleKey: req.TitleKey,
		Status:   req.Status,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	announcements, total, err := s.announcementRepo.GetList(ctx, repoReq)
	if err != nil {
		return nil, err
	}

	// 收集所有公告ID，批量获取多语言内容
	announcementIds := make([]int64, 0, len(announcements))
	for _, announcement := range announcements {
		announcementIds = append(announcementIds, announcement.Id)
	}

	allContents, err := s.announcementRepo.GetAllContentsByAnnouncementIds(ctx, announcementIds)
	if err != nil {
		return nil, err
	}

	// 按 announcement_id 分组内容
	contentMap := make(map[int64][]announcementModel.AnnouncementContentItem, len(announcements))
	for _, c := range allContents {
		normalized := consts.NormalizeLanguage(c.Language)
		if normalized == "" {
			continue
		}
		contentMap[c.AnnouncementId] = append(contentMap[c.AnnouncementId], announcementModel.AnnouncementContentItem{
			Language: normalized,
			Title:    c.Title,
			Content:  c.Content,
		})
	}

	// 转换为响应格式
	items := make([]*announcementModel.AnnouncementListItem, 0, len(announcements))
	for _, announcement := range announcements {
		publishStr := formatPublishTime(announcement.PublishTime, announcement.CreatedAt)
		items = append(items, &announcementModel.AnnouncementListItem{
			Id:          announcement.Id,
			TitleKey:    announcement.TitleKey,
			Status:      announcement.Status,
			Priority:    announcement.Priority,
			PublishTime: publishStr,
			CreatedAt:   publishStr,
			UpdatedAt:   announcement.UpdatedAt.Format("2006-01-02 15:04:05"),
			Contents:    contentMap[announcement.Id],
		})
	}

	return &announcementModel.GetAnnouncementListRes{
		PageRes: model.PageRes{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    total,
			Pages:    (total + req.PageSize - 1) / req.PageSize,
		},
		List: items,
	}, nil
}

// UpdateById 根据ID更新公告
func (s *announcementService) UpdateById(ctx context.Context, req *UpdateAnnouncementReq) error {
	supportedContents := filterSupportedContents(req.Contents)
	if len(req.Contents) > 0 && len(supportedContents) == 0 {
		return gerror.New("no supported language content provided")
	}

	// 转换请求格式
	repoReq := &repository.UpdateAnnouncementReq{
		Id:       req.Id,
		TitleKey: req.TitleKey,
		Status:   req.Status,
		Priority: req.Priority,
		Contents: make([]repository.AnnouncementContentReq, 0, len(req.Contents)),
	}

	for _, content := range supportedContents {
		repoReq.Contents = append(repoReq.Contents, repository.AnnouncementContentReq{
			Language: content.Language,
			Title:    content.Title,
			Content:  content.Content,
		})
	}

	return s.announcementRepo.UpdateById(ctx, repoReq)
}

func filterSupportedContents(contents []AnnouncementContentReq) []AnnouncementContentReq {
	if len(contents) == 0 {
		return nil
	}

	result := make([]AnnouncementContentReq, 0, len(contents))
	indexByLanguage := make(map[string]int, len(contents))
	for _, content := range contents {
		normalized := consts.NormalizeLanguage(content.Language)
		if normalized == "" {
			continue
		}
		content.Language = normalized
		if idx, exists := indexByLanguage[normalized]; exists {
			result[idx] = content
			continue
		}
		indexByLanguage[normalized] = len(result)
		result = append(result, content)
	}
	return result
}

// DeleteById 根据ID删除公告
func (s *announcementService) DeleteById(ctx context.Context, id int64) error {
	return s.announcementRepo.DeleteById(ctx, id)
}

// GetActiveList 获取启用的公告列表（前端）
func (s *announcementService) GetActiveList(ctx context.Context, req *GetActiveAnnouncementListReq) (*announcementModel.GetActiveAnnouncementListRes, error) {
	// 验证语言代码
	if !consts.IsValidLanguage(req.Language) {
		req.Language = consts.LanguageDefault
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	announcements, total, err := s.announcementRepo.GetActiveList(ctx, req.Language, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}

	// 转换为响应格式
	items := make([]*announcementModel.ActiveAnnouncementItem, 0, len(announcements))
	for _, announcement := range announcements {
		if announcement.Status != 1 {
			continue
		}
		publishStr := formatPublishTime(announcement.PublishTime, announcement.CreatedAt)
		items = append(items, &announcementModel.ActiveAnnouncementItem{
			Id:          announcement.Id,
			Title:       announcement.Title,
			Content:     announcement.Content,
			PublishTime: publishStr,
			CreatedAt:   publishStr,
		})
	}

	return &announcementModel.GetActiveAnnouncementListRes{
		PageRes: model.PageRes{
			Page:     req.Page,
			PageSize: req.PageSize,
			Total:    total,
			Pages:    (total + req.PageSize - 1) / req.PageSize,
		},
		List: items,
	}, nil
}

// GetActiveTopList 获取启用的公告前N条（首页轮询）
func (s *announcementService) GetActiveTopList(ctx context.Context, req *GetActiveTopListReq) (*announcementModel.GetActiveTopListRes, error) {
	// 验证语言代码
	if !consts.IsValidLanguage(req.Language) {
		req.Language = consts.LanguageDefault
	}

	announcements, err := s.announcementRepo.GetActiveTopList(ctx, req.Language, 3)
	if err != nil {
		return nil, err
	}

	// 转换为响应格式
	items := make([]*announcementModel.HomeAnnouncementItem, 0, len(announcements))
	for _, announcement := range announcements {
		if announcement.Status != 1 {
			continue
		}
		items = append(items, &announcementModel.HomeAnnouncementItem{
			Id:    announcement.Id,
			Title: announcement.Title,
		})
	}

	return &announcementModel.GetActiveTopListRes{
		List: items,
	}, nil
}

// GetActiveDetail 获取启用的公告详情（前端）
func (s *announcementService) GetActiveDetail(ctx context.Context, req *GetActiveDetailReq) (*announcementModel.GetActiveDetailRes, error) {
	// 验证语言代码
	if !consts.IsValidLanguage(req.Language) {
		req.Language = consts.LanguageDefault
	}

	// 调用仓储层获取公告详情
	announcement, err := s.announcementRepo.GetActiveDetail(ctx, req.Id, req.Language)
	if err != nil {
		return nil, err
	}

	if announcement == nil {
		return nil, gerror.New("公告不存在或已禁用")
	}

	publishStr := formatPublishTime(announcement.PublishTime, announcement.CreatedAt)
	return &announcementModel.GetActiveDetailRes{
		Id:          announcement.Id,
		Title:       announcement.Title,
		Content:     announcement.Content,
		PublishTime: publishStr,
		CreatedAt:   publishStr,
	}, nil
}

// GetSupportedLanguages 获取支持的语言列表
func (s *announcementService) GetSupportedLanguages(ctx context.Context) (*announcementModel.GetSupportedLanguagesRes, error) {
	// 构建支持的语言列表
	languages := make([]announcementModel.LanguageItem, 0, len(consts.SupportedLanguages))
	for _, code := range consts.SupportedLanguages {
		languages = append(languages, announcementModel.LanguageItem{
			Code: code,
			Name: consts.GetLanguageName(code),
		})
	}

	return &announcementModel.GetSupportedLanguagesRes{
		Languages: languages,
	}, nil
}

func formatPublishTime(publish *time.Time, created time.Time) string {
	if publish != nil && !publish.IsZero() {
		return publish.Format("2006-01-02 15:04:05")
	}
	return created.Format("2006-01-02 15:04:05")
}
