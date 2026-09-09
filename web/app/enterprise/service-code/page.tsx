import { redirect } from "next/navigation"

type SearchParams = Record<string, string | string[] | undefined>

function firstParam(value: string | string[] | undefined) {
  return Array.isArray(value) ? value[0] : value
}

export default function EnterpriseServiceCodePage({ searchParams }: { searchParams?: SearchParams }) {
  const params = new URLSearchParams({ view: "service-codes" })
  const productID = firstParam(searchParams?.product_id) || firstParam(searchParams?.productId)
  const search = firstParam(searchParams?.search)
  if (productID) params.set("product_id", productID)
  if (search) params.set("search", search)
  redirect(`/enterprise/devices?${params.toString()}`)
}
