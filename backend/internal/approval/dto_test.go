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
