package link

import (
	"strings"
	"testing"
)

func TestGenerateCode(t *testing.T) {
	for _, n := range []int{1, 7, 32} {
		code := generateCode(n)
		if len(code) != n {
			t.Errorf("generateCode(%d) length = %d", n, len(code))
		}
		for _, r := range code {
			if !strings.ContainsRune(alphabet, r) {
				t.Errorf("generateCode(%d) = %q contains %q outside alphabet", n, code, r)
			}
		}
	}
}

func TestGenerateCodeUnique(t *testing.T) {
	seen := make(map[string]bool)
	for range 10000 {
		code := generateCode(codeLength)
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
	}
}
