"use client"

import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from "react"
import { toast } from "sonner"
import {
  Loader2Icon,
  WifiIcon,
  WifiOffIcon,
  AlertCircleIcon,
  XIcon,
} from "lucide-react"

import { cn } from "@/lib/utils"
import { useI18n } from "@/i18n/provider"
import { readSession } from "@/lib/auth"
import { createWebSocketBaseUrl } from "@/lib/api/websocket"

export type RealtimeConnectionStatusValue =
  | "connecting"
  | "connected"
  | "disconnected";

type RealtimeConnectionStatusProps = {
  status: RealtimeConnectionStatusValue;
  compact?: boolean;
};

const statusTextKey: Record<RealtimeConnectionStatusValue, string> = {
  connecting: "realtime.connecting",
  connected: "realtime.connected",
  disconnected: "realtime.disconnected",
};

const compactStatusTextKey: Record<RealtimeConnectionStatusValue, string> = {
  connecting: "realtime.compactConnecting",
  connected: "realtime.compactConnected",
  disconnected: "realtime.compactDisconnected",
};

export function RealtimeConnectionStatus({
  status,
  compact = false,
}: RealtimeConnectionStatusProps) {
  const t = useI18n();
  const toneClass =
    status === "connected"
      ? "border-primary/20 bg-primary/10 text-primary"
      : status === "connecting"
        ? "border-amber-200/80 bg-amber-50 text-foreground dark:border-amber-800/40 dark:bg-amber-950/40"
        : "border-destructive/20 bg-destructive/10 text-destructive";

  return (
    <div
      className={cn(
        compact
          ? "inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-rhd-2xs font-medium tracking-[0.01em]"
          : "inline-flex items-center gap-2 rounded-full border px-2.5 py-1 text-rhd-xs font-medium tracking-[0.02em]",
        toneClass,
        status === "connecting" && "animate-pulse"
      )}
      role="status"
      aria-label={t(statusTextKey[status])}
    >
      <span
        className={cn(
          "inline-block size-2 rounded-full",
          status === "connected"
            ? "bg-primary shadow-[0_0_0_4px_rgba(37,99,235,0.14)]"
            : status === "connecting"
              ? "bg-amber-500 shadow-[0_0_0_4px_rgba(245,158,11,0.16)]"
              : "bg-destructive shadow-[0_0_0_4px_rgba(239,68,68,0.14)]",
        )}
      />
      <span>
        {t(compact ? compactStatusTextKey[status] : statusTextKey[status])}
      </span>
    </div>
  );
}

// ====================================================================
// Realtime Socket Hook — manages a WebSocket connection with
// exponential-backoff reconnection and toast notifications.
// ====================================================================

export type UseRealtimeSocketOptions = {
  /** Build the WebSocket URL (called when connecting/reconnecting) */
  buildUrl: () => string;
  /** Whether reconnection is allowed (e.g. auth check) */
  canReconnect?: () => boolean;
  /** Called on each incoming message */
  onMessage?: (event: MessageEvent) => void;
  /** Called when a new ticket is detected (parsed from notification) */
  onNewTicket?: (ticketId: number, ticketNo: string) => void;
  /** Called when connection status changes */
  onStatusChange?: (status: RealtimeConnectionStatusValue) => void;
  /** Ping interval in ms (default 30s) */
  pingIntervalMs?: number;
  /** Show toast for connection errors */
  showErrorToast?: boolean;
};

