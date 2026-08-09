# **OGC API - Tiles : Spécification pour les MVT (Mapbox Vector Tiles)**

*Conformité OGC + Intégration Technique*

***

## **1. Contexte et Portée**

### **1.1 Objectif**

- **Standardiser** l'accès aux **tuiles vectorielles (MVT)** via une **API RESTful**, en alignement avec les principes **OGC (Open Geospatial Consortium)**.
- **Interopérabilité** : Permettre aux clients (QGIS, Leaflet, OpenLayers, etc.) de consommer des MVT de manière unifiée et conforme aux normes OGC.
- **Extension d'OGC API - Tiles** : Les MVT sont supportées via le **module Vector Tiles (Part 2)**.

### **1.2 Public Cible**

- Développeurs de **serveurs géospatiaux** (Go, Python, Java, etc.).
- Intégrateurs de **clients SIG** (QGIS, applications web, etc.).
- Architectes de **solutions géospatiales** utilisant des tuiles vectorielles.

***

## **2. Architecture OGC pour les MVT**

### **2.1 Modèle de Données MVT**

- **Format** : Binaire **Protobuf** (`.pbf`).
- **Structure** :
  - Une tuile MVT contient une ou plusieurs **couches** (*layers*).
  - Chaque couche contient des **features** (géométries + attributs).
  - Les géométries sont **découpées** selon la tuile et **simplifiées** pour optimiser la taille.
- **Encodage** :
  - Coordonnées en **entiers** (relative à la tuile, 4096x4096 par défaut).
  - Attributs typés (entier, chaîne, booléen, etc.).
- **Content-Type** :
  - `application/x-protobuf` (recommandé par OGC).
  - `application/vnd.mapbox-vector-tile` (alternatif, historique).

### **2.2 Système de Tuilage**

- **WebMercatorQuad** (EPSG:3857) : **Standard pour les MVT** dans OGC API - Tiles.
- **Autres systèmes** : `WorldCRS84Quad` (EPSG:4326) possible, mais moins courant.
- **Schéma de tuilage** : `{z}/{x}/{y}` (ex: `12/1234/5678`).

### **2.3 Hiérarchie des Endpoints (OGC API - Tiles)**

\<mui:table-metadata title="Endpoints OGC pour MVT" />

| **Endpoint**                                | **Méthode HTTP** | **Description**                                                                         | **Exemple**                                                          |
| :------------------------------------------ | :--------------- | :-------------------------------------------------------------------------------------- | :------------------------------------------------------------------- |
| `/conformance`                              | `GET`            | Liste les standards OGC implémentés. Doit inclure `ogcapi-tiles-1` et `ogcapi-tiles-2`. | `GET /conformance`                                                   |
| `/api`                                      | `GET`            | Document **OpenAPI 3.0** décrivant l'API.                                               | `GET /api`                                                           |
| `/collections`                              | `GET`            | Liste des collections disponibles (chaque collection peut avoir un type `vector`).      | `GET /collections`                                                   |
| `/collections/{collectionId}`               | `GET`            | Métadonnées de la collection (doit inclure `"type": "vector"`).                         | `GET /collections/batiments`                                         |
| `/collections/{collectionId}/tiles`         | `GET`            | Accès aux tuiles MVT. **Doit supporter `f=pbf`**.                                       | `GET /collections/batiments/tiles/WebMercatorQuad/{z}/{x}/{y}?f=pbf` |
| `/collections/{collectionId}/tilejson.json` | `GET`            | Métadonnées au format **TileJSON** (recommandé pour les clients MVT).                   | `GET /collections/batiments/tilejson.json`                           |

***

## **3. Spécifications Techniques pour les MVT**

### **3.1 Format des Tuiles MVT**

