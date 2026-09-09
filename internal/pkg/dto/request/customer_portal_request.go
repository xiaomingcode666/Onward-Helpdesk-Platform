package request

type CustomerPortalListRequest struct {
	Page           int
	Limit          int
	Locale         string
	Keyword        string
	Filter         string
	DeviceID       int64
	ConversationID int64
	TicketID       int64
	MeetingID      string
}

func (r CustomerPortalListRequest) GetPage() int {
	if r.Page <= 0 {
		return 1
	}
	return r.Page
}

func (r CustomerPortalListRequest) GetLimit() int {
	if r.Limit <= 0 {
		return 20
	}
	if r.Limit > 100 {
		return 100
	}
	return r.Limit
}

func (r CustomerPortalListRequest) Offset() int {
	return (r.GetPage() - 1) * r.GetLimit()
}
