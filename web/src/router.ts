import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from './views/DashboardView.vue'
import SourcesView from './views/SourcesView.vue'
import TablesView from './views/TablesView.vue'
import TableDetailView from './views/TableDetailView.vue'
import SettingsView from './views/SettingsView.vue'

declare module 'vue-router' {
    interface RouteMeta {
        title: string
    }
}

export const router = createRouter({
    history: createWebHistory('/ui/'),
    routes: [
        { path: '/', redirect: '/dashboard' },
        {
            path: '/dashboard',
            name: 'dashboard',
            component: DashboardView,
            meta: { title: 'Tableau de bord' }
        },
        { path: '/sources', name: 'sources', component: SourcesView, meta: { title: 'Sources' } },
        {
            path: '/tables',
            name: 'tables',
            component: TablesView,
            meta: { title: 'Tables disponibles' }
        },
        {
            path: '/tables/:tableId',
            name: 'table-detail',
            component: TableDetailView,
            props: true,
            meta: { title: 'Table' }
        },
        {
            path: '/settings',
            name: 'settings',
            component: SettingsView,
            meta: { title: 'Paramètres' }
        }
    ]
})
