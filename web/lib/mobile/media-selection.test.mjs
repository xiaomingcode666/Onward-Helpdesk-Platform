import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"
import vm from "node:vm"
import ts from "typescript"

async function loadModule() {
  const source = await readFile(new URL("./media-selection.ts", import.meta.url), "utf8")
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2022,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "media-selection.ts",
  })
  const sandbox = { exports: {}, module: { exports: {} } }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return sandbox.module.exports
}

test("mobile media selection rejects empty, oversized, and non-image files", async () => {
  const { MOBILE_MEDIA_MAX_BYTES, validateMobileMediaSelection } = await loadModule()

  assert.equal(validateMobileMediaSelection({ size: 0, type: "image/jpeg" }, "image"), "empty")
  assert.equal(
    validateMobileMediaSelection({ size: MOBILE_MEDIA_MAX_BYTES + 1, type: "image/jpeg" }, "image"),
    "too_large",
  )
  assert.equal(validateMobileMediaSelection({ size: 1024, type: "application/pdf" }, "image"), "not_image")
  assert.equal(validateMobileMediaSelection({ size: 1024, type: "image/heic" }, "image"), null)
  assert.equal(validateMobileMediaSelection({ size: 1024, type: "application/pdf" }, "attachment"), null)
})

test("mobile file sizes stay compact and readable", async () => {
  const { formatMobileFileSize } = await loadModule()

  assert.equal(formatMobileFileSize(0), "0 B")
  assert.equal(formatMobileFileSize(512), "512 B")
  assert.equal(formatMobileFileSize(1536), "2 KB")
  assert.equal(formatMobileFileSize(1.5 * 1024 * 1024), "1.5 MB")
  assert.equal(formatMobileFileSize(12.4 * 1024 * 1024), "12 MB")
})
