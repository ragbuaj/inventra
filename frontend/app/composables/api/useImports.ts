export interface ImportCellError { column: string, error_key: string }

/** GET/POST /imports* response shape (importer/dto.go jobToMap). */
export interface ImportJob {
  id: string
  target: string
  format: string
  filename: string
  status: string
  total_rows: number
  success_rows: number
  failed_rows: number
  office_id?: string
  request_id?: string
  approval_status?: string
  error_key?: string
  created_at: string
  finished_at?: string
  progress?: { phase: string, done: number, total: number }
}

/**
 * GET /imports/:id/rows row shape (importer/dto.go rowToMap). Beyond the
 * fixed fields below, the object carries one string property per target
 * column (the row's raw uploaded values) directly at the top level — NOT
 * nested under a `data` key (see openapi.yaml ImportRow.additionalProperties
 * and dto.go rowToMap, which spreads `m[k] = v` for each column). Use
 * `importRowColumns`/`importRowValue` to read those column values.
 */
export interface ImportRow {
  id: string
  row_no: number
  valid: boolean
  errors: ImportCellError[]
  result_ref?: string
  [column: string]: unknown
}

// Fixed/meta keys on an ImportRow — everything else is a target-column value
// (the row's raw uploaded data, keyed by column name).
const IMPORT_ROW_META_KEYS: ReadonlySet<string> = new Set(['id', 'row_no', 'valid', 'errors', 'result_ref'])

/** Column names (target-column keys) present on a row, excluding meta fields. */
export function importRowColumns(row: ImportRow): string[] {
  return Object.keys(row).filter(k => !IMPORT_ROW_META_KEYS.has(k))
}

/** A target-column's raw string value for a row (empty string if absent). */
export function importRowValue(row: ImportRow, column: string): string {
  const v = row[column]
  return typeof v === 'string' ? v : ''
}

export interface ImportRowsOpts {
  onlyErrors?: boolean
  limit?: number
  offset?: number
}

export interface ImportJobsOpts {
  limit?: number
  offset?: number
}

/**
 * Bulk-import engine (backend importer/routes.go, mounted at /imports).
 * Downloads (template, error report) go through requestBlob — auth is a
 * Bearer token in the Authorization header, not a cookie, so a plain
 * `<a href>` URL would not carry it.
 */
export function useImports() {
  const { request, requestBlob } = useApiClient()

  async function uploadImport(target: string, file: File): Promise<ImportJob> {
    const form = new FormData()
    form.append('file', file)
    form.append('target', target)
    return request<ImportJob>('/imports', { method: 'POST', body: form })
  }

  async function getJob(id: string): Promise<ImportJob> {
    return request<ImportJob>(`/imports/${id}`)
  }

  async function getRows(id: string, opts: ImportRowsOpts = {}): Promise<{ data: ImportRow[], total: number, limit: number, offset: number }> {
    const query: Record<string, string> = {}
    if (opts.onlyErrors !== undefined) query.only_errors = String(opts.onlyErrors)
    if (opts.limit !== undefined) query.limit = String(opts.limit)
    if (opts.offset !== undefined) query.offset = String(opts.offset)
    return request<{ data: ImportRow[], total: number, limit: number, offset: number }>(`/imports/${id}/rows`, { query })
  }

  async function listJobs(target: string, opts: ImportJobsOpts = {}): Promise<{ data: ImportJob[], total: number, limit: number, offset: number }> {
    const query: Record<string, string> = { target }
    if (opts.limit !== undefined) query.limit = String(opts.limit)
    if (opts.offset !== undefined) query.offset = String(opts.offset)
    return request<{ data: ImportJob[], total: number, limit: number, offset: number }>('/imports', { query })
  }

  async function confirmJob(id: string): Promise<ImportJob> {
    return request<ImportJob>(`/imports/${id}/confirm`, { method: 'POST' })
  }

  async function cancelJob(id: string): Promise<ImportJob> {
    return request<ImportJob>(`/imports/${id}/cancel`, { method: 'POST' })
  }

  async function getTemplate(target: string, format: 'csv' | 'xlsx'): Promise<Blob> {
    return requestBlob('/imports/template', { query: { target, format } })
  }

  async function getErrorReport(id: string): Promise<Blob> {
    return requestBlob(`/imports/${id}/error-report`)
  }

  return { uploadImport, getJob, getRows, listJobs, confirmJob, cancelJob, getTemplate, getErrorReport }
}
