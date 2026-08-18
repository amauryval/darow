import { ref } from 'vue'
import { listTableProvenance } from '../client/tables'
import type { TableProvenance } from '../client/types'
import { errorMessage } from '../client/http'

// The catalogue's provenance index: which table came from which workDir file.
// The sources screen reads it to show what is already loaded, and to propose a
// free name for the next load.
export function useTableProvenance() {
    const entries = ref<TableProvenance[]>([])
    const error = ref<string | null>(null)

    async function reload(): Promise<void> {
        try {
            const page = await listTableProvenance()
            entries.value = page.tables
            error.value = null
        } catch (e) {
            error.value = errorMessage(e, 'Impossible de lire les tables déjà chargées')
        }
    }

    // A layer's tables are those of its own layer, a plain file's those with no
    // layer at all — the two never mix within one source.
    function tablesOf(sourceName: string, layerName?: string): TableProvenance[] {
        return entries.value.filter(
            (entry) =>
                entry.sourceName === sourceName && (entry.sourceLayer ?? '') === (layerName ?? '')
        )
    }

    function tableNames(): string[] {
        return entries.value.map((entry) => entry.tableName)
    }

    return { entries, error, reload, tablesOf, tableNames }
}
