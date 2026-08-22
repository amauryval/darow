import { describe, expect, it } from 'vitest'
import { validateUploadSize } from './uploadValidation'

function file(name: string, size: number): File {
    return new File(['x'.repeat(size)], name)
}

describe('validateUploadSize', () => {
    it('accepts a file at or under the running deployment cap', () => {
        expect(validateUploadSize(file('routes.geojson', 1024), 1024)).toBeNull()
    })

    it('names the file and the cap when it is over', () => {
        expect(validateUploadSize(file('routes.geojson', 2048), 1024)).toBe(
            'routes.geojson dépasse la taille maximale autorisée (1.0 Ko).'
        )
    })
})
