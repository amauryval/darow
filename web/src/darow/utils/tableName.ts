export const tableNameOf = (file: string): string =>
    file
        .replace(/\.[^.]+$/, '')
        .toLowerCase()
        .replace(/[^a-z0-9_]+/g, '_')
        .replace(/^([0-9])/, '_$1')
        .slice(0, 63)
