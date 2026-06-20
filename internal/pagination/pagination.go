package pagination

import (
	"errors"
	"fmt"
	"math"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 1000
)

var (
	ErrInvalidPageSize    = errors.New("pagination: page size must be positive")
	ErrInvalidPageNumber  = errors.New("pagination: page number must be positive")
	ErrPageSizeExceedsMax = errors.New("pagination: page size exceeds maximum allowed")
	ErrNilData            = errors.New("pagination: data slice cannot be nil")
)

type CursorDirection string

const (
	CursorForward  CursorDirection = "forward"
	CursorBackward CursorDirection = "backward"
)

type CursorPageRequest struct {
	Cursor    string
	Direction CursorDirection
	Size      int
}

func NewCursorPageRequest(cursor string, direction CursorDirection, size int) (*CursorPageRequest, error) {
	if size <= 0 {
		return nil, ErrInvalidPageSize
	}
	if size > MaxPageSize {
		return nil, ErrPageSizeExceedsMax
	}
	if direction == "" {
		direction = CursorForward
	}
	return &CursorPageRequest{
		Cursor:    cursor,
		Direction: direction,
		Size:      size,
	}, nil
}

type OffsetPageRequest struct {
	Page int
	Size int
}

func NewOffsetPageRequest(page, size int) (*OffsetPageRequest, error) {
	if page <= 0 {
		return nil, ErrInvalidPageNumber
	}
	if size <= 0 {
		return nil, ErrInvalidPageSize
	}
	if size > MaxPageSize {
		return nil, ErrPageSizeExceedsMax
	}
	return &OffsetPageRequest{
		Page: page,
		Size: size,
	}, nil
}

func (r *OffsetPageRequest) Offset() int {
	return (r.Page - 1) * r.Size
}

func (r *OffsetPageRequest) Limit() int {
	return r.Size
}

type CursorPageMeta struct {
	StartCursor   string
	EndCursor     string
	HasNextPage   bool
	HasPrevPage   bool
	NextCursor    string
	PrevCursor    string
	TotalCount    *int64
	TotalPages    *int
	CurrentCursor string
	PageSize      int
}

type OffsetPageMeta struct {
	CurrentPage int
	PageSize    int
	TotalPages  int
	TotalCount  int64
	HasNextPage bool
	HasPrevPage bool
}

type PageResponse[T any] struct {
	Data    []T
	Meta    any
	Nav     any
	Success bool
}

type CursorNav struct {
	NextCursor string
	PrevCursor string
}

type OffsetNav struct {
	FirstPage int
	LastPage  int
	PrevPage  *int
	NextPage  *int
}

func extractCursors[T any](data []T, cursorFn func(T) string) (start, end string) {
	if len(data) == 0 {
		return "", ""
	}
	if cursorFn != nil {
		return cursorFn(data[0]), cursorFn(data[len(data)-1])
	}
	return "", ""
}

func BuildCursorResponse[T any](
	items []T,
	req *CursorPageRequest,
	cursorFn func(T) string,
	hasMoreAfter bool,
	hasMoreBefore bool,
) *PageResponse[T] {
	data := items
	if data == nil {
		data = []T{}
	}

	startCursor, endCursor := extractCursors(data, cursorFn)

	meta := &CursorPageMeta{
		StartCursor:   startCursor,
		EndCursor:     endCursor,
		HasNextPage:   hasMoreAfter,
		HasPrevPage:   hasMoreBefore,
		CurrentCursor: req.Cursor,
		PageSize:      req.Size,
	}

	if hasMoreAfter && endCursor != "" {
		meta.NextCursor = endCursor
	}
	if hasMoreBefore && startCursor != "" {
		meta.PrevCursor = startCursor
	}

	nav := &CursorNav{
		NextCursor: meta.NextCursor,
		PrevCursor: meta.PrevCursor,
	}

	return &PageResponse[T]{
		Data:    data,
		Meta:    meta,
		Nav:     nav,
		Success: true,
	}
}

func BuildCursorResponseWithTotal[T any](
	items []T,
	req *CursorPageRequest,
	cursorFn func(T) string,
	hasMoreAfter bool,
	hasMoreBefore bool,
	totalCount int64,
) *PageResponse[T] {
	resp := BuildCursorResponse(items, req, cursorFn, hasMoreAfter, hasMoreBefore)
	meta := resp.Meta.(*CursorPageMeta)
	meta.TotalCount = &totalCount

	if req.Size > 0 {
		tp := int(math.Ceil(float64(totalCount) / float64(req.Size)))
		meta.TotalPages = &tp
	}

	return resp
}

