<script setup lang="ts">
import { ManageEmpty, SkeletonList } from '~/components/manage'
import type { AssetItem, AssetSecurityDetail } from '~/types/asset-admin'
import { assetScanAttemptPresentation, assetSecurityPresentation } from '~/utils/asset-security'

const props = withDefaults(defineProps<{
  asset?: AssetItem | null
  detail?: AssetSecurityDetail | null
  loading?: boolean
  retrying?: boolean
}>(), {
  asset: null,
  detail: null,
  loading: false,
  retrying: false
})

const emit = defineEmits<{
  retry: []
  reject: []
  delete: []
}>()

const open = defineModel<boolean>('open', { required: true })
const currentAsset = computed(() => props.detail?.asset ?? props.asset)
const presentation = computed(() => currentAsset.value ? assetSecurityPresentation(currentAsset.value) : null)
const canRetry = computed(() => {
  const asset = currentAsset.value
  return Boolean(
    asset
    && props.detail?.quarantineAvailable
    && asset.securityState !== 'ready'
    && asset.scanStatus !== 'running'
  )
})
const canReject = computed(() => {
  const asset = currentAsset.value
  return Boolean(
    asset
    && asset.securityState === 'quarantined'
    && (asset.scanStatus === 'pending' || asset.scanStatus === 'failed')
  )
})

function briefDate(value?: string) {
  return value ? value.replace('T', ' ').replace('Z', '').slice(0, 19) : '—'
}

function shortHash(value?: string) {
  if (!value) return '—'
  return value.length > 28 ? `${value.slice(0, 14)}…${value.slice(-10)}` : value
}

function closeModal() {
  open.value = false
}
</script>

