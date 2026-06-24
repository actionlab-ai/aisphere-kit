package db

import "testing"

func TestApplySortSafeIgnoresUnknownSort(t *testing.T) {
	req := NormalizePage(PageRequest{Page: 0, PageSize: 0, SortBy: "bad", SortOrder: "desc"})
	if req.Page != 1 || req.PageSize != 20 {
		t.Fatalf("NormalizePage mismatch: %+v", req)
	}
}
