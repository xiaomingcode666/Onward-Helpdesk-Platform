"use client"

import { useCallback, useEffect, useState } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { ArrowLeftIcon, BookOpenIcon, BoxesIcon, ChartNoAxesColumnIcon, HistoryIcon, ShieldAlertIcon } from "lucide-react"
import { Skeleton } from "antd"
import { FilterTabs, RailopsButton, StatusTag, type RailopsTabItem, type StatusTagTone } from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { ErrorState } from "@/components/shared/error-states"
import { useI18n } from "@/i18n/provider"
import { getProductProfile, type ProductServiceProfile } from "@/lib/api/enterprise-products"
import { FaultStatsTab } from "../fault-stats-tab"
import { ManualsTab } from "../manuals-tab"
import { ModelsTab } from "../models-tab"
import { ModulesTab } from "../modules-tab"
import { ProductRepairHistoryTab } from "./product-repair-history-tab"

const tabs = [
  { value: "faultStats", icon: ChartNoAxesColumnIcon, permission: "product.view" },
  { value: "repairHistory", icon: HistoryIcon, permission: "ticket.repairHistory.view" },
  { value: "models", icon: BoxesIcon, permission: "product.view" },
  { value: "modules", icon: ShieldAlertIcon, permission: "product.view" },
  { value: "manuals", icon: BookOpenIcon, permission: "product.view" },
] as const

type ProductArchiveTab = (typeof tabs)[number]["value"]

function parseTab(value: string | null): ProductArchiveTab {
  return tabs.some((tab) => tab.value === value) ? value as ProductArchiveTab : "faultStats"
}

function productArchiveStatusTone(status?: string): StatusTagTone {
  switch (status) {
    case "active":
      return "success"
    case "inactive":
      return "disabled"
    case "discontinued":
      return "warning"
    default:
      return "neutral"
  }
}

