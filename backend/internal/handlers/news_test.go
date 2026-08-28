package handlers

import (
	"testing"
)

func TestValidatePageParams(t *testing.T) {
	tests := []struct {
		name            string
		page            int
		perPage         int
		defaultPerPage  int
		maxPerPage      int
		expectedPage    int
		expectedPerPage int
	}{
		{"normal", 1, 30, 30, 100, 1, 30},
		{"page zero", 0, 30, 30, 100, 1, 30},
		{"page negative", -5, 30, 30, 100, 1, 30},
		{"per_page zero", 1, 0, 30, 100, 1, 30},
		{"per_page negative", 1, -10, 30, 100, 1, 30},
		{"per_page exceeds max", 1, 200, 30, 100, 1, 100},
		{"page exceeds max", 999, 30, 30, 100, 999, 30},
		{"custom max", 1, 200, 20, 50, 1, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, perPage := validatePageParams(tt.page, tt.perPage, tt.defaultPerPage, tt.maxPerPage)
			if page != tt.expectedPage {
				t.Errorf("page: expected %d, got %d", tt.expectedPage, page)
			}
			if perPage != tt.expectedPerPage {
				t.Errorf("perPage: expected %d, got %d", tt.expectedPerPage, perPage)
			}
		})
	}
}