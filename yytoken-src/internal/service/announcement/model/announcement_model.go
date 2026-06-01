package model

import "XWFrame/internal/frame/model"

// AnnouncementListItem 公告列表项（后台）
type AnnouncementListItem struct {
	Id          int64                     `json:"id"`
	TitleKey    string                    `json:"title_key"`
	Status      int                       `json:"status"`
	Priority    int                       `json:"priority"`
	PublishTime string                    `json:"publish_time"`
	CreatedAt   string                    `json:"created_at"`
	UpdatedAt   string                    `json:"updated_at"`
	Contents    []AnnouncementContentItem `json:"contents"`
}

// AnnouncementContentItem 公告内容项
type AnnouncementContentItem struct {
	Language string `json:"language"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

// GetAnnouncementListRes 获取公告列表响应（后台）
type GetAnnouncementListRes struct {
	model.PageRes
	List []*AnnouncementListItem `json:"list"`
}

// AnnouncementDetailRes 公告详情响应（后台）
type AnnouncementDetailRes struct {
	Id          int64                     `json:"id"`
	TitleKey    string                    `json:"title_key"`
	Status      int                       `json:"status"`
	Priority    int                       `json:"priority"`
	PublishTime string                    `json:"publish_time"`
	CreatedAt   string                    `json:"created_at"`
	UpdatedAt   string                    `json:"updated_at"`
	Contents    []AnnouncementContentItem `json:"contents"`
}

// ActiveAnnouncementItem 启用公告项（前端）
type ActiveAnnouncementItem struct {
	Id          int64  `json:"id"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	PublishTime string `json:"publish_time"`
	CreatedAt   string `json:"created_at"`
}

// GetActiveAnnouncementListRes 获取启用公告列表响应（前端）
type GetActiveAnnouncementListRes struct {
	model.PageRes
	List []*ActiveAnnouncementItem `json:"list"`
}

// HomeAnnouncementItem 首页公告项（首页轮询）
type HomeAnnouncementItem struct {
	Id    int64  `json:"id"`
	Title string `json:"title"`
}

// GetActiveTopListRes 获取启用公告前N条响应（首页轮询）
type GetActiveTopListRes struct {
	List []*HomeAnnouncementItem `json:"list"`
}

// GetActiveDetailRes 获取启用公告详情响应（前端）
type GetActiveDetailRes struct {
	Id          int64  `json:"id"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	PublishTime string `json:"publish_time"`
	CreatedAt   string `json:"created_at"`
}

// LanguageItem 语言项
type LanguageItem struct {
	Code string `json:"code"` // 语言代码
	Name string `json:"name"` // 语言名称
}

// GetSupportedLanguagesRes 获取支持的语言列表响应
type GetSupportedLanguagesRes struct {
	Languages []LanguageItem `json:"languages"`
}
