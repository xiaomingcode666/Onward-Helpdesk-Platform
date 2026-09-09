"use client"

import {
  useState,
  type MouseEvent,
  type ReactNode,
} from "react"
import {
  Clock3Icon,
  MailIcon,
  PhoneIcon,
} from "lucide-react"

import { ContentModule, StatusTag } from "@railops/ui"
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { useI18n } from "@/i18n/provider"
import { translateCurrentMessage } from "@/i18n/messages"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { cn } from "@/lib/utils"

export type PersonCardData = {
  name: string
  role?: string
  avatar?: string
  onlineText?: string
  company?: string
  department?: string
  position?: string
  phone?: string
  email?: string
  note?: string
}

function initials(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return "?"
  const head = trimmed.match(/\p{Letter}/u)?.[0] ?? trimmed[0]
  return head.toUpperCase()
}

const personCardI18nPrefix = "opsComponentsExtract.personCard."
type PersonCardT = ReturnType<typeof useI18n>
const pc = (t: PersonCardT, key: string, values?: Record<string, string | number>) =>
  t(`${personCardI18nPrefix}${key}`, values)

function contactLine(t: PersonCardT, value?: string) {
  const trimmed = value?.trim()
  return trimmed || pc(t, "notSet")
}

export function formatRelativeOnlineText(value?: string | null) {
  const raw = String(value || "").trim()
  if (!raw) return translateCurrentMessage("opsComponentsExtract.personCard.onlineUnknown")

  const parsed = Date.parse(raw)
  if (!Number.isFinite(parsed)) return raw

  const minutes = Math.max(0, Math.floor((Date.now() - parsed) / 60000))
  if (minutes < 1) return translateCurrentMessage("opsComponentsExtract.personCard.onlineJustNow")
  if (minutes < 60) return translateCurrentMessage("opsComponentsExtract.personCard.onlineMinutes", { minutes })

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return translateCurrentMessage("opsComponentsExtract.personCard.onlineHours", { hours })
  return translateCurrentMessage("opsComponentsExtract.personCard.onlineDays", { days: Math.floor(hours / 24) })
}

export function PersonCard({
  person,
  trigger,
  side = "top",
  align = "start",
  className,
}: {
  person: PersonCardData
  trigger: ReactNode
  side?: "top" | "bottom" | "left" | "right"
  align?: "start" | "center" | "end"
  className?: string
}) {
  const t = useI18n()
  const [open, setOpen] = useState(false)
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <button
            type="button"
            className={cn(
              "inline-flex h-auto shrink-0 cursor-pointer items-center justify-center self-start p-0 outline-none ring-0 transition hover:bg-transparent focus-visible:ring-2 focus-visible:ring-primary/40",
              open && "rounded-md bg-primary/10 ring-2 ring-primary/20",
            )}
            aria-label={person.name}
            aria-haspopup="dialog"
            aria-expanded={open}
            onClick={(event: MouseEvent<HTMLButtonElement> & { preventBaseUIHandler?: () => void }) => {
              event.preventBaseUIHandler?.()
              setOpen((current) => !current)
            }}
          />
        }
      >
        {trigger}
      </PopoverTrigger>
      <PopoverContent
        align={align}
        side={side}
        sideOffset={10}
        className={cn("w-[22rem] overflow-hidden p-0 shadow-xl ring-1 ring-border/80", className)}
      >
        <ContentModule className="border-0 bg-card p-0 ring-0 shadow-none">
            <div className="border-b border-border bg-muted/30 px-4 py-4">
              <div className="flex items-start gap-3">
                <div className="relative shrink-0">
                  <Avatar className="size-12 ring-2 ring-background">
                    <AvatarImage src={person.avatar || ""} alt={person.name} />
                    <AvatarFallback className="bg-background text-base font-semibold text-foreground">
                      {initials(person.name)}
                    </AvatarFallback>
                  </Avatar>
                  <span className="absolute -bottom-0.5 -right-0.5 size-3.5 rounded-full border-2 border-background bg-primary" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 items-center gap-2">
                    <div className="truncate text-base font-semibold leading-6 text-foreground">{person.name}</div>
                    {person.role ? (
                      <StatusTag tone="neutral" className="h-5 shrink-0 border-primary/20 bg-primary/10 px-1.5 text-rhd-xs font-medium text-primary">
                        {person.role}
                      </StatusTag>
                    ) : null}
                  </div>
                  <div className="mt-1 flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
                    <Clock3Icon className="size-3.5 shrink-0 text-primary" />
                    <span className="truncate">{person.onlineText || pc(t, "onlineUnknown")}</span>
                  </div>
                  {person.note ? (
                    <div className="mt-2 line-clamp-2 text-xs leading-5 text-muted-foreground">{person.note}</div>
                  ) : null}
                </div>
              </div>
            </div>

            <div className="space-y-3 px-4 py-4">
              <div className="grid grid-cols-2 gap-2 border-t border-border pt-3 text-xs leading-5">
                <div className="min-w-0 rounded-md bg-muted/40 px-2.5 py-2">
                  <div className="flex items-center gap-1.5 text-muted-foreground">
                    <PhoneIcon className="size-3.5" />
                    <span>{pc(t, "phone")}</span>
                  </div>
                  <div className="mt-1 truncate font-medium text-foreground">{contactLine(t, person.phone)}</div>
                </div>
                <div className="min-w-0 rounded-md bg-muted/40 px-2.5 py-2">
                  <div className="flex items-center gap-1.5 text-muted-foreground">
                    <MailIcon className="size-3.5" />
                    <span>{pc(t, "email")}</span>
                  </div>
                  <div className="mt-1 truncate font-medium text-foreground">{contactLine(t, person.email)}</div>
                </div>
              </div>
            </div>
        </ContentModule>
      </PopoverContent>
    </Popover>
  )
}
