import type { Assignment, AssignmentKind, Operand } from '../client/types'
import { coerceByType, type ColumnKind } from './columnTypes'

// The editable form of an Operand: raw text for a literal, held apart from
// column so switching kind never loses what was typed in the other one.
export interface OperandDraft {
    kind: 'column' | 'literal'
    column: string
    literal: string
}

export function createOperandDraft(): OperandDraft {
    return { kind: 'literal', column: '', literal: '' }
}

// The editable form of an Assignment: every arm's fields are held at once,
// so switching the arm never loses what was typed into another one.
export interface AssignmentDraft {
    kind: AssignmentKind
    constant: string
    concat: OperandDraft[]
    arithmeticLeft: OperandDraft
    arithmeticRight: OperandDraft
    arithmeticOperator: string
    expression: string
}

export function createAssignmentDraft(): AssignmentDraft {
    return {
        kind: 'constant',
        constant: '',
        concat: [createOperandDraft()],
        arithmeticLeft: createOperandDraft(),
        arithmeticRight: createOperandDraft(),
        arithmeticOperator: '+',
        expression: ''
    }
}

// The arms representable for a column's type, mirroring service.typeFamily:
// concatenation always renders text, arithmetic always renders a number, so
// offering either on a boolean or datetime column would only ever be
// refused by the server's own type-preempt check. Making the mismatch
// unrepresentable here is cheaper than letting it round-trip.
export function assignmentArms(kind: ColumnKind): AssignmentKind[] {
    switch (kind) {
        case 'integer':
        case 'number':
            return ['constant', 'arithmetic', 'expression']
        case 'text':
            return ['constant', 'concat', 'expression']
        default:
            return ['constant', 'expression']
    }
}

function toOperand(draft: OperandDraft): Operand {
    return draft.kind === 'column'
        ? { kind: 'column', column: draft.column }
        : { kind: 'literal', literal: draft.literal }
}

function isOperandComplete(draft: OperandDraft): boolean {
    return draft.kind === 'column' ? draft.column !== '' : draft.literal.trim() !== ''
}

export function toAssignment(draft: AssignmentDraft, targetType: string): Assignment {
    switch (draft.kind) {
        case 'constant':
            return { kind: 'constant', constant: coerceByType(targetType, draft.constant) }
        case 'concat':
            return { kind: 'concat', concat: draft.concat.map(toOperand) }
        case 'arithmetic':
            return {
                kind: 'arithmetic',
                arithmetic: {
                    left: toOperand(draft.arithmeticLeft),
                    right: toOperand(draft.arithmeticRight),
                    operator: draft.arithmeticOperator
                }
            }
        case 'expression':
            return { kind: 'expression', expression: draft.expression.trim() }
    }
}

export function isAssignmentComplete(draft: AssignmentDraft): boolean {
    switch (draft.kind) {
        case 'constant':
            return true // the empty string is a deliberate NULL (coerceByType)
        case 'concat':
            return draft.concat.length > 0 && draft.concat.every(isOperandComplete)
        case 'arithmetic':
            return (
                isOperandComplete(draft.arithmeticLeft) && isOperandComplete(draft.arithmeticRight)
            )
        case 'expression':
            return draft.expression.trim() !== ''
    }
}
