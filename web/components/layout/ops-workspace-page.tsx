"use client"

import { useEffect, useMemo, useState, type KeyboardEvent, type ReactNode } from "react"
import { EyeIcon } from "lucide-react"
import { Button as AntButton } from "antd"
import type { TableColumnsType } from "antd"
import {
  ContentModule,
  DataTable,
  FilterTabs,
  SearchField,
  StatusTag,
  TableToolbar,
  type StatusTagTone,
} from "@railops/ui"

import { PageHeader } from "@/components/layout/page-header"
import { useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"

export type WorkspaceMetric = {
  label: string
  meta: string
  tone?: "blue" | "green" | "amber" | "red" | "slate"
  value: string
}

export type WorkspaceAction = {
  disabled?: boolean
  href?: string
  label: string
  onClick?: () => void
  primary?: boolean
}

export type WorkspaceColumn = {
  key: string
  label: string
}

export type WorkspaceRowAction = {
  disabled?: boolean
  icon?: ReactNode
  label: string
  onClick: () => void
  tone?: "default" | "danger"
}

export type WorkspaceRow = {
  accessibleLabel?: string
  actions?: WorkspaceRowAction[]
  cells: Record<string, ReactNode>
  id: string
  onSelect?: () => void
  searchText?: string
  selected?: boolean
  tags?: string[]
}

export type WorkspacePanel = {
  items: Array<{
    label: string
    meta?: string
    value: ReactNode
  }>
  title: string
}

const statusToneMap: Record<NonNullable<WorkspaceMetric["tone"]>, StatusTagTone> = {
  amber: "warning",
  blue: "blue",
  green: "success",
  red: "error",
  slate: "neutral",
}

const WORKSPACE_PAGE_SIZE = 10

const opsWorkspaceI18nPrefix = "opsComponentsExtract.opsWorkspace."
type OpsWorkspaceT = ReturnType<typeof useI18n>
const ops = (t: OpsWorkspaceT, key: string, values?: Record<string, string | number>) =>
  t(`${opsWorkspaceI18nPrefix}${key}`, values)

export function StatusPill({
  children,
  tone = "slate",
}: {
  children: ReactNode
  tone?: keyof typeof statusToneMap
}) {
  return (
    <StatusTag tone={statusToneMap[tone]}>
      {children}
    </StatusTag>
  )
}

function Action({ action }: { action: WorkspaceAction }) {
  return (
    <AntButton
      type={action.primary ? "primary" : "default"}
      href={action.href}
      onClick={action.onClick}
      disabled={action.disabled}
    >
      {action.label}
    </AntButton>
  )
}

export function OpsWorkspacePage(props: {
  actions?: WorkspaceAction[]
  className?: string
  columns: WorkspaceColumn[]
  emptyMessage?: string
  inlineSearch?: boolean
  metrics: WorkspaceMetric[]
  panels: WorkspacePanel[]
  rows: WorkspaceRow[]
  searchPlaceholder?: string
  searchValue?: string
  showMetrics?: boolean
  onSearchChange?: (value: string) => void
  tabs?: string[]
  title: string
  toolbar?: ReactNode
}) {
  const t = useI18n()
  const {
    actions = [],
    className,
    columns,
    emptyMessage = ops(t, "emptyMessage"),
    inlineSearch = false,
    metrics,
    panels,
    rows,
    searchPlaceholder = ops(t, "searchPlaceholder"),
    searchValue,
    showMetrics = true,
    onSearchChange,
    tabs = [],
    title,
    toolbar,
  } = props
  const [activeTab, setActiveTab] = useState(tabs[0] ?? "")
  const [currentPage, setCurrentPage] = useState(1)
  const [internalSearch, setInternalSearch] = useState("")
  const effectiveActiveTab = tabs.includes(activeTab) ? activeTab : tabs[0] ?? ""
  const search = searchValue ?? internalSearch
  const handleSearchChange = (value: string) => {
    if (onSearchChange) {
      onSearchChange(value)
      return
    }
    setInternalSearch(value)
  }
  const visibleRows = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    return rows.filter((row) => {
      if (effectiveActiveTab && effectiveActiveTab !== tabs[0] && !row.tags?.includes(effectiveActiveTab)) {
        return false
      }
      return !keyword || (row.searchText ?? "").toLowerCase().includes(keyword)
    })
  }, [effectiveActiveTab, rows, search, tabs])
  useEffect(() => {
    setCurrentPage(1)
  }, [effectiveActiveTab, search, rows.length])
  const hasSelectableRows = rows.some((row) => row.onSelect)
  const hasRowActions = rows.some((row) => row.actions?.length)
  const totalPages = Math.max(1, Math.ceil(visibleRows.length / WORKSPACE_PAGE_SIZE))
  const normalizedPage = Math.min(currentPage, totalPages)
  const pagedRows = useMemo(() => {
    const start = (normalizedPage - 1) * WORKSPACE_PAGE_SIZE
    return visibleRows.slice(start, start + WORKSPACE_PAGE_SIZE)
  }, [normalizedPage, visibleRows])
  const tabItems = useMemo(() => tabs.map((tab, index) => ({
    count: index === 0 ? rows.length : rows.filter((row) => row.tags?.includes(tab)).length,
    label: tab,
    value: tab,
  })), [rows, tabs])
  const tableColumns = useMemo<TableColumnsType<WorkspaceRow>>(() => {
    const baseColumns: TableColumnsType<WorkspaceRow> = columns.map((column) => ({
      key: column.key,
      title: column.label,
      render: (_value, row) => row.cells[column.key],
    }))
    if (hasRowActions) {
      baseColumns.push({
        align: "right",
        key: "actions",
        title: ops(t, "actionsColumn"),
        width: 168,
        render: (_value, row) => row.actions?.length ? (
          <div className="rhd-railops-ops-row-actions">
            {row.actions.map((action) => (
              <AntButton
                key={action.label}
                danger={action.tone === "danger"}
                disabled={action.disabled}
                icon={action.icon}
                size="small"
                type="text"
                onClick={(event) => {
                  event.stopPropagation()
                  action.onClick()
                }}
              >
                {action.label}
              </AntButton>
            ))}
          </div>
        ) : null,
      })
    }
    if (hasSelectableRows) {
      baseColumns.push({
        align: "center",
        key: "detail",
        title: ops(t, "view"),
        width: 58,
        render: (_value, row) => row.onSelect ? (
          <AntButton
            aria-label={ops(t, "viewDetail", { label: row.accessibleLabel || ops(t, "record") })}
            className="rhd-railops-ops-view-action"
            icon={<EyeIcon className="size-4" />}
            size="small"
            title={ops(t, "viewDetailTitle")}
            type="text"
            onClick={(event) => {
              event.stopPropagation()
              row.onSelect?.()
            }}
          />
        ) : null,
      })
    }
    return baseColumns
  }, [columns, hasRowActions, hasSelectableRows, t])

  return (
    <div className={cn("rhd-railops-ops-page space-y-4", className)}>
      <PageHeader
        title={title}
        actions={actions.length ? (
          <>
            {actions.map((action) => (
              <Action key={action.label} action={action} />
            ))}
          </>
        ) : undefined}
      />

      {showMetrics && metrics.length ? (
        <section className="rhd-railops-ops-metrics" aria-label={ops(t, "metricsAria")}>
          {metrics.map((metric) => (
            <div
              key={metric.label}
              className="rhd-railops-ops-metric"
            >
              <div className="text-xs font-medium text-muted-foreground">{metric.label}</div>
              <div className="mt-2 flex items-baseline gap-2">
                <span className="text-2xl font-semibold text-foreground">{metric.value}</span>
                {metric.meta ? (
                  <StatusTag tone={statusToneMap[metric.tone ?? "slate"]}>
                    {metric.meta}
                  </StatusTag>
                ) : null}
              </div>
            </div>
          ))}
        </section>
      ) : null}

      <section className={cn("rhd-railops-ops-workspace", panels.length > 0 && "has-panels")}>
        <ContentModule
          className="rhd-railops-ops-list-module"
          title={ops(t, "dataList")}
          note={ops(t, "count", { count: visibleRows.length })}
        >
          {toolbar || inlineSearch ? (
            <div className="rhd-railops-ops-extra-toolbar">
              <div className="flex flex-wrap items-center gap-2">
                {toolbar}
                {inlineSearch ? (
                  <SearchField
                    allowClear
                    aria-label={searchPlaceholder}
                    className="rhd-railops-ops-search rhd-railops-ops-inline-search"
                    placeholder={searchPlaceholder}
                    value={search}
                    onChange={(event) => handleSearchChange(event.target.value)}
                  />
                ) : null}
              </div>
            </div>
          ) : null}
          <TableToolbar
            tabs={tabItems.length ? (
              <FilterTabs
                ariaLabel={ops(t, "filterAria", { title })}
                items={tabItems}
                value={effectiveActiveTab}
                onChange={setActiveTab}
              />
            ) : undefined}
            search={inlineSearch ? undefined : (
              <SearchField
                allowClear
                aria-label={searchPlaceholder}
                className="rhd-railops-ops-search"
                placeholder={searchPlaceholder}
                value={search}
                onChange={(event) => handleSearchChange(event.target.value)}
              />
            )}
          />
          <DataTable<WorkspaceRow>
            className="rhd-railops-ops-table"
            columns={tableColumns}
            current={normalizedPage}
            dataSource={pagedRows}
            emptyDescription={rows.length ? ops(t, "noFilteredRecords") : emptyMessage}
            footerNote={ops(t, "footerNote", { count: visibleRows.length, page: normalizedPage, totalPages })}
            pageSize={WORKSPACE_PAGE_SIZE}
            pagination={false}
            rowClassName={(row) => cn(row.onSelect && "is-clickable", row.selected && "is-selected")}
            rowKey="id"
            scroll={{ x: 980 }}
            total={visibleRows.length}
            onPageChange={(page) => setCurrentPage(page)}
            onRow={(row) => ({
              "aria-label": row.accessibleLabel,
              "aria-selected": row.selected || undefined,
              onClick: row.onSelect,
              onKeyDown: (event) => handleRowKeyDown(event, row.onSelect),
              tabIndex: row.onSelect ? 0 : undefined,
            })}
          />
        </ContentModule>

        {panels.length > 0 ? (
          <aside className="rhd-railops-ops-panels">
            {panels.map((panel) => (
              <ContentModule key={panel.title} className="rhd-railops-ops-panel" title={panel.title}>
                <div className="space-y-3">
                  {panel.items.map((item) => (
                    <div key={`${panel.title}-${item.label}`} className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="text-sm font-medium text-foreground">{item.label}</div>
                        {item.meta ? (
                          <div className="mt-0.5 text-xs text-muted-foreground">{item.meta}</div>
                        ) : null}
                      </div>
                      <div className="shrink-0 text-right text-sm font-semibold text-foreground">
                        {item.value}
                      </div>
                    </div>
                  ))}
                </div>
              </ContentModule>
            ))}
          </aside>
        ) : null}
      </section>
    </div>
  )
}

function handleRowKeyDown(event: KeyboardEvent<HTMLElement>, onSelect?: () => void) {
  if (!onSelect || (event.key !== "Enter" && event.key !== " ")) return
  event.preventDefault()
  onSelect()
}
