import { onMounted, onUnmounted, ref } from 'vue'
import { readyz } from '../client/ops'

export type ServerStatus = 'checking' | 'ok' | 'down'

const STATUS_LABEL: Record<ServerStatus, string> = {
    checking: 'Vérification…',
    ok: 'Opérationnel',
    down: 'Indisponible'
}

const PROBE_INTERVAL_MS = 20_000

// Periodic readyz probe backing the status banner. Registers and clears its
// own interval (§9, symmetry of effects), so any caller can mount it without
// remembering the cleanup.
export function useServerStatus() {
    const status = ref<ServerStatus>('checking')
    let interval: ReturnType<typeof setInterval> | undefined

    async function check() {
        try {
            await readyz()
            status.value = 'ok'
        } catch {
            status.value = 'down'
        }
    }

    onMounted(() => {
        check()
        interval = setInterval(check, PROBE_INTERVAL_MS)
    })
    onUnmounted(() => clearInterval(interval))

    return { status, label: STATUS_LABEL }
}
