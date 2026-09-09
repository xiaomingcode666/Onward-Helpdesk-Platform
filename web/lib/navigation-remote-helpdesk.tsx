import {
  BadgeCheckIcon,
  BarChart3Icon,
  BellIcon,
  BookOpenIcon,
  BookOpenTextIcon,
  Building2Icon,
  ChartNoAxesCombinedIcon,
  CircleUserRoundIcon,
  CpuIcon,
  DatabaseZapIcon,
  // Globe2Icon,
  HeadphonesIcon,
  HistoryIcon,
  HomeIcon,
  KeyRoundIcon,
  LandmarkIcon,
  LifeBuoyIcon,
  LinkIcon,
  LockKeyholeIcon,
  MessageSquareTextIcon,
  MonitorCogIcon,
  NetworkIcon,
  PackageIcon,
  ReceiptTextIcon,
  QrCodeIcon,
  ScanLineIcon,
  ShieldCheckIcon,
  SquareKanbanIcon,
  StoreIcon,
  TicketCheckIcon,
  UsersIcon,
  VideoIcon,
  WorkflowIcon,
  WrenchIcon,
  type LucideIcon,
} from "lucide-react"

export type RemoteHelpdeskDomain =
  | "platform"
  | "enterprise"
  | "partner"
  | "customer"
  | "mobile"

export type RemoteHelpdeskNavItem = {
  key: string
  titleKey: string
  href: string
  domain: RemoteHelpdeskDomain
  icon: LucideIcon
  requiredPermission?: string | readonly string[]
  requiredFeature?: string | readonly string[]
  badge?: string
  aliases?: string[]
}

export type RemoteHelpdeskRouteGroup = {
  domain: RemoteHelpdeskDomain
  titleKey: string
  basePath: string
  icon: LucideIcon
  items: RemoteHelpdeskNavItem[]
}

export const platformNavItems: RemoteHelpdeskNavItem[] = [
  {
    key: "overview",
    titleKey: "remoteNav.platform.overview",
    href: "/platform",
    domain: "platform",
    icon: HomeIcon,
    requiredPermission: "tenant.view",
  },
  {
    key: "tenants",
    titleKey: "remoteNav.platform.tenants",
    href: "/platform/tenants",
    domain: "platform",
    icon: Building2Icon,
    requiredPermission: "tenant.view",
  },
  {
    key: "usage",
    titleKey: "remoteNav.platform.usage",
    href: "/platform/usage",
    domain: "platform",
    icon: ChartNoAxesCombinedIcon,
    requiredPermission: ["report.view", "finance.view"],
  },
  {
    key: "models",
    titleKey: "remoteNav.platform.models",
    href: "/platform/models",
    domain: "platform",
    icon: CpuIcon,
    requiredPermission: ["aiConfig.view", "finance.view"],
  },
  {
    key: "ops",
    titleKey: "remoteNav.platform.ops",
    href: "/platform/ops",
    domain: "platform",
    icon: MonitorCogIcon,
    requiredPermission: "tenant.view",
  },
  {
    key: "access-governance",
    titleKey: "remoteNav.platform.access-governance",
    href: "/platform/access-governance",
    domain: "platform",
    icon: NetworkIcon,
    requiredPermission: "tenantIntegrationConfig.view",
  },
  // {
  //   key: "globalization",
  //   titleKey: "remoteNav.platform.globalization",
  //   href: "/platform/globalization",
  //   domain: "platform",
  //   icon: Globe2Icon,
  //   requiredPermission: "tenant.view",
  // },
  {
    key: "staff",
    titleKey: "remoteNav.platform.staff",
    href: "/platform/staff",
    domain: "platform",
    icon: CircleUserRoundIcon,
    requiredPermission: "user.view",
  },
  {
    key: "permissions",
    titleKey: "remoteNav.platform.permissions",
    href: "/platform/permissions",
    domain: "platform",
    icon: KeyRoundIcon,
    requiredPermission: "role.view",
  },
  {
    key: "audit",
    titleKey: "remoteNav.platform.audit",
    href: "/platform/audit",
    domain: "platform",
    icon: ShieldCheckIcon,
    requiredPermission: "session.view",
    aliases: ["/platform/iam-audit"],
  },
  {
    key: "system-config",
    titleKey: "remoteNav.platform.system-config",
    href: "/platform/system-config",
    domain: "platform",
    icon: BookOpenTextIcon,
    requiredPermission: "systemIntro.view",
    aliases: ["/platform/system-intro"],
  },
]

