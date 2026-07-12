package importer

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// TestRowToMap_SkipsUnderscoreKeys documents contract (a): the row's data
// jsonb may carry internal, importer-resolved fields (e.g. "_office_id")
// stamped by ValidateRows for the target's own Execute phase — those must
// never reach the API response, only the user-facing columns should.
func TestRowToMap_SkipsUnderscoreKeys(t *testing.T) {
	data, err := json.Marshal(map[string]string{
		"nama":       "Meja Kerja",
		"_office_id": "11111111-1111-1111-1111-111111111111",
	})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	row := sqlc.ImportImportRow{
		ID:    uuid.New(),
		RowNo: 3,
		Data:  data,
		Valid: true,
	}

	m := rowToMap(row)

	if m["nama"] != "Meja Kerja" {
		t.Fatalf("expected nama to be exposed, got %v", m["nama"])
	}
	if _, ok := m["_office_id"]; ok {
		t.Fatal("_office_id must not be exposed in the response map")
	}
	if m["row_no"] != int32(3) {
		t.Fatalf("row_no mismatch: got %v", m["row_no"])
	}
	if m["valid"] != true {
		t.Fatalf("valid mismatch: got %v", m["valid"])
	}
	if _, ok := m["errors"]; !ok {
		t.Fatal("errors key must always be present (even if empty)")
	}
}

// TestRowToMap_ResultRef covers the result_ref pass-through when set.
func TestRowToMap_ResultRef(t *testing.T) {
	ref := "AST-0001"
	row := sqlc.ImportImportRow{ID: uuid.New(), RowNo: 1, Valid: true, ResultRef: &ref}
	m := rowToMap(row)
	if m["result_ref"] != ref {
		t.Fatalf("result_ref mismatch: got %v", m["result_ref"])
	}
}

// TestJobToMap asserts the response shape includes the documented fields and
// never leaks internal storage plumbing (object_key) or soft-delete columns.
func TestJobToMap(t *testing.T) {
	oid := uuid.New()
	objectKey := "imports/x/aset.xlsx"
	job := sqlc.ImportImportJob{
		ID:          uuid.New(),
		Target:      "asset",
		Format:      "xlsx",
		Filename:    "aset.xlsx",
		ObjectKey:   &objectKey,
		Status:      sqlc.SharedImportStatusValidated,
		TotalRows:   10,
		SuccessRows: 8,
		FailedRows:  2,
		CreatedByID: uuid.New(),
		OfficeID:    &oid,
	}

	m := jobToMap(job)

	for _, key := range []string{"id", "target", "status", "total_rows", "success_rows", "failed_rows"} {
		if _, ok := m[key]; !ok {
			t.Fatalf("missing key %q in jobToMap output", key)
		}
	}
	if m["target"] != "asset" {
		t.Fatalf("target mismatch: got %v", m["target"])
	}
	if m["status"] != "validated" {
		t.Fatalf("status mismatch: got %v", m["status"])
	}
	if m["total_rows"] != int32(10) || m["success_rows"] != int32(8) || m["failed_rows"] != int32(2) {
		t.Fatalf("count fields mismatch: %v %v %v", m["total_rows"], m["success_rows"], m["failed_rows"])
	}
	if _, ok := m["object_key"]; ok {
		t.Fatal("object_key must never be exposed (internal storage plumbing)")
	}
	if _, ok := m["deleted_at"]; ok {
		t.Fatal("deleted_at must never be exposed")
	}
}

// TestSvcError exercises the sentinel -> HTTP status mapping (contract h).
func TestSvcError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not-found", ErrNotFound, http.StatusNotFound},
		{"forbidden", ErrForbidden, http.StatusForbidden},
		{"unknown-target", ErrUnknownTarget, http.StatusUnprocessableEntity},
		{"bad-state", ErrBadState, http.StatusConflict},
		{"conflict", ErrConflict, http.StatusConflict},
		{"bad-format", ErrBadFormat, http.StatusBadRequest},
		{"unmapped", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			h.svcError(c, tc.err)
			if w.Code != tc.want {
				t.Errorf("%v: got status %d want %d", tc.err, w.Code, tc.want)
			}
		})
	}
}
