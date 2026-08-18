package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"darow/internal/settings"
)

// paginationParams reads the offset/limit query params, sizing limit through
// the already-resolved settings.Limit for the family of pages being read.
// strict rejects a malformed or out-of-range value with an error instead of
// falling back to it silently — what OGC API - Features' Core requires and
// every other caller does not.
func paginationParams(req *http.Request, limit settings.Limit, strict bool) (offset, size int64, err error) {
	if raw := req.URL.Query().Get("offset"); raw != "" {
		v, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil || v < 0 {
			if strict {
				return 0, 0, fmt.Errorf("invalid offset: %q", raw)
			}
		} else {
			offset = v
		}
	}

	raw := req.URL.Query().Get("limit")
	requested, perr := strconv.ParseInt(raw, 10, 64)
	if strict && raw != "" && (perr != nil || requested < 1) {
		return 0, 0, fmt.Errorf("invalid limit: %q", raw)
	}

	return offset, limit.Resolve64(requested), nil
}
