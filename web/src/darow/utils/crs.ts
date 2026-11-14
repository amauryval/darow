// sridOf reads an EPSG code typed as text, 0 (longitude and latitude) when empty or not a code.
export const sridOf = (text: string): number => {
    const srid = Number.parseInt(text, 10)
    return srid > 0 ? srid : 0
}
