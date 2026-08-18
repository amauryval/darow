import { describe, expect, it } from 'vitest'
import { toggleInList } from './lists'

describe('toggleInList', () => {
    it('appends a name the list does not hold', () => {
        expect(toggleInList(['a'], 'b')).toEqual(['a', 'b'])
    })

    it('drops a name the list already holds', () => {
        expect(toggleInList(['a', 'b', 'c'], 'b')).toEqual(['a', 'c'])
    })

    it('leaves the input list untouched', () => {
        const list = ['a']
        toggleInList(list, 'b')
        expect(list).toEqual(['a'])
    })
})
