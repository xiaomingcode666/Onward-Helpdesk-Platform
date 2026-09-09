"use client"

import { useI18n } from "@/i18n/provider"
import { getProductUsage, type ProductUsageMetric } from "@/lib/api/enterprise-products"
import { ProductResourceTable, type ProductResourceColumn } from "./product-resource-table"

export function ProductUsageTab({ productId }: { productId: number }) {
  const t = useI18n()
  const columns: ProductResourceColumn<ProductUsageMetric>[] = [
    { key: "metric", label: t("productArchive.columns.metric"), render: (item) => <strong className="font-medium text-foreground">{item.metric}</strong> },
    { key: "current", label: t("productArchive.columns.currentMonth"), render: (item) => `${item.current_month.toLocaleString()} ${item.unit}` },
    { key: "last", label: t("productArchive.columns.lastMonth"), render: (item) => `${item.last_month.toLocaleString()} ${item.unit}` },
    { key: "change", label: t("productArchive.columns.change"), render: (item) => item.last_month ? `${(((item.current_month - item.last_month) / item.last_month) * 100).toFixed(1)}%` : "-" },
  ]
  return <ProductResourceTable productId={productId} load={getProductUsage} columns={columns} rowKey={(item) => item.metric} emptyKey="productArchive.empty.usage" paginated={false} />
}
