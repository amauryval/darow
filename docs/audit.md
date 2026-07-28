# Audit GeoAPI — limites actuelles et plan d'évolution

Audit réalisé le 2026-07-28 sur la branche `develop` (commit `daf70f1`).
Périmètre : `cmd/`, `internal/` (config, storage, service, router, webui), `web/`, outillage
(`mise.toml`, tests). Objectif : identifier ce qui bloque la **fiabilité**, la **séparation des
responsabilités** et surtout l'**évolutivité** de l'application, puis proposer un plan par étapes.

Quand une recommandation contredit `docs/spec.md` (qui fait autorité), c'est signalé explicitement
par **⚠ conflit spec** : la décision revient au mainteneur, l'audit ne tranche pas.

---

## 1. Synthèse

L'architecture en 3 couches (router → service → storage) est **saine dans son intention** : les
dépendances vont bien dans un seul sens, la config est injectée, aucun état global, le mock de
storage permet des tests de service/router sans base. Les points durs du domaine (validation
d'identifiants, filtres via DSL, sérialisation des écritures DuckDB, cache de tuiles invalidé par
`UpdatedAt`) sont traités et documentés.

Les limites sont ailleurs, et elles sont de trois natures :

1. **Étanchéité des couches incomplète** : le service reçoit une interface « dieu » `Storage` qui
   expose `Exec`/`Query` bruts, le router dépend de structs concrètes, `SourceService` appelle
   `TableService` sans interface, et `TableService` porte 5 responsabilités.
2. **Trous de correction/sécurité** : bypass du validateur `/derive` (lecture de fichiers locaux),
   colonnes accentuées qui cassent la lecture des rows, métadonnées orphelines en cas d'échec
   d'import, N+1 sur `ColumnType`, aucun timeout serveur.
3. **Fondations d'évolutivité manquantes** : pas de `context` dans le storage, pas de migrations du
   schéma `metadata`, pas de versionnement d'API, pas de CI, pas de healthcheck, SRID figé,
   formats de source câblés en dur.

État de l'outillage à l'instant de l'audit :

| Vérification | Résultat |
| --- | --- |
| `go test ./...` | **FAIL** — `Test_Router_GetTile` (Content-Type attendu `application/x-protobuf`, obtenu `application/vnd.mapbox-vector-tile`) |
| `gofmt -l .` | **1 fichier** non formaté — `internal/storage/duckdb/adapter.go` (ordre des imports) |
| `go vet ./...` | OK |
| Couverture | router 69,4 % · service 68,5 % · storage 100 % · duckdb 74,4 % · **web 0 %** |
| CI / Dockerfile | **absents** |

---

## 2. Séparation des responsabilités

### 2.1 Ce qui est bien posé

- Sens des dépendances respecté : `router → service → storage`, aucun import inverse.
- Config chargée une fois dans `main.go` et injectée par sous-struct (`config.APIConfig` au router,
  `config.SourcesConfig` au `SourceService`) — pas de `*config.Config` global baladé partout.
- `GeoService` ne dépend que de `storage.GeoTableManager` : c'est exactement le bon niveau de
  granularité, à généraliser.
- Le DTO HTTP (`internal/router/dto.go`) est distinct du modèle storage (`storage.Metadata`) : le
  contrat d'API ne fuit pas le modèle interne.
- `internal/webui` isole l'`embed` du build front ; le front passe exclusivement par `src/client/`.

### 2.2 SEP-1 — `storage.Storage` est une interface « dieu » (ISP)

`TableService` reçoit `storage.Storage`, agrégat de 5 interfaces (~30 méthodes) qui inclut
`SQLWrapper` : **la couche service a donc accès à `Exec()` et `Query()` en SQL brut**. Rien
n'empêche (aujourd'hui ou dans 6 mois, par un contributeur pressé) de court-circuiter tout le
travail de validation d'identifiants en écrivant du SQL directement dans un service. C'est un
contournement possible de la règle 1 par construction.

Effets collatéraux : le mock `mocks.Storage` doit implémenter 30 méthodes pour tester une méthode ;
`SQLWrapper` expose `*sql.Rows`/`*sql.Tx` de `database/sql` dans une interface censée abstraire le
moteur — un backend non-SQL (ou un pool géré différemment) ne peut pas l'implémenter.

