import assert from "node:assert/strict"
import { readFile } from "node:fs/promises"
import test from "node:test"

const source = await readFile(new URL("./page.tsx", import.meta.url), "utf8")

test("tenant administration never embeds a reusable default password", () => {
  assert.doesNotMatch(source, /DEFAULT_PASSWORD/)
  assert.match(source, /adminPassword:\s*""/)
  assert.match(source, /setAdminPassword\(""\)/)
  assert.match(source, /type="password"[\s\S]*?autoComplete="new-password"/)
})

test("tenant administration exposes an edit action for trial and profile updates", () => {
  assert.match(source, /updatePlatformTenant/)
  assert.match(source, /<PencilIcon \/>[\s\S]*?t\("platformExtract\.tenants\.editTenant"\)/)
  assert.match(source, /formalTenant:\s*!trialEndsAt/)
  assert.match(source, /trialEndsAt:\s*editForm\.formalTenant \? "" : editForm\.trialEndsAt/)
})

test("tenant list enterprise column shows only the tenant name", () => {
  const enterpriseColumn = source.slice(source.indexOf('dataIndex: "name"'), source.indexOf('dataIndex: "adminDisplayName"'))
  assert.match(enterpriseColumn, /\{tenant\.name\}/)
  assert.doesNotMatch(enterpriseColumn, /tenant\.code|brandName/)
})

test("tenant administrator password reset has distinct submitting, error, and completion states", () => {
  assert.match(source, /const \[passwordError, setPasswordError\] = useState\(""\)/)
  assert.match(source, /setPasswordResult\(result\)[\s\S]*?toast\.success/)
  assert.match(source, /passwordResult \? \([\s\S]*?<RailopsButton variant="primary" onClick=\{closePasswordDialog\}/)
  assert.match(source, /\{passwordError \? \([\s\S]*?role="alert"/)
  assert.doesNotMatch(source, /setError\(err instanceof Error \? err\.message : t\("platformExtract\.tenants\.adminPasswordReset"\)\)/)
})

test("tenant editing is grouped into functional tabs without splitting form state", () => {
  assert.match(source, /type TenantEditTab = "profile" \| "service" \| "branding" \| "localization"/)
  assert.match(source, /<UnderlineTabs[\s\S]*?value=\{editActiveTab\}/)
  for (const tab of ["profile", "service", "branding", "localization"]) {
    assert.match(source, new RegExp(`editActiveTab === "${tab}"`))
  }
  assert.match(source, /setEditActiveTab\("localization"\)[\s\S]*?submitLocaleTimezoneRequired/)
  assert.match(source, /form="edit-tenant-form"/)
})

test("tenant and customer default languages are configured independently", () => {
  assert.match(source, /defaultLocale:\s*editForm\.defaultLocale/)
  assert.match(source, /customerDefaultLocale:\s*editForm\.customerDefaultLocale/)
  assert.match(source, /platformExtract\.tenants\.defaultLanguage/)
  assert.match(source, /platformExtract\.tenants\.customerDefaultLanguage/)
  assert.match(source, /withLanguageDefaults\(editForm\.defaultLocale, editForm\.customerDefaultLocale/)
})

test("tenant branding uses a persistent logo upload instead of a path input", () => {
  assert.match(source, /uploadPlatformTenantLogo\(file\)/)
  assert.match(source, /logoAssetId:\s*result\.assetId/)
  assert.match(source, /accept="image\/png,image\/jpeg,image\/webp"/)
  assert.doesNotMatch(source, /placeholder="\/images\/tenants\/example\/logo\.png"/)
})

test("tenant branding offers selectable customer portal theme previews", () => {
  assert.match(source, /function TenantCustomerThemePicker/)
  assert.match(source, /rhd-tenant-theme-preview-compact/)
  assert.match(source, /onClick=\{\(\) => onChange\(option\.value\)\}/)
  assert.match(source, /setPreviewTheme\(option\.value\)/)
  assert.match(source, /customerThemePreviewAction/)
  assert.match(source, /<TenantThemePreview[\s\S]*?expanded/)
  assert.match(source, /role="radiogroup"/)
  assert.match(source, /role="radio"/)
  assert.match(source, /aria-checked=\{selected\}/)
  for (const theme of ["default", "odt-intelligence", "clinical-calm", "signal-coral"]) {
    assert.match(source, new RegExp(`value: "${theme}"`))
  }
  assert.doesNotMatch(source, /const customerThemeOptions =/)
})

test("server console details are hidden while the enterprise entry is disabled", () => {
  assert.match(source, /\{value\.serverConsoleEnabled \? \([\s\S]*?serverConsoleName[\s\S]*?serverConsoleMode[\s\S]*?serverConsoleUrl[\s\S]*?\) : null\}/)
})

test("tenant edit modal keeps its frame fixed while the form body scrolls", () => {
  const editModal = source.slice(source.indexOf('rootClassName="rhd-platform-tenant-edit-modal"'), source.indexOf('open={Boolean(passwordTenant)}'))
  assert.match(editModal, /rootClassName="rhd-platform-tenant-edit-modal"/)
  assert.doesNotMatch(editModal, /styles=\{\{ body: \{ maxHeight: "90vh"/)
})

test("tenant row actions keep one primary command and group administrative actions", () => {
  assert.match(source, /<LogInIcon \/>[\s\S]*?t\("platformExtract\.tenants\.enterTenant"\)/)
  assert.match(source, /aria-label=\{t\("platformExtract\.tenants\.manageTenant", \{ name: tenant\.name \}\)\}/)
  assert.match(source, /<DropdownMenuItem[\s\S]*?<PencilIcon \/>[\s\S]*?t\("platformExtract\.tenants\.editTenant"\)/)
  assert.match(source, /<DropdownMenuItem[\s\S]*?<KeyRoundIcon \/>[\s\S]*?t\("platformExtract\.tenants\.resetPassword"\)/)
  assert.match(source, /variant=\{tenant\.status === 1 \? "default" : "destructive"\}/)
})

test("tenant freeze uses an in-product confirmation with impact context", () => {
  assert.doesNotMatch(source, /window\.confirm/)
  assert.match(source, /freezePlatformTenant\(statusTenant\.id, freezeReason\.trim\(\)\)/)
  assert.match(source, /<StandardModal[\s\S]*title=\{statusTenant\?\.status === 1 \? t\("platformExtract\.tenants\.unfreeze"\) : t\("platformExtract\.tenants\.freeze"\)\}/)
  assert.match(source, /aria-label=\{t\("platformExtract\.tenants\.freezeReason"\)\}/)
  assert.match(source, /statusTenant\.status !== 1 && !freezeReason\.trim\(\)/)
  assert.match(source, /t\("platformExtract\.tenants\.confirmFreeze"\)/)
  assert.match(source, /t\("platformExtract\.tenants\.confirmUnfreeze"\)/)
})
