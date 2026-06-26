<script setup lang="ts">
// Banner-ratio cover crop + upload. Canvas crop ported from the donor
// (nuxtblog/web UserCoverCrop); upload posts the cropped JPEG to the IdP cover
// proxy (cookie-authed), which stores it on the asset service and commits it to
// the profile.
const props = defineProps<{
  editable: boolean
  modelValue: string
}>()

const emit = defineEmits<{ 'update:modelValue': [url: string] }>()

const { call } = useApi()
const toast = useToast()

const COVER_CW = 640
const COVER_CH = 200
const COVER_OUT_W = 1280
const COVER_OUT_H = 255
const COVER_RATIO = COVER_OUT_H / COVER_OUT_W
const HIT = 14

const coverFileInput = ref<HTMLInputElement | null>(null)
const cropModalOpen = ref(false)
const cropCanvas = ref<HTMLCanvasElement | null>(null)
const cropImage = ref<HTMLImageElement | null>(null)
const uploading = ref(false)

let _cIX = 0, _cIY = 0, _cIW = 0, _cIH = 0
let _cCX = 0, _cCY = 0, _cCW = 0

type DragMode = 'move' | 'tl' | 'tr' | 'bl' | 'br' | null
let _cDM: DragMode = null, _cDSX = 0, _cDSY = 0
let _cDR = { x: 0, y: 0, w: 0 }

function _drawHandle(ctx: CanvasRenderingContext2D, x: number, y: number) {
  const s = 9
  ctx.fillStyle = '#fff'
  ctx.fillRect(x - s / 2, y - s / 2, s, s)
  ctx.strokeStyle = 'rgba(0,0,0,0.4)'
  ctx.lineWidth = 1
  ctx.strokeRect(x - s / 2, y - s / 2, s, s)
}

function _toCC(clientX: number, clientY: number, cv: HTMLCanvasElement) {
  const r = cv.getBoundingClientRect()
  return { x: (clientX - r.left) * (COVER_CW / r.width), y: (clientY - r.top) * (COVER_CH / r.height) }
}

function _getCMode(x: number, y: number): DragMode {
  const CH = _cCW * COVER_RATIO
  const corners: [DragMode, number, number][] = [
    ['tl', _cCX, _cCY], ['tr', _cCX + _cCW, _cCY],
    ['bl', _cCX, _cCY + CH], ['br', _cCX + _cCW, _cCY + CH],
  ]
  for (const [m, cx, cy] of corners)
    if (Math.abs(x - cx) < HIT && Math.abs(y - cy) < HIT) return m
  if (x > _cCX && x < _cCX + _cCW && y > _cCY && y < _cCY + CH) return 'move'
  return null
}

function _applyCDrag(x: number, y: number) {
  const dx = x - _cDSX
  const { x: sx, y: sy, w: sw } = _cDR
  const min = 40
  let nx = _cCX, ny = _cCY, nw = _cCW
  if (_cDM === 'move') { nx = sx + dx; ny = sy + (y - _cDSY); nw = sw }
  else if (_cDM === 'br') { nw = Math.max(min, sw + dx); nx = sx; ny = sy }
  else if (_cDM === 'tl') { nw = Math.max(min, sw - dx); nx = sx + sw - nw; ny = sy + (sw - nw) * COVER_RATIO }
  else if (_cDM === 'tr') { nw = Math.max(min, sw + dx); nx = sx; ny = sy + (sw - nw) * COVER_RATIO }
  else if (_cDM === 'bl') { nw = Math.max(min, sw - dx); nx = sx + sw - nw; ny = sy }
  const nh = nw * COVER_RATIO
  nx = Math.max(_cIX, Math.min(_cIX + _cIW - nw, nx))
  ny = Math.max(_cIY, Math.min(_cIY + _cIH - nh, ny))
  nw = Math.min(nw, _cIX + _cIW - nx)
  if (nw * COVER_RATIO > _cIY + _cIH - ny) nw = (_cIY + _cIH - ny) / COVER_RATIO
  _cCX = nx; _cCY = ny; _cCW = Math.max(min, nw)
}

