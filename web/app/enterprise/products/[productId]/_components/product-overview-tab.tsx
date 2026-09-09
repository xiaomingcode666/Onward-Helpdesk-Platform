"use client"

import { BookOpenIcon, BoxesIcon, LanguagesIcon, ShieldCheckIcon } from "lucide-react"
import { StatusTag, type StatusTagTone } from "@railops/ui"

import { useI18n } from "@/i18n/provider"
import type { ProductServiceProfile } from "@/lib/api/enterprise-products"

function statusTone(status?: string): StatusTagTone {
  switch (status) {
    case "active":
      return "success"
    case "inactive":
      return "disabled"
    case "discontinued":
      return "warning"
    default:
      return "neutral"
  }
}

function statusLabel(t: ReturnType<typeof useI18n>, status?: string) {
  if (status === "active") return t("productArchive.status.active")
  if (status === "inactive") return t("productArchive.status.inactive")
  if (status === "discontinued") return t("productArchive.status.discontinued")
  return status || t("productArchive.notConfigured")
}

export function ProductOverviewTab({ profile }: { profile: ProductServiceProfile }) {
  const t = useI18n()
  const product = profile.product
  const service = profile.serviceProfile
  const metrics = [
    { label: t("productArchive.metrics.devices"), value: product.device_count, icon: BoxesIcon },
    { label: t("productArchive.metrics.tickets"), value: profile.totalTicketCount, icon: ShieldCheckIcon },
    { label: t("productArchive.metrics.knowledge"), value: profile.totalKnowledgeEntryCount, icon: BookOpenIcon },
    { label: t("productArchive.metrics.aiResolve"), value: `${product.ai_resolve_rate || 0}%`, icon: LanguagesIcon },
  ]
  return (
    <div className="rhd-railops-product-archive-overview">
      <div className="rhd-railops-product-fault-metrics">
        {metrics.map(({ label, value, icon: Icon }) => (
          <div key={label} className="rhd-railops-product-micro-metric">
            <span className="rhd-railops-product-metric-label"><span>{label}</span><Icon className="size-3.5" /></span>
            <strong>{value}</strong>
          </div>
        ))}
      </div>
      <div className="rhd-railops-product-info-grid">
        <Info label={t("productArchive.fields.productLine")} value={product.product_line || "-"} />
        <Info label={t("productArchive.fields.category")} value={product.category || "-"} />
        <Info label={t("productArchive.fields.owner")} value={product.owner_name || t("productArchive.notConfigured")} />
        <Info label={t("productArchive.fields.locales")} value={(service.support_locales || []).join(", ") || product.default_locale || "-"} />
        <Info label={t("productArchive.fields.regions")} value={(service.support_regions || []).join(", ") || "-"} />
        <Info label={t("productArchive.fields.safetyLevel")} value={service.safety_level || "-"} />
        <Info label={t("productArchive.fields.video")} value={service.meeting_enabled ? t("productArchive.enabled") : t("productArchive.disabled")} />
        <Info label={t("productArchive.fields.knowledgeBase")} value={service.knowledge_base_name || t("productArchive.notConfigured")} />
        <div className="rhd-railops-product-info-item">
          <span>{t("productArchive.fields.status")}</span>
          <StatusTag tone={statusTone(product.status)}>{statusLabel(t, product.status)}</StatusTag>
        </div>
      </div>
      {product.description ? <p className="max-w-4xl text-sm leading-6 text-muted-foreground">{product.description}</p> : null}
    </div>
  )
}

function Info({ label, value }: { label: string; value: string }) {
  return <div className="rhd-railops-product-info-item"><span>{label}</span><strong>{value}</strong></div>
}
