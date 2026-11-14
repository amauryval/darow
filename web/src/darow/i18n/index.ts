import type { LocaleMessages } from '@myapplibs/web-platform/i18n'
import { fr, type DarowMessages } from './fr'

export const darowMessages: LocaleMessages<DarowMessages> = { fr }

declare module '@myapplibs/web-platform/i18n/keys' {
    interface Catalog {
        darow: DarowMessages
    }
}
