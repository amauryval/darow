# Plan — Repository pour le catalogue (`metadata` et suites)

Plan de refonte de la gestion des données de catalogue : aujourd'hui la table `metadata` est gérée
directement par `duckdb.Adapter`, au milieu des méthodes de données. L'objectif est d'isoler le
catalogue derrière un repository dédié, et de rendre l'ajout d'une **nouvelle table de catalogue**
(styles, tags, jobs d'import, historique…) une opération mécanique plutôt qu'une modification
diffuse.

Ce plan est un document de conception : il décrit la cible et l'ordre des étapes. Il ne fixe pas
les noms au caractère près (qui doivent être en anglais).

**Plan jumeau.** `docs/plan-repository-data.md` applique le même pattern à l'autre moitié du
storage : les tables **référencées** par `metadata` (les `t_<uuid>` : lignes, schéma, géométrie).
Les deux se mènent **en parallèle** — fichiers disjoints — et ne se rejoignent qu'à l'étape commune
de démantèlement de `storage.Storage` (§5 du plan données, étape 8 ici). Seule contrainte : figer
l'arborescence cible des deux (§2.2 de chaque plan) avant de démarrer l'un ou l'autre.

---

## 1. Problème

### 1.1 Deux domaines dans un seul adapter

`duckdb.Adapter` porte aujourd'hui deux responsabilités sans rapport :

- **Le catalogue** : la table système `metadata`, un enregistrement par table logique.
- **Les données** : les tables `t_<uuid>`, dont le schéma est dynamique et découvert par `DESCRIBE`.

Les deux ont des propriétés opposées. Le catalogue a un schéma fixe, connu à la compilation,
versionné, avec des colonnes typées et un besoin de requêtes structurées (tri, pagination, jointure).
Les tables de données ont un schéma inconnu, ne se versionnent pas, et se manipulent par
introspection. Les mélanger fait que le code du catalogue hérite des habitudes des données :
`map[string]any`, allowlists de chaînes, `fmt.Sprintf` partout.

### 1.2 Symptômes actuels

| Symptôme | Localisation |
|---|---|
| La liste des colonnes est répétée 4 fois (DDL, `SELECT`, allowlist d'update, colonnes triables) | `duckdb/adapter.go:24`, `:195`, `:259`, `:127` |
| Écritures non typées : `UpdateMetadata(ctx, id, map[string]any)` | `storage/metadata.go:55` |
| Un champ inconnu échoue à l'exécution, pas à la compilation | `duckdb/adapter.go:266-268` |
| La pagination de `/tables` est faite en Go après chargement de tous les ids | `service/metadata.go:48-67` |
| `LoadedSourceNames` fait un `GetMetadata` par table à chaque refresh de sources | `service/metadata.go:133-154` |
| Le framework de migration existe mais la liste est vide, alors que le schéma a évolué | `duckdb/migrations.go:15`, `duckdb/adapter.go:19-23` |
| `rowCount` est un `INTEGER` 32 bits pour un `int64` Go | `duckdb/adapter.go:33` |
| Suppression d'une table = deux `Exec` non transactionnels (DROP puis DELETE) | `duckdb/adapter.go:244-248` |

### 1.3 Ce qui casse à la prochaine table

Dès qu'une deuxième table de catalogue apparaît — un style de couche, un tag, un job d'import — il
faut aujourd'hui :

1. Ajouter son DDL dans une constante à côté de `metadataSchema`, ou l'y coller.
2. Écrire ses `SELECT`/`INSERT`/`UPDATE` à la main dans `adapter.go`, qui grossit.
3. Se souvenir de la supprimer dans `Adapter.Remove`, manuellement, sans transaction, sans filet si
   on oublie — DuckDB n'appliquera aucune contrainte de clé étrangère qu'on n'a pas écrite.
4. Étendre `storage.Storage`, donc les mocks, donc les tests, pour des méthodes qui n'ont rien à
   voir avec le contrat storage générique qu'un futur adapter PostGIS doit tenir.

C'est ce point 3 qui est dangereux : la suppression en cascade devient une convention orale.

---

## 2. Cible

### 2.1 Vocabulaire

- **Catalogue** : l'ensemble des tables système décrivant les tables logiques. `metadata` en est la
  table racine ; les autres s'y rattachent par `tableId`.
- **Repository** : un type par table de catalogue, propriétaire exclusif de son SQL, exposant des
  méthodes typées sur des structs de domaine. Aucun `map[string]any`, aucune allowlist de chaînes.
- **Catalog** : l'agrégat qui possède les repositories, la connexion, les migrations, et les
  opérations transverses (suppression en cascade, transaction).

### 2.2 Découpage cible

```
internal/storage/
├── catalog/                  ← nouveau, indépendant du moteur
│   ├── catalog.go            ← interface Catalog + types transverses (ListParams, Page)
│   ├── metadata.go           ← Metadata, MetadataPatch, MetadataFilter, MetadataRepository
│   └── (styles.go, tags.go, jobs.go… au fil des besoins)
├── duckdb/
│   ├── catalog/              ← implémentation DuckDB du catalogue
│   │   ├── catalog.go        ← ouverture, registre des tables, cascade, WithTx
│   │   ├── metadata.go       ← metadataRepo : colonnes, scan, requêtes
│   │   └── migrations.go     ← liste versionnée, déplacée ici
│   ├── adapter.go            ← ne garde que Query/QueryRow/Exec/Begin + le lock d'écriture
│   ├── table.go, mvt.go, features.go, create.go, derive.go  ← inchangés (données)
└── (metadata.go actuel : les interfaces MetadataManager/Reader/Writer disparaissent)
```

`storage.Storage` cesse d'agréger `MetadataManager`. Un adapter de données implémente
`TableManager + GeoTableManager + TableCreator` ; le catalogue est un composant à part, injecté
séparément dans `core.NewServices`. La suite de conformité `storagetest` se scinde en conséquence :
une suite « données » et une suite « catalogue ».

### 2.3 Contrat `MetadataRepository`

Typé, sans `map`, avec les opérations que les services font aujourd'hui à la main :

| Méthode | Remplace | Gain |
|---|---|---|
| `Get(ctx, tableId) (Metadata, error)` | `GetMetadata` | erreur `NotFound` typée (audit §3.2) |
| `List(ctx, ListParams) (Page[Metadata], error)` | `TableIds` + N × `GetMetadata` + découpe Go | une requête, `LIMIT/OFFSET/ORDER BY` en SQL, `total` par `COUNT(*)` |
| `BySource(ctx) (map[SourceKey]string, error)` | `LoadedSourceNames` | une requête au lieu de 1+N |
| `Insert(ctx, NewMetadata) (tableId string, err error)` | `Create` | champs nommés à la création (dont `sourceName`/`sourceLayer`, aujourd'hui posés dans un `UpdateMetadata` séparé) |
| `Patch(ctx, tableId, MetadataPatch) error` | `UpdateMetadata(map)` | champs pointeurs, vérifiés à la compilation |
| `SetDerived(ctx, tableId, rowCount int64, bbox string) error` | moitié de `RefreshDerivedMetadata` | sépare le calcul (données) de l'écriture (catalogue) |
| `Delete(ctx, tableId) error` | moitié de `Remove` | ne touche plus au `DROP TABLE`, qui est du domaine données |

`ListParams` porte tri (validé contre l'allowlist du repository), offset, limite, et un filtre
optionnel (`IsGeo *bool`, `SourceName *string`) — ce que l'UI demandera tôt ou tard et qui est
aujourd'hui impossible sans tout charger.

### 2.4 Une seule déclaration de colonnes

Chaque repository expose sa liste de colonnes une fois, et en dérive `SELECT`, `INSERT`, l'allowlist
de tri et l'allowlist de patch. Ajouter un champ = éditer une struct + une migration. Le compilateur
signale tout le reste.

### 2.5 Registre et cascade

`Catalog` tient un registre des repositories rattachés à `tableId`. `Catalog.DeleteTable(ctx,
tableId)` ouvre une transaction, appelle `Delete(tableId)` sur **chaque** repository enregistré,
puis commit — la cascade devient structurelle et non plus une ligne à ne pas oublier. Une nouvelle
table de catalogue s'enregistre à sa construction ; elle est nettoyée sans que quiconque édite le
code de suppression.

Le `DROP TABLE t_<uuid>` reste côté adapter données, orchestré par `MetadataService.RemoveTable`
(catalogue d'abord ou données d'abord — à trancher à l'implémentation, avec le même traitement de
l'échec partiel que `Rollback` aujourd'hui).

### 2.6 Migrations réellement utilisées

`schemaMigrations` devient une liste non vide, avec la dette actuelle rattrapée :

1. `ALTER TABLE metadata ADD COLUMN sourceName TEXT` (aujourd'hui hors migration)
2. `ALTER TABLE metadata ADD COLUMN sourceLayer TEXT`
3. `rowCount` en `BIGINT` (audit §3.8)

Le mécanisme existant (`schema_version`, seed à `len(migrations)` pour une base neuve, seed à `0`
pour une base préexistante) est correct et se conserve tel quel. Chaque nouvelle table de catalogue
arrive comme une entrée de migration supplémentaire, jamais comme une édition du DDL initial.

Un test doit ouvrir une base fabriquée à l'ancien schéma et vérifier qu'elle migre — c'est le test
qui manque aujourd'hui et qui aurait attrapé le problème `sourceName`.

### 2.7 Ce que devient `MetadataService`

Il perd son SQL implicite et redevient ce que son nom annonce : un cache et une orchestration.

- `GetMetadata` : cache LRU devant `repo.Get` — inchangé.
- `ListTables` : délègue à `repo.List`, ne pagine plus en Go.
- `LoadedSourceNames` : délègue à `repo.BySource`.
- `RefreshDerived` : demande le `COUNT(*)`/bbox à l'adapter **données**, écrit via
  `repo.SetDerived`, invalide le cache. Le mélange actuel (une méthode storage qui lit les données
  *et* écrit le catalogue) disparaît.
- `Rollback`, `Create`, `UpdateMetadata` : inchangés en surface, retypés en dessous.

Aucun handler ne change. `handlers/ports.go` reste identique — c'est le bon signe que le découpage
est au bon endroit.

---

## 3. Étapes

Chaque étape est livrable et testable seule, dans cet ordre. Chaque étape inclut ses tests
(règle 17).

**Étape 1 — Extraire le catalogue sans changer le contrat.**
Créer `storage/duckdb/catalog/`, y déplacer `metadataSchema`, `migrations.go`, et les six méthodes
metadata de `adapter.go`. `Adapter` délègue au repository. Aucune signature publique ne bouge, tous
les tests existants passent inchangés. Étape purement mécanique, elle isole le risque des suivantes.

**Étape 2 — Rattraper la dette de migration.**
Remplir `schemaMigrations` (`sourceName`, `sourceLayer`, `rowCount` en `BIGINT`). Ajouter le test
d'ouverture d'une base à l'ancien schéma. C'est le seul correctif de compatibilité en attente ; il
doit précéder tout changement de forme du schéma.

**Étape 3 — Typer les écritures.**
Introduire `MetadataPatch` (champs pointeurs) et `NewMetadata`, dériver la liste de colonnes d'une
seule déclaration, supprimer l'allowlist de chaînes. Adapter `MetadataService` et ses trois
appelants (`Derive`, `Import`, `TablesHandler.RenameTable`). Supprime par construction le
`"unsupported metadata field"` runtime.

**Étape 4 — Pousser la pagination et les jointures en SQL.**
`List(ctx, ListParams)` avec `COUNT(*)` + `LIMIT/OFFSET`, `BySource` en une requête. Retirer la
découpe Go de `ListTables` et la boucle de `LoadedSourceNames`. Gain mesurable dès quelques
centaines de tables ; c'est l'étape qui justifie économiquement le reste.

**Étape 5 — Erreurs typées.**
`catalog.ErrNotFound` retourné par `Get` sur `sql.ErrNoRows` uniquement ; `ResolveTable` passe à
`response.MapServiceError`. Corrige l'audit §3.2 — une panne base cesse de se déguiser en 404.

**Étape 6 — Registre et cascade transactionnelle.**
`Catalog.Register` + `DeleteTable` transactionnel. Sans consommateur immédiat (une seule table),
mais c'est le prérequis de l'étape 7 et le moment où l'écrire est le moins cher.

**Étape 7 — Première table fille (au besoin réel, pas avant).**
Quand un besoin arrive — style de couche, tags, historique d'import — il s'implémente comme :
une struct de domaine, un repository, une migration, un `Register`. Rien d'autre. C'est le test de
validité du plan : si l'ajout demande de toucher `adapter.go`, `storage.Storage` ou les mocks, le
découpage a échoué.

**Étape 8 — Scinder `storagetest`.**
Une suite « données » (`TableManager`/`GeoTableManager`/`TableCreator`) et une suite « catalogue ».
Retirer `MetadataManager` de `storage.Storage`.

Les étapes 1, 2 et 5 sont indépendantes des autres et peuvent partir immédiatement. Les étapes 3 et
4 se font ensemble ou dans cet ordre. Les étapes 6 à 8 peuvent attendre un besoin.

---

## 4. Candidats de tables filles

Non planifiées, listées pour vérifier que le découpage tient. Toutes se rattachent à `tableId` et
se nettoient par la cascade de 2.5.

| Table | Contenu | Déclencheur probable |
|---|---|---|
| `table_style` | style de rendu par couche (couleur, épaisseur, palette par attribut) | l'UI carte, dès qu'un style n'est plus codé en dur côté Vue |
| `table_tag` | étiquettes libres, n↔n | organisation d'un catalogue à quelques centaines de tables |
| `table_column_meta` | alias d'affichage, unité, description par colonne | affichage lisible des colonnes importées (noms bruts de shapefile) |
| `import_job` | historique et état d'un import (fichier, couche, srid, erreur, durée) | import asynchrone d'un gros fichier, aujourd'hui synchrone et bloquant |
| `table_permission` | ACL par table | le jour où l'auth existe (audit §4.8) |

Aucune ne demande de modifier `metadata` : c'est le critère qui valide que la table racine est au
bon niveau de granularité.

---

## 5. Risques

| Risque | Traitement |
|---|---|
| L'étape 1 déplace beaucoup de code d'un coup | Aucun changement de signature : la revue se fait au diff de déplacement, les tests existants sont le filet |
| Les bases existantes en dev cassent à l'étape 2 | C'est déjà le cas aujourd'hui (limitation assumée) ; la migration les *répare* au lieu de les casser |
| `storage.Storage` amputé casse les mocks | Mécanique, à faire en une fois à l'étape 8, après stabilisation |
| Sur-conception : une seule table de catalogue aujourd'hui | Étapes 1 à 5 sont rentables seules (N+1, typage, erreurs) ; 6 à 8 attendent un besoin réel |
