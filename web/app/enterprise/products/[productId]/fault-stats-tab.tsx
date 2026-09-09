"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import { ArrowDownIcon, ArrowUpIcon, DownloadIcon, MinusIcon, RefreshCwIcon } from "lucide-react"
import type { TableColumnsType } from "antd"
import { ContentModule, DataTable, RailopsButton, StatusTag, type StatusTagTone } from "@railops/ui"

import { RailopsDateRangeFilter } from "@/components/railops/date-range-filter"
import { getProductFaultStats, rebuildProductFaultStats } from "@/lib/api/enterprise-products"
import { ErrorState, EmptyState, LoadingSkeleton } from "@/components/shared/error-states"
import { useI18n } from "@/i18n/provider"
import type { FaultStat, ProductFaultStats } from "@/lib/api/enterprise-products"
import { downloadCsv, type CsvRow } from "@/lib/csv-export"

interface FaultStatsTabProps {
  productId: number
}

type I18nT = ReturnType<typeof useI18n>
type FaultRange = "30d" | "90d" | "180d"

export function FaultStatsTab({ productId }: FaultStatsTabProps) {
  const t = useI18n()
  const [stats, setStats] = useState<ProductFaultStats | null>(null)
  const [range, setRange] = useState<FaultRange>("90d")
  const [loading, setLoading] = useState(true)
  const [statsLoaded, setStatsLoaded] = useState(false)
  const [error, setError] = useState("")
  const [rebuilding, setRebuilding] = useState(false)
  const [rebuildMessage, setRebuildMessage] = useState("")
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  const load = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const res = await getProductFaultStats(productId, range)
      if (res.success) {
        setStats(res.data)
        setStatsLoaded(true)
      } else {
        setError(res.error?.message || t("productArchive.faultStats.loadFailed"))
      }
    } catch (err) {
      setError(
        err instanceof Error ? err.message : t("productArchive.faultStats.loadFailed")
      )
    } finally {
      setLoading(false)
    }
  }, [productId, range, t])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    setPage(1)
  }, [range, stats?.stats.length])

  async function handleRebuild() {
    setRebuilding(true)
    setError("")
    setRebuildMessage("")
    try {
      const response = await rebuildProductFaultStats(productId, range)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("productArchive.faultStats.rebuildFailed"))
      }
      setRebuildMessage(
        response.data.status === "completed"
          ? t("productArchive.faultStats.rebuildCompleted")
          : t("productArchive.faultStats.rebuildQueued")
      )
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : t("productArchive.faultStats.rebuildFailed"))
    } finally {
      setRebuilding(false)
    }
  }

  function handleExport() {
    if (!stats?.stats.length) return
    const rows: CsvRow[] = [
      [t("exportsExtract.csvFaultStats.header.title")],
      [t("exportsExtract.csvFaultStats.header.productId"), productId],
      [t("exportsExtract.csvFaultStats.header.range"), rangeLabel(t, range)],
      [t("exportsExtract.csvFaultStats.header.generatedAt"), stats.generated_at || ""],
      [t("exportsExtract.csvFaultStats.header.exportedAt"), new Date().toLocaleString("zh-CN", { hour12: false })],
      [],
      [t("exportsExtract.csvFaultStats.header.summary")],
      [t("exportsExtract.csvFaultStats.header.faultTotal"), stats.total_faults],
      [t("exportsExtract.csvFaultStats.header.affectedDevices"), stats.affected_devices],
      [],
      [
        t("exportsExtract.csvFaultStats.columns.part"),
        t("exportsExtract.csvFaultStats.columns.faultType"),
        t("exportsExtract.csvFaultStats.columns.model"),
        t("exportsExtract.csvFaultStats.columns.count"),
        t("exportsExtract.csvFaultStats.columns.share"),
        t("exportsExtract.csvFaultStats.columns.trend"),
        t("exportsExtract.csvFaultStats.columns.severity"),
      ],
      ...stats.stats.map((item) => [
        item.part,
        item.fault_type,
        item.model_name,
        item.count,
        `${item.percentage.toFixed(1)}%`,
        item.trend,
        severityLabel(t, item.severity),
      ]),
    ]
    downloadCsv(t("exportsExtract.csvFaultStats.filename", { productId, range }), rows)
  }

  const initialLoading = loading && !statsLoaded
  const hasStats = (stats?.stats.length ?? 0) > 0
  const topFault = stats?.stats[0]
  const tableSummary = stats
    ? `${rangeLabel(t, range)} · ${t("pagination.total", { total: formatStatNumber(stats.stats.length) })}`
    : rangeLabel(t, range)
  const pagedStats = useMemo(
    () => (stats?.stats ?? []).slice((page - 1) * pageSize, page * pageSize),
    [page, pageSize, stats?.stats],
  )
  const faultColumns: TableColumnsType<FaultStat> = [
    {
      title: t("productArchive.faultStats.columns.fault"),
      dataIndex: "fault_type",
      key: "fault_type",
      width: 180,
      align: "left",
      render: (_, item) => (
        <div className="rhd-railops-table-cell">
          <strong>{item.fault_type || "-"}</strong>
          <span>{item.part || "-"}</span>
        </div>
      ),
    },
    {
      title: t("productArchive.faultStats.columns.model"),
      dataIndex: "model_name",
      key: "model_name",
      width: 150,
      align: "left",
      render: (value) => value || "-",
    },
    {
      title: t("productArchive.faultStats.columns.count"),
      dataIndex: "count",
      key: "count",
      width: 110,
      align: "right",
      render: (value) => formatStatNumber(Number(value)),
    },
    {
      title: t("productArchive.faultStats.columns.share"),
      dataIndex: "percentage",
      key: "percentage",
      width: 180,
      align: "left",
      render: (value) => {
        const percent = Number(value) || 0
        return (
          <div className="rhd-railops-percent-cell">
            <span className="rhd-railops-percent-track">
              <i style={{ width: `${Math.max(0, Math.min(percent, 100))}%` }} />
            </span>
            <strong>{percent.toFixed(1)}%</strong>
          </div>
        )
      },
    },
    {
      title: t("productArchive.faultStats.columns.trend"),
      dataIndex: "trend",
      key: "trend",
      width: 130,
      align: "center",
      render: (value) => <TrendStatus t={t} trend={String(value)} />,
    },
    {
      title: t("productArchive.faultStats.columns.severity"),
      dataIndex: "severity",
      key: "severity",
      width: 130,
      align: "center",
      render: (value) => (
        <StatusTag tone={severityTone(String(value))}>
          {severityLabel(t, String(value))}
        </StatusTag>
      ),
    },
  ]

  return (
    <div className="rhd-railops-product-tab rhd-railops-product-fault-tab">
      <section className="rhd-railops-product-fault-shell">
        <div className="rhd-railops-product-fault-head">
          <div>
            <strong>{t("productCenter.tabs.faultStats")}</strong>
            <span>{tableSummary}</span>
          </div>
          <RangeSelector value={range} onChange={setRange} rebuilding={rebuilding} exportDisabled={!stats?.stats.length || loading} onExport={handleExport} onRebuild={handleRebuild} />
        </div>

        {error ? (
          <div className="rhd-railops-product-tab-error">
            <ErrorState
              title={t("productArchive.faultStats.loadFailed")}
              description={error}
              action={{ label: t("productArchive.retry"), onClick: load }}
            />
          </div>
        ) : null}
        {initialLoading ? <LoadingSkeleton count={3} layout="table" /> : null}
        {!initialLoading && !error && !hasStats ? (
          <EmptyState
            title={t("productArchive.faultStats.emptyTitle")}
            className="rhd-railops-product-tab-empty"
          />
        ) : null}
        {!initialLoading && hasStats && stats ? (
          <>
            {rebuildMessage ? (
              <div className="rhd-railops-product-tab-notice">
                <StatusTag tone="blue">{rebuildMessage}</StatusTag>
              </div>
            ) : null}
            <div className="rhd-railops-product-fault-metrics">
              <FaultMetric label={t("productArchive.faultStats.summary.faults")} value={formatStatNumber(stats.total_faults)} />
              <FaultMetric label={t("productArchive.faultStats.summary.devices")} value={formatStatNumber(stats.affected_devices)} />
              <FaultMetric label={t("productArchive.faultStats.summary.topFault")} value={topFault?.fault_type || "-"} />
              <FaultMetric
                label={t("productArchive.faultStats.summary.peakShare")}
                value={topFault ? `${topFault.percentage.toFixed(1)}%` : "-"}
              />
            </div>

            <ContentModule
              className="rhd-railops-product-table-module rhd-railops-product-fault-table-module"
              title={t("productArchive.faultStats.distributionTitle")}
            >
              <DataTable<FaultStat>
                className="rhd-railops-product-table rhd-railops-product-fault-table"
                columns={faultColumns}
                dataSource={pagedStats}
                rowKey={(item) => `${item.part}-${item.fault_type}-${item.model_name}`}
                scroll={{ x: 900 }}
                size="middle"
                total={stats.stats.length}
                current={page}
                pageSize={pageSize}
                paginationProps={{ showSizeChanger: true }}
                onPageChange={(nextPage, nextPageSize) => {
                  if (nextPageSize !== pageSize) {
                    setPageSize(nextPageSize)
                    setPage(1)
                    return
                  }
                  setPage(nextPage)
                }}
              />
            </ContentModule>
          </>
        ) : null}
      </section>
    </div>
  )
}

