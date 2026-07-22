<script setup lang="ts">
import type { DropdownMenuItem } from "@nuxt/ui";
import type {
  CollectionControl,
  CollectionControlValue,
  CollectionPanelMessages,
  CollectionPanelState,
} from "@yueli/ui/collection";
import {
  CollectionPanel,
  CollectionViewToggle,
} from "@yueli/ui/collection/pattern";
import type {
  AssetItem,
  AssetMaintenanceTask,
  AssetSite,
} from "~/types/asset-admin";

type SelectOption = { label: string; value: string };

const {
  assets,
  total,
  filterCount,
  spaceOptions,
  siteOptions,
  profileOptions,
  visibilityOptions,
  mimeOptions,
  sortOptions,
  selectedIds,
  pageSelected,
  pageIndeterminate,
  pageSizeItems,
  sites,
  state = "ready",
  errorMessage = "",
  queueing = false,
  queueError = "",
  taskId = "",
  task,
  controllingTaskId = "",
  actionsFor,
} = defineProps<{
  assets: readonly AssetItem[];
  total: number;
  filterCount: number;
  spaceOptions: SelectOption[];
  siteOptions: SelectOption[];
  profileOptions: SelectOption[];
  visibilityOptions: SelectOption[];
  mimeOptions: SelectOption[];
  sortOptions: SelectOption[];
  selectedIds: readonly string[];
  pageSelected: boolean;
  pageIndeterminate: boolean;
  pageSizeItems: Array<{ label: string; value: number }>;
  sites: AssetSite[];
  state?: CollectionPanelState;
  errorMessage?: string;
  queueing?: boolean;
  queueError?: string;
  taskId?: string;
  task?: AssetMaintenanceTask;
  controllingTaskId?: string;
  actionsFor: (asset: AssetItem) => DropdownMenuItem[][];
}>();

const emit = defineEmits<{
  search: [value: string];
  retry: [];
  toggleOne: [id: string];
  togglePage: [value: boolean];
  queueSelected: [];
  clearSelection: [];
  taskAction: [action: "pause" | "resume" | "cancel"];
  openMaintenance: [];
  dismissTask: [];
}>();

const search = defineModel<string>("search", { required: true });
const spaceKey = defineModel<string>("spaceKey", { required: true });
const siteKey = defineModel<string>("siteKey", { required: true });
const profileKey = defineModel<string>("profileKey", { required: true });
const visibility = defineModel<string>("visibility", { required: true });
const mime = defineModel<string>("mime", { required: true });
const sort = defineModel<string>("sort", { required: true });
const direction = defineModel<"asc" | "desc">("direction", { required: true });
const view = defineModel<"grid" | "list">("view", { required: true });
const page = defineModel<number>("page", { required: true });
const pageSize = defineModel<number>("pageSize", { required: true });

const selected = computed(() => new Set(selectedIds));

function isSelected(id: string) {
  return selected.value.has(id);
}

function toggleAsset(id: string, next: boolean) {
  if (next !== isSelected(id)) emit("toggleOne", id);
}

