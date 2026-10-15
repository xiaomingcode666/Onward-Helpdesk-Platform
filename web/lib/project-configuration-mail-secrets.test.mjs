import assert from "node:assert/strict"
import test from "node:test"

import { withManagedMailSecrets } from "./project-configuration-editing.ts"

function documentWithMail(mail) {
  return {
    schema_version: 2,
    tenant_id: 1,
    environment: "development",
    projects: [],
    secret_refs: ["secret://managed-smtp", "secret://managed-imap", "secret://app-key"],
    runtime: { mail, integrations: [{ provider: "app", secret_ref: "", key_ref: "secret://app-key" }] },
  }
}

const noInput = { smtpPassword: "", imapPassword: "" }

test("blank password inputs keep previously saved managed mail secrets declared", () => {
  const result = withManagedMailSecrets(documentWithMail({
    enabled: false,
    password_ref: "secret://managed-smtp",
    imap: { enabled: false, password_ref: "secret://managed-imap" },
  }), noInput)
  assert.equal(result.runtime.mail.password_ref, "secret://managed-smtp")
  assert.equal(result.runtime.mail.imap.password_ref, "secret://managed-imap")
  assert.ok(result.secret_refs.includes("secret://managed-smtp"))
  assert.ok(result.secret_refs.includes("secret://managed-imap"))
  assert.ok(result.secret_refs.includes("secret://app-key"))
})

test("unconfigured mail declares no mail secrets", () => {
  const result = withManagedMailSecrets(documentWithMail({ enabled: false, password_ref: "", imap: { enabled: false, password_ref: "" } }), noInput)
  assert.equal(result.runtime.mail.password_ref, "")
  assert.deepEqual(result.secret_refs, ["secret://app-key"])
})

test("typed passwords switch mail to managed secrets", () => {
  const result = withManagedMailSecrets(documentWithMail({ enabled: true, password_ref: "secret://old-smtp", imap: { enabled: true, password_ref: "" } }), { smtpPassword: "code-1", imapPassword: "code-2" })
  assert.equal(result.runtime.mail.password_ref, "secret://managed-smtp")
  assert.equal(result.runtime.mail.imap.password_ref, "secret://managed-imap")
  assert.ok(!result.secret_refs.includes("secret://old-smtp"))
})
