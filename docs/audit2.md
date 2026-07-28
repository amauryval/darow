# Audit GeoAPI n°2 — consolidation

Audit réalisé le 2026-07-28 sur la branche `develop` (commit `6dc6a08`), après application de la
majeure partie du plan de `docs/audit.md`. Objectif : **vérifier** ce qui a réellement été corrigé
(par exécution, pas par lecture seule) et identifier ce que les nouvelles fonctionnalités ont
introduit ou laissé ouvert.

Chaque défaut listé en §3 a été **reproduit** ; la trace d'exécution est citée. Les vérifications
ont été faites via des tests jetables, supprimés depuis : l'arbre de travail est inchangé.

---

## 1. Synthèse

Le premier audit est appliqué sérieusement. Les trois blocs structurants sont là et tiennent :

- **Séparation des couches** : `SQLWrapper` est sorti de l'agrégat `Storage`, chaque service
  déclare ses ports chez lui (`metadataStore`, `columnTyper`, `rowLister`…), le router dépend
  d'interfaces (`ports.go`) et non plus de structs concrètes, `TableService` est éclaté en 6
  services. `ExportService` est sorti du router. **Aucun service ne peut plus appeler `Exec`/`Query`.**
- **Sécurité de `/derive`** : la liste noire de mots-clés est remplacée par une validation
  structurelle sur l'AST de DuckDB (`json_serialize_sql`). Les 4 requêtes de contournement du
  premier audit sont désormais **rejetées**, y compris depuis une sous-requête, et
  `SELECT 'insert' AS x` est accepté.
- **Fondations** : `context` partout, CI, Dockerfile, `/healthz`+`/readyz`, arrêt gracieux, `slog`,
  ETag sur les tuiles, config env + validation, `/api/v1`, registre de formats, suite de conformité
  storage, détection du CRS source.

État de l'outillage, vérifié :

| Vérification | Résultat |
| --- | --- |
| `go test ./...` | **OK** |
| `gofmt -l .` / `go vet ./...` | **OK** — aucun fichier, aucun diagnostic |
| Couverture | config 93,0 % · router 78,4 % · service 78,9 % · storage 95,7 % · duckdb 78,3 % |
| CI / Dockerfile | **présents** (`.github/workflows/ci.yml`) |

Ce qui reste ne relève plus de l'architecture mais de **deux angles morts** :

1. **Les chemins d'écriture n'ont pas suivi les chemins de lecture.** GEO-1 a corrigé l'import et
   la lecture ; l'écriture d'une géométrie via l'API reste en CRS non converti et place les points
   **à côté de Null Island**. C'est le défaut le plus grave de cet audit (§3.1).
2. **Les cas non nominaux ne sont pas couverts.** pkey inexistant, géométrie NULL, projection
   partielle dans `/derive`, `limit` absent : chacun casse, et la nouvelle couche de streaming rend
   deux de ces échecs invisibles pour le client (200 déjà envoyé).

S'ajoute une régression de conception : `PATCH /tables/{id}` (renommage, livré à l'étape 8) casse
le lien source↔table, qui est indexé sur `tableName` (§3.6).

---

## 2. Vérification du premier audit

Statut établi par exécution ou lecture ciblée, pas d'après les annotations `[FAIT]`.

