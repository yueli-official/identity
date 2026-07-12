import type { AssetItem, AssetReference } from '~/types/asset-admin'

interface UseAssetLibraryActionsOptions {
  reloadAll: () => Promise<void>
}

export function useAssetLibraryActions(options: UseAssetLibraryActionsOptions) {
  const { call } = useApi()
  const toast = useToast()
  const references = ref<AssetReference[]>([])
  const referenceAsset = ref<AssetItem | null>(null)
  const referencesOpen = computed({ get: () => !!referenceAsset.value, set: value => { if (!value) referenceAsset.value = null } })
  const loadingReferences = ref(false)
  const deleteAssetTarget = ref<AssetItem | null>(null)
  const deleteAssetOpen = computed({ get: () => !!deleteAssetTarget.value, set: value => { if (!value) deleteAssetTarget.value = null } })
  const deletingAsset = ref(false)

  function requestDeleteAsset(asset: AssetItem) {
    deleteAssetTarget.value = asset
  }

  async function confirmDeleteAsset() {
    if (!deleteAssetTarget.value) return
    deletingAsset.value = true
    try {
      await call(`/api/v1/admin/assets-proxy/library/${deleteAssetTarget.value.id}`, { method: 'DELETE' })
      toast.add({ title: '素材已删除', color: 'success', icon: 'i-tabler-check' })
      deleteAssetTarget.value = null
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '删除素材失败', description: (error as Error)?.message, color: 'error' })
    } finally {
      deletingAsset.value = false
    }
  }

  async function openReferences(asset: AssetItem) {
    referenceAsset.value = asset
    references.value = []
    loadingReferences.value = true
    try {
      const data = await call<{ items: AssetReference[] }>('/api/v1/admin/assets-proxy/references', {
        params: { assetId: asset.id, page: 1, size: 50 }
      })
      references.value = data.items ?? []
    } catch (error) {
      toast.add({ title: '加载引用失败', description: (error as Error)?.message, color: 'error' })
    } finally {
      loadingReferences.value = false
    }
  }

  return {
    references: shallowReadonly(references), referenceAsset: shallowReadonly(referenceAsset), referencesOpen,
    loadingReferences: readonly(loadingReferences), openReferences,
    deleteAssetTarget: shallowReadonly(deleteAssetTarget), deleteAssetOpen, deletingAsset: readonly(deletingAsset),
    requestDeleteAsset, confirmDeleteAsset
  }
}
