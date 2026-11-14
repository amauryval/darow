# Findings

The gaps the audits found between the specs and the code, one row per finding, grouped by kind.
Scope is where the fix lands: `darow` or `platform`, with its side (`back`, `front`), or `project`.
Identifiers are stable: `BLK-n`, `LIM-n`, `TODO-n`, `IMP-n`, with an `F` for a front finding and a
`P` for a project finding.

## Blockers

What breaks correct use — fixed before any new feature.

| Id    | Scope      | Finding                                                                                                                                       | Effect                                                                                     | Reference                 |
| ----- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------ | ------------------------- |

_None identified to date._

## Limitations

What is deliberately not done — scope decisions, not defects.

| Id     | Scope       | Finding                                                                                        | Reason                                                         | Reference                 |
| ------ | ----------- | ---------------------------------------------------------------------------------------------- | -------------------------------------------------------------- | ------------------------- |
| LIM-1  | darow back  | The source files are not archived                                                              | inputs, uploaded again when lost; the archives stay small      | ADR-0002                  |
| LIM-2  | darow back  | OGC API - Features Parts 1 and 2 only: no CQL filter, no HTML                                  | the Core, GeoJSON and CRS classes are what a GIS client needs  | ADR-0004, ADR-0007        |
| LIM-4  | darow back  | The first start downloads DuckDB's spatial extension                                           | DuckDB's own distribution of its extensions                    | `spec/darow/back.md` §5.1 |
| LIM-5  | darow back  | A column transform has no undo but the platform's archives                                     | the preview shows what it writes before it writes              | `spec/darow/back.md` §4.6 |
| LIM-6  | darow back  | No WFS 2.0 service (GetCapabilities, DescribeFeatureType, GetFeature in GML) | OGC API - Features, its successor, is what QGIS, GDAL and ArcGIS Pro read; WFS would serve only older clients | ADR-0004, `spec/darow/back.md` §6.5 |
| LIM-7  | darow back  | A tile filtered by the selection (`"__id" IN (…)`) once failed with DuckDB's `INTERNAL Error: Operation requires a flat vector` (spatial extension's optimizer), not reproduced; the engine reopens the file on such an error and renders the tile again | a bug of DuckDB's, beyond darow; its effect is contained, its message logged | `spec/darow/back.md` §5.1 |
| LIM-F2 | darow front | A neighbour is snapped to once its exact geometry is read (`getRow`, on approach; those around the edited object beforehand) | the tiles only find the features; their geometries are simplified and quantized | `spec/darow/front.md` §8 |

## Todo

Identified debt — gaps between the spec and the code.

| Id     | Scope      | Finding                                                                                                                                         | Reference                         |
| ------ | ---------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------- |

_None identified to date._

## Improvements

Undecided leads, to be decided before being planned.

| Id     | Scope       | Lead                                                                |
| ------ | ----------- | ------------------------------------------------------------------- |
| IMP-1  | platform    | A desktop binary (`cmd/darow-desktop`), as MyPortfolio's            |
| IMP-2  | darow back  | An HTTP cache of the tiles keyed on the table's last change         |
| IMP-3  | darow front | The open table follows another client's writes: on `darow-tables` naming it, its rows, detail and tiles' revision read again (today only the list is) |
| IMP-4  | darow back  | A CSV read by `read_csv` rather than GDAL's `ST_Read`: typed columns (numbers sorted and compared as numbers), no `OGC_FID` column added |