export const enterpriseNavItems: RemoteHelpdeskNavItem[] = [
  {
    key: "workbench",
    titleKey: "remoteNav.enterprise.workbench",
    href: "/enterprise",
    domain: "enterprise",
    icon: SquareKanbanIcon,
    requiredPermission: "ticket.view",
    aliases: [
      "/enterprise/workbench",
      "/enterprise/workbench/tasks",
      "/enterprise/workbench/overview",
      "/enterprise/workbench/insights",
      "/enterprise/workbench/resources",
    ],
  },
  {
    key: "tickets",
    titleKey: "remoteNav.enterprise.tickets",
    href: "/enterprise/tickets",
    domain: "enterprise",
    icon: TicketCheckIcon,
    requiredPermission: "ticket.view",
  },
  {
    key: "ticket-workbench",
    titleKey: "remoteNav.enterprise.ticket-workbench",
    href: "/enterprise/ticket-workbench",
    domain: "enterprise",
    icon: MessageSquareTextIcon,
    requiredPermission: "ticket.view",
  },
  {
    key: "products",
    titleKey: "remoteNav.enterprise.products",
    href: "/enterprise/products",
    domain: "enterprise",
    icon: PackageIcon,
    requiredPermission: "product.view",
    requiredFeature: "product",
  },
  {
    key: "devices",
    titleKey: "remoteNav.enterprise.devices",
    href: "/enterprise/devices",
    domain: "enterprise",
    icon: DatabaseZapIcon,
    requiredPermission: "device.view",
    requiredFeature: "device",
    aliases: ["/enterprise/service-codes", "/enterprise/service-code"],
  },
  {
    key: "knowledge",
    titleKey: "remoteNav.enterprise.knowledge",
    href: "/enterprise/knowledge",
    domain: "enterprise",
    icon: BookOpenIcon,
    requiredPermission: "knowledgeBase.view",
    requiredFeature: "ai",
  },
  {
    key: "diagnosis",
    titleKey: "remoteNav.enterprise.diagnosis",
    href: "/enterprise/diagnosis",
    domain: "enterprise",
    icon: CpuIcon,
    requiredPermission: "ticket.view",
    requiredFeature: "deviceDiagnosis",
  },
  {
    key: "ai",
    titleKey: "remoteNav.enterprise.ai",
    href: "/enterprise/ai",
    domain: "enterprise",
    icon: HeadphonesIcon,
    requiredPermission: "aiAgent.view",
    requiredFeature: "ai",
  },
  {
    key: "video",
    titleKey: "remoteNav.enterprise.video",
    href: "/enterprise/video",
    domain: "enterprise",
    icon: VideoIcon,
    requiredPermission: "meeting.view",
  },
  {
    key: "access",
    titleKey: "remoteNav.enterprise.access",
    href: "/enterprise/access",
    domain: "enterprise",
    icon: LinkIcon,
    requiredPermission: "tenantIntegrationConfig.view",
  },
  {
    key: "server-console",
    titleKey: "remoteNav.enterprise.server-console",
    href: "/enterprise/server-console",
    domain: "enterprise",
    icon: MonitorCogIcon,
    requiredPermission: "tenantIntegrationConfig.view",
    requiredFeature: "serverConsole",
  },
  {
    key: "workflow",
    titleKey: "remoteNav.enterprise.workflow",
    href: "/enterprise/workflow",
    domain: "enterprise",
    icon: WorkflowIcon,
    requiredPermission: "aiWorkflow.view",
    requiredFeature: "ai",
    aliases: ["/enterprise/workflows", "/enterprise/workflow/"],
  },
  {
    key: "reports",
    titleKey: "remoteNav.enterprise.reports",
    href: "/enterprise/reports",
    domain: "enterprise",
    icon: BarChart3Icon,
    requiredPermission: "report.view",
  },
  {
    key: "people",
    titleKey: "remoteNav.enterprise.people",
    href: "/enterprise/people",
    domain: "enterprise",
    icon: UsersIcon,
    requiredPermission: "user.view",
  },
  {
    key: "customer-users",
    titleKey: "remoteNav.enterprise.customer-users",
    href: "/enterprise/customer-users",
    domain: "enterprise",
    icon: StoreIcon,
    requiredPermission: "customer.view",
  },
  {
    key: "partners",
    titleKey: "remoteNav.enterprise.partners",
    href: "/enterprise/partners",
    domain: "enterprise",
    icon: LandmarkIcon,
    requiredPermission: "user.view",
  },
  {
    key: "permissions",
    titleKey: "remoteNav.enterprise.permissions",
    href: "/enterprise/permissions",
    domain: "enterprise",
    icon: LockKeyholeIcon,
    requiredPermission: "role.view",
  },
  {
    key: "org",
    titleKey: "remoteNav.enterprise.org",
    href: "/enterprise/org",
    domain: "enterprise",
    icon: NetworkIcon,
    requiredPermission: "user.view",
  },
  {
    key: "audit",
    titleKey: "remoteNav.enterprise.audit",
    href: "/enterprise/audit",
    domain: "enterprise",
    icon: ShieldCheckIcon,
    requiredPermission: "session.view",
    aliases: ["/enterprise/iam-audit"],
  },
  {
    key: "usage",
    titleKey: "remoteNav.enterprise.usage",
    href: "/enterprise/usage",
    domain: "enterprise",
    icon: ReceiptTextIcon,
    requiredPermission: "productAIUsageCredential.view",
    requiredFeature: "ai",
  },
  {
    key: "notifications",
    titleKey: "remoteNav.enterprise.notifications",
    href: "/enterprise/notifications",
    domain: "enterprise",
    icon: BellIcon,
    requiredPermission: "notification.view",
  },
]

