"use client"

import type { ComponentProps } from "react"
import { useMemo } from "react"
import Link from "next/link"

import { useI18n } from "@/i18n/provider"
import { AppLogoMark } from "@/components/brand-logo"
import {
  afterSalesNavSections,
  filterDashboardNavForSession,
  filterDashboardSecondaryNavForSession,
} from "@/lib/navigation"
import { useAuth } from "@/components/auth-provider"
import { NavMain } from "@/components/nav-main"
import { NavSecondary } from "@/components/nav-secondary"
import { NavUser } from "@/components/nav-user"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"

export function AppSidebar({ ...props }: ComponentProps<typeof Sidebar>) {
  const t = useI18n()
  const { session } = useAuth()
  const navSections = useMemo(
    () => filterDashboardNavForSession(session?.permissions, session?.roles),
    [session?.permissions, session?.roles]
  )
  const secondaryNavItems = useMemo(
    () => filterDashboardSecondaryNavForSession(session?.permissions, session?.roles),
    [session?.permissions, session?.roles]
  )
  const user = {
    name: session?.user.nickname || session?.user.username || t("common.notSignedIn"),
    email: session?.user.username || t("common.notSignedIn"),
    avatar: session?.user.avatar || "",
  }

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              size="lg"
              className="group-data-[collapsible=icon]:justify-center"
              render={<Link href="/dashboard" />}
            >
              <AppLogoMark alt={t("app.brand")} className="size-8" imageClassName="p-0.5" priority />
              <div className="grid min-w-0 flex-1 text-left leading-tight group-data-[collapsible=icon]:hidden">
                <span className="truncate text-sm font-semibold">{t("app.brand")}</span>
                <span className="truncate text-xs text-muted-foreground">{t("nav.afterSalesReception")}</span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        {afterSalesNavSections.map((section) => (
          <NavMain
            key={section.titleKey}
            icon={section.icon}
            sectionKey={section.titleKey}
            title={t(section.titleKey)}
            items={section.items.map((item) => ({
              ...item,
              title: t(item.titleKey),
              tier: item.tier,
            }))}
          />
        ))}
        <div className="h-1" />
        {navSections.map((section) => (
          <NavMain
            key={section.titleKey}
            icon={section.icon}
            sectionKey={section.titleKey}
            title={t(section.titleKey)}
            items={section.items.map((item) => ({
              ...item,
              title: t(item.titleKey),
            }))}
          />
        ))}
        {secondaryNavItems.length > 0 ? (
          <NavSecondary items={secondaryNavItems} className="mt-auto" />
        ) : null}
      </SidebarContent>
      <SidebarFooter>
        <NavUser user={user} />
      </SidebarFooter>
    </Sidebar>
  )
}
