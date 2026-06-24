package db

import "gorm.io/gorm"

type PageRequest struct {
	Page     int
	PageSize int
	Sort     string
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
	req = NormalizePage(req)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return PageResult[T]{}, err
	}
	var items []T
	q := query.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize)
	if req.Sort != "" {
		q = q.Order(req.Sort)
	}
	if err := q.Find(&items).Error; err != nil {
		return PageResult[T]{}, err
	}
	return PageResult[T]{Items: items, Total: total, Page: req.Page, PageSize: req.PageSize}, nil
}
