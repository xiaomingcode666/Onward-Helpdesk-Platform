"use client";

import { CheckCheckIcon, EyeIcon, MessageCircleMoreIcon } from "lucide-react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
} from "react";

import { ImMessageHTML } from "@/components/im-message-html";
import { useImageLightbox } from "@/components/image-lightbox";
import { ProjectDialog } from "@/components/project-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import {
  type AdminConversation,
  type AdminConversationDetail,
  type AdminMessage,
} from "@/lib/api/admin";
import { renderIMMessageHTML } from "@/lib/im-message";
import { formatDateTime } from "@/lib/utils";
import { useI18n } from "@/i18n/provider";

type ConversationDetailDialogProps = {
  open: boolean;
  loading: boolean;
  saving: boolean;
  readOnly?: boolean;
  item: AdminConversation | null;
  detail: AdminConversationDetail | null;
  messages?: AdminMessage[] | null;
  /** Whether older messages are available through cursor pagination. */
  messagesHasMore?: boolean;
  loadingMoreMessages?: boolean;
  canAssignConversation?: boolean;
  canDispatchConversation?: boolean;
  canTransferConversation?: boolean;
  canCloseConversation?: boolean;
  onLoadMoreMessages?: () => void | Promise<void>;
  onOpenChange: (open: boolean) => void;
  onOpenAssign: () => void;
  onDispatch: () => Promise<void>;
  onOpenTransfer: () => void;
  onRead: () => Promise<void>;
  onOpenClose: () => void;
};

function getStatusMeta(
  status: number,
  t: (key: string, values?: Record<string, string | number>) => string
) {
  switch (status) {
    case 1:
      return { label: t("conversation.filterAiServing"), variant: "secondary" as const };
    case 2:
      return { label: t("conversation.filterPending"), variant: "outline" as const };
    case 3:
      return { label: t("conversation.filterActive"), variant: "secondary" as const };
    case 4:
      return { label: t("conversation.filterClosed"), variant: "outline" as const };
    default:
      return { label: t("conversationMonitor.unknown"), variant: "outline" as const };
  }
}

function getServiceModeLabel(
  mode: number,
  t: (key: string, values?: Record<string, string | number>) => string
) {
  switch (mode) {
    case 1:
      return t("conversationMonitor.serviceAi");
    case 2:
      return t("conversationMonitor.serviceHuman");
    case 3:
      return t("conversationMonitor.serviceAiFirst");
    default:
      return t("conversationMonitor.serviceUndefined");
  }
}

function getSenderLabel(
  message: AdminMessage,
  t: (key: string, values?: Record<string, string | number>) => string
) {
  switch (message.senderType) {
    case "agent":
      return message.senderName || t("conversationMonitor.senderAgent");
    case "customer":
      return message.senderName || t("conversationMonitor.senderCustomer");
    case "ai":
      return t("miscTailExtract.dashboardMisc.conversationMonitor.senderAi");
    case "system":
      return t("conversationMonitor.senderSystem");
    default:
      return message.senderType;
  }
}

function getParticipantIdentity(
  participant: NonNullable<AdminConversationDetail["participants"]>[number],
) {
  return participant.participantId || participant.externalParticipantId || "-";
}