**Recommandation** : définir les ports **côté consommateur** (paquet `service`), en interfaces
étroites (`MetadataStore`, `RowStore`, `SchemaStore`, `TableMaterializer`, `GeoStore`), l'adaptateur
DuckDB les satisfaisant implicitement. Sortir `SQLWrapper` de l'agrégat `Storage` : il reste un
détail interne du paquet `duckdb`.
**⚠ conflit spec** : `docs/spec.md` §4.4 définit `Storage` comme cet agrégat et §4.6 place `SQLWrapper` dans le contrat — l'amendement de la spec doit précéder le refactor. => ne pas se préocupper de spec.md pour 4.4 et 4.6 !

### 2.3 SEP-2 — `TableService` cumule 5 responsabilités

`internal/service/table.go` + `rows.go` + `schema.go` + `derive.go` = un seul type qui gère :
métadonnées + cache, CRUD de rows, schéma/colonnes, dérivation SQL, import de fichiers. Chaque
nouvelle capacité (export, jointures, statistiques…) grossira le même type, et tout consommateur
partiel (le `SourceService` n'a besoin que de l'import) dépend du tout.

**Recommandation** : découper en `MetadataService`, `RowService`, `SchemaService`,
`DeriveService`, `ImportService`, partageant un `metadataCache` injecté. Le router consomme les
services dont il a besoin, pas un bloc.

### 2.4 SEP-3 — Couplage service → service sans contrat

`SourceService` détient un `*TableService` concret et appelle `LoadedSourceNames()`,
`ListLayers()`, `ImportFromFile()`. Ces trois méthodes n'existent sur `TableService` que pour
`SourceService` : ce sont des dépendances de `SourceService` qui devraient être **son** interface
(`type tableImporter interface { ... }`), déclarée dans le même paquet et satisfaite par
l'`ImportService`.

### 2.5 SEP-4 — Le router dépend de structs concrètes

`router.Router` référence `*service.TableService`, `*service.GeoService`, `*service.SourceService`.
Tester un handler impose donc de construire de vrais services + un mock de storage complet
(cf. `setupRouter`). Des interfaces déclarées dans `internal/router` rendraient les tests de
handlers réellement unitaires et permettraient d'ajouter des décorateurs (métriques, autorisation,
cache) sans toucher aux handlers.

### 2.6 SEP-5 — Logique métier résiduelle dans le router

`handlers_export.go` construit le GeoJSON/CSV : il assemble features, appelle `geoService` **et**
`tableService`, décide du format par défaut. C'est de la logique d'export (domaine), pas du
transport. `updateRow` (handlers_rows.go:147) fait pire : il **boucle sur les colonnes** et appelle
`UpdateRow` une fois par colonne — l'atomicité d'un PATCH multi-colonnes est décidée dans le
handler, et un échec en milieu de boucle laisse la ligne à moitié modifiée.

**Recommandation** : `ExportService.Export(tableId, format) (io.Reader, contentType, error)` côté
service ; `UpdateRow(tableId, pkey, updates map[string]any)` en une seule opération côté service et
storage.

### 2.7 SEP-6 — Le formatage des valeurs est côté service, le formatage des dates côté router

`service.convertValue` transforme les `time.Time` en ISO 8601 (règle 12) mais `router.dto.go`
reformate `CreatedAt`/`UpdatedAt` avec son propre layout. Deux endroits décident de la
représentation JSON. À terme, tout doit se décider dans les DTO du router (le service renvoie des
types Go), ou tout dans le service — pas les deux.

### 2.8 SEP-7 — Frontend

Séparation `client/` (typé, seul à faire du `fetch`) vs `views/` respectée, aucune infraction
constatée. Limite : la logique d'orchestration (pagination, chargement, gestion d'erreur) est
dupliquée dans chaque vue — acceptable et conforme au CLAUDE.md aujourd'hui, à réévaluer si une
4ᵉ vue apparaît.

---

## 3. Correction et sécurité

### 3.1 SEC-1 (critique) — `/derive` permet de lire des fichiers locaux arbitraires

`service.validateSelectOnly` filtre par **liste noire de mots-clés**. Or les fonctions-table DuckDB
n'y figurent pas. Vérifié en exécutant le validateur :

```
SELECT * FROM read_text('/etc/hostname')   -> accepté (nil)
SELECT * FROM read_csv('/etc/passwd')      -> accepté (nil)
SELECT * FROM duckdb_settings()            -> accepté (nil)
SELECT name FROM metadata                  -> accepté (nil)
```

