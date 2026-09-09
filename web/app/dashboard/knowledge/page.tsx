"use client"

import { Button } from "@/components/ui/button"
import { DetailDrawer, UnderlineTabs, type RailopsTabItem } from "@railops/ui"
import type { KnowledgeBase } from "@/lib/api/admin"
import { exportKnowledgeFAQs } from "@/lib/api/admin"
import { useI18n } from "@/i18n/provider"
import {
  Bug,
  DownloadIcon,
  LayoutGridIcon,
  LayoutListIcon,
  PanelLeftCloseIcon,
  PanelLeftOpenIcon,
  PlusIcon,
  RefreshCwIcon,
  UploadIcon,
} from "lucide-react"
import { useEffect, useState, type PointerEvent } from "react"
import { toast } from "sonner"
import { DebugPanel } from "./_components/debug-panel"
import { DocumentList, type DocumentListActionState } from "./_components/document-list"
import { FAQList, type FAQListActionState } from "./_components/faq-list"
import { KnowledgeBaseList } from "./_components/knowledge-base-list"
import { RetrieveLogList } from "./_components/retrieve-log-list"

const KNOWLEDGE_BASE_LIST_WIDTH_STORAGE_KEY = "knowledge-base-list-width"
const KNOWLEDGE_BASE_LIST_MIN_WIDTH = 240
const KNOWLEDGE_BASE_LIST_MAX_WIDTH = 520
const KNOWLEDGE_BASE_LIST_DEFAULT_WIDTH = 320

