import type { ColumnSchema, RowValue } from '../client/types'

export function columnType(columns: ColumnSchema[], name: string): string {
    return columns.find((c) => c.name === name)?.type ?? ''
}

// Text-ish columns are the ones whose values are worth suggesting: on a
// number, a date or a boolean the distinct list is either meaningless or
// unbounded, and the user types the value directly.
export function isTextType(type: string): boolean {
    return /CHAR|TEXT|STRING|UUID|ENUM/.test(type.toUpperCase())
}

// What a column's type asks of the control that writes it. The database has
// dozens of type names and a form has four behaviours, so the names are
// reduced here, once, rather than re-tested wherever a field is built.
export type ColumnKind = 'boolean' | 'integer' | 'number' | 'date' | 'timestamp' | 'text'

export function columnKind(type: string): ColumnKind {
    const t = type.toUpperCase()
    if (t.includes('BOOL')) return 'boolean'
    if (t.includes('TIMESTAMP') || t.includes('DATETIME')) return 'timestamp'
    if (t.includes('DATE')) return 'date'
    if (/INT|HUGEINT/.test(t)) return 'integer'
    if (/DOUBLE|FLOAT|DECIMAL|REAL|NUMERIC/.test(t)) return 'number'
    return 'text'
}

// The typed value a write carries, from the text a field holds. The empty
// string is **NULL** for every kind, booleans included: it is the only way to
// write "no value" from a form, and a boolean silently turned into `false`
// wrote a fact the user never stated.
export function coerceByType(type: string, raw: string): RowValue {
    switch (columnKind(type)) {
        case 'boolean':
            return raw === '' ? null : raw === 'true' || raw === '1'
        case 'integer':
        case 'number': {
            if (raw === '') return null
            const n = Number(raw)
            return Number.isNaN(n) ? raw : n
        }
        default:
            return raw === '' ? null : raw
    }
}

// Why a value cannot be written, in the user's words, or null when it can —
// stated before the write rather than discovered as a server refusal for a
// whole session (§8.2). It says what is expected, never just "invalid".
export function typeError(type: string, raw: string): string | null {
    if (raw === '') return null
    switch (columnKind(type)) {
        case 'boolean':
            return raw === 'true' || raw === 'false' ? null : 'Valeur attendue : vrai ou faux'
        case 'integer':
            return Number.isInteger(Number(raw)) ? null : 'Nombre entier attendu'
        case 'number':
            return Number.isNaN(Number(raw)) ? 'Nombre attendu' : null
        case 'date':
            return /^\d{4}-\d{2}-\d{2}$/.test(raw) ? null : 'Date attendue, au format AAAA-MM-JJ'
        case 'timestamp':
            return Number.isNaN(Date.parse(raw))
                ? 'Date et heure attendues, au format AAAA-MM-JJ hh:mm'
                : null
        default:
            return null
    }
}
