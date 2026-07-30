# Audit de la partie Go — GeoAPI

Audit du code Go à l'état actuel de `develop`, après l'exécution des deux plans de refactoring
(`plan-repository-metadata.md`, `plan-repository-data.md`). Il décrit ce que le code fait
aujourd'hui. Chaque constat renvoie à un fichier et une ligne.

`go vet` et `gofmt -l` sont propres, l'intégralité des tests passe (`mise run test`).

---

## 1. Synthèse

**Verdict : les deux refactorings ont abouti et la nouvelle couche est saine, mais l'ancienne n'a
pas été retirée. Le dépôt porte aujourd'hui deux couches d'accès aux données complètes en
parallèle, dont une entièrement morte en production.**

Les repositories (`storage/catalog`, `storage/data`, `duckdb/catalog`, `duckdb/data`) sont bien
faits : contrats backend-agnostiques séparés de DuckDB, résolution et validation des identifiants
une fois pour toutes dans `data.Resolve`, suites de conformité partagées. Les services consomment
déjà exclusivement cette nouvelle couche.

Trois familles de problèmes :

1. **Dette de migration non soldée** (§2) — l'ancienne surface `storage.TableManager` /
   `GeoTableManager` et ses ~20 méthodes d'adaptateur ne sont plus appelées que par les mocks et la
   suite de conformité ; l'agrégat `catalog.Catalog` (cascade transactionnelle) est écrit mais
   jamais branché.
2. **Défauts de correction ciblés** (§3) — suppression de table non transactionnelle, incohérence
   de CRS sur `POST /bbox`, ETag pouvant être malformé, classifications d'erreurs qui renvoient 400
   ou 500 à tort.
3. **Performance** (§4) — deux points chauds réels : `RefreshDerivedMetadata` recalculé
   intégralement après chaque écriture d'une seule ligne, et `SchemaService.Types` reconstruit une
   map par ligne convertie.

Et une quatrième, transverse, traitée à part : **les commentaires renvoient massivement à un
référentiel externe** (`règle N`, `NEW-BUG-N`, `PERF-N`, `audit §3.12`…) au lieu de décrire le code
(§6).

### Métriques

| | |
|---|---|
| Fichiers Go (hors `docs/`, `web/node_modules/`) | 118 |
| Lignes Go | ~15 300 |
| Paquets | 15 |
| Références `règle N` / tags d'audit dans les commentaires | 194, sur 53 fichiers |
| Méthodes d'adaptateur mortes en production | ~20 |

---

## 2. Dette de migration : deux couches d'accès en parallèle

### 2.1 `storage.TableManager` / `GeoTableManager` sont morts en production

`internal/storage/metadata.go` déclare toujours `TableManager`, `TableReader`, `TableWriter` et
`GeoTableManager` — les contrats « une `Metadata` par appel » d'avant le refactoring.
`duckdb/table.go`, `duckdb/features.go` et `duckdb/mvt.go` (293 lignes) les implémentent, mais
uniquement en déléguant : chacune des 20 méthodes fait `repo, err := a.Table(meta)` puis un appel de
repository.

**Aucun service, aucun handler n'appelle plus une seule de ces méthodes.** Elles ne sont maintenues
en vie que par :

- `core/core.go:26-27` — `storageAdapter` embarque encore `storage.TableManager` et
  `storage.GeoTableManager`, ce qui force tout futur adaptateur (et le mock) à les implémenter ;
- `storage/mocks/storage_mock.go` — 250 lignes de mock pour une surface non appelée ;
- `storage/storagetest/data_suite.go` — la suite de conformité « données » teste `storage.Storage`
  au lieu de `data.TableResolver`.

Conséquence pratique : chaque méthode de délégation **refait un `data.Open` (donc un
`ValidateIdentifier` + `QuoteIdentifier`) à chaque appel**, exactement le coût que `TableResolver`
devait supprimer. Le commentaire de `duckdb/table.go:14-16` décrit d'ailleurs lui-même ces méthodes
comme celles que les services n'utilisent plus.

**Action.** Supprimer `duckdb/table.go`, `duckdb/features.go`, `duckdb/mvt.go`, les interfaces
`TableManager`/`TableReader`/`TableWriter`/`GeoTableManager` de `storage/metadata.go` et l'agrégat
`storage.Storage` de `aggregate.go` ; réduire `core.storageAdapter` à
`catalog.MetadataManager + data.TableResolver + storage.TableCreator` ; réécrire `data_suite.go`
contre `data.TableResolver` et amputer `storage_mock.go` d'autant. `storage.Row`, `ColumnInfo`,
`GeoFeature`, `GeoQuery` et `SortField` restent — la nouvelle couche les utilise.

