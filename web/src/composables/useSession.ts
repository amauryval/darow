import { computed, ref } from 'vue'
import { getSession } from '../client/session'
import type { Session } from '../client/types'

// Module-level, like useSettings: every caller shares one loaded session
// instead of each fetching /session. It is read once per page load — the
// gateway owns the session's lifetime, so nothing this application does can
// change who is calling; only a full navigation through the gateway can.
const current = ref<Session | null>(null)
let inFlight: Promise<void> | null = null

// A server that never answered is treated as authentication off: the UI
// stays usable, and a write that is in fact refused surfaces its own 401
// rather than a screen locked on a failed probe.
const OPEN: Session = { enabled: false, authenticated: false, groups: [], canWrite: true }

async function load() {
    try {
        current.value = await getSession()
    } catch {
        current.value = OPEN
    }
}

function ensureLoaded(): Promise<void> {
    if (!inFlight) inFlight = load()
    return inFlight
}

// Who is calling, and whether they may write. There is no sign-in screen
// here: signIn/signOut leave the application for the gateway's own pages,
// through a full navigation — the session is a cookie the SPA never sees,
// so it has to come back through the gateway to acquire one.
export function useSession() {
    ensureLoaded()

    const session = computed(() => current.value ?? OPEN)
    const enabled = computed(() => session.value.enabled)
    const authenticated = computed(() => session.value.authenticated)
    const canWrite = computed(() => session.value.canWrite)
    const displayName = computed(() => session.value.name || session.value.userId || '')
    const canSignIn = computed(() => enabled.value && Boolean(session.value.loginUrl))
    const canSignOut = computed(() => enabled.value && Boolean(session.value.logoutUrl))

    // A caller authenticated by the gateway but in none of the server's
    // write groups: the refusal is settled, and no sign-in of theirs will
    // lift it. Telling them so beats letting every write control fail.
    const writeRefused = computed(() => authenticated.value && !canWrite.value)

    function go(url: string | undefined) {
        if (url) window.location.assign(url)
    }

    function signIn() {
        go(session.value.loginUrl)
    }

    function signOut() {
        go(session.value.logoutUrl)
    }

    return {
        session,
        enabled,
        authenticated,
        canWrite,
        writeRefused,
        displayName,
        canSignIn,
        canSignOut,
        signIn,
        signOut
    }
}
