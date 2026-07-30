# Audit de la partie Go — GeoAPI

Audit du code Go du dépôt à l'état actuel de la branche `develop`. Il décrit ce que le code fait
aujourd'hui, ce qui est solide, et ce qui pose problème. Chaque constat renvoie à un fichier et une
ligne. Il ne propose pas de refonte globale : l'architecture en place est saine, les constats sont
ciblés.

---

## 1. Synthèse

**Verdict : base saine, bien testée, avec une dérive documentaire et quelques défauts de
correction ciblés.**

L'architecture à trois couches (`router` → `service` → `storage`) est respectée sans fuite : aucun
handler ne touche SQL, aucun service ne construit d'identifiant SQL. Le pattern « interface déclarée
côté consommateur » (`handlers/ports.go`, `service/derive.go`, `core/core.go`) est appliqué
systématiquement, ce qui rend chaque dépendance visible au point d'appel et chaque couche testable
isolément. La discipline de sécurité SQL (`ValidateIdentifier` / `QuoteIdentifier` / paramètres liés)
est tenue partout où j'ai vérifié, y compris sur les chemins générés dynamiquement (ORDER BY, SET,
EXCLUDE, filtres). La validation de `/derive` par l'AST DuckDB (`json_serialize_sql`) plutôt que par
blacklist de mots-clés est nettement au-dessus de la moyenne du genre.

Les problèmes sont de trois ordres :

1. **Un N+1 systématique sur le schéma** (`ColumnType` fait un `DESCRIBE` complet par colonne), en
   contradiction directe avec le commentaire qui prétend l'inverse.
2. **Une classification d'erreurs incomplète** : plusieurs erreurs client remontent en 500, et une
   panne base remonte en 404.
3. **Une couche « catalogue » (`metadata`) diluée dans l'adapter DuckDB** — sans repository, sans
   migration réellement utilisable, avec pagination et jointures faites en Go. C'est ce point qui
   justifie le plan séparé (`docs/plan-repository-metadata.md`).

### Métriques

| Indicateur | Valeur |
|---|---|
| Fichiers Go (hors `docs/`, `node_modules`) | 87 |
| Lignes Go (idem) | ~13 300 |
| `go vet` | ✅ propre |
| `gofmt -l` | ✅ propre |
| `go test ./...` | ✅ tous les paquets passent |
| Couverture `router` | 98,8 % |
| Couverture `router/validation` | 100 % |
| Couverture `router/handlers` | 91,4 % |
| Couverture `config` | 93,3 % |
| Couverture `core` | 87,1 % |
| Couverture `storage/duckdb` | 80,2 % |
| Couverture `service` | 77,4 % |
| Couverture `storage` | 74,1 % |
| Couverture `router/response` | 32,0 % |

---

## 2. Ce qui est solide

- **Sécurité SQL.** Deux primitives distinctes avec des rôles clairement séparés et documentés
  (`internal/storage/identifier.go`) : `ValidateIdentifier` pour les identifiants générés côté
  serveur, `QuoteIdentifier` pour tout nom de colonne pouvant venir d'un fichier importé. Les deux
  sont appelées avant *chaque* interpolation. Aucune valeur n'est interpolée : `BuildFilter`
  (`storage/filter.go`) whiteliste colonne et opérateur, et lie systématiquement la valeur.
- **Validation de `/derive` par le parseur DuckDB.** `validateDeriveQuery`
  (`storage/duckdb/derive.go:44`) sérialise la requête en AST et la parcourt pour rejeter toute
  `TABLE_FUNCTION` et toute `BASE_TABLE` non enregistrée dans `metadata`. C'est la bonne approche :
  un `read_csv`, un `read_text` ou un `duckdb_settings` ne peut pas passer, là où une regex aurait
  échoué. Le regex `selectOnlyRe` (`service/derive.go:12`) n'est qu'un fail-fast, et le code le dit.
- **Séparation ports/adapters cohérente.** `handlers/ports.go` déclare six interfaces étroites avec
  des assertions de conformité en bas de fichier ; `core/core.go` fait de même pour l'adapter. Les
  handlers ne connaissent jamais un type de service concret — sauf `ExportHandler` (voir 4.6).
- **Suite de conformité storage.** `storage/storagetest/suite.go` est une batterie de tests
  comportementaux exécutable contre n'importe quel adapter, branchée sur DuckDB via
  `duckdb/conformance_test.go`. C'est exactement ce qu'il faut pour qu'un futur adapter PostGIS
  prouve qu'il tient le même contrat.
