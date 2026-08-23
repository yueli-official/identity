<script setup lang="ts">
import { PageHeader } from "@yueli/ui/admin";
import { createAccountNotifier } from "~/utils/feedback";

definePageMeta({ middleware: "auth", layout: "account" });
useSeoMeta({ title: "登录会话" });

interface SessionEntry {
  id: string;
  createdAt: string;
  lastSeen: string;
  ip: string;
  userAgent: string;
  current: boolean;
}
interface SessionPage {
  current?: SessionEntry;
  entries: SessionEntry[];
  total: number;
}

const pageSize = 20;
const currentPage = ref(1);
const { call } = useApi();
const toast = createAccountNotifier(useToast());
const { data, pending, refresh } = await useAsyncData(
  "account-session-page",
  () =>
    call<SessionPage>(
      `/api/v1/session/list?limit=${pageSize}&offset=${(currentPage.value - 1) * pageSize}`,
    ),
  { watch: [currentPage] },
);
const totalPages = computed(() => Math.max(1, Math.ceil((data.value?.total ?? 0) / pageSize)));
const rangeStart = computed(() =>
  data.value?.total ? (currentPage.value - 1) * pageSize + 1 : 0,
);
const rangeEnd = computed(() =>
  Math.min(currentPage.value * pageSize, data.value?.total ?? 0),
);

const revoking = ref("");
async function onRevoke(id: string) {
  revoking.value = id;
  try {
    await call(`/api/v1/session/${id}`, { method: "DELETE" });
    if ((data.value?.entries.length ?? 0) === 1 && currentPage.value > 1) {
      currentPage.value -= 1;
    } else {
      await refresh();
    }
  } catch (error: any) {
    toast.add({
      title: "无法退出会话",
      description: identityErrorMessage(error, {
        context: "session",
        fallback: "暂时无法退出该登录会话。",
      }),
      color: "error",
    });
  } finally {
    revoking.value = "";
  }
}

const loggingOutOthers = ref(false);
async function onLogoutOthers() {
  loggingOutOthers.value = true;
  try {
    await call("/api/v1/auth/logout-others", { method: "POST" });
    currentPage.value = 1;
    await refresh();
  } catch (error: any) {
    toast.add({
      title: "无法退出其他会话",
      description: identityErrorMessage(error, {
        context: "session",
        fallback: "请稍后重试。",
      }),
      color: "error",
    });
  } finally {
    loggingOutOthers.value = false;
  }
}

function formatDate(value: string) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN");
}

function deviceIcon(userAgent: string) {
  return /mobile|android|iphone|ipad/i.test(userAgent)
    ? "i-tabler-device-mobile"
    : "i-tabler-device-desktop";
}

function shortUserAgent(userAgent: string) {
  if (!userAgent) return "未知设备";
  const browser = /edg/i.test(userAgent)
    ? "Edge"
    : /chrome/i.test(userAgent)
      ? "Chrome"
      : /firefox/i.test(userAgent)
        ? "Firefox"
        : /safari/i.test(userAgent)
          ? "Safari"
          : "浏览器";
  const os = /windows/i.test(userAgent)
    ? "Windows"
    : /mac/i.test(userAgent)
      ? "macOS"
      : /android/i.test(userAgent)
        ? "Android"
        : /iphone|ipad/i.test(userAgent)
          ? "iOS"
          : /linux/i.test(userAgent)
            ? "Linux"
            : "";
  return os ? `${browser} · ${os}` : browser;
}

function previousPage() {
  if (currentPage.value > 1) currentPage.value -= 1;
}

function nextPage() {
  if (currentPage.value < totalPages.value) currentPage.value += 1;
}
</script>

