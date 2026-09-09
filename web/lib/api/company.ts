import { request } from "@/lib/api/client"
import type { PageResult } from "@/lib/api/admin"

export type AdminCompany = {
  id: number
  name: string
  code: string
  customerCount: number
  status: number
  remark: string
  createdAt: string
  updatedAt: string
}

export type CreateAdminCompanyPayload = {
  name: string
  code: string
  remark: string
}

export type UpdateAdminCompanyPayload = CreateAdminCompanyPayload & {
  id: number
}

function toQueryString(query?: Record<string, string | number | undefined>) {
  if (!query) {
    return ""
  }
  const params = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    if (value === undefined || value === "") {
      return
    }
    params.set(key, String(value))
  })
  const output = params.toString()
  return output ? `?${output}` : ""
}

export function fetchCompanies(query?: Record<string, string | number | undefined>) {
  return request<PageResult<AdminCompany>>(
    `/api/dashboard/company/list${toQueryString(query)}`
  )
}

const COMPANY_CATALOG_PAGE_SIZE = 100

export async function fetchCompanyCatalog(
  query?: Record<string, string | number | undefined>
) {
  const { page: _page, limit: _limit, ...rest } = query ?? {}
  const results: AdminCompany[] = []
  let page = 1

  for (;;) {
    const response = await fetchCompanies({
      ...rest,
      page,
      limit: COMPANY_CATALOG_PAGE_SIZE,
    })
    results.push(...response.results)

    const total = response.page?.total ?? results.length
    if (results.length >= total || response.results.length < COMPANY_CATALOG_PAGE_SIZE) {
      return results
    }
    page += 1
  }
}

export function fetchCompany(id: number) {
  return request<AdminCompany>(`/api/dashboard/company/${id}`)
}

export function createCompany(payload: CreateAdminCompanyPayload) {
  return request<AdminCompany>("/api/dashboard/company/create", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateCompany(payload: UpdateAdminCompanyPayload) {
  return request<void>("/api/dashboard/company/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function updateCompanyStatus(id: number, status: number) {
  return request<void>("/api/dashboard/company/update_status", {
    method: "POST",
    body: JSON.stringify({ id, status }),
  })
}

export function deleteCompany(id: number) {
  return request<void>("/api/dashboard/company/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}
