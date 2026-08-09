# OGC API - Features (Part 1: Core) - Summary for Go Implementation with Chi

> **Objective**: This document summarizes the **OGC API - Features (Core)** specifications for implementation in **Go** using the **Chi router**. It details the **endpoints**, **input parameters**, **output structures**, and provides code examples to get started quickly.

***

## 📌 Table of Contents

1. [Introduction](#-introduction)
2. [Resources and Endpoints](#-resources-and-endpoints)
3. [Inputs (Requests and Parameters)](#-inputs-requests-and-parameters)
4. [Outputs (Responses and Structures)](#-outputs-responses-and-structures)
5. [Go Implementation Example with Chi](#-go-implementation-example-with-chi)
6. [Key Implementation Points](#-key-implementation-points)
7. [Useful Links](#-useful-links)

***

## 📖 Introduction

### What is OGC API - Features?

**OGC API - Features** is a standard by the **Open Geospatial Consortium (OGC)** that defines a **standardized Web API** for accessing **geospatial features** (real-world objects like buildings, rivers, etc.).

- **HTTP/HTTPS Based**: Uses HTTP methods (GET for Core).
- **Modular**: The **Core** (Part 1) defines basic functionalities. Other parts extend these functionalities (e.g., write operations, complex queries).
- **Supported Encodings**: GeoJSON (recommended), HTML, GML (Simple Features Profile).
- **Coordinate Reference System (CRS)**: By default, **WGS 84 (EPSG:4326)** with `longitude/latitude` order.

### Use Cases

- **Discovery**: Allows clients to discover available **feature collections** and their metadata.
- **Querying**: Allows retrieving **features** based on simple criteria (e.g., bounding box, date, result limit).

***

## 🌐 Resources and Endpoints

The API exposes several **resources** accessible via **standardized endpoints**. Each endpoint returns structured data in **JSON** (or other encodings like GeoJSON).

### Endpoints Table

| **Resource**                | **Path**                                        | **HTTP Method** | **Description**                                                                                 |
| :-------------------------- | :---------------------------------------------- | :-------------- | :---------------------------------------------------------------------------------------------- |
| **Landing Page**            | `/`                                             | GET             | API homepage. Contains links to other resources.                                                |
| **Conformance Declaration** | `/conformance`                                  | GET             | List of conformance classes implemented (e.g., Core, GeoJSON).                                  |
| **Feature Collections**     | `/collections`                                  | GET             | List of available feature collections.                                                          |
| **Feature Collection**      | `/collections/{collectionId}`                   | GET             | Metadata for a specific collection (e.g., name, description, spatial extent).                   |
| **Features**                | `/collections/{collectionId}/items`             | GET             | List of features in a collection, with support for filters (`bbox`, `datetime`, `limit`, etc.). |
| **Feature**                 | `/collections/{collectionId}/items/{featureId}` | GET             | Details of a specific feature (identified by `featureId`).                                      |

***

## 📥 Inputs (Requests and Parameters)

**GET** requests can include **filter parameters** to refine results. Below are the **standardized parameters** for the `/collections/{collectionId}/items` endpoint.

### 1. Common Parameters

| **Parameter** | **Type**  | **Description**                                                                              | **Example**                                               |
| :------------ | :-------- | :------------------------------------------------------------------------------------------- | :-------------------------------------------------------- |
| `bbox`        | `string`  | Bounding box (`minx,miny,maxx,maxy`) in **WGS 84 (longitude/latitude)**. Separators: commas. | `bbox=-180,-90,180,90`                                    |
| `datetime`    | `string`  | Temporal filter (ISO 8601). Can be a timestamp or a range.                                   | `datetime=2026-08-08` or `datetime=2026-01-01/2026-12-31` |
| `limit`       | `integer` | Maximum number of features to return (default: 10).                                          | `limit=20`                                                |
| `offset`      | `integer` | Offset for pagination (default: 0).                                                          | `offset=10`                                               |
| `filter`      | `string`  | Filter on feature properties (syntax: **CQL2**).                                             | `filter=population>1000000`                               |

### 2. Parameter Combinations

- Parameters can be **combined** (e.g., `bbox=-180,-90,180,90&datetime=2026-08-08&limit=50`).
- Parameter order **does not matter**.
- If a parameter is **invalid**, the API must return an **HTTP 400 error**.

***

## 📤 Outputs (Responses and Structures)

Responses are **always** in **JSON** format (or GeoJSON, HTML, GML depending on supported encoding). Below are the basic structures for each endpoint.

***

### 1. Landing Page (`/`)

**Format**: JSON

```json
{
  "title": "OGC API - Features",
  "description": "A Go implementation of the OGC Features API using Chi.",
  "version": "1.0",
  "links": [
    {
      "href": "/collections",
      "rel": "collections",
      "type": "application/json",
      "title": "Feature Collections"
    },
    {
      "href": "/conformance",
      "rel": "conformance",
      "type": "application/json",
      "title": "Conformance Declaration"
    },
    {
      "href": "/api",
      "rel": "service-desc",
      "type": "application/vnd.oai.openapi+json;version=3.0",
      "title": "API Definition"
    }
  ]
}
```

**Go Structure**:

```go
type LandingPage struct {
    Title       string `json:"title"`
    Description string `json:"description"`
    Version     string `json:"version"`
    Links       []Link `json:"links"`
}

type Link struct {
    Href  string `json:"href"`
    Rel   string `json:"rel"`
    Type  string `json:"type,omitempty"`
    Title string `json:"title,omitempty"`
}
```

***

### 2. Conformance Declaration (`/conformance`)

**Format**: JSON

```json
{
  "conformsTo": [
    "http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core",
    "http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson"
  ]
}
```

**Go Structure**:

```go
type ConformanceDeclaration struct {
    ConformsTo []string `json:"conformsTo"`
}
```

***

### 3. Feature Collections (`/collections`)

**Format**: JSON

```json
{
  "collections": [
    {
      "id": "buildings",
      "title": "Buildings",
      "description": "A collection of buildings in France.",
      "extent": {
        "spatial": [-5.14, 41.36, 9.56, 51.09],
        "temporal": [
          {"begin": "2020-01-01T00:00:00Z", "end": "2026-08-08T00:00:00Z"}
        ]
      },
      "links": [
        {"href": "/collections/buildings", "rel": "self", "type": "application/json"},
        {"href": "/collections/buildings/items", "rel": "items", "type": "application/geo+json"}
      ]
    }
  ],
  "links": []
}
```

**Go Structure**:

```go
type FeatureCollections struct {
    Collections []Collection `json:"collections"`
    Links       []Link       `json:"links"`
}

type Collection struct {
    ID          string `json:"id"`
    Title       string `json:"title"`
    Description string `json:"description"`
    Extent      Extent `json:"extent"`
    Links       []Link `json:"links"`
}

type Extent struct {
    Spatial  []float64 `json:"spatial"` // [minx, miny, maxx, maxy]
    Temporal []Time   `json:"temporal,omitempty"`
}

type Time struct {
    Begin string `json:"begin,omitempty"`
    End   string `json:"end,omitempty"`
}
```

***

### 4. Feature Collection (`/collections/{collectionId}`)

**Format**: JSON

```json
{
  "id": "buildings",
  "title": "Buildings",
  "description": "A collection of buildings in France.",
  "extent": {
    "spatial": [-5.14, 41.36, 9.56, 51.09]
  },
  "links": [
    {"href": "/collections/buildings", "rel": "self", "type": "application/json"},
    {"href": "/collections/buildings/items", "rel": "items", "type": "application/geo+json"}
  ]
}
```

**Go Structure**:
Uses the `Collection` structure defined above.

***

### 5. Features (`/collections/{collectionId}/items`)

**Format**: **GeoJSON** (recommended)

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "id": "1",
      "geometry": {
        "type": "Point",
        "coordinates": [2.3522, 48.8566]
      },
      "properties": {
        "name": "Eiffel Tower",
        "height": 330
      },
      "links": []
    }
  ],
  "links": [
    {"href": "/collections/buildings/items?limit=10", "rel": "self", "type": "application/geo+json"}
  ],
  "numberMatched": 1
}
```

**Go Structure**:

```go
type FeatureCollection struct {
    Type         string    `json:"type"`
    Features     []Feature `json:"features"`
    Links        []Link    `json:"links"`
    NumberMatched int       `json:"numberMatched"`
}

type Feature struct {
    Type       string                 `json:"type"`
    ID         string                 `json:"id"`
    Geometry   *Geometry              `json:"geometry"`
    Properties map[string]interface{} `json:"properties"`
    Links      []Link                 `json:"links,omitempty"`
}

type Geometry struct {
    Type        string      `json:"type"`
    Coordinates interface{} `json:"coordinates"`
}
```

***

### 6. Feature (`/collections/{collectionId}/items/{featureId}`)

**Format**: **GeoJSON**

```json
{
  "type": "Feature",
  "id": "1",
  "geometry": {
    "type": "Point",
    "coordinates": [2.3522, 48.8566]
  },
  "properties": {
    "name": "Eiffel Tower",
    "height": 330
  },
  "links": []
}
```

**Go Structure**:
Uses the `Feature` structure defined above.

***

## 💻 Go Implementation Example with Chi

Here’s a **minimal example** to implement the API in Go using the [`Chi`](https://github.com/go-chi/chi) router.

### 1. Dependencies

Install `chi`:

```bash
go get -u github.com/go-chi/chi/v5
```

***

### 2. Full Code

```go
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// --- Structures ---

type LandingPage struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Links       []Link `json:"links"`
}

type Link struct {
	Href  string `json:"href"`
	Rel   string `json:"rel"`
	Type  string `json:"type,omitempty"`
	Title string `json:"title,omitempty"`
}

type ConformanceDeclaration struct {
	ConformsTo []string `json:"conformsTo"`
}

type FeatureCollections struct {
	Collections []Collection `json:"collections"`
	Links       []Link       `json:"links"`
}

type Collection struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Extent      Extent `json:"extent"`
	Links       []Link `json:"links"`
}

type Extent struct {
	Spatial  []float64 `json:"spatial"`
	Temporal []Time   `json:"temporal,omitempty"`
}

type Time struct {
	Begin string `json:"begin,omitempty"`
	End   string `json:"end,omitempty"`
}

type FeatureCollection struct {
	Type         string    `json:"type"`
	Features     []Feature `json:"features"`
	Links        []Link    `json:"links"`
	NumberMatched int       `json:"numberMatched"`
}

type Feature struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Geometry   *Geometry              `json:"geometry"`
	Properties map[string]interface{} `json:"properties"`
	Links      []Link                 `json:"links,omitempty"`
}

type Geometry struct {
	Type        string      `json:"type"`
	Coordinates interface{} `json:"coordinates"`
}

// --- Helper Function to Send JSON Responses ---
func respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

// --- Handlers ---

func getLandingPage(w http.ResponseWriter, r *http.Request) {
	landingPage := LandingPage{
		Title:       "OGC API - Features",
		Description: "A Go implementation of the OGC Features API using Chi.",
		Version:     "1.0",
		Links: []Link{
			{Href: "/collections", Rel: "collections", Type: "application/json", Title: "Feature Collections"},
			{Href: "/conformance", Rel: "conformance", Type: "application/json", Title: "Conformance Declaration"},
			{Href: "/api", Rel: "service-desc", Type: "application/vnd.oai.openapi+json;version=3.0", Title: "API Definition"},
		},
	}
	respondWithJSON(w, http.StatusOK, landingPage)
}

func getConformance(w http.ResponseWriter, r *http.Request) {
	conformance := ConformanceDeclaration{
		ConformsTo: []string{
			"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core",
			"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson",
		},
	}
	respondWithJSON(w, http.StatusOK, conformance)
}

func getCollections(w http.ResponseWriter, r *http.Request) {
	collections := []Collection{
		{
			ID:          "buildings",
			Title:       "Buildings",
			Description: "A collection of buildings in France.",
			Extent: Extent{
				Spatial: []float64{-5.14, 41.36, 9.56, 51.09}, // Metropolitan France
			},
			Links: []Link{
				{Href: "/collections/buildings", Rel: "self", Type: "application/json"},
				{Href: "/collections/buildings/items", Rel: "items", Type: "application/geo+json"},
			},
		},
	}
	respondWithJSON(w, http.StatusOK, FeatureCollections{Collections: collections})
}

func getCollection(w http.ResponseWriter, r *http.Request) {
	collectionID := chi.URLParam(r, "collectionId")
	// Simplified example: return a static collection
	collection := Collection{
		ID:          collectionID,
		Title:       "Buildings",
		Description: "A collection of buildings in France.",
		Extent: Extent{
			Spatial: []float64{-5.14, 41.36, 9.56, 51.09},
		},
		Links: []Link{
			{Href: "/collections/" + collectionID, Rel: "self", Type: "application/json"},
			{Href: "/collections/" + collectionID + "/items", Rel: "items", Type: "application/geo+json"},
		},
	}
	respondWithJSON(w, http.StatusOK, collection)
}

func getFeatures(w http.ResponseWriter, r *http.Request) {
	collectionID := chi.URLParam(r, "collectionId")
	// Retrieve query parameters
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "10"
	}
	bbox := r.URL.Query().Get("bbox")
	datetime := r.URL.Query().Get("datetime")
	// offset := r.URL.Query().Get("offset")
	// filter := r.URL.Query().Get("filter")

	// Example: Return static features
	features := []Feature{
		{
			Type: "Feature",
			ID:   "1",
			Geometry: &Geometry{
				Type:        "Point",
				Coordinates: []float64{2.3522, 48.8566}, // Paris
			},
			Properties: map[string]interface{}{
				"name": "Eiffel Tower",
				"height": 330,
			},
			Links: []Link{
				{Href: "/collections/" + collectionID + "/items/1", Rel: "self", Type: "application/geo+json"},
			},
		},
		{
			Type: "Feature",
			ID:   "2",
			Geometry: &Geometry{
				Type:        "Point",
				Coordinates: []float64{2.2945, 48.8584}, // Notre-Dame de Paris
			},
			Properties: map[string]interface{}{
				"name": "Notre-Dame de Paris",
				"height": 69,
			},
		},
	}

	featureCollection := FeatureCollection{
		Type:     "FeatureCollection",
		Features: features,
		Links: []Link{
			{Href: "/collections/" + collectionID + "/items?limit=" + limit, Rel: "self", Type: "application/geo+json"},
		},
		NumberMatched: len(features),
	}
	respondWithJSON(w, http.StatusOK, featureCollection)
}

