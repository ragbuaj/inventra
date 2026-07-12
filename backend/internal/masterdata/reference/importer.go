package reference

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/importer"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Reference import column names. Both targets (provinces, cities) share the
// same "nama"/"kode" naming; cities additionally has "provinsi".
const (
	provColName = "nama"
	provColCode = "kode"

	cityColName     = "nama"
	cityColProvince = "provinsi"
	cityColCode     = "kode"

	brandColName = "nama"

	unitColName   = "nama"
	unitColSymbol = "simbol"

	modelColBrand = "merek"
	modelColName  = "nama"
)

// Service holds the sqlc.Queries needed by the reference target importers
// (provinces, cities). It is deliberately separate from the generic reference
// engine (engine.go), whose pool-based writes cannot join the import worker's
// transaction: target.Execute receives a tx-bound *sqlc.Queries (qtx), and
// the generic engine only ever holds a *pgxpool.Pool. So the reference
// import targets use dedicated sqlc queries (db/queries/reference_import.sql)
// through this Service instead of the engine's InsertTx, exactly like the
// employee/office importers use their own *sqlc.Queries — this keeps the
// anti-poisoning discipline (see Execute below) uniform and transactional
// across every import target.
type Service struct {
	q *sqlc.Queries
}

// NewService constructs the reference-importer service.
func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

// referenceImporter is a reference-target import: it validates a batch of
// province or city rows and creates them directly (no approval). resource
// selects which of the two entities this instance handles. It implements
// importer.TargetImporter.
type referenceImporter struct {
	s        *Service
	resource string // "provinces" | "cities"
}

// NewImporter returns the reference import target for the given resource
// ("provinces" or "cities"), for registration with the generic import engine.
func NewImporter(s *Service, resource string) importer.TargetImporter {
	return referenceImporter{s: s, resource: resource}
}

// Target returns the importer's registry key, namespaced under "reference:"
// (see importer.Service.PermissionKey's "reference:" prefix mapping to
// masterdata.global.manage).
func (r referenceImporter) Target() string { return "reference:" + r.resource }

// NeedsApproval reports that reference (master-data) imports are created
// directly by the worker, without maker-checker approval.
func (referenceImporter) NeedsApproval() bool { return false }

// Columns describes the import template for the importer's resource.
func (r referenceImporter) Columns() []importer.ColumnSpec {
	switch r.resource {
	case "provinces":
		return []importer.ColumnSpec{
			{Name: provColName, Required: true, Kind: "text"},
			{Name: provColCode, Required: false, Kind: "text"},
		}
	case "cities":
		return []importer.ColumnSpec{
			{Name: cityColName, Required: true, Kind: "text"},
			{Name: cityColProvince, Required: true, Kind: "lookup"},
			{Name: cityColCode, Required: false, Kind: "text"},
		}
	case "brands":
		return []importer.ColumnSpec{
			{Name: brandColName, Required: true, Kind: "text"},
		}
	case "units":
		return []importer.ColumnSpec{
			{Name: unitColName, Required: true, Kind: "text"},
			{Name: unitColSymbol, Required: false, Kind: "text"},
		}
	case "models":
		return []importer.ColumnSpec{
			{Name: modelColBrand, Required: true, Kind: "lookup"},
			{Name: modelColName, Required: true, Kind: "text"},
		}
	default:
		return nil
	}
}

// ValidateRows loads the lookup sets, then runs the pure row validation for
// the importer's resource. Splitting the DB step (buildProvinceLookups /
// buildCityLookups) from the pure step (validateProvinceRows /
// validateCityRows) keeps the business rules unit-testable without a
// database — mirrors internal/masterdata/employee/importer.go.
func (r referenceImporter) ValidateRows(ctx context.Context, rows []importer.RawRow, scope importer.Scope) ([]importer.RowResult, error) {
	switch r.resource {
	case "provinces":
		lk, err := r.s.buildProvinceLookups(ctx)
		if err != nil {
			return nil, err
		}
		return validateProvinceRows(rows, lk), nil
	case "cities":
		lk, err := r.s.buildCityLookups(ctx)
		if err != nil {
			return nil, err
		}
		return validateCityRows(rows, lk), nil
	case "brands":
		lk, err := r.s.buildBrandLookups(ctx)
		if err != nil {
			return nil, err
		}
		return validateBrandRows(rows, lk), nil
	case "units":
		lk, err := r.s.buildUnitLookups(ctx)
		if err != nil {
			return nil, err
		}
		return validateUnitRows(rows, lk), nil
	case "models":
		lk, err := r.s.buildModelLookups(ctx)
		if err != nil {
			return nil, err
		}
		return validateModelRows(rows, lk), nil
	default:
		return nil, importer.ErrUnknownTarget
	}
}

