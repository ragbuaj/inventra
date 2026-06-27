package approval

import "testing"

func TestSubmitRequest_Validate(t *testing.T) {
	r := SubmitRequest{Type: "asset_create", Amount: "150000000", OfficeID: "not-a-uuid"}
	if err := r.validate(); err == nil {
		t.Fatal("expected invalid office_id error")
	}
	r.OfficeID = "11111111-1111-1111-1111-111111111111"
	if err := r.validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestThresholdRequest_Validate(t *testing.T) {
	valid := ThresholdRequest{
		RequestType:   "asset_create",
		AmountFrom:    "0",
		RequiredLevel: "office",
		StepOrder:     1,
	}

	// happy path
	if err := valid.validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}

	// invalid request_type
	bad := valid
	bad.RequestType = "unknown_type"
	if err := bad.validate(); err == nil {
		t.Fatal("expected error for invalid request_type")
	}

	// invalid required_level
	bad = valid
	bad.RequiredLevel = "global"
	if err := bad.validate(); err == nil {
		t.Fatal("expected error for invalid required_level")
	}

	// validateUpdate: valid required_level passes regardless of request_type
	upd := ThresholdRequest{RequiredLevel: "pusat"}
	if err := upd.validateUpdate(); err != nil {
		t.Fatalf("expected valid update, got %v", err)
	}

	// validateUpdate: invalid required_level fails
	upd.RequiredLevel = "bad"
	if err := upd.validateUpdate(); err == nil {
		t.Fatal("expected error for invalid required_level in update")
	}
}

func TestThresholdRequest_ValidateAllTypes(t *testing.T) {
	level := "wilayah"
	types := []string{"asset_create", "asset_disposal", "asset_transfer", "assignment", "maintenance", "valuation_exclusion"}
	for _, rt := range types {
		req := ThresholdRequest{RequestType: rt, AmountFrom: "0", RequiredLevel: level, StepOrder: 1}
		if err := req.validate(); err != nil {
			t.Errorf("type %q should be valid, got %v", rt, err)
		}
	}
}

func TestThresholdRequest_ValidateAllLevels(t *testing.T) {
	rt := "asset_create"
	levels := []string{"office", "office_subtree", "wilayah", "pusat"}
	for _, lvl := range levels {
		req := ThresholdRequest{RequestType: rt, AmountFrom: "0", RequiredLevel: lvl, StepOrder: 1}
		if err := req.validate(); err != nil {
			t.Errorf("level %q should be valid, got %v", lvl, err)
		}
	}
}
