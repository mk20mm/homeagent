/**
 * 设计令牌：对齐 docs/ui/design-system.md（苹果简约风）。
 * 禁止内联样式写颜色/间距，必须引用本文件（CONVENTIONS-frontend §5）。
 */

export const tokens = {
  color: {
    bg: '#F2F3F7', // 主背景：浅灰底
    card: '#FFFFFF', // 卡片背景
    primary: '#0A84FF', // 苹果蓝：按钮/链接
    secondary: '#8E8E93', // 灰：次要文字
    success: '#34C759', // 苹果绿：打卡成功
    warning: '#FF9500', // 橙：提醒/超支
    danger: '#FF3B30', // 红：撤销/删除
    ai: '#7C3AED', // AI 紫：对话气泡/图标
    divider: '#E5E5EA', // 极细分隔线
    text: '#000000',
    textSecondary: '#8E8E93',
  },
  radius: {
    card: '16px',
    button: '12px',
    tag: '8px',
  },
  spacing: {
    page: '16px', // 页面边距
    cardGap: '12px', // 卡片间距
    inner: '14px', // 元素内距
  },
  fontSize: {
    title: '22px',
    heading: '17px',
    body: '15px',
    caption: '12px',
  },
  fontFamily: '-apple-system, "SF Pro", "PingFang SC", "Microsoft YaHei", sans-serif',
  shadow: {
    card: '0 2px 12px rgba(0, 0, 0, 0.06)',
  },
} as const

/** 适老模式：放大字号，不改组件逻辑（CONVENTIONS-frontend §5） */
export const elderTokens = {
  fontSize: {
    title: '26px',
    heading: '21px',
    body: '19px',
    caption: '16px',
  },
} as const

export type Tokens = typeof tokens
