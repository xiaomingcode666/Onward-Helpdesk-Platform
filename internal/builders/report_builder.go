package builders

import (
	"strconv"

	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/services"
)

func BuildSupplierPerformance(items []services.SupplierPerformanceAggregate) []response.SupplierPerformanceResponse {
	ret := make([]response.SupplierPerformanceResponse, 0, len(items))
	for i := range items {
		item := items[i]
		ret = append(ret, response.SupplierPerformanceResponse{
			SupplierID:         strconv.FormatInt(item.SupplierID, 10),
			SupplierName:       item.SupplierName,
			SupplierNo:         item.SupplierNo,
			TicketsAssigned:    item.TicketsAssigned,
			TicketsProcessing:  item.TicketsProcessing,
			TicketsCompleted:   item.TicketsCompleted,
			TicketsResponded:   item.TicketsResponded,
			AvgResponseMinutes: item.AvgResponseMinutes,
			CompletionRate:     item.CompletionRate,
			AverageRating:      item.AverageRating,
			RatingCount:        item.RatingCount,
		})
	}
	return ret
}
