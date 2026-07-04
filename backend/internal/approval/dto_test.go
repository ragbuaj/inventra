package approval

import "testing"

func TestSubmitRequest_Validate(t *testing.T) {
	r := SubmitRequest{
		Type:     "asset_create",
		Amount:   "150000000",
		OfficeID: "not-a-uuid",
		Payload:  []byte(`{"purchase_cost":"150000000"}`),
	}
	if err := r.validate(); err == nil {
		t.Fatal("expected invalid office_id error")
	}
	r.OfficeID = "11111111-1111-1111-1111-111111111111"
	if err := r.validate(); err != nil {
		t.Fatalf("expected valid, got %v", err)
	}
}

func TestSubmitRequest_Validate_AssetCreateAmount(t *testing.T) {
	const office = "11111111-1111-1111-1111-111111111111"

	cases := []struct {
		name    string
		amount  string
		payload string
		wantErr bool
	}{
		{"amount equals purchase_cost", "250000000", `{"purchase_cost":"250000000"}`, false},
		{"numeric equality across formats", "1000", `{"purchase_cost":"1000.00"}`, false},
		{"amount understates purchase_cost", "0", `{"purchase_cost":"250000000"}`, true},
		{"amount overstates purchase_cost", "999999999", `{"purchase_cost":"1000"}`, true},
		{"no purchase_cost requires zero amount", "0", `{"name":"x"}`, false},
		{"no purchase_cost rejects nonzero amount", "5000000", `{"name":"x"}`, true},
		{"null purchase_cost requires zero amount", "0", `{"purchase_cost":null}`, false},
		{"nil payload requires zero amount", "0", "", false},
		{"nil payload rejects nonzero amount", "5000000", "", true},
		{"malformed payload rejected", "0", `{not-json`, true},
		{"non-numeric amount rejected", "abc", `{"purchase_cost":"1000"}`, true},
		{"non-numeric purchase_cost rejected", "1000", `{"purchase_cost":"abc"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := SubmitRequest{Type: "asset_create", Amount: tc.amount, OfficeID: office}
			if tc.payload != "" {
				r.Payload = []byte(tc.payload)
			}
			err := r.validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
		})
	}
}

func TestSubmitRequest_Validate_OtherTypesSkipAmountCheck(t *testing.T) {
	const office = "11111111-1111-1111-1111-111111111111"
	target := "22222222-2222-2222-2222-222222222222"
	for _, typ := range []string{"asset_disposal", "valuation_exclusion"} {
		r := SubmitRequest{Type: typ, Amount: "150000000", OfficeID: office, TargetID: &target}
		if err := r.validate(); err != nil {
			t.Errorf("type %q: expected valid without payload cross-check, got %v", typ, err)
		}
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
