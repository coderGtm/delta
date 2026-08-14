package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

type SortOrder struct {
	Field string
	Desc  bool
}

type PageParams struct {
	Page   int
	Size   int
	Sorted bool
	Sort   []SortOrder
}

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
