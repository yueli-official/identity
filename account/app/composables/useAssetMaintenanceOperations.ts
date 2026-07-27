import type { Ref } from 'vue'
import { createAccountNotifier } from '~/utils/feedback'
import { apiErrorMessage } from '../utils/api-errors'
import type {
  AssetBatchRebuildResult,
  AssetItem,
  AssetMaintenanceTask,
  AssetOrphanObjectForm,
  AssetOrphanObjectResult,
  AssetProfile,
  AssetPruneForm,
  AssetPruneResult,
  AssetStorageBackend,
  AssetStorageMigrationForm,
  AssetStorageMigrationResult,
  AssetSweepResult
} from '~/types/asset-admin'

interface UseAssetMaintenanceOperationsOptions {
  storageBackends: Readonly<Ref<readonly AssetStorageBackend[]>>
  fetchMaintenanceTasks: () => Promise<void>
  showQueuedTask: (title: string, task: AssetMaintenanceTask) => void
  reloadAll: () => Promise<void>
}

export interface AssetMaintenanceOperationFeedback {
  title: string
  description: string
  tone: 'success' | 'warning'
}

export function useAssetMaintenanceOperations(options: UseAssetMaintenanceOperationsOptions) {
  const { call } = useApi()
  const toast = createAccountNotifier(useToast())
  const allBackends = '__all__'
  const operationFeedback = ref<AssetMaintenanceOperationFeedback | null>(null)

  const rebuildingAssetId = ref('')
  const sweepingStaging = ref(false)
  const pruneOpen = ref(false)
  const pruningUnreferenced = ref(false)
  const prunePreview = ref<AssetPruneResult | null>(null)
  const pruneForm = reactive<AssetPruneForm>({ olderThanDays: 30, limit: 50 })
  const orphanObjectsOpen = ref(false)
  const auditingOrphanObjects = ref(false)
  const orphanObjectsPreview = ref<AssetOrphanObjectResult | null>(null)
  const orphanObjectsForm = reactive<AssetOrphanObjectForm>({ olderThanDays: 7, limit: 100, backend: allBackends })
  const storageMigrationOpen = ref(false)
  const migratingStorage = ref(false)
  const storageMigrationPreview = ref<AssetStorageMigrationResult | null>(null)
  const storageMigrationForm = reactive<AssetStorageMigrationForm>({ sourceBackend: '', targetBackend: '', limit: 50 })
  const batchRebuildOpen = ref(false)
  const batchRebuilding = ref(false)
  const batchRebuildProfile = ref<AssetProfile | null>(null)
  const batchRebuildPreview = ref<AssetBatchRebuildResult | null>(null)
  const batchRebuildLimit = ref(50)

  async function rebuildDerivatives(asset: AssetItem) {
    rebuildingAssetId.value = asset.id
    operationFeedback.value = null
    try {
      const data = await call<{ generated: number }>(`/api/v1/admin/assets-proxy/library/${asset.id}/derivatives/rebuild`, { method: 'POST' })
      operationFeedback.value = { title: '派生图已重建', description: `生成 ${data.generated ?? 0} 个 Variant`, tone: 'success' }
    } catch (error) {
      toast.add({ title: '重建派生图失败', description: apiErrorMessage(error, { fallback: '暂时无法重建派生图。' }), color: 'error' })
    } finally {
      rebuildingAssetId.value = ''
    }
  }

  async function previewBatchRebuild(profile: AssetProfile, limit = batchRebuildLimit.value) {
    batchRebuildProfile.value = profile
    batchRebuildPreview.value = null
    batchRebuildLimit.value = limit
    batchRebuilding.value = true
    try {
      batchRebuildPreview.value = await call<AssetBatchRebuildResult>('/api/v1/admin/assets-proxy/maintenance/rebuild-derivatives', {
        method: 'POST',
        body: { siteKey: profile.siteKey, profileKey: profile.profileKey, limit, dryRun: true }
      })
      await options.fetchMaintenanceTasks()
      batchRebuildOpen.value = true
    } catch (error) {
      toast.add({ title: '批量重建预检失败', description: apiErrorMessage(error, { fallback: '暂时无法完成批量重建预检。' }), color: 'error' })
    } finally {
      batchRebuilding.value = false
    }
  }

  async function refreshBatchRebuildPreview(limit: number) {
    if (!batchRebuildProfile.value) return
    await previewBatchRebuild(batchRebuildProfile.value, limit)
  }

  async function confirmBatchRebuild(limit = batchRebuildLimit.value) {
    if (!batchRebuildProfile.value) return
    operationFeedback.value = null
    batchRebuildLimit.value = limit
    batchRebuilding.value = true
    try {
      const profile = batchRebuildProfile.value
      const data = await call<AssetBatchRebuildResult>('/api/v1/admin/assets-proxy/maintenance/rebuild-derivatives', {
        method: 'POST',
        body: { siteKey: profile.siteKey, profileKey: profile.profileKey, limit, dryRun: false }
      })
      if (data.task) {
        options.showQueuedTask('批量派生图重建已排队', data.task)
        batchRebuildOpen.value = false
        batchRebuildPreview.value = null
        await options.fetchMaintenanceTasks()
        return
      }
      const failed = data.errors?.length ?? 0
      operationFeedback.value = {
        title: '批量派生图重建完成',
        description: `处理 ${data.rebuilt ?? 0} 个素材，生成 ${data.generated ?? 0} 个 Variant${failed ? `，${failed} 个失败` : ''}`,
        tone: failed ? 'warning' : 'success'
      }
      batchRebuildOpen.value = false
      batchRebuildPreview.value = null
      await options.fetchMaintenanceTasks()
    } catch (error) {
      toast.add({ title: '批量重建失败', description: apiErrorMessage(error, { fallback: '暂时无法批量重建派生图。' }), color: 'error' })
    } finally {
      batchRebuilding.value = false
    }
  }

  async function sweepStaging() {
    operationFeedback.value = null
    sweepingStaging.value = true
    try {
      const data = await call<{ items: AssetSweepResult[], removed: number }>('/api/v1/admin/assets-proxy/maintenance/sweep-staging', { method: 'POST' })
      const skipped = (data.items ?? []).filter(item => item.skipped).length
      const failed = (data.items ?? []).filter(item => item.error).length
      operationFeedback.value = {
        title: '暂存清理完成',
        description: `删除 ${data.removed ?? 0} 个对象${skipped ? `，跳过 ${skipped} 个后端` : ''}${failed ? `，${failed} 个异常` : ''}`,
        tone: failed ? 'warning' : 'success'
      }
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '暂存清理失败', description: apiErrorMessage(error, { fallback: '暂时无法清理暂存文件。' }), color: 'error' })
    } finally {
      sweepingStaging.value = false
    }
  }

  async function previewPruneUnreferenced(value?: AssetPruneForm) {
    if (value) Object.assign(pruneForm, value)
    prunePreview.value = null
    pruningUnreferenced.value = true
    try {
      prunePreview.value = await call<AssetPruneResult>('/api/v1/admin/assets-proxy/maintenance/prune-unreferenced', {
        method: 'POST', body: { ...pruneForm, dryRun: true }
      })
      await options.fetchMaintenanceTasks()
      pruneOpen.value = true
    } catch (error) {
      toast.add({ title: '无引用素材预检失败', description: apiErrorMessage(error, { fallback: '暂时无法完成无引用素材预检。' }), color: 'error' })
    } finally {
      pruningUnreferenced.value = false
    }
  }

  async function confirmPruneUnreferenced(value?: AssetPruneForm) {
    if (value) Object.assign(pruneForm, value)
    operationFeedback.value = null
    pruningUnreferenced.value = true
    try {
      const data = await call<AssetPruneResult>('/api/v1/admin/assets-proxy/maintenance/prune-unreferenced', {
        method: 'POST', body: { ...pruneForm, dryRun: false }
      })
      if (data.task) {
        options.showQueuedTask('无引用素材清理已排队', data.task)
        pruneOpen.value = false
        prunePreview.value = null
        await options.fetchMaintenanceTasks()
        return
      }
      const failed = data.errors?.length ?? 0
      operationFeedback.value = {
        title: '无引用素材清理完成',
        description: `删除 ${data.deleted ?? 0} 个素材${failed ? `，${failed} 个失败` : ''}`,
        tone: failed ? 'warning' : 'success'
      }
      pruneOpen.value = false
      prunePreview.value = null
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '无引用素材清理失败', description: apiErrorMessage(error, { fallback: '暂时无法清理无引用素材。' }), color: 'error' })
    } finally {
      pruningUnreferenced.value = false
    }
  }

  function orphanRequestBody(dryRun: boolean) {
    return {
      olderThanDays: orphanObjectsForm.olderThanDays,
      limit: orphanObjectsForm.limit,
      backend: orphanObjectsForm.backend !== allBackends ? orphanObjectsForm.backend : '',
      dryRun
    }
  }

  async function previewOrphanObjects(value?: AssetOrphanObjectForm) {
    if (value) Object.assign(orphanObjectsForm, value)
    orphanObjectsPreview.value = null
    auditingOrphanObjects.value = true
    try {
      orphanObjectsPreview.value = await call<AssetOrphanObjectResult>('/api/v1/admin/assets-proxy/maintenance/orphan-objects', {
        method: 'POST', body: orphanRequestBody(true)
      })
      await options.fetchMaintenanceTasks()
      orphanObjectsOpen.value = true
    } catch (error) {
      toast.add({ title: '孤儿对象预检失败', description: apiErrorMessage(error, { fallback: '暂时无法完成孤儿对象预检。' }), color: 'error' })
    } finally {
      auditingOrphanObjects.value = false
    }
  }

  async function confirmPruneOrphanObjects(value?: AssetOrphanObjectForm) {
    if (value) Object.assign(orphanObjectsForm, value)
    operationFeedback.value = null
    auditingOrphanObjects.value = true
    try {
      const data = await call<AssetOrphanObjectResult>('/api/v1/admin/assets-proxy/maintenance/orphan-objects', {
        method: 'POST', body: orphanRequestBody(false)
      })
      if (data.task) {
        options.showQueuedTask('孤儿对象清理已排队', data.task)
        orphanObjectsOpen.value = false
        orphanObjectsPreview.value = null
        await options.fetchMaintenanceTasks()
        return
      }
      const failed = (data.items ?? []).reduce((sum, item) => sum + (item.errors?.length ?? 0), 0)
      operationFeedback.value = {
        title: '孤儿对象清理完成',
        description: `删除 ${data.deleted ?? 0} 个对象${failed ? `，${failed} 个失败` : ''}`,
        tone: failed ? 'warning' : 'success'
      }
      orphanObjectsOpen.value = false
      orphanObjectsPreview.value = null
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '孤儿对象清理失败', description: apiErrorMessage(error, { fallback: '暂时无法清理孤儿对象。' }), color: 'error' })
    } finally {
      auditingOrphanObjects.value = false
    }
  }

  function openStorageMigration() {
    const enabled = options.storageBackends.value.filter(backend => backend.enabled !== false)
    storageMigrationForm.sourceBackend = enabled[0]?.name || ''
    storageMigrationForm.targetBackend = enabled.find(backend => backend.name !== storageMigrationForm.sourceBackend)?.name || ''
    storageMigrationForm.limit = 50
    storageMigrationPreview.value = null
    storageMigrationOpen.value = true
  }

  async function previewStorageMigration(value?: AssetStorageMigrationForm) {
    if (value) Object.assign(storageMigrationForm, value)
    storageMigrationPreview.value = null
    migratingStorage.value = true
    try {
      storageMigrationPreview.value = await call<AssetStorageMigrationResult>('/api/v1/admin/assets-proxy/maintenance/migrate-storage', {
        method: 'POST', body: { ...storageMigrationForm, dryRun: true }
      })
      await options.fetchMaintenanceTasks()
    } catch (error) {
      toast.add({ title: '存储迁移预检失败', description: apiErrorMessage(error, { fallback: '暂时无法完成存储迁移预检。' }), color: 'error' })
    } finally {
      migratingStorage.value = false
    }
  }

  async function confirmStorageMigration(value?: AssetStorageMigrationForm) {
    if (value) Object.assign(storageMigrationForm, value)
    operationFeedback.value = null
    migratingStorage.value = true
    try {
      const data = await call<AssetStorageMigrationResult>('/api/v1/admin/assets-proxy/maintenance/migrate-storage', {
        method: 'POST', body: { ...storageMigrationForm, dryRun: false }
      })
      if (data.task) {
        options.showQueuedTask('存储迁移已排队', data.task)
        storageMigrationOpen.value = false
        storageMigrationPreview.value = null
        await options.fetchMaintenanceTasks()
        return
      }
      const failed = data.errors?.length ?? 0
      operationFeedback.value = {
        title: '存储迁移完成',
        description: `迁移 ${data.migrated ?? 0} 个素材${failed ? `，${failed} 个失败` : ''}`,
        tone: failed ? 'warning' : 'success'
      }
      storageMigrationOpen.value = false
      storageMigrationPreview.value = null
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '存储迁移失败', description: apiErrorMessage(error, { fallback: '暂时无法迁移存储对象。' }), color: 'error' })
    } finally {
      migratingStorage.value = false
    }
  }

  return {
    operationFeedback: shallowReadonly(operationFeedback),
    dismissOperationFeedback: () => { operationFeedback.value = null },
    rebuildingAssetId: readonly(rebuildingAssetId),
    sweepingStaging: readonly(sweepingStaging),
    pruneOpen,
    pruningUnreferenced: readonly(pruningUnreferenced),
    prunePreview: shallowReadonly(prunePreview),
    pruneForm,
    orphanObjectsOpen,
    auditingOrphanObjects: readonly(auditingOrphanObjects),
    orphanObjectsPreview: shallowReadonly(orphanObjectsPreview),
    orphanObjectsForm,
    storageMigrationOpen,
    migratingStorage: readonly(migratingStorage),
    storageMigrationPreview: shallowReadonly(storageMigrationPreview),
    storageMigrationForm,
    batchRebuildOpen,
    batchRebuilding: readonly(batchRebuilding),
    batchRebuildProfile: shallowReadonly(batchRebuildProfile),
    batchRebuildPreview: shallowReadonly(batchRebuildPreview),
    batchRebuildLimit: readonly(batchRebuildLimit),
    rebuildDerivatives,
    previewBatchRebuild,
    refreshBatchRebuildPreview,
    confirmBatchRebuild,
    sweepStaging,
    previewPruneUnreferenced,
    confirmPruneUnreferenced,
    previewOrphanObjects,
    confirmPruneOrphanObjects,
    openStorageMigration,
    previewStorageMigration,
    confirmStorageMigration
  }
}
