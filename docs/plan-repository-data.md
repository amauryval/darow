# Plan — Repository pour les tables de données

Pendant du plan catalogue (`docs/plan-repository-metadata.md`), appliqué cette fois aux **tables
référencées par `metadata`** : les tables physiques `t_<uuid>` et tout ce qu'on en lit ou y écrit
(lignes, schéma, géométrie, tuiles, features).

Ce plan est un document de conception : il décrit la cible et l'ordre des étapes. Il ne fixe pas
les noms au caractère près (qui doivent être en anglais).

---

## 0. Réponse à la question de séquencement

**Les deux plans se font en parallèle, pas l'un après l'autre.** Ils touchent des fichiers
disjoints :

| Plan | Fichiers |
|---|---|
| Catalogue | `duckdb/adapter.go`, `duckdb/migrations.go`, `service/metadata.go` |
| Données | `duckdb/table.go`, `duckdb/mvt.go`, `duckdb/features.go`, `duckdb/create.go`, `service/rows.go`, `service/schema.go`, `service/geo.go` |

Ils ne se croisent qu'en **deux points**, tous deux en fin de parcours :

1. **`storage.Storage` et `storagetest`.** Le plan catalogue en retire `MetadataManager` ; celui-ci
   en retire `TableManager`/`GeoTableManager`. C'est la même opération, faite une fois, quand les
   deux sont prêts (étape commune, §5).
2. **`Metadata` comme entrée.** Ce plan-ci consomme la `Metadata` que le catalogue produit. Tant que
   la struct ne change pas de forme — et elle ne change pas, seul son *chemin d'écriture* est
   retypé — l'un n'attend pas l'autre.

**Ordre recommandé :** commencer par les étapes 1 et 2 du plan catalogue (la dette de migration est
la seule urgence réelle du dépôt), puis mener les deux plans de front. Les étapes 1 à 4 de ce
plan-ci sont indépendantes de tout ce qui se passe côté catalogue.

Un point à ne **pas** faire en parallèle : les deux plans veulent renommer/déplacer
`internal/storage/`. Se mettre d'accord sur l'arborescence cible (§2.2 ici et §2.2 du plan
catalogue) **avant** de démarrer l'un ou l'autre, sinon les deux se marchent dessus au premier
déplacement de fichier.

---

## 1. Problème

### 1.1 Un `Metadata` passé de main en main, revalidé partout

Chaque méthode de `table.go`, `mvt.go` et `features.go` prend `(ctx, meta, …)` et recommence le
même préambule : valider `physicalName`, guillemeter `geometryColumn`, assembler par `fmt.Sprintf`.

| Motif | Occurrences |
|---|---|
| `ValidateIdentifier(meta.PhysicalName)` | 23 |
| `QuoteIdentifier(meta.GeometryColumn)` | 10 |
| Projection `EXCLUDE (<geom>)` réécrite | 4 |
| Bloc de sélection d'attributs dupliqué | 2 (`features.go:30`, `:115`) |
| Scanners génériques quasi identiques | 2 (`table.go:184`, `features.go:165`) |

La règle 1 est donc tenue **par discipline, répétée 23 fois**, jamais par construction. Elle tient
aujourd'hui — l'audit l'a vérifiée — mais chaque nouvelle méthode est une occasion de l'oublier, et
rien ne le signalerait à la compilation. `InsertRow` valide d'ailleurs deux fois (audit §3.11) :
personne ne peut suivre.

### 1.2 Des états illégaux représentables

Rien dans les types n'empêche d'appeler une opération géométrique sur une table sans géométrie.
C'est exactement ce qui se produit aujourd'hui : `GeoHandler` ne teste jamais `IsGeo`, et
`/tiles`, `/geometry-types`, `/bbox`, `/filtered-extent` sur une table scalaire descendent jusqu'à
`QuoteIdentifier("")` pour ressortir en **500** (audit §3.12). L'invariant existe, il est juste
écrit dans un seul handler sur deux.

### 1.3 Aucun point de passage unique

Les défauts suivants ont tous la même cause racine — il n'y a **aucun endroit** où les mettre :

| Défaut | Audit |
|---|---|
| N+1 sur le schéma : un `DESCRIBE` par colonne | §3.1 |
| `SELECT DISTINCT` et `FilterByBbox` sans plafond | §3.9 |
| `IN ()` sur liste vide | §3.10 |
| Double validation morte | §3.11 |
| Erreur WKT non typée → 500 | §3.3 |

Chacun se corrige aujourd'hui par un `if` ajouté dans 3 à 20 méthodes, ou pas du tout. Avec un
repository, chacun se corrige **une fois**, à la construction ou dans le constructeur de requête.