export const customerNavItems: RemoteHelpdeskNavItem[] = [
  {
    key: "chat",
    titleKey: "remoteNav.customer.chat",
    href: "/customer/chat",
    domain: "customer",
    icon: MessageSquareTextIcon,
    aliases: ["/customer", "/customer/home"],
  },
  {
    key: "tickets",
    titleKey: "remoteNav.customer.tickets",
    href: "/customer/tickets",
    domain: "customer",
    icon: TicketCheckIcon,
    aliases: ["/customer/ticket"],
  },
  {
    key: "meeting",
    titleKey: "remoteNav.customer.meeting",
    href: "/customer/meeting",
    domain: "customer",
    icon: VideoIcon,
  },
  {
    key: "devices",
    titleKey: "remoteNav.customer.devices",
    href: "/customer/devices",
    domain: "customer",
    icon: DatabaseZapIcon,
    requiredFeature: "device",
    aliases: ["/customer/history"],
  },
  {
    key: "my",
    titleKey: "remoteNav.customer.my",
    href: "/customer/my",
    domain: "customer",
    icon: CircleUserRoundIcon,
  },
]

export const partnerNavItems: RemoteHelpdeskNavItem[] = [
  {
    key: "overview",
    titleKey: "remoteNav.partner.overview",
    href: "/partner",
    domain: "partner",
    icon: HomeIcon,
    requiredPermission: "ticket.view",
  },
  {
    key: "tickets",
    titleKey: "remoteNav.partner.tickets",
    href: "/partner/tickets",
    domain: "partner",
    icon: TicketCheckIcon,
    requiredPermission: "ticket.view",
  },
  {
    key: "conversations",
    titleKey: "remoteNav.partner.conversations",
    href: "/partner/conversations",
    domain: "partner",
    icon: MessageSquareTextIcon,
    requiredPermission: "ticket.view",
  },
  {
    key: "video",
    titleKey: "remoteNav.partner.video",
    href: "/partner/video",
    domain: "partner",
    icon: VideoIcon,
    requiredPermission: "meeting.view",
  },
  {
    key: "people",
    titleKey: "remoteNav.partner.people",
    href: "/partner/people",
    domain: "partner",
    icon: UsersIcon,
    requiredPermission: "partnerMember.view",
  },
]

