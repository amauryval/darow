package sources

import "time"

// Source is one file of the directory.
type Source struct {
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

// Extension is a kind of file a source may be.
type Extension string

// The extensions of the sources, any case.
const (
	ExtensionGeoJSON    Extension = ".geojson"
	ExtensionJSON       Extension = ".json"
	ExtensionCSV        Extension = ".csv"
	ExtensionGeoPackage Extension = ".gpkg"
	ExtensionParquet    Extension = ".parquet"
	ExtensionFlatGeobuf Extension = ".fgb"
	ExtensionKML        Extension = ".kml"
)
