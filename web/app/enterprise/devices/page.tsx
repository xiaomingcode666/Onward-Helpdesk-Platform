"use client"

import {
  CopyIcon,
  CpuIcon,
  DownloadIcon,
  EyeIcon,
  FileClockIcon,
  PencilIcon,
  PlusIcon,
  PrinterIcon,
  QrCodeIcon,
  RefreshCwIcon,
  UploadIcon,
  UsersIcon,
  WrenchIcon,
} from "lucide-react"
import Link from "next/link"
import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type ReactNode } from "react"
import { toast } from "sonner"
import { Button as AntButton, Empty as AntEmpty, Pagination as AntPagination, QRCode, Table as AntTable } from "antd"
import type { TableColumnsType } from "antd"
import {
  ContentModule,
  DetailDrawer,
  StandardModal,
  FilterTabs,
  SearchField,
  SelectField,
  StatusTag,
  TableToolbar,
  type RailopsTabItem,
  type StatusTagTone,
} from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { PageHeader } from "@/components/layout/page-header"
import { CanUseButton } from "@/components/layout/permission-guard"
import { ProductTree, buildProductTreeGroups, getProductTreeDisplayName } from "@/components/product/product-tree"
import { EmptyState, ErrorState } from "@/components/shared/error-states"
import { useI18n } from "@/i18n/provider"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { Textarea } from "@/components/ui/textarea"
import { batchImportDevices, createDevice, fetchDevice, fetchDevices, updateDevice } from "@/lib/api/devices"
import { fetchProductCount, listAllProducts, listProductModels, type ProductModel } from "@/lib/api/enterprise-products"
import type { BatchImportDevicePayload, DeviceBatchImportResult, DeviceDetailDTO, DeviceListItem, ProductListItem, UpdateDevicePayload } from "@/lib/api/types"
import { customerDisplayName } from "@/lib/customer-identity"
import { buildEnterpriseTicketWorkbenchPath } from "@/lib/ticket-workbench-route"
import { cn } from "@/lib/utils"

const PAGE_SIZE = 12
type DeviceLoadOptions = { productId?: string; search?: string; selectedId?: string; page?: number }
type DeviceDetailTab = "bindings" | "tickets" | "repairs"
type I18nT = ReturnType<typeof useI18n>

const emptyDeviceForm = {
  service_code: "",
  product_id: "",
  model_id: "0",
  region_code: "",
  install_date: "",
  warranty_end: "",
  description: "",
}

type DeviceFormState = typeof emptyDeviceForm

function dateText(value?: string) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value.slice(0, 10)
  return date.toLocaleDateString()
}

function dateInputValue(value?: string) {
  if (!value) return ""
  const match = value.match(/^\d{4}-\d{2}-\d{2}/)
  if (match) return match[0]
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ""
  return date.toISOString().slice(0, 10)
}

function extractDeviceDescription(value?: string) {
  const raw = value?.trim() || ""
  if (!raw) return ""
  try {
    const parsed = JSON.parse(raw) as { description?: unknown }
    return typeof parsed.description === "string" ? parsed.description : raw
  } catch {
    return raw
  }
}

function buildDeviceBasePayload(form: DeviceFormState, includeEmptyModel = false): UpdateDevicePayload {
  const productId = Number(form.product_id)
  const modelId = Number(form.model_id)
  return {
    ...(Number.isFinite(productId) && productId > 0 ? { product_id: productId } : {}),
    ...(Number.isFinite(modelId) && modelId > 0 ? { model_id: modelId } : includeEmptyModel ? { model_id: 0 } : {}),
    region_code: form.region_code.trim(),
    install_date: form.install_date,
    warranty_end: form.warranty_end,
    description: form.description.trim(),
  }
}

function deviceStatusLabel(t: I18nT, status?: string) {
  switch (status) {
    case "active":
      return t("enterpriseDevices.status.active")
    case "maintenance":
      return t("enterpriseDevices.status.maintenance")
    case "inactive":
      return t("enterpriseDevices.status.inactive")
    case "operational":
      return t("enterpriseDevices.status.operational")
    case "archived":
      return t("enterpriseDevices.status.archived")
    case "decommissioned":
      return t("enterpriseDevices.status.decommissioned")
    default:
      return status || "-"
  }
}

function warrantyLabel(t: I18nT, status?: string) {
  switch (status) {
    case "active":
      return t("enterpriseDevices.warranty.active")
    case "expiring":
      return t("enterpriseDevices.warranty.expiring")
    case "expired":
      return t("enterpriseDevices.warranty.expired")
    default:
      return t("enterpriseDevices.warranty.unknown")
  }
}

function toneClass(tone: "green" | "amber" | "red" | "blue" | "slate") {
  switch (tone) {
    case "green":
      return "border-primary/20 bg-primary/10 text-primary"
    case "amber":
      return "border-amber-200 bg-amber-50 text-foreground"
    case "red":
      return "border-border bg-muted text-muted-foreground"
    case "blue":
      return "border-primary/20 bg-primary/10 text-primary"
    default:
      return "border-border bg-muted text-muted-foreground"
  }
}

function statusTone(status?: string) {
  if (status === "active" || status === "operational") return toneClass("green")
  if (status === "maintenance") return toneClass("amber")
  if (status === "inactive" || status === "archived" || status === "decommissioned") return toneClass("slate")
  return toneClass("blue")
}

function formatProductOptionLabel(t: I18nT, product: ProductListItem) {
  const label = getProductTreeDisplayName(product)
  const code = product.code?.trim()
  const parts = [label, code && code !== label ? code : ""].filter((part): part is string => Boolean(part))
  if (parts.length > 0) {
    return parts.join(" · ")
  }
  return t("enterpriseDevices.productFallback", { id: product.id })
}

function formatSelectedProductLabel(t: I18nT, products: ProductListItem[], productId: string, emptyLabel: string, zeroLabel: string) {
  if (!productId) {
    return emptyLabel
  }
  if (productId === "0") {
    return zeroLabel
  }
  const product = products.find((item) => String(item.id) === productId)
  return product ? formatProductOptionLabel(t, product) : t("enterpriseDevices.productFallback", { id: productId })
}

function statusTagTone(status?: string): StatusTagTone {
  if (status === "active" || status === "operational") return "success"
  if (status === "maintenance") return "warning"
  if (status === "inactive" || status === "archived" || status === "decommissioned") return "neutral"
  return "blue"
}

function warrantyTagTone(status?: string): StatusTagTone {
  if (status === "active") return "success"
  if (status === "expiring") return "warning"
  if (status === "expired") return "error"
  return "neutral"
}

function ticketStatusLabel(t: I18nT, status?: string) {
  switch (status) {
    case "pending_acceptance":
      return t("enterpriseDevices.ticketStatus.pendingAcceptance")
    case "pending_dispatch":
      return t("enterpriseDevices.ticketStatus.pendingDispatch")
    case "accepted":
      return t("enterpriseDevices.ticketStatus.accepted")
    case "pending_assignee_accept":
      return t("enterpriseDevices.ticketStatus.pendingAssigneeAccept")
    case "in_progress":
    case "processing":
      return t("enterpriseDevices.ticketStatus.processing")
    case "video_support":
      return t("enterpriseDevices.ticketStatus.videoSupport")
    case "supplier_support":
      return t("enterpriseDevices.ticketStatus.supplierSupport")
    case "resolved":
    case "pending_customer_confirm":
      return t("enterpriseDevices.ticketStatus.pendingCustomerConfirm")
    case "closed":
    case "done":
      return t("enterpriseDevices.ticketStatus.closed")
    case "reopened":
      return t("enterpriseDevices.ticketStatus.reopened")
    case "cancelled":
      return t("enterpriseDevices.ticketStatus.cancelled")
    default:
      return status || "-"
  }
}

function buildFallbackServiceCodeEntryURL(serviceCode?: string) {
  const code = serviceCode?.trim()
  return code ? `/mobile?serviceCode=${encodeURIComponent(code)}` : ""
}

function normalizeQRCodeURL(value: string) {
  if (!value.trim()) return ""
  if (typeof window === "undefined") return value
  try {
    return new URL(value, window.location.origin).toString()
  } catch {
    return value
  }
}

function deviceServiceCodeEntryURL(detail: Pick<DeviceDetailDTO, "entry_url" | "qr_url" | "service_code">) {
  return normalizeQRCodeURL(detail.qr_url || detail.entry_url || buildFallbackServiceCodeEntryURL(detail.service_code))
}

