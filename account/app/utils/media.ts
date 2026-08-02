export interface MediaRef {
  mediaKey: string
}

const MEDIA_KEY = /^[0-9A-Za-z]{20,32}$/
const RENDITION = /^[a-z0-9]+(?:-[a-z0-9]+)*$/

export function userMediaUrl(reference: MediaRef | null | undefined, rendition: string): string {
  const mediaKey = reference?.mediaKey || ''
  if (!MEDIA_KEY.test(mediaKey) || !RENDITION.test(rendition)) return ''
  return `/media/${mediaKey}?format=webp&name=${rendition}`
}
