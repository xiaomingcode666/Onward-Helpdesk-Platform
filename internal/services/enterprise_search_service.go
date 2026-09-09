package services

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var EnterpriseSearchService = newEnterpriseSearchService()

func newEnterpriseSearchService() *enterpriseSearchService {
	return &enterpriseSearchService{}
}

type enterpriseSearchService struct{}

func (s *enterpriseSearchService) Search(tenantID int64, query string, scopes []string, operator *dto.AuthPrincipal) (*dto.EnterpriseSearchResultsDTO, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	query = strings.TrimSpace(query)
	enabledScopes := normalizeEnterpriseSearchScopes(scopes)
	filterSearchScopesByPermission(enabledScopes, operator)
	visibleProductIDs, restrictedProducts := enterpriseSearchProductScope(tenantID, operator)
	result := &dto.EnterpriseSearchResultsDTO{
		Items:          []dto.EnterpriseSearchResultDTO{},
		Query:          query,
		Scopes:         sortedSearchScopes(enabledScopes),
		Suggestions:    []string{},
		RecentSearches: []string{},
	}
	if query == "" {
		return result, nil
	}
	if enabledScopes["ticket"] {
		result.Items = append(result.Items, s.searchTickets(tenantID, query, operator)...)
	}
	if enabledScopes["device"] {
		result.Items = append(result.Items, s.searchDevices(tenantID, query, visibleProductIDs, restrictedProducts)...)
	}
	if enabledScopes["knowledge"] {
		result.Items = append(result.Items, s.searchKnowledge(tenantID, query, visibleProductIDs, restrictedProducts)...)
	}
	if enabledScopes["product"] {
		result.Items = append(result.Items, s.searchProducts(tenantID, query, visibleProductIDs, restrictedProducts)...)
	}
	result.Total = len(result.Items)
	return result, nil
}

func (s *enterpriseSearchService) searchTickets(tenantID int64, query string, operator *dto.AuthPrincipal) []dto.EnterpriseSearchResultDTO {
	list, err := EnterpriseTicketService.ListForOperator(tenantID, EnterpriseTicketQuery{Page: 1, PageSize: 5, Search: query}, operator)
	if err != nil || list == nil {
		return nil
	}
	items := make([]dto.EnterpriseSearchResultDTO, 0, len(list.Items))
	for _, ticket := range list.Items {
		items = append(items, dto.EnterpriseSearchResultDTO{
			ID:          fmt.Sprintf("ticket:%d", ticket.ID),
			Type:        "ticket",
			Title:       fmt.Sprintf("%s %s", ticket.TicketNo, ticket.Title),
			Description: compactSearchDescription(ticket.CustomerName, ticket.ProductName, ticket.DeviceNo),
			URL:         fmt.Sprintf("/tickets/%d", ticket.ID),
			Metadata: map[string]string{
				"priority": ticket.Priority,
				"status":   ticket.Status,
			},
		})
	}
	return items
}

func (s *enterpriseSearchService) searchDevices(tenantID int64, query string, visibleProductIDs map[int64]bool, restricted bool) []dto.EnterpriseSearchResultDTO {
	devices, err := EnterpriseDeviceService.List(tenantID, EnterpriseDeviceQuery{Search: query, Page: 1, PageSize: 5})
	if err != nil || devices == nil {
		return nil
	}
	items := make([]dto.EnterpriseSearchResultDTO, 0, len(devices.Items))
	for _, device := range devices.Items {
		if restricted && !visibleProductIDs[device.ProductID] {
			continue
		}
		items = append(items, dto.EnterpriseSearchResultDTO{
			ID:          fmt.Sprintf("device:%d", device.ID),
			Type:        "device",
			Title:       device.DeviceNo,
			Description: compactSearchDescription(device.ProductName, device.ModelName, device.RegionCode),
			URL:         "/enterprise/devices?search=" + url.QueryEscape(device.DeviceNo),
			Metadata: map[string]string{
				"serialNo": device.SerialNo,
				"status":   device.Status,
			},
		})
		if len(items) == 5 {
			break
		}
	}
	return items
}

