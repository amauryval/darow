//go:build apidoc

// The swag blocks of the routes repository/resource serves for the saved filters, read by
// `mise run generate-api-doc` and never compiled.
package rest

import "github.com/amauryval/darow/internal/darow/filters"

// docListFilter list the saved filters.
//
//	@Summary	List the saved filters
//	@Tags	darow
//	@Produce	json
//	@Success	200	{array}	filters.Filter
//	@x-access	"public"
//	@Router	/darow/filters [get]
func docListFilter() {}

// docCreateFilter save a filter.
//
//	@Summary	Save a filter
//	@Tags	darow
//	@Param	body	body	filters.Filter	true	"The filter, without id"
//	@Accept	json
//	@Produce	json
//	@Success	201	{object}	filters.Filter
//	@Failure	400	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/filters [post]
func docCreateFilter() {}

// docGetFilter read a saved filter.
//
//	@Summary	Read a saved filter
//	@Tags	darow
//	@Param	id	path	string	true	"Filter id"
//	@Produce	json
//	@Success	200	{object}	filters.Filter
//	@Failure	404	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/filters/{id} [get]
func docGetFilter() {}

// docReplaceFilter replace a saved filter.
//
//	@Summary	Replace a saved filter
//	@Tags	darow
//	@Param	id	path	string	true	"Filter id"
//	@Param	body	body	filters.Filter	true	"The whole filter"
//	@Accept	json
//	@Produce	json
//	@Success	200	{object}	filters.Filter
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/filters/{id} [put]
func docReplaceFilter() {}

// docDeleteFilter delete a saved filter.
//
//	@Summary	Delete a saved filter
//	@Tags	darow
//	@Param	id	path	string	true	"Filter id"
//	@Success	204
//	@Failure	404	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/filters/{id} [delete]
func docDeleteFilter() {}
