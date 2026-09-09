"use client"

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react"

import {
  DEFAULT_LOCALE,
  type AppLocale,
  configureLocale,
  normalizeLocale,
  normalizeSelectableLocale,
} from "@/i18n/config"
import { translateMessage } from "@/i18n/messages"
import { fetchPublicConfig } from "@/lib/api/config"
import { AUTH_SESSION_CHANGED_EVENT, readSession, type AuthSession } from "@/lib/auth"
import { getSessionPortal } from "@/lib/login-portals"

type LocaleContextValue = {
  locale: AppLocale
  setLocale: (locale: AppLocale) => void
  t: (key: string, values?: Record<string, string | number>) => string
}

const LocaleContext = createContext<LocaleContextValue>({
  locale: DEFAULT_LOCALE,
  setLocale: () => {},
  t: (key) => key,
})

const LOCALE_STORAGE_KEY = "remote-helpdesk-locale"

function localeStorageKey(session: AuthSession | null) {
  if (!session || session.tenantId <= 0) {
    return LOCALE_STORAGE_KEY
  }
  return `${LOCALE_STORAGE_KEY}:${getSessionPortal(session.domainType)}:${session.tenantId}`
}

function writeBrowserLocale(locale: AppLocale) {
  window.localStorage.setItem(localeStorageKey(readSession()), locale)
}

function readRouteDefaultLocale(): AppLocale | null {
  if (typeof window === "undefined") {
    return null
  }
  const pathname = window.location.pathname
  if (pathname === "/mobile" || pathname.startsWith("/mobile/") || pathname.startsWith("/c/")) {
    return "en-US"
  }
  return null
}

function readBrowserLocale(): AppLocale | null {
  if (typeof window === "undefined") {
    return null
  }
  const session = readSession()
  const storedLocale = window.localStorage.getItem(localeStorageKey(session))
  if (storedLocale) {
    return normalizeSelectableLocale(storedLocale)
  }
  if (session && getSessionPortal(session.domainType) === "customer" && session.customerDefaultLocale) {
    return normalizeLocale(session.customerDefaultLocale)
  }
  if (session?.tenantDefaultLocale) {
    return normalizeLocale(session.tenantDefaultLocale)
  }
  const accountLocale = session?.user?.locale || session?.locale
  if (accountLocale) {
    return normalizeLocale(accountLocale)
  }
  return null
}

export function AppI18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<AppLocale>(DEFAULT_LOCALE)
  const [isLocaleReady, setIsLocaleReady] = useState(false)

  const applyLocale = useCallback((nextLocale: AppLocale, persist = false) => {
    const configuredLocale = configureLocale(nextLocale)
    setLocaleState(configuredLocale)
    document.documentElement.lang = configuredLocale
    document.title = translateMessage(configuredLocale, "app.metadataTitle")
    if (persist) {
      writeBrowserLocale(configuredLocale)
    }
    setIsLocaleReady(true)
  }, [])

  useEffect(() => {
    let cancelled = false
    let settled = false

    const settleLocale = (configuredLocale: AppLocale) => {
      if (cancelled || settled) {
        return
      }
      settled = true
      window.clearTimeout(fallbackTimer)
      applyLocale(configuredLocale)
    }

    const fallbackTimer = window.setTimeout(() => {
      settleLocale(configureLocale(DEFAULT_LOCALE))
    }, 3000)

    const storedLocale = readBrowserLocale()
    if (storedLocale) {
      settleLocale(storedLocale)
      return () => {
        cancelled = true
        window.clearTimeout(fallbackTimer)
      }
    }

    const routeDefaultLocale = readRouteDefaultLocale()
    if (routeDefaultLocale) {
      settleLocale(routeDefaultLocale)
      return () => {
        cancelled = true
        window.clearTimeout(fallbackTimer)
      }
    }

    fetchPublicConfig()
      .then((config) => configureLocale(normalizeSelectableLocale(config.language)))
      .catch(() => configureLocale(DEFAULT_LOCALE))
      .then(settleLocale)

    return () => {
      cancelled = true
      window.clearTimeout(fallbackTimer)
    }
  }, [applyLocale])

  useEffect(() => {
    function handleSessionChanged() {
      const storedLocale = readBrowserLocale()
      if (storedLocale) {
        applyLocale(storedLocale)
      }
    }
    window.addEventListener(AUTH_SESSION_CHANGED_EVENT, handleSessionChanged)
    return () => {
      window.removeEventListener(AUTH_SESSION_CHANGED_EVENT, handleSessionChanged)
    }
  }, [applyLocale])

  useEffect(() => {
    document.title = translateMessage(locale, "app.metadataTitle")
  }, [locale])

  const value = useMemo<LocaleContextValue>(
    () => ({
      locale,
      t: (key, values) => translateMessage(locale, key, values),
      setLocale: (nextLocale) => applyLocale(nextLocale, true),
    }),
    [applyLocale, locale]
  )

  if (!isLocaleReady) {
    return null
  }

  return (
    <LocaleContext.Provider value={value}>
      {children}
    </LocaleContext.Provider>
  )
}

export function useAppLocale() {
  return useContext(LocaleContext)
}

export function useI18n() {
  return useContext(LocaleContext).t
}
