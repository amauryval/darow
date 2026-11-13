package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"

	"github.com/MyAppLibs/go-pkg/apperr"
)

const webMercatorOrigin = 20037508.342789244

// TileLayer is the name of the one layer of every vector tile.
const TileLayer = "features"

// Tile returns the vector tile z/x/y of the table's geometry, each feature carrying its row number
// as id; the tile is empty for a table without geometry. At most Limits.Tiles render at once, the
// others waiting until their request ends.
func (e *Engine) Tile(ctx context.Context, name string, z, x, y int, condition string) ([]byte, error) {
	if z < 0 || z > 24 || x < 0 || y < 0 || x >= 1<<z || y >= 1<<z {
		return nil, fmt.Errorf("tile %d/%d/%d: %w", z, x, y, apperr.ErrNotFound)
	}
	select {
	case e.tiles <- struct{}{}:
		defer func() { <-e.tiles }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var tile []byte
	err := e.read(ctx, func(db *sql.DB) error {
		table, err := describe(ctx, db, name)
		if err != nil || table.Geometry == "" {
			return err
		}
		clause, err := where(ctx, db, table, condition)
		if err != nil {
			return err
		}
		query, queryArgs := tileQuery(table, z, x, y, clause)

		return evaluated("where", db.QueryRowContext(ctx, query, queryArgs...).Scan(&tile))
	})

	return tile, err
}

// The envelope is inlined: the spatial index is used only against a constant. Each feature's id is
// its row's __id, the tile's own feature id, which the map reads as is.
func tileQuery(table Table, z, x, y int, clause string) (string, []any) {
	span := 2 * webMercatorOrigin / math.Pow(2, float64(z))
	minX := -webMercatorOrigin + float64(x)*span
	maxY := webMercatorOrigin - float64(y)*span
	envelope := fmt.Sprintf("ST_MakeEnvelope(%v, %v, %v, %v)", minX, maxY-span, minX+span, maxY)
	g := quote(table.Geometry)
	filter := "WHERE ST_Intersects_Extent(" + g + ", " + envelope + ")"
	if clause != "" {
		filter += " AND " + clause[len(" WHERE "):]
	}

	return fmt.Sprintf(`SELECT ST_AsMVT({'geom': geom, 'id': id}, '%s', 4096, 'geom', 'id') FROM (
SELECT TRY(ST_AsMVTGeom(ST_Simplify(%s, ?), ST_Extent(%s))) AS geom, %s AS id FROM %s %s) WHERE geom IS NOT NULL`,
		TileLayer, g, envelope, quote(IDColumn), quote(table.Name), filter), []any{span / 4096}
}

// FeatureQuery is a page of a table's features: those intersecting BBox (in BBoxCRS, CRS84 when
// empty; none when nil), by row number, in CRS (CRS84 when empty), their
// coordinates rounded to Decimals digits.
type FeatureQuery struct {
	BBox     *Extent
	BBoxCRS  string
	Offset   int
	Limit    int
	Decimals int
	CRS      string
}

// Features is a page of features, each a GeoJSON Feature whose id is its row number, and the count
// of features the query matches.
type Features struct {
	Features []json.RawMessage
	Matched  int64
}

// Features returns a page of the table's features; a table without geometry has none.
func (e *Engine) Features(ctx context.Context, name string, q FeatureQuery) (Features, error) {
	page := Features{Features: []json.RawMessage{}}
	err := e.read(ctx, func(db *sql.DB) error {
		table, err := describe(ctx, db, name)
		if err != nil || table.Geometry == "" {
			return err
		}
		clause, args := featureFilter(table, q)
		crs := orCRS84(q.CRS)
		if err := checkCRS(ctx, db, crs); err != nil {
			return err
		}
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+quote(name)+clause, args...).Scan(&page.Matched); err != nil {
			return err
		}
		query := fmt.Sprintf("%s%s ORDER BY %s LIMIT ? OFFSET ?", selectFeatures(table, q.Decimals), clause, quote(IDColumn))
		page.Features, err = readFeatures(ctx, db, query, append(append([]any{crs}, args...), q.Limit, q.Offset)...)

		return err
	})

	return page, err
}

// Feature returns the feature numbered id in crs (CRS84 when empty), its coordinates rounded to
// decimals digits.
func (e *Engine) Feature(ctx context.Context, name string, id int64, decimals int, crs string) (json.RawMessage, error) {
	var feature json.RawMessage
	err := e.read(ctx, func(db *sql.DB) error {
		table, err := describe(ctx, db, name)
		if err != nil {
			return err
		}
		if table.Geometry == "" {
			return fmt.Errorf("table %s has no geometry: %w", name, apperr.ErrNotFound)
		}
		crs = orCRS84(crs)
		if err := checkCRS(ctx, db, crs); err != nil {
			return err
		}
		features, err := readFeatures(ctx, db, selectFeatures(table, decimals)+" WHERE "+quote(IDColumn)+" = ?", crs, id)
		if err != nil {
			return err
		}
		if len(features) == 0 {
			return fmt.Errorf("feature %d: %w", id, apperr.ErrNotFound)
		}
		feature = features[0]

		return nil
	})

	return feature, err
}

func featureFilter(table Table, q FeatureQuery) (string, []any) {
	if q.BBox == nil {
		return "", nil
	}
	box := fmt.Sprintf(" WHERE ST_Intersects(%s, ST_Transform(ST_MakeEnvelope(?, ?, ?, ?), ?, 'EPSG:3857'))", quote(table.Geometry))

	return box, []any{q.BBox[0], q.BBox[1], q.BBox[2], q.BBox[3], orCRS84(q.BBoxCRS)}
}

func selectFeatures(table Table, decimals int) string {
	fields, properties := "", ""
	for _, c := range table.Columns {
		if !isGeometry(c.Type) {
			fields += fmt.Sprintf(", %s: %s", literal(c.Name), quote(c.Name))
		}
	}
	properties = "json_object()"
	if fields != "" {
		properties = "{" + fields[2:] + "}"
	}

	geometry := fmt.Sprintf("ST_ReducePrecision(%s, %v)", toCRS(table.Geometry), math.Pow10(-decimals))

	return fmt.Sprintf(`SELECT %s AS id, ST_AsGeoJSON(%s) AS geometry, %s AS properties FROM %s`,
		quote(IDColumn), geometry, properties, quote(table.Name))
}

func readFeatures(ctx context.Context, q queryer, query string, args ...any) ([]json.RawMessage, error) {
	return readJSON(ctx, q, `SELECT json_object('type', 'Feature', 'id', id, 'geometry', geometry::JSON, 'properties', to_json(properties))::VARCHAR FROM (`+query+`)`, args...)
}