function triggerUpload() { coverFileInput.value?.click() }

function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const img = new Image()
  img.onload = () => {
    cropImage.value = img
    const scale = Math.min(COVER_CW / img.width, COVER_CH / img.height)
    _cIW = img.width * scale
    _cIH = img.height * scale
    _cIX = (COVER_CW - _cIW) / 2
    _cIY = (COVER_CH - _cIH) / 2
    let w = _cIW * 0.9
    if (w * COVER_RATIO > _cIH * 0.9) w = (_cIH * 0.9) / COVER_RATIO
    _cCW = w
    _cCX = _cIX + (_cIW - w) / 2
    _cCY = _cIY + (_cIH - w * COVER_RATIO) / 2
    cropModalOpen.value = true
    nextTick(drawCrop)
  }
  img.src = URL.createObjectURL(file)
  if (coverFileInput.value) coverFileInput.value.value = ''
}

function drawCrop() {
  const canvas = cropCanvas.value
  if (!canvas || !cropImage.value) return
  const ctx = canvas.getContext('2d')!
  const CH = _cCW * COVER_RATIO
  ctx.clearRect(0, 0, COVER_CW, COVER_CH)
  ctx.fillStyle = '#111'
  ctx.fillRect(0, 0, COVER_CW, COVER_CH)
  ctx.drawImage(cropImage.value, _cIX, _cIY, _cIW, _cIH)
  ctx.fillStyle = 'rgba(0,0,0,0.55)'
  ctx.fillRect(0, 0, COVER_CW, _cCY)
  ctx.fillRect(0, _cCY + CH, COVER_CW, COVER_CH - _cCY - CH)
  ctx.fillRect(0, _cCY, _cCX, CH)
  ctx.fillRect(_cCX + _cCW, _cCY, COVER_CW - _cCX - _cCW, CH)
  ctx.strokeStyle = 'rgba(255,255,255,0.9)'
  ctx.lineWidth = 1.5
  ctx.strokeRect(_cCX, _cCY, _cCW, CH)
  ctx.strokeStyle = 'rgba(255,255,255,0.2)'
  ctx.lineWidth = 1
  for (let i = 1; i < 3; i++) {
    ctx.beginPath(); ctx.moveTo(_cCX + (_cCW * i) / 3, _cCY); ctx.lineTo(_cCX + (_cCW * i) / 3, _cCY + CH); ctx.stroke()
    ctx.beginPath(); ctx.moveTo(_cCX, _cCY + (CH * i) / 3); ctx.lineTo(_cCX + _cCW, _cCY + (CH * i) / 3); ctx.stroke()
  }
  _drawHandle(ctx, _cCX, _cCY)
  _drawHandle(ctx, _cCX + _cCW, _cCY)
  _drawHandle(ctx, _cCX, _cCY + CH)
  _drawHandle(ctx, _cCX + _cCW, _cCY + CH)
}

function onMouseDown(e: MouseEvent) {
  const { x, y } = _toCC(e.clientX, e.clientY, cropCanvas.value!)
  _cDM = _getCMode(x, y)
  if (_cDM) { _cDSX = x; _cDSY = y; _cDR = { x: _cCX, y: _cCY, w: _cCW } }
}
function onMouseMove(e: MouseEvent) {
  if (!_cDM) return
  const { x, y } = _toCC(e.clientX, e.clientY, cropCanvas.value!)
  _applyCDrag(x, y); drawCrop()
}
function onMouseUp() { _cDM = null }
function onTouchStart(e: TouchEvent) {
  const t = e.touches[0]!
  const { x, y } = _toCC(t.clientX, t.clientY, cropCanvas.value!)
  _cDM = _getCMode(x, y)
  if (_cDM) { _cDSX = x; _cDSY = y; _cDR = { x: _cCX, y: _cCY, w: _cCW } }
}
function onTouchMove(e: TouchEvent) {
  e.preventDefault()
  if (!_cDM) return
  const t = e.touches[0]!
  const { x, y } = _toCC(t.clientX, t.clientY, cropCanvas.value!)
  _applyCDrag(x, y); drawCrop()
}
function onTouchEnd() { _cDM = null }

