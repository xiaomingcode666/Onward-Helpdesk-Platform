import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const source = await readFile(new URL("./auth-provider.tsx", import.meta.url), "utf8");

test("refreshProfile preserves the stored token when the profile payload omits it", () => {
  assert.match(source, /accessToken:\s*profile\.accessToken\s*\|\|\s*stored\.accessToken/);
  assert.match(source, /expiresAt:\s*profile\.expiresAt\s*\|\|\s*stored\.expiresAt/);
});

test("refreshProfile keeps tenant presentation and language context", () => {
  assert.match(source, /tenantDefaultLocale:\s*profile\.tenantDefaultLocale\s*\|\|\s*stored\.tenantDefaultLocale/);
  assert.match(source, /customerDefaultLocale:\s*profile\.customerDefaultLocale\s*\|\|\s*stored\.customerDefaultLocale/);
  assert.match(source, /tenantBranding:\s*profile\.tenantBranding\s*\|\|\s*stored\.tenantBranding/);
  assert.match(source, /externalPortals:\s*profile\.externalPortals\s*\|\|\s*stored\.externalPortals/);
});

test("refreshProfile ignores stale responses after another role has become active", () => {
  assert.match(source, /function authSessionRefreshKey\(session: AuthSession\)/);
  assert.match(source, /const storedRefreshKey = authSessionRefreshKey\(stored\)/);
  assert.match(source, /authSessionRefreshKey\(latest\) !== storedRefreshKey/);
  assert.match(source, /permissions:\s*profile\.permissions\s*\|\|\s*stored\.permissions\s*\|\|\s*\[\]/);
  assert.match(source, /roles:\s*profile\.roles\s*\|\|\s*stored\.roles\s*\|\|\s*\[\]/);
});

test("refreshProfile only clears session for explicit auth error codes", () => {
  assert.match(source, /errorCode\s*===\s*3000\s*\|\|\s*errorCode\s*===\s*3002/);
  assert.match(source, /clearPortalSession\(getSessionPortal\(stored\.domainType\)\)/);
  assert.doesNotMatch(source, /catch\s*\([^)]*\)\s*\{\s*clearSession\(\)/);
});

test("auth redirects preserve deep-link query and hash in next", () => {
  assert.match(source, /window\.location\.pathname\}\$\{window\.location\.search\}\$\{window\.location\.hash\}/);
  assert.match(source, /const params = new URLSearchParams\(\{ next \}\)/);
  assert.doesNotMatch(source, /next=\$\{encodeURIComponent\(pathnameRef\.current/);
});
