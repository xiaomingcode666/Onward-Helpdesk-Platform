package enterprise

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func setupEnterpriseContractDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	sqls.SetDB(db)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	modelsToMigrate := []any{
		&models.User{},
		&models.Tenant{},
		&models.ProductLine{},
		&models.Product{},
		&models.ProductServiceProfile{},
		&models.ProductKnowledgeBinding{},
		&models.ProductModel{},
		&models.Device{},
		&models.DeviceWarrantyRecord{},
		&models.DeviceSoftwareVersion{},
		&models.ServiceCodeBatch{},
		&models.ServiceCode{},
		&models.CustomerOrg{},
		&models.CustomerUser{},
		&models.CustomerRegistrationGrant{},
		&models.TenantMember{},
		&models.Department{},
		&models.AgentTeam{},
		&models.AgentTeamMember{},
		&models.AgentProfile{},
		&models.EngineerProfile{},
		&models.PartnerCompany{},
		&models.PartnerAccount{},
		&models.PartnerContract{},
		&models.AuthRole{},
		&models.AuthRoleBinding{},
		&models.AuthRolePermission{},
		&models.AuthAuditLog{},
		&models.AuditLog{},
		&models.CustomerDeviceBinding{},
		&models.Company{},
		&models.Customer{},
		&models.User{},
		&models.Conversation{},
		&models.Message{},
		&models.Asset{},
		&models.Ticket{},
		&models.TicketProgress{},
		&models.TicketRepairRecord{},
		&models.TicketFeedback{},
		&models.DeviceServiceRecord{},
		&models.MeetingRoomJitsi{},
		&models.MeetingParticipant{},
		&models.DiagnosisSession{},
		&models.FaultTreeNode{},
		&models.Notification{},
		&models.AIConfig{},
		&models.AIAgent{},
		&models.AIWorkflow{},
		&models.AIWorkflowVersion{},
		&models.KnowledgeBase{},
		&models.KnowledgeDocument{},
		&models.KnowledgeFAQ{},
		&models.KnowledgeRevision{},
		&models.ProductManualFile{},
		&models.ProductKnowledgeLink{},
		&models.KnowledgeCandidate{},
		&models.ProductQualitySignal{},
		&models.Sub2APITenantAccount{},
		&models.TenantSubscription{},
		&models.TenantQuotaOverride{},
		&models.ProductAIUsageCredential{},
		&models.ProductAIUsageEvent{},
		&models.ProductResourceProvisioningJob{},
		&models.SystemConfig{},
		&models.ProductModule{},
		&models.ProductModuleModelLink{},
		&models.ProductFaultStatsDaily{},
		&models.FaultStatsEventInbox{},
		&models.ProductFaultStatsRebuildJob{},
		&models.KnowledgeIndexSyncTask{},
		&models.DomainEvent{},
		&models.OutboxRecord{},
		&services.SLAPolicy{},
		&services.SLAPauseRecord{},
		&services.SLAViolation{},
		&services.ServiceCalendar{},
		&services.EscalationRule{},
		&models.DSARRequest{},
		&models.DSARExecutionLog{},
		&models.DataBreachRecord{},
		&models.DataRegionPolicy{},
	}
	for _, model := range modelsToMigrate {
		if err := db.AutoMigrate(model); err != nil {
			if strings.Contains(err.Error(), "index idx_tenant_id already exists") {
				continue
			}
			t.Fatalf("auto migrate %T: %v", model, err)
		}
	}

	return db
}

func TestEnterpriseFaultTreeManagementLifecycle(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1088)

	createCtx, createRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		"/api/enterprise/v1/diagnosis/fault-tree-nodes",
		1088,
		fmt.Sprintf(`{"product_id":%d,"title":"无法启动","description":"设备上电后无响应","node_type":"symptom","fault_pattern":"persistent","risk_level":"high","trigger_conditions":"[\"no-power\"]","order_index":10}`, product.ID),
	)
	DiagnosisFaultTreeNodeCreate(createCtx)
	var created dto.EnterpriseFaultTreeNodeDTO
	decodeEnterpriseData(t, createRec, &created)
	if created.ID == "" || created.ProductID != product.ID || created.Status != "draft" {
		t.Fatalf("unexpected created fault tree node: %+v", created)
	}

	updateCtx, updateRec := newEnterpriseContractJSONContext(
		http.MethodPatch,
		"/api/enterprise/v1/diagnosis/fault-tree-nodes/"+created.ID,
		1088,
		`{"title":"设备无法启动","status":"published"}`,
		gin.Param{Key: "nodeId", Value: created.ID},
	)
	DiagnosisFaultTreeNodeUpdate(updateCtx)
	var updated dto.EnterpriseFaultTreeNodeDTO
	decodeEnterpriseData(t, updateRec, &updated)
	if updated.Title != "设备无法启动" || updated.Status != "published" {
		t.Fatalf("unexpected updated fault tree node: %+v", updated)
	}

	listCtx, listRec := newEnterpriseContractContext(
		http.MethodGet,
		fmt.Sprintf("/api/enterprise/v1/diagnosis/fault-tree-nodes?product_id=%d", product.ID),
		1088,
	)
	DiagnosisFaultTreeNodeList(listCtx)
	var rows []dto.EnterpriseFaultTreeNodeDTO
	decodeEnterpriseData(t, listRec, &rows)
	if len(rows) != 1 || rows[0].ID != created.ID || rows[0].TenantID != 1088 {
		t.Fatalf("unexpected fault tree list: %+v", rows)
	}
}

func TestEnterpriseSLAHandlersIgnoreRequestTenantAndRejectForeignResources(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	now := time.Now()

	createCtx, createRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		"/api/enterprise/v1/sla/policy/create",
		1,
		`{"tenantId":"2","name":"Tenant 1 P1","priority":"p1","frtMinutes":10,"assignmentMinutes":20,"resolutionMinutes":60}`,
	)
	SlaPostPolicyCreate(createCtx)
	var created dto.EnterpriseSLAPolicyDTO
	decodeEnterpriseData(t, createRec, &created)
	if created.TenantID != "1" {
		t.Fatalf("created policy tenant = %q, want authenticated tenant 1", created.TenantID)
	}

	foreignPolicy := services.SLAPolicy{
		ID: "foreign-policy", TenantID: "2", Name: "Tenant 2 P2", Priority: "p2",
		FRTMinutes: 10, Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&foreignPolicy).Error; err != nil {
		t.Fatalf("create foreign policy: %v", err)
	}
	updateCtx, updateRec := newEnterpriseContractJSONContext(
		http.MethodPatch,
		"/api/enterprise/v1/sla/policy/"+foreignPolicy.ID,
		1,
		`{"name":"overwritten"}`,
		gin.Param{Key: "id", Value: foreignPolicy.ID},
	)
	SlaPatchPolicyUpdate(updateCtx)
	if envelope := decodeEnterpriseEnvelope(t, updateRec); envelope.Success {
		t.Fatalf("cross-tenant policy update succeeded: %s", updateRec.Body.String())
	}
	var persisted services.SLAPolicy
	if err := db.Where("id = ?", foreignPolicy.ID).First(&persisted).Error; err != nil {
		t.Fatalf("reload foreign policy: %v", err)
	}
	if persisted.Name != foreignPolicy.Name {
		t.Fatalf("foreign policy changed to %q", persisted.Name)
	}

	foreignTicket := models.Ticket{
		TicketNo: "SLA-HANDLER-T2", TenantID: 2, Status: enums.TicketStatusInProgress,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&foreignTicket).Error; err != nil {
		t.Fatalf("create foreign ticket: %v", err)
	}
	pauseCtx, pauseRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		"/api/enterprise/v1/sla/pause",
		1,
		fmt.Sprintf(`{"ticketId":"%d","reason":"waiting_customer"}`, foreignTicket.ID),
	)
	SlaPostPause(pauseCtx)
	if envelope := decodeEnterpriseEnvelope(t, pauseRec); envelope.Success {
		t.Fatalf("cross-tenant SLA pause succeeded: %s", pauseRec.Body.String())
	}
	var pauseCount int64
	if err := db.Model(&services.SLAPauseRecord{}).Where("ticket_id = ?", fmt.Sprint(foreignTicket.ID)).Count(&pauseCount).Error; err != nil {
		t.Fatalf("count pauses: %v", err)
	}
	if pauseCount != 0 {
		t.Fatalf("cross-tenant pause records = %d, want 0", pauseCount)
	}
}

func TestEnterprisePrivacyHandlersIgnoreRequestTenantAndRejectForeignResources(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	now := time.Now()
	customer := models.Customer{
		Name: "Privacy Customer", PrimaryEmail: "privacy@example.com", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	ticket := models.Ticket{
		TicketNo: "PRIVACY-HANDLER-T1", TenantID: 1, CustomerID: customer.ID,
		Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create customer ticket: %v", err)
	}

	submitCtx, submitRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		"/api/enterprise/v1/gdpr/dsar/submit",
		1,
		fmt.Sprintf(`{"tenantId":"2","subjectType":"customer","subjectId":"%d","subjectEmail":"privacy@example.com","requestType":"access"}`, customer.ID),
	)
	GdprPostDsarSubmit(submitCtx)
	if envelope := decodeEnterpriseEnvelope(t, submitRec); !envelope.Success {
		t.Fatalf("owned DSAR submission failed: %s", submitRec.Body.String())
	}
	var created models.DSARRequest
	if err := db.Order("requested_at DESC").First(&created).Error; err != nil {
		t.Fatalf("load created DSAR: %v", err)
	}
	if created.TenantID != "1" {
		t.Fatalf("created DSAR tenant = %q, want authenticated tenant 1", created.TenantID)
	}

	foreign := models.DSARRequest{
		ID: "foreign-dsar", TenantID: "2", SubjectType: "customer", SubjectID: fmt.Sprint(customer.ID),
		RequestType: "access", Status: "pending", DeadlineAt: now.AddDate(0, 0, 30), RequestedAt: now,
	}
	if err := db.Create(&foreign).Error; err != nil {
		t.Fatalf("create foreign DSAR: %v", err)
	}
	statusCtx, statusRec := newEnterpriseContractContext(
		http.MethodGet,
		"/api/enterprise/v1/gdpr/dsar/"+foreign.ID+"/status",
		1,
		gin.Param{Key: "id", Value: foreign.ID},
	)
	GdprGetDsarStatus(statusCtx)
	if envelope := decodeEnterpriseEnvelope(t, statusRec); envelope.Success {
		t.Fatalf("cross-tenant DSAR status read succeeded: %s", statusRec.Body.String())
	}

	breachCtx, breachRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		"/api/enterprise/v1/gdpr/breach/report",
		1,
		`{"tenantId":"2","breachType":"data_leak","description":"credential exposure","affectedUsersCount":1}`,
	)
	GdprPostBreachReport(breachCtx)
	if envelope := decodeEnterpriseEnvelope(t, breachRec); !envelope.Success {
		t.Fatalf("breach report failed: %s", breachRec.Body.String())
	}
	var breach models.DataBreachRecord
	if err := db.Order("detected_at DESC").First(&breach).Error; err != nil {
		t.Fatalf("load breach: %v", err)
	}
	if breach.TenantID != "1" {
		t.Fatalf("created breach tenant = %q, want authenticated tenant 1", breach.TenantID)
	}
}

func newEnterpriseContractContext(method string, path string, tenantID int64, params ...gin.Param) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(method, path, nil)
	ctx.Set("tenantId", fmt.Sprint(tenantID))
	principal := &dto.AuthPrincipal{
		UserID:      0,
		Username:    "enterprise-contract-test",
		TenantID:    tenantID,
		Status:      enums.StatusOk,
		Permissions: constants.PermissionCodes(),
	}
	ctx.Set("authPrincipal", principal)
	ctx.Set("middlewareAuthPrincipal", principal)
	ctx.Params = params
	return ctx, rec
}

func newEnterpriseContractJSONContext(method string, path string, tenantID int64, body string, params ...gin.Param) (*gin.Context, *httptest.ResponseRecorder) {
	ctx, rec := newEnterpriseContractContext(method, path, tenantID, params...)
	ctx.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, rec
}

func decodeEnterpriseData(t *testing.T, rec *httptest.ResponseRecorder, target any) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\nbody=%s", err, rec.Body.String())
	}
	if !envelope.Success {
		t.Fatalf("success = false, message = %q, body = %s", envelope.Message, rec.Body.String())
	}
	if err := json.Unmarshal(envelope.Data, target); err != nil {
		t.Fatalf("decode data: %v\nbody=%s", err, rec.Body.String())
	}
}

func assertEnterpriseEnvelopeError(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if envelope := decodeEnterpriseEnvelope(t, rec); envelope.Success {
		t.Fatalf("success = true, want false, body = %s", rec.Body.String())
	}
}

func decodeEnterpriseEnvelope(t *testing.T, rec *httptest.ResponseRecorder) struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
} {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\nbody=%s", err, rec.Body.String())
	}
	return envelope
}

func seedEnterpriseProductOwner(t *testing.T, db *gorm.DB, tenantID, userID int64, displayName string) *models.TenantMember {
	t.Helper()
	now := time.Now()
	user := models.User{
		ID:          userID,
		Username:    fmt.Sprintf("product-owner-%d", userID),
		Nickname:    displayName,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create product owner user: %v", err)
	}
	member := models.TenantMember{
		TenantID:    tenantID,
		UserID:      user.ID,
		DisplayName: displayName,
		MemberNo:    fmt.Sprintf("PO-%d", userID),
		JobTitle:    "Product Owner",
		MemberType:  "employee",
		Status:      enums.StatusOk,
		JoinedAt:    &now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create product owner member: %v", err)
	}
	return &member
}

func seedEnterpriseProductContract(t *testing.T, db *gorm.DB, tenantID int64) models.Product {
	t.Helper()
	now := time.Now()
	tenant := models.Tenant{
		ID:            tenantID,
		Name:          fmt.Sprintf("Tenant %d", tenantID),
		Industry:      "machinery",
		CountryRegion: "US",
		DefaultLocale: "en-US",
		Timezone:      "UTC",
		Status:        enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Where("id = ?", tenantID).FirstOrCreate(&tenant).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	product := models.Product{
		TenantID:      tenantID,
		Code:          "HP-3000",
		Name:          "Hydraulic Power Unit HP-3000",
		Category:      "hydraulic",
		DefaultLocale: "en-US",
		Status:        enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	return product
}

func buildEnterpriseTestDOCX(t *testing.T, paragraphs ...string) []byte {
	t.Helper()

	buf := &bytes.Buffer{}
	writer := zip.NewWriter(buf)
	doc, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create docx document.xml: %v", err)
	}

	var body strings.Builder
	body.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	body.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, paragraph := range paragraphs {
		body.WriteString(`<w:p><w:r><w:t>`)
		body.WriteString(paragraph)
		body.WriteString(`</w:t></w:r></w:p>`)
	}
	body.WriteString(`</w:body></w:document>`)

	if _, err := doc.Write([]byte(body.String())); err != nil {
		t.Fatalf("write docx xml: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close docx writer: %v", err)
	}
	return buf.Bytes()
}

func TestEnterpriseProductModelCreateIsolatesTenant(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	productA := seedEnterpriseProductContract(t, db, 11001)
	productB := seedEnterpriseProductContract(t, db, 11002)

	ctx, rec := newEnterpriseContractJSONContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/models",
		11002,
		`{"model_code":"HP-X","name":"Cross Tenant Model"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(productA.ID)},
	)
	ProductModelCreate(ctx)
	envelope := decodeEnterpriseEnvelope(t, rec)
	if envelope.Success {
		t.Fatalf("cross-tenant model create success = true, want false")
	}

	ctx, rec = newEnterpriseContractJSONContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/models",
		11002,
		`{"model_code":"HP-B","name":"Tenant B Model"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(productB.ID)},
	)
	ProductModelCreate(ctx)
	var created map[string]any
	decodeEnterpriseData(t, rec, &created)
	if created["model_code"] != "HP-B" {
		t.Fatalf("model_code = %v, want HP-B", created["model_code"])
	}
}