Gain : ~700 lignes de production et de mock supprimées, une seule façon d'accéder à une table.

### 2.2 `catalog.Catalog` : la cascade transactionnelle n'est pas branchée

`duckdb/catalog/catalog.go:36-77` définit `Deleter`, `Catalog.Register` et `Catalog.DeleteTable` —
la suppression d'une table et de tous ses dépendants dans une transaction unique. **Rien n'appelle
`DeleteTable` ni `Register`.**

`Adapter.Remove` (`duckdb/adapter.go:159`) délègue à `Repository.Remove`
(`duckdb/catalog/metadata.go:213`), qui fait deux `Exec` indépendants :

```
DROP TABLE IF EXISTS <physicalName>    -- Exec 1
DELETE FROM metadata WHERE tableId = ? -- Exec 2
```

Si le second échoue, la ligne de métadonnées survit à sa table physique : toute lecture ultérieure
sur ce `tableId` échoue en 500 (« table t_… does not exist »), et la table n'est plus supprimable
sans intervention manuelle. C'est précisément le scénario que `Catalog.DeleteTable` prévient.

**Action.** Router `Adapter.Remove` vers `Catalog.DeleteTable` et y intégrer le `DROP TABLE` dans la
même transaction — ou supprimer `Deleter`/`Register`/`DeleteTable` si le besoin de cascade n'existe
pas. Ne pas laisser les deux.

### 2.3 Fuites de couche résiduelles

- `duckdb/derive.go:21` — `registeredPhysicalNames` fait un `SELECT physicalName FROM metadata` en
  SQL brut depuis le paquet `duckdb`, alors que le catalogue a un repository dédié. Doit devenir une
  méthode du `Repository` catalogue.
- `duckdb/create.go:19` — `describeColumns` n'est qu'un alias d'une ligne sur `data.DescribeColumns`.
  À supprimer, appeler directement.
- `storage.SQLWrapper` n'a plus qu'un consommateur, le harnais de test
  `storagetest/catalog_suite.go:22`. Son commentaire dans `aggregate.go:32-35` lui prête encore un
  rôle de production qu'il n'a plus.
- `catalog.MetadataReader.TableExists` n'a aucun appelant en production, seulement des tests. À
  retirer du contrat ou à assumer explicitement.
- `data.PkeysToArgs` et `data.Placeholders` sont exportés et testés mais ne servent qu'à
  `PkeyInClause`, dans le même paquet. À rendre privés.

---

## 3. Défauts de correction

### 3.1 `POST /tables/{id}/bbox` interprète le bbox en EPSG:3857

`duckdb/data/geo.go:97-106` (`PkeysInBbox`) construit `ST_MakeEnvelope(?, ?, ?, ?)` **sans
reprojection**, alors que la colonne géométrique est stockée en EPSG:3857. Le client doit donc
fournir des mètres Web Mercator.

Tous les autres bbox de l'API sont en OGC:CRS84 : `Metadata.Bbox`, `GeoRepository.Extent`
(`geo.go:138`), et le chemin bbox de `Features` (`geo.go:256`) qui, lui, transforme bien
l'enveloppe. Un client qui passe des degrés à `/bbox` reçoit silencieusement zéro résultat.

**Action.** Aligner `PkeysInBbox` sur `Features` :
`ST_Transform(ST_MakeEnvelope(?,?,?,?), 'OGC:CRS84', 'EPSG:3857')`. C'est une rupture de contrat
pour tout client existant — à annoncer.

### 3.2 L'ETag des tuiles peut être malformé

`handlers/geo.go:208-212` :

```go
return fmt.Sprintf(`"%s-%d-%d-%d-%d-%s-%s-%v"`, ..., q.Filter, q.Args)
```

`q.Args` contient des valeurs fournies par le client via `?filter=`. Une valeur contenant un
guillemet double produit un `ETag` avec un guillemet non échappé au milieu — en-tête invalide
(RFC 9110 §8.8.3), que les caches intermédiaires peuvent rejeter ou tronquer. Le même motif alimente
la clé du cache applicatif (`service/geo.go:434`), où il est inoffensif.

**Action.** Hacher la clé (`fnv` ou `sha256` tronqué) plutôt que de l'interpoler telle quelle. Cela
raccourcit aussi un ETag qui peut aujourd'hui faire plusieurs kilo-octets sur un filtre complexe.

