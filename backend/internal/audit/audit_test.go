package audit

import "testing"

type sample struct {
	Name      string `json:"name"`
	Level     int    `json:"level"`
	UpdatedAt string `json:"updated_at"`
}

func TestDiffCreate(t *testing.T) {
	d := Diff(nil, sample{Name: "Lantai 1", Level: 1})
	if got, ok := d["name"]; !ok || got["after"] != "Lantai 1" || got["before"] != nil {
		t.Fatalf("create diff for name = %#v, want only after", got)
	}
	if _, ok := d["level"]; !ok {
		t.Fatalf("create diff missing level: %#v", d)
	}
	if _, ok := d["before"]; ok {
		t.Fatalf("create diff should not invent a 'before' field")
	}
}

func TestDiffDelete(t *testing.T) {
	d := Diff(sample{Name: "Lantai 1", Level: 1}, nil)
	got, ok := d["name"]
	if !ok || got["before"] != "Lantai 1" {
		t.Fatalf("delete diff for name = %#v, want before set", got)
	}
	if _, has := got["after"]; has {
		t.Fatalf("delete diff should not have an 'after' field: %#v", got)
	}
}

func TestDiffUpdateOnlyChangedFields(t *testing.T) {
	before := sample{Name: "Lantai 1", Level: 1}
	after := sample{Name: "Lantai 1", Level: 2}
	d := Diff(before, after)

	if _, ok := d["name"]; ok {
		t.Fatalf("unchanged 'name' should not appear in diff: %#v", d)
	}
	got, ok := d["level"]
	if !ok {
		t.Fatalf("changed 'level' missing from diff: %#v", d)
	}
	// JSON round-trips numbers to float64.
	if got["before"] != float64(1) || got["after"] != float64(2) {
		t.Fatalf("level diff = %#v, want before 1 / after 2", got)
	}
}

func TestDiffIgnoresTimestamps(t *testing.T) {
	before := sample{Name: "Lantai 1", UpdatedAt: "2026-06-01T00:00:00Z"}
	after := sample{Name: "Lantai 1", UpdatedAt: "2026-06-26T00:00:00Z"}
	d := Diff(before, after)
	if len(d) != 0 {
		t.Fatalf("only updated_at changed → diff should be empty, got %#v", d)
	}
}

func TestValidAction(t *testing.T) {
	for _, a := range []string{"create", "update", "delete"} {
		if !validAction(a) {
			t.Errorf("validAction(%q) = false, want true", a)
		}
	}
	for _, a := range []string{"", "read", "list", "CREATE"} {
		if validAction(a) {
			t.Errorf("validAction(%q) = true, want false", a)
		}
	}
}
