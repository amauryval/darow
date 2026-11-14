package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/MyAppLibs/go-pkg/apperr"
	"github.com/MyAppLibs/go-pkg/httpx"

	"github.com/amauryval/darow/docs"
	"github.com/amauryval/darow/internal/darow/engine"
)

// The OGC API - Features constants: the CRS URIs, the media types, the largest page.
const (
	ogcCRS84    = "http://www.opengis.net/def/crs/OGC/1.3/CRS84"
	ogcEPSG     = "http://www.opengis.net/def/crs/EPSG/0/"
	ogcStorage  = ogcEPSG + "3857"
	ogcGeoJSON  = "application/geo+json"
	ogcJSON     = "application/json"
	ogcMaxLimit = 10000
)

var ogcConformance = []string{
	"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core",
	"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson",
	"http://www.opengis.net/spec/ogcapi-features-2/1.0/conf/crs",
}

// ogcCRSList is the CRSs a collection names; any other EPSG code is served all the same.
var ogcCRSList = []string{ogcCRS84, ogcEPSG + "4326", ogcStorage}

var ogcItemParams = map[string]bool{"limit": true, "offset": true, "bbox": true, "bbox-crs": true, "crs": true, "datetime": true, "f": true}

// Link is a link of an OGC API document.
type Link struct {
	Href  string `json:"href"`
	Rel   string `json:"rel"`
	Type  string `json:"type,omitempty"`
	Title string `json:"title,omitempty"`
}

// Landing is the landing page of the OGC API.
type Landing struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Links       []Link `json:"links"`
}

// Conformance lists the conformance classes of the OGC API.
type Conformance struct {
	ConformsTo []string `json:"conformsTo"`
}

// Collection is a table with a geometry, as an OGC API collection.
type Collection struct {
	ID         string            `json:"id"`
	Title      string            `json:"title"`
	ItemType   string            `json:"itemType"`
	CRS        []string          `json:"crs"`
	StorageCRS string            `json:"storageCrs"`
	Extent     *CollectionExtent `json:"extent,omitempty"`
	Links      []Link            `json:"links"`
}

// CollectionExtent is the spatial extent of a collection.
type CollectionExtent struct {
	Spatial SpatialExtent `json:"spatial"`
}

// SpatialExtent is one box in CRS84.
type SpatialExtent struct {
	BBox [][4]float64 `json:"bbox"`
	CRS  string       `json:"crs"`
}

// Collections lists the collections.
type Collections struct {
	Links       []Link       `json:"links"`
	Collections []Collection `json:"collections"`
}

// FeatureCollection is a page of a collection's items.
type FeatureCollection struct {
	Type           string            `json:"type"`
	Features       []json.RawMessage `json:"features" swaggertype:"array,object"`
	TimeStamp      string            `json:"timeStamp"`
	NumberMatched  int64             `json:"numberMatched"`
	NumberReturned int               `json:"numberReturned"`
	Links          []Link            `json:"links"`
}

func (s *Server) registerOGC(r chi.Router) {
	r.Get("/", s.ogcLanding)
	r.Get("/api", s.ogcAPI)
	r.Get("/conformance", s.ogcConformance)
	r.Get("/collections", s.ogcCollections)
	r.Get("/collections/{table}", s.ogcCollection)
	r.Get("/collections/{table}/items", s.ogcItems)
	r.Get("/collections/{table}/items/{id}", s.ogcItem)
}

// ogcLanding serves the landing page of the OGC API - Features service.
//
//	@Summary	OGC API - Features landing page
//	@Tags	darow
//	@Produce	json
//	@Success	200	{object}	Landing
//	@x-access	"public"
//	@Router	/darow/ogc [get]
func (s *Server) ogcLanding(w http.ResponseWriter, r *http.Request) {
	base := ogcBase(r)
	_ = httpx.EncodeJSONOK(w, Landing{
		Title:       "darow",
		Description: "The tables of darow as OGC API - Features collections",
		Links: []Link{
			{Href: base, Rel: "self", Type: ogcJSON, Title: "This document"},
			{Href: base + "/api", Rel: "service-desc", Type: ogcJSON, Title: "The API definition"},
			{Href: base + "/conformance", Rel: "conformance", Type: ogcJSON, Title: "Conformance classes"},
			{Href: base + "/collections", Rel: "data", Type: ogcJSON, Title: "Collections"},
		},
	})
}

