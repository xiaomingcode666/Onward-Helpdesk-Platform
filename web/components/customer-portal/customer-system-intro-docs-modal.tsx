"use client"

import { useCallback, useEffect, useState } from "react"
import { useI18n, useAppLocale } from "@/i18n/provider"
import { Button } from "@/components/ui/button"
import { formatFileSize } from "@/lib/im-message"
import { fetchCustomerSystemIntroDocs } from "@/lib/api/customer-system-docs"
import type { CustomerPortalSystemIntroDoc } from "@/lib/api/customer-portal-types"
import { FileTextIcon, ArrowLeftIcon, ExternalLinkIcon, Loader2Icon, RefreshCwIcon } from "lucide-react"
import { StandardModal } from "@railops/ui"

function formatDateTime(value?: string, locale = "zh-CN") {
  if (!value) return ""
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(locale, { year: "numeric", month: "2-digit", day: "2-digit" }).format(date)
}

type PreviewKind = "image" | "video" | "pdf" | "unsupported"

function previewKindOf(mimeType: string): PreviewKind {
  if (mimeType.startsWith("image/")) return "image"
  if (mimeType.startsWith("video/")) return "video"
  if (mimeType === "application/pdf" || mimeType.startsWith("text/")) return "pdf"
  return "unsupported"
}

export function CustomerSystemIntroDocsModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const [loading, setLoading] = useState(false)
  const [loadFailed, setLoadFailed] = useState(false)
  const [docs, setDocs] = useState<CustomerPortalSystemIntroDoc[]>([])
  const [selected, setSelected] = useState<CustomerPortalSystemIntroDoc | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setLoadFailed(false)
    try {
      setDocs(await fetchCustomerSystemIntroDocs())
    } catch {
      setLoadFailed(true)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open) {
      setSelected(null)
      void load()
    }
  }, [open, load])

  const openOriginal = (doc: CustomerPortalSystemIntroDoc) => {
    if (doc.url) window.open(doc.url, "_blank", "noopener,noreferrer")
  }

  return (
    <StandardModal
      open={open}
      onCancel={onClose}
      width={900}
      title={t("customerChat.systemIntroDocsTitle")}
      footer={selected ? (
        <>
          <Button type="button" variant="outline" onClick={() => setSelected(null)}>
            <ArrowLeftIcon className="mr-2 size-4" />
            {t("customerChat.systemIntroBack")}
          </Button>
          <Button type="button" onClick={() => openOriginal(selected)}>
            <ExternalLinkIcon className="mr-2 size-4" />
            {t("customerChat.systemIntroOpenOriginal")}
          </Button>
        </>
      ) : null}
    >
      {selected ? (
        <div className="flex flex-col gap-4">
          <div className="flex items-center gap-3">
            <span className="grid size-10 shrink-0 place-items-center rounded-md bg-primary/10 text-primary">
              <FileTextIcon className="size-5" />
            </span>
            <span className="min-w-0 flex-1">
              <span className="block truncate font-medium text-foreground">{selected.title || selected.filename}</span>
              <span className="mt-0.5 block text-xs text-muted-foreground">
                {selected.filename} · {formatFileSize(selected.file_size)}
              </span>
            </span>
          </div>
          <div className="overflow-hidden rounded-md border border-border bg-muted/30">
            {previewKindOf(selected.mime_type) === "image" ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img src={selected.url} alt={selected.title || selected.filename} className="h-[68vh] w-full object-contain" />
            ) : previewKindOf(selected.mime_type) === "video" ? (
              <video src={selected.url} controls className="h-[68vh] w-full object-contain" />
            ) : previewKindOf(selected.mime_type) === "pdf" ? (
              <iframe src={selected.url} sandbox="" title={selected.title || selected.filename} className="h-[68vh] w-full" />
            ) : (
              <div className="grid h-[68vh] place-items-center px-6 text-center">
                <div>
                  <FileTextIcon className="mx-auto size-8 text-muted-foreground" />
                  <p className="mt-3 text-sm font-medium text-foreground">{t("customerChat.systemIntroPreviewUnsupported")}</p>
                  <p className="mt-1 text-xs text-muted-foreground">{t("customerChat.systemIntroPreviewUnsupportedDesc")}</p>
                </div>
              </div>
            )}
          </div>
        </div>
      ) : loading ? (
        <div className="grid place-items-center py-16">
          <Loader2Icon className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : loadFailed ? (
        <div className="grid place-items-center py-16 text-center">
          <div>
            <p className="text-sm font-medium text-foreground">{t("customerChat.systemIntroDocsLoadFailed")}</p>
            <Button type="button" variant="outline" className="mt-4" onClick={() => void load()}>
              <RefreshCwIcon className="mr-2 size-4" />
              {t("common.retry")}
            </Button>
          </div>
        </div>
      ) : docs.length === 0 ? (
        <div className="grid place-items-center py-16 text-center">
          <div>
            <FileTextIcon className="mx-auto size-8 text-muted-foreground" />
            <p className="mt-3 text-sm font-medium text-foreground">{t("customerChat.systemIntroDocsEmpty")}</p>
          </div>
        </div>
      ) : (
        <div className="max-h-[68vh] overflow-y-auto">
          <ul className="divide-y divide-border">
            {docs.map((doc) => (
              <li key={doc.id}>
                <button
                  type="button"
                  onClick={() => setSelected(doc)}
                  className="flex w-full items-center gap-3 rounded-md px-3 py-4 text-left transition hover:bg-primary/5"
                >
                  <span className="grid size-10 shrink-0 place-items-center rounded-md bg-primary/10 text-primary">
                    <FileTextIcon className="size-5" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-medium text-foreground">{doc.title || doc.filename}</span>
                    <span className="mt-0.5 block truncate text-xs text-muted-foreground">
                      {doc.filename} · {formatFileSize(doc.file_size)}
                    </span>
                  </span>
                  <span className="shrink-0 text-xs text-muted-foreground">{formatDateTime(doc.published_at, locale)}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </StandardModal>
  )
}
