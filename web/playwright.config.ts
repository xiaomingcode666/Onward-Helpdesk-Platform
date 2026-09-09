import { defineConfig } from "@playwright/test"

export default defineConfig({
  testDir: "./e2e",
  testMatch: [
    "aftersales-closed-loop.live.spec.ts",
    "aftersales-adversarial.live.spec.ts",
    "aftersales-capability.live.spec.ts",
    "aftersales-ui-quality.live.spec.ts",
    "production-ai-routing.live.spec.ts",
    "workflow-product.live.spec.ts",
    "login-prefill.live.spec.ts",
    "partner-engineer-isolation.live.spec.ts",
    "conversation-language.live.spec.ts",
  ],
  timeout: 180_000,
  expect: { timeout: 30_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [["list"]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    ...(process.env.CI ? {} : { channel: "chrome" as const }),
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
    video: "retain-on-failure",
  },
})