func TestEnterpriseProductQualitySignalResolve(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 11003)
	now := time.Now()
	signal := models.ProductQualitySignal{
		TenantID:    11003,
		ProductID:   product.ID,
		SignalType:  "repeat_fault",
		Severity:    "warning",
		Title:       "Pressure sensor repeat faults",
		Description: "Pressure sensor faults repeated across recent tickets.",
		MetricValue: 9.5,
		SampleCount: 6,
		SourceType:  "fault_stats",
		SourceID:    "E-42",
		DetectedAt:  now,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	if err := db.Create(&signal).Error; err != nil {
		t.Fatalf("create product quality signal: %v", err)
	}

	ctx, rec := newEnterpriseContractJSONContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/quality-signals/1/_resolve",
		11003,
		`{"resolution":"Knowledge article published"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "signalId", Value: fmt.Sprint(signal.ID)},
	)
	ProductQualitySignalResolve(ctx)
	var resolved map[string]any
	decodeEnterpriseData(t, rec, &resolved)
	if resolved["status"] != "resolved" {
		t.Fatalf("status = %v, want resolved", resolved["status"])
	}

	var stored models.ProductQualitySignal
	if err := db.First(&stored, signal.ID).Error; err != nil {
		t.Fatalf("load product quality signal: %v", err)
	}
	if stored.ResolvedAt == nil {
		t.Fatalf("ResolvedAt = nil, want timestamp")
	}
}

func TestProductListIncludesRealDeviceAndTicketCounts(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1001)
	now := time.Now()
	if err := db.Create(&models.Device{
		TenantID:     1001,
		DeviceNo:     "DEV-1001",
		ProductID:    product.ID,
		SerialNo:     "SN-1001",
		RegionCode:   "US-WEST",
		Status:       enums.StatusOk,
		DeviceStatus: string(enums.DeviceStatusOperational),
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID:    1001,
		ProductID:   product.ID,
		TicketNo:    "TK-1001",
		Title:       "Cooling pump fault",
		Description: "Cooling pump fault",
		Source:      enums.TicketSourceManual,
		Status:      enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/products", 1001)
	ProductList(ctx)

	var rows []map[string]any
	decodeEnterpriseData(t, rec, &rows)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	if rows[0]["device_count"] != float64(1) {
		t.Fatalf("device_count = %v, want 1", rows[0]["device_count"])
	}
	if rows[0]["ticket_count"] != float64(1) {
		t.Fatalf("ticket_count = %v, want 1", rows[0]["ticket_count"])
	}
}

func TestEngineerCatalogAndTicketEndpointsFollowAuthorizedProductScope(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	tenantID := int64(1088)
	productA := seedEnterpriseProductContract(t, db, tenantID)
	now := time.Now()
	productB := models.Product{
		TenantID: tenantID, Code: "SCOPE-B", Name: "Scope Product B", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&productB).Error; err != nil {
		t.Fatalf("create product B: %v", err)
	}

	rootTeam := models.AgentTeam{
		TenantID: tenantID, Name: services.TechnicalAfterSalesTeamName,
		TeamType: services.AgentTeamTypeTechnicalRepair, Status: enums.StatusOk,
	}
	productTeam := models.AgentTeam{
		TenantID: tenantID, ProductID: productA.ID, Name: productA.Name,
		TeamType: services.AgentTeamTypeProductRepair, Status: enums.StatusOk,
	}
	for _, team := range []*models.AgentTeam{&rootTeam, &productTeam} {
		if err := db.Create(team).Error; err != nil {
			t.Fatalf("create team: %v", err)
		}
	}
	const engineerUserID int64 = 8801
	if err := db.Create(&models.AgentProfile{
		TenantID: tenantID, UserID: engineerUserID, TeamID: rootTeam.ID,
		AgentCode: "SCOPE-ENGINEER", DisplayName: "Scope Engineer", Status: enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create engineer profile: %v", err)
	}
	for _, teamID := range []int64{rootTeam.ID, productTeam.ID} {
		if err := db.Create(&models.AgentTeamMember{
			TenantID: tenantID, TeamID: teamID, UserID: engineerUserID,
			DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
		}).Error; err != nil {
			t.Fatalf("create team membership: %v", err)
		}
	}

	devices := []models.Device{
		{TenantID: tenantID, ProductID: productA.ID, DeviceNo: "SCOPE-DEVICE-A", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: tenantID, ProductID: productB.ID, DeviceNo: "SCOPE-DEVICE-B", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&devices).Error; err != nil {
		t.Fatalf("create scoped devices: %v", err)
	}
	tickets := []models.Ticket{
		{TenantID: tenantID, ProductID: productA.ID, CurrentTeamID: productTeam.ID, TicketNo: "SCOPE-TICKET-A", Title: "Authorized product", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: tenantID, ProductID: productB.ID, CurrentTeamID: rootTeam.ID, TicketNo: "SCOPE-TICKET-B-HIDDEN", Title: "Unauthorized root queue product", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: tenantID, ProductID: productB.ID, CurrentTeamID: rootTeam.ID, CurrentAssigneeID: engineerUserID, TicketNo: "SCOPE-TICKET-B-MINE", Title: "Directly assigned exception", Status: enums.TicketStatusProcessing, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}
	if err := db.Create(&tickets).Error; err != nil {
		t.Fatalf("create scoped tickets: %v", err)
	}

	setEngineer := func(ctx *gin.Context) {
		principal := ctx.MustGet("authPrincipal").(*dto.AuthPrincipal)
		principal.UserID = engineerUserID
		principal.DomainType = models.DomainTypeEnterprise
		principal.Roles = []string{services.EnterpriseRoleEngineer}
		principal.Permissions = []string{
			constants.PermissionProductView.Code,
			constants.PermissionDeviceView.Code,
			constants.PermissionTicketView.Code,
		}
	}

	productCtx, productRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/products", tenantID)
	setEngineer(productCtx)
	ProductList(productCtx)
	var products []dto.EnterpriseProductListItemDTO
	decodeEnterpriseData(t, productRec, &products)
	if len(products) != 1 || products[0].ID != productA.ID {
		t.Fatalf("engineer products = %+v, want only product A", products)
	}

	detailCtx, detailRec := newEnterpriseContractContext(
		http.MethodGet, fmt.Sprintf("/api/enterprise/v1/products/%d", productA.ID), tenantID,
		gin.Param{Key: "id", Value: fmt.Sprint(productA.ID)},
	)
	setEngineer(detailCtx)
	ProductGet(detailCtx)
	var detail dto.EnterpriseProductDetailDTO
	decodeEnterpriseData(t, detailRec, &detail)
	if detail.ID != productA.ID {
		t.Fatalf("engineer product detail = %+v, want product A", detail)
	}

	profileCtx, profileRec := newEnterpriseContractContext(
		http.MethodGet, fmt.Sprintf("/api/enterprise/v1/products/%d/profile", productA.ID), tenantID,
		gin.Param{Key: "id", Value: fmt.Sprint(productA.ID)},
	)
	setEngineer(profileCtx)
	ProductProfile(profileCtx)
	var profile dto.EnterpriseProductProfileDTO
	decodeEnterpriseData(t, profileRec, &profile)
	if profile.Product.ID != productA.ID {
		t.Fatalf("engineer product profile = %+v, want product A", profile.Product)
	}

	readOnlyCtx, readOnlyRec := newEnterpriseContractJSONContext(
		http.MethodPatch, fmt.Sprintf("/api/enterprise/v1/products/%d", productA.ID), tenantID,
		`{"description":"should be rejected"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(productA.ID)},
	)
	setEngineer(readOnlyCtx)
	ProductUpdate(readOnlyCtx)
	if envelope := decodeEnterpriseEnvelope(t, readOnlyRec); envelope.Success {
		t.Fatal("product repair team member updated product without owner/admin authority")
	}

	deviceCtx, deviceRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/devices", tenantID)
	setEngineer(deviceCtx)
	DeviceList(deviceCtx)
	var visibleDevices dto.EnterpriseListResponse[dto.EnterpriseDeviceListItemDTO]
	decodeEnterpriseData(t, deviceRec, &visibleDevices)
	if visibleDevices.Total != 1 || len(visibleDevices.Items) != 1 || visibleDevices.Items[0].ID != devices[0].ID {
		t.Fatalf("engineer devices = %+v, want only product A device", visibleDevices)
	}

	ticketCtx, ticketRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/tickets?page_size=20", tenantID)
	setEngineer(ticketCtx)
	TicketList(ticketCtx)
	var visibleTickets dto.EnterpriseListResponse[dto.EnterpriseTicketListItemDTO]
	decodeEnterpriseData(t, ticketRec, &visibleTickets)
	if visibleTickets.Total != 2 {
		t.Fatalf("engineer ticket total = %d, want authorized product plus direct assignment; items=%+v", visibleTickets.Total, visibleTickets.Items)
	}
	for _, item := range visibleTickets.Items {
		if item.TicketNo == "SCOPE-TICKET-B-HIDDEN" {
			t.Fatal("technical repair root leaked another product's ticket")
		}
	}

	forbiddenCtx, forbiddenRec := newEnterpriseContractContext(
		http.MethodGet, fmt.Sprintf("/api/enterprise/v1/products/%d", productB.ID), tenantID,
		gin.Param{Key: "id", Value: fmt.Sprint(productB.ID)},
	)
	setEngineer(forbiddenCtx)
	ProductGet(forbiddenCtx)
	if envelope := decodeEnterpriseEnvelope(t, forbiddenRec); envelope.Success {
		t.Fatal("engineer opened an unauthorized product detail")
	}
	hiddenTicketCtx, hiddenTicketRec := newEnterpriseContractContext(http.MethodPost, "/api/enterprise/v1/tickets/_mutation-result", tenantID)
	setEngineer(hiddenTicketCtx)
	writeEnterpriseTicketListItem(hiddenTicketCtx, tickets[1].ID)
	if envelope := decodeEnterpriseEnvelope(t, hiddenTicketRec); envelope.Success {
		t.Fatalf("hidden ticket mutation response succeeded with data=%s", string(envelope.Data))
	}

	admin := &dto.AuthPrincipal{TenantID: tenantID, UserID: 8802, Roles: []string{services.EnterpriseRoleAdmin}}
	adminProducts, err := services.ProductCenterService.ListEnterpriseProductsForOperator(tenantID, 1, 20, admin)
	if err != nil || len(adminProducts) != 2 {
		t.Fatalf("admin products = %+v, err = %v, want tenant-wide products", adminProducts, err)
	}
}

