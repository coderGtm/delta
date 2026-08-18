package httpapi

import (
	"math"
	"net/http"
	"strconv"
	"strings"
)

// Pagination caps: maxSize bounds the items-per-page, and maxPage keeps the
// page*size offset within the int32 range used for SQL LIMIT/OFFSET values.
const (
	maxSize = 10000
	maxPage = math.MaxInt32 / maxSize
)

// SortOrder is a single requested sort: a field name and whether it should be
// applied in descending order.
type SortOrder struct {
	Field string // Requested sort field.
	Desc  bool   // Whether to sort in descending order.
}

// PageParams holds the parsed pagination and sort parameters of a request.
type PageParams struct {
	Page   int         // Zero-based page number.
	Size   int         // Number of items per page.
	Sorted bool        // Whether at least one sort field was requested.
	Sort   []SortOrder // Requested sorts, in order.
}

// ParsePageParams extracts page, size, and sort parameters from the request
// query string. Invalid or out-of-range values fall back to defaults (page 0,
// size 20). Valid values are clamped to keep them bounded: size is capped at
// maxSize (10000) and page is capped at maxPage (214748), so the offset
// page*size always fits in an int32 and cannot overflow the SQL LIMIT/OFFSET
// values used by listing endpoints.
func ParsePageParams(r *http.Request) PageParams {
	p := PageParams{Page: 0, Size: 20}
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			p.Page = n
		}
	}
	if v := r.URL.Query().Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			p.Size = n
		}
	}
	if p.Size > maxSize {
		p.Size = maxSize
	}
	if p.Page > maxPage {
		p.Page = maxPage
	}
	for _, raw := range r.URL.Query()["sort"] {
		seg := strings.Split(raw, ",")
		field, dir := seg[0], ""
		if len(seg) > 1 {
			dir = seg[1]
		}
		if field == "" {
			continue
		}
		p.Sort = append(p.Sort, SortOrder{Field: field, Desc: strings.EqualFold(dir, "desc")})
		p.Sorted = true
	}
	return p
}

// OrderClause maps requested sort fields to SQL column names; unknown fields
// are skipped. Returns "" when there are no sortable columns. The caller
// applies its default sort when p.Sorted is false.
func (p PageParams) OrderClause(sortable map[string]string) (string, []any) {
	cols := make([]string, 0, len(p.Sort))
	args := make([]any, 0)
	for _, so := range p.Sort {
		col, ok := sortable[so.Field]
		if !ok {
			continue
		}
		dir := "ASC"
		if so.Desc {
			dir = "DESC"
		}
		cols = append(cols, col+" "+dir)
	}
	if len(cols) == 0 {
		return "", nil
	}
	return " ORDER BY " + strings.Join(cols, ", "), args
}
