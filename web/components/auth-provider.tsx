"use client"

import {
  createContext,
  startTransition,
  useContext,
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react"
import { usePathname, useRouter } from "next/navigation"

import { fetchProfile, logout } from "@/lib/api/auth"
import {
  AUTH_SESSION_EXPIRED_EVENT,
  clearPortalSession,
  readSession,
  writeSession,
  type AuthSession,
} from "@/lib/auth"
import { getPortalDefaultPath, getPortalFromPath, getSessionPortal } from "@/lib/login-portals"

type AuthContextValue = {
  session: AuthSession | null
  ready: boolean
  refreshProfile: () => Promise<void>
  signOut: (destination?: string) => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

function authSessionRefreshKey(session: AuthSession) {
  return [
    session.accessToken,
    session.domainType ?? "",
    session.tenantId ?? 0,
    session.subjectType ?? "",
    session.subjectId ?? 0,
    session.partnerAccountId ?? 0,
    session.supportGrantId ?? 0,
    session.supportMode ?? "",
  ].join("|")
}

function profileMatchesSession(stored: AuthSession, profile: AuthSession) {
  if (stored.user?.id && profile.user?.id && stored.user.id !== profile.user.id) return false
  if (stored.domainType && profile.domainType && stored.domainType !== profile.domainType) return false
  if (stored.tenantId && profile.tenantId && stored.tenantId !== profile.tenantId) return false
  if (stored.subjectType && profile.subjectType && stored.subjectType !== profile.subjectType) return false
  if (stored.subjectId && profile.subjectId && stored.subjectId !== profile.subjectId) return false
  if (stored.partnerAccountId && profile.partnerAccountId && stored.partnerAccountId !== profile.partnerAccountId) return false
  return true
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const pathname = usePathname()
  const pathnameRef = useRef(pathname)
  const currentPathRef = useRef(pathname)
  const router = useRouter()
  const [session, setSession] = useState<AuthSession | null>(null)
  const [ready, setReady] = useState(false)
  const isLoginRoute =
    (pathname?.startsWith("/dashboard/login") ?? false) ||
    (pathname?.startsWith("/platform/login") ?? false) ||
    (pathname?.startsWith("/customer/login") ?? false)
  const requiresAuth =
    ((pathname?.startsWith("/dashboard") ?? false) ||
      (pathname?.startsWith("/enterprise") ?? false) ||
      (pathname?.startsWith("/partner") ?? false) ||
      (pathname?.startsWith("/customer") ?? false) ||
      (pathname?.startsWith("/platform") ?? false)) &&
    !isLoginRoute

  useEffect(() => {
    pathnameRef.current = pathname
  }, [pathname])

  useEffect(() => {
    if (typeof window === "undefined") {
      currentPathRef.current = pathname
      return
    }
    currentPathRef.current = `${window.location.pathname}${window.location.search}${window.location.hash}`
  }, [pathname])

  const loginNextPath = useCallback(() => {
    if (typeof window !== "undefined") {
      return `${window.location.pathname}${window.location.search}${window.location.hash}` || "/enterprise"
    }
    return currentPathRef.current || pathnameRef.current || "/enterprise"
  }, [])

  const loginRedirectPath = useCallback(() => {
    const next = loginNextPath()
    const portal = getPortalFromPath(next)
    const params = new URLSearchParams({ next })
    if (portal === "platform") {
      return `/platform/login?${params.toString()}`
    }
    if (portal === "customer") {
      return `/customer/login?${params.toString()}`
    }
    return `/dashboard/login?${params.toString()}`
  }, [loginNextPath])

  const refreshProfile = useCallback(async () => {
    const stored = readSession()
    if (!stored) {
      setSession(null)
      setReady(true)
      return
    }
    const storedRefreshKey = authSessionRefreshKey(stored)

    try {
      const profile = await fetchProfile()
      const latest = readSession()
      if (!latest || authSessionRefreshKey(latest) !== storedRefreshKey) {
        setReady(true)
        return
      }
      if (!profileMatchesSession(stored, profile)) {
        clearPortalSession(getSessionPortal(stored.domainType))
        setSession(null)
        if (requiresAuth) {
          startTransition(() => {
            router.replace(loginRedirectPath())
          })
        }
        return
      }
      const nextDomainType = profile.domainType || stored.domainType
      const nextTenantId = profile.tenantId || stored.tenantId
      const membershipApplies = Boolean(nextTenantId) && (nextDomainType === "enterprise" || nextDomainType === "partner")
      const nextSession: AuthSession = {
        ...stored,
        user: profile.user || stored.user,
        permissions: profile.permissions || stored.permissions || [],
        roles: profile.roles || stored.roles || [],
        accessToken: profile.accessToken || stored.accessToken,
        expiresAt: profile.expiresAt || stored.expiresAt,
        tenantId: nextTenantId,
        domainType: nextDomainType,
        subjectType: profile.subjectType || stored.subjectType,
        subjectId: profile.subjectId || stored.subjectId,
        partnerAccountId: profile.partnerAccountId || stored.partnerAccountId,
        supportGrantId: profile.supportGrantId || stored.supportGrantId,
        supportMode: profile.supportMode || stored.supportMode,
        impersonatedBy: profile.impersonatedBy || stored.impersonatedBy,
        featureFlags: profile.featureFlags || stored.featureFlags,
        locale: profile.locale || profile.user?.locale || stored.locale,
        tenantDefaultLocale: profile.tenantDefaultLocale || stored.tenantDefaultLocale,
        customerDefaultLocale: profile.customerDefaultLocale || stored.customerDefaultLocale,
        timezone: profile.timezone || profile.user?.timezone || stored.timezone,
        availablePortals: profile.availablePortals || stored.availablePortals,
        requiresPortalChoice: profile.requiresPortalChoice ?? stored.requiresPortalChoice,
        membership: membershipApplies ? (profile.membership || stored.membership) : undefined,
        tenantBranding: profile.tenantBranding || stored.tenantBranding,
        externalPortals: profile.externalPortals || stored.externalPortals,
      }
      writeSession(nextSession)
      setSession(nextSession)
    } catch (error) {
      const errorCode = (error as Error & { errorCode?: number }).errorCode
      if (errorCode === 3000 || errorCode === 3002) {
        clearPortalSession(getSessionPortal(stored.domainType))
        setSession(null)
        if (requiresAuth) {
          startTransition(() => {
            router.replace(loginRedirectPath())
          })
        }
      }
    } finally {
      setReady(true)
    }
  }, [loginRedirectPath, requiresAuth, router])

  async function signOut(destination?: string) {
    try {
      await logout()
    } finally {
      setSession(null)
      startTransition(() => {
        router.replace(destination || loginRedirectPath())
      })
    }
  }

  useEffect(() => {
    function handleAuthExpired() {
      setSession(null)
      if (requiresAuth) {
        startTransition(() => {
          router.replace(loginRedirectPath())
        })
      }
    }

    window.addEventListener(AUTH_SESSION_EXPIRED_EVENT, handleAuthExpired)
    return () => {
      window.removeEventListener(AUTH_SESSION_EXPIRED_EVENT, handleAuthExpired)
    }
  }, [loginRedirectPath, requiresAuth, router])

  useEffect(() => {
    const stored = readSession()
    setSession(stored)
    if (stored) {
      void refreshProfile()
      return
    }

    setReady(true)
    if (requiresAuth) {
      startTransition(() => {
        router.replace(loginRedirectPath())
      })
    }
  }, [loginRedirectPath, requiresAuth, refreshProfile, router])

  useEffect(() => {
    if (!requiresAuth || !ready || !session || !pathname) return
    const currentPortal = getPortalFromPath(pathname)
    const sessionPortal = getSessionPortal(session.domainType)
    if (currentPortal !== sessionPortal) {
      router.replace(getPortalDefaultPath(sessionPortal))
    }
  }, [pathname, ready, requiresAuth, router, session])

  return (
    <AuthContext.Provider value={{ session, ready, refreshProfile, signOut }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error("useAuth must be used within AuthProvider")
  }
  return ctx
}
