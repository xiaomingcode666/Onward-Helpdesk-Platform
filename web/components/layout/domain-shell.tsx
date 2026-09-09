"use client"

import {
  ArrowLeftIcon,
  CalendarClockIcon,
  HouseIcon,
  MenuIcon,
  PanelLeftCloseIcon,
  PanelLeftOpenIcon,
  SearchIcon,
  XIcon,
} from "lucide-react"
import type { MenuProps } from "antd"
import Link from "next/link"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import {
  useEffect,
  useMemo,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react"

import { IconButton, SidebarNavigation, TopHeader } from "@railops/ui"

import { NotificationBell } from "@/components/layout/notification-bell"
import { AccountMenu } from "@/components/layout/account-menu"
import { AppLogoMark } from "@/components/brand-logo"
import { EngineerLoginBriefingModal } from "@/components/layout/engineer-login-briefing-modal"
import { CanAccessMenu } from "@/components/layout/permission-guard"
import { ForbiddenState } from "@/components/shared/error-states"
import { RouteLoadingPage } from "@/components/shared/loading-states"
import { Skeleton } from "@/components/ui/skeleton"
import { useAuth } from "@/components/auth-provider"
import { useI18n } from "@/i18n/provider"
import {
  canAccessRemoteHelpdeskNavItem,
  getRemoteHelpdeskNavItemForPathname,
  isRemoteHelpdeskNavItemActive,
  remoteHelpdeskRouteGroups,
  type RemoteHelpdeskDomain,
  type RemoteHelpdeskNavItem,
  type RemoteHelpdeskSidebarSection,
} from "@/lib/navigation-remote-helpdesk"
import { cn } from "@/lib/utils"
import { registerNativeMobilePushNotifications } from "@/lib/mobile/push-registration"
import { getCustomerThemeClassName, normalizeCustomerTheme } from "@/lib/customer-theme"

type ShellDomain = Exclude<RemoteHelpdeskDomain, "mobile">

type DomainShellProps = {
  children: ReactNode
  domain: ShellDomain
  permissions?: readonly string[]
  sections: RemoteHelpdeskSidebarSection[]
  sidebarHeader?: ReactNode
  sidebarFooter?: ReactNode
  sidebarTitleKey?: string
  showDomainTabs?: boolean
}

const domainMarks: Record<ShellDomain, string> = {
  platform: "P",
  enterprise: "E",
  partner: "S",
  customer: "C",
}

const domainTopbarKeys: Record<ShellDomain, string> = {
  platform: "remoteTopbar.platformTab",
  enterprise: "remoteTopbar.enterpriseTab",
  partner: "remoteTopbar.partnerTab",
  customer: "remoteTopbar.customerTab",
}

const shellLayoutVars = {
  "--remote-shell-sidebar-width": "240px",
} as CSSProperties

const REMOTE_SHELL_SIDEBAR_COLLAPSED_KEY = "remote-shell.sidebar.collapsed"
const REMOTE_SHELL_SIDEBAR_WIDTH = "240px"
const REMOTE_SHELL_SIDEBAR_COLLAPSED_WIDTH = "64px"

function DesktopTabs({ domain }: { domain: ShellDomain }) {
  const t = useI18n()

  return (
    <nav className="hidden items-center gap-1 lg:flex" aria-label={t("remoteShell.domainNavigation")}>
      {remoteHelpdeskRouteGroups
        .filter((group) => group.domain !== "mobile" && group.domain !== "partner")
        .map((group) => {
          const groupDomain = group.domain as ShellDomain
          const active = groupDomain === domain
          return (
            <Link
              key={group.domain}
              href={group.basePath}
              aria-current={active ? "page" : undefined}
              className={cn(
                "inline-flex h-8 items-center gap-1.5 rounded-md px-2.5 text-xs font-medium text-[var(--railops-text-secondary)] transition hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)]",
                active && "bg-[var(--railops-primary)] text-white hover:bg-[var(--railops-primary-hover)] hover:text-white"
              )}
            >
              <span className={cn(
                "grid size-5 place-items-center rounded bg-[var(--railops-surface-muted)] text-rhd-xs text-[var(--railops-text-secondary)]",
                active && "bg-white/15 text-white"
              )}>
                {domainMarks[groupDomain]}
              </span>
              {t(domainTopbarKeys[groupDomain])}
            </Link>
          )
        })}
    </nav>
  )
}

function SidebarNav({
  collapsed,
  footer,
  header,
  onCollapseToggle,
  onNavigate,
  pathname,
  permissions,
  featureFlags,
  sections,
  titleKey,
}: {
  collapsed: boolean
  footer?: ReactNode
  header?: ReactNode
  onCollapseToggle?: () => void
  onNavigate?: () => void
  pathname: string | null
  permissions?: readonly string[]
  featureFlags?: Record<string, boolean>
  sections: RemoteHelpdeskSidebarSection[]
  titleKey?: string
}) {
  const t = useI18n()
  const router = useRouter()
  const [optimisticSelection, setOptimisticSelection] = useState<{
    key: string
    pathname: string | null
  } | null>(null)
  const visibleSections = useMemo(
    () =>
      sections
        .map((section) => ({
          ...section,
          items: section.items.filter((item) =>
            canAccessRemoteHelpdeskNavItem(item, permissions, featureFlags)
          ),
        }))
        .filter((section) => section.items.length > 0),
    [featureFlags, permissions, sections]
  )
  const activeMenuItemKey = useMemo(
    () =>
      visibleSections
        .flatMap((section) => section.items)
        .find((item) => isRemoteHelpdeskNavItemActive(pathname, item))
        ?.key,
    [pathname, visibleSections]
  )
  const menuItemByKey = useMemo(
    () => new Map(
      visibleSections
        .flatMap((section) => section.items)
        .map((item) => [item.key, item])
    ),
    [visibleSections]
  )
  const defaultOpenKeys = useMemo(
    () => visibleSections.filter((section) => !section.flat).map((section) => section.labelKey),
    [visibleSections]
  )
  const optimisticSelectedKey = optimisticSelection?.pathname === pathname
    ? optimisticSelection.key
    : null
  const selectedMenuItemKey = optimisticSelectedKey && menuItemByKey.has(optimisticSelectedKey)
    ? optimisticSelectedKey
    : activeMenuItemKey

  const handleMenuClick: MenuProps["onClick"] = (event) => {
    const item = menuItemByKey.get(String(event.key))
    if (!item) return
    setOptimisticSelection({ key: item.key, pathname })
    onNavigate?.()
    router.push(item.href)
  }
  const menuItems = useMemo<MenuProps["items"]>(() =>
    visibleSections.flatMap((section): NonNullable<MenuProps["items"]> => {
      const renderItem = (item: RemoteHelpdeskNavItem) => {
        const Icon = item.icon
        return {
          key: item.key,
          icon: <Icon className="rhd-railops-shell-menu-icon" />,
          onMouseEnter: () => router.prefetch(item.href),
          onFocus: () => router.prefetch(item.href),
          label: (
            <span
              aria-current={isRemoteHelpdeskNavItemActive(pathname, item) ? "page" : undefined}
              title={t(item.titleKey)}
              className="rhd-railops-shell-menu-link"
            >
              <span className="rhd-railops-shell-menu-text is-secondary">{t(item.titleKey)}</span>
              {item.badge ? <span className="rhd-railops-shell-menu-badge">{item.badge}</span> : null}
            </span>
          ),
          title: t(item.titleKey),
        }
      }
      if (section.flat) {
        return section.items.map(renderItem) as NonNullable<MenuProps["items"]>
      }
      const SectionIcon = section.icon ?? section.items[0]?.icon
      return [{
        key: section.labelKey,
        icon: SectionIcon ? <SectionIcon className="rhd-railops-shell-menu-icon" /> : undefined,
        label: <span className="rhd-railops-shell-menu-section is-primary">{t(section.labelKey)}</span>,
        popupClassName: "rhd-railops-shell-menu-popup",
        title: t(section.labelKey),
        children: section.items.map(renderItem),
      }] as NonNullable<MenuProps["items"]>
    }),
    [pathname, router, t, visibleSections]
  )

  const [openKeysState, setOpenKeysState] = useState<string[]>(() =>
    collapsed ? [] : defaultOpenKeys
  )
  useEffect(() => {
    setOpenKeysState(collapsed ? [] : defaultOpenKeys)
  }, [collapsed, defaultOpenKeys])

  const brand = (
    <>
      <div className="rhd-railops-shell-sidebar-tools">
        <div className={cn("min-w-0 text-xs font-semibold uppercase tracking-wide text-muted-foreground", collapsed && "sr-only")}>
          {t(titleKey ?? "remoteShell.sections")}
        </div>
        {onCollapseToggle ? (
          <IconButton
            icon={collapsed ? <PanelLeftOpenIcon className="size-4" /> : <PanelLeftCloseIcon className="size-4" />}
            tooltip={collapsed ? t("remoteShell.expandNavigation") : t("remoteShell.collapseNavigation")}
            aria-label={collapsed ? t("remoteShell.expandNavigation") : t("remoteShell.collapseNavigation")}
            onClick={onCollapseToggle}
          />
        ) : null}
      </div>
      {!collapsed ? header : null}
    </>
  )

  return (
    <SidebarNavigation
      className="rhd-railops-shell-sider"
      brand={brand}
      footer={footer}
      items={menuItems}
      selectedKeys={selectedMenuItemKey ? [selectedMenuItemKey] : []}
      openKeys={openKeysState}
      collapsed={collapsed}
      width={240}
      collapsedWidth={64}
      onClick={handleMenuClick}
      onOpenChange={(keys) => setOpenKeysState(keys as string[])}
    />
  )
}

export function DomainShell({
  children,
  domain,
  permissions,
  sections,
  sidebarFooter,
  sidebarHeader,
  sidebarTitleKey,
  showDomainTabs = false,
}: DomainShellProps) {
  const t = useI18n()
  const { ready, session } = useAuth()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const [mobileNavOpen, setMobileNavOpen] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const effectivePermissions = permissions ?? session?.permissions ?? []
  const effectiveFeatureFlags = session?.featureFlags
  const tenantBranding = domain === "enterprise" || domain === "customer" ? session?.tenantBranding : undefined
  const shellBrandName = tenantBranding?.brandName?.trim() || t("remoteTopbar.brandTitle")
  const shellBrandLogo = tenantBranding?.logoUrl?.trim() || ""
  const customerTheme = normalizeCustomerTheme(tenantBranding?.customerTheme)
  const customerThemeClassName = domain === "customer" ? getCustomerThemeClassName(customerTheme) : undefined
  const activeItem = useMemo(
    () => {
      const item = getRemoteHelpdeskNavItemForPathname(pathname)
      return item?.domain === domain ? item : undefined
    },
    [domain, pathname]
  )
  const isEmployeePortalMode = session?.supportMode === "employee_portal"
  const isMyWorkScheduleRoute = pathname?.startsWith("/enterprise/org/schedules") && searchParams?.get("view") === "mine"
  const routePermission = pathname?.startsWith("/enterprise/org/schedules")
    ? (isEmployeePortalMode || isMyWorkScheduleRoute ? null : "agentTeamSchedule.view")
    : activeItem?.requiredPermission
  const canAccessPage = CanAccessMenu(routePermission, effectivePermissions) &&
    canAccessRemoteHelpdeskNavItem(activeItem, effectivePermissions, effectiveFeatureFlags)
  const domainHomeHref = remoteHelpdeskRouteGroups.find((group) => group.domain === domain)?.basePath || "/"
  const accessDeniedTitle = t("remoteShell.accessDenied")
  const canShowEngineerBriefing = domain === "enterprise" && (
    isEmployeePortalMode || (!session?.supportGrantId && !session?.supportMode)
  )
  const canShowWorkScheduleShortcut = domain === "enterprise" && (
    isEmployeePortalMode || CanAccessMenu("agentTeamSchedule.view", effectivePermissions)
  )
  const workScheduleHref = "/enterprise/org/schedules?view=mine"
  const shouldAutoOpenEngineerBriefing = Boolean(
    isEmployeePortalMode ||
      pathname?.startsWith("/enterprise/ticket-workbench") ||
      pathname?.startsWith("/enterprise/workbench") ||
      pathname?.startsWith("/enterprise/products"),
  )

  useEffect(() => {
    try {
      const stored = window.localStorage.getItem(REMOTE_SHELL_SIDEBAR_COLLAPSED_KEY)
      if (stored !== null) {
        setSidebarCollapsed(stored === "true")
      }
    } catch {
      // Ignore storage access issues.
    }
  }, [])

  useEffect(() => {
    try {
      window.localStorage.setItem(REMOTE_SHELL_SIDEBAR_COLLAPSED_KEY, String(sidebarCollapsed))
    } catch {
      // Ignore storage access issues.
    }
  }, [sidebarCollapsed])

  useEffect(() => {
    if (domain !== "enterprise" || !ready || !session) return
    void registerNativeMobilePushNotifications()
  }, [domain, ready, session])

  const shellLayoutStyle = {
    ...shellLayoutVars,
    "--remote-shell-sidebar-width": sidebarCollapsed
      ? REMOTE_SHELL_SIDEBAR_COLLAPSED_WIDTH
      : REMOTE_SHELL_SIDEBAR_WIDTH,
  } as CSSProperties

  if (!ready || !session) {
    return (
      <div
        className={cn(
          "rhd-railops-domain-shell min-h-screen bg-[var(--railops-layout-background)] text-[var(--railops-text)]",
          `rhd-railops-domain-${domain}`
        )}
        aria-busy="true"
        aria-live="polite"
      >
        <header className="border-b border-[var(--railops-border-light)] bg-[var(--railops-surface)] shadow-[var(--railops-header-shadow)]">
          <div className="flex min-h-12 items-center gap-2.5 px-3 sm:px-4">
            <Skeleton className="size-7 shrink-0 rounded-md" />
            <Skeleton className="h-4 w-36" />
            <div className="ml-auto flex items-center gap-2">
              <Skeleton className="hidden h-8 w-24 rounded-md md:block" />
              <Skeleton className="size-8 rounded-md" />
              <Skeleton className="size-8 rounded-full" />
            </div>
          </div>
        </header>
        <div
          className="grid min-h-[calc(100vh-49px)] lg:grid-cols-[var(--remote-shell-sidebar-width)_minmax(0,1fr)]"
          style={shellLayoutStyle}
        >
          <aside className="hidden bg-[var(--railops-surface)] p-2.5 lg:block" aria-hidden="true">
            <Skeleton className="h-10 w-full" />
            <div className="mt-4 space-y-2.5">
              {Array.from({ length: 7 }, (_, index) => (
                <Skeleton key={index} className="h-8 w-full rounded-md" />
              ))}
            </div>
          </aside>
          <main className="min-w-0 overflow-x-hidden">
            <div className="w-full px-3 py-3 sm:px-4 sm:py-4 xl:px-5">
              <RouteLoadingPage label={t("auth.checkingSession")} />
            </div>
          </main>
        </div>
      </div>
    )
  }

  return (
    <div className={cn(
      "rhd-railops-domain-shell min-h-screen bg-[var(--railops-layout-background)] text-[var(--railops-text)]",
      `rhd-railops-domain-${domain}`,
      customerThemeClassName,
    )}
      data-customer-theme={domain === "customer" ? customerTheme : undefined}
    >
      <TopHeader className="rhd-railops-shell-top-header"
        actions={
          <>
            <button
              type="button"
              className="hidden h-8 shrink-0 items-center gap-1.5 rounded-md border border-[var(--railops-border)] bg-[var(--railops-surface)] px-2.5 text-xs text-[var(--railops-text-secondary)] shadow-[var(--railops-card-shadow)] transition hover:border-[#bfdbfe] hover:bg-[var(--railops-primary-bg)] hover:text-[var(--railops-primary-hover)] md:inline-flex xl:px-3"
              aria-label={t("remoteShell.search")}
              title={t("remoteShell.search")}
            >
              <SearchIcon className="size-3.5" />
              <span className="hidden xl:inline">{t("remoteShell.search")}</span>
            </button>
            {canShowWorkScheduleShortcut ? (
              <Link
                href={workScheduleHref}
                className="hidden h-8 shrink-0 items-center gap-1.5 rounded-md border border-[#bfdbfe] bg-[var(--railops-primary-bg)] px-2.5 text-xs font-medium text-[var(--railops-primary-hover)] shadow-[var(--railops-card-shadow)] transition hover:border-[var(--railops-primary)] md:inline-flex"
                aria-label={t("railopsExtract.shell.workSchedule")}
                title={t("railopsExtract.shell.workSchedule")}
              >
                <CalendarClockIcon className="size-3.5" />
                <span>{t("railopsExtract.shell.workSchedule")}</span>
              </Link>
            ) : null}
            {canShowWorkScheduleShortcut && canShowEngineerBriefing ? (
              <span className="hidden h-5 w-px shrink-0 bg-[var(--railops-border-light)] md:block" aria-hidden="true" />
            ) : null}
            {canShowEngineerBriefing ? (
              <EngineerLoginBriefingModal autoOpen={shouldAutoOpenEngineerBriefing} session={session} />
            ) : null}
            <NotificationBell domain={domain} />
            <AccountMenu domain={domain} />
          </>
        }
      >
        <button
          type="button"
          className="grid size-8 shrink-0 place-items-center rounded-md border border-[var(--railops-border)] bg-[var(--railops-surface)] text-[var(--railops-text)] shadow-[var(--railops-card-shadow)] lg:hidden"
          aria-label={t("remoteShell.openNavigation")}
          onClick={() => setMobileNavOpen(true)}
        >
          <MenuIcon className="size-4" />
        </button>

        <div className="flex min-w-0 items-center gap-3">
          {shellBrandLogo ? (
            <span className="inline-flex h-8 w-28 shrink-0 items-center overflow-hidden">
              {/* Tenant logos may be same-origin assets or tenant-managed HTTPS resources. */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={shellBrandLogo} alt={shellBrandName} className="max-h-8 max-w-28 object-contain object-left" />
            </span>
          ) : (
            <AppLogoMark alt={shellBrandName} className="size-7" imageClassName="p-0.5" priority />
          )}
          <div className="min-w-0">
            <div className="max-w-36 truncate text-rhd-xl font-bold sm:max-w-64">{shellBrandName}</div>
          </div>
        </div>

        {showDomainTabs ? <DesktopTabs domain={domain} /> : null}
      </TopHeader>
      {session?.supportGrantId ? (
        <div className="flex min-h-7 items-center justify-between gap-3 border-t border-[#fde68a] bg-[var(--railops-warning-bg)] px-3 text-rhd-xs font-medium text-[#92400e] sm:px-4">
          <span className="truncate">
            {t(session.domainType === "customer"
              ? "account.customerSupportBanner"
              : session.supportMode === "employee_portal"
                ? "account.employeeSupportBanner"
                : "account.platformSupportBanner", {
              tenantId: session.tenantId,
              grantId: session.supportGrantId,
              operator: session.impersonatedBy || session.user?.username || "-",
            })}
          </span>
        </div>
      ) : null}

      <div
        className="grid min-h-[calc(100vh-49px)] lg:grid-cols-[var(--remote-shell-sidebar-width)_minmax(0,1fr)]"
        style={shellLayoutStyle}
      >
        <aside className="rhd-railops-shell-aside hidden min-w-0 overflow-hidden bg-[var(--railops-surface)] lg:block">
          <SidebarNav
            collapsed={sidebarCollapsed}
            footer={sidebarFooter}
            header={sidebarHeader}
            onCollapseToggle={() => setSidebarCollapsed((value) => !value)}
            pathname={pathname}
            permissions={effectivePermissions}
            featureFlags={effectiveFeatureFlags}
            sections={sections}
            titleKey={sidebarTitleKey}
          />
        </aside>

        {mobileNavOpen ? (
          <div className="fixed inset-0 z-40 bg-slate-950/35 lg:hidden">
            <div className="rhd-railops-shell-mobile-panel h-full w-[min(84vw,288px)] bg-[var(--railops-surface)] shadow-[var(--railops-float-shadow)]">
              <div className="flex h-11 items-center justify-end px-3">
                <button
                  type="button"
                  className="grid size-8 place-items-center rounded-md text-[var(--railops-text-secondary)]"
                  aria-label={t("remoteShell.closeNavigation")}
                  onClick={() => setMobileNavOpen(false)}
                >
                  <XIcon className="size-4" />
                </button>
              </div>
              <SidebarNav
                collapsed={false}
                footer={sidebarFooter}
                header={sidebarHeader}
                pathname={pathname}
                permissions={effectivePermissions}
                featureFlags={effectiveFeatureFlags}
                sections={sections}
                titleKey={sidebarTitleKey}
                onNavigate={() => setMobileNavOpen(false)}
              />
            </div>
          </div>
        ) : null}

        <main className="min-w-0 overflow-x-hidden">
          <div className="w-full px-3 py-3 sm:px-4 sm:py-4 xl:px-5">
            {canAccessPage ? (
              children
            ) : (
              <ForbiddenState
                title={accessDeniedTitle === "remoteShell.accessDenied" ? t("errorStates.forbiddenTitle") : accessDeniedTitle}
                action={{ label: t("common.backHome"), href: domainHomeHref, icon: HouseIcon }}
                secondaryAction={{ label: t("common.back"), onClick: () => window.history.back(), variant: "outline", icon: ArrowLeftIcon }}
              />
            )}
          </div>
        </main>
      </div>
    </div>
  )
}