### 3.3 `numberMatched` OGC est le compte de la page

`handlers/features.go:252` renvoie `len(features)` pour `numberMatched` **et** `numberReturned`. La
spécification OGC API - Features définit `numberMatched` comme le nombre total d'éléments
correspondant à la requête, indépendamment de la pagination. Un client conforme qui s'en sert pour
dimensionner sa pagination se trompe systématiquement.

**Action.** Soit ajouter un `COUNT(*)` sur les mêmes critères (`GeoRepository` a déjà tout ce qu'il
faut pour le construire), soit omettre le champ — la spécification l'autorise quand le total est
inconnu. Ne pas laisser une valeur fausse.

### 3.4 Statuts HTTP en dur là où l'erreur peut venir de l'autre côté

`response.WriteError` ne masque le message qu'à partir de 500 — délibéré et correct. Mais plusieurs
handlers court-circuitent `MapServiceError` :

**Erreurs internes classées 400** (le message DuckDB, contenant le `physicalName`, part au client) :

- `handlers/tables.go:240-241` — `AddColumn` : toute erreur devient 400, panne DuckDB comprise.
- `handlers/tables.go:154-155` — `RenameTable` : idem sur `UpdateMetadata`.

**Erreurs clientes classées 500** :

- `handlers/rows.go:80-81` — `ListRows` : un `storage.InvalidSortError` (colonne de tri inconnue,
  fournie par le client) remonte en 500 au lieu de 400.
- `handlers/rows.go:158`, `:199`, `:243`, `:269` et `handlers/tables.go:161`, `:183`, `:205` —
  mêmes 500 en dur, où un `NotFoundError` ou un `ValidationError` peut passer.

**Action.** `response.MapServiceError(err)` partout ; c'est déjà le cas dans la majorité des
handlers, ces occurrences sont des oublis.

### 3.5 `Filtered` ne trie pas

`duckdb/data/rows.go:77-94` construit `SELECT … FROM … WHERE … LIMIT ?` sans `ORDER BY`, alors que
toutes les autres lectures paginées ajoutent `__pkey` en départage. DuckDB ne garantit aucun ordre :
deux appels identiques à `POST /tables/{id}/filter` peuvent renvoyer des lignes différentes dès que
le résultat dépasse la limite.

**Action.** Ajouter `ORDER BY __pkey`.

### 3.6 `Update` et `GeoJSONByPkey` ne se protègent pas d'une liste de pkeys vide

`data/rows.go:217` (`Update`) et `data/geo.go:154` (`GeoJSONByPkey`) appellent `PkeyInClause` sans
vérifier `len(pkeys) > 0`, produisant `__pkey IN ()` — erreur de syntaxe DuckDB. `ByPkey` et
`Delete` ont le garde ; le contrat `data.RowRepository` ne le promet d'ailleurs que pour ces
deux-là. Aucun appelant actuel ne déclenche le cas, mais `Features` avec `Pkeys: []string{}` non-nil
y arrive.

**Action.** Déplacer le garde dans `PkeyInClause` (renvoyer `FALSE` pour une liste vide), ce qui
règle les quatre appelants d'un coup.

### 3.7 Le cache de schéma n'est pas invalidé à la suppression d'une table

`service/metadata.go:76-82` (`RemoveTable`) invalide le cache de métadonnées mais pas celui de
`SchemaService`, indexé sur `tableId`. L'entrée survit jusqu'à son TTL (300 s par défaut). Sans
conséquence fonctionnelle — les `tableId` sont des UUID, donc pas de collision — mais c'est une
entrée retenue pour rien et une asymétrie avec l'invalidation faite par `AddColumn`.

### 3.8 `clusterByHilbert` laisse une fenêtre où la table n'existe pas

`duckdb/create.go:122-153` fait `CREATE TABLE tmp AS …`, `DROP TABLE t_x`, `ALTER TABLE tmp RENAME
TO t_x` en **trois `Exec` distincts**. Chacun prend et relâche `writeMu`, et les lectures ne
prennent jamais ce verrou. Entre le `DROP` et le `RENAME`, une requête concurrente sur ce `tableId`
(dont la ligne de métadonnées existe déjà, `Create` ayant été appelé en premier) échoue en 500.

La fenêtre est courte et suppose qu'un client interroge une table pendant son import, mais c'est
atteignable via l'UI, où l'import et un rafraîchissement de liste sont concurrents.

**Action.** Exécuter les trois instructions dans un seul `WithTx`.

