"use client"

import { ChevronDownIcon, ChevronRightIcon } from "lucide-react"
import { SearchField } from "@railops/ui"
import { useMemo, useState, type ReactNode } from "react"

import { Skeleton } from "@/components/ui/skeleton"
import { useI18n } from "@/i18n/provider"
import { translateCurrentMessage } from "@/i18n/messages"
import type { ProductListItem } from "@/lib/api/types"
import { cn } from "@/lib/utils"

export type ProductTreeGroup = {
  key: string
  label: string
  products: ProductListItem[]
}

export type ProductTreePinnedItem = {
  key: string
  label: string
  meta?: ReactNode
  badge?: ReactNode
  selected?: boolean
  onSelect?: () => void
}

export type ProductTreeFilterOption =
  | string
  | {
      label: string
      value: string
    }

type ProductTreeProps = {
  title: string
  totalCount: number
  countLabel?: ReactNode
  groups: ProductTreeGroup[]
  mode?: "grouped" | "directory"
  rootLabel?: string
  rootMeta?: ReactNode
  loading?: boolean
  selectedGroupKey?: string | null
  selectedProductId?: number | null
  rootSelected?: boolean
  pinnedItems?: ProductTreePinnedItem[]
  search?: string
  searchPlaceholder?: string
  searchAriaLabel?: string
  onSearchChange?: (value: string) => void
  filters?: readonly ProductTreeFilterOption[]
  activeFilter?: string
  onFilterChange?: (value: string) => void
  onSelectRoot?: () => void
  onSelectGroup?: (group: ProductTreeGroup) => void
  onSelectProduct?: (product: ProductListItem, group: ProductTreeGroup) => void
  headerAction?: ReactNode
  emptyText?: string
  renderGroupMeta?: (group: ProductTreeGroup) => ReactNode
  renderProductMeta?: (product: ProductListItem) => ReactNode
  renderProductBadge?: (product: ProductListItem) => ReactNode
}

const productTreeI18nPrefix = "opsComponentsExtract.productTree."
type ProductTreeT = ReturnType<typeof useI18n>
const ptr = (t: ProductTreeT, key: string, values?: Record<string, string | number>) =>
  t(`${productTreeI18nPrefix}${key}`, values)

export function getProductDirectoryLabel(product: ProductListItem) {
  const productLine = product.product_line?.trim()
  const category = product.category?.trim()
  return productLine || category || translateCurrentMessage("opsComponentsExtract.productTree.unassignedProduct")
}

type ProductDisplayAlias = ProductListItem & {
  product_name?: string
  productName?: string
  title?: string
  product_code?: string
  productCode?: string
}

export function getProductTreeDisplayName(product: ProductListItem) {
  const item = product as ProductDisplayAlias
  const candidates = [
    item.name,
    item.product_name,
    item.productName,
    item.title,
    item.code,
    item.product_code,
    item.productCode,
  ]
  const label = candidates
    .map((value) => value?.trim())
    .find((value): value is string => Boolean(value))
  return label || translateCurrentMessage("opsComponentsExtract.productTree.productIdFallback", { id: product.id })
}

export function buildProductTreeGroups(products: ProductListItem[]) {
  const grouped = new Map<string, ProductTreeGroup>()
  products.forEach((product) => {
    const key = getProductDirectoryLabel(product)
    const current = grouped.get(key)
    if (current) {
      current.products.push(product)
      return
    }
    grouped.set(key, {
      key,
      label: key,
      products: [product],
    })
  })
  return Array.from(grouped.values())
}

