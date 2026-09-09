import {
  Building2Icon,
  CalendarClockIcon,
  CpuIcon,
  FileTextIcon,
  GlobeIcon,
  KeyRoundIcon,
  LayoutDashboardIcon,
  MessageSquareCodeIcon,
  MessageSquareMoreIcon,
  MessageSquareTextIcon,
  PackageIcon,
  ShieldCheckIcon,
  TagsIcon,
  UserCogIcon,
  UsersIcon,
  VideoIcon,
  WorkflowIcon,
} from "lucide-react";
import type { ReactNode } from "react";

/** Keep in sync with backend internal/pkg/constants/auth.go RoleCodeSuperAdmin. */
export const DASHBOARD_ROLE_SUPER_ADMIN = "super_admin";

export type DashboardNavMenuItem = {
  title: string;
  titleKey: string;
  url: string;
  icon: ReactNode;
};

export type DashboardNavItemConfig = Omit<DashboardNavMenuItem, "title"> & {
  /**
   * Keep in sync with backend Permission.Code. Missing value means any signed-in
   * admin can see the module.
   */
  requiredPermission?: string;
  /** P0 / P1 tier marking for visual differentiation in the sidebar */
  tier?: "p0" | "p1";
};

export type DashboardNavSectionConfig = {
  titleKey: string;
  icon: ReactNode;
  items: DashboardNavItemConfig[];
};

function navItemVisible(
  item: DashboardNavItemConfig,
  superAdmin: boolean,
  permissionSet: Set<string>,
): boolean {
  if (superAdmin) {
    return true;
  }
  if (!item.requiredPermission) {
    return true;
  }
  return permissionSet.has(item.requiredPermission);
}

export function filterDashboardNavForSession(
  permissions: readonly string[] | undefined,
  roles: readonly string[] | undefined,
): { titleKey: string; icon: ReactNode; items: DashboardNavMenuItem[] }[] {
  const superAdmin = roles?.includes(DASHBOARD_ROLE_SUPER_ADMIN) ?? false;
  const permissionSet = new Set(permissions ?? []);
  return dashboardNavSections
    .map((section) => ({
      titleKey: section.titleKey,
      icon: section.icon,
      items: section.items
        .filter((item) => navItemVisible(item, superAdmin, permissionSet))
        .map(({ titleKey, url, icon }) => ({ title: titleKey, titleKey, url, icon })),
    }))
    .filter((section) => section.items.length > 0);
}

export function filterDashboardSecondaryNavForSession(
  permissions: readonly string[] | undefined,
  roles: readonly string[] | undefined,
): DashboardNavMenuItem[] {
  const superAdmin = roles?.includes(DASHBOARD_ROLE_SUPER_ADMIN) ?? false;
  const permissionSet = new Set(permissions ?? []);
  return dashboardSecondaryNav
    .filter((item) => navItemVisible(item, superAdmin, permissionSet))
    .map(({ titleKey, url, icon }) => ({ title: titleKey, titleKey, url, icon }));
}