function RangeSelector({
  exportDisabled,
  value,
  onChange,
  onExport,
  rebuilding,
  onRebuild,
}: {
  exportDisabled: boolean
  value: "30d" | "90d" | "180d"
  onChange: (value: "30d" | "90d" | "180d") => void
  onExport: () => void
  rebuilding: boolean
  onRebuild: () => void
}) {
  const t = useI18n()
  const items = useMemo(
    () => (["30d", "90d", "180d"] as const).map((item) => ({
      value: item,
      label: rangeLabel(t, item),
    })),
    [t],
  )

  return (
    <div className="rhd-railops-product-tab-toolbar">
      <RailopsDateRangeFilter
        ariaLabel={t("productArchive.faultStats.distributionTitle")}
        className="rhd-railops-product-range-filter"
        items={items}
        value={value}
        onChange={onChange}
        actions={(
          <div className="flex items-center gap-2">
            <RailopsButton
              size="small"
              icon={<DownloadIcon className="size-3.5" />}
              onClick={onExport}
              disabled={exportDisabled}
            >
              {t("exportsExtract.csvFaultStats.exportCsv")}
            </RailopsButton>
            <RailopsButton
              size="small"
              className="rhd-railops-product-fault-rebuild"
              icon={<RefreshCwIcon className={rebuilding ? "size-3.5 animate-spin" : "size-3.5"} />}
              onClick={onRebuild}
              disabled={rebuilding}
            >
              {rebuilding ? t("productArchive.faultStats.rebuilding") : t("productArchive.faultStats.rebuild")}
            </RailopsButton>
          </div>
        )}
      />
    </div>
  )
}

