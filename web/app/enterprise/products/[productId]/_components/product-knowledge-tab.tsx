"use client"

import { useCallback, useEffect, useState } from "react"
import Link from "next/link"

import { Skeleton } from "antd"
import { RailopsButton, StatusTag } from "@railops/ui"

import { EmptyState, ErrorState } from "@/components/shared/error-states"
import { useI18n } from "@/i18n/provider"
import {
  getProductKnowledgeCoverage,
  getProductKnowledgeDocuments,
  getProductKnowledgeLinks,
  type ProductKnowledgeCoverageDetail,
  type ProductKnowledgeDocumentFile,
  type ProductKnowledgeLink,
  type ProductManualLink,
} from "@/lib/api/enterprise-products"

type ProductKnowledgeBindingRow = {
  id: number
  title: string
  linkType: string
  language: string
  version: string
  publishStatus: string
  reviewStatus: string
  indexStatus: string
  updatedAt: string
}

type ProductKnowledgeTabData = {
  coverage: ProductKnowledgeCoverageDetail
  links: ProductKnowledgeBindingRow[]
  documents: ProductKnowledgeDocumentFile[]
  documentsAccessible: boolean
}

export function ProductKnowledgeTab({ productId }: { productId: number }) {
  const t = useI18n()
  const [data, setData] = useState<ProductKnowledgeTabData | null>(null)
  const [error, setError] = useState("")
  const [version, setVersion] = useState(0)
  const load = useCallback(async () => {
    setError("")
    try {
      const [coverageResponse, linksResponse, documentsResponse] = await Promise.all([
        getProductKnowledgeCoverage(productId),
        getProductKnowledgeLinks(productId, { limit: 10 }),
        getProductKnowledgeDocuments(productId, { limit: 10 }),
      ])
      if (!coverageResponse.success || !coverageResponse.data) {
        throw new Error(coverageResponse.error?.message || t("productArchive.loadFailed"))
      }
      const links = linksResponse.success && linksResponse.data
        ? linksResponse.data.map(mapDetailedKnowledgeLink)
        : coverageResponse.data.links.map(mapCoverageKnowledgeLink)
      setData({
        coverage: coverageResponse.data,
        links,
        documents: documentsResponse.success && documentsResponse.data ? documentsResponse.data.items : [],
        documentsAccessible: Boolean(documentsResponse.success && documentsResponse.data),
      })
    } catch (value) {
      setError(value instanceof Error ? value.message : t("productArchive.loadFailed"))
    }
  }, [productId, t])

  useEffect(() => {
    void load()
  }, [load, version])
  const blockingError = Boolean(error && !data)
  if (data && !data.links.length && !data.documents.length && !data.coverage.linked_entries) return <EmptyState title={t("productArchive.empty.knowledge")} action={{ label: t("productArchive.openKnowledge"), href: `/enterprise/knowledge?product_id=${productId}` }} />
  return (
    <div className="space-y-4">
      {error ? (
        <div className="rounded-md border border-destructive/20 bg-destructive/5 px-4 py-2 text-sm text-destructive">
          {error}
        </div>
      ) : null}
      {blockingError ? (
        <div className="rounded-md border border-border bg-card p-4">
          <ErrorState
            title={t("productArchive.loadFailed")}
            description={error}
            action={{ label: t("productArchive.retry"), onClick: () => setVersion((value) => value + 1) }}
          />
        </div>
      ) : null}
      {data ? (
        <div className="grid gap-3 sm:grid-cols-3">
          <Metric label={t("productArchive.metrics.coverage")} value={`${data.coverage.coverage_score}%`} />
          <Metric label={t("productArchive.metrics.linkedKnowledge")} value={String(data.coverage.linked_entries)} />
          <Metric label={t("productArchive.metrics.pendingCandidates")} value={String(data.coverage.pending_candidates)} />
        </div>
      ) : !blockingError ? (
        <KnowledgeMetricLoading />
      ) : null}
      {data?.coverage.gaps.length ? <div className="rounded-md border border-border bg-muted px-4 py-3 text-sm text-muted-foreground">{data.coverage.gaps.join(" · ")}</div> : null}
      {data ? (
        <KnowledgeSection title={`${t("productArchive.knowledgeBindings")} · ${data.links.length}/${data.coverage.linked_entries}`} empty={t("productArchive.empty.knowledgeBindings")}>
          {data.links.map((link) => (
            <div key={link.id} className="grid gap-2 px-4 py-3 text-sm md:grid-cols-[minmax(0,1.4fr)_0.8fr_0.8fr_0.8fr_auto] md:items-center">
              <div className="min-w-0">
                <div className="truncate font-medium text-foreground">{link.title}</div>
                <div className="mt-0.5 text-xs text-muted-foreground">{link.linkType} · {link.language || "-"} · {link.version || "-"}</div>
              </div>
              <KnowledgeStatus value={link.publishStatus} />
              <KnowledgeStatus value={link.reviewStatus} />
              <KnowledgeStatus value={link.indexStatus} />
              <span className="text-xs text-muted-foreground">{link.updatedAt || "-"}</span>
            </div>
          ))}
        </KnowledgeSection>
      ) : !blockingError ? (
        <KnowledgeSectionLoading title={t("productArchive.knowledgeBindings")} columns={5} />
      ) : null}
      {data ? (
        <KnowledgeSection title={t("productArchive.knowledgeDocuments")} empty={t("productArchive.empty.knowledgeDocuments")}>
          {data.documentsAccessible ? data.documents.map((document) => (
            <div key={document.id} className="grid gap-2 px-4 py-3 text-sm md:grid-cols-[minmax(0,1.5fr)_0.8fr_0.8fr_auto] md:items-center">
              <div className="min-w-0">
                <div className="truncate font-medium text-foreground">{document.title || document.filename}</div>
                <div className="mt-0.5 truncate text-xs text-muted-foreground">{document.filename || document.source_type}</div>
                {document.index_error ? <div className="mt-1 text-xs text-destructive">{document.index_error}</div> : null}
              </div>
              <KnowledgeStatus value={document.review_status} />
              <KnowledgeStatus value={document.index_status} />
              <span className="text-xs text-muted-foreground">{document.updated_at || "-"}</span>
            </div>
          )) : <p className="px-4 py-6 text-center text-sm text-muted-foreground">{t("productArchive.knowledgeDocumentsRestricted")}</p>}
        </KnowledgeSection>
      ) : !blockingError ? (
        <KnowledgeSectionLoading title={t("productArchive.knowledgeDocuments")} columns={4} />
      ) : null}
      <div className="flex justify-end">
        {data ? (
          <Link href={`/enterprise/knowledge?product_id=${productId}`}>
            <RailopsButton>{t("productArchive.openKnowledge")}</RailopsButton>
          </Link>
        ) : !blockingError ? (
          <Skeleton.Node active className="w-full" style={{ width: "7rem", height: 36 }} />
        ) : null}
      </div>
    </div>
  )
}

