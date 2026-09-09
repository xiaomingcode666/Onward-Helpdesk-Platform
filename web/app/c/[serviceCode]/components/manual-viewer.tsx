"use client"

import { useState, useCallback, useMemo, useRef, type ElementRef } from "react"
import {
  FileTextIcon,
  SearchIcon,
  MenuIcon,
  XIcon,
  Loader2Icon,
  ChevronRightIcon,
  FileIcon,
  AlertCircleIcon,
  BookOpenIcon,
  ExternalLinkIcon,
} from "lucide-react"

import { SearchField } from "@railops/ui"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Skeleton } from "@/components/ui/skeleton"
import { Separator } from "@/components/ui/separator"
import { useI18n } from "@/i18n/provider"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type ManualState = "loading" | "viewing" | "searching"

interface TocItem {
  id: string
  label: string
  page?: number
  content?: string
  contentType?: string
  url?: string
  children?: TocItem[]
}

interface ManualViewerProps {
  /** Table of contents */
  toc?: TocItem[]
  /** Title of the manual */
  title?: string
  /** URL to the PDF or web manual */
  sourceUrl?: string
  /** Called when a TOC item is selected */
  onNavigate?: (item: TocItem) => void
}

const DEFAULT_TOC: TocItem[] = []

// ---------------------------------------------------------------------------
// Search highlight helper
// ---------------------------------------------------------------------------

