import { get } from './http'
import type { AuthConfig, AuthSession } from './types'

// How the SPA signs in — the server-driven login/logout paths, or a demo
// deployment's empty config (composables/useAuth.ts). A read like any
// other — it answers an anonymous request too, since the SPA has to know
// how to log in before anyone has.
export function getAuthConfig(): Promise<AuthConfig> {
    return get('/auth/config')
}

// The current caller's verdict, answered for any bearer token, valid or not
// (http.ts attaches it once set) — which is what lets the UI offer a sign-in
// rather than discover the refusal on the first write.
export function getAuthSession(): Promise<AuthSession> {
    return get('/auth/session')
}
