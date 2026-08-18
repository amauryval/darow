import { computed, ref } from 'vue'
import { getAuthSession } from '../client/auth'
import { errorMessage, setAuthToken, setUnauthorizedHandler } from '../client/http'
import type { AuthSession } from '../client/types'
import {
    appPath,
    clearSignedIn,
    currentAccessToken,
    currentProfile,
    isOidcEnabled,
    markSignedIn,
    oidcManager,
    type UserProfile
} from '../auth/oidc'
import { useToast } from './useToast'

// The application's own role names — literal strings, not an enum from an
// import: goauth maps a provider's own role claims onto exactly these two
// server-side, and the browser never sees the provider's names.
const ADMIN_ROLE = 'admin'
const GUEST_ROLE = 'guest'

// A re-render must never refetch /auth/session — this caches the last read,
// keyed by the token it was read for, for a few tens of seconds.
const SESSION_TTL_MS = 30000

// Every tab drops its user and session on a logout message from another —
// posted before signOut clears anything locally, so a token already valid
// elsewhere still expires on its own schedule (a broadcast tidies the UI,
// it revokes nothing server-side).
const LOGOUT_CHANNEL = 'darow.auth.logout'

const emptySession = (): AuthSession => ({ logged_in: false, authorized: false })
const emptyProfile = (): UserProfile => ({ name: '', email: '' })

// Module-level shared state, the same exception useToast/useSettings rely
// on (rule 0.11): every caller reads the one session this application has,
// instead of each fetching /auth/session on its own.
const session = ref<AuthSession>(emptySession())
const profile = ref<UserProfile>(emptyProfile())

let lifecycleWired = false
let logoutChannel: BroadcastChannel | null = null
let pendingRefresh: Promise<void> | null = null
let cachedToken: string | null = null
let cachedAt = 0

const isCacheFresh = (token: string | null) =>
    cachedAt !== 0 && token === cachedToken && Date.now() - cachedAt < SESSION_TTL_MS

function forgetSession() {
    session.value = emptySession()
    profile.value = emptyProfile()
    setAuthToken(null)
    cachedToken = null
    cachedAt = 0
}

export function useAuth() {
    const toast = useToast()

    const enabled = computed(() => isOidcEnabled())
    const loggedIn = computed(() => session.value.logged_in)
    const authorized = computed(() => session.value.authorized)
    const roles = computed(() => session.value.roles ?? [])
    const isAdmin = computed(() => roles.value.includes(ADMIN_ROLE))
    const isGuest = computed(() => roles.value.includes(GUEST_ROLE))
    const displayName = computed(() => profile.value.name)

    async function loadSession() {
        try {
            const token = await currentAccessToken()
            if (isCacheFresh(token)) {
                return
            }

            setAuthToken(token)
            profile.value = (await currentProfile()) ?? emptyProfile()
            // Always asked, token or not: a demo backend's verifier ignores
            // the token entirely and answers every caller as an
            // administrator, and a real one answers an empty token with an
            // unauthenticated session — either way this is the one source
            // of truth for logged_in/authorized/roles, never assumed
            // locally from token presence.
            session.value = await getAuthSession()
            cachedToken = token
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

    async function signIn() {
        try {
            await oidcManager().signinRedirect({ state: appPath(window.location.pathname) })
        } catch (err) {
            toast.error(errorMessage(err, 'Impossible de démarrer la connexion'))
        }
    }

    async function signOut() {
        clearSignedIn()
        logoutChannel?.postMessage(Date.now())
        forgetSession()

        if (!isOidcEnabled()) {
            return
        }

        try {
            const user = await oidcManager().getUser()
            await oidcManager().removeUser()
            await oidcManager().signoutRedirect({ id_token_hint: user?.id_token })
        } catch (err) {
            window.location.assign('/')
        }
    }

    if (!lifecycleWired && isOidcEnabled()) {
        lifecycleWired = true

        logoutChannel = new BroadcastChannel(LOGOUT_CHANNEL)
        logoutChannel.onmessage = () => {
            void oidcManager().removeUser()
            forgetSession()
        }

        oidcManager().events.addUserLoaded(() => {
            markSignedIn()
            void refreshSession()
        })
        // A renewal that fails is usually a session ended somewhere else —
        // treated the same as an expiry, not as a distinct error.
        oidcManager().events.addAccessTokenExpired(() => {
            clearSignedIn()
            forgetSession()
            toast.error('Votre session a expiré, reconnectez-vous.')
        })
        oidcManager().events.addSilentRenewError(() => {
            clearSignedIn()
            void oidcManager().removeUser()
            forgetSession()
        })

        // A 401 on any call means the held token is no longer good — drop
        // it and ask for a fresh read rather than leaving http.ts holding a
        // token every subsequent request will fail with too.
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