- **Gestion du cycle de vie.** `cmd/server/main.go` gère `SIGINT`/`SIGTERM`, drain gracieux borné,
  et remonte les erreurs de `ListenAndServe`. `core.Setup` ferme la connexion sur chaque chemin
  d'échec.
- **Export en flux borné.** `ExportService.forEachPage` (`service/export.go:104`) pagine, et
  `ExportHandler` neutralise explicitement `WriteTimeout` avant de streamer
  (`handlers/export.go:72`), avec la route sortie du groupe `middleware.Timeout`
  (`router/routes.go:153`). Ce couple de subtilités est correctement traité et commenté.
- **Cohérence CRS.** La géométrie est stockée en EPSG:3857, reprojetée une fois à l'import, et
  reconvertie en `OGC:CRS84` (et non EPSG:4326, dont l'ordre d'axes officiel est lat/lon) sur chaque
  sortie GeoJSON et sur `bbox`. Le piège est identifié et commenté aux quatre endroits concernés.

---

## 3. Défauts de correction

### 3.1 `ColumnType` : un `DESCRIBE` complet par colonne (N+1)

`SchemaService.Schema` (`internal/service/schema.go:41-58`) appelle `storage.Columns` puis
`storage.ColumnType` pour **chaque** colonne. Or `Adapter.ColumnType`
(`internal/storage/duckdb/table.go:68`) exécute un `DESCRIBE` complet et scanne le résultat jusqu'à
trouver la colonne. Une table à 40 colonnes coûte donc **41 requêtes `DESCRIBE`** par miss de cache.

Le commentaire de `SchemaService.Schema` affirme le contraire : « one DESCRIBE-driven population,
not one per column, per cache miss ». La documentation décrit une intention non implémentée.

C'est le défaut de performance le plus net du code : il frappe chaque expiration de cache schéma,
sur un chemin emprunté par `/schema`, `/rows` (conversion de types), `/filter`, `/export`, et par
`ValidateColumns` sur chaque tuile avec `attrs`.

**Correctif :** exposer `Adapter.describeColumns` (déjà présent, `duckdb/create.go:18`) via une
méthode `ColumnsWithTypes(ctx, meta) ([]ColumnSchema, error)` et faire de `Schema` un seul appel.
`ColumnType` peut rester pour compatibilité mais ne devrait plus être sur le chemin chaud.

### 3.2 Une panne base renvoie 404