- **Schéma Protobuf** : Définis par [vector\_tile.proto](https://github.com/mapbox/vector-tile-spec/blob/master/2.1/vector_tile.proto).

- **Champs obligatoires** :
  - `layers` : Liste des couches dans la tuile.
  - Chaque `layer` doit avoir :
    - `name` (string) : Nom de la couche.
    - `features` (liste) : Features de la couche.
    - `extent` (uint32) : Étendue de la couche (par défaut 4096).

- **Exemple de requête** :
  ```http
  GET /collections/batiments/tiles/WebMercatorQuad/12/1234/5678?f=pbf
  Accept: application/x-protobuf
  ```

### **3.2 TileJSON (Métadonnées pour MVT)**

- **Format** : JSON conforme au schéma [TileJSON 3.0.0](https://github.com/mapbox/tilejson-spec).
- **Champs obligatoires (OGC)** :
  ```json
  {
    "tilejson": "3.0.0",
    "name": "Bâtiments (MVT)",
    "version": "1.0.0",
    "scheme": "xyz",
    "tiles": [
      "https://mon-serveur.example.com/collections/batiments/tiles/WebMercatorQuad/{z}/{x}/{y}.pbf"
    ],
    "minzoom": 0,
    "maxzoom": 14,
    "bounds": [-180, -85.0511, 180, 85.0511],
    "center": [0, 0, 2],
    "attribution": "© Ma Organisation",
    "vector_layers": [
      {
        "id": "batiments",
        "description": "Couche des bâtiments",
        "minzoom": 10,
        "maxzoom": 14,
        "fields": {
          "id": "Identifiant unique",
          "hauteur": "Hauteur en mètres (float)"
        }
      }
    ]
  }
  ```
- **Champs spécifiques OGC** :
  - `vector_layers` : **Obligatoire** pour les MVT. Décrit les couches disponibles dans les tuiles.
  - `bounds` : **Obligatoire** pour définir l'étendue géographique (en `EPSG:4326`).

***

## **4. Implémentation en Go**

### **4.1 Structure de Base pour un Serveur MVT**

```go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
)

// TileJSON représente les métadonnées d'un service MVT.
type TileJSON struct {
	TileJSON     string            `json:"tilejson"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Scheme       string            `json:"scheme"`
	Tiles        []string          `json:"tiles"`
	MinZoom      int               `json:"minzoom"`
	MaxZoom      int               `json:"maxzoom"`
	Bounds       []float64         `json:"bounds"`
	Center       []float64         `json:"center"`
	Attribution  string            `json:"attribution"`
	VectorLayers []VectorLayer     `json:"vector_layers"`
}

type VectorLayer struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	MinZoom     int               `json:"minzoom"`
	MaxZoom     int               `json:"maxzoom"`
	Fields      map[string]string `json:"fields"`
}

// Handler pour /collections/{collectionId}/tilejson.json
func getTileJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	collectionID := vars["collectionId"]

	tileJSON := TileJSON{
		TileJSON: "3.0.0",
		Name:     collectionID,
		Version:  "1.0.0",
		Scheme:   "xyz",
		Tiles: []string{
			"https://mon-serveur.example.com/collections/" + collectionID + "/tiles/WebMercatorQuad/{z}/{x}/{y}.pbf",
		},
		MinZoom:  0,
		MaxZoom:  14,
		Bounds:   []float64{-180, -85.0511, 180, 85.0511},
		Center:   []float64{0, 0, 2},
		Attribution: "© Ma Organisation",
		VectorLayers: []VectorLayer{
			{
				ID:          "batiments",
				Description: "Couche des bâtiments",
				MinZoom:     10,
				MaxZoom:     14,
				Fields: map[string]string{
					"id":       "Identifiant unique",
					"hauteur":  "Hauteur en mètres",
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tileJSON)
}

// Handler pour /collections/{collectionId}/tiles/{tileMatrixSetId}/{tileMatrix}/{tileRow}/{tileCol}
func getMVTTile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	collectionID := vars["collectionId"]
	tileMatrixSetID := vars["tileMatrixSetId"]
	tileMatrix, _ := strconv.Atoi(vars["tileMatrix"])
	tileRow, _ := strconv.Atoi(vars["tileRow"])
	tileCol, _ := strconv.Atoi(vars["tileCol"])

	// Exemple : Récupérer la tuile depuis un fichier ou une base de données
	pbfPath := filepath.Join("data", collectionID, tileMatrixSetID, strconv.Itoa(tileMatrix), strconv.Itoa(tileRow), strconv.Itoa(tileCol)+".pbf")

	// Envoi du fichier .pbf
	http.ServeFile(w, r, pbfPath)
	w.Header().Set("Content-Type", "application/x-protobuf")
}

func main() {
	r := mux.NewRouter()

	// Endpoints OGC API - Tiles
	r.HandleFunc("/conformance", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"conformsTo": []string{
				"http://www.opengis.net/spec/ogcapi-tiles-1/1.0/conf/core",
				"http://www.opengis.net/spec/ogcapi-tiles-2/1.0/conf/vector-tiles",
			},
		})
	}).Methods("GET")

	r.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "openapi.json") // Fichier OpenAPI 3.0
	}).Methods("GET")

	r.HandleFunc("/collections", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"collections": []map[string]interface{}{
				{
					"id":   "batiments",
					"title": "Bâtiments",
					"type":  "vector",
					"links": []map[string]interface{}{
						{
							"rel":  "tilejson",
							"href": "/collections/batiments/tilejson.json",
							"type": "application/json",
						},
					},
				},
			},
		})
	}).Methods("GET")

	r.HandleFunc("/collections/{collectionId}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		collectionID := vars["collectionId"]
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   collectionID,
			"title": collectionID,
			"type":  "vector",
			"extent": map[string]interface{}{
				"spatial": map[string]interface{}{
					"bbox": [][]float64{{-180, -85.0511}, {180, 85.0511}},
					"crs":  "http://www.opengis.net/def/crs/OGC/1.3/CRS84",
				},
			},
		})
	}).Methods("GET")

	// Endpoints spécifiques MVT
	r.HandleFunc("/collections/{collectionId}/tilejson.json", getTileJSON).Methods("GET")
	r.HandleFunc("/collections/{collectionId}/tiles/{tileMatrixSetId}/{tileMatrix}/{tileRow}/{tileCol}", getMVTTile).Methods("GET")

	log.Println("Serveur démarré sur :8080")
	http.ListenAndServe(":8080", r)
}
```

### **4.2 Points Clés pour l'Implémentation en Go**

1. **Gestion des Tuiles MVT** :
   - Utiliser une bibliothèque comme [`go-protobuf`](https://github.com/golang/protobuf) pour encoder/décoder les MVT.
   - Stocker les tuiles dans un système de fichiers ou une base de données (ex: **DuckDB** pour les données vectorielles).

2. **Conformité OGC** :
   - **`/conformance`** : Doit lister les standards `ogcapi-tiles-1` et `ogcapi-tiles-2`.
   - **`/api`** : Doit fournir un document **OpenAPI 3.0** décrivant l'API.
   - **`/collections`** : Doit inclure le type `"vector"` pour les collections MVT.

3. **TileJSON** :
   - **Obligatoire** pour les clients MVT (ex: QGIS).
   - Doit inclure :
     - `vector_layers` : Description des couches dans les tuiles.
     - `bounds` : Étendue géographique en `EPSG:4326`.

4. **Système de Tuilage** :
   - **WebMercatorQuad** (EPSG:3857) est le standard pour les MVT.
   - Les coordonnées des tuiles suivent le schéma `{z}/{x}/{y}`.

***

## **5. Exemple de Fichier OpenAPI 3.0**

```yaml
openapi: 3.0.0
info:
  title: OGC API - Tiles (MVT)
  version: 1.0.0
  description: API conforme à OGC API - Tiles pour les tuiles vectorielles (MVT).