function drawCenteredText(
  context: CanvasRenderingContext2D,
  text: string,
  y: number,
  maxWidth: number,
) {
  const source = text.trim()
  if (!source) return
  let output = source
  while (context.measureText(output).width > maxWidth && output.length > 1) {
    output = output.slice(0, -2)
  }
  if (output !== source) output += "..."
  context.fillText(output, context.canvas.width / 2, y)
}

function buildServiceCodeLabelDataURL({
  detail,
  entryURL,
  qrCanvas,
  t,
}: {
  detail: DeviceDetailDTO
  entryURL: string
  qrCanvas: HTMLCanvasElement
  t: I18nT
}) {
  const canvas = document.createElement("canvas")
  canvas.width = 720
  canvas.height = 960
  const context = canvas.getContext("2d")
  if (!context) return ""
  context.fillStyle = "#ffffff"
  context.fillRect(0, 0, canvas.width, canvas.height)
  context.strokeStyle = "#d8dee8"
  context.lineWidth = 3
  context.strokeRect(24, 24, canvas.width - 48, canvas.height - 48)
  context.fillStyle = "#111827"
  context.textAlign = "center"
  context.textBaseline = "middle"
  context.font = "700 40px Arial, sans-serif"
  drawCenteredText(context, t("enterpriseDevices.detail.qrLabelTitle"), 94, 600)
  context.font = "700 34px Arial, sans-serif"
  drawCenteredText(context, detail.service_code || detail.device_no, 150, 600)
  context.drawImage(qrCanvas, 180, 208, 360, 360)
  context.fillStyle = "#374151"
  context.font = "600 24px Arial, sans-serif"
  drawCenteredText(context, detail.product_name || "-", 628, 600)
  context.fillStyle = "#6b7280"
  context.font = "400 22px Arial, sans-serif"
  drawCenteredText(context, detail.model_name || detail.region_code || "-", 674, 600)
  context.font = "400 18px Arial, sans-serif"
  drawCenteredText(context, entryURL, 742, 620)
  context.fillStyle = "#111827"
  context.font = "600 20px Arial, sans-serif"
  drawCenteredText(context, t("enterpriseDevices.detail.qrScanHint"), 834, 600)
  return canvas.toDataURL("image/png")
}

function bindingStatusLabel(t: I18nT, status?: string) {
  switch (status) {
    case "active":
      return t("enterpriseDevices.detail.bindingStatusActive")
    case "inactive":
      return t("enterpriseDevices.detail.bindingStatusInactive")
    default:
      return status || "-"
  }
}

