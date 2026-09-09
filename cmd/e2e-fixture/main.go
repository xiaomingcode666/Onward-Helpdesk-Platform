package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/pkg/utils"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

const fixturePassword = "E2EPass123!"

type fixture struct {
	tenantID                       int64
	productID                      int64
	productDeviceID                int64
	productSecondaryDeviceID       int64
	productAgentID                 int64
	productAgentReleaseID          int64
	productWorkflowID              int64
	productWorkflowVersionID       int64
	aiOnlyProductID                int64
	aiOnlyDeviceID                 int64
	aiOnlyDeviceNo                 string
	aiOnlyAgentID                  int64
	aiOnlyAgentReleaseID           int64
	aiOnlyWorkflowID               int64
	aiOnlyWorkflowVersionID        int64
	aiOnlyKnowledgeDocumentID      int64
	tenantDefaultAgentID           int64
	tenantDefaultAgentReleaseID    int64
	tenantDefaultWorkflowID        int64
	tenantDefaultWorkflowVersionID int64
	knowledgeDocumentID            int64
	serviceCode                    string
	customerUsername               string
	customerDeviceNo               string
	engineerUsername               string
	supplierUsername               string
	supplierModule                 string
	adminUsername                  string
}

func main() {
	configPath := flag.String("config", "config/config.e2e.yaml", "path to the E2E config file")
	envOut := flag.String("env-out", ".tmp/e2e.env", "dotenv output path")
	tenantName := flag.String("tenant-name", "", "existing tenant name to seed; empty creates an isolated tenant")
	adversarialTenants := flag.Bool("adversarial-tenants", true, "seed a second isolated tenant and emit E2E_T1/E2E_T2 variables")
	flag.Parse()

	if os.Getenv("RHD_E2E_ALLOW_SEED") != "1" {
		fatalf("refusing to seed without RHD_E2E_ALLOW_SEED=1")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fatalf("load config: %v", err)
	}
	config.SetCurrent(cfg)
	if root := strings.TrimSpace(cfg.Storage.Local.Root); root != "" {
		if err = os.MkdirAll(root, 0o755); err != nil {
			fatalf("create local storage root: %v", err)
		}
	}
	if _, err = bootstrap.InitDB(cfg.DB); err != nil {
		fatalf("initialize database: %v", err)
	}
	if err = ensureE2EProviderHostCanBeSeeded(sqls.DB()); err != nil {
		fatalf("validate isolated E2E database: %v", err)
	}
	if err = bootstrap.InitMigrations(); err != nil {
		fatalf("run migrations: %v", err)
	}
	if err = vectordb.Init(&cfg.VectorDB); err != nil {
		fatalf("initialize vector database: %v", err)
	}
	now := time.Now().UTC()
	seeded, err := seedFixture(now, *tenantName)
	if err != nil {
		fatalf("seed fixture: %v", err)
	}
	var adversarialTenant *fixture
	if *adversarialTenants {
		adversarialTenant, err = seedFixture(now.Add(time.Second), "")
		if err != nil {
			fatalf("seed adversarial fixture: %v", err)
		}
	}
	if err = writeEnv(*envOut, seeded, adversarialTenant); err != nil {
		fatalf("write fixture environment: %v", err)
	}
	if adversarialTenant != nil {
		fmt.Printf("P0 E2E fixture ready: tenant=%d product=%d adversarial_tenant=%d service_code=%s env=%s\n", seeded.tenantID, seeded.productID, adversarialTenant.tenantID, seeded.serviceCode, *envOut)
	} else {
		fmt.Printf("P0 E2E fixture ready: tenant=%d product=%d service_code=%s env=%s\n", seeded.tenantID, seeded.productID, seeded.serviceCode, *envOut)
	}
}