export const mobileNavItems: RemoteHelpdeskNavItem[] = [
  {
    key: "entry",
    titleKey: "remoteNav.mobile.entry",
    href: "/c/[serviceCode]",
    domain: "mobile",
    icon: ScanLineIcon,
  },
  {
    key: "login",
    titleKey: "remoteNav.mobile.login",
    href: "/c/[serviceCode]?state=login",
    domain: "mobile",
    icon: BadgeCheckIcon,
  },
  {
    key: "scan",
    titleKey: "remoteNav.mobile.scan",
    href: "/c/[serviceCode]?state=scan",
    domain: "mobile",
    icon: QrCodeIcon,
  },
  {
    key: "chat",
    titleKey: "remoteNav.mobile.chat",
    href: "/c/[serviceCode]?state=chat",
    domain: "mobile",
    icon: MessageSquareTextIcon,
  },
  {
    key: "voice",
    titleKey: "remoteNav.mobile.voice",
    href: "/c/[serviceCode]?state=voice",
    domain: "mobile",
    icon: HeadphonesIcon,
  },
  {
    key: "diagnosis",
    titleKey: "remoteNav.mobile.diagnosis",
    href: "/c/[serviceCode]?state=diagnosis",
    domain: "mobile",
    icon: CpuIcon,
  },
  {
    key: "guide",
    titleKey: "remoteNav.mobile.guide",
    href: "/c/[serviceCode]?state=guide",
    domain: "mobile",
    icon: WrenchIcon,
  },
  {
    key: "tickets",
    titleKey: "remoteNav.mobile.tickets",
    href: "/c/[serviceCode]?state=tickets",
    domain: "mobile",
    icon: TicketCheckIcon,
  },
  {
    key: "history",
    titleKey: "remoteNav.mobile.history",
    href: "/c/[serviceCode]?state=history",
    domain: "mobile",
    icon: HistoryIcon,
  },
  {
    key: "meeting",
    titleKey: "remoteNav.mobile.meeting",
    href: "/c/[serviceCode]?state=ticketVideo",
    domain: "mobile",
    icon: VideoIcon,
  },
  {
    key: "devices",
    titleKey: "remoteNav.mobile.devices",
    href: "/c/[serviceCode]?state=devices",
    domain: "mobile",
    icon: DatabaseZapIcon,
  },
  {
    key: "my",
    titleKey: "remoteNav.mobile.my",
    href: "/c/[serviceCode]?state=my",
    domain: "mobile",
    icon: CircleUserRoundIcon,
  },
]

export const remoteHelpdeskRouteGroups: RemoteHelpdeskRouteGroup[] = [
  {
    domain: "platform",
    titleKey: "remoteGroup.platform",
    basePath: "/platform",
    icon: ShieldCheckIcon,
    items: platformNavItems,
  },
  {
    domain: "enterprise",
    titleKey: "remoteGroup.enterprise",
    basePath: "/enterprise",
    icon: HeadphonesIcon,
    items: enterpriseNavItems,
  },
  {
    domain: "partner",
    titleKey: "remoteGroup.partner",
    basePath: "/partner",
    icon: LandmarkIcon,
    items: partnerNavItems,
  },
  {
    domain: "customer",
    titleKey: "remoteGroup.customer",
    basePath: "/customer",
    icon: LifeBuoyIcon,
    items: customerNavItems,
  },
  {
    domain: "mobile",
    titleKey: "remoteGroup.mobile",
    basePath: "/c/[serviceCode]",
    icon: ScanLineIcon,
    items: mobileNavItems,
  },
]