export function ProductDetailWorkspace({
  productId,
  routeMode = "dynamic",
}: {
  productId: number
  routeMode?: "dynamic" | "query"
}) {
  const t = useI18n()
  const { session, ready } = useAuth()
  const router = useRouter()
  const searchParams = useSearchParams()
  const requestedTab = parseTab(searchParams.get("tab"))
  const visibleTabs = session
    ? tabs.filter((tab) => session.permissions.includes(tab.permission))
    : []
  const activeTab = visibleTabs.some((tab) => tab.value === requestedTab)
    ? requestedTab
    : visibleTabs[0]?.value ?? "faultStats"
  const [profile, setProfile] = useState<ProductServiceProfile | null>(null)
  const [error, setError] = useState("")
  const [version, setVersion] = useState(0)

  const loadProfile = useCallback(async () => {
    if (!ready || !session) return
    setError("")
    try {
      const response = await getProductProfile(productId)
      if (!response.success || !response.data) throw new Error(response.error?.message || t("productArchive.loadFailed"))
      setProfile(response.data)
    } catch (value) {
      setError(value instanceof Error ? value.message : t("productArchive.loadFailed"))
    }
  }, [productId, ready, session, t])

  useEffect(() => {
    void loadProfile()
  }, [loadProfile, version])

  const setTab = (tab: ProductArchiveTab) => {
    const params = new URLSearchParams(searchParams.toString())
    params.set("tab", tab)
    if (routeMode === "query") {
      params.set("product_id", String(productId))
      router.replace(`/enterprise/products?${params.toString()}`, { scroll: false })
      return
    }
    router.replace(`/enterprise/products/${productId}?${params.toString()}`, { scroll: false })
  }

  const product = profile?.product ?? null
  const profileError = Boolean(error && !profile)
  const profilePending = !profileError && (!ready || !session || !profile)
  const showTabPlaceholders = !ready || !session
  const statusLabel = product
    ? product.status === "active"
      ? t("productArchive.status.active")
      : product.status === "inactive"
        ? t("productArchive.status.inactive")
        : product.status === "discontinued"
          ? t("productArchive.status.discontinued")
          : product.status || t("productArchive.notConfigured")
    : ""
  const tabItems: RailopsTabItem[] = visibleTabs.map(({ value, icon: Icon }) => ({
    value,
    label: t(`productCenter.tabs.${value}`),
    icon: <Icon className="size-3.5" />,
  }))

  return (
    <div className="min-h-[calc(100vh-97px)] bg-muted/20">
      <header className="rhd-railops-product-archive-header">
        <div className="rhd-railops-product-archive-header-inner">
          <Link href="/enterprise/products">
            <RailopsButton className="rhd-railops-product-archive-back" size="small">
              <ArrowLeftIcon className="size-3.5" />
              {t("productArchive.back")}
            </RailopsButton>
          </Link>
          <div className="rhd-railops-product-archive-identity">
            {product ? (
              <>
                <div className="rhd-railops-product-archive-title-row">
                  <h1 title={product.name}>{product.name}</h1>
                  <StatusTag tone={productArchiveStatusTone(product.status)}>{statusLabel}</StatusTag>
                </div>
                <div className="rhd-railops-product-archive-meta-row">
                  <span>{product.code || `PROD-${product.id}`}</span>
                  <span>{product.product_line || product.category || "-"}</span>
                  <span>{t("productArchive.fields.owner")} {product.owner_name || t("productArchive.notConfigured")}</span>
                  <span>{t("productArchive.updated")} {product.updated_at || "-"}</span>
                </div>
                {product.description ? (
                  <p className="rhd-railops-product-archive-description" title={product.description}>
                    {product.description}
                  </p>
                ) : null}
              </>
            ) : (
              <div className="rhd-railops-product-archive-loading-title" role="status" aria-busy="true" aria-label={t("common.loadingData")}>
                <div className="flex flex-wrap items-center gap-2">
                  <Skeleton.Node active className="w-full" style={{ width: "14rem", maxWidth: "68vw", height: 28 }} />
                  <Skeleton.Node active className="w-full" style={{ width: "4rem", height: 24 }} />
                </div>
                <Skeleton.Node active className="w-full" style={{ width: "32rem", maxWidth: "100%", height: 16 }} />
              </div>
            )}
          </div>
        </div>
      </header>

      <nav className="rhd-railops-product-archive-tabs" aria-label={t("productArchive.tabsLabel")}>
        <div className="mx-auto max-w-[1600px] px-4 lg:px-6">
          <div className="rhd-railops-product-archive-tabs-inner">
            {showTabPlaceholders ? (
              Array.from({ length: 5 }).map((_, index) => (
                <div key={index} className="flex h-11 items-center px-3" role="status" aria-busy="true" aria-label={t("common.loadingData")}>
                  <Skeleton.Node active className="w-full" style={{ width: "5rem", height: 16 }} />
                </div>
              ))
            ) : (
              <FilterTabs
                ariaLabel={t("productArchive.tabsLabel")}
                items={tabItems}
                value={activeTab}
                onChange={(value) => setTab(value as ProductArchiveTab)}
              />
            )}
          </div>
        </div>
      </nav>

      <main className="rhd-railops-product-archive-main">
        {profileError ? (
          <div className="rhd-railops-product-tab-error">
            <ErrorState
              title={t("productArchive.loadFailed")}
              description={error}
              action={{ label: t("productArchive.retry"), onClick: () => setVersion((value) => value + 1) }}
            />
          </div>
        ) : null}
        {profilePending ? <ProductDetailWorkspaceLoading /> : null}
        {profile && activeTab === "faultStats" ? <FaultStatsTab productId={productId} /> : null}
        {profile && activeTab === "repairHistory" ? <ProductRepairHistoryTab productId={productId} /> : null}
        {profile && activeTab === "models" ? <ModelsTab productId={productId} /> : null}
        {profile && activeTab === "modules" ? <ModulesTab productId={productId} /> : null}
        {profile && activeTab === "manuals" ? <ManualsTab productId={productId} /> : null}
      </main>
    </div>
  )
}

function ProductDetailWorkspaceLoading() {
  const t = useI18n()
  return (
    <div className="grid gap-4" role="status" aria-busy="true" aria-label={t("enterpriseExtract.products.archiveLoadingAria")}>
      <div className="rhd-railops-product-fault-metrics">
        {Array.from({ length: 4 }).map((_, index) => (
          <div key={index} className="rhd-railops-product-loading-cell">
            <Skeleton.Node active className="w-full" style={{ width: "5rem", height: 12 }} />
            <Skeleton.Node active className="mt-3 w-full" style={{ width: "6rem", height: 28 }} />
            <Skeleton.Node active className="mt-2 w-full" style={{ width: "8rem", maxWidth: "100%", height: 12 }} />
          </div>
        ))}
      </div>
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="rhd-railops-product-loading-panel">
          <Skeleton.Node active className="w-full" style={{ width: "9rem", height: 20 }} />
          <Skeleton.Node active className="mt-4 w-full" style={{ width: "100%", height: 128 }} />
          <Skeleton.Node active className="mt-3 w-full" style={{ width: "100%", height: 80 }} />
        </div>
        <div className="rhd-railops-product-loading-panel">
          <Skeleton.Node active className="w-full" style={{ width: "7rem", height: 20 }} />
          <Skeleton.Node active className="mt-4 w-full" style={{ width: "100%", height: 40 }} />
          <Skeleton.Node active className="mt-3 w-full" style={{ width: "100%", height: 40 }} />
        </div>
      </div>
    </div>
  )
}
