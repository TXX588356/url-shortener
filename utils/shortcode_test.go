package utils

import (
	"fmt"
	"strings"
	"testing"
)

func TestGenerateShortCode(t *testing.T) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, length := range []int{4, 8, 12} {
		t.Run(fmt.Sprintf("length_%d", length), func(t *testing.T) {
			code, err := GenerateShortCode(length)
			if err != nil {
				t.Fatalf("GenerateShortCode(%d): %v", length, err)
			}
			if len(code) != length {
				t.Errorf("length = %d, want %d", len(code), length)
			}
			for _, c := range code {
				if !strings.ContainsRune(charset, c) {
					t.Errorf("short code %q contains invalid character %q", code, c)
				}
			}
		})
	}
}
