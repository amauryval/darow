import { describe, expect, it } from 'vitest'
import { sridOf } from '@/darow/utils/crs'

describe('sridOf', () => {
    it.each([
        { text: '2154', want: 2154 },
        { text: ' 3857 ', want: 3857 },
        { text: '', want: 0 },
        { text: 'abc', want: 0 },
        { text: '-4', want: 0 }
    ])('reads $text as $want', ({ text, want }) => {
        expect(sridOf(text)).toBe(want)
    })
})
