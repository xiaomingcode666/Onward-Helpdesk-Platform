import type { ThemeConfig } from 'antd';

export const railopsPalette = {
  blue: {
    50: '#EFF6FF', 100: '#DBEAFE', 200: '#BFDBFE', 300: '#93C5FD', 400: '#60A5FA',
    500: '#3B82F6', 600: '#2563EB', 700: '#1D4ED8', 800: '#1E40AF', 900: '#1E3A8A',
  },
  success: {
    50: '#ECFDF5', 100: '#D1FAE5', 200: '#A7F3D0', 300: '#6EE7B7', 400: '#34D399',
    500: '#10B981', 600: '#059669', 700: '#047857', 800: '#065F46', 900: '#064E3B',
  },
  warning: {
    50: '#FFFBEB', 100: '#FEF3C7', 200: '#FDE68A', 300: '#FCD34D', 400: '#FBBF24',
    500: '#F59E0B', 600: '#D97706', 700: '#B45309', 800: '#92400E', 900: '#78350F',
  },
  error: {
    50: '#FEF2F2', 100: '#FEE2E2', 200: '#FECACA', 300: '#FCA5A5', 400: '#F87171',
    500: '#EF4444', 600: '#DC2626', 700: '#B91C1C', 800: '#991B1B', 900: '#7F1D1D',
  },
  neutral: {
    50: '#F8FAFC', 100: '#F1F5F9', 200: '#E2E8F0', 300: '#CBD5E1', 400: '#94A3B8',
    500: '#64748B', 600: '#475569', 700: '#334155', 800: '#1E293B', 900: '#0F172A',
  },
} as const;

// 字号全刻度 — CSS 变量(--railops-font-*)与 AntD token 的唯一来源。
// 若有新档位:加到这里,typography 的 fontSize/SM/LG/XL 已由此派生,无需再改。
export const fontScale = { '2xs': 10, xs: 11, sm: 12, md: 13, lg: 14, xl: 15, '2xl': 16, '3xl': 18, '4xl': 20, '5xl': 24 } as const;

export const railopsTokens = {
  color: {
    primary: railopsPalette.blue[500], primaryHover: railopsPalette.blue[600], primaryActive: railopsPalette.blue[700],
    success: railopsPalette.success[500], warning: railopsPalette.warning[500], error: railopsPalette.error[500], info: railopsPalette.blue[500],
    text: railopsPalette.neutral[800], textSecondary: railopsPalette.neutral[500], textTertiary: railopsPalette.neutral[400],
    textDisabled: railopsPalette.neutral[300], border: railopsPalette.neutral[200], borderLight: railopsPalette.neutral[100],
    borderStrong: '#d1d5db', surfaceHover: '#f3f4f6', primarySoft: '#dbeafe',
    layoutBackground: '#F5F7FA', surface: '#FFFFFF', surfaceMuted: railopsPalette.neutral[50],
  },
  // 深色模式覆写 — 只列与浅色不同的值(由 scripts/gen-tokens-css.mjs 生成 .dark 块)。
  // 目前 app 未启用 dark(html 写死 data-palette="plain"),此区块为脚手架预留。
  dark: {
    text: '#e5e7eb', textSecondary: '#9ca3af', border: '#374151', borderStrong: '#4b5563',
    surfaceMuted: '#111827', surfaceHover: '#1f2937', primaryBg: '#1e3a5f', primarySoft: '#1e3a5f',
    cardShadow: '0 1px 3px rgba(0, 0, 0, 0.3)',
  },
  // 字号全刻度 — CSS 变量(--railops-font-*)与 AntD token 的唯一来源。
  fontScale,
  typography: {
    fontFamily: 'Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
    fontSize: fontScale.md, fontSizeSM: fontScale.sm,
    fontSizeLG: fontScale['2xl'], fontSizeXL: fontScale['4xl'], fontSizeHeading: fontScale['5xl'],
    lineHeight: 1.5714, lineHeightSM: 1.6667, lineHeightLG: 1.5, fontWeightRegular: 400, fontWeightMedium: 500, fontWeightStrong: 600,
  },
  spacing: { 0: 0, 1: 4, 2: 8, 3: 12, 4: 16, 5: 20, 6: 24, 8: 32, 10: 40, 12: 48 } as const,
  radii: { sm: 4, control: 6, module: 8, pill: 999 } as const,
  shadow: {
    none: 'none', card: '0 1px 3px rgba(15, 23, 42, 0.04)', header: '0 2px 8px rgba(15, 23, 42, 0.06)',
    dropdown: '0 8px 24px rgba(15, 23, 42, 0.10)', modal: '0 16px 40px rgba(15, 23, 42, 0.14)', focus: '0 0 0 2px rgba(59, 130, 246, 0.14)',
  },
  layout: { sidebarWidth: 240, sidebarCollapsedWidth: 64, headerHeight: 56, pagePadding: 24, pagePaddingMobile: 16, moduleGap: 24, contentGap: 16, controlGap: 8 } as const,
  control: { heightSM: 24, height: 32, heightSearch: 36, heightLG: 40 } as const,
  // Legacy aliases keep v1 consumers source-compatible while new code uses semantic groups above.
  primary: railopsPalette.blue[500], success: railopsPalette.success[500], warning: railopsPalette.warning[500], error: railopsPalette.error[500], info: railopsPalette.blue[500],
  text: railopsPalette.neutral[800], textSecondary: railopsPalette.neutral[500], textTertiary: railopsPalette.neutral[400], border: railopsPalette.neutral[200],
  layoutBackground: '#F5F7FA', surface: '#FFFFFF', radius: 8, controlRadius: 6, controlHeight: 32, pagePadding: 24, moduleGap: 24, controlGap: 8,
  cardShadow: '0 1px 3px rgba(15, 23, 42, 0.04)', floatShadow: '0 8px 24px rgba(15, 23, 42, 0.12)',
} as const;

