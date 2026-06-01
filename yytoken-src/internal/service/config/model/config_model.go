package model

// GetConfigReq 获取配置请求（C端）
type GetConfigReq struct {
	KeyName string `json:"key_name" v:"required#键名不能为空" dc:"配置键名"`
}

// GetConfigRes 获取配置响应（C端）
type GetConfigRes struct {
	KeyName     string `json:"key_name" dc:"配置键名"`
	KeyValue    string `json:"key_value" dc:"配置值"`
	Description string `json:"description" dc:"配置描述"`
}

// GetConfigListReq 获取配置列表请求（管理端）
type GetConfigListReq struct {
	KeyName  string `json:"key_name" dc:"配置键名（模糊查询）"`
	Page     int    `json:"page" v:"min:1#页码必须大于0" dc:"页码"`
	PageSize int    `json:"page_size" v:"min:1|max:100#每页数量必须在1-100之间" dc:"每页数量"`
}

// ConfigListItem 配置列表项（管理端）
type ConfigListItem struct {
	Id          int64  `json:"id" dc:"配置ID"`
	KeyName     string `json:"key_name" dc:"配置键名"`
	KeyValue    string `json:"key_value" dc:"配置值"`
	Description string `json:"description" dc:"配置描述"`
	CreatedAt   string `json:"created_at" dc:"创建时间"`
	UpdatedAt   string `json:"updated_at" dc:"更新时间"`
}

// GetConfigListRes 获取配置列表响应（管理端）
type GetConfigListRes struct {
	List     []*ConfigListItem `json:"list" dc:"配置列表"`
	Total    int               `json:"total" dc:"总数"`
	Page     int               `json:"page" dc:"当前页"`
	PageSize int               `json:"page_size" dc:"每页数量"`
}

// GetConfigDetailRes 获取配置详情响应（管理端）
type GetConfigDetailRes struct {
	Id          int64  `json:"id" dc:"配置ID"`
	KeyName     string `json:"key_name" dc:"配置键名"`
	KeyValue    string `json:"key_value" dc:"配置值"`
	Description string `json:"description" dc:"配置描述"`
	CreatedAt   string `json:"created_at" dc:"创建时间"`
	UpdatedAt   string `json:"updated_at" dc:"更新时间"`
}

// UpdateConfigByIdReq 根据ID更新配置请求（管理端）
type UpdateConfigByIdReq struct {
	Id          int64  `json:"id" v:"required#配置ID不能为空" dc:"配置ID"`
	KeyValue    string `json:"key_value" v:"required#配置值不能为空" dc:"配置值"`
	Description string `json:"description" dc:"配置描述"`
}