Chacune matérialise le résultat dans une table exposée ensuite par l'API : **lecture de n'importe
quel fichier lisible par le process**, plus lecture de la table `metadata` interne (et, si `httpfs`
était installé, SSRF sortante). Une liste noire ne peut pas être exhaustive.

Correctifs cumulatifs recommandés :
1. `SET enable_external_access = false` sur la connexion (bloque toutes les fonctions-table
   fichier/réseau) — attention : à activer **après** l'import `ST_Read`, ou en isolant les imports
   sur une connexion dédiée ;
2. valider par **liste blanche structurelle** plutôt que par regex : `EXPLAIN` la requête et
   inspecter les tables référencées, ou parser et exiger que les seules relations citées soient les
   `physicalName` connus ;
3. exclure `metadata` des relations autorisées.

Effet de bord actuel de la liste noire, dans l'autre sens (faux positifs) : `SELECT 'insert' AS x`
est rejeté — un littéral de chaîne contenant un mot-clé rend la requête impossible.

### 3.2 SEC-2 — Aucune authentification, aucune limite de taille de requête

Tous les endpoints d'écriture (`DELETE /tables/{id}`, `POST /rows`, `/derive`, `/sources/upload`)
sont ouverts. Aucun `http.MaxBytesReader` : un `POST /rows` avec un corps JSON de plusieurs Go est
décodé en mémoire (`json.NewDecoder(req.Body)` sans borne). L'upload est borné (`io.LimitReader` sur
`MaxSizeBytes`) — c'est le seul.

### 3.3 SEC-3 — Les erreurs internes fuient vers le client

`WriteError` renvoie `err.Error()` brut : messages DuckDB, chemins de fichiers, `physicalName`. Et
aucun log côté serveur pour les 500 — une erreur de production n'est donc visible que par le
client qui l'a déclenchée.

### 3.4 BUG-1 (élevé) — Toute table avec une colonne accentuée ou espacée casse `/rows`

`ValidateIdentifier` n'accepte que `^[a-zA-Z_][a-zA-Z0-9_]*$`. `service.convertRow` appelle
`ColumnType(tableId, col)` pour **chaque colonne**, qui valide le nom → erreur. Conséquence : un
GeoJSON ou CSV français avec une propriété `nom_région`, `Population 2020` ou `n°` s'importe
correctement (`SELECT *` ne valide rien) mais `GET /tables/{id}/rows` renvoie 500 systématiquement.
Le cas est très courant sur des données réelles.

