package pagination

import (
	"errors"
	"fmt"
	"testing"
)

type TestItem struct {
	ID   string
	Name string
	Rank int
}

func itemCursor(item TestItem) string {
	return item.ID
}

func makeTestItems(n int) []TestItem {
	items := make([]TestItem, n)
	for i := 0; i < n; i++ {
		items[i] = TestItem{
			ID:   fmt.Sprintf("item-%05d", i+1),
			Name: fmt.Sprintf("Item %d", i+1),
			Rank: i + 1,
		}
	}
	return items
}

func TestConstants(t *testing.T) {
	if DefaultPageSize != 20 {
		t.Errorf("expected DefaultPageSize 20, got %d", DefaultPageSize)
	}
	if MaxPageSize != 1000 {
		t.Errorf("expected MaxPageSize 1000, got %d", MaxPageSize)
	}
}

func TestCursorDirectionConstants(t *testing.T) {
	if CursorForward != "forward" {
		t.Errorf("expected CursorForward 'forward', got '%s'", CursorForward)
	}
	if CursorBackward != "backward" {
		t.Errorf("expected CursorBackward 'backward', got '%s'", CursorBackward)
	}
}

func TestNewCursorPageRequest(t *testing.T) {
	tests := []struct {
		name      string
		cursor    string
		direction CursorDirection
		size      int
		wantErr   error
		checkFn   func(t *testing.T, req *CursorPageRequest)
	}{
		{
			name:      "valid forward",
			cursor:    "abc123",
			direction: CursorForward,
			size:      20,
			wantErr:   nil,
			checkFn: func(t *testing.T, req *CursorPageRequest) {
				if req.Cursor != "abc123" {
					t.Errorf("expected cursor 'abc123', got '%s'", req.Cursor)
				}
				if req.Direction != CursorForward {
					t.Errorf("expected direction forward, got %s", req.Direction)
				}
				if req.Size != 20 {
					t.Errorf("expected size 20, got %d", req.Size)
				}
			},
		},
		{
			name:      "valid backward",
			cursor:    "xyz",
			direction: CursorBackward,
			size:      50,
			wantErr:   nil,
			checkFn: func(t *testing.T, req *CursorPageRequest) {
				if req.Direction != CursorBackward {
					t.Errorf("expected direction backward, got %s", req.Direction)
				}
			},
		},
		{
			name:      "empty direction defaults to forward",
			cursor:    "cur",
			direction: "",
			size:      10,
			wantErr:   nil,
			checkFn: func(t *testing.T, req *CursorPageRequest) {
				if req.Direction != CursorForward {
					t.Errorf("expected default direction forward, got %s", req.Direction)
				}
			},
		},
		{
			name:      "empty cursor is allowed",
			cursor:    "",
			direction: CursorForward,
			size:      10,
			wantErr:   nil,
			checkFn: func(t *testing.T, req *CursorPageRequest) {
				if req.Cursor != "" {
					t.Errorf("expected empty cursor, got '%s'", req.Cursor)
				}
			},
		},
		{
			name:      "zero size",
			cursor:    "cur",
			direction: CursorForward,
			size:      0,
			wantErr:   ErrInvalidPageSize,
		},
		{
			name:      "negative size",
			cursor:    "cur",
			direction: CursorForward,
			size:      -5,
			wantErr:   ErrInvalidPageSize,
		},
		{
			name:      "size exceeds max",
			cursor:    "cur",
			direction: CursorForward,
			size:      1001,
			wantErr:   ErrPageSizeExceedsMax,
		},
		{
			name:      "size at max boundary",
			cursor:    "cur",
			direction: CursorForward,
			size:      1000,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewCursorPageRequest(tt.cursor, tt.direction, tt.size)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				if req != nil {
					t.Error("expected nil request on error")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if req == nil {
					t.Fatal("expected non-nil request")
				}
				if tt.checkFn != nil {
					tt.checkFn(t, req)
				}
			}
		})
	}
}

func TestNewOffsetPageRequest(t *testing.T) {
	tests := []struct {
		name      string
		page      int
		size      int
		wantErr   error
		checkFn   func(t *testing.T, req *OffsetPageRequest)
	}{
		{
			name:    "valid first page",
			page:    1,
			size:    20,
			wantErr: nil,
			checkFn: func(t *testing.T, req *OffsetPageRequest) {
				if req.Page != 1 {
					t.Errorf("expected page 1, got %d", req.Page)
				}
				if req.Size != 20 {
					t.Errorf("expected size 20, got %d", req.Size)
				}
			},
		},
		{
			name:    "valid large page",
			page:    100,
			size:    50,
			wantErr: nil,
		},
		{
			name:    "zero page",
			page:    0,
			size:    10,
			wantErr: ErrInvalidPageNumber,
		},
		{
			name:    "negative page",
			page:    -1,
			size:    10,
			wantErr: ErrInvalidPageNumber,
		},
		{
			name:    "zero size",
			page:    1,
			size:    0,
			wantErr: ErrInvalidPageSize,
		},
		{
			name:    "negative size",
			page:    1,
			size:    -10,
			wantErr: ErrInvalidPageSize,
		},
		{
			name:    "size exceeds max",
			page:    1,
			size:    1001,
			wantErr: ErrPageSizeExceedsMax,
		},
		{
			name:    "boundary max size",
			page:    1,
			size:    1000,
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewOffsetPageRequest(tt.page, tt.size)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				if req != nil {
					t.Error("expected nil request on error")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if req == nil {
					t.Fatal("expected non-nil request")
				}
				if tt.checkFn != nil {
					tt.checkFn(t, req)
				}
			}
		})
	}
}

