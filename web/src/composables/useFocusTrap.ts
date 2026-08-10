import { onMounted, onUnmounted, type Ref } from 'vue'

const FOCUSABLE_SELECTOR =
    'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

// A control carrying this attribute (Modal.vue's ✕) is still part of the
// Tab/Shift+Tab cycle but is never the *initial* focus target — that has to
// land on the modal's first meaningful field (§8.5), not its
// destructive-adjacent close control.
const SKIP_INITIAL_FOCUS_ATTR = 'data-focus-trap-skip-initial'

function focusablesOf(container: HTMLElement | null): HTMLElement[] {
    if (!container) return []
    return Array.from(container.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))
}

// The first focusable that isn't marked skip-initial, or that control itself
// when it's all the container has — a modal whose slot renders nothing
// focusable yet (a loading skeleton) still needs somewhere to send focus.
function initialFocusTargetOf(container: HTMLElement | null): HTMLElement | null {
    const items = focusablesOf(container)
    return items.find((el) => !el.hasAttribute(SKIP_INITIAL_FOCUS_ATTR)) ?? items[0] ?? null
}

// One open/close cycle: activate() moves focus inside and holds it there,
// deactivate() restores it to whatever had it before.
function createFocusTrap(container: Ref<HTMLElement | null>) {
    let previouslyFocused: HTMLElement | null = null

    function onKeydown(e: KeyboardEvent) {
        if (e.key !== 'Tab') return
        const items = focusablesOf(container.value)
        if (items.length === 0) return
        const first = items[0]
        const last = items[items.length - 1]
        if (e.shiftKey && document.activeElement === first) {
            e.preventDefault()
            last.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
            e.preventDefault()
            first.focus()
        }
    }

    function activate() {
        previouslyFocused =
            document.activeElement instanceof HTMLElement ? document.activeElement : null
        initialFocusTargetOf(container.value)?.focus()
        document.addEventListener('keydown', onKeydown)
    }

    function deactivate() {
        document.removeEventListener('keydown', onKeydown)
        previouslyFocused?.focus()
    }

    return { activate, deactivate }
}

// Modal focus discipline (§8.7): focus moves inside on opening, stays
// trapped by Tab/Shift+Tab while open, returns to the triggering element on
// closing. For a component whose own mount/unmount is the open/close cycle
// (Modal.vue, created/destroyed by the caller's v-if).
export function useFocusTrap(container: Ref<HTMLElement | null>) {
    const trap = createFocusTrap(container)
    onMounted(trap.activate)
    onUnmounted(trap.deactivate)
}
