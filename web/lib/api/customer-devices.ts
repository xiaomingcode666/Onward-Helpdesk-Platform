"use client"

import { buildCustomerPortalQueryString } from "@/lib/api/customer-portal-query"
import { customerPortalRequest } from "@/lib/api/customer-portal-client"
import type {
  CustomerPortalDevice,
  CustomerPortalListQuery,
  CustomerPortalManualFile,
  CustomerPortalPage,
} from "@/lib/api/customer-portal-types"

export function fetchCustomerDevices() {
  return customerPortalRequest<CustomerPortalDevice[]>("/devices")
}

export function fetchCustomerDevicesPage(query?: CustomerPortalListQuery) {
  return customerPortalRequest<CustomerPortalPage<CustomerPortalDevice>>(`/devices/page${buildCustomerPortalQueryString(query)}`)
}

export function fetchCustomerDeviceAccess(deviceId: number) {
  return customerPortalRequest<{ device_id: number; accessible: boolean }>(`/devices/${deviceId}/access`)
}

export type BindCustomerDevicePayload = {
  serviceCode: string
  deviceNo?: string
  serialNo?: string
  regionCode?: string
}

export type BindCustomerDeviceResponse = {
  deviceId: number
  deviceNo?: string
  productName?: string
  bindingRole?: string
  status: number
}

export function bindCustomerDevice(payload: BindCustomerDevicePayload) {
  return customerPortalRequest<BindCustomerDeviceResponse>("/devices/bind", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function fetchCustomerDeviceManuals(deviceId: number) {
  return customerPortalRequest<CustomerPortalManualFile[]>(`/devices/${deviceId}/manuals`)
}
