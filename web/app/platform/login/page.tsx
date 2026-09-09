import { LoginForm } from "@/components/login-form"
import { Suspense } from "react"

export default function PlatformLoginPage() {
  return (
    <Suspense fallback={<div className="min-h-svh bg-[var(--railops-layout-background)]" />}>
      <LoginForm forcedPortal="platform" />
    </Suspense>
  )
}
