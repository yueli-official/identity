import dayjs from "dayjs";
import "dayjs/locale/zh-cn";
import relativeTime from "dayjs/plugin/relativeTime";

dayjs.extend(relativeTime);
dayjs.locale("zh-cn");

export function rel(iso?: string): string {
  return iso ? dayjs(iso).fromNow() : "";
}

export function abs(iso?: string): string {
  return iso ? dayjs(iso).format("YYYY年M月D日") : "";
}
