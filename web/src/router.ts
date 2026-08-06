import { createRouter, createWebHistory } from 'vue-router'
import SourcesView from './views/SourcesView.vue'
import TablesView from './views/TablesView.vue'
import TableDetailView from './views/TableDetailView.vue'

declare module 'vue-router' {
    interface RouteMeta {
        title: string
    }
}

export const router = createRouter({
    history: createWebHistory('/ui/'),
    routes: [
        { path: '/', redirect: '/tables' },
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
        }
    ]
})
