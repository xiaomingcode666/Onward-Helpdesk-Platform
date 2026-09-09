package services

import "remotehelpdesk/internal/models"

// ConnectorTemplate 连接器模板定义
type ConnectorTemplate struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	ConnectorType  string `json:"connectorType"`
	AuthType       string `json:"authType"`
	BaseURL        string `json:"baseUrl,omitempty"`
	TemplateConfig string `json:"templateConfig,omitempty"`
}

// GetConnectorTemplates 返回所有预置连接器模板
func GetConnectorTemplates() []*ConnectorTemplate {
	return []*ConnectorTemplate{
		erpSapB1Template(),
		erpOdooTemplate(),
		crmSalesforceTemplate(),
		crmHubspotTemplate(),
		iotMQTTGenericTemplate(),
		deviceLogHTTPTemplate(),
	}
}

// GetConnectorTemplateByCode 根据模板代码获取模板
func GetConnectorTemplateByCode(code string) *ConnectorTemplate {
	for _, t := range GetConnectorTemplates() {
		if t.Code == code {
			return t
		}
	}
	return nil
}

// erpSapB1Template SAP Business One OpenAPI 模板
func erpSapB1Template() *ConnectorTemplate {
	return &ConnectorTemplate{
		Code:          "erp_sap_b1",
		Name:          "SAP Business One",
		Description:   "SAP Business One Service Layer REST API 连接器，支持客户、设备、备件和质保数据查询",
		ConnectorType: "openapi",
		AuthType:      "basic",
		BaseURL:       "https://[tenant]:50000/b1s/v1",
		TemplateConfig: `{
			"apiVersion": "v1",
			"endpoints": {
				"customers": "/BusinessPartners",
				"devices": "/Items",
				"warranties": "/Contracts",
				"spareParts": "/Stock"
			},
			"auth": {
				"type": "basic",
				"companyDB": "SBODemoUS"
			},
			"fieldMappings": [
				{"external": "CardCode", "standard": "customer.externalId"},
				{"external": "CardName", "standard": "customer.name"},
				{"external": "ItemCode", "standard": "device.externalId"},
				{"external": "ItemName", "standard": "device.name"}
			]
		}`,
	}
}

// erpOdooTemplate Odoo JSON-RPC 模板
func erpOdooTemplate() *ConnectorTemplate {
	return &ConnectorTemplate{
		Code:          "erp_odoo",
		Name:          "Odoo ERP",
		Description:   "Odoo ERP JSON-RPC API 连接器，支持客户、产品和销售订单数据查询",
		ConnectorType: "openapi",
		AuthType:      "api_key",
		BaseURL:       "https://[tenant].odoo.com/api/v1",
		TemplateConfig: `{
			"apiVersion": "v1",
			"endpoints": {
				"customers": "/res.partner/search_read",
				"devices": "/product.product/search_read",
				"serviceHistory": "/sale.order/search_read"
			},
			"rpcMethod": "call",
			"auth": {
				"type": "api_key",
				"header": "X-Odoo-API-Key"
			},
			"fieldMappings": [
				{"external": "id", "standard": "customer.externalId"},
				{"external": "name", "standard": "customer.name"},
				{"external": "email", "standard": "customer.email"},
				{"external": "phone", "standard": "customer.phone"}
			]
		}`,
	}
}

// crmSalesforceTemplate Salesforce REST API 模板
func crmSalesforceTemplate() *ConnectorTemplate {
	return &ConnectorTemplate{
		Code:          "crm_salesforce",
		Name:          "Salesforce CRM",
		Description:   "Salesforce REST API 连接器，支持客户、合同和工单数据查询",
		ConnectorType: "openapi",
		AuthType:      "oauth2",
		BaseURL:       "https://[instance].salesforce.com/services/data/v58.0",
		TemplateConfig: `{
			"apiVersion": "v58.0",
			"endpoints": {
				"customers": "/sobjects/Account",
				"contacts": "/sobjects/Contact",
				"contracts": "/sobjects/Contract",
				"cases": "/sobjects/Case"
			},
			"auth": {
				"type": "oauth2",
				"grantType": "client_credentials",
				"tokenUrl": "https://login.salesforce.com/services/oauth2/token"
			},
			"fieldMappings": [
				{"external": "Id", "standard": "customer.externalId"},
				{"external": "Name", "standard": "customer.name"},
				{"external": "AccountNumber", "standard": "customer.code"}
			]
		}`,
	}
}