func getFeature(w http.ResponseWriter, r *http.Request) {
	collectionID := chi.URLParam(r, "collectionId")
	featureID := chi.URLParam(r, "featureId")

	// Example: Return a static feature
	feature := Feature{
		Type: "Feature",
		ID:   featureID,
		Geometry: &Geometry{
			Type:        "Point",
			Coordinates: []float64{2.3522, 48.8566},
		},
		Properties: map[string]interface{}{
			"name": "Eiffel Tower",
			"height": 330,
		},
		Links: []Link{
			{Href: "/collections/" + collectionID + "/items/" + featureID, Rel: "self", Type: "application/geo+json"},
		},
	}
	respondWithJSON(w, http.StatusOK, feature)
}

// --- Main ---

func main() {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Endpoints
	r.Get("/", getLandingPage)
	r.Get("/conformance", getConformance)
	r.Get("/collections", getCollections)
	r.Get("/collections/{collectionId}", getCollection)
	r.Get("/collections/{collectionId}/items", getFeatures)
	r.Get("/collections/{collectionId}/items/{featureId}", getFeature)

	// Start the server
	log.Println("Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", r))
}
```

***

## 🔑 Key Implementation Points

### 1. Parameter Handling

- **Parse** parameters like `bbox`, `datetime`, `limit`, `filter` (CQL2) from requests.
- **Validate** parameters (e.g., `bbox` must have 4 numeric values).
- **Combine** filters to return the correct features.

### 2. Supported Encodings

- **GeoJSON** (recommended for features).
- **HTML** (for documentation pages).
- **GML** (optional, for traditional GIS clients).

### 3. Error Handling

Return appropriate **HTTP status codes**:

- `400 Bad Request`: Invalid parameters.
- `404 Not Found`: Resource not found (e.g., non-existent collection or feature).
- `500 Internal Server Error`: Server error.

### 4. Pagination

- Use `limit` and `offset` to handle large collections.
- Include `next`/`prev` links in responses.

### 5. Metadata

- Add **links** (`links`) in all responses for navigation.
- Include `numberMatched` in `FeatureCollection` to indicate the total number of results.

### 6. Coordinate Reference System (CRS)

- By default, geometries must be in **WGS 84 (EPSG:4326)** with `longitude/latitude` order.
- If other CRS are supported, indicate them in the collection metadata.

### 7. OpenAPI

- Provide an **OpenAPI 3.0 definition** of the API (endpoint `/api`).
- Use tools like [`swaggo/swag`](https://github.com/swaggo/swag) to generate OpenAPI documentation in Go.

***

## 🔗 Useful Links

- **Full Specification**: [OGC API - Features - Part 1: Core](https://docs.ogc.org/is/17-069r3/17-069r3.html)
- **GeoJSON**: [RFC 7946](https://tools.ietf.org/html/rfc7946)
- **OpenAPI 3.0**: [OpenAPI Specification](https://spec.openapis.org/oas/v3.0.2)
- **CQL2 (Common Query Language)**: [OGC CQL2](https://docs.ogc.org/DRAFTS/21-065.html)
- **Chi Router**: [Chi GitHub](https://github.com/go-chi/chi)

## 📝 Additional Notes

- This document is based on **version 1.0** of the **OGC API - Features - Part 1: Core** standard (published October 14, 2019).
- For a complete implementation, refer to the [official specification](https://docs.ogc.org/is/17-069r3/17-069r3.html).
- The code examples are **simplified** and should be adapted to your use case (e.g., database connection, advanced error handling).