export default function EnterpriseDevicesPage() {
  const { session } = useAuth()
  const t = useI18n()
  const [devices, setDevices] = useState<DeviceListItem[]>([])
  const [deviceTotal, setDeviceTotal] = useState(0)
  const [products, setProducts] = useState<ProductListItem[]>([])
  const [productTotal, setProductTotal] = useState(0)
  const [selectedDeviceId, setSelectedDeviceId] = useState("")
  const [selectedDevice, setSelectedDevice] = useState<DeviceDetailDTO | null>(null)
  const [productsLoaded, setProductsLoaded] = useState(false)
  const [productSearch, setProductSearch] = useState("")
  const [search, setSearch] = useState("")
  const [productId, setProductId] = useState("")
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [devicesLoaded, setDevicesLoaded] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [batchOpen, setBatchOpen] = useState(false)
  const [editingDevice, setEditingDevice] = useState<DeviceDetailDTO | null>(null)
  const [error, setError] = useState("")
  const [detailError, setDetailError] = useState("")

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const value = params.get("productId") || params.get("product_id")
    if (value) setProductId(value)
    const keyword = params.get("search")
    if (keyword) setSearch(keyword)
  }, [])

  const loadCatalog = useCallback(async () => {
    try {
      const [productList, countResponse] = await Promise.all([
        listAllProducts(),
        fetchProductCount().catch(() => null),
      ])
      setProducts(productList)
      setProductTotal(countResponse?.success && countResponse.data ? countResponse.data.total : productList.length)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("enterpriseDevices.loadCatalogFailed"))
    } finally {
      setProductsLoaded(true)
    }
  }, [t])

  const loadDevices = useCallback(async (options?: DeviceLoadOptions) => {
    setLoading(true)
    setError("")
    const activeProductId = options?.productId ?? productId
    const activeSearch = options?.search ?? search
    const activePage = options?.page ?? 1
    try {
      const res = await fetchDevices({
        product_id: activeProductId ? Number(activeProductId) : undefined,
        search: activeSearch,
        page: activePage,
        page_size: PAGE_SIZE,
      })
      if (!res.success || !res.data) {
        throw new Error(res.error?.message || t("enterpriseDevices.loadDevicesFailed"))
      }
      setDevices(res.data.items)
      setDeviceTotal(res.data.total)
      setSelectedDeviceId((current) => {
        if (options?.selectedId && res.data.items.some((item) => String(item.id) === options.selectedId)) return options.selectedId
        if (current && res.data.items.some((item) => String(item.id) === current)) return current
        return ""
      })
      setDevicesLoaded(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("enterpriseDevices.loadDevicesFailed"))
    } finally {
      setLoading(false)
    }
  }, [productId, search, t])

  useEffect(() => {
    void loadCatalog()
  }, [loadCatalog])

  useEffect(() => {
    setPage(1)
    void loadDevices({ page: 1 })
  }, [loadDevices])

  useEffect(() => {
    if (!selectedDeviceId) {
      setSelectedDevice(null)
      return
    }
    setDetailLoading(true)
    setDetailError("")
    fetchDevice(Number(selectedDeviceId))
      .then((res) => {
        if (res.success && res.data) {
          setSelectedDevice(res.data)
        } else {
          setSelectedDevice(null)
          setDetailError(res.error?.message || t("enterpriseDevices.loadDetailFailed"))
        }
      })
      .catch((err) => {
        setSelectedDevice(null)
        setDetailError(err instanceof Error ? err.message : t("enterpriseDevices.loadDetailFailed"))
      })
      .finally(() => setDetailLoading(false))
  }, [selectedDeviceId, t])

  const filteredProducts = useMemo(() => {
    const keyword = productSearch.trim().toLowerCase()
    if (!keyword) return products
    return products.filter((product) => {
      const label = getProductTreeDisplayName(product)
      return `${label} ${product.code} ${product.product_line} ${product.category}`.toLowerCase().includes(keyword)
    })
  }, [productSearch, products])
  const productTreeGroups = useMemo(() => buildProductTreeGroups(filteredProducts), [filteredProducts])
  const selectedProduct = productId
    ? products.find((item) => String(item.id) === productId)
    : null
  const selectedProductId = productId && Number.isFinite(Number(productId)) ? Number(productId) : null
  const unboundDeviceCount = devices.filter((item) => (item.binding_count || 0) === 0).length
  const canCreateDevice = CanUseButton("device.create", session?.permissions)
  const canUpdateDevice = CanUseButton("device.update", session?.permissions)
  const hasDevices = devices.length > 0
  const initialDeviceListLoading = loading && !devicesLoaded
  const deviceListRefreshing = loading && devicesLoaded
  const blockingDeviceError = Boolean(error) && !loading && !hasDevices
  const deviceRefreshError = Boolean(error) && !loading && hasDevices
  const deviceColumns = useMemo<TableColumnsType<DeviceListItem>>(() => [
    {
      title: t("enterpriseDevices.columns.device"),
      key: "device",
      width: 142,
      render: (_, device) => (
        <div className="rhd-railops-table-cell">
          <strong className="rhd-railops-service-code">{device.service_code || device.device_no}</strong>
          <span>{device.region_code || t("enterpriseDevices.regionUnset")}</span>
        </div>
      ),
    },
    {
      title: t("enterpriseDevices.columns.productModel"),
      key: "productModel",
      width: 158,
      render: (_, device) => {
        const productModelMeta = [device.product_code?.trim(), device.model_name?.trim()]
          .filter((value): value is string => Boolean(value))
          .join(" · ")
        return (
          <div className="rhd-railops-table-cell">
            <strong>{device.product_name || "-"}</strong>
            <span>{productModelMeta || "-"}</span>
          </div>
        )
      },
    },
    {
      title: t("enterpriseDevices.columns.customerOwnership"),
      key: "customer",
      width: 130,
      render: (_, device) => (
        <div className="rhd-railops-table-cell">
          <strong>{customerDisplayName(device.customer_name)}</strong>
          <span>{t("enterpriseDevices.bindingCount", { count: device.binding_count || 0 })}</span>
        </div>
      ),
    },
    {
      title: t("enterpriseDevices.columns.region"),
      dataIndex: "region_code",
      width: 56,
      align: "center",
      render: (value?: string) => value || "-",
    },
    {
      title: t("enterpriseDevices.columns.tickets"),
      key: "tickets",
      width: 52,
      align: "center",
      render: (_, device) => (
        <span className="rhd-railops-number-cell">
          <strong>{device.open_ticket_count || 0}</strong>
          <span>/ {device.total_ticket_count || 0}</span>
        </span>
      ),
    },
    {
      title: t("enterpriseDevices.columns.meeting"),
      dataIndex: "meeting_count",
      width: 52,
      align: "center",
      render: (value?: number) => value || 0,
    },
    {
      title: t("enterpriseDevices.columns.warranty"),
      key: "warranty",
      width: 78,
      align: "center",
      render: (_, device) => (
        <div className="rhd-railops-centered-cell">
          <StatusTag tone={warrantyTagTone(device.warranty_status)}>
            {warrantyLabel(t, device.warranty_status)}
          </StatusTag>
          <span>{dateText(device.warranty_end)}</span>
        </div>
      ),
    },
    {
      title: t("enterpriseDevices.columns.status"),
      key: "status",
      width: 64,
      align: "center",
      render: (_, device) => (
        <StatusTag tone={statusTagTone(device.status)}>
          {deviceStatusLabel(t, device.status)}
        </StatusTag>
      ),
    },
    {
      title: "",
      key: "actions",
      width: 44,
      align: "center",
      render: (_, device) => (
        <AntButton
          aria-label={t("common.showDetail")}
          className="rhd-railops-row-action"
          icon={<EyeIcon className="size-4" />}
          size="small"
          title={t("common.showDetail")}
          type="text"
          onClick={(event) => {
            event.stopPropagation()
            setSelectedDeviceId(String(device.id))
          }}
        />
      ),
    },
  ], [t])

  const refreshAll = useCallback(async () => {
    await Promise.all([loadCatalog(), loadDevices({ page })])
  }, [loadCatalog, loadDevices, page])

  const handleSelectProduct = useCallback((nextProductId: string) => {
    setProductId(nextProductId)
    setPage(1)
    setSelectedDeviceId("")
  }, [])

  const handlePageChange = useCallback((nextPage: number) => {
    setPage(nextPage)
    void loadDevices({ page: nextPage })
  }, [loadDevices])

  const handleDeviceCreated = useCallback(async (device: DeviceListItem) => {
    const nextProductId = device.product_id ? String(device.product_id) : ""
    setProductId(nextProductId)
    setSearch("")
    setPage(1)
    await Promise.all([
      loadCatalog(),
      loadDevices({ productId: nextProductId, search: "", selectedId: String(device.id) }),
    ])
  }, [loadCatalog, loadDevices])

  const handleBatchImported = useCallback(async (result: DeviceBatchImportResult) => {
    const firstCreated = result.created?.[0]
    if (!firstCreated) {
      await refreshAll()
      return
    }
    const nextProductId = firstCreated.product_id ? String(firstCreated.product_id) : ""
    setProductId(nextProductId)
    setSearch("")
    setPage(1)
    await Promise.all([
      loadCatalog(),
      loadDevices({ productId: nextProductId, search: "", selectedId: String(firstCreated.id) }),
    ])
  }, [loadCatalog, loadDevices, refreshAll])

  const handleDeviceUpdated = useCallback(async (detail: DeviceDetailDTO) => {
    const nextProductId = detail.product_id ? String(detail.product_id) : productId
    setSelectedDevice(detail)
    setSelectedDeviceId(String(detail.id))
    setProductId(nextProductId)
    setSearch("")
    setPage(1)
    await Promise.all([
      loadCatalog(),
      loadDevices({ productId: nextProductId, search: "", selectedId: String(detail.id), page: 1 }),
    ])
  }, [loadCatalog, loadDevices, productId])

  return (
    <div className="space-y-4">
      <PageHeader
        title={t("enterpriseDevices.pageTitle")}
        actions={
          <>
            {canCreateDevice ? (
              <Button onClick={() => setCreateOpen(true)}>
                <PlusIcon className="size-4" />
                {t("enterpriseDevices.addDevice")}
              </Button>
            ) : null}
            {canCreateDevice ? (
              <Button variant="outline" onClick={() => setBatchOpen(true)}>
                <UploadIcon className="size-4" />
                {t("enterpriseDevices.batchImport")}
              </Button>
            ) : null}
            <Button variant="outline" onClick={refreshAll}>
              <RefreshCwIcon className="size-4" />
              {t("enterpriseDevices.refresh")}
            </Button>
          </>
        }
      />

      <CreateDeviceDialog
        open={createOpen}
        products={products}
        initialProductId={productId}
        onCreated={handleDeviceCreated}
        onOpenChange={setCreateOpen}
      />
      <BatchImportDevicesDialog
        open={batchOpen}
        products={products}
        initialProductId={productId}
        onImported={handleBatchImported}
        onOpenChange={setBatchOpen}
      />
      <EditDeviceDialog
        open={Boolean(editingDevice)}
        detail={editingDevice}
        products={products}
        onUpdated={handleDeviceUpdated}
        onOpenChange={(open) => {
          if (!open) setEditingDevice(null)
        }}
      />

      <section className="rhd-railops-device-workspace">
        <div className="rhd-railops-product-panel rhd-railops-product-directory-panel rhd-railops-device-product-panel">
          <ProductTree
            title={t("enterpriseDevices.productPanelTitle")}
            totalCount={productTotal}
            countLabel={t("enterpriseDevices.productCount", { count: productTotal })}
            groups={productTreeGroups}
            mode="directory"
            rootLabel={t("enterpriseDevices.allProducts")}
            loading={!productsLoaded}
            selectedProductId={selectedProductId}
            rootSelected={!productId}
            search={productSearch}
            searchAriaLabel={t("enterpriseDevices.searchProducts")}
            searchPlaceholder={`${t("enterpriseDevices.searchProducts")}...`}
            onSearchChange={setProductSearch}
            onSelectRoot={() => handleSelectProduct("")}
            onSelectProduct={(product) => handleSelectProduct(String(product.id))}
            emptyText={t("enterpriseDevices.noMatchedProducts")}
            renderProductMeta={(product) => <small className="kb-product-code">{product.code || `PROD-${product.id}`}</small>}
            renderProductBadge={(product) => (
              <span className={cn("ent-product-list-badge", (product.device_count || 0) === 0 && "muted")}>
                {t("enterpriseDevices.boundCount", { count: product.device_count || 0 })}
              </span>
            )}
          />
        </div>
        <ContentModule
          className="rhd-railops-device-list-module"
          title={t("enterpriseDevices.workbenchTitle")}
          note={selectedProduct ? (selectedProduct.code || `PROD-${selectedProduct.id}`) : t("enterpriseDevices.allProducts")}
          extra={
            <StatusTag tone="blue">
              {selectedProduct ? formatProductOptionLabel(t, selectedProduct) : t("enterpriseDevices.allProducts")}
            </StatusTag>
          }
        >
          <TableToolbar
            search={
              <SearchField
                allowClear
                className="rhd-railops-device-search"
                placeholder={t("enterpriseDevices.searchPlaceholder")}
                value={search}
                onChange={(event) => setSearch(event.target.value)}
              />
            }
            actions={
              <span className="rhd-railops-table-summary">
                {t("enterpriseDevices.totalSummary", { count: deviceTotal, unbound: unboundDeviceCount })}
              </span>
            }
          />

          {deviceRefreshError ? (
            <div className="rhd-railops-refresh-error">
              <span>{t("enterpriseDevices.loadFailed")}</span>
              <Button variant="outline" size="sm" onClick={() => void loadDevices({ page })} disabled={loading}>{t("common.retry")}</Button>
            </div>
          ) : null}

          {deviceListRefreshing ? (
            <div className="rhd-railops-refreshing" role="status" aria-busy="true">
              {t("common.loading")}
            </div>
          ) : null}

          {initialDeviceListLoading ? (
            <div className="rhd-railops-table-skeleton">
              {Array.from({ length: 8 }).map((_, index) => <Skeleton key={index} className="h-12 w-full" />)}
            </div>
          ) : blockingDeviceError ? (
            <ErrorState
              title={t("enterpriseDevices.loadFailed")}
              description={error}
              action={{ label: t("common.retry"), onClick: () => void loadDevices({ page }) }}
              className="rounded-none border-0"
            />
          ) : (
            <>
              <div className="rhd-railops-device-table-wrap" aria-busy={deviceListRefreshing}>
                <AntTable<DeviceListItem>
                  className="rhd-railops-device-table"
                  columns={deviceColumns}
                  dataSource={devices}
                  locale={{
                    emptyText: (
                      <div className="rhd-railops-table-empty">
                        <AntEmpty image={AntEmpty.PRESENTED_IMAGE_SIMPLE} description={t("enterpriseDevices.noDevicesTitle")} />
                        {canCreateDevice ? (
                          <Button type="button" onClick={() => setCreateOpen(true)}>
                            <PlusIcon className="size-4" />
                            {t("enterpriseDevices.addDevice")}
                          </Button>
                        ) : null}
                      </div>
                    ),
                  }}
                  pagination={false}
                  rowClassName={(device) => String(device.id) === selectedDeviceId ? "is-selected" : ""}
                  rowKey="id"
                  scroll={{ x: 780 }}
                  onRow={(device) => ({
                    onClick: () => setSelectedDeviceId(String(device.id)),
                  })}
                />
              </div>
              <div className="rhd-railops-table-footer">
                <span>{t("enterpriseDevices.totalSummary", { count: deviceTotal, unbound: unboundDeviceCount })}</span>
                <AntPagination
                  current={page}
                  hideOnSinglePage={false}
                  pageSize={PAGE_SIZE}
                  showSizeChanger={false}
                  size="small"
                  total={deviceTotal}
                  onChange={handlePageChange}
                />
              </div>
            </>
          )}
        </ContentModule>
      </section>

      <DetailDrawer
        destroyOnHidden
        open={Boolean(selectedDeviceId)}
        title={selectedDevice?.service_code || selectedDevice?.device_no || t("enterpriseDevices.selectDeviceTitle")}
        onClose={() => setSelectedDeviceId("")}
      >
        {detailLoading ? (
          <DetailSkeleton />
        ) : detailError ? (
          <ErrorState title={t("enterpriseDevices.detailLoadFailed")} description={detailError} className="border-0 bg-transparent" />
        ) : selectedDevice ? (
          <DeviceDetailPanel
            detail={selectedDevice}
            canUpdate={canUpdateDevice}
            onEdit={() => setEditingDevice(selectedDevice)}
          />
        ) : (
          <EmptyState title={t("enterpriseDevices.selectDeviceTitle")} className="border-0 bg-transparent" />
        )}
      </DetailDrawer>
    </div>
  )
}

