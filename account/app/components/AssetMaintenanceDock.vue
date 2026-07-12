<script setup lang="ts">
import { ManageCollectionDock, ManagePagination } from '@platform/manage/components'
import type { AssetMaintenanceTask as MaintenanceTask } from '~/types/asset-admin'

interface MaintenanceTaskError {
  id?: string
  filename?: string
  error: string
}

interface MaintenanceTaskResult {
  candidates?: number
  rebuilt?: number
  generated?: number
  errors?: MaintenanceTaskError[]
}

const {
  total,
  selectedCount,
  pageSelected,
  pageIndeterminate,
  queueing = false,
  queueError = '',
  taskId = '',
  task,
  controllingTaskId = '',
  pageSizeItems,
  totalPages
} = defineProps<{
  total: number
  selectedCount: number
  pageSelected: boolean
  pageIndeterminate: boolean
  queueing?: boolean
  queueError?: string
  taskId?: string
  task?: MaintenanceTask
  controllingTaskId?: string
  pageSizeItems: Array<{ label: string, value: number }>
  totalPages: number
}>()

const emit = defineEmits<{
  togglePage: [value: boolean]
  queueSelected: []
  clearSelection: []
  taskAction: [action: 'pause' | 'resume' | 'cancel']
  openMaintenance: []
  dismissTask: []
}>()

const page = defineModel<number>('page', { required: true })
const pageSize = defineModel<number>('pageSize', { required: true })

const result = computed<MaintenanceTaskResult>(() => parseJSON(task?.result))
const payload = computed<{ ids?: string[] }>(() => parseJSON(task?.payload))
const candidates = computed(() => result.value.candidates || payload.value.ids?.length || 0)
const processed = computed(() => (result.value.rebuilt || 0) + (result.value.errors?.length || 0))
const percent = computed(() => {
  if (!candidates.value) return task?.status === 'completed' ? 100 : 0
  return Math.min(100, Math.round(processed.value / candidates.value * 100))
})

function parseJSON<T extends object>(value?: string): T {
  if (!value || value === '{}') return {} as T
  try {
    return JSON.parse(value) as T
  } catch {
    return {} as T
  }
}

function status(taskValue: MaintenanceTask) {
  if (taskValue.status === 'failed') return { label: '失败', color: 'error' as const }
  if (taskValue.status === 'queued') return { label: '排队中', color: 'neutral' as const }
  if (taskValue.status === 'running') return { label: '执行中', color: 'warning' as const }
  if (taskValue.status === 'retrying') return { label: '重试中', color: 'warning' as const }
  if (taskValue.status === 'paused') return { label: '已暂停', color: 'neutral' as const }
  if (taskValue.status === 'completed') return { label: '完成', color: 'success' as const }
  return { label: '已取消', color: 'neutral' as const }
}

function canPause(taskValue: MaintenanceTask) {
  return taskValue.status === 'queued' || taskValue.status === 'running' || taskValue.status === 'retrying'
}

function canResume(taskValue: MaintenanceTask) {
  return taskValue.status === 'paused'
}

function canCancel(taskValue: MaintenanceTask) {
  return canPause(taskValue) || taskValue.status === 'paused'
}

function taskIcon(taskValue?: MaintenanceTask) {
  if (taskValue && (taskValue.status === 'failed' || taskValue.status === 'cancelled')) return 'i-tabler-alert-triangle'
  if (taskValue?.status === 'completed') return 'i-tabler-circle-check'
  return 'i-tabler-progress'
}

function taskIconClass(taskValue?: MaintenanceTask) {
  if (taskValue && (taskValue.status === 'failed' || taskValue.status === 'cancelled')) return 'text-warning'
  if (taskValue?.status === 'completed') return 'text-success'
  return 'text-primary'
}
</script>

<template>
  <ManageCollectionDock :with-sidebar="false" label="素材库选择、任务状态与分页">
    <template #selection>
      <template v-if="taskId">
        <UIcon :name="taskIcon(task)" :class="taskIconClass(task)" class="size-4 shrink-0" />
        <div class="min-w-0" role="status" aria-live="polite">
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-sm font-medium text-default">所选素材派生图重建</span>
            <UBadge v-if="task" :label="status(task).label" :color="status(task).color" variant="soft" size="sm" />
            <span v-else class="text-xs text-muted">正在读取任务…</span>
          </div>
          <div v-if="task" class="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted">
            <UProgress :model-value="percent" size="xs" class="w-24" />
            <span>{{ processed }}/{{ candidates || '—' }} 已处理</span>
            <span v-if="result.generated">生成 {{ result.generated }} 个派生文件</span>
            <span v-if="result.errors?.length" class="text-warning">{{ result.errors.length }} 个未完成</span>
          </div>
        </div>
        <UButton
          v-if="task && canPause(task)"
          icon="i-tabler-player-pause"
          label="暂停"
          color="neutral"
          variant="ghost"
          size="sm"
          :loading="controllingTaskId === `${task.id}:pause`"
          @click="emit('taskAction', 'pause')"
        />
        <UButton
          v-if="task && canResume(task)"
          icon="i-tabler-player-play"
          label="恢复"
          color="neutral"
          variant="ghost"
          size="sm"
          :loading="controllingTaskId === `${task.id}:resume`"
          @click="emit('taskAction', 'resume')"
        />
        <UButton
          v-if="task && canCancel(task)"
          icon="i-tabler-ban"
          label="取消任务"
          color="error"
          variant="ghost"
          size="sm"
          :loading="controllingTaskId === `${task.id}:cancel`"
          @click="emit('taskAction', 'cancel')"
        />
        <UButton label="维护记录" color="neutral" variant="soft" size="sm" @click="emit('openMaintenance')" />
        <UButton icon="i-tabler-x" color="neutral" variant="ghost" square size="sm" aria-label="关闭任务状态" @click="emit('dismissTask')" />
      </template>

      <template v-else>
        <UCheckbox :model-value="pageSelected" :indeterminate="pageIndeterminate" aria-label="选择当前页素材" @update:model-value="emit('togglePage', $event === true)" />
        <template v-if="selectedCount">
          <span class="text-sm text-default">已选 {{ selectedCount }}</span>
          <UButton icon="i-tabler-refresh-dot" label="后台重建派生图" color="primary" variant="soft" size="sm" :loading="queueing" @click="emit('queueSelected')" />
          <span class="text-xs text-muted">仅图片会处理，其它类型记录为未完成</span>
          <span v-if="queueError" role="alert" class="text-xs text-error">{{ queueError }}</span>
          <UButton label="取消选择" color="neutral" variant="ghost" size="sm" :disabled="queueing" @click="emit('clearSelection')" />
        </template>
        <span v-else class="text-xs">共 {{ total }} 个素材</span>
      </template>
    </template>

    <template #pagination>
      <USelect v-model="pageSize" :items="pageSizeItems" value-key="value" size="sm" class="w-20" />
      <ManagePagination v-model="page" :total-pages="totalPages" class="!mt-0" />
    </template>
  </ManageCollectionDock>
</template>
