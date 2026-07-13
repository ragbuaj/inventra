package auth

import "testing"

func TestGenerateEmailChangeToken_UniqueAndHashed(t *testing.T) {
	raw1, hash1, err := GenerateEmailChangeToken()
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	raw2, hash2, _ := GenerateEmailChangeToken()
	if raw1 == raw2 || hash1 == hash2 {
		t.Fatalf("tokens/hashes must be unique")
	}
	if hash1 != HashEmailChangeToken(raw1) {
		t.Fatalf("hash not stable for raw token")
	}
	if raw1 == hash1 {
		t.Fatalf("raw token must not equal its hash")
	}
}

func TestEmailChangeTokenRoundtrip(t *testing.T) {
	raw, hash, err := GenerateEmailChangeToken()
	if err != nil || raw == "" || hash == "" {
		t.Fatalf("gen: %v", err)
	}
	if HashEmailChangeToken(raw) != hash {
		t.Fatal("hash mismatch")
	}
}

func TestHashEmailChangeToken_Deterministic(t *testing.T) {
	if HashEmailChangeToken("abc") != HashEmailChangeToken("abc") {
		t.Fatalf("hash must be deterministic")
	}
	if len(HashEmailChangeToken("abc")) != 64 {
		t.Fatalf("expected 64 hex chars for sha256")
	}
}
