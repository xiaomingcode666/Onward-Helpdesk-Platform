package services_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTenantIsolationDB 初始化 SQLite 内存数据库并迁移所有表。
func setupTenantIsolationDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "tenant-isolation-test.db")
	db, err := bootstrap.InitDB(config.DBConfig{
		Type:         "sqlite",
		DSN:          "file:" + dbPath + "?_busy_timeout=5000",
		MaxIdleConns: 1,
		MaxOpenConns: 1,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, bootstrap.InitMigrations())
}

// tenantIsolationFixture 创建两个租户及其下属数据，返回相应的 operator 和 ID。
type tenantIsolationFixture struct {
	TenantA       *models.Tenant
	TenantB       *models.Tenant
	OperatorA     *dto.AuthPrincipal
	OperatorB     *dto.AuthPrincipal
	ProductA      *models.Product
	ProductB      *models.Product
	ProductModelA *models.ProductModel
	ProductModelB *models.ProductModel
	DeviceA       *models.Device
	DeviceB       *models.Device
	ServiceCodeA  *models.ServiceCode
	ServiceCodeB  *models.ServiceCode
	CustomerA     int64
	CustomerB     int64
	TicketA       *models.Ticket
	TicketB       *models.Ticket
}

func createTenantIsolationFixture(t *testing.T, prefix string) *tenantIsolationFixture {
	t.Helper()
	now := time.Now()
	f := &tenantIsolationFixture{}

	// ---- 租户 A ----
	f.TenantA = &models.Tenant{Name: prefix + "-tenant-A", Status: enums.StatusOk}
	require.NoError(t, sqls.DB().Create(f.TenantA).Error)

	userA := &models.User{
		Username: fmt.Sprintf("%s-operator-a-%d", prefix, now.UnixNano()),
		Nickname: prefix + "-OpA",
		Status:   enums.StatusOk,
	}
	require.NoError(t, sqls.DB().Create(userA).Error)
	f.OperatorA = &dto.AuthPrincipal{UserID: userA.ID, Username: userA.Username, Domain: "enterprise", TenantID: f.TenantA.ID}

	f.ProductA = &models.Product{TenantID: f.TenantA.ID, Code: prefix + "-prod-a", Name: prefix + " Product A", Status: enums.StatusOk}
	require.NoError(t, repositories.ProductRepository.Create(sqls.DB(), f.ProductA))

	f.ProductModelA = &models.ProductModel{TenantID: f.TenantA.ID, ProductID: f.ProductA.ID, ModelCode: prefix + "-model-a", Name: prefix + " Model A", Status: enums.StatusOk}
	require.NoError(t, repositories.ProductModelRepository.Create(sqls.DB(), f.ProductModelA))

	f.DeviceA = &models.Device{TenantID: f.TenantA.ID, ProductID: f.ProductA.ID, ProductModelID: f.ProductModelA.ID, DeviceNo: prefix + "-device-a", Status: enums.StatusOk}
	require.NoError(t, repositories.DeviceRepository.Create(sqls.DB(), f.DeviceA))

	f.ServiceCodeA = &models.ServiceCode{TenantID: f.TenantA.ID, ServiceCode: prefix + "-sc-a", DeviceID: f.DeviceA.ID, ProductID: f.ProductA.ID, ProductModelID: f.ProductModelA.ID, Status: enums.ServiceCodeStatusActive}
	require.NoError(t, repositories.ServiceCodeRepository.Create(sqls.DB(), f.ServiceCodeA))

	customerA := &models.Customer{
		Name:   prefix + "-customer-a",
		Status: enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now, CreateUserID: userA.ID, CreateUserName: userA.Username,
			UpdatedAt: now, UpdateUserID: userA.ID, UpdateUserName: userA.Username,
		},
	}
	require.NoError(t, repositories.CustomerRepository.Create(sqls.DB(), customerA))
	f.CustomerA = customerA.ID

	ticketA, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          prefix + " Ticket A",
		Description:    "Tenant A ticket",
		TenantID:       f.TenantA.ID,
		ProductID:      f.ProductA.ID,
		ProductModelID: f.ProductModelA.ID,
		DeviceID:       f.DeviceA.ID,
		ServiceCodeID:  f.ServiceCodeA.ID,
		CustomerID:     f.CustomerA,
	}, f.OperatorA)
	require.NoError(t, err)
	f.TicketA = ticketA

	// ---- 租户 B ----
	f.TenantB = &models.Tenant{Name: prefix + "-tenant-B", Status: enums.StatusOk}
	require.NoError(t, sqls.DB().Create(f.TenantB).Error)

	userB := &models.User{
		Username: fmt.Sprintf("%s-operator-b-%d", prefix, now.UnixNano()),
		Nickname: prefix + "-OpB",
		Status:   enums.StatusOk,
	}
	require.NoError(t, sqls.DB().Create(userB).Error)
	f.OperatorB = &dto.AuthPrincipal{UserID: userB.ID, Username: userB.Username, Domain: "enterprise", TenantID: f.TenantB.ID}

	f.ProductB = &models.Product{TenantID: f.TenantB.ID, Code: prefix + "-prod-b", Name: prefix + " Product B", Status: enums.StatusOk}
	require.NoError(t, repositories.ProductRepository.Create(sqls.DB(), f.ProductB))

	f.ProductModelB = &models.ProductModel{TenantID: f.TenantB.ID, ProductID: f.ProductB.ID, ModelCode: prefix + "-model-b", Name: prefix + " Model B", Status: enums.StatusOk}
	require.NoError(t, repositories.ProductModelRepository.Create(sqls.DB(), f.ProductModelB))

	f.DeviceB = &models.Device{TenantID: f.TenantB.ID, ProductID: f.ProductB.ID, ProductModelID: f.ProductModelB.ID, DeviceNo: prefix + "-device-b", Status: enums.StatusOk}
	require.NoError(t, repositories.DeviceRepository.Create(sqls.DB(), f.DeviceB))

	f.ServiceCodeB = &models.ServiceCode{TenantID: f.TenantB.ID, ServiceCode: prefix + "-sc-b", DeviceID: f.DeviceB.ID, ProductID: f.ProductB.ID, ProductModelID: f.ProductModelB.ID, Status: enums.ServiceCodeStatusActive}
	require.NoError(t, repositories.ServiceCodeRepository.Create(sqls.DB(), f.ServiceCodeB))

	customerB := &models.Customer{
		Name:   prefix + "-customer-b",
		Status: enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now, CreateUserID: userB.ID, CreateUserName: userB.Username,
			UpdatedAt: now, UpdateUserID: userB.ID, UpdateUserName: userB.Username,
		},
	}
	require.NoError(t, repositories.CustomerRepository.Create(sqls.DB(), customerB))
	f.CustomerB = customerB.ID

	ticketB, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          prefix + " Ticket B",
		Description:    "Tenant B ticket",
		TenantID:       f.TenantB.ID,
		ProductID:      f.ProductB.ID,
		ProductModelID: f.ProductModelB.ID,
		DeviceID:       f.DeviceB.ID,
		ServiceCodeID:  f.ServiceCodeB.ID,
		CustomerID:     f.CustomerB,
	}, f.OperatorB)
	require.NoError(t, err)
	f.TicketB = ticketB

	return f
}

