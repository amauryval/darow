import { watchEffect, type Ref } from 'vue'

// document.title follows the route and names the object being viewed, not
// just its category (§9) — a view calls this with a ref/computed that
// resolves once its data (e.g. a table name) is available.
export function useDocumentTitle(title: Ref<string>) {
    watchEffect(() => {
        document.title = `Darow — ${title.value}`
    })
}
