package request

// PlatformSystemIntroUpdateRequest 系统介绍文档编辑请求。空值字段表示保持原值。
type PlatformSystemIntroUpdateRequest struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`   // 空 = 保持
	SortNo *int   `json:"sort_no"` // nil = 保持
	Status string `json:"status"`  // "draft" | "published"；空 = 保持
}

// PlatformSystemIntroDeleteRequest 系统介绍文档删除请求。
type PlatformSystemIntroDeleteRequest struct {
	ID int64 `json:"id"`
}
