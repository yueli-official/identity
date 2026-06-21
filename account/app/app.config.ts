import { platformAppConfig } from '@platform/ui/app-config'

// Account center theme = the 'account' preset (teal). Shared neutral/card/icons
// live in @platform/ui. See flightdeck/specs/2026-06-21-platform-ui-theme-layer.md.
export default defineAppConfig(platformAppConfig('account'))
