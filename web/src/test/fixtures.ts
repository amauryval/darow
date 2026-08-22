import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { vi } from 'vitest'

// Shared doubles and helpers for the front tests.

// Runs a composable inside a real component instance, so onMounted, watchers
// and their teardown behave as they do in the application. `unmount` is what
// exercises the teardown half.
export function withSetup<T>(composable: () => T): { result: T; unmount: () => void } {
    const captured: T[] = []
    const wrapper = mount(
        defineComponent({
            setup() {
                captured.push(composable())
                return () => h('div')
            }
        })
    )
    return { result: captured[0], unmount: () => wrapper.unmount() }
}

// One microtask turn plus Vue's own flush: what a test waits on after an
// action whose promises are already resolved.
export async function flush(): Promise<void> {
    await new Promise((resolve) => setTimeout(resolve, 0))
}

// jsdom implements no IntersectionObserver, which every infinite-scroll list
// registers on mount. The double records what is observed and lets a test
// say "the sentinel came into view".
export interface ObserverDouble {
    observed: Element[]
    disconnected: boolean
    trigger(isIntersecting?: boolean): void
}

export function stubIntersectionObserver(): ObserverDouble {
    const double: ObserverDouble = {
        observed: [],
        disconnected: false,
        trigger: () => {}
    }
    class Stub {
        constructor(private callback: IntersectionObserverCallback) {
            double.trigger = (isIntersecting = true) =>
                this.callback(
                    [{ isIntersecting } as IntersectionObserverEntry],
                    this as unknown as IntersectionObserver
                )
        }
        observe(element: Element) {
            double.observed.push(element)
        }
        unobserve(element: Element) {
            double.observed = double.observed.filter((seen) => seen !== element)
        }
        disconnect() {
            double.disconnected = true
            double.observed = []
        }
    }
    vi.stubGlobal('IntersectionObserver', Stub)
    return double
}