function normalizePath(path: string) {
  const [pathname] = path.split("?")
  return pathname !== "/" ? pathname.replace(/\/+$/, "") : pathname
}

function isMobileServicePath(pathname: string, targetPath: string) {
  return targetPath === "/c/[serviceCode]" && /^\/c\/[^/]+/.test(pathname)
}

export function isRemoteHelpdeskNavItemActive(
  pathname: string | null | undefined,
  item: Pick<RemoteHelpdeskNavItem, "href" | "aliases">
) {
  if (!pathname) {
    return false
  }

  const currentPath = normalizePath(pathname)
  const targets = [item.href, ...(item.aliases ?? [])].map(normalizePath)

  return targets.some((targetPath) => {
    if (isMobileServicePath(currentPath, targetPath)) {
      return true
    }
    if (
      targetPath === "/platform" ||
      targetPath === "/enterprise" ||
      targetPath === "/partner" ||
      targetPath === "/customer"
    ) {
      return currentPath === targetPath
    }
    return currentPath === targetPath || currentPath.startsWith(`${targetPath}/`)
  })
}

export function getRemoteHelpdeskRouteGroup(domain: RemoteHelpdeskDomain) {
  return remoteHelpdeskRouteGroups.find((group) => group.domain === domain)
}

export function getRemoteHelpdeskNavItems(domain: RemoteHelpdeskDomain) {
  return getRemoteHelpdeskRouteGroup(domain)?.items ?? []
}

export function getRemoteHelpdeskDomainForPathname(
  pathname: string | null | undefined
): RemoteHelpdeskDomain | null {
  if (!pathname) {
    return null
  }
  const currentPath = normalizePath(pathname)
  if (currentPath.startsWith("/platform")) {
    return "platform"
  }
  if (currentPath.startsWith("/enterprise")) {
    return "enterprise"
  }
  if (currentPath.startsWith("/partner")) {
    return "partner"
  }
  if (currentPath.startsWith("/customer")) {
    return "customer"
  }
  if (/^\/c\/[^/]+/.test(currentPath)) {
    return "mobile"
  }
  return null
}

const remoteHelpdeskBreadcrumbOverrides: Array<{
  childrenOnly?: boolean
  exact?: boolean
  match: string
  keys: string[]
}> = [
  {
    exact: true,
    match: "/enterprise/workbench/tasks",
    keys: ["remoteNav.enterprise.workbench", "enterpriseWorkbench.views.tasks"],
  },
  {
    exact: true,
    match: "/enterprise/workbench/overview",
    keys: ["remoteNav.enterprise.workbench", "enterpriseWorkbench.views.overview"],
  },
  {
    exact: true,
    match: "/enterprise/workbench/insights",
    keys: ["remoteNav.enterprise.workbench", "enterpriseWorkbench.views.insights"],
  },
  {
    exact: true,
    match: "/enterprise/workbench/resources",
    keys: ["remoteNav.enterprise.workbench", "enterpriseWorkbench.views.resources"],
  },
  {
    childrenOnly: true,
    match: "/enterprise/ai/",
    keys: ["remoteNav.enterprise.ai", "remoteBreadcrumb.enterprise.agentDetail"],
  },
  {
    match: "/enterprise/meeting-room",
    keys: ["remoteNav.enterprise.video", "remoteBreadcrumb.enterprise.meetingRoom"],
  },
  {
    exact: true,
    match: "/enterprise/models",
    keys: ["remoteNav.enterprise.models"],
  },
  {
    match: "/enterprise/org/members",
    keys: ["remoteNav.enterprise.org", "remoteBreadcrumb.enterprise.members"],
  },
  {
    match: "/enterprise/org/schedules",
    keys: ["remoteNav.enterprise.org", "remoteBreadcrumb.enterprise.schedules"],
  },
  {
    childrenOnly: true,
    match: "/enterprise/products/",
    keys: ["remoteNav.enterprise.products", "remoteBreadcrumb.enterprise.productDetail"],
  },
  {
    match: "/enterprise/service-code",
    keys: ["remoteNav.enterprise.service-codes"],
  },
  {
    match: "/enterprise/service-codes",
    keys: ["remoteNav.enterprise.service-codes"],
  },
  {
    childrenOnly: true,
    match: "/enterprise/tickets/",
    keys: ["remoteNav.enterprise.tickets", "remoteBreadcrumb.enterprise.ticketDetail"],
  },
  {
    childrenOnly: true,
    match: "/enterprise/workflow/",
    keys: ["remoteNav.enterprise.workflow", "remoteBreadcrumb.enterprise.workflowDetail"],
  },
]