<template>
  <UModal
    v-model:open="open"
    title="素材安全详情"
    description="检查隔离状态、扫描证据与每次处理结果。"
    :ui="{ content: 'max-w-3xl' }"
  >
    <template #body>
      <div class="space-y-4">
        <div v-if="currentAsset && presentation" class="rounded-lg border border-default bg-elevated/30 p-4">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-highlighted">{{ currentAsset.filename || currentAsset.id }}</p>
              <p class="mt-1 truncate font-mono text-xs text-dimmed">{{ currentAsset.id }}</p>
            </div>
            <UBadge
              :label="presentation.label"
              :icon="presentation.icon"
              :color="presentation.color"
              variant="soft"
              size="md"
              class="shrink-0"
            />
          </div>
          <p class="mt-3 text-sm text-muted">{{ presentation.description }}</p>
          <p v-if="currentAsset.scanFailureCode" class="mt-2 text-xs text-error">
            故障代码：<span class="font-mono">{{ currentAsset.scanFailureCode }}</span>
          </p>
          <p v-if="detail?.detectedThreat" class="mt-2 text-sm font-medium text-error">
            检出威胁：{{ detail.detectedThreat }}
          </p>
        </div>

        <SkeletonList v-if="loading" :rows="5" />

        <template v-else-if="detail">
          <section aria-labelledby="asset-security-evidence">
            <div class="mb-2 flex items-center justify-between gap-3">
              <h3 id="asset-security-evidence" class="text-xs font-medium uppercase tracking-wide text-muted">安全证据</h3>
              <UBadge
                :label="detail.quarantineAvailable ? '隔离原文可用' : '隔离原文已清除'"
                :icon="detail.quarantineAvailable ? 'i-tabler-lock-check' : 'i-tabler-trash'"
                :color="detail.quarantineAvailable ? 'neutral' : 'warning'"
                variant="soft"
                size="sm"
              />
            </div>
            <dl class="grid overflow-hidden rounded-lg border border-default bg-default sm:grid-cols-2">
              <div class="border-b border-default px-3 py-2.5 sm:border-r">
                <dt class="text-xs text-muted">扫描器</dt>
                <dd class="mt-0.5 text-sm text-highlighted">{{ detail.scannerEngine || '—' }}<span v-if="detail.scannerEngineVersion"> {{ detail.scannerEngineVersion }}</span></dd>
              </div>
              <div class="border-b border-default px-3 py-2.5">
                <dt class="text-xs text-muted">签名版本</dt>
                <dd class="mt-0.5 break-all font-mono text-xs text-highlighted">{{ detail.scannerSignatureVersion || '—' }}</dd>
              </div>
              <div class="border-b border-default px-3 py-2.5 sm:border-r">
                <dt class="text-xs text-muted">权威类型</dt>
                <dd class="mt-0.5 text-sm text-highlighted">{{ detail.asset.detectedMime || '—' }}</dd>
              </div>
              <div class="border-b border-default px-3 py-2.5">
                <dt class="text-xs text-muted">最近扫描</dt>
                <dd class="mt-0.5 text-sm text-highlighted">{{ briefDate(detail.lastScanAt) }}</dd>
              </div>
              <div class="border-b border-default px-3 py-2.5 sm:border-b-0 sm:border-r">
                <dt class="text-xs text-muted">接收内容 SHA-256</dt>
                <dd :title="detail.sourceHash" class="mt-0.5 break-all font-mono text-xs text-highlighted">{{ shortHash(detail.sourceHash) }}</dd>
              </div>
              <div class="px-3 py-2.5">
                <dt class="text-xs text-muted">发布内容 SHA-256</dt>
                <dd :title="detail.contentHash" class="mt-0.5 break-all font-mono text-xs text-highlighted">{{ shortHash(detail.contentHash) }}</dd>
              </div>
            </dl>
          </section>

          <section aria-labelledby="asset-security-attempts">
            <div class="mb-2 flex items-center justify-between">
              <h3 id="asset-security-attempts" class="text-xs font-medium uppercase tracking-wide text-muted">处理记录</h3>
              <span class="text-xs text-dimmed">最近 {{ detail.attempts.length }} 次</span>
            </div>
            <ManageEmpty v-if="!detail.attempts.length" icon="i-tabler-shield-off" text="还没有安全处理记录" />
            <ol v-else class="max-h-80 overflow-y-auto rounded-lg border border-default bg-default">
              <li
                v-for="attempt in detail.attempts"
                :key="attempt.id"
                class="border-b border-default px-3 py-3 last:border-b-0"
              >
                <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
                  <UBadge
                    :label="assetScanAttemptPresentation(attempt.status).label"
                    :icon="assetScanAttemptPresentation(attempt.status).icon"
                    :color="assetScanAttemptPresentation(attempt.status).color"
                    variant="soft"
                    size="sm"
                  />
                  <span class="font-mono text-xs text-dimmed">{{ attempt.id }}</span>
                  <time class="text-xs text-muted sm:ml-auto">{{ briefDate(attempt.startedAt) }}</time>
                </div>
                <p v-if="attempt.threat" class="mt-2 text-sm font-medium text-error">威胁：{{ attempt.threat }}</p>
                <p v-if="attempt.failureCode" class="mt-2 text-xs text-error">故障：<span class="font-mono">{{ attempt.failureCode }}</span></p>
                <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted">
                  <span v-if="attempt.scannerEngine">{{ attempt.scannerEngine }} {{ attempt.engineVersion }}</span>
                  <span v-if="attempt.signatureVersion">签名 {{ attempt.signatureVersion }}</span>
                  <span v-if="attempt.detectedMime">{{ attempt.detectedMime }}</span>
                  <span v-if="attempt.sanitized">已净化元数据</span>
                  <span v-if="attempt.finishedAt">完成 {{ briefDate(attempt.finishedAt) }}</span>
                </div>
              </li>
            </ol>
          </section>
        </template>

        <ManageEmpty v-else icon="i-tabler-shield-question" text="没有可显示的安全详情" />
      </div>
    </template>

    <template #footer>
      <div class="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
        <UButton
          label="删除素材"
          icon="i-tabler-trash"
          color="error"
          variant="ghost"
          class="justify-center"
          :disabled="loading || retrying || !currentAsset"
          @click="emit('delete')"
        />
        <div class="grid grid-cols-3 gap-2 sm:flex">
          <UButton label="关闭" color="neutral" variant="ghost" class="justify-center" :disabled="retrying" @click="closeModal" />
          <UButton
            label="拒绝交付"
            icon="i-tabler-user-shield"
            color="warning"
            variant="soft"
            class="justify-center"
            :disabled="loading || retrying || !canReject"
            @click="emit('reject')"
          />
          <UButton
            label="重新安全处理"
            icon="i-tabler-refresh"
            class="justify-center"
            :loading="retrying"
            :disabled="loading || !canRetry"
            @click="emit('retry')"
          />
        </div>
      </div>
    </template>
  </UModal>
</template>
