export type MobileMediaKind = "image" | "attachment" | "audio"

export type MobileMediaSelectionError = "empty" | "too_large" | "not_image" | null

// Keep this aligned with the production storage.maxUploadSizeMB setting.
export const MOBILE_MEDIA_MAX_BYTES = 20 * 1024 * 1024

export function validateMobileMediaSelection(
  file: Pick<File, "size" | "type">,
  kind: MobileMediaKind,
  maxBytes = MOBILE_MEDIA_MAX_BYTES,
): MobileMediaSelectionError {
  if (!Number.isFinite(file.size) || file.size <= 0) return "empty"
  if (file.size > maxBytes) return "too_large"
  if (kind === "image" && !file.type.trim().toLowerCase().startsWith("image/")) return "not_image"
  return null
}

export function formatMobileFileSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B"
  if (bytes < 1024) return `${Math.round(bytes)} B`
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`
  const megabytes = bytes / (1024 * 1024)
  return `${megabytes >= 10 ? Math.round(megabytes) : megabytes.toFixed(1)} MB`
}