const controls = computed<CollectionControl[]>(() => [
  {
    kind: "select",
    id: "spaceKey",
    label: "资源空间",
    value: spaceKey.value,
    options: spaceOptions,
    searchPlaceholder: "搜索资源空间…",
    class: "w-40",
  },
  {
    kind: "select",
    id: "siteKey",
    label: "站点",
    value: siteKey.value,
    options: siteOptions,
    searchPlaceholder: "搜索站点…",
    class: "w-40",
  },
  {
    kind: "select",
    id: "profileKey",
    label: "Profile",
    value: profileKey.value,
    options: profileOptions,
    searchPlaceholder: "搜索 Profile…",
    class: "w-40",
  },
  {
    kind: "select",
    id: "visibility",
    label: "可见性",
    value: visibility.value,
    options: visibilityOptions,
    class: "w-28",
  },
  {
    kind: "select",
    id: "mime",
    label: "文件类型",
    value: mime.value,
    options: mimeOptions,
    class: "w-28",
  },
  {
    kind: "select",
    id: "sort",
    label: "素材排序",
    value: sort.value,
    options: sortOptions,
    icon: "i-tabler-arrows-sort",
    class: "w-32",
  },
  {
    kind: "direction",
    id: "direction",
    label: "排序方向",
    value: direction.value,
    ascendingLabel: "切换为倒序",
    descendingLabel: "切换为正序",
  },
]);
const messages: CollectionPanelMessages = {
  searchPlaceholder: "搜索文件名、标题、替代文本或 ID…",
  searchAction: "搜索",
  filtersAction: "筛选",
  activeFilters: (count) => `筛选（${count}）`,
  clearFilters: "清除筛选",
  selectPage: "选择当前页素材",
  selectItem: (label) => `选择素材：${label}`,
  bulkRegion: "素材批量操作",
  selected: (count) => `已选择 ${count} 个素材`,
  selectAllResults: "选择全部结果",
  clearSelection: "取消选择",
  emptyTitle: "没有匹配的资源",
  emptyDescription: "请调整搜索或资源筛选后重试。",
  errorTitle: "素材加载失败",
  retry: "重新加载",
  showing: (first, last, count) => `显示 ${first}–${last}，共 ${count} 个`,
  pageSize: "每页",
  pageSizeControl: "每页素材数量",
  pageSizeOption: (value) => `${value} 个`,
};
function changeControl(id: string, value: CollectionControlValue) {
  if (typeof value !== "string") return;
  if (id === "spaceKey") spaceKey.value = value;
  if (id === "siteKey") siteKey.value = value;
  if (id === "profileKey") profileKey.value = value;
  if (id === "visibility") visibility.value = value;
  if (id === "mime") mime.value = value;
  if (id === "sort") sort.value = value;
  if (id === "direction" && (value === "asc" || value === "desc"))
    direction.value = value;
}
function clearFilters() {
  spaceKey.value = "__all__";
  siteKey.value = "__all__";
  profileKey.value = "__all__";
  visibility.value = "__all__";
  mime.value = "__all__";
}
const assetKey = (asset: AssetItem) => asset.id;
const assetLabel = (asset: AssetItem) => asset.filename || asset.id;

function siteName(key: string) {
  return sites.find((site) => site.siteKey === key)?.name || key;
}

