import { describe, expect, it } from 'vitest'
import {
    createLeaf,
    describeDraft,
    draftOf,
    fromConditions,
    isComplete,
    nextNodeId,
    nodeOf,
    patchGroupBool,
    patchLeaf,
    pruneActive,
    removeNode,
    takesValue,
    toggleNot,
    updateChildren,
    type DraftNode
} from './filterTree'

// The editor holds text; the API takes typed values. The real coercion is
// coerceByType's job, so the tests use a visible stand-in.
const coerce = (column: string, value: string) => `${column}:${value}`

function group(children: DraftNode[], bool: 'AND' | 'OR' = 'AND'): DraftNode {
    return { id: nextNodeId(), kind: 'group', bool, children, not: false, active: true }
}

describe('takesValue', () => {
    it('holds for an operator comparing against a value', () => {
        expect(takesValue('=')).toBe(true)
        expect(takesValue('LIKE')).toBe(true)
    })

    it('fails for the two operators testing the column itself', () => {
        expect(takesValue('IS NULL')).toBe(false)
        expect(takesValue('IS NOT NULL')).toBe(false)
    })
})

describe('createLeaf', () => {
    it('starts an empty, active equality on the given column', () => {
        const leaf = createLeaf('name')

        expect(leaf).toMatchObject({
            kind: 'leaf',
            column: 'name',
            operator: '=',
            value: '',
            not: false,
            active: true
        })
    })

    it('gives each node its own identifier', () => {
        expect(createLeaf('a').id).not.toBe(createLeaf('a').id)
    })
})

describe('pruneActive', () => {
    it('coerces a leaf’s value and emits the condition', () => {
        const leaf = { ...createLeaf('height'), operator: '>', value: '10' }

        expect(pruneActive([leaf], coerce)).toEqual([
            { column: 'height', operator: '>', value: 'height:10', not: undefined }
        ])
    })

    it('emits no value for a valueless operator', () => {
        const leaf = { ...createLeaf('name'), operator: 'IS NULL', value: 'ignorée' }

        expect(pruneActive([leaf], coerce)[0].value).toBeUndefined()
    })

    it('carries a negation and omits it when unset', () => {
        const negated = { ...createLeaf('name'), not: true }

        expect(pruneActive([negated], coerce)[0].not).toBe(true)
        expect(pruneActive([createLeaf('name')], coerce)[0].not).toBeUndefined()
    })

    it('drops an inactive node entirely', () => {
        const leaf = { ...createLeaf('name'), active: false }

        expect(pruneActive([leaf], coerce)).toEqual([])
    })

    it('drops a group left with no active child', () => {
        const child = { ...createLeaf('name'), active: false }

        expect(pruneActive([group([child])], coerce)).toEqual([])
    })

    it('nests a group holding at least one active child', () => {
        const kept = createLeaf('name')

        expect(pruneActive([group([kept], 'OR')], coerce)).toEqual([
            {
                bool: 'OR',
                conditions: [{ column: 'name', operator: '=', value: 'name:', not: undefined }],
                not: undefined
            }
        ])
    })
})

describe('tree manipulation', () => {
    it('removes a node by id and leaves the others alone', () => {
        const [a, b] = [createLeaf('a'), createLeaf('b')]

        expect(removeNode([a, b], a.id)).toEqual([b])
    })

    it('replaces a group’s children', () => {
        const child = createLeaf('a')
        const parent = group([])
        const replaced = updateChildren([parent], parent.id, [child])

        expect(replaced[0]).toMatchObject({ kind: 'group', children: [child] })
    })

    it('patches a leaf’s own fields', () => {
        const leaf = createLeaf('a')

        expect(patchLeaf([leaf], leaf.id, { operator: '>', value: '5' })[0]).toMatchObject({
            column: 'a',
            operator: '>',
            value: '5'
        })
    })

    it('leaves a group untouched when a leaf patch names it', () => {
        const node = group([])

        expect(patchLeaf([node], node.id, { value: '5' })[0]).toEqual(node)
    })

    it('switches a group’s connector, never a leaf', () => {
        const node = group([])
        const leaf = createLeaf('a')

        expect(patchGroupBool([node], node.id, 'OR')[0]).toMatchObject({ bool: 'OR' })
        expect(patchGroupBool([leaf], leaf.id, 'OR')[0]).toEqual(leaf)
    })

    it('toggles the negation of a leaf as of a group', () => {
        const leaf = createLeaf('a')
        const node = group([])

        expect(toggleNot([leaf], leaf.id)[0].not).toBe(true)
        expect(toggleNot([node], node.id)[0].not).toBe(true)
    })
})