// ogcAPI serves the API definition the landing page links to, public for the clients (QGIS) that
// read it before the collections.
//
//	@Summary	OGC API - Features API definition
//	@Tags	darow
//	@Produce	json
//	@Success	200	{object}	object
//	@x-access	"public"
//	@Router	/darow/ogc/api [get]
func (s *Server) ogcAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", ogcJSON)
	_, _ = w.Write(docs.Docs().API)
}

// ogcConformance lists the conformance classes.
//
//	@Summary	OGC API - Features conformance classes
//	@Tags	darow
//	@Produce	json
//	@Success	200	{object}	Conformance
//	@x-access	"public"
//	@Router	/darow/ogc/conformance [get]
func (s *Server) ogcConformance(w http.ResponseWriter, _ *http.Request) {
	_ = httpx.EncodeJSONOK(w, Conformance{ConformsTo: ogcConformance})
}

// ogcCollections lists the tables having a geometry as collections.
//
//	@Summary	OGC API - Features collections
//	@Tags	darow
//	@Produce	json
//	@Success	200	{object}	Collections
//	@x-access	"public"
//	@Router	/darow/ogc/collections [get]
func (s *Server) ogcCollections(w http.ResponseWriter, r *http.Request) {
	tables, err := s.engine.List(r.Context())
	if err != nil {
		httpx.FromError(w, "list collections", err)
		return
	}
	base := ogcBase(r)
	collections := Collections{Links: []Link{{Href: base + "/collections", Rel: "self", Type: ogcJSON}}, Collections: []Collection{}}
	for _, table := range tables {
		if table.Geometry == "" {
			continue
		}
		collection, err := s.collection(r.Context(), base, table)
		if err != nil {
			httpx.FromError(w, "list collections", err)
			return
		}
		collections.Collections = append(collections.Collections, collection)
	}
	_ = httpx.EncodeJSONOK(w, collections)
}

// ogcCollection describes one collection.
//
//	@Summary	OGC API - Features collection
//	@Tags	darow
//	@Param	table	path	string	true	"Collection id: the table's name"
//	@Produce	json
//	@Success	200	{object}	Collection
//	@Failure	404	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/ogc/collections/{table} [get]
func (s *Server) ogcCollection(w http.ResponseWriter, r *http.Request) {
	table, err := s.geoTable(r)
	if err != nil {
		httpx.FromError(w, "read collection", err)
		return
	}
	collection, err := s.collection(r.Context(), ogcBase(r), table)
	if err != nil {
		httpx.FromError(w, "read collection", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, collection)
}

// ogcItems serves a page of a collection's features.
//
//	@Summary	OGC API - Features items
//	@Tags	darow
//	@Param	table	path	string	true	"Collection id: the table's name"
//	@Param	limit	query	int	false	"Features served, the table's featuresPageSize by default, at most 10000"
//	@Param	offset	query	int	false	"Features skipped"
//	@Param	bbox	query	string	false	"west,south,east,north in CRS84, or in bbox-crs and its axis order"
//	@Param	bbox-crs	query	string	false	"CRS URI of bbox, CRS84 by default"
//	@Param	crs	query	string	false	"CRS URI of the features, CRS84 by default"
//	@Param	datetime	query	string	false	"Accepted; the collections have no temporal property"
//	@Produce	application/geo+json
//	@Success	200	{object}	FeatureCollection
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/ogc/collections/{table}/items [get]
func (s *Server) ogcItems(w http.ResponseWriter, r *http.Request) {
	table, err := s.geoTable(r)
	if err != nil {
		httpx.FromError(w, "list items", err)
		return
	}
	effective, err := s.effective(r, table.Name)
	if err != nil {
		httpx.FromError(w, "list items", err)
		return
	}
	query, err := ogcQuery(r, effective)
	if err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, apperr.CodeInvalid, err.Error())
		return
	}
	page, err := s.engine.Features(r.Context(), table.Name, query)
	if err != nil {
		ogcFailure(w, "list items", err)
		return
	}
	w.Header().Set("Content-Type", ogcGeoJSON)
	w.Header().Set("Content-Crs", "<"+ogcCRSURI(r.URL.Query().Get("crs"))+">")
	_ = json.NewEncoder(w).Encode(FeatureCollection{
		Type: "FeatureCollection", Features: page.Features, TimeStamp: time.Now().UTC().Format(time.RFC3339),
		NumberMatched: page.Matched, NumberReturned: len(page.Features),
		Links: pageLinks(ogcBase(r)+"/collections/"+table.Name+"/items", r, query, page.Matched),
	})
}

