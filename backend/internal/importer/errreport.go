// This file implements the failed-row error-report generator: given the
// already-failed rows of a bulk-import job (as persisted sqlc rows), it
// produces a downloadable CSV/XLSX file with the target's original columns
// plus a trailing "keterangan" column listing each row's validation errors,
// so a user can correct the data and re-import.
package importer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"

	"github.com/xuri/excelize/v2"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// BuildErrorReport produces a file body listing the given rows (the caller
// is expected to have already filtered these down to the failed/invalid
// rows): the original cols in order, plus a trailing "keterangan" column
// joining each row's error_key values (comma-separated). format is "csv" or
// "xlsx". Unknown formats return ErrBadFormat.
func BuildErrorReport(format string, cols []ColumnSpec, rows []sqlc.ImportImportRow) (body []byte, contentType, ext string, err error) {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}

	records := make([][]string, 0, len(rows)+1)
	header := make([]string, 0, len(names)+1)
	header = append(header, names...)
	header = append(header, "keterangan")
	records = append(records, header)

	for _, row := range rows {
		var data map[string]string
		if len(row.Data) > 0 {
			if uErr := json.Unmarshal(row.Data, &data); uErr != nil {
				return nil, "", "", uErr
			}
		}
		var cellErrors []CellError
		if len(row.Errors) > 0 {
			if uErr := json.Unmarshal(row.Errors, &cellErrors); uErr != nil {
				return nil, "", "", uErr
			}
		}
		keys := make([]string, len(cellErrors))
		for i, ce := range cellErrors {
			keys[i] = ce.ErrorKey
		}

		rec := make([]string, 0, len(names)+1)
		for _, name := range names {
			rec = append(rec, data[name])
		}
		rec = append(rec, strings.Join(keys, ", "))
		records = append(records, rec)
	}

	switch strings.ToLower(format) {
	case "csv":
		var buf bytes.Buffer
		// Prepend the UTF-8 BOM so Excel on a Windows locale reads the
		// "keterangan" error text as UTF-8 instead of Windows-1252 (which
		// mojibakes non-ASCII). Must be written before the csv.Writer so it
		// lands at byte offset 0 of the final output.
		buf.Write([]byte{0xEF, 0xBB, 0xBF})
		w := csv.NewWriter(&buf)
		if wErr := w.WriteAll(records); wErr != nil {
			return nil, "", "", wErr
		}
		w.Flush()
		if fErr := w.Error(); fErr != nil {
			return nil, "", "", fErr
		}
		return buf.Bytes(), "text/csv", "csv", nil
	case "xlsx":
		f := excelize.NewFile()
		defer f.Close()
		sheet := f.GetSheetName(0)
		for r, rec := range records {
			for c, val := range rec {
				cell, cellErr := excelize.CoordinatesToCellName(c+1, r+1)
				if cellErr != nil {
					return nil, "", "", cellErr
				}
				if setErr := f.SetCellValue(sheet, cell, val); setErr != nil {
					return nil, "", "", setErr
				}
			}
		}
		buf, wErr := f.WriteToBuffer()
		if wErr != nil {
			return nil, "", "", wErr
		}
		return buf.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx", nil
	default:
		return nil, "", "", ErrBadFormat
	}
}
