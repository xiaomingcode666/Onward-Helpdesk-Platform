import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import ts from "typescript"
import vm from "node:vm"

async function loadNavigationModule() {
  const source = await readFile(
    new URL("./navigation-remote-helpdesk.tsx", import.meta.url),
    "utf8"
  )
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
      jsx: ts.JsxEmit.ReactJSX,
    },
    fileName: "navigation-remote-helpdesk.tsx",
  })
  const iconModule = new Proxy(
    {},
    {
      get:
        (_target, property) =>
        function IconStub() {
          return property
        },
    }
  )
  const sandbox = {
    exports: {},
    module: { exports: {} },
    require: (specifier) => {
      if (specifier === "lucide-react") {
        return iconModule
      }
      if (specifier === "react/jsx-runtime") {
        return {
          jsx: () => ({}),
          jsxs: () => ({}),
          Fragment: "Fragment",
        }
      }
      throw new Error(`Unexpected import: ${specifier}`)
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("remote helpdesk navigation exposes platform, enterprise, partner, customer, and mobile route groups", async () => {
  const { remoteHelpdeskRouteGroups } = await loadNavigationModule()

  assert.deepEqual(
    Array.from(remoteHelpdeskRouteGroups, (group) => group.domain),
    ["platform", "enterprise", "partner", "customer", "mobile"]
  )

  assert.equal(remoteHelpdeskRouteGroups.find((group) => group.domain === "platform").basePath, "/platform")
  assert.equal(remoteHelpdeskRouteGroups.find((group) => group.domain === "enterprise").basePath, "/enterprise")
  assert.equal(remoteHelpdeskRouteGroups.find((group) => group.domain === "partner").basePath, "/partner")
  assert.equal(remoteHelpdeskRouteGroups.find((group) => group.domain === "customer").basePath, "/customer")
  assert.equal(remoteHelpdeskRouteGroups.find((group) => group.domain === "mobile").basePath, "/c/[serviceCode]")
})

test("enterprise navigation keeps the P0 prototype IA entries reachable", async () => {
  const { enterpriseNavItems } = await loadNavigationModule()

  const enterpriseSlugs = Array.from(enterpriseNavItems, (item) => item.key)

  assert.deepEqual(
    enterpriseSlugs.filter((key) =>
      [
        "products",
        "tickets",
        "knowledge",
        "video",
        "usage",
        "access",
        "reports",
        "people",
        "partners",
        "audit",
      ].includes(key)
    ),
    [
      "tickets",
      "products",
      "knowledge",
      "video",
      "access",
      "reports",
      "people",
      "partners",
      "audit",
      "usage",
    ]
  )

  assert.equal(
    enterpriseNavItems.find((item) => item.key === "products").href,
    "/enterprise/products"
  )
  assert.equal(
    enterpriseNavItems.find((item) => item.key === "tickets").href,
    "/enterprise/tickets"
  )
  assert.equal(
    enterpriseNavItems.find((item) => item.key === "knowledge").href,
    "/enterprise/knowledge"
  )
})

test("enterprise sidebar sections match the prototype menu hierarchy", async () => {
  const { getEnterpriseSidebarSections } = await loadNavigationModule()
  const actualSections = JSON.parse(
    JSON.stringify(
      getEnterpriseSidebarSections().map((section) => ({
        labelKey: section.labelKey,
        items: section.items.map((item) => item.key),
      }))
    )
  )

  assert.deepEqual(
    actualSections,
    [
      {
        labelKey: "remoteSidebar.workbench",
        items: [
          "workbench",
          "tickets",
          "ticket-workbench",
          "video",
          "reports",
          "notifications",
        ],
      },
      {
        labelKey: "remoteSidebar.productCenter",
        items: ["products", "devices"],
      },
      {
        labelKey: "remoteSidebar.aiData",
        items: ["knowledge", "ai", "usage"],
      },
      {
        labelKey: "remoteSidebar.workflowAccess",
        items: ["workflow"],
      },
      {
        labelKey: "remoteSidebar.management",
        items: [
          "server-console",
          "people",
          "customer-users",
          "partners",
          "org",
          "permissions",
          "audit",
        ],
      },
    ]
  )
})

test("enterprise AI-disabled tenants cannot see AI-only navigation", async () => {
  const {
    enterpriseNavItems,
    filterRemoteHelpdeskNavForPermissions,
  } = await loadNavigationModule()

  const visibleKeys = filterRemoteHelpdeskNavForPermissions(
    enterpriseNavItems,
    undefined,
    { ai: false, product: true, device: true, deviceDiagnosis: false }
  ).map((item) => item.key)

  assert.equal(visibleKeys.includes("knowledge"), false)
  assert.equal(visibleKeys.includes("diagnosis"), false)
  assert.equal(visibleKeys.includes("ai"), false)
  assert.equal(visibleKeys.includes("usage"), false)
  assert.equal(visibleKeys.includes("workflow"), false)
  assert.equal(visibleKeys.includes("tickets"), true)
  assert.equal(visibleKeys.includes("products"), true)
})

test("knowledge-support tenants do not expose equipment navigation", async () => {
  const {
    customerNavItems,
    enterpriseNavItems,
    filterRemoteHelpdeskNavForPermissions,
  } = await loadNavigationModule()

  const featureFlags = {
    ai: true,
    knowledgeSupport: true,
    product: false,
    device: false,
    serviceCode: false,
    deviceDiagnosis: false,
  }
  const enterpriseKeys = filterRemoteHelpdeskNavForPermissions(
    enterpriseNavItems,
    undefined,
    featureFlags
  ).map((item) => item.key)
  const customerKeys = filterRemoteHelpdeskNavForPermissions(
    customerNavItems,
    undefined,
    featureFlags
  ).map((item) => item.key)

  assert.equal(enterpriseKeys.includes("products"), false)
  assert.equal(enterpriseKeys.includes("devices"), false)
  assert.equal(enterpriseKeys.includes("diagnosis"), false)
  assert.equal(enterpriseKeys.includes("knowledge"), true)
  assert.equal(enterpriseKeys.includes("workflow"), true)
  assert.equal(customerKeys.includes("devices"), false)
  assert.equal(customerKeys.includes("chat"), true)
})

test("enterprise workbench split pages stay grouped under the workbench nav item", async () => {
  const {
    enterpriseNavItems,
    getRemoteHelpdeskNavItemForPathname,
    isRemoteHelpdeskNavItemActive,
  } = await loadNavigationModule()
  const workbenchItem = enterpriseNavItems.find((item) => item.key === "workbench")
  const splitPaths = [
    "/enterprise/workbench",
    "/enterprise/workbench/tasks",
    "/enterprise/workbench/overview",
    "/enterprise/workbench/insights",
    "/enterprise/workbench/resources",
  ]

  assert.ok(workbenchItem)
  assert.deepEqual(JSON.parse(JSON.stringify(workbenchItem.aliases)), splitPaths)

  for (const path of splitPaths) {
    assert.equal(getRemoteHelpdeskNavItemForPathname(path)?.key, "workbench")
    assert.equal(isRemoteHelpdeskNavItemActive(path, workbenchItem), true)
  }
})

test("enterprise breadcrumbs use the shared domain and route contract", async () => {
  const { getRemoteHelpdeskBreadcrumbKeys, getRemoteHelpdeskPageTitle } = await loadNavigationModule()

  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise"))),
    ["remoteNav.enterprise.workbench"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/workbench/tasks"))),
    ["remoteNav.enterprise.workbench", "enterpriseWorkbench.views.tasks"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/workbench/overview"))),
    ["remoteNav.enterprise.workbench", "enterpriseWorkbench.views.overview"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/workbench/insights"))),
    ["remoteNav.enterprise.workbench", "enterpriseWorkbench.views.insights"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/workbench/resources"))),
    ["remoteNav.enterprise.workbench", "enterpriseWorkbench.views.resources"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/ticket-workbench?ticket_id=42"))),
    ["remoteNav.enterprise.ticket-workbench"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/tickets"))),
    ["remoteNav.enterprise.tickets"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/service-codes"))),
    ["remoteNav.enterprise.service-codes"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/models"))),
    ["remoteNav.enterprise.models"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/video"))),
    ["remoteNav.enterprise.video"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/ai/18?view=release"))),
    ["remoteNav.enterprise.ai", "remoteBreadcrumb.enterprise.agentDetail"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/org/members?teamId=7"))),
    ["remoteNav.enterprise.org", "remoteBreadcrumb.enterprise.members"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/products/42"))),
    ["remoteNav.enterprise.products", "remoteBreadcrumb.enterprise.productDetail"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/tickets/42"))),
    ["remoteNav.enterprise.tickets", "remoteBreadcrumb.enterprise.ticketDetail"]
  )
  assert.deepEqual(
    JSON.parse(JSON.stringify(getRemoteHelpdeskBreadcrumbKeys("/enterprise/workflow/9"))),
    ["remoteNav.enterprise.workflow", "remoteBreadcrumb.enterprise.workflowDetail"]
  )
  assert.equal(getRemoteHelpdeskPageTitle("/enterprise/ticket-workbench"), "remoteNav.enterprise.ticket-workbench")
  assert.equal(getRemoteHelpdeskPageTitle("/enterprise/workbench/tasks"), "enterpriseWorkbench.views.tasks")
  assert.equal(getRemoteHelpdeskPageTitle("/enterprise/workbench/overview"), "enterpriseWorkbench.views.overview")
  assert.equal(getRemoteHelpdeskPageTitle("/enterprise/workbench/insights"), "enterpriseWorkbench.views.insights")
  assert.equal(getRemoteHelpdeskPageTitle("/enterprise/workbench/resources"), "enterpriseWorkbench.views.resources")
  assert.equal(getRemoteHelpdeskPageTitle("/enterprise/products/42"), "remoteBreadcrumb.enterprise.productDetail")
  assert.equal(getRemoteHelpdeskPageTitle("/enterprise/video"), "remoteNav.enterprise.video")
  assert.equal(getRemoteHelpdeskPageTitle("/enterprise/workflow/9"), "remoteBreadcrumb.enterprise.workflowDetail")
})

test("platform sidebar sections match the platform menu hierarchy", async () => {
  const { getPlatformSidebarSections } = await loadNavigationModule()
  const actualSections = JSON.parse(
    JSON.stringify(
      getPlatformSidebarSections().map((section) => ({
        labelKey: section.labelKey,
        items: section.items.map((item) => item.key),
      }))
    )
  )

  assert.deepEqual(
    actualSections,
    [
      {
        labelKey: "remoteSidebar.operations",
        items: ["overview", "usage"],
      },
      {
        labelKey: "remoteSidebar.aiData",
        items: ["models", "access-governance"],
      },
      {
        labelKey: "remoteSidebar.system",
        items: ["ops"],
      },
      {
        labelKey: "remoteSidebar.management",
        items: ["staff", "tenants", "permissions", "audit"],
      },
    ]
  )
})

test("permission groups follow each domain menu order", async () => {
  const {
    getRemoteHelpdeskPermissionCategories,
    getRemoteHelpdeskPermissionGroupOrder,
  } = await loadNavigationModule()

  assert.deepEqual(
    Array.from(getRemoteHelpdeskPermissionGroupOrder("platform")),
    [
      "report",
      "productAIUsageCredential",
      "aiConfig",
      "tenantIntegrationConfig",
      "user",
      "tenant",
      "role",
      "permission",
      "session",
      "platform",
    ],
  )
  assert.deepEqual(
    Array.from(getRemoteHelpdeskPermissionGroupOrder("enterprise")).slice(0, 9),
    [
      "ticket",
      "conversation",
      "meeting",
      "report",
      "notification",
      "channel",
      "quickReply",
      "tag",
      "product",
    ],
  )
  assert.deepEqual(
    Array.from(getRemoteHelpdeskPermissionCategories("platform"), (category) => ({
      labelKey: category.labelKey,
      groupKeys: Array.from(category.groupKeys),
    })),
    [
      {
        labelKey: "remoteSidebar.operations",
        groupKeys: ["report", "productAIUsageCredential"],
      },
      {
        labelKey: "remoteSidebar.aiData",
        groupKeys: ["aiConfig", "tenantIntegrationConfig"],
      },
      {
        labelKey: "remoteSidebar.management",
        groupKeys: ["user", "tenant", "role", "permission", "session", "platform"],
      },
    ],
  )
})

test("customer sidebar sections match the prototype menu hierarchy", async () => {
  const { getCustomerSidebarSections } = await loadNavigationModule()
  const actualSections = JSON.parse(
    JSON.stringify(
      getCustomerSidebarSections().map((section) => ({
        labelKey: section.labelKey,
        items: section.items.map((item) => item.key),
      }))
    )
  )

  assert.deepEqual(actualSections, [
    {
      labelKey: "remoteSidebar.service",
      items: ["chat", "tickets", "meeting", "devices", "my"],
    },
  ])
})

test("remote helpdesk navigation keeps canonical three-end route aliases stable", async () => {
  const {
    customerNavItems,
    enterpriseNavItems,
    platformNavItems,
  } = await loadNavigationModule()

  assert.equal(
    platformNavItems.find((item) => item.key === "audit").href,
    "/platform/audit"
  )
  assert.deepEqual(
    Array.from(platformNavItems.find((item) => item.key === "audit").aliases),
    ["/platform/iam-audit"]
  )

  assert.equal(
    enterpriseNavItems.find((item) => item.key === "audit").href,
    "/enterprise/audit"
  )
  assert.equal(
    enterpriseNavItems.find((item) => item.key === "devices").href,
    "/enterprise/devices"
  )
  assert.deepEqual(
    Array.from(enterpriseNavItems.find((item) => item.key === "devices").aliases),
    ["/enterprise/service-codes", "/enterprise/service-code"]
  )
  assert.equal(
    enterpriseNavItems.some((item) => item.key === "service-codes"),
    false
  )
  assert.equal(
    enterpriseNavItems.some((item) => item.key === "labels"),
    false
  )

  assert.equal(
    customerNavItems.find((item) => item.key === "devices").href,
    "/customer/devices"
  )
  assert.deepEqual(
    Array.from(customerNavItems.find((item) => item.key === "tickets").aliases),
    ["/customer/ticket"]
  )
})
