import { uiPreset } from '@platform/ui/app-config'

export default defineAppConfig({
  ui: {
    ...uiPreset,
    colors: { ...uiPreset.colors, primary: 'teal' } // account center = teal #14b8a6
  }
})