describe('describeDraft', () => {
    it('joins the conditions by the connector, in French', () => {
        const conditions = [
            { ...createLeaf('height'), operator: '>', value: '10' },
            { ...createLeaf('name'), value: 'Nantes' }
        ]

        expect(describeDraft('AND', conditions)).toBe('height > 10 et name = Nantes')
        expect(describeDraft('OR', conditions)).toBe('height > 10 ou name = Nantes')
    })

    it('omits the value of a valueless operator', () => {
        const leaf = { ...createLeaf('name'), operator: 'IS NULL', value: 'x' }

        expect(describeDraft('AND', [leaf])).toBe('name IS NULL')
    })

    it('spells a nested group out in parentheses and marks a negation', () => {
        const inner = group([createLeaf('a')], 'OR')

        expect(describeDraft('AND', [{ ...inner, not: true }])).toBe('NON (a = )')
    })
})

describe('draftOf', () => {
    it('starts a fresh draft on one empty leaf', () => {
        const draft = draftOf(null, 'name')

        expect(draft.bool).toBe('AND')
        expect(draft.children).toHaveLength(1)
        expect(draft.children[0]).toMatchObject({ kind: 'leaf', column: 'name' })
    })

    it('presents a bare leaf as a one-child AND', () => {
        const leaf = createLeaf('name')

        expect(draftOf(leaf, 'other')).toEqual({ bool: 'AND', children: [leaf] })
    })

    it('opens a group on its own connector and children', () => {
        const child = createLeaf('a')

        expect(draftOf(group([child], 'OR'), 'other')).toEqual({ bool: 'OR', children: [child] })
    })
})

describe('nodeOf', () => {
    it('answers null on an empty list — an empty group is refused server-side', () => {
        expect(nodeOf('AND', [], true)).toBeNull()
    })

    it('collapses a single condition back to a bare leaf', () => {
        const leaf = createLeaf('name')

        expect(nodeOf('AND', [leaf], false)).toEqual({ ...leaf, active: false })
    })

    it('wraps several conditions in a group carrying the connector', () => {
        const children = [createLeaf('a'), createLeaf('b')]

        expect(nodeOf('OR', children, true)).toMatchObject({
            kind: 'group',
            bool: 'OR',
            children,
            not: false,
            active: true
        })
    })
})

describe('isComplete', () => {
    it('refuses a leaf with no column, or a blank value on a valued operator', () => {
        expect(isComplete(createLeaf(''))).toBe(false)
        expect(isComplete(createLeaf('name'))).toBe(false)
        expect(isComplete({ ...createLeaf('name'), value: '   ' })).toBe(false)
    })

    it('accepts a valueless operator with no value', () => {
        expect(isComplete({ ...createLeaf('name'), operator: 'IS NULL' })).toBe(true)
    })

    it('requires a group to be non-empty and complete throughout', () => {
        const filled = { ...createLeaf('name'), value: 'Nantes' }

        expect(isComplete(group([]))).toBe(false)
        expect(isComplete(group([filled, createLeaf('a')]))).toBe(false)
        expect(isComplete(group([filled]))).toBe(true)
    })
})

describe('fromConditions', () => {
    it('rebuilds a leaf, stringifying the typed value', () => {
        const [node] = fromConditions([{ column: 'height', operator: '>', value: 10 }], true)

        expect(node).toMatchObject({ kind: 'leaf', column: 'height', value: '10', active: true })
    })

    it('maps a null or absent value back to the empty string', () => {
        const [nulled, absent] = fromConditions(
            [
                { column: 'a', operator: '=', value: null },
                { column: 'b', operator: 'IS NULL' }
            ],
            true
        )

        expect(nulled).toMatchObject({ value: '' })
        expect(absent).toMatchObject({ value: '' })
    })

    it('rebuilds a group and marks its children active', () => {
        const [node] = fromConditions(
            [{ bool: 'OR', conditions: [{ column: 'a', operator: '=', value: 1 }], not: true }],
            false
        )

        expect(node).toMatchObject({ kind: 'group', bool: 'OR', not: true, active: false })
        expect(node.kind === 'group' && node.children[0]).toMatchObject({ active: true })
    })

    it('round-trips an active tree through pruneActive', () => {
        const leaf = { ...createLeaf('height'), operator: '>', value: '10' }
        const conditions = pruneActive([leaf], (_, value) => value)

        expect(fromConditions(conditions, true)[0]).toMatchObject({
            column: 'height',
            operator: '>',
            value: '10'
        })
    })
})