function formatBytes(value: number) {
  if (!value) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(
    Math.floor(Math.log(value) / Math.log(1024)),
    units.length - 1,
  );
  return `${(value / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}
</script>

<template>
  <section class="space-y-4" aria-label="中央素材库">
    <CollectionPanel
      v-model:search="search"
      :items="assets"
      :item-key="assetKey"
      :item-label="assetLabel"
      :controls="controls"
      :messages="messages"
      :total="total"
      :state="state"
      :error-message="errorMessage"
      :page="page"
      :page-size="pageSize"
      :page-sizes="pageSizeItems.map((item) => item.value)"
      :active-filter-count="filterCount"
      :layout="view === 'grid' ? 'grid' : 'rows'"
      selectable
      :selection-count="selectedIds.length"
      :page-selected="pageSelected"
      :page-indeterminate="pageIndeterminate"
      :is-selected="isSelected"
      label="中央素材库"
      @search="emit('search', $event)"
      @control-change="changeControl"
      @clear-filters="clearFilters"
      @retry="emit('retry')"
      @toggle-page="emit('togglePage', $event)"
      @toggle-item="toggleAsset"
      @clear-selection="emit('clearSelection')"
      @page-change="page = $event"
      @page-size-change="pageSize = $event"
    >
      <template #view>
        <CollectionViewToggle
          v-model="view"
          :items="[
            { key: 'grid', label: '网格', icon: 'i-tabler-layout-grid' },
            { key: 'list', label: '列表', icon: 'i-tabler-list' },
          ]"
        />
      </template>

      <template #columns>
        <div class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
          <span>素材、空间与文件信息</span>
          <span class="hidden w-44 text-right md:block">存储、策略与操作</span>
        </div>
      </template>

      <template #bulk-actions>
        <span class="hidden text-xs text-muted sm:inline"
          >仅图片会处理，其它类型记录为未完成</span
        >
        <UButton
          icon="i-tabler-refresh-dot"
          label="后台重建派生图"
          color="primary"
          variant="soft"
          size="xs"
          :loading="queueing"
          @click="emit('queueSelected')"
        />
      </template>

      <template #item="{ item: asset }">
        <div
          v-if="view === 'grid'"
          class="-m-4 flex min-h-full flex-col overflow-hidden rounded-lg"
        >
          <div
            class="relative aspect-[4/3] overflow-hidden border-b border-default bg-elevated"
          >
            <img
              v-if="asset.cdnUrl && asset.mime.startsWith('image/')"
              :src="asset.cdnUrl"
              :alt="asset.filename"
              loading="lazy"
              class="size-full object-cover transition duration-300 hover:scale-[1.03]"
            />
            <div v-else class="grid size-full place-items-center">
              <UIcon name="i-tabler-file" class="size-9 text-muted" />
            </div>
            <UDropdownMenu :items="actionsFor(asset)">
              <UButton
                icon="i-tabler-dots-vertical"
                color="neutral"
                variant="solid"
                square
                size="xs"
                class="absolute right-2 top-2"
                :aria-label="`素材操作：${assetLabel(asset)}`"
              />
            </UDropdownMenu>
          </div>
          <div class="min-w-0 p-3">
            <h2 class="truncate text-sm font-semibold text-highlighted">
              {{ asset.filename || asset.id }}
            </h2>
            <p class="mt-1 truncate text-xs text-muted">
              {{ siteName(asset.siteKey) }} ·
              {{ asset.profileKey || "default" }}
            </p>
            <div
              class="mt-3 flex items-end justify-between gap-3 border-t border-default pt-2.5 text-xs text-muted"
            >
              <div class="min-w-0">
                <p class="truncate">
                  {{ formatBytes(asset.size)
                  }}<span v-if="asset.width && asset.height">
                    · {{ asset.width }}×{{ asset.height }}</span
                  >
                </p>
                <p class="mt-0.5 truncate">
                  {{ asset.storageBackend || "local" }}
                </p>
              </div>
              <span v-if="asset.refCount" class="shrink-0 text-warning"
                >{{ asset.refCount }} 引用</span
              >
            </div>
          </div>
        </div>

        <div
          v-else
          class="grid min-w-0 gap-3 sm:grid-cols-[3.5rem_minmax(0,1fr)] md:grid-cols-[3.5rem_minmax(0,1fr)_11rem_auto] md:items-center"
        >
          <div
            class="hidden size-14 place-items-center overflow-hidden rounded-lg bg-elevated sm:grid"
          >
            <img
              v-if="asset.cdnUrl && asset.mime.startsWith('image/')"
              :src="asset.cdnUrl"
              :alt="asset.filename"
              loading="lazy"
              class="size-full object-cover"
            />
            <UIcon v-else name="i-tabler-file" class="size-5 text-muted" />
          </div>
          <div class="min-w-0">
            <p class="truncate text-sm font-semibold text-highlighted">
              {{ asset.filename || asset.id }}
            </p>
            <p class="mt-0.5 truncate text-xs text-muted">
              {{ asset.mime }} · {{ formatBytes(asset.size)
              }}<span v-if="asset.width && asset.height">
                · {{ asset.width }}×{{ asset.height }}</span
              >
            </p>
            <p class="mt-1 truncate text-xs text-dimmed">
              空间 {{ asset.spaceKey || "default" }} ·
              {{ siteName(asset.siteKey) }} / {{ asset.profileKey }}
            </p>
          </div>
          <div class="min-w-0 text-xs md:text-right">
            <p class="truncate text-default">
              {{ asset.storageBackend || "local" }}
            </p>
            <p class="mt-0.5 truncate text-muted">
              {{ asset.visibility }} · {{ asset.deliveryPolicy || "public" }}
            </p>
            <p v-if="asset.refCount" class="mt-1 text-warning">
              {{ asset.refCount }} 个引用
            </p>
          </div>
          <div class="flex justify-end">
            <UDropdownMenu :items="actionsFor(asset)"
              ><UButton
                icon="i-tabler-dots-vertical"
                color="neutral"
                variant="ghost"
                square
                size="xs"
                :aria-label="`素材操作：${assetLabel(asset)}`"
            /></UDropdownMenu>
          </div>
        </div>
      </template>
    </CollectionPanel>

    <UAlert
      v-if="queueError"
      color="error"
      variant="subtle"
      icon="i-tabler-alert-circle"
      title="无法创建重建任务"
      :description="queueError"
    />

    <AssetMaintenanceDock
      v-if="taskId"
      :task-id="taskId"
      :task="task"
      :controlling-task-id="controllingTaskId"
      @task-action="emit('taskAction', $event)"
      @open-maintenance="emit('openMaintenance')"
      @dismiss-task="emit('dismissTask')"
    />
  </section>
</template>