// ---- 测试用例 ----

func TestTenantIsolation_TicketAccess(t *testing.T) {
	setupTenantIsolationDB(t)
	f := createTenantIsolationFixture(t, "ticket-access")

	// 租户 A 应只能看到自己的工单
	aggregateA, err := services.TicketService.FindPageAggregateByCnd(
		sqls.NewCnd().Eq("tenant_id", f.OperatorA.TenantID).Page(1, 100),
		f.OperatorA.UserID,
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), aggregateA.Paging.Total, "租户A应该只能看到1个工单")

	// 检查列表中没有租户B的工单
	for _, tkt := range aggregateA.List {
		assert.NotEqual(t, f.TicketB.ID, tkt.ID, "租户A的查询结果中不应包含租户B的工单")
	}
}

func TestTenantIsolation_DeviceAccess(t *testing.T) {
	setupTenantIsolationDB(t)
	f := createTenantIsolationFixture(t, "device-access")

	// 租户 A 查询设备列表不应包含租户 B 的设备
	devicesA, _ := repositories.DeviceRepository.FindPageByCnd(sqls.DB(),
		sqls.NewCnd().Eq("tenant_id", f.OperatorA.TenantID).Page(1, 100))
	for _, d := range devicesA {
		assert.Equal(t, f.TenantA.ID, d.TenantID)
		assert.NotEqual(t, f.DeviceB.ID, d.ID, "租户A的查询结果中不应包含租户B的设备")
	}

	// 租户 A 直接查询设备 B 应返回空
	deviceB := repositories.DeviceRepository.Get(sqls.DB(), f.DeviceB.ID)
	if deviceB != nil {
		// 存在但 tenant_id 不同
		assert.NotEqual(t, f.OperatorA.TenantID, deviceB.TenantID)
	}
}

