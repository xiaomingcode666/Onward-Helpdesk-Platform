"use client";

import {
  memo,
  type ReactNode,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import {
  AlertTriangleIcon,
  ForwardIcon,
  Loader2Icon,
  LockKeyholeIcon,
  MessageSquareTextIcon,
  ShieldCheckIcon,
  TimerIcon,
  UserCheckIcon,
  WorkflowIcon,
} from "lucide-react";
import { SearchField, StandardModal } from "@railops/ui";
import { toast } from "sonner";

import { ConversationTransferDialog } from "@/components/conversation-actions/transfer-dialog";
import { ImMessageHTML } from "@/components/im-message-html";
import { useImageLightbox } from "@/components/image-lightbox";
import { JsonTreeViewer } from "@/components/json-tree-viewer";
import { ProjectDialog } from "@/components/project-dialog";
import { useI18n } from "@/i18n/provider";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable";
import { useIsLgUp } from "@/hooks/use-lg-media";
import {
  assignAgentConversation,
  fetchAgentConversations,
  forwardAgentImage,
  type AgentMessage,
} from "@/lib/api/agent";
import {
  fetchAIWorkflowRun,
  type AIWorkflowNodeRun,
  type AIWorkflowRun,
} from "@/lib/api/admin";
import { readSession } from "@/lib/auth";
import { renderIMMessageHTML } from "@/lib/im-message";
import {
  agentConversationSelectors,
  useAgentConversationsStore,
  type AgentConversationFilterKey,
} from "@/lib/stores/agent-conversations";
import { formatDateTime, generateUUID } from "@/lib/utils";
import { AgentMessageEditor } from "./agent-message-editor";

const EMPTY_AGENT_MESSAGES: AgentMessage[] = [];

type WorkflowRunDetailT = ReturnType<typeof useI18n>;
const wrd = (
  t: WorkflowRunDetailT,
  key: string,
  values?: Record<string, string | number>,
) => t(`dashboardExtract.workflowRunDetail.${key}`, values);

type ComposerNoticeTone = "muted" | "ai" | "action";

type ComposerNoticeProps = {
  icon: ReactNode;
  message: string;
  tone?: ComposerNoticeTone;
  action?: ReactNode;
};

function getComposerNoticeClassName(tone: ComposerNoticeTone) {
  if (tone === "ai") {
    return {
      wrap: "border-primary/15 bg-primary/5",
      icon: "bg-primary/10 text-primary",
    };
  }

  if (tone === "action") {
    return {
      wrap: "border-border bg-muted/35",
      icon: "bg-background text-foreground shadow-sm",
    };
  }

  return {
    wrap: "border-border bg-muted/30",
    icon: "bg-muted text-muted-foreground",
  };
}

function ComposerNotice({
  icon,
  message,
  tone = "muted",
  action,
}: ComposerNoticeProps) {
  const className = getComposerNoticeClassName(tone);

  return (
    <div className="flex h-full min-h-0 items-center justify-center px-4 py-3">
      <div
        className={`flex w-full max-w-lg items-center gap-3 rounded-md border px-4 py-3 text-sm shadow-sm ${className.wrap}`}
      >
        <div
          className={`flex size-9 shrink-0 items-center justify-center rounded-md ${className.icon}`}
        >
          {icon}
        </div>
        <div className="min-w-0 flex-1 leading-6 text-foreground">
          {message}
        </div>
        {action ? <div className="shrink-0">{action}</div> : null}
      </div>
    </div>
  );
}

export function ChatPanel() {
  const t = useI18n();
  const conversation = useAgentConversationsStore(
    agentConversationSelectors.selectedConversation,
  );
  const messages =
    useAgentConversationsStore((state) => state.messages) ??
    EMPTY_AGENT_MESSAGES;
  const loading = useAgentConversationsStore((state) => state.messagesLoading);
  const sending = useAgentConversationsStore((state) => state.sending);
  const uploadingAsset = useAgentConversationsStore(
    (state) => state.uploadingAsset,
  );
  const sendMessage = useAgentConversationsStore((state) => state.sendMessage);
  const uploadImage = useAgentConversationsStore((state) => state.uploadImage);
  const sendAttachment = useAgentConversationsStore((state) => state.sendAttachment);
  const sendVoice = useAgentConversationsStore((state) => state.sendVoice);
  const markSelectedConversationRead = useAgentConversationsStore(
    (state) => state.markSelectedConversationRead,
  );
  const recallMessage = useAgentConversationsStore((state) => state.recallMessage);
  const recallingMessageId = useAgentConversationsStore(
    (state) => state.recallingMessageId,
  );
  const loadConversations = useAgentConversationsStore((state) => state.loadConversations);
  const loadMessages = useAgentConversationsStore((state) => state.loadMessages);
  const loadOlderMessages = useAgentConversationsStore(
    (state) => state.loadOlderMessages,
  );
  const messagesHasMore = useAgentConversationsStore(
    (state) => state.messagesHasMore,
  );
  const messagesLoadingMore = useAgentConversationsStore(
    (state) => state.messagesLoadingMore,
  );
  const conversationFilter = useAgentConversationsStore((state) => state.conversationFilter);
  const setConversationFilter = useAgentConversationsStore(
    (state) => state.setConversationFilter,
  );
  const messagesContainerRef = useRef<HTMLDivElement>(null);
  const messagesContentRef = useRef<HTMLDivElement>(null);
  const scrollBottomRafRef = useRef<number | null>(null);
  const shouldStickToBottomRef = useRef(true);
  const prependScrollAnchorRef = useRef<{ height: number; top: number } | null>(
    null,
  );
  const [claiming, setClaiming] = useState(false);
  const [claimDialogOpen, setClaimDialogOpen] = useState(false);
  const [transferDialogOpen, setTransferDialogOpen] = useState(false);
  const [workflowRunDialogOpen, setWorkflowRunDialogOpen] = useState(false);
  const [workflowRunLoading, setWorkflowRunLoading] = useState(false);
  const [activeWorkflowRun, setActiveWorkflowRun] =
    useState<AIWorkflowRun | null>(null);
  const [forwardImageMessage, setForwardImageMessage] = useState<AgentMessage | null>(null);
  const isLgUp = useIsLgUp();
  const isClosedConversation = conversation?.status === 4;
  const isPendingConversation = conversation?.status === 2;
  const showMessageEditor = !isClosedConversation && !isPendingConversation;
  const currentUserId = readSession()?.user?.id ?? 0;

  const switchToMyActiveIfNeeded = () => {
    if (conversationFilter !== "pending") {
      return;
    }
    setConversationFilter("active" satisfies AgentConversationFilterKey);
  };

  const getViewport = useCallback(
    () => messagesContainerRef.current,
    [],
  );

  const isNearBottom = useCallback(
    (element: HTMLElement, threshold = 80) =>
      element.scrollHeight - element.scrollTop - element.clientHeight <=
      threshold,
    [],
  );

  const scrollToBottom = useCallback(() => {
    const viewport = getViewport();
    if (!viewport) {
      return;
    }
    viewport.scrollTop = viewport.scrollHeight;
  }, [getViewport]);

  /**
   * Match the widget message list: keep scrolling for a few frames until
   * scrollHeight stabilizes, which prevents stacked scroll jumps.
   */
  const scheduleScrollToBottom = useCallback(
    (attempts = 4) => {
      if (scrollBottomRafRef.current !== null) {
        cancelAnimationFrame(scrollBottomRafRef.current);
      }
      const run = (remaining: number, previousHeight = -1) => {
        scrollBottomRafRef.current = requestAnimationFrame(() => {
          const viewport = getViewport();
          if (!viewport) {
            scrollBottomRafRef.current = null;
            return;
          }
          const currentHeight = viewport.scrollHeight;
          scrollToBottom();
          if (remaining > 1 && currentHeight !== previousHeight) {
            run(remaining - 1, currentHeight);
            return;
          }
          scrollBottomRafRef.current = null;
        });
      };
      run(attempts);
    },
    [getViewport, scrollToBottom],
  );

  const handleImageSettled = useCallback(() => {
    if (!shouldStickToBottomRef.current) {
      return;
    }
    scheduleScrollToBottom();
  }, [scheduleScrollToBottom]);

  const maybeMarkConversationRead = useCallback(() => {
    const viewport = getViewport();
    if (!viewport || !conversation || loading) {
      return;
    }
    if (
      typeof document !== "undefined" &&
      document.visibilityState !== "visible"
    ) {
      return;
    }
    if (!isNearBottom(viewport)) {
      return;
    }
    void markSelectedConversationRead().catch((error) => {
      toast.error(error instanceof Error ? error.message : t("conversation.markReadFailed"));
    });
  }, [
    conversation,
    getViewport,
    isNearBottom,
    loading,
    markSelectedConversationRead,
    t,
  ]);

  useEffect(() => {
    const viewport = getViewport();
    if (!viewport) {
      return;
    }

    const handleScroll = () => {
      shouldStickToBottomRef.current = isNearBottom(viewport);
      if (shouldStickToBottomRef.current) {
        maybeMarkConversationRead();
      }
    };

    handleScroll();
    viewport.addEventListener("scroll", handleScroll);
    return () => {
      viewport.removeEventListener("scroll", handleScroll);
    };
  }, [conversation?.id, getViewport, isNearBottom, maybeMarkConversationRead]);

  useLayoutEffect(() => {
    shouldStickToBottomRef.current = true;
    scheduleScrollToBottom();
    return () => {
      if (scrollBottomRafRef.current !== null) {
        cancelAnimationFrame(scrollBottomRafRef.current);
        scrollBottomRafRef.current = null;
      }
    };
  }, [conversation?.id, scheduleScrollToBottom]);

  useLayoutEffect(() => {
    const viewport = getViewport();
    if (!viewport) {
      return;
    }
    const anchor = prependScrollAnchorRef.current;
    if (anchor) {
      prependScrollAnchorRef.current = null;
      const nextHeight = viewport.scrollHeight;
      viewport.scrollTop = nextHeight - anchor.height + anchor.top;
      return;
    }
    if (shouldStickToBottomRef.current) {
      scheduleScrollToBottom();
    }
  }, [messages, getViewport, scheduleScrollToBottom]);

  useEffect(() => {
    const content = messagesContentRef.current;
    if (!content) {
      return;
    }

    const observer = new ResizeObserver(() => {
      if (!shouldStickToBottomRef.current) {
        return;
      }
      scheduleScrollToBottom();
    });

    observer.observe(content);
    return () => {
      observer.disconnect();
    };
  }, [conversation?.id, scheduleScrollToBottom]);

  useEffect(() => {
    maybeMarkConversationRead();
  }, [maybeMarkConversationRead, messages.length]);

  useEffect(() => {
    const handleVisibilityChange = () => {
      if (document.visibilityState === "visible") {
        maybeMarkConversationRead();
      }
    };
    const handleFocus = () => {
      maybeMarkConversationRead();
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);
    window.addEventListener("focus", handleFocus);
    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
      window.removeEventListener("focus", handleFocus);
    };
  }, [maybeMarkConversationRead]);

  const handleLoadOlder = async () => {
    const viewport = getViewport();
    if (!viewport || messagesLoadingMore || !messagesHasMore) {
      return;
    }
    prependScrollAnchorRef.current = {
      height: viewport.scrollHeight,
      top: viewport.scrollTop,
    };
    try {
      await loadOlderMessages();
    } catch (error) {
      prependScrollAnchorRef.current = null;
      toast.error(error instanceof Error ? error.message : t("conversation.loadHistoryFailed"));
    }
  };

  const handleSend = async (html: string) => {
    if (!conversation || sending || isClosedConversation) return;
    try {
      shouldStickToBottomRef.current = true;
      await sendMessage(html);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("conversation.sendMessageFailed"));
    }
  };

  const handleClaim = async () => {
    if (!conversation || claiming) return;
    const session = readSession();
    if (!session?.user?.id) {
      toast.error(t("conversation.claimRequiresSignIn"));
      return;
    }

    setClaiming(true);
    try {
      await assignAgentConversation(
        conversation.id,
        session.user.id,
        t("conversation.claimReason"),
      );

      switchToMyActiveIfNeeded();
      setClaimDialogOpen(false);
      toast.success(t("conversation.claimSuccess"));
      await reloadConversationData(conversation.id);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("conversation.claimFailed"));
    } finally {
      setClaiming(false);
    }
  };

  const reloadConversationData = async (conversationId: number) => {
    await loadConversations();
    await loadMessages(conversationId, { forceLoading: true, reset: true });
  };

  const openWorkflowRunDetail = useCallback(
    async (runId: number) => {
      setWorkflowRunDialogOpen(true);
      setWorkflowRunLoading(true);
      try {
        const data = await fetchAIWorkflowRun(runId);
        setActiveWorkflowRun(data);
      } catch (error) {
        toast.error(error instanceof Error ? error.message : wrd(t, "errors.loadDetailFailed"));
        setWorkflowRunDialogOpen(false);
      } finally {
        setWorkflowRunLoading(false);
      }
    },
    [],
  );

  if (!conversation) {
    return (
      <div className="mt-10 flex flex-1 items-center justify-center px-4">
        <div className="text-center text-muted-foreground">
          <p className="text-lg">{t("conversation.empty")}</p>
          <p className="mt-1 text-sm lg:hidden">
            {t("conversation.noConversationMobile")}
          </p>
          <p className="mt-1 hidden text-sm lg:block">
            {t("conversation.selectConversationToChat")}
          </p>
        </div>
      </div>
    );
  }

  const messagesScroll = (
    <div
      ref={messagesContainerRef}
      className="h-full min-h-0 flex-1 overflow-y-auto p-4 remote-helpdesk-scrollbar"
    >
      <div ref={messagesContentRef} className="flex flex-col">
        {!loading && messages.length > 0 && messagesHasMore ? (
          <div className="mb-4 flex justify-center">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={messagesLoadingMore}
              onClick={() => void handleLoadOlder()}
            >
              {messagesLoadingMore ? t("conversation.loading") : t("conversation.loadOlder")}
            </Button>
          </div>
        ) : null}
        {loading ? (
          <div className="py-8 text-center text-sm text-muted-foreground">
            {t("conversation.loading")}
          </div>
        ) : messages.length > 0 ? (
          messages.map((message) => (
            <MessageItem
              key={message.id}
              message={message}
              onImageSettled={handleImageSettled}
              canRecall={message.senderType === "agent" && message.senderId === currentUserId}
              recalling={recallingMessageId === message.id}
              onRecall={async (messageId) => {
                await recallMessage(messageId);
              }}
              onOpenWorkflowRun={openWorkflowRunDetail}
              onForwardImage={setForwardImageMessage}
            />
          ))
        ) : (
          <div className="py-8 text-center text-sm text-muted-foreground">
            {t("conversation.emptyMessages")}
          </div>
        )}
      </div>
    </div>
  );

  const bottomPanel = (
    <div className="h-full overflow-auto border-t border-border/80 bg-card">
      {isClosedConversation ? (
        <ComposerNotice
          icon={<LockKeyholeIcon className="size-4" />}
          message={t("conversation.closedNotice")}
        />
      ) : conversation?.status === 1 ? (
        <ComposerNotice
          icon={<MessageSquareTextIcon className="size-4" />}
          message={t("conversation.aiServingNotice")}
          tone="ai"
        />
      ) : isPendingConversation ? (
        <ComposerNotice
          icon={<UserCheckIcon className="size-4" />}
          message={t("conversation.claimCurrent")}
          tone="action"
          action={
            <Button
              onClick={() => setClaimDialogOpen(true)}
              disabled={claiming}
              size="sm"
            >
              {claiming ? t("conversation.claiming") : t("conversation.claim")}
            </Button>
          }
        />
      ) : (
        <div className="flex h-full min-h-0 flex-col">
          <div className="min-h-0 flex-1">
            <AgentMessageEditor
              disabled={!conversation || sending}
              uploadingAsset={uploadingAsset}
              onSend={handleSend}
              onUploadImage={async (file) => {
                shouldStickToBottomRef.current = true;
                const uploaded = await uploadImage(file);
                return uploaded;
              }}
              onSendAttachment={async (file) => {
                shouldStickToBottomRef.current = true;
                try {
                  await sendAttachment(file);
                } catch (error) {
                  toast.error(error instanceof Error ? error.message : t("conversation.sendAttachmentFailed"));
                }
              }}
              onSendVoice={async (file, durationSeconds) => {
                shouldStickToBottomRef.current = true;
                try {
                  await sendVoice(file, durationSeconds);
                } catch (error) {
                  toast.error(error instanceof Error ? error.message : t("conversation.sendVoiceFailed"));
                }
              }}
            />
          </div>
        </div>
      )}
    </div>
  );

  return (
    <div className="flex h-full min-h-0 flex-1 flex-col overflow-hidden">
      {isLgUp ? (
        <ResizablePanelGroup
          orientation="vertical"
          className="flex min-h-0 flex-1 flex-col"
        >
          <ResizablePanel
            defaultSize={showMessageEditor ? "72%" : "82%"}
            minSize="35%"
            className="min-h-0"
          >
            {messagesScroll}
          </ResizablePanel>
          <ResizableHandle withHandle />
          <ResizablePanel
            defaultSize={showMessageEditor ? "28%" : "18%"}
            minSize={showMessageEditor ? "18%" : "12%"}
            maxSize={showMessageEditor ? "55%" : "30%"}
            className="min-h-0"
          >
            {bottomPanel}
          </ResizablePanel>
        </ResizablePanelGroup>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <div className="min-h-0 flex-1">{messagesScroll}</div>
          <div className="shrink-0 pb-[env(safe-area-inset-bottom)] lg:pb-0">
            {bottomPanel}
          </div>
        </div>
      )}
      <StandardModal
        open={claimDialogOpen}
        onCancel={() => {
          if (claiming) {
            return;
          }
          setClaimDialogOpen(false);
        }}
        title={t("conversation.claimTitle")}
        width={448}
        footer={
          <>
            <Button
              type="button"
              variant="outline"
              disabled={claiming}
              onClick={() => setClaimDialogOpen(false)}
            >
              {t("conversation.cancel")}
            </Button>
            <Button
              type="button"
              disabled={claiming}
              onClick={() => void handleClaim()}
            >
              {claiming ? t("conversation.claiming") : t("conversation.confirmClaim")}
            </Button>
          </>
        }
      >
        <div className="text-sm text-muted-foreground">
          {conversation
            ? `${t("conversation.claimConfirmPrefix")}${
                conversation.customerName ||
                `${t("conversation.customerFallbackPrefix")}${conversation.customerId || conversation.id}`
              }${t("conversation.claimConfirmSuffix")}`
            : t("conversation.claimCurrent")}
        </div>
      </StandardModal>
      <ConversationTransferDialog
        open={transferDialogOpen}
        mode="transfer"
        conversationId={conversation.id}
        onOpenChange={setTransferDialogOpen}
        onSuccess={async () => {
          await reloadConversationData(conversation.id);
        }}
      />
      <WorkflowRunDetailDialog
        open={workflowRunDialogOpen}
        loading={workflowRunLoading}
        run={activeWorkflowRun}
        t={t}
        onOpenChange={(open) => {
          setWorkflowRunDialogOpen(open);
          if (!open) {
            setActiveWorkflowRun(null);
          }
        }}
      />
      <ForwardImageDialog
        message={forwardImageMessage}
        sourceConversationId={conversation.id}
        onOpenChange={(open) => {
          if (!open) setForwardImageMessage(null);
        }}
        onForwarded={async () => {
          setForwardImageMessage(null);
          await loadConversations();
        }}
      />
    </div>
  );
}