// --- provinces --------------------------------------------------------------

// provinceLookups holds the case-insensitive lookup sets the province
// importer validates rows against. existingCodes/existingNames are built
// once per batch by buildProvinceLookups; consumed by the pure
// validateProvinceRows.
type provinceLookups struct {
	// existingCodes is the set of all existing (non-deleted) province codes,
	// lower-cased. Province codes are uniquely constrained
	// (uq_provinces_code), so this DB-backed check is authoritative.
	existingCodes map[string]bool
	// existingNames is the set of all existing (non-deleted) province names,
	// lower-cased. Province names carry NO unique constraint, so this is a
	// soft, best-effort validation-time check only (see validateProvinceRows).
	existingNames map[string]bool
}

// buildProvinceLookups loads all existing (non-deleted) provinces — their
// codes for the dupKode rule and their names for the soft dupNama rule — via
// a single ListProvincesLookup call.
func (s *Service) buildProvinceLookups(ctx context.Context) (provinceLookups, error) {
	lk := provinceLookups{
		existingCodes: map[string]bool{},
		existingNames: map[string]bool{},
	}
	provs, err := s.q.ListProvincesLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, p := range provs {
		if p.Code != nil {
			if k := normCode(*p.Code); k != "" {
				lk.existingCodes[k] = true
			}
		}
		if k := normKey(p.Name); k != "" {
			lk.existingNames[k] = true
		}
	}
	return lk, nil
}

// validateProvinceRows validates raw province rows against pre-loaded
// lookups, returning one RowResult per input row (same order). It performs
// NO database access: all resolution is against lk, so the full rule set is
// unit-testable with hand-built lookups.
//
// nama is required; kode is optional. A non-empty kode that duplicates an
// existing DB code or an earlier row in this batch (both case-insensitive)
// fails "dupKode". A non-empty nama that duplicates an existing DB name or an
// earlier row in this batch (both case-insensitive) fails "dupNama" — this is
// a SOFT, validation-time-only rule since province names carry no DB unique
// constraint (Execute never needs to guard against a 23505 on name).
// Provinces have no lookup/scope concept and never need approval routing, so
// NormalizedRef is left empty and no internal id is stamped into Data.
func validateProvinceRows(rows []importer.RawRow, lk provinceLookups) []importer.RowResult {
	results := make([]importer.RowResult, len(rows))
	seenCodes := map[string]bool{}
	seenNames := map[string]bool{}

	for i, raw := range rows {
		data := map[string]string{
			provColName: trim(raw.Cells[provColName]),
			provColCode: trim(raw.Cells[provColCode]),
		}
		var errs []importer.CellError
		add := func(col, key string) { errs = append(errs, importer.CellError{Column: col, ErrorKey: key}) }

		if data[provColName] == "" {
			add(provColName, "required")
		}

		// kode: optional; not a duplicate within this file, not already in DB
		// (both case-insensitive).
		if v := data[provColCode]; v != "" {
			key := normCode(v)
			switch {
			case lk.existingCodes[key]:
				add(provColCode, "dupKode")
			case seenCodes[key]:
				add(provColCode, "dupKode")
			default:
				seenCodes[key] = true
			}
		}

		// nama: soft duplicate check (no DB constraint) — not a duplicate
		// within this file, not already loaded as an existing province name.
		if v := data[provColName]; v != "" {
			key := normKey(v)
			switch {
			case lk.existingNames[key]:
				add(provColName, "dupNama")
			case seenNames[key]:
				add(provColName, "dupNama")
			default:
				seenNames[key] = true
			}
		}

		results[i] = importer.RowResult{
			RowNo:  raw.RowNo,
			Valid:  len(errs) == 0,
			Data:   data,
			Errors: errs,
		}
	}

	return results
}

// --- cities -------------------------------------------------------------

