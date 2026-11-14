import type { FrontModule } from '@myapplibs/web-platform/module'
import { darowMessages } from './i18n'
import { icons } from './icons'
import { DarowRoutes } from './routeNames'
import { darowRoutes } from './router/routes'
import { setupDarowEvents } from './events'

export const darowModule: FrontModule = {
    name: 'darow',
    labelKey: 'darow.nav.app',
    icon: icons.App,
    messages: darowMessages,
    routes: darowRoutes,
    nav: {
        main: [
            { name: DarowRoutes.TABLES, labelKey: 'darow.nav.tables', icon: icons.Tables },
            { name: DarowRoutes.FILTERS, labelKey: 'darow.nav.filters', icon: icons.Filter },
            { name: DarowRoutes.SQL, labelKey: 'darow.nav.sql', icon: icons.Sql }
        ],
        config: [
            { name: DarowRoutes.SOURCES, labelKey: 'darow.nav.sources', icon: icons.Sources },
            { name: DarowRoutes.SETTINGS, labelKey: 'darow.nav.settings', icon: icons.Settings }
        ]
    },
    setup: setupDarowEvents
}