function highlightText(text: string, query: string): React.ReactNode {
  if (!query.trim()) return text
  const regex = new RegExp(`(${query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")})`, "gi")
  const parts = text.split(regex)
  return parts.map((part, i) =>
    regex.test(part) ? (
      <span key={i} className="rounded bg-yellow-200 px-0.5 text-foreground">
        {part}
      </span>
    ) : (
      part
    )
  )
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export function ManualViewer({
  toc = DEFAULT_TOC,
  title,
  sourceUrl,
  onNavigate,
}: ManualViewerProps) {
  const t = useI18n()

  const [state, setState] = useState<ManualState>("viewing")
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [currentSection, setCurrentSection] = useState<string>(toc[0]?.id ?? "")
  const [expandedSections, setExpandedSections] = useState<Set<string>>(
    new Set(toc.map((s) => s.id))
  )
  const [searchQuery, setSearchQuery] = useState("")
  const [searchResults, setSearchResults] = useState<TocItem[]>([])
  const searchInputRef = useRef<ElementRef<typeof SearchField>>(null)

  // Toggle section expansion
  const toggleExpanded = useCallback((sectionId: string) => {
    setExpandedSections((prev) => {
      const next = new Set(prev)
      if (next.has(sectionId)) next.delete(sectionId)
      else next.add(sectionId)
      return next
    })
  }, [])

  // Navigate to section
  const handleNavigate = useCallback(
    (item: TocItem) => {
      setCurrentSection(item.id)
      setSidebarOpen(false)
      onNavigate?.(item)
    },
    [onNavigate]
  )

  // Search logic
  const performSearch = useCallback(
    (query: string) => {
      setSearchQuery(query)
      if (!query.trim()) {
        setSearchResults([])
        setState("viewing")
        return
      }

      setState("searching")

      // Search through flat list of all TOC items
      const results: TocItem[] = []
      const flatten = (items: TocItem[]) => {
        items.forEach((item) => {
          if (item.label.toLowerCase().includes(query.toLowerCase())) {
            results.push(item)
          }
          if (item.children) flatten(item.children)
        })
      }
      flatten(toc)

      setSearchResults(results)
    },
    [toc]
  )

  const clearSearch = useCallback(() => {
    setSearchQuery("")
    setSearchResults([])
    setState("viewing")
    searchInputRef.current?.focus()
  }, [])

  // Flatten TOC for quick lookup
  const flatTocMap = useMemo(() => {
    const map = new Map<string, TocItem>()
    const walk = (items: TocItem[]) => {
      items.forEach((item) => {
        map.set(item.id, item)
        if (item.children) walk(item.children)
      })
    }
    walk(toc)
    return map
  }, [toc])

  const activeSection = flatTocMap.has(currentSection)
    ? currentSection
    : toc[0]?.id ?? ""
  const currentItem = flatTocMap.get(activeSection)

  // Find parent section
  const parentSection = useMemo(() => {
    for (const section of toc) {
      if (section.id === activeSection) return section
      if (section.children?.some((c) => c.id === activeSection)) return section
    }
    return toc[0]
  }, [toc, activeSection])

  // ---- Render sidebar ----
  const renderSidebar = () => (
    <div className="flex h-full flex-col">
      {/* Sidebar header */}
      <div className="flex items-center justify-between border-b px-4 py-3">
        <h3 className="text-sm font-medium">
          {t("portalExtract.serviceCode.manualViewer.toc")}
        </h3>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={() => setSidebarOpen(false)}
          className="size-8"
        >
          <XIcon className="size-4" />
        </Button>
      </div>

      {/* Search inside sidebar */}
      <div className="px-4 py-2">
        <SearchField
          ref={searchInputRef}
          allowClear
          className="rhd-railops-search-narrow"
          value={searchQuery}
          onChange={(e) => performSearch(e.target.value)}
          placeholder={t("portalExtract.serviceCode.manualViewer.searchPlaceholder")}
        />
      </div>

      <Separator />

      {/* TOC content */}
      <div className="flex-1 overflow-y-auto">
        {searchQuery ? (
          // Search results
          <div className="space-y-0.5 p-2">
            {searchResults.length === 0 ? (
              <div className="flex flex-col items-center gap-2 py-8">
                <AlertCircleIcon className="size-8 text-muted-foreground" />
                <p className="text-xs text-muted-foreground">
                  {t("portalExtract.serviceCode.manualViewer.noResults")}
                </p>
              </div>
            ) : (
              searchResults.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => handleNavigate(item)}
                  className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition-colors hover:bg-muted ${
                    activeSection === item.id ? "bg-muted font-medium" : ""
                  }`}
                >
                  <FileIcon className="size-3.5 shrink-0 text-muted-foreground" />
                  <span className="truncate">
                    {highlightText(item.label, searchQuery)}
                  </span>
                  {item.page && (
                    <span className="ml-auto shrink-0 text-rhd-2xs text-muted-foreground">
                      p.{item.page}
                    </span>
                  )}
                </button>
              ))
            )}
          </div>
        ) : (
          // Normal TOC tree
          <div className="space-y-0.5 p-2">
            {toc.map((section) => {
              const isExpanded = expandedSections.has(section.id)
              const isActive = activeSection === section.id || section.children?.some((c) => c.id === activeSection)

              return (
                <div key={section.id}>
                  <button
                    type="button"
                    onClick={() => {
                      if (section.children) {
                        toggleExpanded(section.id)
                      }
                      handleNavigate(section)
                    }}
                    className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition-colors hover:bg-muted ${
                      isActive ? "bg-muted font-medium" : ""
                    }`}
                  >
                    {section.children ? (
                      <ChevronRightIcon
                        className={`size-3.5 shrink-0 text-muted-foreground transition-transform ${
                          isExpanded ? "rotate-90" : ""
                        }`}
                      />
                    ) : (
                      <FileIcon className="size-3.5 shrink-0 text-muted-foreground" />
                    )}
                    <span className="truncate">{section.label}</span>
                    {section.page && (
                      <span className="ml-auto shrink-0 text-rhd-2xs text-muted-foreground">
                        p.{section.page}
                      </span>
                    )}
                  </button>

                  {section.children && isExpanded && (
                    <div className="ml-3 space-y-0.5 border-l pl-2">
                      {section.children.map((child) => (
                        <button
                          key={child.id}
                          type="button"
                          onClick={() => handleNavigate(child)}
                          className={`flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-left text-sm transition-colors hover:bg-muted ${
                            activeSection === child.id ? "bg-muted font-medium text-primary" : ""
                          }`}
                        >
                          <FileTextIcon className="size-3 shrink-0 text-muted-foreground" />
                          <span className="truncate">{child.label}</span>
                          {child.page && (
                            <span className="ml-auto shrink-0 text-rhd-2xs text-muted-foreground">
                              p.{child.page}
                            </span>
                          )}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )

  // ---- Render content area ----
  const renderContent = () => {
    if (toc.length === 0) {
      return (
        <div className="flex flex-col items-center gap-3 px-4 py-12 text-center">
          <BookOpenIcon className="size-10 text-muted-foreground" />
          <p className="text-sm font-medium text-muted-foreground">
            {t("portalExtract.serviceCode.manualViewer.emptyTitle")}
          </p>
        </div>
      )
    }

    if (state === "loading") {
      return (
        <div className="space-y-4 p-4">
          <Skeleton className="h-6 w-48" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-5/6" />
          <Skeleton className="aspect-video w-full rounded-lg" />
          <Skeleton className="h-4 w-4/5" />
          <Skeleton className="h-4 w-3/5" />
          <div className="flex items-center justify-center gap-2 py-4">
            <Loader2Icon className="size-5 animate-spin text-muted-foreground" />
            <p className="text-xs text-muted-foreground">
              {t("portalExtract.serviceCode.manualViewer.loading")}
            </p>
          </div>
        </div>
      )
    }

    if (state === "searching") {
      return (
        <div className="space-y-3 p-4">
          <div className="flex items-center gap-2">
            <SearchIcon className="size-4 text-muted-foreground" />
            <p className="text-sm text-muted-foreground">
              {t("portalExtract.serviceCode.manualViewer.searchResults", {
                query: searchQuery,
              })}
            </p>
            <Badge variant="secondary" className="text-rhd-2xs">
              {searchResults.length}
            </Badge>
          </div>

          {searchResults.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-10">
              <AlertCircleIcon className="size-10 text-muted-foreground" />
              <p className="text-sm text-muted-foreground">
                {t("portalExtract.serviceCode.manualViewer.noSearchResults")}
              </p>
              <Button type="button" variant="outline" size="sm" onClick={clearSearch}>
                {t("portalExtract.serviceCode.manualViewer.clearSearch")}
              </Button>
            </div>
          ) : (
            <div className="space-y-2">
              {searchResults.map((item) => {
                const parent = toc.find(
                  (s) => s.id === item.id || s.children?.some((c) => c.id === item.id)
                )
                return (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => handleNavigate(item)}
                    className="flex w-full items-start gap-2 rounded-lg border p-3 text-left transition-colors hover:bg-muted"
                  >
                    <FileTextIcon className="mt-0.5 size-4 shrink-0 text-primary" />
                    <div className="min-w-0 flex-1">
                      <p className="text-sm font-medium">
                        {highlightText(item.label, searchQuery)}
                      </p>
                      {parent && (
                        <p className="mt-0.5 text-rhd-2xs text-muted-foreground">
                          {parent.label}
                          {item.page && ` · p.${item.page}`}
                        </p>
                      )}
                    </div>
                    <ChevronRightIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                  </button>
                )
              })}
            </div>
          )}
        </div>
      )
    }

    const content = currentItem?.content?.trim() ?? ""
    const activeSourceUrl = currentItem?.url || sourceUrl

    return (
      <div className="space-y-4 p-4">
        {/* Section header */}
        <div>
          <p className="text-xs text-muted-foreground">
            {parentSection?.label}
            {currentItem?.page && ` / p.${currentItem.page}`}
          </p>
          <h2 className="mt-1 text-lg font-medium">
            {currentItem?.label ?? t("portalExtract.serviceCode.manualViewer.selectSection")}
          </h2>
        </div>

        <Separator />

        {content ? (
          <div className="rounded-lg border bg-muted/20 p-4">
            <div className="whitespace-pre-wrap text-sm leading-6">
              {content}
            </div>
          </div>
        ) : activeSourceUrl ? (
          <div className="overflow-hidden rounded-lg border bg-muted/20">
            <iframe
              title={currentItem?.label || title || "Product manual"}
              src={activeSourceUrl}
              className="aspect-[3/4] w-full bg-white"
            />
            <div className="flex items-center justify-between gap-3 border-t bg-background p-3">
              <span className="truncate text-xs text-muted-foreground">{currentItem?.label}</span>
              <Button
                type="button"
                size="sm"
                variant="outline"
                render={<a href={activeSourceUrl} target="_blank" rel="noreferrer" />}
              >
                <ExternalLinkIcon className="size-4" />
                {t("portalExtract.serviceCode.manualViewer.openFile")}
              </Button>
            </div>
          </div>
        ) : (
          <div className="flex aspect-[3/4] flex-col items-center justify-center gap-3 rounded-lg border-2 border-dashed border-muted-foreground/30 bg-muted/30">
            <BookOpenIcon className="size-12 text-muted-foreground/50" />
            <div className="text-center">
              <p className="text-sm font-medium text-muted-foreground">
                {t("portalExtract.serviceCode.manualViewer.noContent")}
              </p>
              <p className="mt-1 text-xs text-muted-foreground/60">
                {currentItem?.label}
              </p>
            </div>
          </div>
        )}

        {/* Page navigation hint */}
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>
            {t("portalExtract.serviceCode.manualViewer.currentSection", {
              section: currentItem?.label ?? "",
            })}
          </span>
          {currentItem?.page && (
            <Badge variant="outline" className="text-rhd-2xs">
              {t("portalExtract.serviceCode.manualViewer.page", {
                page: currentItem.page,
              })}
            </Badge>
          )}
        </div>
      </div>
    )
  }

  // ---- Main render ----

  return (
    <div className="flex h-full flex-col">
      {/* Top bar */}
      <div className="flex items-center gap-2 border-b px-4 py-3">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={() => setSidebarOpen(true)}
          className="size-8 shrink-0"
          aria-label={t("portalExtract.serviceCode.manualViewer.openToc")}
        >
          <MenuIcon className="size-5" />
        </Button>

        <div className="min-w-0 flex-1">
          <h1 className="truncate text-sm font-medium">
            {title ?? t("portalExtract.serviceCode.manualViewer.title")}
          </h1>
        </div>

        {/* Search toggle */}
        <Button
          type="button"
          variant="ghost"
          size="icon"
          onClick={() => searchInputRef.current?.focus()}
          className="size-8 shrink-0"
          aria-label={t("portalExtract.serviceCode.manualViewer.search")}
        >
          <SearchIcon className="size-4" />
        </Button>
      </div>

      {/* Content area */}
      <div className="relative flex-1 overflow-y-auto">
        {/* Sidebar overlay (mobile) */}
        {sidebarOpen && (
          <>
            <div
              className="absolute inset-0 z-40 bg-black/50"
              onClick={() => setSidebarOpen(false)}
            />
            <div className="absolute inset-y-0 left-0 z-50 w-72 max-w-[85vw] border-r bg-background shadow-xl">
              {renderSidebar()}
            </div>
          </>
        )}

        {renderContent()}
      </div>
    </div>
  )
}
