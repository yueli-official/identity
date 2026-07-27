export interface SocialPlatform {
  key: string;
  label: string;
  icon: string;
  placeholder: string;
}

export const SOCIAL_PLATFORMS: SocialPlatform[] = [
  { key: "github", label: "GitHub", icon: "i-tabler-brand-github", placeholder: "https://github.com/用户名" },
  { key: "x", label: "X", icon: "i-tabler-brand-x", placeholder: "https://x.com/用户名" },
  { key: "weibo", label: "微博", icon: "i-tabler-brand-weibo", placeholder: "https://weibo.com/用户名" },
  { key: "zhihu", label: "知乎", icon: "i-tabler-brand-zhihu", placeholder: "https://www.zhihu.com/people/用户名" },
  { key: "bilibili", label: "Bilibili", icon: "i-tabler-brand-bilibili", placeholder: "https://space.bilibili.com/UID" },
  { key: "youtube", label: "YouTube", icon: "i-tabler-brand-youtube", placeholder: "https://youtube.com/@用户名" },
  { key: "telegram", label: "Telegram", icon: "i-tabler-brand-telegram", placeholder: "https://t.me/用户名" },
  { key: "linkedin", label: "LinkedIn", icon: "i-tabler-brand-linkedin", placeholder: "https://linkedin.com/in/用户名" },
  { key: "wechat", label: "微信", icon: "i-tabler-brand-wechat", placeholder: "微信号 或 主页链接" },
  { key: "mail", label: "邮箱", icon: "i-tabler-mail", placeholder: "you@example.com" },
  { key: "website", label: "网站", icon: "i-tabler-world", placeholder: "https://example.com" },
];

const byKey = new Map(SOCIAL_PLATFORMS.map((platform) => [platform.key, platform]));
const byLabel = new Map(SOCIAL_PLATFORMS.map((platform) => [platform.label.toLowerCase(), platform]));
const matchers: [string, string[]][] = [
  ["github", ["github"]],
  ["x", ["x.com", "twitter"]],
  ["weibo", ["weibo", "微博"]],
  ["zhihu", ["zhihu", "知乎"]],
  ["bilibili", ["bilibili", "b23.tv"]],
  ["youtube", ["youtube", "youtu.be"]],
  ["telegram", ["t.me", "telegram"]],
  ["linkedin", ["linkedin"]],
  ["wechat", ["wechat", "weixin", "微信"]],
  ["mail", ["mailto", "@"]],
];

export function socialPlatform(link: { label?: string; url?: string }): SocialPlatform {
  const exact = byLabel.get((link.label || "").trim().toLowerCase());
  if (exact) return exact;
  const source = `${link.label || ""} ${link.url || ""}`.toLowerCase();
  for (const [key, needles] of matchers) {
    if (needles.some((needle) => source.includes(needle))) {
      return byKey.get(key)!;
    }
  }
  return byKey.get("website")!;
}