function getNormalizedPathMatchLength(currentPath: string, targetPath: string) {
  if (isMobileServicePath(currentPath, targetPath)) {
    return targetPath.length
  }
  if (
    targetPath === "/platform" ||
    targetPath === "/enterprise" ||
    targetPath === "/partner" ||
    targetPath === "/customer"
  ) {
    return currentPath === targetPath ? targetPath.length : 0
  }
  return currentPath === targetPath || currentPath.startsWith(`${targetPath}/`)
    ? targetPath.length
    : 0
}

function getRemoteHelpdeskNavItemMatchLength(
  pathname: string,
  item: Pick<RemoteHelpdeskNavItem, "href" | "aliases">
) {
  const currentPath = normalizePath(pathname)
  const targets = [item.href, ...(item.aliases ?? [])].map(normalizePath)
  return targets.reduce(
    (longest, targetPath) => Math.max(longest, getNormalizedPathMatchLength(currentPath, targetPath)),
    0
  )
}

export function getRemoteHelpdeskNavItemForPathname(
  pathname: string | null | undefined
) {
  if (!pathname) {
    return null
  }

  let matchedItem: RemoteHelpdeskNavItem | null = null
  let longestMatch = 0

  for (const group of remoteHelpdeskRouteGroups) {
    for (const item of group.items) {
      const matchLength = getRemoteHelpdeskNavItemMatchLength(pathname, item)
      if (matchLength >= longestMatch && matchLength > 0) {
        matchedItem = item
        longestMatch = matchLength
      }
    }
  }

  return matchedItem
}

export function getRemoteHelpdeskBreadcrumbKeys(pathname: string | null | undefined) {
  if (!pathname) {
    return []
  }

  const currentPath = normalizePath(pathname)
  const override = remoteHelpdeskBreadcrumbOverrides.find(({ childrenOnly, exact, match }) => {
    const targetPath = normalizePath(match)
    if (childrenOnly) {
      return currentPath.startsWith(`${targetPath}/`)
    }
    if (exact) {
      return currentPath === targetPath
    }
    return currentPath === targetPath || currentPath.startsWith(`${targetPath}/`)
  })
  if (override) {
    return [...override.keys]
  }

  const item = getRemoteHelpdeskNavItemForPathname(currentPath)
  if (item) {
    return [item.titleKey]
  }

  const domain = getRemoteHelpdeskDomainForPathname(currentPath)
  const group = domain ? getRemoteHelpdeskRouteGroup(domain) : null
  return group ? [group.titleKey] : []
}

export function getRemoteHelpdeskPageTitle(pathname: string | null | undefined) {
  const breadcrumbKeys = getRemoteHelpdeskBreadcrumbKeys(pathname)
  if (breadcrumbKeys.length > 1) {
    return breadcrumbKeys[breadcrumbKeys.length - 1]
  }

  const item = getRemoteHelpdeskNavItemForPathname(pathname)
  if (item) {
    return item.titleKey
  }
  const domain = getRemoteHelpdeskDomainForPathname(pathname)
  return domain ? getRemoteHelpdeskRouteGroup(domain)?.titleKey ?? "remoteShell.brand" : "remoteShell.brand"
}

