"use client"

import { useState } from "react"
import { ImageIcon, PaperclipIcon, SendHorizonalIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { useAppLocale } from "@/i18n/provider"

interface Message {
  id: number
  role: "customer" | "agent" | "ai"
  content: string
  timestamp: string
}

type ChatPanelProps = {
  messages?: Message[]
  sending?: boolean
  onSend?: (content: string) => void
}

export function ChatPanel({ messages = [], sending = false, onSend }: ChatPanelProps) {
  const { locale, t } = useAppLocale()
  const [input, setInput] = useState("")

  const handleSend = () => {
    const content = input.trim()
    if (!content || !onSend) return
    onSend(content)
    setInput("")
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 space-y-3 overflow-y-auto p-4">
        {messages.length === 0 ? (
          <div className="rounded-lg border bg-muted/30 p-4 text-center text-sm text-muted-foreground">
            {t("portalExtract.serviceCode.chat.emptyMessages")}
          </div>
        ) : (
          messages.map((msg) => {
          const isCustomer = msg.role === "customer"
          return (
            <div
              key={msg.id}
              className={`flex ${isCustomer ? "justify-end" : "justify-start"}`}
            >
              <div
                className={`max-w-[85%] rounded-lg px-3 py-2 text-sm leading-6 ${
                  isCustomer
                    ? "rounded-tr-sm bg-primary text-primary-foreground"
                    : msg.role === "ai"
                      ? "rounded-tl-sm bg-muted"
                      : "rounded-tl-sm border bg-background"
                }`}
              >
                {msg.content}
                <p className="mt-1 text-right text-rhd-2xs opacity-60">
                  {new Date(msg.timestamp).toLocaleTimeString(locale, {
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </p>
              </div>
            </div>
          )
          })
        )}
      </div>
      <div className="flex items-center gap-2 border-t p-3">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="shrink-0"
          aria-label={t("portalExtract.serviceCode.chat.attachFile")}
          title={t("portalExtract.serviceCode.chat.attachFile")}
        >
          <PaperclipIcon className="size-5" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="shrink-0"
          aria-label={t("portalExtract.serviceCode.chat.attachImage")}
          title={t("portalExtract.serviceCode.chat.attachImage")}
        >
          <ImageIcon className="size-5" />
        </Button>
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={t("portalExtract.serviceCode.chat.inputPlaceholder")}
          className="flex-1"
        />
        <Button
          type="button"
          size="icon"
          className="shrink-0"
          disabled={!input.trim() || sending || !onSend}
          onClick={handleSend}
          aria-label={t("portalExtract.serviceCode.chat.send")}
          title={t("portalExtract.serviceCode.chat.send")}
        >
          <SendHorizonalIcon className="size-5" />
        </Button>
      </div>
    </div>
  )
}
