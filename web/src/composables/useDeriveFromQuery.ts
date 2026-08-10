import { ref } from 'vue'
import { deriveTable } from '../client/tables'
import { errorMessage } from '../client/http'

// Free-SQL derivation: a top-level route, scoped to no table (TODO-F38) — the
// query may read any registered table. Split from `useDeriveTable`, whose
// guided mode is scoped to one source table and has a spec to build rather
// than a bare query string.
export function useDeriveFromQuery() {
    const isLoading = ref(false)
    const error = ref('')

    async function submit(query: string, newTableName: string) {
        isLoading.value = true
        error.value = ''
        try {
            return await deriveTable(query, newTableName)
        } catch (e) {
            error.value = errorMessage(e, 'Impossible de dériver la table')
            return null
        } finally {
            isLoading.value = false
        }
    }

    return { isLoading, error, submit }
}
