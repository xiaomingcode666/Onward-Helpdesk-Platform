import type { ReactNode } from "react"

import { RouteBreadcrumbs } from "@/components/layout/route-breadcrumbs"
import { cn } from "@/lib/utils"

type PageHeaderProps = {
  actions?: ReactNode
  backAction?: ReactNode
  className?: string
  title: ReactNode
}

export function PageHeader(props: PageHeaderProps) {
  const {
    actions,
    backAction,
    className,
    title,
  } = props

  return (
    <header
      className={cn(
        "flex min-w-0 flex-col gap-2 border-b border-[var(--railops-border-light)] pb-2.5 sm:flex-row sm:items-end sm:justify-between",
        className
      )}
    >
      <div className="min-w-0">
        {backAction ? <div className="mb-3 flex items-center">{backAction}</div> : null}
        <RouteBreadcrumbs size="page" />
        <h1 className="sr-only">{title}</h1>
      </div>
      {actions ? (
        <div className="flex shrink-0 flex-wrap items-center gap-2 sm:justify-end">
          {actions}
        </div>
      ) : null}
    </header>
  )
}
