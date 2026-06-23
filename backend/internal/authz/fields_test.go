package authz

import "testing"

func TestFilterView(t *testing.T) {
	data := map[string]any{
		"id":            "abc",
		"name":          "Laptop",
		"purchase_cost": 1500,
		"book_value":    1200,
	}
	policies := map[string]FieldPolicy{
		"purchase_cost": {CanView: false},
		"book_value":    {CanView: false},
		"name":          {CanView: true},
		// "id" has no policy -> stays visible (default-allow)
	}

	FilterView(policies, data)

	if _, ok := data["purchase_cost"]; ok {
		t.Error("purchase_cost should be filtered out")
	}
	if _, ok := data["book_value"]; ok {
		t.Error("book_value should be filtered out")
	}
	if _, ok := data["name"]; !ok {
		t.Error("name should remain (can_view=true)")
	}
	if _, ok := data["id"]; !ok {
		t.Error("id should remain (no policy = default-allow)")
	}
}
