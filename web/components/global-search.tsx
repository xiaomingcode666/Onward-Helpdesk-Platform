"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { useRouter } from "next/navigation"
import {
  SearchIcon,
  Loader2Icon,
  FileTextIcon,
  MonitorIcon,
  UsersIcon,
  BookOpenIcon,
  PackageIcon,
  CommandIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import { useI18n } from "@/i18n/provider"
import {
  globalSearch,
  type SearchResult,
  type SearchResultType,
} from "@/lib/api/search"

const SCOPE_OPTIONS: { value: SearchResultType; labelKey: string }[] = [
  { value: "ticket", labelKey: "globalSearch.scopeTickets" },
  { value: "device", labelKey: "globalSearch.scopeDevices" },
  { value: "customer", labelKey: "globalSearch.scopeCustomers" },
  { value: "knowledge", labelKey: "globalSearch.scopeKnowledge" },
  { value: "product", labelKey: "globalSearch.scopeProducts" },
]

const TYPE_ICONS: Record<SearchResultType, typeof FileTextIcon> = {
  ticket: FileTextIcon,
  device: MonitorIcon,
  customer: UsersIcon,
  knowledge: BookOpenIcon,
  product: PackageIcon,
}

const TYPE_LABEL_KEYS: Record<SearchResultType, string> = {
  ticket: "globalSearch.groupTickets",
  device: "globalSearch.groupDevices",
  customer: "globalSearch.groupCustomers",
  knowledge: "globalSearch.groupKnowledge",
  product: "globalSearch.groupProducts",
}

const METADATA_KEYS = ["code", "serialNo", "productLine", "category", "priority"] as const

function formatSearchMetadata(result: SearchResult) {
  const metadata = result.metadata ?? {}
  return METADATA_KEYS.map((key) => metadata[key]?.trim())
    .filter((value): value is string => Boolean(value))
    .slice(0, 3)
}

// ---- Component ----

export function GlobalSearch() {
  const t = useI18n()
  const router = useRouter()
  const inputRef = useRef<HTMLInputElement>(null)

  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const [scopes, setScopes] = useState<SearchResultType[]>([
    "ticket",
    "device",
    "customer",
    "knowledge",
    "product",
  ])
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [hasSearched, setHasSearched] = useState(false)

  // Keyboard shortcut: Cmd+K / Ctrl+K
  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        setOpen((prev) => !prev)
      }
    }
    document.addEventListener("keydown", down)
    return () => document.removeEventListener("keydown", down)
  }, [])

  // Perform search
  const doSearch = useCallback(async (q: string) => {
    if (!q.trim()) {
      setResults([])
      setHasSearched(false)
      return
    }
    if (scopes.length === 0) {
      setResults([])
      setHasSearched(true)
      return
    }
    setLoading(true)
    setHasSearched(true)
    try {
      const response = await globalSearch(q, scopes)
      setResults(response.success ? response.data?.items || [] : [])
    } catch {
      setResults([])
    } finally {
      setLoading(false)
    }
  }, [scopes])

  useEffect(() => {
    const timer = setTimeout(() => {
      if (query) {
        void doSearch(query)
      } else {
        setResults([])
        setHasSearched(false)
      }
    }, 200)
    return () => clearTimeout(timer)
  }, [query, doSearch])

  function toggleScope(scope: SearchResultType) {
    setScopes((prev) =>
      prev.includes(scope)
        ? prev.filter((s) => s !== scope)
        : [...prev, scope]
    )
  }

  function handleSelect(result: SearchResult) {
    setOpen(false)
    setQuery("")
    router.push(result.url)
  }

  const groupedResults = results.reduce<
    Record<SearchResultType, SearchResult[]>
  >(
    (acc, r) => {
      if (!acc[r.type]) acc[r.type] = []
      acc[r.type].push(r)
      return acc
    },
    { ticket: [], device: [], customer: [], knowledge: [], product: [] }
  )

  const hasAnyResults = results.length > 0

  return (
    <>
      {/* Search trigger button */}
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="relative h-9 w-full justify-start rounded-lg text-sm text-muted-foreground sm:pr-12 md:w-64 lg:w-80"
        onClick={() => setOpen(true)}
      >
        <SearchIcon className="mr-2 size-4 shrink-0" />
        <span className="truncate">
          {t("globalSearch.searchHint") || "Search tickets, devices..."}
        </span>
        <kbd className="pointer-events-none absolute right-2 hidden h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-rhd-2xs font-medium opacity-100 sm:flex">
          <CommandIcon className="size-3" />
          K
        </kbd>
      </Button>

      <CommandDialog open={open} onOpenChange={setOpen}>
        <CommandInput
          ref={inputRef}
          placeholder={t("globalSearch.placeholder") || "Search tickets, devices, customers..."}
          value={query}
          onValueChange={setQuery}
        />

        {/* Scope filters */}
        <div className="flex flex-wrap gap-1.5 border-b px-3 py-2">
          {SCOPE_OPTIONS.map((scope) => (
            <button
              key={scope.value}
              type="button"
              onClick={() => toggleScope(scope.value)}
              className={`rounded-full px-2.5 py-1 text-xs font-medium transition-colors ${
                scopes.includes(scope.value)
                  ? "bg-primary text-primary-foreground"
                  : "bg-muted text-muted-foreground hover:bg-muted/80"
              }`}
            >
              {t(scope.labelKey) || scope.value}
            </button>
          ))}
        </div>

        <CommandList>
          {/* Loading state */}
          {loading && (
            <div className="flex items-center justify-center py-12">
              <Loader2Icon className="size-6 animate-spin text-muted-foreground" />
            </div>
          )}

          {/* Empty state */}
          {!loading && hasSearched && !hasAnyResults && (
            <CommandEmpty>
              <div className="py-8 text-center">
                <SearchIcon className="mx-auto mb-3 size-10 text-muted-foreground" />
	                <p className="text-sm font-medium">
	                  {t("globalSearch.noResults") || "No results found"}
	                </p>
	              </div>
	            </CommandEmpty>
          )}

          {/* Results */}
          {!loading &&
            hasAnyResults &&
            (Object.entries(groupedResults) as [SearchResultType, SearchResult[]][]).map(
              ([type, items]) => {
                if (items.length === 0) return null
                const Icon = TYPE_ICONS[type]
                return (
                  <CommandGroup
                    key={type}
                    heading={
                      <span className="flex items-center gap-2">
                        <Icon className="size-3.5" />
                        {t(TYPE_LABEL_KEYS[type]) || type}
                        <Badge variant="secondary" className="ml-1 text-rhd-2xs">
                          {items.length}
                        </Badge>
                      </span>
                    }
                  >
	                    {items.map((result) => {
	                      const metadataValues = formatSearchMetadata(result)
	                      return (
	                        <CommandItem
	                          key={`${result.type}-${result.id}`}
	                          value={`${result.type}-${result.id}`}
	                          onSelect={() => handleSelect(result)}
	                          className="cursor-pointer"
	                        >
	                          <div className="flex items-start gap-3">
	                            <div className="flex size-8 items-center justify-center rounded-md bg-muted">
	                              <Icon className="size-4 text-muted-foreground" />
	                            </div>
	                            <div className="min-w-0 flex-1">
	                              <div className="flex items-center gap-2">
	                                <p className="truncate text-sm font-medium">
	                                  {result.title}
	                                </p>
	                                {result.metadata?.status && (
	                                  <Badge
	                                    variant="outline"
	                                    className="shrink-0 text-rhd-2xs"
	                                  >
	                                    {result.metadata.status.replace(/_/g, " ")}
	                                  </Badge>
	                                )}
	                              </div>
	                              {metadataValues.length > 0 ? (
	                                <div className="mt-1 flex flex-wrap gap-1">
	                                  {metadataValues.map((value) => (
	                                    <span key={value} className="max-w-32 truncate rounded bg-muted px-1.5 py-0.5 text-rhd-2xs font-medium text-muted-foreground">
	                                      {value.replace(/_/g, " ")}
	                                    </span>
	                                  ))}
	                                </div>
	                              ) : null}
	                            </div>
	                          </div>
	                        </CommandItem>
	                      )
	                    })}
                  </CommandGroup>
                )
              }
            )}

          {loading === false && !hasSearched && !query && (
            <div className="py-12 text-center text-sm text-muted-foreground">
              {t("globalSearch.searchHint") || "Start typing to search across all modules"}
            </div>
          )}
        </CommandList>
      </CommandDialog>
    </>
  )
}
