import type { Ref } from 'vue'
import { createAccountNotifier } from '~/utils/feedback'
import { apiErrorMessage } from '../utils/api-errors'
import type { AssetMaintenanceTask } from '~/types/asset-admin'

type MaintenanceAction = 'pause' | 'resume' | 'cancel'

interface UseAssetMaintenanceTasksOptions {
  selectedTaskId: Ref<string>
  getSelectedAssetIds: () => string[]
  clearSelection: () => void
  refreshAfterSettled: () => Promise<void>
}

export function useAssetMaintenanceTasks(options: UseAssetMaintenanceTasksOptions) {
  const { call } = useApi()
  const toast = createAccountNotifier(useToast())
  const maintenanceTasks = ref<AssetMaintenanceTask[]>([])
  const controllingMaintenanceTaskId = ref('')
  const queueingSelectedRebuild = ref(false)
  const selectedRebuildError = ref('')
  const selectedTask = computed(() => maintenanceTasks.value.find(task => task.id === options.selectedTaskId.value))
  const hasActiveMaintenanceTask = computed(() => maintenanceTasks.value.some(task => (
    task.status === 'queued' || task.status === 'running' || task.status === 'retrying'
  )))
  const activeMaintenanceCount = computed(() => maintenanceTasks.value.filter(task => (
    task.status === 'queued' || task.status === 'running' || task.status === 'retrying'
  )).length)

  let pollTimer: ReturnType<typeof setInterval> | undefined
  let pollInFlight = false
  let pollErrorShown = false

  function stopPolling() {
    if (!pollTimer) return
    clearInterval(pollTimer)
    pollTimer = undefined
  }

  watch(hasActiveMaintenanceTask, (active) => {
    if (active && !pollTimer) {
      pollTimer = setInterval(() => { void pollMaintenanceTasks() }, 5000)
      return
    }
    if (!active) stopPolling()
  }, { immediate: true })

  onScopeDispose(stopPolling)

  async function fetchMaintenanceTasks() {
    const data = await call<{ items: AssetMaintenanceTask[] }>('/api/v1/admin/assets-proxy/maintenance/tasks', {
      params: { page: 1, size: 8 }
    })
    const items = data.items ?? []
    if (options.selectedTaskId.value && !items.some(task => task.id === options.selectedTaskId.value)) {
      try {
        const selected = await call<{ task: AssetMaintenanceTask }>(`/api/v1/admin/assets-proxy/maintenance/tasks/${options.selectedTaskId.value}`)
        items.unshift(selected.task)
      } catch {
        // Keep the recent task list useful when an old deep-linked task no longer exists.
      }
    }
    maintenanceTasks.value = items
  }

  async function pollMaintenanceTasks() {
    if (pollInFlight) return
    pollInFlight = true
    const hadActiveTask = hasActiveMaintenanceTask.value
    try {
      await fetchMaintenanceTasks()
      if (hadActiveTask && !hasActiveMaintenanceTask.value) await options.refreshAfterSettled()
      pollErrorShown = false
    } catch (error) {
      if (!pollErrorShown) {
        toast.add({ title: '维护任务状态刷新失败', description: apiErrorMessage(error, { fallback: '暂时无法刷新维护任务状态。' }), color: 'warning' })
        pollErrorShown = true
      }
    } finally {
      pollInFlight = false
    }
  }

  function showQueuedMaintenanceTask(title: string, task: AssetMaintenanceTask) {
    const queued = { ...task, summary: task.summary || title }
    maintenanceTasks.value = [queued, ...maintenanceTasks.value.filter(item => item.id !== task.id)]
    options.selectedTaskId.value = task.id
  }

  async function controlMaintenanceTask(task: AssetMaintenanceTask, action: MaintenanceAction, silent = false) {
    controllingMaintenanceTaskId.value = `${task.id}:${action}`
    try {
      await call(`/api/v1/admin/assets-proxy/maintenance/tasks/${task.id}/${action}`, { method: 'POST' })
      await fetchMaintenanceTasks()
    } catch (error) {
      if (silent) selectedRebuildError.value = apiErrorMessage(error, { fallback: '暂时无法操作维护任务。' })
      else toast.add({ title: '维护任务操作失败', description: apiErrorMessage(error, { fallback: '暂时无法操作维护任务。' }), color: 'error' })
    } finally {
      controllingMaintenanceTaskId.value = ''
    }
  }

  function dismissSelectedTask() {
    options.selectedTaskId.value = ''
    selectedRebuildError.value = ''
  }

  function controlSelectedTask(action: MaintenanceAction) {
    if (selectedTask.value) void controlMaintenanceTask(selectedTask.value, action, true)
  }

  async function queueSelectedRebuild() {
    const ids = options.getSelectedAssetIds()
    if (!ids.length || queueingSelectedRebuild.value) return
    queueingSelectedRebuild.value = true
    selectedRebuildError.value = ''
    try {
      const response = await call<{ task?: AssetMaintenanceTask }>('/api/v1/admin/assets-proxy/maintenance/rebuild-derivatives', {
        method: 'POST',
        body: { ids, dryRun: false }
      })
      if (!response.task?.id) throw new Error('后台未返回任务 ID')
      maintenanceTasks.value = [response.task, ...maintenanceTasks.value.filter(task => task.id !== response.task?.id)]
      options.selectedTaskId.value = response.task.id
      options.clearSelection()
    } catch (error) {
      selectedRebuildError.value = apiErrorMessage(error, { fallback: '暂时无法创建后台任务。' })
    } finally {
      queueingSelectedRebuild.value = false
    }
  }

  return {
    maintenanceTasks: shallowReadonly(maintenanceTasks),
    controllingMaintenanceTaskId: readonly(controllingMaintenanceTaskId),
    queueingSelectedRebuild: readonly(queueingSelectedRebuild),
    selectedRebuildError: readonly(selectedRebuildError),
    selectedTask,
    activeMaintenanceCount,
    fetchMaintenanceTasks,
    showQueuedMaintenanceTask,
    controlMaintenanceTask,
    dismissSelectedTask,
    controlSelectedTask,
    queueSelectedRebuild
  }
}
