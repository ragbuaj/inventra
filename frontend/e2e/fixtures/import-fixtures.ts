// Fixture builders for the bulk-import Playwright e2e (frontend/e2e/import.spec.ts).
//
// Every data row must resolve against real master data (kategori/kantor/lokasi
// are looked up by name or code, case-insensitively — see
// backend/internal/asset/importer.go buildAssetLookups and
// backend/internal/masterdata/employee/importer.go buildEmployeeLookups) that
// exists in the target backend at test time. Because those names are created
// fresh per run in import.spec.ts's `beforeAll` (unique name/code per run —
// this dev DB is never reset between local runs), the fixtures are built here
// as functions rather than static .csv files: each call bakes in the caller's
// actual office/category/room names and a run-unique suffix so uploaded
// batches never collide with rows from a previous run.
//
// CSV column order matches each target's ColumnSpec exactly (header matching
// is case-insensitive and order-insensitive on the backend, but keeping this
// order mirrors the downloaded template for readability):
//   asset:    asset_tag, nama, kategori, kantor, tgl_beli, harga, vendor, lokasi
//   employee: kode, nama, email, telepon, kantor, status, departemen, jabatan

function toCsv(header: string[], rows: string[][]): string {
  const esc = (v: string): string => (/[",\n]/.test(v) ? `"${v.replace(/"/g, '""')}"` : v)
  return [header, ...rows].map(r => r.map(esc).join(',')).join('\n') + '\n'
}

export interface AssetHappyPathInputs {
  officeName: string
  categoryName: string
  roomName: string
  run: string
}

export interface AssetHappyPathFixture {
  filename: string
  csv: string
  row1Name: string
  row2Name: string
  badRowName: string
}

/**
 * Two valid rows + one invalid row (an unknown `kategori`) — drives the asset
 * import happy-path scenario: preview shows 2 valid / 1 error, confirm
 * creates a maker-checker `asset_import` request for the 2 valid rows.
 */
export function buildAssetHappyPathCsv({ officeName, categoryName, roomName, run }: AssetHappyPathInputs): AssetHappyPathFixture {
  const row1Name = `E2E Import Meja ${run}`
  const row2Name = `E2E Import Kursi ${run}`
  const badRowName = `E2E Import BadCat ${run}`
  const header = ['asset_tag', 'nama', 'kategori', 'kantor', 'tgl_beli', 'harga', 'vendor', 'lokasi']
  const rows = [
    ['', row1Name, categoryName, officeName, '2026-06-01', '700000', '', roomName],
    ['', row2Name, categoryName, officeName, '2026-06-02', '800000', '', roomName],
    ['', badRowName, `Nonexistent Category ${run}`, officeName, '2026-06-03', '500000', '', roomName]
  ]
  return { filename: `assets-${run}.csv`, csv: toCsv(header, rows), row1Name, row2Name, badRowName }
}

export interface AssetValidationRejectionInputs {
  officeAName: string
  officeBName: string
  categoryName: string
  roomName: string
  run: string
}

export interface AssetValidationRejectionFixture {
  filename: string
  csv: string
  badDateRowName: string
  multiOfficeRowName: string
  validRowName: string
}

/**
 * A bad-date row (`tgl_beli` not in ISO form) plus a cross-office row (a
 * second, different `kantor` than the batch's first resolved office) — drives
 * the "validation rejected in preview" scenario. Never confirmed by the spec,
 * so it never reaches the approval flow.
 */
export function buildAssetValidationRejectionCsv(
  { officeAName, officeBName, categoryName, roomName, run }: AssetValidationRejectionInputs
): AssetValidationRejectionFixture {
  const badDateRowName = `E2E Import BadDate ${run}`
  const multiOfficeRowName = `E2E Import MultiOffice ${run}`
  const validRowName = `E2E Import RejectValid ${run}`
  const header = ['asset_tag', 'nama', 'kategori', 'kantor', 'tgl_beli', 'harga', 'vendor', 'lokasi']
  const rows = [
    // First row resolves officeA — becomes the batch's reference office, even
    // though it also carries its own bad-date error.
    ['', badDateRowName, categoryName, officeAName, '31/12/2025', '600000', '', roomName],
    // Resolves a DIFFERENT office than the batch reference -> multiOffice.
    ['', multiOfficeRowName, categoryName, officeBName, '2026-06-04', '650000', '', ''],
    ['', validRowName, categoryName, officeAName, '2026-06-05', '700000', '', roomName]
  ]
  return { filename: `assets-reject-${run}.csv`, csv: toCsv(header, rows), badDateRowName, multiOfficeRowName, validRowName }
}

export interface EmployeeImportInputs {
  officeName: string
  run: string
}

export interface EmployeeImportFixture {
  filename: string
  csv: string
  codeA: string
  codeB: string
  nameA: string
  nameB: string
}

/**
 * Two valid rows + two invalid rows (an implausible email, an unknown
 * `status`) — drives the employee import scenario (no approval needed) and
 * the error-report download (failed_rows > 0 on the completed job).
 */
export function buildEmployeeCsv({ officeName, run }: EmployeeImportInputs): EmployeeImportFixture {
  const codeA = `E2EIMPA${run}`
  const codeB = `E2EIMPB${run}`
  const nameA = `E2E Import Employee A ${run}`
  const nameB = `E2E Import Employee B ${run}`
  const header = ['kode', 'nama', 'email', 'telepon', 'kantor', 'status', 'departemen', 'jabatan']
  const rows = [
    [codeA, nameA, `e2eimp.a.${run}@inventra.local`, '', officeName, 'active', '', ''],
    [codeB, nameB, '', '', officeName, 'active', '', ''],
    [`E2EIMPC${run}`, `E2E Import Employee C ${run}`, 'not-an-email', '', officeName, 'active', '', ''],
    [`E2EIMPD${run}`, `E2E Import Employee D ${run}`, '', '', officeName, 'unknown_status', '', '']
  ]
  return { filename: `employees-${run}.csv`, csv: toCsv(header, rows), codeA, codeB, nameA, nameB }
}

// ---------------------------------------------------------------------------
// Master-data import targets registered in Tasks 2-4 (frontend/e2e/import-
// masterdata.spec.ts): office, reference:provinces, reference:cities,
// reference:brands, reference:models. Column order matches each target's
// ColumnSpec (see backend/internal/masterdata/office/importer.go Columns and
// backend/internal/masterdata/reference/importer.go Columns):
//   office:              kode, nama, tipe, induk, aktif
//   reference:provinces: nama, kode
//   reference:cities:    nama, provinsi, kode
//   reference:brands:    nama
//   reference:models:    merek, nama
// Each builder bakes in two valid rows plus one row that fails validation
// (a duplicate kode/nama, or an unresolvable lookup) so every flow also
// exercises the error-count badge + downloadable error report, mirroring
// buildEmployeeCsv above.
// ---------------------------------------------------------------------------

export interface OfficeImportInputs {
  officeTypeName: string
  run: string
}

export interface OfficeImportFixture {
  filename: string
  csv: string
  nameA: string
  nameB: string
  dupRowName: string
}

/**
 * Two valid root offices (empty `induk` — fine for an AllScope/global caller,
 * see office/importer.go validateOfficeRows) + one row reusing row A's `kode`
 * -> dupKode.
 */
export function buildOfficeCsv({ officeTypeName, run }: OfficeImportInputs): OfficeImportFixture {
  const codeA = `E2EOFFA${run}`
  const nameA = `E2E Office Import A ${run}`
  const nameB = `E2E Office Import B ${run}`
  const dupRowName = `E2E Office Import Dup ${run}`
  const header = ['kode', 'nama', 'tipe', 'induk', 'aktif']
  const rows = [
    [codeA, nameA, officeTypeName, '', ''],
    [`E2EOFFB${run}`, nameB, officeTypeName, '', 'ya'],
    [codeA, dupRowName, officeTypeName, '', '']
  ]
  return { filename: `offices-${run}.csv`, csv: toCsv(header, rows), nameA, nameB, dupRowName }
}

export interface ProvinceImportInputs { run: string }

export interface ProvinceImportFixture {
  filename: string
  csv: string
  nameA: string
  nameB: string
  dupRowName: string
}

/** Two valid provinces + one row reusing row A's `kode` -> dupKode. */
export function buildProvinceCsv({ run }: ProvinceImportInputs): ProvinceImportFixture {
  const codeA = `EPIA${run}`
  const nameA = `E2E Provinsi Import A ${run}`
  const nameB = `E2E Provinsi Import B ${run}`
  const dupRowName = `E2E Provinsi Import Dup ${run}`
  const header = ['nama', 'kode']
  const rows = [
    [nameA, codeA],
    [nameB, `EPIB${run}`],
    [dupRowName, codeA]
  ]
  return { filename: `provinces-${run}.csv`, csv: toCsv(header, rows), nameA, nameB, dupRowName }
}

export interface CityImportInputs {
  provinceName: string
  run: string
}

export interface CityImportFixture {
  filename: string
  csv: string
  nameA: string
  nameB: string
  badProvinceRowName: string
}

/**
 * Two valid cities under `provinceName` + one row whose `provinsi` does not
 * resolve to any known province -> "provinsi" error.
 */
export function buildCityCsv({ provinceName, run }: CityImportInputs): CityImportFixture {
  const nameA = `E2E Kota Import A ${run}`
  const nameB = `E2E Kota Import B ${run}`
  const badProvinceRowName = `E2E Kota Import BadProv ${run}`
  const header = ['nama', 'provinsi', 'kode']
  const rows = [
    [nameA, provinceName, `ECIA${run}`],
    [nameB, provinceName, `ECIB${run}`],
    [badProvinceRowName, `Nonexistent Provinsi ${run}`, `ECIC${run}`]
  ]
  return { filename: `cities-${run}.csv`, csv: toCsv(header, rows), nameA, nameB, badProvinceRowName }
}

export interface BrandImportInputs { run: string }

export interface BrandImportFixture {
  filename: string
  csv: string
  nameA: string
  nameB: string
}

/** Two valid brands + one row reusing row A's `nama` -> dupNama (soft, name-only unique). */
export function buildBrandCsv({ run }: BrandImportInputs): BrandImportFixture {
  const nameA = `E2E Brand Import A ${run}`
  const nameB = `E2E Brand Import B ${run}`
  const header = ['nama']
  const rows = [[nameA], [nameB], [nameA]]
  return { filename: `brands-${run}.csv`, csv: toCsv(header, rows), nameA, nameB }
}

export interface ModelImportInputs {
  brandName: string
  run: string
}

export interface ModelImportFixture {
  filename: string
  csv: string
  nameA: string
  nameB: string
  unknownBrandRowName: string
}

/**
 * Two valid models under `brandName` + one row whose `merek` does not resolve
 * to any known brand -> "merek" error.
 */
export function buildModelCsv({ brandName, run }: ModelImportInputs): ModelImportFixture {
  const nameA = `E2E Model Import A ${run}`
  const nameB = `E2E Model Import B ${run}`
  const unknownBrandRowName = `E2E Model Import Unknown ${run}`
  const header = ['merek', 'nama']
  const rows = [
    [brandName, nameA],
    [brandName, nameB],
    [`Nonexistent Merek ${run}`, unknownBrandRowName]
  ]
  return { filename: `models-${run}.csv`, csv: toCsv(header, rows), nameA, nameB, unknownBrandRowName }
}