### 1.4 Ce que ça coûte à la prochaine évolution

Ajouter une opération sur les données (un `COUNT` filtré, un `ST_Centroid`, un export de tuile
vectorielle dans un autre format) demande aujourd'hui de recopier le préambule, de recopier la
projection, de recopier le scanner, et de se souvenir des cinq gardes ci-dessus. Le code obtenu
sera correct ou non selon la vigilance du moment.

---

## 2. Cible

### 2.1 Le principe : une poignée typée, validée à la construction

Au lieu d'un adapter plat qui reçoit `Metadata` à chaque appel, un **repository lié à une table**,
obtenu une fois par requête HTTP.

« Lié à une table » désigne une **instance**, pas un type : les trois types (`TableRepository`,
`RowRepository`, `GeoRepository`) sont écrits une fois pour toutes, et c'est la valeur construite à
partir d'une `Metadata` donnée qui est propre à une table, le temps d'une requête. Ajouter une table
logique n'ajoute pas une ligne de code.

- La résolution `Metadata` → repository **valide les identifiants une seule fois** et pré-calcule
  ce qui sert partout (nom physique validé, colonne géométrique guillemetée, projection
  d'exclusion). L'existence de l'objet **est** la preuve que ses identifiants sont sûrs : la
  règle 1 devient structurelle.
- Les méthodes perdent leur paramètre `meta`. Une signature devient
  `Rows().ByPkey(ctx, pkeys)` au lieu de `GetRowsByPkey(ctx, meta, pkeys)`.
- **L'accès géo n'est obtenable que depuis une table géo.** Une méthode `Geo() (GeoRepository,
  bool)` (ou un `GeoRepository` distinct rendu par la résolution) rend l'invariant de 1.2
  impossible à oublier : un handler qui veut la géométrie doit passer par un chemin qui,
  structurellement, n'existe pas pour une table scalaire.

C'est le même mouvement que le plan catalogue — remplacer une convention par un type — appliqué à
l'autre moitié du storage.

### 2.2 Découpage cible

```
internal/storage/
├── catalog/                  ← plan catalogue
├── data/                     ← nouveau : contrats des tables de données
│   ├── table.go              ← TableRepository (résolution, schéma), Row
│   ├── rows.go               ← RowRepository : lecture/écriture de lignes
│   ├── geo.go                ← GeoRepository : tuiles, features, emprises
│   └── params.go             ← RowQuery, Page, SortField, GeoQuery
├── duckdb/
│   ├── catalog/              ← plan catalogue
│   ├── data/
│   │   ├── open.go           ← Metadata → repository : LA validation d'identifiants
│   │   ├── sql.go            ← constructeur de requêtes interne (projection, IN, ORDER BY, LIMIT)
│   │   ├── scan.go           ← scanner générique unique (Row et GeoFeature)
│   │   ├── rows.go           ← CRUD
│   │   ├── schema.go         ← DESCRIBE unique (supprime le N+1)
│   │   └── geo.go            ← tuiles, features, extent
│   └── create.go, derive.go  ← création de table : inchangés dans leur logique
└── storagetest/              ← scindé en suite « catalogue » et suite « données »
```

`create.go`/`derive.go` restent à part : ils créent une table qui **n'existe pas encore**, donc ils
n'ont pas de `Metadata` complète à résoudre. Ils gardent leur préambule propre, avec un seul
`ValidateIdentifier(physicalName)` chacun.

### 2.3 Contrats

**`TableRepository`** — obtenu par résolution depuis une `Metadata`, porte le schéma et donne accès
aux deux sous-repositories.

| Méthode | Remplace | Gain |
|---|---|---|
| `Schema(ctx) ([]Column, error)` | `Columns` + N × `ColumnType` | **une seule** requête `DESCRIBE` (audit §3.1) |
| `Types(ctx) (map[string]string, error)` | idem, côté `SchemaService` | dérivé du même appel |
| `AddColumn(ctx, name, typ) error` | identique | inchangé |
| `Count(ctx) (int64, error)` | `RowsCount` | inchangé |
| `Rows() RowRepository` | — | groupe les opérations de lignes |
| `Geo() (GeoRepository, bool)` | — | **n'existe que pour une table géo** (audit §3.12) |

**`RowRepository`** — plus de `meta`, plus de préambule.

| Méthode | Remplace | Gain |
|---|---|---|
| `Pkeys(ctx, sort, offset, limit)` | `GetPkeys` | tri validé au même endroit |
| `ByPkey(ctx, pkeys)` | `GetRowsByPkey` | **garde liste vide** (audit §3.10) |
| `Filtered(ctx, RowQuery)` | `GetRowsWithFilter` | plafond appliqué au constructeur (audit §3.9) |
| `Distinct(ctx, column, limit)` | `GetDistinctValues` | plafond obligatoire dans la signature |
| `Insert(ctx, values)` / `InsertMany(ctx, rows)` | `InsertRow` / `BulkInsert` | erreur WKT typée (audit §3.3) |
| `Update(ctx, pkeys, updates)` | `Update` | idem |
| `Delete(ctx, pkeys)` | `RemoveRows` | **garde liste vide** |

**`GeoRepository`** — inaccessible sur une table scalaire.

| Méthode | Remplace |
|---|---|
| `Tile(ctx, z, x, y, attrs, q)` | `Tile` |
| `Features(ctx, FeatureQuery)` | `GetFeatures` |
| `FeaturesInTile(ctx, z, x, y, FeatureQuery)` | `GetFeaturesTile` |
| `PkeysInBbox(ctx, bbox, limit)` | `FilterByBbox` — plafond obligatoire |
| `Extent(ctx, q)` | `FilteredExtent` |
| `GeoJSONByPkey(ctx, pkeys)` | `GetGeometryGeoJSON` |
| `Types(ctx)` | `GeometryTypes` |

`FeatureQuery` regroupe ce que `GetFeatures` prend aujourd'hui en sept paramètres positionnels
(`pkeys`, `bbox`, `attrs`, `q`, `limit`, `offset`) — la signature actuelle est au bord de
l'illisible et les deux appels de `features.go` diffèrent uniquement par des `nil` positionnels.

### 2.4 Le constructeur de requêtes interne

Un petit assembleur **non exporté**, propre au paquet `duckdb/data`, qui centralise les fragments
recopiés aujourd'hui : projection avec exclusion de la géométrie, clause `__pkey IN (…)`, `ORDER BY`
avec départage `__pkey` (règle 8), `LIMIT/OFFSET`, composition des `WHERE`.

Il n'est **pas** un ORM ni un query-builder générique : c'est le vocabulaire fermé de ce domaine, et
il ne doit accepter que des identifiants déjà validés par la résolution (§2.1). C'est ce qui permet
que l'assemblage SQL cesse d'être un `fmt.Sprintf` par méthode.

### 2.5 Scanner unique

`scanRows` et `scanFeatureRows` font la même chose à un champ près (`__geojson`). Un scanner
générique unique, paramétré par les colonnes réservées à extraire (`__pkey`, `__geojson`), remplace
les deux.

### 2.6 Ce que deviennent les services

Comme pour le catalogue, la couche service **ne change pas de rôle**, seulement de vocabulaire :

- `RowService` reçoit un `TableRepository` au lieu d'un `TableManager` + `Metadata` ; sa conversion
  de types (règle 12) et son `RefreshDerived` conditionnel (règle 9, règle 22) sont inchangés.
- `SchemaService` garde son cache, mais alimenté par un seul appel.
- `GeoService` garde son cache de tuiles, alimenté par un `GeoRepository`.

**Les handlers ne changent pas.** `handlers/ports.go` reste identique : c'est, là encore, le signe
que la couture est au bon endroit.

Le seul changement visible côté router est bénéfique : la résolution `Metadata` → repository est le
point naturel où `GeoHandler` obtient (ou non) son accès géo, ce qui referme §3.12 sans ajouter de
`if` dans cinq handlers.

---

## 3. Étapes

Chaque étape est livrable et testable seule, dans cet ordre. Chaque étape inclut ses tests
(règle 17).

**Étape 1 — Résolution et validation unique.**
Créer `duckdb/data`, y introduire la résolution `Metadata → repository` avec la validation
d'identifiants et les fragments pré-calculés. Réimplémenter `TableManager`/`GeoTableManager`
**par-dessus**, sans toucher aux signatures publiques : les 23 préambules disparaissent, tous les
tests existants passent inchangés. Étape mécanique, elle isole le risque.

**Étape 2 — Constructeur de requêtes + scanner unique.**
Sortir les fragments dupliqués (§2.4, §2.5). Rien ne change pour l'appelant ; le diff est une
déduplication. C'est le moment où le paquet devient relisible.

**Étape 3 — Schéma en une requête.**
`Schema(ctx)` par un seul `DESCRIBE`, `Types` dérivé. Supprime le N+1 (audit §3.1) et corrige le
commentaire de `SchemaService` qui décrit aujourd'hui une intention non implémentée. Gain mesurable
immédiatement sur `/rows`, `/filter`, `/export`.

**Étape 4 — Gardes et plafonds.**
Liste vide sur `ByPkey`/`Delete` (audit §3.10), plafond obligatoire dans la signature de `Distinct`
et de `PkeysInBbox` (audit §3.9), erreur WKT typée (audit §3.3). Ces trois correctifs ne sont
possibles proprement qu'une fois qu'un point de passage unique existe — c'est le retour sur
investissement des étapes 1 et 2.

**Étape 5 — Exposer les repositories aux services.**
Remplacer `TableManager`/`GeoTableManager` par `TableRepository`/`GeoRepository` dans les ports de
`RowService`, `SchemaService`, `GeoService`, `ExportService`. Adapter les mocks. Les handlers et
`ports.go` ne bougent pas.

**Étape 6 — Rendre l'accès géo structurel.**
`Geo() (GeoRepository, bool)`, `GeoHandler` obtient son accès par ce chemin et renvoie 404 quand il
n'existe pas. Referme audit §3.12 par construction plutôt que par un `if` recopié.

**Étape 7 — `FeatureQuery`.**
Remplacer les sept paramètres positionnels de `GetFeatures` par une struct. Purement ergonomique,
donc en dernier — mais c'est ce qui rend l'ajout d'un futur critère (un `datetime`, un tri) non
invasif.

Les étapes 1 à 4 sont indépendantes du plan catalogue et peuvent démarrer immédiatement après ses
étapes 1 et 2. Les étapes 5 à 7 gagnent à être faites après l'étape 5 du plan catalogue (erreurs
typées), sans en dépendre strictement.

