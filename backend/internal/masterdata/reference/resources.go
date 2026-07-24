package reference

// referenceResources declares the simple reference master data served by the
// generic engine. Complex entities (categories, offices, employees, floors, rooms)
// live in their own packages.
var referenceResources = []resource{
	{Path: "office-types", Table: "office_types", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "tier", Type: typeEnum, EnumType: "shared.approver_level", Enum: []string{"pusat", "wilayah", "office"}},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	// departments moved off the generic engine to the scoped `department` sub-package
	// (internal/masterdata/department) so data scope is enforced on read AND write —
	// it stays mounted at /departments with the same JSON shape (frontend unchanged).
	{Path: "positions", Table: "positions", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	{Path: "units", Table: "units", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "symbol", Type: typeText},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	{Path: "maintenance-categories", Table: "maintenance_categories", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	{Path: "problem-categories", Table: "problem_categories", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	{Path: "brands", Table: "brands", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	{Path: "vendors", Table: "vendors", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "contact_name", Type: typeText},
		{Name: "phone", Type: typeText},
		{Name: "email", Type: typeText},
		{Name: "address", Type: typeText},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	{Path: "provinces", Table: "provinces", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "code", Type: typeText, Search: true},
	}},
	{Path: "cities", Table: "cities", OrderBy: "name", Columns: []column{
		{Name: "province_id", Type: typeUUID, Required: true},
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "code", Type: typeText, Search: true},
	}},
	{Path: "models", Table: "models", OrderBy: "name", Columns: []column{
		{Name: "brand_id", Type: typeUUID, Required: true},
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	// Legacy-parity Fase 4 masters.
	{Path: "office-classes", Table: "office_classes", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	{Path: "executor-divisions", Table: "executor_divisions", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	{Path: "companies", Table: "companies", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	// max_floors nullable (NULL = "25+"). min_floors is the default ordering.
	{Path: "building-classifications", Table: "building_classifications", OrderBy: "min_floors", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "min_floors", Type: typeInt, Required: true},
		{Name: "max_floors", Type: typeInt},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
}
