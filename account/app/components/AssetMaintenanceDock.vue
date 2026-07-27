<script setup lang="ts">
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
  taskId = '',
  task,
  controllingTaskId = '',
} = defineProps<{
  taskId?: string
  task?: MaintenanceTask
  controllingTaskId?: string
}>()

const emit = defineEmits<{
  taskAction: [action: 'pause' | 'resume' | 'cancel']
  openMaintenance: []
  dismissTask: []
}>()

const result = computed<MaintenanceTaskResult>(() => parseJSON(task?.result))
const payload = computed<{ ids?: string[] }>(() => parseJSON(task?.payload))
const candidates = computed(() => result.value.candidates || payload.value.ids?.length || 0)
const processed = computed(() => (result.value.rebuilt || 0) + (result.value.errors?.length || 0))
const percent = computed(() => {
  if (!candidates.value) return task?.status === 'completed' ? 100 : 0
  return Math.min(100, Math.round((processed.value / candidates.value) * 100))
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
  <div v-if="taskId" class="flex flex-wrap items-center gap-2 rounded-lg border border-default bg-default px-3 py-2.5" role="status" aria-live="polite">
    <UIcon :name="taskIcon(task)" :class="taskIconClass(task)" class="size-4 shrink-0" />
    <div class="min-w-0 flex-1">
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
      size="xs"
      :loading="controllingTaskId === `${task.id}:pause`"
      @click="emit('taskAction', 'pause')"
    />
    <UButton
      v-if="task && canResume(task)"
      icon="i-tabler-player-play"
      label="恢复"
      color="neutral"
      variant="ghost"
      size="xs"
      :loading="controllingTaskId === `${task.id}:resume`"
      @click="emit('taskAction', 'resume')"
    />
    <UButton
      v-if="task && canCancel(task)"
      icon="i-tabler-ban"
      label="取消任务"
      color="error"
      variant="ghost"
      size="xs"
      :loading="controllingTaskId === `${task.id}:cancel`"
      @click="emit('taskAction', 'cancel')"
    />
    <UButton label="维护记录" color="neutral" variant="soft" size="xs" @click="emit('openMaintenance')" />
    <UButton icon="i-tabler-x" color="neutral" variant="ghost" square size="xs" aria-label="关闭任务状态" @click="emit('dismissTask')" />
  </div>
</template>
