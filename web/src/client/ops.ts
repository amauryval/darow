import { getRoot } from './http'
import type { Diagnostics } from './types'

// Readiness probe — resolves if the server can currently serve traffic
// (storage reachable); rejects with an ApiError on 503 otherwise.
export function readyz(): Promise<void> {
    return getRoot('/readyz')
}

// Storage engine diagnostics — on-disk size, buffer-manager memory usage,
// bytes spilled to disk, loaded extensions. Outside /api/v1, same as
// /healthz and /readyz.
export function getDiagnostics(): Promise<Diagnostics> {
    return getRoot('/diagnostics')
}