// crmHubspotTemplate HubSpot API 模板
func crmHubspotTemplate() *ConnectorTemplate {
	return &ConnectorTemplate{
		Code:          "crm_hubspot",
		Name:          "HubSpot CRM",
		Description:   "HubSpot CRM API 连接器，支持公司、联系人和工单数据查询",
		ConnectorType: "openapi",
		AuthType:      "api_key",
		BaseURL:       "https://api.hubapi.com/crm/v3",
		TemplateConfig: `{
			"apiVersion": "v3",
			"endpoints": {
				"companies": "/objects/companies",
				"contacts": "/objects/contacts",
				"tickets": "/objects/tickets"
			},
			"auth": {
				"type": "api_key",
				"header": "Authorization",
				"prefix": "Bearer "
			},
			"fieldMappings": [
				{"external": "id", "standard": "customer.externalId"},
				{"external": "properties.name", "standard": "customer.name"},
				{"external": "properties.domain", "standard": "customer.domain"}
			]
		}`,
	}
}

// iotMQTTGenericTemplate 通用 MQTT 设备数据模板
func iotMQTTGenericTemplate() *ConnectorTemplate {
	return &ConnectorTemplate{
		Code:          "iot_mqtt_generic",
		Name:          "通用 MQTT 设备接入",
		Description:   "MQTT Broker 设备遥测和事件数据接入连接器，支持设备状态、遥测和告警数据订阅",
		ConnectorType: models.ConnectorTypeMQTT,
		AuthType:      "basic",
		BaseURL:       "mqtts://[broker]:8883",
		TemplateConfig: `{
			"protocol": "mqtt",
			"version": "3.1.1",
			"unknownDevicePolicy": "dead_letter",
			"qos": 1,
			"topics": {
				"telemetry": "devices/+/telemetry",
				"events": "devices/+/events",
				"status": "devices/+/status"
			},
			"cleanSession": false,
			"autoReconnect": true,
			"keepAliveSeconds": 60,
			"fieldMappings": [
				{"external": "message_id", "standard": "connector.messageId"},
				{"external": "device_id", "standard": "device.serialNo"},
				{"external": "temperature", "standard": "deviceTelemetry.metrics.temp"},
				{"external": "humidity", "standard": "deviceTelemetry.metrics.humidity"},
				{"external": "fault_code", "standard": "faultCode.code"},
				{"external": "severity", "standard": "deviceEvent.severity"},
				{"external": "recorded_at", "standard": "deviceTelemetry.recordedAt"}
			]
		}`,
	}
}

// deviceLogHTTPTemplate 设备日志 HTTP 采集模板
func deviceLogHTTPTemplate() *ConnectorTemplate {
	return &ConnectorTemplate{
		Code:          "device_log_http",
		Name:          "设备日志 HTTP 采集",
		Description:   "通过 HTTP API 采集设备运行日志、故障码和诊断数据",
		ConnectorType: "openapi",
		AuthType:      "api_key",
		BaseURL:       "https://[device-platform]/api/v1",
		TemplateConfig: `{
			"apiVersion": "v1",
			"endpoints": {
				"deviceLogs": "/logs",
				"faultCodes": "/faults",
				"diagnostics": "/diagnostics"
			},
			"auth": {
				"type": "api_key",
				"header": "X-API-Key"
			},
			"pagination": {
				"type": "cursor",
				"pageParam": "cursor",
				"limitParam": "limit",
				"defaultLimit": 100
			},
			"fieldMappings": [
				{"external": "device_id", "standard": "device.externalId"},
				{"external": "log_level", "standard": "deviceLog.level"},
				{"external": "message", "standard": "deviceLog.message"},
				{"external": "timestamp", "standard": "deviceLog.recordedAt"},
				{"external": "fault_code", "standard": "faultCode.code"}
			]
		}`,
	}
}
