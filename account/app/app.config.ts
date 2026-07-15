import { platformAppConfig } from '@platform/ui/app-config'

// Account center theme = the 'account' preset (teal); shared theme primitives
// stay in @platform/ui so companion apps do not fork the platform palette.
export default defineAppConfig(platformAppConfig('account'))
