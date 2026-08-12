import { formatSize } from './sourceRows'

// Mirrors the server's default sources.maxSizeBytes (config.go) — there is no
// endpoint exposing the running deployment's actual value, so a custom
// config still only gets caught server-side (§8.5 in spec/front).
export const MAX_UPLOAD_BYTES = 100 * 1024 * 1024

export function validateUploadSize(file: File): string | null {
    if (file.size <= MAX_UPLOAD_BYTES) return null
    return `${file.name} dépasse la taille maximale autorisée (${formatSize(MAX_UPLOAD_BYTES)}).`
}
