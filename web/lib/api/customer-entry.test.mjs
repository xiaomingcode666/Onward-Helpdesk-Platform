import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import { describe, it } from "node:test"
import ts from "typescript"
import vm from "node:vm"

function plain(value) {
  return JSON.parse(JSON.stringify(value))
}

async function loadModule(requestResult) {
  const source = await readFile(
    new URL("./customer-entry.ts", import.meta.url),
    "utf8"
  )
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2017,
      module: ts.ModuleKind.CommonJS,
    },
    fileName: "customer-entry.ts",
  })
  const sandbox = {
    exports: {},
    module: { exports: {} },
    persistedSession: null,
    require(path) {
      if (path === "@/lib/api/client") {
        return {
          request: async () => {
            if (requestResult !== undefined) return requestResult
            throw new Error("request should not be called by buildCustomerEntryState")
          },
        }
      }
      if (path === "@/lib/api/im") {
        return {
          persistCustomerSession: (session) => {
            sandbox.persistedSession = session
          },
        }
      }
      if (path === "@/i18n/config") {
        return {
          readStoredLocale: () => "zh-CN",
        }
      }
      throw new Error(`Unexpected import: ${path}`)
    },
  }
  sandbox.exports = sandbox.module.exports
  vm.runInNewContext(compiled.outputText, sandbox)
  return { ...sandbox.module.exports, sandbox }
}

describe("buildCustomerEntryState", () => {
  it("returns invalid for missing or invalid service code resolution", async () => {
    const { buildCustomerEntryState } = await loadModule()

    assert.deepEqual(plain(buildCustomerEntryState(null)), {
      kind: "invalid",
      reason: "missing",
    })
    assert.deepEqual(
      plain(buildCustomerEntryState({
        valid: false,
        revoked: false,
        needRegister: false,
        serviceCode: "BAD-CODE",
      })),
      {
        kind: "invalid",
        serviceCode: "BAD-CODE",
        reason: "notFound",
      }
    )
  })

  it("returns revoked before registration or session states", async () => {
    const { buildCustomerEntryState } = await loadModule()

    assert.deepEqual(
      plain(buildCustomerEntryState({
        valid: true,
        revoked: true,
        needRegister: true,
        serviceCode: "RHD-REVOKED",
        revokedAt: "2026-07-20T10:00:00Z",
      })),
      {
        kind: "revoked",
        serviceCode: "RHD-REVOKED",
        revokedAt: "2026-07-20T10:00:00Z",
      }
    )
  })

  it("returns needRegister when service code is valid but has no bound device", async () => {
    const { buildCustomerEntryState } = await loadModule()

    assert.deepEqual(
      plain(buildCustomerEntryState({
        valid: true,
        revoked: false,
        needRegister: true,
        serviceCode: "RHD-NEW",
        product: {
          id: 11,
          name: "Atlas Compressor",
        },
      })),
      {
        kind: "needRegister",
        serviceCode: "RHD-NEW",
        product: {
          id: 11,
          name: "Atlas Compressor",
        },
      }
    )
  })

	it("requires account binding with resolved service context", async () => {
    const { buildCustomerEntryState } = await loadModule()

    assert.deepEqual(
      plain(buildCustomerEntryState({
        valid: true,
        revoked: false,
        needRegister: false,
        serviceCode: "RHD-READY",
        entrySession: {
          id: 91,
          token: "guest-token",
          status: "active",
        },
        product: {
          id: 21,
          name: "Solar Inverter",
        },
        device: {
          id: 31,
          deviceNo: "INV-00031",
          serialNo: "SN-31",
          modelName: "S-900",
        },
      })),
      {
		kind: "needRegister",
		serviceCode: "RHD-READY",
        product: {
          id: 21,
          name: "Solar Inverter",
        },
        device: {
          id: 31,
          deviceNo: "INV-00031",
          serialNo: "SN-31",
          modelName: "S-900",
        },
      }
    )
  })
})

it("persists the exchanged entry session for authenticated media URLs", async () => {
  const session = {
    customerSessionToken: "entry-media-token",
    expiresAt: "2099-01-01T00:00:00Z",
    identityKey: "entry-identity",
    customer: { id: 9, name: "Medical Customer" },
  }
  const { exchangeCustomerEntrySession, sandbox } = await loadModule(session)

  const result = await exchangeCustomerEntrySession({
    entrySessionId: 12,
    visitorId: "visitor-1",
    visitorToken: "visitor-token",
  })

  assert.deepEqual(plain(result), session)
  assert.deepEqual(plain(sandbox.persistedSession), session)
})
