"use client"

import Link from "next/link"
import { useI18n } from "@/i18n/provider"
import { getProductRepairHistory, type ProductRepairRecord } from "@/lib/api/enterprise-products"
import { buildEnterpriseTicketWorkbenchPath } from "@/lib/ticket-workbench-route"
import { ProductResourceTable, type ProductResourceColumn } from "./product-resource-table"

export function ProductRepairHistoryTab({ productId }: { productId: number }) {
  const t = useI18n()
  const columns: ProductResourceColumn<ProductRepairRecord>[] = [
    { key: "ticket", label: t("productArchive.columns.ticket"), render: (item) => <Link className="font-mono font-medium text-primary hover:underline" href={buildEnterpriseTicketWorkbenchPath(item.ticket_id)}>{item.ticket_no}</Link> },
    { key: "device", label: t("productArchive.columns.device"), render: (item) => item.device_no || "-" },
    { key: "fault", label: t("productArchive.columns.fault"), render: (item) => item.fault_type || "-" },
    { key: "resolution", label: t("productArchive.columns.resolution"), className: "max-w-md", render: (item) => item.resolution || "-" },
    { key: "technician", label: t("productArchive.columns.technician"), render: (item) => item.technician || "-" },
    { key: "completed", label: t("productArchive.columns.completed"), render: (item) => item.completed_at || "-" },
  ]
  return <ProductResourceTable productId={productId} load={getProductRepairHistory} columns={columns} rowKey={(item) => item.id} emptyKey="productArchive.empty.repairs" />
}
