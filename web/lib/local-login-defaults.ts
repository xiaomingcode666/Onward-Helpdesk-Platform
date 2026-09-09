import type { LoginPortal } from "@/lib/login-portals"

export type LocalLoginDefault = {
  username: string
  password: string
}

function temporaryDefault(username: string, password: string | undefined): LocalLoginDefault {
  return { username, password: password ?? "" }
}

// Temporary acceptance shortcut. Remove this module and its build variables before public launch.
export const LOCAL_LOGIN_DEFAULTS: Record<LoginPortal, LocalLoginDefault> = {
  platform: temporaryDefault(process.env.NEXT_PUBLIC_LOCAL_LOGIN_PLATFORM_USERNAME ?? "admin", process.env.NEXT_PUBLIC_LOCAL_LOGIN_PLATFORM_PASSWORD),
  enterprise: temporaryDefault(process.env.NEXT_PUBLIC_LOCAL_LOGIN_ENTERPRISE_USERNAME ?? "e2e.product1.engineer", process.env.NEXT_PUBLIC_LOCAL_LOGIN_ENTERPRISE_PASSWORD),
  partner: temporaryDefault(process.env.NEXT_PUBLIC_LOCAL_LOGIN_PARTNER_USERNAME ?? "e2e.product1.partner", process.env.NEXT_PUBLIC_LOCAL_LOGIN_PARTNER_PASSWORD),
  customer: temporaryDefault(process.env.NEXT_PUBLIC_LOCAL_LOGIN_CUSTOMER_USERNAME ?? "e2e.t1.customer.real", process.env.NEXT_PUBLIC_LOCAL_LOGIN_CUSTOMER_PASSWORD),
}