function CreateDeviceDialog({
  initialProductId,
  onCreated,
  onOpenChange,
  open,
  products,
}: {
  initialProductId: string
  onCreated: (device: DeviceListItem) => Promise<void> | void
  onOpenChange: (open: boolean) => void
  open: boolean
  products: ProductListItem[]
}) {
  const t = useI18n()
  const [form, setForm] = useState(emptyDeviceForm)
  const [models, setModels] = useState<ProductModel[]>([])
  const [modelsLoading, setModelsLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!open) return
    const initial = initialProductId && products.some((product) => String(product.id) === initialProductId)
      ? initialProductId
      : "0"
    setForm({ ...emptyDeviceForm, product_id: initial })
    setError("")
  }, [initialProductId, open, products])

  useEffect(() => {
    if (!open || !form.product_id || form.product_id === "0") {
      setModels([])
      return
    }
    let cancelled = false
    setModelsLoading(true)
    listProductModels(Number(form.product_id))
      .then((response) => {
        if (cancelled) return
        if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("enterpriseDevices.dialog.loadModelsFailed"))
        }
        setModels(response.data)
        setForm((current) => {
          if (current.product_id !== form.product_id) return current
          if (current.model_id === "0" || response.data.some((item) => String(item.id) === current.model_id)) return current
          return { ...current, model_id: "0" }
        })
      })
      .catch((err) => {
        if (cancelled) return
        setModels([])
        setError(err instanceof Error ? err.message : t("enterpriseDevices.dialog.loadModelsFailed"))
      })
      .finally(() => {
        if (!cancelled) setModelsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [form.product_id, open])

  function handleOpenChange(nextOpen: boolean) {
    if (saving) return
    onOpenChange(nextOpen)
  }

  async function handleSubmit() {
    const serviceCode = form.service_code.trim()
    if (!serviceCode) {
      setError(t("enterpriseDevices.dialog.serviceCodeRequired"))
      return
    }

    setSaving(true)
    setError("")
    try {
      const response = await createDevice({
        service_code: serviceCode,
        ...buildDeviceBasePayload(form),
      })
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("enterpriseDevices.dialog.addFailed"))
      }
      onOpenChange(false)
      setForm(emptyDeviceForm)
      await onCreated(response.data)
      toast.success(t("enterpriseDevices.dialog.addSuccess", { code: response.data.service_code || response.data.device_no }))
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterpriseDevices.dialog.addFailed")
      setError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => handleOpenChange(false)}
      title={t("enterpriseDevices.dialog.createTitle")}
      width={672}
      rootClassName="rhd-railops-scrollable-modal"
      footer={
        <>
          <Button type="button" variant="outline" onClick={() => handleOpenChange(false)} disabled={saving}>
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            form="create-device-form"
            disabled={saving || products.length === 0 || !form.service_code.trim()}
          >
            {saving ? t("enterpriseDevices.dialog.adding") : t("enterpriseDevices.dialog.confirmAdd")}
          </Button>
        
        </>
      }
    >
        {products.length === 0 ? (
          <div className="rounded-md border border-dashed border-border p-6 text-center">
            <h3 className="text-sm font-semibold text-foreground">{t("enterpriseDevices.dialog.noProductTitle")}</h3>
            <p className="mt-1 text-xs text-muted-foreground">{t("enterpriseDevices.dialog.noProductDescription")}</p>
            <Button className="mt-4" render={<Link href="/enterprise/products" />}>
              {t("enterpriseDevices.dialog.goToProducts")}
            </Button>
          </div>
        ) : (
          <form
            id="create-device-form"
            className="grid gap-4 py-1 sm:grid-cols-2"
            onSubmit={(event) => {
              event.preventDefault()
              void handleSubmit()
            }}
          >
            <label className="grid gap-2 text-sm font-medium sm:col-span-2">
              <span>{t("enterpriseDevices.dialog.serviceCode")}</span>
              <Input
                autoFocus
                required
                aria-label={t("enterpriseDevices.dialog.serviceCode")}
                className="font-mono uppercase"
                placeholder={t("enterpriseDevices.dialog.serviceCodePlaceholder")}
                value={form.service_code}
                onChange={(event) => {
                  setError("")
                  setForm((current) => ({ ...current, service_code: event.target.value }))
                }}
              />
            </label>
            <SelectField
              label={t("enterpriseDevices.dialog.product")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("enterpriseDevices.dialog.product"),
                value: form.product_id,
                onChange: (value) => {
                  setError("")
                  setForm((current) => ({ ...current, product_id: value ?? "", model_id: "0" }))
                },
                options: [
                  ...(form.product_id !== "0" && !products.some((item) => String(item.id) === form.product_id)
                    ? [{ value: form.product_id, label: formatSelectedProductLabel(t, products, form.product_id, t("enterpriseDevices.dialog.productFromServiceCode"), t("enterpriseDevices.dialog.productFromServiceCode")) }]
                    : []),
                  { value: "0", label: t("enterpriseDevices.dialog.productFromServiceCode") },
                  ...products.map((product) => ({ value: String(product.id), label: formatProductOptionLabel(t, product) })),
                ],
                style: { width: "100%" },
              }}
            />
            <SelectField
              label={t("enterpriseDevices.dialog.productModel")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("enterpriseDevices.dialog.productModel"),
                value: form.model_id,
                onChange: (value) => setForm((current) => ({ ...current, model_id: value ?? "0" })),
                disabled: modelsLoading || !form.product_id,
                options: [
                  ...(modelsLoading ? [{ value: form.model_id, label: t("common.loading") }] : []),
                  { value: "0", label: t("enterpriseDevices.modelSelect.none") },
                  ...models.map((model) => ({ value: String(model.id), label: model.name || model.model_code })),
                ],
                style: { width: "100%" },
              }}
            />
            <label className="grid gap-2 text-sm font-medium">
              <span>{t("enterpriseDevices.dialog.region")}</span>
              <Input
                aria-label={t("enterpriseDevices.dialog.region")}
                placeholder={t("enterpriseDevices.dialog.regionPlaceholder")}
                value={form.region_code}
                onChange={(event) => setForm((current) => ({ ...current, region_code: event.target.value }))}
              />
            </label>
            <label className="grid gap-2 text-sm font-medium">
              <span>{t("enterpriseDevices.dialog.installDate")}</span>
              <Input
                aria-label={t("enterpriseDevices.dialog.installDate")}
                type="date"
                value={form.install_date}
                onChange={(event) => setForm((current) => ({ ...current, install_date: event.target.value }))}
              />
            </label>
            <label className="grid gap-2 text-sm font-medium">
              <span>{t("enterpriseDevices.dialog.warrantyEnd")}</span>
              <Input
                aria-label={t("enterpriseDevices.dialog.warrantyEnd")}
                type="date"
                value={form.warranty_end}
                onChange={(event) => setForm((current) => ({ ...current, warranty_end: event.target.value }))}
              />
            </label>
            <label className="grid gap-2 text-sm font-medium sm:col-span-2">
              <span>{t("enterpriseDevices.dialog.notes")}</span>
              <Textarea
                aria-label={t("enterpriseDevices.dialog.notes")}
                placeholder={t("enterpriseDevices.dialog.notesPlaceholder")}
                value={form.description}
                onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))}
              />
            </label>
          </form>
        )}

        {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
        
    </StandardModal>
  )
}

