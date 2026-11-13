import type { Column } from '@/darow/api/engine.gen'

export interface RowSort {
    column: string
    desc: boolean
}

export const COLUMN_PREFIX = 'col-'
export const NO_SORT: RowSort = { column: '', desc: false }

// rowSortOf reads the sort of the rows' table: a column id is the prefix and the column's name.
export const rowSortOf = (sorting: { id: string; desc: boolean }[]): RowSort => {
    const [first] = sorting
    if (!first) return NO_SORT
    const column = first.id.startsWith(COLUMN_PREFIX)
        ? first.id.slice(COLUMN_PREFIX.length)
        : first.id
    return { column, desc: first.desc }
}

export const sameSort = (a: RowSort, b: RowSort) => a.column === b.column && a.desc === b.desc

export const textOf = (value: unknown): string => {
    if (value === null || value === undefined) return ''
    if (typeof value === 'object') return JSON.stringify(value)
    return String(value)
}

export const draftOf = (columns: Column[], row: Record<string, unknown>): Record<string, string> =>
    Object.fromEntries(columns.map((column) => [column.name, textOf(row[column.name])]))

export const changedValues = (
    columns: Column[],
    row: Record<string, unknown>,
    draft: Record<string, string>
): Record<string, string | null> => {
    const changed: Record<string, string | null> = {}
    for (const column of columns) {
        const value = draft[column.name] ?? ''
        if (value !== textOf(row[column.name])) changed[column.name] = value === '' ? null : value
    }
    return changed
}

export const rowIdOf = (row: Record<string, unknown>): number | null => {
    const id = row.__id
    return typeof id === 'number' ? id : null
}