type MessageItemProps = {
  message: AgentMessage;
  onImageSettled: () => void;
  canRecall: boolean;
  recalling: boolean;
  onRecall: (messageId: number) => Promise<void>;
  onOpenWorkflowRun: (runId: number) => Promise<void>;
  onForwardImage: (message: AgentMessage) => void;
};

const MessageItem = memo(
  function MessageItem({
    message,
    onImageSettled,
    canRecall,
    recalling,
    onRecall,
    onOpenWorkflowRun,
    onForwardImage,
  }: MessageItemProps) {
    const t = useI18n();
    const { open: openImageLightbox } = useImageLightbox();
    const isCustomer = message.senderType === "customer";
    const isAi = message.senderType === "ai";
    const isPartner = message.senderType === "partner";
    const isSystem = message.senderType === "system";
    const isAgentSide = message.senderType === "agent" || isPartner || isAi;
    const isRecalled = Boolean(message.recalledAt) || message.sendStatus === 6;
    const senderName = isCustomer
      ? message.senderName || t("conversation.customerSender")
      : isAi
        ? wrd(t, "sender.onlineAgent")
			: message.senderName || (isPartner ? wrd(t, "sender.partnerSupport") : t("conversation.agentSender"));
    const agentAvatarSrc =
      isAgentSide && !isAi && message.senderAvatar?.trim()
        ? message.senderAvatar.trim()
        : undefined;
    const avatarFallback = isAi ? wrd(t, "sender.avatarFallback") : senderName.charAt(0);
    const htmlContent = isRecalled
      ? `<p>${t("conversation.messageRecalledHtml")}</p>`
      : buildMessageHTML(message);
    const bubbleClassName = isAi
      ? "border border-primary/15 bg-primary/5 text-foreground shadow-sm"
      : isAgentSide
        ? "bg-primary text-primary-foreground shadow-sm"
        : "border border-border/70 bg-muted/60 text-foreground shadow-sm";
    const htmlClassName = isAi
      ? "[&_a]:text-foreground [&_a]:underline [&_img]:rounded-md"
      : isAgentSide
        ? "[&_p]:text-primary-foreground [&_a]:text-primary-foreground [&_a]:underline [&_img]:rounded-md"
        : "[&_a]:text-foreground [&_a]:underline [&_img]:rounded-md";
    const avatarClassName = isAi
      ? "border border-primary/20 bg-primary/10 text-xs text-foreground"
      : isAgentSide
        ? "bg-primary text-xs text-primary-foreground"
        : "border border-border/70 bg-muted/60 text-xs text-foreground";
    const recalledBubbleClassName = isAgentSide
      ? "border border-dashed border-primary/20 bg-primary/10 text-primary"
      : "border border-dashed border-border/70 bg-muted/40 text-muted-foreground";
    const recalledHtmlClassName = isAgentSide
      ? "[&_p]:text-primary"
      : "[&_p]:text-muted-foreground";
    const showRecallAction = canRecall && !isRecalled;
    const showForwardAction = message.messageType === "image" && !isRecalled;

    if (isSystem) {
      return (
        <div className="mb-4 flex justify-center">
          <div className="max-w-[80%] rounded-md border border-border/70 bg-muted/60 px-3 py-2 text-center text-xs leading-5 text-muted-foreground">
            <ImMessageHTML html={htmlContent} onImageSettled={onImageSettled} />
            <div className="mt-1 text-rhd-2xs">{formatDateTime(message.sentAt || "")}</div>
          </div>
        </div>
      );
    }

    return (
      <div
        className={`mb-4 flex items-start gap-2 ${
          isAgentSide ? "justify-end" : "justify-start"
        }`}
      >
        {isAgentSide ? (
          <>
            <div className="flex max-w-[70%] flex-col items-end">
              <div className="mb-1 text-xs text-muted-foreground">
                {senderName}
              </div>
              <div
                className={`w-fit rounded-2xl px-3 py-2 text-left ${
                  isRecalled ? recalledBubbleClassName : bubbleClassName
                }`}
              >
                <ImMessageHTML
                  html={htmlContent}
                  className={isRecalled ? recalledHtmlClassName : htmlClassName}
                  onImageSettled={onImageSettled}
                  onImageClick={isRecalled ? undefined : openImageLightbox}
                />
              </div>
              <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                <span>{formatDateTime(message.sentAt || "")}</span>
                {isRecalled ? <span>{t("conversation.messageRecalled")}</span> : null}
                {message.sendStatus === 2 && !isRecalled && (
                  <span>
                    {message.customerRead
                      ? t("conversation.customerRead")
                      : t("conversation.customerUnread")}
                  </span>
                )}
                {showRecallAction ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-auto px-1 py-0 text-xs text-muted-foreground"
                    disabled={recalling}
                    onClick={() => {
                      void onRecall(message.id).catch((error) => {
                        toast.error(error instanceof Error ? error.message : t("conversation.recallFailed"));
                      });
                    }}
                  >
                    {recalling ? t("conversation.recalling") : t("conversation.recall")}
                  </Button>
                ) : null}
                {showForwardAction ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-auto gap-1 px-1 py-0 text-xs text-muted-foreground"
                    onClick={() => onForwardImage(message)}
                  >
                    <ForwardIcon className="size-3" />
                    {t("conversation.forwardImage")}
                  </Button>
                ) : null}
                {isAi && message.workflowRunId ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-auto gap-1 px-1 py-0 text-xs text-muted-foreground"
                    onClick={() => {
                      void onOpenWorkflowRun(message.workflowRunId ?? 0);
                    }}
                  >
                    <WorkflowIcon className="size-3" />
                    {wrd(t, "actions.runDetail")}
                  </Button>
                ) : null}
              </div>
            </div>
            <Avatar className="size-8 shrink-0">
              <AvatarImage src={agentAvatarSrc ?? ""} />
              <AvatarFallback className={avatarClassName}>
                {isAi ? <MessageSquareTextIcon className="size-4" /> : avatarFallback}
              </AvatarFallback>
            </Avatar>
          </>
        ) : (
          <>
            <Avatar className="size-8 shrink-0">
              <AvatarImage src="" />
              <AvatarFallback className={avatarClassName}>
                {t("conversation.customerAvatar")}
              </AvatarFallback>
            </Avatar>
            <div className="max-w-[70%]">
              <div className="mb-1 text-xs text-muted-foreground">
                {senderName}
              </div>
              <div
                className={`w-fit rounded-2xl px-3 py-2 ${
                  isRecalled ? recalledBubbleClassName : bubbleClassName
                }`}
              >
                <ImMessageHTML
                  html={htmlContent}
                  className={isRecalled ? recalledHtmlClassName : htmlClassName}
                  onImageSettled={onImageSettled}
                  onImageClick={isRecalled ? undefined : openImageLightbox}
                />
              </div>
              <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                <span>{formatDateTime(message.sentAt || "")}</span>
                {isRecalled ? <span>{t("conversation.messageRecalled")}</span> : null}
                {showForwardAction ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-auto gap-1 px-1 py-0 text-xs text-muted-foreground"
                    onClick={() => onForwardImage(message)}
                  >
                    <ForwardIcon className="size-3" />
                    {t("conversation.forwardImage")}
                  </Button>
                ) : null}
              </div>
            </div>
          </>
        )}
      </div>
    );
  },
  (prevProps, nextProps) =>
    prevProps.message === nextProps.message &&
    prevProps.onImageSettled === nextProps.onImageSettled &&
    prevProps.canRecall === nextProps.canRecall &&
    prevProps.recalling === nextProps.recalling &&
    prevProps.onRecall === nextProps.onRecall &&
    prevProps.onOpenWorkflowRun === nextProps.onOpenWorkflowRun &&
    prevProps.onForwardImage === nextProps.onForwardImage,
);

