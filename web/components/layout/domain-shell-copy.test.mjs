import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./domain-shell.tsx", import.meta.url), "utf8")
const navigationSource = await readFile(new URL("../../lib/navigation-remote-helpdesk.tsx", import.meta.url), "utf8")
const localeMessages = await Promise.all([
  "../../messages/zh-CN.json",
  "../../messages/en-US.json",
  "../../messages/es-ES.json",
].map(async (file) => JSON.parse(await readFile(new URL(file, import.meta.url), "utf8"))))

test("domain shell navigation hover copy stays concise", () => {
  assert.doesNotMatch(source, /title=\{[^}]*descriptionKey/)
  assert.doesNotMatch(source, /TooltipContent[\s\S]{0,240}descriptionKey/)
  assert.doesNotMatch(source, /text-background\/80/)
  assert.doesNotMatch(source, /max-w-xs items-start whitespace-normal/)
  assert.doesNotMatch(source, /accessDeniedDescription/)
  assert.doesNotMatch(source, /errorStates\.forbiddenDescription/)
})

test("remote navigation config keeps labels only", () => {
  assert.doesNotMatch(navigationSource, /descriptionKey/)
  assert.doesNotMatch(navigationSource, /subtitleKey/)

  for (const messages of localeMessages) {
    for (const key of Object.keys(messages.remoteGroup ?? {})) {
      assert.equal(key.endsWith("Subtitle"), false, `remoteGroup.${key} should not keep subtitle copy`)
    }

    for (const [sectionKey, section] of Object.entries(messages.remoteNav ?? {})) {
      for (const key of Object.keys(section)) {
        assert.equal(key.endsWith("Desc"), false, `remoteNav.${sectionKey}.${key} should not keep description copy`)
      }
    }

    assert.equal(
      Object.hasOwn(messages.remoteShell ?? {}, "accessDeniedDescription"),
      false,
      "remoteShell.accessDeniedDescription should not keep unused helper copy",
    )
  }
})
