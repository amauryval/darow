import { formatSize } from './sourceRows'

// The cap is the running deployment's own (GET /sources/limits →
// sources.maxSizeBytes), passed in by the caller: nothing here assumes the
// default config.yaml value, which a custom config drifts from silently.
export function validateUploadSize(file: File, maxBytes: number): string | null {
    if (file.size <= maxBytes) return null
    return `${file.name} dépasse la taille maximale autorisée (${formatSize(maxBytes)}).`
}
