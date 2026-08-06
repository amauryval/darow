import { ref } from 'vue'

export interface Toast {
    id: number
    kind: 'success' | 'error'
    message: string
}

// Module-level state: every component sharing this composable reads/writes
// the same toast queue, which is what lets Toast.vue (mounted once in
// App.vue) render toasts pushed from any view.
const toasts = ref<Toast[]>([])
let nextId = 0

// Only a success toast dismisses itself: it confirms an action already
// done, nothing is lost by it disappearing. An error toast may carry a
// detailed server message (and now a request identifier, §6.4 on the back
// side) the user needs time to read or copy, so it stays until dismissed
// by hand (§8.4).
function push(kind: Toast['kind'], message: string) {
    const id = nextId++
    toasts.value.push({ id, kind, message })
    if (kind === 'success') {
        setTimeout(() => dismiss(id), 4000)
    }
}

function dismiss(id: number) {
    toasts.value = toasts.value.filter((t) => t.id !== id)
}

export function useToast() {
    return {
        toasts,
        success: (message: string) => push('success', message),
        error: (message: string) => push('error', message),
        dismiss
    }
}
