import { uiPreset } from '@platform/ui/app-config'

export default defineAppConfig({
  ui: {
    ...uiPreset,
    colors: { ...uiPreset.colors, primary: 'teal' }, // account center = teal #14b8a6
    // Project rule: all icons are Tabler (i-tabler-*). Remap Nuxt UI's built-in
    // component icons (which default to lucide) so the chrome matches authored
    // icons and the app renders fully offline with @iconify-json/tabler bundled.
    icons: {
      arrowLeft: 'i-tabler-arrow-left',
      arrowRight: 'i-tabler-arrow-right',
      check: 'i-tabler-check',
      chevronDoubleLeft: 'i-tabler-chevrons-left',
      chevronDoubleRight: 'i-tabler-chevrons-right',
      chevronDown: 'i-tabler-chevron-down',
      chevronLeft: 'i-tabler-chevron-left',
      chevronRight: 'i-tabler-chevron-right',
      chevronUp: 'i-tabler-chevron-up',
      close: 'i-tabler-x',
      ellipsis: 'i-tabler-dots',
      external: 'i-tabler-external-link',
      loading: 'i-tabler-loader-2',
      minus: 'i-tabler-minus',
      plus: 'i-tabler-plus',
      search: 'i-tabler-search',
      light: 'i-tabler-sun',
      dark: 'i-tabler-moon',
      system: 'i-tabler-device-desktop'
    }
  }
})
