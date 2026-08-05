// Rendering helpers for a geometry read one row at a time. A real geometry
// runs to hundreds of thousands of characters, so what is shown and what is
// copied are two different things: the DOM gets a bounded excerpt, the
// clipboard gets the whole value.
const DISPLAY_LIMIT = 5000;

export function formatGeoJSON(geometry: GeoJSON.Geometry | null): string {
  return geometry === null ? "" : JSON.stringify(geometry, null, 2);
}

// Cuts text down to what a modal can render without freezing the page,
// stating in French that what is shown is an excerpt.
export function excerpt(text: string, limit = DISPLAY_LIMIT): string {
  if (text.length <= limit) return text;
  return `${text.slice(0, limit)}\n… (extrait — copier pour obtenir la valeur complète)`;
}
