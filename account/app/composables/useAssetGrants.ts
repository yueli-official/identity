import type { AssetGrant, AssetGrantForm, AssetItem, CreatedAssetGrant } from '~/types/asset-admin'
import { createPlatformNotifier } from '@platform/ui/feedback'
import { apiErrorMessage } from '../utils/api-errors'

interface UseAssetGrantsOptions {
  reloadAll: () => Promise<void>
}

export function useAssetGrants(options: UseAssetGrantsOptions) {
  const { call } = useApi()
  const toast = createPlatformNotifier(useToast())
  const grants = ref<AssetGrant[]>([])
  const totalGrants = ref(0)
  const grantPage = ref(1)
  const grantPageSize = 20
  const revokeGrantTarget = ref<AssetGrant | null>(null)
  const revokeGrantOpen = computed({ get: () => !!revokeGrantTarget.value, set: value => { if (!value) revokeGrantTarget.value = null } })
  const revokingGrant = ref(false)
  const createGrantOpen = ref(false)
  const creatingGrant = ref(false)
  const grantAsset = ref<AssetItem | null>(null)
  const createdGrant = ref<CreatedAssetGrant | null>(null)
  const grantForm = reactive<AssetGrantForm>({
    variantKey: 'original', policy: 'oneTime', subjectId: '', expiresIn: 3600, maxUses: 1, reason: 'admin-issued'
  })

  watch(grantPage, fetchGrants)

  async function fetchGrants() {
    const data = await call<{ items: AssetGrant[], total: number }>('/api/v1/admin/assets-proxy/grants', {
      params: { page: grantPage.value, size: grantPageSize }
    })
    grants.value = data.items ?? []
    totalGrants.value = data.total ?? 0
  }

  function requestRevokeGrant(grant: AssetGrant) {
    revokeGrantTarget.value = grant
  }

  async function confirmRevokeGrant() {
    if (!revokeGrantTarget.value) return
    revokingGrant.value = true
    try {
      await call(`/api/v1/admin/assets-proxy/grants/${revokeGrantTarget.value.id}/revoke`, { method: 'POST' })
      revokeGrantTarget.value = null
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '撤销授权失败', description: apiErrorMessage(error, { fallback: '暂时无法撤销该授权。' }), color: 'error' })
    } finally {
      revokingGrant.value = false
    }
  }

  function openCreateGrant(asset: AssetItem) {
    grantAsset.value = asset
    createdGrant.value = null
    Object.assign(grantForm, {
      variantKey: 'original',
      policy: asset.deliveryPolicy && asset.deliveryPolicy !== 'public' ? asset.deliveryPolicy : 'oneTime',
      subjectId: '', expiresIn: 3600, maxUses: 1, reason: `admin:${asset.filename || asset.id}`
    })
    createGrantOpen.value = true
  }

  async function createGrant(value: AssetGrantForm) {
    if (!grantAsset.value) return
    Object.assign(grantForm, value)
    creatingGrant.value = true
    try {
      createdGrant.value = await call<CreatedAssetGrant>('/api/v1/admin/assets-proxy/grants/create', {
        method: 'POST', body: { ...grantForm, assetId: grantAsset.value.id }
      })
      await fetchGrants()
    } catch (error) {
      toast.add({ title: '生成交付链接失败', description: apiErrorMessage(error, { fallback: '暂时无法生成交付链接。' }), color: 'error' })
    } finally {
      creatingGrant.value = false
    }
  }

  return {
    grants: shallowReadonly(grants), totalGrants: readonly(totalGrants), grantPage, grantPageSize, fetchGrants,
    revokeGrantTarget: shallowReadonly(revokeGrantTarget), revokeGrantOpen, revokingGrant: readonly(revokingGrant), requestRevokeGrant, confirmRevokeGrant,
    createGrantOpen, creatingGrant: readonly(creatingGrant), grantAsset: shallowReadonly(grantAsset), createdGrant: shallowReadonly(createdGrant),
    grantForm, openCreateGrant, createGrant
  }
}
