import { computed, ref } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import { isAbortError } from '@myapplibs/web-platform/api/sendForm'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import { useConfirm } from '@myapplibs/web-platform/components/composables/useConfirm'
import type { LoadRequest } from '@/darow/api/rest.gen'
import * as api from '@/darow/api/darow'
import { useSourcesStore } from '@/darow/stores/sources'
import { useTablesStore } from '@/darow/stores/tables'

const loading = ref(false)
const progress = ref<number | null>(null)
let upload: AbortController | null = null

export function useSources() {
    const store = useSourcesStore()
    const tables = useTablesStore()
    const { attempt } = useAttempt()
    const { confirmDialog } = useConfirm()

    const load = async () => {
        if (!store.loaded) await attempt(t('darow.sources.loadFailed'), store.fetch, loading)
    }

    const send = (file: File) => {
        upload = new AbortController()
        progress.value = 0
        return attempt(t('darow.sources.uploadFailed'), async () => {
            try {
                await api.uploadSource(file, {
                    signal: upload?.signal,
                    onProgress: (fraction) => (progress.value = fraction)
                })
                await store.fetch()
                return t('darow.sources.uploaded', { name: file.name })
            } catch (error) {
                if (!isAbortError(error)) throw error
            } finally {
                progress.value = null
            }
        })
    }

    const cancel = () => upload?.abort()

    const remove = async (name: string): Promise<boolean> => {
        if (!(await confirmDialog(t('darow.sources.confirmDelete', { name })))) return false
        return attempt(t('darow.sources.deleteFailed'), async () => {
            await api.deleteSource(name)
            await store.fetch()
            return t('darow.sources.deleted')
        })
    }

    const layers = async (name: string): Promise<string[]> => {
        let found: string[] = []
        await attempt(t('darow.sources.layersFailed'), async () => {
            found = await api.sourceLayers(name)
        })
        return found
    }

    const loadInto = (name: string, request: LoadRequest) =>
        attempt(t('darow.sources.loadIntoFailed'), async () => {
            await api.loadSource(name, request)
            await tables.fetch()
            return t('darow.sources.loaded', { table: request.table })
        })

    return {
        sources: computed(() => store.items),
        loading,
        progress,
        load,
        send,
        cancel,
        remove,
        layers,
        loadInto
    }
}
