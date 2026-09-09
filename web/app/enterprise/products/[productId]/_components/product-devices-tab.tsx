"use client"

import Link from "next/link"
import { useI18n } from "@/i18n/provider"
import { getProductDevices, type ProductDevice } from "@/lib/api/enterprise-products"
import { ProductResourceStatusTag, ProductResourceTable, type ProductResourceColumn } from "./product-resource-table"

export function ProductDevicesTab({ productId }: { productId: number }) {
  const t = useI18n()
  const columns: ProductResourceColumn<ProductDevice>[] = [
    { key: "device", label: t("productArchive.columns.device"), render: (item) => <Link className="font-medium text-primary hover:underline" href={`/enterprise/devices?productId=${productId}&search=${encodeURIComponent(item.device_no)}`}>{item.device_no}</Link> },
    { key: "serial", label: t("productArchive.columns.serial"), render: (item) => item.serial_no || "-" },
    { key: "model", label: t("productArchive.columns.model"), render: (item) => item.model_name || "-" },
    { key: "region", label: t("productArchive.columns.region"), render: (item) => item.region_code || "-" },
    { key: "status", label: t("productArchive.columns.status"), render: (item) => <ProductResourceStatusTag value={item.status} /> },
    { key: "active", label: t("productArchive.columns.lastActive"), render: (item) => item.last_active_at || "-" },
  ]
  return <ProductResourceTable productId={productId} load={getProductDevices} columns={columns} rowKey={(item) => item.id} emptyKey="productArchive.empty.devices" />
}
