// Package authzadmin implements Superadmin management of the configurable
// authorization layer: roles, role-permissions, data-scope policies, and
// field-permissions, with Redis cache invalidation on every change.
package authzadmin

// PermissionItem is one assignable permission key with a human label.
type PermissionItem struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// PermissionGroup groups related permission keys for display.
type PermissionGroup struct {
	Group string           `json:"group"`
	Items []PermissionItem `json:"items"`
}

// permissionCatalog is the canonical source of truth for assignable permission
// keys. Keys here must match what the code enforces via RequirePermission; the
// "Cadangan" group reserves keys for modules not yet built so the seed can grant
// them forward-looking without failing validation.
var permissionCatalog = []PermissionGroup{
	{Group: "Sistem", Items: []PermissionItem{
		{"user.manage", "Kelola user"},
		{"role.manage", "Kelola peran & RBAC"},
		{"scope.manage", "Kelola data scope"},
		{"fieldperm.manage", "Kelola field permission"},
		{"audit.view", "Lihat audit trail"},
	}},
	{Group: "Master Data", Items: []PermissionItem{
		{"masterdata.global.manage", "Kelola master data global"},
		{"masterdata.office.manage", "Kelola kantor & pegawai"},
	}},
	{Group: "Aset", Items: []PermissionItem{
		{"asset.view", "Lihat aset"},
		{"asset.manage", "Kelola aset"},
	}},
	{Group: "Persetujuan", Items: []PermissionItem{
		{"request.create", "Buat pengajuan"},
		{"request.decide", "Setujui/tolak pengajuan"},
		{"approval.config.manage", "Kelola ambang persetujuan"},
	}},
	{Group: "Cadangan", Items: []PermissionItem{
		{"report.view", "Lihat laporan"},
		{"report.export", "Ekspor laporan"},
		{"maintenance.manage", "Kelola maintenance"},
		{"depreciation.manage", "Kelola penyusutan"},
		{"valuation.exclude.approve", "Setujui pengecualian valuasi"},
		{"assignment.manage", "Kelola penugasan aset"},
	}},
}

// knownPermissions is the flattened set of catalog keys for O(1) validation.
var knownPermissions = func() map[string]bool {
	m := map[string]bool{}
	for _, g := range permissionCatalog {
		for _, it := range g.Items {
			m[it.Key] = true
		}
	}
	return m
}()

// IsKnownPermission reports whether key is an assignable catalog permission.
func IsKnownPermission(key string) bool { return knownPermissions[key] }

// ScopeLevels returns the valid data-scope levels (matches shared.scope_level enum).
func ScopeLevels() []string {
	return []string{"global", "office_subtree", "office", "own"}
}

// ScopeModules returns the known data-scope module strings the handlers resolve
// scope for, plus the '*' default sentinel.
func ScopeModules() []string {
	return []string{"*", "offices", "employees", "assets", "requests"}
}

// CatalogResponse is the GET /authz/catalog payload for the admin UI.
func CatalogResponse() map[string]any {
	return map[string]any{
		"permissions":   permissionCatalog,
		"scope_levels":  ScopeLevels(),
		"scope_modules": ScopeModules(),
	}
}
