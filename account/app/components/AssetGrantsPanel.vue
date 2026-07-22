<script setup lang="ts">
import type { DropdownMenuItem } from "@nuxt/ui";
import { ManageEmpty } from "@platform/manage/components";
import { CollectionPagination } from "@yueli/ui/collection/pattern";
import { abs } from "@platform/ui/date";
import type { AssetGrant } from "~/types/asset-admin";

const { grants, total, pageSize, actionsFor } = defineProps<{
  grants: AssetGrant[];
  total: number;
  pageSize: number;
  actionsFor: (grant: AssetGrant) => DropdownMenuItem[][];
}>();
const page = defineModel<number>("page", { required: true });
const totalPages = computed(() => Math.max(1, Math.ceil(total / pageSize)));

function status(grant: AssetGrant): {
  label: string;
  color: "success" | "warning" | "neutral";
} {
  if (grant.revokedAt) return { label: "已撤销", color: "neutral" };
  if (grant.expiresAt && new Date(grant.expiresAt).getTime() <= Date.now())
    return { label: "已过期", color: "warning" };
  if (grant.maxUses > 0 && grant.usedCount >= grant.maxUses)
    return { label: "已用完", color: "warning" };
  return { label: "有效", color: "success" };
}

function auditText(grant: AssetGrant) {
  if (grant.revokedBy) return `撤销 ${grant.revokedBy}`;
  return grant.createdByService ? `签发 ${grant.createdByService}` : "系统签发";
}
</script>

<template>
  <section class="space-y-4" aria-labelledby="asset-grants-heading">
    <div>
      <h2
        id="asset-grants-heading"
        class="text-sm font-semibold text-highlighted"
      >
        交付授权
      </h2>
      <p class="mt-1 text-xs text-muted">
        查看私有素材的签发用途、使用次数、有效期与撤销状态。
      </p>
    </div>
    <ManageEmpty
      v-if="!grants.length"
      icon="i-tabler-key-off"
      text="还没有交付授权"
    />
    <div
      v-else
      class="overflow-hidden rounded-lg border border-default bg-default"
    >
      <div
        v-for="grant in grants"
        :key="grant.id"
        class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 border-b border-default px-4 py-3 last:border-b-0"
      >
        <span
          class="grid size-10 place-items-center rounded-lg bg-primary/10 text-primary"
        >
          <UIcon name="i-tabler-key" class="size-5" />
        </span>
        <div class="min-w-0">
          <div class="flex min-w-0 flex-wrap items-center gap-1.5">
            <span class="truncate text-sm font-medium text-highlighted">{{
              grant.reason || grant.policy
            }}</span>
            <UBadge
              :label="status(grant).label"
              :color="status(grant).color"
              variant="soft"
              size="sm"
            />
            <UBadge
              :label="grant.purpose || 'delivery'"
              color="info"
              variant="soft"
              size="sm"
            />
            <UBadge
              :label="grant.policy"
              color="neutral"
              variant="soft"
              size="sm"
            />
          </div>
          <p class="mt-0.5 truncate font-mono text-xs text-muted">
            {{ grant.assetId }}
          </p>
          <div class="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-muted">
            <span>{{ grant.usedCount }} / {{ grant.maxUses }} 次</span>
            <ClientOnly>
              <span>过期 {{ abs(grant.expiresAt) }}</span>
              <template #fallback><span>过期 …</span></template>
            </ClientOnly>
            <span>{{ grant.lastUsedAt ? "已使用" : "尚未使用" }}</span>
            <span>{{ auditText(grant) }}</span>
          </div>
        </div>
        <UDropdownMenu :items="actionsFor(grant)">
          <UButton
            icon="i-tabler-dots-vertical"
            color="neutral"
            variant="ghost"
            square
            size="sm"
            :aria-label="`授权操作：${grant.id}`"
          />
        </UDropdownMenu>
      </div>
    </div>
    <div
      v-if="total"
      class="flex flex-wrap items-center justify-between gap-3 text-sm text-muted"
    >
      <span>共 {{ total }} 条授权</span>
      <CollectionPagination v-model="page" :total-pages="totalPages" />
    </div>
  </section>
</template>
