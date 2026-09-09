"use client"

import type { BreadcrumbProps } from "antd"
import { usePathname } from "next/navigation"

import { HeaderCrumbs } from "@/components/layout/header-crumbs"
import { useI18n } from "@/i18n/provider"
import { getRemoteHelpdeskBreadcrumbKeys } from "@/lib/navigation-remote-helpdesk"

type RouteBreadcrumbsProps = {
  className?: string
  size?: "page" | "sm"
}

export function RouteBreadcrumbs({ className, size = "sm" }: RouteBreadcrumbsProps) {
  const pathname = usePathname()
  const t = useI18n()
  const keys = getRemoteHelpdeskBreadcrumbKeys(pathname)

  if (keys.length === 0) {
    return null
  }

  return (
    <HeaderCrumbs className={className} size={size}>
      {keys.map((key) => t(key)).join(" / ")}
    </HeaderCrumbs>
  )
}

/**
 * 产出当前路由的 antd Breadcrumb items（供 PageShell breadcrumb 使用），
 * 与 RouteBreadcrumbs 共用同一路由 → 文案 key 映射。
 */
export function useRouteBreadcrumbItems(): BreadcrumbProps["items"] {
  const pathname = usePathname()
  const t = useI18n()
  const keys = getRemoteHelpdeskBreadcrumbKeys(pathname)

  if (keys.length === 0) {
    return undefined
  }
  return keys.map((key) => ({ title: t(key) }))
}
