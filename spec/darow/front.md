# darow, front — Specification

The target state of the `darow` module's Vue side, `web/src/darow`, hosted by the shell of
`@myapplibs/web-platform` (`web-platform/spec/front.md`). The platform's rules hold here; this
document names only what the module adds. `spec/darow/back.md` is the authority on the API it
calls.

## §0 Rules

1. The module reaches the API through `api/darow.ts` alone, over the platform's `getJson`,
   `sendJson`, `checkStatus`, `sendForm` and `resourceApi`. The types of the wire are generated from
   the Go packages (`api/*.gen.ts`, `tygo.yaml`), never written by hand.
2. Logic lives in `composables/` and `utils/`; a view orchestrates, a component of `views/` is
   presentational (props in, events out). Every composable and every `utils` file has its vitest
   test.
3. Every text is a key of `i18n/fr.ts`; a server failure is shown through `useAttempt` (its toast
   reads the first field error of a 422, `web-platform/spec/areas/i18n.md`).
4. The pages use the platform's components as they are — `PageTemplate`, `ListDetailLayout`,
   `ListRow`, `DataTable`, `TablePager`, `BaseMap`, `BaseModal`, `Form*`, `BaseButton` —, never a
   button or field class typed again.
5. A write control is shown only when the user may make it (`useCanEdit`): `canEdit` — no sign-in
   configured, the guest or the administrator — for the rows, columns and saved filters; `canAdmin`
   — no sign-in or the administrator — for the sources, the settings, renaming and deleting a table
   (ADR-0006).

## §1 Dependencies

The module needs nothing beside the platform's dependencies (`web-platform/spec/front.md` §1): its
map is the platform's `BaseMap` (MapLibre), its tables the platform's `DataTable`.

## §2 Module

`darowModule`: name `darow`, label `darow.nav.app`, icon `Map`; `nav.main` Tables (`darow-tables`),
Filters (`darow-filters`, icon `Filter`) and SQL (`darow-sql`); `nav.config` Sources (`darow-sources`) and Settings (`darow-settings`).

`src/main.ts`: `createShell({ title: 'darow', mark: 'D', modules: [darowModule] })`.
`src/darow/icons.ts` gathers the module's Lucide components.

## §3 Structure

```
web/src/main.ts                createShell({ title: 'darow', mark: 'D', modules: [darowModule] })
web/src/i18n.setup.ts          registers the catalog for the tests
web/src/darow/
  module.ts                    darowModule: FrontModule
  routeNames.ts                DarowRoutes
  router/routes.ts             the routes of §4.1
  icons.ts                     the module's Lucide icons
  events.ts                    setupDarowEvents: darow-tables, darow-sources, darow-filters
  i18n/fr.ts, i18n/index.ts    the catalog (§5)
  api/*.gen.ts                 generated: engine, rest, sources, filters
  api/darow.ts                 the client
  stores/tables.ts             the tables (list)
  stores/sources.ts            the sources (list)
  stores/filters.ts            the saved filters (useResource)
  composables/useCanEdit.ts    canEdit, canAdmin
  composables/useColumns.ts    add, rename, drop, transform a column, its preview
  composables/useSettings.ts   the global settings and a table's
  composables/useTables.ts     the list of the tables, its search
  composables/useTable.ts      the open table: detail, page of rows, condition, active filters, row, actions
  composables/useMapEditing.ts the open table's geometries edited on its map: the row in hand, its changed neighbours, hidden, click, save, cancel
  composables/useRowGeometries.ts the platform's useExactGeometries over the table's layers, each geometry read from its row
  composables/usePanes.ts      the map and the table of the open table, shown or hidden, split
  composables/useSavedFilters.ts the saved filters: all, a table's, save (create or replace), delete
  composables/useFilterEditor.ts the filters page: search, the filter edited, its table and columns
  composables/useSources.ts    upload, delete, layers, load into a table
  composables/useSql.ts        the derived table: its query, the tables whose columns it offers
  composables/useRowWkt.ts     a row's geometry as WKT in an EPSG code, its copy
  utils/conditions.ts          ID_COLUMN, isGeometry, attributeColumns, sqlIdentifier, columnIdentifier, insertToken, andAll, selectionClause, filterFits, distinctColumns
  utils/duckdbDocs.ts          DUCKDB_DOCS, the pages of DuckDB's documentation offered on the SQL page
  utils/rowValues.ts           textOf, draftOf, changedValues, rowIdOf, rowSortOf, sameSort
  utils/tableLayers.ts         TABLE_LAYERS: the platform's vectorLayers of the open table (source darow-table, tile layer features)
  utils/tableName.ts           tableNameOf: a table name suggested from a file name
  utils/geometry.ts            a row's geometry: read, as text and back, its preview
  utils/settingsForm.ts        SETTING_FIELDS, the settings as form texts and back
  utils/crs.ts                 sridOf: an EPSG code typed as text, 0 when empty or not a code
  utils/tableLinks.ts          tableLinks: the addresses another tool reads a table at
  utils/catalog.ts             listed, COLUMN_TYPES, SOURCE_FORMATS: the generated unions ColumnType and Extension as lists, each value type-checked
  views/TablesView.vue         the tables page (§8)
  views/TableHeader.vue, LinksMenu.vue, PanesBar.vue, FiltersBar.vue, FiltersModal.vue, TableRows.vue, TableMap.vue, NameModal.vue,
                               ExportModal.vue
  views/WhereEditor.vue        a SQL condition (v-model) and its assistant, shared by the three pages
  views/FiltersView.vue        the filters page (§9), views/FilterForm.vue
  views/TableEditors.vue       the editors of the open table: RowModal (with GeometryEditor and
                               RowWkt), ColumnsModal (with TransformForm), TableSettingsModal
  views/SettingsView.vue       the global settings, views/SettingsFields.vue
  views/SourcesView.vue        the sources page (§10), views/LoadSourceModal.vue
  views/SqlView.vue            the SQL page (§10)
```

