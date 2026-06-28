package authzadmin

import (
	"errors"
	"testing"
)

func TestDedupePermissions_RejectsUnknownAndDeduplicates(t *testing.T) {
	out, err := dedupePermissions([]string{"asset.view", "asset.view", "role.manage"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("want 2 deduped, got %v", out)
	}
	if _, err := dedupePermissions([]string{"asset.view", "bogus.key"}); !errors.Is(err, ErrUnknownPermission) {
		t.Fatalf("want ErrUnknownPermission, got %v", err)
	}
}

func TestValidateScopePolicies(t *testing.T) {
	ok, err := validateScopePolicies([]ScopePolicyInput{{Module: "*", ScopeLevel: "global"}, {Module: "assets", ScopeLevel: "own"}})
	if err != nil || len(ok) != 2 {
		t.Fatalf("expected 2 valid, got %v err=%v", ok, err)
	}
	// invalid level
	if _, err := validateScopePolicies([]ScopePolicyInput{{Module: "*", ScopeLevel: "nope"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for bad level, got %v", err)
	}
	// empty module
	if _, err := validateScopePolicies([]ScopePolicyInput{{Module: "", ScopeLevel: "own"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for empty module, got %v", err)
	}
	// duplicate module
	if _, err := validateScopePolicies([]ScopePolicyInput{{Module: "assets", ScopeLevel: "own"}, {Module: "assets", ScopeLevel: "global"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for dup module, got %v", err)
	}
}

func TestValidateFieldPerms(t *testing.T) {
	if _, err := validateFieldPerms([]FieldPermInput{{Entity: "", Field: "x", CanView: true}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for empty entity, got %v", err)
	}
	if _, err := validateFieldPerms([]FieldPermInput{{Entity: "assets", Field: "cost"}, {Entity: "assets", Field: "cost"}}); !errors.Is(err, ErrValidation) {
		t.Fatalf("want ErrValidation for dup (entity,field), got %v", err)
	}
	ok, err := validateFieldPerms([]FieldPermInput{{Entity: "assets", Field: "purchase_cost", CanView: false}})
	if err != nil || len(ok) != 1 {
		t.Fatalf("expected 1 valid, got %v err=%v", ok, err)
	}
}
