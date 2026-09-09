"use client"

import { CalendarDaysIcon } from "lucide-react"
import type { ReactNode } from "react"

import { FilterTabs, RailopsButton, type RailopsTabItem } from "@railops/ui"
import { useI18n } from "@/i18n/provider"

export const RAILOPS_DEFAULT_TIMEZONE = "Asia/Shanghai"

export type RailopsDateRangeValue = "7d" | "14d" | "30d" | "custom"
export type RailopsDateRangePreset = Exclude<RailopsDateRangeValue, "custom">
export type RailopsDateRange = {
  startDate: string
  endDate: string
}

export const RAILOPS_DATE_RANGE_ITEMS: RailopsTabItem[] = [
  { value: "7d", label: "7d" },
  { value: "14d", label: "14d" },
  { value: "30d", label: "30d" },
  { value: "custom", label: "custom" },
]

export function buildRailopsDateRangeItems(labels?: Partial<Record<RailopsDateRangeValue, string>>) {
  if (!labels) return RAILOPS_DATE_RANGE_ITEMS
  return RAILOPS_DATE_RANGE_ITEMS.map((item) => ({
    ...item,
    label: labels[item.value as RailopsDateRangeValue] ?? item.label,
  }))
}

export function formatRailopsDateInput(value: Date, timeZone = RAILOPS_DEFAULT_TIMEZONE) {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(value)
  const map = Object.fromEntries(parts.map((part) => [part.type, part.value]))
  return `${map.year}-${map.month}-${map.day}`
}

export function getRailopsDateRange(
  range: RailopsDateRangePreset,
  timeZone = RAILOPS_DEFAULT_TIMEZONE,
): RailopsDateRange {
  const days = Number(range.replace("d", ""))
  const end = new Date()
  const start = new Date(end)
  start.setDate(end.getDate() - Math.max(days - 1, 0))
  return {
    startDate: formatRailopsDateInput(start, timeZone),
    endDate: formatRailopsDateInput(end, timeZone),
  }
}

type RailopsDateRangeFilterProps<TValue extends string = RailopsDateRangeValue> = {
  ariaLabel: string
  items?: RailopsTabItem[]
  value: TValue
  onChange: (value: TValue) => void
  title?: string
  summary?: ReactNode
  startDate?: string
  endDate?: string
  startLabel?: string
  endLabel?: string
  applyLabel?: string
  loading?: boolean
  disabled?: boolean
  actions?: ReactNode
  className?: string
  onStartDateChange?: (value: string) => void
  onEndDateChange?: (value: string) => void
  onApply?: () => void
}

export function RailopsDateRangeFilter<TValue extends string>({
  actions,
  applyLabel,
  ariaLabel,
  className,
  disabled,
  endDate,
  endLabel,
  items,
  loading,
  onApply,
  onChange,
  onEndDateChange,
  onStartDateChange,
  startDate,
  startLabel,
  summary,
  title,
  value,
}: RailopsDateRangeFilterProps<TValue>) {
  const t = useI18n()
  const showDateFields = Boolean(onStartDateChange && onEndDateChange && value === "custom")
  const classNames = ["rhd-railops-date-filter", className].filter(Boolean).join(" ")
  const resolvedItems = items ?? buildRailopsDateRangeItems({
    "7d": t("railopsExtract.dateRange.last7"),
    "14d": t("railopsExtract.dateRange.last14"),
    "30d": t("railopsExtract.dateRange.last30"),
    custom: t("railopsExtract.dateRange.custom"),
  })
  const tabItems = disabled || loading
    ? resolvedItems.map((item) => ({ ...item, disabled: true }))
    : resolvedItems

  return (
    <div className={classNames}>
      <div className="rhd-railops-date-filter-main">
        {title || summary ? (
          <div className="rhd-railops-date-filter-title">
            <CalendarDaysIcon className="size-4" />
            <div>
              {title ? <strong>{title}</strong> : null}
              {summary ? <span>{summary}</span> : null}
            </div>
          </div>
        ) : null}
        <FilterTabs
          ariaLabel={ariaLabel}
          items={tabItems}
          value={value}
          onChange={(nextValue) => onChange(nextValue as TValue)}
        />
      </div>

      {showDateFields || actions || onApply ? (
        <div className="rhd-railops-date-filter-fields">
          {showDateFields ? (
            <>
              <label className="rhd-railops-date-field">
                <span>{startLabel ?? t("railopsExtract.dateRange.start")}</span>
                <input
                  type="date"
                  value={startDate || ""}
                  disabled={disabled || loading}
                  onChange={(event) => onStartDateChange?.(event.target.value)}
                />
              </label>
              <label className="rhd-railops-date-field">
                <span>{endLabel ?? t("railopsExtract.dateRange.end")}</span>
                <input
                  type="date"
                  value={endDate || ""}
                  disabled={disabled || loading}
                  onChange={(event) => onEndDateChange?.(event.target.value)}
                />
              </label>
            </>
          ) : null}
          {actions}
          {onApply ? (
            <RailopsButton
              className="rhd-railops-date-filter-apply"
              disabled={disabled}
              loading={loading}
              variant="primary"
              onClick={onApply}
            >
              {applyLabel ?? t("railopsExtract.dateRange.apply")}
            </RailopsButton>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}
