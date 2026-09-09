"use client"

import Link from "next/link"
import { useRouter } from "next/navigation"
import { useEffect } from "react"

import { useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"

type RouteAliasProps = {
  className?: string
  title?: string
  to: string
}

export function RouteAlias({
  className,
  title,
  to,
}: RouteAliasProps) {
  const router = useRouter()
  const t = useI18n()
  const resolvedTitle = title ?? t("routeAlias.title")

  useEffect(() => {
    router.replace(to)
  }, [router, to])

  return (
    <div className={cn("flex min-h-[40vh] items-center justify-center bg-[var(--railops-layout-background)] px-6 py-16 text-xs text-[var(--railops-text-secondary)]", className)}>
      <Link
        href={to}
        className="grid min-w-[220px] gap-1 rounded-md bg-[var(--railops-surface)] px-4 py-3 text-[var(--railops-text)] shadow-[var(--railops-card-shadow)] transition hover:text-[var(--railops-primary-hover)]"
      >
        <span className="font-semibold">{resolvedTitle}</span>
      </Link>
    </div>
  )
}
