import { describe, expect, it } from 'vitest'
import { nextTick, ref } from 'vue'
import { useDocumentTitle } from './useDocumentTitle'
import { withSetup } from '../test/fixtures'

describe('useDocumentTitle', () => {
    it('names the object being viewed, under the product name', () => {
        withSetup(() => useDocumentTitle(ref('routes')))

        expect(document.title).toBe('Darow — routes')
    })

    it('follows the title once the view’s data resolves', async () => {
        const title = ref('Chargement…')
        withSetup(() => useDocumentTitle(title))

        title.value = 'routes'
        await nextTick()

        expect(document.title).toBe('Darow — routes')
    })
})