func seedFixture(now time.Time, tenantName string) (*fixture, error) {
	db := sqls.DB()
	suffix := now.Format("20060102-150405")
	bootstrapUser := repositories.UserRepository.FindOne(db, sqls.NewCnd().Eq("username", constants.BootstrapAdminUsername))
	if bootstrapUser == nil {
		return nil, fmt.Errorf("bootstrap administrator is missing")
	}
	auditOperator := &dto.AuthPrincipal{UserID: bootstrapUser.ID, Username: bootstrapUser.Username}
	tenant, err := resolveFixtureTenant(db, now, tenantName, auditOperator)
	if err != nil {
		return nil, err
	}
	auditOperator.TenantID = tenant.ID
	auditOperator.DomainType = models.DomainTypeEnterprise
	if err := services.EnsureTenantDefaultIAMRolesDB(db, tenant.ID, auditOperator); err != nil {
		return nil, err
	}
	if err := seedAIConfigs(db, tenant.ID, auditOperator); err != nil {
		return nil, err
	}
	if err := ensureE2EKnowledgeIndex(context.Background(), auditOperator); err != nil {
		return nil, err
	}

	adminUsername := "e2e.admin." + suffix
	admin, err := services.EnterpriseIAMService.InviteMember(tenant.ID, request.EnterpriseMemberInviteRequest{
		Username: adminUsername, DisplayName: "P0 Acceptance Admin", Password: fixturePassword,
		Email: adminUsername + "@example.test", JobTitle: "Service Administrator",
		RoleCodes: []string{services.EnterpriseRoleAdmin},
	}, auditOperator)
	if err != nil {
		return nil, err
	}
	operator := &dto.AuthPrincipal{
		UserID: admin.User.ID, Username: admin.User.Username, TenantID: tenant.ID,
		DomainType: models.DomainTypeEnterprise, SubjectType: models.SubjectTypeTenantMember,
		SubjectID: admin.Member.ID, MemberID: admin.Member.ID,
	}
	tenantDefaultAgent, err := services.TenantDefaultAIAgentService.EnsureDB(db, tenant.ID, operator)
	if err != nil {
		return nil, err
	}
	if tenantDefaultAgent == nil || tenantDefaultAgent.ActiveReleaseID <= 0 {
		return nil, fmt.Errorf("tenant default AI agent was not provisioned with an active release")
	}
	tenantDefaultRelease := repositories.AIAgentReleaseRepository.Get(db, tenantDefaultAgent.ActiveReleaseID)
	if tenantDefaultRelease == nil || tenantDefaultRelease.AgentID != tenantDefaultAgent.ID ||
		tenantDefaultRelease.TenantID != tenant.ID ||
		tenantDefaultRelease.Status != enums.StatusOk ||
		tenantDefaultRelease.ReviewStatus != enums.AIAgentReviewStatusApproved ||
		tenantDefaultRelease.DeploymentStatus != models.AIAgentReleaseDeploymentActive {
		return nil, fmt.Errorf("tenant default AI agent active release is invalid")
	}

	product, err := services.ProductService.CreateProduct(request.CreateProductRequest{
		TenantID: tenant.ID, ProductLine: "Industrial Power", Code: "P0-POWER-" + suffix,
		Name: "P0 Power Controller " + suffix, Description: "Deterministic P0 acceptance product",
		Category: "power-controller", OwnerMemberID: admin.Member.ID, DefaultLocale: "zh-CN",
	}, operator)
	if err != nil {
		return nil, err
	}
	productModel, err := services.ProductModelService.CreateProductModel(request.CreateProductModelRequest{
		TenantID: tenant.ID, ProductID: product.ID, ModelCode: "P0-MODEL-" + suffix,
		Name: "Power Controller 48V", VersionPolicy: "semantic", RegionScopeJSON: `["US","EU","CN"]`,
	}, operator)
	if err != nil {
		return nil, err
	}
	profile := repositories.ProductServiceProfileRepository.GetByProductID(db, product.ID)
	if profile == nil || profile.DefaultKnowledgeBaseID <= 0 {
		return nil, fmt.Errorf("product knowledge base was not provisioned")
	}
	knowledgeEntry, err := services.EnterpriseKnowledgeService.CreateEntry(tenant.ID, dto.EnterpriseKnowledgeMutationRequest{
		KnowledgeBaseID: profile.DefaultKnowledgeBaseID,
		Title:           "P0 controller reset and indicator guidance",
		Content: "故障码 RHD-FLOW-ALPHA-7742 的标准排查：先断开设备主电源并等待 30 秒，确认储能释放后检查保护输入端。" +
			"正常状态灯应为绿色慢闪；红灯常亮时不要继续上电，应保持断电并转交技术工程师。",
		Category:          "troubleshooting",
		Type:              "document",
		Visibility:        "public",
		Tags:              []string{"reset", "indicator", "safety"},
		RelatedProductIDs: []int64{product.ID},
		FaultCodes:        []string{"RHD-FLOW-ALPHA-7742"},
		Language:          "zh-CN",
	}, operator)
	if err != nil {
		return nil, err
	}
	if knowledgeEntry.ID%10 != 1 {
		return nil, fmt.Errorf("seeded knowledge entry is not a document")
	}
	if _, err = services.EnterpriseKnowledgeService.UpdateEntryStatus(tenant.ID, knowledgeEntry.ID, "review", operator); err != nil {
		return nil, err
	}
	if _, err = services.EnterpriseKnowledgeService.UpdateEntryStatus(tenant.ID, knowledgeEntry.ID, "published", operator); err != nil {
		return nil, err
	}
	knowledgeDocumentID := knowledgeEntry.ID / 10
	productAgent, err := services.ProductAIAgentService.EnsureProductCustomerAgent(tenant.ID, product.ID, operator)
	if err != nil {
		return nil, err
	}
	productRelease, err := deployAgentDraft(productAgent.ID, "P0 collaboration acceptance fixture", operator)
	if err != nil {
		return nil, err
	}
	productAgent = repositories.AIAgentRepository.Get(db, productAgent.ID)
	if productAgent == nil || productAgent.ActiveReleaseID != productRelease.ID {
		return nil, fmt.Errorf("collaboration product AI agent release was not activated")
	}

	aiOnlyProduct, err := services.ProductService.CreateProduct(request.CreateProductRequest{
		TenantID: tenant.ID, ProductLine: "Industrial Diagnostics", Code: "P0-DIAG-" + suffix,
		Name: "P0 Diagnostic Unit " + suffix, Description: "Deterministic AI-only routing product",
		Category: "diagnostic-unit", OwnerMemberID: admin.Member.ID, DefaultLocale: "zh-CN",
	}, operator)
	if err != nil {
		return nil, err
	}
	aiOnlyProductModel, err := services.ProductModelService.CreateProductModel(request.CreateProductModelRequest{
		TenantID: tenant.ID, ProductID: aiOnlyProduct.ID, ModelCode: "P0-DIAG-MODEL-" + suffix,
		Name: "Diagnostic Unit X1", VersionPolicy: "semantic", RegionScopeJSON: `["US","EU","CN"]`,
	}, operator)
	if err != nil {
		return nil, err
	}
	aiOnlyProfile := repositories.ProductServiceProfileRepository.GetByProductID(db, aiOnlyProduct.ID)
	if aiOnlyProfile == nil || aiOnlyProfile.DefaultKnowledgeBaseID <= 0 {
		return nil, fmt.Errorf("AI-only product knowledge base was not provisioned")
	}
	aiOnlyKnowledgeEntry, err := services.EnterpriseKnowledgeService.CreateEntry(tenant.ID, dto.EnterpriseKnowledgeMutationRequest{
		KnowledgeBaseID: aiOnlyProfile.DefaultKnowledgeBaseID,
		Title:           "P0 diagnostic unit pressure reset guidance",
		Content: "故障码 RHD-AI-ONLY-BETA-8841 表示压力采样需要安全复位。先关闭执行机构并等待 20 秒，" +
			"确认压力归零后检查传感器接头；若读数仍漂移，应保持停机并记录读数，不得带压拆卸。",
		Category:          "troubleshooting",
		Type:              "document",
		Visibility:        "public",
		Tags:              []string{"pressure", "reset", "safety"},
		RelatedProductIDs: []int64{aiOnlyProduct.ID},
		FaultCodes:        []string{"RHD-AI-ONLY-BETA-8841"},
		Language:          "zh-CN",
	}, operator)
	if err != nil {
		return nil, err
	}
	if aiOnlyKnowledgeEntry.ID%10 != 1 {
		return nil, fmt.Errorf("seeded AI-only knowledge entry is not a document")
	}
	if _, err = services.EnterpriseKnowledgeService.UpdateEntryStatus(tenant.ID, aiOnlyKnowledgeEntry.ID, "review", operator); err != nil {
		return nil, err
	}
	if _, err = services.EnterpriseKnowledgeService.UpdateEntryStatus(tenant.ID, aiOnlyKnowledgeEntry.ID, "published", operator); err != nil {
		return nil, err
	}
	aiOnlyKnowledgeDocumentID := aiOnlyKnowledgeEntry.ID / 10
	aiOnlyAgent, err := services.ProductAIAgentService.EnsureProductCustomerAgent(tenant.ID, aiOnlyProduct.ID, operator)
	if err != nil {
		return nil, err
	}
	_, aiOnlyWorkflowVersion, err := services.AIWorkflowService.EnsurePlatformDeviceAIOnlyWorkflowDB(db)
	if err != nil {
		return nil, err
	}
	if aiOnlyWorkflowVersion == nil {
		return nil, fmt.Errorf("AI-only workflow stable version was not provisioned")
	}
	if err = services.AIWorkflowService.BindAgentWorkflowVersion(aiOnlyAgent.ID, aiOnlyWorkflowVersion.ID, operator); err != nil {
		return nil, err
	}
	aiOnlyRelease, err := deployAgentDraft(aiOnlyAgent.ID, "P0 AI-only acceptance fixture", operator)
	if err != nil {
		return nil, err
	}
	aiOnlyAgent = repositories.AIAgentRepository.Get(db, aiOnlyAgent.ID)
	if aiOnlyAgent == nil || aiOnlyAgent.ActiveReleaseID != aiOnlyRelease.ID {
		return nil, fmt.Errorf("AI-only product agent release was not activated")
	}

	engineerUsername := "e2e.engineer." + suffix
	engineer, err := services.EnterpriseIAMService.InviteMember(tenant.ID, request.EnterpriseMemberInviteRequest{
		Username: engineerUsername, DisplayName: "P0 Field Engineer", Password: fixturePassword,
		Email: engineerUsername + "@example.test", JobTitle: "Field Service Engineer",
		RoleCodes: []string{services.EnterpriseRoleEngineer}, DispatchEnabled: true,
	}, operator)
	if err != nil {
		return nil, err
	}
	productTeam := services.ProductSupportOrganizationService.FindProductRepairTeam(db, tenant.ID, product.ID)
	tenantTeam, err := services.ProductSupportOrganizationService.EnsureTenantTechnicalRepairTeamDB(db, tenant.ID, operator)
	if err != nil {
		return nil, err
	}
	if productTeam == nil || tenantTeam == nil || engineer.Engineer == nil {
		return nil, fmt.Errorf("repair teams or engineer profile were not provisioned")
	}
	for _, teamID := range []int64{productTeam.ID, tenantTeam.ID} {
		if _, err = services.AgentTeamMemberService.EnsureMemberDB(
			db, tenant.ID, teamID, engineer.User.ID, engineer.Member.ID, 10, true, operator,
		); err != nil {
			return nil, err
		}
	}
	agentProfile := repositories.AgentProfileRepository.FindOne(db, sqls.NewCnd().Eq("user_id", engineer.User.ID))
	if agentProfile == nil {
		return nil, fmt.Errorf("dispatch profile was not provisioned")
	}
	if err = repositories.AgentProfileRepository.Updates(db, agentProfile.ID, map[string]any{
		"team_id": productTeam.ID, "service_status": enums.ServiceStatusIdle,
		"auto_assign_enabled": true, "max_concurrent_count": 5, "updated_at": now,
	}); err != nil {
		return nil, err
	}
	engineerOperator := &dto.AuthPrincipal{
		UserID:     engineer.User.ID,
		Username:   engineerUsername,
		TenantID:   tenant.ID,
		DomainType: models.DomainTypeEnterprise,
		Domain:     models.DomainTypeEnterprise,
	}
	if _, err = services.AgentWorkStatusService.UpdateMyStatus(request.UpdateAgentWorkStatusRequest{
		Status: services.AgentWorkStatusAvailable,
	}, engineerOperator, now); err != nil {
		return nil, err
	}
	if err = createAlwaysOnSchedule(productTeam.ID, engineer.User.ID, operator); err != nil {
		return nil, err
	}

	customerUsername := "e2e.customer." + suffix
	customer, err := services.EnterpriseIAMService.AuthorizeCustomerUser(tenant.ID, request.EnterpriseCustomerUserAuthorizeRequest{
		Username: customerUsername, DisplayName: "P0 Site Operator", Password: fixturePassword,
		Email: customerUsername + "@example.test", CustomerOrg: "P0 Customer " + suffix,
		RoleCodes: []string{services.CustomerRoleUser}, Locale: "zh-CN", Timezone: "UTC",
	}, operator)
	if err != nil {
		return nil, err
	}
	deviceNo := "P0-DEVICE-" + suffix
	device, err := services.DeviceService.CreateDevice(request.CreateDeviceRequest{
		TenantID: tenant.ID, DeviceNo: deviceNo, ProductID: product.ID, ProductModelID: productModel.ID,
		SerialNo: "SN-" + suffix, CustomerOrgID: customer.CustomerOrg.ID,
		InstallLocationJSON: `{"site":"P0 lab"}`, RegionCode: "US", Source: "e2e", MetadataJSON: `{}`,
	}, operator)
	if err != nil {
		return nil, err
	}
	confirmedAt := now
	if err = db.Create(&models.CustomerDeviceBinding{
		TenantID: tenant.ID, CustomerOrgID: customer.CustomerOrg.ID, CustomerUserID: customer.CustomerUser.ID,
		DeviceID: device.ID, BindingRole: "owner", PermissionFlagsJSON: `{}`, Source: "e2e",
		Status: enums.StatusOk, ConfirmedAt: &confirmedAt, AuditFields: utils.BuildAuditFields(operator),
	}).Error; err != nil {
		return nil, err
	}
	secondaryDevice, err := services.DeviceService.CreateDevice(request.CreateDeviceRequest{
		TenantID: tenant.ID, DeviceNo: "P0-DEVICE-SECONDARY-" + suffix,
		ProductID: product.ID, ProductModelID: productModel.ID, SerialNo: "SN-SECONDARY-" + suffix,
		CustomerOrgID: customer.CustomerOrg.ID, InstallLocationJSON: `{"site":"P0 secondary lab"}`,
		RegionCode: "US", Source: "e2e", MetadataJSON: `{}`,
	}, operator)
	if err != nil {
		return nil, err
	}
	if err = db.Create(&models.CustomerDeviceBinding{
		TenantID: tenant.ID, CustomerOrgID: customer.CustomerOrg.ID, CustomerUserID: customer.CustomerUser.ID,
		DeviceID: secondaryDevice.ID, BindingRole: "owner", PermissionFlagsJSON: `{}`, Source: "e2e",
		Status: enums.StatusOk, ConfirmedAt: &confirmedAt, AuditFields: utils.BuildAuditFields(operator),
	}).Error; err != nil {
		return nil, err
	}
	aiOnlyDeviceNo := "P0-AI-DEVICE-" + suffix
	aiOnlyDevice, err := services.DeviceService.CreateDevice(request.CreateDeviceRequest{
		TenantID: tenant.ID, DeviceNo: aiOnlyDeviceNo, ProductID: aiOnlyProduct.ID, ProductModelID: aiOnlyProductModel.ID,
		SerialNo: "AI-SN-" + suffix, CustomerOrgID: customer.CustomerOrg.ID,
		InstallLocationJSON: `{"site":"P0 AI-only lab"}`, RegionCode: "US", Source: "e2e", MetadataJSON: `{}`,
	}, operator)
	if err != nil {
		return nil, err
	}
	if err = db.Create(&models.CustomerDeviceBinding{
		TenantID: tenant.ID, CustomerOrgID: customer.CustomerOrg.ID, CustomerUserID: customer.CustomerUser.ID,
		DeviceID: aiOnlyDevice.ID, BindingRole: "owner", PermissionFlagsJSON: `{}`, Source: "e2e",
		Status: enums.StatusOk, ConfirmedAt: &confirmedAt, AuditFields: utils.BuildAuditFields(operator),
	}).Error; err != nil {
		return nil, err
	}

	partnerUsername := "e2e.supplier." + suffix
	partner, err := services.EnterpriseIAMService.InvitePartnerAdmin(tenant.ID, request.EnterprisePartnerAdminInviteRequest{
		PartnerNo: "P0-SUPPLIER-" + suffix, PartnerName: "P0 Power Supplier " + suffix,
		PartnerType: "module_supplier", CountryRegion: "US", ContactName: "P0 Supplier Engineer",
		Username: partnerUsername, DisplayName: "P0 Supplier Engineer", Password: fixturePassword,
		Email: partnerUsername + "@example.test",
	}, operator)
	if err != nil {
		return nil, err
	}
	moduleName := "P0-Power-Module-" + suffix
	module, err := services.ProductModuleService.CreateModule(services.CreateProductModuleRequest{
		TenantID: tenant.ID, ProductID: product.ID, ModuleCode: "PWR-" + suffix, Name: moduleName,
		DefaultSupplierID: partner.PartnerCompany.ID, IsSafetyCritical: true,
	}, operator)
	if err != nil {
		return nil, err
	}
	if _, err = services.ProductModuleService.CreateModelLink(module.ID, productModel.ID, tenant.ID, operator); err != nil {
		return nil, err
	}

	batch := &models.ServiceCodeBatch{
		TenantID: tenant.ID, BatchNo: "P0-BATCH-" + suffix, Mode: enums.ServiceCodeModeTraceable,
		ProductID: product.ID, ProductModelID: productModel.ID, Quantity: 1,
		GeneratedCount: 1, Status: enums.StatusOk, AuditFields: utils.BuildAuditFields(operator),
	}
	if err = db.Create(batch).Error; err != nil {
		return nil, err
	}
	serviceCode := "P0-SERVICE-" + strings.ReplaceAll(suffix, "-", "")
	if err = db.Create(&models.ServiceCode{
		TenantID: tenant.ID, BatchID: batch.ID, ServiceCode: serviceCode,
		Mode: enums.ServiceCodeModeTraceable, DeviceID: device.ID, ProductID: product.ID,
		ProductModelID: productModel.ID, Status: enums.ServiceCodeStatusActive,
		ActivatedAt: &confirmedAt, BoundAt: &confirmedAt, AuditFields: utils.BuildAuditFields(operator),
	}).Error; err != nil {
		return nil, err
	}

	return &fixture{
		tenantID:  tenant.ID,
		productID: product.ID, productDeviceID: device.ID, productSecondaryDeviceID: secondaryDevice.ID,
		productAgentID:        productAgent.ID,
		productAgentReleaseID: productRelease.ID, productWorkflowID: productRelease.WorkflowID,
		productWorkflowVersionID: productRelease.WorkflowVersionID,
		aiOnlyProductID:          aiOnlyProduct.ID, aiOnlyDeviceID: aiOnlyDevice.ID, aiOnlyDeviceNo: aiOnlyDeviceNo,
		aiOnlyAgentID: aiOnlyAgent.ID, aiOnlyAgentReleaseID: aiOnlyRelease.ID,
		aiOnlyWorkflowID: aiOnlyRelease.WorkflowID, aiOnlyWorkflowVersionID: aiOnlyRelease.WorkflowVersionID,
		aiOnlyKnowledgeDocumentID: aiOnlyKnowledgeDocumentID,
		tenantDefaultAgentID:      tenantDefaultAgent.ID, tenantDefaultAgentReleaseID: tenantDefaultRelease.ID,
		tenantDefaultWorkflowID:        tenantDefaultRelease.WorkflowID,
		tenantDefaultWorkflowVersionID: tenantDefaultRelease.WorkflowVersionID,
		knowledgeDocumentID:            knowledgeDocumentID, serviceCode: serviceCode,
		customerUsername: customerUsername, customerDeviceNo: deviceNo,
		engineerUsername: engineerUsername, supplierUsername: partnerUsername,
		supplierModule: moduleName, adminUsername: adminUsername,
	}, nil
}

