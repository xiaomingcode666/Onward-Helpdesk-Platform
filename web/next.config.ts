import type { NextConfig } from "next"
import { availableParallelism } from "node:os"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const backendBaseUrl =
  process.env.NEXT_API_BASE_URL?.trim() ||
  process.env.NEXT_PUBLIC_API_BASE_URL?.trim() ||
  "http://127.0.0.1:8083"
const productionBasePath = ""

/**
 * 构建目标拆分（见 docs/design/modules/18-product-center-knowledge-integration-remediation.md §12.1）：
 *
 * - 默认（`pnpm build` / `pnpm dev` / `pnpm start`）：SaaS 管理后台构建。不做静态导出，
 *   /enterprise/products/[productId] 等数据库动态路由可按需渲染，
 *   真实产品 ID 可以直接打开和刷新，并继续通过 Next rewrites 转发
 *   `/api/*`、`/storage/*` 到 Go 后端。
 * - 前端容器静态包（`NEXT_STATIC_EXPORT=1 pnpm build`，Docker web target
 *   使用）：`output: "export"` 生成 web/out 供 Nginx 托管。
 *   构建期无法枚举的动态路由按设计不包含在该包中。
 */
const staticExport = process.env.NEXT_STATIC_EXPORT === "1"
const configDir = dirname(fileURLToPath(import.meta.url))
const railopsUISourceDir = resolve(configDir, "packages/railops-ui/src")
const railopsUIAliases = {
  "@railops/ui/charts": join(railopsUISourceDir, "charts.ts"),
  "@railops/ui/styles.css": join(railopsUISourceDir, "styles.css"),
  "@railops/ui/tokens": join(railopsUISourceDir, "tokens-entry.ts"),
  "@railops/ui": join(railopsUISourceDir, "index.ts"),
}
const railopsUITurbopackAliases = {
  "@railops/ui/charts": "./packages/railops-ui/src/charts.ts",
  "@railops/ui/styles.css": "./packages/railops-ui/src/styles.css",
  "@railops/ui/tokens": "./packages/railops-ui/src/tokens-entry.ts",
  "@railops/ui": "./packages/railops-ui/src/index.ts",
}

export default function nextConfig(): NextConfig {
  const config: NextConfig = {
    ...(staticExport ? { output: "export" as const } : {}),
    basePath: productionBasePath,
    assetPrefix: `${productionBasePath}/`,
    allowedDevOrigins: ["127.0.0.1", "localhost"],
    trailingSlash: false,
    devIndicators: false,
    transpilePackages: ["@railops/ui"],
    typescript: { ignoreBuildErrors: true },
    turbopack: {
      // Turbopack resolves local aliases from the Next.js project root.
      resolveAlias: railopsUITurbopackAliases,
    },
    webpack(webpackConfig, { dev }) {
      if (dev) {
        // 注意：不要在此设置 devtool —— Next.js 16 dev 模式强制 eval-source-map
        //（实测会打印 improper-devtool 警告并被覆盖回默认，layout chunk 约 27MB 属正常）。
        // 大改动触发全量重编译时 webpack 默认并行占满所有核心（曾达 400% CPU），
        // 会拖死开发机。限制并行度只跑一半核心，编译稍慢但系统始终流畅。
        webpackConfig.parallelism = Math.max(2, Math.floor(availableParallelism() * 0.5))
      }
      webpackConfig.resolve ??= {}
      webpackConfig.resolve.alias ??= {}
      Object.assign(webpackConfig.resolve.alias, {
        "@railops/ui$": railopsUIAliases["@railops/ui"],
        "@railops/ui/charts$": railopsUIAliases["@railops/ui/charts"],
        "@railops/ui/styles.css$": railopsUIAliases["@railops/ui/styles.css"],
        "@railops/ui/tokens$": railopsUIAliases["@railops/ui/tokens"],
      })
      return webpackConfig
    },
  }

  if (staticExport) {
    return config
  }

  return {
    ...config,
    async rewrites() {
      return [
        {
          source: "/api/:path*",
          destination: `${backendBaseUrl}/api/:path*`,
        },
        {
          source: "/storage/:path*",
          destination: `${backendBaseUrl}/storage/:path*`,
        },
      ]
    },
  }
}
