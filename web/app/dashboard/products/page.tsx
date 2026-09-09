"use client"

import { useEffect, useMemo, useRef, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import {
  BanIcon,
  BookOpenIcon,
  CheckCircle2Icon,
  ClipboardListIcon,
  CpuIcon,
  HistoryIcon,
  MessageSquareMoreIcon,
  PackageIcon,
  QrCodeIcon,
  RefreshCwIcon,
  ScanBarcodeIcon,
  Settings2Icon,
} from "lucide-react"
import { toast } from "sonner"

import { DashboardPage } from "@/components/dashboard-page"
import {
  createDashboardStatusColumn,
  createDashboardStatusToggleAction,
  DashboardCrudPage,
  type DashboardCrudFormInputValue,
} from "@/components/dashboard/crud"
import { OptionCombobox } from "@/components/option-combobox"
import { Badge } from "@/components/ui/badge"
import { UnderlineTabs, type RailopsTabItem } from "@railops/ui"
import {
  createDevice,
  createProduct,
  createProductModel,
  createProductServiceProfile,
  createServiceCodeBatch,
  deleteDevice,
  deleteProduct,
  deleteProductModel,
  deleteProductServiceProfile,
  deleteServiceCodeBatch,
  fetchDevice,
  fetchDevices,
  fetchProductConversations,
  fetchProduct,
  fetchProductDevices,
  fetchProductKnowledgeCoverage,
  fetchProductListAll,
  fetchProductModel,
  fetchProductModelListAll,
  fetchProductModels,
  fetchProductRepairHistory,
  fetchProducts,
  fetchProductServiceCodes,
  fetchProductServiceProfile,
  fetchProductServiceProfiles,
  fetchProductTickets,
  fetchServiceCode,
  fetchServiceCodeBatch,
  fetchServiceCodeBatches,
  fetchServiceCodes,
  generateServiceCodes,
  revokeServiceCode,
  updateDevice,
  updateDeviceStatus,
  updateProduct,
  updateProductModel,
  updateProductModelStatus,
  updateProductServiceProfile,
  updateProductServiceProfileStatus,
  updateProductStatus,
  updateServiceCodeBatchStatus,
  type AdminDevice,
  type AdminProduct,
  type AdminProductModel,
  type AdminProductServiceProfile,
  type AdminServiceCode,
  type AdminServiceCodeBatch,
  type CreateAdminDevicePayload,
  type CreateAdminProductModelPayload,
  type CreateAdminProductPayload,
  type CreateAdminProductServiceProfilePayload,
  type CreateAdminServiceCodeBatchPayload,
  type ProductConversation,
  type ProductKnowledgeCoverage,
  type ProductTicket,
} from "@/lib/api/remote-helpdesk"
import { getEnumOptions } from "@/lib/enums"
import { Status, StatusLabels } from "@/lib/generated/enums"
import { useI18n } from "@/i18n/provider"
import { Button } from "@/components/ui/button"

type LookupState<TItem> = {
  items: TItem[]
  loading: boolean
  loaded: boolean
}

type SetFormValue = (name: string, value: DashboardCrudFormInputValue) => void

const tenantProductCache = new Map<string, LookupState<AdminProduct>>()
const tenantProductRequests = new Map<string, Promise<AdminProduct[]>>()
const productModelCache = new Map<string, LookupState<AdminProductModel>>()
const productModelRequests = new Map<string, Promise<AdminProductModel[]>>()

function toPositiveId(value: unknown) {
  const id = Number(value)
  return Number.isFinite(id) && id > 0 ? id : null
}

function tenantProductKey(tenantId: number) {
  return String(tenantId)
}

function productModelKey(tenantId: number, productId: number) {
  return `${tenantId}:${productId}`
}

function formatReferenceParts(parts: string[]) {
  const output = parts
    .map((part) => part.trim())
    .filter((part) => part.length > 0)
  return output.length > 0 ? output.join(" / ") : "-"
}

function formatProductOptionLabel(product: AdminProduct) {
  return formatReferenceParts([
    `#${product.id}`,
    product.code,
    product.name,
  ])
}

function formatProductModelOptionLabel(model: AdminProductModel) {
  return formatReferenceParts([
    `#${model.id}`,
    model.modelCode,
    model.name,
  ])
}

function buildProductOptions(products: AdminProduct[], selectedId: number | null) {
  const options = products.map((product) => ({
    value: String(product.id),
    label: formatProductOptionLabel(product),
  }))

  if (selectedId && !options.some((option) => option.value === String(selectedId))) {
    return [{ value: String(selectedId), label: `#${selectedId}` }, ...options]
  }

  return options
}

function buildProductModelOptions({
  models,
  selectedId,
  noneLabel,
}: {
  models: AdminProductModel[]
  selectedId: number | null
  noneLabel: string
}) {
  const options = models.map((model) => ({
    value: String(model.id),
    label: formatProductModelOptionLabel(model),
  }))

  if (selectedId && !options.some((option) => option.value === String(selectedId))) {
    options.unshift({ value: String(selectedId), label: `#${selectedId}` })
  }

  return [{ value: "0", label: noneLabel }, ...options]
}

function readCachedLookup<TItem>(
  cache: Map<string, LookupState<TItem>>,
  key: string | null
): LookupState<TItem> {
  if (!key) {
    return { items: [], loading: false, loaded: false }
  }
  return cache.get(key) ?? { items: [], loading: false, loaded: false }
}

function loadTenantProducts(tenantId: number) {
  const key = tenantProductKey(tenantId)
  const cached = tenantProductCache.get(key)

  if (cached?.loaded) {
    return Promise.resolve(cached.items)
  }

  const pending = tenantProductRequests.get(key)
  if (pending) {
    return pending
  }

  tenantProductCache.set(key, {
    items: cached?.items ?? [],
    loading: true,
    loaded: false,
  })

  const request = fetchProductListAll({ tenantId })
    .then((items) => {
      const safeItems = Array.isArray(items) ? items : []
      tenantProductCache.set(key, {
        items: safeItems,
        loading: false,
        loaded: true,
      })
      return safeItems
    })
    .catch(() => {
      tenantProductCache.delete(key)
      return []
    })
    .finally(() => {
      tenantProductRequests.delete(key)
    })

  tenantProductRequests.set(key, request)
  return request
}

function loadProductModels(tenantId: number, productId: number) {
  const key = productModelKey(tenantId, productId)
  const cached = productModelCache.get(key)

  if (cached?.loaded) {
    return Promise.resolve(cached.items)
  }

  const pending = productModelRequests.get(key)
  if (pending) {
    return pending
  }

  productModelCache.set(key, {
    items: cached?.items ?? [],
    loading: true,
    loaded: false,
  })

  const request = fetchProductModelListAll({ tenantId, productId })
    .then((items) => {
      const safeItems = Array.isArray(items) ? items : []
      productModelCache.set(key, {
        items: safeItems,
        loading: false,
        loaded: true,
      })
      return safeItems
    })
    .catch(() => {
      productModelCache.delete(key)
      return []
    })
    .finally(() => {
      productModelRequests.delete(key)
    })

  productModelRequests.set(key, request)
  return request
}

function invalidateTenantProducts(tenantId?: number) {
  if (!tenantId) return
  tenantProductCache.delete(tenantProductKey(tenantId))
}

function invalidateProductModels(tenantId?: number, productId?: number) {
  if (!tenantId || !productId) return
  productModelCache.delete(productModelKey(tenantId, productId))
}

function useTenantProducts(tenantId: number | null) {
  const key = tenantId ? tenantProductKey(tenantId) : null
  const [lookup, setLookup] = useState<LookupState<AdminProduct>>(() =>
    readCachedLookup(tenantProductCache, key)
  )

  useEffect(() => {
    let cancelled = false
    const syncLookup = (nextLookup: LookupState<AdminProduct>) => {
      queueMicrotask(() => {
        if (!cancelled) {
          setLookup(nextLookup)
        }
      })
    }

    if (!tenantId || !key) {
      syncLookup({ items: [], loading: false, loaded: false })
      return
    }

    const request = loadTenantProducts(tenantId)
    syncLookup(readCachedLookup(tenantProductCache, key))
    void request.then((items) => {
      if (cancelled) return
      setLookup({ items, loading: false, loaded: true })
    })

    return () => {
      cancelled = true
    }
  }, [key, tenantId])

  return lookup
}

function useProductModels(tenantId: number | null, productId: number | null) {
  const key =
    tenantId && productId ? productModelKey(tenantId, productId) : null
  const [lookup, setLookup] = useState<LookupState<AdminProductModel>>(() =>
    readCachedLookup(productModelCache, key)
  )

  useEffect(() => {
    let cancelled = false
    const syncLookup = (nextLookup: LookupState<AdminProductModel>) => {
      queueMicrotask(() => {
        if (!cancelled) {
          setLookup(nextLookup)
        }
      })
    }

    if (!tenantId || !productId || !key) {
      syncLookup({ items: [], loading: false, loaded: false })
      return
    }

    const request = loadProductModels(tenantId, productId)
    syncLookup(readCachedLookup(productModelCache, key))
    void request.then((items) => {
      if (cancelled) return
      setLookup({ items, loading: false, loaded: true })
    })

    return () => {
      cancelled = true
    }
  }, [key, productId, tenantId])

  return lookup
}

function ProductSelectField({
  name,
  value,
  values,
  setValue,
  placeholder,
  tenantFirstPlaceholder,
  loadingPlaceholder,
  emptyText,
  clearModelFieldName,
}: {
  name: string
  value: DashboardCrudFormInputValue
  values: Record<string, DashboardCrudFormInputValue>
  setValue: SetFormValue
  placeholder: string
  tenantFirstPlaceholder: string
  loadingPlaceholder: string
  emptyText: string
  clearModelFieldName?: string
}) {
  const tenantId = toPositiveId(values.tenantId)
  const selectedId = toPositiveId(value)
  const selectedValue = selectedId ? String(selectedId) : ""
  const lookup = useTenantProducts(tenantId)
  const previousTenantId = useRef<number | null | undefined>(undefined)
  const options = useMemo(
    () => buildProductOptions(lookup.items, selectedId),
    [lookup.items, selectedId]
  )

  useEffect(() => {
    if (previousTenantId.current === undefined) {
      previousTenantId.current = tenantId
      return
    }
    if (previousTenantId.current === tenantId) return

    previousTenantId.current = tenantId
    setValue(name, "")
    if (clearModelFieldName) {
      setValue(clearModelFieldName, "0")
    }
  }, [clearModelFieldName, name, setValue, tenantId])

  const selectPlaceholder = !tenantId
    ? tenantFirstPlaceholder
    : lookup.loading
      ? loadingPlaceholder
      : placeholder

  return (
    <OptionCombobox
      value={selectedValue}
      options={options}
      placeholder={selectPlaceholder}
      emptyText={tenantId ? emptyText : tenantFirstPlaceholder}
      disabled={!tenantId || lookup.loading}
      onChange={(nextValue) => {
        setValue(name, nextValue)
        if (clearModelFieldName) {
          setValue(clearModelFieldName, "0")
        }
      }}
    />
  )
}

function ProductModelSelectField({
  name,
  value,
  values,
  setValue,
  placeholder,
  tenantFirstPlaceholder,
  productFirstPlaceholder,
  loadingPlaceholder,
  emptyText,
  noneLabel,
}: {
  name: string
  value: DashboardCrudFormInputValue
  values: Record<string, DashboardCrudFormInputValue>
  setValue: SetFormValue
  placeholder: string
  tenantFirstPlaceholder: string
  productFirstPlaceholder: string
  loadingPlaceholder: string
  emptyText: string
  noneLabel: string
}) {
  const tenantId = toPositiveId(values.tenantId)
  const productId = toPositiveId(values.productId)
  const selectedId = toPositiveId(value)
  const selectedValue = selectedId ? String(selectedId) : "0"
  const lookup = useProductModels(tenantId, productId)
  const lookupKey =
    tenantId && productId ? productModelKey(tenantId, productId) : null
  const previousLookupKey = useRef<string | null | undefined>(undefined)
  const options = useMemo(
    () =>
      buildProductModelOptions({
        models: lookup.items,
        selectedId,
        noneLabel,
      }),
    [lookup.items, noneLabel, selectedId]
  )

  useEffect(() => {
    if (previousLookupKey.current === undefined) {
      previousLookupKey.current = lookupKey
      return
    }
    if (previousLookupKey.current === lookupKey) return

    previousLookupKey.current = lookupKey
    setValue(name, "0")
  }, [lookupKey, name, setValue])

  const selectPlaceholder = !tenantId
    ? tenantFirstPlaceholder
    : !productId
      ? productFirstPlaceholder
      : lookup.loading
        ? loadingPlaceholder
        : placeholder

  return (
    <OptionCombobox
      value={selectedValue}
      options={options}
      placeholder={selectPlaceholder}
      emptyText={productId ? emptyText : productFirstPlaceholder}
      disabled={!tenantId || !productId || lookup.loading}
      onChange={(nextValue) => setValue(name, nextValue)}
    />
  )
}

function ProductReference({
  tenantId,
  productId,
}: {
  tenantId: number
  productId: number
}) {
  const normalizedTenantId = toPositiveId(tenantId)
  const normalizedProductId = toPositiveId(productId)
  const lookup = useTenantProducts(normalizedTenantId)
  const product = lookup.items.find((item) => item.id === normalizedProductId)

  if (!normalizedProductId) {
    return <span className="text-muted-foreground">-</span>
  }

  if (!product) {
    return <span className="text-muted-foreground">#{normalizedProductId}</span>
  }

  return (
    <div className="min-w-0">
      <div className="truncate font-medium">
        {product.name || product.code || `#${product.id}`}
      </div>
      <div className="mt-1 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
        <Badge variant="outline">{product.code || `#${product.id}`}</Badge>
        <span>#{product.id}</span>
      </div>
    </div>
  )
}

function ProductModelReference({
  tenantId,
  productId,
  productModelId,
}: {
  tenantId: number
  productId: number
  productModelId: number
}) {
  const normalizedTenantId = toPositiveId(tenantId)
  const normalizedProductId = toPositiveId(productId)
  const normalizedModelId = toPositiveId(productModelId)
  const lookup = useProductModels(normalizedTenantId, normalizedProductId)
  const model = lookup.items.find((item) => item.id === normalizedModelId)

  if (!normalizedModelId) {
    return <span className="text-muted-foreground">-</span>
  }

  if (!model) {
    return <span className="text-muted-foreground">#{normalizedModelId}</span>
  }

  return (
    <div className="min-w-0">
      <div className="truncate font-medium">
        {model.name || model.modelCode || `#${model.id}`}
      </div>
      <div className="mt-1 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
        <Badge variant="outline">{model.modelCode || `#${model.id}`}</Badge>
        <span>#{model.id}</span>
      </div>
    </div>
  )
}

function DeviceProductModelReference({
  tenantId,
  productId,
  productModelId,
}: {
  tenantId: number
  productId: number
  productModelId: number
}) {
  return (
    <div className="min-w-0 space-y-2">
      <ProductReference tenantId={tenantId} productId={productId} />
      <div className="border-t pt-2">
        <ProductModelReference
          tenantId={tenantId}
          productId={productId}
          productModelId={productModelId}
        />
      </div>
    </div>
  )
}

function getStatusLabel(status: Status, t: (key: string) => string) {
  if (status === Status.Disabled) {
    return t("status.disabled")
  }
  if (status === Status.Deleted) {
    return t("status.deleted")
  }
  return t("status.ok")
}

function statusOptions(t: (key: string) => string) {
  return [
    { value: "all", label: t("status.all") },
    ...getEnumOptions(StatusLabels)
      .filter((item) => Number(item.value) !== Status.Deleted)
      .map((item) => ({
        value: String(item.value),
        label: getStatusLabel(item.value as Status, t),
      })),
  ]
}

function serviceCodeModeOptions(t: (key: string) => string) {
  return [
    { value: "traceable", label: t("productCenter.serviceCode.modeTraceable") },
    { value: "general", label: t("productCenter.serviceCode.modeGeneral") },
  ]
}

function serviceCodeStatusOptions(t: (key: string) => string) {
  return [
    { value: "all", label: t("status.all") },
    { value: "active", label: t("productCenter.serviceCode.statusActive") },
    { value: "revoked", label: t("productCenter.serviceCode.statusRevoked") },
    { value: "disabled", label: t("productCenter.serviceCode.statusDisabled") },
  ]
}

function getServiceCodeModeLabel(mode: string, t: (key: string) => string) {
  if (mode === "general") {
    return t("productCenter.serviceCode.modeGeneral")
  }
  return t("productCenter.serviceCode.modeTraceable")
}

function getServiceCodeStatusLabel(status: string, t: (key: string) => string) {
  if (status === "revoked") {
    return t("productCenter.serviceCode.statusRevoked")
  }
  if (status === "disabled") {
    return t("productCenter.serviceCode.statusDisabled")
  }
  return t("productCenter.serviceCode.statusActive")
}

function statusColumn<TItem>({
  label,
  getStatus,
  getLabel,
}: {
  label: string
  getStatus: (item: TItem) => Status
  getLabel: (status: Status) => string
}) {
  return createDashboardStatusColumn<TItem, Status>({
    label,
    className: "w-24",
    getStatus,
    getLabel: (status) => getLabel(status),
    getBadgeVariant: (status) => (status === Status.Ok ? "default" : "outline"),
  })
}

function commonFormLabels(t: (key: string) => string) {
  return {
    create: t("productCenter.create"),
    save: t("productCenter.save"),
    saving: t("productCenter.saving"),
    cancel: t("productCenter.cancel"),
    loadingDetail: t("productCenter.loadingDetail"),
    required: t("productCenter.required"),
    invalidNumber: t("productCenter.invalidNumber"),
    minValue: () => t("productCenter.invalidNumber"),
    maxValue: () => t("productCenter.invalidNumber"),
  }
}

function commonPageLabels<TItem, TPayload>(
  t: (key: string, values?: Record<string, string | number>) => string,
  scope: "product" | "profile" | "model" | "device" | "batch" | "code",
  getItemName: (item: TItem) => string,
  getPayloadName: (payload: TPayload) => string
) {
  const prefix = `productCenter.${scope}`
  return {
    refresh: t("productCenter.refresh"),
    create: t(`${prefix}.new`),
    query: t("productCenter.query"),
    loading: t(`${prefix}.loading`),
    empty: t(`${prefix}.empty`),
    actions: t("productCenter.actions"),
    edit: t("productCenter.edit"),
    delete: t("productCenter.delete"),
    processing: t("productCenter.processing"),
    moreActions: (item: TItem) =>
      t("productCenter.moreActions", { name: getItemName(item) }),
    loadFailed: t(`${prefix}.loadFailed`),
    saveFailed: t(`${prefix}.saveFailed`),
    deleteFailed: t(`${prefix}.deleteFailed`),
    created: (payload: TPayload) =>
      t(`${prefix}.created`, { name: getPayloadName(payload) }),
    updated: (item: TItem) => t(`${prefix}.updated`, { name: getItemName(item) }),
    deleted: (item: TItem) => t(`${prefix}.deleted`, { name: getItemName(item) }),
  }
}

type ProductCenterTab =
  | "products"
  | "overview"
  | "profiles"
  | "models"
  | "devices"
  | "serviceCodes"
  | "tickets"
  | "repairHistory"
  | "conversations"
  | "knowledge"

type ProductCenterT = (
  key: string,
  values?: Record<string, string | number>
) => string

type ProductOverviewState = {
  deviceTotal: number
  serviceCodeTotal: number
  ticketTotal: number
  repairHistoryTotal: number
  knowledge: ProductKnowledgeCoverage
}

const productCenterTabValues = new Set<ProductCenterTab>([
  "products",
  "overview",
  "profiles",
  "models",
  "devices",
  "serviceCodes",
  "tickets",
  "repairHistory",
  "conversations",
  "knowledge",
])

function normalizeProductCenterTab(value: string | null): ProductCenterTab {
  if (value && productCenterTabValues.has(value as ProductCenterTab)) {
    return value as ProductCenterTab
  }
  return "products"
}

function EmptyProductContext({ t }: { t: ProductCenterT }) {
  return (
    <div className="rounded-md border border-dashed p-6 text-sm text-muted-foreground">
      {t("productCenter.dossier.selectProduct")}
    </div>
  )
}

function ProductContextPanel({
  productId,
  activeTab,
  t,
  openTab,
}: {
  productId: number | null
  activeTab: ProductCenterTab
  t: ProductCenterT
  openTab: (tab: ProductCenterTab) => void
}) {
  const [product, setProduct] = useState<AdminProduct | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    const scheduleState = (fn: () => void) => {
      queueMicrotask(() => {
        if (!cancelled) fn()
      })
    }
    if (!productId) {
      scheduleState(() => setProduct(null))
      return () => {
        cancelled = true
      }
    }
    scheduleState(() => setLoading(true))
    void fetchProduct(productId)
      .then((item) => {
        if (!cancelled) setProduct(item)
      })
      .catch(() => {
        if (!cancelled) setProduct(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [productId])

  return (
    <div className="mb-4 rounded-md border bg-background p-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <div className="text-xs font-medium uppercase text-muted-foreground">
            {t("productCenter.dossier.current")}
          </div>
          <div className="mt-1 flex min-w-0 flex-wrap items-center gap-2">
            <span className="truncate text-base font-semibold">
              {product?.name ||
                (loading
                  ? t("productCenter.loadingDetail")
                  : productId
                    ? `#${productId}`
                    : t("productCenter.dossier.none"))}
            </span>
            {product ? <Badge variant="outline">{product.code}</Badge> : null}
            {product?.tenantId ? (
              <Badge variant="outline">
                {t("productCenter.tenantId")}: {product.tenantId}
              </Badge>
            ) : null}
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {([
            ["overview", PackageIcon],
            ["devices", ScanBarcodeIcon],
            ["serviceCodes", QrCodeIcon],
            ["tickets", ClipboardListIcon],
            ["repairHistory", HistoryIcon],
            ["conversations", MessageSquareMoreIcon],
            ["knowledge", BookOpenIcon],
          ] as const).map(([tab, Icon]) => (
            <Button
              key={tab}
              type="button"
              variant={activeTab === tab ? "default" : "outline"}
              size="sm"
              disabled={!productId}
              onClick={() => openTab(tab)}
            >
              <Icon />
              {t(`productCenter.tabs.${tab}`)}
            </Button>
          ))}
        </div>
      </div>
    </div>
  )
}

function ProductDossierOverview({
  productId,
  t,
  openTab,
}: {
  productId: number | null
  t: ProductCenterT
  openTab: (tab: ProductCenterTab) => void
}) {
  const [summary, setSummary] = useState<ProductOverviewState | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    const scheduleState = (fn: () => void) => {
      queueMicrotask(() => {
        if (!cancelled) fn()
      })
    }
    if (!productId) {
      scheduleState(() => setSummary(null))
      return () => {
        cancelled = true
      }
    }
    scheduleState(() => setLoading(true))
    void Promise.all([
      fetchProductDevices(productId, { page: 1, limit: 1 }),
      fetchProductServiceCodes(productId, { page: 1, limit: 1 }),
      fetchProductTickets(productId, { page: 1, limit: 1 }),
      fetchProductRepairHistory(productId, { page: 1, limit: 1 }),
      fetchProductKnowledgeCoverage(productId),
    ])
      .then(([devices, serviceCodes, tickets, repairHistory, knowledge]) => {
        if (cancelled) return
        setSummary({
          deviceTotal: devices.page.total,
          serviceCodeTotal: serviceCodes.page.total,
          ticketTotal: tickets.page.total,
          repairHistoryTotal: repairHistory.page.total,
          knowledge,
        })
      })
      .catch(() => {
        if (!cancelled) setSummary(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [productId])

  if (!productId) return <EmptyProductContext t={t} />

  const metrics = [
    ["devices", summary?.deviceTotal ?? 0, ScanBarcodeIcon],
    ["serviceCodes", summary?.serviceCodeTotal ?? 0, QrCodeIcon],
    ["tickets", summary?.ticketTotal ?? 0, ClipboardListIcon],
    ["repairHistory", summary?.repairHistoryTotal ?? 0, HistoryIcon],
    ["knowledge", summary?.knowledge.bindingCount ?? 0, BookOpenIcon],
  ] as const

  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-5">
        {metrics.map(([tab, value, Icon]) => (
          <button
            key={tab}
            type="button"
            className="rounded-md border bg-background p-4 text-left transition-colors hover:bg-muted"
            onClick={() => openTab(tab)}
          >
            <div className="flex items-center justify-between text-sm text-muted-foreground">
              <span>{t(`productCenter.tabs.${tab}`)}</span>
              <Icon className="size-4" />
            </div>
            <div className="mt-3 text-2xl font-semibold">
              {loading ? "-" : value}
            </div>
          </button>
        ))}
      </div>
      <div className="rounded-md border bg-background p-4 text-sm text-muted-foreground">
        {t("productCenter.dossier.knowledgeBase")}:{" "}
        {summary?.knowledge.defaultKnowledgeBaseId
          ? `#${summary.knowledge.defaultKnowledgeBaseId}`
          : "-"}
      </div>
    </div>
  )
}

function ProductTicketListPanel({
  productId,
  deviceId,
  mode,
  t,
}: {
  productId: number | null
  deviceId?: number | null
  mode: "tickets" | "repairHistory"
  t: ProductCenterT
}) {
  const [items, setItems] = useState<ProductTicket[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    const scheduleState = (fn: () => void) => {
      queueMicrotask(() => {
        if (!cancelled) fn()
      })
    }
    if (!productId) {
      scheduleState(() => {
        setItems([])
        setTotal(0)
      })
      return () => {
        cancelled = true
      }
    }
    scheduleState(() => setLoading(true))
    const query = { page: 1, limit: 20, deviceId: deviceId ?? undefined }
    const loader =
      mode === "repairHistory"
        ? fetchProductRepairHistory(productId, query)
        : fetchProductTickets(productId, query)
    void loader
      .then((page) => {
        if (cancelled) return
        setItems(page.results ?? [])
        setTotal(page.page.total)
      })
      .catch(() => {
        if (cancelled) return
        setItems([])
        setTotal(0)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [deviceId, mode, productId])

  if (!productId) return <EmptyProductContext t={t} />

  return (
    <div className="rounded-md border bg-background">
      <div className="flex items-center justify-between border-b p-4">
        <div className="font-medium">{t(`productCenter.tabs.${mode}`)}</div>
        <Badge variant="outline">{loading ? "-" : total}</Badge>
      </div>
      <div className="divide-y">
        {items.length === 0 ? (
          <div className="p-6 text-sm text-muted-foreground">
            {loading ? t("productCenter.loadingDetail") : t("productCenter.dossier.empty")}
          </div>
        ) : (
          items.map((item) => (
            <div key={item.id} className="grid gap-2 p-4 md:grid-cols-[1fr_160px_140px]">
              <div className="min-w-0">
                <div className="truncate font-medium">{item.title}</div>
                <div className="mt-1 truncate text-sm text-muted-foreground">
                  {item.symptomSummary || item.description || "-"}
                </div>
              </div>
              <div className="text-sm text-muted-foreground">
                {item.ticketNo || `#${item.id}`}
              </div>
              <div className="text-sm text-muted-foreground">
                {item.updatedAt || item.createdAt || "-"}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function ProductConversationsPanel({
  productId,
  t,
}: {
  productId: number | null
  t: ProductCenterT
}) {
  const [items, setItems] = useState<ProductConversation[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    const scheduleState = (fn: () => void) => {
      queueMicrotask(() => {
        if (!cancelled) fn()
      })
    }
    if (!productId) {
      scheduleState(() => {
        setItems([])
        setTotal(0)
      })
      return () => {
        cancelled = true
      }
    }
    scheduleState(() => setLoading(true))
    void fetchProductConversations(productId, { page: 1, limit: 20 })
      .then((page) => {
        if (cancelled) return
        setItems(page.results ?? [])
        setTotal(page.page.total)
      })
      .catch(() => {
        if (cancelled) return
        setItems([])
        setTotal(0)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [productId])

  if (!productId) return <EmptyProductContext t={t} />

  return (
    <div className="rounded-md border bg-background">
      <div className="flex items-center justify-between border-b p-4">
        <div className="font-medium">{t("productCenter.tabs.conversations")}</div>
        <Badge variant="outline">{loading ? "-" : total}</Badge>
      </div>
      <div className="divide-y">
        {items.length === 0 ? (
          <div className="p-6 text-sm text-muted-foreground">
            {loading ? t("productCenter.loadingDetail") : t("productCenter.dossier.empty")}
          </div>
        ) : (
          items.map((item) => (
            <div key={item.id} className="grid gap-2 p-4 md:grid-cols-[1fr_160px]">
              <div className="min-w-0">
                <div className="truncate font-medium">
                  {item.customerName || `#${item.customerId}`}
                </div>
                <div className="mt-1 truncate text-sm text-muted-foreground">
                  {item.lastMessageSummary || "-"}
                </div>
              </div>
              <div className="text-sm text-muted-foreground">
                {item.lastActiveAt || item.lastMessageAt || "-"}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

function ProductKnowledgePanel({
  productId,
  t,
}: {
  productId: number | null
  t: ProductCenterT
}) {
  const [coverage, setCoverage] = useState<ProductKnowledgeCoverage | null>(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    let cancelled = false
    const scheduleState = (fn: () => void) => {
      queueMicrotask(() => {
        if (!cancelled) fn()
      })
    }
    if (!productId) {
      scheduleState(() => setCoverage(null))
      return () => {
        cancelled = true
      }
    }
    scheduleState(() => setLoading(true))
    void fetchProductKnowledgeCoverage(productId)
      .then((item) => {
        if (!cancelled) setCoverage(item)
      })
      .catch(() => {
        if (!cancelled) setCoverage(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [productId])

  if (!productId) return <EmptyProductContext t={t} />

  return (
    <div className="rounded-md border bg-background p-4">
      <div className="grid gap-3 md:grid-cols-3">
        <div>
          <div className="text-sm text-muted-foreground">
            {t("productCenter.dossier.defaultKnowledgeBase")}
          </div>
          <div className="mt-2 text-xl font-semibold">
            {loading
              ? "-"
              : coverage?.defaultKnowledgeBaseId
                ? `#${coverage.defaultKnowledgeBaseId}`
                : "-"}
          </div>
        </div>
        <div>
          <div className="text-sm text-muted-foreground">
            {t("productCenter.dossier.knowledgeBindings")}
          </div>
          <div className="mt-2 text-xl font-semibold">
            {loading ? "-" : coverage?.bindingCount ?? 0}
          </div>
        </div>
        <div>
          <div className="text-sm text-muted-foreground">
            {t("productCenter.status")}
          </div>
          <div className="mt-2">
            <Badge variant={coverage?.status === Status.Ok ? "default" : "outline"}>
              {getStatusLabel((coverage?.status ?? 0) as Status, t)}
            </Badge>
          </div>
        </div>
      </div>
    </div>
  )
}

export default function DashboardProductsPage() {
  const t = useI18n()
  const router = useRouter()
  const searchParams = useSearchParams()
  const activeTab = normalizeProductCenterTab(searchParams.get("tab"))

  const productTabs: RailopsTabItem[] = [
    { value: "products", label: t("productCenter.tabs.products"), icon: <PackageIcon /> },
    { value: "overview", label: t("productCenter.tabs.overview"), icon: <PackageIcon /> },
    { value: "profiles", label: t("productCenter.tabs.profiles"), icon: <Settings2Icon /> },
    { value: "models", label: t("productCenter.tabs.models"), icon: <CpuIcon /> },
    { value: "devices", label: t("productCenter.tabs.devices"), icon: <ScanBarcodeIcon /> },
    { value: "serviceCodes", label: t("productCenter.tabs.serviceCodes"), icon: <QrCodeIcon /> },
    { value: "tickets", label: t("productCenter.tabs.tickets"), icon: <ClipboardListIcon /> },
    { value: "repairHistory", label: t("productCenter.tabs.repairHistory"), icon: <HistoryIcon /> },
    { value: "conversations", label: t("productCenter.tabs.conversations"), icon: <MessageSquareMoreIcon /> },
    { value: "knowledge", label: t("productCenter.tabs.knowledge"), icon: <BookOpenIcon /> },
  ]
  const selectedProductId = toPositiveId(searchParams.get("productId"))
  const selectedDeviceId = toPositiveId(searchParams.get("deviceId"))
  const listStatusOptions = statusOptions(t)
  const modeOptions = serviceCodeModeOptions(t)
  const serviceCodeListStatusOptions = serviceCodeStatusOptions(t)
  const localeOptions = [
    { value: "en", label: "English" },
    { value: "en-US", label: "English (US)" },
    { value: "zh-CN", label: "简体中文" },
    { value: "es", label: "Español" },
    { value: "ar", label: "العربية" },
  ]
  const sourceOptions = [
    { value: "manual", label: t("productCenter.device.sourceManual") },
    { value: "import", label: t("productCenter.device.sourceImport") },
    { value: "api", label: t("productCenter.device.sourceApi") },
  ]

  function updateProductCenterUrl({
    tab,
    productId,
    deviceId,
  }: {
    tab?: ProductCenterTab
    productId?: number | null
    deviceId?: number | null
  }) {
    const params = new URLSearchParams(searchParams.toString())
    params.set("tab", tab ?? activeTab)
    const nextProductId = productId === undefined ? selectedProductId : productId
    if (nextProductId) {
      params.set("productId", String(nextProductId))
    } else {
      params.delete("productId")
    }
    const nextDeviceId = deviceId === undefined ? selectedDeviceId : deviceId
    if (nextDeviceId) {
      params.set("deviceId", String(nextDeviceId))
    } else {
      params.delete("deviceId")
    }
    const query = params.toString()
    router.replace(query ? `/dashboard/products?${query}` : "/dashboard/products")
  }

  function openProductTab(tab: ProductCenterTab) {
    updateProductCenterUrl({ tab })
  }

  function openProductDossier(
    productId: number,
    tab: ProductCenterTab = "overview",
    deviceId: number | null = null
  ) {
    updateProductCenterUrl({ tab, productId, deviceId })
  }

  return (
    <DashboardPage>
      <ProductContextPanel
        productId={selectedProductId}
        activeTab={activeTab}
        t={t}
        openTab={openProductTab}
      />
      <div className="min-h-0">
        <div className="w-full overflow-x-auto [&_.railops-underline-tab-list]:min-w-[640px]">
          <UnderlineTabs
            ariaLabel={t("miscTailExtract.dashboardMisc.products.productCenterTabsAria")}
            items={productTabs}
            value={activeTab}
            onChange={(value) => updateProductCenterUrl({ tab: normalizeProductCenterTab(value) })}
          />
        </div>
        {activeTab === "overview" ? (
          <div>
          <ProductDossierOverview
            productId={selectedProductId}
            t={t}
            openTab={openProductTab}
          />
          </div>
        ) : null}
        {activeTab === "products" ? (
          <div>
          <DashboardCrudPage<AdminProduct, CreateAdminProductPayload>
            layout="fragment"
            filters={[
              {
                name: "tenantId",
                label: t("productCenter.tenantId"),
                placeholder: t("productCenter.tenantId"),
                defaultValue: "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "code",
                label: t("productCenter.product.code"),
                placeholder: t("productCenter.product.filterCode"),
                defaultValue: "",
                trim: true,
                className: "w-full sm:w-44",
              },
              {
                name: "name",
                label: t("productCenter.product.name"),
                placeholder: t("productCenter.product.filterName"),
                defaultValue: "",
                trim: true,
                className: "w-full sm:w-56",
              },
              {
                name: "status",
                label: t("status.all"),
                type: "select",
                defaultValue: "all",
                allValue: "all",
                options: listStatusOptions,
                className: "w-full sm:w-36",
              },
            ]}
            columns={[
              {
                key: "product",
                label: t("productCenter.product.columnProduct"),
                render: (item) => (
                  <div className="min-w-0">
                    <div className="font-medium">{item.name}</div>
                    <div className="mt-1 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
                      <Badge variant="outline">{item.code}</Badge>
                      <span>{item.category || "-"}</span>
                    </div>
                  </div>
                ),
              },
              {
                key: "tenantId",
                label: t("productCenter.tenantId"),
                className: "w-24",
                render: (item) => item.tenantId,
              },
              {
                key: "defaultLocale",
                label: t("productCenter.product.defaultLocale"),
                className: "w-32",
                render: (item) => item.defaultLocale || "-",
              },
              statusColumn<AdminProduct>({
                label: t("productCenter.status"),
                getStatus: (item) => item.status as Status,
                getLabel: (status) => getStatusLabel(status, t),
              }),
            ]}
            fetchList={fetchProducts}
            getItemId={(item) => item.id}
            createItem={(payload) =>
              createProduct(payload).then((result) => {
                invalidateTenantProducts(payload.tenantId)
                return result
              })
            }
            updateItem={(item, payload) =>
              updateProduct({ id: item.id, ...payload }).then((result) => {
                invalidateTenantProducts(item.tenantId)
                invalidateTenantProducts(payload.tenantId)
                return result
              })
            }
            deleteItem={(item) =>
              deleteProduct(item.id).then((result) => {
                invalidateTenantProducts(item.tenantId)
                return result
              })
            }
            canDelete={(item) => item.status !== Status.Deleted}
            form={{
              fetchDetail: fetchProduct,
              fields: [
                {
                  name: "tenantId",
                  label: t("productCenter.tenantId"),
                  type: "number",
                  defaultValue: "1",
                  min: 1,
                  step: 1,
                  valueType: "number",
                  required: true,
                  requiredMessage: t("productCenter.tenantIdRequired"),
                },
                {
                  name: "code",
                  label: t("productCenter.product.code"),
                  placeholder: "HP",
                  required: true,
                  trim: true,
                  requiredMessage: t("productCenter.product.codeRequired"),
                },
                {
                  name: "name",
                  label: t("productCenter.product.name"),
                  placeholder: t("productCenter.product.namePlaceholder"),
                  required: true,
                  trim: true,
                  requiredMessage: t("productCenter.product.nameRequired"),
                },
                {
                  name: "category",
                  label: t("productCenter.product.category"),
                  placeholder: t("productCenter.product.categoryPlaceholder"),
                  trim: true,
                },
                {
                  name: "productLineId",
                  label: t("productCenter.product.productLineId"),
                  type: "number",
                  defaultValue: "0",
                  min: 0,
                  step: 1,
                  valueType: "number",
                },
                {
                  name: "ownerMemberId",
                  label: t("productCenter.product.ownerMemberId"),
                  type: "number",
                  defaultValue: "0",
                  min: 0,
                  step: 1,
                  valueType: "number",
                },
                {
                  name: "defaultLocale",
                  label: t("productCenter.product.defaultLocale"),
                  type: "select",
                  defaultValue: "en",
                  options: localeOptions,
                },
              ],
              transformSubmitValues: (values) => ({
                tenantId: Number(values.tenantId),
                productLineId: Number(values.productLineId),
                code: String(values.code ?? ""),
                name: String(values.name ?? ""),
                category: String(values.category ?? ""),
                ownerMemberId: Number(values.ownerMemberId),
                defaultLocale: String(values.defaultLocale ?? "en"),
              }),
              labels: {
                ...commonFormLabels(t),
                createTitle: t("productCenter.product.createTitle"),
                editTitle: t("productCenter.product.editTitle"),
              },
            }}
            rowActions={[
              {
                key: "openDossier",
                icon: <PackageIcon />,
                label: t("productCenter.product.openDossier"),
                run: ({ item }) => openProductDossier(item.id),
              },
              createDashboardStatusToggleAction<AdminProduct, Status>({
                disabled: (item) => item.status === Status.Deleted,
                icon: (item) =>
                  item.status === Status.Ok ? <BanIcon /> : <CheckCircle2Icon />,
                label: (item) =>
                  item.status === Status.Ok
                    ? t("productCenter.disable")
                    : t("productCenter.enable"),
                getNextStatus: (item) =>
                  item.status === Status.Ok ? Status.Disabled : Status.Ok,
                updateStatus: (item, nextStatus) =>
                  updateProductStatus(item.id, nextStatus).then((result) => {
                    invalidateTenantProducts(item.tenantId)
                    return result
                  }),
                successMessage: (item, nextStatus) =>
                  t(
                    nextStatus === Status.Ok
                      ? "productCenter.product.enabled"
                      : "productCenter.product.disabled",
                    { name: item.name }
                  ),
                errorMessage: t("productCenter.statusUpdateFailed"),
              }),
            ]}
            labels={commonPageLabels<AdminProduct, CreateAdminProductPayload>(
              t,
              "product",
              (item) => item.name,
              (payload) => payload.name
            )}
          />
          </div>
        ) : null}
        {activeTab === "profiles" ? (
          <div>
          <DashboardCrudPage<
            AdminProductServiceProfile,
            CreateAdminProductServiceProfilePayload
          >
            layout="fragment"
            filters={[
              {
                name: "tenantId",
                label: t("productCenter.tenantId"),
                placeholder: t("productCenter.tenantId"),
                defaultValue: "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "productId",
                label: t("productCenter.productId"),
                placeholder: t("productCenter.productId"),
                defaultValue: selectedProductId ? String(selectedProductId) : "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "safetyLevel",
                label: t("productCenter.profile.safetyLevel"),
                placeholder: t("productCenter.profile.safetyLevel"),
                defaultValue: "",
                trim: true,
                className: "w-full sm:w-40",
              },
              {
                name: "status",
                label: t("status.all"),
                type: "select",
                defaultValue: "all",
                allValue: "all",
                options: listStatusOptions,
                className: "w-full sm:w-36",
              },
            ]}
            columns={[
              {
                key: "product",
                label: t("productCenter.product.columnProduct"),
                className: "w-56",
                render: (item) => (
                  <ProductReference
                    tenantId={item.tenantId}
                    productId={item.productId}
                  />
                ),
              },
              {
                key: "support",
                label: t("productCenter.profile.supportScope"),
                render: (item) => (
                  <div className="min-w-0 space-y-1 font-mono text-xs text-muted-foreground">
                    <div className="truncate">{item.supportLocalesJson || "[]"}</div>
                    <div className="truncate">{item.supportRegionsJson || "[]"}</div>
                  </div>
                ),
              },
              {
                key: "policy",
                label: t("productCenter.profile.policy"),
                render: (item) => (
                  <div className="min-w-0">
                    <div className="font-medium">
                      {item.safetyLevel || t("productCenter.profile.noSafetyLevel")}
                    </div>
                    <div className="mt-1 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
                      <Badge variant="outline">
                        {item.meetingEnabled
                          ? t("productCenter.profile.meetingEnabled")
                          : t("productCenter.profile.meetingDisabled")}
                      </Badge>
                      <span>
                        {t("productCenter.profile.knowledgeBaseShort", {
                          id: item.defaultKnowledgeBaseId,
                        })}
                      </span>
                    </div>
                  </div>
                ),
              },
              statusColumn<AdminProductServiceProfile>({
                label: t("productCenter.status"),
                getStatus: (item) => item.status as Status,
                getLabel: (status) => getStatusLabel(status, t),
              }),
            ]}
            fetchList={fetchProductServiceProfiles}
            getItemId={(item) => item.id}
            createItem={createProductServiceProfile}
            updateItem={(item, payload) =>
              updateProductServiceProfile({ id: item.id, ...payload })
            }
            deleteItem={(item) => deleteProductServiceProfile(item.id)}
            canDelete={(item) => item.status !== Status.Deleted}
            form={{
              fetchDetail: fetchProductServiceProfile,
              fields: [
                {
                  name: "tenantId",
                  label: t("productCenter.tenantId"),
                  type: "number",
                  defaultValue: "1",
                  min: 1,
                  step: 1,
                  valueType: "number",
                  required: true,
                  requiredMessage: t("productCenter.tenantIdRequired"),
                },
                {
                  name: "productId",
                  label: t("productCenter.productId"),
                  type: "custom",
                  defaultValue: "",
                  valueType: "number",
                  required: true,
                  requiredMessage: t("productCenter.productIdRequired"),
                  render: ({ name, value, values, setValue }) => (
                    <ProductSelectField
                      name={name}
                      value={value}
                      values={values}
                      setValue={setValue}
                      placeholder={t("productCenter.product.selectPlaceholder")}
                      tenantFirstPlaceholder={t(
                        "productCenter.product.selectTenantFirst"
                      )}
                      loadingPlaceholder={t(
                        "productCenter.product.loadingOptions"
                      )}
                      emptyText={t("productCenter.product.emptyOptions")}
                    />
                  ),
                },
                {
                  name: "supportLocalesJson",
                  label: t("productCenter.profile.supportLocalesJson"),
                  type: "json",
                  defaultValue: "[\"en\"]",
                  rows: 3,
                },
                {
                  name: "supportRegionsJson",
                  label: t("productCenter.profile.supportRegionsJson"),
                  type: "json",
                  defaultValue: "[]",
                  rows: 3,
                },
                {
                  name: "warrantyPolicyJson",
                  label: t("productCenter.profile.warrantyPolicyJson"),
                  type: "json",
                  defaultValue: "{}",
                  rows: 4,
                },
                {
                  name: "servicePolicyJson",
                  label: t("productCenter.profile.servicePolicyJson"),
                  type: "json",
                  defaultValue: "{}",
                  rows: 4,
                },
                {
                  name: "safetyLevel",
                  label: t("productCenter.profile.safetyLevel"),
                  placeholder: "standard",
                  trim: true,
                },
                {
                  name: "defaultFlowTemplateId",
                  label: t("productCenter.profile.defaultFlowTemplateId"),
                  type: "number",
                  defaultValue: "0",
                  min: 0,
                  step: 1,
                  valueType: "number",
                },
                {
                  name: "defaultKnowledgeBaseId",
                  label: t("productCenter.profile.defaultKnowledgeBaseId"),
                  type: "number",
                  defaultValue: "0",
                  min: 0,
                  step: 1,
                  valueType: "number",
                },
                {
                  name: "meetingEnabled",
                  label: t("productCenter.profile.meetingEnabled"),
                  type: "switch",
                  defaultValue: true,
                  valueType: "boolean",
                },
              ],
              transformSubmitValues: (values) => ({
                tenantId: Number(values.tenantId),
                productId: Number(values.productId),
                supportLocalesJson: String(values.supportLocalesJson ?? "[]"),
                supportRegionsJson: String(values.supportRegionsJson ?? "[]"),
                warrantyPolicyJson: String(values.warrantyPolicyJson ?? "{}"),
                safetyLevel: String(values.safetyLevel ?? ""),
                defaultFlowTemplateId: Number(values.defaultFlowTemplateId),
                defaultKnowledgeBaseId: Number(values.defaultKnowledgeBaseId),
                meetingEnabled: Boolean(values.meetingEnabled),
                servicePolicyJson: String(values.servicePolicyJson ?? "{}"),
              }),
              labels: {
                ...commonFormLabels(t),
                createTitle: t("productCenter.profile.createTitle"),
                editTitle: t("productCenter.profile.editTitle"),
              },
            }}
            rowActions={[
              createDashboardStatusToggleAction<
                AdminProductServiceProfile,
                Status
              >({
                disabled: (item) => item.status === Status.Deleted,
                icon: (item) =>
                  item.status === Status.Ok ? <BanIcon /> : <CheckCircle2Icon />,
                label: (item) =>
                  item.status === Status.Ok
                    ? t("productCenter.disable")
                    : t("productCenter.enable"),
                getNextStatus: (item) =>
                  item.status === Status.Ok ? Status.Disabled : Status.Ok,
                updateStatus: (item, nextStatus) =>
                  updateProductServiceProfileStatus(item.id, nextStatus),
                successMessage: (item, nextStatus) =>
                  t(
                    nextStatus === Status.Ok
                      ? "productCenter.profile.enabled"
                      : "productCenter.profile.disabled",
                    { name: `#${item.productId}` }
                  ),
                errorMessage: t("productCenter.statusUpdateFailed"),
              }),
            ]}
            labels={commonPageLabels<
              AdminProductServiceProfile,
              CreateAdminProductServiceProfilePayload
            >(
              t,
              "profile",
              (item) => `#${item.productId}`,
              (payload) => `#${payload.productId}`
            )}
          />
          </div>
        ) : null}
        {activeTab === "models" ? (
          <div>
          <DashboardCrudPage<AdminProductModel, CreateAdminProductModelPayload>
            layout="fragment"
            filters={[
              {
                name: "tenantId",
                label: t("productCenter.tenantId"),
                placeholder: t("productCenter.tenantId"),
                defaultValue: "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "productId",
                label: t("productCenter.productId"),
                placeholder: t("productCenter.productId"),
                defaultValue: selectedProductId ? String(selectedProductId) : "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "modelCode",
                label: t("productCenter.model.modelCode"),
                placeholder: t("productCenter.model.filterCode"),
                defaultValue: "",
                trim: true,
                className: "w-full sm:w-44",
              },
              {
                name: "status",
                label: t("status.all"),
                type: "select",
                defaultValue: "all",
                allValue: "all",
                options: listStatusOptions,
                className: "w-full sm:w-36",
              },
            ]}
            columns={[
              {
                key: "model",
                label: t("productCenter.model.columnModel"),
                render: (item) => (
                  <div className="min-w-0">
                    <div className="font-medium">{item.name}</div>
                    <div className="mt-1 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
                      <Badge variant="outline">{item.modelCode}</Badge>
                      <span>{item.versionPolicy || "-"}</span>
                    </div>
                  </div>
                ),
              },
              {
                key: "productId",
                label: t("productCenter.product.columnProduct"),
                className: "w-52",
                render: (item) => (
                  <ProductReference
                    tenantId={item.tenantId}
                    productId={item.productId}
                  />
                ),
              },
              {
                key: "regionScope",
                label: t("productCenter.model.regionScope"),
                render: (item) => (
                  <span className="font-mono text-xs text-muted-foreground">
                    {item.regionScopeJson || "[]"}
                  </span>
                ),
              },
              statusColumn<AdminProductModel>({
                label: t("productCenter.status"),
                getStatus: (item) => item.status as Status,
                getLabel: (status) => getStatusLabel(status, t),
              }),
            ]}
            fetchList={fetchProductModels}
            getItemId={(item) => item.id}
            createItem={(payload) =>
              createProductModel(payload).then((result) => {
                invalidateProductModels(payload.tenantId, payload.productId)
                return result
              })
            }
            updateItem={(item, payload) =>
              updateProductModel({ id: item.id, ...payload }).then((result) => {
                invalidateProductModels(item.tenantId, item.productId)
                invalidateProductModels(payload.tenantId, payload.productId)
                return result
              })
            }
            deleteItem={(item) =>
              deleteProductModel(item.id).then((result) => {
                invalidateProductModels(item.tenantId, item.productId)
                return result
              })
            }
            canDelete={(item) => item.status !== Status.Deleted}
            form={{
              fetchDetail: fetchProductModel,
              fields: [
                {
                  name: "tenantId",
                  label: t("productCenter.tenantId"),
                  type: "number",
                  defaultValue: "1",
                  min: 1,
                  step: 1,
                  valueType: "number",
                  required: true,
                  requiredMessage: t("productCenter.tenantIdRequired"),
                },
                {
                  name: "productId",
                  label: t("productCenter.productId"),
                  type: "custom",
                  defaultValue: "",
                  valueType: "number",
                  required: true,
                  requiredMessage: t("productCenter.productIdRequired"),
                  render: ({ name, value, values, setValue }) => (
                    <ProductSelectField
                      name={name}
                      value={value}
                      values={values}
                      setValue={setValue}
                      placeholder={t("productCenter.product.selectPlaceholder")}
                      tenantFirstPlaceholder={t(
                        "productCenter.product.selectTenantFirst"
                      )}
                      loadingPlaceholder={t(
                        "productCenter.product.loadingOptions"
                      )}
                      emptyText={t("productCenter.product.emptyOptions")}
                    />
                  ),
                },
                {
                  name: "modelCode",
                  label: t("productCenter.model.modelCode"),
                  placeholder: "HP-200",
                  required: true,
                  trim: true,
                  requiredMessage: t("productCenter.model.codeRequired"),
                },
                {
                  name: "name",
                  label: t("productCenter.model.name"),
                  placeholder: t("productCenter.model.namePlaceholder"),
                  required: true,
                  trim: true,
                  requiredMessage: t("productCenter.model.nameRequired"),
                },
                {
                  name: "versionPolicy",
                  label: t("productCenter.model.versionPolicy"),
                  placeholder: t("productCenter.model.versionPolicyPlaceholder"),
                  trim: true,
                },
                {
                  name: "regionScopeJson",
                  label: t("productCenter.model.regionScope"),
                  type: "json",
                  defaultValue: "[]",
                  rows: 4,
                  colSpan: 2,
                },
              ],
              transformSubmitValues: (values) => ({
                tenantId: Number(values.tenantId),
                productId: Number(values.productId),
                modelCode: String(values.modelCode ?? ""),
                name: String(values.name ?? ""),
                versionPolicy: String(values.versionPolicy ?? ""),
                regionScopeJson: String(values.regionScopeJson ?? "[]"),
              }),
              labels: {
                ...commonFormLabels(t),
                createTitle: t("productCenter.model.createTitle"),
                editTitle: t("productCenter.model.editTitle"),
              },
            }}
            rowActions={[
              createDashboardStatusToggleAction<AdminProductModel, Status>({
                disabled: (item) => item.status === Status.Deleted,
                icon: (item) =>
                  item.status === Status.Ok ? <BanIcon /> : <CheckCircle2Icon />,
                label: (item) =>
                  item.status === Status.Ok
                    ? t("productCenter.disable")
                    : t("productCenter.enable"),
                getNextStatus: (item) =>
                  item.status === Status.Ok ? Status.Disabled : Status.Ok,
                updateStatus: (item, nextStatus) =>
                  updateProductModelStatus(item.id, nextStatus).then((result) => {
                    invalidateProductModels(item.tenantId, item.productId)
                    return result
                  }),
                successMessage: (item, nextStatus) =>
                  t(
                    nextStatus === Status.Ok
                      ? "productCenter.model.enabled"
                      : "productCenter.model.disabled",
                    { name: item.name }
                  ),
                errorMessage: t("productCenter.statusUpdateFailed"),
              }),
            ]}
            labels={commonPageLabels<
              AdminProductModel,
              CreateAdminProductModelPayload
            >(
              t,
              "model",
              (item) => item.name,
              (payload) => payload.name
            )}
          />
          </div>
        ) : null}
        {activeTab === "devices" ? (
          <div>
          <DashboardCrudPage<AdminDevice, CreateAdminDevicePayload>
            layout="fragment"
            filters={[
              {
                name: "tenantId",
                label: t("productCenter.tenantId"),
                placeholder: t("productCenter.tenantId"),
                defaultValue: "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "productId",
                label: t("productCenter.productId"),
                placeholder: t("productCenter.productId"),
                defaultValue: selectedProductId ? String(selectedProductId) : "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "deviceNo",
                label: t("productCenter.device.deviceNo"),
                placeholder: t("productCenter.device.filterDeviceNo"),
                defaultValue: "",
                trim: true,
                className: "w-full sm:w-44",
              },
              {
                name: "serialNo",
                label: t("productCenter.device.serialNo"),
                placeholder: t("productCenter.device.filterSerialNo"),
                defaultValue: "",
                trim: true,
                className: "w-full sm:w-44",
              },
              {
                name: "status",
                label: t("status.all"),
                type: "select",
                defaultValue: "all",
                allValue: "all",
                options: listStatusOptions,
                className: "w-full sm:w-36",
              },
            ]}
            columns={[
              {
                key: "device",
                label: t("productCenter.device.columnDevice"),
                render: (item) => (
                  <div className="min-w-0">
                    <div className="font-medium">{item.deviceNo}</div>
                    <div className="mt-1 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
                      <Badge variant="outline">{item.serialNo || "-"}</Badge>
                      <span>{item.regionCode || "-"}</span>
                    </div>
                  </div>
                ),
              },
              {
                key: "product",
                label: t("productCenter.device.columnProduct"),
                className: "w-60",
                render: (item) => (
                  <DeviceProductModelReference
                    tenantId={item.tenantId}
                    productId={item.productId}
                    productModelId={item.productModelId}
                  />
                ),
              },
              {
                key: "source",
                label: t("productCenter.device.source"),
                className: "w-24",
                render: (item) => item.source || "-",
              },
              statusColumn<AdminDevice>({
                label: t("productCenter.status"),
                getStatus: (item) => item.status as Status,
                getLabel: (status) => getStatusLabel(status, t),
              }),
            ]}
            fetchList={fetchDevices}
            getItemId={(item) => item.id}
            createItem={createDevice}
            updateItem={(item, payload) => updateDevice({ id: item.id, ...payload })}
            deleteItem={(item) => deleteDevice(item.id)}
            canDelete={(item) => item.status !== Status.Deleted}
            form={{
              fetchDetail: fetchDevice,
              fields: [
                {
                  name: "tenantId",
                  label: t("productCenter.tenantId"),
                  type: "number",
                  defaultValue: "1",
                  min: 1,
                  step: 1,
                  valueType: "number",
                  required: true,
                  requiredMessage: t("productCenter.tenantIdRequired"),
                },
                {
                  name: "deviceNo",
                  label: t("productCenter.device.deviceNo"),
                  placeholder: "DEV-1001",
                  required: true,
                  trim: true,
                  requiredMessage: t("productCenter.device.deviceNoRequired"),
                },
                {
                  name: "productId",
                  label: t("productCenter.productId"),
                  type: "custom",
                  defaultValue: "",
                  valueType: "number",
                  required: true,
                  requiredMessage: t("productCenter.productIdRequired"),
                  render: ({ name, value, values, setValue }) => (
                    <ProductSelectField
                      name={name}
                      value={value}
                      values={values}
                      setValue={setValue}
                      placeholder={t("productCenter.product.selectPlaceholder")}
                      tenantFirstPlaceholder={t(
                        "productCenter.product.selectTenantFirst"
                      )}
                      loadingPlaceholder={t(
                        "productCenter.product.loadingOptions"
                      )}
                      emptyText={t("productCenter.product.emptyOptions")}
                      clearModelFieldName="productModelId"
                    />
                  ),
                },
                {
                  name: "productModelId",
                  label: t("productCenter.modelId"),
                  type: "custom",
                  defaultValue: "0",
                  valueType: "number",
                  render: ({ name, value, values, setValue }) => (
                    <ProductModelSelectField
                      name={name}
                      value={value}
                      values={values}
                      setValue={setValue}
                      placeholder={t("productCenter.model.selectPlaceholder")}
                      tenantFirstPlaceholder={t(
                        "productCenter.product.selectTenantFirst"
                      )}
                      productFirstPlaceholder={t(
                        "productCenter.model.selectProductFirst"
                      )}
                      loadingPlaceholder={t("productCenter.model.loadingOptions")}
                      emptyText={t("productCenter.model.emptyOptions")}
                      noneLabel={t("productCenter.model.noModel")}
                    />
                  ),
                },
                {
                  name: "serialNo",
                  label: t("productCenter.device.serialNo"),
                  placeholder: "HP-2026-0001",
                  trim: true,
                },
                {
                  name: "regionCode",
                  label: t("productCenter.device.regionCode"),
                  placeholder: "US-WEST",
                  trim: true,
                },
                {
                  name: "source",
                  label: t("productCenter.device.source"),
                  type: "select",
                  defaultValue: "manual",
                  options: sourceOptions,
                },
                {
                  name: "customerOrgId",
                  label: t("productCenter.device.customerOrgId"),
                  type: "number",
                  defaultValue: "0",
                  min: 0,
                  step: 1,
                  valueType: "number",
                },
                {
                  name: "externalDeviceId",
                  label: t("productCenter.device.externalDeviceId"),
                  placeholder: t("productCenter.device.externalDeviceIdPlaceholder"),
                  trim: true,
                },
                {
                  name: "installLocationJson",
                  label: t("productCenter.device.installLocationJson"),
                  type: "json",
                  defaultValue: "{}",
                  rows: 4,
                  colSpan: 2,
                },
                {
                  name: "metadataJson",
                  label: t("productCenter.device.metadataJson"),
                  type: "json",
                  defaultValue: "{}",
                  rows: 4,
                  colSpan: 2,
                },
              ],
              transformSubmitValues: (values) => ({
                tenantId: Number(values.tenantId),
                deviceNo: String(values.deviceNo ?? ""),
                productId: Number(values.productId),
                productModelId: Number(values.productModelId),
                serialNo: String(values.serialNo ?? ""),
                customerOrgId: Number(values.customerOrgId),
                externalDeviceId: String(values.externalDeviceId ?? ""),
                installLocationJson: String(values.installLocationJson ?? "{}"),
                regionCode: String(values.regionCode ?? ""),
                source: String(values.source ?? "manual"),
                metadataJson: String(values.metadataJson ?? "{}"),
              }),
              labels: {
                ...commonFormLabels(t),
                createTitle: t("productCenter.device.createTitle"),
                editTitle: t("productCenter.device.editTitle"),
              },
            }}
            rowActions={[
              {
                key: "repairHistory",
                icon: <HistoryIcon />,
                label: t("productCenter.device.openRepairHistory"),
                run: ({ item }) =>
                  openProductDossier(item.productId, "repairHistory", item.id),
              },
              createDashboardStatusToggleAction<AdminDevice, Status>({
                disabled: (item) => item.status === Status.Deleted,
                icon: (item) =>
                  item.status === Status.Ok ? <BanIcon /> : <CheckCircle2Icon />,
                label: (item) =>
                  item.status === Status.Ok
                    ? t("productCenter.disable")
                    : t("productCenter.enable"),
                getNextStatus: (item) =>
                  item.status === Status.Ok ? Status.Disabled : Status.Ok,
                updateStatus: (item, nextStatus) =>
                  updateDeviceStatus(item.id, nextStatus),
                successMessage: (item, nextStatus) =>
                  t(
                    nextStatus === Status.Ok
                      ? "productCenter.device.enabled"
                      : "productCenter.device.disabled",
                    { name: item.deviceNo }
                  ),
                errorMessage: t("productCenter.statusUpdateFailed"),
              }),
            ]}
            labels={commonPageLabels<AdminDevice, CreateAdminDevicePayload>(
              t,
              "device",
              (item) => item.deviceNo,
              (payload) => payload.deviceNo
            )}
          />
          </div>
        ) : null}
        {activeTab === "serviceCodes" ? (
          <div className="space-y-6">
          <DashboardCrudPage<AdminServiceCodeBatch, CreateAdminServiceCodeBatchPayload>
            layout="fragment"
            filters={[
              {
                name: "tenantId",
                label: t("productCenter.tenantId"),
                placeholder: t("productCenter.tenantId"),
                defaultValue: "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "productId",
                label: t("productCenter.productId"),
                placeholder: t("productCenter.productId"),
                defaultValue: selectedProductId ? String(selectedProductId) : "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "batchNo",
                label: t("productCenter.batch.batchNo"),
                placeholder: t("productCenter.batch.filterBatchNo"),
                defaultValue: "",
                trim: true,
                className: "w-full sm:w-44",
              },
              {
                name: "mode",
                label: t("productCenter.serviceCode.mode"),
                type: "select",
                defaultValue: "all",
                allValue: "all",
                options: [
                  { value: "all", label: t("status.all") },
                  ...modeOptions,
                ],
                className: "w-full sm:w-40",
              },
              {
                name: "status",
                label: t("status.all"),
                type: "select",
                defaultValue: "all",
                allValue: "all",
                options: listStatusOptions,
                className: "w-full sm:w-36",
              },
            ]}
            columns={[
              {
                key: "batch",
                label: t("productCenter.batch.columnBatch"),
                render: (item) => (
                  <div className="min-w-0">
                    <div className="font-medium">{item.batchNo}</div>
                    <div className="mt-1 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
                      <Badge variant="outline">
                        {getServiceCodeModeLabel(item.mode, t)}
                      </Badge>
                      <span>
                        {item.generatedCount}/{item.quantity}
                      </span>
                    </div>
                  </div>
                ),
              },
              {
                key: "product",
                label: t("productCenter.device.columnProduct"),
                className: "w-60",
                render: (item) => (
                  <DeviceProductModelReference
                    tenantId={item.tenantId}
                    productId={item.productId}
                    productModelId={item.productModelId}
                  />
                ),
              },
              {
                key: "exported",
                label: t("productCenter.batch.exportedCount"),
                className: "w-24",
                render: (item) => item.exportedCount,
              },
              statusColumn<AdminServiceCodeBatch>({
                label: t("productCenter.status"),
                getStatus: (item) => item.status as Status,
                getLabel: (status) => getStatusLabel(status, t),
              }),
            ]}
            fetchList={fetchServiceCodeBatches}
            getItemId={(item) => item.id}
            createItem={createServiceCodeBatch}
            updateItem={async () => undefined}
            canEdit={() => false}
            deleteItem={(item) => deleteServiceCodeBatch(item.id)}
            canDelete={(item) => item.status !== Status.Deleted}
            form={{
              fetchDetail: fetchServiceCodeBatch,
              fields: [
                {
                  name: "tenantId",
                  label: t("productCenter.tenantId"),
                  type: "number",
                  defaultValue: "1",
                  min: 1,
                  step: 1,
                  valueType: "number",
                  required: true,
                  requiredMessage: t("productCenter.tenantIdRequired"),
                },
                {
                  name: "batchNo",
                  label: t("productCenter.batch.batchNo"),
                  placeholder: "BATCH-202607",
                  required: true,
                  trim: true,
                  requiredMessage: t("productCenter.batch.batchNoRequired"),
                },
                {
                  name: "mode",
                  label: t("productCenter.serviceCode.mode"),
                  type: "select",
                  defaultValue: "traceable",
                  options: modeOptions,
                },
                {
                  name: "productId",
                  label: t("productCenter.productId"),
                  type: "custom",
                  defaultValue: "",
                  valueType: "number",
                  required: true,
                  requiredMessage: t("productCenter.productIdRequired"),
                  render: ({ name, value, values, setValue }) => (
                    <ProductSelectField
                      name={name}
                      value={value}
                      values={values}
                      setValue={setValue}
                      placeholder={t("productCenter.product.selectPlaceholder")}
                      tenantFirstPlaceholder={t(
                        "productCenter.product.selectTenantFirst"
                      )}
                      loadingPlaceholder={t(
                        "productCenter.product.loadingOptions"
                      )}
                      emptyText={t("productCenter.product.emptyOptions")}
                      clearModelFieldName="productModelId"
                    />
                  ),
                },
                {
                  name: "productModelId",
                  label: t("productCenter.modelId"),
                  type: "custom",
                  defaultValue: "0",
                  valueType: "number",
                  render: ({ name, value, values, setValue }) => (
                    <ProductModelSelectField
                      name={name}
                      value={value}
                      values={values}
                      setValue={setValue}
                      placeholder={t("productCenter.model.selectPlaceholder")}
                      tenantFirstPlaceholder={t(
                        "productCenter.product.selectTenantFirst"
                      )}
                      productFirstPlaceholder={t(
                        "productCenter.model.selectProductFirst"
                      )}
                      loadingPlaceholder={t("productCenter.model.loadingOptions")}
                      emptyText={t("productCenter.model.emptyOptions")}
                      noneLabel={t("productCenter.model.noModel")}
                    />
                  ),
                },
                {
                  name: "quantity",
                  label: t("productCenter.batch.quantity"),
                  type: "number",
                  defaultValue: "100",
                  min: 1,
                  step: 1,
                  valueType: "number",
                  required: true,
                  requiredMessage: t("productCenter.batch.quantityRequired"),
                },
                {
                  name: "labelTemplateId",
                  label: t("productCenter.batch.labelTemplateId"),
                  type: "number",
                  defaultValue: "0",
                  min: 0,
                  step: 1,
                  valueType: "number",
                },
              ],
              transformSubmitValues: (values) => ({
                tenantId: Number(values.tenantId),
                batchNo: String(values.batchNo ?? ""),
                mode: String(values.mode ?? "traceable"),
                productId: Number(values.productId),
                productModelId: Number(values.productModelId),
                quantity: Number(values.quantity),
                labelTemplateId: Number(values.labelTemplateId),
              }),
              labels: {
                ...commonFormLabels(t),
                createTitle: t("productCenter.batch.createTitle"),
                editTitle: t("productCenter.batch.editTitle"),
              },
            }}
            rowActions={[
              {
                key: "generate",
                icon: <RefreshCwIcon />,
                label: t("productCenter.batch.generate"),
                disabled: (item) =>
                  item.status !== Status.Ok || item.generatedCount >= item.quantity,
                confirm: (item) => ({
                  title: t("productCenter.batch.generateConfirmTitle"),
                  description: t("productCenter.batch.generateConfirmDescription", {
                    count: Math.min(item.quantity - item.generatedCount, 10),
                    batchNo: item.batchNo,
                  }),
                  confirmText: t("productCenter.batch.generate"),
                }),
                run: async ({ item, reload }) => {
                  const count = Math.min(item.quantity - item.generatedCount, 10)
                  if (count <= 0) return
                  const codes = await generateServiceCodes(item.id, count)
                  toast.success(
                    t("productCenter.batch.generated", {
                      count: codes.length,
                      batchNo: item.batchNo,
                    })
                  )
                  await reload()
                },
              },
              createDashboardStatusToggleAction<AdminServiceCodeBatch, Status>({
                disabled: (item) => item.status === Status.Deleted,
                icon: (item) =>
                  item.status === Status.Ok ? <BanIcon /> : <CheckCircle2Icon />,
                label: (item) =>
                  item.status === Status.Ok
                    ? t("productCenter.disable")
                    : t("productCenter.enable"),
                getNextStatus: (item) =>
                  item.status === Status.Ok ? Status.Disabled : Status.Ok,
                updateStatus: (item, nextStatus) =>
                  updateServiceCodeBatchStatus(item.id, nextStatus),
                successMessage: (item, nextStatus) =>
                  t(
                    nextStatus === Status.Ok
                      ? "productCenter.batch.enabled"
                      : "productCenter.batch.disabled",
                    { name: item.batchNo }
                  ),
                errorMessage: t("productCenter.statusUpdateFailed"),
              }),
            ]}
            labels={commonPageLabels<
              AdminServiceCodeBatch,
              CreateAdminServiceCodeBatchPayload
            >(
              t,
              "batch",
              (item) => item.batchNo,
              (payload) => payload.batchNo
            )}
          />

          <DashboardCrudPage<AdminServiceCode, Record<string, never>>
            layout="fragment"
            showToolbarActions={false}
            filters={[
              {
                name: "tenantId",
                label: t("productCenter.tenantId"),
                placeholder: t("productCenter.tenantId"),
                defaultValue: "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "batchId",
                label: t("productCenter.serviceCode.batchId"),
                placeholder: t("productCenter.serviceCode.batchId"),
                defaultValue: "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "productId",
                label: t("productCenter.productId"),
                placeholder: t("productCenter.productId"),
                defaultValue: selectedProductId ? String(selectedProductId) : "",
                valueType: "number",
                trim: true,
                className: "w-full sm:w-32",
              },
              {
                name: "serviceCode",
                label: t("productCenter.serviceCode.serviceCode"),
                placeholder: t("productCenter.serviceCode.filterCode"),
                defaultValue: "",
                trim: true,
                className: "w-full sm:w-48",
              },
              {
                name: "status",
                label: t("status.all"),
                type: "select",
                defaultValue: "all",
                allValue: "all",
                options: serviceCodeListStatusOptions,
                className: "w-full sm:w-36",
              },
            ]}
            columns={[
              {
                key: "serviceCode",
                label: t("productCenter.serviceCode.columnCode"),
                render: (item) => (
                  <div className="min-w-0">
                    <div className="font-mono font-medium">{item.serviceCode}</div>
                    <div className="mt-1 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
                      <Badge variant="outline">
                        {getServiceCodeModeLabel(item.mode, t)}
                      </Badge>
                      <span>#{item.batchId}</span>
                    </div>
                  </div>
                ),
              },
              {
                key: "product",
                label: t("productCenter.device.columnProduct"),
                className: "w-60",
                render: (item) => (
                  <DeviceProductModelReference
                    tenantId={item.tenantId}
                    productId={item.productId}
                    productModelId={item.productModelId}
                  />
                ),
              },
              {
                key: "deviceId",
                label: t("productCenter.deviceId"),
                className: "w-24",
                render: (item) => (item.deviceId > 0 ? `#${item.deviceId}` : "-"),
              },
              {
                key: "status",
                label: t("productCenter.status"),
                className: "w-28",
                render: (item) => (
                  <Badge
                    variant={item.status === "active" ? "default" : "outline"}
                  >
                    {getServiceCodeStatusLabel(item.status, t)}
                  </Badge>
                ),
              },
            ]}
            fetchList={fetchServiceCodes}
            getItemId={(item) => item.id}
            createItem={async () => undefined}
            updateItem={async () => undefined}
            canEdit={() => false}
            form={{
              fetchDetail: fetchServiceCode,
              fields: [],
              transformSubmitValues: () => ({}),
              labels: {
                ...commonFormLabels(t),
                createTitle: t("productCenter.code.createTitle"),
                editTitle: t("productCenter.code.editTitle"),
              },
            }}
            rowActions={[
              {
                key: "deviceRepairHistory",
                icon: <HistoryIcon />,
                label: t("productCenter.device.openRepairHistory"),
                disabled: (item) => item.deviceId <= 0,
                run: ({ item }) =>
                  openProductDossier(item.productId, "repairHistory", item.deviceId),
              },
              {
                key: "revoke",
                icon: <BanIcon />,
                label: t("productCenter.serviceCode.revoke"),
                variant: "destructive",
                disabled: (item) => item.status !== "active",
                confirm: (item) => ({
                  title: t("productCenter.serviceCode.revokeConfirmTitle"),
                  description: t(
                    "productCenter.serviceCode.revokeConfirmDescription",
                    { code: item.serviceCode }
                  ),
                  confirmText: t("productCenter.serviceCode.revoke"),
                  variant: "destructive",
                }),
                run: async ({ item, reload }) => {
                  await revokeServiceCode(item.id)
                  toast.success(
                    t("productCenter.serviceCode.revoked", {
                      code: item.serviceCode,
                    })
                  )
                  await reload()
                },
              },
            ]}
            labels={commonPageLabels<AdminServiceCode, Record<string, never>>(
              t,
              "code",
              (item) => item.serviceCode,
              () => ""
            )}
          />
          </div>
        ) : null}
        {activeTab === "tickets" ? (
          <div>
          <ProductTicketListPanel
            productId={selectedProductId}
            mode="tickets"
            t={t}
          />
          </div>
        ) : null}
        {activeTab === "repairHistory" ? (
          <div>
          <ProductTicketListPanel
            productId={selectedProductId}
            deviceId={selectedDeviceId}
            mode="repairHistory"
            t={t}
          />
          </div>
        ) : null}
        {activeTab === "conversations" ? (
          <div>
          <ProductConversationsPanel productId={selectedProductId} t={t} />
          </div>
        ) : null}
        {activeTab === "knowledge" ? (
          <div>
          <ProductKnowledgePanel productId={selectedProductId} t={t} />
          </div>
        ) : null}
      </div>
    </DashboardPage>
  )
}
