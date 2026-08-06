// What a save says it wrote. The session states nothing else in words: what
// is editable is what is drawn as editable, and a bar announcing its own
// loading is a bar talking about its plumbing (§8.8).

export function summarizeWrites(geometries: number, inserted: number, attributes = 0): string {
    const parts: string[] = []
    if (geometries > 0) parts.push(`${geometries} géométrie(s) mise(s) à jour`)
    if (attributes > 0) parts.push(`${attributes} ligne(s) modifiée(s)`)
    if (inserted > 0) parts.push(`${inserted} ligne(s) ajoutée(s)`)
    return `${parts.join(', ')}.`
}
