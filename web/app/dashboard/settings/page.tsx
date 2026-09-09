"use client"

import Link from "next/link"
import {
  BellIcon,
  Building2Icon,
  KeyRoundIcon,
  LockKeyholeIcon,
  MailIcon,
  ShieldCheckIcon,
  UsersRoundIcon,
} from "lucide-react"

import { DashboardPage, DashboardToolbar } from "@/components/dashboard-page"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useI18n } from "@/i18n/provider"

const settingSections = [
  {
    key: "general",
    icon: Building2Icon,
    titleKey: "settings.general.title",
    itemKeys: ["settings.general.enterpriseName", "settings.general.defaultLanguage", "settings.general.timezone"],
    href: "/dashboard/companies",
  },
  {
    key: "notifications",
    icon: BellIcon,
    titleKey: "settings.notifications.title",
    itemKeys: ["settings.notifications.websocketNotifications", "settings.notifications.emailNotifications", "settings.notifications.ticketStatus"],
    href: "/dashboard/notifications",
  },
  {
    key: "security",
    icon: ShieldCheckIcon,
    titleKey: "settings.security.title",
    itemKeys: ["nav.users", "nav.roles", "settings.security.audit"],
    href: "/dashboard/users",
  },
]

const accessEntries = [
  { icon: UsersRoundIcon, labelKey: "settings.accessUsersAndRoles", href: "/dashboard/users" },
  { icon: MailIcon, labelKey: "nav.channels", href: "/dashboard/channels" },
  { icon: KeyRoundIcon, labelKey: "nav.aiConfigs", href: "/dashboard/ai-configs" },
]

export default function DashboardSettingsPage() {
  const t = useI18n()

  return (
    <DashboardPage>
      <DashboardToolbar
        actions={
          <Button variant="outline" render={<Link href="/dashboard/roles" />}>
            <LockKeyholeIcon />
            {t("nav.roles")}
          </Button>
        }
      >
        <div>
          <h1 className="text-xl font-semibold">{t("settings.title")}</h1>
        </div>
      </DashboardToolbar>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div className="grid gap-4 md:grid-cols-3">
          {settingSections.map((section) => {
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
                <CardContent className="space-y-3 p-4 pt-2">
                  <div className="flex flex-wrap gap-2">
                    {section.itemKeys.map((itemKey) => (
                      <Badge key={itemKey} variant="outline" className="rounded-md">
                        {t(itemKey)}
                      </Badge>
                    ))}
                  </div>
                  <Button variant="outline" className="w-full justify-between" render={<Link href={section.href} />}>
                    {t("settings.openConfig")}
                  </Button>
                </CardContent>
              </Card>
            )
          })}
        </div>

        <Card className="rounded-md shadow-none">
          <CardHeader className="p-4 pb-2">
            <CardTitle className="text-base">{t("settings.commonEntries")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 p-4 pt-2">
            {accessEntries.map((item) => {
              const Icon = item.icon
              return (
                <Button key={item.href} variant="outline" className="w-full justify-start" render={<Link href={item.href} />}>
                  <Icon className="size-4" />
                  {t(item.labelKey)}
                </Button>
              )
            })}
          </CardContent>
        </Card>
      </div>
    </DashboardPage>
  )
}
