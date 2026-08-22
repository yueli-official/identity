import { apiErrorMessage } from '../utils/api-errors'

import type {
  AssetStorageBackend,
  AssetStorageBackendDetail,
  AssetStorageBackendEvent,
  AssetStorageBackendForm
} from '~/types/asset-admin'
import { createAccountNotifier } from '~/utils/feedback'
import {
  normalizeStorageBackendForSubmit,
  storageBackendDefaultsForType,
} from '~/utils/asset-storage-backend'

interface UseAssetStorageBackendsOptions {
  reloadAll: () => Promise<void>
}

export function useAssetStorageBackends(options: UseAssetStorageBackendsOptions) {
  const { call } = useApi()
  const toast = createAccountNotifier(useToast())

  const storageBackends = ref<AssetStorageBackend[]>([])
  const storageBackendOpen = ref(false)
  const savingStorageBackend = ref(false)
  const loadingStorageBackend = ref(false)
  const storageBackendEditingName = ref('')
  const storageBackendDetail = ref<AssetStorageBackendDetail | null>(null)
  const storageBackendEvents = ref<AssetStorageBackendEvent[]>([])
  const checkingStorageBackend = ref(false)
  const rotatingStorageBackend = ref(false)
  const rotateStorageBackendOpen = ref(false)
  const deleteStorageBackendOpen = ref(false)
  const deletingStorageBackend = ref(false)
  const storageBackendForm = reactive<AssetStorageBackendForm>({
    name: '',
    type: 's3',
    enabled: true,
    ...storageBackendDefaultsForType('s3')
  })

  const storageBackendTypeItems = [
    { label: 'S3 兼容', value: 's3' },
    { label: '腾讯云 COS', value: 'cos' },
    { label: '阿里云 OSS', value: 'oss' }
  ]

  function setStorageBackends(value: AssetStorageBackend[]) {
    storageBackends.value = value
  }

  function assignStorageBackendForm(detail?: Partial<AssetStorageBackendDetail>) {
    const type = detail?.type || 's3'
    const defaults = storageBackendDefaultsForType(type)
    Object.assign(storageBackendForm, {
      name: detail?.name || '',
      type,
      enabled: detail?.enabled ?? true,
      endpoint: detail?.endpoint || defaults.endpoint,
      region: detail?.region || defaults.region,
      bucketPublic: detail?.bucketPublic || defaults.bucketPublic,
      bucketPrivate: detail?.bucketPrivate || defaults.bucketPrivate,
      accessKey: detail?.accessKey || defaults.accessKey,
      secretKey: defaults.secretKey,
      publicBaseUrl: detail?.publicBaseUrl || defaults.publicBaseUrl,
      pathStyle: detail?.pathStyle ?? defaults.pathStyle,
      useSsl: detail?.useSsl ?? defaults.useSsl
    })
  }

  async function fetchStorageBackendEvents(name = storageBackendEditingName.value) {
    if (!name) {
      storageBackendEvents.value = []
      return
    }
    const data = await call<{ items: AssetStorageBackendEvent[] }>(`/api/v1/admin/assets-proxy/storage-backends/${encodeURIComponent(name)}/events`)
    storageBackendEvents.value = data.items ?? []
  }

  async function reloadStorageBackends() {
    const data = await call<{ items: AssetStorageBackend[] }>('/api/v1/admin/assets-proxy/storage-backends')
    storageBackends.value = data.items ?? []
  }

  async function editStorageBackend(backend?: AssetStorageBackend) {
    storageBackendEditingName.value = backend?.managed ? backend.name : ''
    storageBackendDetail.value = null
    storageBackendEvents.value = []
    assignStorageBackendForm({
      name: backend?.name || '',
      type: backend?.type || 's3',
      enabled: backend?.enabled !== false
    })
    storageBackendOpen.value = true
    if (!backend?.managed) return

    loadingStorageBackend.value = true
    try {
      const data = await call<{ backend: AssetStorageBackendDetail }>(`/api/v1/admin/assets-proxy/storage-backends/${backend.name}`)
      assignStorageBackendForm(data.backend)
      storageBackendDetail.value = data.backend
      await fetchStorageBackendEvents(backend.name)
    } catch (error) {
      toast.add({ title: '加载存储后端失败', description: apiErrorMessage(error, { fallback: '暂时无法加载存储后端。' }), color: 'error' })
    } finally {
      loadingStorageBackend.value = false
    }
  }

  async function saveStorageBackend(value: AssetStorageBackendForm) {
    Object.assign(storageBackendForm, normalizeStorageBackendForSubmit(value))
    savingStorageBackend.value = true
    try {
      await call('/api/v1/admin/assets-proxy/storage-backends', { method: 'POST', body: storageBackendForm })
      storageBackendOpen.value = false
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '保存存储后端失败', description: apiErrorMessage(error, { fallback: '暂时无法保存存储后端。' }), color: 'error' })
    } finally {
      savingStorageBackend.value = false
    }
  }

  async function confirmDeleteStorageBackend() {
    if (!storageBackendEditingName.value) return
    deletingStorageBackend.value = true
    try {
      await call(`/api/v1/admin/assets-proxy/storage-backends/${storageBackendEditingName.value}`, { method: 'DELETE' })
      deleteStorageBackendOpen.value = false
      storageBackendOpen.value = false
      storageBackendEditingName.value = ''
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '删除存储后端失败', description: apiErrorMessage(error, { fallback: '暂时无法删除存储后端。' }), color: 'error' })
    } finally {
      deletingStorageBackend.value = false
    }
  }

  async function checkStorageBackendHealth() {
    if (!storageBackendEditingName.value) return
    checkingStorageBackend.value = true
    try {
      const data = await call<{ backend: AssetStorageBackendDetail }>(
        `/api/v1/admin/assets-proxy/storage-backends/${encodeURIComponent(storageBackendEditingName.value)}/health-check`,
        { method: 'POST' }
      )
      storageBackendDetail.value = data.backend
      if (data.backend.lastHealthOk === false) {
        toast.add({ title: '健康检查失败', description: data.backend.lastHealthError || undefined, color: 'error', icon: 'i-tabler-alert-triangle' })
      }
      await Promise.all([reloadStorageBackends(), fetchStorageBackendEvents()])
    } catch (error) {
      toast.add({ title: '健康检查失败', description: apiErrorMessage(error, { fallback: '暂时无法完成健康检查。' }), color: 'error' })
    } finally {
      checkingStorageBackend.value = false
    }
  }

  async function confirmRotateStorageBackendSecret(secret: string) {
    if (!storageBackendEditingName.value || !secret) return
    rotatingStorageBackend.value = true
    try {
      const data = await call<{ backend: AssetStorageBackendDetail }>(
        `/api/v1/admin/assets-proxy/storage-backends/${encodeURIComponent(storageBackendEditingName.value)}/rotate-secret`,
        { method: 'POST', body: { secretKey: secret } }
      )
      storageBackendDetail.value = data.backend
      rotateStorageBackendOpen.value = false
      await Promise.all([reloadStorageBackends(), fetchStorageBackendEvents()])
    } catch (error) {
      toast.add({ title: '密钥轮换失败', description: apiErrorMessage(error, { fallback: '暂时无法轮换存储密钥。' }), color: 'error' })
    } finally {
      rotatingStorageBackend.value = false
    }
  }

  return {
    storageBackends: readonly(storageBackends),
    setStorageBackends,
    storageBackendOpen,
    savingStorageBackend: readonly(savingStorageBackend),
    loadingStorageBackend: readonly(loadingStorageBackend),
    storageBackendEditingName: readonly(storageBackendEditingName),
    storageBackendDetail: readonly(storageBackendDetail),
    storageBackendEvents: readonly(storageBackendEvents),
    checkingStorageBackend: readonly(checkingStorageBackend),
    rotatingStorageBackend: readonly(rotatingStorageBackend),
    rotateStorageBackendOpen,
    deleteStorageBackendOpen,
    deletingStorageBackend: readonly(deletingStorageBackend),
    storageBackendForm,
    storageBackendTypeItems,
    editStorageBackend,
    saveStorageBackend,
    confirmDeleteStorageBackend,
    fetchStorageBackendEvents,
    checkStorageBackendHealth,
    confirmRotateStorageBackendSecret
  }
}