| Item | Statut | Vérification |
| --- | --- | --- |
| SEC-1 (`/derive` lit des fichiers) | **corrigé** | `read_text`/`read_csv`/`duckdb_settings` → `table function "…" is not allowed` ; `SELECT * FROM metadata` → `unknown table "metadata"` ; idem depuis une sous-requête |
| BUG-1 (colonnes accentuées) | **corrigé** | import d'un GeoJSON avec `nom_région` et `Population 2020` → `GetRowsByPkey` OK, `ColumnType("nom_région")` = `VARCHAR` |
| BUG-2 (métadonnées orphelines) | **corrigé** | `Rollback` sur chaque branche d'échec de `ImportFromFile`/`DeriveTable` |
| BUG-3 (TOCTOU sur `Load`) | **corrigé** | `mu` tenu sur tout le check+import ; `List()` renvoie une copie profonde |
| BUG-4 (test rouge, gofmt) | **corrigé** | suite verte, `gofmt -l` vide |
| BUG-5 (collision `pkey`) | **corrigé** | contrat explicite : `pkey` gagne, la colonne homonyme est écartée |
| SEC-2 (taille de requête) | **corrigé** | `maxBodyBytes`, avec rejet anticipé sur `Content-Length` |
| SEC-2 (authentification) | **non traité** | tous les endpoints d'écriture restent ouverts — jamais planifié dans le plan (§4.6) |
| SEC-3 (fuite d'erreurs) | **partiel** | `WriteError` masque bien les 5xx, mais les handlers renvoient les erreurs storage en 4xx, où le détail passe (§3.7) |
| PERF-1 (N+1 `ColumnType`) | **corrigé** | `SchemaService.Types` caché ; `convertRow` ne fait plus d'I/O par cellule |
| PERF-2 (export non borné) | **corrigé** | pagination `forEachPage` + streaming |
| PERF-4 (`context`) | **corrigé** | propagé sur toute la surface storage et service |
| PERF-5 (caches non bornés) | **corrigé** | LRU expirables, taille et TTL en config |
| SEP-1 → SEP-6 | **corrigés** | voir §1 |
| OPS-1/OPS-2, HTTP-1/HTTP-2 | **corrigés** | timeouts + `Shutdown`, `/healthz`+`/readyz`, `slog`+RequestID, `writeJSON`, ETag |
| GEO-1 (CRS source) | **corrigé en lecture** | `DetectCRS` + reprojection depuis le CRS réel + override `?srid=` — **mais pas en écriture** (§3.1) |
| GEO-2 (multi-géométrie) | **corrigé** | refus explicite documenté |
| GEO-3 (attributs de tuile) | **corrigé côté serveur** | `?attrs=` + simplification par zoom — non consommé par l'UI (§4.5) |
| Endpoints manquants | **exposés** | `/filter`, `/distinct`, `PATCH /tables/{id}`, `DELETE /sources/{name}` — `/filter` et `/distinct` sans client front (§4.5) |
| Migrations `metadata` | **volontairement reporté** | risque rappelé en §4.4 |
| PERF-3 (pool DuckDB) | **volontairement écarté** | `SetMaxOpenConns(1)` assumé |

---

## 3. Nouveaux défauts

### 3.1 NEW-GEO-1 (critique) — Une géométrie écrite par l'API est placée au mauvais endroit

`CreateFromFile` reprojette la source vers EPSG:3857 (correct depuis GEO-1). Mais
`Adapter.InsertRow` / `Adapter.Update` écrivent le WKT fourni par le client via
`ST_GeomFromText(...)` **sans aucune transformation** (`table.go:330`, `table.go:402`). Le WKT est
donc interprété comme étant déjà en mètres Web Mercator, alors que tout le reste de l'API — bbox
des métadonnées, `ST_AsGeoJSON` de l'export — parle en lon/lat.

Reproduit : insertion de `POINT(2.35 48.85)` (Paris) dans une table géo, puis relecture reprojetée
en lon/lat :

```
inserted POINT(2.35 48.85) reads back as lon/lat = POINT (2.1110409176808754e-05 0.00043882701628809596)
```

Soit ~0,00002° / 0,0004° : le point atterrit **au large du golfe de Guinée**, pas à Paris. Aucune
erreur, aucun avertissement. La ligne est ensuite comptée dans le bbox de la table, qu'elle fausse
pour tous les consommateurs (le front cadre la carte dessus).

**Recommandation** : symétriser avec la lecture — `ST_Transform(ST_GeomFromText(?), 'OGC:CRS84',
'EPSG:3857')` à l'insertion et à l'update, et documenter le CRS d'entrée dans la règle 22. Test :
insérer `POINT(2.35 48.85)`, relire le bbox, vérifier qu'il encadre Paris.

=> faut probleme car le front se chargera de générer des données à la bonne projection: NE PAS FAIRE

### 3.2 NEW-BUG-1 (élevé) — `PATCH /rows/{pkey}` sur une clé inexistante fait paniquer le handler

`updateRow` (`handlers_rows.go:153`) relit la ligne après l'update et déréférence le résultat sans
tester `nil`, alors que `RowService.GetRow` renvoie explicitement `(nil, nil)` pour une ligne
absente. `Update` sur un pkey inconnu ne remonte aucune erreur (0 ligne affectée), donc le chemin
est atteint dès qu'un client se trompe de pkey.

Reproduit :

```
panic: runtime error: invalid memory address or nil pointer dereference
PATCH unknown pkey -> status=500 body=
```

`middleware.Recoverer` rattrape la panique, mais le client reçoit un **500 au corps vide** (pas
même le JSON d'erreur), et la pile est écrite dans les logs à chaque occurrence. Le cas correct est
un 404, comme `getRow` le fait déjà.

### 3.3 NEW-BUG-2 (élevé) — L'export GeoJSON casse dès qu'une géométrie est NULL

`GetGeometryGeoJSON` scanne `ST_AsGeoJSON(geom)` dans un `string`. Une géométrie NULL — parfaitement
légale, produite par toute `Feature` GeoJSON à `"geometry": null` — fait échouer le scan :

```
GetGeometryGeoJSON -> map[] err=sql: Scan error on column index 1 …: converting NULL to string is unsupported
```

Deux conséquences en cascade :

1. La page **entière** échoue, pas seulement la ligne fautive : `map[]`, aucune géométrie exportée.
2. `ExportService.Export` est appelé **après** que le handler a écrit `200` et les en-têtes
   (`handlers_export.go:51-53`). L'erreur ne peut plus changer le statut : elle est seulement
   loggée, et le client reçoit un corps tronqué à `{"type":"FeatureCollection","features":[` —
   **du JSON invalide, avec un 200**. Un client ne peut pas distinguer ça d'un export réussi.

Même si le scan était corrigé, `json.Marshal` d'un `json.RawMessage` vide échouerait à son tour
(`unexpected end of JSON input`) : il faut traiter le cas NULL explicitement en émettant
`"geometry": null`.

**Recommandation** : `sql.NullString` côté storage + `null` en sortie ; et, pour le streaming en
général, écrire le premier octet du corps le plus tard possible (une première page réussie avant
d'engager le statut) ou fermer la connexion brutalement pour signaler la troncature.

### 3.4 NEW-BUG-3 (élevé) — `/derive` n'accepte en pratique que `SELECT *`

`CreateFromQuery` crée un index unique sur `__pkey` (`create.go:236`) après avoir matérialisé la
table. Or `__pkey` est une colonne **interne**, jamais exposée par l'API (les lignes sortent sous la
clé `pkey`, et `GetRowsByPkey` la retire du résultat). Un utilisateur qui écrit la requête naturelle
`SELECT nom FROM …` obtient :

```
derive without __pkey -> err=Binder Error: Table "t_2ef678c6ecca4b4ab952108d6049e39f" does not have a column named "__pkey"
derive SELECT * -> err=<nil>
```

Donc : toute projection partielle, toute agrégation (`GROUP BY`), toute jointure qui ne propage pas
`__pkey` échoue. La fonctionnalité `/derive` est réduite à `SELECT * [WHERE …]`. Le message d'erreur
renvoyé au client **fuit le `physicalName`** (voir §3.7).

**Recommandation** : générer la clé côté serveur comme le fait `CreateFromFile` —
`CREATE TABLE x AS SELECT uuid()::TEXT AS __pkey, * FROM (<requête>) s`, en refusant une requête qui
projette déjà `__pkey`. Tests : `SELECT nom FROM t` et un `GROUP BY` doivent aboutir.

### 3.5 NEW-BUG-4 (moyen) — `POST /tables/{id}/filter` sans `limit` renvoie toujours zéro ligne

`filterRows` passe `body.Limit` tel quel ; absent du JSON, il vaut `0`, et le storage émet
`LIMIT 0`. Reproduit :

```
filter limit=0  -> 0 rows (err=<nil>)
filter limit=10 -> 1 rows
```

Un client qui omet `limit` reçoit `{"rows":[]}` avec un 200 : il conclut que rien ne correspond au
filtre. Par ailleurs `limit` n'est **pas plafonné** par `apiCfg.MaxRowLimit`, contrairement à
`GET /rows` qui passe par `paginationParams` — un `limit: 100000000` matérialise toute la table en
mémoire, ce qui annule localement le travail de PERF-2.

**Recommandation** : réutiliser `paginationParams` (défaut + plafond) pour `/filter`.

### 3.6 NEW-BUG-5 (moyen) — Le renommage d'une table casse le lien avec sa source

`MetadataService.LoadedSourceNames` construit sa map en clé sur `meta.TableName`
(`metadata.go:130`), et `SourceService.Refresh` la consulte avec le **nom de fichier** de la source
(`source.go:122`, et `nom.gpkg::couche` pour les couches). Le lien source↔table est donc une
égalité de chaînes sur un champ que `PATCH /tables/{id}` rend librement modifiable depuis l'étape 8.

Deux effets, sans aucune protection :

- renommer une table chargée depuis `communes.geojson` la « détache » : la source réapparaît comme
  non chargée, et un second `POST /sources/{name}/load` **réimporte le même fichier dans une
  deuxième table** — exactement le doublon que BUG-3 avait fermé côté concurrence ;
- renommer n'importe quelle table en `communes.geojson` la fait passer pour la table de cette
  source : `DELETE /sources/communes.geojson` supprimerait alors **la mauvaise table**.

Rien n'impose l'unicité de `tableName` en base (seul `physicalName` est `UNIQUE`), donc deux tables
peuvent porter le même nom et la map en retient une arbitrairement.

**Recommandation** : stocker le lien explicitement (colonne `sourceName`/`sourceLayer` dans
`metadata`, alimentée par `ImportFromFile`) au lieu de le déduire de `tableName` — ce qui rend
`tableName` purement cosmétique, comme le renommage le suppose. Nécessite une migration de schéma
(§4.4).

### 3.7 NEW-SEC-1 (moyen) — Les erreurs internes fuient toujours, par la porte des 4xx

SEC-3 est corrigé pour les 5xx : `WriteError` remplace le message par `http.StatusText(code)` et
logge. Mais les handlers mappent systématiquement **toute** erreur de service vers un 4xx :
`deriveTable` → 400, `filterRows` → 400, `addRows`/`updateRow` → 400, `loadSource` → 409,
`deleteSource`/`getSourceMetadata` → 404. Le détail interne repart donc au client sans filtre. Le
message obtenu en §3.4 en est l'illustration exacte :

```
Binder Error: Table "t_2ef678c6ecca4b4ab952108d6049e39f" does not have a column named "__pkey"
```

`physicalName` est exposé, et avec lui la structure interne. Le même chemin renvoie du texte
d'erreur DuckDB brut pour un WKT invalide, un chemin de fichier pour un import raté, etc.

Ce défaut a un second visage : le **statut est faux**. Une panne storage pendant un
`POST /sources/{name}/load` est annoncée `409 Conflict`, une panne pendant `DELETE /sources/{name}`
est annoncée `404 Not Found`. Un client (ou une sonde) ne peut pas distinguer « ta requête est
invalide » de « le serveur est cassé », et un opérateur ne voit rien dans les logs puisque le
chemin 4xx ne logge pas.

**Recommandation** : typer les erreurs métier (comme `ExportValidationError` le fait déjà, avec
`errors.As` côté handler) et faire du 500 le défaut, pas l'exception — l'inverse de la situation
actuelle.

### 3.8 NEW-SEC-2 (faible) — La validation de `/derive` ne couvre pas les fonctions scalaires

La validation d'AST allow-liste les **relations** (tables de base, CTE) et rejette les
fonctions-table. Elle ne dit rien des fonctions **scalaires**, qui ne sont donc pas contraintes.
Vérifié bout en bout :

```
ACCEPTED SELECT current_setting('temp_directory') => "/tmp/…/a.db.tmp"
ACCEPTED SELECT current_setting('extension_directory') => ""
REJECTED SELECT CAST((SELECT count(*) FROM duckdb_settings()) AS TEXT) : table function "duckdb_settings" is not allowed
REJECTED SELECT (SELECT tableName FROM metadata LIMIT 1)          : unknown table "metadata"
```

L'impact aujourd'hui est limité (divulgation de chemins du serveur). `getenv` n'existe pas dans ce
build DuckDB — vérifié : `Catalog Error: Scalar Function with name getenv does not exist!` — donc
pas d'exfiltration de variables d'environnement. Mais **cette garantie tient au catalogue du build,
pas au validateur** : une mise à jour de DuckDB ou l'installation d'une extension peut l'annuler
sans que rien dans le code ne change.

Le point 1 de la recommandation SEC-1 du premier audit — `SET enable_external_access = false` sur
la connexion — n'a pas été appliqué. C'est la défense en profondeur qui manque ici : elle coupe la
classe entière, y compris les fonctions scalaires à venir.

### 3.9 NEW-BUG-6 (critique) — Ordre de suppression d'une source

`SourceService.Delete` retire l'entrée du cache **et** le fichier du disque, puis supprime la table
(`source.go:340-350`). Si `RemoveTable` échoue, le fichier et l'entrée sont déjà perdus : la table
survit sans source, définitivement, et le handler annonce un `404` (§3.7) pour une erreur qui n'a
rien d'un « non trouvé ». Inverser l'ordre (tables d'abord, fichier ensuite) rend l'échec réparable
par un simple retry.

=> NON le fichier NE DOIT PAS être supprimé !!!!!

### 3.10 NEW-BUG-7 (faible) — L'export est plafonné par les timeouts HTTP

`middleware.Timeout` (30 s par défaut) et `http.Server.WriteTimeout` (30 s) s'appliquent aussi au
handler d'export, qui streame. Sur une table volumineuse, le contexte est annulé en cours de route :
le client reçoit un fichier **tronqué avec un 200**, même mécanique qu'en §3.3. PERF-2 a supprimé le
plafond mémoire, pas le plafond temporel.

**Recommandation** : exclure les routes de streaming du middleware de timeout (chi permet un
sous-routeur sans ce middleware) et laisser `WriteTimeout` à 0 pour elles, ou basculer l'export sur
un mode asynchrone (job + polling) si des tables de plusieurs millions de lignes sont attendues.

---

## 4. Points de conception restants

### 4.1 Le cache de métadonnées distribue des pointeurs partagés

`MetadataService.GetMetadata` met en cache un `*storage.Metadata` et renvoie **le même pointeur** à
tous les appelants (`metadata.go:30-40`), y compris au `tableMiddleware` qui le place dans le
contexte de chaque requête. Aucun consommateur ne le mute aujourd'hui, mais rien ne l'empêche : une
seule mutation corromprait le cache pour toutes les requêtes suivantes, et sous `-race` le défaut
n'apparaîtrait que si l'écriture est concurrente. Renvoyer une copie (la struct est plate, la copie
est triviale) supprime l'invariant non gardé.

### 4.2 L'adaptateur relit les métadonnées à chaque méthode

Presque toutes les méthodes de `Adapter` commencent par `a.GetMetadata(ctx, tableId)`, qui exécute
un `SELECT` — le cache vit au niveau service et ne protège pas l'adaptateur. Un
`GET /rows` coûte ainsi 3 `SELECT … FROM metadata` (`GetPkeys`, `GetRowsByPkey`, `RowsCount`) en
plus des requêtes utiles, et un export de 100 pages en coûte 300, plus 100 `COUNT(*)` puisque
`ListRows` recompte le total à chaque page. Ce n'est plus le N+1 par cellule de PERF-1, mais c'est
le même motif d'un cran au-dessus. Passer `*storage.Metadata` (déjà résolu par le middleware) le
long de l'appel, plutôt que le `tableId`, supprimerait la quasi-totalité de ces requêtes.

### 4.3 Les redirections non versionnées sont devenues du code mort

`/tables`, `/tables/*`, `/sources`, `/sources/*` redirigent en 308 vers `/api/v1` « le temps que le
front migre ». Le front est migré (`web/src/client/http.ts` : `API_BASE = "/api/v1"`, sans exception).
Ces routes n'ont plus de consommateur connu et élargissent la surface à la racine, là où
cohabitent `/ui`, `/swagger`, `/healthz`. À supprimer, ou à documenter comme contrat public
supporté — mais pas à laisser en état transitoire indéfini.

### 4.4 L'absence de migrations devient bloquante

Reporté explicitement (« PAS POUR LE MOMENT ») au premier audit, ce choix est maintenant sur le
chemin critique : la correction propre de §3.6 demande une colonne `sourceName` dans `metadata`, et
`CREATE TABLE IF NOT EXISTS` ne la fera jamais parvenir aux bases existantes. Toute base créée avant
le changement restera silencieusement sans la colonne. Un mécanisme minimal (table
`schema_version` + liste ordonnée de `ALTER TABLE` appliqués au démarrage) est un préalable, pas un
raffinement.

### 4.5 Des capacités livrées côté serveur restent invisibles côté produit

Le premier audit reprochait à `BuildFilter`/`GetDistinctValues` d'être « du code mort qui donne
l'illusion d'une capacité livrée ». Les endpoints existent désormais, mais le motif s'est déplacé
d'un cran : `web/src/client/` n'a **aucune fonction** pour `POST /tables/{id}/filter` ni pour
`GET /tables/{id}/columns/{col}/distinct`, et `geo.ts:tileUrl` ne transmet jamais `?attrs=`, donc la
sélection d'attributs de GEO-3 n'est utilisée par personne. Ce sont trois fonctionnalités testées,
documentées dans Swagger, et sans utilisateur. Soit on les branche dans l'UI (filtre par colonne +
infobulle sur la carte, ce qui est précisément la valeur produit visée), soit on assume qu'elles
sont une API publique pour des clients tiers — mais le statu quo empêche de savoir si elles
fonctionnent réellement en usage réel (§3.5 en est la démonstration : le défaut `limit` absent
n'aurait pas survécu à un premier appel depuis l'UI).

### 4.6 L'authentification n'a jamais été planifiée

SEC-2 du premier audit signalait que tous les endpoints d'écriture sont ouverts. Le plan en 8 étapes
n'a traité que la taille des corps de requête ; l'authentification n'apparaît dans aucune étape.
Aujourd'hui `DELETE /api/v1/sources/{name}` — qui supprime un fichier **et** ses tables — est
accessible sans aucune forme d'identification. C'est acceptable pour un service sur `localhost`,
mais ce n'est écrit nulle part : ni `config.yaml`, ni `docs/spec.md`, ni le README ne posent que le
service n'est pas destiné à être exposé. **Décision à prendre explicitement** : soit un middleware
d'authentification (une clé d'API partagée suffirait au besoin actuel), soit une mention en toutes
lettres du modèle de menace assumé.

---

## 5. Plan de consolidation

Trois lots, indépendamment livrables, chacun laissant la branche verte.

### Lot A — Corriger les chemins non nominaux *(≈ 1 j)*

1. **NEW-GEO-1** : `ST_Transform(ST_GeomFromText(?), 'OGC:CRS84', 'EPSG:3857')` sur `InsertRow` et
   `Update`. Test : insérer Paris en lon/lat, vérifier que le bbox l'encadre.
2. **NEW-BUG-1** : `updateRow` renvoie 404 quand `GetRow` rend `nil`.
3. **NEW-BUG-2** : `sql.NullString` dans `GetGeometryGeoJSON`, `"geometry": null` en sortie. Test :
   export d'une table contenant une `Feature` sans géométrie.
4. **NEW-BUG-3** : `__pkey` généré par `CreateFromQuery` autour de la requête utilisateur. Tests :
   `SELECT nom FROM t` et un `GROUP BY` aboutissent.
5. **NEW-BUG-4** : `/filter` passe par `paginationParams` (défaut + plafond `MaxRowLimit`).
6. **NEW-BUG-6** : dans `SourceService.Delete`, supprimer les tables avant le fichier.

*Critère de sortie* : chacun des 6 cas a un test de non-régression qui échouait avant.

### Lot B — Contrat d'erreur et surface d'attaque *(≈ 1 j)*

1. **NEW-SEC-1** : typer les erreurs métier (`errors.As` comme pour `ExportValidationError`) ;
   500 par défaut, 4xx seulement sur erreur typée. Test : une panne storage simulée sur `/derive`
   renvoie 500 sans `physicalName` dans le corps, et logge.
2. **NEW-SEC-2** : `SET enable_external_access = false` après le chargement de `spatial`, imports
   `ST_Read` isolés sur une connexion dédiée. Test : `current_setting('temp_directory')` et un
   `read_csv` restent rejetés ou vides.
3. **NEW-BUG-7** : sous-routeur sans `middleware.Timeout` pour `/export`, `WriteTimeout` neutralisé
   sur ce chemin.
4. [NE PAS FAIRE] §4.6 : trancher l'authentification — middleware à clé d'API, ou modèle de menace écrit dans
   `docs/spec.md`.

### Lot C — Dette de conception *(≈ 2 j)*

1. Migrations `metadata` (`schema_version` + `ALTER TABLE` ordonnés au démarrage) — préalable au
   point suivant (§4.4).
2. **NEW-BUG-5** : colonnes `sourceName`/`sourceLayer` dans `metadata`, alimentées par
   `ImportFromFile` ; `LoadedSourceNames` ne dépend plus de `tableName`. Tests : renommer une table
   chargée ne la détache pas de sa source ; renommer une table en un nom de fichier ne la fait pas
   passer pour la table de cette source.
3. §4.1 : `GetMetadata` renvoie une copie.
4. §4.2 : faire circuler `*storage.Metadata` au lieu de `tableId` dans l'adaptateur (ou cacher les
   métadonnées côté storage) ; `ListRows` ne recompte pas le total à chaque page d'export.