export function useRealtimeSocket(options: UseRealtimeSocketOptions) {
  const [status, setStatus] = useState<RealtimeConnectionStatusValue>("disconnected");
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const reconnectAttemptRef = useRef(0);
  const connectRef = useRef<() => void>(() => undefined);
  const mountedRef = useRef(true);
  const maxReconnectDelay = 30000;
  const baseReconnectDelay = 1000;

  const canReconnect = useCallback(() => {
    return options.canReconnect?.() ?? Boolean(readSession()?.accessToken);
  }, [options]);

  const updateStatus = useCallback((newStatus: RealtimeConnectionStatusValue) => {
    if (!mountedRef.current) return;
    setStatus(newStatus);
    options.onStatusChange?.(newStatus);
  }, [options]);

  const clearTimers = useCallback(() => {
    if (reconnectTimerRef.current !== null) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    if (pingTimerRef.current !== null) {
      clearInterval(pingTimerRef.current);
      pingTimerRef.current = null;
    }
  }, []);

  const scheduleReconnect = useCallback(() => {
    if (!mountedRef.current || !canReconnect()) return;
    if (reconnectTimerRef.current !== null) return;

    const delay = Math.min(
      baseReconnectDelay * 2 ** reconnectAttemptRef.current,
      maxReconnectDelay
    );
    reconnectAttemptRef.current += 1;
    updateStatus("connecting");

    reconnectTimerRef.current = setTimeout(() => {
      reconnectTimerRef.current = null;
      if (mountedRef.current && canReconnect()) {
        connectRef.current();
      }
    }, delay);
  }, [canReconnect, updateStatus]);

  const connect = useCallback(() => {
    if (!mountedRef.current || !canReconnect()) return;

    // Close existing socket
    if (socketRef.current) {
      try { socketRef.current.close(); } catch { /* ignore */ }
      socketRef.current = null;
    }

    updateStatus("connecting");

    let ws: WebSocket;
    try {
      ws = new WebSocket(options.buildUrl());
    } catch {
      updateStatus("disconnected");
      if (options.showErrorToast) {
        toast.error("Failed to create WebSocket connection");
      }
      scheduleReconnect();
      return;
    }

    socketRef.current = ws;

    ws.addEventListener("open", () => {
      if (!mountedRef.current || socketRef.current !== ws) return;
      clearTimers();
      reconnectAttemptRef.current = 0;
      updateStatus("connected");

      // Start ping
      pingTimerRef.current = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          try { ws.send(JSON.stringify({ type: "ping" })); } catch { /* ignore */ }
        }
      }, options.pingIntervalMs ?? 30000);
    });

    ws.addEventListener("message", (event) => {
      if (socketRef.current !== ws) return;
      options.onMessage?.(event);

      // Try to parse and detect ticket notifications
      try {
        const data = JSON.parse(event.data);
        if (data.type === "notification" && data.payload) {
          const payload = data.payload;
          if (payload.biz_type === "ticket" && payload.biz_id) {
            options.onNewTicket?.(payload.biz_id, payload.biz_no || "");
            // Show toast for new ticket events
            if (payload.notification_type?.includes("assigned") || payload.notification_type?.includes("created")) {
              toast(payload.title || "New Ticket", {
                description: payload.content,
                duration: 5000,
              });
            }
          }
        }
      } catch {
        // Silent — non-JSON or unparseable messages
      }
    });

    ws.addEventListener("close", () => {
      clearTimers();
      if (socketRef.current === ws) {
        socketRef.current = null;
      }
      if (mountedRef.current && canReconnect()) {
        scheduleReconnect();
      } else {
        updateStatus("disconnected");
      }
    });

    ws.addEventListener("error", () => {
      if (socketRef.current !== ws) return;
      updateStatus("disconnected");
      if (options.showErrorToast) {
        toast.error("WebSocket connection error — retrying...");
      }
      scheduleReconnect();
    });
  }, [options, canReconnect, clearTimers, scheduleReconnect, updateStatus]);

  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

  const disconnect = useCallback(() => {
    clearTimers();
    if (socketRef.current) {
      try { socketRef.current.close(); } catch { /* ignore */ }
      socketRef.current = null;
    }
    updateStatus("disconnected");
  }, [clearTimers, updateStatus]);

  useEffect(() => {
    mountedRef.current = true;
    const connectTimer = window.setTimeout(connect, 0);

    return () => {
      window.clearTimeout(connectTimer);
      mountedRef.current = false;
      disconnect();
    };
  }, [connect, disconnect]);

  return { status, connect, disconnect };
}