// cityLookups holds the case-insensitive lookup maps the city importer
// validates rows against. Built once per batch by buildCityLookups; consumed
// by the pure validateCityRows.
type cityLookups struct {
	// provinces maps a lower-cased province name OR code to its id.
	provinces map[string]uuid.UUID
	// existingCodes is the set of all existing (non-deleted) city codes,
	// lower-cased. cities.code IS uniquely constrained (uq_cities_code), so
	// this DB-backed check is authoritative — mirrors
	// provinceLookups.existingCodes.
	existingCodes map[string]bool
}

// buildCityLookups loads all provinces into a name/code lookup for the
// cities importer's "provinsi" column, and all existing city codes for the
// "kode" dupKode rule. cities.code IS uniquely constrained (uq_cities_code),
// so — like provinces — the validate-time dupKode check is DB-backed, not
// in-file-only; Execute's GetCityByCode pre-check remains as the TOCTOU
// guard for races between validate and execute.
func (s *Service) buildCityLookups(ctx context.Context) (cityLookups, error) {
	lk := cityLookups{
		provinces:     map[string]uuid.UUID{},
		existingCodes: map[string]bool{},
	}
	provs, err := s.q.ListProvincesLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, p := range provs {
		addKey(lk.provinces, p.Name, p.ID)
		if p.Code != nil {
			addKey(lk.provinces, *p.Code, p.ID)
		}
	}

	codes, err := s.q.ListCityCodes(ctx)
	if err != nil {
		return lk, err
	}
	for _, c := range codes {
		if c == nil {
			continue
		}
		if k := normCode(*c); k != "" {
			lk.existingCodes[k] = true
		}
	}
	return lk, nil
}

// validateCityRows validates raw city rows against pre-loaded lookups,
// returning one RowResult per input row (same order). It performs NO
// database access: all resolution is against lk, so the full rule set is
// unit-testable with hand-built lookups.
//
// nama and provinsi are required; provinsi is resolved by name OR code,
// case-insensitively, against lk.provinces — a miss fails "provinsi". kode is
// optional; a non-empty kode that duplicates an existing DB code or an
// earlier row in this batch (both case-insensitive) fails "dupKode" — same
// rule as validateProvinceRows. A valid row's resolved province id is
// stamped into Data under
// "_province_id" for Execute to consume without re-resolving. Cities never
// need approval routing, so NormalizedRef is deliberately left empty.
func validateCityRows(rows []importer.RawRow, lk cityLookups) []importer.RowResult {
	results := make([]importer.RowResult, len(rows))
	seenCodes := map[string]bool{}

	for i, raw := range rows {
		data := map[string]string{
			cityColName:     trim(raw.Cells[cityColName]),
			cityColProvince: trim(raw.Cells[cityColProvince]),
			cityColCode:     trim(raw.Cells[cityColCode]),
		}
		var errs []importer.CellError
		add := func(col, key string) { errs = append(errs, importer.CellError{Column: col, ErrorKey: key}) }

		for _, col := range []string{cityColName, cityColProvince} {
			if data[col] == "" {
				add(col, "required")
			}
		}

		// provinsi: resolved by name OR code, case-insensitive.
		var provinceID uuid.UUID
		hasProvince := false
		if v := data[cityColProvince]; v != "" {
			if id, ok := lk.provinces[normKey(v)]; ok {
				provinceID = id
				hasProvince = true
			} else {
				add(cityColProvince, "provinsi")
			}
		}

		// kode: optional; not a duplicate within this file, not already in DB
		// (both case-insensitive).
		if v := data[cityColCode]; v != "" {
			key := normCode(v)
			switch {
			case lk.existingCodes[key]:
				add(cityColCode, "dupKode")
			case seenCodes[key]:
				add(cityColCode, "dupKode")
			default:
				seenCodes[key] = true
			}
		}

		if hasProvince {
			data["_province_id"] = provinceID.String()
		}

		valid := len(errs) == 0
		res := importer.RowResult{
			RowNo:  raw.RowNo,
			Valid:  valid,
			Data:   data,
			Errors: errs,
		}
		if !valid {
			// Drop internal resolution stamps from invalid rows to keep their
			// persisted data clean (they never reach the executor).
			delete(res.Data, "_province_id")
		}
		results[i] = res
	}

	return results
}

// --- brands -----------------------------------------------------------------

