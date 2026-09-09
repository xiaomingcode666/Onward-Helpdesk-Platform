"use client"

import { UserIcon, QrCodeIcon, ShieldCheckIcon, LogOutIcon } from "lucide-react"

import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { useI18n } from "@/i18n/provider"

interface ProfilePanelProps {
  serviceCode?: string
}

export function ProfilePanel({ serviceCode }: ProfilePanelProps) {
  const t = useI18n()

  return (
    <div className="space-y-3 p-4">
      <Card>
        <CardContent className="flex flex-col items-center py-6">
          <Avatar className="size-16">
            <AvatarFallback className="text-lg">
              {t("portalExtract.serviceCode.profile.initial")}
            </AvatarFallback>
          </Avatar>
          <p className="mt-3 text-lg font-medium">
            {t("portalExtract.serviceCode.profile.accountTitle")}
          </p>
          <p className="text-sm text-muted-foreground">Customer Account</p>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="divide-y">
          <div className="flex items-center justify-between py-3">
            <div className="flex items-center gap-3">
              <QrCodeIcon className="size-5 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">
                  {t("portalExtract.serviceCode.profile.serviceCode")}
                </p>
                <p className="text-xs text-muted-foreground">
                  {serviceCode || "-"}
                </p>
              </div>
            </div>
          </div>
          <div className="flex items-center justify-between py-3">
            <div className="flex items-center gap-3">
              <UserIcon className="size-5 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">
                  {t("portalExtract.serviceCode.profile.identity")}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t("portalExtract.serviceCode.profile.verifiedCustomer")}
                </p>
              </div>
            </div>
          </div>
          <div className="flex items-center justify-between py-3">
            <div className="flex items-center gap-3">
              <ShieldCheckIcon className="size-5 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">
                  {t("portalExtract.serviceCode.profile.privacy")}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t("portalExtract.serviceCode.profile.pendingConfirmation")}
                </p>
              </div>
            </div>
          </div>
        </CardContent>
      </Card>

      <Separator />

      <Button type="button" variant="outline" className="w-full text-destructive">
        <LogOutIcon className="size-4" />
        {t("portalExtract.serviceCode.profile.exit")}
      </Button>
    </div>
  )
}
