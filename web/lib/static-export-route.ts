export const STATIC_EXPORT_PARAM = "__static-export__"

export function isStaticExportRouteParam(value?: string | null) {
  return value === STATIC_EXPORT_PARAM
}
