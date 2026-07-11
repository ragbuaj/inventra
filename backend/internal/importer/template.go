// This file implements the template-generation stage of the bulk-import
// engine: producing a header-only CSV or XLSX file from a target's
// ColumnSpec list, for the user to fill in and re-upload. Required columns
// keep their bare name in the machine header — the "*" marker lives only in
// UI badges, not in the file itself.
package importer

import (
	"strings"

	"github.com/xuri/excelize/v2"
)

// BuildTemplate produces a header-only file body for the given format ("csv"
// or "xlsx") from cols. It returns the file body, its content type, its file
// extension (without a leading dot), and any error. Unknown formats return
// ErrBadFormat.
func BuildTemplate(format string, cols []ColumnSpec) (body []byte, contentType, ext string, err error) {
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}

	switch strings.ToLower(format) {
	case "csv":
		body = []byte(strings.Join(names, ",") + "\n")
		return body, "text/csv", "csv", nil
	case "xlsx":
		f := excelize.NewFile()
		defer f.Close()
		sheet := f.GetSheetName(0)
		for i, name := range names {
			cell, cellErr := excelize.CoordinatesToCellName(i+1, 1)
			if cellErr != nil {
				return nil, "", "", cellErr
			}
			if setErr := f.SetCellValue(sheet, cell, name); setErr != nil {
				return nil, "", "", setErr
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
