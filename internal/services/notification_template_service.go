package services

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

const (
	NotificationTemplateStatusDraft    = "draft"
	NotificationTemplateStatusApproved = "approved"
	NotificationTemplateStatusRetired  = "retired"

	NotificationTemplateChannelInApp = "in_app"
	NotificationTemplateChannelEmail = "email"

	NotificationDeliveryAttemptStatusSent    = "sent"
	NotificationDeliveryAttemptStatusFailed  = "failed"
	NotificationDeliveryAttemptStatusBlocked = "blocked"
	NotificationDeliveryAttemptStatusSkipped = "skipped"

	defaultNotificationTemplateLanguage = "zh-CN"
	platformNotificationTemplateTenant  = int64(0)

	// NotificationTemplateCodeGeneric 是通用兜底模板，任何通知类型都能用它渲染。
	NotificationTemplateCodeGeneric = "notification_generic"

	// 以下 code 用于工单状态变化时更新已有通知的文案。
	NotificationTemplateCodeTicketCreatedAssigned     = "ticket_created_assigned"
	NotificationTemplateCodeTicketAssignedTransferred = "ticket_assigned_transferred"
	NotificationTemplateCodeTicketAssignedAccepted    = "ticket_assigned_accepted"
	NotificationTemplateCodeTicketAssignedCancelled   = "ticket_assigned_cancelled"
	NotificationTemplateCodeTicketAssignedRecovered   = "ticket_assigned_recovered"

	// notificationTemplateMissingPrefix 标识「缺少已批准模板」导致的发送阻断。
	notificationTemplateMissingPrefix = "通知缺少已批准的模板"
)

var notificationTemplateSupportedChannels = []string{NotificationTemplateChannelInApp, NotificationTemplateChannelEmail, "wxwork", "sms"}
var notificationTemplateSupportedLanguages = []string{"zh-CN", "en-US", "es-ES"}

var notificationTemplateVariablePattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_]+)\s*\}\}`)

var NotificationTemplateService = newNotificationTemplateService()

func newNotificationTemplateService() *notificationTemplateService {
	return &notificationTemplateService{}
}

type notificationTemplateService struct{}

// NotificationTemplateListQuery 通知模板列表查询条件。
type NotificationTemplateListQuery struct {
	TenantID int64
	Code     string
	Channel  string
	Language string
	Status   string
	Keyword  string
	Page     int
	PageSize int
}

// NotificationDeliveryAttemptListQuery 发送尝试记录查询条件。
type NotificationDeliveryAttemptListQuery struct {
	TenantID   int64
	Channel    string
	Status     string
	DeliveryID int64
	Page       int
	PageSize   int
}

func (s *notificationTemplateService) List(query NotificationTemplateListQuery) (*response.NotificationTemplateListResponse, error) {
	if query.TenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	page, pageSize := normalizeNotificationTemplatePaging(query.Page, query.PageSize)
	cnd := sqls.NewCnd().
		In("tenant_id", notificationTemplateTenantScope(query.TenantID)).
		Eq("status", enums.StatusOk).
		Desc("id").
		Page(page, pageSize)
	if code := normalizeNotificationTemplateCode(query.Code); code != "" {
		cnd = cnd.Like("code", code)
	}
	if channel := normalizeNotificationTemplateChannel(query.Channel); channel != "" {
		cnd = cnd.Eq("channel", channel)
	}
	if language := strings.TrimSpace(query.Language); language != "" {
		cnd = cnd.Eq("language", normalizeNotificationTemplateLanguage(language))
	}
	if status := normalizeNotificationTemplateStatus(query.Status); status != "" {
		cnd = cnd.Eq("approval_status", status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		cnd = cnd.Where("(name LIKE ? OR code LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	items, paging := repositories.NotificationTemplateRepository.FindPageByCnd(sqls.DB(), cnd)
	result := &response.NotificationTemplateListResponse{
		Items:    make([]response.NotificationTemplateResponse, 0, len(items)),
		Page:     page,
		PageSize: pageSize,
	}
	if paging != nil {
		result.Total = paging.Total
	}
	for i := range items {
		result.Items = append(result.Items, buildNotificationTemplateResponse(&items[i], query.TenantID))
	}
	return result, nil
}

func (s *notificationTemplateService) Get(tenantID, id int64) (*response.NotificationTemplateResponse, error) {
	item, err := s.requireVisible(tenantID, id)
	if err != nil {
		return nil, err
	}
	ret := buildNotificationTemplateResponse(item, tenantID)
	return &ret, nil
}

func (s *notificationTemplateService) Create(tenantID, operatorID int64, req request.SaveNotificationTemplateRequest) (*response.NotificationTemplateResponse, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	code, channel, language, err := validateNotificationTemplateScope(req.Code, req.Channel, req.Language)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.TitleTemplate) == "" && strings.TrimSpace(req.ContentTemplate) == "" {
		return nil, errorsx.InvalidParam("title or content template is required")
	}
	if existing := s.findExact(tenantID, code, channel, language); existing != nil {
		return nil, errorsx.InvalidParam("同语言同渠道的模板 code 已存在，请直接编辑该模板")
	}
	now := time.Now()
	variables, err := encodeNotificationTemplateVariables(req.Variables, req.TitleTemplate, req.ContentTemplate)
	if err != nil {
		return nil, err
	}
	item := &models.NotificationTemplate{
		TenantID:        tenantID,
		Code:            code,
		Name:            strings.TrimSpace(req.Name),
		Channel:         channel,
		Language:        language,
		TitleTemplate:   strings.TrimSpace(req.TitleTemplate),
		ContentTemplate: strings.TrimSpace(req.ContentTemplate),
		VariablesJSON:   variables,
		ApprovalStatus:  NotificationTemplateStatusDraft,
		Status:          int(enums.StatusOk),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if item.Name == "" {
		item.Name = code
	}
	if err := repositories.NotificationTemplateRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}
	ret := buildNotificationTemplateResponse(item, tenantID)
	_ = operatorID
	return &ret, nil
}

func (s *notificationTemplateService) Update(tenantID, operatorID, id int64, req request.SaveNotificationTemplateRequest) (*response.NotificationTemplateResponse, error) {
	item, err := s.requireOwned(tenantID, id)
	if err != nil {
		return nil, err
	}
	code, channel, language, err := validateNotificationTemplateScope(req.Code, req.Channel, req.Language)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.TitleTemplate) == "" && strings.TrimSpace(req.ContentTemplate) == "" {
		return nil, errorsx.InvalidParam("title or content template is required")
	}
	if existing := s.findExact(tenantID, code, channel, language); existing != nil && existing.ID != item.ID {
		return nil, errorsx.InvalidParam("同语言同渠道的模板 code 已存在")
	}
	variables, err := encodeNotificationTemplateVariables(req.Variables, req.TitleTemplate, req.ContentTemplate)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = code
	}
	now := time.Now()
	// 内容一旦修改就回到草稿状态，必须重新批准后才能用于发送。
	columns := map[string]any{
		"code":             code,
		"name":             name,
		"channel":          channel,
		"language":         language,
		"title_template":   strings.TrimSpace(req.TitleTemplate),
		"content_template": strings.TrimSpace(req.ContentTemplate),
		"variables_json":   variables,
		"approval_status":  NotificationTemplateStatusDraft,
		"approved_by":      0,
		"approved_at":      nil,
		"updated_at":       now,
	}
	if err := repositories.NotificationTemplateRepository.Updates(sqls.DB(), item.ID, columns); err != nil {
		return nil, err
	}
	updated := repositories.NotificationTemplateRepository.Get(sqls.DB(), item.ID)
	if updated == nil {
		return nil, errorsx.InvalidParam("通知模板不存在")
	}
	ret := buildNotificationTemplateResponse(updated, tenantID)
	_ = operatorID
	return &ret, nil
}

func (s *notificationTemplateService) Approve(tenantID, operatorID, id int64) (*response.NotificationTemplateResponse, error) {
	item, err := s.requireOwned(tenantID, id)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.TitleTemplate) == "" && strings.TrimSpace(item.ContentTemplate) == "" {
		return nil, errorsx.InvalidParam("模板内容为空，无法批准")
	}
	now := time.Now()
	if err := repositories.NotificationTemplateRepository.Updates(sqls.DB(), item.ID, map[string]any{
		"approval_status": NotificationTemplateStatusApproved,
		"approved_by":     operatorID,
		"approved_at":     now,
		"updated_at":      now,
	}); err != nil {
		return nil, err
	}
	updated := repositories.NotificationTemplateRepository.Get(sqls.DB(), item.ID)
	if updated == nil {
		return nil, errorsx.InvalidParam("通知模板不存在")
	}
	ret := buildNotificationTemplateResponse(updated, tenantID)
	return &ret, nil
}

func (s *notificationTemplateService) Retire(tenantID, operatorID, id int64) (*response.NotificationTemplateResponse, error) {
	item, err := s.requireOwned(tenantID, id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := repositories.NotificationTemplateRepository.Updates(sqls.DB(), item.ID, map[string]any{
		"approval_status": NotificationTemplateStatusRetired,
		"updated_at":      now,
	}); err != nil {
		return nil, err
	}
	updated := repositories.NotificationTemplateRepository.Get(sqls.DB(), item.ID)
	if updated == nil {
		return nil, errorsx.InvalidParam("通知模板不存在")
	}
	ret := buildNotificationTemplateResponse(updated, tenantID)
	_ = operatorID
	return &ret, nil
}

func (s *notificationTemplateService) Delete(tenantID, id int64) error {
	if _, err := s.requireOwned(tenantID, id); err != nil {
		return err
	}
	return repositories.NotificationTemplateRepository.Delete(sqls.DB(), id)
}

// Preview 渲染模板并给出敏感信息检查结果，供管理员在批准前自查。
func (s *notificationTemplateService) Preview(tenantID int64, req request.PreviewNotificationTemplateRequest) (*response.NotificationTemplatePreviewResponse, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	language := normalizeNotificationTemplateLanguage(req.Language)
	data := map[string]string{"Language": language}
	for key, value := range req.Variables {
		data[strings.TrimSpace(key)] = value
	}
	title := strings.TrimSpace(RenderNotificationTemplateText(req.TitleTemplate, data))
	content := strings.TrimSpace(RenderNotificationTemplateText(req.ContentTemplate, data))
	result := &response.NotificationTemplatePreviewResponse{Title: title, Content: content}
	if finding := ScanNotificationSensitiveContent(title + "\n" + content); finding != nil {
		result.Blocked = true
		result.RuleCode = finding.Code
		result.RuleLabel = finding.Label
	}
	return result, nil
}

// SeedDrafts 用平台标准模板给租户生成一份草稿，避免管理员从空白开始。
func (s *notificationTemplateService) SeedDrafts(tenantID, operatorID int64) (int, error) {
	if tenantID <= 0 {
		return 0, errorsx.InvalidParam("tenantId is required")
	}
	created := 0
	now := time.Now()
	for _, seed := range defaultNotificationTemplateSeeds() {
		language := seed.Language
		if existing := s.findExact(tenantID, seed.Code, seed.Channel, language); existing != nil {
			continue
		}
		variables, err := encodeNotificationTemplateVariables(nil, seed.Title, seed.Body)
		if err != nil {
			return created, err
		}
		item := &models.NotificationTemplate{
			TenantID:        tenantID,
			Code:            seed.Code,
			Name:            seed.Name,
			Channel:         seed.Channel,
			Language:        language,
			TitleTemplate:   seed.Title,
			ContentTemplate: seed.Body,
			VariablesJSON:   variables,
			ApprovalStatus:  NotificationTemplateStatusDraft,
			Status:          int(enums.StatusOk),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := repositories.NotificationTemplateRepository.Create(sqls.DB(), item); err != nil {
			return created, err
		}
		created++
	}
	_ = operatorID
	return created, nil
}

// ApplyToNotification 在通知落库前套用「已批准」的站内信模板，并按接收人语言渲染。
// 通知必须由已批准模板生成：指定 code 没有模板时回退到通用模板；
// 连通用模板都没有就阻断本次发送，并写入一条发送尝试记录，而不是直接发原文。
func (s *notificationTemplateService) ApplyToNotification(item *models.Notification, recipientUserID int64, variables map[string]string) error {
	if item == nil || item.TenantID <= 0 {
		return nil
	}
	code := normalizeNotificationTemplateCode(item.TemplateCode)
	if code == "" {
		code = normalizeNotificationTemplateCode(item.NotificationType)
	}
	if code == "" {
		code = NotificationTemplateCodeGeneric
	}
	language := s.RecipientLanguage(item.TenantID, recipientUserID)
	item.Language = language
	tpl := s.ResolveApproved(item.TenantID, code, NotificationTemplateChannelInApp, language)
	if tpl == nil && code != NotificationTemplateCodeGeneric {
		// 该通知类型没有专属模板时，用已批准的通用模板渲染，保证「每条通知都来自已批准模板」。
		tpl = s.ResolveApproved(item.TenantID, NotificationTemplateCodeGeneric, NotificationTemplateChannelInApp, language)
	}
	if tpl == nil {
		detail := "缺少已批准的站内信模板：code=" + code + "，语言=" + language
		NotificationDeliveryAttemptService.RecordBlockedWithReason(item, NotificationTemplateChannelInApp, "no_approved_template", detail)
		return errorsx.InvalidParam(notificationTemplateMissingPrefix + "：" + detail + "，已阻断发送")
	}
	data := notificationTemplateRenderData(item, language)
	for key, value := range variables {
		data[strings.TrimSpace(key)] = value
	}
	title := strings.TrimSpace(RenderNotificationTemplateText(tpl.TitleTemplate, data))
	content := strings.TrimSpace(RenderNotificationTemplateText(tpl.ContentTemplate, data))
	if title == "" {
		title = item.Title
	}
	if content == "" {
		content = item.Content
	}
	item.TemplateID = tpl.ID
	item.TemplateCode = tpl.Code
	item.Language = tpl.Language
	if finding := ScanNotificationSensitiveContent(title + "\n" + content); finding != nil {
		NotificationDeliveryAttemptService.RecordBlocked(item, NotificationTemplateChannelInApp, finding, tpl.Code, tpl.Language)
		return errorsx.InvalidParam(notificationSensitiveBlockedPrefix + "：" + finding.Label + "，已阻断发送")
	}
	item.Title = title
	item.Content = content
	return nil
}

// ResolveApproved 挑选可用于发送的模板：语言匹配优先，其次租户自定义覆盖平台默认。
func (s *notificationTemplateService) ResolveApproved(tenantID int64, code, channel, language string) *models.NotificationTemplate {
	code = normalizeNotificationTemplateCode(code)
	channel = normalizeNotificationTemplateChannel(channel)
	if code == "" || channel == "" {
		return nil
	}
	language = normalizeNotificationTemplateLanguage(language)
	items := repositories.NotificationTemplateRepository.Find(sqls.DB(), sqls.NewCnd().
		In("tenant_id", notificationTemplateTenantScope(tenantID)).
		Eq("code", code).
		Eq("channel", channel).
		Eq("approval_status", NotificationTemplateStatusApproved).
		Eq("status", enums.StatusOk))
	var best *models.NotificationTemplate
	bestScore := -1
	for i := range items {
		candidate := &items[i]
		if strings.TrimSpace(candidate.TitleTemplate) == "" && strings.TrimSpace(candidate.ContentTemplate) == "" {
			continue
		}
		score := 0
		switch {
		case candidate.Language == language:
			score += 10
		case candidate.Language == defaultNotificationTemplateLanguage:
			score += 5
		}
		if candidate.TenantID == tenantID {
			score += 2
		}
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	return best
}

// RecipientLanguage 读取接收人语言，缺省按默认语言处理。
func (s *notificationTemplateService) RecipientLanguage(_ int64, userID int64) string {
	if userID <= 0 {
		return defaultNotificationTemplateLanguage
	}
	user := repositories.UserRepository.Get(sqls.DB(), userID)
	if user == nil {
		return defaultNotificationTemplateLanguage
	}
	return normalizeNotificationTemplateLanguage(user.Locale)
}

func (s *notificationTemplateService) ListAttempts(query NotificationDeliveryAttemptListQuery) (*response.NotificationDeliveryAttemptListResponse, error) {
	if query.TenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	page, pageSize := normalizeNotificationTemplatePaging(query.Page, query.PageSize)
	cnd := sqls.NewCnd().Eq("tenant_id", query.TenantID).Desc("id").Page(page, pageSize)
	if deliveryID := query.DeliveryID; deliveryID > 0 {
		cnd = cnd.Eq("delivery_id", deliveryID)
	}
	if channel := normalizeNotificationTemplateChannel(query.Channel); channel != "" {
		cnd = cnd.Eq("channel", channel)
	}
	if status := strings.ToLower(strings.TrimSpace(query.Status)); status != "" {
		cnd = cnd.Eq("status", status)
	}
	items, paging := repositories.NotificationDeliveryAttemptRepository.FindPageByCnd(sqls.DB(), cnd)
	result := &response.NotificationDeliveryAttemptListResponse{
		Items:    make([]response.NotificationDeliveryAttemptResponse, 0, len(items)),
		Page:     page,
		PageSize: pageSize,
	}
	if paging != nil {
		result.Total = paging.Total
	}
	deliveries := repositories.NotificationDeliveryRepository.FindByIDs(sqls.DB(), notificationDeliveryIDs(items))
	for i := range items {
		row := response.NotificationDeliveryAttemptResponse{
			ID:         items[i].ID,
			TenantID:   items[i].TenantID,
			DeliveryID: items[i].DeliveryID,
			AttemptNo:  items[i].AttemptNo,
			Channel:    items[i].Channel,
			Status:     items[i].Status,
			Reason:     items[i].Reason,
			Detail:     items[i].Detail,
			CreatedAt:  formatNotificationTime(items[i].CreatedAt),
		}
		if delivery, ok := deliveries[items[i].DeliveryID]; ok {
			row.NotificationID = delivery.NotificationID
			row.TemplateCode = delivery.TemplateCode
			row.Language = delivery.Language
			row.RecipientID = delivery.RecipientID
		}
		result.Items = append(result.Items, row)
	}
	return result, nil
}

func (s *notificationTemplateService) findExact(tenantID int64, code, channel, language string) *models.NotificationTemplate {
	items := repositories.NotificationTemplateRepository.Find(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", tenantID).
		Eq("code", code).
		Eq("channel", channel).
		Eq("language", language).
		Eq("status", enums.StatusOk).
		Limit(1))
	if len(items) == 0 {
		return nil
	}
	return &items[0]
}

func (s *notificationTemplateService) requireOwned(tenantID, id int64) (*models.NotificationTemplate, error) {
	item := repositories.NotificationTemplateRepository.Get(sqls.DB(), id)
	if item == nil || item.Status != int(enums.StatusOk) {
		return nil, errorsx.InvalidParam("通知模板不存在")
	}
	if tenantID <= 0 || item.TenantID != tenantID {
		return nil, errorsx.Forbidden("只能维护本租户的通知模板")
	}
	return item, nil
}

func (s *notificationTemplateService) requireVisible(tenantID, id int64) (*models.NotificationTemplate, error) {
	item := repositories.NotificationTemplateRepository.Get(sqls.DB(), id)
	if item == nil || item.Status != int(enums.StatusOk) {
		return nil, errorsx.InvalidParam("通知模板不存在")
	}
	if tenantID <= 0 || (item.TenantID != tenantID && item.TenantID != platformNotificationTemplateTenant) {
		return nil, errorsx.Forbidden("通知模板不在当前租户范围内")
	}
	return item, nil
}

func notificationTemplateTenantScope(tenantID int64) []int64 {
	if tenantID <= 0 {
		return []int64{platformNotificationTemplateTenant}
	}
	return []int64{platformNotificationTemplateTenant, tenantID}
}

func notificationTemplateRenderData(item *models.Notification, language string) map[string]string {
	data := map[string]string{
		"Title":            item.Title,
		"Content":          item.Content,
		"RecipientName":    item.RecipientName,
		"ActionURL":        item.ActionURL,
		"NotificationType": item.NotificationType,
		"Category":         item.Category,
		"Level":            item.Level,
		"Language":         language,
	}
	if item.BizID > 0 {
		data["BizID"] = fmt.Sprintf("%d", item.BizID)
	}
	return data
}

func buildNotificationTemplateResponse(item *models.NotificationTemplate, tenantID int64) response.NotificationTemplateResponse {
	source := "platform"
	if item.TenantID > 0 && item.TenantID == tenantID {
		source = "tenant"
	}
	ret := response.NotificationTemplateResponse{
		ID:              item.ID,
		TenantID:        item.TenantID,
		Code:            item.Code,
		Name:            item.Name,
		Channel:         item.Channel,
		Language:        item.Language,
		TitleTemplate:   item.TitleTemplate,
		ContentTemplate: item.ContentTemplate,
		Variables:       decodeNotificationTemplateVariables(item.VariablesJSON),
		ApprovalStatus:  normalizeNotificationTemplateStatus(item.ApprovalStatus),
		ApprovedBy:      item.ApprovedBy,
		Editable:        source == "tenant",
		Source:          source,
		UpdatedAt:       formatNotificationTime(item.UpdatedAt),
	}
	if item.ApprovedAt != nil {
		ret.ApprovedAt = formatNotificationTime(*item.ApprovedAt)
	}
	return ret
}

func formatNotificationTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func notificationDeliveryIDs(items []models.NotificationDeliveryAttempt) []int64 {
	ret := make([]int64, 0, len(items))
	for i := range items {
		if items[i].DeliveryID > 0 {
			ret = append(ret, items[i].DeliveryID)
		}
	}
	return ret
}

func normalizeNotificationTemplatePaging(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func normalizeNotificationTemplateCode(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeNotificationTemplateChannel(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeNotificationTemplateLanguage(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return defaultNotificationTemplateLanguage
	}
	switch {
	case strings.HasPrefix(strings.ToLower(normalized), "zh"):
		return "zh-CN"
	case strings.HasPrefix(strings.ToLower(normalized), "en"):
		return "en-US"
	case strings.HasPrefix(strings.ToLower(normalized), "es"):
		return "es-ES"
	default:
		return normalized
	}
}

func normalizeNotificationTemplateStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case NotificationTemplateStatusApproved:
		return NotificationTemplateStatusApproved
	case NotificationTemplateStatusRetired:
		return NotificationTemplateStatusRetired
	case NotificationTemplateStatusDraft:
		return NotificationTemplateStatusDraft
	default:
		return ""
	}
}

func validateNotificationTemplateScope(code, channel, language string) (string, string, string, error) {
	normalizedCode := normalizeNotificationTemplateCode(code)
	if normalizedCode == "" {
		return "", "", "", errorsx.InvalidParam("模板 code 不能为空，例如 ticket_assigned")
	}
	if len(normalizedCode) > 64 {
		return "", "", "", errorsx.InvalidParam("模板 code 过长")
	}
	normalizedChannel := normalizeNotificationTemplateChannel(channel)
	if normalizedChannel == "" {
		return "", "", "", errorsx.InvalidParam("模板渠道不能为空，例如 in_app 或 email")
	}
	if !containsNotificationTemplateValue(notificationTemplateSupportedChannels, normalizedChannel) {
		return "", "", "", errorsx.InvalidParam("不支持的模板渠道：" + normalizedChannel)
	}
	normalizedLanguage := normalizeNotificationTemplateLanguage(language)
	if !containsNotificationTemplateValue(notificationTemplateSupportedLanguages, normalizedLanguage) {
		return "", "", "", errorsx.InvalidParam("不支持的模板语言：" + normalizedLanguage)
	}
	return normalizedCode, normalizedChannel, normalizedLanguage, nil
}

func containsNotificationTemplateValue(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

// RenderNotificationTemplateText 用 {{变量}} 占位符渲染模板，未提供的变量保持原样以便排查。
func RenderNotificationTemplateText(text string, data map[string]string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return notificationTemplateVariablePattern.ReplaceAllStringFunc(text, func(match string) string {
		parts := notificationTemplateVariablePattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		key := strings.TrimSpace(parts[1])
		if value, ok := data[key]; ok {
			return value
		}
		if value, ok := data[strings.ToLower(key)]; ok {
			return value
		}
		return match
	})
}

func encodeNotificationTemplateVariables(declared []string, texts ...string) (string, error) {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, name := range declared {
		value := strings.TrimSpace(name)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	for _, text := range texts {
		for _, match := range notificationTemplateVariablePattern.FindAllStringSubmatch(text, -1) {
			if len(match) < 2 {
				continue
			}
			value := strings.TrimSpace(match[1])
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			result = append(result, value)
		}
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return "[]", err
	}
	return string(raw), nil
}

func decodeNotificationTemplateVariables(raw string) []string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return []string{}
	}
	result := make([]string, 0)
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return []string{}
	}
	return result
}
