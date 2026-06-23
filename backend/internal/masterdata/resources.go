package masterdata

// referenceResources declares the simple reference master data served by the
// generic engine. Complex entities (categories, offices, employees) are separate.
var referenceResources = []resource{
	{Path: "office-types", Table: "office_types", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
	{Path: "departments", Table: "departments", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "code", Type: typeText, Search: true},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
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
}