func TestEnterpriseProductOwnerCanEditAndTransferOwner(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	tenantID := int64(1091)
	product := seedEnterpriseProductContract(t, db, tenantID)
	owner := seedEnterpriseProductOwner(t, db, tenantID, 109101, "Current Product Owner")
	nextOwner := seedEnterpriseProductOwner(t, db, tenantID, 109102, "Next Product Owner")
	teamMember := seedEnterpriseProductOwner(t, db, tenantID, 109103, "Product Team Engineer")
	now := time.Now()

	if err := db.Model(&models.Product{}).Where("id = ?", product.ID).Updates(map[string]any{
		"owner_member_id": owner.ID,
		"updated_at":      now,
	}).Error; err != nil {
		t.Fatalf("assign product owner: %v", err)
	}
	product.OwnerMemberID = owner.ID
	team, err := services.ProductSupportOrganizationService.EnsureProductRepairTeamDB(
		db,
		&product,
		&dto.AuthPrincipal{TenantID: tenantID, UserID: 9001, Username: "ent-admin", Roles: []string{services.EnterpriseRoleAdmin}},
	)
	if err != nil {
		t.Fatalf("ensure product repair team: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID:        tenantID,
		TeamID:          team.ID,
		UserID:          teamMember.UserID,
		MemberID:        teamMember.ID,
		DispatchEnabled: true,
		DispatchWeight:  1,
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product team member: %v", err)
	}

	setMemberEngineer := func(ctx *gin.Context, member *models.TenantMember) {
		principal := ctx.MustGet("authPrincipal").(*dto.AuthPrincipal)
		principal.UserID = member.UserID
		principal.Username = fmt.Sprintf("member-%d", member.ID)
		principal.TenantID = tenantID
		principal.DomainType = models.DomainTypeEnterprise
		principal.SubjectType = models.SubjectTypeTenantMember
		principal.SubjectID = member.ID
		principal.MemberID = member.ID
		principal.Roles = []string{services.EnterpriseRoleEngineer}
		principal.Permissions = []string{constants.PermissionProductView.Code}
	}

	ownerCtx, ownerRec := newEnterpriseContractJSONContext(
		http.MethodPatch,
		fmt.Sprintf("/api/enterprise/v1/products/%d", product.ID),
		tenantID,
		fmt.Sprintf(`{"name":"Owner Maintained Product","owner_member_id":%d}`, nextOwner.ID),
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	setMemberEngineer(ownerCtx, owner)
	ProductUpdate(ownerCtx)
	var updated dto.EnterpriseProductListItemDTO
	decodeEnterpriseData(t, ownerRec, &updated)
	if updated.Name != "Owner Maintained Product" || updated.OwnerMemberID != nextOwner.ID || updated.OwnerUserID != nextOwner.UserID {
		t.Fatalf("owner update result = %+v, want transferred owner", updated)
	}

	var stored models.Product
	if err := db.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("reload product: %v", err)
	}
	if stored.OwnerMemberID != nextOwner.ID {
		t.Fatalf("stored owner_member_id = %d, want %d", stored.OwnerMemberID, nextOwner.ID)
	}
	var storedTeam models.AgentTeam
	if err := db.First(&storedTeam, team.ID).Error; err != nil {
		t.Fatalf("reload product team: %v", err)
	}
	if storedTeam.LeaderUserID != nextOwner.UserID {
		t.Fatalf("product team leader_user_id = %d, want %d", storedTeam.LeaderUserID, nextOwner.UserID)
	}

	memberDetailCtx, memberDetailRec := newEnterpriseContractContext(
		http.MethodGet,
		fmt.Sprintf("/api/enterprise/v1/products/%d", product.ID),
		tenantID,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	setMemberEngineer(memberDetailCtx, teamMember)
	ProductGet(memberDetailCtx)
	var memberDetail dto.EnterpriseProductDetailDTO
	decodeEnterpriseData(t, memberDetailRec, &memberDetail)
	if memberDetail.ID != product.ID {
		t.Fatalf("team member detail = %+v, want product", memberDetail)
	}

	memberUpdateCtx, memberUpdateRec := newEnterpriseContractJSONContext(
		http.MethodPatch,
		fmt.Sprintf("/api/enterprise/v1/products/%d", product.ID),
		tenantID,
		`{"description":"team member should be read only"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	setMemberEngineer(memberUpdateCtx, teamMember)
	ProductUpdate(memberUpdateCtx)
	if envelope := decodeEnterpriseEnvelope(t, memberUpdateRec); envelope.Success {
		t.Fatal("non-owner product team member updated product")
	}
}

func TestProductDevicesEndpointUsesStoredDevices(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1002)
	now := time.Now()
	model := models.ProductModel{
		TenantID:    1002,
		ProductID:   product.ID,
		ModelCode:   "HP-A",
		Name:        "HP Model A",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := db.Create(&models.Device{
		TenantID:       1002,
		DeviceNo:       "DEV-2001",
		ProductID:      product.ID,
		ProductModelID: model.ID,
		SerialNo:       "SN-2001",
		RegionCode:     "EU",
		Status:         enums.StatusOk,
		DeviceStatus:   string(enums.DeviceStatusOperational),
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/products/1/devices", 1002, gin.Param{Key: "id", Value: fmt.Sprint(product.ID)})
	ProductDevices(ctx)

	var page struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	decodeEnterpriseData(t, rec, &page)
	if len(page.Items) != 1 || page.Total != 1 {
		t.Fatalf("page = %#v, want one device", page)
	}
	if page.Items[0]["device_no"] != "DEV-2001" {
		t.Fatalf("device_no = %v, want DEV-2001", page.Items[0]["device_no"])
	}
	if page.Items[0]["model_name"] != "HP Model A" {
		t.Fatalf("model_name = %v, want HP Model A", page.Items[0]["model_name"])
	}
}

func TestProductRepairHistoryEndpointMapsTicketAndDeviceContext(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1005)
	now := time.Now()
	device := models.Device{
		TenantID:     1005,
		DeviceNo:     "DEV-REPAIR-1",
		ProductID:    product.ID,
		SerialNo:     "SN-REPAIR-1",
		RegionCode:   "EU",
		Status:       enums.StatusOk,
		DeviceStatus: string(enums.DeviceStatusOperational),
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	ticket := models.Ticket{
		TenantID:    1005,
		ProductID:   product.ID,
		DeviceID:    device.ID,
		TicketNo:    "TK-REPAIR-1",
		Title:       "Pump seal leak",
		Status:      enums.TicketStatusClosed,
		FaultCode:   "LEAK",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "Engineer A"},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&models.DeviceServiceRecord{
		TenantID:          1005,
		ProductID:         product.ID,
		DeviceID:          device.ID,
		TicketID:          ticket.ID,
		ServiceType:       "repair",
		Summary:           "Seal replaced",
		Solution:          "Replaced leaking pump seal",
		VisibleToCustomer: true,
		OccurredAt:        now,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create service record: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/products/1/repair-history", 1005, gin.Param{Key: "id", Value: fmt.Sprint(product.ID)})
	ProductRepairHistory(ctx)

	var page struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	decodeEnterpriseData(t, rec, &page)
	if len(page.Items) != 1 || page.Total != 1 {
		t.Fatalf("repair history page = %#v, want one item; body=%s", page, rec.Body.String())
	}
	if page.Items[0]["ticket_no"] != "TK-REPAIR-1" || page.Items[0]["device_no"] != "DEV-REPAIR-1" || page.Items[0]["fault_type"] != "LEAK" {
		t.Fatalf("repair history context not mapped: %#v", page.Items[0])
	}
	if page.Items[0]["resolution"] != "Replaced leaking pump seal" || page.Items[0]["completed_at"] == "" {
		t.Fatalf("repair history resolution/time not mapped: %#v", page.Items[0])
	}
}

func TestEnterpriseDeviceListMapsStoredDevices(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1006)
	now := time.Now()
	model := models.ProductModel{
		TenantID:    1006,
		ProductID:   product.ID,
		ModelCode:   "HP-M1",
		Name:        "HP Model 1",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	device := models.Device{
		TenantID:       1006,
		DeviceNo:       "DEV-LIST-1",
		ProductID:      product.ID,
		ProductModelID: model.ID,
		SerialNo:       "SN-LIST-1",
		RegionCode:     "US-WEST",
		Status:         enums.StatusOk,
		DeviceStatus:   string(enums.DeviceStatusOperational),
		LastServiceAt:  &now,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	customerUser := models.CustomerUser{
		TenantID:      1006,
		CustomerOrgID: 0,
		DisplayName:   "List Customer",
		Email:         "list.customer@example.com",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customerUser).Error; err != nil {
		t.Fatalf("create customer user: %v", err)
	}
	if err := db.Create(&models.CustomerDeviceBinding{
		TenantID:       1006,
		CustomerUserID: customerUser.ID,
		DeviceID:       device.ID,
		Status:         enums.StatusOk,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer device binding: %v", err)
	}
	if err := db.Create(&models.Device{
		TenantID:    9999,
		DeviceNo:    "DEV-OTHER-TENANT",
		ProductID:   product.ID,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create other tenant device: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/devices?search=DEV-LIST", 1006)
	DeviceList(ctx)

	var page dto.EnterpriseListResponse[dto.EnterpriseDeviceListItemDTO]
	decodeEnterpriseData(t, rec, &page)
	if len(page.Items) != 1 {
		t.Fatalf("device list length = %d, want 1; body=%s", len(page.Items), rec.Body.String())
	}
	item := page.Items[0]
	if item.DeviceNo != "DEV-LIST-1" || item.ProductName != product.Name || item.ModelName != model.Name {
		t.Fatalf("device context not mapped: %+v", item)
	}
	if item.Status != "active" || item.LastServiceAt == "" {
		t.Fatalf("device status/time not mapped: %+v", item)
	}
	if item.CustomerName != "List Customer" || item.BindingCount != 1 {
		t.Fatalf("customer binding context not mapped: %+v", item)
	}
}

func TestEnterpriseDeviceCreateUsesTenantAndProductContext(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1009)
	now := time.Now()
	model := models.ProductModel{
		TenantID:    1009,
		ProductID:   product.ID,
		ModelCode:   "HP-CREATE",
		Name:        "HP Create Model",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	serviceCode := models.ServiceCode{
		TenantID:       1009,
		ServiceCode:    "SC-CREATE-1",
		Mode:           enums.ServiceCodeModeTraceable,
		ProductID:      product.ID,
		ProductModelID: model.ID,
		Status:         enums.ServiceCodeStatusActive,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&serviceCode).Error; err != nil {
		t.Fatalf("create service code: %v", err)
	}

	body := fmt.Sprintf(`{"service_code":" sc-create-1 ","product_id":%d,"model_id":%d,"region_code":"US-EAST","description":"First installed unit"}`, product.ID, model.ID)
	ctx, rec := newEnterpriseContractJSONContext(http.MethodPost, "/api/enterprise/v1/devices", 1009, body)
	DeviceCreate(ctx)

	var created dto.EnterpriseDeviceListItemDTO
	decodeEnterpriseData(t, rec, &created)
	if created.DeviceNo != "SC-CREATE-1" || created.ServiceCode != "SC-CREATE-1" || created.ProductName != product.Name || created.ModelName != model.Name {
		t.Fatalf("created device context not mapped: %+v", created)
	}
	if created.Status != "active" || created.RegionCode != "US-EAST" {
		t.Fatalf("created device status/region not mapped: %+v", created)
	}
	stored := services.DeviceService.Get(created.ID)
	if stored == nil || stored.TenantID != 1009 || !strings.Contains(stored.MetadataJSON, "First installed unit") {
		t.Fatalf("stored device not tenant scoped or metadata missing: %+v", stored)
	}
	boundCode := repositories.ServiceCodeRepository.Get(db, serviceCode.ID)
	if boundCode == nil || boundCode.DeviceID != created.ID || boundCode.Status != enums.ServiceCodeStatusBound {
		t.Fatalf("service code not bound to created device: %+v", boundCode)
	}

	duplicateCtx, duplicateRec := newEnterpriseContractJSONContext(http.MethodPost, "/api/enterprise/v1/devices", 1009, body)
	DeviceCreate(duplicateCtx)
	assertEnterpriseEnvelopeError(t, duplicateRec)
}

func TestEnterpriseDeviceDatesPersistAcrossCreateUpdateAndRead(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1016)
	now := time.Now()
	model := models.ProductModel{
		TenantID:    1016,
		ProductID:   product.ID,
		ModelCode:   "HP-DATES",
		Name:        "HP Dates Model",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	createCtx, createRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		"/api/enterprise/v1/devices",
		1016,
		fmt.Sprintf(`{"service_code":"SC-DATES-1","product_id":%d,"model_id":%d,"install_date":"2025-01-02","warranty_end":"2030-01-02"}`, product.ID, model.ID),
	)
	DeviceCreate(createCtx)
	var created dto.EnterpriseDeviceListItemDTO
	decodeEnterpriseData(t, createRec, &created)
	if !strings.HasPrefix(created.InstallDate, "2025-01-02") || !strings.HasPrefix(created.WarrantyEnd, "2030-01-02") {
		t.Fatalf("created dates = install %q, warranty %q", created.InstallDate, created.WarrantyEnd)
	}

	stored := repositories.DeviceRepository.Get(db, created.ID)
	if stored == nil || stored.InstalledAt == nil || stored.InstalledAt.Format(time.DateOnly) != "2025-01-02" {
		t.Fatalf("stored install date = %#v", stored)
	}
	warranty := repositories.DeviceWarrantyRecordRepository.FindOne(db, sqls.NewCnd().Eq("tenant_id", int64(1016)).Eq("device_id", created.ID))
	if warranty == nil || warranty.EndAt.Format(time.DateOnly) != "2030-01-02" {
		t.Fatalf("stored warranty = %#v", warranty)
	}

	readCtx, readRec := newEnterpriseContractContext(
		http.MethodGet,
		fmt.Sprintf("/api/enterprise/v1/devices/%d", created.ID),
		1016,
		gin.Param{Key: "id", Value: fmt.Sprint(created.ID)},
	)
	DeviceGet(readCtx)
	var read dto.EnterpriseDeviceDetailDTO
	decodeEnterpriseData(t, readRec, &read)
	if !strings.HasPrefix(read.InstallDate, "2025-01-02") || !strings.HasPrefix(read.WarrantyEnd, "2030-01-02") {
		t.Fatalf("read dates = install %q, warranty %q", read.InstallDate, read.WarrantyEnd)
	}

	updateCtx, updateRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		fmt.Sprintf("/api/enterprise/v1/devices/%d", created.ID),
		1016,
		fmt.Sprintf(`{"install_date":"2025-03-04T12:30:00+08:00","warranty_end":"2031-04-05T00:00:00+08:00"}`),
		gin.Param{Key: "id", Value: fmt.Sprint(created.ID)},
	)
	DeviceUpdate(updateCtx)
	var updated dto.EnterpriseDeviceDetailDTO
	decodeEnterpriseData(t, updateRec, &updated)
	if !strings.HasPrefix(updated.InstallDate, "2025-03-04") || !strings.HasPrefix(updated.WarrantyEnd, "2031-04-05") {
		t.Fatalf("updated dates = install %q, warranty %q", updated.InstallDate, updated.WarrantyEnd)
	}

	partialCtx, partialRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		fmt.Sprintf("/api/enterprise/v1/devices/%d", created.ID),
		1016,
		`{"install_date":"2025/06/07"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(created.ID)},
	)
	DeviceUpdate(partialCtx)
	var partial dto.EnterpriseDeviceDetailDTO
	decodeEnterpriseData(t, partialRec, &partial)
	if !strings.HasPrefix(partial.InstallDate, "2025-06-07") || !strings.HasPrefix(partial.WarrantyEnd, "2031-04-05") {
		t.Fatalf("partial update dates = install %q, warranty %q", partial.InstallDate, partial.WarrantyEnd)
	}
}

func TestEnterpriseDeviceCreateCanAutoCreateServiceCode(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1015)
	now := time.Now()
	model := models.ProductModel{
		TenantID:    1015,
		ProductID:   product.ID,
		ModelCode:   "HP-AUTO",
		Name:        "HP Auto Model",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}

	body := fmt.Sprintf(`{"service_code":" auto-1015-001 ","product_id":%d,"model_id":%d,"region_code":"US-WEST","description":"Generated on device create"}`, product.ID, model.ID)
	ctx, rec := newEnterpriseContractJSONContext(http.MethodPost, "/api/enterprise/v1/devices", 1015, body)
	DeviceCreate(ctx)

	var created dto.EnterpriseDeviceListItemDTO
	decodeEnterpriseData(t, rec, &created)
	if created.DeviceNo != "AUTO-1015-001" || created.ServiceCode != "AUTO-1015-001" {
		t.Fatalf("unexpected created device: %+v", created)
	}

	stored := services.DeviceService.Get(created.ID)
	if stored == nil || stored.TenantID != 1015 || stored.ProductID != product.ID || stored.ProductModelID != model.ID {
		t.Fatalf("stored device mismatch: %+v", stored)
	}
	code := repositories.ServiceCodeRepository.GetByCode(db, "AUTO-1015-001")
	if code == nil || code.DeviceID != created.ID || code.Status != enums.ServiceCodeStatusBound || code.ProductID != product.ID || code.ProductModelID != model.ID {
		t.Fatalf("auto-created service code not bound correctly: %+v", code)
	}
	if code.Mode != enums.ServiceCodeModeTraceable {
		t.Fatalf("auto-created service code mode = %q, want traceable", code.Mode)
	}
}

func TestEnterpriseDeviceBatchImportReturnsPartialRowErrors(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1014)
	now := time.Now()
	model := models.ProductModel{
		TenantID:    1014,
		ProductID:   product.ID,
		ModelCode:   "HP-BATCH",
		Name:        "HP Batch Model",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := db.Create(&[]models.ServiceCode{
		{TenantID: 1014, ServiceCode: "SC-BATCH-1", Mode: enums.ServiceCodeModeTraceable, ProductID: product.ID, ProductModelID: model.ID, Status: enums.ServiceCodeStatusActive, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1014, ServiceCode: "SC-BATCH-2", Mode: enums.ServiceCodeModeTraceable, ProductID: product.ID, ProductModelID: model.ID, Status: enums.ServiceCodeStatusActive, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	}).Error; err != nil {
		t.Fatalf("create service codes: %v", err)
	}

	body := fmt.Sprintf(`{"devices":[{"service_code":"SC-BATCH-1","product_id":%d,"model_id":%d,"region_code":"EU"},{"service_code":"SC-BATCH-1","product_id":%d,"model_id":%d,"region_code":"EU"}]}`, product.ID, model.ID, product.ID, model.ID)
	ctx, rec := newEnterpriseContractJSONContext(http.MethodPost, "/api/enterprise/v1/devices/_batch-import", 1014, body)
	DeviceBatchImport(ctx)

	var result dto.EnterpriseDeviceBatchImportResult
	decodeEnterpriseData(t, rec, &result)
	if result.Total != 2 || result.Succeeded != 1 || result.Failed != 1 || len(result.Created) != 1 || len(result.Errors) != 1 {
		t.Fatalf("unexpected batch result: %+v", result)
	}
	if result.Created[0].DeviceNo != "SC-BATCH-1" || result.Created[0].ServiceCode != "SC-BATCH-1" || result.Created[0].ProductName != product.Name {
		t.Fatalf("created row not mapped: %+v", result.Created[0])
	}
	if result.Errors[0].Row != 2 || !strings.Contains(result.Errors[0].Reason, "service code is already bound to a device") {
		t.Fatalf("unexpected row error: %+v", result.Errors[0])
	}
}

func TestEnterpriseDeviceDetailAggregatesServiceContext(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1012)
	now := time.Now()
	model := models.ProductModel{
		TenantID:    1012,
		ProductID:   product.ID,
		ModelCode:   "HP-DETAIL",
		Name:        "HP Detail Model",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	customer := models.Customer{Name: "Detail Customer", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	device := models.Device{
		TenantID:       1012,
		DeviceNo:       "DEV-DETAIL-1",
		ProductID:      product.ID,
		ProductModelID: model.ID,
		SerialNo:       "SN-DETAIL-1",
		CustomerOrgID:  customer.ID,
		RegionCode:     "EU",
		Status:         enums.StatusOk,
		DeviceStatus:   string(enums.DeviceStatusOperational),
		LastServiceAt:  &now,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&models.ServiceCode{
		TenantID:    1012,
		ServiceCode: "SC-DETAIL-1",
		DeviceID:    device.ID,
		ProductID:   product.ID,
		Status:      enums.ServiceCodeStatusBound,
		BoundAt:     &now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create service code: %v", err)
	}
	if err := db.Create(&models.DeviceWarrantyRecord{
		TenantID:       1012,
		DeviceID:       device.ID,
		ProductID:      product.ID,
		ProductModelID: model.ID,
		StartAt:        now.AddDate(0, -1, 0),
		EndAt:          now.AddDate(0, 3, 0),
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create warranty: %v", err)
	}
	if err := db.Create(&models.DeviceSoftwareVersion{
		TenantID:      1012,
		DeviceID:      device.ID,
		ComponentType: "firmware",
		ComponentName: "Controller",
		Version:       "1.2.3",
		InstalledAt:   now,
		CreatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("create software version: %v", err)
	}
	ticket := models.Ticket{
		TenantID:       1012,
		ProductID:      product.ID,
		ProductModelID: model.ID,
		DeviceID:       device.ID,
		CustomerID:     customer.ID,
		TicketNo:       "TK-DETAIL-1",
		Title:          "Detail fault",
		Description:    "Detail fault",
		Source:         enums.TicketSourceManual,
		Status:         enums.TicketStatusInProgress,
		SLADueAt:       &now,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&models.TicketRepairRecord{
		TenantID:          1012,
		TicketID:          ticket.ID,
		DeviceID:          device.ID,
		ProductID:         product.ID,
		RootCause:         "Pressure leak",
		Solution:          "Replaced seal",
		FinishedAt:        &now,
		VisibleToCustomer: true,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create repair: %v", err)
	}
	if err := db.Create(&models.MeetingRoomJitsi{
		ID:        "meeting-detail-1",
		TenantID:  1012,
		TicketID:  fmt.Sprint(ticket.ID),
		RoomName:  "rhd-detail-1",
		Status:    "active",
		CreatedBy: "0",
		StartedAt: &now,
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := db.Create(&models.MeetingRoomJitsi{
		ID:        "meeting-detail-2",
		TenantID:  1012,
		TicketID:  fmt.Sprint(ticket.ID),
		RoomName:  "rhd-detail-2",
		Status:    "ended",
		CreatedBy: "0",
		EndedAt:   &now,
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create ended meeting: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, fmt.Sprintf("/api/enterprise/v1/devices/%d", device.ID), 1012, gin.Param{Key: "id", Value: fmt.Sprint(device.ID)})
	DeviceGet(ctx)
	var detail map[string]any
	decodeEnterpriseData(t, rec, &detail)
	if detail["device_no"] != "DEV-DETAIL-1" || detail["service_code"] != "SC-DETAIL-1" {
		t.Fatalf("device detail identity not mapped: %#v", detail)
	}
	if detail["open_ticket_count"].(float64) != 1 || detail["meeting_count"].(float64) != 2 || detail["active_meeting_count"].(float64) != 1 {
		t.Fatalf("device ticket/meeting counts not mapped: %#v", detail)
	}
	if got := detail["software_versions"].([]any); len(got) != 1 {
		t.Fatalf("software_versions len = %d, want 1", len(got))
	}
	if got := detail["repair_history"].([]any); len(got) != 1 {
		t.Fatalf("repair_history len = %d, want 1", len(got))
	}
	if got := detail["recent_tickets"].([]any); len(got) != 1 {
		t.Fatalf("recent_tickets len = %d, want 1", len(got))
	}
}

func TestEnterpriseMeetingNotificationAndWorkbenchUseStoredData(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1013)
	now := time.Now()
	customer := models.Customer{Name: "Workbench Customer", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	device := models.Device{
		TenantID:      1013,
		DeviceNo:      "DEV-WB-1",
		ProductID:     product.ID,
		CustomerOrgID: customer.ID,
		Status:        enums.StatusOk,
		DeviceStatus:  string(enums.DeviceStatusOperational),
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	ticket := models.Ticket{
		TenantID:    1013,
		ProductID:   product.ID,
		DeviceID:    device.ID,
		CustomerID:  customer.ID,
		TicketNo:    "TK-WB-1",
		Title:       "Workbench fault",
		Description: "Workbench fault",
		Source:      enums.TicketSourceManual,
		Status:      enums.TicketStatusInProgress,
		SLADueAt:    &now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&models.MeetingRoomJitsi{
		ID:        "meeting-wb-1",
		TenantID:  1013,
		TicketID:  fmt.Sprint(ticket.ID),
		RoomName:  "rhd-wb-1",
		Status:    "active",
		CreatedBy: "0",
		StartedAt: &now,
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := db.Create(&models.MeetingRoomJitsi{
		ID:        "meeting-other-tenant",
		TenantID:  9999,
		TicketID:  fmt.Sprint(ticket.ID),
		RoomName:  "rhd-other",
		Status:    "active",
		CreatedBy: "0",
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create other meeting: %v", err)
	}
	if err := db.Create(&models.Notification{
		TenantID:         1013,
		Title:            "Meeting invite",
		Content:          "Join the meeting",
		NotificationType: "meeting.invite",
		BizType:          "meeting",
		BizID:            1,
		DeliveryStatus:   "sent",
		Status:           int(enums.StatusOk),
		CreatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("create notification: %v", err)
	}
	if err := db.Create(&models.Notification{
		TenantID:         9999,
		Title:            "Other tenant",
		NotificationType: "meeting.invite",
		BizType:          "meeting",
		DeliveryStatus:   "sent",
		Status:           int(enums.StatusOk),
		CreatedAt:        now,
	}).Error; err != nil {
		t.Fatalf("create other notification: %v", err)
	}
	if err := db.Create(&models.ProductAIUsageCredential{
		TenantID:        1013,
		ProductID:       product.ID,
		Sub2APIKeyID:    "31013",
		KeyName:         "HP-3000 diagnosis key",
		QuotaLimit:      100,
		QuotaUsed:       82,
		Currency:        "USD",
		ProvisionStatus: "ready",
		Status:          enums.StatusOk,
		LastSyncedAt:    &now,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product usage credential: %v", err)
	}
	if err := db.Create(&models.Sub2APITenantAccount{
		TenantID:         1013,
		Sub2APIAccountID: "t-000001",
		LoginEmail:       "t-000001@remotedesk.com",
		Balance:          19.13,
		Status:           enums.StatusOk,
	}).Error; err != nil {
		t.Fatalf("create sub2api tenant account: %v", err)
	}

	meetingCtx, meetingRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/meetings", 1013)
	MeetingGetList(meetingCtx)
	var meetingPayload map[string]any
	decodeEnterpriseData(t, meetingRec, &meetingPayload)
	if items := meetingPayload["items"].([]any); len(items) != 1 {
		t.Fatalf("meeting items len = %d, want 1; payload=%#v", len(items), meetingPayload)
	}

	notificationCtx, notificationRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/notifications", 1013)
	NotificationList(notificationCtx)
	var notificationPayload map[string]any
	decodeEnterpriseData(t, notificationRec, &notificationPayload)
	if items := notificationPayload["items"].([]any); len(items) != 1 {
		t.Fatalf("notification items len = %d, want 1; payload=%#v", len(items), notificationPayload)
	}

	workbenchCtx, workbenchRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/workbench/overview", 1013)
	WorkbenchGetOverview(workbenchCtx)
	var workbench map[string]any
	decodeEnterpriseData(t, workbenchRec, &workbench)
	if queue := workbench["queue"].([]any); len(queue) != 1 {
		t.Fatalf("workbench queue len = %d, want 1; payload=%#v", len(queue), workbench)
	}
	if meetings := workbench["meetings"].([]any); len(meetings) != 1 {
		t.Fatalf("workbench meetings len = %d, want 1; payload=%#v", len(meetings), workbench)
	}
	if notifications := workbench["notifications"].([]any); len(notifications) != 1 {
		t.Fatalf("workbench notifications len = %d, want 1; payload=%#v", len(notifications), workbench)
	}
	for _, item := range workbench["queues"].([]any) {
		card := item.(map[string]any)
		if _, ok := card["items"].([]any); !ok {
			t.Fatalf("workbench queue card items should be an array; card=%#v", card)
		}
	}
	var overview dto.EnterpriseWorkbenchOverviewDTO
	decodeEnterpriseData(t, workbenchRec, &overview)
	if overview.Scope.Restricted || overview.Scope.Mode != "tenant" {
		t.Fatalf("workbench scope = %+v, want unrestricted tenant scope", overview.Scope)
	}
	if overview.Summary.OpenTickets != 1 || overview.Summary.SLARiskTickets != 1 || overview.Summary.ActiveMeetings != 1 || overview.Summary.TotalProducts != 1 || overview.Summary.TotalDevices != 1 {
		t.Fatalf("workbench summary = %+v, want visible tenant summary", overview.Summary)
	}
	if overview.Usage.QuotaLimit != 100 || overview.Usage.QuotaUsed != 82 || overview.Usage.RiskyProductCount != 1 {
		t.Fatalf("workbench usage = %+v, want product usage summary", overview.Usage)
	}
	if overview.Usage.Label != "账户余额" || overview.Usage.AccountBalance != 19.13 {
		t.Fatalf("workbench account balance = %+v, want USD 19.13", overview.Usage)
	}
	if overview.Usage.KeyCount != 1 || overview.Usage.RiskyKeyCount != 1 || overview.Usage.CriticalKeyCount != 0 {
		t.Fatalf("workbench key usage = %+v, want one warning key", overview.Usage)
	}
	if len(overview.QuotaAlerts) != 1 || overview.QuotaAlerts[0].ProductID != product.ID || overview.QuotaAlerts[0].APIKeyID != "31013" || overview.QuotaAlerts[0].Severity != "warning" {
		t.Fatalf("workbench quota alerts = %+v, want product API key warning", overview.QuotaAlerts)
	}
	if len(overview.Alerts) == 0 || overview.Alerts[0].Key == "" {
		t.Fatalf("workbench alerts not populated: %+v", overview.Alerts)
	}
	if len(overview.Products) != 1 || overview.Products[0].ProductID != product.ID || overview.Products[0].UsagePercent < 80 {
		t.Fatalf("workbench product load not populated: %+v", overview.Products)
	}
}

func TestEnterpriseWorkbenchOverviewScopesEngineerToProductTeam(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	tenantID := int64(1044)
	productA := seedEnterpriseProductContract(t, db, tenantID)
	now := time.Now()
	productB := models.Product{
		TenantID:      tenantID,
		Code:          "PUMP-B",
		Name:          "Pump Product B",
		Category:      "pump",
		DefaultLocale: "en-US",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&productB).Error; err != nil {
		t.Fatalf("create product b: %v", err)
	}
	teamA := models.AgentTeam{
		TenantID:      tenantID,
		ProductID:     productA.ID,
		TeamType:      "product_repair",
		Name:          "HP-3000 维修组",
		Status:        enums.StatusOk,
		SystemManaged: true,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	teamB := models.AgentTeam{
		TenantID:      tenantID,
		ProductID:     productB.ID,
		TeamType:      "product_repair",
		Name:          "PUMP-B 维修组",
		Status:        enums.StatusOk,
		SystemManaged: true,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&teamA).Error; err != nil {
		t.Fatalf("create team a: %v", err)
	}
	if err := db.Create(&teamB).Error; err != nil {
		t.Fatalf("create team b: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID:           tenantID,
		UserID:             4401,
		TeamID:             teamA.ID,
		DisplayName:        "Engineer A",
		ServiceStatus:      enums.ServiceStatusIdle,
		AutoAssignEnabled:  true,
		MaxConcurrentCount: 3,
		Status:             enums.StatusOk,
		AuditFields:        models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create engineer profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID:        tenantID,
		TeamID:          teamA.ID,
		UserID:          4401,
		DispatchEnabled: true,
		DispatchWeight:  1,
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create team a membership: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID:      tenantID,
		ProductID:     productA.ID,
		CurrentTeamID: teamA.ID,
		TicketNo:      "TK-SCOPE-A",
		Title:         "Visible product ticket",
		Source:        enums.TicketSourceManual,
		Status:        enums.TicketStatusPendingAcceptance,
		SLADueAt:      ptrTime(now.Add(90 * time.Minute)),
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create visible ticket: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID:      tenantID,
		ProductID:     productB.ID,
		CurrentTeamID: teamB.ID,
		TicketNo:      "TK-SCOPE-B",
		Title:         "Hidden product ticket",
		Source:        enums.TicketSourceManual,
		Status:        enums.TicketStatusPendingAcceptance,
		SLADueAt:      ptrTime(now.Add(30 * time.Minute)),
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create hidden ticket: %v", err)
	}
	if err := db.Create(&models.ProductAIUsageCredential{
		TenantID:        tenantID,
		ProductID:       productA.ID,
		QuotaLimit:      200,
		QuotaUsed:       50,
		Currency:        "USD",
		ProvisionStatus: "ready",
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product a usage: %v", err)
	}
	if err := db.Create(&models.ProductAIUsageCredential{
		TenantID:        tenantID,
		ProductID:       productB.ID,
		QuotaLimit:      500,
		QuotaUsed:       450,
		Currency:        "USD",
		ProvisionStatus: "ready",
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product b usage: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/workbench/overview", tenantID)
	principal := ctx.MustGet("authPrincipal").(*dto.AuthPrincipal)
	principal.UserID = 4401
	principal.Roles = []string{services.EnterpriseRoleEngineer}
	WorkbenchGetOverview(ctx)

	var overview dto.EnterpriseWorkbenchOverviewDTO
	decodeEnterpriseData(t, rec, &overview)
	if !overview.Scope.Restricted || overview.Scope.TeamID != teamA.ID || overview.Scope.ProductID != productA.ID {
		t.Fatalf("engineer scope = %+v, want product team scope", overview.Scope)
	}
	if overview.Summary.OpenTickets != 1 || overview.Summary.SLARiskTickets != 1 || overview.Summary.TotalProducts != 1 {
		t.Fatalf("engineer summary = %+v, want only product A ticket", overview.Summary)
	}
	if len(overview.Queue) != 1 || overview.Queue[0].TicketNo != "TK-SCOPE-A" {
		t.Fatalf("engineer queue = %+v, want only product A ticket", overview.Queue)
	}
	if overview.Usage.QuotaLimit != 200 || overview.Usage.QuotaUsed != 50 || overview.Usage.ProductCount != 1 {
		t.Fatalf("engineer usage = %+v, want only product A usage", overview.Usage)
	}
	if len(overview.QuotaAlerts) != 0 || overview.Usage.RiskyKeyCount != 0 {
		t.Fatalf("engineer quota alerts = %+v, want hidden product key excluded", overview.QuotaAlerts)
	}
	if len(overview.Products) != 1 || overview.Products[0].ProductID != productA.ID {
		t.Fatalf("engineer products = %+v, want only product A", overview.Products)
	}
}

func TestEnterpriseWorkbenchOverviewSupportsMultiProductEngineerScope(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	tenantID := int64(1045)
	productA := seedEnterpriseProductContract(t, db, tenantID)
	now := time.Now()
	productB := models.Product{
		TenantID:      tenantID,
		Code:          "MULTI-PUMP-B",
		Name:          "Multi Product B",
		Category:      "pump",
		DefaultLocale: "en-US",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&productB).Error; err != nil {
		t.Fatalf("create product b: %v", err)
	}
	teamA := models.AgentTeam{
		TenantID:      tenantID,
		ProductID:     productA.ID,
		TeamType:      "product_repair",
		Name:          "Multi A 维修组",
		Status:        enums.StatusOk,
		SystemManaged: true,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	teamB := models.AgentTeam{
		TenantID:      tenantID,
		ProductID:     productB.ID,
		TeamType:      "product_repair",
		Name:          "Multi B 维修组",
		Status:        enums.StatusOk,
		SystemManaged: true,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&teamA).Error; err != nil {
		t.Fatalf("create team a: %v", err)
	}
	if err := db.Create(&teamB).Error; err != nil {
		t.Fatalf("create team b: %v", err)
	}
	if err := db.Create(&models.AgentProfile{
		TenantID:           tenantID,
		UserID:             4501,
		TeamID:             teamA.ID,
		DisplayName:        "Multi Engineer",
		ServiceStatus:      enums.ServiceStatusIdle,
		AutoAssignEnabled:  true,
		MaxConcurrentCount: 3,
		Status:             enums.StatusOk,
		AuditFields:        models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create engineer profile: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID:        tenantID,
		TeamID:          teamA.ID,
		UserID:          4501,
		DispatchEnabled: true,
		DispatchWeight:  1,
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create team a membership: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID:        tenantID,
		TeamID:          teamB.ID,
		UserID:          4501,
		DispatchEnabled: true,
		DispatchWeight:  1,
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create team b membership: %v", err)
	}
	for _, item := range []models.Ticket{
		{
			TenantID:      tenantID,
			ProductID:     productA.ID,
			CurrentTeamID: teamA.ID,
			TicketNo:      "TK-MULTI-A",
			Title:         "Visible product A ticket",
			Source:        enums.TicketSourceManual,
			Status:        enums.TicketStatusPendingAcceptance,
			AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
		},
		{
			TenantID:      tenantID,
			ProductID:     productB.ID,
			CurrentTeamID: teamB.ID,
			TicketNo:      "TK-MULTI-B",
			Title:         "Visible product B ticket",
			Source:        enums.TicketSourceManual,
			Status:        enums.TicketStatusPendingAcceptance,
			AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
		},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create ticket %s: %v", item.TicketNo, err)
		}
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/workbench/overview", tenantID)
	principal := ctx.MustGet("authPrincipal").(*dto.AuthPrincipal)
	principal.UserID = 4501
	principal.Roles = []string{services.EnterpriseRoleEngineer}
	WorkbenchGetOverview(ctx)

	var overview dto.EnterpriseWorkbenchOverviewDTO
	decodeEnterpriseData(t, rec, &overview)
	if !overview.Scope.Restricted || overview.Scope.TeamID != 0 || overview.Scope.ProductID != 0 {
		t.Fatalf("multi-product engineer single scope fields = %+v, want only collection fields", overview.Scope)
	}
	if len(overview.Scope.TeamIDs) != 2 || len(overview.Scope.ProductIDs) != 2 || overview.Scope.Label != "我的 2 个产品维修组" {
		t.Fatalf("multi-product engineer scope = %+v, want two team/product ids", overview.Scope)
	}
	if overview.Summary.OpenTickets != 2 || overview.Summary.TotalProducts != 2 {
		t.Fatalf("multi-product engineer summary = %+v, want both products", overview.Summary)
	}
	if len(overview.Products) != 2 {
		t.Fatalf("multi-product engineer products = %+v, want two products", overview.Products)
	}
}

func TestEnterpriseServiceCodeListsMapStoredCodesAndBatches(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1007)
	now := time.Now()
	batch := models.ServiceCodeBatch{
		TenantID:       1007,
		BatchNo:        "BATCH-LIST-1",
		Mode:           enums.ServiceCodeModeTraceable,
		ProductID:      product.ID,
		Quantity:       10,
		Status:         enums.StatusOk,
		GeneratedCount: 3,
		ExportedCount:  2,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&batch).Error; err != nil {
		t.Fatalf("create batch: %v", err)
	}
	device := models.Device{
		TenantID:     1007,
		DeviceNo:     "DEV-SC-1",
		ProductID:    product.ID,
		SerialNo:     "SN-SC-1",
		Status:       enums.StatusOk,
		DeviceStatus: string(enums.DeviceStatusOperational),
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&models.ServiceCode{
		TenantID:    1007,
		BatchID:     batch.ID,
		ServiceCode: "SC-LIST-1",
		Mode:        enums.ServiceCodeModeTraceable,
		DeviceID:    device.ID,
		ProductID:   product.ID,
		Status:      enums.ServiceCodeStatusBound,
		ActivatedAt: &now,
		BoundAt:     &now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create service code: %v", err)
	}

	codeCtx, codeRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/service-codes?search=SC-LIST", 1007)
	ServiceCodeList(codeCtx)
	var codes dto.EnterpriseListResponse[map[string]any]
	decodeEnterpriseData(t, codeRec, &codes)
	if codes.Total != 1 || len(codes.Items) != 1 {
		t.Fatalf("service code list length = %d, want 1; body=%s", len(codes.Items), codeRec.Body.String())
	}
	if codes.Items[0]["service_code"] != "SC-LIST-1" || codes.Items[0]["batch_no"] != "BATCH-LIST-1" || codes.Items[0]["product_name"] != product.Name {
		t.Fatalf("service code context not mapped: %#v", codes.Items[0])
	}
	if codes.Items[0]["bound_device"] != "DEV-SC-1" || codes.Items[0]["status"] != "bound" {
		t.Fatalf("service code binding/status not mapped: %#v", codes.Items[0])
	}

	batchCtx, batchRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/service-code-batches", 1007)
	ServiceCodeBatchList(batchCtx)
	var batches dto.EnterpriseListResponse[map[string]any]
	decodeEnterpriseData(t, batchRec, &batches)
	if batches.Total != 1 || len(batches.Items) != 1 {
		t.Fatalf("batch list length = %d, want 1; body=%s", len(batches.Items), batchRec.Body.String())
	}
	if batches.Items[0]["batch_no"] != "BATCH-LIST-1" || batches.Items[0]["generated"].(float64) != 3 || batches.Items[0]["exported"].(float64) != 2 {
		t.Fatalf("batch counts not mapped: %#v", batches.Items[0])
	}
}

func TestEnterpriseTicketListMapsStoredTickets(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1003)
	now := time.Now()
	futureSLA := now.Add(2 * time.Hour)
	device := models.Device{
		TenantID:     1003,
		DeviceNo:     "DEV-3001",
		ProductID:    product.ID,
		SerialNo:     "SN-3001",
		RegionCode:   "US",
		Status:       enums.StatusOk,
		DeviceStatus: string(enums.DeviceStatusOperational),
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	customer := models.Customer{Name: "Acme Operator", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	conversation := models.Conversation{
		TenantID:           1003,
		CustomerID:         customer.ID,
		CustomerName:       "Acme Operator",
		Status:             enums.IMConversationStatusActive,
		LastMessageSummary: "Pressure abnormal",
		LastMessageAt:      now,
		LastActiveAt:       now,
		AuditFields:        models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	conversation.CustomerID = customer.ID
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID:       1003,
		ProductID:      product.ID,
		DeviceID:       device.ID,
		CustomerID:     customer.ID,
		ConversationID: conversation.ID,
		TicketNo:       "TK-3001",
		Title:          "Pressure abnormal",
		Description:    "Pressure abnormal",
		Source:         enums.TicketSourceConversation,
		Channel:        "im",
		Status:         enums.TicketStatusPending,
		SLADueAt:       &now,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID:    1003,
		ProductID:   product.ID,
		DeviceID:    device.ID,
		CustomerID:  customer.ID,
		TicketNo:    "TK-3002",
		Title:       "Unrelated ticket",
		Description: "Unrelated ticket",
		Source:      enums.TicketSourceManual,
		Status:      enums.TicketStatusPending,
		SLADueAt:    &futureSLA,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create unrelated ticket: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, fmt.Sprintf("/api/enterprise/v1/tickets?page_size=10&conversation_id=%d", conversation.ID), 1003)
	TicketList(ctx)

	var payload struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	decodeEnterpriseData(t, rec, &payload)
	if payload.Total != 1 || len(payload.Items) != 1 {
		t.Fatalf("tickets = (%d, %d), want total 1 len 1", payload.Total, len(payload.Items))
	}
	if payload.Items[0]["ticket_no"] != "TK-3001" {
		t.Fatalf("ticket_no = %v, want TK-3001", payload.Items[0]["ticket_no"])
	}
	if got := int64(payload.Items[0]["conversation_id"].(float64)); got != conversation.ID {
		t.Fatalf("conversation_id = %d, want %d", got, conversation.ID)
	}
	if payload.Items[0]["source"] != string(enums.TicketSourceConversation) || payload.Items[0]["channel"] != "im" {
		t.Fatalf("source/channel not mapped: %#v", payload.Items[0])
	}
	if payload.Items[0]["product_name"] != product.Name {
		t.Fatalf("product_name = %v, want %s", payload.Items[0]["product_name"], product.Name)
	}

	for _, tc := range []struct {
		name      string
		query     string
		wantTotal int64
	}{
		{name: "ticket no", query: "search=TK-3001", wantTotal: 1},
		{name: "customer", query: "search=Acme", wantTotal: 2},
		{name: "device", query: "search=DEV-3001", wantTotal: 2},
		{name: "product", query: "search=" + url.QueryEscape(product.Name), wantTotal: 2},
		{name: "sla breached", query: "sla_breached=true", wantTotal: 1},
		{name: "sla risk", query: "sla_risk=true", wantTotal: 2},
		{name: "pending abstract status", query: "status=pending", wantTotal: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, rec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/tickets?page_size=10&"+tc.query, 1003)
			TicketList(ctx)
			var searchPayload struct {
				Items []map[string]any `json:"items"`
				Total int64            `json:"total"`
			}
			decodeEnterpriseData(t, rec, &searchPayload)
			if searchPayload.Total != tc.wantTotal {
				t.Fatalf("total = %d, want %d; items=%#v", searchPayload.Total, tc.wantTotal, searchPayload.Items)
			}
		})
	}

	summaryCtx, summaryRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/tickets/summary", 1003)
	TicketSummary(summaryCtx)
	var summary dto.EnterpriseTicketSummaryDTO
	decodeEnterpriseData(t, summaryRec, &summary)
	if summary.Total != 2 || summary.Pending != 2 || summary.SLARisk != 2 || summary.Processing != 0 || summary.Done != 0 {
		t.Fatalf("ticket summary = %+v, want total/pending/sla risk 2 and processing/done 0", summary)
	}
}

func TestEnterpriseTicketMineFilterUsesAuthenticatedUser(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	now := time.Now()
	for _, ticket := range []models.Ticket{
		{TenantID: 1003, CurrentAssigneeID: 701, TicketNo: "TK-MINE-701", Title: "My ticket", Status: enums.TicketStatusProcessing, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{TenantID: 1003, CurrentAssigneeID: 702, TicketNo: "TK-MINE-702", Title: "Other ticket", Status: enums.TicketStatusPending, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	} {
		if err := db.Create(&ticket).Error; err != nil {
			t.Fatalf("create assigned ticket: %v", err)
		}
	}

	listCtx, listRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/tickets?mine=1", 1003)
	principal, _ := listCtx.Get("authPrincipal")
	principal.(*dto.AuthPrincipal).UserID = 701
	TicketList(listCtx)
	var listPayload struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	decodeEnterpriseData(t, listRec, &listPayload)
	if listPayload.Total != 1 || len(listPayload.Items) != 1 || listPayload.Items[0]["ticket_no"] != "TK-MINE-701" {
		t.Fatalf("personal ticket list = %+v, want authenticated user's ticket", listPayload)
	}

	summaryCtx, summaryRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/tickets/summary?mine=true", 1003)
	principal, _ = summaryCtx.Get("authPrincipal")
	principal.(*dto.AuthPrincipal).UserID = 701
	TicketSummary(summaryCtx)
	var summary dto.EnterpriseTicketSummaryDTO
	decodeEnterpriseData(t, summaryRec, &summary)
	if summary.Total != 1 || summary.Processing != 1 || summary.Pending != 0 {
		t.Fatalf("personal ticket summary = %+v, want only authenticated user's processing ticket", summary)
	}
}

func TestEnterpriseTicketWorkbenchUsesTenantScopedStoredContext(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	tenantID := int64(1030)
	product := seedEnterpriseProductContract(t, db, tenantID)
	now := time.Now().UTC().Truncate(time.Second)

	company := models.Company{
		Name:        "Northwind Service",
		Code:        "NW-1030",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&company).Error; err != nil {
		t.Fatalf("create company: %v", err)
	}
	customer := models.Customer{
		Name:          "Maria Gomez",
		CompanyID:     company.ID,
		PrimaryEmail:  "maria@example.com",
		PrimaryMobile: "+1-555-0100",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	device := models.Device{
		TenantID:     tenantID,
		DeviceNo:     "DEV-WORKBENCH-1",
		ProductID:    product.ID,
		SerialNo:     "SN-WORKBENCH-1",
		RegionCode:   "US",
		Status:       enums.StatusOk,
		DeviceStatus: string(enums.DeviceStatusOperational),
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&models.DeviceWarrantyRecord{
		TenantID:    tenantID,
		DeviceID:    device.ID,
		ProductID:   product.ID,
		StartAt:     now.AddDate(-1, 0, 0),
		EndAt:       now.AddDate(1, 0, 0),
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create warranty: %v", err)
	}
	conversation := models.Conversation{
		TenantID:           tenantID,
		CustomerID:         customer.ID,
		CustomerName:       customer.Name,
		ProductID:          product.ID,
		DeviceID:           device.ID,
		Status:             enums.IMConversationStatusPending,
		Priority:           2,
		LastMessageSummary: "Hydraulic pressure is unstable",
		LastMessageAt:      now,
		LastActiveAt:       now,
		AuditFields:        models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := db.Create(&models.Conversation{
		TenantID:      9999,
		CustomerName:  "Other tenant conversation",
		Status:        enums.IMConversationStatusPending,
		LastMessageAt: now,
		LastActiveAt:  now,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create other tenant conversation: %v", err)
	}

	asset := models.Asset{
		TenantID:   tenantID,
		AssetID:    "asset-workbench-1",
		Provider:   enums.AssetProviderLocal,
		StorageKey: "tests/workbench/manual.pdf",
		Filename:   "controller-log.pdf",
		FileSize:   2048,
		MimeType:   "application/pdf",
		Status:     enums.AssetStatusSuccess,
		AuditFields: models.AuditFields{
			CreatedAt:      now,
			UpdatedAt:      now,
			CreateUserName: "Support Engineer",
		},
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	if err := db.Create(&models.Message{
		ConversationID: conversation.ID,
		ClientMsgID:    "asset-message-1",
		SenderType:     enums.IMSenderTypeAgent,
		MessageType:    enums.IMMessageTypeAttachment,
		Content:        asset.Filename,
		Payload:        `{"assetId":"asset-workbench-1"}`,
		SendStatus:     enums.IMMessageStatusSent,
		SentAt:         &now,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create attachment message: %v", err)
	}

	ticket := models.Ticket{
		TenantID:          tenantID,
		ProductID:         product.ID,
		DeviceID:          device.ID,
		CustomerID:        customer.ID,
		ConversationID:    conversation.ID,
		TicketNo:          "TK-WORKBENCH-1",
		Title:             "Hydraulic pressure unstable",
		Description:       "Pressure fluctuates after startup",
		Source:            enums.TicketSourceConversation,
		Channel:           "im",
		Status:            enums.TicketStatusInProgress,
		FaultCode:         "HYD-PRESSURE",
		SymptomSummary:    "Pressure fluctuates",
		DiagnosisSummary:  "Inspect pressure sensor",
		CurrentAssigneeID: 0,
		SLADueAt:          ptrTime(now.Add(4 * time.Hour)),
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	flowOperator := models.User{
		Username:    "workbench.flow.engineer",
		Nickname:    "Flow Engineer",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&flowOperator).Error; err != nil {
		t.Fatalf("create flow operator: %v", err)
	}
	for _, progress := range []models.TicketProgress{
		{
			TenantID:  tenantID,
			TicketID:  ticket.ID,
			EventType: enums.TicketProgressEventAccepted,
			Content:   "Accepted ticket",
			AuthorID:  flowOperator.ID,
			CreatedAt: now,
		},
		{
			TenantID:  tenantID,
			TicketID:  ticket.ID,
			EventType: enums.TicketProgressEventAssigned,
			Content:   "Assigned ticket",
			AuthorID:  flowOperator.ID,
			CreatedAt: now.Add(time.Minute),
		},
		{
			TenantID:  tenantID,
			TicketID:  ticket.ID,
			EventType: enums.TicketProgressEventRepairCompleted,
			Content:   "Repair completed",
			AuthorID:  flowOperator.ID,
			CreatedAt: now.Add(2 * time.Minute),
		},
	} {
		if err := db.Create(&progress).Error; err != nil {
			t.Fatalf("create flow progress: %v", err)
		}
	}
	if err := db.Create(&models.DiagnosisSession{
		ID:              "diagnosis-workbench-1",
		TenantID:        tenantID,
		ConversationID:  fmt.Sprint(conversation.ID),
		DeviceID:        fmt.Sprint(device.ID),
		ProductID:       fmt.Sprint(product.ID),
		Status:          "escalated",
		FaultCodes:      `["HYD-PRESSURE"]`,
		Resolution:      "Inspect and recalibrate the pressure sensor",
		ConfidenceScore: 0.91,
		CreatedAt:       now,
		BaseModel:       models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create diagnosis: %v", err)
	}
	if err := db.Create(&models.TicketRepairRecord{
		TenantID:    tenantID,
		TicketID:    ticket.ID,
		DeviceID:    device.ID,
		ProductID:   product.ID,
		Solution:    "Recalibrated pressure sensor",
		PartsJSON:   `[{"name":"Pressure sensor","quantity":1}]`,
		CostHours:   1.5,
		FinishedAt:  &now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now, CreateUserName: "Support Engineer"},
	}).Error; err != nil {
		t.Fatalf("create repair: %v", err)
	}
	if err := db.Create(&models.TicketFeedback{
		TenantID:    tenantID,
		TicketID:    ticket.ID,
		Rating:      5,
		TagsJSON:    `["fast","professional"]`,
		Comment:     "Resolved quickly",
		Status:      "submitted",
		SubmittedAt: now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create feedback: %v", err)
	}
	meeting := models.MeetingRoomJitsi{
		ID:        "meeting-workbench-1",
		TenantID:  tenantID,
		TicketID:  fmt.Sprint(ticket.ID),
		RoomName:  "rhd-workbench-1",
		Status:    "active",
		CreatedBy: "7",
		StartedAt: &now,
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&meeting).Error; err != nil {
		t.Fatalf("create meeting: %v", err)
	}
	if err := db.Create(&models.MeetingParticipant{
		ID:              "participant-workbench-1",
		MeetingID:       meeting.ID,
		UserID:          "7",
		UserType:        "enterprise",
		ParticipantName: "Support Engineer",
		JoinedAt:        &now,
		BaseModel:       models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create participant: %v", err)
	}

	detailCtx, detailRec := newEnterpriseContractContext(http.MethodGet, fmt.Sprintf("/api/enterprise/v1/tickets/%d", ticket.ID), tenantID, gin.Param{Key: "id", Value: fmt.Sprint(ticket.ID)})
	TicketGet(detailCtx)
	var aggregate dto.TicketAggregateDTO
	decodeEnterpriseData(t, detailRec, &aggregate)
	if aggregate.Customer.Company != company.Name || aggregate.DeviceContext.WarrantyEnd == "" {
		t.Fatalf("customer/warranty context not mapped: %+v %+v", aggregate.Customer, aggregate.DeviceContext)
	}
	if aggregate.DiagnosisSnapshot == nil || aggregate.DiagnosisSnapshot.DiagnosisSessionID != "diagnosis-workbench-1" || aggregate.DiagnosisSnapshot.Confidence != 0.91 {
		t.Fatalf("diagnosis not mapped: %+v", aggregate.DiagnosisSnapshot)
	}
	if aggregate.Meeting.MeetingID != meeting.ID || aggregate.Meeting.ParticipantCount != 1 || aggregate.Meeting.Status != "active" {
		t.Fatalf("meeting not mapped: %+v", aggregate.Meeting)
	}
	if len(aggregate.Assets) != 1 || aggregate.Assets[0].FileName != asset.Filename {
		t.Fatalf("assets not mapped: %+v", aggregate.Assets)
	}
	if len(aggregate.Repair.Parts) != 1 || aggregate.Repair.CostHours != 1.5 || aggregate.Repair.TechnicianName != "Support Engineer" {
		t.Fatalf("repair not mapped: %+v", aggregate.Repair)
	}
	if aggregate.Feedback == nil || aggregate.Feedback.Rating != 5 || aggregate.Feedback.Comment != "Resolved quickly" {
		t.Fatalf("feedback not mapped: %+v", aggregate.Feedback)
	}
	flowSteps := make(map[string]dto.TicketFlowStepDTO, len(aggregate.Flow.Steps))
	for _, step := range aggregate.Flow.Steps {
		flowSteps[step.Name] = step
	}
	for _, stepName := range []string{"Accept", "Dispatch", "Process", "Repair"} {
		if flowSteps[stepName].CompletedAt == "" || flowSteps[stepName].CompletedBy != flowOperator.Nickname {
			t.Fatalf("flow step %s does not use stored progress: %+v", stepName, flowSteps[stepName])
		}
	}

	knowledgeCreateCtx, knowledgeCreateRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		fmt.Sprintf("/api/enterprise/v1/tickets/%d/knowledge-candidates", ticket.ID),
		tenantID,
		`{}`,
		gin.Param{Key: "id", Value: fmt.Sprint(ticket.ID)},
	)
	TicketKnowledgeCandidateCreate(knowledgeCreateCtx)
	var createdCandidate dto.TicketKnowledgeCandidateDTO
	decodeEnterpriseData(t, knowledgeCreateRec, &createdCandidate)
	expectedKnowledgeTitle := ticket.FaultCode + " Recalibrated pressure sensor"
	if !createdCandidate.Created || createdCandidate.TicketID != ticket.ID || createdCandidate.Title != expectedKnowledgeTitle || createdCandidate.ReviewStatus != string(enums.KnowledgeCandidateReviewStatusLowQuality) {
		t.Fatalf("knowledge candidate not created from ticket: %+v", createdCandidate)
	}

	knowledgeDuplicateCtx, knowledgeDuplicateRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		fmt.Sprintf("/api/enterprise/v1/tickets/%d/knowledge-candidates", ticket.ID),
		tenantID,
		`{}`,
		gin.Param{Key: "id", Value: fmt.Sprint(ticket.ID)},
	)
	TicketKnowledgeCandidateCreate(knowledgeDuplicateCtx)
	var duplicateCandidate dto.TicketKnowledgeCandidateDTO
	decodeEnterpriseData(t, knowledgeDuplicateRec, &duplicateCandidate)
	if duplicateCandidate.Created || duplicateCandidate.ID != createdCandidate.ID {
		t.Fatalf("knowledge candidate create is not idempotent: first=%+v duplicate=%+v", createdCandidate, duplicateCandidate)
	}

	knowledgeListCtx, knowledgeListRec := newEnterpriseContractContext(
		http.MethodGet,
		fmt.Sprintf("/api/enterprise/v1/tickets/%d/knowledge-candidates", ticket.ID),
		tenantID,
		gin.Param{Key: "id", Value: fmt.Sprint(ticket.ID)},
	)
	TicketKnowledgeCandidateList(knowledgeListCtx)
	var candidates []dto.TicketKnowledgeCandidateDTO
	decodeEnterpriseData(t, knowledgeListRec, &candidates)
	if len(candidates) != 1 || candidates[0].ID != createdCandidate.ID {
		t.Fatalf("knowledge candidates not listed: %+v", candidates)
	}

	queueCtx, queueRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/workbench/queue?page=1&page_size=20", tenantID)
	WorkbenchGetQueue(queueCtx)
	var queue dto.EnterpriseWorkbenchQueueDTO
	decodeEnterpriseData(t, queueRec, &queue)
	if len(queue.Conversations) != 1 || queue.Conversations[0].ID != conversation.ID || queue.Conversations[0].DeviceNo != device.DeviceNo {
		t.Fatalf("tenant queue conversations not mapped: %+v", queue.Conversations)
	}
	if queue.ConversationsPagination.Total != 1 || queue.ConversationsPagination.Page != 1 || queue.ConversationsPagination.PageSize != 20 {
		t.Fatalf("tenant queue conversation pagination mismatch: %+v", queue.ConversationsPagination)
	}
	if len(queue.Tickets) != 1 || queue.Tickets[0].ID != ticket.ID {
		t.Fatalf("tenant queue tickets not mapped: %+v", queue.Tickets)
	}
	if queue.TicketsPagination.Total != 1 || queue.TicketsPagination.Page != 1 || queue.TicketsPagination.PageSize != 20 {
		t.Fatalf("tenant queue ticket pagination mismatch: %+v", queue.TicketsPagination)
	}

	progressCtx, progressRec := newEnterpriseContractJSONContext(
		http.MethodPost,
		fmt.Sprintf("/api/enterprise/v1/tickets/%d/progress", ticket.ID),
		tenantID,
		`{"content":"Checked pressure sensor wiring"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(ticket.ID)},
	)
	TicketProgressCreate(progressCtx)
	var updated dto.TicketAggregateDTO
	decodeEnterpriseData(t, progressRec, &updated)
	if len(updated.Timeline) == 0 || updated.Timeline[len(updated.Timeline)-1].Content != "Checked pressure sensor wiring" {
		t.Fatalf("progress not returned in aggregate: %+v", updated.Timeline)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func TestEnterpriseKnowledgeEntriesListCombinesDocumentsAndFAQs(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	now := time.Now()
	kb := models.KnowledgeBase{
		TenantID:      1004,
		Name:          "HP Knowledge",
		KnowledgeType: "document",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	if err := db.Create(&models.KnowledgeDocument{
		TenantID:        1004,
		KnowledgeBaseID: kb.ID,
		Title:           "HP Operation Manual",
		Content:         "operate safely",
		Status:          enums.StatusOk,
		IndexStatus:     enums.KnowledgeDocumentIndexStatusIndexed,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := db.Create(&models.KnowledgeFAQ{
		TenantID:        1004,
		KnowledgeBaseID: kb.ID,
		Question:        "How to reset alarm E204?",
		Answer:          "Reset after inspection.",
		Status:          enums.StatusOk,
		IndexStatus:     enums.KnowledgeDocumentIndexStatusIndexed,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create faq: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/knowledge/entries?page_size=20", 1004)
	KnowledgeEntryList(ctx)

	var payload struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	}
	decodeEnterpriseData(t, rec, &payload)
	if payload.Total != 2 || len(payload.Items) != 2 {
		t.Fatalf("entries = (%d, %d), want total 2 len 2", payload.Total, len(payload.Items))
	}
}

func TestEnterpriseKnowledgeEntryCRUDPersistsMetadataAndWorkflow(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 1010)

	ctx, rec := newEnterpriseContractJSONContext(
		http.MethodPost,
		"/api/enterprise/v1/knowledge/entries",
		1010,
		fmt.Sprintf(`{
			"title":"HP-3000 Alarm E204 Guide",
			"content":"Check pressure, inspect the hydraulic line, and restart after clearing the root cause.",
			"category":"Repair Guide",
			"tags":["alarm","hydraulic"],
			"related_product_ids":[%d],
			"fault_codes":["E204"],
			"language":"en"
		}`, product.ID),
	)
	KnowledgeEntryCreate(ctx)

	var created struct {
		ID              int64    `json:"id"`
		Status          string   `json:"status"`
		Tags            []string `json:"tags"`
		FaultCodes      []string `json:"fault_codes"`
		RelatedProducts []struct {
			ID int64 `json:"id"`
		} `json:"related_products"`
	}
	decodeEnterpriseData(t, rec, &created)
	if created.ID <= 0 || created.Status != "draft" {
		t.Fatalf("created entry = %#v, want draft with encoded id", created)
	}
	if len(created.Tags) != 2 || created.Tags[0] != "alarm" {
		t.Fatalf("tags not persisted: %#v", created.Tags)
	}
	if len(created.FaultCodes) != 1 || created.FaultCodes[0] != "E204" {
		t.Fatalf("fault codes not persisted: %#v", created.FaultCodes)
	}
	if len(created.RelatedProducts) != 1 || created.RelatedProducts[0].ID != product.ID {
		t.Fatalf("related products not persisted: %#v", created.RelatedProducts)
	}

	ctx, rec = newEnterpriseContractContext(
		http.MethodPost,
		fmt.Sprintf("/api/enterprise/v1/knowledge/entries/%d/_submit", created.ID),
		1010,
		gin.Param{Key: "id", Value: fmt.Sprint(created.ID)},
	)
	KnowledgeEntrySubmit(ctx)
	var submitted struct {
		Status string `json:"status"`
	}
	decodeEnterpriseData(t, rec, &submitted)
	if submitted.Status != "review" {
		t.Fatalf("submitted status = %s, want review", submitted.Status)
	}

	ctx, rec = newEnterpriseContractContext(
		http.MethodPost,
		fmt.Sprintf("/api/enterprise/v1/knowledge/entries/%d/_publish", created.ID),
		1010,
		gin.Param{Key: "id", Value: fmt.Sprint(created.ID)},
	)
	KnowledgeEntryPublish(ctx)
	var published struct {
		Status string `json:"status"`
	}
	decodeEnterpriseData(t, rec, &published)
	if published.Status != "published" {
		t.Fatalf("published status = %s, want published", published.Status)
	}

	ctx, rec = newEnterpriseContractContext(
		http.MethodGet,
		fmt.Sprintf("/api/enterprise/v1/knowledge/entries/%d", created.ID),
		1010,
		gin.Param{Key: "id", Value: fmt.Sprint(created.ID)},
	)
	KnowledgeEntryGet(ctx)
	var detail struct {
		Status string `json:"status"`
	}
	decodeEnterpriseData(t, rec, &detail)
	if detail.Status != "published" {
		t.Fatalf("detail status = %s, want published", detail.Status)
	}
}

func TestEnterpriseGlobalSearchReturnsStoredTenantData(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	now := time.Now()
	product := seedEnterpriseProductContract(t, db, 1011)
	model := models.ProductModel{
		TenantID:    1011,
		ProductID:   product.ID,
		ModelCode:   "HP-MODEL",
		Name:        "HP Model",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&model).Error; err != nil {
		t.Fatalf("create model: %v", err)
	}
	if err := db.Create(&models.Device{
		TenantID:       1011,
		DeviceNo:       "DEV-HP-SEARCH",
		SerialNo:       "SN-HP-SEARCH",
		ProductID:      product.ID,
		ProductModelID: model.ID,
		RegionCode:     "US-WEST",
		Status:         enums.StatusOk,
		DeviceStatus:   string(enums.DeviceStatusOperational),
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := db.Create(&models.Ticket{
		TenantID:    1011,
		TicketNo:    "TK-HP-SEARCH",
		Title:       "HP pressure search ticket",
		ProductID:   product.ID,
		Status:      enums.TicketStatusPending,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	kb := models.KnowledgeBase{
		TenantID:      1011,
		Name:          "HP Search Knowledge",
		KnowledgeType: "document",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&kb).Error; err != nil {
		t.Fatalf("create knowledge base: %v", err)
	}
	if err := db.Create(&models.KnowledgeDocument{
		TenantID:        1011,
		KnowledgeBaseID: kb.ID,
		Title:           "HP search maintenance guide",
		Content:         "searchable content",
		Status:          enums.StatusOk,
		IndexStatus:     enums.KnowledgeDocumentIndexStatusIndexed,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create knowledge document: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/search/global?q=HP&scopes=ticket,device,knowledge,product", 1011)
	GlobalSearch(ctx)

	var payload struct {
		Items []struct {
			Type string `json:"type"`
		} `json:"items"`
		Total int `json:"total"`
	}
	decodeEnterpriseData(t, rec, &payload)
	if payload.Total < 4 {
		t.Fatalf("total = %d, want at least 4; items=%#v", payload.Total, payload.Items)
	}
	seen := map[string]bool{}
	for _, item := range payload.Items {
		seen[item.Type] = true
	}
	for _, want := range []string{"ticket", "device", "knowledge", "product"} {
		if !seen[want] {
			t.Fatalf("missing search type %s in %#v", want, seen)
		}
	}
}

func TestEnterpriseIAMEndpointsReadTenantScopedIdentityData(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	now := time.Now()
	tenantID := int64(1022)
	otherTenantID := int64(1023)

	for _, tenant := range []models.Tenant{
		{ID: tenantID, Name: "IAM Tenant", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
		{ID: otherTenantID, Name: "Other IAM Tenant", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}},
	} {
		if err := db.Create(&tenant).Error; err != nil {
			t.Fatalf("create tenant: %v", err)
		}
	}

	user := models.User{Username: "iam.engineer", Nickname: "IAM Engineer", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	department := models.Department{
		TenantID:       tenantID,
		DepartmentCode: "NA-L2",
		Name:           "North America L2",
		Path:           "/service/na-l2",
		Depth:          2,
		RegionCode:     "NA",
		Status:         enums.StatusOk,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&department).Error; err != nil {
		t.Fatalf("create department: %v", err)
	}
	member := models.TenantMember{
		TenantID:     tenantID,
		UserID:       user.ID,
		DepartmentID: department.ID,
		MemberNo:     "ENG-001",
		DisplayName:  "IAM Engineer",
		JobTitle:     "L2 Engineer",
		MemberType:   "engineer",
		Status:       enums.StatusOk,
		JoinedAt:     &now,
		AuditFields:  models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create tenant member: %v", err)
	}
	if err := db.Create(&models.EngineerProfile{
		TenantID:           tenantID,
		MemberID:           member.ID,
		SkillTagsJSON:      `["hydraulic","controller"]`,
		LanguagesJSON:      `["en-US"]`,
		ServiceRegionsJSON: `["NA"]`,
		Timezone:           "America/Los_Angeles",
		MaxTicketLoad:      6,
		DispatchEnabled:    true,
		Status:             enums.StatusOk,
		AuditFields:        models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create engineer profile: %v", err)
	}
	product := models.Product{
		TenantID:    tenantID,
		Name:        "Hydraulic Pump",
		Code:        "HP-01",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	productTeam := models.AgentTeam{
		TenantID:    tenantID,
		ProductID:   product.ID,
		TeamType:    services.AgentTeamTypeProductRepair,
		Name:        "Hydraulic Pump 维修组",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&productTeam).Error; err != nil {
		t.Fatalf("create product repair team: %v", err)
	}
	if err := db.Create(&models.AgentTeamMember{
		TenantID:        tenantID,
		TeamID:          productTeam.ID,
		UserID:          user.ID,
		MemberID:        member.ID,
		DispatchEnabled: true,
		DispatchWeight:  1,
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create product team membership: %v", err)
	}
	role := models.AuthRole{
		TenantID:    tenantID,
		DomainType:  models.DomainTypeEnterprise,
		Code:        "engineer",
		Name:        "Engineer",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := db.Create(&models.AuthRoleBinding{
		TenantID:    tenantID,
		DomainType:  models.DomainTypeEnterprise,
		RoleID:      role.ID,
		SubjectType: models.SubjectTypeTenantMember,
		SubjectID:   member.ID,
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create role binding: %v", err)
	}
	if err := db.Create(&models.AuthRolePermission{
		TenantID:       tenantID,
		RoleID:         role.ID,
		PermissionCode: "menu:enterprise.tickets",
		Effect:         "allow",
		Status:         enums.StatusOk,
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create role permission: %v", err)
	}
	customerOrg := models.CustomerOrg{
		TenantID:      tenantID,
		CustomerNo:    "CUST-001",
		Name:          "Atlas Machines",
		CountryRegion: "US",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&customerOrg).Error; err != nil {
		t.Fatalf("create customer org: %v", err)
	}
	if err := db.Create(&models.CustomerUser{
		TenantID:      tenantID,
		CustomerOrgID: customerOrg.ID,
		DisplayName:   "Maria Gomez",
		Email:         "maria@example.com",
		Locale:        "en-US",
		Timezone:      "America/New_York",
		Status:        enums.StatusOk,
		LastSeenAt:    &now,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create customer user: %v", err)
	}
	partner := models.PartnerCompany{
		TenantID:      tenantID,
		PartnerNo:     "PART-001",
		Name:          "EuroHydraulic",
		PartnerType:   "field_service",
		CountryRegion: "DE",
		ContactName:   "Anna Weber",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&partner).Error; err != nil {
		t.Fatalf("create partner: %v", err)
	}
	if err := db.Create(&models.PartnerAccount{
		TenantID:         tenantID,
		PartnerCompanyID: partner.ID,
		DisplayName:      "Partner Engineer",
		Email:            "partner@example.com",
		Status:           enums.StatusOk,
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create partner account: %v", err)
	}
	if err := db.Create(&models.PartnerContract{
		TenantID:         tenantID,
		PartnerCompanyID: partner.ID,
		ContractNo:       "PC-001",
		EffectiveFrom:    now,
		ServiceLevel:     "L2",
		Status:           enums.StatusOk,
		AuditFields:      models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create partner contract: %v", err)
	}
	if err := db.Create(&models.AuthAuditLog{
		TenantID:         tenantID,
		DomainType:       models.DomainTypeEnterprise,
		ActorUserID:      user.ID,
		ActorSubjectType: models.SubjectTypeTenantMember,
		ActorSubjectID:   member.ID,
		TargetType:       "auth_role",
		TargetID:         fmt.Sprint(role.ID),
		Action:           "auth_policy.saved",
		RiskLevel:        models.RiskLevelHigh,
		Status:           models.AuditStatusSuccess,
		OccurredAt:       now,
	}).Error; err != nil {
		t.Fatalf("create audit log: %v", err)
	}
	if err := db.Create(&models.TenantMember{
		TenantID:    otherTenantID,
		UserID:      user.ID,
		DisplayName: "Other Tenant Member",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create other tenant member: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/iam/members", tenantID)
	IAMMemberList(ctx)
	var members struct {
		Results []struct {
			DisplayName    string   `json:"display_name"`
			DepartmentName string   `json:"department_name"`
			Roles          []string `json:"roles"`
			Dispatch       bool     `json:"dispatch_enabled"`
			ProductGroups  []struct {
				TeamName    string `json:"team_name"`
				ProductName string `json:"product_name"`
			} `json:"product_groups"`
		} `json:"results"`
		Page struct {
			Total int64 `json:"total"`
		} `json:"page"`
	}
	decodeEnterpriseData(t, rec, &members)
	if members.Page.Total != 1 || len(members.Results) != 1 {
		t.Fatalf("member total/results = %d/%d, want 1/1", members.Page.Total, len(members.Results))
	}
	if members.Results[0].DepartmentName != department.Name || !members.Results[0].Dispatch || len(members.Results[0].Roles) != 1 || members.Results[0].Roles[0] != role.Code {
		t.Fatalf("unexpected member payload: %#v", members.Results[0])
	}
	if len(members.Results[0].ProductGroups) != 1 || members.Results[0].ProductGroups[0].ProductName != product.Name || members.Results[0].ProductGroups[0].TeamName != productTeam.Name {
		t.Fatalf("unexpected member product groups: %#v", members.Results[0].ProductGroups)
	}

	customerCtx, customerRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/iam/customer-users", tenantID)
	IAMCustomerUserList(customerCtx)
	var customers struct {
		Results []struct {
			CustomerOrgName string `json:"customer_org_name"`
		} `json:"results"`
	}
	decodeEnterpriseData(t, customerRec, &customers)
	if len(customers.Results) != 1 || customers.Results[0].CustomerOrgName != customerOrg.Name {
		t.Fatalf("unexpected customers: %#v", customers.Results)
	}

	partnerCtx, partnerRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/iam/partners", tenantID)
	IAMPartnerList(partnerCtx)
	var partners struct {
		Results []struct {
			Name          string `json:"name"`
			AccountCount  int64  `json:"account_count"`
			ContractCount int64  `json:"contract_count"`
		} `json:"results"`
	}
	decodeEnterpriseData(t, partnerRec, &partners)
	if len(partners.Results) != 1 || partners.Results[0].Name != partner.Name || partners.Results[0].AccountCount != 1 || partners.Results[0].ContractCount != 1 {
		t.Fatalf("unexpected partners: %#v", partners.Results)
	}

	deptCtx, deptRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/iam/departments", tenantID)
	IAMDepartmentList(deptCtx)
	var departments struct {
		Results []struct {
			Name        string `json:"name"`
			MemberCount int64  `json:"member_count"`
		} `json:"results"`
	}
	decodeEnterpriseData(t, deptRec, &departments)
	if len(departments.Results) != 1 || departments.Results[0].Name != department.Name || departments.Results[0].MemberCount != 1 {
		t.Fatalf("unexpected departments: %#v", departments.Results)
	}

	roleCtx, roleRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/iam/roles", tenantID)
	IAMRoleList(roleCtx)
	var roles []struct {
		Code        string   `json:"code"`
		Permissions []string `json:"permissions"`
	}
	decodeEnterpriseData(t, roleRec, &roles)
	if len(roles) != 1 || roles[0].Code != role.Code || len(roles[0].Permissions) != 1 {
		t.Fatalf("unexpected roles: %#v", roles)
	}

	auditCtx, auditRec := newEnterpriseContractContext(http.MethodGet, "/api/enterprise/v1/iam/audit", tenantID)
	IAMAuditList(auditCtx)
	var audits struct {
		Results []struct {
			Action string `json:"action"`
		} `json:"results"`
	}
	decodeEnterpriseData(t, auditRec, &audits)
	if len(audits.Results) != 1 || audits.Results[0].Action != "auth_policy.saved" {
		t.Fatalf("unexpected audits: %#v", audits.Results)
	}
}

// newEnterprisePrincipalContext 构造携带企业认证主体的请求上下文，
// 同时写入中间件与旧版 service 两个 principal 存储键。
func newEnterprisePrincipalContext(method string, path string, tenantID int64, body string, params ...gin.Param) (*gin.Context, *httptest.ResponseRecorder) {
	ctx, rec := newEnterpriseContractJSONContext(method, path, tenantID, body, params...)
	principal := &dto.AuthPrincipal{
		UserID:      9001,
		Username:    "ent-admin",
		TenantID:    tenantID,
		Roles:       []string{services.EnterpriseRoleAdmin},
		Permissions: constants.PermissionCodes(),
	}
	ctx.Set("middlewareAuthPrincipal", principal)
	ctx.Set("authPrincipal", principal)
	return ctx, rec
}

// TestEnterpriseProductCreateUpdatePersistsAllFields 覆盖 P0 验收：
// 产品创建/更新请求 DTO 与前端 snake_case 契约统一，
// description、产品线与默认语言完整保存，刷新（重新读取）后不丢字段。
func TestEnterpriseProductCreateUpdatePersistsAllFields(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	now := time.Now()
	tenantID := int64(12001)
	if err := db.Create(&models.Tenant{
		ID:          tenantID,
		Name:        "Product Contract Tenant",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	future := now.Add(24 * time.Hour)
	if err := db.Create(&models.Sub2APITenantAccount{
		TenantID: tenantID, Sub2APIAccountID: "product-contract-tenant", AccountName: "Product Contract Tenant",
		LoginEmail: "product-contract@example.test", LoginPasswordCiphertext: "test-encrypted-password",
		AccessTokenExpiresAt: &future, AccountStatus: "active", ProvisionStatus: "active", Balance: 10,
		DefaultKeyID: "product-contract-key", DefaultKeyName: "Product Contract Key",
		DefaultKeyCiphertext: "test-encrypted-key", DefaultKeyStatus: "active", DefaultKeyExpiresAt: &future,
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create tenant AI workspace: %v", err)
	}
	owner := seedEnterpriseProductOwner(t, db, tenantID, 1200101, "Product Owner")

	missingOwnerCtx, missingOwnerRec := newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products",
		tenantID,
		`{"code":"missing-owner","name":"Missing Owner","description":"No owner","category":"hydraulic","product_line":"Excavator Line","default_locale":"zh-CN"}`,
	)
	ProductCreate(missingOwnerCtx)
	assertEnterpriseEnvelopeError(t, missingOwnerRec)

	// 1. 创建：snake_case 请求体必须完整保存 description / product_line / default_locale。
	createCtx, createRec := newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products",
		tenantID,
		fmt.Sprintf(`{"name":"Hydraulic Pump HP-9000","description":"High pressure hydraulic pump","category":"hydraulic","product_line":"Excavator Line","owner_member_id":%d,"default_locale":"zh-CN"}`, owner.ID),
	)
	ProductCreate(createCtx)
	var created dto.EnterpriseProductListItemDTO
	decodeEnterpriseData(t, createRec, &created)

	if !strings.HasPrefix(created.Code, "PROD-") {
		t.Fatalf("created code = %q, want generated PROD- code", created.Code)
	}
	if created.Description != "High pressure hydraulic pump" {
		t.Fatalf("created description = %q, want persisted", created.Description)
	}
	if created.ProductLine != "Excavator Line" {
		t.Fatalf("created product_line = %q, want %q", created.ProductLine, "Excavator Line")
	}
	if created.DefaultLocale != "zh-CN" {
		t.Fatalf("created default_locale = %q, want zh-CN", created.DefaultLocale)
	}
	if created.Status != "active" {
		t.Fatalf("created status = %q, want active", created.Status)
	}
	if created.OwnerMemberID != owner.ID || created.OwnerUserID != owner.UserID || created.OwnerName != owner.DisplayName {
		t.Fatalf("created owner = (%d, %d, %q), want (%d, %d, %q)", created.OwnerMemberID, created.OwnerUserID, created.OwnerName, owner.ID, owner.UserID, owner.DisplayName)
	}

	var stored models.Product
	if err := db.First(&stored, created.ID).Error; err != nil {
		t.Fatalf("load created product: %v", err)
	}
	if stored.Description != "High pressure hydraulic pump" {
		t.Fatalf("db description = %q, want persisted", stored.Description)
	}
	if stored.DefaultLocale != "zh-CN" {
		t.Fatalf("db default_locale = %q, want zh-CN", stored.DefaultLocale)
	}
	if stored.OwnerMemberID != owner.ID {
		t.Fatalf("db owner_member_id = %d, want %d", stored.OwnerMemberID, owner.ID)
	}
	var productTeam models.AgentTeam
	if err := db.Where("tenant_id = ? AND product_id = ? AND team_type = ?", tenantID, stored.ID, services.AgentTeamTypeProductRepair).First(&productTeam).Error; err != nil {
		t.Fatalf("load product repair team: %v", err)
	}
	if productTeam.LeaderUserID != owner.UserID {
		t.Fatalf("product team leader_user_id = %d, want owner user %d", productTeam.LeaderUserID, owner.UserID)
	}
	nextOwner := seedEnterpriseProductOwner(t, db, tenantID, 1200102, "Next Product Owner")
	if err := services.AgentTeamService.UpdateAgentTeam(request.UpdateAgentTeamRequest{
		ID:             productTeam.ID,
		Name:           productTeam.Name,
		LeaderUserID:   nextOwner.UserID,
		AssignmentMode: productTeam.AssignmentMode,
		Status:         int(productTeam.Status),
		Description:    productTeam.Description,
		Remark:         productTeam.Remark,
	}, &dto.AuthPrincipal{UserID: 9001, Username: "ent-admin", TenantID: tenantID}); err != nil {
		t.Fatalf("update product team leader: %v", err)
	}
	if err := db.First(&stored, created.ID).Error; err != nil {
		t.Fatalf("reload product after leader change: %v", err)
	}
	if stored.OwnerMemberID != nextOwner.ID {
		t.Fatalf("product owner after leader change = %d, want %d", stored.OwnerMemberID, nextOwner.ID)
	}
	if err := db.First(&productTeam, productTeam.ID).Error; err != nil {
		t.Fatalf("reload product team after leader change: %v", err)
	}
	if productTeam.LeaderUserID != nextOwner.UserID {
		t.Fatalf("product team leader after change = %d, want %d", productTeam.LeaderUserID, nextOwner.UserID)
	}
	finalOwner := seedEnterpriseProductOwner(t, db, tenantID, 1200103, "Final Product Owner")
	var line models.ProductLine
	if err := db.First(&line, stored.ProductLineID).Error; err != nil {
		t.Fatalf("product line not created: %v", err)
	}
	if line.TenantID != tenantID || line.Name != "Excavator Line" {
		t.Fatalf("product line = (tenant %d, %q), want tenant %d / Excavator Line", line.TenantID, line.Name, tenantID)
	}

	// 2. PATCH 部分字段：未携带字段保持原值（合并语义），状态同步更新。
	updateCtx, updateRec := newEnterprisePrincipalContext(
		http.MethodPatch,
		"/api/enterprise/v1/products/"+fmt.Sprint(created.ID),
		tenantID,
		fmt.Sprintf(`{"description":"Updated pump description","status":"inactive","owner_member_id":%d}`, finalOwner.ID),
		gin.Param{Key: "id", Value: fmt.Sprint(created.ID)},
	)
	ProductUpdate(updateCtx)
	var updated dto.EnterpriseProductListItemDTO
	decodeEnterpriseData(t, updateRec, &updated)

	if updated.Description != "Updated pump description" {
		t.Fatalf("updated description = %q, want updated", updated.Description)
	}
	if updated.Status != "inactive" {
		t.Fatalf("updated status = %q, want inactive", updated.Status)
	}
	if updated.Name != "Hydraulic Pump HP-9000" || updated.DefaultLocale != "zh-CN" || updated.ProductLine != "Excavator Line" {
		t.Fatalf("PATCH cleared untouched fields: name=%q locale=%q line=%q", updated.Name, updated.DefaultLocale, updated.ProductLine)
	}
	if updated.OwnerMemberID != finalOwner.ID || updated.OwnerUserID != finalOwner.UserID || updated.OwnerName != finalOwner.DisplayName {
		t.Fatalf("updated owner = (%d, %d, %q), want (%d, %d, %q)", updated.OwnerMemberID, updated.OwnerUserID, updated.OwnerName, finalOwner.ID, finalOwner.UserID, finalOwner.DisplayName)
	}

	var afterUpdate models.Product
	if err := db.First(&afterUpdate, created.ID).Error; err != nil {
		t.Fatalf("reload updated product: %v", err)
	}
	if afterUpdate.Status != enums.StatusDisabled {
		t.Fatalf("db status = %d, want %d (disabled)", afterUpdate.Status, enums.StatusDisabled)
	}
	if afterUpdate.Name != "Hydraulic Pump HP-9000" {
		t.Fatalf("db name = %q, want unchanged", afterUpdate.Name)
	}
	if afterUpdate.OwnerMemberID != finalOwner.ID {
		t.Fatalf("db owner_member_id = %d, want %d", afterUpdate.OwnerMemberID, finalOwner.ID)
	}
	if err := db.First(&productTeam, productTeam.ID).Error; err != nil {
		t.Fatalf("reload product team after owner patch: %v", err)
	}
	if productTeam.LeaderUserID != finalOwner.UserID {
		t.Fatalf("product team leader after owner patch = %d, want %d", productTeam.LeaderUserID, finalOwner.UserID)
	}

	// 3. PATCH 不携带 description 时不得清空已有描述。
	patchCtx, patchRec := newEnterprisePrincipalContext(
		http.MethodPatch,
		"/api/enterprise/v1/products/"+fmt.Sprint(created.ID),
		tenantID,
		`{"name":"Hydraulic Pump HP-9000"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(created.ID)},
	)
	ProductUpdate(patchCtx)
	var patched dto.EnterpriseProductListItemDTO
	decodeEnterpriseData(t, patchRec, &patched)
	if patched.Description != "Updated pump description" {
		t.Fatalf("description after unrelated PATCH = %q, want kept", patched.Description)
	}

	// 4. 跨租户更新必须失败。
	if err := db.Create(&models.Tenant{
		ID:          12002,
		Name:        "Other Tenant",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create other tenant: %v", err)
	}
	otherCtx, otherRec := newEnterprisePrincipalContext(
		http.MethodPatch,
		"/api/enterprise/v1/products/"+fmt.Sprint(created.ID),
		12002,
		`{"name":"Hijacked"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(created.ID)},
	)
	ProductUpdate(otherCtx)
	if envelope := decodeEnterpriseEnvelope(t, otherRec); envelope.Success {
		t.Fatalf("cross-tenant update success = true, want false")
	}
}

// TestEnterpriseProductModelLifecycle 覆盖 P1：型号启停（停用前校验设备引用）。
func TestEnterpriseProductModelLifecycle(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 13001)
	now := time.Now()

	// 创建型号
	ctx, rec := newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/models",
		13001,
		`{"model_code":"HP-M1","name":"Model One","version_policy":"v1","region_scope_json":"[\"US\"]"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ProductModelCreate(ctx)
	var model map[string]any
	decodeEnterpriseData(t, rec, &model)
	if model["model_code"] != "HP-M1" || model["status"] != "active" {
		t.Fatalf("created model = %#v", model)
	}
	modelID := int64(model["id"].(float64))

	// 停用型号（无引用，应成功）
	ctx, rec = newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/models/1/_disable",
		13001,
		``,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "modelId", Value: fmt.Sprint(modelID)},
	)
	ProductModelDisable(ctx)
	if envelope := decodeEnterpriseEnvelope(t, rec); !envelope.Success {
		t.Fatalf("model disable failed: %s", envelope.Message)
	}
	var stored models.ProductModel
	if err := db.First(&stored, modelID).Error; err != nil || stored.Status != enums.StatusDisabled {
		t.Fatalf("model status = %v, want disabled", stored.Status)
	}

	// 重新启用
	ctx, rec = newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/models/1/_enable",
		13001,
		``,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "modelId", Value: fmt.Sprint(modelID)},
	)
	ProductModelEnable(ctx)
	if envelope := decodeEnterpriseEnvelope(t, rec); !envelope.Success {
		t.Fatalf("model enable failed: %s", envelope.Message)
	}

	// 存在设备引用时停用必须被拒绝
	if err := db.Create(&models.Device{
		TenantID:       13001,
		DeviceNo:       "DEV-M1",
		ProductID:      product.ID,
		ProductModelID: modelID,
		Status:         enums.StatusOk,
		DeviceStatus:   string(enums.DeviceStatusOperational),
		AuditFields:    models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	ctx, rec = newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/models/1/_disable",
		13001,
		``,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "modelId", Value: fmt.Sprint(modelID)},
	)
	ProductModelDisable(ctx)
	if envelope := decodeEnterpriseEnvelope(t, rec); envelope.Success {
		t.Fatalf("model disable with devices success = true, want false")
	}
}

// TestEnterpriseModuleModelReplace 覆盖 P1：模块适用型号整体替换与跨产品校验。
func TestEnterpriseModuleModelReplace(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 13002)
	other := seedEnterpriseProductContract(t, db, 13003)
	now := time.Now()

	module := models.ProductModule{
		TenantID:    13002,
		ProductID:   product.ID,
		ModuleCode:  "HYD-PUMP",
		Name:        "Hydraulic Pump",
		Status:      enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&module).Error; err != nil {
		t.Fatalf("create module: %v", err)
	}
	modelA := models.ProductModel{TenantID: 13002, ProductID: product.ID, ModelCode: "MA", Name: "A", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	modelB := models.ProductModel{TenantID: 13002, ProductID: product.ID, ModelCode: "MB", Name: "B", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	foreignModel := models.ProductModel{TenantID: 13003, ProductID: other.ID, ModelCode: "MX", Name: "X", Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now}}
	for _, m := range []*models.ProductModel{&modelA, &modelB, &foreignModel} {
		if err := db.Create(m).Error; err != nil {
			t.Fatalf("create model: %v", err)
		}
	}

	// 整体替换为 A+B
	ctx, rec := newEnterprisePrincipalContext(
		http.MethodPut,
		"/api/enterprise/v1/products/1/modules/1/models",
		13002,
		fmt.Sprintf(`{"model_ids":[%d,%d]}`, modelA.ID, modelB.ID),
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "moduleId", Value: fmt.Sprint(module.ID)},
	)
	ProductModuleModelsReplace(ctx)
	var replaced struct {
		ModelIDs []int64 `json:"model_ids"`
	}
	decodeEnterpriseData(t, rec, &replaced)
	if len(replaced.ModelIDs) != 2 {
		t.Fatalf("model_ids = %v, want 2 links", replaced.ModelIDs)
	}

	// 再次替换为仅 A：B 的关联应被删除（整体替换语义）
	ctx, rec = newEnterprisePrincipalContext(
		http.MethodPut,
		"/api/enterprise/v1/products/1/modules/1/models",
		13002,
		fmt.Sprintf(`{"model_ids":[%d]}`, modelA.ID),
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "moduleId", Value: fmt.Sprint(module.ID)},
	)
	ProductModuleModelsReplace(ctx)
	decodeEnterpriseData(t, rec, &replaced)
	if len(replaced.ModelIDs) != 1 || replaced.ModelIDs[0] != modelA.ID {
		t.Fatalf("model_ids after replace = %v, want [%d]", replaced.ModelIDs, modelA.ID)
	}
	var linkCount int64
	db.Model(&models.ProductModuleModelLink{}).Where("product_module_id = ?", module.ID).Count(&linkCount)
	if linkCount != 1 {
		t.Fatalf("link count = %d, want 1", linkCount)
	}

	// 跨产品型号必须被拒绝
	ctx, rec = newEnterprisePrincipalContext(
		http.MethodPut,
		"/api/enterprise/v1/products/1/modules/1/models",
		13002,
		fmt.Sprintf(`{"model_ids":[%d]}`, foreignModel.ID),
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "moduleId", Value: fmt.Sprint(module.ID)},
	)
	ProductModuleModelsReplace(ctx)
	if envelope := decodeEnterpriseEnvelope(t, rec); envelope.Success {
		t.Fatalf("cross-product model replace success = true, want false")
	}
}

// TestEnterpriseManualLifecycle 覆盖 P1：手册挂载、发布、下架与归属校验。
func TestEnterpriseManualLifecycle(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 13004)
	now := time.Now()
	kb := models.KnowledgeBase{
		TenantID:      13004,
		Name:          "Product Manuals",
		KnowledgeType: "document",
		Status:        enums.StatusOk,
		AuditFields:   models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&kb).Error; err != nil {
		t.Fatalf("create kb: %v", err)
	}
	doc := models.KnowledgeDocument{
		TenantID:        13004,
		KnowledgeBaseID: kb.ID,
		Title:           "HP-3000 Operation Manual",
		Content:         "HP-3000 operation and repair instructions.",
		ReviewStatus:    "published",
		Language:        "en-US",
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&doc).Error; err != nil {
		t.Fatalf("create doc: %v", err)
	}
	revision := models.KnowledgeRevision{
		TenantID:        doc.TenantID,
		KnowledgeBaseID: doc.KnowledgeBaseID,
		EntryType:       "document",
		EntryID:         doc.ID,
		VersionNo:       1,
		Title:           doc.Title,
		Content:         doc.Content,
		Language:        doc.Language,
		ReviewStatus:    "published",
		PublishedAt:     &now,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&revision).Error; err != nil {
		t.Fatalf("create published revision: %v", err)
	}
	if err := db.Model(&doc).Updates(map[string]any{
		"current_revision_id":   revision.ID,
		"published_revision_id": revision.ID,
	}).Error; err != nil {
		t.Fatalf("attach published revision: %v", err)
	}
	foreignDoc := models.KnowledgeDocument{
		TenantID:        99999,
		KnowledgeBaseID: 99999,
		Title:           "Foreign",
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&foreignDoc).Error; err != nil {
		t.Fatalf("create foreign doc: %v", err)
	}

	// 跨租户条目必须被拒绝
	ctx, rec := newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/manuals/_link",
		13004,
		fmt.Sprintf(`{"knowledge_base_id":%d,"knowledge_entry_id":%d,"link_type":"manual","language":"en-US"}`, kb.ID, foreignDoc.ID),
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ProductManualLinkCreate(ctx)
	if envelope := decodeEnterpriseEnvelope(t, rec); envelope.Success {
		t.Fatalf("foreign entry link success = true, want false")
	}

	// 正常挂载
	ctx, rec = newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/manuals/_link",
		13004,
		fmt.Sprintf(`{"knowledge_base_id":%d,"knowledge_entry_id":%d,"link_type":"manual","language":"en-US","version":"v1","visibility":"public"}`, kb.ID, doc.ID),
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ProductManualLinkCreate(ctx)
	var link map[string]any
	decodeEnterpriseData(t, rec, &link)
	if link["publish_status"] != "draft" || link["entry_review_status"] != "published" {
		t.Fatalf("link = %#v", link)
	}
	linkID := int64(link["id"].(float64))

	// 重复挂载相同维度必须被拒绝
	ctx, rec = newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/manuals/_link",
		13004,
		fmt.Sprintf(`{"knowledge_base_id":%d,"knowledge_entry_id":%d,"link_type":"manual","language":"en-US","version":"v1","visibility":"public"}`, kb.ID, doc.ID),
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ProductManualLinkCreate(ctx)
	if envelope := decodeEnterpriseEnvelope(t, rec); envelope.Success {
		t.Fatalf("duplicate link success = true, want false")
	}

	// 发布
	ctx, rec = newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/manuals/1/_publish",
		13004,
		``,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "linkId", Value: fmt.Sprint(linkID)},
	)
	ProductManualPublish(ctx)
	var published map[string]any
	decodeEnterpriseData(t, rec, &published)
	if published["publish_status"] != "published" {
		t.Fatalf("publish_status = %v, want published", published["publish_status"])
	}

	// 下架
	ctx, rec = newEnterprisePrincipalContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/manuals/1/_deprecate",
		13004,
		``,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "linkId", Value: fmt.Sprint(linkID)},
	)
	ProductManualDeprecate(ctx)
	var deprecated map[string]any
	decodeEnterpriseData(t, rec, &deprecated)
	if deprecated["publish_status"] != "deprecated" {
		t.Fatalf("publish_status = %v, want deprecated", deprecated["publish_status"])
	}

	// 解除关联
	ctx, rec = newEnterprisePrincipalContext(
		http.MethodDelete,
		"/api/enterprise/v1/products/1/manuals/1",
		13004,
		``,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "linkId", Value: fmt.Sprint(linkID)},
	)
	ProductManualDelete(ctx)
	if envelope := decodeEnterpriseEnvelope(t, rec); !envelope.Success {
		t.Fatalf("manual delete failed: %s", envelope.Message)
	}
	var stored models.ProductKnowledgeLink
	if err := db.First(&stored, linkID).Error; err != nil || stored.Status != enums.StatusDeleted {
		t.Fatalf("link status = %v, want deleted", stored.Status)
	}
}

func TestEnterpriseProductManualFileUploadListAndDelete(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 13006)
	storageRoot := t.TempDir()
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Default:         enums.AssetProviderLocal,
			MaxUploadSizeMB: 20,
			Local: config.LocalStorageConfig{
				Root:    storageRoot,
				BaseURL: "/storage",
			},
		},
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "manual.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte("remotehelpdesk manual content")); err != nil {
		t.Fatalf("write upload body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/manual-files/_upload",
		13006,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/enterprise/v1/products/1/manual-files/_upload", body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	principal := &dto.AuthPrincipal{
		UserID:   9001,
		Username: "ent-admin",
		TenantID: 13006,
	}
	ctx.Set("middlewareAuthPrincipal", principal)
	ctx.Set("authPrincipal", principal)
	ProductManualFileUpload(ctx)

	var uploaded dto.EnterpriseProductManualFileDTO
	decodeEnterpriseData(t, rec, &uploaded)
	if uploaded.Filename != "manual.pdf" {
		t.Fatalf("filename = %q, want manual.pdf", uploaded.Filename)
	}
	if uploaded.AssetID <= 0 || uploaded.URL == "" {
		t.Fatalf("uploaded manual = %#v", uploaded)
	}

	ctx, rec = newEnterpriseContractJSONContext(
		http.MethodPatch,
		"/api/enterprise/v1/products/1/manual-files/1",
		13006,
		`{"title":"HP-300 用户手册","language":"en","version":"v2.0","visibility":"internal"}`,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "manualFileId", Value: fmt.Sprint(uploaded.ID)},
	)
	ctx.Set("middlewareAuthPrincipal", principal)
	ctx.Set("authPrincipal", principal)
	ProductManualFileUpdate(ctx)
	var updated dto.EnterpriseProductManualFileDTO
	decodeEnterpriseData(t, rec, &updated)
	if updated.Title != "HP-300 用户手册" || updated.Language != "en" || updated.Version != "v2.0" || updated.Visibility != "internal" {
		t.Fatalf("updated manual metadata = %#v", updated)
	}
	customerFiles, err := services.ProductManualFileService.ListCustomerManualFiles(13006, product.ID)
	if err != nil {
		t.Fatalf("list customer manual files: %v", err)
	}
	if len(customerFiles) != 0 {
		t.Fatalf("internal manual exposed to customer: %#v", customerFiles)
	}

	ctx, rec = newEnterpriseContractContext(
		http.MethodGet,
		"/api/enterprise/v1/products/1/manual-files",
		13006,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ProductManualFiles(ctx)
	var list []dto.EnterpriseProductManualFileDTO
	decodeEnterpriseData(t, rec, &list)
	if len(list) != 1 {
		t.Fatalf("manual file count = %d, want 1", len(list))
	}
	if list[0].Title != updated.Title || list[0].Visibility != "internal" {
		t.Fatalf("listed manual metadata = %#v, want updated values", list[0])
	}

	ctx, rec = newEnterpriseContractContext(
		http.MethodGet,
		"/api/enterprise/v1/products/1/manual-files/1/content",
		13006,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "manualFileId", Value: fmt.Sprint(uploaded.ID)},
	)
	ProductManualFileContent(ctx)
	if rec.Code != http.StatusOK {
		t.Fatalf("manual file content status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "remotehelpdesk manual content" {
		t.Fatalf("manual file content = %q", got)
	}
	if disposition := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(disposition, "inline") {
		t.Fatalf("content disposition = %q, want inline", disposition)
	}

	ctx, rec = newEnterpriseContractContext(
		http.MethodDelete,
		"/api/enterprise/v1/products/1/manual-files/1",
		13006,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
		gin.Param{Key: "manualFileId", Value: fmt.Sprint(uploaded.ID)},
	)
	ctx.Set("middlewareAuthPrincipal", principal)
	ctx.Set("authPrincipal", principal)
	ProductManualFileDelete(ctx)
	if envelope := decodeEnterpriseEnvelope(t, rec); !envelope.Success {
		t.Fatalf("manual file delete failed: %s", envelope.Message)
	}

	ctx, rec = newEnterpriseContractContext(
		http.MethodGet,
		"/api/enterprise/v1/products/1/manual-files",
		13006,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ProductManualFiles(ctx)
	list = nil
	decodeEnterpriseData(t, rec, &list)
	if len(list) != 0 {
		t.Fatalf("manual file count after delete = %d, want 0", len(list))
	}

	asset := services.AssetService.Get(uploaded.AssetID)
	if asset == nil || asset.Status != enums.AssetStatusDeleted {
		t.Fatalf("asset status = %#v, want deleted", asset)
	}
}

func TestEnterpriseManualAndKnowledgeDocumentUploadsKeepDistinctSources(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 13007)
	storageRoot := t.TempDir()
	config.SetCurrent(&config.Config{
		Storage: config.StorageConfig{
			Default:         enums.AssetProviderLocal,
			MaxUploadSizeMB: 20,
			Local: config.LocalStorageConfig{
				Root:    storageRoot,
				BaseURL: "/storage",
			},
		},
	})

	principal := &dto.AuthPrincipal{
		UserID:   9002,
		Username: "ent-admin",
		TenantID: 13007,
		Status:   enums.StatusOk,
	}
	if _, err := services.ProductCenterService.CreateProductKnowledgeBase(
		13007,
		product.ID,
		dto.EnterpriseProductKnowledgeBaseCreateRequest{
			Name:           "产品测试1 知识库",
			SupportLocales: []string{"en-US"},
		},
		principal,
	); err != nil {
		t.Fatalf("create product knowledge base: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "manual.docx")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(buildEnterpriseTestDOCX(t, "RemoteHelpDesk knowledge sync", "DOCX manual content")); err != nil {
		t.Fatalf("write docx upload body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	ctx, rec := newEnterpriseContractContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/manual-files/_upload",
		13007,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/enterprise/v1/products/1/manual-files/_upload", body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Set("middlewareAuthPrincipal", principal)
	ctx.Set("authPrincipal", principal)
	ProductManualFileUpload(ctx)

	var uploaded dto.EnterpriseProductManualFileDTO
	decodeEnterpriseData(t, rec, &uploaded)
	if uploaded.AssetID <= 0 {
		t.Fatalf("manual AssetID = %d, want > 0", uploaded.AssetID)
	}
	if uploaded.KnowledgeBaseID <= 0 || uploaded.KnowledgeDocumentID <= 0 {
		t.Fatalf("manual knowledge sync = %#v, want knowledge base and document IDs", uploaded)
	}
	if uploaded.RAGSyncStatus == "" {
		t.Fatalf("manual RAG sync status is empty: %#v", uploaded)
	}
	var manual models.ProductManualFile
	if err := db.First(&manual, uploaded.ID).Error; err != nil {
		t.Fatalf("load uploaded manual: %v", err)
	}
	if manual.AssetID != uploaded.AssetID || manual.KnowledgeDocumentID != uploaded.KnowledgeDocumentID {
		t.Fatalf("manual record = %#v, want independent asset and knowledge document references", manual)
	}
	var manualDocument models.KnowledgeDocument
	if err := db.First(&manualDocument, uploaded.KnowledgeDocumentID).Error; err != nil {
		t.Fatalf("load manual knowledge document: %v", err)
	}
	if manualDocument.SourceType != "product_manual" || manualDocument.SourceReferenceID != manual.ID || manualDocument.SourceAssetID != manual.AssetID {
		t.Fatalf("manual knowledge source = %#v, want product_manual source tied to manual %d and asset %d", manualDocument, manual.ID, manual.AssetID)
	}
	if manualDocument.ReviewStatus != "published" || !strings.Contains(manualDocument.Content, "DOCX manual content") {
		t.Fatalf("manual knowledge document = %#v, want published extracted content", manualDocument)
	}
	if manualDocument.PublishedRevisionID <= 0 || manualDocument.CurrentRevisionID != manualDocument.PublishedRevisionID {
		t.Fatalf("manual knowledge revision = %#v, want current published revision", manualDocument)
	}
	var manualLink models.ProductKnowledgeLink
	if err := db.First(&manualLink, manual.KnowledgeLinkID).Error; err != nil {
		t.Fatalf("load manual knowledge link: %v", err)
	}
	if manualLink.LinkType != "manual" || manualLink.PublishStatus != "published" || manualLink.KnowledgeEntryID != manualDocument.ID {
		t.Fatalf("manual knowledge link = %#v, want published manual link", manualLink)
	}

	knowledgeBody := &bytes.Buffer{}
	knowledgeWriter := multipart.NewWriter(knowledgeBody)
	knowledgePart, err := knowledgeWriter.CreateFormFile("file", "knowledge.docx")
	if err != nil {
		t.Fatalf("create knowledge form file: %v", err)
	}
	if _, err := knowledgePart.Write(buildEnterpriseTestDOCX(t, "RemoteHelpDesk vector knowledge", "Knowledge-only extracted content")); err != nil {
		t.Fatalf("write knowledge docx upload body: %v", err)
	}
	if err := knowledgeWriter.Close(); err != nil {
		t.Fatalf("close knowledge multipart writer: %v", err)
	}
	ctx, rec = newEnterpriseContractContext(
		http.MethodPost,
		"/api/enterprise/v1/products/1/knowledge-documents/_upload",
		13007,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/enterprise/v1/products/1/knowledge-documents/_upload", knowledgeBody)
	ctx.Request.Header.Set("Content-Type", knowledgeWriter.FormDataContentType())
	ctx.Set("middlewareAuthPrincipal", principal)
	ctx.Set("authPrincipal", principal)
	ProductKnowledgeDocumentUpload(ctx)

	var knowledgeUpload dto.EnterpriseProductKnowledgeDocumentDTO
	decodeEnterpriseData(t, rec, &knowledgeUpload)
	if knowledgeUpload.ID <= 0 || knowledgeUpload.SourceAssetID <= 0 || knowledgeUpload.KnowledgeLinkID <= 0 {
		t.Fatalf("knowledge upload = %#v, want document, asset and link IDs", knowledgeUpload)
	}
	if knowledgeUpload.SourceType != "uploaded_document" || !knowledgeUpload.Deletable {
		t.Fatalf("knowledge upload source = %#v, want deletable uploaded_document", knowledgeUpload)
	}
	var document models.KnowledgeDocument
	if err := db.First(&document, knowledgeUpload.ID).Error; err != nil {
		t.Fatalf("load uploaded knowledge document: %v", err)
	}
	if document.SourceAssetID != knowledgeUpload.SourceAssetID {
		t.Fatalf("document SourceAssetID = %d, want %d", document.SourceAssetID, knowledgeUpload.SourceAssetID)
	}
	if document.SourceType != "uploaded_document" || document.SourceReferenceID != document.SourceAssetID || document.ReviewStatus != "published" {
		t.Fatalf("uploaded knowledge source = %#v, want published uploaded_document", document)
	}
	if !strings.Contains(document.Content, "Knowledge-only extracted content") {
		t.Fatalf("document content = %q, want extracted knowledge content", document.Content)
	}
	if document.PublishedRevisionID <= 0 || document.CurrentRevisionID != document.PublishedRevisionID {
		t.Fatalf("uploaded knowledge revision = %#v, want current published revision", document)
	}
	var link models.ProductKnowledgeLink
	if err := db.First(&link, knowledgeUpload.KnowledgeLinkID).Error; err != nil {
		t.Fatalf("load product knowledge link: %v", err)
	}
	if link.ProductID != product.ID || link.KnowledgeEntryID != document.ID {
		t.Fatalf("knowledge link = %#v, want product %d and document %d", link, product.ID, document.ID)
	}
	if link.LinkType != "uploaded_document" || link.PublishStatus != "published" || link.Visibility != "public" {
		t.Fatalf("knowledge link = %#v, want public published uploaded_document link", link)
	}

	ctx, rec = newEnterpriseContractContext(
		http.MethodGet,
		"/api/enterprise/v1/products/1/knowledge-documents",
		13007,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ProductKnowledgeDocuments(ctx)
	var documents dto.EnterpriseListResponse[dto.EnterpriseProductKnowledgeDocumentDTO]
	decodeEnterpriseData(t, rec, &documents)
	if documents.Total != 2 || len(documents.Items) != 2 {
		t.Fatalf("product knowledge document count = %d, want manual copy and direct upload", len(documents.Items))
	}
	sources := make(map[string]dto.EnterpriseProductKnowledgeDocumentDTO, len(documents.Items))
	for _, item := range documents.Items {
		sources[item.SourceType] = item
	}
	if item, ok := sources["product_manual"]; !ok || item.Deletable || item.SourceReferenceID != manual.ID {
		t.Fatalf("manual document list item = %#v, want non-deletable product_manual source", item)
	}
	if item, ok := sources["uploaded_document"]; !ok || !item.Deletable || item.SourceReferenceID != document.SourceAssetID {
		t.Fatalf("uploaded document list item = %#v, want deletable uploaded_document source", item)
	}

	candidateScoredAt := time.Now()
	candidate := &models.KnowledgeCandidate{
		TenantID:        13007,
		ProductID:       product.ID,
		SourceType:      "ticket",
		SourceID:        "TK-13007-1",
		TicketID:        701,
		Title:           "液压设备压力异常处理",
		Suggestion:      "检查压力传感器并重新校准。",
		KnowledgeBaseID: uploaded.KnowledgeBaseID,
		QualityScore:    90,
		ValueScore:      70,
		CandidateScore:  83,
		ScoreVersion:    enums.KnowledgeCandidateScoreVersion,
		ScoredAt:        &candidateScoredAt,
		ReviewStatus:    "pending",
		Status:          enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt:      time.Now(),
			CreateUserID:   principal.UserID,
			CreateUserName: principal.Username,
			UpdatedAt:      time.Now(),
			UpdateUserID:   principal.UserID,
			UpdateUserName: principal.Username,
		},
	}
	if err := db.Create(candidate).Error; err != nil {
		t.Fatalf("create pending knowledge candidate: %v", err)
	}
	approved, err := services.KnowledgeCandidateReviewService.ApproveCandidate(
		13007,
		candidate.ID,
		dto.EnterpriseKnowledgeCandidateApproveRequest{Language: "zh-CN", Visibility: "public", Publish: true},
		principal,
	)
	if err != nil {
		t.Fatalf("approve knowledge candidate: %v", err)
	}
	if approved.ReviewStatus != "approved" || approved.KnowledgeEntryID <= 0 {
		t.Fatalf("approved candidate = %#v, want approved knowledge entry", approved)
	}
	var approvedDocument models.KnowledgeDocument
	if err := db.Where("source_type = ? AND source_reference_id = ?", "knowledge_candidate", candidate.ID).First(&approvedDocument).Error; err != nil {
		t.Fatalf("load approved knowledge entry document: %v", err)
	}
	if approvedDocument.ReviewStatus != "published" || approvedDocument.Status != enums.StatusOk {
		t.Fatalf("approved knowledge document = %#v, want published", approvedDocument)
	}
	var approvedLink models.ProductKnowledgeLink
	if err := db.Where("tenant_id = ? AND product_id = ? AND knowledge_base_id = ? AND knowledge_entry_id = ? AND status = ?",
		13007, product.ID, approvedDocument.KnowledgeBaseID, approvedDocument.ID, enums.StatusOk).
		First(&approvedLink).Error; err != nil {
		t.Fatalf("load approved product knowledge link: %v", err)
	}
	if approvedLink.Visibility != "public" || approvedLink.PublishStatus != "published" {
		t.Fatalf("approved product knowledge link = %#v, want public published link", approvedLink)
	}
	entryList, err := services.EnterpriseKnowledgeService.ListEntries(13007, services.EnterpriseKnowledgeQuery{
		Page:      1,
		PageSize:  20,
		ProductID: product.ID,
	})
	if err != nil {
		t.Fatalf("list product knowledge entries: %v", err)
	}
	if len(entryList.Items) != 1 || entryList.Items[0].Status != "published" {
		t.Fatalf("knowledge entries = %#v, want one published ticket entry", entryList.Items)
	}

	ctx, rec = newEnterpriseContractContext(
		http.MethodGet,
		"/api/enterprise/v1/products/1/knowledge-documents",
		13007,
		gin.Param{Key: "id", Value: fmt.Sprint(product.ID)},
	)
	ProductKnowledgeDocuments(ctx)
	documents = dto.EnterpriseListResponse[dto.EnterpriseProductKnowledgeDocumentDTO]{}
	decodeEnterpriseData(t, rec, &documents)
	if documents.Total != 3 || len(documents.Items) != 3 {
		t.Fatalf("product RAG document count after approval = %d, want three distinct sources", len(documents.Items))
	}
	sources = make(map[string]dto.EnterpriseProductKnowledgeDocumentDTO, len(documents.Items))
	for _, item := range documents.Items {
		sources[item.SourceType] = item
	}
	if item, ok := sources["knowledge_entry"]; !ok || item.Deletable || item.SourceReferenceID != candidate.ID || item.ReviewStatus != "published" {
		t.Fatalf("approved entry document list item = %#v, want non-deletable published knowledge_entry", item)
	}

	draftCandidate := &models.KnowledgeCandidate{
		TenantID:        13007,
		ProductID:       product.ID,
		SourceType:      "ticket",
		SourceID:        "TK-13007-2",
		TicketID:        702,
		Title:           "液压设备待复核方案",
		Suggestion:      "复核压力参数后再发布。",
		KnowledgeBaseID: uploaded.KnowledgeBaseID,
		QualityScore:    90,
		ValueScore:      70,
		CandidateScore:  83,
		ScoreVersion:    enums.KnowledgeCandidateScoreVersion,
		ScoredAt:        &candidateScoredAt,
		ReviewStatus:    "pending",
		Status:          enums.StatusOk,
		AuditFields:     utils.BuildAuditFields(principal),
	}
	if err := db.Create(draftCandidate).Error; err != nil {
		t.Fatalf("create draft knowledge candidate: %v", err)
	}
	draftApproved, err := services.KnowledgeCandidateReviewService.ApproveCandidate(
		13007,
		draftCandidate.ID,
		dto.EnterpriseKnowledgeCandidateApproveRequest{Language: "zh-CN", Publish: false},
		principal,
	)
	if err != nil {
		t.Fatalf("approve knowledge candidate as draft: %v", err)
	}
	var draftDocument models.KnowledgeDocument
	if err := db.Where("source_type = ? AND source_reference_id = ?", "knowledge_candidate", draftCandidate.ID).First(&draftDocument).Error; err != nil {
		t.Fatalf("load approved draft knowledge entry: %v", err)
	}
	if draftApproved.ReviewStatus != "approved" || draftDocument.ReviewStatus != "draft" || draftDocument.Status != enums.StatusDisabled {
		t.Fatalf("approved draft = %#v, document = %#v, want non-indexed draft", draftApproved, draftDocument)
	}
}

// TestFaultStatsProjectionIdempotentAndCompensated 覆盖 P2：
// 事件重复消费不重复累计，重新打开产生补偿。
func TestFaultStatsProjectionIdempotentAndCompensated(t *testing.T) {
	db := setupEnterpriseContractDB(t)
	product := seedEnterpriseProductContract(t, db, 13005)
	base := events.ProductFaultStatsEvent{
		EventID:   "ticket.closed:777",
		EventType: events.FaultStatsEventTicketClosed,
		TenantID:  13005,
		ProductID: product.ID,
		FaultCode: "E-42",
		Delta:     1,
	}
	for i := 0; i < 3; i++ {
		if err := services.ProductFaultStatsService.ProjectEvent(base); err != nil {
			t.Fatalf("project event attempt %d: %v", i, err)
		}
	}
	stats := services.ProductFaultStatsService.FindByProductAndDateRange(product.ID, time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 1))
	var total int64
	for _, stat := range stats {
		total += stat.TicketCount
	}
	if total != 1 {
		t.Fatalf("ticket_count = %d after duplicate events, want 1", total)
	}

	// 补偿：重新打开工单
	compensation := events.ProductFaultStatsEvent{
		EventID:   "ticket.reopened:777:1",
		EventType: events.FaultStatsEventTicketReopened,
		TenantID:  13005,
		ProductID: product.ID,
		FaultCode: "E-42",
		Delta:     -1,
	}
	if err := services.ProductFaultStatsService.ProjectEvent(compensation); err != nil {
		t.Fatalf("project compensation: %v", err)
	}
	stats = services.ProductFaultStatsService.FindByProductAndDateRange(product.ID, time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 1))
	total = 0
	for _, stat := range stats {
		total += stat.TicketCount
	}
	if total != 0 {
		t.Fatalf("ticket_count = %d after compensation, want 0", total)
	}

	// 重建任务：从工单事实表重算
	now := time.Now()
	if err := db.Create(&models.Ticket{
		TenantID:    13005,
		ProductID:   product.ID,
		TicketNo:    "TK-REBUILD",
		Title:       "Rebuild source",
		Source:      enums.TicketSourceManual,
		Status:      enums.TicketStatusClosed,
		FaultCode:   "E-99",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("create ticket: %v", err)
	}
	job := &models.ProductFaultStatsRebuildJob{
		TenantID:    13005,
		ProductID:   product.ID,
		Status:      "pending",
		RangeDays:   90,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatalf("create rebuild job: %v", err)
	}
	if err := services.ProductFaultStatsService.ExecuteRebuild(job.ID); err != nil {
		t.Fatalf("execute rebuild: %v", err)
	}
	stats = services.ProductFaultStatsService.FindByProductAndDateRange(product.ID, time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, 1))
	total = 0
	sawE99 := false
	for _, stat := range stats {
		total += stat.TicketCount
		if stat.FaultCode == "E-99" {
			sawE99 = true
		}
	}
	if total != 1 || !sawE99 {
		t.Fatalf("rebuilt stats total = %d sawE99 = %v, want 1/true", total, sawE99)
	}
}