---

## 4. Ce que ce plan referme

Sept constats de `docs/audit-go.md` se dissolvent dans ce plan plutôt que d'être corrigés
séparément :

| Constat | Étape |
|---|---|
| §3.1 N+1 sur le schéma | 3 |
| §3.3 erreur WKT non typée | 4 |
| §3.9 lectures sans plafond | 4 |
| §3.10 `IN ()` sur liste vide | 4 |
| §3.11 double validation morte | 1 |
| §3.12 endpoints géo sans contrôle `IsGeo` | 6 |
| §4.10 absence de repository côté données | 1–2 |

C'est l'argument économique du plan : pris isolément, chacun de ces correctifs est un patch de
quelques lignes recopié dans N méthodes. Pris ensemble, ils sont la conséquence d'un seul
changement de forme.

---

## 5. Étape commune aux deux plans

À faire **une fois**, quand les deux plans ont atteint leur étape 5 :

1. Démanteler l'agrégat `storage.Storage` : le catalogue et les données sont deux composants
   distincts, injectés séparément dans `core.NewServices`. `storage.Storage` n'a plus de raison
   d'exister sous cette forme.
2. Scinder `storagetest` en une suite « catalogue » et une suite « données », chacune exécutable
   contre son propre adapter. La valeur de la suite (prouver qu'un futur adapter PostGIS tient le
   contrat) est préservée, et même améliorée : un adapter pourra tenir un contrat sans tenir
   l'autre.
