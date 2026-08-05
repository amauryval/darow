// The sentences the map's edit bar states, and the one thing they have to get
// right: they say what the session is waiting for, in the order that matters.
// A load in flight comes first because everything else is provisional under
// it; a row waiting for its geometry comes before the general instruction,
// since it is the one thing that will be lost if the user walks away (§8.3).

export interface EditingState {
  loading: boolean;
  truncated: boolean;
  // A row whose attributes were collected in the grid and whose geometry is
  // the first shape the session draws.
  pendingRow: boolean;
  // The view came back with nothing to edit — told apart from a view not yet
  // answered for, which says nothing at all.
  empty: boolean;
  max: number;
}

export function editingPrompt(state: EditingState): string {
  if (state.loading) return "Chargement des géométries de cette vue…";
  if (state.pendingRow) return "Nouvelle ligne : dessinez sa géométrie, elle sera enregistrée avec ses attributs.";
  if (state.truncated)
    return `Trop de géométries : seules les ${state.max} premières de cette session sont modifiables. Zoomez pour travailler sur un secteur plus petit.`;
  if (state.empty) return "Aucune géométrie à modifier dans cette vue — déplacez la carte, ou dessinez une forme.";
  return "Déplacez les sommets. Une limite partagée bouge des deux côtés à la fois.";
}

export function summarizeWrites(updated: number, inserted: number): string {
  const parts: string[] = [];
  if (updated > 0) parts.push(`${updated} géométrie(s) mise(s) à jour`);
  if (inserted > 0) parts.push(`${inserted} ligne(s) ajoutée(s)`);
  return `${parts.join(", ")}.`;
}
