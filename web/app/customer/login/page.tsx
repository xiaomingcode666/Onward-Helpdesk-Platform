import { LoginForm } from "@/components/login-form"
import { Suspense } from "react"

export default function CustomerLoginPage() {
  return (
    <Suspense fallback={<div className="min-h-svh bg-[var(--railops-layout-background)]" />}>
      <LoginForm forcedPortal="customer" />
    </Suspense>
  )
}
