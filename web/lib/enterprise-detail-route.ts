export function normalizeEnterpriseDetailId(value: unknown): number {
  const normalized = typeof value === "number" ? value : Number(String(value ?? "").trim())
  return Number.isSafeInteger(normalized) && normalized > 0 ? normalized : 0
}

function buildEnterpriseDetailPath(
  pathname: string,
  parameter: string,
  value: unknown,
  currentSearch?: string | URLSearchParams,
): string {
  const id = normalizeEnterpriseDetailId(value)
  if (id <= 0) return pathname

  const search = new URLSearchParams(
    typeof currentSearch === "string" ? currentSearch : currentSearch?.toString(),
  )
  search.set(parameter, String(id))
  return `${pathname}?${search.toString()}`
}

export function buildEnterpriseWorkflowPath(
  workflowId: unknown,
  currentSearch?: string | URLSearchParams,
): string {
  return buildEnterpriseDetailPath(
    "/enterprise/workflow",
    "workflowId",
    workflowId,
    currentSearch,
  )
}

export function buildEnterpriseAIPath(
  agentId: unknown,
  currentSearch?: string | URLSearchParams,
): string {
  return buildEnterpriseDetailPath("/enterprise/ai", "agentId", agentId, currentSearch)
}

export function buildEnterpriseProductPath(
  productId: unknown,
  currentSearch?: string | URLSearchParams,
): string {
  return buildEnterpriseDetailPath(
    "/enterprise/products",
    "product_id",
    productId,
    currentSearch,
  )
}
