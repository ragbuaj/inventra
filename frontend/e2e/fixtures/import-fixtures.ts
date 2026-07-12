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
//   employee: kode, nama, email, telepon, kantor, status

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
  const header = ['kode', 'nama', 'email', 'telepon', 'kantor', 'status']
  const rows = [
    [codeA, nameA, `e2eimp.a.${run}@inventra.local`, '', officeName, 'active'],
    [codeB, nameB, '', '', officeName, 'active'],
    [`E2EIMPC${run}`, `E2E Import Employee C ${run}`, 'not-an-email', '', officeName, 'active'],
    [`E2EIMPD${run}`, `E2E Import Employee D ${run}`, '', '', officeName, 'unknown_status']
  ]
  return { filename: `employees-${run}.csv`, csv: toCsv(header, rows), codeA, codeB, nameA, nameB }
}
