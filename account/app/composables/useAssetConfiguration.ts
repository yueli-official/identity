import type { Ref } from 'vue'
import { createPlatformNotifier } from '@platform/ui/feedback'
import type {
  AssetProfile,
  AssetSite,
  AssetStorageBackend,
  AssetVariant
} from '~/types/asset-admin'

interface AssetConfigurationData {
  sites: AssetSite[]
  profiles: AssetProfile[]
  variants: AssetVariant[]
}

interface UseAssetConfigurationOptions {
  storageBackends: Readonly<Ref<readonly AssetStorageBackend[]>>
  currentSiteKey: Ref<string>
  allSiteKey: string
  reloadAll: () => Promise<void>
}

export function useAssetConfiguration(options: UseAssetConfigurationOptions) {
  const { call } = useApi()
  const toast = createPlatformNotifier(useToast())
  const sites = ref<AssetSite[]>([])
  const profiles = ref<AssetProfile[]>([])
  const variants = ref<AssetVariant[]>([])

  const profileOpen = ref(false)
  const savingProfile = ref(false)
  const profileForm = reactive<AssetProfile>({
    siteKey: 'platform', profileKey: '', purpose: '', storageBackend: '', allowedExt: 'jpg,jpeg,png,webp',
    maxSizeBytes: 20 * 1024 * 1024, defaultVisibility: 'public', defaultDeliveryPolicy: 'public', keepOriginal: true,
    assetCount: 0, variantCount: 0
  })
  const siteOpen = ref(false)
  const savingSite = ref(false)
  const siteForm = reactive<AssetSite>({
    siteKey: '', name: '', defaultStorageBackend: 'local', enabled: true, assetCount: 0, profileCount: 0, variantCount: 0
  })
  const variantOpen = ref(false)
  const savingVariant = ref(false)
  const variantForm = reactive<AssetVariant>({
    id: '', siteKey: 'platform', profileKey: 'default', variantKey: '', width: 800, height: 600,
    mode: 'resize', format: 'source', quality: 85, version: 1, enabled: true
  })
  const deleteVariantTarget = ref<AssetVariant | null>(null)
  const deleteVariantOpen = computed({ get: () => !!deleteVariantTarget.value, set: value => { if (!value) deleteVariantTarget.value = null } })
  const deletingVariant = ref(false)
  const deleteProfileTarget = ref<AssetProfile | null>(null)
  const deleteProfileOpen = computed({ get: () => !!deleteProfileTarget.value, set: value => { if (!value) deleteProfileTarget.value = null } })
  const deletingProfile = ref(false)

  watch(() => profileForm.defaultVisibility, (visibility) => {
    if (visibility === 'public') {
      profileForm.defaultDeliveryPolicy = 'public'
      return
    }
    if (!profileForm.defaultDeliveryPolicy || profileForm.defaultDeliveryPolicy === 'public') {
      profileForm.defaultDeliveryPolicy = 'signed'
    }
  })

  function setConfigurationData(data: AssetConfigurationData) {
    sites.value = data.sites
    profiles.value = data.profiles
    variants.value = data.variants
  }

  function editProfile(profile?: AssetProfile) {
    Object.assign(profileForm, profile ?? {
      siteKey: sites.value[0]?.siteKey || 'platform', profileKey: '', purpose: '', storageBackend: '',
      allowedExt: 'jpg,jpeg,png,webp', maxSizeBytes: 20 * 1024 * 1024, defaultVisibility: 'public',
      defaultDeliveryPolicy: 'public', keepOriginal: true, assetCount: 0, variantCount: 0
    })
    profileOpen.value = true
  }

  async function saveProfile(value: AssetProfile) {
    Object.assign(profileForm, value)
    profileForm.defaultDeliveryPolicy = profileForm.defaultVisibility === 'public' ? 'public' : 'signed'
    savingProfile.value = true
    try {
      await call('/api/v1/admin/assets-proxy/profiles', { method: 'POST', body: { ...profileForm } })
      profileOpen.value = false
      await options.reloadAll()
    } finally {
      savingProfile.value = false
    }
  }

  function editSite(site?: AssetSite) {
    Object.assign(siteForm, site ?? {
      siteKey: '', name: '',
      defaultStorageBackend: options.storageBackends.value.find(backend => backend.isDefault)?.name || options.storageBackends.value[0]?.name || 'local',
      enabled: true, assetCount: 0, profileCount: 0, variantCount: 0
    })
    siteOpen.value = true
  }

  async function saveSite(value: AssetSite) {
    Object.assign(siteForm, value)
    savingSite.value = true
    try {
      await call('/api/v1/admin/assets-proxy/sites', { method: 'POST', body: siteForm })
      siteOpen.value = false
      await options.reloadAll()
    } finally {
      savingSite.value = false
    }
  }

  function editVariant(variant?: AssetVariant, profile?: AssetProfile) {
    const fallbackSite = options.currentSiteKey.value !== options.allSiteKey ? options.currentSiteKey.value : 'platform'
    Object.assign(variantForm, variant ?? {
      id: '', siteKey: profile?.siteKey || fallbackSite, profileKey: profile?.profileKey || 'default', variantKey: '',
      width: 800, height: 600, mode: 'resize', format: 'source', quality: 85, version: 1, enabled: true
    })
    variantOpen.value = true
  }

  async function saveVariant(value: AssetVariant) {
    Object.assign(variantForm, value)
    savingVariant.value = true
    try {
      await call('/api/v1/admin/assets-proxy/variants', { method: 'POST', body: variantForm })
      variantOpen.value = false
      await options.reloadAll()
    } finally {
      savingVariant.value = false
    }
  }

  function requestDeleteVariant(variant: AssetVariant) {
    deleteVariantTarget.value = variant
  }

  async function confirmDeleteVariant() {
    if (!deleteVariantTarget.value) return
    deletingVariant.value = true
    try {
      await call(`/api/v1/admin/assets-proxy/variants/${deleteVariantTarget.value.id}`, { method: 'DELETE' })
      deleteVariantTarget.value = null
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '删除 Variant 失败', description: (error as Error)?.message, color: 'error' })
    } finally {
      deletingVariant.value = false
    }
  }

  function requestDeleteProfile(profile: AssetProfile) {
    deleteProfileTarget.value = profile
  }

  async function confirmDeleteProfile() {
    if (!deleteProfileTarget.value) return
    deletingProfile.value = true
    try {
      const target = deleteProfileTarget.value
      await call(`/api/v1/admin/assets-proxy/profiles/${target.siteKey}/${target.profileKey}`, { method: 'DELETE' })
      deleteProfileTarget.value = null
      await options.reloadAll()
    } catch (error) {
      toast.add({ title: '删除 Profile 失败', description: (error as Error)?.message, color: 'error' })
    } finally {
      deletingProfile.value = false
    }
  }

  return {
    sites: shallowReadonly(sites), profiles: shallowReadonly(profiles), variants: shallowReadonly(variants), setConfigurationData,
    profileOpen, savingProfile: readonly(savingProfile), profileForm, editProfile, saveProfile,
    siteOpen, savingSite: readonly(savingSite), siteForm, editSite, saveSite,
    variantOpen, savingVariant: readonly(savingVariant), variantForm, editVariant, saveVariant,
    deleteVariantTarget: shallowReadonly(deleteVariantTarget), deleteVariantOpen, deletingVariant: readonly(deletingVariant), requestDeleteVariant, confirmDeleteVariant,
    deleteProfileTarget: shallowReadonly(deleteProfileTarget), deleteProfileOpen, deletingProfile: readonly(deletingProfile), requestDeleteProfile, confirmDeleteProfile
  }
}
