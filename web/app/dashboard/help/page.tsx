"use client"

import Link from "next/link"
import {
  BookOpenIcon,
  LifeBuoyIcon,
  MessageSquareTextIcon,
  ShieldCheckIcon,
  WrenchIcon,
} from "lucide-react"

import { DashboardPage, DashboardToolbar } from "@/components/dashboard-page"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useI18n } from "@/i18n/provider"

const helpSections = [
  {
    key: "usage",
    icon: BookOpenIcon,
    titleKey: "help.step1",
    links: [
      { href: "/dashboard/products", labelKey: "nav.productCenter" },
      { href: "/dashboard/tickets", labelKey: "nav.tickets" },
      { href: "/dashboard/conversation-monitor", labelKey: "nav.conversationMonitor" },
    ],
  },
  {
    key: "access",
    icon: ShieldCheckIcon,
    titleKey: "help.step2",
    links: [
      { href: "/dashboard/users", labelKey: "nav.users" },
      { href: "/dashboard/roles", labelKey: "nav.roles" },
      { href: "/dashboard/channels", labelKey: "nav.channels" },
    ],
  },
  {
    key: "knowledge",
    icon: BookOpenIcon,
    titleKey: "help.step3",
    links: [
      { href: "/dashboard/knowledge", labelKey: "nav.knowledge" },
      { href: "/dashboard/ai-agents", labelKey: "nav.aiAgents" },
      { href: "/dashboard/skill-definition", labelKey: "nav.skillDefinition" },
    ],
  },
]

const supportChecks = [
  { icon: MessageSquareTextIcon, labelKey: "help.checkConversation" },
  { icon: WrenchIcon, labelKey: "help.checkDevice" },
  { icon: LifeBuoyIcon, labelKey: "help.checkPermission" },
]

export default function DashboardHelpPage() {
  const t = useI18n()

  return (
    <DashboardPage>
      <DashboardToolbar
        actions={
          <Button variant="outline" render={<Link href="/dashboard/conversation-monitor" />}>
            <MessageSquareTextIcon />
            {t("nav.conversationMonitor")}
          </Button>
        }
      >
        <div>
          <h1 className="text-xl font-semibold">{t("help.title")}</h1>
        </div>
      </DashboardToolbar>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div className="grid gap-4 md:grid-cols-3">
          {helpSections.map((section) => {
            const Icon = section.icon
            return (
              <Card key={section.key} className="rounded-md shadow-none">
                <CardHeader className="p-4 pb-2">
                  <CardTitle className="flex items-center gap-2 text-base">
                    <span className="grid size-8 place-items-center rounded-md bg-primary/10 text-primary">
                      <Icon className="size-4" />
                    </span>
                    {t(section.titleKey)}
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-2 p-4 pt-2">
                  {section.links.map((item) => (
                    <Button key={item.href} variant="outline" className="w-full justify-between" render={<Link href={item.href} />}>
                      {t(item.labelKey)}
                    </Button>
                  ))}
                </CardContent>
              </Card>
            )
          })}
        </div>

        <Card className="rounded-md shadow-none">
          <CardHeader className="p-4 pb-2">
            <CardTitle className="text-base">{t("help.checkTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 p-4 pt-2">
            {supportChecks.map((item) => {
              const Icon = item.icon
              return (
                <div key={item.labelKey} className="flex items-center gap-3 rounded-md border border-border px-3 py-2.5">
                  <Icon className="size-4 text-muted-foreground" />
                  <span className="text-sm">{t(item.labelKey)}</span>
                </div>
              )
            })}
            <Badge variant="secondary" className="rounded-md">{t("help.badge")}</Badge>
          </CardContent>
        </Card>
      </div>
    </DashboardPage>
  )
}
