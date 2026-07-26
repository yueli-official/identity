<script setup lang="ts">
import { createPlatformNotifier } from '@platform/ui/feedback'

// Square avatar crop + upload. The canvas crop interaction is ported from the
// donor (nuxtblog/web UserAvatarCrop); the upload posts the cropped JPEG to the
// IdP avatar proxy (cookie-authed), which stores it on the asset service and
// commits it to the profile, returning the public URL.
defineProps<{
  editable: boolean
  modelValue: string
  initial?: string
}>()

const emit = defineEmits<{ 'update:modelValue': [url: string] }>()

const { call } = useApi()
const toast = createPlatformNotifier(useToast())

const AVATAR_C = 320
const HIT = 14

const avatarFileInput = ref<HTMLInputElement | null>(null)
const cropModalOpen = ref(false)
const cropCanvas = ref<HTMLCanvasElement | null>(null)
const cropImage = ref<HTMLImageElement | null>(null)
const uploading = ref(false)

let _aIX = 0, _aIY = 0, _aIW = 0, _aIH = 0
let _aCX = 0, _aCY = 0, _aCZ = 0

type DragMode = 'move' | 'tl' | 'tr' | 'bl' | 'br' | null
let _aDM: DragMode = null, _aDSX = 0, _aDSY = 0
let _aDR = { x: 0, y: 0, sz: 0 }

function _toC(clientX: number, clientY: number, cv: HTMLCanvasElement) {
  const r = cv.getBoundingClientRect()
  return { x: (clientX - r.left) * (AVATAR_C / r.width), y: (clientY - r.top) * (AVATAR_C / r.height) }
}

function _drawHandle(ctx: CanvasRenderingContext2D, x: number, y: number) {
  const s = 9
  ctx.fillStyle = '#fff'
  ctx.fillRect(x - s / 2, y - s / 2, s, s)
  ctx.strokeStyle = 'rgba(0,0,0,0.4)'
  ctx.lineWidth = 1
  ctx.strokeRect(x - s / 2, y - s / 2, s, s)
}

function _getAMode(x: number, y: number): DragMode {
  const corners: [DragMode, number, number][] = [
    ['tl', _aCX, _aCY], ['tr', _aCX + _aCZ, _aCY],
    ['bl', _aCX, _aCY + _aCZ], ['br', _aCX + _aCZ, _aCY + _aCZ],
  ]
  for (const [m, cx, cy] of corners)
    if (Math.abs(x - cx) < HIT && Math.abs(y - cy) < HIT) return m
  if (x > _aCX && x < _aCX + _aCZ && y > _aCY && y < _aCY + _aCZ) return 'move'
  return null
}

function _applyADrag(x: number, y: number) {
  const dx = x - _aDSX, dy = y - _aDSY
  const { x: sx, y: sy, sz: ssz } = _aDR
  const min = 40
  let nx = _aCX, ny = _aCY, nsz = _aCZ
  if (_aDM === 'move') { nx = sx + dx; ny = sy + dy; nsz = ssz }
  else if (_aDM === 'br') { nsz = Math.max(min, ssz + (dx + dy) / 2); nx = sx; ny = sy }
  else if (_aDM === 'tl') { nsz = Math.max(min, ssz - (dx + dy) / 2); nx = sx + ssz - nsz; ny = sy + ssz - nsz }
  else if (_aDM === 'tr') { nsz = Math.max(min, ssz + (dx - dy) / 2); nx = sx; ny = sy + ssz - nsz }
  else if (_aDM === 'bl') { nsz = Math.max(min, ssz + (-dx + dy) / 2); nx = sx + ssz - nsz; ny = sy }
  nx = Math.max(_aIX, Math.min(_aIX + _aIW - nsz, nx))
  ny = Math.max(_aIY, Math.min(_aIY + _aIH - nsz, ny))
  nsz = Math.min(nsz, _aIX + _aIW - nx, _aIY + _aIH - ny)
  _aCX = nx; _aCY = ny; _aCZ = Math.max(min, nsz)
}

function triggerUpload() { avatarFileInput.value?.click() }

function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const img = new Image()
  img.onload = () => {
    cropImage.value = img
    const scale = Math.min(AVATAR_C / img.width, AVATAR_C / img.height)
    _aIW = img.width * scale
    _aIH = img.height * scale
    _aIX = (AVATAR_C - _aIW) / 2
    _aIY = (AVATAR_C - _aIH) / 2
    const sz = Math.min(_aIW, _aIH) * 0.85
    _aCZ = sz
    _aCX = _aIX + (_aIW - sz) / 2
    _aCY = _aIY + (_aIH - sz) / 2
    cropModalOpen.value = true
    nextTick(drawCrop)
  }
  img.src = URL.createObjectURL(file)
  if (avatarFileInput.value) avatarFileInput.value.value = ''
}

