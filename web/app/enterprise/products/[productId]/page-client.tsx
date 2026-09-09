"use client"

import { useParams } from "next/navigation"
import { ProductDetailWorkspace } from "./_components/product-detail-workspace"

export default function EnterpriseProductDetailPageClient() {
  const params = useParams<{ productId: string }>()
  const productId = Number(params.productId)
  return <ProductDetailWorkspace productId={productId} />
}
