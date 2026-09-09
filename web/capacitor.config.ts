import type { CapacitorConfig } from "@capacitor/cli"

const config: CapacitorConfig = {
  appId: process.env.CAPACITOR_APP_ID?.trim() || "com.digintelspace.remotehelpdesk",
  appName: process.env.CAPACITOR_APP_NAME?.trim() || "RemoteHelpDesk",
  webDir: "out",
  android: {
    // Keep the WebView on a secure origin in production. API traffic still uses HTTPS.
    allowMixedContent: false,
  },
  ios: {
    contentInset: "automatic",
  },
  plugins: {
    SplashScreen: {
      launchAutoHide: true,
      backgroundColor: "#0f172a",
      showSpinner: false,
    },
    PushNotifications: {
      presentationOptions: ["badge", "sound", "banner", "list"],
    },
  },
}

export default config
