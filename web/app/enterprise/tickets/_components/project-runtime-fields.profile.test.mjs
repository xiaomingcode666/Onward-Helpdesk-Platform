import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./project-runtime-fields.tsx", import.meta.url), "utf8")

test("selecting a profile copies times from the stored target configuration", () => {
  assert.match(source, /r\.targets\.find\(\(item, j\) => j !== i && item\.profile === profile\)/)
  assert.match(source, /response_minutes: preset\.response_minutes/)
  assert.match(source, /assignment_minutes: preset\.assignment_minutes/)
  assert.match(source, /resolution_minutes: preset\.resolution_minutes/)
})

test("mail credentials use the write-only managed secret state", () => {
  assert.match(source, /value=\{mailSecrets\?\.smtpPassword \?\? \"\"\}/)
  assert.match(source, /updateMailSecrets\(\{ smtpPassword: e\.target\.value \}\)/)
  assert.match(source, /value=\{mailSecrets\?\.imapPassword \?\? \"\"\}/)
  assert.match(source, /updateMailSecrets\(\{ imapPassword: e\.target\.value \}\)/)
  assert.doesNotMatch(source, /field\(pc\("smtpPassword"\), r\.mail\.password_ref/)
  assert.doesNotMatch(source, /field\(pc\("imapPassword"\), imap\.password_ref/)
})
