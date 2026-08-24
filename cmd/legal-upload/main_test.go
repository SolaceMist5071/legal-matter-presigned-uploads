package main

import (
	"errors"
	"testing"

	"example.com/legal-browser-uploads/internal/infrai"
)

func TestIsBucketExists(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"matching API code", &infrai.APIError{Status: 409, Code: "STORAGE_BUCKET_EXISTS"}, true},
		{"other API code", &infrai.APIError{Status: 409, Code: "STORAGE_CONFLICT"}, false},
		{"non-API error", errors.New("network failure"), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isBucketExists(test.err); got != test.want {
				t.Fatalf("isBucketExists() = %v, want %v", got, test.want)
			}
		})
	}
}
