package services

import (
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

// DeviceSoftwareVersionInfo 设备软件版本信息结构
type DeviceSoftwareVersionInfo struct {
	ComponentType string    `json:"componentType"`
	ComponentName string    `json:"componentName"`
	Version       string    `json:"version"`
	InstalledAt   time.Time `json:"installedAt"`
	Source        string    `json:"source"`
}

// DeviceVersionHistoryRecord 版本变更历史记录
type DeviceVersionHistoryRecord struct {
	ID            int64     `json:"id"`
	ComponentType string    `json:"componentType"`
	ComponentName string    `json:"componentName"`
	Version       string    `json:"version"`
	ReleaseNotes  string    `json:"releaseNotes,omitempty"`
	InstalledAt   time.Time `json:"installedAt"`
	Source        string    `json:"source"`
	InstalledBy   string    `json:"installedBy,omitempty"`
	Checksum      string    `json:"checksum,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

var DeviceSoftwareService = newDeviceSoftwareService()

func newDeviceSoftwareService() *deviceSoftwareService {
	return &deviceSoftwareService{}
}

type deviceSoftwareService struct {
}

// RecordSoftwareVersion 记录设备固件/软件版本
// operator 为 nil 时表示系统自动记录
func (s *deviceSoftwareService) RecordSoftwareVersion(tenantID, deviceID int64, req DeviceSoftwareVersionInfo, operator *dto.AuthPrincipal) (*models.DeviceSoftwareVersion, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	if deviceID <= 0 {
		return nil, errorsx.InvalidParam("deviceId is required")
	}
	if req.ComponentType == "" {
		return nil, errorsx.InvalidParam("componentType is required")
	}
	if req.ComponentName == "" {
		return nil, errorsx.InvalidParam("componentName is required")
	}
	if req.Version == "" {
		return nil, errorsx.InvalidParam("version is required")
	}

	installedAt := req.InstalledAt
	if installedAt.IsZero() {
		installedAt = time.Now()
	}

	source := req.Source
	if source == "" {
		source = "manual"
	}

	installedByType := ""
	var installedByID int64
	if operator != nil {
		installedByType = "member"
		installedByID = operator.UserID
	} else {
		installedByType = "system"
	}

	item := &models.DeviceSoftwareVersion{
		TenantID:        tenantID,
		DeviceID:        deviceID,
		ComponentType:   req.ComponentType,
		ComponentName:   req.ComponentName,
		Version:         req.Version,
		InstalledAt:     installedAt,
		Source:          source,
		InstalledByType: installedByType,
		InstalledByID:   installedByID,
		MetadataJSON:    "{}",
		CreatedAt:       time.Now(),
	}

	if err := repositories.DeviceSoftwareVersionRepository.Create(sqls.DB(), item); err != nil {
		return nil, err
	}

	// 更新设备的 last_service_at 时间戳
	now := time.Now()
	_ = repositories.DeviceRepository.Updates(sqls.DB(), deviceID, map[string]any{
		"last_service_at": &now,
		"updated_at":      now,
	})

	return item, nil
}

// GetDeviceSoftwareVersions 获取设备当前所有软件版本（每个组件取最新一条）
func (s *deviceSoftwareService) GetDeviceSoftwareVersions(tenantID, deviceID int64) ([]DeviceSoftwareVersionInfo, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	if deviceID <= 0 {
		return nil, errorsx.InvalidParam("deviceId is required")
	}

	allVersions := repositories.DeviceSoftwareVersionRepository.GetLatestByDevice(sqls.DB(), tenantID, deviceID)
	if len(allVersions) == 0 {
		return []DeviceSoftwareVersionInfo{}, nil
	}

	// 按 component_type + component_name 去重，取最新版本
	latestMap := make(map[string]*models.DeviceSoftwareVersion)
	for i := range allVersions {
		key := allVersions[i].ComponentType + ":" + allVersions[i].ComponentName
		if existing, ok := latestMap[key]; ok {
			if allVersions[i].InstalledAt.After(existing.InstalledAt) {
				latestMap[key] = &allVersions[i]
			}
		} else {
			latestMap[key] = &allVersions[i]
		}
	}

	result := make([]DeviceSoftwareVersionInfo, 0, len(latestMap))
	for _, v := range latestMap {
		result = append(result, DeviceSoftwareVersionInfo{
			ComponentType: v.ComponentType,
			ComponentName: v.ComponentName,
			Version:       v.Version,
			InstalledAt:   v.InstalledAt,
			Source:        v.Source,
		})
	}
	return result, nil
}

// GetDeviceVersionHistory 获取设备指定组件的版本变更历史
func (s *deviceSoftwareService) GetDeviceVersionHistory(tenantID, deviceID int64, componentType string) ([]DeviceVersionHistoryRecord, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenantId is required")
	}
	if deviceID <= 0 {
		return nil, errorsx.InvalidParam("deviceId is required")
	}
	if componentType == "" {
		return nil, errorsx.InvalidParam("componentType is required")
	}

	list := repositories.DeviceSoftwareVersionRepository.GetHistoryByDeviceComponent(sqls.DB(), tenantID, deviceID, componentType)
	if len(list) == 0 {
		return []DeviceVersionHistoryRecord{}, nil
	}

	result := make([]DeviceVersionHistoryRecord, 0, len(list))
	for _, v := range list {
		installedBy := ""
		if v.InstalledByType != "" {
			installedBy = v.InstalledByType
		}
		result = append(result, DeviceVersionHistoryRecord{
			ID:            v.ID,
			ComponentType: v.ComponentType,
			ComponentName: v.ComponentName,
			Version:       v.Version,
			ReleaseNotes:  v.ReleaseNotes,
			InstalledAt:   v.InstalledAt,
			Source:        v.Source,
			InstalledBy:   installedBy,
			Checksum:      v.Checksum,
			CreatedAt:     v.CreatedAt,
		})
	}
	return result, nil
}