`TableResolver.ResolveTable` (`internal/router/validation/resolver.go:41`) mappe **toute** erreur de
`GetMetadata` en 404. `Adapter.GetMetadata` (`duckdb/adapter.go:207`) ne distingue `sql.ErrNoRows`
que dans le message d'erreur, pas dans un type. Conséquence : une connexion DuckDB tombée, un
timeout de contexte, ou une colonne manquante après migration ratée sont tous rapportés au client
comme « table inexistante », et ne sont **pas loggués** (`WriteError` ne loggue qu'à partir de 500).

**Correctif :** introduire un `storage.NotFoundError` retourné par `GetMetadata` sur `ErrNoRows`
uniquement, et faire de `ResolveTable` un `response.MapServiceError(err)` comme partout ailleurs.

### 3.3 Erreurs client classées en 500

Plusieurs chemins renvoient une erreur DuckDB brute au mapping par défaut (500) alors que la faute
est celle de l'appelant :

- `RowsHandler.AddRows` / `UpdateRow` : un nom de colonne inexistant ou un type incompatible
  produit une erreur `Exec` → `MapServiceError` → 500. Aucune validation de colonnes n'est faite
  côté handler, contrairement à `ListRows` qui, lui, appelle `ValidateColumns` sur le `sort`.
- `TablesHandler.AddColumn` (`handlers/tables.go:221`) : le 400 est en dur, ce qui est le bon
  choix ici — mais l'incohérence avec `AddRows` est arbitraire.
- `WKT` invalide sur la colonne géométrique : `Adapter.Update`/`InsertRow` enveloppent l'erreur dans
  un message mixte français/anglais (`"update failed (invalid WKT si colonne géométrique)"`,
  `duckdb/table.go:332`) sans type dédié — donc 500, et le message n'atteint jamais le client.

**Correctif :** valider les colonnes d'un insert/patch contre `ValidateColumns` (le validateur est
déjà embarqué dans `RowsHandler`, il n'est simplement pas utilisé sur ces deux routes), et typer
l'erreur WKT.

### 3.4 `SourceService.Delete` : mutation partiellement hors verrou

`internal/service/source.go:387-429`. `found := s.sources[index]` copie la struct, puis le verrou est
relâché. Ensuite :

- `found.Loaded = false` / `found.TableId = ""` (lignes 407-408) écrivent dans **la copie** — sans
  effet, code mort.
- `found.Layers[i].Loaded = false` (lignes 415-416) écrivent à travers le **slice partagé** avec
  l'entrée vivante du cache, **sans détenir `mu`**.

Le second point est une écriture concurrente réelle : un `List` simultané fait `cloneSources` sous
verrou et lit `src.Layers[...]` pendant que `Delete` y écrit. `go test -race` ne l'attrape pas
faute de test concurrent sur ce chemin.

**Correctif :** ne rien muter hors verrou. Reprendre `mu` pour l'étape finale et faire la mise à
jour d'état au même endroit que la suppression de l'entrée (qui, elle, est déjà correcte).

### 3.5 `numberMatched` OGC est faux

`handlers/features.go:251` : `"numberMatched": len(features)`. Le standard OGC API - Features définit
`numberMatched` comme le nombre total d'entités correspondant à la requête **indépendamment de la
pagination**, et `numberReturned` comme le nombre renvoyé dans la page. Les deux sont ici la même
valeur. Un client OGC conforme conclura toujours qu'il tient la totalité du jeu.

**Correctif :** soit un `COUNT(*)` sur le même prédicat (coûteux mais correct).

### 3.6 `POST /tables/{id}/bbox` attend des coordonnées EPSG:3857

`Adapter.FilterByBbox` (`duckdb/mvt.go:103-106`) compare la géométrie (stockée en 3857) à un
`ST_MakeEnvelope` **non reprojeté**, alors que `GetFeatures` (`duckdb/features.go:303-305`)
transforme explicitement son bbox depuis `OGC:CRS84`. Deux endpoints bbox, deux CRS d'entrée
différents, aucun des deux documenté côté API. Un appelant qui passe du lon/lat à `/bbox` obtient
silencieusement zéro résultat.

**Correctif :** aligner sur `OGC:CRS84` (le CRS déjà utilisé par `Metadata.Bbox`, `filtered-extent`
et OGC items), et le documenter dans l'annotation Swagger.

### 3.7 ETag potentiellement malformé

`tileETag` (`handlers/geo.go:207`) formate `q.Args` avec `%v` à l'intérieur de guillemets. Les args
proviennent de valeurs de filtre fournies par le client (JSON). Une valeur contenant un `"` produit
un en-tête `ETag` syntaxiquement invalide ; un caractère de contrôle produit un en-tête rejeté par
`net/http`. Même remarque, sans impact protocolaire, pour la clé de cache `GeoService.GetTile`
(`service/geo.go:41`).

**Correctif :** hacher (`fnv`/`sha256` tronqué) la concaténation des composants au lieu de les
interpoler tels quels.

### 3.8 `rowCount` est un `INTEGER` DuckDB

`duckdb/adapter.go:33` déclare `rowCount INTEGER` (32 bits signés) là où `storage.Metadata.RowCount`
est un `int64` et où `RefreshDerivedMetadata` y écrit un `COUNT(*)`. Au-delà de 2,1 milliards de
lignes l'écriture échoue. Peu probable à court terme, mais c'est une incohérence de type gratuite —
`BIGINT` est le bon type.

### 3.9 Requêtes de lecture sans borne

- `GetDistinctValues` (`duckdb/table.go:264`) : `SELECT DISTINCT` sans `LIMIT`. Une colonne à forte
  cardinalité (un identifiant, une date) renvoie la colonne entière au client. Il faut paginer cela.
- `FilterByBbox` (`duckdb/mvt.go:102`) : renvoie **tous** les pkeys intersectants, sans `LIMIT`. Il faut paginer cela.
  Un bbox mondial sur une table à 5 M de lignes matérialise 5 M d'UUID en mémoire puis en JSON.

Les deux sont atteignables sans authentification. Ce sont les seuls chemins de lecture non bornés :
tout le reste passe par `maxRowLimit` / `maxListLimit` / `defaultFeaturesLimit`.

**Correctif :** Il faut paginer cela. mettre en place une pagination

### 3.10 `IN ()` sur liste vide

`GetRowsByPkey` et `RemoveRows` (`duckdb/table.go:196`, `:338`) construisent `IN (%s)` avec
`placeholders(0)` si la liste est vide → `IN ()`, erreur de syntaxe DuckDB. Les handlers HTTP
protègent aujourd'hui les deux chemins, mais les méthodes de service (`RowService.GetRowsByPkey`,
`RemoveRows`) sont exportées et ne garantissent rien. Un garde en tête de méthode storage
(retourner vide / no-op) supprime le piège pour tout appelant futur.

