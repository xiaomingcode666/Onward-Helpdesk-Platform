"use client"

import { request } from "@/lib/api/client"

export type PlatformSystemIntroDoc = {
  id: number
  asset_id: number
  title: string
  filename: string
  file_size: number
  mime_type: string
  sort_no: number
  status: "draft" | "published"
  url: string
  published_at: string
  created_at: string
  updated_at: string
  create_user_name: string
}

export type PlatformSystemIntroListResult = {
  items: PlatformSystemIntroDoc[]
  total: number
}

export function fetchPlatformSystemIntroDocs(params: { page?: number; limit?: number; keyword?: string }) {
  const search = new URLSearchParams()
  if (params.page) search.set("page", String(params.page))
  if (params.limit) search.set("limit", String(params.limit))
  if (params.keyword) search.set("keyword", params.keyword)
  const qs = search.toString()
  return request<PlatformSystemIntroListResult>(`/api/platform/system-intro/list${qs ? `?${qs}` : ""}`)
}

export function uploadPlatformSystemIntroDoc(file: File, payload?: { title?: string; sortNo?: number }) {
  const formData = new FormData()
  formData.set("file", file)
  if (payload?.title) formData.set("title", payload.title)
  if (typeof payload?.sortNo === "number") formData.set("sort_no", String(payload.sortNo))
  return request<PlatformSystemIntroDoc>("/api/platform/system-intro/create", {
    method: "POST",
    body: formData,
  })
}

export function updatePlatformSystemIntroDoc(payload: {
  id: number
  title?: string
  sort_no?: number
  status?: "draft" | "published"
}) {
  return request<PlatformSystemIntroDoc>("/api/platform/system-intro/update", {
    method: "POST",
    body: JSON.stringify(payload),
  })
}

export function deletePlatformSystemIntroDoc(id: number) {
  return request<void>("/api/platform/system-intro/delete", {
    method: "POST",
    body: JSON.stringify({ id }),
  })
}