type brandLookups struct{ existingNames map[string]bool }

func (s *Service) buildBrandLookups(ctx context.Context) (brandLookups, error) {
	lk := brandLookups{existingNames: map[string]bool{}}
	brands, err := s.q.ListBrandsLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, b := range brands {
		if k := normKey(b.Name); k != "" {
			lk.existingNames[k] = true
		}
	}
	return lk, nil
}

// validateBrandRows: nama required and unique (uq_brands_name) — a name
// duplicating the DB or an earlier in-batch row (case-insensitive) fails
// dupNama. No lookup/scope/approval concept.
func validateBrandRows(rows []importer.RawRow, lk brandLookups) []importer.RowResult {
	results := make([]importer.RowResult, len(rows))
	seen := map[string]bool{}
	for i, raw := range rows {
		data := map[string]string{brandColName: trim(raw.Cells[brandColName])}
		var errs []importer.CellError
		add := func(col, key string) { errs = append(errs, importer.CellError{Column: col, ErrorKey: key}) }
		if data[brandColName] == "" {
			add(brandColName, "required")
		} else {
			key := normKey(data[brandColName])
			switch {
			case lk.existingNames[key], seen[key]:
				add(brandColName, "dupNama")
			default:
				seen[key] = true
			}
		}
		results[i] = importer.RowResult{RowNo: raw.RowNo, Valid: len(errs) == 0, Data: data, Errors: errs}
	}
	return results
}

// --- units ------------------------------------------------------------------

type unitLookups struct{ existingNames map[string]bool }

func (s *Service) buildUnitLookups(ctx context.Context) (unitLookups, error) {
	lk := unitLookups{existingNames: map[string]bool{}}
	names, err := s.q.ListUnitNames(ctx)
	if err != nil {
		return lk, err
	}
	for _, n := range names {
		if k := normKey(n); k != "" {
			lk.existingNames[k] = true
		}
	}
	return lk, nil
}

// validateUnitRows: nama required and unique (uq_units_name); simbol optional.
func validateUnitRows(rows []importer.RawRow, lk unitLookups) []importer.RowResult {
	results := make([]importer.RowResult, len(rows))
	seen := map[string]bool{}
	for i, raw := range rows {
		data := map[string]string{
			unitColName:   trim(raw.Cells[unitColName]),
			unitColSymbol: trim(raw.Cells[unitColSymbol]),
		}
		var errs []importer.CellError
		add := func(col, key string) { errs = append(errs, importer.CellError{Column: col, ErrorKey: key}) }
		if data[unitColName] == "" {
			add(unitColName, "required")
		} else {
			key := normKey(data[unitColName])
			switch {
			case lk.existingNames[key], seen[key]:
				add(unitColName, "dupNama")
			default:
				seen[key] = true
			}
		}
		results[i] = importer.RowResult{RowNo: raw.RowNo, Valid: len(errs) == 0, Data: data, Errors: errs}
	}
	return results
}

// --- models -----------------------------------------------------------------

type modelLookups struct {
	brands        map[string]uuid.UUID // brand name (lower) -> id
	existingPairs map[string]bool      // brandID + "\x00" + lower(name)
}

func modelPairKey(brandID uuid.UUID, name string) string {
	return brandID.String() + "\x00" + normKey(name)
}

func (s *Service) buildModelLookups(ctx context.Context) (modelLookups, error) {
	lk := modelLookups{brands: map[string]uuid.UUID{}, existingPairs: map[string]bool{}}
	brands, err := s.q.ListBrandsLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, b := range brands {
		addKey(lk.brands, b.Name, b.ID)
	}
	models, err := s.q.ListModelsLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, m := range models {
		lk.existingPairs[modelPairKey(m.BrandID, m.Name)] = true
	}
	return lk, nil
}

