import { del, get, post, postForm } from './http'
import type { Deleted, SourceCRS, SourceLimits, SourcesPage, UploadResult } from './types'

export function listSources(offset: number, limit: number, sort?: string): Promise<SourcesPage> {
    const sortParam = sort ? `&sort=${encodeURIComponent(sort)}` : ''
    return get(`/sources?offset=${offset}&limit=${limit}${sortParam}`)
}

export function refreshSources(offset: number, limit: number, sort?: string): Promise<SourcesPage> {
    const sortParam = sort ? `&sort=${encodeURIComponent(sort)}` : ''
    return post(`/sources/refresh?offset=${offset}&limit=${limit}${sortParam}`)
}

// The size cap and the extension whitelist an upload is checked against
// server-side, both config-only (sources.maxSizeBytes/allowedExtensions).
export function getSourceLimits(): Promise<SourceLimits> {
    return get('/sources/limits')
}

// layer is required for a multi-layer (.gpkg) source, omitted otherwise. srid
// pins the source CRS, required if getSourceMetadata reports a null srid.
// tableName names the created table; replaceTableId reloads that table's data
// in place instead of creating a second one.
export interface LoadOptions {
    layer?: string
    srid?: number
    tableName?: string
    replaceTableId?: string
}

export function loadSource(name: string, options: LoadOptions = {}): Promise<{ tableId: string }> {
    const params = new URLSearchParams()
    if (options.layer) params.set('layer', options.layer)
    if (options.srid) params.set('srid', String(options.srid))
    if (options.tableName) params.set('tableName', options.tableName)
    if (options.replaceTableId) params.set('replace', options.replaceTableId)
    const query = params.toString() ? `?${params.toString()}` : ''
    return post(`/sources/${encodeURIComponent(name)}/load${query}`)
}

// Previews a source's detected CRS, so the caller can show it to the user
// (with an editable override) before loading.
export function getSourceMetadata(name: string, layer?: string): Promise<SourceCRS> {
    const query = layer ? `?layer=${encodeURIComponent(layer)}` : ''
    return get(`/sources/${encodeURIComponent(name)}/metadata${query}`)
}

// Writes file into workDir and returns the rescanned Source, layers included.
// Loading it into a table is a separate call to loadSource.
export function uploadSource(file: File): Promise<UploadResult> {
    const form = new FormData()
    form.append('file', file)
    return postForm('/sources/upload', form)
}

// Erases the source file from workDir. The tables already loaded from it are
// kept — they hold their own copy of the data.
export function deleteSource(name: string): Promise<Deleted> {
    return del(`/sources/${encodeURIComponent(name)}`)
}
