import { defineStore } from 'pinia'
import { ref, shallowRef } from 'vue'
import type { Source } from '@/darow/api/sources.gen'
import { listSources } from '@/darow/api/darow'

export const useSourcesStore = defineStore('darow-sources', () => {
    const items = shallowRef<Source[]>([])
    const loaded = ref(false)

    const fetch = async () => {
        items.value = await listSources()
        loaded.value = true
    }

    return { items, loaded, fetch }
})
