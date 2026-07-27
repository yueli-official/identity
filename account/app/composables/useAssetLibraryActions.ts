import type { AssetItem, AssetReference, AssetSecurityDetail } from '~/types/asset-admin'
import { createAccountNotifier } from '~/utils/feedback'
import { apiErrorMessage } from '../utils/api-errors'

interface UseAssetLibraryActionsOptions {
  reloadAll: () => Promise<void>
}

export function useAssetLibraryActions(options: UseAssetLibraryActionsOptions) {
  const { call } = useApi()
  const toast = createAccountNotifier(useToast())
  const references = ref<AssetReference[]>([])
  const referenceAsset = ref<AssetItem | null>(null)
  const referencesOpen = computed({ get: () => !!referenceAsset.value, set: value => { if (!value) referenceAsset.value = null } })
  const loadingReferences = ref(false)
  const deleteAssetTarget = ref<AssetItem | null>(null)
  const deleteAssetOpen = computed({ get: () => !!deleteAssetTarget.value, set: value => { if (!value) deleteAssetTarget.value = null } })
  const deletingAsset = ref(false)
  const securityAsset = ref<AssetItem | null>(null)
  const securityDetail = ref<AssetSecurityDetail | null>(null)
  const securityOpen = computed({
    get: () => !!securityAsset.value,
    set: (value) => {
      if (!value) {
        securityAsset.value = null
        securityDetail.value = null
      }
    }
  })
  const loadingSecurity = ref(false)
  const retryingSecurity = ref(false)
  const securityRejectTarget = ref<AssetItem | null>(null)
  const securityRejectOpen = computed({
    get: () => !!securityRejectTarget.value,
    set: value => { if (!value) securityRejectTarget.value = null }
  })
  const rejectingSecurity = ref(false)

  function requestDeleteAsset(asset: AssetItem) {
    deleteAssetTarget.value = asset
  }

  async function confirmDeleteAsset() {
    if (!deleteAssetTarget.value) return
    deletingAsset.value = true
    try {
      await call(`/api/v1/admin/assets-proxy/library/${deleteAssetTarget.value.id}`, { method: 'DELETE' })
      deleteAssetTarget.value = null
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '删除素材失败', description: apiErrorMessage(error, { fallback: '暂时无法删除该素材。' }), color: 'error' })
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
      toast.add({ title: '加载引用失败', description: apiErrorMessage(error, { fallback: '暂时无法加载素材引用。' }), color: 'error' })
    } finally {
      loadingReferences.value = false
    }
  }

  async function loadSecurity(asset: AssetItem) {
    loadingSecurity.value = true
    try {
      const data = await call<{ security: AssetSecurityDetail }>(
        `/api/v1/admin/assets-proxy/library/${asset.id}/security`,
        { params: { limit: 20 } }
      )
      securityDetail.value = data.security
      securityAsset.value = data.security.asset
    } catch (error) {
      toast.add({ title: '加载安全详情失败', description: apiErrorMessage(error, { fallback: '暂时无法加载安全详情。' }), color: 'error' })
    } finally {
      loadingSecurity.value = false
    }
  }

  async function openSecurity(asset: AssetItem) {
    securityAsset.value = asset
    securityDetail.value = null
    await loadSecurity(asset)
  }

  async function retrySecurity() {
    if (!securityAsset.value) return
    retryingSecurity.value = true
    try {
      await call(`/api/v1/admin/assets-proxy/library/${securityAsset.value.id}/security/retry`, { method: 'POST' })
      // feedback-contract: the operation refreshes both the security drawer and library, so no single inline surface persists.
      toast.add({ title: '已重新进入安全处理队列', color: 'success' })
      await Promise.all([loadSecurity(securityAsset.value), options.reloadAll()])
    } catch (error) {
      toast.add({ title: '重新处理失败', description: apiErrorMessage(error, { fallback: '暂时无法重新处理该素材。' }), color: 'error' })
    } finally {
      retryingSecurity.value = false
    }
  }

  function deleteFromSecurity() {
    if (!securityAsset.value) return
    const asset = securityAsset.value
    securityOpen.value = false
    requestDeleteAsset(asset)
  }

  function requestRejectSecurity() {
    if (!securityAsset.value) return
    securityRejectTarget.value = securityAsset.value
    securityOpen.value = false
  }

  async function confirmRejectSecurity(reason: string) {
    if (!securityRejectTarget.value) return
    rejectingSecurity.value = true
    try {
      await call(`/api/v1/admin/assets-proxy/library/${securityRejectTarget.value.id}/security/reject`, {
        method: 'POST',
        body: { reason }
      })
      // feedback-contract: rejection closes the security flow and refreshes the library, leaving no persistent inline target.
      toast.add({ title: '已拒绝素材交付', color: 'success' })
      securityRejectTarget.value = null
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '拒绝素材失败', description: apiErrorMessage(error, { fallback: '暂时无法拒绝该素材。' }), color: 'error' })
    } finally {
      rejectingSecurity.value = false
    }
  }

  return {
    references: shallowReadonly(references), referenceAsset: shallowReadonly(referenceAsset), referencesOpen,
    loadingReferences: readonly(loadingReferences), openReferences,
    deleteAssetTarget: shallowReadonly(deleteAssetTarget), deleteAssetOpen, deletingAsset: readonly(deletingAsset),
    requestDeleteAsset, confirmDeleteAsset,
    securityAsset: shallowReadonly(securityAsset), securityDetail: shallowReadonly(securityDetail), securityOpen,
    loadingSecurity: readonly(loadingSecurity), retryingSecurity: readonly(retryingSecurity),
    openSecurity, retrySecurity, deleteFromSecurity,
    securityRejectTarget: shallowReadonly(securityRejectTarget), securityRejectOpen,
    rejectingSecurity: readonly(rejectingSecurity), requestRejectSecurity, confirmRejectSecurity
  }
}
