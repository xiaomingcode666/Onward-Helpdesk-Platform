import type { ReactNode } from "react"

type PermissionMode = "all" | "any"

type PermissionRule = string | readonly string[] | null | undefined

function toPermissionList(required: PermissionRule) {
  if (!required) {
    return []
  }
  return Array.isArray(required) ? [...required] : [required]
}

function canUsePermission(
  required: PermissionRule,
  permissions: readonly string[] | undefined,
  mode: PermissionMode = "any"
) {
  const requiredList = toPermissionList(required)
  if (requiredList.length === 0) {
    return true
  }
  if (!permissions) {
    return false
  }
  const permissionSet = new Set(permissions)
  return mode === "all"
    ? requiredList.every((permission) => permissionSet.has(permission))
    : requiredList.some((permission) => permissionSet.has(permission))
}

export function CanAccessMenu(
  requiredPermission: PermissionRule,
  permissions: readonly string[] | undefined
) {
  return canUsePermission(requiredPermission, permissions, "any")
}

export function CanUseButton(
  requiredPermission: PermissionRule,
  permissions: readonly string[] | undefined,
  mode: PermissionMode = "any"
) {
  return canUsePermission(requiredPermission, permissions, mode)
}

export function PermissionGuard({
  children,
  fallback = null,
  mode = "any",
  permissions,
  required,
}: {
  children: ReactNode
  fallback?: ReactNode
  mode?: PermissionMode
  permissions?: readonly string[]
  required?: PermissionRule
}) {
  if (!CanUseButton(required, permissions, mode)) {
    return <>{fallback}</>
  }
  return <>{children}</>
}