// ogcItem serves one feature of a collection.
//
//	@Summary	OGC API - Features item
//	@Tags	darow
//	@Param	table	path	string	true	"Collection id: the table's name"
//	@Param	id	path	int	true	"Feature id: the row number"
//	@Param	crs	query	string	false	"CRS URI of the feature, CRS84 by default"
//	@Produce	application/geo+json
//	@Success	200	{object}	object
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/ogc/collections/{table}/items/{id} [get]
func (s *Server) ogcItem(w http.ResponseWriter, r *http.Request) {
	id, err := rowID(r)
	if err != nil {
		httpx.FromError(w, "read item", err)
		return
	}
	name := chi.URLParam(r, "table")
	effective, err := s.effective(r, name)
	if err != nil {
		httpx.FromError(w, "read item", err)
		return
	}
	crs, err := ogcCRS(r.URL.Query().Get("crs"))
	if err != nil {
		httpx.WriteProblem(w, http.StatusBadRequest, apperr.CodeInvalid, "crs: "+err.Error())
		return
	}
	raw, err := s.engine.Feature(r.Context(), name, id, effective.CoordinateDecimals, crs)
	if err != nil {
		ogcFailure(w, "read item", err)
		return
	}
	var feature map[string]any
	if err := json.Unmarshal(raw, &feature); err != nil {
		httpx.FromError(w, "read item", err)
		return
	}
	items := ogcBase(r) + "/collections/" + name
	feature["links"] = []Link{
		{Href: items + "/items/" + strconv.FormatInt(id, 10), Rel: "self", Type: ogcGeoJSON},
		{Href: items, Rel: "collection", Type: ogcJSON},
	}
	w.Header().Set("Content-Type", ogcGeoJSON)
	w.Header().Set("Content-Crs", "<"+ogcCRSURI(r.URL.Query().Get("crs"))+">")
	_ = json.NewEncoder(w).Encode(feature)
}

func (s *Server) geoTable(r *http.Request) (engine.Table, error) {
	table, err := s.engine.Describe(r.Context(), chi.URLParam(r, "table"))
	if err != nil {
		return engine.Table{}, err
	}
	if table.Geometry == "" {
		return engine.Table{}, apperr.ErrNotFound
	}

	return table, nil
}

func (s *Server) collection(ctx context.Context, base string, table engine.Table) (Collection, error) {
	extent, err := s.engine.Extent(ctx, table.Name)
	if err != nil {
		return Collection{}, err
	}
	self := base + "/collections/" + table.Name
	collection := Collection{
		ID: table.Name, Title: table.Name, ItemType: "feature", CRS: ogcCRSList, StorageCRS: ogcStorage,
		Links: []Link{
			{Href: self, Rel: "self", Type: ogcJSON},
			{Href: self + "/items", Rel: "items", Type: ogcGeoJSON},
		},
	}
	if extent != nil {
		collection.Extent = &CollectionExtent{Spatial: SpatialExtent{BBox: [][4]float64{*extent}, CRS: ogcCRS84}}
	}

	return collection, nil
}