func TestOffsetPageRequestOffsetAndLimit(t *testing.T) {
	tests := []struct {
		name         string
		page         int
		size         int
		wantOffset   int
		wantLimit    int
	}{
		{"page 1 size 20", 1, 20, 0, 20},
		{"page 2 size 20", 2, 20, 20, 20},
		{"page 5 size 10", 5, 10, 40, 10},
		{"page 10 size 50", 10, 50, 450, 50},
		{"page 1 size 1", 1, 1, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &OffsetPageRequest{Page: tt.page, Size: tt.size}
			if req.Offset() != tt.wantOffset {
				t.Errorf("expected offset %d, got %d", tt.wantOffset, req.Offset())
			}
			if req.Limit() != tt.wantLimit {
				t.Errorf("expected limit %d, got %d", tt.wantLimit, req.Limit())
			}
		})
	}
}

func TestValidateOffsetRequest(t *testing.T) {
	tests := []struct {
		name    string
		page    int
		size    int
		wantErr error
	}{
		{"valid", 1, 10, nil},
		{"zero page", 0, 10, ErrInvalidPageNumber},
		{"negative page", -1, 10, ErrInvalidPageNumber},
		{"zero size", 1, 0, ErrInvalidPageSize},
		{"negative size", 1, -5, ErrInvalidPageSize},
		{"size exceeds max", 1, 1001, ErrPageSizeExceedsMax},
		{"boundary", 1, 1000, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOffsetRequest(tt.page, tt.size)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateCursorRequest(t *testing.T) {
	tests := []struct {
		name      string
		direction CursorDirection
		size      int
		wantErr   bool
	}{
		{"valid forward", CursorForward, 10, false},
		{"valid backward", CursorBackward, 10, false},
		{"empty direction", "", 10, false},
		{"invalid direction", CursorDirection("sideways"), 10, true},
		{"zero size", CursorForward, 0, true},
		{"negative size", CursorForward, -5, true},
		{"too large size", CursorForward, 1001, true},
		{"boundary max", CursorForward, 1000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCursorRequest(tt.direction, tt.size)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestBuildCursorResponseForward(t *testing.T) {
	items := makeTestItems(10)
	req, _ := NewCursorPageRequest("item-00003", CursorForward, 10)

	resp := BuildCursorResponse(items, req, itemCursor, true, true)

	if !resp.Success {
		t.Error("expected Success=true")
	}
	if len(resp.Data) != 10 {
		t.Errorf("expected 10 data items, got %d", len(resp.Data))
	}

	meta, ok := resp.Meta.(*CursorPageMeta)
	if !ok {
		t.Fatal("Meta should be *CursorPageMeta")
	}

	if meta.StartCursor != "item-00001" {
		t.Errorf("expected StartCursor 'item-00001', got '%s'", meta.StartCursor)
	}
	if meta.EndCursor != "item-00010" {
		t.Errorf("expected EndCursor 'item-00010', got '%s'", meta.EndCursor)
	}
	if !meta.HasNextPage {
		t.Error("expected HasNextPage=true")
	}
	if !meta.HasPrevPage {
		t.Error("expected HasPrevPage=true")
	}
	if meta.NextCursor != "item-00010" {
		t.Errorf("expected NextCursor 'item-00010', got '%s'", meta.NextCursor)
	}
	if meta.PrevCursor != "item-00001" {
		t.Errorf("expected PrevCursor 'item-00001', got '%s'", meta.PrevCursor)
	}
	if meta.CurrentCursor != "item-00003" {
		t.Errorf("expected CurrentCursor 'item-00003', got '%s'", meta.CurrentCursor)
	}
	if meta.PageSize != 10 {
		t.Errorf("expected PageSize 10, got %d", meta.PageSize)
	}
	if meta.TotalCount != nil {
		t.Error("expected TotalCount=nil without explicit total")
	}
	if meta.TotalPages != nil {
		t.Error("expected TotalPages=nil without explicit total")
	}

	nav, ok := resp.Nav.(*CursorNav)
	if !ok {
		t.Fatal("Nav should be *CursorNav")
	}
	if nav.NextCursor != "item-00010" {
		t.Errorf("nav NextCursor mismatch")
	}
	if nav.PrevCursor != "item-00001" {
		t.Errorf("nav PrevCursor mismatch")
	}
}

func TestBuildCursorResponseNoMorePages(t *testing.T) {
	items := makeTestItems(5)
	req, _ := NewCursorPageRequest("", CursorForward, 10)

	resp := BuildCursorResponse(items, req, itemCursor, false, false)

	meta := resp.Meta.(*CursorPageMeta)
	if meta.HasNextPage {
		t.Error("expected HasNextPage=false")
	}
	if meta.HasPrevPage {
		t.Error("expected HasPrevPage=false")
	}
	if meta.NextCursor != "" {
		t.Errorf("expected empty NextCursor, got '%s'", meta.NextCursor)
	}
	if meta.PrevCursor != "" {
		t.Errorf("expected empty PrevCursor, got '%s'", meta.PrevCursor)
	}
}

func TestBuildCursorResponseEmptyData(t *testing.T) {
	req, _ := NewCursorPageRequest("cursor-x", CursorForward, 10)

	resp := BuildCursorResponse[TestItem](nil, req, itemCursor, false, false)

	if len(resp.Data) != 0 {
		t.Errorf("expected empty data, got %d items", len(resp.Data))
	}
	meta := resp.Meta.(*CursorPageMeta)
	if meta.StartCursor != "" {
		t.Errorf("expected empty StartCursor, got '%s'", meta.StartCursor)
	}
	if meta.EndCursor != "" {
		t.Errorf("expected empty EndCursor, got '%s'", meta.EndCursor)
	}
}

func TestBuildCursorResponseNilCursorFn(t *testing.T) {
	items := makeTestItems(5)
	req, _ := NewCursorPageRequest("", CursorForward, 10)

	resp := BuildCursorResponse(items, req, nil, true, true)

	meta := resp.Meta.(*CursorPageMeta)
	if meta.StartCursor != "" {
		t.Errorf("expected empty StartCursor with nil cursorFn, got '%s'", meta.StartCursor)
	}
	if meta.EndCursor != "" {
		t.Errorf("expected empty EndCursor with nil cursorFn, got '%s'", meta.EndCursor)
	}
	if meta.NextCursor != "" {
		t.Errorf("NextCursor should be empty when end cursor is empty")
	}
}

func TestBuildCursorResponseWithTotal(t *testing.T) {
	items := makeTestItems(10)
	req, _ := NewCursorPageRequest("start", CursorForward, 10)

	resp := BuildCursorResponseWithTotal(items, req, itemCursor, true, false, int64(100))

	meta := resp.Meta.(*CursorPageMeta)
	if meta.TotalCount == nil {
		t.Fatal("expected TotalCount to be set")
	}
	if *meta.TotalCount != 100 {
		t.Errorf("expected TotalCount 100, got %d", *meta.TotalCount)
	}
	if meta.TotalPages == nil {
		t.Fatal("expected TotalPages to be set")
	}
	if *meta.TotalPages != 10 {
		t.Errorf("expected TotalPages 10, got %d", *meta.TotalPages)
	}
}

func TestBuildCursorResponseWithTotalRounding(t *testing.T) {
	items := makeTestItems(10)
	req, _ := NewCursorPageRequest("start", CursorForward, 10)

	resp := BuildCursorResponseWithTotal(items, req, itemCursor, true, false, int64(105))

	meta := resp.Meta.(*CursorPageMeta)
	if *meta.TotalPages != 11 {
		t.Errorf("expected TotalPages 11 (ceil(105/10)), got %d", *meta.TotalPages)
	}
}

func TestSetTotalOnCursorResponse(t *testing.T) {
	items := makeTestItems(5)
	req, _ := NewCursorPageRequest("cur", CursorForward, 5)
	resp := BuildCursorResponse(items, req, itemCursor, false, false)

	err := resp.SetTotal(int64(23))
	if err != nil {
		t.Fatalf("SetTotal failed: %v", err)
	}

	meta := resp.Meta.(*CursorPageMeta)
	if meta.TotalCount == nil || *meta.TotalCount != 23 {
		t.Errorf("expected TotalCount 23")
	}
	if meta.TotalPages == nil || *meta.TotalPages != 5 {
		t.Errorf("expected TotalPages 5 (ceil(23/5)), got %v", meta.TotalPages)
	}
}

func TestBuildOffsetResponseFirstPage(t *testing.T) {
	items := makeTestItems(10)
	req, _ := NewOffsetPageRequest(1, 10)

	resp := BuildOffsetResponse(items, req, int64(35))

	if !resp.Success {
		t.Error("expected Success=true")
	}
	if len(resp.Data) != 10 {
		t.Errorf("expected 10 items, got %d", len(resp.Data))
	}

	meta, ok := resp.Meta.(*OffsetPageMeta)
	if !ok {
		t.Fatal("Meta should be *OffsetPageMeta")
	}

	if meta.CurrentPage != 1 {
		t.Errorf("expected CurrentPage 1, got %d", meta.CurrentPage)
	}
	if meta.PageSize != 10 {
		t.Errorf("expected PageSize 10, got %d", meta.PageSize)
	}
	if meta.TotalPages != 4 {
		t.Errorf("expected TotalPages 4 (ceil(35/10)), got %d", meta.TotalPages)
	}
	if meta.TotalCount != 35 {
		t.Errorf("expected TotalCount 35, got %d", meta.TotalCount)
	}
	if meta.HasPrevPage {
		t.Error("expected HasPrevPage=false on page 1")
	}
	if !meta.HasNextPage {
		t.Error("expected HasNextPage=true on page 1 of 4")
	}

	nav, ok := resp.Nav.(*OffsetNav)
	if !ok {
		t.Fatal("Nav should be *OffsetNav")
	}
	if nav.FirstPage != 1 {
		t.Errorf("expected FirstPage 1, got %d", nav.FirstPage)
	}
	if nav.LastPage != 4 {
		t.Errorf("expected LastPage 4, got %d", nav.LastPage)
	}
	if nav.PrevPage != nil {
		t.Errorf("expected PrevPage=nil on page 1")
	}
	if nav.NextPage == nil || *nav.NextPage != 2 {
		t.Errorf("expected NextPage=2 on page 1")
	}
}

func TestBuildOffsetResponseMiddlePage(t *testing.T) {
	items := makeTestItems(10)
	req, _ := NewOffsetPageRequest(3, 10)

	resp := BuildOffsetResponse(items, req, int64(50))

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.CurrentPage != 3 {
		t.Errorf("expected CurrentPage 3, got %d", meta.CurrentPage)
	}
	if meta.TotalPages != 5 {
		t.Errorf("expected TotalPages 5, got %d", meta.TotalPages)
	}
	if !meta.HasPrevPage {
		t.Error("expected HasPrevPage=true on page 3")
	}
	if !meta.HasNextPage {
		t.Error("expected HasNextPage=true on page 3 of 5")
	}

	nav := resp.Nav.(*OffsetNav)
	if nav.PrevPage == nil || *nav.PrevPage != 2 {
		t.Errorf("expected PrevPage=2")
	}
	if nav.NextPage == nil || *nav.NextPage != 4 {
		t.Errorf("expected NextPage=4")
	}
}

func TestBuildOffsetResponseLastPage(t *testing.T) {
	items := makeTestItems(5)
	req, _ := NewOffsetPageRequest(4, 10)

	resp := BuildOffsetResponse(items, req, int64(35))

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.CurrentPage != 4 {
		t.Errorf("expected CurrentPage 4, got %d", meta.CurrentPage)
	}
	if meta.TotalPages != 4 {
		t.Errorf("expected TotalPages 4, got %d", meta.TotalPages)
	}
	if !meta.HasPrevPage {
		t.Error("expected HasPrevPage=true on last page")
	}
	if meta.HasNextPage {
		t.Error("expected HasNextPage=false on last page")
	}

	nav := resp.Nav.(*OffsetNav)
	if nav.PrevPage == nil || *nav.PrevPage != 3 {
		t.Errorf("expected PrevPage=3 on last page")
	}
	if nav.NextPage != nil {
		t.Errorf("expected NextPage=nil on last page")
	}
}

func TestBuildOffsetResponseBeyondLastPage(t *testing.T) {
	items := makeTestItems(10)
	req, _ := NewOffsetPageRequest(10, 10)

	resp := BuildOffsetResponse(items, req, int64(35))

	if len(resp.Data) != 0 {
		t.Errorf("expected empty data when page beyond total pages, got %d items", len(resp.Data))
	}

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.CurrentPage != 10 {
		t.Errorf("expected CurrentPage 10, got %d", meta.CurrentPage)
	}
	if meta.TotalPages != 4 {
		t.Errorf("expected TotalPages 4, got %d", meta.TotalPages)
	}
	if !meta.HasPrevPage {
		t.Error("expected HasPrevPage=true (page 10 > 1)")
	}
	if meta.HasNextPage {
		t.Error("expected HasNextPage=false")
	}
}

func TestBuildOffsetResponseZeroTotal(t *testing.T) {
	req, _ := NewOffsetPageRequest(1, 10)

	resp := BuildOffsetResponse[TestItem](nil, req, int64(0))

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.TotalCount != 0 {
		t.Errorf("expected TotalCount 0, got %d", meta.TotalCount)
	}
	if meta.TotalPages != 0 {
		t.Errorf("expected TotalPages 0, got %d", meta.TotalPages)
	}
	if meta.HasPrevPage {
		t.Error("expected HasPrevPage=false")
	}
	if meta.HasNextPage {
		t.Error("expected HasNextPage=false")
	}

	nav := resp.Nav.(*OffsetNav)
	if nav.LastPage != 0 {
		t.Errorf("expected LastPage 0, got %d", nav.LastPage)
	}
	if nav.PrevPage != nil {
		t.Error("expected PrevPage=nil")
	}
	if nav.NextPage != nil {
		t.Error("expected NextPage=nil")
	}
}

func TestBuildOffsetResponseExactDivision(t *testing.T) {
	items := makeTestItems(10)
	req, _ := NewOffsetPageRequest(5, 10)

	resp := BuildOffsetResponse(items, req, int64(50))

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.TotalPages != 5 {
		t.Errorf("expected TotalPages 5, got %d", meta.TotalPages)
	}
	if meta.HasNextPage {
		t.Error("expected HasNextPage=false on exact last page")
	}
}

func TestBuildOffsetResponseNilData(t *testing.T) {
	req, _ := NewOffsetPageRequest(1, 10)
	resp := BuildOffsetResponse[TestItem](nil, req, int64(5))

	if resp.Data == nil {
		t.Error("expected non-nil empty slice for nil input")
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected 0 items, got %d", len(resp.Data))
	}
}

func TestSetTotalOnOffsetResponse(t *testing.T) {
	items := makeTestItems(5)
	req, _ := NewOffsetPageRequest(2, 5)
	resp := BuildOffsetResponse(items, req, int64(0))

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.TotalCount != 0 {
		t.Errorf("expected initial TotalCount 0")
	}

	err := resp.SetTotal(int64(23))
	if err != nil {
		t.Fatalf("SetTotal failed: %v", err)
	}

	if meta.TotalCount != 23 {
		t.Errorf("expected TotalCount 23, got %d", meta.TotalCount)
	}
	if meta.TotalPages != 5 {
		t.Errorf("expected TotalPages 5 (ceil(23/5)), got %d", meta.TotalPages)
	}
	if !meta.HasNextPage {
		t.Error("expected HasNextPage=true (page 2 of 5)")
	}
	if !meta.HasPrevPage {
		t.Error("expected HasPrevPage=true (page 2 > 1)")
	}

	nav := resp.Nav.(*OffsetNav)
	if nav.LastPage != 5 {
		t.Errorf("expected nav.LastPage 5, got %d", nav.LastPage)
	}
	if nav.PrevPage == nil || *nav.PrevPage != 1 {
		t.Error("expected nav.PrevPage=1")
	}
	if nav.NextPage == nil || *nav.NextPage != 3 {
		t.Error("expected nav.NextPage=3")
	}
}

func TestSetTotalOnOffsetResponseLastPageCalculated(t *testing.T) {
	items := makeTestItems(5)
	req, _ := NewOffsetPageRequest(1, 10)
	resp := BuildOffsetResponse(items, req, int64(0))

	err := resp.SetTotal(int64(5))
	if err != nil {
		t.Fatalf("SetTotal failed: %v", err)
	}

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.TotalPages != 1 {
		t.Errorf("expected TotalPages 1, got %d", meta.TotalPages)
	}
	if meta.HasNextPage {
		t.Error("expected HasNextPage=false on page 1 of 1")
	}
	if meta.HasPrevPage {
		t.Error("expected HasPrevPage=false on page 1")
	}

	nav := resp.Nav.(*OffsetNav)
	if nav.PrevPage != nil {
		t.Error("expected PrevPage=nil")
	}
	if nav.NextPage != nil {
		t.Error("expected NextPage=nil")
	}
}

func TestSetTotalUnsupportedMeta(t *testing.T) {
	resp := &PageResponse[TestItem]{
		Meta: "not a valid meta type",
	}

	err := resp.SetTotal(int64(100))
	if err == nil {
		t.Error("expected error for unsupported meta type")
	}
}

func TestBuildEmptyOffsetResponse(t *testing.T) {
	req, _ := NewOffsetPageRequest(1, 20)

	resp := BuildEmptyOffsetResponse[TestItem](req)

	if !resp.Success {
		t.Error("expected Success=true")
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected empty data, got %d items", len(resp.Data))
	}

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.CurrentPage != 1 {
		t.Errorf("expected CurrentPage 1, got %d", meta.CurrentPage)
	}
	if meta.PageSize != 20 {
		t.Errorf("expected PageSize 20, got %d", meta.PageSize)
	}
	if meta.TotalCount != 0 {
		t.Errorf("expected TotalCount 0, got %d", meta.TotalCount)
	}
	if meta.TotalPages != 0 {
		t.Errorf("expected TotalPages 0, got %d", meta.TotalPages)
	}
	if meta.HasNextPage || meta.HasPrevPage {
		t.Error("expected no navigation on empty response")
	}

	nav := resp.Nav.(*OffsetNav)
	if nav.FirstPage != 1 {
		t.Errorf("expected FirstPage 1, got %d", nav.FirstPage)
	}
	if nav.LastPage != 0 {
		t.Errorf("expected LastPage 0, got %d", nav.LastPage)
	}
}

func TestBuildEmptyCursorResponse(t *testing.T) {
	req, _ := NewCursorPageRequest("cursor-abc", CursorForward, 20)

	resp := BuildEmptyCursorResponse[TestItem](req)

	if !resp.Success {
		t.Error("expected Success=true")
	}
	if len(resp.Data) != 0 {
		t.Errorf("expected empty data, got %d items", len(resp.Data))
	}

	meta := resp.Meta.(*CursorPageMeta)
	if meta.CurrentCursor != "cursor-abc" {
		t.Errorf("expected CurrentCursor 'cursor-abc', got '%s'", meta.CurrentCursor)
	}
	if meta.PageSize != 20 {
		t.Errorf("expected PageSize 20, got %d", meta.PageSize)
	}
	if meta.HasNextPage || meta.HasPrevPage {
		t.Error("expected no navigation flags on empty response")
	}

	nav := resp.Nav.(*CursorNav)
	if nav.NextCursor != "" {
		t.Errorf("expected empty NextCursor, got '%s'", nav.NextCursor)
	}
	if nav.PrevCursor != "" {
		t.Errorf("expected empty PrevCursor, got '%s'", nav.PrevCursor)
	}
}

func TestCursorResponseBackwardDirection(t *testing.T) {
	items := makeTestItems(10)
	req, _ := NewCursorPageRequest("item-00020", CursorBackward, 10)

	resp := BuildCursorResponse(items, req, itemCursor, true, true)

	meta := resp.Meta.(*CursorPageMeta)
	if meta.CurrentCursor != "item-00020" {
		t.Errorf("expected CurrentCursor preserved, got '%s'", meta.CurrentCursor)
	}
	if meta.StartCursor != "item-00001" {
		t.Errorf("expected StartCursor 'item-00001', got '%s'", meta.StartCursor)
	}
	if meta.EndCursor != "item-00010" {
		t.Errorf("expected EndCursor 'item-00010', got '%s'", meta.EndCursor)
	}
	if !meta.HasNextPage {
		t.Error("expected HasNextPage=true")
	}
	if !meta.HasPrevPage {
		t.Error("expected HasPrevPage=true")
	}
}

func TestSetTotalThenSetAgainCursor(t *testing.T) {
	items := makeTestItems(5)
	req, _ := NewCursorPageRequest("cur", CursorForward, 5)
	resp := BuildCursorResponse(items, req, itemCursor, false, false)

	resp.SetTotal(int64(10))
	meta1 := resp.Meta.(*CursorPageMeta)
	if *meta1.TotalCount != 10 || *meta1.TotalPages != 2 {
		t.Fatalf("first SetTotal failed: count=%v pages=%v", meta1.TotalCount, meta1.TotalPages)
	}

	resp.SetTotal(int64(20))
	meta2 := resp.Meta.(*CursorPageMeta)
	if *meta2.TotalCount != 20 || *meta2.TotalPages != 4 {
		t.Errorf("second SetTotal should overwrite: count=%v pages=%v", *meta2.TotalCount, *meta2.TotalPages)
	}
}

func TestSetTotalThenSetAgainOffset(t *testing.T) {
	items := makeTestItems(10)
	req, _ := NewOffsetPageRequest(2, 10)
	resp := BuildOffsetResponse(items, req, int64(0))

	resp.SetTotal(int64(15))
	meta1 := resp.Meta.(*OffsetPageMeta)
	if meta1.TotalCount != 15 || meta1.TotalPages != 2 {
		t.Fatalf("first SetTotal failed")
	}
	if !meta1.HasPrevPage || meta1.HasNextPage {
		t.Errorf("first SetTotal nav wrong: hasPrev=%v hasNext=%v", meta1.HasPrevPage, meta1.HasNextPage)
	}

	resp.SetTotal(int64(30))
	meta2 := resp.Meta.(*OffsetPageMeta)
	if meta2.TotalCount != 30 || meta2.TotalPages != 3 {
		t.Errorf("second SetTotal should overwrite: count=%d pages=%d", meta2.TotalCount, meta2.TotalPages)
	}
	if !meta2.HasNextPage {
		t.Error("second SetTotal should enable HasNextPage")
	}
}

func TestOffsetPagePage1WithLargeTotal(t *testing.T) {
	items := makeTestItems(100)
	req, _ := NewOffsetPageRequest(1, 100)

	resp := BuildOffsetResponse(items, req, int64(9999))

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.TotalPages != 100 {
		t.Errorf("expected TotalPages 100 (ceil(9999/100)), got %d", meta.TotalPages)
	}
	if !meta.HasNextPage {
		t.Error("expected HasNextPage=true")
	}
	if meta.HasPrevPage {
		t.Error("expected HasPrevPage=false on page 1")
	}
}

func TestCursorResponseNoCursorFnEmptyItems(t *testing.T) {
	req, _ := NewCursorPageRequest("", CursorForward, 10)
	resp := BuildCursorResponse[TestItem]([]TestItem{}, req, itemCursor, false, false)

	meta := resp.Meta.(*CursorPageMeta)
	if meta.StartCursor != "" || meta.EndCursor != "" {
		t.Error("empty items should have empty cursors")
	}
	if meta.NextCursor != "" || meta.PrevCursor != "" {
		t.Error("empty items should have empty nav cursors")
	}
}

func TestCursorResponseHasMoreButNoEndCursor(t *testing.T) {
	items := makeTestItems(5)
	req, _ := NewCursorPageRequest("", CursorForward, 10)

	resp := BuildCursorResponse(items, req, nil, true, true)

	meta := resp.Meta.(*CursorPageMeta)
	if meta.HasNextPage && meta.NextCursor != "" {
		t.Error("when end cursor is empty, NextCursor should remain empty even with HasNextPage=true")
	}
	if meta.HasPrevPage && meta.PrevCursor != "" {
		t.Error("when start cursor is empty, PrevCursor should remain empty even with HasPrevPage=true")
	}
}

func TestExtractCursors(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		s, e := extractCursors[TestItem](nil, itemCursor)
		if s != "" || e != "" {
			t.Errorf("expected empty for nil slice, got s='%s' e='%s'", s, e)
		}
	})

	t.Run("single item", func(t *testing.T) {
		items := []TestItem{{ID: "only"}}
		s, e := extractCursors(items, itemCursor)
		if s != "only" || e != "only" {
			t.Errorf("expected both 'only', got s='%s' e='%s'", s, e)
		}
	})

	t.Run("nil cursor fn", func(t *testing.T) {
		items := makeTestItems(5)
		s, e := extractCursors(items, nil)
		if s != "" || e != "" {
			t.Errorf("expected empty for nil cursorFn, got s='%s' e='%s'", s, e)
		}
	})

	t.Run("multiple items", func(t *testing.T) {
		items := makeTestItems(10)
		s, e := extractCursors(items, itemCursor)
		if s != "item-00001" {
			t.Errorf("expected first cursor, got '%s'", s)
		}
		if e != "item-00010" {
			t.Errorf("expected last cursor, got '%s'", e)
		}
	})
}

func TestErrorVariables(t *testing.T) {
	if ErrInvalidPageSize == nil || ErrInvalidPageSize.Error() == "" {
		t.Error("ErrInvalidPageSize should be set")
	}
	if ErrInvalidPageNumber == nil || ErrInvalidPageNumber.Error() == "" {
		t.Error("ErrInvalidPageNumber should be set")
	}
	if ErrPageSizeExceedsMax == nil || ErrPageSizeExceedsMax.Error() == "" {
		t.Error("ErrPageSizeExceedsMax should be set")
	}
	if ErrNilData == nil || ErrNilData.Error() == "" {
		t.Error("ErrNilData should be set")
	}
}

func TestGenericStringType(t *testing.T) {
	strItems := []string{"a", "b", "c", "d", "e"}
	req, _ := NewOffsetPageRequest(1, 5)

	resp := BuildOffsetResponse(strItems, req, int64(5))

	if len(resp.Data) != 5 {
		t.Errorf("expected 5 string items, got %d", len(resp.Data))
	}
	if resp.Data[0] != "a" || resp.Data[4] != "e" {
		t.Error("string data not preserved correctly")
	}

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.TotalPages != 1 {
		t.Errorf("expected 1 total page, got %d", meta.TotalPages)
	}
}

func TestGenericIntType(t *testing.T) {
	intItems := []int{10, 20, 30}
	req, _ := NewCursorPageRequest("cur1", CursorForward, 3)

	intCursorFn := func(i int) string {
		return fmt.Sprintf("int-%d", i)
	}

	resp := BuildCursorResponseWithTotal(intItems, req, intCursorFn, false, false, int64(3))

	if len(resp.Data) != 3 {
		t.Errorf("expected 3 int items, got %d", len(resp.Data))
	}
	if resp.Data[0] != 10 || resp.Data[2] != 30 {
		t.Error("int data not preserved correctly")
	}

	meta := resp.Meta.(*CursorPageMeta)
	if meta.StartCursor != "int-10" || meta.EndCursor != "int-30" {
		t.Errorf("cursor fn not applied correctly: start='%s' end='%s'", meta.StartCursor, meta.EndCursor)
	}
}

func TestBuildOffsetResponsePage1ZeroTotalEmptyInput(t *testing.T) {
	req, _ := NewOffsetPageRequest(1, 20)
	resp := BuildOffsetResponse[TestItem](makeTestItems(0), req, int64(0))

	if len(resp.Data) != 0 {
		t.Errorf("expected 0 items")
	}
	meta := resp.Meta.(*OffsetPageMeta)
	if meta.TotalPages != 0 {
		t.Errorf("expected 0 total pages")
	}
}

func TestCursorSetTotalZeroSize(t *testing.T) {
	items := makeTestItems(5)
	req := &CursorPageRequest{Cursor: "cur", Direction: CursorForward, Size: 0}
	resp := BuildCursorResponse(items, req, itemCursor, false, false)

	err := resp.SetTotal(int64(100))
	if err != nil {
		t.Fatalf("SetTotal should not fail for zero page size")
	}

	meta := resp.Meta.(*CursorPageMeta)
	if meta.TotalCount == nil || *meta.TotalCount != 100 {
		t.Error("TotalCount should be set")
	}
	if meta.TotalPages != nil {
		t.Error("TotalPages should be nil when PageSize is zero")
	}
}

func TestOffsetSetTotalZeroPageSize(t *testing.T) {
	items := makeTestItems(5)
	req := &OffsetPageRequest{Page: 1, Size: 0}
	resp := BuildOffsetResponse(items, req, int64(0))

	err := resp.SetTotal(int64(100))
	if err != nil {
		t.Fatalf("SetTotal should not fail")
	}

	meta := resp.Meta.(*OffsetPageMeta)
	if meta.TotalCount != 100 {
		t.Errorf("expected TotalCount 100, got %d", meta.TotalCount)
	}
	if meta.TotalPages != 0 {
		t.Errorf("expected TotalPages 0 for zero PageSize, got %d", meta.TotalPages)
	}
}

func TestCursorResponsePreservesInputItems(t *testing.T) {
	items := makeTestItems(7)
	req, _ := NewCursorPageRequest("", CursorForward, 7)

	resp := BuildCursorResponse(items, req, itemCursor, false, false)

	for i, item := range resp.Data {
		if item.ID != items[i].ID || item.Name != items[i].Name || item.Rank != items[i].Rank {
			t.Errorf("item %d mismatch: expected %v, got %v", i, items[i], item)
		}
	}
}

func TestOffsetResponsePreservesInputItems(t *testing.T) {
	items := makeTestItems(8)
	req, _ := NewOffsetPageRequest(2, 8)

	resp := BuildOffsetResponse(items, req, int64(24))

	for i, item := range resp.Data {
		if item.ID != items[i].ID {
			t.Errorf("item %d ID mismatch", i)
		}
	}
}

func TestFullCursorPaginationWorkflow(t *testing.T) {
	allItems := makeTestItems(25)
	pageSize := 10

	req1, _ := NewCursorPageRequest("", CursorForward, pageSize)
	page1Items := allItems[0:10]
	hasMore1 := len(allItems) > 10

	resp1 := BuildCursorResponseWithTotal(page1Items, req1, itemCursor, hasMore1, false, int64(len(allItems)))

	if !resp1.Success {
		t.Fatal("page1 response should succeed")
	}
	meta1 := resp1.Meta.(*CursorPageMeta)
	if meta1.EndCursor == "" {
		t.Fatal("page1 should have EndCursor")
	}
	if !meta1.HasNextPage {
		t.Error("page1 should have next page")
	}
	if meta1.HasPrevPage {
		t.Error("page1 (first) should not have prev page")
	}
	if *meta1.TotalCount != 25 {
		t.Errorf("expected total 25, got %d", *meta1.TotalCount)
	}
	if *meta1.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", *meta1.TotalPages)
	}

	req2, _ := NewCursorPageRequest(meta1.EndCursor, CursorForward, pageSize)
	page2Items := allItems[10:20]
	resp2 := BuildCursorResponse(page2Items, req2, itemCursor, true, true)

	meta2 := resp2.Meta.(*CursorPageMeta)
	if !meta2.HasNextPage {
		t.Error("page2 should have next page")
	}
	if !meta2.HasPrevPage {
		t.Error("page2 should have prev page")
	}

	req3, _ := NewCursorPageRequest(meta2.EndCursor, CursorForward, pageSize)
	page3Items := allItems[20:25]
	resp3 := BuildCursorResponse(page3Items, req3, itemCursor, false, true)

	meta3 := resp3.Meta.(*CursorPageMeta)
	if meta3.HasNextPage {
		t.Error("page3 (last) should not have next page")
	}
	if !meta3.HasPrevPage {
		t.Error("page3 should have prev page")
	}
	if len(resp3.Data) != 5 {
		t.Errorf("last page should have 5 items, got %d", len(resp3.Data))
	}
}

func TestFullOffsetPaginationWorkflow(t *testing.T) {
	allItems := makeTestItems(25)
	totalCount := int64(len(allItems))
	pageSize := 10

	req1, _ := NewOffsetPageRequest(1, pageSize)
	resp1 := BuildOffsetResponse(allItems[0:10], req1, totalCount)
	meta1 := resp1.Meta.(*OffsetPageMeta)
	if meta1.CurrentPage != 1 {
		t.Errorf("expected page 1, got %d", meta1.CurrentPage)
	}
	if meta1.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", meta1.TotalPages)
	}
	if meta1.HasPrevPage {
		t.Error("page1 should not have prev")
	}
	if !meta1.HasNextPage {
		t.Error("page1 should have next")
	}
	nav1 := resp1.Nav.(*OffsetNav)
	if nav1.NextPage == nil || *nav1.NextPage != 2 {
		t.Error("page1 nav: expected next page 2")
	}

	req2, _ := NewOffsetPageRequest(2, pageSize)
	resp2 := BuildOffsetResponse(allItems[10:20], req2, totalCount)
	meta2 := resp2.Meta.(*OffsetPageMeta)
	if !meta2.HasPrevPage || !meta2.HasNextPage {
		t.Error("page2 should have both prev and next")
	}
	nav2 := resp2.Nav.(*OffsetNav)
	if *nav2.PrevPage != 1 || *nav2.NextPage != 3 {
		t.Error("page2 nav incorrect")
	}

	req3, _ := NewOffsetPageRequest(3, pageSize)
	resp3 := BuildOffsetResponse(allItems[20:25], req3, totalCount)
	meta3 := resp3.Meta.(*OffsetPageMeta)
	if !meta3.HasPrevPage || meta3.HasNextPage {
		t.Error("page3 nav flags incorrect")
	}
	nav3 := resp3.Nav.(*OffsetNav)
	if *nav3.PrevPage != 2 || nav3.NextPage != nil {
		t.Error("page3 nav incorrect")
	}

	req4, _ := NewOffsetPageRequest(4, pageSize)
	resp4 := BuildOffsetResponse(allItems[0:0], req4, totalCount)
	if len(resp4.Data) != 0 {
		t.Error("page beyond total should return empty data")
	}
	meta4 := resp4.Meta.(*OffsetPageMeta)
	if meta4.CurrentPage != 4 {
		t.Errorf("current page should still be 4, got %d", meta4.CurrentPage)
	}
}
