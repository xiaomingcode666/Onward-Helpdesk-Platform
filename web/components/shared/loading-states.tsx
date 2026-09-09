"use client"

import { Loader2Icon } from "lucide-react"

import { Skeleton } from "@/components/ui/skeleton"
import { useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"

type ModuleLoadingVariant = "detail" | "list" | "metrics" | "table"

function LoadingRows({ count, variant }: { count: number; variant: ModuleLoadingVariant }) {
  if (variant === "metrics") {
    return (
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {Array.from({ length: count }, (_, index) => (
          <div key={index} className="min-w-0 border-l border-[var(--railops-border-light)] py-1 pl-3">
            <Skeleton className="h-3 w-20" />
            <Skeleton className="mt-3 h-7 w-24" />
            <Skeleton className="mt-2 h-3 w-28 max-w-full" />
          </div>
        ))}
      </div>
    )
  }

  if (variant === "table") {
    return (
      <div className="overflow-hidden rounded-[8px] bg-[var(--railops-surface)] shadow-[var(--railops-card-shadow)]">
        <div className="grid grid-cols-[minmax(0,1.4fr)_minmax(100px,0.8fr)_96px] gap-4 border-b border-[var(--railops-border-light)] bg-[var(--railops-surface-muted)] px-4 py-3">
          <Skeleton className="h-3 w-28" />
          <Skeleton className="h-3 w-20" />
          <Skeleton className="h-3 w-16" />
        </div>
        {Array.from({ length: count }, (_, index) => (
          <div key={index} className="grid min-h-11 grid-cols-[minmax(0,1.4fr)_minmax(100px,0.8fr)_96px] items-center gap-4 border-b border-[var(--railops-border-light)] px-4 last:border-b-0">
            <div className="min-w-0 space-y-2">
              <Skeleton className="h-4 w-2/3" />
              <Skeleton className="h-3 w-1/3" />
            </div>
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-6 w-16 rounded-md" />
          </div>
        ))}
      </div>
    )
  }

  if (variant === "detail") {
    return (
      <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_280px]">
        <div className="space-y-3 rounded-[8px] bg-[var(--railops-surface)] p-4 shadow-[var(--railops-card-shadow)]">
          <Skeleton className="h-5 w-48 max-w-full" />
          <Skeleton className="h-28 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
        <div className="space-y-3 rounded-[8px] bg-[var(--railops-surface)] p-4 shadow-[var(--railops-card-shadow)]">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      </div>
    )
  }

  return (
    <div className="divide-y divide-[var(--railops-border-light)] overflow-hidden rounded-[8px] bg-[var(--railops-surface)] shadow-[var(--railops-card-shadow)]">
      {Array.from({ length: count }, (_, index) => (
        <div key={index} className="flex min-h-11 items-center gap-3 px-4 py-3">
          <Skeleton className="size-9 shrink-0 rounded-md" />
          <div className="min-w-0 flex-1 space-y-2">
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-3 w-1/3" />
          </div>
          <Skeleton className="h-6 w-16 rounded-md" />
        </div>
      ))}
    </div>
  )
}

export function ModuleLoading({
  className,
  count = 4,
  label,
  variant = "list",
}: {
  className?: string
  count?: number
  label?: string
  variant?: ModuleLoadingVariant
}) {
  const t = useI18n()
  const resolvedLabel = label ?? t("common.loadingData")
  return (
    <div
      className={cn("relative min-w-0", className)}
      role="status"
      aria-busy="true"
      aria-label={resolvedLabel}
    >
      <div className="mb-2 flex h-7 items-center gap-2 text-xs font-medium text-[var(--railops-text-secondary)]">
        <Loader2Icon className="size-3.5 animate-spin" aria-hidden="true" />
        <span>{resolvedLabel}</span>
      </div>
      <LoadingRows count={count} variant={variant} />
    </div>
  )
}

export function RouteLoadingPage({ label }: { label?: string }) {
  const t = useI18n()
  const resolvedLabel = label ?? t("common.loading")
  return (
    <div className="grid min-w-0 gap-4" role="status" aria-busy="true" aria-label={resolvedLabel}>
      <header className="flex min-h-14 items-center justify-between gap-4 border-b border-[var(--railops-border-light)] pb-3">
        <div className="min-w-0 space-y-2">
          <Skeleton className="h-6 w-44 max-w-[60vw]" />
          <Skeleton className="h-3 w-72 max-w-[75vw]" />
        </div>
        <div className="flex shrink-0 items-center gap-2 text-xs font-medium text-[var(--railops-text-secondary)]">
          <span className="hidden sm:inline">{resolvedLabel}</span>
        </div>
      </header>
      <LoadingRows count={4} variant="metrics" />
      <div className="grid gap-4 xl:grid-cols-2">
        <LoadingRows count={4} variant="list" />
        <LoadingRows count={4} variant="list" />
      </div>
      <LoadingRows count={5} variant="table" />
    </div>
  )
}
