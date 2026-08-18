package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestParsePageParamsDefaults(t *testing.T) {
	p := ParsePageParams(httptest.NewRequest("GET", "/x", nil))
	if p.Page != 0 || p.Size != 20 || p.Sorted {
		t.Fatalf("defaults wrong: %+v", p)
	}
}

func TestParsePageParamsFull(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?page=2&size=10&sort=updatedAt,desc&sort=name,asc", nil)
	p := ParsePageParams(req)
	if p.Page != 2 || p.Size != 10 || !p.Sorted {
		t.Fatalf("parse wrong: %+v", p)
	}
	if len(p.Sort) != 2 || p.Sort[0] != (SortOrder{"updatedAt", true}) || p.Sort[1] != (SortOrder{"name", false}) {
		t.Fatalf("sort wrong: %+v", p.Sort)
	}
}

func TestParsePageParamsBadValuesFallBack(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?page=abc&size=-5", nil)
	p := ParsePageParams(req)
	if p.Page != 0 || p.Size != 20 {
		t.Fatalf("fallback wrong: %+v", p)
	}
}

func TestParsePageParamsClamps(t *testing.T) {
	req := httptest.NewRequest("GET", "/x?page=999999999&size=50000", nil)
	p := ParsePageParams(req)
	if p.Size != maxSize {
		t.Fatalf("size = %d, want %d", p.Size, maxSize)
	}
	if p.Page != maxPage {
		t.Fatalf("page = %d, want %d", p.Page, maxPage)
	}
	if product := p.Page * p.Size; product > int(^uint32(0)>>1) {
		t.Fatalf("page*size = %d overflows int32", product)
	}
}
