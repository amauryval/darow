// The sentences the map's edit bar states, and the one thing they have to get
// right: they say what the session is waiting for, in the order that matters.
// A load in flight comes first because everything else is provisional under
// it; a row waiting for its geometry comes before the general instruction,
// since it is the one thing that will be lost if the user walks away (§8.3).
//
// What the session holds is *not* one of them. The working set loads and
// unloads itself as the map moves, and a bar announcing that is a bar talking
// about its own plumbing: what is editable is what is drawn as editable, and
// that is the whole message (§8.8).

export interface EditingState {
  loading: boolean;
  // A row whose attributes were collected in the grid and whose geometry is
  // the first shape the session draws.
  pendingRow: boolean;
  // The view came back with nothing to edit — told apart from a view not yet
  // answered for, which says nothing at all.
  empty: boolean;
}

export function summarizeWrites(updated: number, inserted: number): string {
  const parts: string[] = [];
  if (updated > 0) parts.push(`${updated} géométrie(s) mise(s) à jour`);
  if (inserted > 0) parts.push(`${inserted} ligne(s) ajoutée(s)`);
  return `${parts.join(", ")}.`;
}
