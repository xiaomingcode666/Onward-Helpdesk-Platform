"use client"

import {
  ArrowLeftIcon,
  BatteryWarningIcon,
  HouseIcon,
  Link2OffIcon,
  QrCodeIcon,
  VideoOffIcon,
  type LucideIcon,
} from "lucide-react"

import {
  EmptyState as RailopsEmptyState,
  ErrorState as RailopsErrorState,
  ForbiddenState as RailopsForbiddenState,
  RailopsButton,
} from "@railops/ui"

import { useI18n } from "@/i18n/provider"
import { Skeleton } from "@/components/ui/skeleton"

// ============================================================
// Shared error / empty / forbidden / quota / connector states
// 统一委托 @railops/ui 状态组件（保留对外 props 兼容层）
// ============================================================

type ActionConfig = {
  label?: string
  onClick?: () => void
  href?: string
  variant?: "default" | "outline" | "secondary"
  icon?: LucideIcon
}

function RailopsStateActionButton({ action }: { action: ActionConfig }) {
  const ActionIcon = action.icon
  const children = (
    <>
      {ActionIcon ? <ActionIcon className="size-4" /> : null}
      {action.label || "Retry"}
    </>
  )
  return (
    <RailopsButton
      href={action.href}
      variant={action.variant === "default" ? "primary" : "default"}
      onClick={action.onClick}
    >
      {children}
    </RailopsButton>
  )
}

type ErrorStateProps = {
  title?: string
  description?: string
  action?: ActionConfig
  secondaryAction?: ActionConfig
  className?: string
}

/**
 * Generic error state with retry button.
 */
export function ErrorState({ title, description, action, className }: ErrorStateProps) {
  const t = useI18n()
  return (
    <RailopsErrorState
      title={title || t("errorStates.loadFailedTitle")}
      description={description}
      action={action ? <RailopsStateActionButton action={action} /> : undefined}
      className={className}
    />
  )
}

/**
 * Empty state — shown when a list or search returns no results.
 */
export function EmptyState({ title, description, action, className }: ErrorStateProps) {
  const t = useI18n()
  return (
    <RailopsEmptyState
      title={title || t("errorStates.emptyTitle")}
      description={description}
      action={action ? <RailopsStateActionButton action={action} /> : undefined}
      className={className}
    />
  )
}

/**
 * Forbidden / access denied state.
 */
export function ForbiddenState({ title, description, action, secondaryAction, className }: ErrorStateProps) {
  const t = useI18n()
  return (
    <RailopsForbiddenState
      title={title || t("errorStates.forbiddenTitle")}
      description={description}
      action={
        action
          ? <RailopsStateActionButton action={action} />
          : <RailopsStateActionButton action={{ label: t("common.backHome"), href: "/", icon: HouseIcon }} />
      }
      secondaryAction={
        secondaryAction
          ? <RailopsStateActionButton action={secondaryAction} />
          : <RailopsStateActionButton action={{ label: t("common.back"), variant: "outline", onClick: () => window.history.back(), icon: ArrowLeftIcon }} />
      }
      className={className}
    />
  )
}

/** 领域状态统一容器（复用 railops 状态面板视觉）。 */
function DomainStateShell({
  icon: Icon,
  title,
  description,
  action,
  className,
  iconClassName,
}: ErrorStateProps & { icon: LucideIcon; iconClassName?: string }) {
  return (
    <div className={`railops-state-panel ${className || ""}`}>
      <div className="railops-state-body">
        <Icon className={`railops-state-icon ${iconClassName}`} />
        {title ? <div className="railops-state-title">{title}</div> : null}
        {description ? <div className="railops-state-description">{description}</div> : null}
        {action ? <div className="railops-state-actions"><RailopsStateActionButton action={action} /></div> : null}
      </div>
    </div>
  )
}

/**
 * Quota exceeded state.
 */
export function QuotaExceededState({ title, description, action, className }: ErrorStateProps) {
  const t = useI18n()
  return (
    <DomainStateShell
      icon={BatteryWarningIcon}
      title={title || t("errorStates.quotaExceededTitle")}
      description={description}
      action={action || { label: t("common.viewPlans") }}
      className={className}
      iconClassName="railops-state-icon-warning"
    />
  )
}

