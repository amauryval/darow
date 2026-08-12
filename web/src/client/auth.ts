import { get } from './http'
import type { AuthConfig, AuthSession } from './types'

// The provider parameters the OIDC manager is built from (auth/oidc.ts). A
// read like any other — it answers an anonymous request too, since the SPA
// has to know how to log in before anyone has.
export function getAuthConfig(): Promise<AuthConfig> {
    return get('/auth/config')
}

// The current caller's verdict. Answers for any bearer token, valid or not
// — http.ts attaches it automatically once one is set — which is what lets
// the UI offer a sign-in rather than discover the refusal on the first
// write.
export function getAuthSession(): Promise<AuthSession> {
    return get('/auth/session')
}
