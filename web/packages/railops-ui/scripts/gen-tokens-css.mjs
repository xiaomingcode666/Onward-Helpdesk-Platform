// 生成 src/tokens.css — 设计 token 的 CSS 出口。
// 唯一数据源: src/tokens.ts (railopsTokens / railopsPalette / fontScale)。
// 改 token 后执行: node scripts/gen-tokens-css.mjs (或 npm run tokens)
import { readFile, writeFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

const { railopsTokens, railopsPalette, fontScale } = await import(
  fileURLToPath(new URL('../src/tokens.ts', import.meta.url))
)

const P = railopsPalette

// token 名 → CSS 变量名 (仅生成 CSS 侧会用到的)
const colorVars = [
  ['primary', railopsTokens.color.primary],
  ['primary-hover', railopsTokens.color.primaryHover],
  ['primary-active', railopsTokens.color.primaryActive],
  ['primary-bg', P.blue[50]],
  ['primary-soft', railopsTokens.color.primarySoft],
  ['success', railopsTokens.color.success],
  ['success-bg', P.success[50]],
  ['warning', railopsTokens.color.warning],
  ['warning-bg', P.warning[50]],
  ['error', railopsTokens.color.error],
  ['error-bg', P.error[50]],
  ['text', railopsTokens.color.text],
  ['text-secondary', railopsTokens.color.textSecondary],
  ['text-tertiary', railopsTokens.color.textTertiary],
  ['border', railopsTokens.color.border],
  ['border-light', railopsTokens.color.borderLight],
  ['border-strong', railopsTokens.color.borderStrong],
  ['surface', railopsTokens.color.surface],
  ['surface-muted', railopsTokens.color.surfaceMuted],
  ['surface-hover', railopsTokens.color.surfaceHover],
  ['layout-background', railopsTokens.color.layoutBackground],
]

const shadowVars = [
  ['card-shadow', railopsTokens.shadow.card],
  ['header-shadow', railopsTokens.shadow.header],
  ['dropdown-shadow', railopsTokens.shadow.dropdown],
  ['modal-shadow', railopsTokens.shadow.modal],
  ['float-shadow', railopsTokens.floatShadow],
]

const fontVars = Object.entries(fontScale).map(([step, px]) => [`font-${step}`, `${px}px`])

const darkVars = [
  ['text', railopsTokens.dark.text],
  ['text-secondary', railopsTokens.dark.textSecondary],
  ['border', railopsTokens.dark.border],
  ['border-strong', railopsTokens.dark.borderStrong],
  ['surface-muted', railopsTokens.dark.surfaceMuted],
  ['surface-hover', railopsTokens.dark.surfaceHover],
  ['primary-bg', railopsTokens.dark.primaryBg],
  ['primary-soft', railopsTokens.dark.primarySoft],
  ['card-shadow', railopsTokens.dark.cardShadow],
]

const block = (vars) =>
  vars.map(([name, value]) => `  --railops-${name}: ${value};`).join('\n')

const css = `/* AUTO-GENERATED from src/tokens.ts by scripts/gen-tokens-css.mjs — DO NOT EDIT.
   改设计 token 后执行 npm run tokens 重新生成。 */

:root {
${block([...colorVars, ...shadowVars, ...fontVars])}
}

.dark {
${block(darkVars)}
}
`

const out = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'src', 'tokens.css')
await writeFile(out, css)
console.log(`tokens.css written (${colorVars.length + shadowVars.length + fontVars.length} root vars, ${darkVars.length} dark vars)`)