function FaultMetric({
  label,
  value,
}: {
  label: string
  value: string
}) {
  return (
    <div className="rhd-railops-product-micro-metric">
      <span>{label}</span>
      <strong title={value}>{value}</strong>
    </div>
  )
}

function TrendStatus({ t, trend }: { t: I18nT; trend: string }) {
  if (trend === "up") {
    return (
      <StatusTag tone="error">
        <ArrowUpIcon className="size-3" />
        {t("productArchive.faultStats.trends.up")}
      </StatusTag>
    )
  }
  if (trend === "down") {
    return (
      <StatusTag tone="success">
        <ArrowDownIcon className="size-3" />
        {t("productArchive.faultStats.trends.down")}
      </StatusTag>
    )
  }
  return (
    <StatusTag tone="neutral">
      <MinusIcon className="size-3" />
      {t("productArchive.faultStats.trends.flat")}
    </StatusTag>
  )
}

function severityTone(severity: string): StatusTagTone {
  switch ((severity || "").toLowerCase()) {
    case "critical":
    case "high":
      return "error"
    case "medium":
      return "warning"
    case "low":
      return "success"
    default:
      return "neutral"
  }
}

function formatStatNumber(value: number) {
  return new Intl.NumberFormat("zh-CN").format(Number.isFinite(value) ? value : 0)
}

function rangeLabel(t: I18nT, value: FaultRange) {
  switch (value) {
    case "30d":
      return t("productArchive.faultStats.range.30d")
    case "90d":
      return t("productArchive.faultStats.range.90d")
    case "180d":
      return t("productArchive.faultStats.range.180d")
  }
}

function severityLabel(t: I18nT, severity: string) {
  switch ((severity || "").toLowerCase()) {
    case "critical":
      return t("productArchive.faultStats.severity.critical")
    case "high":
      return t("productArchive.faultStats.severity.high")
    case "medium":
      return t("productArchive.faultStats.severity.medium")
    case "low":
      return t("productArchive.faultStats.severity.low")
    default:
      return severity || t("productArchive.faultStats.severity.default")
  }
}
