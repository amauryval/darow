import { Log, UserManager, WebStorageStateStore } from 'oidc-client-ts'
import { getAuthConfig } from '../client/auth'

// The exact path a human registers at the identity provider, built from Vite's
// base (`/ui/`) and not from the router's path alone: the app is served under
// that prefix, so hardcoding `/auth/callback` would name a path that 404s.
export const OIDC_CALLBACK_PATH = `${import.meta.env.BASE_URL}auth/callback`.replace(/\/+/g, '/')

// The counterpart to OIDC_CALLBACK_PATH: strips Vite's base off a raw
// pathname so vue-router can be handed it. The router prepends the base
// itself, so a path still carrying it would get it twice (/ui/ui/dashboard).
export const appPath = (pathname: string): string => {
    const base = import.meta.env.BASE_URL
    return pathname.startsWith(base) ? `/${pathname.slice(base.length)}` : pathname
}

let manager: UserManager | null = null

// Built once at boot from GET /auth/config, through client/auth.ts like every
// other endpoint (rule 0.1). Null when any of the three fields is empty: that
// is demo mode, with no provider and no login control to show.
export const initOidc = async (): Promise<void> => {
    if (import.meta.env.DEV) {
        Log.setLogger(console)
        Log.setLevel(Log.DEBUG)
    }

    let config
    try {
        config = await getAuthConfig()
    } catch {
        // A server that never answered /auth/config leaves the application
        // usable with no login control, exactly like useSession's failed
        // probe on the session read itself.
        return
    }

    if (!config.issuer_url || !config.client_id || !config.scopes) {
        return
    }

    manager = new UserManager({
        authority: config.issuer_url,
        client_id: config.client_id,
        redirect_uri: `${window.location.origin}${OIDC_CALLBACK_PATH}`,
        post_logout_redirect_uri: window.location.origin,
        response_type: 'code',
        // From the server, not a literal: a provider (Pocket ID) only emits
        // some claims when the matching scope is requested.
        scope: config.scopes,
        loadUserInfo: true,
        automaticSilentRenew: true,
        // Both stores on sessionStorage: tokens and the PKCE verifier are
        // per-tab, which keeps a token out of any storage another tab (or a
        // long-lived XSS) can read.
        userStore: new WebStorageStateStore({ store: window.sessionStorage }),
        stateStore: new WebStorageStateStore({ store: window.sessionStorage })
    })
}

export const isOidcEnabled = (): boolean => manager !== null

// Non-secret localStorage flags — "someone is signed in here" and "a silent
// signin is already in flight for this tab" — under a prefix of their own, to
// avoid a collision with another application on a shared origin.
const SIGNED_IN_KEY = 'darow.auth.signed-in'
const SILENT_ATTEMPT_KEY = 'darow.auth.silent-attempted'

export const markSignedIn = (): void => window.localStorage.setItem(SIGNED_IN_KEY, '1')

export const clearSignedIn = (): void => window.localStorage.removeItem(SIGNED_IN_KEY)

export const wasSignedIn = (): boolean => window.localStorage.getItem(SIGNED_IN_KEY) !== null

// The sessionStorage gate is the second half of boot recovery: without it, a
// user the provider grants no role keeps `wasSignedIn()` true forever and
// bounces to the provider and back on every load.
export const trySilentSignin = async (returnTo: string): Promise<boolean> => {
    if (!manager || window.sessionStorage.getItem(SILENT_ATTEMPT_KEY)) {
        return false
    }

    window.sessionStorage.setItem(SILENT_ATTEMPT_KEY, returnTo)

    try {
        await manager.signinRedirect({ prompt: 'none', state: returnTo })
        return true
    } catch {
        window.sessionStorage.removeItem(SILENT_ATTEMPT_KEY)
        return false
    }
}

export const forgetSilentAttempt = (): void => window.sessionStorage.removeItem(SILENT_ATTEMPT_KEY)

export const silentAttemptReturnTo = (): string | null =>
    window.sessionStorage.getItem(SILENT_ATTEMPT_KEY)

// Boot recovery: a per-tab token costs a second tab starting logged out, so
// this asks the provider for a fresh one with no interaction. Never runs on
// the callback route itself — the callback owns that redirect.
export const restoreBrowserSession = async (): Promise<void> => {
    if (!manager || !wasSignedIn() || window.location.pathname === OIDC_CALLBACK_PATH) {
        return
    }

    const user = await manager.getUser()
    if (user && !user.expired) {
        return
    }

    await trySilentSignin(appPath(window.location.pathname) + window.location.search)
}

export const oidcManager = (): UserManager => {
    if (!manager) {
        throw new Error("L'authentification n'est pas configurée sur ce serveur.")
    }
    return manager
}

export const currentAccessToken = async (): Promise<string | null> => {
    if (!manager) {
        return null
    }

    const user = await manager.getUser()
    if (!user || user.expired) {
        return null
    }
    return user.access_token
}

export interface UserProfile {
    name: string
    email: string
}

// The display name comes from the ID token's own profile, never from
// /auth/session — the access token carries no name at all, so a display
// name built from the session read would have nothing to read.
export const currentProfile = async (): Promise<UserProfile | null> => {
    if (!manager) {
        return null
    }

    const user = await manager.getUser()
    if (!user || user.expired) {
        return null
    }

    const name = user.profile.name ?? user.profile.preferred_username ?? user.profile.email ?? ''
    return { name, email: user.profile.email ?? '' }
}
