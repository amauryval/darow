import type { Meta } from '@myapplibs/web-platform/api/document.gen'
import { checkStatus, getJson, sendJson } from '@myapplibs/web-platform/api/http'
import { APIV1Root } from '@myapplibs/web-platform/api/paths'
import { resourceApi } from '@myapplibs/web-platform/api/resource'
import { sendForm, type SendFormOptions } from '@myapplibs/web-platform/api/sendForm'
import type { Rows, Table, TransformPreview } from './engine.gen'
import type { Filter } from './filters.gen'
import type {
    LoadRequest,
    RowRequest,
    RowsRequest,
    TableDetail,
    TransformResult,
    WKT,
    WrittenRows
} from './rest.gen'
import type { Effective, TableSettings, Values } from './settings.gen'
import type { Source } from './sources.gen'
import type { RowSort } from '@/darow/utils/rowValues'

export const DAROW_ROOT = `${APIV1Root}/darow`

export type FilterInput = Omit<Filter, keyof Meta<string>>

export type ExportFormat = 'csv' | 'geojson'

export interface RowPage {
    offset: number
    limit: number
    where: string
    sort: RowSort
}

const tableUrl = (name: string) => `${DAROW_ROOT}/tables/${encodeURIComponent(name)}`
const sourceUrl = (name: string) => `${DAROW_ROOT}/sources/${encodeURIComponent(name)}`

const withQuery = (url: string, params: Record<string, string>) => {
    const query = new URLSearchParams(Object.entries(params).filter(([, value]) => value !== ''))
    return query.size ? `${url}?${query}` : url
}

const sendFor = async <T>(url: string, method: string, body?: unknown): Promise<T> => {
    const response = await sendJson(url, method, body)
    await checkStatus(response)
    return response.json()
}

const sendEmpty = async (url: string, method: string): Promise<void> => {
    await checkStatus(await sendJson(url, method))
}

export const listTables = () => getJson<Table[]>(`${DAROW_ROOT}/tables`)

export const getTable = (name: string) => getJson<TableDetail>(tableUrl(name))

export const renameTable = (name: string, to: string) =>
    sendFor<Table>(tableUrl(name), 'PATCH', { name: to })

export const deleteTable = (name: string) => sendEmpty(tableUrl(name), 'DELETE')

export const deriveTable = (name: string, sql: string) =>
    sendFor<Table>(`${DAROW_ROOT}/tables`, 'POST', { name, sql })

export const listRows = (name: string, page: RowPage) =>
    getJson<Rows>(
        withQuery(`${tableUrl(name)}/rows`, {
            offset: String(page.offset),
            limit: String(page.limit),
            where: page.where,
            sort: page.sort.column,
            desc: page.sort.desc ? 'true' : ''
        })
    )

export const getRow = (name: string, id: number) =>
    getJson<Record<string, unknown>>(`${tableUrl(name)}/rows/${id}`)

export const getRowWkt = (name: string, id: number, srid: number) =>
    getJson<WKT>(withQuery(`${tableUrl(name)}/rows/${id}/wkt`, { srid: srid ? String(srid) : '' }))

export const insertRow = (name: string, request: RowRequest) =>
    sendFor<Record<string, unknown>>(`${tableUrl(name)}/rows`, 'POST', request)

export const updateRow = (name: string, id: number, request: RowRequest) =>
    sendFor<Record<string, unknown>>(`${tableUrl(name)}/rows/${id}`, 'PATCH', request)

export const writeRows = (name: string, request: RowsRequest) =>
    sendFor<WrittenRows>(`${tableUrl(name)}/rows`, 'PATCH', request)

export const deleteRow = (name: string, id: number) =>
    sendEmpty(`${tableUrl(name)}/rows/${id}`, 'DELETE')

export const tilesUrl = (name: string, where: string, revision?: number) =>
    withQuery(`${window.location.origin}${tableUrl(name)}/tiles/{z}/{x}/{y}`, {
        where,
        v: revision === undefined ? '' : String(revision)
    })

export const exportUrl = (name: string, format: ExportFormat, where: string, srid: number) =>
    withQuery(`${window.location.origin}${tableUrl(name)}/export`, {
        format,
        where,
        srid: srid ? String(srid) : ''
    })

export const ogcServiceUrl = () => `${window.location.origin}${DAROW_ROOT}/ogc`

export const listSources = () => getJson<Source[]>(`${DAROW_ROOT}/sources`)

export const uploadSource = async (file: File, options: SendFormOptions) => {
    const form = new FormData()
    form.append('file', file)
    const response = await sendForm(`${DAROW_ROOT}/sources`, form, options)
    await checkStatus(response)
}

export const deleteSource = (name: string) => sendEmpty(sourceUrl(name), 'DELETE')

export const sourceLayers = (name: string) => getJson<string[]>(`${sourceUrl(name)}/layers`)

export const loadSource = (name: string, request: LoadRequest) =>
    sendFor<Table>(`${sourceUrl(name)}/load`, 'POST', request)

export const filtersApi = resourceApi<Filter, FilterInput>(`${DAROW_ROOT}/filters`)

const columnUrl = (name: string, column: string) =>
    `${tableUrl(name)}/columns/${encodeURIComponent(column)}`

export const addColumn = (name: string, column: string, type: string) =>
    sendFor<Table>(`${tableUrl(name)}/columns`, 'POST', { name: column, type })

export const renameColumn = (name: string, column: string, to: string) =>
    sendFor<Table>(columnUrl(name, column), 'PATCH', { name: to })

export const dropColumn = (name: string, column: string) =>
    sendFor<Table>(columnUrl(name, column), 'DELETE')

export const transformColumn = (name: string, column: string, expression: string, where: string) =>
    sendFor<TransformResult>(`${columnUrl(name, column)}/transform`, 'POST', { expression, where })

export const previewTransform = (name: string, column: string, expression: string, where: string) =>
    sendFor<TransformPreview>(`${columnUrl(name, column)}/transform/preview`, 'POST', {
        expression,
        where
    })

export const getSettings = () => getJson<Effective>(`${DAROW_ROOT}/settings`)

export const putSettings = (values: Values) =>
    sendFor<Effective>(`${DAROW_ROOT}/settings`, 'PUT', values)

export const getTableSettings = (name: string) =>
    getJson<TableSettings>(`${tableUrl(name)}/settings`)

export const putTableSettings = (name: string, values: Values) =>
    sendFor<TableSettings>(`${tableUrl(name)}/settings`, 'PUT', values)