function mapDetailedKnowledgeLink(link: ProductManualLink): ProductKnowledgeBindingRow {
  return {
    id: link.id,
    title: link.title,
    linkType: link.link_type,
    language: link.language,
    version: link.version,
    publishStatus: link.publish_status,
    reviewStatus: link.entry_review_status,
    indexStatus: link.index_status,
    updatedAt: link.updated_at,
  }
}

function mapCoverageKnowledgeLink(link: ProductKnowledgeLink): ProductKnowledgeBindingRow {
  return {
    id: link.id,
    title: link.title,
    linkType: link.type,
    language: link.language,
    version: link.version,
    publishStatus: "",
    reviewStatus: "",
    indexStatus: "",
    updatedAt: link.updated_at,
  }
}

function KnowledgeSection({ title, empty, children }: { title: string; empty: string; children: React.ReactNode }) {
  const rows = Array.isArray(children) ? children : [children]
  const visibleRows = rows.filter(Boolean)
  return (
    <section className="overflow-hidden rounded-md border border-border bg-card">
      <h3 className="border-b border-border bg-muted px-4 py-3 text-sm font-medium text-foreground">{title}</h3>
      {visibleRows.length ? <div className="divide-y divide-border">{children}</div> : <p className="px-4 py-6 text-center text-sm text-muted-foreground">{empty}</p>}
    </section>
  )
}

function KnowledgeStatus({ value }: { value?: string }) {
  const normalized = value?.trim() || "-"
  const successful = ["published", "indexed", "approved", "succeeded", "active"].includes(normalized)
  const failed = ["failed", "rejected", "deprecated", "error"].includes(normalized)
  return <StatusTag tone="neutral" className={successful ? "border-primary/20 bg-primary/10 text-primary" : failed ? "border-destructive/20 bg-destructive/10 text-destructive" : "border-border bg-muted text-muted-foreground"}>{normalized}</StatusTag>
}

function Metric({ label, value }: { label: string; value: string }) {
  return <div className="rounded-md border border-border bg-card p-4"><span className="text-xs text-muted-foreground">{label}</span><strong className="mt-2 block text-2xl font-semibold text-foreground">{value}</strong></div>
}

function KnowledgeMetricLoading() {
  const t = useI18n()
  return (
    <div className="grid gap-3 sm:grid-cols-3" role="status" aria-busy="true" aria-label={t("enterpriseExtract.products.knowledgeStatsLoading")}>
      {Array.from({ length: 3 }).map((_, index) => (
        <div key={index} className="rounded-md border border-border bg-card p-4">
          <Skeleton.Node active className="w-full" style={{ width: "5rem", height: 12 }} />
          <Skeleton.Node active className="mt-3 w-full" style={{ width: "4rem", height: 28 }} />
        </div>
      ))}
    </div>
  )
}

function KnowledgeSectionLoading({ title, columns }: { title: string; columns: number }) {
  const t = useI18n()
  return (
    <section className="overflow-hidden rounded-md border border-border bg-card" role="status" aria-busy="true" aria-label={t("enterpriseExtract.products.sectionLoadingAria", { title })}>
      <h3 className="border-b border-border bg-muted px-4 py-3 text-sm font-medium text-foreground">{title}</h3>
      <div className="divide-y divide-border">
        {Array.from({ length: 4 }).map((_, rowIndex) => (
          <div key={rowIndex} className="flex flex-wrap items-center gap-3 px-4 py-3">
            {Array.from({ length: columns }).map((__, columnIndex) => (
              <Skeleton.Node
                key={columnIndex}
                active
                className={columnIndex === 0 ? "flex-1 w-full" : "w-full"}
                style={columnIndex === 0 ? { minWidth: "10rem", height: 16 } : { width: "5rem", height: 24 }}
              />
            ))}
          </div>
        ))}
      </div>
    </section>
  )
}