function EditDeviceDialog({
  detail,
  onOpenChange,
  onUpdated,
  open,
  products,
}: {
  detail: DeviceDetailDTO | null
  onOpenChange: (open: boolean) => void
  onUpdated: (device: DeviceDetailDTO) => Promise<void> | void
  open: boolean
  products: ProductListItem[]
}) {
  const t = useI18n()
  const [form, setForm] = useState<DeviceFormState>(emptyDeviceForm)
  const [models, setModels] = useState<ProductModel[]>([])
  const [modelsLoading, setModelsLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!open || !detail) return
    setForm({
      service_code: detail.service_code || detail.device_no || "",
      product_id: detail.product_id ? String(detail.product_id) : "",
      model_id: detail.model_id ? String(detail.model_id) : "0",
      region_code: detail.region_code || "",
      install_date: dateInputValue(detail.install_date),
      warranty_end: dateInputValue(detail.warranty_end),
      description: extractDeviceDescription(detail.description),
    })
    setError("")
  }, [detail, open])

  useEffect(() => {
    if (!open || !form.product_id || form.product_id === "0") {
      setModels([])
      return
    }
    let cancelled = false
    setModelsLoading(true)
    listProductModels(Number(form.product_id))
      .then((response) => {
        if (cancelled) return
        if (!response.success || !response.data) {
          throw new Error(response.error?.message || t("enterpriseDevices.dialog.loadModelsFailed"))
        }
        setModels(response.data)
        setForm((current) => {
          if (current.product_id !== form.product_id) return current
          if (current.model_id === "0" || response.data.some((item) => String(item.id) === current.model_id)) return current
          return { ...current, model_id: "0" }
        })
      })
      .catch((err) => {
        if (cancelled) return
        setModels([])
        setError(err instanceof Error ? err.message : t("enterpriseDevices.dialog.loadModelsFailed"))
      })
      .finally(() => {
        if (!cancelled) setModelsLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [form.product_id, open, t])

  function handleOpenChange(nextOpen: boolean) {
    if (saving) return
    onOpenChange(nextOpen)
  }

  async function handleSubmit() {
    if (!detail) return
    setSaving(true)
    setError("")
    try {
      const response = await updateDevice(detail.id, buildDeviceBasePayload(form, true))
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("enterpriseDevices.dialog.updateFailed"))
      }
      onOpenChange(false)
      await onUpdated(response.data)
      toast.success(t("enterpriseDevices.dialog.updateSuccess", { code: response.data.service_code || response.data.device_no }))
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterpriseDevices.dialog.updateFailed")
      setError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => handleOpenChange(false)}
      title={t("enterpriseDevices.dialog.editTitle")}
      width={672}
      rootClassName="rhd-railops-scrollable-modal"
      footer={
        <>
          <Button type="button" variant="outline" onClick={() => handleOpenChange(false)} disabled={saving}>
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            form="edit-device-form"
            disabled={saving || !detail}
          >
            {saving ? t("enterpriseDevices.dialog.saving") : t("enterpriseDevices.dialog.save")}
          </Button>
        </>
      }
    >
      {detail ? (
        <form
          id="edit-device-form"
          className="grid gap-4 py-1 sm:grid-cols-2"
          onSubmit={(event) => {
            event.preventDefault()
            void handleSubmit()
          }}
        >
          <label className="grid gap-2 text-sm font-medium sm:col-span-2">
            <span>{t("enterpriseDevices.dialog.serviceCode")}</span>
            <Input
              aria-label={t("enterpriseDevices.dialog.serviceCode")}
              className="font-mono uppercase"
              disabled
              value={form.service_code}
            />
          </label>
          <SelectField
            label={t("enterpriseDevices.dialog.product")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("enterpriseDevices.dialog.product"),
              value: form.product_id,
              onChange: (value) => {
                setError("")
                setForm((current) => ({ ...current, product_id: value ?? "", model_id: "0" }))
              },
              options: [
                ...(form.product_id && !products.some((item) => String(item.id) === form.product_id)
                  ? [{ value: form.product_id, label: formatSelectedProductLabel(t, products, form.product_id, t("enterpriseDevices.dialog.product"), t("enterpriseDevices.dialog.product")) }]
                  : []),
                ...products.map((product) => ({ value: String(product.id), label: formatProductOptionLabel(t, product) })),
              ],
              style: { width: "100%" },
            }}
          />
          <SelectField
            label={t("enterpriseDevices.dialog.productModel")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("enterpriseDevices.dialog.productModel"),
              value: form.model_id,
              onChange: (value) => setForm((current) => ({ ...current, model_id: value ?? "0" })),
              disabled: modelsLoading || !form.product_id,
              options: [
                ...(modelsLoading ? [{ value: form.model_id, label: t("common.loading") }] : []),
                { value: "0", label: t("enterpriseDevices.modelSelect.none") },
                ...models.map((model) => ({ value: String(model.id), label: model.name || model.model_code })),
              ],
              style: { width: "100%" },
            }}
          />
          <label className="grid gap-2 text-sm font-medium">
            <span>{t("enterpriseDevices.dialog.region")}</span>
            <Input
              aria-label={t("enterpriseDevices.dialog.region")}
              placeholder={t("enterpriseDevices.dialog.regionPlaceholder")}
              value={form.region_code}
              onChange={(event) => setForm((current) => ({ ...current, region_code: event.target.value }))}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium">
            <span>{t("enterpriseDevices.dialog.installDate")}</span>
            <Input
              aria-label={t("enterpriseDevices.dialog.installDate")}
              type="date"
              value={form.install_date}
              onChange={(event) => setForm((current) => ({ ...current, install_date: event.target.value }))}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium">
            <span>{t("enterpriseDevices.dialog.warrantyEnd")}</span>
            <Input
              aria-label={t("enterpriseDevices.dialog.warrantyEnd")}
              type="date"
              value={form.warranty_end}
              onChange={(event) => setForm((current) => ({ ...current, warranty_end: event.target.value }))}
            />
          </label>
          <label className="grid gap-2 text-sm font-medium sm:col-span-2">
            <span>{t("enterpriseDevices.dialog.notes")}</span>
            <Textarea
              aria-label={t("enterpriseDevices.dialog.notes")}
              placeholder={t("enterpriseDevices.dialog.notesPlaceholder")}
              value={form.description}
              onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))}
            />
          </label>
        </form>
      ) : null}

      {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
    </StandardModal>
  )
}

