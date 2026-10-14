package enums

type ServiceStatus int

const (
	ServiceStatusIdle ServiceStatus = 0
	ServiceStatusBusy ServiceStatus = 1
)

var ServiceStatusValues = []ServiceStatus{
	ServiceStatusIdle,
	ServiceStatusBusy,
}

var serviceStatusLabelMap = map[ServiceStatus]string{
	ServiceStatusIdle: "enum.ServiceStatusIdle",
	ServiceStatusBusy: "enum.ServiceStatusBusy",
}

func IsValidServiceStatus(status ServiceStatus) bool {
	for _, item := range ServiceStatusValues {
		if item == status {
			return true
		}
	}
	return false
}

func GetServiceStatusLabel(status ServiceStatus, locale string) string {
	return enumLabel(locale, serviceStatusLabelMap[status])
}