### 3.9 `Migrate` ne peut pas rattraper une base « à jour mais sans `schema_version` »

`duckdb/catalog/migrations.go:30-70` : si la table `metadata` existe mais que `schema_version` est
absente, la version est forcée à 0 et toutes les migrations sont rejouées. Pour une base créée par
une version du code postérieure aux colonnes `sourceName`/`sourceLayer` mais antérieure à
`schema_version`, le premier `ALTER TABLE metadata ADD COLUMN sourceName` échoue et **le serveur
refuse de démarrer**.

**Action.** Rendre les migrations tolérantes (tester la présence de la colonne avant l'`ALTER`, ou
ignorer « column already exists »), ou déduire la version initiale de la présence des colonnes
plutôt que de la seule existence de la table.

---

## 4. Performance

### 4.1 `RefreshDerivedMetadata` : recalcul complet après chaque écriture d'une ligne

C'est le point chaud principal. `duckdb/catalog/metadata.go:273-320` fait, à chaque appel :

1. un `SELECT` de la ligne de métadonnées ;
2. un `SELECT COUNT(*)` sur **toute** la table ;
3. pour une table géo, un `MIN/MAX` sur `ST_Transform` de **toute** la colonne géométrique ;
4. un `UPDATE` des métadonnées.

Et il est appelé après chaque `InsertRow`, `RemoveRows` et `BulkInsertRows` (`service/rows.go:274`,
`:291`, `:307`). Insérer une ligne dans une table d'un million de lignes déclenche donc une
agrégation reprojetée sur le million ; supprimer une ligne aussi.

DuckDB étant colonnaire, le `COUNT(*)` est bon marché ; le `ST_Transform` sur toute la colonne ne
l'est pas du tout.

**Actions, par ordre de rapport/effort :**

- Ne recalculer le bbox que si l'écriture touche la géométrie. `UpdateRow` fait déjà cette
  distinction (`service/rows.go:255`) ; `InsertRow`/`RemoveRows`/`BulkInsertRows` non. Un
  `RefreshDerived(ctx, tableId, touchesGeometry bool)` suffit : un insert scalaire n'aura plus qu'un
  `COUNT(*)` et un `UPDATE`.
- Supprimer le `GetMetadata` en tête : le service a déjà la `Metadata` résolue, il peut la passer.
- À terme, agréger le bbox en EPSG:3857 puis ne transformer que les 4 coordonnées du résultat, au
  lieu de reprojeter chaque géométrie avant d'agréger. C'est le vrai gain : de O(lignes)
  transformations à 1.

### 4.2 `SchemaService.Types` reconstruit une map par ligne

`service/rows.go:77-88` : `convertRow` appelle `s.schema.Types(ctx, meta)` — et `convertRows` (`:90`)
appelle `convertRow` **pour chaque ligne**. `SchemaService.Types` (`service/schema.go:67`) lit bien
le cache, puis **reconstruit intégralement la map nom → type à chaque appel**.

Pour une page de 100 lignes × 30 colonnes : 100 allocations de map et 3 000 insertions pour un
résultat identique 100 fois. Sur un export de 100 000 lignes par lots de 1 000, 100 000
reconstructions.

**Action.** Résoudre les types une fois dans `convertRows` et les passer à `convertRow` — deux
lignes de diff, gain proportionnel à la taille de page. Accessoirement, mettre la map elle-même en
cache dans `SchemaService`.

### 4.3 `ListRows` fait trois requêtes là où deux suffisent

`service/rows.go:105-135` : `Pkeys` (requête 1) → `ByPkey` (requête 2) → `Count` (requête 3). Le
détour par les pkeys applique tri et pagination deux fois, et la seconde requête refait une
recherche `IN (…)` sur 100 valeurs au lieu de lire des lignes déjà localisées.

**Action.** Une seule requête `SELECT * EXCLUDE(geom) FROM t ORDER BY … LIMIT ? OFFSET ?` donne le
même résultat. Garder `Pkeys` pour les appelants qui n'ont réellement besoin que des clés
(`FilterByBbox`). Le `Count` reste séparé ; `ListRowsPage` (export) a déjà raison de ne pas le
faire.

### 4.4 Points mineurs

- `duckdb/adapter.go:46` — `db.SetMaxOpenConns(4)` est en dur. C'est le paramètre de tuning le plus
  susceptible d'être ajusté par un exploitant : il doit vivre dans `config.DuckDBConfig`.
