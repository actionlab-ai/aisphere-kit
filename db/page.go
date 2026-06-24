package db

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PageRequest struct {
	Page     int
	PageSize int
	// Sort is kept for trusted internal callers only. Do not pass raw HTTP input
	// into Sort. Prefer SortBy + SortOrder with an allowed-column map.
	Sort string
	// SortBy is a logical external sort key, resolved through an allowlist.
	SortBy string
	// SortOrder supports "asc" or "desc". Any other value is treated as asc.
	SortOrder string
}

type PageResult[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func NormalizePage(req PageRequest) PageRequest {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 200 {
		req.PageSize = 200
	}
	return req
}

func Paginate[T any](query *gorm.DB, req PageRequest) (PageResult[T], error) {
	return paginateWithSort[T](query, req, nil)
}

// PaginateSafe applies sorting only when req.SortBy is present in allowedSorts.
// allowedSorts maps API-level sort keys to database column names, for example:
// map[string]string{"created_at": "created_at", "name": "name"}.
func PaginateSafe[T any](query *gorm.DB, req PageRequest, allowedSorts map[string]string) (PageResult[T], error) {
	return paginateWithSort[T](query, req, allowedSorts)
}

func paginateWithSort[T any](query *gorm.DB, req PageRequest, allowedSorts map[string]string) (PageResult[T], error) {
	req = NormalizePage(req)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return PageResult[T]{}, err
	}
	var items []T
	q := query.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	q = ApplySort(q, req, allowedSorts)
	if err := q.Find(&items).Error; err != nil {
		return PageResult[T]{}, err
	}
	return PageResult[T]{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}

func ApplySort(q *gorm.DB, req PageRequest, allowedSorts map[string]string) *gorm.DB {
	if len(allowedSorts) > 0 {
		column, ok := allowedSorts[req.SortBy]
		if !ok || strings.TrimSpace(column) == "" {
			return q
		}
		return q.Order(clause.OrderByColumn{
			Column: clause.Column{Name: column},
			Desc:   strings.EqualFold(req.SortOrder, "desc"),
		})
	}
	if req.Sort != "" {
		return q.Order(req.Sort)
	}
	return q
}
