package importer

import (
	"testing"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// TestPermissionKey exercises the pure target->permission-key mapping. It
// registers stub targets first to document that PermissionKey maps by target
// string alone — it does not require (or consult) the registry.
func TestPermissionKey(t *testing.T) {
	s := &Service{reg: registry{}}
	s.reg["asset"] = stubTarget{name: "asset"}
	s.reg["reference:cities"] = stubTarget{name: "reference:cities"}

	cases := map[string]string{
		"asset":            "asset.manage",
		"employee":         "masterdata.employee.manage",
		"office":           "masterdata.office.manage",
		"reference:cities": "masterdata.global.manage",
	}
	for target, want := range cases {
		got, err := s.PermissionKey(target)
		if err != nil || got != want {
			t.Fatalf("%s: got %q err %v", target, got, err)
		}
	}

	if _, err := s.PermissionKey("ghost"); err != ErrUnknownTarget {
		t.Fatalf("want ErrUnknownTarget, got %v", err)
	}
}

// TestAssertOwner covers the ownership check used by the handler (and by the
// service's own job methods) to gate access to a job by its creator.
func TestAssertOwner(t *testing.T) {
	s := &Service{}
	userID := uuid.New()
	job := sqlc.ImportImportJob{CreatedByID: userID}

	if err := s.assertOwner(job, userID); err != nil {
		t.Fatalf("expected nil for matching owner, got %v", err)
	}

	other := uuid.New()
	if err := s.assertOwner(job, other); err != ErrForbidden {
		t.Fatalf("want ErrForbidden for mismatched owner, got %v", err)
	}
}