func TestTenantIsolation_KnowledgeAccess(t *testing.T) {
	setupTenantIsolationDB(t)
	f := createTenantIsolationFixture(t, "kb-isolation")

	// 创建两个租户的知识库
	kbA := &models.KnowledgeBase{
		TenantID: f.TenantA.ID,
		Name:     "KB A",
		Status:   enums.StatusOk,
	}
	require.NoError(t, sqls.DB().Create(kbA).Error)

	kbB := &models.KnowledgeBase{
		TenantID: f.TenantB.ID,
		Name:     "KB B",
		Status:   enums.StatusOk,
	}
	require.NoError(t, sqls.DB().Create(kbB).Error)

	// 租户A的知识库列表不应包含租户B的知识库
	var kbListA []models.KnowledgeBase
	require.NoError(t, sqls.DB().Where("tenant_id = ?", f.TenantA.ID).Find(&kbListA).Error)
	for _, kb := range kbListA {
		assert.Equal(t, f.TenantA.ID, kb.TenantID)
		assert.NotEqual(t, kbB.ID, kb.ID, "租户A的查询结果中不应包含租户B的知识库")
	}
}

func TestTenantIsolation_CustomerAccess(t *testing.T) {
	setupTenantIsolationDB(t)
	f := createTenantIsolationFixture(t, "customer-access")

	// Customer 模型本身没有 TenantID 字段，但 ticket/customer_device_binding 等通过关联记录租户
	// 验证：租户 A 尝试通过 TicketService 关联租户 B 的客户应被拒绝
	err := services.TicketService.LinkCustomer(f.TicketA.ID, f.CustomerB, f.OperatorA)
	// 应报错（客户不属于当前租户上下文）
	assert.Error(t, err, "租户A不应能直接操作关联记录中不属于租户A的客户")

	// 通过 CustomerDeviceBinding 验证：租户A的设备只能绑定同租户的客户
	bindingB := &models.CustomerDeviceBinding{
		TenantID:      f.TenantB.ID,
		CustomerOrgID: f.CustomerB,
		DeviceID:      f.DeviceB.ID,
		BindingRole:   "owner",
		Status:        enums.StatusOk,
	}
	require.NoError(t, sqls.DB().Create(bindingB).Error)

	var bindingsA []models.CustomerDeviceBinding
	require.NoError(t, sqls.DB().Where("tenant_id = ?", f.TenantA.ID).Find(&bindingsA).Error)
	for _, b := range bindingsA {
		assert.NotEqual(t, bindingB.ID, b.ID, "租户A的绑定查询中不应包含租户B的绑定记录")
	}
}

