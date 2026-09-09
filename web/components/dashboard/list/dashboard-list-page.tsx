"use client"

import type { ReactNode } from "react"
import { RefreshCwIcon } from "lucide-react"
import { SearchField } from "@railops/ui"

import {
  DashboardPage,
  DashboardTableShell,
  DashboardTableStateRow,
  DashboardToolbar,
} from "@/components/dashboard-page"
import { ListPagination } from "@/components/list-pagination"
import { OptionCombobox } from "@/components/option-combobox"
import { TagSelector } from "@/components/tag-selector"
import { Button } from "@/components/ui/button"
import type { TagTree } from "@/lib/api/admin"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type {
  DashboardCrudPageResult,
  DashboardCrudQueryFilter,
} from "@/components/dashboard/crud"
import {
  useDashboardPagedList,
  type DashboardPagedListOptions,
} from "./use-dashboard-paged-list"

export type DashboardListFilter = DashboardCrudQueryFilter & {
  label: string
  placeholder?: string
  defaultValue: string | number
  type?: "text" | "select" | "segment" | "tag"
  className?: string
  options?: ReadonlyArray<{ value: string; label: string; icon?: ReactNode }>
  tags?: TagTree[]
  searchPlaceholder?: string
  emptyText?: string
}

export type DashboardListColumn<TItem> = {
  key: string
  label: ReactNode
  className?: string
  render: (item: TItem, context: DashboardListRenderContext<TItem>) => ReactNode
}

export type DashboardListRenderContext<TItem> = {
  result: DashboardCrudPageResult<TItem>
  loading: boolean
  reload: () => Promise<void>
  resetFilters: () => void
}

export type DashboardListPageProps<TItem> = {
  filters?: DashboardListFilter[]
  fetchList: DashboardPagedListOptions<TItem>["fetchList"]
  columns?: DashboardListColumn<TItem>[]
  getItemId?: (item: TItem) => string | number
  renderContent?: (context: DashboardListRenderContext<TItem>) => ReactNode
  renderToolbarActions?: (context: DashboardListRenderContext<TItem>) => ReactNode
  getRowClassName?: (item: TItem) => string | undefined
  onRowClick?: (item: TItem) => void
  pageSize?: number
  enabled?: boolean
  reloadKey?: string | number | null
  layout?: "page" | "fragment"
  showToolbar?: boolean
  tableShellClassName?: string
  labels: {
    refresh?: string
    query?: string
    loading: string
    empty: string
    loadFailed: string
  }
}

export function DashboardListPage<TItem>({
  filters = [],
  fetchList,
  columns,
  getItemId,
  renderContent,
  renderToolbarActions,
  getRowClassName,
  onRowClick,
  pageSize,
  enabled,
  reloadKey,
  layout = "page",
  showToolbar = true,
  tableShellClassName,
  labels,
}: DashboardListPageProps<TItem>) {
  const list = useDashboardPagedList<TItem>({
    filters,
    fetchList,
    pageSize,
    enabled,
    reloadKey,
    loadFailed: labels.loadFailed,
  })
  const renderContext: DashboardListRenderContext<TItem> = {
    result: list.result,
    loading: list.loading,
    reload: list.loadData,
    resetFilters: list.resetFilters,
  }

  const content = (
    <>
      {showToolbar ? (
        <DashboardToolbar
          actions={
            <>
              {labels.refresh ? (
                <Button
                  variant="outline"
                  onClick={() => void list.loadData()}
                  disabled={list.loading}
                >
                  <RefreshCwIcon className={list.loading ? "animate-spin" : undefined} />
                  {labels.refresh}
                </Button>
              ) : null}
              {renderToolbarActions?.(renderContext)}
            </>
          }
        >
          {filters.map((filter) => {
            const value = list.draftFilters[filter.name]
            if (filter.type === "segment") {
              return (
                <div
                  key={filter.name}
                  className={filter.className ?? "flex flex-wrap gap-2"}
                >
                  {(filter.options ?? []).map((option) => (
                    <Button
                      key={option.value}
                      variant={String(value) === option.value ? "default" : "outline"}
                      onClick={() => list.applyFilter(filter.name, option.value)}
                    >
                      {option.label}
                    </Button>
                  ))}
                </div>
              )
            }
            if (filter.type === "select") {
              return (
                <div key={filter.name} className={filter.className ?? "w-full sm:w-40"}>
                  <OptionCombobox
                    value={String(value ?? "")}
                    onChange={(nextValue) =>
                      list.setDraftFilter(filter.name, nextValue || filter.defaultValue)
                    }
                    placeholder={filter.placeholder ?? filter.label}
                    searchPlaceholder={filter.searchPlaceholder}
                    emptyText={filter.emptyText}
                    options={[...(filter.options ?? [])]}
                  />
                </div>
              )
            }
            if (filter.type === "tag") {
              return (
                <div key={filter.name} className={filter.className ?? "w-full sm:w-48"}>
                  <TagSelector
                    mode="single"
                    value={Number(value ?? filter.defaultValue)}
                    onChange={(nextValue) => list.setDraftFilter(filter.name, nextValue)}
                    tags={filter.tags ?? []}
                    placeholder={filter.placeholder ?? filter.label}
                    searchPlaceholder={filter.searchPlaceholder}
                    emptyText={filter.emptyText}
                    rootOption={{
                      value: Number(filter.allValue ?? filter.defaultValue),
                      label: filter.placeholder ?? filter.label,
                    }}
                  />
                </div>
              )
            }

            return (
              <div key={filter.name} className={filter.className ?? "w-full sm:w-64"}>
                <SearchField
                  allowClear
                  value={String(value ?? "")}
                  onChange={(event) => list.applyFilter(filter.name, event.target.value)}
                  placeholder={filter.placeholder ?? filter.label}
                  aria-label={filter.label}
                />
              </div>
            )
          })}
        </DashboardToolbar>
      ) : null}

      <DashboardTableShell
        className={tableShellClassName}
        pagination={
          <ListPagination
            page={list.result.page.page}
            total={list.result.page.total}
            limit={list.result.page.limit}
            loading={list.loading}
            onPageChange={list.handlePageChange}
            onLimitChange={list.handleLimitChange}
          />
        }
      >
        {renderContent ? (
          renderContent(renderContext)
        ) : columns ? (
          <Table>
            <TableHeader className="bg-muted/40">
              <TableRow>
                {columns.map((column) => (
                  <TableHead key={column.key} className={column.className}>
                    {column.label}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {list.result.results.map((item, index) => {
                const key = getItemId ? getItemId(item) : index
                return (
                  <TableRow
                    key={key}
                    className={getRowClassName?.(item)}
                    onClick={onRowClick ? () => onRowClick(item) : undefined}
                  >
                    {columns.map((column) => (
                      <TableCell key={column.key} className={column.className}>
                        {column.render(item, renderContext)}
                      </TableCell>
                    ))}
                  </TableRow>
                )
              })}
              {list.loading || list.result.results.length === 0 ? (
                <DashboardTableStateRow
                  colSpan={columns.length}
                  loading={list.loading}
                  loadingText={labels.loading}
                  emptyText={labels.empty}
                />
              ) : null}
            </TableBody>
          </Table>
        ) : null}
      </DashboardTableShell>
    </>
  )

  if (layout === "fragment") {
    return content
  }

  return <DashboardPage>{content}</DashboardPage>
}
