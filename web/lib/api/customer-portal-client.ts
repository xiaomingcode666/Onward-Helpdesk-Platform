"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import { request } from "@/lib/api/client"
import { readSession } from "@/lib/auth"
import { readCustomerPortalRuntimeConfig } from "@/lib/api/customer-portal-runtime"

async function buildCustomerPortalHeaders(headers?: HeadersInit) {
	const accountSession = readSession()
	if (accountSession?.domainType === "customer" && accountSession.accessToken) {
		return {
			Authorization: `Bearer ${accountSession.accessToken}`,
			...(headers as Record<string, string> | undefined),
		}
	}
	throw new Error(translateCurrentMessage("miscTailExtract.libMisc.customerPortal.formalAccountLoginRequired"))
}

export async function customerPortalRequest<T>(
  path: string,
  init?: RequestInit
) {
  const runtimeConfig = readCustomerPortalRuntimeConfig()
  const headers = await buildCustomerPortalHeaders(init?.headers)
  return request<T>(`/api/customer/v1${path}`, {
    ...init,
    skipAuth: false,
    headers,
    baseUrl: runtimeConfig.baseUrl,
  })
}