function getMessageLayout(message: AdminMessage) {
  if (message.senderType === "customer") {
    return {
      rowClassName: "justify-start",
      bubbleClassName: "border-border/70 bg-muted/60 text-foreground shadow-sm",
      htmlClassName: "[&_a]:text-foreground [&_a]:underline [&_img]:rounded-md",
      recalledBubbleClassName:
        "border-dashed border-border/70 bg-muted/40 text-muted-foreground",
      recalledHtmlClassName: "text-muted-foreground [&_p]:text-muted-foreground",
      metaClassName: "text-left",
    };
  }
  if (message.senderType === "system") {
    return {
      rowClassName: "justify-center",
      bubbleClassName:
        "border-dashed border-border bg-muted/60 text-muted-foreground",
      htmlClassName: "[&_a]:text-foreground [&_a]:underline [&_img]:rounded-md",
      recalledBubbleClassName:
        "border-dashed border-border/70 bg-muted/40 text-muted-foreground",
      recalledHtmlClassName: "text-muted-foreground [&_p]:text-muted-foreground",
      metaClassName: "text-center",
    };
  }
  if (message.senderType === "ai") {
    return {
      rowClassName: "justify-end",
      bubbleClassName: "border-primary/15 bg-primary/5 text-foreground shadow-sm",
      htmlClassName: "[&_a]:text-foreground [&_a]:underline [&_img]:rounded-md",
      recalledBubbleClassName:
        "border-dashed border-primary/20 bg-primary/10 text-primary",
      recalledHtmlClassName: "text-primary [&_p]:text-primary",
      metaClassName: "text-right",
    };
  }
  return {
    rowClassName: "justify-end",
    bubbleClassName: "border-transparent bg-primary text-primary-foreground shadow-sm",
    htmlClassName:
      "[&_p]:text-primary-foreground [&_a]:text-primary-foreground [&_a]:underline [&_img]:rounded-md",
    recalledBubbleClassName:
      "border-dashed border-primary/20 bg-primary/10 text-primary",
    recalledHtmlClassName: "text-primary [&_p]:text-primary",
    metaClassName: "text-right",
  };
}

