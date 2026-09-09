"use client"

import "@ant-design/v5-patch-for-react-19"

import { ConfigProvider } from "antd"
import enUS from "antd/locale/en_US"
import esES from "antd/locale/es_ES"
import zhCN from "antd/locale/zh_CN"
import type { ReactNode } from "react"

import { RailopsLocaleProvider, railopsTheme } from "@railops/ui"
import { useAppLocale } from "@/i18n/provider"

export function RailopsProvider({ children }: { children: ReactNode }) {
  const { locale } = useAppLocale()
  const antdLocale = locale === "en-US" ? enUS : locale === "es-ES" ? esES : zhCN

  return (
    <RailopsLocaleProvider locale={locale}>
      <ConfigProvider locale={antdLocale} theme={railopsTheme}>{children}</ConfigProvider>
    </RailopsLocaleProvider>
  )
}