function ForwardImageDialog({
  message,
  sourceConversationId,
  onOpenChange,
  onForwarded,
}: {
  message: AgentMessage | null;
  sourceConversationId: number;
  onOpenChange: (open: boolean) => void;
  onForwarded: () => Promise<void>;
}) {
  const t = useI18n();
  const [keyword, setKeyword] = useState("");
  const [targets, setTargets] = useState<Array<{ id: number; customerName: string }>>([]);
  const [targetId, setTargetId] = useState(0);
  const [loading, setLoading] = useState(false);
  const [forwarding, setForwarding] = useState(false);

  useEffect(() => {
    if (!message) return;
    let cancelled = false;
    const timer = window.setTimeout(() => {
      setLoading(true);
      fetchAgentConversations({ status: 3, keyword: keyword.trim() || undefined, page: 1, limit: 50 })
        .then((result) => {
          if (cancelled) return;
          setTargets(
            result.results
              .filter((item) => item.id !== sourceConversationId)
              .map((item) => ({ id: item.id, customerName: item.customerName })),
          );
        })
        .catch((error) => {
          if (!cancelled) {
            toast.error(error instanceof Error ? error.message : t("conversation.forwardLoadFailed"));
          }
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
    }, 200);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [keyword, message, sourceConversationId, t]);

  useEffect(() => {
    if (message) {
      setKeyword("");
      setTargetId(0);
    }
  }, [message]);

  const submit = async () => {
    if (!message || !targetId || forwarding) return;
    setForwarding(true);
    try {
      await forwardAgentImage({
        sourceMessageId: message.id,
        targetConversationId: targetId,
        clientMsgId: generateUUID(),
      });
      toast.success(t("conversation.forwardSuccess"));
      await onForwarded();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("conversation.forwardFailed"));
    } finally {
      setForwarding(false);
    }
  };

  return (
    <StandardModal
      open={Boolean(message)}
      onCancel={() => {
        if (forwarding) {
          return;
        }
        onOpenChange(false);
      }}
      title={t("conversation.forwardImageTitle")}
      width={448}
      footer={
        <>
          <Button variant="outline" disabled={forwarding} onClick={() => onOpenChange(false)}>
            {t("conversation.cancel")}
          </Button>
          <Button disabled={!targetId || forwarding} onClick={() => void submit()}>
            {forwarding ? <Loader2Icon className="size-4 animate-spin" /> : <ForwardIcon className="size-4" />}
            {forwarding ? t("conversation.forwarding") : t("conversation.forwardConfirm")}
          </Button>
        </>
      }
    >
      {message ? (
        <div className="flex min-h-28 items-center justify-center overflow-hidden border bg-muted/20 p-2">
          <ImMessageHTML
            html={buildMessageHTML(message)}
            className="[&_img]:my-0 [&_img]:max-h-40 [&_img]:max-w-full"
          />
        </div>
      ) : null}
      <div>
        <SearchField
          allowClear
          placeholder={t("conversation.forwardSearchPlaceholder")}
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
        />
      </div>
      <div className="max-h-64 space-y-1 overflow-y-auto border p-1">
        {loading ? (
          <div className="flex min-h-24 items-center justify-center text-muted-foreground">
            <Loader2Icon className="size-4 animate-spin" />
          </div>
        ) : targets.length ? (
          targets.map((target) => (
            <button
              key={target.id}
              type="button"
              className={`flex w-full items-center justify-between px-3 py-2 text-left text-sm hover:bg-muted ${targetId === target.id ? "bg-muted font-medium" : ""}`}
              onClick={() => setTargetId(target.id)}
            >
              <span className="truncate">{target.customerName || `#${target.id}`}</span>
              <span className="ml-3 shrink-0 text-xs text-muted-foreground">#{target.id}</span>
            </button>
          ))
        ) : (
          <div className="flex min-h-24 items-center justify-center text-sm text-muted-foreground">
            {t("conversation.forwardEmpty")}
          </div>
        )}
      </div>
    </StandardModal>
  );
}

function buildMessageHTML(message: {
  messageType: string;
  content: string;
  payload?: string;
}) {
  return renderIMMessageHTML(message);
}

function WorkflowRunDetailDialog({
  open,
  loading,
  run,
  t,
  onOpenChange,
}: {
  open: boolean;
  loading: boolean;
  run: AIWorkflowRun | null;
  t: WorkflowRunDetailT;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <ProjectDialog
      open={open}
      onOpenChange={onOpenChange}
      title={
        <span className="flex items-center gap-2">
          <WorkflowIcon className="size-4" />
          {wrd(t, "labels.title")}
        </span>
      }
      size="xl"
      allowFullscreen
      footer={
        <Button variant="outline" onClick={() => onOpenChange(false)}>
          {wrd(t, "actions.close")}
        </Button>
      }
    >
      {loading ? (
        <div className="px-6 py-10 text-sm text-muted-foreground">
          {wrd(t, "loading.detail")}
        </div>
      ) : run ? (
        <div className="space-y-4 px-6 pb-6">
          <div className="grid gap-2 rounded-lg border bg-muted/20 p-3 text-sm md:grid-cols-2">
            <WorkflowRunDetailRow
              label={<span className="inline-flex items-center gap-1"><WorkflowIcon className="size-3.5" />Workflow</span>}
              value={`#${run.workflowId} / v${run.workflowVersionId}`}
            />
            <WorkflowRunDetailRow label={wrd(t, "labels.conversation")} value={`#${run.conversationId}`} />
            <WorkflowRunDetailRow label={wrd(t, "labels.message")} value={`#${run.messageId}`} />
            <WorkflowRunDetailRow label={<span className="inline-flex items-center gap-1"><ShieldCheckIcon className="size-3.5" />{wrd(t, "labels.agentConfig")}</span>} value={`#${run.aiAgentId}`} />
            <WorkflowRunDetailRow
              label={wrd(t, "labels.status")}
              value={run.statusName || String(run.status)}
            />
            <WorkflowRunDetailRow
              label={wrd(t, "labels.started")}
              value={run.startedAt ? formatDateTime(run.startedAt) : ""}
            />
            <WorkflowRunDetailRow
              label={wrd(t, "labels.ended")}
              value={run.endedAt ? formatDateTime(run.endedAt) : ""}
            />
            <WorkflowRunDetailRow label={wrd(t, "labels.interruptNode")} value={run.interruptNodeId || ""} />
          </div>
          {run.errorMessage ? (
            <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
              {run.errorMessage}
            </div>
          ) : null}
          <div className="space-y-3">
            {(run.nodes ?? []).map((node) => (
              <WorkflowNodeRunBlock key={node.id} node={node} t={t} />
            ))}
            {!run.nodes || run.nodes.length === 0 ? (
              <p className="text-sm text-muted-foreground">{wrd(t, "empty.noNodes")}</p>
            ) : null}
          </div>
        </div>
      ) : (
        <div className="px-6 py-10 text-sm text-muted-foreground">
          {wrd(t, "empty.noRun")}
        </div>
      )}
    </ProjectDialog>
  );
}

function WorkflowRunDetailRow({
  label,
  value,
}: {
  label: ReactNode;
  value: string;
}) {
  const empty = !value.trim();
  return (
    <div className="flex gap-2.5 text-sm leading-snug">
      <span className="w-17 shrink-0 pt-px text-xs text-muted-foreground">
        {label}
      </span>
      <span
        className={`min-w-0 flex-1 break-all ${
          empty ? "text-muted-foreground" : "text-foreground"
        }`}
      >
        {empty ? "—" : value}
      </span>
    </div>
  );
}

function WorkflowNodeRunBlock({ node, t }: { node: AIWorkflowNodeRun; t: WorkflowRunDetailT }) {
  const inputValue = safeParseJSON(node.inputPreview);
  const outputValue = safeParseJSON(node.outputPreview);

  return (
    <div className="rounded-lg border bg-background p-3">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-2">
            <span className="truncate text-sm font-medium text-foreground">
              {node.nodeId || `Node #${node.id}`}
            </span>
            <WorkflowRunStatusBadge statusName={node.statusName} />
          </div>
          <div className="mt-1 text-xs text-muted-foreground">
            {node.nodeType || "unknown"}
          </div>
        </div>
        <div className="flex items-center gap-1 text-xs text-muted-foreground">
          <TimerIcon className="size-3.5" />
          {node.durationMs} ms
        </div>
      </div>
      {node.errorMessage ? (
        <div className="mt-3 flex items-start gap-1.5 rounded-md bg-destructive/5 px-3 py-2 text-xs text-destructive">
          <AlertTriangleIcon className="mt-0.5 size-3.5 shrink-0" />
          <span className="break-all">{node.errorMessage}</span>
        </div>
      ) : null}
      <div className="mt-3 grid gap-3 lg:grid-cols-2">
        <WorkflowPreviewBlock title={wrd(t, "labels.input")} raw={node.inputPreview} value={inputValue} />
        <WorkflowPreviewBlock title={wrd(t, "labels.output")} raw={node.outputPreview} value={outputValue} />
      </div>
    </div>
  );
}

function WorkflowPreviewBlock({
  title,
  raw,
  value,
}: {
  title: string;
  raw: string;
  value: unknown;
}) {
  return (
    <div className="min-w-0">
      <div className="mb-1 text-xs font-medium text-muted-foreground">{title}</div>
      {value !== null ? (
        <JsonTreeViewer value={value} collapsed={2} />
      ) : raw.trim() ? (
        <pre className="max-h-72 overflow-auto rounded-md border bg-muted/20 p-3 text-xs whitespace-pre-wrap break-all">
          {raw}
        </pre>
      ) : (
        <div className="rounded-md border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
          —
        </div>
      )}
    </div>
  );
}

function WorkflowRunStatusBadge({ statusName }: { statusName: string }) {
  const normalized = statusName.trim();
  const variant =
    normalized === "failed"
      ? "destructive"
      : normalized === "interrupted"
        ? "outline"
        : "secondary";
  return (
    <Badge variant={variant} className="h-5 px-1.5 text-rhd-xs">
      {normalized || "unknown"}
    </Badge>
  );
}

function safeParseJSON(raw: string): unknown | null {
  const trimmed = raw.trim();
  if (!trimmed) {
    return null;
  }
  try {
    return JSON.parse(trimmed) as unknown;
  } catch {
    return null;
  }
}
