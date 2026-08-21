import { computed, ref } from 'vue'
import { getAuthConfig, getAuthSession } from '../client/auth'
import { apiUrl, errorMessage, setUnauthorizedHandler } from '../client/http'
import type { AuthSession } from '../client/types'
import { useToast } from './useToast'

// The application's own role names — literal strings, not an enum from an
// import: goauth maps a provider's own role claims onto exactly these two
// server-side, and the browser never sees the provider's names.
const ADMIN_ROLE = 'admin'
const GUEST_ROLE = 'guest'

// A re-render must never refetch /auth/session — this caches the last read
// for a few tens of seconds.
const SESSION_TTL_MS = 30000

const emptySession = (): AuthSession => ({ logged_in: false, authorized: false })

// Module-level shared state, the same exception useToast/useSettings rely
// on (rule 0.11): every caller reads the one session this application has,
// instead of each fetching /auth/session on its own.
const session = ref<AuthSession>(emptySession())

// Whether the server drives a login flow at all — false in demo mode, where
// there is no provider and nothing to sign in or out of.
const enabledRef = ref(false)

let configLoaded = false
let lifecycleWired = false
let pendingRefresh: Promise<void> | null = null
let cachedAt = 0

function forgetSession() {
    session.value = emptySession()
    cachedAt = 0
}

// GET /auth/config names how to sign in, read once and cached: it never
// changes for the life of a running deployment.
async function ensureConfig() {
    if (configLoaded) {
        return
    }
    configLoaded = true

    try {
        const config = await getAuthConfig()
        enabledRef.value = config.server_flow
    } catch {
        // A server that never answered /auth/config leaves the application
        // usable with no login control, exactly like a failed session read.
        enabledRef.value = false
    }
}

export function useAuth() {
    const toast = useToast()

    const enabled = computed(() => enabledRef.value)
    const loggedIn = computed(() => session.value.logged_in)
    const authorized = computed(() => session.value.authorized)
    const roles = computed(() => session.value.roles ?? [])
    const isAdmin = computed(() => roles.value.includes(ADMIN_ROLE))
    const isGuest = computed(() => roles.value.includes(GUEST_ROLE))
    // The ID token's own claims, carried on the session response itself:
    // the server drives the login flow, so this browser never holds an ID
    // token of its own to read a name from.
    const displayName = computed(() => {
        const profile = session.value.profile
        return profile?.name || profile?.username || profile?.email || ''
    })

    async function loadSession() {
        await ensureConfig()

        if (cachedAt !== 0 && Date.now() - cachedAt < SESSION_TTL_MS) {
            return
        }

        try {
            session.value = await getAuthSession()
            cachedAt = Date.now()
        } catch (err) {
            forgetSession()
            toast.error(errorMessage(err, 'Impossible de vérifier la session'))
        }
    }

    // Concurrent callers (a mount storm on navigation) share the one load
    // in flight rather than each firing their own /auth/session request.
    async function refreshSession() {
        pendingRefresh = pendingRefresh ?? loadSession()
        try {
            await pendingRefresh
        } finally {
            pendingRefresh = null
        }
    }

    // Signing in is a navigation, never a fetch: the endpoint redirects to
    // the provider, which serves its own page cross-origin, so fetch cannot
    // follow it. return_to must be a path on this application — the current
    // one, which already carries the /ui/ base.
    function signIn() {
        const returnTo = window.location.pathname + window.location.search
        window.location.assign(`${apiUrl('/auth/login')}?return_to=${encodeURIComponent(returnTo)}`)
    }

    // Signing out is a POST, so that no cross-site page can sign a visitor
    // out by linking to it — a form submission rather than fetch, so a
    // provider that also ends the session there can answer with a redirect
    // fetch could not follow cross-origin.
    function signOut() {
        const form = document.createElement('form')
        form.method = 'POST'
        form.action = apiUrl('/auth/logout')
        document.body.appendChild(form)
        form.submit()
    }

    if (!lifecycleWired) {
        lifecycleWired = true

        // A 401 on any call means the session cookie is no longer good —
        // drop it and ask for a fresh read rather than leaving the UI
        // showing a session every subsequent request will fail with too.
        setUnauthorizedHandler(() => {
            forgetSession()
            void refreshSession()
        })
    }

    return {
        enabled,
        loggedIn,
        authorized,
        roles,
        isAdmin,
        isGuest,
        displayName,
        signIn,
        signOut,
        refreshSession
    }
}