function BatchImportDevicesDialog({
  initialProductId,
  onImported,
  onOpenChange,
  open,
  products,
}: {
  initialProductId: string
  onImported: (result: DeviceBatchImportResult) => Promise<void> | void
  onOpenChange: (open: boolean) => void
  open: boolean
  products: ProductListItem[]
}) {
  const t = useI18n()
  const [defaultProductId, setDefaultProductId] = useState("")
  const [csvText, setCsvText] = useState("")
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  const [result, setResult] = useState<DeviceBatchImportResult | null>(null)
  const defaultProduct = defaultProductId && defaultProductId !== "0"
    ? products.find((product) => String(product.id) === defaultProductId)
    : null

  useEffect(() => {
    if (!open) return
    const initial = initialProductId && products.some((product) => String(product.id) === initialProductId)
      ? initialProductId
      : "0"
    setDefaultProductId(initial)
    setCsvText("")
    setError("")
    setResult(null)
  }, [initialProductId, open, products])

  function handleOpenChange(nextOpen: boolean) {
    if (saving) return
    onOpenChange(nextOpen)
  }

  async function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ""
    if (!file) return
    try {
      setError("")
      setResult(null)
      setCsvText(await file.text())
    } catch (err) {
      setError(err instanceof Error ? err.message : t("enterpriseDevices.dialog.readCsvFailed"))
    }
  }

  function handleDownloadTemplate() {
    const blob = new Blob([deviceImportTemplate(t, defaultProductId)], { type: "text/csv;charset=utf-8" })
    const url = URL.createObjectURL(blob)
    const link = document.createElement("a")
    link.href = url
    link.download = "device-import-template.csv"
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  }

  async function handleSubmit() {
    setSaving(true)
    setError("")
    setResult(null)
    try {
      const rows = parseDeviceImportCSV(t, csvText, defaultProductId)
      const response = await batchImportDevices(rows)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("enterpriseDevices.dialog.importFailed"))
      }
      setResult(response.data)
      await onImported(response.data)
      if (response.data.failed > 0) {
        toast.warning(t("enterpriseDevices.dialog.importWarning", { succeeded: response.data.succeeded, failed: response.data.failed }))
      } else {
        toast.success(t("enterpriseDevices.dialog.importSuccess", { succeeded: response.data.succeeded }))
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : t("enterpriseDevices.dialog.importFailed")
      setError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => handleOpenChange(false)}
      title={t("enterpriseDevices.batchDialog.title")}
      width={768}
      rootClassName="rhd-railops-scrollable-modal"
      footer={
        <>
          <Button type="button" variant="outline" onClick={() => handleOpenChange(false)} disabled={saving}>
            {t("common.close")}
          </Button>
          <Button
            type="button"
            onClick={() => void handleSubmit()}
            disabled={saving || products.length === 0 || !csvText.trim()}
          >
            {saving ? t("enterpriseDevices.batchDialog.importing") : t("enterpriseDevices.batchDialog.confirmImport")}
          </Button>
        
        </>
      }
    >
        {products.length === 0 ? (
          <div className="rounded-md border border-dashed border-border p-6 text-center">
            <h3 className="text-sm font-semibold text-foreground">{t("enterpriseDevices.dialog.noProductTitle")}</h3>
            <p className="mt-1 text-xs text-muted-foreground">{t("enterpriseDevices.batchDialog.noProductDescription")}</p>
            <Button className="mt-4" render={<Link href="/enterprise/products" />}>
              {t("enterpriseDevices.dialog.goToProducts")}
            </Button>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_220px]">
              <SelectField
                label={t("enterpriseDevices.batchDialog.defaultProduct")}
                style={{ marginBottom: 0 }}
                selectProps={{
                  "aria-label": t("enterpriseDevices.batchDialog.defaultProduct"),
                  value: defaultProductId,
                  onChange: (value) => {
                    setDefaultProductId(value ?? "0")
                    setResult(null)
                    setError("")
                  },
                  options: [
                    ...(defaultProductId !== "0" && !products.some((item) => String(item.id) === defaultProductId)
                      ? [{ value: defaultProductId, label: formatSelectedProductLabel(t, products, defaultProductId, t("enterpriseDevices.batchDialog.selectDefaultProduct"), t("enterpriseDevices.batchDialog.noDefaultProduct")) }]
                      : []),
                    { value: "0", label: t("enterpriseDevices.batchDialog.noDefaultProduct") },
                    ...products.map((product) => ({ value: String(product.id), label: formatProductOptionLabel(t, product) })),
                  ],
                  style: { width: "100%" },
                }}
              />
              <div className="flex items-end gap-2">
                <Button type="button" variant="outline" className="flex-1" onClick={handleDownloadTemplate}>
                  <DownloadIcon className="size-4" />
                  {t("enterpriseDevices.batchDialog.template")}
                </Button>
                <label className="inline-flex h-8 flex-1 cursor-pointer items-center justify-center gap-1.5 rounded-lg border border-border bg-background px-2.5 text-sm font-medium hover:bg-muted">
                  <UploadIcon className="size-4" />
                  {t("enterpriseDevices.batchDialog.upload")}
                  <input className="sr-only" type="file" accept=".csv,text/csv" onChange={handleFileChange} />
                </label>
              </div>
            </div>

            <div className="rounded-md border border-border bg-muted p-3 text-xs leading-5 text-muted-foreground">
              {defaultProduct
                ? t("enterpriseDevices.batchDialog.currentDefaultProduct", { name: defaultProduct.name || defaultProduct.code || t("enterpriseDevices.productFallback", { id: defaultProduct.id }), id: defaultProduct.id })
                : t("enterpriseDevices.batchDialog.noDefaultProductNote")}
              <div className="mt-1">
                {t("enterpriseDevices.batchDialog.fieldsHint")}
              </div>
            </div>

            <label className="grid gap-2 text-sm font-medium">
              <span>{t("enterpriseDevices.batchDialog.csvContent")}</span>
              <Textarea
                className="min-h-56 font-mono text-xs"
                aria-label={t("enterpriseDevices.batchDialog.csvContent")}
                placeholder={deviceImportTemplate(t, defaultProductId)}
                value={csvText}
                onChange={(event) => {
                  setCsvText(event.target.value)
                  setResult(null)
                  setError("")
                }}
              />
            </label>
          </div>
        )}

        {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
        {result ? (
          <div className="rounded-md border border-border bg-card p-3 text-sm">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="outline" className={toneClass("green")}>{t("enterpriseDevices.batchDialog.successCount", { count: result.succeeded })}</Badge>
              <Badge variant="outline" className={result.failed > 0 ? toneClass("red") : toneClass("slate")}>{t("enterpriseDevices.batchDialog.failureCount", { count: result.failed })}</Badge>
              <span className="text-xs text-muted-foreground">{t("enterpriseDevices.batchDialog.totalRows", { count: result.total })}</span>
            </div>
            {result.errors?.length ? (
              <div className="mt-3 max-h-32 overflow-y-auto rounded-md bg-muted p-2 text-xs text-muted-foreground">
                {result.errors.slice(0, 20).map((item) => (
                  <div key={`${item.row}-${item.reason}`}>{t("enterpriseDevices.batchDialog.rowError", { row: item.row, reason: item.reason })}</div>
                ))}
                {result.errors.length > 20 ? <div>{t("enterpriseDevices.batchDialog.remainingErrors", { count: result.errors.length - 20 })}</div> : null}
              </div>
            ) : null}
          </div>
        ) : null}

        
    </StandardModal>
  )
}

function deviceImportTemplate(t: I18nT, productId: string) {
  const hasDefaultProduct = Number(productId) > 0
  return [
    "service_code,product_id,model_id,region_code,description",
    hasDefaultProduct
      ? "SC-2026-001,,,US-WEST,installed unit"
      : `SC-2026-001,${t("enterpriseDevices.batchDialog.templateProductIdPlaceholder")},,US-WEST,installed unit`,
  ].join("\n")
}

function parseDeviceImportCSV(t: I18nT, input: string, fallbackProductId: string): BatchImportDevicePayload[] {
  const records = parseCSVRecords(input)
  if (records.length <= 1) {
    throw new Error(t("enterpriseDevices.batchDialog.csvNeedHeader"))
  }
  const headers = records[0].map(normalizeImportHeader)
  const rows: BatchImportDevicePayload[] = []

  records.slice(1).forEach((record, index) => {
    if (record.every((value) => !value.trim())) return
    const rowNumber = index + 2
    const serviceCode = importCell(record, headers, ["servicecode", "devicecode", "qrcode"]).trim()
    if (!serviceCode) {
      throw new Error(t("enterpriseDevices.batchDialog.csvMissingServiceCode", { row: rowNumber }))
    }
    const fallback = Number(fallbackProductId) > 0 ? fallbackProductId : ""
    const productID = parseOptionalImportID(
      t,
      importCell(record, headers, ["productid"]),
      rowNumber,
      "product_id",
    )
    const fallbackID = parseOptionalImportID(t, fallback, rowNumber, "product_id")
    const modelID = parseOptionalImportID(
      t,
      importCell(record, headers, ["modelid", "productmodelid"]),
      rowNumber,
      "model_id",
    )
    rows.push({
      service_code: serviceCode,
      ...(productID > 0 ? { product_id: productID } : fallbackID > 0 ? { product_id: fallbackID } : {}),
      ...(modelID > 0 ? { model_id: modelID } : {}),
      region_code: importCell(record, headers, ["regioncode"]).trim(),
      description: importCell(record, headers, ["description", "remark", "notes"]).trim(),
    })
  })

  if (rows.length === 0) {
    throw new Error(t("enterpriseDevices.batchDialog.csvNoRows"))
  }
  if (rows.length > 500) {
    throw new Error(t("enterpriseDevices.batchDialog.csvTooManyRows"))
  }
  return rows
}

