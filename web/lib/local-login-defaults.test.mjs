import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const defaultsSource = await readFile(new URL("./local-login-defaults.ts", import.meta.url), "utf8")
const formSource = await readFile(new URL("../components/login-form.tsx", import.meta.url), "utf8")
const dockerfileSource = await readFile(new URL("../../Dockerfile", import.meta.url), "utf8")
const productionDockerfileSource = await readFile(new URL("../../Dockerfile.prod", import.meta.url), "utf8")
const onePanelComposeSource = await readFile(new URL("../../deploy/1panel/docker-compose.yml", import.meta.url), "utf8")

test("temporary login defaults cover all four formal portals", () => {
  for (const portal of ["platform", "enterprise", "partner", "customer"]) {
    assert.match(defaultsSource, new RegExp(`\\b${portal}:\\s*temporaryDefault\\(`))
  }
  assert.match(formSource, /LOCAL_LOGIN_DEFAULTS\[portal\]/)
  assert.match(formSource, /LOCAL_LOGIN_DEFAULTS\.customer/)
  assert.match(formSource, /defaultValue=\{loginDefault\.username\}/)
  assert.match(formSource, /defaultValue=\{loginDefault\.password\}/)
  assert.match(formSource, /<StaffLoginForm\s+key=\{portal\}/)
})

test("temporary defaults map the formal acceptance accounts without source-controlled passwords", () => {
  assert.match(defaultsSource, /process\.env\.NEXT_PUBLIC_LOCAL_LOGIN_PLATFORM_USERNAME \?\? "admin"/)
  assert.match(defaultsSource, /process\.env\.NEXT_PUBLIC_LOCAL_LOGIN_ENTERPRISE_USERNAME \?\? "e2e\.product1\.engineer"/)
  assert.match(defaultsSource, /process\.env\.NEXT_PUBLIC_LOCAL_LOGIN_PARTNER_USERNAME \?\? "e2e\.product1\.partner"/)
  assert.match(defaultsSource, /process\.env\.NEXT_PUBLIC_LOCAL_LOGIN_CUSTOMER_USERNAME \?\? "e2e\.t1\.customer\.real"/)
  for (const portal of ["PLATFORM", "ENTERPRISE", "PARTNER", "CUSTOMER"]) {
    assert.match(defaultsSource, new RegExp(`NEXT_PUBLIC_LOCAL_LOGIN_${portal}_PASSWORD`))
  }
  assert.doesNotMatch(defaultsSource, /NEXT_PUBLIC_ENABLE_LOCAL_LOGIN_DEFAULTS/)
  assert.doesNotMatch(defaultsSource, /InitialPass|abcd@|password:\s*"[^\"]+"/)
})

test("production image builds receive every formal portal password explicitly", () => {
  const passwordVariables = [
    "NEXT_PUBLIC_LOCAL_LOGIN_PLATFORM_PASSWORD",
    "NEXT_PUBLIC_LOCAL_LOGIN_ENTERPRISE_PASSWORD",
    "NEXT_PUBLIC_LOCAL_LOGIN_PARTNER_PASSWORD",
    "NEXT_PUBLIC_LOCAL_LOGIN_CUSTOMER_PASSWORD",
  ]

  for (const variable of passwordVariables) {
    for (const source of [dockerfileSource, productionDockerfileSource]) {
      assert.match(source, new RegExp(`ARG ${variable}`))
      assert.match(source, new RegExp(`${variable}=\\$\\{${variable}\\}`))
    }
    assert.match(
      onePanelComposeSource,
      new RegExp(`${variable}: \\$\\{${variable}:\\?${variable} is required\\}`),
    )
  }
})
