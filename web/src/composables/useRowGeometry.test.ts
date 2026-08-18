import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useRowGeometry } from './useRowGeometry'
import { useToast } from './useToast'
import { getRowGeometry } from '../client/tables'
import type { RowGeometry } from '../client/types'

vi.mock('../client/tables', () => ({ getRowGeometry: vi.fn() }))

const read = vi.mocked(getRowGeometry)

const answer: RowGeometry = {
    pkey: 'p1',
    geometry: { type: 'Point', coordinates: [1, 2] },
    wkt: 'POINT (1 2)',
    crs: 'OGC:CRS84'
}

function setup() {
    return useRowGeometry(ref('t1'))
}

describe('useRowGeometry', () => {
    beforeEach(() => {
        useToast().toasts.value = []
        read.mockResolvedValue(answer)
    })

    it('reads one row’s geometry in lon/lat by default', async () => {
        const { open, pkey, geometry, wkt, crs, loading } = setup()

        open('p1')
        expect(loading.value).toBe(true)
        await vi.waitFor(() => expect(loading.value).toBe(false))

        expect(read).toHaveBeenCalledWith('t1', 'p1', null)
        expect(pkey.value).toBe('p1')
        expect(geometry.value).toEqual(answer.geometry)
        expect(wkt.value).toBe('POINT (1 2)')
        expect(crs.value).toBe('OGC:CRS84')
    })

    it('drops the previous rendering when another row is opened', async () => {
        const { open, geometry } = setup()
        open('p1')
        await vi.waitFor(() => expect(geometry.value).not.toBeNull())

        read.mockReturnValue(new Promise(() => {}))
        open('p2')

        expect(geometry.value).toBeNull()
    })

    it('lets the row opened last own the modal', async () => {
        const slow = new Promise<typeof answer>((resolve) =>
            setTimeout(() => resolve({ ...answer, wkt: 'ancien' }), 20)
        )
        read.mockReturnValueOnce(slow).mockResolvedValueOnce({ ...answer, wkt: 'récent' })
        const { open, wkt } = setup()

        open('p1')
        open('p2')
        await vi.waitFor(() => expect(wkt.value).toBe('récent'))
        await slow

        expect(wkt.value).toBe('récent')
    })

    it('re-reads the same row in another projection, showing nothing meanwhile', async () => {
        const { open, reproject, geometry, srid } = setup()
        open('p1')
        await vi.waitFor(() => expect(geometry.value).not.toBeNull())

        read.mockReturnValue(new Promise(() => {}))
        reproject(2154)

        expect(srid.value).toBe(2154)
        expect(geometry.value).toBeNull()
        expect(read).toHaveBeenLastCalledWith('t1', 'p1', 2154)
    })

    it('states a failed read and retries it', async () => {
        read.mockRejectedValueOnce(new Error('illisible'))
        const { open, retry, error, geometry } = setup()

        open('p1')
        await vi.waitFor(() =>
            expect(error.value).toBe(
                'Impossible de charger la géométrie de cette ligne : illisible'
            )
        )

        retry()
        await vi.waitFor(() => expect(geometry.value).not.toBeNull())
        expect(error.value).toBe('')
    })

    it('retries nothing when no row is open', () => {
        const { retry } = setup()

        retry()

        expect(read).not.toHaveBeenCalled()
    })

    it('clears everything on close', async () => {
        const { open, close, pkey, geometry, wkt, crs, srid, error } = setup()
        open('p1')
        await vi.waitFor(() => expect(geometry.value).not.toBeNull())

        close()

        expect(pkey.value).toBeNull()
        expect(geometry.value).toBeNull()
        expect(wkt.value).toBe('')
        expect(crs.value).toBe('')
        expect(srid.value).toBeNull()
        expect(error.value).toBe('')
    })

    it('confirms a copy to the clipboard', async () => {
        const writeText = vi.fn().mockResolvedValue(undefined)
        vi.stubGlobal('navigator', { clipboard: { writeText } })
        const { copy } = setup()

        await copy('POINT (1 2)', 'WKT')

        expect(writeText).toHaveBeenCalledWith('POINT (1 2)')
        expect(useToast().toasts.value[0].message).toBe('WKT copié dans le presse-papiers.')
    })

    it('toasts a clipboard the browser refused', async () => {
        vi.stubGlobal('navigator', {
            clipboard: { writeText: vi.fn().mockRejectedValue(new Error('refusé')) }
        })
        const { copy } = setup()

        await copy('POINT (1 2)', 'WKT')

        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de copier dans le presse-papiers : refusé'
        })
    })
})