function parseCSVRecords(input: string) {
  const rows: string[][] = []
  let row: string[] = []
  let cell = ""
  let quoted = false
  for (let index = 0; index < input.length; index += 1) {
    const char = input[index]
    const next = input[index + 1]
    if (char === "\"") {
      if (quoted && next === "\"") {
        cell += "\""
        index += 1
      } else {
        quoted = !quoted
      }
      continue
    }
    if (char === "," && !quoted) {
      row.push(cell)
      cell = ""
      continue
    }
    if ((char === "\n" || char === "\r") && !quoted) {
      if (char === "\r" && next === "\n") index += 1
      row.push(cell)
      rows.push(row)
      row = []
      cell = ""
      continue
    }
    cell += char
  }
  row.push(cell)
  if (row.some((value) => value.trim())) rows.push(row)
  return rows
}

function normalizeImportHeader(value: string) {
  return value.trim().toLowerCase().replace(/[\s_-]/g, "")
}

function importCell(record: string[], headers: string[], aliases: string[]) {
  const index = headers.findIndex((header) => aliases.includes(header))
  if (index < 0 || index >= record.length) return ""
  return record[index] ?? ""
}

function parseOptionalImportID(t: I18nT, value: string, rowNumber: number, label: string) {
  const source = value.trim()
  if (!source) return 0
  if (!/^\d+$/.test(source)) {
    throw new Error(t("enterpriseDevices.batchDialog.csvInvalidId", { row: rowNumber, field: label }))
  }
  const parsed = Number(source)
  if (!Number.isSafeInteger(parsed) || parsed < 0) {
    throw new Error(t("enterpriseDevices.batchDialog.csvInvalidIdRange", { row: rowNumber, field: label }))
  }
  return parsed
}

function modelSelectLabel(t: I18nT, models: ProductModel[], selectedId: string) {
  if (!selectedId || selectedId === "0") return t("enterpriseDevices.modelSelect.none")
  return models.find((item) => String(item.id) === selectedId)?.name || t("enterpriseDevices.modelSelect.none")
}

function DeviceDetailPanel({
  canUpdate,
  detail,
  onEdit,
}: {
  canUpdate: boolean
  detail: DeviceDetailDTO
  onEdit: () => void
}) {
  const t = useI18n()
  const [activeDetailTab, setActiveDetailTab] = useState<DeviceDetailTab>("bindings")
  const [qrPreviewOpen, setQrPreviewOpen] = useState(false)
  const qrPreviewRef = useRef<HTMLDivElement | null>(null)
  const qrValue = useMemo(() => deviceServiceCodeEntryURL(detail), [detail])
  const canGenerateQRCode = Boolean(detail.service_code && qrValue)
  const detailTabs = useMemo<RailopsTabItem[]>(() => [
    {
      value: "bindings",
      label: t("enterpriseDevices.detail.customerBindingsTitle"),
      count: detail.customer_bindings.length,
      icon: <UsersIcon className="size-3.5" />,
    },
    {
      value: "tickets",
      label: t("enterpriseDevices.detail.recentTicketsTitle"),
      count: detail.recent_tickets.length,
      icon: <FileClockIcon className="size-3.5" />,
    },
    {
      value: "repairs",
      label: t("enterpriseDevices.detail.repairHistoryTitle"),
      count: detail.repair_history.length,
      icon: <WrenchIcon className="size-3.5" />,
    },
  ], [detail.customer_bindings.length, detail.recent_tickets.length, detail.repair_history.length, t])

  const getQRCodeCanvas = useCallback(() => {
    return qrPreviewRef.current?.querySelector("canvas") ?? null
  }, [])

  const copyEntryURL = useCallback(async () => {
    if (!qrValue) return
    try {
      await navigator.clipboard.writeText(qrValue)
      toast.success(t("enterpriseDevices.detail.qrLinkCopied"))
    } catch {
      toast.error(t("enterpriseDevices.detail.qrLinkCopyFailed"))
    }
  }, [qrValue, t])

  const buildLabelDataURL = useCallback(() => {
    const canvas = getQRCodeCanvas()
    if (!canvas || !qrValue) {
      toast.error(t("enterpriseDevices.detail.qrCanvasUnavailable"))
      return ""
    }
    return buildServiceCodeLabelDataURL({ detail, entryURL: qrValue, qrCanvas: canvas, t })
  }, [detail, getQRCodeCanvas, qrValue, t])

  const downloadQRCodeLabel = useCallback(() => {
    const dataURL = buildLabelDataURL()
    if (!dataURL) return
    const link = document.createElement("a")
    link.href = dataURL
    link.download = `${detail.service_code || detail.device_no || "service-code"}-qr-label.png`
    document.body.appendChild(link)
    link.click()
    link.remove()
  }, [buildLabelDataURL, detail.device_no, detail.service_code])

  const printQRCodeLabel = useCallback(() => {
    const dataURL = buildLabelDataURL()
    if (!dataURL) return
    const printWindow = window.open("", "_blank", "noopener,noreferrer")
    if (!printWindow) {
      toast.error(t("enterpriseDevices.detail.qrPrintBlocked"))
      return
    }
    const doc = printWindow.document
    doc.open()
    doc.write("<!doctype html><html><head><title></title></head><body></body></html>")
    doc.close()
    doc.title = t("enterpriseDevices.detail.qrPreviewTitle")
    doc.body.style.margin = "0"
    doc.body.style.minHeight = "100vh"
    doc.body.style.display = "grid"
    doc.body.style.placeItems = "center"
    doc.body.style.background = "#f3f4f6"
    const image = doc.createElement("img")
    image.src = dataURL
    image.alt = detail.service_code || detail.device_no || t("enterpriseDevices.detail.qrPreviewTitle")
    image.style.width = "360px"
    image.style.maxWidth = "90vw"
    image.style.background = "#fff"
    image.style.boxShadow = "0 12px 36px rgba(15, 23, 42, 0.16)"
    doc.body.appendChild(image)
    image.onload = () => {
      printWindow.focus()
      printWindow.print()
    }
  }, [buildLabelDataURL, detail.device_no, detail.service_code, t])

  return (
    <div className="rhd-railops-detail-stack rhd-railops-device-detail-stack">
      <section className="rhd-railops-detail-section rhd-railops-detail-summary rhd-railops-device-detail-card">
        <div className="rhd-railops-device-detail-hero">
          <div className="rhd-railops-device-detail-emblem">
            <CpuIcon className="size-5" />
          </div>
          <div className="rhd-railops-device-detail-title">
            <div className="min-w-0">
              <h2>{detail.service_code || detail.device_no}</h2>
              <p>{detail.product_name || "-"} · {detail.model_name || "-"}</p>
            </div>
          </div>
	          <div className="rhd-railops-device-detail-status">
	            <span>{detail.device_no || detail.service_code || "-"}</span>
	            {canUpdate ? (
	              <Button type="button" variant="outline" size="sm" onClick={onEdit}>
	                <PencilIcon className="size-3.5" />
	                {t("enterpriseDevices.detail.edit")}
	              </Button>
	            ) : null}
	          </div>
        </div>

        <div className="rhd-railops-device-detail-meta-bar">
          <Badge variant="outline" className={statusTone(detail.status)}>
            {deviceStatusLabel(t, detail.status)}
          </Badge>
          <span>{t("enterpriseDevices.detail.region")} {detail.region_code || "-"}</span>
          <span>{t("enterpriseDevices.detail.customerVisible")} {detail.customer_visible ? t("enterpriseDevices.detail.yes") : t("enterpriseDevices.detail.no")}</span>
        </div>

        <div className="rhd-railops-detail-summary-body">
          <div className="rhd-railops-device-detail-fields">
            <Field label={t("enterpriseDevices.detail.serviceCode")} value={detail.service_code || t("enterpriseDevices.detail.unbound")} mono />
            <Field label={t("enterpriseDevices.detail.region")} value={detail.region_code} />
            <Field label={t("enterpriseDevices.detail.installDate")} value={dateText(detail.install_date)} />
            <Field label={t("enterpriseDevices.detail.warrantyStatus")} value={`${warrantyLabel(t, detail.warranty_status)} · ${dateText(detail.warranty_end)}`} />
            <Field label={t("enterpriseDevices.detail.customerVisible")} value={detail.customer_visible ? t("enterpriseDevices.detail.yes") : t("enterpriseDevices.detail.no")} />
          </div>
        </div>

        {canGenerateQRCode ? (
          <div className="rhd-railops-device-qr-panel">
            <div ref={qrPreviewRef} className="rhd-railops-device-qr-code" aria-label={t("enterpriseDevices.detail.qrTitle")}>
              <QRCode
                bordered={false}
                bgColor="#ffffff"
                errorLevel="M"
                size={136}
                type="canvas"
                value={qrValue}
              />
            </div>
            <div className="rhd-railops-device-qr-content">
              <strong>{t("enterpriseDevices.detail.qrTitle")}</strong>
              <p>{t("enterpriseDevices.detail.qrDescription")}</p>
              <span className="rhd-railops-device-qr-url">{qrValue}</span>
              <div className="rhd-railops-device-qr-actions">
                <Button type="button" size="sm" onClick={() => setQrPreviewOpen(true)}>
                  <QrCodeIcon className="size-3.5" />
                  {t("enterpriseDevices.detail.generateQr")}
                </Button>
                <Button type="button" variant="outline" size="sm" onClick={() => setQrPreviewOpen(true)}>
                  <EyeIcon className="size-3.5" />
                  {t("enterpriseDevices.detail.previewQr")}
                </Button>
                <Button type="button" variant="outline" size="sm" onClick={downloadQRCodeLabel}>
                  <DownloadIcon className="size-3.5" />
                  {t("enterpriseDevices.detail.downloadQrLabel")}
                </Button>
              </div>
            </div>
          </div>
        ) : (
          <div className="rhd-railops-device-qr-empty">
            <QrCodeIcon className="size-4" />
            <span>{t("enterpriseDevices.detail.qrUnavailable")}</span>
          </div>
        )}
      </section>

      <section className="rhd-railops-detail-section rhd-railops-detail-tabs">
        <FilterTabs
          ariaLabel={t("enterpriseDevices.detail.tabsLabel")}
          items={detailTabs}
          value={activeDetailTab}
          onChange={(value) => setActiveDetailTab(value as DeviceDetailTab)}
        />
        <div className="rhd-railops-detail-tab-panel">
          {activeDetailTab === "bindings" ? <CustomerBindingsSection detail={detail} t={t} /> : null}
          {activeDetailTab === "tickets" ? <RecentTicketsSection detail={detail} t={t} /> : null}
          {activeDetailTab === "repairs" ? <RepairHistorySection detail={detail} t={t} /> : null}
        </div>
      </section>

      <StandardModal
        open={qrPreviewOpen}
        onCancel={() => setQrPreviewOpen(false)}
        title={t("enterpriseDevices.detail.qrPreviewTitle")}
        width={520}
        footer={
          <>
            <Button type="button" variant="outline" onClick={() => void copyEntryURL()}>
              <CopyIcon className="size-4" />
              {t("enterpriseDevices.detail.copyQrLink")}
            </Button>
            <Button type="button" variant="outline" onClick={printQRCodeLabel}>
              <PrinterIcon className="size-4" />
              {t("enterpriseDevices.detail.printQrLabel")}
            </Button>
            <Button type="button" onClick={downloadQRCodeLabel}>
              <DownloadIcon className="size-4" />
              {t("enterpriseDevices.detail.downloadQrLabel")}
            </Button>
          </>
        }
      >
        <div className="rhd-railops-device-qr-preview">
          <QRCode
            bordered
            bgColor="#ffffff"
            errorLevel="M"
            size={240}
            type="svg"
            value={qrValue || " "}
          />
          <div className="rhd-railops-device-qr-preview-meta">
            <strong className="font-mono">{detail.service_code || "-"}</strong>
            <span>{detail.product_name || "-"} · {detail.model_name || "-"}</span>
            <code>{qrValue || "-"}</code>
          </div>
        </div>
      </StandardModal>
    </div>
  )
}

