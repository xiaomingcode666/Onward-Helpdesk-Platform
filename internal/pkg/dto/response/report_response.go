package response

type SupplierPerformanceResponse struct {
	SupplierID         string  `json:"supplier_id"`
	SupplierName       string  `json:"supplier_name"`
	SupplierNo         string  `json:"supplier_no"`
	TicketsAssigned    int64   `json:"tickets_assigned"`
	TicketsProcessing  int64   `json:"tickets_processing"`
	TicketsCompleted   int64   `json:"tickets_completed"`
	TicketsResponded   int64   `json:"tickets_responded"`
	AvgResponseMinutes float64 `json:"avg_response_minutes"`
	CompletionRate     float64 `json:"completion_rate"`
	AverageRating      float64 `json:"average_rating"`
	RatingCount        int64   `json:"rating_count"`
}
