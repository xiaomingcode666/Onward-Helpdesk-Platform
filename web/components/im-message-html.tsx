"use client"

import { memo, useEffect, useRef } from "react"

import { useImageLightboxOptional } from "@/components/image-lightbox"
import { translateCurrentMessage } from "@/i18n/messages"
import { buildImMediaURL, fetchImMediaAsset } from "@/lib/api/im"

type ImMessageHTMLProps = {
  html: string
  className?: string
  onImageSettled?: () => void
  onImageClick?: (src: string, alt?: string) => void
}

function ImMessageHTMLComponent({
  html,
  className = "",
  onImageSettled,
  onImageClick,
}: ImMessageHTMLProps) {
  const lightbox = useImageLightboxOptional()
  const containerRef = useRef<HTMLDivElement>(null)
  const onImageSettledRef = useRef(onImageSettled)
  const onImageClickRef = useRef(onImageClick ?? lightbox?.open)

  useEffect(() => {
    onImageSettledRef.current = onImageSettled
  }, [onImageSettled])

  useEffect(() => {
    onImageClickRef.current = onImageClick ?? lightbox?.open
  }, [lightbox, onImageClick])

  useEffect(() => {
    const container = containerRef.current
    if (!container) {
      return
    }
    const objectUrls = new Set<string>()
    const abortControllers: AbortController[] = []
    const mediaCleanups: Array<() => void> = []
    const images = Array.from(container.querySelectorAll("img"))
    const cleanups = images.map((image) => {
      const handleSettled = () => onImageSettledRef.current?.()
      const handleClick = () => {
        const src = image.getAttribute("src")
        if (src) {
          const alt = image.getAttribute("alt") ?? undefined
          if (onImageClickRef.current) onImageClickRef.current(src, alt)
          else window.open(src, "_blank", "noopener,noreferrer")
        }
      }
      const handleKeyDown = (event: KeyboardEvent) => {
        if (event.key !== "Enter" && event.key !== " ") return
        event.preventDefault()
        handleClick()
      }

      image.addEventListener("load", handleSettled)
      image.addEventListener("error", handleSettled)
      image.addEventListener("click", handleClick)
      image.addEventListener("keydown", handleKeyDown)

      if (image.complete && image.getAttribute("src")) {
        onImageSettledRef.current?.()
      }

      image.classList.add("max-w-full", "cursor-zoom-in")
      image.setAttribute("role", "button")
      image.setAttribute("tabindex", "0")
      image.setAttribute(
        "aria-label",
        image.getAttribute("alt")?.trim() || translateCurrentMessage("lightbox.previewImage"),
      )

      return () => {
        image.removeEventListener("load", handleSettled)
        image.removeEventListener("error", handleSettled)
        image.removeEventListener("click", handleClick)
        image.removeEventListener("keydown", handleKeyDown)
      }
    })

    const mediaElements = Array.from(
      container.querySelectorAll<HTMLImageElement | HTMLAudioElement | HTMLVideoElement>(
        "img[data-asset-id], audio[data-asset-id], video[data-asset-id]",
      ),
    )
    for (const media of mediaElements) {
      const assetId = media.getAttribute("data-asset-id")?.trim()
      if (!assetId) continue

	      const stateHost = media.closest<HTMLElement>(".im-image, .im-audio, .im-video") ?? media
	      let status = stateHost.querySelector<HTMLElement>(".im-media-status")
			let standaloneStatus = false
			if (!status && stateHost === media) {
				standaloneStatus = true
				status = document.createElement("span")
				status.className = "im-media-status"
				status.setAttribute("role", "status")
				status.setAttribute("aria-live", "polite")
				const statusSpinner = document.createElement("span")
				statusSpinner.className = "im-media-spinner"
				statusSpinner.setAttribute("aria-hidden", "true")
				const label = document.createElement("span")
				label.className = "im-media-status-label"
				label.textContent = translateCurrentMessage("supportChat.mediaLoading")
				const retryButton = document.createElement("button")
				retryButton.type = "button"
				retryButton.className = "im-media-retry hidden"
				retryButton.textContent = translateCurrentMessage("supportChat.mediaRetry")
				status.append(statusSpinner, label, retryButton)
				media.insertAdjacentElement("beforebegin", status)
				mediaCleanups.push(() => status?.remove())
			}
	      const statusLabel = status?.querySelector<HTMLElement>(".im-media-status-label")
      const spinner = status?.querySelector<HTMLElement>(".im-media-spinner")
      const retry = status?.querySelector<HTMLButtonElement>(".im-media-retry")
      let progress = status?.querySelector<HTMLProgressElement>(".im-media-progress")
      if (!progress && status) {
        progress = document.createElement("progress")
        progress.className = "im-media-progress"
        progress.max = 100
        progress.value = 0
        progress.setAttribute("aria-label", translateCurrentMessage("supportChat.mediaLoading"))
        status.append(progress)
      }
      let activeController: AbortController | null = null
      let activeObjectUrl = ""
      let fallbackAttempted = false
      let fallbackInFlight = false

      const setProgress = (value: number) => {
        if (!progress || !Number.isFinite(value)) return
        progress.value = Math.min(100, Math.max(0, Math.round(value)))
      }

      const updateNativeProgress = () => {
        if (!(media instanceof HTMLMediaElement) || !Number.isFinite(media.duration) || media.duration <= 0) return
        if (media.buffered.length === 0) return
        const bufferedEnd = media.buffered.end(media.buffered.length - 1)
        const percent = (bufferedEnd / media.duration) * 100
        setProgress(percent)
        if (statusLabel && percent < 100) {
          statusLabel.textContent = `${translateCurrentMessage("supportChat.mediaLoading")} ${Math.round(percent)}%`
        }
      }

      const setState = (state: "loading" | "ready" | "error") => {
        stateHost.setAttribute("data-media-state", state)
        media.setAttribute("data-media-state", state)
        media.setAttribute("aria-busy", state === "loading" ? "true" : "false")
        if (media instanceof HTMLImageElement) {
          if (stateHost === media && !standaloneStatus) {
            media.classList.toggle("im-media-image-loading", state === "loading")
            media.classList.toggle("im-media-image-error", state === "error")
          } else {
            media.classList.toggle("hidden", state !== "ready")
          }
        } else {
          // Keep native audio/video controls mounted while the asset downloads.
          // The browser can show the player shell and the progress status instead
          // of replacing the message with a blank loading state.
          media.classList.remove("hidden")
          if (media instanceof HTMLMediaElement) {
            media.controls = true
            media.preload = "metadata"
          }
        }
        if (status) {
          status.classList.toggle("hidden", state === "ready")
          if (statusLabel) {
            statusLabel.textContent = translateCurrentMessage(
              state === "error" ? "supportChat.mediaLoadFailed" : "supportChat.mediaLoading",
            )
          }
          spinner?.classList.toggle("hidden", state !== "loading")
          retry?.classList.toggle("hidden", state !== "error")
          if (progress) {
            progress.classList.toggle("hidden", state !== "loading")
            if (state === "error") progress.value = 0
          }
        }
      }

      const loadAuthenticatedBlob = async () => {
        if (fallbackInFlight) return
        fallbackAttempted = true
        fallbackInFlight = true
        activeController?.abort()
        const controller = new AbortController()
        activeController = controller
        abortControllers.push(controller)
        if (activeObjectUrl) {
          URL.revokeObjectURL(activeObjectUrl)
          objectUrls.delete(activeObjectUrl)
          activeObjectUrl = ""
        }
        setProgress(0)
        setState("loading")
        try {
          const blob = await fetchImMediaAsset(assetId, controller.signal, (loadedBytes, totalBytes) => {
            if (controller.signal.aborted || activeController !== controller || totalBytes <= 0) return
            const percent = (loadedBytes / totalBytes) * 100
            setProgress(percent)
            if (statusLabel) {
              statusLabel.textContent = `${translateCurrentMessage("supportChat.mediaLoading")} ${Math.round(percent)}%`
            }
          })
          if (controller.signal.aborted || activeController !== controller) return
          activeObjectUrl = URL.createObjectURL(blob)
          objectUrls.add(activeObjectUrl)
          media.setAttribute("src", activeObjectUrl)
          if (media instanceof HTMLMediaElement) {
            media.load()
          }
        } catch (error: unknown) {
          if (controller.signal.aborted || error instanceof DOMException && error.name === "AbortError") return
          setState("error")
          onImageSettledRef.current?.()
        } finally {
          fallbackInFlight = false
        }
      }

      const handleReady = () => {
        setProgress(100)
        setState("ready")
      }
      const hasPlayableMediaData = () => media instanceof HTMLMediaElement && media.readyState >= 2
      const handleMediaReady = () => {
        updateNativeProgress()
        if (hasPlayableMediaData()) handleReady()
      }
      const handleMediaMetadata = () => updateNativeProgress()
      const handleError = () => {
        if (!fallbackAttempted) {
          void loadAuthenticatedBlob()
          return
        }
        if (fallbackInFlight) return
        setState("error")
        onImageSettledRef.current?.()
      }
      const handleProgress = () => updateNativeProgress()
      const readyEvent = media instanceof HTMLImageElement ? "load" : null
      if (readyEvent) media.addEventListener(readyEvent, handleReady)
      media.addEventListener("error", handleError)
      if (media instanceof HTMLMediaElement) {
        media.addEventListener("progress", handleProgress)
        media.addEventListener("loadedmetadata", handleMediaMetadata)
        media.addEventListener("loadeddata", handleMediaReady)
        media.addEventListener("canplay", handleMediaReady)
        if (hasPlayableMediaData()) handleReady()
      }
      mediaCleanups.push(() => {
        if (readyEvent) media.removeEventListener(readyEvent, handleReady)
        media.removeEventListener("error", handleError)
        if (media instanceof HTMLMediaElement) {
          media.removeEventListener("progress", handleProgress)
          media.removeEventListener("loadedmetadata", handleMediaMetadata)
          media.removeEventListener("loadeddata", handleMediaReady)
          media.removeEventListener("canplay", handleMediaReady)
        }
        activeController?.abort()
      })

      const loadMedia = async () => {
        activeController?.abort()
        const controller = new AbortController()
        activeController = controller
        if (activeObjectUrl) {
          URL.revokeObjectURL(activeObjectUrl)
          objectUrls.delete(activeObjectUrl)
          activeObjectUrl = ""
        }
        fallbackAttempted = false
        setProgress(0)
        setState("loading")
        try {
          // Native media loading keeps the response streamable. The browser
          // can request metadata and byte ranges instead of waiting for a Blob.
          media.setAttribute("src", buildImMediaURL(assetId))
          if (media instanceof HTMLMediaElement) {
            media.preload = media instanceof HTMLVideoElement ? "metadata" : "auto"
            media.load()
          }
        } catch (error: unknown) {
          if (controller.signal.aborted || error instanceof DOMException && error.name === "AbortError") return
          setState("error")
          onImageSettledRef.current?.()
        }
      }
      let loadStarted = false
      let visibilityObserver: IntersectionObserver | null = null
      const startLoad = () => {
        if (loadStarted) return
        loadStarted = true
        visibilityObserver?.disconnect()
        visibilityObserver = null
        void loadMedia()
      }
      const handleRetry = () => {
        media.removeAttribute("src")
        fallbackAttempted = false
        loadStarted = false
        startLoad()
      }
      retry?.addEventListener("click", handleRetry)
      mediaCleanups.push(() => {
        retry?.removeEventListener("click", handleRetry)
        visibilityObserver?.disconnect()
      })
      if (typeof IntersectionObserver === "undefined") {
        startLoad()
      } else {
        visibilityObserver = new IntersectionObserver(
          (entries) => {
            if (entries.some((entry) => entry.isIntersecting)) startLoad()
          },
          { rootMargin: "480px 0px" },
        )
        // The media element itself is hidden until its Blob is ready. Observe
        // the visible message placeholder or the request would never start.
        visibilityObserver.observe(standaloneStatus && status ? status : stateHost)
      }
    }

    const attachmentLinks = Array.from(
      container.querySelectorAll<HTMLAnchorElement>("a.im-attachment-link[data-asset-id]"),
    )
    for (const link of attachmentLinks) {
      const handleDownload = async (event: MouseEvent) => {
        event.preventDefault()
        const assetId = link.getAttribute("data-asset-id")?.trim()
        if (!assetId || link.getAttribute("aria-busy") === "true") return
        link.setAttribute("aria-busy", "true")
        link.classList.add("opacity-60")
        const controller = new AbortController()
        abortControllers.push(controller)
        try {
          const blob = await fetchImMediaAsset(assetId, controller.signal)
          const objectUrl = URL.createObjectURL(blob)
          objectUrls.add(objectUrl)
          const download = document.createElement("a")
          download.href = objectUrl
          download.download = link.getAttribute("download") || "attachment"
          download.click()
          window.setTimeout(() => {
            URL.revokeObjectURL(objectUrl)
            objectUrls.delete(objectUrl)
          }, 1_000)
        } catch (error: unknown) {
          if (!(error instanceof DOMException && error.name === "AbortError")) {
            link.setAttribute("title", translateCurrentMessage("supportChat.mediaLoadFailed"))
          }
        } finally {
          link.setAttribute("aria-busy", "false")
          link.classList.remove("opacity-60")
        }
      }
      link.addEventListener("click", handleDownload)
      mediaCleanups.push(() => link.removeEventListener("click", handleDownload))
    }

    return () => {
      cleanups.forEach((cleanup) => cleanup())
      mediaCleanups.forEach((cleanup) => cleanup())
      abortControllers.forEach((controller) => controller.abort())
      objectUrls.forEach((objectUrl) => URL.revokeObjectURL(objectUrl))
      objectUrls.clear()
    }
  }, [html])

  return (
    <div
      ref={containerRef}
      className={`break-words text-sm [&_p]:m-0 [&_p+*]:mt-2 [&_h1]:m-0 [&_h1]:text-base [&_h1]:font-semibold [&_h1+*]:mt-2 [&_h2]:m-0 [&_h2]:text-rhd-xl [&_h2]:font-semibold [&_h2+*]:mt-2 [&_h3]:m-0 [&_h3]:font-semibold [&_h3+*]:mt-2 [&_h4]:m-0 [&_h4]:font-medium [&_h4+*]:mt-2 [&_ul]:my-2 [&_ul]:list-disc [&_ul]:pl-5 [&_ol]:my-2 [&_ol]:list-decimal [&_ol]:pl-5 [&_li]:my-1 [&_blockquote]:my-2 [&_blockquote]:border-l-2 [&_blockquote]:border-current/20 [&_blockquote]:pl-3 [&_blockquote]:opacity-90 [&_pre]:my-2 [&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:bg-black/6 [&_pre]:px-3 [&_pre]:py-2 [&_pre]:text-rhd-md [&_pre]:leading-6 [&_code]:rounded [&_code]:bg-black/6 [&_code]:px-1 [&_code]:py-0.5 [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_hr]:my-3 [&_hr]:border-current/10 [&_table]:my-2 [&_table]:w-full [&_table]:border-collapse [&_th]:border [&_th]:border-current/10 [&_th]:px-2 [&_th]:py-1 [&_th]:text-left [&_th]:font-medium [&_td]:border [&_td]:border-current/10 [&_td]:px-2 [&_td]:py-1 [&_img]:my-2 [&_img]:max-h-64 [&_img]:rounded-md [&_img]:object-contain [&_.im-media-image-loading]:h-40 [&_.im-media-image-loading]:w-56 [&_.im-media-image-loading]:animate-pulse [&_.im-media-image-loading]:cursor-wait [&_.im-media-image-loading]:bg-muted [&_.im-media-image-error]:h-20 [&_.im-media-image-error]:w-56 [&_.im-media-image-error]:bg-destructive/10 [&_.im-media-status]:flex [&_.im-media-status]:min-h-10 [&_.im-media-status]:min-w-[220px] [&_.im-media-status]:flex-wrap [&_.im-media-status]:items-center [&_.im-media-status]:justify-center [&_.im-media-status]:gap-2 [&_.im-media-status]:rounded-md [&_.im-media-status]:bg-muted [&_.im-media-status]:px-3 [&_.im-media-status]:py-2 [&_.im-media-status]:text-xs [&_.im-media-status]:text-muted-foreground [&_.im-media-retry]:rounded-md [&_.im-media-retry]:border [&_.im-media-retry]:border-current/20 [&_.im-media-retry]:px-2 [&_.im-media-retry]:py-1 [&_.im-media-retry]:font-medium [&_.im-media-retry]:text-foreground hover:[&_.im-media-retry]:bg-background/80 [&_.im-media-transfer]:flex [&_.im-media-transfer]:min-h-14 [&_.im-media-transfer]:min-w-[220px] [&_.im-media-transfer]:items-center [&_.im-media-transfer]:gap-3 [&_.im-media-transfer]:rounded-md [&_.im-media-transfer]:bg-muted/70 [&_.im-media-transfer]:px-3 [&_.im-media-transfer]:py-2 [&_.im-media-transfer]:text-xs [&_.im-media-transfer-failed]:bg-destructive/10 [&_.im-media-transfer-content]:flex [&_.im-media-transfer-content]:min-w-0 [&_.im-media-transfer-content]:flex-col [&_.im-media-transfer-content]:gap-0.5 [&_.im-media-transfer-meta]:max-w-64 [&_.im-media-transfer-meta]:truncate [&_.im-media-transfer-meta]:opacity-65 [&_.im-media-spinner]:size-4 [&_.im-media-spinner]:shrink-0 [&_.im-media-spinner]:animate-spin [&_.im-media-spinner]:rounded-full [&_.im-media-spinner]:border-2 [&_.im-media-spinner]:border-current/25 [&_.im-media-spinner]:border-t-current [&_.im-audio]:min-w-[220px] [&_.im-audio_audio]:h-10 [&_.im-audio_audio]:w-full [&_.im-video_video]:max-h-72 [&_.im-video_video]:max-w-full [&_.im-video_video]:rounded-md [&_.im-audio-meta]:mt-1 [&_.im-audio-meta]:flex [&_.im-audio-meta]:justify-between [&_.im-audio-meta]:gap-3 [&_.im-audio-meta]:text-rhd-xs [&_.im-audio-meta]:opacity-70 [&_.im-attachment]:min-w-0 [&_.im-attachment-link]:flex [&_.im-attachment-link]:min-w-0 [&_.im-attachment-link]:items-center [&_.im-attachment-link]:gap-3 [&_.im-attachment-link]:rounded-xl [&_.im-attachment-link]:no-underline [&_.im-attachment-link]:transition-colors hover:[&_.im-attachment-link]:bg-black/5 [&_.im-attachment-icon]:flex [&_.im-attachment-icon]:size-10 [&_.im-attachment-icon]:shrink-0 [&_.im-attachment-icon]:items-center [&_.im-attachment-icon]:justify-center [&_.im-attachment-icon]:rounded-xl [&_.im-attachment-icon]:bg-black/5 [&_.im-attachment-icon_svg]:size-5 [&_.im-attachment-content]:flex [&_.im-attachment-content]:min-w-0 [&_.im-attachment-content]:flex-col [&_.im-attachment-title]:truncate [&_.im-attachment-title]:font-medium [&_.im-attachment-meta]:text-xs [&_.im-attachment-meta]:opacity-70 ${className}`}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}

export const ImMessageHTML = memo(
  ImMessageHTMLComponent,
  (prevProps, nextProps) =>
    prevProps.html === nextProps.html &&
    prevProps.className === nextProps.className &&
    prevProps.onImageSettled === nextProps.onImageSettled &&
    prevProps.onImageClick === nextProps.onImageClick
)
