import { defineStore } from 'pinia'
import { ref } from 'vue'
import { useResource } from '@myapplibs/web-platform/stores/useResource'
import { filtersApi } from '@/darow/api/darow'

export const useFiltersStore = defineStore('darow-filters', () => {
    const docs = useResource(filtersApi)
    const loaded = ref(false)

    const fetch = async () => {
        await docs.fetch()
        loaded.value = true
    }

    return { ...docs, fetch, loaded }
})
