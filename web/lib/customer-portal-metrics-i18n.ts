import type { CustomerPortalMetric } from "@/lib/api/customer-portal-types"

export function localizeCustomerPortalMetric(
  t: (key: string) => string,
  metric: CustomerPortalMetric,
): CustomerPortalMetric {
  switch (metric.key) {
    case "devices":
      return { ...metric, label: t("customerMetrics.devices.label"), meta: t("customerMetrics.devices.meta") }
    case "conversations":
      return { ...metric, label: t("customerMetrics.conversations.label"), meta: t("customerMetrics.conversations.meta") }
    case "tickets":
      return { ...metric, label: t("customerMetrics.tickets.label"), meta: t("customerMetrics.tickets.meta") }
    case "meetings":
      return { ...metric, label: t("customerMetrics.meetings.label"), meta: t("customerMetrics.meetings.meta") }
    default:
      return metric
  }
}
