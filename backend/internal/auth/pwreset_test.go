package auth

import "testing"

func TestGenerateResetToken_UniqueAndHashed(t *testing.T) {
	raw1, hash1, err := GenerateResetToken()
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	raw2, hash2, _ := GenerateResetToken()
	if raw1 == raw2 || hash1 == hash2 {
		t.Fatalf("tokens/hashes must be unique")
	}
	if hash1 != HashResetToken(raw1) {
		t.Fatalf("hash not stable for raw token")
	}
	if raw1 == hash1 {
		t.Fatalf("raw token must not equal its hash")
	}
}

func TestHashResetToken_Deterministic(t *testing.T) {
	if HashResetToken("abc") != HashResetToken("abc") {
		t.Fatalf("hash must be deterministic")
	}
	if len(HashResetToken("abc")) != 64 {
		t.Fatalf("expected 64 hex chars for sha256")
	}
}