servers:
  - url: http://localhost:8080
    description: Serveur local

paths:
  /conformance:
    get:
      summary: Liste des standards OGC implémentés
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  conformsTo:
                    type: array
                    items:
                      type: string
                      example: "http://www.opengis.net/spec/ogcapi-tiles-1/1.0/conf/core"

  /collections:
    get:
      summary: Liste des collections disponibles
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: object
                properties:
                  collections:
                    type: array
                    items:
                      type: object
                      properties:
                        id:
                          type: string
                          example: "batiments"
                        type:
                          type: string
                          example: "vector"
                        links:
                          type: array
                          items:
                            type: object
                            properties:
                              rel:
                                type: string
                                example: "tilejson"
                              href:
                                type: string
                                example: "/collections/batiments/tilejson.json"

  /collections/{collectionId}/tilejson.json:
    get:
      summary: Métadonnées TileJSON pour une collection MVT
      parameters:
        - name: collectionId
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/TileJSON'

  /collections/{collectionId}/tiles/{tileMatrixSetId}/{tileMatrix}/{tileRow}/{tileCol}:
    get:
      summary: Récupère une tuile MVT
      parameters:
        - name: collectionId
          in: path
          required: true
          schema:
            type: string
        - name: tileMatrixSetId
          in: path
          required: true
          schema:
            type: string
            example: "WebMercatorQuad"
        - name: tileMatrix
          in: path
          required: true
          schema:
            type: integer
        - name: tileRow
          in: path
          required: true
          schema:
            type: integer
        - name: tileCol
          in: path
          required: true
          schema:
            type: integer
        - name: f
          in: query
          required: false
          schema:
            type: string
            default: "pbf"
      responses:
        '200':
          description: OK
          content:
            application/x-protobuf:
              schema:
                type: string
                format: binary

