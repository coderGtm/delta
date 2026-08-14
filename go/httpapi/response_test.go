package httpapi

import (
	"testing"
)

func TestNewPageResponse(t *testing.T) {
	empty := NewPageResponse([]int{}, 0, PageParams{Page: 0, Size: 20})
	if empty.TotalPages != 0 || !empty.First || !empty.Last || !empty.Empty || empty.TotalElements != 0 {
		t.Errorf("empty page wrong: %+v", empty)
	}

	first := NewPageResponse([]int{1, 2}, 2, PageParams{Page: 0, Size: 20})
	if first.TotalPages != 1 || !first.First || !first.Last || first.Empty {
		t.Errorf("single-page response wrong: %+v", first)
	}

	mid := NewPageResponse([]int{3, 4}, 61, PageParams{Page: 2, Size: 20})
	if mid.TotalPages != 4 || mid.First || mid.Last || mid.Empty {
		t.Errorf("middle-page response wrong: %+v", mid)
	}

	last := NewPageResponse([]int{9}, 45, PageParams{Page: 2, Size: 20})
	if last.TotalPages != 3 || last.First || !last.Last || last.Empty {
		t.Errorf("last-page response wrong: %+v", last)
	}
}
