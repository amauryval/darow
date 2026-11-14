import { watch } from 'vue'
import { t } from '@myapplibs/web-platform/i18n'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import {
    useCellChanges,
    type CellChanges,
    type TableChanges
} from '@myapplibs/web-platform/components/table/useCellChanges'
import * as api from '@/darow/api/darow'
import { useTable } from './useTable'

const rowsOf = (changes: CellChanges) =>
    Object.entries(changes).map(([id, row]) => ({
        id: Number(id),
        values: Object.fromEntries(
            Object.entries(row).map(([column, text]) => [column, text === '' ? null : text])
        )
    }))

const requestOf = ({ cells, rows, columns }: TableChanges) => ({
    rows: rowsOf(cells),
    deleted: rows.map(Number),
    dropped: columns
})

// useCellEditing writes the cells changed, the rows and columns removed in the grid in one
// request, and stops with another table.
export function useCellEditing() {
    const table = useTable()
    const { attempt } = useAttempt()

    const cells = useCellChanges((changes) =>
        attempt(t('darow.row.saveFailed'), async () => {
            await api.writeRows(table.name.value, requestOf(changes))
            await table.changed()
            return t('darow.cellEdit.saved')
        })
    )

    watch(table.name, cells.close)

    return cells
}
