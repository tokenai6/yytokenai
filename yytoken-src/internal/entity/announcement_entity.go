package entity

import (
	"time"

	"XWFrame/internal/frame/model"
)

// AnnouncementEntity 公告实体
type AnnouncementEntity struct {
	model.BaseEntity
	TitleKey    string     `json:"title_key" g:"column:title_key;not null;comment:公告标题键名"`
	Status      int        `json:"status" g:"column:status;default:1;comment:状态：0-禁用，1-启用"`
	Priority    int        `json:"priority" g:"column:priority;default:0;comment:优先级，数字越大优先级越高"`
	PublishTime *time.Time `json:"publish_time" g:"column:publish_time;comment:发布时间"`
}

// AnnouncementContentEntity 公告内容实体
type AnnouncementContentEntity struct {
	model.BaseEntity
	AnnouncementId int64  `json:"announcement_id" g:"column:announcement_id;not null;comment:公告ID"`
	Language       string `json:"language" g:"column:language;not null;comment:语言代码"`
	Title          string `json:"title" g:"column:title;not null;comment:公告标题"`
	Content        string `json:"content" g:"column:content;not null;comment:公告内容"`
}
