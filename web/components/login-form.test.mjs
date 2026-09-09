import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const source = await readFile(new URL("./login-form.tsx", import.meta.url), "utf8");
const showcaseSource = await readFile(new URL("./platform-login-showcase.tsx", import.meta.url), "utf8");
const localeFiles = [
  "../messages/zh-CN.json",
  "../messages/en-US.json",
  "../messages/es-ES.json",
];

test("password login refreshes the shared auth state before navigating", () => {
  assert.match(source, /const \{ session, ready, refreshProfile \} = useAuth\(\)/);
  assert.match(source, /function navigateAfterAuth\(destination: string\) \{\s*window\.location\.replace\(destination\)\s*\}/);
  assert.match(
    source,
    /await loginWithPassword\([\s\S]*?await refreshProfile\(\)[\s\S]*?navigateAfterAuth\(resolveSessionDestination/,
  );
});

test("login form uses the removable defaults module for all portal forms", () => {
  assert.match(source, /import \{ LOCAL_LOGIN_DEFAULTS \} from "@\/lib\/local-login-defaults"/);
  assert.match(source, /const loginDefault = LOCAL_LOGIN_DEFAULTS\[portal\]/);
  assert.match(source, /const loginDefault = LOCAL_LOGIN_DEFAULTS\.customer/);
  assert.match(source, /defaultValue=\{loginDefault\.username\}/);
  assert.match(source, /defaultValue=\{loginDefault\.password\}/);
});

test("login entry keeps portal copy concise", async () => {
  assert.doesNotMatch(source, /accountHintKey/);
  assert.doesNotMatch(showcaseSource, /platformVisual\.description/);
  assert.doesNotMatch(showcaseSource, /platformVisual\.aiDescription/);
  assert.doesNotMatch(showcaseSource, /BotIcon/);

  for (const key of [
    "auth.staffFormDescription",
    "auth.customer.formDescription",
    "auth.customer.accountLoginHint",
    "auth.customer.inviteRegistrationHint",
    "auth.customer.deviceRegistrationHint",
  ]) {
    assert.doesNotMatch(source, new RegExp(key.replaceAll(".", "\\.")));
  }

  assert.doesNotMatch(source, /detail=\{t\("auth\.customer\.(inviteRegistrationHint|deviceRegistrationHint)"\)\}/);

  for (const file of localeFiles) {
    const messages = JSON.parse(await readFile(new URL(file, import.meta.url), "utf8"));
    assert.equal(Object.hasOwn(messages.auth, "staffFormDescription"), false, `${file} should not keep staffFormDescription`);
    assert.equal(Object.hasOwn(messages.auth.platformVisual, "description"), false, `${file} should not keep platformVisual.description`);
    assert.equal(Object.hasOwn(messages.auth.platformVisual, "aiDescription"), false, `${file} should not keep platformVisual.aiDescription`);
    assert.equal(Object.values(messages.auth.platformVisual).some((value) => typeof value === "string" && /AI|IA|Full audit trail|GLOBAL AFTER-SALES OPERATIONS|全链路审计|完整的售后服务闭环/.test(value)), false, `${file} should keep login showcase copy compact`);
    assert.equal(Object.hasOwn(messages.auth.customer, "formDescription"), false, `${file} should not keep customer.formDescription`);
    assert.equal(Object.hasOwn(messages.auth.customer, "accountLoginHint"), false, `${file} should not keep accountLoginHint`);
    assert.equal(Object.hasOwn(messages.auth.customer, "inviteRegistrationHint"), false, `${file} should not keep inviteRegistrationHint`);
    assert.equal(Object.hasOwn(messages.auth.customer, "deviceRegistrationHint"), false, `${file} should not keep deviceRegistrationHint`);
    for (const [portal, config] of Object.entries(messages.auth.portals)) {
      assert.equal(Object.hasOwn(config, "description"), false, `${file} should not keep ${portal}.description`);
      assert.equal(Object.hasOwn(config, "accountHint"), false, `${file} should not keep ${portal}.accountHint`);
      assert.equal(Object.hasOwn(config, "highlight1"), false, `${file} should not keep ${portal}.highlight1`);
      assert.equal(Object.hasOwn(config, "highlight2"), false, `${file} should not keep ${portal}.highlight2`);
      assert.equal(Object.hasOwn(config, "highlight3"), false, `${file} should not keep ${portal}.highlight3`);
    }
  }
});

test("login page uses the RailOps industrial auth shell", async () => {
  assert.match(source, /RailOpsLoginShell/);
  assert.match(source, /RailOpsLoginCard/);
  assert.match(source, /rhd-railops-auth-shell/);
  assert.match(source, /rhd-railops-auth-card/);
  assert.match(source, /AppLogoMark/);
  assert.match(source, /login-factory-support\.jpg/);
  assert.match(source, /auth\.loginHero\.highlight/);
  assert.match(source, /rhd-railops-auth-segmented/);
  assert.doesNotMatch(source, /AatssLogoMark/);
  assert.doesNotMatch(source, /AatssLoginShell/);
  assert.doesNotMatch(source, /login-hero-latest\.webp/);
  assert.match(showcaseSource, /rhd-railops-login-showcase/);
  assert.match(showcaseSource, /serviceSignals/);

  const zhMessages = JSON.parse(await readFile(new URL("../messages/zh-CN.json", import.meta.url), "utf8"));
  assert.equal(zhMessages.remoteTopbar.brandTitle, "AI辅助技术支持平台 AATSS");
  assert.equal(zhMessages.remoteTopbar.brandSub, "AI赋能的远程诊断与专家协作平台");
  assert.equal(zhMessages.auth.loginHero.cardTitle, "欢迎登录");
  assert.equal(zhMessages.auth.loginHero.submit, "立即登录");
});

test("customer login exposes the guest device code help entry", async () => {
  assert.match(source, /GuestDeviceInfoDialog/);
  assert.match(source, /CUSTOMER_GUEST_SERVICE_CODE/);
  assert.match(source, /auth\.customer\.guestInfo/);
  assert.match(source, /auth\.customer\.guestQuestionLimit/);

  for (const file of localeFiles) {
    const messages = JSON.parse(await readFile(new URL(file, import.meta.url), "utf8"));
    assert.equal(typeof messages.auth.customer.guestDeviceTitle, "string", `${file} should define guestDeviceTitle`);
    assert.equal(typeof messages.auth.customer.guestQuestionLimit, "string", `${file} should define guestQuestionLimit`);
    assert.doesNotMatch(messages.auth.customer.guestInfo, /说明|description/i, `${file} should keep guest info label concise`);
  }
});
