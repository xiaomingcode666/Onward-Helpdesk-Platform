package response

// PlatformSystemIntroListResponse 系统介绍文档分页列表响应。
type PlatformSystemIntroListResponse struct {
	Items []PlatformSystemIntroDocResponse `json:"items"`
	Total int64                            `json:"total"`
}

// PlatformSystemIntroDocResponse 系统介绍文档管理端响应。
type PlatformSystemIntroDocResponse struct {
	ID             int64  `json:"id"`
	AssetID        int64  `json:"asset_id"`
	Title          string `json:"title"`
	Filename       string `json:"filename"`
	FileSize       int64  `json:"file_size"`
	MimeType       string `json:"mime_type"`
	SortNo         int    `json:"sort_no"`
	Status         string `json:"status"` // draft | published
	URL            string `json:"url"`
	PublishedAt    string `json:"published_at"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	CreateUserName string `json:"create_user_name"`
}