- `handlers/geo.go:115` — `max-age=60` en dur, même remarque.
- `data/geo.go:275` — quand `Attrs` est vide, la projection est
  `SELECT __pkey, * EXCLUDE (geom), …` : `__pkey` est sélectionné **deux fois** et transite en double
  sur chaque feature. `ScanFeatureRows` l'absorbe sans dommage, mais c'est de la bande passante
  DuckDB → Go gratuite.
- `handlers/features.go:218-229` — `featuresPaginationParams` n'applique **aucun plafond** au `limit`
  client, contrairement à `RowsHandler.paginationParams`. `?limit=10000000` sur
  `/ogc/collections/{id}/items` matérialise autant de features en mémoire. Le commentaire justifie
  le choix par « c'est le bbox qui doit borner », mais le bbox est optionnel. Même chose pour
  `GetTileFeatures` (`handlers/geo.go:185-190`). Un `maxFeaturesLimit` en config s'impose.

---

## 5. Ce qui est solide

À conserver tel quel :

- **La discipline SQL.** Tout identifiant interpolé passe par `ValidateIdentifier` ou
  `QuoteIdentifier`, toute valeur est liée. Vérifié sur les chemins générés dynamiquement :
  `ORDER BY`, `SET`, `EXCLUDE`, filtres, projections d'attributs, noms d'attributs MVT.
- **La validation de `/derive` par l'AST DuckDB** (`duckdb/derive.go`) plutôt que par blacklist de
  mots-clés, avec collecte des noms de CTE et rejet de toute `TABLE_FUNCTION`. Nettement au-dessus
  de la moyenne du genre.
- **Le pattern « interface déclarée côté consommateur »**, appliqué systématiquement
  (`handlers/ports.go`, `service/derive.go`, `validation/`, `core/core.go`). Chaque dépendance est
  visible au point d'appel.
- **La séparation contrats / implémentation** apportée par les refactorings : `storage/catalog` et
  `storage/data` sont dépourvus de toute référence à DuckDB, `storage/sort` est délibérément sans
  dépendance pour casser le cycle. Un adaptateur PostGIS est réellement branchable.
- **Les suites de conformité** (`storagetest/`) exécutées contre l'adaptateur DuckDB.
- **La gestion du timeout d'export** — sortie du `Group` chi *et* effacement du `WriteDeadline` — et
  le `maxBodyBytes` qui rejette sur `Content-Length` avant de lire le corps.
- **La couverture de tests**, homogène : chaque handler, service et repository a son fichier.

---

## 6. Nettoyage des commentaires

Chantier le plus volumineux : **194 références à un référentiel externe, dans 53 fichiers.**

### 6.1 Ce que le code contient

| Forme | Occurrences | Statut |
|---|---|---|
| `règle N` | ~150 | Défini en `spec.md` §0 |
| `§4.2`, `§5.1`, `§4.1`… | ~22 | Défini en `spec.md` (sections) |
| `PERF-N`, `GEO-N`, `HTTP-2`, `NEW-SEC-1`, `NEW-BUG-N` | ~40 | Défini en `spec.md` §11 (glossaire) |
| `audit §3.12`, `audit §3.10`, `audit §1.2`, `docs/audit.md §5` | 12 | **Pendantes** — `docs/audit.md` n'existe pas |
| `§0.2`, `§1.3`, `§2.2` | 3 | **Pendantes** — ces sections n'existent pas |

### 6.2 Le problème

Ces références étaient un pont vers des documents d'audit successifs. Le pont n'a survécu que par un
glossaire (`spec.md` §11) qui traduit des codes en phrases — ce qui prouve que les codes n'apportent
rien que la phrase n'apporterait directement.

Concrètement :

```go
// ListRows returns a page of rows ordered by sort, or deterministically by
// pkey when empty (règle 8), with values converted to native JSON types
// (règle 12) and the geometry column always excluded (règle 7).
```

Le lecteur — humain ou agent — doit ouvrir `spec.md`, chercher §0, compter jusqu'à trois règles
distinctes, pour n'apprendre exactement que ce que la phrase disait déjà. La référence est du bruit
pur, et elle vieillit mal : 15 occurrences pointent déjà vers un fichier ou une section supprimés.

Plusieurs commentaires racontent en outre **l'histoire** plutôt que l'état : `create.go:84`
(« vérifié contre un point connu (Paris) quand ceci a été corrigé pour la première fois »),
`import.go:77`, `metadata.go:114`, `source.go:386`, `routes.go:79` et `:152`, `sources.go:113`
(« ne fait pas partie de la table d'endpoints d'origine de `spec.md` »). Le `CLAUDE.md` du dépôt
l'interdit explicitement : *« Instructions describe … the state of the related code. It does not
contain information about the past or old choices. »*