export const dashboardNavSections: DashboardNavSectionConfig[] = [
  // {
  //   title: "Overview",
  //   items: [
  //     {
  //       title: "Overview",
  //       url: "/",
  //       icon: <LayoutDashboardIcon />,
  //     },
  //   ],
  // },
  {
    titleKey: "nav.receptionCenter",
    icon: <MessageSquareTextIcon />,
    items: [
      {
        titleKey: "nav.overview",
        url: "/dashboard",
        icon: <LayoutDashboardIcon />,
      },
      {
        titleKey: "nav.conversations",
        url: "/dashboard/conversations",
        icon: <MessageSquareTextIcon />,
        requiredPermission: "conversation.view",
      },
      {
        titleKey: "nav.tickets",
        url: "/dashboard/tickets",
        icon: <FileTextIcon />,
        requiredPermission: "ticket.view",
      },
      {
        titleKey: "nav.conversationMonitor",
        url: "/dashboard/conversation-monitor",
        icon: <MessageSquareTextIcon />,
        requiredPermission: "conversation.view",
      },
      {
        titleKey: "nav.customers",
        url: "/dashboard/customers",
        icon: <UsersIcon />,
        requiredPermission: "customer.view",
      },
      {
        titleKey: "nav.companies",
        url: "/dashboard/companies",
        icon: <Building2Icon />,
        requiredPermission: "company.view",
      },
      {
        titleKey: "nav.productCenter",
        url: "/dashboard/products",
        icon: <PackageIcon />,
        requiredPermission: "product.view",
      },
    ],
  },
  {
    titleKey: "nav.agentConfig",
    icon: <UserCogIcon />,
    items: [
      {
        titleKey: "nav.tags",
        url: "/dashboard/tags",
        icon: <TagsIcon />,
        requiredPermission: "tag.view",
      },
      {
        titleKey: "nav.quickReplies",
        url: "/dashboard/quick-replies",
        icon: <MessageSquareMoreIcon />,
        requiredPermission: "quickReply.view",
      },
      {
        titleKey: "nav.agents",
        url: "/dashboard/agents",
        icon: <UserCogIcon />,
        requiredPermission: "agent.view",
      },
      {
        titleKey: "nav.agentTeamSchedules",
        url: "/dashboard/agent-team-schedules",
        icon: <CalendarClockIcon />,
        requiredPermission: "agentTeamSchedule.view",
      },
      {
        titleKey: "nav.channels",
        url: "/dashboard/channels",
        icon: <GlobeIcon />,
        requiredPermission: "channel.view",
      },
    ],
  },
  // Shared model-capability tools remain available for dashboard administration.
  {
    titleKey: "nav.aiCapabilities",
    icon: <WorkflowIcon />,
    items: [
      {
        titleKey: "nav.knowledge",
        url: "/dashboard/knowledge",
        icon: <FileTextIcon />,
        requiredPermission: "knowledgeBase.view",
      },
      {
        titleKey: "nav.aiConfigs",
        url: "/dashboard/ai-configs",
        icon: <CpuIcon />,
        requiredPermission: "aiConfig.view",
      },
      {
        titleKey: "nav.aiAgents",
        url: "/enterprise/ai",
        icon: <MessageSquareTextIcon />,
        requiredPermission: "aiAgent.view",
      },
      {
        titleKey: "nav.skillDefinition",
        url: "/dashboard/skill-definition",
        icon: <MessageSquareCodeIcon />,
        requiredPermission: "skillDefinition.view",
      },
      {
        titleKey: "nav.mcp",
        url: "/dashboard/mcp",
        icon: <MessageSquareCodeIcon />,
        requiredPermission: "mcp.view",
      },
      {
        titleKey: "nav.workflowRuns",
        url: "/dashboard/ai-workflow-runs",
        icon: <WorkflowIcon />,
        requiredPermission: "aiAgent.view",
      },
    ],
  },
  {
    titleKey: "nav.system",
    icon: <ShieldCheckIcon />,
    items: [
      {
        titleKey: "nav.users",
        url: "/dashboard/users",
        icon: <UsersIcon />,
        requiredPermission: "user.view",
      },
      {
        titleKey: "nav.roles",
        url: "/dashboard/roles",
        icon: <ShieldCheckIcon />,
        requiredPermission: "role.view",
      },
      {
        titleKey: "nav.permissions",
        url: "/dashboard/permissions",
        icon: <KeyRoundIcon />,
        requiredPermission: "permission.view",
      },
    ],
  },
];

export const dashboardSecondaryNav: DashboardNavItemConfig[] = [
  // {
  //   title: "System Settings",
  //   url: "/settings",
  //   icon: <Settings2Icon />,
  // },
  // {
  //   title: "Help Center",
  //   url: "/help",
  //   icon: <LifeBuoyIcon />,
  // },
];

