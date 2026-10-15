package pagination

import (
	"net/url"
	"slices"
	"testing"
)

func TestNewNormalizes(t *testing.T) {
	tests := []struct {
		name                 string
		number, size, total  int
		wantNumber, wantSize int
		wantTotal            int
	}{
		{"valid input is kept", 3, 10, 95, 3, 10, 95},
		{"page below one", 0, 10, 5, 1, 10, 5},
		{"negative page", -4, 10, 5, 1, 10, 5},
		{"zero size gets the default", 1, 0, 5, 1, DefaultSize, 5},
		{"huge size is capped", 1, 5000, 5, 1, MaxSize, 5},
		{"negative total", 1, 10, -3, 1, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.number, tt.size, tt.total)
			if p.Number != tt.wantNumber || p.Size != tt.wantSize || p.Total != tt.wantTotal {
				t.Errorf("New = %+v, want {%d %d %d}", p, tt.wantNumber, tt.wantSize, tt.wantTotal)
			}
		})
	}
}

func TestPageMath(t *testing.T) {
	tests := []struct {
		name               string
		page               Page
		offset, totalPages int
		hasPrev, hasNext   bool
		from, to           int
	}{
		{"first of several", New(1, 20, 95), 0, 5, false, true, 1, 20},
		{"middle", New(3, 20, 95), 40, 5, true, true, 41, 60},
		{"last partial page", New(5, 20, 95), 80, 5, true, false, 81, 95},
		{"exact multiple", New(2, 10, 20), 10, 2, true, false, 11, 20},
		{"empty collection", New(1, 10, 0), 0, 0, false, false, 0, 0},
		{"past the end", New(9, 10, 25), 80, 3, true, false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.page
			if p.Offset() != tt.offset || p.Limit() != p.Size || p.TotalPages() != tt.totalPages {
				t.Errorf("Offset/Limit/TotalPages = %d/%d/%d, want %d/%d/%d",
					p.Offset(), p.Limit(), p.TotalPages(), tt.offset, p.Size, tt.totalPages)
			}
			if p.HasPrev() != tt.hasPrev || p.HasNext() != tt.hasNext {
				t.Errorf("HasPrev/HasNext = %v/%v, want %v/%v", p.HasPrev(), p.HasNext(), tt.hasPrev, tt.hasNext)
			}
			if from, to := p.Range(); from != tt.from || to != tt.to {
				t.Errorf("Range = %d-%d, want %d-%d", from, to, tt.from, tt.to)
			}
		})
	}
}

func TestWindow(t *testing.T) {
	tests := []struct {
		name string
		page Page
		n    int
		want []int
	}{
		{"start of the list", New(1, 10, 100), 5, []int{1, 2, 3, 4, 5}},
		{"centered", New(5, 10, 100), 5, []int{3, 4, 5, 6, 7}},
		{"end of the list", New(10, 10, 100), 5, []int{6, 7, 8, 9, 10}},
		{"fewer pages than the window", New(2, 10, 25), 5, []int{1, 2, 3}},
		{"even window", New(5, 10, 100), 4, []int{3, 4, 5, 6}},
		{"empty", New(1, 10, 0), 5, nil},
		{"invalid window size", New(1, 10, 100), 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.page.Window(tt.n); !slices.Equal(got, tt.want) {
				t.Errorf("Window(%d) = %v, want %v", tt.n, got, tt.want)
			}
		})
	}
}

func TestSlice(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}
	tests := []struct {
		page Page
		want []int
	}{
		{New(1, 3, 7), []int{1, 2, 3}},
		{New(2, 3, 7), []int{4, 5, 6}},
		{New(3, 3, 7), []int{7}},
		{New(4, 3, 7), nil},
	}
	for _, tt := range tests {
		if got := Slice(items, tt.page); !slices.Equal(got, tt.want) {
			t.Errorf("Slice(page %d) = %v, want %v", tt.page.Number, got, tt.want)
		}
	}
}

func TestFromQuery(t *testing.T) {
	tests := []struct {
		query        string
		number, size int
	}{
		{"page=3&per_page=50", 3, 50},
		{"", 1, DefaultSize},
		{"page=abc&per_page=-5", 1, DefaultSize},
		{"page=2&per_page=1000", 2, MaxSize},
	}
	for _, tt := range tests {
		q, _ := url.ParseQuery(tt.query)
		if p := FromQuery(q, 500); p.Number != tt.number || p.Size != tt.size || p.Total != 500 {
			t.Errorf("FromQuery(%q) = %+v, want number %d size %d", tt.query, p, tt.number, tt.size)
		}
	}
}