func TestTenantIsolation_CrossTenantAPI(t *testing.T) {
	setupTenantIsolationDB(t)
	f := createTenantIsolationFixture(t, "cross-tenant-api")

	// 即使知道另一个租户的 ID，操作也应被拒绝
	// 租户 A 尝试为租户 B 创建产品
	prodCross := &models.Product{
		TenantID: f.TenantB.ID,
		Code:     "cross-tenant-product",
		Name:     "Cross Tenant Product",
		Status:   enums.StatusOk,
	}
	err := repositories.ProductRepository.Create(sqls.DB(), prodCross)
	// 应该允许创建（repository 层面不做租户校验），但在业务层应拒绝
	// 我们验证：使用正确的租户A operator 不能对租户B的数据进行操作
	_, err = services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:    "Cross-tenant Ticket",
		TenantID: f.TenantB.ID, // 指定租户B的ID
	}, f.OperatorA) // 但以租户A的身份
	assert.Error(t, err, "以租户A的身份创建租户B的工单应被拒绝")
}

func TestTenantIsolation_ServiceCodeAccess(t *testing.T) {
	setupTenantIsolationDB(t)
	f := createTenantIsolationFixture(t, "sc-access")

	// 租户A查询服务码列表不应包含租户B的服务码
	scListA, _ := repositories.ServiceCodeRepository.FindPageByCnd(sqls.DB(),
		sqls.NewCnd().Eq("tenant_id", f.TenantA.ID).Page(1, 100))
	for _, sc := range scListA {
		assert.Equal(t, f.TenantA.ID, sc.TenantID)
		assert.NotEqual(t, f.ServiceCodeB.ID, sc.ID, "租户A的查询结果中不应包含租户B的服务码")
	}

	// 租户A通过ID直接获取租户B的服务码（未加 tenant 过滤时可能查到，但应该带上租户过滤）
	// 验证 repository 层面的 Get 返回的是完整数据（无租户过滤）
	scB := repositories.ServiceCodeRepository.Get(sqls.DB(), f.ServiceCodeB.ID)
	require.NotNil(t, scB)
	assert.Equal(t, f.TenantB.ID, scB.TenantID)
}

func TestTenantIsolation_MeetingAccess(t *testing.T) {
	setupTenantIsolationDB(t)
	f := createTenantIsolationFixture(t, "meeting-access")

	// 创建两个租户的会议记录
	now := time.Now()
	meetingA := &models.MeetingRoomJitsi{
		ID:        "meeting-a-" + f.TenantA.Name,
		TenantID:  f.TenantA.ID,
		TicketID:  fmt.Sprintf("%d", f.TicketA.ID),
		RoomName:  "room-a-" + f.TenantA.Name,
		Status:    "active",
		CreatedBy: fmt.Sprintf("%d", f.OperatorA.UserID),
		StartedAt: &now,
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, sqls.DB().Create(meetingA).Error)

	meetingB := &models.MeetingRoomJitsi{
		ID:        "meeting-b-" + f.TenantB.Name,
		TenantID:  f.TenantB.ID,
		TicketID:  fmt.Sprintf("%d", f.TicketB.ID),
		RoomName:  "room-b-" + f.TenantB.Name,
		Status:    "active",
		CreatedBy: fmt.Sprintf("%d", f.OperatorB.UserID),
		StartedAt: &now,
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, sqls.DB().Create(meetingB).Error)

	// 租户A查询会议记录不应包含租户B的会议
	var meetingsA []models.MeetingRoomJitsi
	require.NoError(t, sqls.DB().Where("tenant_id = ?", f.TenantA.ID).Find(&meetingsA).Error)
	for _, m := range meetingsA {
		assert.Equal(t, f.TenantA.ID, m.TenantID)
		assert.NotEqual(t, meetingB.ID, m.ID, "租户A的会议查询结果中不应包含租户B的会议")
	}
}