export const afterSalesNavSections: DashboardNavSectionConfig[] = [
  {
    titleKey: "nav.afterSalesReception",
    icon: <MessageSquareTextIcon />,
    items: [
      {
        titleKey: "nav.workbench",
        url: "/ticket-workbench",
        icon: <MessageSquareMoreIcon />,
        tier: "p0",
      },
      {
        titleKey: "nav.conversations",
        url: "/dashboard/conversations",
        icon: <MessageSquareTextIcon />,
        requiredPermission: "conversation.view",
        tier: "p0",
      },
      {
        titleKey: "nav.tickets",
        url: "/tickets",
        icon: <FileTextIcon />,
        tier: "p0",
      },
    ],
  },
  {
    titleKey: "nav.afterSalesAssets",
    icon: <PackageIcon />,
    items: [
      {
        titleKey: "nav.products",
        url: "/products",
        icon: <PackageIcon />,
        tier: "p0",
      },
      {
        titleKey: "nav.devices",
        url: "/dashboard/products?tab=devices",
        icon: <FileTextIcon />,
        requiredPermission: "product.view",
        tier: "p0",
      },
      {
        titleKey: "nav.serviceCodes",
        url: "/dashboard/products?tab=serviceCodes",
        icon: <FileTextIcon />,
        requiredPermission: "product.view",
        tier: "p0",
      },
    ],
  },
  {
    titleKey: "nav.afterSalesKnowledge",
    icon: <FileTextIcon />,
    items: [
      {
        titleKey: "nav.knowledge",
        url: "/dashboard/knowledge",
        icon: <FileTextIcon />,
        requiredPermission: "knowledgeBase.view",
        tier: "p0",
      },
    ],
  },
  {
    titleKey: "nav.afterSalesCollaboration",
    icon: <VideoIcon />,
    items: [
      {
        titleKey: "nav.meetings",
        url: "/dashboard/ai-workflow-runs",
        icon: <VideoIcon />,
        tier: "p0",
      },
    ],
  },
  {
    titleKey: "nav.afterSalesReports",
    icon: <FileTextIcon />,
    items: [
      {
        titleKey: "nav.reports",
        url: "/dashboard",
        icon: <FileTextIcon />,
        tier: "p0",
      },
      {
        titleKey: "nav.usage",
        url: "/platform/usage",
        icon: <FileTextIcon />,
        tier: "p1",
      },
    ],
  },
  {
    titleKey: "nav.afterSalesSystem",
    icon: <ShieldCheckIcon />,
    items: [
      {
        titleKey: "nav.settings",
        url: "/enterprise/org",
        icon: <FileTextIcon />,
        tier: "p0",
      },
    ],
  },
];

export const dashboardQuickActions = [
  {
    title: "View Conversations",
    icon: <MessageSquareTextIcon />,
  },
  {
    title: "Invite Members",
    icon: <UserCogIcon />,
  },
  {
    title: "Configure Reception",
    icon: <MessageSquareCodeIcon />,
  },
] as const;

export function getPageTitle(pathname: string): string {
  return getPageTitleKey(pathname);
}

function pathMatchesNavItem(pathname: string, itemUrl: string): boolean {
  const [targetPath] = itemUrl.split("?");
  return pathname === targetPath || pathname.startsWith(targetPath + "/");
}

export function getPageBreadcrumbKeys(pathname: string): string[] {
  let matchedCrumbs = ["nav.dashboardHome"];
  let longestMatch = 0;

  for (const section of [...dashboardNavSections, ...afterSalesNavSections]) {
    for (const item of section.items) {
      if (pathMatchesNavItem(pathname, item.url)) {
        const [targetPath] = item.url.split("?");
        const matchLength = targetPath.length;
        if (matchLength > longestMatch) {
          longestMatch = matchLength;
          matchedCrumbs = [item.titleKey];
        }
      }
    }
  }

  for (const item of dashboardSecondaryNav) {
    if (pathMatchesNavItem(pathname, item.url)) {
      const [targetPath] = item.url.split("?");
      const matchLength = targetPath.length;
      if (matchLength > longestMatch) {
        longestMatch = matchLength;
        matchedCrumbs = [item.titleKey];
      }
    }
  }

  return matchedCrumbs;
}

export function getPageTitleKey(pathname: string): string {
  return getPageBreadcrumbKeys(pathname).at(-1) ?? "nav.dashboardHome";
}