function drawCrop() {
  const canvas = cropCanvas.value
  if (!canvas || !cropImage.value) return
  const ctx = canvas.getContext('2d')!
  const C = AVATAR_C
  ctx.clearRect(0, 0, C, C)
  ctx.fillStyle = '#111'
  ctx.fillRect(0, 0, C, C)
  ctx.drawImage(cropImage.value, _aIX, _aIY, _aIW, _aIH)
  ctx.fillStyle = 'rgba(0,0,0,0.55)'
  ctx.fillRect(0, 0, C, _aCY)
  ctx.fillRect(0, _aCY + _aCZ, C, C - _aCY - _aCZ)
  ctx.fillRect(0, _aCY, _aCX, _aCZ)
  ctx.fillRect(_aCX + _aCZ, _aCY, C - _aCX - _aCZ, _aCZ)
  ctx.strokeStyle = 'rgba(255,255,255,0.9)'
  ctx.lineWidth = 1.5
  ctx.strokeRect(_aCX, _aCY, _aCZ, _aCZ)
  ctx.strokeStyle = 'rgba(255,255,255,0.2)'
  ctx.lineWidth = 1
  for (let i = 1; i < 3; i++) {
    ctx.beginPath(); ctx.moveTo(_aCX + (_aCZ * i) / 3, _aCY); ctx.lineTo(_aCX + (_aCZ * i) / 3, _aCY + _aCZ); ctx.stroke()
    ctx.beginPath(); ctx.moveTo(_aCX, _aCY + (_aCZ * i) / 3); ctx.lineTo(_aCX + _aCZ, _aCY + (_aCZ * i) / 3); ctx.stroke()
  }
  _drawHandle(ctx, _aCX, _aCY)
  _drawHandle(ctx, _aCX + _aCZ, _aCY)
  _drawHandle(ctx, _aCX, _aCY + _aCZ)
  _drawHandle(ctx, _aCX + _aCZ, _aCY + _aCZ)
}

function onMouseDown(e: MouseEvent) {
  const { x, y } = _toC(e.clientX, e.clientY, cropCanvas.value!)
  _aDM = _getAMode(x, y)
  if (_aDM) { _aDSX = x; _aDSY = y; _aDR = { x: _aCX, y: _aCY, sz: _aCZ } }
}
function onMouseMove(e: MouseEvent) {
  if (!_aDM) return
  const { x, y } = _toC(e.clientX, e.clientY, cropCanvas.value!)
  _applyADrag(x, y); drawCrop()
}
function onMouseUp() { _aDM = null }
function onTouchStart(e: TouchEvent) {
  const t = e.touches[0]!
  const { x, y } = _toC(t.clientX, t.clientY, cropCanvas.value!)
  _aDM = _getAMode(x, y)
  if (_aDM) { _aDSX = x; _aDSY = y; _aDR = { x: _aCX, y: _aCY, sz: _aCZ } }
}
function onTouchMove(e: TouchEvent) {
  e.preventDefault()
  if (!_aDM) return
  const t = e.touches[0]!
  const { x, y } = _toC(t.clientX, t.clientY, cropCanvas.value!)
  _applyADrag(x, y); drawCrop()
}
function onTouchEnd() { _aDM = null }

async function confirmCrop() {
  if (!cropImage.value) return
  uploading.value = true
  try {
    const OUT = 400
    const scX = cropImage.value.width / _aIW, scY = cropImage.value.height / _aIH
    const out = document.createElement('canvas')
    out.width = out.height = OUT
    out.getContext('2d')!.drawImage(
      cropImage.value,
      (_aCX - _aIX) * scX, (_aCY - _aIY) * scY, _aCZ * scX, _aCZ * scY,
      0, 0, OUT, OUT,
    )
    const blob = await new Promise<Blob | null>(r => out.toBlob(r, 'image/jpeg', 0.92))
    if (!blob) throw new Error('裁剪失败')
    const fd = new FormData()
    fd.append('file', new File([blob], 'avatar.jpg', { type: 'image/jpeg' }))
    const { avatarUrl } = await call<{ avatarUrl: string }>('/api/v1/session/avatar', { method: 'POST', body: fd })
    emit('update:modelValue', avatarUrl)
    cropModalOpen.value = false
  } catch (err: any) {
    toast.add({
      title: '头像上传失败',
      description: identityErrorMessage(err, {
        context: 'profile',
        fallback: '暂时无法上传头像。',
      }),
      color: 'error',
    })
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <UModal v-model:open="cropModalOpen" title="裁剪头像" :ui="{ content: 'max-w-sm' }">
    <template #body>
      <div class="space-y-4">
        <p class="text-center text-xs text-muted">拖动选框调整裁剪区域</p>
        <div class="flex justify-center">
          <canvas
            ref="cropCanvas" :width="320" :height="320"
            class="cursor-default touch-none" style="max-width: 100%"
            @mousedown="onMouseDown" @mousemove="onMouseMove" @mouseup="onMouseUp" @mouseleave="onMouseUp"
            @touchstart.prevent="onTouchStart" @touchmove.prevent="onTouchMove" @touchend="onTouchEnd" />
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="ghost" label="取消" :disabled="uploading" @click="() => { cropModalOpen = false }" />
        <UButton color="primary" label="确认" :loading="uploading" @click="confirmCrop" />
      </div>
    </template>
  </UModal>

  <div
    v-if="editable"
    class="group/avatar relative size-24 shrink-0 cursor-pointer rounded-full ring-2 ring-primary/20"
    @click="triggerUpload">
    <UAvatar :src="modelValue || undefined" :text="initial" size="3xl" class="size-full" />
    <div class="absolute inset-0 flex flex-col items-center justify-center gap-0.5 rounded-full bg-black/50 opacity-0 transition-opacity group-hover/avatar:opacity-100">
      <UIcon name="i-tabler-camera" class="size-5 text-white" />
      <span class="text-[10px] font-medium text-white">更换头像</span>
    </div>
    <input ref="avatarFileInput" type="file" accept="image/*" class="hidden" @change="onFileChange" >
  </div>
  <UAvatar v-else :src="modelValue || undefined" :text="initial" size="3xl" />
</template>
