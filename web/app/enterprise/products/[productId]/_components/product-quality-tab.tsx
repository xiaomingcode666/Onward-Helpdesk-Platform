"use client"

import { useI18n } from "@/i18n/provider"
import { getProductQualitySignals, type ProductQualitySignal } from "@/lib/api/enterprise-products"
import { ProductResourceStatusTag, ProductResourceTable, type ProductResourceColumn } from "./product-resource-table"

export function ProductQualityTab({ productId }: { productId: number }) {
  const t = useI18n()
  const columns: ProductResourceColumn<ProductQualitySignal>[] = [
    { key: "title", label: t("productArchive.columns.signal"), exportValue: (item) => item.title, render: (item) => <div><strong className="font-medium text-foreground">{item.title}</strong>{item.description ? <p className="mt-1 max-w-md text-xs text-muted-foreground">{item.description}</p> : null}</div> },
    { key: "type", label: t("productArchive.columns.type"), exportValue: (item) => item.signal_type, render: (item) => item.signal_type },
    { key: "severity", label: t("productArchive.columns.severity"), exportValue: (item) => item.severity, render: (item) => <ProductResourceStatusTag value={item.severity} kind="severity" /> },
    { key: "sample", label: t("productArchive.columns.samples"), exportValue: (item) => item.sample_count, render: (item) => item.sample_count },
    { key: "status", label: t("productArchive.columns.status"), exportValue: (item) => item.status, render: (item) => <ProductResourceStatusTag value={item.status} /> },
    { key: "detected", label: t("productArchive.columns.detected"), exportValue: (item) => item.detected_at || "", render: (item) => item.detected_at || "-" },
  ]
  return <ProductResourceTable productId={productId} load={getProductQualitySignals} columns={columns} rowKey={(item) => item.id} emptyKey="productArchive.empty.quality" exportFilename={t("exportsExtract.csvQuality.filename", { productId })} exportTitle={t("exportsExtract.csvQuality.title")} paginated={false} />
}
