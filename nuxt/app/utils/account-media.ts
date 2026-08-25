export function accountMediaUrl(value: string | undefined, accountOrigin: string): string {
  const source = value?.trim() || "";
  if (!source || !source.startsWith("/")) return source;
  try {
    return new URL(source, accountOrigin).toString();
  } catch {
    return source;
  }
}