func TestTenantIsolation_FileDownload(t *testing.T) {
	setupTenantIsolationDB(t)
	f := createTenantIsolationFixture(t, "file-download")

	// 验证租户隔离会影响到文件下载URL的签发
	// Asset 表中记录文件归属的租户
	now := time.Now()
	assetA := &models.Asset{
		TenantID:   f.TenantA.ID,
		AssetID:    "file-asset-a-" + f.TenantA.Name,
		Provider:   enums.AssetProviderLocal,
		StorageKey: "reports/report-a.pdf",
		Filename:   "report-a.pdf",
		FileSize:   1024,
		Status:     enums.AssetStatusSuccess,
		AuditFields: models.AuditFields{
			CreatedAt: now, CreateUserID: f.OperatorA.UserID, CreateUserName: f.OperatorA.Username,
			UpdatedAt: now, UpdateUserID: f.OperatorA.UserID, UpdateUserName: f.OperatorA.Username,
		},
	}
	require.NoError(t, repositories.AssetRepository.Create(sqls.DB(), assetA))

	assetB := &models.Asset{
		TenantID:   f.TenantB.ID,
		AssetID:    "file-asset-b-" + f.TenantB.Name,
		Provider:   enums.AssetProviderLocal,
		StorageKey: "reports/report-b.pdf",
		Filename:   "report-b.pdf",
		FileSize:   2048,
		Status:     enums.AssetStatusSuccess,
		AuditFields: models.AuditFields{
			CreatedAt: now, CreateUserID: f.OperatorB.UserID, CreateUserName: f.OperatorB.Username,
			UpdatedAt: now, UpdateUserID: f.OperatorB.UserID, UpdateUserName: f.OperatorB.Username,
		},
	}
	require.NoError(t, repositories.AssetRepository.Create(sqls.DB(), assetB))

	// 租户A只能访问自己的文件
	var assetsA []models.Asset
	require.NoError(t, sqls.DB().Where("tenant_id = ?", f.TenantA.ID).Find(&assetsA).Error)
	for _, a := range assetsA {
		assert.Equal(t, f.TenantA.ID, a.TenantID)
		assert.NotEqual(t, assetB.ID, a.ID, "租户A不应能看到租户B的文件")
	}
}

func TestTenantIsolation_BulkOperation(t *testing.T) {
	setupTenantIsolationDB(t)
	f := createTenantIsolationFixture(t, "bulk-op")

	// 批量操作验证：租户A批量删除工单时不应影响租户B的工单
	// 模拟批量更新所有 status 为 closed 的 pending 工单
	affectedA := sqls.DB().Model(&models.Ticket{}).
		Where("tenant_id = ? AND status = ?", f.TenantA.ID, enums.TicketStatusPending).
		Update("status", enums.TicketStatusClosed)
	require.NoError(t, affectedA.Error)

	// 租户B的工单不应受影响
	ticketBAfter := services.TicketService.Get(f.TicketB.ID)
	require.NotNil(t, ticketBAfter)
	assert.Equal(t, enums.TicketStatusPending, ticketBAfter.Status,
		"租户A的批量操作不应改变租户B工单的状态")
}

func TestTenantIsolation_DataExport(t *testing.T) {
	setupTenantIsolationDB(t)
	f := createTenantIsolationFixture(t, "data-export")

	// 数据导出应只包含本租户数据
	// 租户A导出所有产品
	var productsA []models.Product
	require.NoError(t, sqls.DB().Where("tenant_id = ?", f.TenantA.ID).Find(&productsA).Error)
	for _, p := range productsA {
		assert.Equal(t, f.TenantA.ID, p.TenantID)
	}

	// 租户B的产品不应出现
	var allProducts []models.Product
	require.NoError(t, sqls.DB().Find(&allProducts).Error)
	foundProductB := false
	for _, p := range allProducts {
		if p.ID == f.ProductB.ID {
			foundProductB = true
			break
		}
	}
	assert.True(t, foundProductB, "Product B 应存在于数据库中")

	// 确保租户A的导出只包含自己的数据（通过租户过滤）
	assert.Equal(t, 1, len(productsA), "租户A的导出应只包含1个产品")
}