// validateModelRows: merek + nama required; merek resolved by brand name
// (case-insensitive); (brand_id, name) unique (uq_models_brand_name).
func validateModelRows(rows []importer.RawRow, lk modelLookups) []importer.RowResult {
	results := make([]importer.RowResult, len(rows))
	seenPairs := map[string]bool{}
	for i, raw := range rows {
		data := map[string]string{
			modelColBrand: trim(raw.Cells[modelColBrand]),
			modelColName:  trim(raw.Cells[modelColName]),
		}
		var errs []importer.CellError
		add := func(col, key string) { errs = append(errs, importer.CellError{Column: col, ErrorKey: key}) }
		for _, col := range []string{modelColBrand, modelColName} {
			if data[col] == "" {
				add(col, "required")
			}
		}
		var brandID uuid.UUID
		hasBrand := false
		if v := data[modelColBrand]; v != "" {
			if id, ok := lk.brands[normKey(v)]; ok {
				brandID = id
				hasBrand = true
			} else {
				add(modelColBrand, "merek")
			}
		}
		if hasBrand && data[modelColName] != "" {
			pk := modelPairKey(brandID, data[modelColName])
			switch {
			case lk.existingPairs[pk], seenPairs[pk]:
				add(modelColName, "dupNama")
			default:
				seenPairs[pk] = true
			}
		}
		if hasBrand {
			data["_brand_id"] = brandID.String()
		}
		valid := len(errs) == 0
		res := importer.RowResult{RowNo: raw.RowNo, Valid: valid, Data: data, Errors: errs}
		if !valid {
			delete(res.Data, "_brand_id")
		}
		results[i] = res
	}
	return results
}

// --- shared helpers -------------------------------------------------------

// addKey inserts a lower-cased, trimmed name/code -> id entry, skipping empties.
func addKey(m map[string]uuid.UUID, name string, id uuid.UUID) {
	if k := normKey(name); k != "" {
		m[k] = id
	}
}

