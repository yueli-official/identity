<script setup lang="ts">
import { ManageEmpty } from '@platform/manage/components'
import { abs } from '@platform/ui/date'
import type { AssetMaintenanceTask, AssetMaintenanceTaskResult } from '~/types/asset-admin'

const {
  tasks,
  controllingTaskId = '',
  sweeping = false,
  pruning = false,
  auditing = false
} = defineProps<{
  tasks: AssetMaintenanceTask[]
  controllingTaskId?: string
  sweeping?: boolean
  pruning?: boolean
  auditing?: boolean
}>()

const emit = defineEmits<{
  refresh: []
  sweepStaging: []
  previewPrune: []
  previewOrphans: []
  control: [task: AssetMaintenanceTask, action: 'pause' | 'resume' | 'cancel']
}>()

function taskTypeLabel(type: string) {
  const labels: Record<string, string> = {
    'sweep-staging': '清理暂存',
    'prune-unreferenced': '无引用素材',
    'orphan-objects': '孤儿对象',
    'rebuild-derivatives': '重建派生图',
    'migrate-storage': '存储迁移'
  }
  return labels[type] || type
}

function taskStatus(task: AssetMaintenanceTask): { label: string, color: 'success' | 'warning' | 'error' | 'neutral' } {
  if (task.status === 'failed') return { label: '失败', color: 'error' }
  if (task.dryRun) return { label: '预检', color: 'warning' }
  if (task.status === 'queued') return { label: '排队中', color: 'neutral' }
  if (task.status === 'running') return { label: '执行中', color: 'warning' }
  if (task.status === 'retrying') return { label: '重试中', color: 'warning' }
  if (task.status === 'paused') return { label: '已暂停', color: 'neutral' }
  if (task.status === 'completed') return { label: '完成', color: 'success' }
  if (task.status === 'cancelled') return { label: '已取消', color: 'neutral' }
  return { label: '未知', color: 'neutral' }
}

function parseTaskResult(value?: string): AssetMaintenanceTaskResult {
  if (!value || value === '{}') return {}
  try {
    return JSON.parse(value) as AssetMaintenanceTaskResult
  } catch {
    return {}
  }
}

function taskResultSummary(task: AssetMaintenanceTask) {
  const result = parseTaskResult(task.result)
  if (result.candidates == null) return ''
  const processed = (result.rebuilt || 0) + (result.errors?.length || 0)
  const parts = [`${processed}/${result.candidates} 已处理`]
  if (result.generated) parts.push(`生成 ${result.generated}`)
  if (result.errors?.length) parts.push(`${result.errors.length} 个未完成`)
  return parts.join(' · ')
}

function canPause(task: AssetMaintenanceTask) {
  return task.status === 'queued' || task.status === 'running' || task.status === 'retrying'
}

function canResume(task: AssetMaintenanceTask) {
  return task.status === 'paused'
}

function canCancel(task: AssetMaintenanceTask) {
  return canPause(task) || task.status === 'paused'
}
</script>

<template>
  <section class="space-y-4" aria-labelledby="asset-maintenance-heading">
    <div class="rounded-lg border border-default bg-default p-4">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 id="asset-maintenance-heading" class="text-sm font-semibold text-highlighted">维护任务</h2>
          <p class="mt-1 text-xs text-muted">低频清理、扫描和迁移操作集中在这里执行，并保留可恢复的任务记录。</p>
        </div>
        <div class="grid grid-cols-1 gap-2 sm:flex sm:flex-wrap sm:justify-end">
          <UButton icon="i-tabler-broom" label="清理暂存" color="neutral" variant="soft" size="sm" :loading="sweeping" @click="emit('sweepStaging')" />
          <UButton icon="i-tabler-unlink" label="清理无引用" color="neutral" variant="soft" size="sm" :loading="pruning" @click="emit('previewPrune')" />
          <UButton icon="i-tabler-database-search" label="扫描孤儿对象" color="neutral" variant="soft" size="sm" :loading="auditing" @click="emit('previewOrphans')" />
        </div>
      </div>

      <div class="mt-4 overflow-hidden rounded-lg border border-default bg-elevated/30">
        <div class="flex items-center justify-between border-b border-default px-3 py-2">
          <h3 class="text-xs font-medium text-muted">最近维护</h3>
          <UTooltip text="刷新任务">
            <UButton icon="i-tabler-refresh" color="neutral" variant="ghost" square size="sm" aria-label="刷新维护任务" @click="emit('refresh')" />
          </UTooltip>
        </div>

        <ManageEmpty v-if="!tasks.length" icon="i-tabler-history" text="还没有维护记录" />
        <div v-else class="divide-y divide-default">
          <div v-for="task in tasks" :key="task.id" class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 px-3 py-3">
            <span class="grid size-9 shrink-0 place-items-center rounded-lg bg-default text-muted">
              <UIcon name="i-tabler-history" class="size-4" />
            </span>
            <div class="min-w-0">
              <div class="flex min-w-0 flex-wrap items-center gap-2">
                <span class="min-w-0 truncate text-sm font-medium text-highlighted">{{ task.summary || taskTypeLabel(task.taskType) }}</span>
                <UBadge :label="taskStatus(task).label" :color="taskStatus(task).color" variant="soft" size="sm" />
              </div>
              <div class="mt-0.5 truncate text-xs text-muted">
                {{ taskTypeLabel(task.taskType) }} ·
                <ClientOnly>
                  <span>{{ abs(task.createdAt) }}</span>
                  <template #fallback><span>…</span></template>
                </ClientOnly>
              </div>
              <div v-if="taskResultSummary(task)" class="mt-0.5 truncate text-xs text-muted">{{ taskResultSummary(task) }}</div>
              <div v-if="task.error" class="mt-1 line-clamp-2 text-xs text-error">{{ task.error }}</div>
            </div>
            <div class="flex shrink-0 items-center gap-1">
              <UTooltip v-if="canPause(task)" text="暂停任务">
                <UButton
                  icon="i-tabler-player-pause"
                  color="neutral"
                  variant="ghost"
                  square
                  size="sm"
                  aria-label="暂停任务"
                  :loading="controllingTaskId === `${task.id}:pause`"
                  @click="emit('control', task, 'pause')"
                />
              </UTooltip>
              <UTooltip v-if="canResume(task)" text="恢复任务">
                <UButton
                  icon="i-tabler-player-play"
                  color="neutral"
                  variant="ghost"
                  square
                  size="sm"
                  aria-label="恢复任务"
                  :loading="controllingTaskId === `${task.id}:resume`"
                  @click="emit('control', task, 'resume')"
                />
              </UTooltip>
              <UTooltip v-if="canCancel(task)" text="取消任务">
                <UButton
                  icon="i-tabler-ban"
                  color="error"
                  variant="ghost"
                  square
                  size="sm"
                  aria-label="取消任务"
                  :loading="controllingTaskId === `${task.id}:cancel`"
                  @click="emit('control', task, 'cancel')"
                />
              </UTooltip>
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