Each directory keeps its tests in `__tests__` beside what they cover.

## §4 Files and routes

### §4.1 Routes

`routeNames.ts`: `DarowRoutes`, one name per route.

| Path             | Name             | Meta                                      |
| ---------------- | ---------------- | ----------------------------------------- |
| `/tables/:name?` | `darow-tables`   | `titleKey: darow.pages.tables`, `?filter=<id>` applies a saved filter |
| `/filters`       | `darow-filters`  | `titleKey: darow.pages.filters`           |
| `/sources`       | `darow-sources`  | `titleKey: darow.pages.sources`           |
| `/sql`           | `darow-sql`      | `admin`, `titleKey: darow.pages.sql`      |
| `/settings`      | `darow-settings` | `admin`, `titleKey: darow.pages.settings` |
| `/`              | redirect         | `darow-tables`                            |

Every page is the `ListDetailLayout` or `PageTemplate` of the platform; a component is imported
lazily.

## §5 Catalog

`i18n/fr.ts` holds `nav`, `pages`, `actions`, `tables`, `table`, `row`, `filters`, `sources`, `sql` (with `docLinks`, one per page of `DUCKDB_DOCS`), `errors` (one per code of `spec/darow/back.md` §8, its
params in braces), `geometry`, `columns`, `settings` (`fields` and `hints` per setting) and
`erasures` (`tables`, `filters`: `label`, `description`), `export` (the export's modal) and `wkt`
(a row's WKT).

## §6 Data

### §6.1 Client

`api/darow.ts` calls every route of `spec/darow/back.md` §6 over the platform's helpers (§0 rule 1):
the tables, their rows (`listRows` of a `RowPage{offset, limit, where, sort}`, the sort sent as
`sort` and `desc`, both left out when empty), a row, its WKT (`getRowWkt(name, id, srid)`), the columns, the
transforms, the tiles' and exports' URLs (`where` the condition applied, `srid` the EPSG code of
the export, left out at 0), the sources, the
settings and the saved filters (`resourceApi`).

### §6.2 Stores and composables

`stores/` holds the lists the events refresh: the tables, the sources, the saved filters. The open
table's state — detail, page of rows, page, page size, sort, draft and applied condition, active saved filter, row
opened, selection (the `__id` of the rows selected), tiles' revision — lives in `useTable`, shared by the views of the tables page (§8).

## §7 Live updates

`setupDarowEvents` refetches a loaded store on `darow-tables`, `darow-sources`, `darow-filters`
(`spec/darow/back.md` §7).

## §8 The tables page

`ListDetailLayout`: the list of the tables (search on the name), the route's `name` opening one. The
detail:

- `TableHeader` — the name and, for `canAdmin`, its rename (`NameModal`, faded until hovered)
  beside it; on the right icon buttons: for `canEdit` a new row (green `+`) and the columns, the menu of the links
  (`LinksMenu`), the export and, for `canAdmin`, the table's settings.
- `LinksMenu` — an icon button opening a popover listing `useTable.links` (`tableLinks`, absolute addresses): for a table
  with a geometry the OGC API service (`/ogc`, the address QGIS is given, unfiltered), the GeoJSON
  export and the vector tiles (layer `features`), then the CSV export; these three carry
  `useTable.conditions`, told « avec les filtres actifs » when there is one. Each entry shows its
  use and its address; a click copies the address (the platform's `useCopy`, a toast).
- The detail fills the pane: `FiltersBar` and `PanesBar` on one line — one bordered group holding,
  for a table with a geometry, the toggles of the table and the map (`usePanes`: the table alone at
  first, never both hidden, hiding the table alone shows the map), then the `ColumnPicker` of
  `TableRows` while it is shown —, then the map and the table on a grid, side by side in two equal columns from `xl` (one
  above the other below it, `usePanes.split`) sharing the space left, each scrolling on its own, then a footer that stays: for `canAdmin` the table's delete (a red
  button, confirmed), and `TablePager` (the count of rows passing the filter, the pages) driving
  `useTable.setPage`.
- `ExportModal` — the format (CSV, and GeoJSON for a table with a geometry) and, with a geometry,
  the EPSG code (`sridOf`, empty for CRS84); exporting is `useTable.download(format, srid)`:
  `window.location` to the export of `useTable.conditions`.
- `FiltersBar` — a filter button opening `FiltersModal`, then a toggle button per saved filter
  offered on the table (`filterFits`: saved on it or on any table, `useSavedFilters.forTable`),
  filled while active (`useTable.activeId`), its condition as title, a click activating or
  deactivating it (`useTable.toggleFilter`, from the first page; one at a time, activating one deactivates the other), the condition
  applied as a badge (its SQL) whose remove takes it out (`useTable.unapply`, from the first page)
  and, while rows are selected: « Sélection (n) », a toggle filled while on
  (`useTable.toggleSelection`, from the first page: `selectionOnly`, the rows kept to
  `"__id" IN (<ids>)`, `selectionClause`, following the selection as it changes), and its clear.
  Every change of the selection goes through `useTable.setSelected` (`pick`, `clearSelection`, the
  table's checkboxes), which reads the first page again while the toggle is on and turns it off
  once nothing is selected.
- `FiltersModal` — the draft in `WhereEditor`, a button to the filters page (where a saved filter
  is deleted) and apply (enabled while the draft differs from what is applied, closing the modal);
  for the writer, a name and save, enabled with a name and a condition, creating a filter of the
  table with the draft.
- `WhereEditor` — the condition as SQL in a textarea, written freely (AND, OR, parentheses,
  functions), and a button per column (`__id`, then each attribute): a click appends its quoted name
  (`sqlIdentifier`) to the text, a space before and after (`insertToken`).
- Opened with `?filter=<id>`, the page activates that saved filter once the table is open, when it
  is offered on the table.
- The rows, the tiles, the export and the transforms read `useTable.conditions`: `andAll` of the
  condition applied, that of the active filter and, while `selectionOnly`, the selection's, each in
  parentheses; opening a table deactivates them. A condition the server refuses is told by its toast.
- `TableMap` — `BaseMap`, the table's vector tiles of `useTable.conditions` as a fill, a line and a
  circle layer (the platform's `addVectorLayers` over `TABLE_LAYERS`), the selected features drawn over them in another colour,
  framed on the table's extent. Over it, top left, the platform's `PickToolbar`: **Sélectionner**,
  a button recentring on the table's extent (the platform's `fitExtent`), the count of the selection and its
  clear. The platform's `bindVectorPicking` over `TABLE_LAYERS.layers`: a click on a feature opens its row; while selecting or with Ctrl/⌘ it
  adds it to the selection or takes it out (`useTable.pick`, `toggle`); a box drawn while selecting
  or with Shift (`bindBoxSelect`) adds the features it holds (`add`); the pointer over a feature.
  For `canEdit`, **Éditer** (`useMapEditing.toggle`, asking before dropping a change) edits the
  geometries on the map itself: the platform's `useGeometryEditor` over `useMapEditing.geometry`,
  snapping to the exact geometries of the table's features (`useRowGeometries`), its neighbours
  `useMapEditing.neighbours`, attached while editing and detached after, its
  `GeometryEditToolbar` (with **Édition topologique**) under the `PickToolbar` with **Nouvel
  objet**, **Annuler les modifications** and **Enregistrer** (both enabled once the geometry or a
  neighbour changed), and bottom left the
  row in hand (« Objet n », or the new object to draw) and the editor's hint. While editing, a
  click on a feature — not while drawing, nor on a handle (the platform's `overEditHandle`) — takes it in hand
  (`useMapEditing.click`, then `edit`: its row read, asking before dropping a change), left out of
  the tiles with the changed neighbours (`useMapEditing.hidden`, `hideVectorFeatures`), the exact geometries around it read beforehand (`useRowGeometries`' `around`);
  **Sélectionner** is off. Saving updates the row's geometry alone, or adds the drawn object and
  opens its row (`RowModal`) for its attributes, with each changed neighbour's geometry: one
  request, `writeRows`, all or none — refused, nothing is written and the edit stays, to save again
  or cancel —, then `useTable.changed` reads the tiles again and the editor forgets its history.
  Cancelling drops the changes and the neighbours'. Another table ends the editing.
- `useRowGeometries` — the platform's `useExactGeometries` over `TABLE_LAYERS.layers`, each
  feature's exact geometry read once from its row (`getRow`, `geometryOf`); cleared when the tiles
  change. The tiles' geometries, simplified and quantized, only find the features.
  `useTable.pick(ids, mode)` folds them into the selection through the platform's `pickIds`.
- `TableRows` — `DataTable` of the page of rows (the table's `rowsPageSize`, read with its settings
  when it opens, numbered by `__id`, a click on it opens the row), `selectable`, its checked rows the
  selection of `useTable`, shared with the map and kept across pages, mounted again for each table
  (`key`), its widths, sort and shown columns kept per table (`darow-<table>-sizing`, `-sorting`,
  `-columns`). The rows come one page at a time from the
  server: the table sorts with `serverSorting`, one column at a time, `__id` included; its `sort`
  goes through `rowSortOf` (a column's id is `col-<name>`) to `useTable.setSort`, which reads the
  first page in that order and ignores a sort it already applies; opening a table forgets the sort
  until its `DataTable` emits its stored one. Each attribute cell is the platform's `EditableCell`,
  read-only outside the editing. For `canEdit`, above the table, the platform's `CellEditToolbar`
  over `useCellEditing` — the platform's `useCellChanges` — opens the cells, asking before dropping
  a change, and, while editing, counts the changed cells and the rows and columns to remove,
  cancels or saves them. While editing, a `RemoveToggle` before each row's number and beside each
  column's label marks it to remove, struck through (`line-through opacity-50`), a second click
  keeping it. A changed cell shows its new text, marked (`changed`), until saved, kept per row
  across pages. Saving asks first when rows or columns are to be removed, then writes every changed
  row, the rows deleted and the columns dropped in one request, `writeRows`, all or none (an emptied
  cell is null) — refused, the changes stay —, then `useTable.changed` reads the table, the page and
  the tiles again. Another table ends
  the editing. It exposes its `DataTable`'s `picker`: the platform's
  `ColumnPicker` over it, on the line of `FiltersBar`, shows or hides the columns.
- `TableEditors`, a sub-view holding the editors of the open table:
    - `RowModal` — a row read alone, or a new one (`{}`), in a wide modal: its attributes as text
      fields on two columns (read-only without `canEdit`) and, for a table with a geometry, `GeometryEditor`: the platform's
      `GeometryEditMap` (h-96) — any geometry drawn (point, line, polygon, a hole in a polygon) or edited by its vertices,
      undone, snapping to the exact vertices and segments of the table's features
      (`useRowGeometries`, topology off) — over the table's tiles (`addVectorLayers`, the edited
      row left out by `hideVectorFeatures`), framed
      on the geometry or, without one, on the table's extent; the toggle « GeoJSON » adds the
      geometry as text above it. Without `canEdit`, a `GeoJsonMap` of the geometry. Save sends the values
      that changed (an emptied field is null) and the geometry only when it changed; a new row is
      added. For a row already saved with a geometry, `RowWkt`, a sub-view over `useRowWkt` and the
      open table's name: its geometry as WKT, read when it opens in CRS84, again in the EPSG code
      typed (« Reprojeter »), copied to the clipboard; « Aucune géométrie » when unset. Every write
      bumps the map tiles' revision (`v` of their URL), so that the map reads
      them again; it starts at the page's load time, so that a page never asks a URL an older
      one cached.
    - `ColumnsModal` — the attributes with rename (`NameModal`) and delete (confirmed), a column
      added (name, type of `COLUMN_TYPES`, `utils/catalog.ts`), and `TransformForm`: the column, the SQL expression, on
      the rows of `useTable.conditions` or on all, a preview (count and up to ten `before`/`after`)
      then the transform.
    - `TableSettingsModal` — the table's four overrides, empty to inherit, the value applied shown
      as placeholder.

## §9 The filters page

`ListDetailLayout`: the saved filters (`<name> · <table>`, « toute table » for a filter of any
table, search on the name or the table, ordered by `sortList` on the name) and, for `canEdit`, the
platform's green `+` (`btn btn-sm btn-square btn-success`) creating a filter of any table. The detail
is a copy of the filter (`useFilterEditor.draft`) in `FilterForm`: its name, its table (empty: any
table) and its condition (`WhereEditor`, its assistant over the table's columns, or the columns of
every table once each, `distinctColumns`), read-only without `canEdit`. The header: open
the table with the filter (`/tables/<table>?filter=<id>`, a filter saved on a table), save (enabled
with a name and a condition: create, or replace the one opened, which stays open) and
delete (confirmed, closing it).

## §10 Sources, SQL and settings

`SourcesView`: for the writer, a `FormFileInput` (the accepted extensions of the back, `SOURCE_FORMATS`) and send,
with the upload's progress and cancel; then the `DataTable` of the sources (name, size in mebibytes by the platform's `formatSize`, date) with,
for the writer, load (`LoadSourceModal`: the table name suggested by `tableNameOf`, the layer when
the file has several, the EPSG code, empty to read it from the file) and delete (confirmed).

`SqlView`: one card « Nouvelle table »: the name of the table to create; the tables read (a
multiple select of every table, `useSql.picked`), each picked one on a line: its name, then a button
per column (its type as title), a click appending `"<table>"` or `"<table>"."<column>"`
(`columnIdentifier`) to the query (`useSql.insert`, `insertToken`); the SELECT, written by hand;
links to DuckDB's documentation (`DUCKDB_DOCS`: SQL, SELECT, functions, spatial functions, in a new
tab); create, which opens the new table.

`SettingsView`: the four global settings (`SettingsFields`, their bounds as hints), saved at once.

## §11 Tests

`utils/__tests__`: `catalog` (the keys of a record, both lists), `conditions` (`sqlIdentifier`, `columnIdentifier`, `insertToken`, `andAll`, `selectionClause`, `filterFits`, `distinctColumns`), `rowValues` (with `rowSortOf`, `sameSort`), `tableName`, `geometry`, `settingsForm`, `crs`, `tableLinks` (the CSV alone without geometry, the OGC service unfiltered, the data under the condition). `composables/__tests__`, the client mocked: `useTable` (open, page
offset, apply from the first page, a sort from the first page once and forgotten by another table,
the condition applied taken out, saved filters activated and deactivated, a selection toggled and added to, its toggle on, following it, off, off once cleared and forgotten by another table, no toggle without a selection, rename followed, row opened, saved and deleted, a row added and the tiles' revision bumped, the links under the conditions, table deleted), `useMapEditing` (a new object at first then a feature in hand, a click taking it in hand unless the editor uses it or editing is off, the hidden features, its geometry saved alone then a change cancelled, a drawn object added with its row opened, a change kept when its drop is refused, stopped by another table, the changed neighbours saved and cancelled, those of a new object saved, an edit kept when the server refuses it), `useCellEditing` (the changed rows written in one request with an emptied cell as null, the rows deleted and the columns dropped in the same one, kept when refused, stopped by another table), `useRowGeometries` (a feature of the table's layers read from its row), `useColumns` (add, rename,
drop reloading the table, preview, transform, a refused transform), `useSettings` (global, a
table's, a refusal), `useSavedFilters` (all and those offered on a table, save of the condition trimmed, a
refusal, delete), `useFilterEditor` (search, open a copy, create a filter of any table, change of table, save new then
edited, replace, delete), `useSql` (create, the columns of the tables picked, a name inserted, a refusal), `useSources` (list, upload, load into a table, layers, delete), `useRowWkt` (CRS84 then
the code typed, an unset geometry, a refusal, the copy), `usePanes` (the table alone at first, the
map beside it then alone, the table back when the map hides, no map without a geometry).