function CustomerBindingsSection({ detail, t }: { detail: DeviceDetailDTO; t: I18nT }) {
  return (
    <section className="rhd-railops-detail-section">
      <SectionTitle icon={<UsersIcon className="size-4" />} title={t("enterpriseDevices.detail.customerBindingsTitle")} meta={t("enterpriseDevices.detail.customerBindingsMeta", { count: detail.customer_bindings.length })} />
      {detail.customer_bindings.length === 0 ? (
        <EmptyState title={t("enterpriseDevices.detail.customerBindingsEmptyTitle")} className="rounded-none border-0 p-8" />
      ) : (
        <div className="divide-y divide-border/70">
          {detail.customer_bindings.map((binding) => (
            <article key={binding.id} className="p-4 text-sm">
              <div className="flex items-center justify-between gap-2">
                <strong className="min-w-0 truncate text-foreground">{customerDisplayName(binding.customer_name, t("enterpriseDevices.detail.customerUserFallback", { id: binding.customer_user_id }))}</strong>
                <Badge variant="outline" className={binding.status === "active" ? toneClass("green") : toneClass("slate")}>
                  {bindingStatusLabel(t, binding.status)}
                </Badge>
              </div>
              <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                <span>{binding.binding_role || t("enterpriseDevices.detail.bindingRoleOwner")}</span>
                <span>{binding.source || "-"}</span>
                <span>{dateText(binding.confirmed_at)}</span>
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  )
}

function RecentTicketsSection({ detail, t }: { detail: DeviceDetailDTO; t: I18nT }) {
  return (
    <section className="rhd-railops-detail-section">
      <SectionTitle icon={<FileClockIcon className="size-4" />} title={t("enterpriseDevices.detail.recentTicketsTitle")} meta={t("enterpriseDevices.detail.recentTicketsMeta", { count: detail.recent_tickets.length })} />
      {detail.recent_tickets.length === 0 ? (
        <EmptyState title={t("enterpriseDevices.detail.recentTicketsEmptyTitle")} className="rounded-none border-0 p-8" />
      ) : (
        <div className="divide-y divide-border/70">
          {detail.recent_tickets.map((ticket) => (
            <Link key={ticket.id} href={buildEnterpriseTicketWorkbenchPath(ticket.id)} className="block p-4 text-sm hover:bg-muted/60">
              <div className="flex items-center justify-between gap-2">
                <strong className="min-w-0 truncate text-primary">{ticket.ticket_no}</strong>
                <Badge variant="outline">{ticketStatusLabel(t, ticket.status)}</Badge>
              </div>
              <p className="mt-1 line-clamp-2 text-muted-foreground">{ticket.title}</p>
              <div className="mt-1 text-xs text-muted-foreground">{dateText(ticket.updated_at)}</div>
            </Link>
          ))}
        </div>
      )}
    </section>
  )
}

function RepairHistorySection({ detail, t }: { detail: DeviceDetailDTO; t: I18nT }) {
  return (
    <section className="rhd-railops-detail-section">
      <SectionTitle icon={<WrenchIcon className="size-4" />} title={t("enterpriseDevices.detail.repairHistoryTitle")} meta={t("enterpriseDevices.detail.repairHistoryMeta", { count: detail.repair_history.length })} />
      {detail.repair_history.length === 0 ? (
        <EmptyState title={t("enterpriseDevices.detail.repairHistoryEmptyTitle")} className="rounded-none border-0 p-8" />
      ) : (
        <div className="divide-y divide-border/70">
          {detail.repair_history.map((record) => (
            <article key={record.id} className="p-4 text-sm">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <strong className="text-foreground">{record.fault_type || t("enterpriseDevices.detail.repairRecordFallback")}</strong>
                  <p className="mt-1 line-clamp-2 text-muted-foreground">{record.resolution || record.repair_method || "-"}</p>
                  <div className="mt-1 text-xs text-muted-foreground">
                    {record.ticket_no || "-"} · {record.technician_name || "-"} · {dateText(record.completed_at)}
                  </div>
                </div>
                <Badge variant="outline" className={record.visible_to_customer ? toneClass("green") : toneClass("slate")}>
                  {record.visible_to_customer ? t("enterpriseDevices.detail.visibleToCustomer") : t("enterpriseDevices.detail.internal")}
                </Badge>
              </div>
            </article>
          ))}
        </div>
      )}
    </section>
  )
}

function Field({ label, mono, value }: { label: string; mono?: boolean; value?: string }) {
  return (
    <div className="rhd-railops-device-detail-field">
      <span>{label}</span>
      <strong className={cn(mono ? "font-mono" : "")}>{value || "-"}</strong>
    </div>
  )
}

function SectionTitle({ icon, meta, title }: { icon: ReactNode; meta: string; title: string }) {
  return (
    <div className="rhd-railops-drawer-module-heading">
      <div>
        <span>{icon}</span>
        <h3>{title}</h3>
      </div>
      <small>{meta}</small>
    </div>
  )
}

function DetailSkeleton() {
  return (
    <div className="space-y-4">
      <Skeleton className="h-52 w-full" />
      <Skeleton className="h-40 w-full" />
      <Skeleton className="h-56 w-full" />
    </div>
  )
}
