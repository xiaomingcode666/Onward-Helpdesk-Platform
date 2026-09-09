"use client"

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react"
import { DownloadIcon, RefreshCwIcon, SearchIcon, XIcon } from "lucide-react"
import { Skeleton } from "antd"
import type { TableColumnsType } from "antd"
import { DataTable, IconButton, RailopsButton, SearchField, StatusTag, TablePagination, type StatusTagTone } from "@railops/ui"

import { EmptyState, ErrorState } from "@/components/shared/error-states"
import { useI18n } from "@/i18n/provider"
import type { ApiResponse } from "@/lib/api/types"
import type { ProductResourcePage, ProductResourceQuery } from "@/lib/api/enterprise-products"
import { downloadCsv, type CsvCell, type CsvRow } from "@/lib/csv-export"

export type ProductResourceColumn<T> = {
  key: string
  label: string
  className?: string
  exportValue?: (item: T) => CsvCell
  render: (item: T) => ReactNode
}

export function ProductResourceStatusTag({
  value,
  kind = "status",
}: {
  value?: string
  kind?: "status" | "priority" | "severity"
}) {
  return (
    <StatusTag tone={resourceStatusTone(value, kind)}>
      {value || "-"}
    </StatusTag>
  )
}

export function ProductResourceTable<T extends object>({
  productId,
  load,
  columns,
  rowKey,
  emptyKey,
  exportFilename,
  exportTitle,
  paginated = true,
}: {
  productId: number
  load: (productId: number, query?: ProductResourceQuery) => Promise<ApiResponse<ProductResourcePage<T> | T[]>>
  columns: ProductResourceColumn<T>[]
  rowKey: (item: T) => string | number
  emptyKey: string
  exportFilename?: string
  exportTitle?: string
  paginated?: boolean
}) {
  const t = useI18n()
  const [items, setItems] = useState<T[]>([])
  const [loading, setLoading] = useState(true)
  const [itemsLoaded, setItemsLoaded] = useState(false)
  const [error, setError] = useState("")
  const [reloadVersion, setReloadVersion] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [total, setTotal] = useState(0)
  const [searchInput, setSearchInput] = useState("")
  const [search, setSearch] = useState("")

  const loadItems = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const response = await load(productId, paginated ? { page, page_size: pageSize, search } : undefined)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("productArchive.loadFailed"))
      }
      if (Array.isArray(response.data)) {
        setItems(response.data)
        setTotal(response.data.length)
      } else {
        setItems(response.data.items)
        setTotal(response.data.total)
      }
      setItemsLoaded(true)
    } catch (value) {
      setError(value instanceof Error ? value.message : t("productArchive.loadFailed"))
    } finally {
      setLoading(false)
    }
  }, [load, page, pageSize, paginated, productId, search, t])

  useEffect(() => {
    void loadItems()
  }, [loadItems, reloadVersion])

  const initialLoading = loading && !itemsLoaded
  const blockingError = error && items.length === 0 && !loading
  const tableColumns = useMemo<TableColumnsType<T>>(
    () => columns.map((column) => ({
      title: column.label,
      key: column.key,
      render: (_value: unknown, item: T) => (
        <div className={column.className || undefined}>{column.render(item)}</div>
      ),
    })),
    [columns],
  )

  const handleExport = useCallback(() => {
    if (!exportFilename || !items.length) return
    const rows: CsvRow[] = [
      [exportTitle || exportFilename.replace(/\.csv$/i, "")],
      [t("exportsExtract.csvResourceTable.exportedAt"), new Date().toLocaleString("zh-CN", { hour12: false })],
      [t("exportsExtract.csvResourceTable.recordCount"), items.length],
      [],
      columns.map((column) => column.label),
      ...items.map((item) => columns.map((column) => column.exportValue?.(item) ?? "")),
    ]
    downloadCsv(exportFilename.endsWith(".csv") ? exportFilename : `${exportFilename}.csv`, rows)
  }, [columns, exportFilename, exportTitle, items, t])

  return (
    <div className="rhd-railops-product-resource-shell">
      <div className="rhd-railops-product-resource-toolbar">
        {paginated ? <form
          className="rhd-railops-product-resource-search"
          onSubmit={(event) => {
            event.preventDefault()
            setPage(1)
            setSearch(searchInput.trim())
          }}
        >
          <SearchField
            value={searchInput}
            onChange={(event) => {
              const value = event.target.value
              setSearchInput(value)
              if (!value && search) {
                setSearch("")
                setPage(1)
              }
            }}
            placeholder={t("productArchive.searchPlaceholder")}
            allowClear
          />
          {searchInput || search ? (
            <IconButton
              icon={<XIcon className="size-4" />}
              tooltip={t("productArchive.clearSearch")}
              aria-label={t("productArchive.clearSearch")}
              onClick={() => {
                setSearchInput("")
                setSearch("")
                setPage(1)
              }}
            />
          ) : null}
          <IconButton
            icon={<SearchIcon className="size-4" />}
            tooltip={t("productArchive.search")}
            aria-label={t("productArchive.search")}
            htmlType="submit"
          />
        </form> : <span />}
        <div className="flex items-center gap-2">
          {exportFilename ? (
            <RailopsButton size="small" onClick={handleExport} disabled={loading || !items.length}>
              <DownloadIcon className="size-3.5" />
              {t("exportsExtract.csvResourceTable.exportCsv")}
            </RailopsButton>
          ) : null}
          <RailopsButton size="small" variant="text" onClick={() => setReloadVersion((value) => value + 1)} disabled={loading}>
            <RefreshCwIcon className={loading ? "size-3.5 animate-spin" : "size-3.5"} />
            {t("productArchive.refresh")}
          </RailopsButton>
        </div>
      </div>
      {error && items.length > 0 ? (
        <div className="rhd-railops-product-table-warning">{error}</div>
      ) : null}
      {initialLoading ? (
        <div className="rhd-railops-table-skeleton">
          {Array.from({ length: 5 }).map((_, index) => <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 44 }} />)}
        </div>
      ) : blockingError ? (
        <ErrorState
          title={t("productArchive.loadFailed")}
          description={error}
          action={{ label: t("productArchive.retry"), onClick: () => setReloadVersion((value) => value + 1) }}
          className="rounded-none border-0"
        />
      ) : items.length ? (
        <div className="rhd-railops-product-table-wrap">
          <DataTable<T>
            className="rhd-railops-product-table rhd-railops-product-resource-table"
            columns={tableColumns}
            dataSource={items}
            rowKey={rowKey}
            scroll={{ x: 720 }}
            size="middle"
            emptyDescription={t(emptyKey)}
          />
        </div>
      ) : (
        <EmptyState title={t(emptyKey)} className="rhd-railops-product-tab-empty" />
      )}
      {paginated && !initialLoading && !blockingError ? <div className="rhd-railops-table-footer">
        <TablePagination
          current={page}
          pageSize={pageSize}
          total={total}
          disabled={loading}
          showSizeChanger
          onChange={(nextPage, nextPageSize) => {
            if (nextPageSize !== pageSize) {
              setPageSize(nextPageSize)
              setPage(1)
              return
            }
            setPage(nextPage)
          }}
        />
      </div> : null}
    </div>
  )
}

function resourceStatusTone(
  value?: string,
  kind: "status" | "priority" | "severity" = "status",
): StatusTagTone {
  const normalized = (value || "").trim().toLowerCase()
  if (!normalized) return "disabled"
  if (kind === "priority" || kind === "severity") {
    if (["critical", "urgent", "high", "p0", "p1"].includes(normalized)) return "error"
    if (["medium", "normal", "p2"].includes(normalized)) return "warning"
    if (["low", "minor", "p3", "p4"].includes(normalized)) return "success"
    return "neutral"
  }
  if (["active", "ready", "enabled", "online", "resolved", "closed", "completed", "finished", "indexed", "bound"].includes(normalized)) return "success"
  if (["pending", "waiting", "processing", "running", "open", "new", "created", "in_progress", "review"].includes(normalized)) return "blue"
  if (["warning", "maintenance", "expiring", "partial_failed", "unbound"].includes(normalized)) return "warning"
  if (["failed", "error", "rejected", "expired", "critical"].includes(normalized)) return "error"
  if (["inactive", "disabled", "archived", "deleted", "cancelled", "canceled"].includes(normalized)) return "disabled"
  return "neutral"
}
