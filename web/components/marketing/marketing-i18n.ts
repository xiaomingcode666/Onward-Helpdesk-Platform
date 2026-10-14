import { translateCurrentMessage } from "@/i18n/messages"

// tm 按当前界面语言读取官网文案；在渲染时调用，切换语言后随页面重新渲染。
export function tm(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(key, values)
}
