"use client"

import Link from "next/link"
import { useI18n } from "@/i18n/provider"
import { getProductServiceCodes, type ProductServiceCode } from "@/lib/api/enterprise-products"
import { ProductResourceStatusTag, ProductResourceTable, type ProductResourceColumn } from "./product-resource-table"

export function ProductServiceCodesTab({ productId }: { productId: number }) {
  const t = useI18n()
  const columns: ProductResourceColumn<ProductServiceCode>[] = [
    { key: "code", label: t("productArchive.columns.serviceCode"), render: (item) => <Link className="font-mono font-medium text-primary hover:underline" href={`/enterprise/devices?view=service-codes&product_id=${productId}&search=${encodeURIComponent(item.service_code)}`}>{item.service_code}</Link> },
    { key: "batch", label: t("productArchive.columns.batch"), render: (item) => item.batch_no || "-" },
    { key: "mode", label: t("productArchive.columns.mode"), render: (item) => item.mode },
    { key: "device", label: t("productArchive.columns.boundDevice"), render: (item) => item.bound_device || "-" },
    { key: "status", label: t("productArchive.columns.status"), render: (item) => <ProductResourceStatusTag value={item.status} /> },
    { key: "created", label: t("productArchive.columns.created"), render: (item) => item.created_at || "-" },
  ]
  return <ProductResourceTable productId={productId} load={getProductServiceCodes} columns={columns} rowKey={(item) => item.id} emptyKey="productArchive.empty.serviceCodes" />
}
