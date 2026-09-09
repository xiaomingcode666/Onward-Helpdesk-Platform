"use client"

import { useEffect, useMemo, useRef, useState } from "react"

import {
  buildDashboardCrudInitialFilters,
  type DashboardCrudFilterStateConfig,
} from "./dashboard-crud-utils"

export function useDashboardCrudFilters(
  filters: ReadonlyArray<DashboardCrudFilterStateConfig>
) {
  const defaultsKey = filters
    .map((filter) => `${filter.name}:${String(filter.defaultValue)}`)
    .join("|")
  const initialFilters = useMemo(
    () => buildDashboardCrudInitialFilters(filters),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [defaultsKey]
  )
  const [draftFilters, setDraftFilters] = useState(initialFilters)
  const [appliedFilters, setAppliedFilters] = useState(initialFilters)
  const applyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    setDraftFilters(initialFilters)
    setAppliedFilters(initialFilters)
    return () => {
      if (applyTimerRef.current) {
        clearTimeout(applyTimerRef.current)
      }
    }
  }, [initialFilters])

  function setDraftFilter(name: string, value: string | number | undefined) {
    setDraftFilters((current) => ({
      ...current,
      [name]: value,
    }))
  }

  function applyFilters() {
    setAppliedFilters(draftFilters)
  }

  function applyFilter(name: string, value: string | number | undefined) {
    setDraftFilters((current) => ({
      ...current,
      [name]: value,
    }))
    // Debounce like the ticket-center live search: only the last keystroke
    // within 320ms triggers the fetch (select/segment changes are discrete,
    // so the short delay is imperceptible there).
    if (applyTimerRef.current) {
      clearTimeout(applyTimerRef.current)
    }
    applyTimerRef.current = setTimeout(() => {
      applyTimerRef.current = null
      setAppliedFilters((current) => ({
        ...current,
        [name]: value,
      }))
    }, 320)
  }

  function resetFilters() {
    setDraftFilters(initialFilters)
    setAppliedFilters(initialFilters)
  }

  return {
    draftFilters,
    appliedFilters,
    setDraftFilter,
    setDraftFilters,
    applyFilter,
    applyFilters,
    resetFilters,
  }
}
