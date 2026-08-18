import { deriveTable } from '../client/tables'
import { useAsyncAction } from './useAsyncAction'

// Free-SQL derivation: a top-level route, scoped to no table (TODO-F38) — the
// query may read any registered table. Split from `useDeriveTable`, whose
// guided mode is scoped to one source table and has a spec to build rather
// than a bare query string.
export function useDeriveFromQuery() {
    const { isLoading, error, run } = useAsyncAction()

    function submit(query: string, newTableName: string) {
        return run(() => deriveTable(query, newTableName), 'Impossible de dériver la table')
    }

    return { isLoading, error, submit }
}