export function filterRemoteHelpdeskNavForPermissions(
  items: readonly RemoteHelpdeskNavItem[],
  permissions: readonly string[] | undefined,
  featureFlags?: Record<string, boolean>
) {
  return items.filter((item) => canAccessRemoteHelpdeskNavItem(item, permissions, featureFlags))
}

export function canAccessRemoteHelpdeskNavItem(
  item: Pick<RemoteHelpdeskNavItem, "requiredPermission" | "requiredFeature"> | undefined,
  permissions: readonly string[] | undefined,
  featureFlags?: Record<string, boolean>
) {
  if (!item) {
    return true
  }
  if (!hasRemoteHelpdeskFeature(item.requiredFeature, featureFlags)) {
    return false
  }
  if (!permissions || !item.requiredPermission) {
    return true
  }
  const permissionSet = new Set(permissions)
  return typeof item.requiredPermission === "string"
    ? permissionSet.has(item.requiredPermission)
    : item.requiredPermission.some((permission) => permissionSet.has(permission))
}

function hasRemoteHelpdeskFeature(
  required: string | readonly string[] | undefined,
  featureFlags?: Record<string, boolean>
) {
  if (!required || !featureFlags) {
    return true
  }
  return typeof required === "string"
    ? featureFlags[required] !== false
    : required.some((feature) => featureFlags[feature] !== false)
}

export type RemoteHelpdeskSidebarSection = {
  labelKey: string
  icon?: LucideIcon
  items: RemoteHelpdeskNavItem[]
  /** 平铺渲染：不包一层分组菜单，子项直接作为一级菜单项（如客户域） */
  flat?: boolean
}

export type RemoteHelpdeskPermissionCategory = {
  key: string
  labelKey: string
  groupKeys: string[]
}

export function getEnterpriseSidebarSections(): RemoteHelpdeskSidebarSection[] {
  const items = new Map(enterpriseNavItems.map((item) => [item.key, item]))
  const sections: RemoteHelpdeskSidebarSection[] = [
    {
      labelKey: "remoteSidebar.workbench",
      icon: SquareKanbanIcon,
      items: ["workbench", "tickets", "ticket-workbench", "video", "reports", "notifications"]
        .map((k) => items.get(k))
        .filter(Boolean) as RemoteHelpdeskNavItem[],
    },
    {
      labelKey: "remoteSidebar.productCenter",
      icon: PackageIcon,
      items: ["products", "devices"]
        .map((k) => items.get(k))
        .filter(Boolean) as RemoteHelpdeskNavItem[],
    },
    {
      labelKey: "remoteSidebar.aiData",
      icon: CpuIcon,
      items: ["models", "knowledge", "ai", "usage"]
        .map((k) => items.get(k))
        .filter(Boolean) as RemoteHelpdeskNavItem[],
    },
    {
      labelKey: "remoteSidebar.workflowAccess",
      icon: WorkflowIcon,
      items: ["workflow"]
        .map((k) => items.get(k))
        .filter(Boolean) as RemoteHelpdeskNavItem[],
    },
    {
      labelKey: "remoteSidebar.management",
      icon: UsersIcon,
      items: ["server-console", "people", "customer-users", "partners", "org", "permissions", "audit"]
        .map((k) => items.get(k))
        .filter(Boolean) as RemoteHelpdeskNavItem[],
    },
  ]
  return sections
}

export function getPlatformSidebarSections(): RemoteHelpdeskSidebarSection[] {
  const items = new Map(platformNavItems.map((item) => [item.key, item]))
  const sections: RemoteHelpdeskSidebarSection[] = [
    {
      labelKey: "remoteSidebar.operations",
      icon: ChartNoAxesCombinedIcon,
      items: ["overview", "usage"]
        .map((k) => items.get(k))
        .filter(Boolean) as RemoteHelpdeskNavItem[],
    },
    {
      labelKey: "remoteSidebar.aiData",
      icon: CpuIcon,
      items: ["models", "access-governance"]
        .map((k) => items.get(k))
        .filter(Boolean) as RemoteHelpdeskNavItem[],
    },
    {
      labelKey: "remoteSidebar.system",
      icon: MonitorCogIcon,
      items: [
        "ops",
        "system-config",
        // "globalization",
      ]
        .map((k) => items.get(k))
        .filter(Boolean) as RemoteHelpdeskNavItem[],
    },
    {
      labelKey: "remoteSidebar.management",
      icon: UsersIcon,
      items: ["staff", "tenants", "permissions", "audit"]
        .map((k) => items.get(k))
        .filter(Boolean) as RemoteHelpdeskNavItem[],
    },
  ]
  return sections
}

