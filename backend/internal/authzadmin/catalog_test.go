package authzadmin

import "testing"

func TestCatalog_NoDuplicatesAndLabeled(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range permissionCatalog {
		if g.Group == "" {
			t.Error("group name must not be empty")
		}
		if len(g.Items) == 0 {
			t.Errorf("group %q has no items", g.Group)
		}
		for _, it := range g.Items {
			if it.Key == "" || it.Label == "" {
				t.Errorf("item in %q missing key or label: %+v", g.Group, it)
			}
			if seen[it.Key] {
				t.Errorf("duplicate permission key: %s", it.Key)
			}
			seen[it.Key] = true
		}
	}
}

func TestIsKnownPermission(t *testing.T) {
	for _, k := range []string{"asset.view", "asset.manage", "role.manage", "request.decide", "approval.config.manage", "disposal.view", "disposal.manage"} {
		if !IsKnownPermission(k) {
			t.Errorf("%s should be known", k)
		}
	}
	for _, k := range []string{"asset.create", "request.approve", "bogus.key", ""} {
		if IsKnownPermission(k) {
			t.Errorf("%s should NOT be known", k)
		}
	}
}

func TestCatalogResponse_Shape(t *testing.T) {
	r := CatalogResponse()
	if _, ok := r["permissions"]; !ok {
		t.Error("missing permissions")
	}
	levels, _ := r["scope_levels"].([]string)
	if len(levels) != 4 {
		t.Errorf("want 4 scope levels, got %v", levels)
	}
	mods, _ := r["scope_modules"].([]string)
	if len(mods) == 0 || mods[0] != "*" {
		t.Errorf("scope_modules should start with '*', got %v", mods)
	}
}

func TestCatalog_DepreciationPermissions(t *testing.T) {
	if !IsKnownPermission("depreciation.view") {
		t.Fatal("depreciation.view must be a known permission")
	}
	if !IsKnownPermission("depreciation.manage") {
		t.Fatal("depreciation.manage must be a known permission")
	}
	found := false
	for _, m := range ScopeModules() {
		if m == "depreciation" {
			found = true
		}
	}
	if !found {
		t.Fatal("scope module 'depreciation' missing")
	}
	// The key must not be duplicated (it used to live in the Cadangan group).
	count := 0
	for _, g := range permissionCatalog {
		for _, it := range g.Items {
			if it.Key == "depreciation.manage" {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("depreciation.manage appears %d times, want 1", count)
	}
}

// enforcedPermissionKeys is every permission key the app actually gates on via
// middleware.RequirePermission (router.go) or Service.PermissionKey (importer).
// Each MUST be assignable through the RBAC admin catalog, otherwise a Superadmin
// can neither grant it nor re-save any role that already holds it (SetRolePermissions
// → dedupePermissions rejects uncataloged keys with ErrUnknownPermission).
var enforcedPermissionKeys = []string{
	"user.manage",
	"role.manage", "scope.manage", "fieldperm.manage",
	"audit.view",
	"masterdata.global.manage", "masterdata.office.manage", "masterdata.employee.manage",
	"asset.view", "asset.manage",
	"request.create", "request.decide", "approval.config.manage",
	"transfer.view", "transfer.manage",
	"disposal.view", "disposal.manage",
	"stockopname.view", "stockopname.manage",
	"depreciation.view", "depreciation.manage",
	"assignment.view", "assignment.manage",
	"maintenance.view", "maintenance.manage",
	"report.view", "report.export",
	"valuation.exclude.approve",
}

// TestCatalog_CoversEveryEnforcedKey guards against the class of gap where a key
// is enforced + seeded in a migration but never added to the assignable catalog.
func TestCatalog_CoversEveryEnforcedKey(t *testing.T) {
	for _, k := range enforcedPermissionKeys {
		if !IsKnownPermission(k) {
			t.Errorf("enforced permission %q is not in the assignable catalog", k)
		}
	}
}

func TestCatalog_StockOpnamePermissions(t *testing.T) {
	if !IsKnownPermission("stockopname.view") || !IsKnownPermission("stockopname.manage") {
		t.Fatal("stockopname.view/manage must be known permissions")
	}
	found := false
	for _, m := range ScopeModules() {
		if m == "stockopname" {
			found = true
		}
	}
	if !found {
		t.Fatal("scope module 'stockopname' missing")
	}
}

func TestCatalog_EmployeeImportPermission(t *testing.T) {
	if !IsKnownPermission("masterdata.employee.manage") {
		t.Fatal("masterdata.employee.manage must be a known permission (gates employee batch import)")
	}
}

func TestCatalog_ReportScopeModule(t *testing.T) {
	if !IsKnownPermission("report.view") {
		t.Fatal("report.view must be a known permission")
	}
	if !IsKnownPermission("report.export") {
		t.Fatal("report.export must be a known permission")
	}
	found := false
	for _, m := range ScopeModules() {
		if m == "report" {
			found = true
		}
	}
	if !found {
		t.Fatal("scope module 'report' missing")
	}
}
