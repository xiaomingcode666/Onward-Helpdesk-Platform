"use client"

import type { ReactNode } from "react"
import { ContentModule, StatePanel } from "@railops/ui"

import { DEFAULT_LOCALE, readStoredLocale, type AppLocale } from "@/i18n/config"
import { translateCurrentMessage } from "@/i18n/messages"

type PlatformTranslate = (key: string, values?: Record<string, string | number>) => string

function activeLocale(locale?: AppLocale) {
  return locale ?? readStoredLocale() ?? DEFAULT_LOCALE
}

export function formatCompactNumber(value: number, locale = activeLocale()) {
  return new Intl.NumberFormat(locale, { notation: "compact", maximumFractionDigits: 1 }).format(value)
}

export function formatNumber(value: number, locale = activeLocale()) {
  return new Intl.NumberFormat(locale).format(value)
}

export function formatCurrency(value: number, locale = activeLocale()) {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(value)
}

export function formatDateTime(value?: string, locale = activeLocale()) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date)
}

export function formatDuration(totalSeconds: number, t: PlatformTranslate = translateCurrentMessage) {
  const days = Math.floor(totalSeconds / 86400)
  const hours = Math.floor((totalSeconds % 86400) / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  if (days > 0) return t("platformExtract.live.durationDaysHours", { days, hours })
  if (hours > 0) return t("platformExtract.live.durationHoursMinutes", { hours, minutes })
  return t("platformExtract.live.durationMinutes", { minutes })
}

export function formatBytes(value: number) {
  if (!value) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
  return `${(value / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`
}

export function PlatformError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <ContentModule className="rhd-railops-platform-error">
      <StatePanel state="error" errorDescription={message} onRetry={onRetry} />
    </ContentModule>
  )
}

export function PlatformEmpty({ title }: { title: string }) {
  return <StatePanel state="empty" emptyDescription={title} className="rhd-railops-platform-empty" />
}