components:
  schemas:
    TileJSON:
      type: object
      properties:
        tilejson:
          type: string
          example: "3.0.0"
        name:
          type: string
          example: "Bâtiments"
        tiles:
          type: array
          items:
            type: string
            example: "http://localhost:8080/collections/batiments/tiles/WebMercatorQuad/{z}/{x}/{y}.pbf"
        minzoom:
          type: integer
          example: 0
        maxzoom:
          type: integer
          example: 14
        bounds:
          type: array
          items:
            type: number
            example: [-180, -85.0511, 180, 85.0511]
        vector_layers:
          type: array
          items:
            type: object
            properties:
              id:
                type: string
                example: "batiments"
              fields:
                type: object
                example: {"id": "Identifiant unique", "hauteur": "Hauteur en mètres"}
```

***

## **6. Validation et Tests**

### **6.1 Outils de Validation**

- **OGC Validator** :
  - <https://cite.opengeospatial.org/teamengine/>
  - Teste la conformité aux standards OGC API - Tiles.

- **cURL** :
  ```bash
  # Vérifier le Content-Type des tuiles MVT
  curl -I http://localhost:8080/collections/batiments/tiles/WebMercatorQuad/12/1234/5678?f=pbf

  # Vérifier le TileJSON
  curl http://localhost:8080/collections/batiments/tilejson.json | jq
  ```

- **QGIS** :
  - Utiliser le plugin **Vector Tiles Reader** pour tester l'intégration.
  - Charger le `tilejson.json` via *Couche > Ajouter une couche > Ajouter une couche de tuiles vectorielles*.

***

## **7. Ressources et Références**

### **7.1 Spécifications Officielles**

- [OGC API - Tiles (Part 1: Core)](https://docs.ogc.org/is/21-057/21-057.html)
- [Spécification MVT (Mapbox)](https://github.com/mapbox/vector-tile-spec)
- [TileJSON Spec](https://github.com/mapbox/tilejson-spec)

### **7.2 Outils et Bibliothèques (Go)**

- **Protobuf** : [go-protobuf](https://github.com/golang/protobuf) (pour encoder/décoder les MVT).
- **Serveur HTTP** : [gorilla/mux](https://github.com/gorilla/mux) (pour le routage).
- **Validation OpenAPI** : [go-swagger](https://github.com/go-swagger/go-swagger).
- **Base de données** : [DuckDB](https://duckdb.org/) (pour stocker/requêter des données vectorielles).

### **7.3 Exemples de Serveurs Conformes**

- [pygeoapi](https://pygeoapi.io/) (Python, supporte OGC API - Tiles + MVT).
- [deegree](https://www.deegree.org/) (Java, supporte OGC API - Tiles).
- [GeoServer](https://www.geoserver.7. Ressources et Référencesorg/) (Java, extensions pour MVT).

***

## **8. Checklist de Conformité OGC pour MVT**

- [ ] **`/conformance`** : Liste les standards `ogcapi-tiles-1` et `ogcapi-tiles-2`.
- [ ] **`/api`** : Fournit un document OpenAPI 3.0 valide.
- [ ] **`/collections`** : Inclut le type `"vector"` pour les collections MVT.
- [ ] **`/collections/{id}/tilejson.json`** : Fournit un TileJSON valide avec `vector_layers`.
- [ ] **Endpoints de tuiles** : Supporte `f=pbf` et renvoie `Content-Type: application/x-protobuf`.
- [ ] **Système de tuilage** : Utilise `WebMercatorQuad` (EPSG:3857).
- [ ] **Métadonnées** : `bounds` en EPSG:4326, `minzoom`/`maxzoom` cohérents.

***

## **9. Annexes**

### **9.1 Exemple de Schéma Protobuf pour MVT**

```protobuf
// Définition simplifiée du schéma MVT (basé sur vector_tile.proto)
syntax = "proto3";

message Tile {
  repeated Layer layers = 1;
}

message Layer {
  string name = 1;
  repeated Feature features = 2;
  uint32 extent = 3;
}

message Feature {
  uint64 id = 1;
  repeated uint32 tags = 2;
  repeated uint32 geometry = 3;
}
```

### **9.2 Exemple de Requête cURL pour Récupérer une Tuile MVT**

```bash
curl -v -H "Accept: application/x-protobuf" \
  "http://localhost:8080/collections/batiments/tiles/WebMercatorQuad/12/1234/5678?f=pbf" \
  --output batiment_12_1234_5678.pbf
```

### **9.3 Intégration avec QGIS**

1. Installer le plugin **Vector Tiles Reader**.
2. Ajouter une couche via *Couche > Ajouter une couche > Ajouter une couche de tuiles vectorielles*.
3. Entrer l'URL du `tilejson.json` ou l'URL directe des tuiles (ex: `http://localhost:8080/collections/batiments/tiles/WebMercatorQuad/{z}/{x}/{y}.pbf`).Untitled
