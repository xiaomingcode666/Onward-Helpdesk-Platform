"use client"

import { MonitorIcon, WifiIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import { useI18n } from "@/i18n/provider"
import type { CustomerEntryDevice } from "@/lib/api/customer-entry"

type DeviceListProps = {
  devices?: CustomerEntryDevice[]
}

export function DeviceList({ devices = [] }: DeviceListProps) {
  const t = useI18n()

  return (
    <div className="space-y-3 p-4">
      {devices.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center py-8">
            <MonitorIcon className="size-10 text-muted-foreground" />
            <p className="mt-2 text-sm text-muted-foreground">
              {t("portalExtract.serviceCode.deviceList.empty")}
            </p>
          </CardContent>
        </Card>
      ) : (
        devices.map((device) => (
        <Card key={device.id}>
          <CardContent className="flex items-start gap-3 p-4">
            <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <MonitorIcon className="size-5" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <p className="truncate text-sm font-medium">{device.deviceNo}</p>
                <Badge
                  variant={device.status === "active" ? "default" : "secondary"}
                  className="text-rhd-2xs"
                >
                  <WifiIcon className="mr-0.5 size-2.5" />
                  {device.status}
                </Badge>
              </div>
              <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                {device.serialNo}
              </p>
              <p className="truncate text-xs text-muted-foreground">
                {device.productName} · {device.regionCode}
              </p>
            </div>
          </CardContent>
        </Card>
        ))
      )}
    </div>
  )
}
