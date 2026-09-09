import { LoginForm } from "@/components/login-form"
import { Suspense } from "react"

export default function LoginPage() {
  return (
    <Suspense fallback={<div className="min-h-svh bg-[#f4f7fb]" />}>
      <LoginForm />
    </Suspense>
  )
}