export function ConversationDetailDialog({
  open,
  loading,
  saving,
  readOnly = false,
  item,
  detail,
  messages: rawMessages,
  messagesHasMore = false,
  loadingMoreMessages = false,
  canAssignConversation = false,
  canDispatchConversation = false,
  canTransferConversation = false,
  canCloseConversation = false,
  onLoadMoreMessages,
  onOpenChange,
  onOpenAssign,
  onDispatch,
  onOpenTransfer,
  onRead,
  onOpenClose,
}: ConversationDetailDialogProps) {
  const t = useI18n();
  const messages = useMemo(
    () => (Array.isArray(rawMessages) ? rawMessages : []),
    [rawMessages],
  );
  const currentConversation = detail ?? item;
  const isClosedConversation = currentConversation?.status === 4;
  const isPendingConversation = currentConversation?.status === 2;
  const statusMeta = currentConversation
    ? getStatusMeta(currentConversation.status, t)
    : null;
  const messageBottomRef = useRef<HTMLDivElement | null>(null);
  const messagesScrollRootRef = useRef<HTMLDivElement | null>(null);
  const loadMoreSentinelRef = useRef<HTMLDivElement | null>(null);
  const pendingScrollAnchorRef = useRef<{
    scrollHeight: number;
    scrollTop: number;
  } | null>(null);
  const prevLoadingMoreRef = useRef(false);
  const { open: openImageLightbox, close: closeImageLightbox } =
    useImageLightbox();

  const getMessagesViewport = useCallback((): HTMLElement | null => {
    return (
      messagesScrollRootRef.current?.querySelector(
        '[data-slot="scroll-area-viewport"]',
      ) ?? null
    );
  }, []);

  useEffect(() => {
    if (!open) {
      closeImageLightbox();
      return;
    }
    if (loading) {
      return;
    }
    const bottom = messageBottomRef.current;
    if (!bottom) {
      return;
    }
    bottom.scrollIntoView({ block: "end", behavior: "smooth" });
  }, [open, loading, closeImageLightbox]);

  useLayoutEffect(() => {
    const wasLoading = prevLoadingMoreRef.current;
    prevLoadingMoreRef.current = loadingMoreMessages;
    if (wasLoading && !loadingMoreMessages && pendingScrollAnchorRef.current) {
      const vp = getMessagesViewport();
      const anchor = pendingScrollAnchorRef.current;
      pendingScrollAnchorRef.current = null;
      if (vp && anchor) {
        const delta = vp.scrollHeight - anchor.scrollHeight;
        vp.scrollTop = anchor.scrollTop + delta;
      }
    }
  }, [loadingMoreMessages, messages, getMessagesViewport]);

  useEffect(() => {
    if (!open || loading || !messagesHasMore || !onLoadMoreMessages) {
      return;
    }
    const root = getMessagesViewport();
    const sentinel = loadMoreSentinelRef.current;
    if (!root || !sentinel) {
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        const hit = entries.some((e) => e.isIntersecting);
        if (!hit || loadingMoreMessages) {
          return;
        }
        const vp = getMessagesViewport();
        if (vp) {
          pendingScrollAnchorRef.current = {
            scrollHeight: vp.scrollHeight,
            scrollTop: vp.scrollTop,
          };
        }
        void onLoadMoreMessages();
      },
      { root, rootMargin: "120px 0px 0px 0px", threshold: 0 },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [
    open,
    loading,
    messagesHasMore,
    loadingMoreMessages,
    messages.length,
    onLoadMoreMessages,
    getMessagesViewport,
  ]);

  return (
    <ProjectDialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        <div className="flex items-center gap-3">
          <span>{currentConversation?.customerName || t("conversationMonitor.detailFallbackTitle")}</span>

          <div>
            {statusMeta ? (
              <Badge variant={statusMeta.variant}>{statusMeta.label}</Badge>
            ) : null}
          </div>
        </div>
      }
      size="xl"
      // allowFullscreen
      defaultFullscreen
      bodyScrollable={false}
      bodyClassName="flex min-h-0 flex-1 flex-col overflow-hidden p-0"
      contentClassName="h-[calc(100vh-40px)] max-h-[calc(100vh-40px)]"
      footer={
        <div className="flex w-full flex-wrap items-center justify-between gap-3">
          <div className="text-sm text-muted-foreground">
            {currentConversation
              ? t("conversationMonitor.lastActivePrefix", {
                  time: formatDateTime(currentConversation.lastMessageAt),
                })
              : t("conversationMonitor.noConversationInfo")}
          </div>
          {readOnly ? (
            <Button variant="outline" onClick={() => onOpenChange(false)}>
              {t("common.close")}
            </Button>
          ) : (
            <div className="flex flex-wrap gap-2">
              {canAssignConversation ? (
                <Button
                  variant="outline"
                  onClick={onOpenAssign}
                  disabled={saving || !currentConversation || currentConversation.status !== 2}
                >
                  <MessageCircleMoreIcon />
                  {saving ? t("conversationMonitor.processing") : t("conversationMonitor.assign")}
                </Button>
              ) : null}
              {canDispatchConversation ? (
                <Button
                  variant="outline"
                  onClick={() => void onDispatch()}
                  disabled={
                    saving || !currentConversation || !isPendingConversation
                  }
                >
                  <MessageCircleMoreIcon />
                  {saving ? t("conversationMonitor.processing") : t("conversationMonitor.retryDispatch")}
                </Button>
              ) : null}
              <Button
                variant="outline"
                onClick={() => void onRead()}
                disabled={saving || !currentConversation}
              >
                <CheckCheckIcon />
                {saving ? t("conversationMonitor.processing") : t("conversationMonitor.markRead")}
              </Button>
              {canTransferConversation ? (
                <Button
                  type="button"
                  variant="outline"
                  onClick={onOpenTransfer}
                  disabled={saving || !currentConversation || currentConversation.status !== 3}
                >
                  <MessageCircleMoreIcon />
                  {saving ? t("conversationMonitor.processing") : t("conversationMonitor.transfer")}
                </Button>
              ) : null}
              {canCloseConversation && !isClosedConversation ? (
                <Button
                  variant="outline"
                  onClick={onOpenClose}
                  disabled={saving || !currentConversation}
                >
                  <EyeIcon />
                  {saving ? t("conversationMonitor.processing") : t("conversationMonitor.close")}
                </Button>
              ) : null}
            </div>
          )}
        </div>
      }
    >
      {loading ? (
        <ConversationDetailLoading label={t("common.loadingData")} />
      ) : currentConversation ? (
        <div className="flex min-h-0 flex-1 flex-row overflow-hidden border-t">
          <aside className="flex w-90 h-full shrink-0 flex-col overflow-hidden bg-muted/20 border-r border-b-0">
            <div className="space-y-4 p-6">
              <div className="grid grid-cols-2 gap-3 text-sm">
                <InfoItem
                  label={t("conversationMonitor.columnServiceMode")}
                  value={getServiceModeLabel(currentConversation.serviceMode, t)}
                />
                <InfoItem
                  label={t("conversationMonitor.currentAssignee")}
                  value={currentConversation.currentAssigneeName || "-"}
                />
                <InfoItem
                  label={t("conversation.channelId")}
                  value={`${currentConversation.channelId || "-"}`}
                />
                <InfoItem
                  label={t("conversationMonitor.agentUnread")}
                  value={`${currentConversation.agentUnreadCount}`}
                />
                <InfoItem
                  label={t("conversationMonitor.customerUnread")}
                  value={`${currentConversation.customerUnreadCount}`}
                />
                <InfoItem
                  label={t("conversationMonitor.columnLastActive")}
                  value={formatDateTime(currentConversation.lastMessageAt)}
                  fullWidth
                />
                <InfoItem
                  label={t("conversationMonitor.closedAt")}
                  value={formatDateTime(currentConversation.closedAt)}
                  fullWidth
                />
              </div>
            </div>

            <Separator />

            <ScrollArea className="min-h-0 flex-1">
              <div className="space-y-4 p-6">
                <section className="space-y-3">
                  <div className="flex items-center justify-between">
                    <p className="text-sm font-medium">
                      {t("conversationMonitor.participants")}
                    </p>
                    <span className="text-xs text-muted-foreground">
                      {t("conversationMonitor.participantCount", {
                        count: detail?.participants?.length ?? 0,
                      })}
                    </span>
                  </div>
                  {detail?.participants?.length ? (
                    <div className="space-y-3">
                      {detail.participants.map((participant) => (
                        <div
                          key={participant.id}
                          className="rounded-lg border bg-background p-3"
                        >
                          <div className="flex items-center justify-between gap-3">
                            <span className="text-sm font-medium">
                              {participant.participantType || "-"}
                            </span>
                          </div>
                          <div className="mt-1 text-sm text-muted-foreground">
                            {t("conversationMonitor.participantIdentity", {
                              value: getParticipantIdentity(participant),
                            })}
                          </div>
                          <div className="mt-1 text-xs text-muted-foreground">
                            {t("conversationMonitor.participantJoinedAt", {
                              time: formatDateTime(participant.joinedAt),
                            })}
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="rounded-lg border border-dashed bg-background p-4 text-sm text-muted-foreground">
                      {t("conversationMonitor.emptyParticipants")}
                    </div>
                  )}
                </section>
              </div>
            </ScrollArea>
          </aside>

          <section className="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background">
            <div ref={messagesScrollRootRef} className="min-h-0 flex-1">
              <ScrollArea className="h-full min-h-0 bg-muted/10">
                <div className="space-y-4 px-6 py-5">
                  {messagesHasMore ? (
                    <div
                      ref={loadMoreSentinelRef}
                      className="flex min-h-8 flex-col items-center justify-center py-2 text-xs text-muted-foreground"
                    >
                      {loadingMoreMessages
                        ? t("conversationMonitor.loadingOlder")
                        : t("conversationMonitor.loadOlderHint")}
                    </div>
                  ) : null}
                  {messages.length ? (
                    messages.map((message) => {
                      const layout = getMessageLayout(message);
                      const isRecalled =
                        Boolean(message.recalledAt) || message.sendStatus === 6;
                      return (
                        <div
                          key={message.id}
                          className={`flex ${layout.rowClassName}`}
                        >
                          <div className="max-w-[85%] space-y-2">
                            <div
                              className={`text-xs text-muted-foreground ${layout.metaClassName}`}
                            >
                              <span>{getSenderLabel(message, t)}</span>
                              <span className="mx-2">·</span>
                              <span>{formatDateTime(message.sentAt)}</span>
                              {isRecalled ? (
                                <>
                                  <span className="mx-2">·</span>
                                  <span>{t("conversationMonitor.messageRecalled")}</span>
                                </>
                              ) : null}
                            </div>
                            <div
                              className={`rounded-2xl border px-4 py-3 text-sm leading-6 ${
                                isRecalled
                                  ? layout.recalledBubbleClassName
                                  : layout.bubbleClassName
                              }`}
                            >
                              {isRecalled ? (
                                <div className={layout.recalledHtmlClassName}>
                                  {t("conversationMonitor.messageRecalledBody")}
                                </div>
                              ) : (
                                <ImMessageHTML
                                  html={renderIMMessageHTML(message)}
                                  className={`${layout.htmlClassName} [&_img]:max-w-full [&_img]:cursor-zoom-in`}
                                  onImageClick={openImageLightbox}
                                />
                              )}
                            </div>
                            <div
                              className={`text-xs text-muted-foreground ${layout.metaClassName}`}
                            >
                              {t("conversationMonitor.readStatus", {
                                agent: message.agentRead
                                  ? t("conversationMonitor.read")
                                  : t("conversationMonitor.unread"),
                                customer: message.customerRead
                                  ? t("conversationMonitor.read")
                                  : t("conversationMonitor.unread"),
                              })}
                            </div>
                          </div>
                        </div>
                      );
                    })
                  ) : (
                    <div className="flex h-full min-h-80 items-center justify-center rounded-xl border border-dashed bg-background text-sm text-muted-foreground">
                      {t("conversationMonitor.emptyMessages")}
                    </div>
                  )}
                  <div ref={messageBottomRef} />
                </div>
              </ScrollArea>
            </div>
          </section>
        </div>
      ) : (
        <div className="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
          {t("conversationMonitor.emptyDetail")}
        </div>
      )}
    </ProjectDialog>
  );
}

function ConversationDetailLoading({ label }: { label: string }) {
  return (
    <div
      className="flex min-h-0 flex-1 flex-row overflow-hidden border-t"
      role="status"
      aria-busy="true"
      aria-label={label}
    >
      <aside className="flex h-full w-90 shrink-0 flex-col overflow-hidden border-r border-b-0 bg-muted/20">
        <div className="space-y-4 p-6">
          <div className="grid grid-cols-2 gap-3">
            {Array.from({ length: 6 }).map((_, index) => (
              <div key={index} className={index > 3 ? "col-span-2" : undefined}>
                <Skeleton className="h-3 w-16" />
                <Skeleton className="mt-2 h-5 w-28 max-w-full" />
              </div>
            ))}
          </div>
        </div>
        <Separator />
        <div className="min-h-0 flex-1 space-y-3 p-6">
          <div className="flex items-center justify-between">
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-3 w-10" />
          </div>
          {Array.from({ length: 3 }).map((_, index) => (
            <div key={index} className="rounded-lg border bg-background p-3">
              <Skeleton className="h-4 w-24" />
              <Skeleton className="mt-2 h-3 w-36" />
              <Skeleton className="mt-2 h-3 w-28" />
            </div>
          ))}
        </div>
      </aside>
      <section className="flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background">
        <div className="min-h-0 flex-1 bg-muted/10 px-6 py-5">
          <div className="space-y-5">
            {Array.from({ length: 5 }).map((_, index) => (
              <div key={index} className={`flex ${index % 2 === 0 ? "justify-start" : "justify-end"}`}>
                <div className="w-[min(520px,85%)] space-y-2">
                  <Skeleton className={`h-3 w-40 ${index % 2 === 0 ? "" : "ml-auto"}`} />
                  <Skeleton className="h-20 rounded-2xl" />
                  <Skeleton className={`h-3 w-32 ${index % 2 === 0 ? "" : "ml-auto"}`} />
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>
    </div>
  );
}

type InfoItemProps = {
  label: string;
  value: string;
  fullWidth?: boolean;
};

function InfoItem({ label, value, fullWidth = false }: InfoItemProps) {
  return (
    <div className={fullWidth ? "col-span-2" : undefined}>
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 break-all font-medium">{value || "-"}</p>
    </div>
  );
}
