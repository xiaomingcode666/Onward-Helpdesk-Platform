import type { Metadata } from "next"
import Script from "next/script"

import { translateCurrentMessage } from "@/i18n/messages"
import { AuthProvider } from "@/components/auth-provider"
import { ConfirmProvider } from "@/components/confirm-provider"
import { ImageLightboxProvider } from "@/components/image-lightbox"
import { RailopsProvider } from "@/components/railops/railops-provider"
import { TooltipProvider } from "@/components/ui/tooltip"
import { Toaster } from "@/components/ui/sonner"
import { AppI18nProvider } from "@/i18n/provider"

import "@/app/globals.css"
import "@railops/ui/styles.css"
import "md-editor-rt/lib/style.css"
import "@/styles/main.scss"
import "@/styles/customer-portal-themes.css"

const paletteScript = `
try {
  const paletteKey = "remote_helpdesk_palette";
  const legacyPaletteKey = "dashboard_palette";
  window.localStorage?.setItem(paletteKey, "plain");
  window.localStorage?.removeItem(legacyPaletteKey);
  document.documentElement.dataset.palette = "plain";
} catch (_) {
  document.documentElement.dataset.palette = "plain";
}
`

export const metadata: Metadata = {
  title: translateCurrentMessage("platformExtract.layout.metadataTitle"),
  description: translateCurrentMessage("platformExtract.layout.metadataDescription"),
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode
}>) {
  return (
    <html lang="zh-CN" data-palette="plain" suppressHydrationWarning>
      <body suppressHydrationWarning>
        <Script id="dashboard-palette" strategy="beforeInteractive">
          {paletteScript}
        </Script>
        <AppI18nProvider>
          <AuthProvider>
            <ConfirmProvider>
              <ImageLightboxProvider>
                <TooltipProvider>
                  <RailopsProvider>{children}</RailopsProvider>
                  <Toaster position="top-center" richColors />
                </TooltipProvider>
              </ImageLightboxProvider>
            </ConfirmProvider>
          </AuthProvider>
        </AppI18nProvider>
      </body>
    </html>
  )
}