export function ProductTree(props: ProductTreeProps) {
  const t = useI18n()
  const {
    title,
    totalCount,
    countLabel,
    groups,
    mode = "grouped",
    rootLabel = ptr(t, "rootLabel"),
    rootMeta,
    loading = false,
    selectedGroupKey,
    selectedProductId,
    rootSelected,
    pinnedItems = [],
    search,
    searchPlaceholder = ptr(t, "searchPlaceholder"),
    searchAriaLabel = ptr(t, "searchAria"),
    onSearchChange,
    filters,
    activeFilter,
    onFilterChange,
    onSelectRoot,
    onSelectGroup,
    onSelectProduct,
    headerAction,
    emptyText = ptr(t, "empty"),
    renderGroupMeta,
    renderProductMeta,
    renderProductBadge,
  } = props
  const directoryRootKey = "__product_directory_root__"
  const [expandedKeys, setExpandedKeys] = useState<Record<string, boolean>>({})
  const flatProducts = useMemo(
    () => groups.flatMap((group) => group.products.map((product) => ({ product, group }))),
    [groups]
  )

  const treeLabel = useMemo(() => ptr(t, "treeLabel", { title }), [t, title])

  const toggleGroup = (groupKey: string) => {
    setExpandedKeys((current) => ({
      ...current,
      [groupKey]: !(current[groupKey] ?? true),
    }))
  }

  return (
    <aside className="ent-product-finder">
      <div className="ent-panel-head">
        <div>
          <strong>{title}</strong>
          <span>{countLabel ?? ptr(t, "count", { count: totalCount })}</span>
        </div>
        {headerAction ? <div className="ent-tree-head-actions">{headerAction}</div> : null}
      </div>

      {typeof search === "string" && onSearchChange ? (
        <SearchField
          allowClear
          aria-label={searchAriaLabel}
          disabled={loading}
          placeholder={searchPlaceholder}
          value={search}
          onChange={(event) => onSearchChange(event.target.value)}
          className="w-full"
        />
      ) : null}

      {filters && filters.length > 0 && activeFilter && onFilterChange ? (
        <div className="ent-product-filter-row">
          {filters.map((item) => (
            <button
              key={typeof item === "string" ? item : item.value}
              type="button"
              className={activeFilter === (typeof item === "string" ? item : item.value) ? "active" : ""}
              disabled={loading}
              onClick={() => onFilterChange(typeof item === "string" ? item : item.value)}
            >
              {typeof item === "string" ? item : item.label}
            </button>
          ))}
        </div>
      ) : null}

      <div className={cn("ent-product-tree", mode === "directory" && "ent-product-tree-directory")} role="tree" aria-label={treeLabel}>
        {loading ? (
          Array.from({ length: 5 }).map((_, index) => (
            <div key={index} className="ent-product-tree-skeleton">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-8 w-5/6" />
            </div>
          ))
        ) : groups.length === 0 && pinnedItems.length === 0 ? (
          <div className="kb-empty-line">{emptyText}</div>
        ) : mode === "directory" ? (
          <>
            {pinnedItems.map((item) => (
              <div key={item.key} className="ent-product-tree-group-row ent-product-tree-pinned-row">
                <span className="ent-product-tree-toggle ent-product-tree-toggle-spacer" aria-hidden="true" />
                <button
                  type="button"
                  className={cn("ent-product-tree-group-button", item.selected && "active")}
                  role="treeitem"
                  aria-selected={item.selected}
                  onClick={item.onSelect}
                >
                  <span className="ent-product-tree-group-label">{item.label}</span>
                  {item.meta || item.badge ? (
                    <span className="ent-product-tree-pinned-meta">
                      {item.meta}
                      {item.badge}
                    </span>
                  ) : null}
                </button>
              </div>
            ))}
            <section className="ent-product-tree-group ent-product-tree-root-group">
              <div className="ent-product-tree-group-row ent-product-tree-root-row">
                <button
                  type="button"
                  className={cn(
                    "ent-product-tree-toggle",
                    expandedKeys[directoryRootKey] ?? true ? "is-expanded" : "is-collapsed"
                  )}
                  aria-label={(expandedKeys[directoryRootKey] ?? true) ? ptr(t, "collapse", { label: rootLabel }) : ptr(t, "expand", { label: rootLabel })}
                  onClick={() => toggleGroup(directoryRootKey)}
                >
                  {(expandedKeys[directoryRootKey] ?? true)
                    ? <ChevronDownIcon className="size-4" />
                    : <ChevronRightIcon className="size-4" />}
                </button>
                <button
                  type="button"
                  className={cn(
                    "ent-product-tree-group-button",
                    (rootSelected ?? (Boolean(onSelectRoot) && !selectedProductId)) && "active"
                  )}
                  aria-expanded={expandedKeys[directoryRootKey] ?? true}
                  onClick={() => onSelectRoot ? onSelectRoot() : toggleGroup(directoryRootKey)}
                >
                  <span className="ent-product-tree-group-label">{rootLabel}</span>
                  <span className="ent-product-tree-group-meta">{rootMeta ?? ptr(t, "productCount", { count: flatProducts.length })}</span>
                </button>
              </div>

              {(expandedKeys[directoryRootKey] ?? true) ? (
                <div className="ent-product-tree-children ent-product-tree-product-children" role="group" aria-label={rootLabel}>
                  {flatProducts.map(({ product, group }) => {
                    const productLabel = getProductTreeDisplayName(product)
                    return (
                      <button
                        key={product.id}
                        type="button"
                        className={cn(
                          "ent-product-list-item ent-product-tree-item",
                          product.id === selectedProductId && "active"
                        )}
                        role="treeitem"
                        aria-selected={product.id === selectedProductId}
                        onClick={() => onSelectProduct?.(product, group)}
                      >
                        <span className="ent-product-list-name" title={productLabel}>{productLabel}</span>
                        {renderProductMeta ? renderProductMeta(product) : null}
                        {renderProductBadge ? renderProductBadge(product) : null}
                      </button>
                    )
                  })}
                </div>
              ) : null}
            </section>
          </>
        ) : (
          groups.map((group) => {
            const expanded = expandedKeys[group.key] ?? true
            const isGroupSelected = selectedGroupKey === group.key && !selectedProductId
            return (
              <section key={group.key} className="ent-product-tree-group">
                <div className="ent-product-tree-group-row">
                  <button
                    type="button"
                    className={cn(
                      "ent-product-tree-toggle",
                      expanded ? "is-expanded" : "is-collapsed"
                    )}
                    aria-label={expanded ? ptr(t, "collapse", { label: group.label }) : ptr(t, "expand", { label: group.label })}
                    onClick={() => toggleGroup(group.key)}
                  >
                    {expanded ? <ChevronDownIcon className="size-4" /> : <ChevronRightIcon className="size-4" />}
                  </button>
                  <button
                    type="button"
                    className={cn("ent-product-tree-group-button", isGroupSelected && "active")}
                    onClick={() => onSelectGroup?.(group)}
                  >
                    <span className="ent-product-tree-group-label">{group.label}</span>
                    <span className="ent-product-tree-group-meta">
                      {renderGroupMeta ? renderGroupMeta(group) : ptr(t, "productCount", { count: group.products.length })}
                    </span>
                  </button>
                </div>

                {expanded ? (
                  <div className="ent-product-tree-children" role="group" aria-label={group.label}>
                    {group.products.map((product) => {
                      const productLabel = getProductTreeDisplayName(product)
                      return (
                        <button
                          key={product.id}
                          type="button"
                          className={cn(
                            "ent-product-list-item ent-product-tree-item",
                            product.id === selectedProductId && "active"
                          )}
                          role="treeitem"
                          aria-selected={product.id === selectedProductId}
                          onClick={() => onSelectProduct?.(product, group)}
                        >
                          <span className="ent-product-list-name" title={productLabel}>{productLabel}</span>
                          {renderProductMeta ? renderProductMeta(product) : null}
                          {renderProductBadge ? renderProductBadge(product) : null}
                        </button>
                      )
                    })}
                  </div>
                ) : null}
              </section>
            )
          })
        )}
      </div>
    </aside>
  )
}
