import type { Source } from '../client/types'

// The tree the sources table renders: a single-layer file is one row, a
// multi-layer one (GeoPackage) is a parent row expanded over its layers —
// only a layer row is loadable, only a file row is deletable.
export interface SourceRow {
    name: string
    sizeBytes?: number
    layerCount?: number
    source: Source
    layerName?: string
    subRows?: SourceRow[]
}

export function toSourceRows(sources: Source[]): SourceRow[] {
    return sources.map((source) => {
        if (source.layers && source.layers.length > 0) {
            return {
                name: source.name,
                sizeBytes: source.sizeBytes,
                layerCount: source.layers.length,
                source,
                subRows: source.layers.map((layer) => ({
                    name: layer,
                    source,
                    layerName: layer
                }))
            }
        }
        return { name: source.name, sizeBytes: source.sizeBytes, source }
    })
}

export function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} o`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} Ko`
    return `${(bytes / (1024 * 1024)).toFixed(1)} Mo`
}
