package handlers

import (
	"testing"
)

func TestValidatePageParamsFeed(t *testing.T) {
	tests := []struct {
		name           string
		page           int
		perPage        int
		defaultPerPage int
		maxPerPage     int
		expectedPage   int
		expectedPerPage int
	}{
		{"valid", 1, 50, 50, 100, 1, 50},
		{"page zero", 0, 50, 50, 100, 1, 50},
		{"per_page too large", 1, 200, 50, 100, 1, 100},
		{"page negative", -5, 50, 50, 100, 1, 50},
		{"per_page zero", 1, 0, 50, 100, 1, 50},
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