// ====================================================================
// RealtimeOfflineBanner — horizontal banner shown when WebSocket
// is disconnected or the browser is offline.
// ====================================================================

type RealtimeOfflineBannerProps = {
  status: RealtimeConnectionStatusValue;
  onDismiss?: () => void;
  className?: string;
};

export function RealtimeOfflineBanner({
  status,
  onDismiss,
  className,
}: RealtimeOfflineBannerProps) {
  const isOnline = useSyncExternalStore(
    subscribeToOnlineStatus,
    () => navigator.onLine,
    () => true,
  );

  const isOffline = !isOnline;

  if (isOnline && status === "connected") return null;

  return (
    <div
      className={cn(
        "flex items-center gap-3 px-4 py-2 text-sm",
        isOffline
          ? "bg-destructive/10 text-destructive border-b border-destructive/20"
          : status === "connecting"
            ? "bg-amber-50 text-amber-700 border-b border-amber-200/80 dark:bg-amber-950/40 dark:text-amber-400 dark:border-amber-800/40"
            : "bg-destructive/10 text-destructive border-b border-destructive/20 dark:bg-destructive/15 dark:text-destructive dark:border-destructive/30",
        className
      )}
      role="alert"
    >
      {isOffline ? (
        <WifiOffIcon className="size-4 shrink-0" />
      ) : status === "connecting" ? (
        <Loader2Icon className="size-4 shrink-0 animate-spin" />
      ) : (
        <AlertCircleIcon className="size-4 shrink-0" />
      )}
      <span className="flex-1 text-xs font-medium">
        {isOffline
          ? "You are offline — some features may be unavailable."
          : status === "connecting"
            ? "Reconnecting to server..."
            : "Connection lost. Retrying automatically..."}
      </span>
      {onDismiss && (
        <button
          type="button"
          onClick={onDismiss}
          className="shrink-0 rounded p-1 hover:bg-background/50 transition-colors"
          aria-label="Dismiss"
        >
          <XIcon className="size-3.5" />
        </button>
      )}
    </div>
  );
}

function subscribeToOnlineStatus(onStoreChange: () => void) {
  window.addEventListener("online", onStoreChange);
  window.addEventListener("offline", onStoreChange);
  return () => {
    window.removeEventListener("online", onStoreChange);
    window.removeEventListener("offline", onStoreChange);
  };
}

// ====================================================================
// RealtimeProvider — a self-contained provider that manages an
// enterprise-event WebSocket connection and exposes the status via
// a React context.
// ====================================================================

import { createContext, useContext, type ReactNode } from "react";

type RealtimeContextValue = {
  status: RealtimeConnectionStatusValue;
};

const RealtimeContext = createContext<RealtimeContextValue>({
  status: "disconnected",
});

type RealtimeProviderProps = {
  children: ReactNode;
  /** Optional custom URL builder. Defaults to enterprise notifications WS */
  buildUrl?: () => string;
  /** Show error toasts */
  showErrorToast?: boolean;
  /** Disable auto-connect (connect manually) */
  autoConnect?: boolean;
};

export function RealtimeProvider({
  children,
  buildUrl,
  showErrorToast = false,
}: RealtimeProviderProps) {
  const defaultBuildUrl = useCallback(() => {
    const session = readSession();
    const baseUrl = createWebSocketBaseUrl();
    const params = new URLSearchParams();
    if (session?.accessToken) {
      params.set("accessToken", session.accessToken);
    }
    return `${baseUrl}/api/ws/enterprise/events?${params.toString()}`;
  }, []);

  const { status } = useRealtimeSocket({
    buildUrl: buildUrl ?? defaultBuildUrl,
    showErrorToast,
    canReconnect: () => Boolean(readSession()?.accessToken),
  });

  return (
    <RealtimeContext.Provider value={{ status }}>
      {children}
    </RealtimeContext.Provider>
  );
}

export function useRealtimeContext() {
  return useContext(RealtimeContext);
}
