// Package importer implements the generic bulk-import engine shared by all
// import targets (assets, employees, offices, reference data, ...). This
// file defines the core value types and the TargetImporter contract that
// each per-domain importer implements, plus the in-memory registry used to
// look targets up by name.
package importer

import (
	"context"
	"sort"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// ColumnSpec describes one column of a target's import template: its name,
// whether it is required, and its value kind (used for parsing/validation).
// Kind is one of "text", "date", "decimal", or "lookup".
type ColumnSpec struct {
	Name     string
	Required bool
	Kind     string // "text" | "date" | "decimal" | "lookup"
}

// RawRow is a single row as read from the uploaded file, before validation:
// the 1-based row number (for error reporting) and the raw cell values keyed
// by column name.
type RawRow struct {
	RowNo int
	Cells map[string]string
}

// CellError describes a validation failure on a single cell of a row.
type CellError struct {
	Column   string `json:"column"`
	ErrorKey string `json:"error_key"`
}

// RowResult is the outcome of validating one RawRow: whether it is valid,
// its normalized data (on success), any per-cell errors, and a
// human-readable reference (e.g. a code/name) used in error reporting and
// duplicate detection.
type RowResult struct {
	RowNo         int
	Valid         bool
	Data          map[string]string
	Errors        []CellError
	NormalizedRef string
}

// Scope carries the caller's data-scope resolution (see internal/authz) into
// validation/execution so a target can enforce per-row visibility.
type Scope struct {
	AllScope  bool
	OfficeIDs []uuid.UUID
	UserID    uuid.UUID
}

// Job represents a bulk-import job record.
type Job struct {
	ID        uuid.UUID
	Target    string
	Format    string
	Filename  string
	OfficeID  *uuid.UUID
	TotalRows int
}

// Row is a validated, persisted row ready for execution (insertion) into the
// target's domain tables.
type Row struct {
	ID    uuid.UUID
	RowNo int
	Data  map[string]string
}

// TargetImporter is implemented by each per-domain import target (asset,
// employee, office, reference data, ...). The generic engine (parser,
// template generator, job service, worker) drives targets purely through
// this interface.
type TargetImporter interface {
	// Target returns the target's registry key (e.g. "asset").
	Target() string
	// Columns describes the template columns this target expects.
	Columns() []ColumnSpec
	// ValidateRows validates raw rows against the target's business rules,
	// returning one RowResult per input row (same order/length as rows).
	ValidateRows(ctx context.Context, rows []RawRow, scope Scope) ([]RowResult, error)
	// Execute persists the previously-validated rows within the given
	// transaction-scoped queries, returning the number of records created.
	Execute(ctx context.Context, qtx *sqlc.Queries, job Job, validRows []Row) (created int, err error)
	// NeedsApproval reports whether imports for this target must go through
	// the maker-checker approval flow before Execute runs.
	NeedsApproval() bool
}

// registry maps a target's registry key to its TargetImporter implementation.
type registry map[string]TargetImporter

// get looks up a TargetImporter by its registry key.
func (r registry) get(target string) (TargetImporter, bool) {
	t, ok := r[target]
	return t, ok
}

// targets returns the registered target keys, sorted for stable output.
func (r registry) targets() []string {
	out := make([]string, 0, len(r))
	for k := range r {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
