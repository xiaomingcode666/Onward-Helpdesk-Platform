"use client"

import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react"

import { SelectField } from "@railops/ui"
import { Button } from "@/components/ui/button"
import { useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"

type ListPaginationProps = {
  page: number
  total: number
  limit: number
  loading?: boolean
  compact?: boolean
  pageSizeOptions?: number[]
  onPageChange: (page: number) => void
  onLimitChange: (limit: number) => void
}

export function ListPagination({
  page,
  total,
  limit,
  loading = false,
  compact = false,
  pageSizeOptions = [10, 20, 50, 100],
  onPageChange,
  onLimitChange,
}: ListPaginationProps) {
  const t = useI18n()
  const totalPages = Math.max(1, Math.ceil(total / limit))
  const canGoPreviousPage = page > 1
  const canGoNextPage = page < totalPages

  function handleLimitChange(value: string | null) {
    if (!value) {
      return
    }
    const nextLimit = Number(value)
    if (!Number.isInteger(nextLimit) || nextLimit <= 0 || nextLimit === limit) {
      return
    }
    onLimitChange(nextLimit)
  }

  return (
    <div className={cn("flex flex-wrap items-center justify-between gap-x-3 gap-y-2 text-sm text-muted-foreground", compact && "gap-x-2 gap-y-1.5 text-rhd-xs")}>
      <div className={cn("flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1", compact && "flex-nowrap whitespace-nowrap")}>
        <span className="tabular-nums">
          {t("pagination.pageSummary", { page, totalPages })}
        </span>
        {!compact ? <span className="tabular-nums">{t("pagination.total", { total })}</span> : null}
      </div>
      <div className={cn("flex flex-wrap items-center gap-2", compact && "ml-auto flex-nowrap")}>
        <SelectField
          style={{ marginBottom: 0 }}
          selectProps={{
            size: "small",
            "aria-label": t("pagination.pageSize", { pageSize: limit }),
            value: String(limit),
            onChange: handleLimitChange,
            options: pageSizeOptions.map((pageSize) => ({
              value: String(pageSize),
              label: t("pagination.pageSize", { pageSize }),
            })),
            popupMatchSelectWidth: 112,
            style: { width: 112 },
          }}
        />
        {!compact || totalPages > 1 ? (
          <>
            <Button
              size="sm"
              variant="outline"
              className={cn(compact && "size-8 px-0")}
              aria-label={compact ? t("pagination.previous") : undefined}
              title={compact ? t("pagination.previous") : undefined}
              onClick={() => onPageChange(page - 1)}
              disabled={loading || !canGoPreviousPage}
            >
              <ChevronLeftIcon className="size-4" />
              {!compact ? t("pagination.previous") : null}
            </Button>
            <Button
              size="sm"
              variant="outline"
              className={cn(compact && "size-8 px-0")}
              aria-label={compact ? t("pagination.next") : undefined}
              title={compact ? t("pagination.next") : undefined}
              onClick={() => onPageChange(page + 1)}
              disabled={loading || !canGoNextPage}
            >
              {!compact ? t("pagination.next") : null}
              <ChevronRightIcon className="size-4" />
            </Button>
          </>
        ) : null}
      </div>
    </div>
  )
}