export const typographyTokens = railopsTokens.typography;
export const spacingTokens = railopsTokens.spacing;
export const radiusTokens = railopsTokens.radii;
export const shadowTokens = railopsTokens.shadow;
export const layoutTokens = railopsTokens.layout;

export const railopsTheme: ThemeConfig = {
  token: {
    colorPrimary: railopsTokens.color.primary,
    colorSuccess: railopsTokens.color.success,
    colorWarning: railopsTokens.color.warning,
    colorError: railopsTokens.color.error,
    colorInfo: railopsTokens.color.info,
    colorText: railopsTokens.color.text,
    colorTextSecondary: railopsTokens.color.textSecondary,
    colorTextTertiary: railopsTokens.color.textTertiary,
    colorTextQuaternary: railopsTokens.color.textDisabled,
    colorBorder: railopsTokens.color.border,
    colorBorderSecondary: railopsTokens.color.borderLight,
    colorBgLayout: railopsTokens.color.layoutBackground,
    colorBgContainer: railopsTokens.color.surface,
    colorFillAlter: railopsTokens.color.surfaceMuted,
    fontFamily: railopsTokens.typography.fontFamily,
    fontSize: railopsTokens.typography.fontSize,
    fontSizeSM: railopsTokens.typography.fontSizeSM,
    fontSizeLG: railopsTokens.typography.fontSizeLG,
    fontSizeXL: railopsTokens.typography.fontSizeXL,
    lineHeight: railopsTokens.typography.lineHeight,
    lineHeightSM: railopsTokens.typography.lineHeightSM,
    lineHeightLG: railopsTokens.typography.lineHeightLG,
    borderRadius: railopsTokens.radii.module,
    borderRadiusSM: railopsTokens.radii.control,
    borderRadiusLG: railopsTokens.radii.module,
    controlHeight: railopsTokens.control.height,
    controlHeightSM: railopsTokens.control.heightSM,
    controlHeightLG: railopsTokens.control.heightLG,
  },
  components: {
    Button: { borderRadius: railopsTokens.radii.control, controlHeight: railopsTokens.control.height, paddingInline: 12, contentFontSize: 12 },
    Input: { borderRadius: railopsTokens.radii.control, controlHeight: railopsTokens.control.height, activeShadow: railopsTokens.shadow.focus },
    Select: { borderRadius: railopsTokens.radii.control, controlHeight: railopsTokens.control.height },
    DatePicker: { borderRadius: railopsTokens.radii.control, controlHeight: railopsTokens.control.height },
    Menu: { itemHeight: 30, itemBorderRadius: railopsTokens.radii.control },
    Table: { headerBg: railopsTokens.color.surfaceMuted, headerColor: railopsTokens.color.textSecondary, rowHoverBg: '#F8FBFF', cellPaddingBlock: 8, cellPaddingInline: 12, cellFontSize: 12, headerSplitColor: 'transparent' },
    Modal: { borderRadiusLG: railopsTokens.radii.module },
    Drawer: { paddingLG: railopsTokens.spacing[6] },
    Form: { itemMarginBottom: railopsTokens.spacing[3], labelColor: railopsTokens.color.text, labelFontSize: 12 },
    Alert: { borderRadiusLG: railopsTokens.radii.control },
    Tag: { borderRadiusSM: railopsTokens.radii.pill },
  },
};
