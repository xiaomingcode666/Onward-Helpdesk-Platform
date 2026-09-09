"use client"

import { BookOpenTextIcon, ChevronDownIcon } from "lucide-react"

import { useI18n } from "@/i18n/provider"
import { parseKnowledgeMessageCitations, type KnowledgeMessageCitation } from "@/lib/im-message"
import { cn } from "@/lib/utils"

function citationTitle(citation: KnowledgeMessageCitation, fallback: string) {
  return citation.documentTitle || citation.faqQuestion || citation.title || citation.sectionPath || fallback
}

export function KnowledgeMessageCitations({
  payload,
  className,
}: {
  payload?: string
  className?: string
}) {
  const t = useI18n()
  const citations = parseKnowledgeMessageCitations(payload)
  if (citations.length === 0) return null

  return (
    <details className={cn("group mt-2 rounded-md border border-border/80 bg-muted/45 text-left", className)}>
      <summary className="flex min-h-8 cursor-pointer list-none items-center gap-2 px-2.5 py-1.5 text-rhd-xs font-medium text-muted-foreground [&::-webkit-details-marker]:hidden">
        <BookOpenTextIcon className="size-3.5 shrink-0" aria-hidden="true" />
        <span className="flex-1">{t("customerEntryExtract.timeline.sources")}</span>
        <span className="text-rhd-2xs tabular-nums">{citations.length}</span>
        <ChevronDownIcon className="size-3.5 shrink-0 transition-transform group-open:rotate-180" aria-hidden="true" />
      </summary>
      <div className="space-y-2 border-t border-border/70 px-2.5 py-2">
        {citations.map((citation, index) => (
          <div key={`${citation.documentId || citation.faqId || 0}-${citation.chunkNo || 0}-${index}`} className="min-w-0">
            <div className="truncate text-rhd-xs font-medium text-foreground">
              {citationTitle(citation, t("customerEntryExtract.timeline.sourceFallback"))}
            </div>
            {citation.sectionPath && citation.sectionPath !== citationTitle(citation, "") ? (
              <div className="mt-0.5 truncate text-rhd-2xs text-muted-foreground">{citation.sectionPath}</div>
            ) : null}
            {citation.snippet ? (
              <p className="mt-0.5 line-clamp-2 text-rhd-xs leading-5 text-muted-foreground">{citation.snippet}</p>
            ) : null}
          </div>
        ))}
      </div>
    </details>
  )
}
