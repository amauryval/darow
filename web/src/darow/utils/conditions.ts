import type { Column } from '@/darow/api/engine.gen'

export const ID_COLUMN = '__id'

export const isGeometry = (column: Column): boolean => column.type.startsWith('GEOMETRY')

export const attributeColumns = (columns: Column[]): Column[] =>
    columns.filter((column) => !isGeometry(column))

export const sqlIdentifier = (name: string): string => `"${name.replaceAll('"', '""')}"`

export const columnIdentifier = (table: string, column: string): string =>
    `${sqlIdentifier(table)}.${sqlIdentifier(column)}`

// insertToken writes a token at the end of a condition, a space before it and after it.
export const insertToken = (where: string, token: string): string =>
    where.trim() ? `${where.trimEnd()} ${token} ` : `${token} `

// andAll keeps the rows passing every condition given, the blank ones left out.
export const andAll = (conditions: string[]): string => {
    const kept = conditions.map((condition) => condition.trim()).filter(Boolean)
    return kept.length === 1 ? kept[0] : kept.map((condition) => `(${condition})`).join(' AND ')
}

export const selectionClause = (ids: number[]): string =>
    `${sqlIdentifier(ID_COLUMN)} IN (${ids.join(', ')})`

// filterFits tells whether a saved filter is offered on a table: saved on it, or on any table.
export const filterFits = (filter: { table: string }, table: string): boolean =>
    filter.table === '' || filter.table === table

// distinctColumns are the columns of the tables, each name once, for a filter of any table.
export const distinctColumns = (tables: { columns: Column[] }[]): Column[] => {
    const seen = new Map<string, Column>()
    for (const column of tables.flatMap((table) => table.columns))
        if (!seen.has(column.name)) seen.set(column.name, column)
    return [...seen.values()]
}