// normKey lower-cases and trims a lookup key.
func normKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// normCode lower-cases and trims a code for case-insensitive duplicate
// detection.
func normCode(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// trim is a short alias for strings.TrimSpace used throughout row parsing.
func trim(s string) string { return strings.TrimSpace(s) }

// --- Execute --------------------------------------------------------------

// Execute creates one province or city per validated row, inside the
// worker's single execute transaction (reference imports have
// NeedsApproval() == false, so the generic worker calls this directly — see
// executePhase in worker.go).
//
// TX-POISONING DEFENSE: exactly like internal/masterdata/employee/importer.go's
// Execute, this runs inside ONE shared transaction for the whole batch. In
// PostgreSQL a unique-violation (23505) POISONS the whole transaction — every
// subsequent command fails with 25P02 "current transaction is aborted" — so a
// single `kode` collision at Create time would abort and roll back the ENTIRE
// batch instead of just that one row. To keep the "fail one row, continue"
// design working, provinces/cities never fire a 23505 in the common case: for
// any non-empty kode we pre-check its availability (against codes consumed
// earlier in THIS batch and against the DB) so a taken code is skipped as a
// failed row rather than inserted. Province/city names carry no equivalent
// constraint (cities.code IS uniquely constrained, provinces.code likewise;
// neither table constrains name), so a Create call is never at risk of a
// name-driven 23505.
func (r referenceImporter) Execute(ctx context.Context, qtx *sqlc.Queries, job importer.Job, validRows []importer.Row) (int, error) {
	switch r.resource {
	case "provinces":
		return executeProvinces(ctx, qtx, validRows)
	case "cities":
		return executeCities(ctx, qtx, validRows)
	case "brands":
		return executeBrands(ctx, qtx, validRows)
	case "units":
		return executeUnits(ctx, qtx, validRows)
	case "models":
		return executeModels(ctx, qtx, validRows)
	default:
		return 0, importer.ErrUnknownTarget
	}
}

func executeProvinces(ctx context.Context, qtx *sqlc.Queries, validRows []importer.Row) (int, error) {
	created := 0
	// Codes consumed by earlier rows in THIS execution (lower-cased). Guards
	// against two rows in the same batch resolving to the same code before
	// either is visible to a DB read.
	usedCodes := map[string]bool{}

	markFailed := func(id uuid.UUID) error {
		errsJSON, mErr := json.Marshal([]importer.CellError{{Column: provColCode, ErrorKey: "dupKode"}})
		if mErr != nil {
			return mErr
		}
		return qtx.MarkRowFailed(ctx, sqlc.MarkRowFailedParams{ID: id, Errors: errsJSON})
	}

	for _, r := range validRows {
		name := trim(r.Data[provColName])
		code := trim(r.Data[provColCode])

		var codeArg *string
		if code != "" {
			codeKey := normCode(code)

			// Pre-check availability BEFORE inserting, so CreateProvince is
			// never called with a taken code (no 23505 is triggered, no tx
			// poisoning).
			if usedCodes[codeKey] {
				if fErr := markFailed(r.ID); fErr != nil {
					return created, fErr
				}
				continue
			}
			// Fresh DB existence check: catches a code that became taken
			// since validation (TOCTOU) or one already committed by a prior
			// batch. GetProvinceByCode is a side-effect-free SELECT;
			// returning pgx.ErrNoRows does NOT poison the tx.
			if _, gErr := qtx.GetProvinceByCode(ctx, &code); gErr == nil {
				if fErr := markFailed(r.ID); fErr != nil {
					return created, fErr
				}
				continue
			} else if !errors.Is(gErr, pgx.ErrNoRows) {
				return created, common.MapDBError(gErr)
			}

			usedCodes[codeKey] = true
			codeArg = &code
		}

		p, err := qtx.CreateProvince(ctx, sqlc.CreateProvinceParams{Name: name, Code: codeArg})
		if err != nil {
			// Residual concurrent-race window (see employee/office Execute):
			// return rather than swallow-and-continue, aborting this attempt
			// cleanly. A retry sees the now-committed code via
			// GetProvinceByCode and skips that row.
			return created, common.MapDBError(err)
		}

		if err := qtx.MarkRowResult(ctx, sqlc.MarkRowResultParams{ID: r.ID, ResultRef: &p.Name}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

func executeCities(ctx context.Context, qtx *sqlc.Queries, validRows []importer.Row) (int, error) {
	created := 0
	// Codes consumed by earlier rows in THIS execution (lower-cased). Guards
	// against two rows in the same batch resolving to the same code before
	// either is visible to a DB read.
	usedCodes := map[string]bool{}

	markFailed := func(id uuid.UUID) error {
		errsJSON, mErr := json.Marshal([]importer.CellError{{Column: cityColCode, ErrorKey: "dupKode"}})
		if mErr != nil {
			return mErr
		}
		return qtx.MarkRowFailed(ctx, sqlc.MarkRowFailedParams{ID: id, Errors: errsJSON})
	}

	for _, r := range validRows {
		provinceID, err := uuid.Parse(r.Data["_province_id"])
		if err != nil {
			return created, common.ErrInvalidReference
		}

		name := trim(r.Data[cityColName])
		code := trim(r.Data[cityColCode])

		var codeArg *string
		if code != "" {
			codeKey := normCode(code)

			// cities.code IS uniquely constrained (uq_cities_code): pre-check
			// availability BEFORE inserting, so CreateCity is never called
			// with a taken code (no 23505 is triggered, no tx poisoning).
			if usedCodes[codeKey] {
				if fErr := markFailed(r.ID); fErr != nil {
					return created, fErr
				}
				continue
			}
			// Fresh DB existence check: catches a code that became taken
			// since validation (TOCTOU) or one already committed by a prior
			// batch. GetCityByCode is a side-effect-free SELECT; returning
			// pgx.ErrNoRows does NOT poison the tx.
			if _, gErr := qtx.GetCityByCode(ctx, &code); gErr == nil {
				if fErr := markFailed(r.ID); fErr != nil {
					return created, fErr
				}
				continue
			} else if !errors.Is(gErr, pgx.ErrNoRows) {
				return created, common.MapDBError(gErr)
			}

			usedCodes[codeKey] = true
			codeArg = &code
		}

		city, cErr := qtx.CreateCity(ctx, sqlc.CreateCityParams{ProvinceID: provinceID, Name: name, Code: codeArg})
		if cErr != nil {
			// Residual concurrent-race window (see employee/office Execute):
			// return rather than swallow-and-continue, aborting this attempt
			// cleanly. A retry sees the now-committed code via GetCityByCode
			// and skips that row.
			return created, common.MapDBError(cErr)
		}

		if err := qtx.MarkRowResult(ctx, sqlc.MarkRowResultParams{ID: r.ID, ResultRef: &city.Name}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

func executeBrands(ctx context.Context, qtx *sqlc.Queries, validRows []importer.Row) (int, error) {
	created := 0
	used := map[string]bool{}
	markFailed := func(id uuid.UUID) error {
		errsJSON, mErr := json.Marshal([]importer.CellError{{Column: brandColName, ErrorKey: "dupNama"}})
		if mErr != nil {
			return mErr
		}
		return qtx.MarkRowFailed(ctx, sqlc.MarkRowFailedParams{ID: id, Errors: errsJSON})
	}
	for _, r := range validRows {
		name := trim(r.Data[brandColName])
		key := normKey(name)
		if used[key] {
			if fErr := markFailed(r.ID); fErr != nil {
				return created, fErr
			}
			continue
		}
		if _, gErr := qtx.GetBrandByName(ctx, name); gErr == nil {
			if fErr := markFailed(r.ID); fErr != nil {
				return created, fErr
			}
			continue
		} else if !errors.Is(gErr, pgx.ErrNoRows) {
			return created, common.MapDBError(gErr)
		}
		b, err := qtx.CreateBrand(ctx, name)
		if err != nil {
			return created, common.MapDBError(err)
		}
		used[key] = true
		if err := qtx.MarkRowResult(ctx, sqlc.MarkRowResultParams{ID: r.ID, ResultRef: &b.Name}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

func executeUnits(ctx context.Context, qtx *sqlc.Queries, validRows []importer.Row) (int, error) {
	created := 0
	used := map[string]bool{}
	markFailed := func(id uuid.UUID) error {
		errsJSON, mErr := json.Marshal([]importer.CellError{{Column: unitColName, ErrorKey: "dupNama"}})
		if mErr != nil {
			return mErr
		}
		return qtx.MarkRowFailed(ctx, sqlc.MarkRowFailedParams{ID: id, Errors: errsJSON})
	}
	for _, r := range validRows {
		name := trim(r.Data[unitColName])
		key := normKey(name)
		if used[key] {
			if fErr := markFailed(r.ID); fErr != nil {
				return created, fErr
			}
			continue
		}
		if _, gErr := qtx.GetUnitByName(ctx, name); gErr == nil {
			if fErr := markFailed(r.ID); fErr != nil {
				return created, fErr
			}
			continue
		} else if !errors.Is(gErr, pgx.ErrNoRows) {
			return created, common.MapDBError(gErr)
		}
		var symbol *string
		if v := trim(r.Data[unitColSymbol]); v != "" {
			symbol = &v
		}
		u, err := qtx.CreateUnit(ctx, sqlc.CreateUnitParams{Name: name, Symbol: symbol})
		if err != nil {
			return created, common.MapDBError(err)
		}
		used[key] = true
		if err := qtx.MarkRowResult(ctx, sqlc.MarkRowResultParams{ID: r.ID, ResultRef: &u.Name}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

func executeModels(ctx context.Context, qtx *sqlc.Queries, validRows []importer.Row) (int, error) {
	created := 0
	usedPairs := map[string]bool{}
	markFailed := func(id uuid.UUID) error {
		errsJSON, mErr := json.Marshal([]importer.CellError{{Column: modelColName, ErrorKey: "dupNama"}})
		if mErr != nil {
			return mErr
		}
		return qtx.MarkRowFailed(ctx, sqlc.MarkRowFailedParams{ID: id, Errors: errsJSON})
	}
	for _, r := range validRows {
		brandID, err := uuid.Parse(r.Data["_brand_id"])
		if err != nil {
			return created, common.ErrInvalidReference
		}
		name := trim(r.Data[modelColName])
		pk := modelPairKey(brandID, name)
		if usedPairs[pk] {
			if fErr := markFailed(r.ID); fErr != nil {
				return created, fErr
			}
			continue
		}
		if _, gErr := qtx.GetModelByBrandAndName(ctx, sqlc.GetModelByBrandAndNameParams{BrandID: brandID, Lower: name}); gErr == nil {
			if fErr := markFailed(r.ID); fErr != nil {
				return created, fErr
			}
			continue
		} else if !errors.Is(gErr, pgx.ErrNoRows) {
			return created, common.MapDBError(gErr)
		}
		m, cErr := qtx.CreateModel(ctx, sqlc.CreateModelParams{BrandID: brandID, Name: name})
		if cErr != nil {
			return created, common.MapDBError(cErr)
		}
		usedPairs[pk] = true
		if err := qtx.MarkRowResult(ctx, sqlc.MarkRowResultParams{ID: r.ID, ResultRef: &m.Name}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}