5. §4.3 : supprimer les redirections non versionnées.
6. [NE PAS FAIRE] §4.5 : brancher `/filter`, `/distinct` et `?attrs=` dans l'UI
7. ajouter le renommage de table dans la GUI, dans la page "sources".
---

## 6. Priorisation

| Priorité | Éléments | Pourquoi maintenant |
| --- | --- | --- |
| **P0** | NEW-GEO-1 | Corruption silencieuse de données géographiques écrites par l'API — plus le temps passe, plus il y a de lignes fausses en base à retrouver |
| **P1** | NEW-BUG-1, -2, -3, -4 | Chacun casse un parcours utilisateur normal ; -2 et -3 renvoient un succès apparent sur un échec réel |
| **P2** | NEW-SEC-1 (contrat d'erreur), NEW-SEC-2, §4.6 | Statuts faux + fuite d'internes + décision d'exposition à prendre avant tout déploiement hors `localhost` |
| **P3** | Migrations, NEW-BUG-5 | Le renommage livré et le lien source↔table sont incompatibles ; la correction propre exige les migrations |
| **P4** | §4.1 à §4.5 | Dette de conception, sans impact utilisateur immédiat |

**Ordre conseillé** : A → B → C. Le lot A est intégralement constitué de correctifs locaux et
testables ; aucun ne dépend du lot B ni du lot C.