func (r *PageResponse[T]) SetTotal(total int64) error {
	switch meta := r.Meta.(type) {
	case *CursorPageMeta:
		meta.TotalCount = &total
		if meta.PageSize > 0 {
			tp := int(math.Ceil(float64(total) / float64(meta.PageSize)))
			meta.TotalPages = &tp
		}
		return nil
	case *OffsetPageMeta:
		meta.TotalCount = total
		if meta.PageSize > 0 {
			meta.TotalPages = int(math.Ceil(float64(total) / float64(meta.PageSize)))
		}
		if meta.CurrentPage < meta.TotalPages {
			meta.HasNextPage = true
		} else {
			meta.HasNextPage = false
		}
		if meta.CurrentPage > 1 && meta.TotalPages > 0 {
			meta.HasPrevPage = true
		} else {
			meta.HasPrevPage = false
		}
		if r.Nav != nil {
			if nav, ok := r.Nav.(*OffsetNav); ok {
				nav.LastPage = meta.TotalPages
				if meta.HasPrevPage {
					prev := meta.CurrentPage - 1
					nav.PrevPage = &prev
				} else {
					nav.PrevPage = nil
				}
				if meta.HasNextPage {
					next := meta.CurrentPage + 1
					nav.NextPage = &next
				} else {
					nav.NextPage = nil
				}
			}
		}
		if meta.CurrentPage > meta.TotalPages {
			r.Data = []T{}
		}
		return nil
	default:
		return fmt.Errorf("pagination: unsupported meta type for SetTotal")
	}
}

func BuildOffsetResponse[T any](
	items []T,
	req *OffsetPageRequest,
	totalCount int64,
) *PageResponse[T] {
	data := items
	if data == nil {
		data = []T{}
	}

	totalPages := 0
	if req.Size > 0 {
		totalPages = int(math.Ceil(float64(totalCount) / float64(req.Size)))
	}

	currentPage := req.Page
	hasNextPage := currentPage < totalPages
	hasPrevPage := currentPage > 1 && totalPages > 0

	actualData := data
	if currentPage > totalPages {
		actualData = []T{}
	}

	meta := &OffsetPageMeta{
		CurrentPage: currentPage,
		PageSize:    req.Size,
		TotalPages:  totalPages,
		TotalCount:  totalCount,
		HasNextPage: hasNextPage,
		HasPrevPage: hasPrevPage,
	}

	nav := &OffsetNav{
		FirstPage: 1,
		LastPage:  totalPages,
	}
	if hasPrevPage {
		prev := currentPage - 1
		nav.PrevPage = &prev
	}
	if hasNextPage {
		next := currentPage + 1
		nav.NextPage = &next
	}

	return &PageResponse[T]{
		Data:    actualData,
		Meta:    meta,
		Nav:     nav,
		Success: true,
	}
}

func BuildEmptyOffsetResponse[T any](req *OffsetPageRequest) *PageResponse[T] {
	meta := &OffsetPageMeta{
		CurrentPage: req.Page,
		PageSize:    req.Size,
		TotalPages:  0,
		TotalCount:  0,
		HasNextPage: false,
		HasPrevPage: false,
	}
	nav := &OffsetNav{
		FirstPage: 1,
		LastPage:  0,
	}
	return &PageResponse[T]{
		Data:    []T{},
		Meta:    meta,
		Nav:     nav,
		Success: true,
	}
}

func BuildEmptyCursorResponse[T any](req *CursorPageRequest) *PageResponse[T] {
	meta := &CursorPageMeta{
		CurrentCursor: req.Cursor,
		PageSize:      req.Size,
	}
	nav := &CursorNav{}
	return &PageResponse[T]{
		Data:    []T{},
		Meta:    meta,
		Nav:     nav,
		Success: true,
	}
}

func ValidateOffsetRequest(page, size int) error {
	if page <= 0 {
		return ErrInvalidPageNumber
	}
	if size <= 0 {
		return ErrInvalidPageSize
	}
	if size > MaxPageSize {
		return ErrPageSizeExceedsMax
	}
	return nil
}

func ValidateCursorRequest(direction CursorDirection, size int) error {
	if size <= 0 {
		return ErrInvalidPageSize
	}
	if size > MaxPageSize {
		return ErrPageSizeExceedsMax
	}
	if direction != "" && direction != CursorForward && direction != CursorBackward {
		return fmt.Errorf("pagination: invalid cursor direction: %s", direction)
	}
	return nil
}

func ValidateData[T any](items []T) error {
	if items == nil {
		return ErrNilData
	}
	return nil
}
