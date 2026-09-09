package enums

// DeviceStatus 设备生命周期状态
type DeviceStatus string

const (
	DeviceStatusNew              DeviceStatus = "new"
	DeviceStatusOperational      DeviceStatus = "operational"
	DeviceStatusUnderMaintenance DeviceStatus = "under_maintenance"
	DeviceStatusDecommissioned   DeviceStatus = "decommissioned"
	DeviceStatusArchived         DeviceStatus = "archived"
)

var validDeviceStatusTransitions = map[DeviceStatus][]DeviceStatus{
	DeviceStatusNew:              {DeviceStatusOperational},
	DeviceStatusOperational:      {DeviceStatusUnderMaintenance, DeviceStatusDecommissioned},
	DeviceStatusUnderMaintenance: {DeviceStatusOperational},
	DeviceStatusDecommissioned:   {DeviceStatusArchived, DeviceStatusOperational},
	DeviceStatusArchived:         {},
}

// IsValidDeviceStatusTransition 校验设备状态转换是否合法
func IsValidDeviceStatusTransition(from, to DeviceStatus) bool {
	allowed, ok := validDeviceStatusTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// IsDeviceStatusTerminal 判断是否为终态（不可再转换）
func IsDeviceStatusTerminal(status DeviceStatus) bool {
	return status == DeviceStatusArchived
}

// IsDeviceStatusOperable 判断设备状态是否允许创建工单等业务操作
func IsDeviceStatusOperable(status DeviceStatus) bool {
	return status == DeviceStatusOperational || status == DeviceStatusUnderMaintenance
}

type ServiceCodeMode string

const (
	ServiceCodeModeTraceable ServiceCodeMode = "traceable"
	ServiceCodeModeGeneral   ServiceCodeMode = "general"
)

type ServiceCodeStatus string

const (
	ServiceCodeStatusGenerated ServiceCodeStatus = "generated"
	ServiceCodeStatusActive    ServiceCodeStatus = "active"
	ServiceCodeStatusBound     ServiceCodeStatus = "bound"
	ServiceCodeStatusExpired   ServiceCodeStatus = "expired"
	ServiceCodeStatusRevoked   ServiceCodeStatus = "revoked"
	ServiceCodeStatusDisabled  ServiceCodeStatus = "disabled"
)

// IsUsableServiceCodeStatus reports whether the code can identify an active
// after-sales context. Binding a general code to a device must not make the
// code unusable for conversations, tickets, or repair history.
func IsUsableServiceCodeStatus(status ServiceCodeStatus) bool {
	return status == ServiceCodeStatusActive || status == ServiceCodeStatusBound
}

// IsTerminalServiceCodeStatus 判断服务码是否为终态（不可再变更）
func IsTerminalServiceCodeStatus(status ServiceCodeStatus) bool {
	return status == ServiceCodeStatusRevoked || status == ServiceCodeStatusExpired
}

// IsRevokedServiceCodeStatus 判断服务码是否为不可用状态
func IsRevokedServiceCodeStatus(status ServiceCodeStatus) bool {
	return status == ServiceCodeStatusRevoked || status == ServiceCodeStatusDisabled
}

// IsExpiredServiceCodeStatus 判断服务码是否已过期
func IsExpiredServiceCodeStatus(status ServiceCodeStatus) bool {
	return status == ServiceCodeStatusExpired
}

type CustomerEntryState string

const (
	CustomerEntryStateInvalid           CustomerEntryState = "invalid"
	CustomerEntryStateRevoked           CustomerEntryState = "revoked"
	CustomerEntryStateExpired           CustomerEntryState = "expired"
	CustomerEntryStateNeedRegister      CustomerEntryState = "needRegister"
	CustomerEntryStateGuestSessionReady CustomerEntryState = "guestSessionReady"
)