export default function DashboardKnowledgeDocumentsPage() {
  const t = useI18n()
  const [selectedKnowledgeBase, setSelectedKnowledgeBase] = useState<KnowledgeBase | null>(null)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [sidebarResizing, setSidebarResizing] = useState(false)
  const [sidebarWidth, setSidebarWidth] = useState(() => {
    if (typeof window === "undefined") {
      return KNOWLEDGE_BASE_LIST_DEFAULT_WIDTH
    }
    const saved = Number(localStorage.getItem(KNOWLEDGE_BASE_LIST_WIDTH_STORAGE_KEY))
    if (!Number.isFinite(saved)) {
      return KNOWLEDGE_BASE_LIST_DEFAULT_WIDTH
    }
    return Math.min(
      KNOWLEDGE_BASE_LIST_MAX_WIDTH,
      Math.max(KNOWLEDGE_BASE_LIST_MIN_WIDTH, saved),
    )
  })
  const [debugPanelOpen, setDebugPanelOpen] = useState(false)
  const [activeTab, setActiveTab] = useState("documents")

  const [documentActionState, setDocumentActionState] = useState<DocumentListActionState | null>(null)
  const [faqActionState, setFAQActionState] = useState<FAQListActionState | null>(null)
  const [exportingFAQ, setExportingFAQ] = useState(false)
  const isFAQKnowledgeBase = selectedKnowledgeBase?.knowledgeType === "faq"

  const knowledgeTabs: RailopsTabItem[] = [
    { value: "documents", label: isFAQKnowledgeBase ? t("knowledge.faq") : t("knowledge.document") },
    { value: "retrieveLogs", label: t("knowledge.retrieveLogs") },
  ]

  useEffect(() => {
    localStorage.setItem(KNOWLEDGE_BASE_LIST_WIDTH_STORAGE_KEY, String(sidebarWidth))
  }, [sidebarWidth])

  function handleSidebarResizePointerDown(event: PointerEvent<HTMLDivElement>) {
    event.preventDefault()
    const startX = event.clientX
    const startWidth = sidebarWidth

    function handlePointerMove(moveEvent: globalThis.PointerEvent) {
      const nextWidth = startWidth + moveEvent.clientX - startX
      setSidebarWidth(
        Math.min(
          KNOWLEDGE_BASE_LIST_MAX_WIDTH,
          Math.max(KNOWLEDGE_BASE_LIST_MIN_WIDTH, nextWidth),
        ),
      )
    }

    function handlePointerEnd() {
      window.removeEventListener("pointermove", handlePointerMove)
      window.removeEventListener("pointerup", handlePointerEnd)
      window.removeEventListener("pointercancel", handlePointerEnd)
      document.body.style.cursor = ""
      document.body.style.userSelect = ""
      setSidebarResizing(false)
    }

    setSidebarResizing(true)
    document.body.style.cursor = "col-resize"
    document.body.style.userSelect = "none"
    window.addEventListener("pointermove", handlePointerMove)
    window.addEventListener("pointerup", handlePointerEnd)
    window.addEventListener("pointercancel", handlePointerEnd)
  }

  async function handleExportFAQ() {
    if (!selectedKnowledgeBase || exportingFAQ) {
      return
    }
    setExportingFAQ(true)
    try {
      await exportKnowledgeFAQs(selectedKnowledgeBase.id)
      toast.success(t("knowledge.exportFAQSuccess"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("knowledge.exportFAQFailed"))
    } finally {
      setExportingFAQ(false)
    }
  }

  return (
    <div className="flex h-full min-h-0 overflow-hidden">
      <div
        className={`relative shrink-0 overflow-hidden ${
          sidebarResizing ? "" : "transition-[width] duration-200"
        }`}
        style={{ width: sidebarCollapsed ? 0 : sidebarWidth }}
      >
        <KnowledgeBaseList
          selectedKnowledgeBaseId={selectedKnowledgeBase?.id ?? null}
          onSelectKnowledgeBase={setSelectedKnowledgeBase}
        />
        {!sidebarCollapsed ? (
          <div
            className="absolute top-0 right-[-3px] z-20 h-full w-1.5 cursor-col-resize transition-colors hover:bg-primary/30"
            onPointerDown={handleSidebarResizePointerDown}
            role="separator"
            aria-orientation="vertical"
            aria-label={t("knowledge.resizeList")}
          />
        ) : null}
      </div>
      <div className="relative shrink-0 bg-background">
        <Button
          variant="outline"
          size="icon"
          className="absolute top-4 left-1/2 z-10 size-7 -translate-x-1/2 rounded-full shadow-sm"
          onClick={() => setSidebarCollapsed((value) => !value)}
          aria-label={sidebarCollapsed ? t("knowledge.expandList") : t("knowledge.collapseList")}
        >
          {sidebarCollapsed ? (
            <PanelLeftOpenIcon className="size-3.5" />
          ) : (
            <PanelLeftCloseIcon className="size-3.5" />
          )}
        </Button>
      </div>
      <div className="min-w-0 min-h-0 h-full flex-1 overflow-hidden">
        <div className="h-full min-h-0">
          <div className="px-6 py-4">
            <div className="flex items-center gap-2">
              <div className="min-w-0 flex-1">
                <UnderlineTabs
                  ariaLabel={t("knowledge.tabsLabel") || t("miscTailExtract.dashboardMisc.knowledge.tabsAriaFallback")}
                  items={knowledgeTabs}
                  value={activeTab}
                  onChange={setActiveTab}
                />
              </div>
              {activeTab === "documents" && !isFAQKnowledgeBase && documentActionState ? (
                <div className="ml-auto flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    onClick={documentActionState.onRefresh}
                    disabled={documentActionState.loading}
                    aria-label={t("knowledge.refreshDocuments")}
                  >
                    <RefreshCwIcon className={documentActionState.loading ? "size-4 animate-spin" : "size-4"} />
                  </Button>
                  <Button
                    variant={documentActionState.viewMode === "list" ? "secondary" : "ghost"}
                    size="icon"
                    className="size-7"
                    onClick={() => documentActionState.onChangeViewMode("list")}
                    aria-label={t("knowledge.listLayout")}
                  >
                    <LayoutListIcon className="size-4" />
                  </Button>
                  <Button
                    variant={documentActionState.viewMode === "grid" ? "secondary" : "ghost"}
                    size="icon"
                    className="size-7"
                    onClick={() => documentActionState.onChangeViewMode("grid")}
                    aria-label={t("knowledge.gridLayout")}
                  >
                    <LayoutGridIcon className="size-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    onClick={() => setDebugPanelOpen(true)}
                    aria-label={t("knowledge.openDebugPanel")}
                  >
                    <Bug className="size-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    onClick={documentActionState.onCreate}
                    aria-label={t("knowledge.newDocument")}
                  >
                    <PlusIcon className="size-4" />
                  </Button>
                </div>
              ) : null}
              {activeTab === "documents" && isFAQKnowledgeBase && faqActionState ? (
                <div className="ml-auto flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    onClick={faqActionState.onRefresh}
                    disabled={faqActionState.loading}
                    aria-label={t("knowledge.refreshFAQ")}
                  >
                    <RefreshCwIcon className={faqActionState.loading ? "size-4 animate-spin" : "size-4"} />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    onClick={faqActionState.onImport}
                    disabled={faqActionState.importing}
                    aria-label={t("knowledge.importFAQ")}
                  >
                    <UploadIcon className="size-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    onClick={() => void handleExportFAQ()}
                    disabled={exportingFAQ}
                    aria-label={t("knowledge.exportFAQ")}
                  >
                    <DownloadIcon className={exportingFAQ ? "size-4 animate-pulse" : "size-4"} />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    onClick={() => setDebugPanelOpen(true)}
                    aria-label={t("knowledge.openDebugPanel")}
                  >
                    <Bug className="size-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="size-7"
                    onClick={faqActionState.onCreate}
                    aria-label={t("knowledge.newFAQ")}
                  >
                    <PlusIcon className="size-4" />
                  </Button>
                </div>
              ) : null}
            </div>
          </div>
          {activeTab === "documents" ? (
            <div className="min-h-0 flex-1">
            {isFAQKnowledgeBase ? (
              <FAQList
                knowledgeBaseId={selectedKnowledgeBase?.id ?? null}
                onActionStateChange={setFAQActionState}
              />
            ) : (
              <DocumentList 
                knowledgeBaseId={selectedKnowledgeBase?.id ?? null}
                onActionStateChange={setDocumentActionState}
              />
            )}
            </div>
          ) : null}
          {activeTab === "retrieveLogs" ? (
            <div className="min-h-0 flex-1">
            <RetrieveLogList
              knowledgeBaseId={selectedKnowledgeBase?.id ?? null}
            />
            </div>
          ) : null}
        </div>
      </div>
      <DetailDrawer
        open={debugPanelOpen}
        onClose={() => setDebugPanelOpen(false)}
        title={t("knowledge.ragDebug")}
        styles={{ body: { padding: 0 } }}
      >
        <DebugPanel knowledgeBaseId={selectedKnowledgeBase?.id ?? null} />
      </DetailDrawer>
    </div>
  )
}
