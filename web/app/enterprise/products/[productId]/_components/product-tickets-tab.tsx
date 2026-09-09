"use client"

import Link from "next/link"
import { useI18n } from "@/i18n/provider"
import { getProductTickets, type ProductTicketBrief } from "@/lib/api/enterprise-products"
import { buildEnterpriseTicketWorkbenchPath } from "@/lib/ticket-workbench-route"
import { ProductResourceStatusTag, ProductResourceTable, type ProductResourceColumn } from "./product-resource-table"

export function ProductTicketsTab({ productId }: { productId: number }) {
  const t = useI18n()
  const columns: ProductResourceColumn<ProductTicketBrief>[] = [
    { key: "ticket", label: t("productArchive.columns.ticket"), render: (item) => <Link className="font-mono font-medium text-primary hover:underline" href={buildEnterpriseTicketWorkbenchPath(item.id)}>{item.ticket_no}</Link> },
    { key: "title", label: t("productArchive.columns.title"), render: (item) => item.title },
    { key: "priority", label: t("productArchive.columns.priority"), render: (item) => <ProductResourceStatusTag value={item.priority} kind="priority" /> },
    { key: "status", label: t("productArchive.columns.status"), render: (item) => <ProductResourceStatusTag value={item.status} /> },
    { key: "created", label: t("productArchive.columns.created"), render: (item) => item.created_at || "-" },
  ]
  return <ProductResourceTable productId={productId} load={getProductTickets} columns={columns} rowKey={(item) => item.id} emptyKey="productArchive.empty.tickets" />
}