### 6.3 Action

1. **Supprimer les 15 références pendantes** (`audit §…`, `docs/audit.md`, `§0.2`, `§1.3`, `§2.2`) —
   sans discussion, elles ne pointent nulle part.
2. **Supprimer les tags d'audit** (`NEW-BUG-N`, `NEW-SEC-1`, `PERF-N`, `GEO-N`, `HTTP-2`) en gardant
   la phrase qui les entoure : elle est autoportante partout où j'ai vérifié. Supprimer ensuite le
   glossaire `spec.md` §11, devenu sans objet.
3. **Supprimer les `règle N` et les `§X.Y`**, en conservant l'énoncé. Les invariants qu'ils
   désignent restent vrais : ils restent documentés dans `spec.md` §0 comme référence, et dans le
   code comme phrase lisible sans détour. Un commentaire doit se suffire à lui-même.
4. **Réécrire au présent les commentaires historiques** listés en §6.2.
5. **Purger les commentaires qui mentent** une fois §2 fait — `duckdb/mvt.go:11-16` prétend rejeter
   une table scalaire par un `NotFoundError` alors que le code fait `geo, _ := repo.Geo()` et
   renvoie une interface nulle avec une erreur nulle (déréférencement nul garanti s'il était
   appelé). Ce fichier étant mort (§2.1), sa suppression règle le cas.

Le tout est mécanisable en une passe, sans impact fonctionnel : `mise run test` doit rester vert à
l'octet près.

---

## 7. Dérive documentaire de `spec.md`

`spec.md` §12 (« Écarts connus entre ce document et le code ») est lui-même périmé sur la moitié de
ses entrées :

| Entrée §12 | Réalité aujourd'hui |
|---|---|
| « Migrations : la liste est vide » | **Faux** — `migrations.go` en contient 7 |
| « `ColumnType` : un `DESCRIBE` par colonne » | **Faux** — `Types` fait un seul `DESCRIBE` |
| « `GetDistinctValues`, `FilterByBbox` : sans plafond » | **Faux** — plafonnés par les handlers |
| « `numberMatched` renvoie le compte de la page » | Toujours vrai (§3.3) |
| « bbox de `POST /bbox` en EPSG:3857 » | Toujours vrai (§3.1) |
| « `TableExists` sans appelant » | Toujours vrai (§2.3) |

Par ailleurs `spec.md` §4.4 décrit encore l'agrégat `Storage` (`TableManager` + `GeoTableManager` +
`TableCreator`) comme le contrat vivant, alors que les services consomment `data.TableResolver`.
`spec.md` doit être repassé après §2.

---

## 8. Recommandations, par ordre de valeur

| # | Action | § | Effort | Risque |
|---|---|---|---|---|
| 1 | Supprimer l'ancienne surface `TableManager`/`GeoTableManager` et ses implémentations | 2.1 | M | Faible — code mort |
| 2 | Rendre la suppression de table transactionnelle (brancher ou retirer `Catalog.DeleteTable`) | 2.2 | S | Faible |
| 3 | `RefreshDerived` : ne recalculer le bbox que si la géométrie a bougé | 4.1 | S | Faible |
| 4 | Résoudre les types une fois par page dans `convertRows` | 4.2 | XS | Nul |
| 5 | Corriger le CRS de `PkeysInBbox` | 3.1 | XS | **Rupture d'API** — à annoncer |
| 6 | Passer les handlers restants à `MapServiceError` | 3.4 | S | Faible |
| 7 | Nettoyer les 194 références de commentaires | 6 | M | Nul |
| 8 | Hacher l'ETag ; `ORDER BY __pkey` sur `Filtered` ; garde pkeys vide dans `PkeyInClause` | 3.2/3.5/3.6 | S | Faible |
| 9 | Plafonner `limit` sur les endpoints features ; `maxOpenConns` en config | 4.4 | S | Faible |
| 10 | `ListRows` en une requête | 4.3 | M | Moyen — chemin très testé |
| 11 | Rendre `Migrate` tolérant ; transaction sur `clusterByHilbert` | 3.9/3.8 | S | Faible |
| 12 | Repasser `spec.md` §4.4, §11, §12 | 7 | S | Nul |

Les points 1 à 4 se tiennent ensemble : ils suppriment la moitié de la surface de code de la couche
storage et les deux points chauds de performance, sans changer un seul comportement observable.
