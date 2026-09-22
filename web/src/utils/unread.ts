/** 未读数封顶 99+（ADR-006：微信式角标） */
export function formatUnread(n: number): string {
  return n > 99 ? '99+' : String(n)
}