### 3.11 Double validation morte

`Adapter.InsertRow` (`duckdb/table.go:350-355`) valide `meta.PhysicalName` deux fois de suite.
Sans conséquence, mais c'est du bruit qui suggère une fusion de patchs mal résolue.

### 3.12 Les endpoints géo ne vérifient pas `IsGeo`

`GeoHandler` n'appelle que `ResolveTable`, jamais un équivalent du `resolveGeoTable` que
`FeaturesHandler` (`handlers/features.go:206`) applique correctement. Les quatre routes
`/tiles/{z}/{x}/{y}`, `/tiles/{z}/{x}/{y}/features`, `/geometry-types`, `/bbox` et
`/filtered-extent` acceptent donc un `tableId` de table non géo. Le `geometryColumn` vide descend
jusqu'à `QuoteIdentifier("")`, qui rejette sur « empty identifier » → **500**, là où la réponse
juste est 404 (la collection géo n'existe pas) ou 400.

Le contrôle existe et est correct pour OGC et pour l'export (`handlers/export.go:54`) ; il manque
exactement là où la géométrie est la seule raison d'être de l'endpoint.

**Correctif :** à court terme, un `resolveGeoTable` partagé par `GeoHandler` et `FeaturesHandler`.
À terme, c'est le genre d'invariant qui doit être structurel plutôt que répété par handler — voir
`docs/plan-repository-data.md`.

---

## 4. Défauts structurels et de conception

### 4.1 Le catalogue `metadata` n'a pas de repository

C'est le point le plus structurant, et il fait l'objet d'un plan dédié :
**`docs/plan-repository-metadata.md`**. En résumé :

- Le DDL du catalogue (`metadataSchema`) est une constante dans `duckdb/adapter.go`, mélangée à
  l'ouverture de connexion et aux méthodes de données.
- Les écritures passent par `UpdateMetadata(map[string]any)` avec une allowlist de chaînes
  reconstruite à chaque appel (`adapter.go:259-262`) — non typé, non vérifié à la compilation.
- La liste des colonnes est répétée quatre fois (DDL, `SELECT` de `GetMetadata`, allowlist
  d'`UpdateMetadata`, `metadataSortColumns`). Ajouter un champ, c'est modifier quatre endroits sans
  filet.
- La pagination de `/tables` est faite **en Go** : `ListTables` (`service/metadata.go:48`) charge
  *tous* les `tableId` puis découpe la tranche, avec un `GetMetadata` par élément.
- `LoadedSourceNames` (`service/metadata.go:133`) fait de même : un `SELECT` global + un
  `GetMetadata` par table, à chaque `POST /sources/refresh`.
- Le framework de migration existe (`duckdb/migrations.go`) mais `schemaMigrations` est **vide**,
  alors que le schéma a déjà évolué : `sourceName`/`sourceLayer` ont été ajoutés hors migration, ce
  que le commentaire d'`adapter.go:19-23` assume explicitement comme « limitation connue ». Toute
  base créée avant ces colonnes est cassée.

### 4.2 `TableCreator` mélange création et introspection de fichiers

`storage/aggregate.go:9` : l'interface porte `CreateFromFile`/`CreateFromQuery` (création) mais
aussi `ListLayers`/`DetectCRS` (introspection d'un fichier source, sans rien créer). Un futur adapter
devra implémenter les quatre même s'il ne lit pas de fichiers. Une interface `SourceInspector`
séparée serait plus juste.

### 4.3 `TableExists` est du code mort en production

`storage/metadata.go:47`. Implémentée, testée, exposée dans l'interface publique et dans les mocks —
aucun appelant hors tests. Soit un service en a besoin (`ResolveTable` pourrait l'utiliser), soit
elle sort du contrat.

### 4.4 `describeColumns` duplique `Columns` / `ColumnType`

`duckdb/create.go:18` et `duckdb/table.go:36`/`:68` font le même `DESCRIBE` avec le même scan
générique, en trois implémentations. Un seul helper interne suffirait — et résoudrait au passage
le N+1 de 3.1.

### 4.5 Le routeur dépend du type concret `*core.Services`

`router.NewRouter(svcs *core.Services, ...)` (`router/routes.go:39`). Le paquet `router` prend une
dépendance sur `core` alors que chaque handler prend déjà des ports étroits. Passer les huit
services en paramètres nommés, ou une interface `Services` déclarée côté `router`, restaurerait la
symétrie du pattern appliqué partout ailleurs.

### 4.6 `ExportHandler` prend le type concret

`handlers/export.go:19` : `exportService *service.ExportService`, alors que `ports.go` définit un
port pour les cinq autres services. Incohérence isolée ; un `exportAPI` de deux méthodes suffit.

### 4.7 Cache schéma non invalidé à la suppression de table

`MetadataService.RemoveTable` invalide le cache métadonnées mais pas le cache schéma de
`SchemaService` (qui est un autre paquet, avec son propre LRU). Les entrées orphelines expirent
seules au TTL. Sans impact fonctionnel (le `tableId` est un UUID neuf à chaque création), mais c'est
une fuite mémoire bornée et une entorse à la règle 11.

### 4.8 Aucune authentification, autorisation, CORS ni rate limiting

Le serveur expose en écriture (`POST /rows`, `DELETE /tables/{id}`, `POST /derive`,
`POST /sources/upload`) sans aucun contrôle d'accès. C'est cohérent avec le périmètre déclaré
(règle 16) et ce n'est donc pas un bug — mais ce n'est écrit nulle part comme une décision assumée.
Il faut que le spec le dise explicitement, faute de quoi le déploiement se fera par défaut.

### 4.9 `web/node_modules` contient un fichier Go dans le module

`web/node_modules/flatted/golang/pkg/flatted/flatted.go` est compilé par `go vet ./...`,
`go build ./...` et `go test ./...`. `node_modules` est gitignoré, donc la CI sur checkout frais ne
le voit pas — mais en local, tout paquet Go déposé par npm entre dans le module. Un
`web/node_modules/.../go.mod` factice, ou un `//go:build ignore`, ou simplement un `go.work`
excluant `web/`, ferme le sujet.

### 4.10 Les tables de données n'ont pas non plus de repository

Symétrique de 4.1, côté données cette fois. Chaque méthode de `table.go`, `mvt.go` et `features.go`
reçoit une `storage.Metadata` et recommence le même préambule : valider `physicalName`, guillemeter
`geometryColumn`, assembler la requête par `fmt.Sprintf`. Le décompte est net :

| Motif | Occurrences |
|---|---|
| `ValidateIdentifier(meta.PhysicalName)` | 23 |
| `QuoteIdentifier(meta.GeometryColumn)` | 10 |
| Projection `EXCLUDE (<geom>)` réécrite | 4 |
| Fonctions de scan génériques quasi identiques (`scanRows`, `scanFeatureRows`) | 2 |
| Blocs de sélection d'attributs identiques (`features.go:30` et `:115`) | 2 |

Conséquences concrètes, toutes déjà relevées ailleurs dans cet audit : la validation d'identifiant
est une **convention répétée 23 fois** plutôt qu'un invariant (règle 1 tenue par discipline, pas par
construction) ; rien n'empêche d'appeler une opération géo sur une table non géo (3.12) ; le N+1
schéma (3.1) vient du fait qu'aucun objet ne porte le schéma d'une table ; les gardes de liste vide
(3.10) et de plafond (3.9) manquent parce qu'il n'y a pas de point de passage unique où les mettre.

Traité par **`docs/plan-repository-data.md`**, conçu pour se dérouler en parallèle de 4.1.

---

## 5. Dérive documentaire

`docs/spec.md` est déclaré source de vérité par son propre préambule (« Toute décision présentée ici
est définitive ») et par la règle 0 du fichier. Il ne l'est plus. Écarts relevés, au-delà de la
refonte prévue :

| Spec | Code |
|---|---|
| §4.1 `metadata` : 10 colonnes | 12 colonnes (`sourceName`, `sourceLayer`) |
| §4.1 index `WHERE isGeo = TRUE` | index sans clause `WHERE` |
| §4.4 signatures sans `ctx`, sur `tableId` | toutes les méthodes prennent `ctx` puis `Metadata` |
| §4.4 `Remove(tableId, pkeys)` | `RemoveRows` (renommée, la spec note le conflit ailleurs) |
| §4.4 `Tile` sans `GeoQuery` | `Tile(..., attrs, q GeoQuery)` |
| §4.4 pas de `GetFeaturesTile` | existe, avec son endpoint |
| §4.4 `GetFeatures(..., limit)` | `(..., limit, offset)` |
| §6.1 : pas de `/upload`, `/metadata`, `DELETE /sources/{name}` | les trois existent |
| §6.1 : pas de `/filtered-extent`, `/tiles/.../features` | les deux existent |
| §6.1 : pas de `/healthz`, `/readyz` | les deux existent |
| §6.2 `middleware.Timeout` global | scopé au groupe `/api/v1`, export exclu |
| §9 arborescence | `internal/core`, `internal/router/handlers`, `.../response`, `.../validation`, `internal/webui` absents |
| §11 « plan d'implémentation » | entièrement réalisé — section obsolète par nature |
| §4.1 « `CREATE TABLE IF NOT EXISTS` est le seul mécanisme de migration nécessaire » | `migrations.go` existe et contredit la phrase |
| §10 exemples de tests | signatures périmées (pas de `ctx`, `cache.New`, `*Metadata`) |

Par ailleurs, le code référence par tag des documents absents du dépôt : `docs/audit.md` (GEO-1,
GEO-2) et `docs/audit2.md` (§4.5, NEW-BUG-*, NEW-SEC-1, PERF-*, HTTP-2). Ces tags apparaissent dans
**~90 commentaires** ; un lecteur (humain ou LLM) ne peut pas les résoudre. Le nouveau `spec.md`
doit fournir un glossaire de ces tags, sans quoi les commentaires du code sont des références
mortes.

---

## 6. Tests

**Points forts.** Suite de conformité storage réutilisable ; mocks testify générés à la main mais
complets ; tests HTTP par handler via `httptest` ; tests d'intégration DuckDB réels (pas de mock du
moteur) ; `mise run test-race` disponible et la CI l'exécute.

**Manques identifiés.**

- **`router/response` à 32 %.** `MapServiceError` est le point de décision central pour le code de
  statut de toute l'API, et il n'a pas de test exhaustif par type d'erreur. C'est le trou de
  couverture le plus coûteux du dépôt, rapporté à ce que la fonction décide.
- **Aucun test de concurrence sur `SourceService`.** Le verrou `mu` porte des invariants explicites
  (le commentaire de la struct décrit le scénario de double `Load`), mais aucun test ne les
  exerce — c'est pour cela que 3.4 passe inaperçu.
- **Aucun test de migration réelle.** `migrations_test.go` teste le mécanisme sur une liste vide.
  Rien ne vérifie qu'une base d'avant `sourceName` s'ouvre — et elle ne s'ouvre pas.
- **Pas de test d'intégration bout-en-bout** (import fichier → tuile → export) sur un serveur monté
  complet. `router_test.go` teste le câblage, pas un scénario.

---

## 7. Recommandations, par ordre de valeur

| # | Action | Effort | Réf. |
|---|---|---|---|
| 1 | Supprimer le N+1 `ColumnType` (une seule requête `DESCRIBE` par schéma) | S | 3.1 |
| 2 | Typer `NotFoundError` côté storage, router en `MapServiceError` | S | 3.2 |
| 3 | Corriger la mutation hors verrou de `SourceService.Delete` + test concurrent | S | 3.4, 6 |
| 4 | Borner `GetDistinctValues` et `FilterByBbox` par config | S | 3.9 |
| 5 | [DONE] Réécrire `docs/spec.md` sur le code réel, avec glossaire des tags | M | 5 |
| 6 | Introduire le repository catalogue + migrations réelles | M/L | 4.1, plan dédié |
| 7 | Valider les colonnes sur `AddRows`/`UpdateRow` (400 au lieu de 500) | S | 3.3 |
| 8 | Aligner le CRS de `POST /bbox` sur `OGC:CRS84` | S | 3.6 |
| 9 | Couvrir `MapServiceError` par type d'erreur | S | 6 |
| 10 | Hacher ETag et clé de cache tuile | S | 3.7 |
| 11 | `rowCount` en `BIGINT` (via une migration réelle) | S | 3.8, 4.1 |
| 12 | Gardes liste vide sur `GetRowsByPkey`/`RemoveRows` | S | 3.10 |
| 13 | Port `exportAPI`, découpler `router` de `*core.Services` | S | 4.5, 4.6 |
| 14 | Statuer explicitement sur auth/CORS/rate limiting dans le spec | S | 4.8 |
| 15 | Sortir `web/node_modules` du module Go | S | 4.9 |

Les points 1 à 4 sont des correctifs isolés, sans dépendance entre eux, réalisables indépendamment.
Le point 6 conditionne 11 et bénéficie de 2 ; il est décrit séparément.