async function confirmCrop() {
  if (!cropImage.value) return
  uploading.value = true
  try {
    const CH = _cCW * COVER_RATIO
    const scX = cropImage.value.width / _cIW, scY = cropImage.value.height / _cIH
    const out = document.createElement('canvas')
    out.width = COVER_OUT_W
    out.height = COVER_OUT_H
    out.getContext('2d')!.drawImage(
      cropImage.value,
      (_cCX - _cIX) * scX, (_cCY - _cIY) * scY, _cCW * scX, CH * scY,
      0, 0, COVER_OUT_W, COVER_OUT_H,
    )
    const blob = await new Promise<Blob | null>(r => out.toBlob(r, 'image/jpeg', 0.92))
    if (!blob) throw new Error('裁剪失败')
    const fd = new FormData()
    fd.append('file', new File([blob], 'cover.jpg', { type: 'image/jpeg' }))
    const { coverUrl } = await call<{ coverUrl: string }>('/api/v1/session/cover', { method: 'POST', body: fd })
    emit('update:modelValue', coverUrl)
    toast.add({ title: '封面已更新', color: 'success', icon: 'i-tabler-check' })
    cropModalOpen.value = false
  } catch (err: any) {
    toast.add({ title: '封面上传失败', description: err?.data?.message || err?.message, color: 'error' })
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <UModal v-model:open="cropModalOpen" title="裁剪封面" :ui="{ content: 'max-w-3xl' }">
    <template #body>
      <div class="space-y-4">
        <p class="text-center text-xs text-muted">拖动选框调整裁剪区域(横幅比例)</p>
        <div class="flex justify-center">
          <canvas
            ref="cropCanvas" :width="640" :height="200"
            class="cursor-default touch-none rounded" style="max-width: 100%"
            @mousedown="onMouseDown" @mousemove="onMouseMove" @mouseup="onMouseUp" @mouseleave="onMouseUp"
            @touchstart.prevent="onTouchStart" @touchmove.prevent="onTouchMove" @touchend="onTouchEnd" />
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="ghost" label="取消" :disabled="uploading" @click="cropModalOpen = false" />
        <UButton color="primary" label="确认" :loading="uploading" @click="confirmCrop" />
      </div>
    </template>
  </UModal>

  <div class="relative h-40 overflow-hidden rounded-xl md:h-48" :class="editable ? 'group/cover' : ''">
    <div
      class="absolute inset-0 bg-gradient-to-br from-primary/30 via-primary/10 to-violet-500/20"
      :style="modelValue ? `background-image:url('${modelValue}');background-size:cover;background-position:center` : ''" />
    <div v-if="modelValue" class="absolute inset-0 bg-black/20" />
    <button
      v-if="editable"
      type="button"
      class="absolute right-3 top-3 flex items-center gap-1.5 rounded-md bg-black/40 px-3 py-1.5 text-xs font-medium text-white opacity-0 transition-opacity hover:bg-black/60 group-hover/cover:opacity-100"
      @click="triggerUpload">
      <UIcon :name="uploading ? 'i-tabler-loader-2' : 'i-tabler-camera'" class="size-3.5" :class="uploading ? 'animate-spin' : ''" />
      {{ uploading ? '上传中…' : '更换封面' }}
    </button>
    <input v-if="editable" ref="coverFileInput" type="file" accept="image/*" class="hidden" @change="onFileChange" >
  </div>
</template>
