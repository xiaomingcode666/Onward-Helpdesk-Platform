import type { BrowserContext } from "@playwright/test"

// The closed-loop suite verifies the application's collaboration lifecycle.
// Only the external Jitsi SDK/iframe is simulated; this does not verify video transport.
export async function installLocalJitsi(context: BrowserContext) {
  await context.route("https://meet.jit.si/**", (route) => route.abort("blockedbyclient"))
  await context.addInitScript(() => {
    class LocalJitsi {
      private listeners = new Map<string, Set<(...args: unknown[]) => void>>()
      private frame = document.createElement("iframe")
      private disposed = false
      private displayName: string

      constructor(_domain: string, options: Record<string, unknown>) {
        this.displayName = (options.userInfo as { displayName?: string } | undefined)?.displayName ?? ""
        this.frame.title = "Local Jitsi lifecycle fixture"
        this.frame.srcdoc = "<!doctype html><html><body>Local meeting fixture</body></html>"
        ;(options.parentNode as HTMLElement).append(this.frame)
        window.setTimeout(() => {
          if (this.disposed) return
          ;(options.onload as (() => void) | undefined)?.()
          this.emit("videoConferenceJoined", { roomName: options.roomName })
        }, 0)
      }

      private emit(event: string, payload?: unknown) {
        this.listeners.get(event)?.forEach((listener) => listener(payload))
      }

      addListener(event: string, listener: (...args: unknown[]) => void) {
        const listeners = this.listeners.get(event) ?? new Set()
        listeners.add(listener)
        this.listeners.set(event, listeners)
      }

      removeListener(event: string, listener: (...args: unknown[]) => void) {
        this.listeners.get(event)?.delete(listener)
      }

      executeCommand(command: string) {
        if (command === "hangup") {
          this.emit("videoConferenceLeft")
          this.emit("readyToClose")
        }
      }

      dispose() {
        this.disposed = true
        this.listeners.clear()
        this.frame.remove()
      }

      async isAudioMuted() { return true }
      async isVideoMuted() { return true }
      getNumberOfParticipants() { return 1 }
      getAvatarURL() { return "" }
      getDisplayName() { return this.displayName }
      getEmail() { return "" }
      getIFrame() { return this.frame }
      async isDeviceListAvailable() { return false }
      async getAvailableDevices() { return { audioInput: [], audioOutput: [], videoInput: [] } }
      async getCurrentDevices() { return {} }
      async setAudioInputDevice() { /* No real media devices in the lifecycle fixture. */ }
    }

    window.JitsiMeetExternalAPI = LocalJitsi
  })
}
