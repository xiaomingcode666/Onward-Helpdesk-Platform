/**
 * Jitsi Meet External API type declarations
 */

interface JitsiMeetExternalAPIImpl {
  dispose(): void
  executeCommand(command: string, ...args: unknown[]): void
  addListener(event: string, listener: (...args: unknown[]) => void): void
  removeListener(event: string, listener: (...args: unknown[]) => void): void
  isAudioMuted(): Promise<boolean>
  isVideoMuted(): Promise<boolean>
  getNumberOfParticipants(): number
  getAvatarURL(): string
  getDisplayName(): string
  getEmail(): string
  getIFrame(): HTMLIFrameElement
  isDeviceListAvailable(): Promise<boolean>
  getAvailableDevices(): Promise<{
    audioInput?: Array<{ deviceId: string; groupId?: string; kind: string; label: string }>
    audioOutput?: Array<{ deviceId: string; groupId?: string; kind: string; label: string }>
    videoInput?: Array<{ deviceId: string; groupId?: string; kind: string; label: string }>
  }>
  getCurrentDevices(): Promise<{
    audioInput?: { deviceId: string; groupId?: string; kind: string; label: string }
    audioOutput?: { deviceId: string; groupId?: string; kind: string; label: string }
    videoInput?: { deviceId: string; groupId?: string; kind: string; label: string }
  }>
  setAudioInputDevice(label: string, deviceId: string): Promise<void>
}

interface JitsiMeetExternalAPIConstructor {
  new (domain: string, options: Record<string, unknown>): JitsiMeetExternalAPIImpl
}

declare const JitsiMeetExternalAPI: JitsiMeetExternalAPIConstructor | undefined

interface Window {
  JitsiMeetExternalAPI?: JitsiMeetExternalAPIConstructor
}
