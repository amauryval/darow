import { useEvents } from '@myapplibs/web-platform/events/useEvents'
import { useFiltersStore } from './stores/filters'
import { useSourcesStore } from './stores/sources'
import { useTablesStore } from './stores/tables'

export const setupDarowEvents = () => {
    const { subscribe } = useEvents()
    subscribe('darow-tables', () => {
        const store = useTablesStore()
        if (store.loaded) void store.fetch()
    })
    subscribe('darow-sources', () => {
        const store = useSourcesStore()
        if (store.loaded) void store.fetch()
    })
    subscribe('darow-filters', () => {
        const store = useFiltersStore()
        if (store.loaded) void store.fetch()
    })
}
