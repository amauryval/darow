package rest

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/MyAppLibs/go-pkg/apperr"
	"github.com/MyAppLibs/go-pkg/httpx"

	"github.com/amauryval/darow/internal/darow/engine"
	"github.com/amauryval/darow/internal/darow/settings"
)

// ogcBase is the absolute address of the OGC API, read from the host and scheme the client asked for.
func ogcBase(r *http.Request) string {
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}

	return scheme + "://" + host + "/api/v1/darow/ogc"
}

// ogcQuery reads the parameters of an items request, refusing an unknown or malformed one.
func ogcQuery(r *http.Request, effective settings.Effective) (engine.FeatureQuery, error) {
	values := r.URL.Query()
	for name := range values {
		if !ogcItemParams[name] {
			return engine.FeatureQuery{}, fmt.Errorf("unknown parameter %s", name)
		}
	}
	limit, err := ogcInt(values.Get("limit"), effective.FeaturesPageSize, 1)
	if err != nil {
		return engine.FeatureQuery{}, fmt.Errorf("limit: %w", err)
	}
	offset, err := ogcInt(values.Get("offset"), 0, 0)
	if err != nil {
		return engine.FeatureQuery{}, fmt.Errorf("offset: %w", err)
	}
	bbox, err := ogcBBox(values.Get("bbox"))
	if err != nil {
		return engine.FeatureQuery{}, fmt.Errorf("bbox: %w", err)
	}
	bboxCRS, err := ogcCRS(values.Get("bbox-crs"))
	if err != nil {
		return engine.FeatureQuery{}, fmt.Errorf("bbox-crs: %w", err)
	}
	crs, err := ogcCRS(values.Get("crs"))
	if err != nil {
		return engine.FeatureQuery{}, fmt.Errorf("crs: %w", err)
	}

	return engine.FeatureQuery{
		BBox: bbox, BBoxCRS: bboxCRS, Offset: offset, Limit: min(limit, ogcMaxLimit),
		Decimals: effective.CoordinateDecimals, CRS: crs,
	}, nil
}

// ogcCRS reads a CRS URI as the engine's name of it: CRS84 when empty, EPSG:<code> in the
// authority's axis order.
func ogcCRS(uri string) (string, error) {
	if uri == "" || uri == ogcCRS84 {
		return engine.CRS84, nil
	}
	code, ok := strings.CutPrefix(uri, ogcEPSG)
	if n, err := strconv.Atoi(code); !ok || err != nil || n <= 0 {
		return "", errors.New("a CRS84 or EPSG URI expected")
	}

	return "EPSG:" + code, nil
}

// ogcCRSURI is the URI of the CRS asked for, CRS84 when none.
func ogcCRSURI(uri string) string {
	if uri == "" {
		return ogcCRS84
	}

	return uri
}

// ogcFailure answers a CRS the engine cannot project to as a 400, any other failure as usual.
func ogcFailure(w http.ResponseWriter, op string, err error) {
	if errors.Is(err, apperr.ErrInvalid) {
		httpx.WriteProblem(w, http.StatusBadRequest, apperr.CodeInvalid, "crs: unknown")
		return
	}
	httpx.FromError(w, op, err)
}

func ogcInt(raw string, fallback, minimum int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum {
		return 0, fmt.Errorf("an integer of at least %d expected", minimum)
	}

	return value, nil
}

// ogcBBox reads four or six numbers; the heights of six are ignored.
func ogcBBox(raw string) (*engine.Extent, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 && len(parts) != 6 {
		return nil, fmt.Errorf("4 or 6 numbers expected")
	}
	numbers := make([]float64, len(parts))
	for i, part := range parts {
		n, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, fmt.Errorf("number expected")
		}
		numbers[i] = n
	}
	if len(numbers) == 6 {
		numbers = []float64{numbers[0], numbers[1], numbers[3], numbers[4]}
	}

	return &engine.Extent{numbers[0], numbers[1], numbers[2], numbers[3]}, nil
}

// pageLinks are the self link of a page and, when there are, the links of the pages around it.
func pageLinks(items string, r *http.Request, q engine.FeatureQuery, matched int64) []Link {
	links := []Link{{Href: pageHref(items, r, q.Offset), Rel: "self", Type: ogcGeoJSON, Title: "This page"}}
	if int64(q.Offset+q.Limit) < matched {
		links = append(links, Link{Href: pageHref(items, r, q.Offset+q.Limit), Rel: "next", Type: ogcGeoJSON, Title: "Next page"})
	}
	if q.Offset > 0 {
		links = append(links, Link{Href: pageHref(items, r, max(q.Offset-q.Limit, 0)), Rel: "prev", Type: ogcGeoJSON, Title: "Previous page"})
	}

	return links
}

func pageHref(items string, r *http.Request, offset int) string {
	values := url.Values{}
	for name, value := range r.URL.Query() {
		values[name] = value
	}
	values.Set("offset", strconv.Itoa(offset))

	return items + "?" + values.Encode()
}