<template>
  <div class="space-y-5 sm:space-y-6">
    <PageHeader title="登录会话" icon="i-tabler-devices">
      <template v-if="data?.total" #actions>
        <UButton
          color="error"
          variant="soft"
          icon="i-tabler-logout-2"
          label="退出其他会话"
          :loading="loggingOutOthers"
          @click="onLogoutOthers"
        />
      </template>
    </PageHeader>

    <UCard>
      <template #header>
        <h2 class="font-semibold text-highlighted">当前会话</h2>
      </template>
      <div v-if="data?.current" class="flex min-w-0 items-center gap-3">
        <span class="grid size-11 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
          <UIcon :name="deviceIcon(data.current.userAgent)" class="size-5" />
        </span>
        <div class="min-w-0 flex-1">
          <p class="flex flex-wrap items-center gap-2 font-medium text-highlighted">
            <span class="truncate">{{ shortUserAgent(data.current.userAgent) }}</span>
            <UBadge color="primary" variant="soft">当前会话</UBadge>
          </p>
          <p class="mt-1 truncate text-sm text-muted">
            {{ data.current.ip || "未知 IP" }} · 最近活跃
            <ClientOnly fallback="…">{{ formatDate(data.current.lastSeen) }}</ClientOnly>
          </p>
        </div>
      </div>
      <div v-else class="py-6 text-center text-sm text-muted">当前会话信息不可用</div>
    </UCard>

    <UCard>
      <template #header>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="font-semibold text-highlighted">其他会话</h2>
          <UBadge color="neutral" variant="soft">{{ data?.total ?? 0 }} 个</UBadge>
        </div>
      </template>

      <div v-if="pending" class="space-y-3" aria-label="正在加载登录会话">
        <USkeleton v-for="item in 5" :key="item" class="h-16 w-full rounded-lg" />
      </div>
      <div
        v-else-if="!data?.entries.length"
        class="rounded-lg border border-dashed border-default px-4 py-10 text-center"
      >
        <span class="mx-auto grid size-10 place-items-center rounded-lg bg-elevated text-muted">
          <UIcon name="i-tabler-device-desktop-check" class="size-5" />
        </span>
        <p class="mt-3 text-sm font-medium text-highlighted">没有其他登录会话</p>
      </div>
      <ul v-else class="divide-y divide-default">
        <li
          v-for="session in data.entries"
          :key="session.id"
          class="flex min-w-0 items-center gap-3 py-4 first:pt-0 last:pb-0"
        >
          <span class="grid size-10 shrink-0 place-items-center rounded-lg bg-elevated text-muted">
            <UIcon :name="deviceIcon(session.userAgent)" class="size-5" />
          </span>
          <div class="min-w-0 flex-1">
            <p class="truncate font-medium text-highlighted">
              {{ shortUserAgent(session.userAgent) }}
            </p>
            <p class="mt-0.5 truncate text-sm text-muted">
              {{ session.ip || "未知 IP" }} ·
              <ClientOnly fallback="…">{{ formatDate(session.lastSeen) }}</ClientOnly>
            </p>
          </div>
          <UButton
            color="neutral"
            variant="ghost"
            icon="i-tabler-logout"
            label="退出"
            :loading="revoking === session.id"
            @click="onRevoke(session.id)"
          />
        </li>
      </ul>

      <template v-if="(data?.total ?? 0) > pageSize" #footer>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="text-sm text-muted">
            显示 {{ rangeStart }}–{{ rangeEnd }}，共 {{ data?.total }} 个会话
          </p>
          <div class="flex items-center gap-2">
            <UButton
              color="neutral"
              variant="outline"
              icon="i-tabler-chevron-left"
              label="上一页"
              :disabled="currentPage <= 1 || pending"
              @click="previousPage"
            />
            <span class="min-w-16 text-center text-sm text-muted">
              {{ currentPage }} / {{ totalPages }}
            </span>
            <UButton
              color="neutral"
              variant="outline"
              trailing-icon="i-tabler-chevron-right"
              label="下一页"
              :disabled="currentPage >= totalPages || pending"
              @click="nextPage"
            />
          </div>
        </div>
      </template>
    </UCard>
  </div>
</template>
