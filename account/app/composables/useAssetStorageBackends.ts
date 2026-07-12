import type {
  AssetStorageBackend,
  AssetStorageBackendDetail,
  AssetStorageBackendEvent,
  AssetStorageBackendForm
} from '~/types/asset-admin'

interface UseAssetStorageBackendsOptions {
  reloadAll: () => Promise<void>
}

export function useAssetStorageBackends(options: UseAssetStorageBackendsOptions) {
  const { call } = useApi()
  const toast = useToast()

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
    endpoint: '',
    region: 'us-east-1',
    bucketPublic: '',
    bucketPrivate: '',
    accessKey: '',
    secretKey: '',
    publicBaseUrl: '',
    pathStyle: true,
    useSsl: false
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
    Object.assign(storageBackendForm, {
      name: detail?.name || '',
      type: detail?.type || 's3',
      enabled: detail?.enabled ?? true,
      endpoint: detail?.endpoint || '',
      region: detail?.region || 'us-east-1',
      bucketPublic: detail?.bucketPublic || '',
      bucketPrivate: detail?.bucketPrivate || '',
      accessKey: detail?.accessKey || '',
      secretKey: '',
      publicBaseUrl: detail?.publicBaseUrl || '',
      pathStyle: detail?.pathStyle ?? true,
      useSsl: detail?.useSsl ?? false
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
      toast.add({ title: '加载存储后端失败', description: (error as Error)?.message, color: 'error' })
    } finally {
      loadingStorageBackend.value = false
    }
  }

  async function saveStorageBackend(value: AssetStorageBackendForm) {
    Object.assign(storageBackendForm, value)
    savingStorageBackend.value = true
    try {
      await call('/api/v1/admin/assets-proxy/storage-backends', { method: 'POST', body: storageBackendForm })
      toast.add({ title: '存储后端已保存', color: 'success', icon: 'i-tabler-check' })
      storageBackendOpen.value = false
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '保存存储后端失败', description: (error as Error)?.message, color: 'error' })
    } finally {
      savingStorageBackend.value = false
    }
  }

  async function confirmDeleteStorageBackend() {
    if (!storageBackendEditingName.value) return
    deletingStorageBackend.value = true
    try {
      await call(`/api/v1/admin/assets-proxy/storage-backends/${storageBackendEditingName.value}`, { method: 'DELETE' })
      toast.add({ title: '存储后端已删除', color: 'success', icon: 'i-tabler-check' })
      deleteStorageBackendOpen.value = false
      storageBackendOpen.value = false
      storageBackendEditingName.value = ''
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '删除存储后端失败', description: (error as Error)?.message, color: 'error' })
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
      toast.add({
        title: data.backend.lastHealthOk === false ? '健康检查失败' : '健康检查通过',
        description: data.backend.lastHealthError || undefined,
        color: data.backend.lastHealthOk === false ? 'error' : 'success',
        icon: data.backend.lastHealthOk === false ? 'i-tabler-alert-triangle' : 'i-tabler-heartbeat'
      })
      await Promise.all([reloadStorageBackends(), fetchStorageBackendEvents()])
    } catch (error) {
      toast.add({ title: '健康检查失败', description: (error as Error)?.message, color: 'error' })
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
      toast.add({ title: '密钥已轮换', color: 'success', icon: 'i-tabler-key' })
      await Promise.all([reloadStorageBackends(), fetchStorageBackendEvents()])
    } catch (error) {
      toast.add({ title: '密钥轮换失败', description: (error as Error)?.message, color: 'error' })
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
