// A table name is not unique server-side: two loads of the same file would
// both be called after it. Suggesting a free name is what keeps a catalogue
// readable without forbidding a deliberate duplicate.
export function suggestTableName(base: string, taken: string[]): string {
    if (!taken.includes(base)) return base
    let index = 2
    while (taken.includes(`${base} (${index})`)) index += 1
    return `${base} (${index})`
}
