// vue-router types a query value as string | null, or an array of those for
// a repeated key — this app never repeats a key, so only the first value
// (or the empty string) is ever meaningful.
export function queryString(v: string | (string | null)[] | null | undefined): string {
    return Array.isArray(v) ? v[0] ?? '' : v ?? ''
}
