package auth

import (
	"strings"
	"testing"
)

func TestHashToken(t *testing.T) {
	if h := hashToken("abc"); len(h) != 64 {
		t.Errorf("hash length = %d", len(h))
	}
	if hashToken("abc") != hashToken("abc") {
		t.Error("hash not deterministic")
	}
	if hashToken("abc") == hashToken("abd") {
		t.Error("hash collision on different input")
	}
}

func TestGenerateRandomTokenShape(t *testing.T) {
	first, err := generateRandomToken()
	if err != nil {
		t.Fatalf("generateRandomToken: %v", err)
	}
	second, err := generateRandomToken()
	if err != nil {
		t.Fatalf("generateRandomToken: %v", err)
	}
	for name, tok := range map[string]string{"first": first, "second": second} {
		if len(tok) != 43 {
			t.Errorf("%s token length = %d, want 43", name, len(tok))
		}
		if strings.Contains(tok, "=") {
			t.Errorf("%s token contains '=' padding", name)
		}
		for _, r := range tok {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				t.Errorf("%s token contains disallowed character %q", name, r)
			}
		}
	}
	if first == second {
		t.Error("two calls returned the same token")
	}
}
