/// <reference types="vite/client" />

import 'vue-router'
import '@tanstack/vue-table'

declare module 'vue-router' {
    interface RouteMeta {
        title: string
    }
}

// TanStack types a column's meta as an opaque, unaugmented interface — this
// is DataTable.vue's own vocabulary for it, read by its rendering alone.
declare module '@tanstack/vue-table' {
    // Declaration merging with TanStack's own ColumnMeta<TData, TValue> — the
    // two type parameters are required to match the interface being extended,
    // even though this augmentation's own fields don't depend on either.
    // eslint-disable-next-line unused-imports/no-unused-vars
    interface ColumnMeta<TData, TValue> {
        align?: 'right'
        class?: string
        pinned?: string
    }
}