func (s *enterpriseSearchService) searchKnowledge(tenantID int64, query string, visibleProductIDs map[int64]bool, restricted bool) []dto.EnterpriseSearchResultDTO {
	list, err := EnterpriseKnowledgeService.ListEntries(tenantID, EnterpriseKnowledgeQuery{Page: 1, PageSize: 50, Search: query})
	if err != nil || list == nil {
		return nil
	}
	items := make([]dto.EnterpriseSearchResultDTO, 0, len(list.Items))
	for _, entry := range list.Items {
		if restricted && len(entry.ProductIDs) > 0 && !hasVisibleSearchProduct(entry.ProductIDs, visibleProductIDs) {
			continue
		}
		items = append(items, dto.EnterpriseSearchResultDTO{
			ID:          fmt.Sprintf("knowledge:%d", entry.ID),
			Type:        "knowledge",
			Title:       entry.Title,
			Description: compactSearchDescription(entry.Category, entry.ProductScope),
			URL:         fmt.Sprintf("/knowledge/%d", entry.ID),
			Metadata: map[string]string{
				"category": entry.Category,
				"status":   entry.Status,
			},
		})
		if len(items) == 5 {
			break
		}
	}
	return items
}

func (s *enterpriseSearchService) searchProducts(tenantID int64, query string, visibleProductIDs map[int64]bool, restricted bool) []dto.EnterpriseSearchResultDTO {
	keyword := "%" + query + "%"
	products := repositories.ProductRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		NotEq("status", enums.StatusDeleted).
		Where("name LIKE ? OR code LIKE ? OR category LIKE ?", keyword, keyword, keyword).
		Desc("updated_at").
		Desc("id"))
	items := make([]dto.EnterpriseSearchResultDTO, 0, len(products))
	for _, product := range products {
		if restricted && !visibleProductIDs[product.ID] {
			continue
		}
		items = append(items, s.buildProductResult(product))
		if len(items) == 5 {
			break
		}
	}
	return items
}

func (s *enterpriseSearchService) buildProductResult(product models.Product) dto.EnterpriseSearchResultDTO {
	productLine := ""
	if product.ProductLineID > 0 {
		if line := repositories.ProductLineRepository.Get(sqls.DB(), product.ProductLineID); line != nil {
			productLine = line.Name
		}
	}
	description := strings.TrimSpace(product.Description)
	if description == "" {
		description = compactSearchDescription(productLine, product.Category)
	}
	return dto.EnterpriseSearchResultDTO{
		ID:          fmt.Sprintf("product:%d", product.ID),
		Type:        "product",
		Title:       product.Name,
		Description: description,
		URL:         fmt.Sprintf("/products/%d", product.ID),
		Metadata: map[string]string{
			"code":        product.Code,
			"status":      enterpriseStatusText(product.Status),
			"productLine": productLine,
		},
	}
}

func normalizeEnterpriseSearchScopes(scopes []string) map[string]bool {
	defaults := map[string]bool{
		"ticket":    true,
		"device":    true,
		"customer":  true,
		"knowledge": true,
		"product":   true,
	}
	if len(scopes) == 0 {
		return defaults
	}
	result := map[string]bool{}
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if _, ok := defaults[scope]; ok {
			result[scope] = true
		}
	}
	if len(result) == 0 {
		return defaults
	}
	return result
}

func filterSearchScopesByPermission(scopes map[string]bool, operator *dto.AuthPrincipal) {
	permissions := map[string]string{
		"ticket":    constants.PermissionTicketView.Code,
		"device":    constants.PermissionDeviceView.Code,
		"knowledge": constants.PermissionKnowledgeBaseView.Code,
		"product":   constants.PermissionProductView.Code,
	}
	for scope, permission := range permissions {
		if operator == nil || !operator.HasPermission(permission) {
			delete(scopes, scope)
		}
	}
	delete(scopes, "customer")
}

func enterpriseSearchProductScope(tenantID int64, operator *dto.AuthPrincipal) (map[int64]bool, bool) {
	scope := resolveEnterpriseProductAccessScope(tenantID, operator)
	if !scope.Restricted {
		return nil, false
	}
	visible := make(map[int64]bool, len(scope.ProductIDs))
	for _, productID := range scope.ProductIDs {
		visible[productID] = true
	}
	return visible, true
}

func hasVisibleSearchProduct(productIDs []int64, visible map[int64]bool) bool {
	for _, productID := range productIDs {
		if visible[productID] {
			return true
		}
	}
	return false
}

func sortedSearchScopes(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func compactSearchDescription(values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " · ")
}
