import { get } from './http'
import type { Session } from './types'

// The current caller. A read like any other: it answers an anonymous
// request too, which is what lets the UI offer a sign-in rather than
// discover the refusal on the first write.
export function getSession(): Promise<Session> {
    return get('/session')
}
