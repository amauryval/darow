import type { RouteRecordRaw } from 'vue-router'
import { DarowRoutes } from '@/darow/routeNames'

const TablesView = () => import('@/darow/views/TablesView.vue')
const FiltersView = () => import('@/darow/views/FiltersView.vue')
const SourcesView = () => import('@/darow/views/SourcesView.vue')
const SqlView = () => import('@/darow/views/SqlView.vue')
const SettingsView = () => import('@/darow/views/SettingsView.vue')

export const darowRoutes: RouteRecordRaw[] = [
    {
        path: '/tables/:name?',
        name: DarowRoutes.TABLES,
        component: TablesView,
        meta: { titleKey: 'darow.pages.tables' }
    },
    {
        path: '/filters',
        name: DarowRoutes.FILTERS,
        component: FiltersView,
        meta: { titleKey: 'darow.pages.filters' }
    },
    {
        path: '/sources',
        name: DarowRoutes.SOURCES,
        component: SourcesView,
        meta: { titleKey: 'darow.pages.sources' }
    },
    {
        path: '/sql',
        name: DarowRoutes.SQL,
        component: SqlView,
        meta: { admin: true, titleKey: 'darow.pages.sql' }
    },
    {
        path: '/settings',
        name: DarowRoutes.SETTINGS,
        component: SettingsView,
        meta: { admin: true, titleKey: 'darow.pages.settings' }
    },
    { path: '/', redirect: { name: DarowRoutes.TABLES } }
]
