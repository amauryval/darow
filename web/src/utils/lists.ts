// Toggles name in and out of list: the gesture behind every "select a
// column" control (a carried-over column, a grouped one) — present, it is
// dropped; absent, it is appended.
export function toggleInList(list: string[], name: string): string[] {
    return list.includes(name) ? list.filter((entry) => entry !== name) : [...list, name]
}