func resolveFixtureTenant(db *gorm.DB, now time.Time, tenantName string, operator *dto.AuthPrincipal) (*models.Tenant, error) {
	tenantName = strings.TrimSpace(tenantName)
	if tenantName != "" {
		items, _, err := repositories.PlatformIAMRepository.FindTenantPage(
			db,
			repositories.PlatformTenantFilter{Search: tenantName, Lifecycle: "active"},
			1,
			200,
		)
		if err != nil {
			return nil, err
		}
		var matched *models.Tenant
		for index := range items {
			if items[index].Name != tenantName {
				continue
			}
			if matched != nil {
				return nil, fmt.Errorf("multiple active tenants are named %q", tenantName)
			}
			item := items[index]
			matched = &item
		}
		if matched == nil {
			return nil, fmt.Errorf("active tenant %q was not found", tenantName)
		}
		return matched, nil
	}

	tenant := &models.Tenant{
		Name: "P0 Acceptance " + now.Format("20060102-150405"), Industry: "industrial-equipment", CountryRegion: "US",
		DefaultLocale: "zh-CN", Timezone: "UTC", DataRegion: "us-east", Status: enums.StatusOk,
		AuditFields: utils.BuildAuditFields(operator),
	}
	if err := repositories.PlatformIAMRepository.CreateTenant(db, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}

func deployAgentDraft(agentID int64, reviewComment string, operator *dto.AuthPrincipal) (*models.AIAgentRelease, error) {
	release, err := services.AIAgentReleaseService.CreateCandidate(agentID, operator)
	if err != nil {
		return nil, err
	}
	if err = services.AIAgentReleaseService.SubmitReview(release.ID, operator); err != nil {
		return nil, err
	}
	if err = services.AIAgentReleaseService.Review(release.ID, true, reviewComment, operator); err != nil {
		return nil, err
	}
	if err = services.AIAgentReleaseService.Deploy(release.ID, operator); err != nil {
		return nil, err
	}
	return release, nil
}

func createAlwaysOnSchedule(teamID, userID int64, operator *dto.AuthPrincipal) error {
	for weekday := 1; weekday <= 7; weekday++ {
		if _, err := services.AgentTeamScheduleService.CreateAgentTeamSchedule(request.CreateAgentTeamScheduleRequest{
			TeamID: teamID, UserID: userID, RepeatType: "weekly",
			Weekday: weekday, StartTime: "00:00", EndTime: "23:59",
			Remark: "P0 acceptance always-on shift",
		}, operator); err != nil {
			return err
		}
	}
	if _, err := services.AgentTeamScheduleService.PublishDraft(teamID, true, operator); err != nil {
		return err
	}
	return nil
}

func seedAIConfigs(db *gorm.DB, tenantID int64, operator *dto.AuthPrincipal) error {
	baseURL := "http://127.0.0.1:18099/v1"
	apiKey := "e2e-api-key"
	if err := ensureE2EProviderHostCanBeSeeded(db); err != nil {
		return err
	}
	encryptedAPIKey, err := secretstore.Encrypt(apiKey)
	if err != nil {
		return err
	}
	if err = repositories.SystemConfigRepository.SaveByKey(db, &models.SystemConfig{
		ConfigKey: "platform.sub2api.host", ConfigValue: baseURL,
		GroupCode: "e2e", Title: "P0 deterministic Sub2API host", Status: enums.StatusOk,
		AuditFields: utils.BuildAuditFields(operator),
	}); err != nil {
		return err
	}
	accountValues := map[string]any{
		"account_name":            "P0 deterministic tenant account",
		"account_status":          "active",
		"provision_status":        "active",
		"default_key_id":          fmt.Sprintf("e2e-key-%d", tenantID),
		"default_key_name":        "P0 deterministic tenant key",
		"default_key_ciphertext":  encryptedAPIKey,
		"default_key_fingerprint": secretstore.Fingerprint(apiKey),
		"default_key_status":      "active",
		"default_llm_model":       "e2e-llm",
		"status":                  enums.StatusOk,
		"update_user_id":          operator.UserID,
		"update_user_name":        operator.Username,
		"updated_at":              time.Now(),
	}
	if existing := repositories.PlatformIAMRepository.FindSub2APIAccountByTenantID(db, tenantID); existing != nil {
		if err = repositories.PlatformIAMRepository.UpdateSub2APIAccount(db, existing.ID, accountValues); err != nil {
			return err
		}
	} else {
		account := &models.Sub2APITenantAccount{
			TenantID: tenantID, Sub2APIAccountID: fmt.Sprintf("e2e-tenant-%d", tenantID),
			AccountName: "P0 deterministic tenant account", AccountStatus: "active",
			ProvisionStatus: "active", DefaultKeyID: fmt.Sprintf("e2e-key-%d", tenantID),
			DefaultKeyName: "P0 deterministic tenant key", DefaultKeyCiphertext: encryptedAPIKey,
			DefaultKeyFingerprint: secretstore.Fingerprint(apiKey), DefaultKeyStatus: "active",
			DefaultLLMModel: "e2e-llm", Status: enums.StatusOk,
			AuditFields: utils.BuildAuditFields(operator),
		}
		if err = repositories.PlatformIAMRepository.CreateSub2APIAccount(db, account); err != nil {
			return err
		}
	}
	items := []models.AIConfig{
		{
			Name: "P0 deterministic LLM", Provider: enums.AIProviderOpenAI,
			BaseURL: baseURL, APIKey: apiKey, ModelType: enums.AIModelTypeLLM,
			ModelName: "e2e-llm", MaxContextTokens: 16_000, MaxOutputTokens: 1_000,
			TimeoutMS: 10_000, MaxRetryCount: 0, Status: enums.StatusOk, SortNo: 10_000,
			Remark: "Deterministic P0 acceptance fixture", AuditFields: utils.BuildAuditFields(operator),
		},
		{
			Name: "P0 deterministic embedding", Provider: enums.AIProviderOpenAI,
			BaseURL: baseURL, APIKey: apiKey, ModelType: enums.AIModelTypeEmbedding,
			ModelName: "e2e-embedding", Dimension: 8, TimeoutMS: 10_000,
			MaxRetryCount: 0, Status: enums.StatusOk, SortNo: 10_000,
			Remark: "Deterministic P0 acceptance fixture", AuditFields: utils.BuildAuditFields(operator),
		},
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for index := range items {
			item := &items[index]
			existing := repositories.AIConfigRepository.FindOne(tx, sqls.NewCnd().
				Eq("name", item.Name).
				Eq("model_type", item.ModelType).
				NotEq("status", enums.StatusDeleted).
				Desc("id"))
			if err := tx.Model(&models.AIConfig{}).
				Where("model_type = ? AND status = ?", item.ModelType, enums.StatusOk).
				Updates(map[string]any{"status": enums.StatusDisabled, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
			if existing == nil {
				if err := repositories.AIConfigRepository.Create(tx, item); err != nil {
					return err
				}
				continue
			}
			if err := repositories.AIConfigRepository.Updates(tx, existing.ID, map[string]any{
				"provider": item.Provider, "base_url": item.BaseURL, "api_key": item.APIKey,
				"model_name": item.ModelName, "dimension": item.Dimension,
				"max_context_tokens": item.MaxContextTokens, "max_output_tokens": item.MaxOutputTokens,
				"timeout_ms": item.TimeoutMS, "max_retry_count": item.MaxRetryCount,
				"status": enums.StatusOk, "sort_no": item.SortNo, "remark": item.Remark,
				"update_user_id": operator.UserID, "update_user_name": operator.Username, "updated_at": time.Now(),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func ensureE2EProviderHostCanBeSeeded(db *gorm.DB) error {
	existingHost := repositories.SystemConfigRepository.FindByKey(db, "platform.sub2api.host")
	if existingHost != nil && !strings.EqualFold(strings.TrimSpace(existingHost.GroupCode), "e2e") {
		return fmt.Errorf("refusing to overwrite non-E2E platform.sub2api.host configuration; use an isolated E2E database")
	}
	return nil
}

func ensureE2EKnowledgeIndex(ctx context.Context, operator *dto.AuthPrincipal) error {
	generation, err := services.KnowledgeIndexGenerationService.EnsureActiveGeneration(ctx, operator)
	if err != nil {
		return fmt.Errorf("activate deterministic E2E knowledge index generation: %w", err)
	}
	if generation == nil || generation.EmbeddingModel != "e2e-embedding" || generation.Dimension != 8 {
		return fmt.Errorf("deterministic E2E knowledge index generation is invalid")
	}
	if err := services.KnowledgeIndexGenerationService.EnsureActiveCollectionSchema(ctx); err != nil {
		return fmt.Errorf("initialize deterministic E2E knowledge index schema: %w", err)
	}
	return nil
}

func writeEnv(path string, value *fixture, adversarialTenant *fixture) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lines := []string{
		fmt.Sprintf("E2E_TENANT_ID=%d", value.tenantID),
		"E2E_PLATFORM_USERNAME=" + constants.BootstrapAdminUsername,
		"E2E_PLATFORM_PASSWORD=" + fixturePassword,
		fmt.Sprintf("E2E_PRODUCT_ID=%d", value.productID),
		fmt.Sprintf("E2E_AGENT_ID=%d", value.productAgentID),
		fmt.Sprintf("E2E_WORKFLOW_ID=%d", value.productWorkflowID),
		fmt.Sprintf("E2E_COLLAB_PRODUCT_ID=%d", value.productID),
		fmt.Sprintf("E2E_COLLAB_DEVICE_ID=%d", value.productDeviceID),
		fmt.Sprintf("E2E_COLLAB_SECONDARY_DEVICE_ID=%d", value.productSecondaryDeviceID),
		fmt.Sprintf("E2E_COLLAB_AGENT_ID=%d", value.productAgentID),
		fmt.Sprintf("E2E_COLLAB_AGENT_RELEASE_ID=%d", value.productAgentReleaseID),
		fmt.Sprintf("E2E_COLLAB_WORKFLOW_ID=%d", value.productWorkflowID),
		fmt.Sprintf("E2E_COLLAB_WORKFLOW_VERSION_ID=%d", value.productWorkflowVersionID),
		fmt.Sprintf("E2E_AI_ONLY_PRODUCT_ID=%d", value.aiOnlyProductID),
		fmt.Sprintf("E2E_AI_ONLY_DEVICE_ID=%d", value.aiOnlyDeviceID),
		"E2E_AI_ONLY_DEVICE_NO=" + value.aiOnlyDeviceNo,
		fmt.Sprintf("E2E_AI_ONLY_AGENT_ID=%d", value.aiOnlyAgentID),
		fmt.Sprintf("E2E_AI_ONLY_AGENT_RELEASE_ID=%d", value.aiOnlyAgentReleaseID),
		fmt.Sprintf("E2E_AI_ONLY_WORKFLOW_ID=%d", value.aiOnlyWorkflowID),
		fmt.Sprintf("E2E_AI_ONLY_WORKFLOW_VERSION_ID=%d", value.aiOnlyWorkflowVersionID),
		fmt.Sprintf("E2E_AI_ONLY_KNOWLEDGE_DOCUMENT_ID=%d", value.aiOnlyKnowledgeDocumentID),
		fmt.Sprintf("E2E_TENANT_DEFAULT_AGENT_ID=%d", value.tenantDefaultAgentID),
		fmt.Sprintf("E2E_TENANT_DEFAULT_AGENT_RELEASE_ID=%d", value.tenantDefaultAgentReleaseID),
		fmt.Sprintf("E2E_TENANT_DEFAULT_WORKFLOW_ID=%d", value.tenantDefaultWorkflowID),
		fmt.Sprintf("E2E_TENANT_DEFAULT_WORKFLOW_VERSION_ID=%d", value.tenantDefaultWorkflowVersionID),
		fmt.Sprintf("E2E_SEED_KNOWLEDGE_DOCUMENT_ID=%d", value.knowledgeDocumentID),
		"E2E_SERVICE_CODE=" + value.serviceCode,
		"E2E_CUSTOMER_USERNAME=" + value.customerUsername,
		"E2E_CUSTOMER_PASSWORD=" + fixturePassword,
		"E2E_CUSTOMER_DEVICE_NO=" + value.customerDeviceNo,
		"E2E_ENGINEER_USERNAME=" + value.engineerUsername,
		"E2E_ENGINEER_PASSWORD=" + fixturePassword,
		"E2E_SUPPLIER_USERNAME=" + value.supplierUsername,
		"E2E_SUPPLIER_PASSWORD=" + fixturePassword,
		"E2E_SUPPLIER_MODULE=" + value.supplierModule,
		"E2E_ADMIN_USERNAME=" + value.adminUsername,
		"E2E_ADMIN_PASSWORD=" + fixturePassword,
		"E2E_FAULT_CODE=RHD-FLOW-ALPHA-7742",
		"E2E_EXPECTED_RESET_SECONDS=30",
	}
	if adversarialTenant != nil {
		lines = appendFixtureEnvAliases(lines, "E2E_T1", value)
		lines = appendFixtureEnvAliases(lines, "E2E_T2", adversarialTenant)
	}
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o600)
}

func appendFixtureEnvAliases(lines []string, prefix string, value *fixture) []string {
	if value == nil {
		return lines
	}
	return append(lines,
		fmt.Sprintf("%s_TENANT_ID=%d", prefix, value.tenantID),
		fmt.Sprintf("%s_PRODUCT_ID=%d", prefix, value.productID),
		prefix+"_CUSTOMER_USERNAME="+value.customerUsername,
		prefix+"_CUSTOMER_PASSWORD="+fixturePassword,
		prefix+"_CUSTOMER_DEVICE_NO="+value.customerDeviceNo,
		prefix+"_ADMIN_USERNAME="+value.adminUsername,
		prefix+"_ADMIN_PASSWORD="+fixturePassword,
		prefix+"_ENGINEER_USERNAME="+value.engineerUsername,
		prefix+"_ENGINEER_PASSWORD="+fixturePassword,
		prefix+"_SUPPLIER_USERNAME="+value.supplierUsername,
		prefix+"_SUPPLIER_PASSWORD="+fixturePassword,
		prefix+"_SUPPLIER_MODULE="+value.supplierModule,
	)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
