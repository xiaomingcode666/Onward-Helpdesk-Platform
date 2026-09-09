import { apiGet } from "@/lib/api/client"
import type { ApiResponse } from "@/lib/api/types"

// ---- Types ----

export type SearchResultType = "ticket" | "device" | "customer" | "knowledge" | "product"

export interface SearchResult {
  id: string
  type: SearchResultType
  title: string
  description: string
  url: string
  metadata?: Record<string, string>
}

export interface SearchResults {
  items: SearchResult[]
  total: number
  query: string
  scopes: string[]
  suggestions: string[]
  recentSearches: string[]
}

// ---- API call ----

/**
 * Perform a global search across multiple scopes.
 *
 * @param query - The search term
 * @param scopes - Array of scope strings (ticket, device, customer, knowledge, product)
 */
export async function globalSearch(
  query: string,
  scopes: string[],
): Promise<ApiResponse<SearchResults>> {
  return apiGet<SearchResults>("/search/global", {
    q: query,
    scopes: scopes.join(","),
  })
}
