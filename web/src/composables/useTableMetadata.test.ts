import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { useTableMetadata } from './useTableMetadata'
import { flush, withSetup } from '../test/fixtures'
import {
    addColumn,
    deleteTable,
    dropColumn,
    getSchema,
    getTable,
    renameColumn,
    renameTable
} from '../client/tables'
import type { Metadata } from '../client/types'

vi.mock('../client/tables', () => ({
    getTable: vi.fn(),
    getSchema: vi.fn(),
    renameTable: vi.fn(),
    deleteTable: vi.fn(),
    addColumn: vi.fn(),
    renameColumn: vi.fn(),
    dropColumn: vi.fn()
}))

const readTable = vi.mocked(getTable)
const readSchema = vi.mocked(getSchema)
const rename = vi.mocked(renameTable)
const remove = vi.mocked(deleteTable)
const add = vi.mocked(addColumn)
const renameCol = vi.mocked(renameColumn)
const drop = vi.mocked(dropColumn)

function metadata(tableName = 'routes'): Metadata {
    return {
        tableId: 't1',
        physicalName: 't1',
        tableName,
        isGeo: true,
        geometryColumn: 'geom',
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
        rowCount: 3,
        srid: 4326
    }
}

const schema = {
    columns: [
        { name: '__pkey', type: 'VARCHAR' },
        { name: 'OGC_FID', type: 'BIGINT' },
        { name: 'name', type: 'VARCHAR' },
        { name: 'geom', type: 'GEOMETRY' }
    ]
}

function setup() {
    const tableId = ref('t1')
    const { result } = withSetup(() => useTableMetadata(tableId))
    return { tableId, ...result }
}

describe('useTableMetadata', () => {
    beforeEach(() => {
        readTable.mockResolvedValue(metadata())
        readSchema.mockResolvedValue(schema)
    })

    it('loads the metadata and the schema as soon as it is pointed at a table', async () => {
        const { table, columns, loadedOnce, loading } = setup()
        expect(loading.value).toBe(true)

        await flush()

        expect(table.value).toEqual(metadata())
        expect(columns.value).toEqual(schema.columns)
        expect(loadedOnce.value).toBe(true)
        expect(loading.value).toBe(false)
    })

    it('reloads when the route moves to another table', async () => {
        const { tableId } = setup()
        await flush()

        tableId.value = 't2'
        await nextTick()
        await flush()

        expect(readTable).toHaveBeenLastCalledWith('t2')
        expect(readSchema).toHaveBeenLastCalledWith('t2')
    })

    it('exposes the initial failure as a retryable state rather than a toast', async () => {
        readTable.mockRejectedValueOnce(new Error('introuvable'))
        const { error, loadedOnce, retry } = setup()
        await flush()

        expect(error.value).toBe('Impossible de charger la table : introuvable')
        expect(loadedOnce.value).toBe(false)

        await retry()
        expect(error.value).toBe('')
        expect(loadedOnce.value).toBe(true)
    })

    it('names the writable columns, leaving out the server-managed ones', async () => {
        const { editableColumns } = setup()
        await flush()

        expect(editableColumns.value).toEqual(['name', 'geom'])
    })

    it('leaves the geometry column out of the columns a row displays', async () => {
        const { rowColumns } = setup()
        await flush()

        expect(rowColumns.value).toEqual(['name'])
    })

    it('renames the table on the answer the server gives', async () => {
        const { table, rename: renameTableName } = setup()
        await flush()
        rename.mockResolvedValue(metadata('grandes_routes'))

        await renameTableName('grandes_routes')

        expect(rename).toHaveBeenCalledWith('t1', 'grandes_routes')
        expect(table.value?.tableName).toBe('grandes_routes')
    })

    it('rethrows a refused mutation — the call site decides how to show it', async () => {
        const { rename: renameTableName } = setup()
        await flush()
        rename.mockRejectedValue(new Error('nom déjà pris'))

        await expect(renameTableName('routes')).rejects.toThrow('nom déjà pris')
    })

    it('deletes the table', async () => {
        const { remove: removeTable } = setup()
        await flush()
        remove.mockResolvedValue({ id: 't1' })

        await removeTable()

        expect(remove).toHaveBeenCalledWith('t1')
    })

    it('reloads the schema after every column mutation', async () => {
        const { addNewColumn, renameExistingColumn, removeColumn } = setup()
        await flush()
        add.mockResolvedValue({ name: 'surface', type: 'DOUBLE' })
        renameCol.mockResolvedValue({ name: 'aire' })
        drop.mockResolvedValue({ name: 'aire' })

        await addNewColumn('surface', 'DOUBLE')
        await renameExistingColumn('surface', 'aire')
        await removeColumn('aire')

        expect(add).toHaveBeenCalledWith('t1', 'surface', 'DOUBLE')
        expect(renameCol).toHaveBeenCalledWith('t1', 'surface', 'aire')
        expect(drop).toHaveBeenCalledWith('t1', 'aire')
        expect(readSchema).toHaveBeenCalledTimes(4)
    })
})
