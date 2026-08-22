import { ref } from 'vue'
import { getVersion } from '../client/ops'
import type { Version } from '../client/types'
import { useAsyncAction } from './useAsyncAction'

// Module-level, like useSettings: the build identity is the same for every
// caller, so App.vue's one load is all any other mount needs.
const current = ref<Version | null>(null)
const { isLoading: loading, error, run: runLoad } = useAsyncAction()
let inFlight: Promise<void> | null = null

async function load() {
    const version = await runLoad(() => getVersion(), 'Impossible de charger la version du serveur')
    if (version === null) return
    current.value = version
}

function ensureLoaded(): Promise<void> {
    if (!inFlight) inFlight = load()
    return inFlight
}

// The running build's identity, fetched once and shared — displayed
// permanently in the sidebar so it is always visible and never stale.
export function useVersion() {
    ensureLoaded()
    return { current, loading, error }
}