/**
 * Connector / integration failure state.
 */
export function ConnectorDownState({ title, description, action, className }: ErrorStateProps) {
  const t = useI18n()
  return (
    <DomainStateShell
      icon={Link2OffIcon}
      title={title || t("errorStates.connectorDownTitle")}
      description={description}
      action={action || { label: t("common.retry") }}
      className={className}
      iconClassName="railops-state-icon-error"
    />
  )
}

/**
 * Jitsi permission denied state.
 */
export function JitsiPermissionState({ title, description, action, className }: ErrorStateProps) {
  const t = useI18n()
  return (
    <DomainStateShell
      icon={VideoOffIcon}
      title={title || t("errorStates.videoPermissionDeniedTitle")}
      description={description}
      action={action}
      className={className}
      iconClassName="railops-state-icon-error"
    />
  )
}

/**
 * Service code revoked state.
 */
export function ScanRevokedState({
  title,
  description,
  action,
  serviceCode,
  revokedAt,
  className,
}: ErrorStateProps & { serviceCode?: string; revokedAt?: string }) {
  const t = useI18n()
  return (
    <div
      className={`flex flex-col items-center justify-center rounded-lg border bg-muted/30 p-12 text-center ${className || ""}`}
    >
      <QrCodeIcon className="mb-4 size-12 text-amber-600" />
      {title ? <h3 className="text-lg font-semibold">{title}</h3> : null}
      {description ? (
        <p className="mt-1 max-w-md text-sm text-muted-foreground">{description}</p>
      ) : (
        <p className="mt-1 max-w-md text-sm text-muted-foreground">
          {t("errorStates.serviceCodeRevokedDescription")}
        </p>
      )}
      {serviceCode || revokedAt ? (
        <div className="mt-4 space-y-2 rounded-md bg-background p-3 text-sm">
          {serviceCode ? (
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">{t("errorStates.serviceCode")}</span>
              <span className="font-mono">{serviceCode}</span>
            </div>
          ) : null}
          {revokedAt ? (
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">{t("errorStates.revokedAt")}</span>
              <span>{revokedAt}</span>
            </div>
          ) : null}
        </div>
      ) : null}
      {action ? <div className="mt-4"><RailopsStateActionButton action={action} /></div> : null}
    </div>
  )
}

/**
 * Generic loading skeleton.
 */
export function LoadingSkeleton({
  count = 3,
  layout = "list",
}: {
  count?: number
  layout?: "list" | "card" | "table"
}) {
  if (layout === "card") {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {Array.from({ length: count }).map((_, i) => (
          <div key={i} className="rounded-lg border p-4">
            <Skeleton className="h-5 w-3/4" />
            <Skeleton className="mt-2 h-3 w-1/2" />
            <Skeleton className="mt-4 h-4 w-full" />
            <Skeleton className="mt-1 h-4 w-2/3" />
            <div className="mt-3 flex gap-1.5">
              <Skeleton className="h-5 w-20 rounded-full" />
              <Skeleton className="h-5 w-16 rounded-full" />
            </div>
          </div>
        ))}
      </div>
    )
  }

  if (layout === "table") {
    return (
      <div className="rounded-lg border">
        <div className="border-b p-4">
          <Skeleton className="h-4 w-1/4" />
        </div>
        {Array.from({ length: count }).map((_, i) => (
          <div key={i} className="flex items-center gap-4 border-b p-4 last:border-b-0">
            <Skeleton className="h-4 flex-1" />
            <Skeleton className="h-4 w-20" />
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-4 w-24" />
          </div>
        ))}
      </div>
    )
  }

  // Default "list" layout
  return (
    <div className="space-y-3">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="flex items-center gap-4 rounded-lg border p-4">
          <Skeleton className="size-10 rounded-full" />
          <div className="flex-1 space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </div>
        </div>
      ))}
    </div>
  )
}