const platformPermissionGroupsByNavKey: Record<string, readonly string[]> = {
  usage: ["report", "productAIUsageCredential", "finance"],
  models: ["aiConfig", "finance"],
  "access-governance": ["tenantIntegrationConfig"],
  // globalization: ["privacy", "privacyRequest", "dataBreach", "dataRetention"],
  staff: ["user"],
  tenants: ["tenant"],
  permissions: ["role", "permission"],
  audit: ["session", "platform"],
  "system-config": ["systemIntro"],
}

const enterprisePermissionGroupsByNavKey: Record<string, readonly string[]> = {
  workbench: ["ticket"],
  "ticket-workbench": ["conversation"],
  video: ["meeting"],
  reports: ["report"],
  notifications: ["notification", "channel", "quickReply", "tag"],
  products: [
    "product",
    "productModel",
    "productServiceProfile",
    "productKnowledgeBinding",
    "serviceCode",
    "serviceCodeBatch",
  ],
  devices: ["device"],
  knowledge: ["knowledgeBase", "knowledgeDocument", "knowledgeFAQ", "asset"],
  ai: ["aiAgent", "aiAgentRelease", "aiConfig", "skillDefinition", "mcp"],
  usage: ["productAIUsageCredential"],
  workflow: ["aiWorkflow"],
  people: ["user", "agent", "agentTeam", "agentTeamSchedule"],
  "customer-users": ["customer", "customerMember"],
  partners: ["partnerMember"],
  org: ["company"],
  permissions: ["role", "permission"],
  audit: ["session"],
}

export function getRemoteHelpdeskPermissionCategories(
  domain: "platform" | "enterprise"
): RemoteHelpdeskPermissionCategory[] {
  const sections = domain === "platform"
    ? getPlatformSidebarSections()
    : getEnterpriseSidebarSections()
  const groupsByNavKey = domain === "platform"
    ? platformPermissionGroupsByNavKey
    : enterprisePermissionGroupsByNavKey
  const seen = new Set<string>()

  return sections
    .map((section) => ({
      key: section.labelKey,
      labelKey: section.labelKey,
      groupKeys: section.items.flatMap((item) =>
        (groupsByNavKey[item.key] ?? []).filter((group) => {
          if (seen.has(group)) return false
          seen.add(group)
          return true
        })
      ),
    }))
    .filter((category) => category.groupKeys.length > 0)
}

export function getRemoteHelpdeskPermissionGroupOrder(
  domain: "platform" | "enterprise"
) {
  return getRemoteHelpdeskPermissionCategories(domain)
    .flatMap((category) => category.groupKeys)
}

export function getCustomerSidebarSections(): RemoteHelpdeskSidebarSection[] {
  const items = new Map(customerNavItems.map((item) => [item.key, item]))
  return [
    {
      flat: true,
      labelKey: "remoteSidebar.service",
      icon: LifeBuoyIcon,
      items: ["chat", "tickets", "meeting", "devices", "my"]
        .map((key) => items.get(key))
        .filter(Boolean) as RemoteHelpdeskNavItem[],
    },
  ]
}

export function getPartnerSidebarSections(): RemoteHelpdeskSidebarSection[] {
  return [
    {
      labelKey: "remoteSidebar.supplierCollaboration",
      icon: LandmarkIcon,
      items: [...partnerNavItems],
    },
  ]
}