**Recommandation** : passer du rejet au **quoting** — `"` + doublement des `"` internes + refus des
octets nuls, pour tout identifiant interpolé.
**⚠ conflit spec** : la règle 1 impose `validateIdentifier` ; c'est un durcissement compatible
(le quoting est strictement plus sûr que l'interpolation nue) mais la formulation de la règle doit
être mise à jour.

### 3.5 BUG-2 — Métadonnées orphelines en cas d'échec de création

`ImportFromFile` (table.go:179) et `DeriveTable` (derive.go:39) font `storage.Create(...)` (insertion
dans `metadata`) **puis** `CreateFromFile`/`CreateFromQuery`. Si la seconde échoue — fichier corrompu,
SQL invalide, ce qui est le cas nominal d'un utilisateur qui se trompe de requête — la ligne
`metadata` reste. La table apparaît dans `GET /tables` avec un `physicalName` qui n'existe pas, et
tout accès à ses rows échoue. Aucun nettoyage, aucune commande de réparation.

**Recommandation** : compensation (`defer` de suppression annulé au succès), ou création du physique
avant l'enregistrement logique.

### 3.6 BUG-3 — TOCTOU sur `SourceService.Load`

`Load` prend un `RLock`, récupère un `*Source` **pointant dans le slice**, relâche le verrou, puis
teste `found.Loaded` et importe. Deux `POST /sources/{name}/load` concurrents passent tous deux le
test et créent **deux tables** pour la même source. Idem entre un `Load` et un `Refresh`.
`List()` renvoie par ailleurs le slice interne sans copie : l'appelant peut muter le cache.

### 3.7 BUG-4 — Test rouge et fichier non formaté sur `develop`

`Test_Router_GetTile` échoue (le handler a été changé en
`application/vnd.mapbox-vector-tile`, le test attend `application/x-protobuf`) et
`internal/storage/duckdb/adapter.go` n'est pas `gofmt`. Sans CI, la branche peut rester rouge
indéfiniment.

### 3.8 BUG-5 — Collision possible sur la clé `pkey`

`router.rowToJSON` aplatit `Pkey` sous la clé `"pkey"` au même niveau que les colonnes : une table
possédant une colonne nommée `pkey` voit une des deux valeurs écrasée. Le front doit d'ailleurs
déjà compenser cet aplatissement (`cellValue()` dans `TableDetailView.vue`).

---

## 4. Performance et robustesse

### 4.1 PERF-1 (élevé) — N+1 massif sur `ColumnType`

`convertRow` appelle `s.storage.ColumnType(tableId, col)` **par cellule**. Chaque appel exécute un
`GetMetadata` (requête SQL, non caché au niveau storage) **plus** un `DESCRIBE` complet.
Pour une page de 100 lignes × 15 colonnes : ~3 000 requêtes SQL pour afficher un tableau.
`Schema()` a le même défaut (1 `DESCRIBE` par colonne).

**Recommandation** : récupérer le schéma une fois par appel (`map[string]string`), et le cacher au
niveau service avec la même invalidation que les métadonnées.

### 4.2 PERF-2 — Export non borné et non streamé

`exportTable` appelle `ListRows(tableId, 0, meta.RowCount)` : tous les pkeys, puis un
`WHERE __pkey IN (?, ?, … )` avec **autant de placeholders que de lignes**, tout en mémoire, plus
une seconde passe pour les géométries. Sur une table d'un million de lignes, c'est au mieux une
saturation mémoire, au pire un échec du driver. `MaxRowLimit` n'est pas appliqué ici.

**Recommandation** : curseur paginé + écriture en streaming dans la réponse, sans matérialiser.

### 4.3 PERF-3 — Un seul connecteur DuckDB : les lectures se bloquent entre elles

`SetMaxOpenConns(1)` sérialise **aussi les lectures**. Une requête de tuile lente bloque tout le
serveur, y compris `/tables`. Le verrou applicatif `writeMu` était fait pour les écritures ; la
limite à 1 connexion, elle, vise le chargement unique de l'extension spatiale — objectif
atteignable autrement (pool avec un hook de connexion qui exécute `LOAD spatial`).

### 4.4 PERF-4 — Pas de `context` dans le storage (sauf `Tile`)

Seul `Tile` propage un `context.Context`. Toutes les autres méthodes storage n'en prennent pas :
aucune annulation possible quand un client abandonne, aucun timeout par requête, et **aucun point
d'accroche pour du tracing/OpenTelemetry plus tard**. C'est la limite d'évolutivité la plus
structurante du storage : l'ajouter plus tard touchera ~30 signatures.

### 4.5 PERF-5 — Caches non bornés et non configurables

`go-cache` n'a pas de limite de taille : le cache de tuiles (des blobs de plusieurs centaines de Ko)
peut croître sans plafond jusqu'à l'expiration. Les TTL (`5*time.Minute`, `10*time.Minute`) sont
codés en dur dans `main.go` — **violation de la règle 18** (tout paramètre ajustable vit dans
`config`). Les caches sont par ailleurs in-process : deux instances derrière un load-balancer
divergent, et ni la scalabilité horizontale ni un cache disque de tuiles ne sont possibles en
l'état.

### 4.6 OPS-1 — Serveur sans timeouts ni arrêt gracieux

`http.ListenAndServe(addr, handler)` : pas de `ReadTimeout`, `WriteTimeout`, `IdleTimeout`
(exposition Slowloris), et un `SIGTERM` coupe les requêtes en vol **et** la fermeture propre de
DuckDB (`defer storageAdapter.Close()` n'est jamais atteint après un signal) — risque de checkpoint
non écrit.

### 4.7 OPS-2 — Observabilité absente

Pas d'endpoint `/healthz` / `/readyz` (impossible à orchestrer en conteneur), pas de métriques, pas
de log structuré ni d'identifiant de requête. `chi/middleware.Logger` écrit du texte non
parsable, `log.Fatalf` partout dans `main`.

### 4.8 HTTP-1 — `Content-Type` absent sur les réponses de succès

Seul `WriteError` positionne `application/json`. Les handlers de succès laissent Go sniffer le type
(souvent `text/plain; charset=utf-8`). Les erreurs de `json.Encode` sont par ailleurs ignorées
partout.

### 4.9 HTTP-2 — `Cache-Control` des tuiles incohérent avec le modèle d'invalidation

`max-age=3600` alors que le commentaire annonce « short max-age since a tile changes as soon as the
next write lands » : un client garde une tuile périmée jusqu'à une heure après une écriture. Un
`ETag` dérivé de `UpdatedAt` (déjà la clé du cache serveur) donnerait une revalidation correcte.

---

## 5. Domaine géospatial

### GEO-1 — Le SRID source est ignoré

`CreateFromFile` reprojette **toujours** `OGC:CRS84 → EPSG:3857`, quel que soit le CRS réel du
fichier. Un Shapefile en Lambert-93 (EPSG:2154), cas ultra-courant sur des données françaises, est
donc reprojeté depuis un CRS qu'il n'utilise pas : géométries silencieusement fausses. Le champ
`metadata.srid` reste à 4326 par défaut alors que le stockage est en 3857 : il ne décrit rien.

**Recommandation** : lire le CRS source via `ST_Read_Meta`, le stocker, et reprojeter depuis lui.

### GEO-2 — Une seule colonne géométrique, détectée par heuristique

`findGeometryColumn` renvoie la **première** colonne de type `GEOMETRY`. Une table multi-géométries
(fréquent en GeoPackage) n'est que partiellement exploitable.

### GEO-3 — Tuiles sans attributs ni généralisation

`ST_AsMVT` n'embarque que `pkey` : impossible de styler ou d'afficher une infobulle côté client sans
une requête par entité. Aucune simplification/`buffer`/`extent` paramétrable selon le zoom : à z=0
sur des polygones détaillés, la requête traite la géométrie pleine résolution.

---

## 6. Évolutivité (au sens « ajouter une fonctionnalité sans tout casser »)

| Ce qu'on voudra faire | Ce qui bloque aujourd'hui |
| --- | --- |
| Ajouter un backend PostGIS | `SQLWrapper` expose `*sql.Rows`/`*sql.Tx` ; `Storage` fait 30 méthodes ; pas de `context` ; aucune suite de tests de conformité réutilisable par adaptateur |
| Faire évoluer le schéma `metadata` | `CREATE TABLE IF NOT EXISTS` seul, **aucun mécanisme de migration ni de version de schéma** : une colonne ajoutée ne parvient jamais aux bases existantes |
| Supporter un nouveau format source | Le multi-couches est câblé en dur (`gpkgExt`, `if ext == gpkgExt` dans 3 branches). Il faut modifier `SourceService` à chaque format, au lieu d'enregistrer un descripteur de format |
| Faire évoluer le contrat d'API | Routes montées à la racine, **sans préfixe de version** ; cohabitation directe avec `/ui` et `/swagger`, et collision possible avec tout futur endpoint racine |
| Exposer le filtrage | `BuildFilter`, `ListRowsWithFilter`, `GetDistinctValues`, `TableExists` sont implémentés **et testés** mais **aucun endpoint ne les appelle** : du code mort qui donne l'illusion d'une capacité livrée |
| Renommer une table | `UpdateMetadata` existe côté service/storage mais **aucun `PATCH /tables/{id}`** ne l'expose |
| Supprimer/remplacer une source | Aucun endpoint : `Upload` refuse d'écraser, donc un fichier déposé est définitif sans accès au disque |
| Déployer | Ni `Dockerfile`, ni CI, ni healthcheck, ni arrêt gracieux, ni configuration par variables d'environnement (le commentaire de `config.yaml` l'annonce pourtant : « surchargeables par variables d'environnement » — **non implémenté**) |
| Faire évoluer le front en confiance | `web/` n'a **aucun test** ni linter ; seul `vue-tsc` protège |

Autres points de config : aucune validation des valeurs chargées (un `maxRowLimit: 0` fait passer
toutes les pages à `limit=0`), et le chemin `config.yaml` est codé en dur dans `main.go` (pas de
flag `--config`) alors que `--workdir` en a un.

---

## 7. Plan d'amélioration par étapes

Chaque étape est indépendamment livrable et laisse la branche verte. Les étapes 1 et 2 sont des
prérequis (on ne refactore pas sur une base rouge, on ne durcit pas après avoir livré).

### Étape 0 — Remettre la base au vert *(≈ 1 h)*

1. Corriger `Test_Router_GetTile` : trancher le `Content-Type` des tuiles
   (`application/vnd.mapbox-vector-tile` est le type officiel) et aligner le test.
2. `gofmt -w internal/storage/duckdb/adapter.go`.
3. Positionner `Content-Type: application/json` sur toutes les réponses de succès (helper
   `writeJSON(w, code, v)` qui gère aussi l'erreur d'encodage).

*Critère de sortie* : `mise run test` et `mise run lint` verts.

### Étape 1 — Filet de sécurité outillage *(≈ 2 h)*

1. CI GitHub Actions : `mise run lint`, `mise run test-race`, `vue-tsc --noEmit` sur chaque push/PR.
2. Ajouter `vitest` + `eslint` au front (même minimal : un test sur `client/http.ts`).
3. `Dockerfile` multi-stage (build UI → build Go cgo/duckdb → image finale) et `.dockerignore`.

*Critère de sortie* : une PR rouge est détectée avant merge.

### Étape 2 — Sécurité *(≈ 1 j)*

1. **SEC-1** : remplacer la liste noire de `/derive` par une validation structurelle (EXPLAIN +
   allowlist de relations = `physicalName` connus, `metadata` exclue) et couper l'accès externe
   DuckDB (`enable_external_access=false`), en isolant les imports `ST_Read` sur une connexion
   dédiée. Tests : les 4 requêtes de bypass listées en §3.1 doivent être rejetées, et
   `SELECT 'insert' AS x` accepté.
2. **BUG-1** : introduire `quoteIdentifier()` (quoting `"` + doublement) et l'utiliser partout où un
   identifiant est interpolé ; `ValidateIdentifier` reste pour les identifiants générés par le
   serveur. Test : import d'un GeoJSON avec `nom_région` → `GET /rows` renvoie 200.
   *(⚠ nécessite un amendement de la règle 1.)*
3. **SEC-2** : `http.MaxBytesReader` sur toutes les routes à corps, taille max en config.
4. **SEC-3** : mapper les erreurs internes vers un message générique + log serveur ; ne renvoyer le
   détail que pour les erreurs de validation (4xx).

### Étape 3 — Corriger les bugs de correction *(≈ 1 j)*

1. **BUG-2** : compensation transactionnelle sur `ImportFromFile` / `DeriveTable`.
2. **BUG-3** : verrou par nom de source dans `SourceService` (map de mutex ou lock global sur toute
   la séquence check+import) ; `List()` renvoie une copie.
3. **SEP-5** : `UpdateRow(tableId, pkey, updates)` en une opération atomique côté service+storage,
   le handler ne boucle plus.
4. **BUG-5** : décider du contrat (`{"pkey": ..., "values": {...}}` ou préfixe réservé) et aligner
   le front.

### Étape 4 — Séparation des responsabilités *(≈ 2–3 j)*

À faire **après** l'amendement de `docs/spec.md` §4.4/§4.6 (⚠ conflit spec).

1. Sortir `SQLWrapper` de l'agrégat `Storage` ; il devient interne au paquet `duckdb`.
2. Déclarer les ports côté consommateur : `service.MetadataStore`, `RowStore`, `SchemaStore`,
   `TableMaterializer`, `GeoStore` ; `router.TableAPI`, `GeoAPI`, `SourceAPI`.
3. Éclater `TableService` en `MetadataService` / `RowService` / `SchemaService` / `DeriveService` /
   `ImportService`, autour d'un `metadataCache` injecté.
4. `SourceService` dépend d'une interface `tableImporter` déclarée chez lui, plus de
   `*TableService`.
5. Extraire `ExportService` du router (CSV/GeoJSON = domaine).
6. Unifier le formatage des dates dans les DTO du router (**SEP-6**).
7. Remplacer les mocks manuels par des mocks générés par port (`mockery`/`moq`) — un mock de 5
   méthodes au lieu de 30.

*Critère de sortie* : aucun service ne peut appeler `Exec`/`Query` ; les tests de handlers ne
construisent plus de service réel.

### Étape 5 — Performance *(≈ 2 j)*

1. [FAIT] **PERF-1** : `SchemaService.Types(tableId) map[string]string` caché et invalidé avec les
   métadonnées ; `convertRow` ne fait plus d'I/O.
2. [FAIT] **PERF-4** : propager `context.Context` sur **toutes** les méthodes storage et service (à faire
   dans le même mouvement que l'étape 4 pour ne casser les signatures qu'une fois) ; timeout par
   requête via middleware.
3. [FAIT] **PERF-2** : export en streaming, borné et paginé.
4. **PERF-3** : [NE PAS FAIRE] pool de connexions DuckDB avec `LOAD spatial` au hook de connexion ; `writeMu`
   conservé pour les écritures uniquement. Valider sous `-race` et sous charge.
5. [FAIT] **PERF-5** : TTL et **taille max** des caches en config (règle 18) ; cache de tuiles borné (LRU).

### Étape 6 — Exploitation *(≈ 1 j)*

1. [FAIT] `http.Server` avec timeouts + `Shutdown(ctx)` sur `SIGINT`/`SIGTERM`, fermeture propre de DuckDB.
2. [FAIT] `/healthz` (process) et `/readyz` (ping DuckDB).
3. [FAIT] `log/slog` structuré + identifiant de requête (`middleware.RequestID`) ; supprimer les
   `log.Fatalf` hors bootstrap.
4. [NE PAS FAIRE] Métriques Prometheus (latence, tuiles servies, hit-ratio des caches).
5. [FAIT] **HTTP-2** : `ETag` sur les tuiles dérivé de `UpdatedAt`, `max-age` court.

### Étape 7 — Fondations d'évolutivité *(≈ 2–3 j)*

1. [PAS POUR LE MOMENT] **Migrations** du schéma `metadata` : table `schema_version` + migrations
   ordonnées appliquées au démarrage. Prérequis à toute évolution du modèle.
2. [FAIT] **Versionner l'API** : monter les routes sous `/api/v1`, garder des redirections depuis la
   racine le temps de migrer le front. Front migré (`web/src/client/http.ts`).
3. [FAIT] **Registre de formats source** : un descripteur par format (`extension`, `multiCouches`) ;
   `SourceService` ne connaît plus `.gpkg` en dur (voir `internal/service/formats.go`).
4. [FAIT] **Config** : override par variables d'environnement (`GEOAPI_*`), flag `--config`,
   validation des valeurs au chargement (rejet explicite plutôt que comportement silencieux).
5. [FAIT] **Suite de conformité storage** : une batterie de tests exécutable contre n'importe quel
   adaptateur (`internal/storage/storagetest`), condition d'entrée d'un futur `PostGISAdapter`.

### Étape 8 — Compléter le domaine *(≈ 2–3 j)*

1. **GEO-1** : détecter le CRS source (`ST_Read_Meta`), le stocker dans `metadata.srid`, reprojeter
   depuis lui. Test de non-régression sur un fichier Lambert-93.
2. Exposer les capacités déjà implémentées : `POST /tables/{id}/filter` (via `BuildFilter`),
   `GET /tables/{id}/columns/{col}/distinct`, `PATCH /tables/{id}` (renommage).
3. Gestion des sources : `DELETE /sources/{name}`, remplacement explicite.
4. **GEO-3** : attributs sélectionnables dans les tuiles + simplification par zoom.
5. **GEO-2** : support multi-colonnes géométriques (ou refus explicite et documenté).

---

## 8. Priorisation

| Priorité | Éléments | Pourquoi maintenant |
| --- | --- | --- |
| **P0** | SEC-1, BUG-1, BUG-4 | Lecture de fichiers arbitraires ; les jeux de données français cassent `/rows` ; branche rouge |
| **P1** | BUG-2, BUG-3, PERF-1, OPS-1, CI | Corruption d'état visible par l'utilisateur, N+1 sur le chemin le plus chaud, pertes de données à l'arrêt |
| **P2** | SEP-1 → SEP-6, PERF-4 (context) | À faire **avant** d'ajouter des fonctionnalités : le coût croît avec chaque endpoint |
| **P3** | Migrations (pas pour le moment!), versionnement d'API, registre de formats, config env | Fondations d'évolutivité, non urgentes mais bloquantes à moyen terme |
| **P4** | GEO-1/2/3, endpoints manquants, observabilité avancée | Valeur produit, une fois la base saine |

**Ordre conseillé** : 0 → 1 → 2 → 3 → 4 (+5.2 dans le même refactor) → 5 → 6 → 7 → 8.
Le point de bascule est l'étape 4 : tant qu'elle n'est pas faite, chaque fonctionnalité ajoutée
alourdit `TableService` et l'interface `Storage`, et le coût du refactor augmente.
