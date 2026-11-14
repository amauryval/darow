import { defineStore } from 'pinia'
import { ref, shallowRef } from 'vue'
import type { Table } from '@/darow/api/engine.gen'
import { listTables } from '@/darow/api/darow'

export const useTablesStore = defineStore('darow-tables', () => {
    const items = shallowRef<Table[]>([])
    const loaded = ref(false)

    const fetch = async () => {
        items.value = await listTables()
        loaded.value = true
    }

    const get = (name: string) => items.value.find((item) => item.name === name)

    return { items, loaded, fetch, get }
})
