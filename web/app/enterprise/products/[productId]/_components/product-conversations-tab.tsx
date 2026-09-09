"use client"

import Link from "next/link"
import { useI18n } from "@/i18n/provider"
import { getProductConversations, type ProductConversationBrief } from "@/lib/api/enterprise-products"
import { customerDisplayName } from "@/lib/customer-identity"
import { ProductResourceStatusTag, ProductResourceTable, type ProductResourceColumn } from "./product-resource-table"

export function ProductConversationsTab({ productId }: { productId: number }) {
  const t = useI18n()
  const columns: ProductResourceColumn<ProductConversationBrief>[] = [
    { key: "conversation", label: t("productArchive.columns.conversation"), render: (item) => <Link className="font-medium text-primary hover:underline" href={`/enterprise/ticket-workbench?conversationId=${item.id}`}>#{item.id}</Link> },
    { key: "customer", label: t("productArchive.columns.customer"), render: (item) => customerDisplayName(item.customer_name) },
    { key: "channel", label: t("productArchive.columns.channel"), render: (item) => item.channel || "-" },
    { key: "mode", label: t("productArchive.columns.serviceMode"), render: (item) => item.service_mode || "-" },
    { key: "status", label: t("productArchive.columns.status"), render: (item) => <ProductResourceStatusTag value={item.status} /> },
    { key: "updated", label: t("productArchive.columns.lastMessage"), render: (item) => item.last_message_at || "-" },
  ]
  return <ProductResourceTable productId={productId} load={getProductConversations} columns={columns} rowKey={(item) => item.id} emptyKey="productArchive.empty.conversations" />
}