3. Régénérer `storage/mocks` en conséquence.

Tant que cette étape n'est pas faite, les deux plans cohabitent sans conflit : les interfaces
actuelles restent en place comme façades au-dessus des repositories.

---

## 6. Risques

| Risque | Traitement |
|---|---|
| Deux refontes simultanées sur `internal/storage/` | Figer l'arborescence cible (§2.2 des deux plans) **avant** de commencer ; ensuite les fichiers touchés sont disjoints |
| L'étape 1 déplace beaucoup de code d'un coup | Aucun changement de signature publique : les tests existants sont le filet, la revue se fait au diff de déplacement |
| Sur-abstraction : « un repository par table » compris comme un type Go par table logique | Le nombre de **types** est fixe : trois (`TableRepository`, `RowRepository`, `GeoRepository`), écrits une fois. Ce qui est « par table », c'est l'**instance** — une valeur construite à partir d'une `Metadata` au début d'une requête, jetée à la fin. Ajouter une table logique n'ajoute aucun type ; un quatrième type ne se justifie que par une famille d'opérations réellement nouvelle, jamais par une table de plus |
| Le constructeur de requêtes dérive vers un mini-ORM | Il reste non exporté et limité au vocabulaire de ce domaine ; toute méthode acceptant une chaîne SQL libre est un échec de conception (règle 1) |
| Coût de résolution par requête | La résolution ne fait que valider des chaînes déjà en mémoire — aucune requête base. La `Metadata` vient du cache (§4.2 du spec), inchangé |
