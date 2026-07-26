// This file implements the template-generation stage of the bulk-import
// engine. The XLSX template carries two sheets: "Data Impor" (header-only,
// the sheet the user fills in and re-uploads) and "Contoh Penggunaan"
// (the same header plus one worked example row that demonstrates how each
// column should be filled). The parser reads the first sheet on re-upload
// (see parser.go), so "Data Impor" MUST be the first sheet. The legacy CSV
// format stays header-only. Required columns keep their bare name in the
// machine header — the "*" marker lives only in UI badges, not in the file.
package importer

import (
	"strings"

	"github.com/xuri/excelize/v2"
)

// Template sheet names. sheetData must be the first sheet: Parse reads sheet
// index 0 on re-upload, so the fill-in sheet has to come first.
const (
	sheetData    = "Data Impor"
	sheetExample = "Contoh Penggunaan"
)

// BuildTemplate produces a template file body for the given format ("csv" or
// "xlsx") from cols. It returns the file body, its content type, its file
// extension (without a leading dot), and any error. Unknown formats return
// ErrBadFormat. The XLSX form has two sheets ("Data Impor" + "Contoh
// Penggunaan"); the CSV form is a single header-only line.
func BuildTemplate(format string, cols []ColumnSpec) (body []byte, contentType, ext string, err error) {
	names := make([]string, len(cols))
	examples := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
		examples[i] = c.Example
	}

	switch strings.ToLower(format) {
	case "csv":
		// Prepend the UTF-8 BOM so Excel on a Windows locale reads the file
		// as UTF-8 instead of Windows-1252 (which mojibakes non-ASCII column
		// names). XLSX is a binary zip format and needs no such marker.
		body = []byte("\xEF\xBB\xBF" + strings.Join(names, ",") + "\n")
		return body, "text/csv", "csv", nil
	case "xlsx":
		return buildTemplateXLSX(names, examples)
	default:
		return nil, "", "", ErrBadFormat
	}
}

// buildTemplateXLSX renders the two-sheet XLSX template. Sheet "Data Impor"
// holds only the header row (the user fills the rows beneath it); sheet
// "Contoh Penggunaan" repeats the header and adds one example row when any
// example value is present.
func buildTemplateXLSX(names, examples []string) (body []byte, contentType, ext string, err error) {
	f := excelize.NewFile()
	defer f.Close()

	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, "", "", err
	}

	// Sheet 1: "Data Impor" — the fill-in sheet, MUST stay first (parsed on
	// re-upload). Rename the default sheet rather than adding a new one so it
	// keeps sheet index 0.
	if err = f.SetSheetName(f.GetSheetName(0), sheetData); err != nil {
		return nil, "", "", err
	}
	if err = writeRow(f, sheetData, 1, names); err != nil {
		return nil, "", "", err
	}

	// Sheet 2: "Contoh Penggunaan" — header + one worked example row.
	if _, err = f.NewSheet(sheetExample); err != nil {
		return nil, "", "", err
	}
	if err = writeRow(f, sheetExample, 1, names); err != nil {
		return nil, "", "", err
	}
	if hasAny(examples) {
		if err = writeRow(f, sheetExample, 2, examples); err != nil {
			return nil, "", "", err
		}
	}

	// Cosmetics: bold header row and give each column a readable width on
	// both sheets. Failures here are non-fatal to the file's usability, but
	// surface them anyway for consistency.
	for _, sheet := range []string{sheetData, sheetExample} {
		if err = styleHeader(f, sheet, len(names), headerStyle); err != nil {
			return nil, "", "", err
		}
		if err = autoWidth(f, sheet, names, examples); err != nil {
			return nil, "", "", err
		}
	}

	buf, wErr := f.WriteToBuffer()
	if wErr != nil {
		return nil, "", "", wErr
	}
	return buf.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx", nil
}

// writeRow writes values across a single 1-based row of the given sheet.
func writeRow(f *excelize.File, sheet string, row int, values []string) error {
	for i, v := range values {
		cell, err := excelize.CoordinatesToCellName(i+1, row)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			return err
		}
	}
	return nil
}

// styleHeader applies the given style to the header cells (row 1) of a sheet.
func styleHeader(f *excelize.File, sheet string, ncols, style int) error {
	if ncols == 0 {
		return nil
	}
	last, err := excelize.CoordinatesToCellName(ncols, 1)
	if err != nil {
		return err
	}
	return f.SetCellStyle(sheet, "A1", last, style)
}

// autoWidth sizes each column to fit the wider of its header name or example
// value, within sane bounds, so the template is legible without manual
// resizing.
func autoWidth(f *excelize.File, sheet string, names, examples []string) error {
	for i, name := range names {
		w := len(name)
		if i < len(examples) && len(examples[i]) > w {
			w = len(examples[i])
		}
		w += 2 // padding
		if w < 12 {
			w = 12
		}
		if w > 48 {
			w = 48
		}
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		if err := f.SetColWidth(sheet, col, col, float64(w)); err != nil {
			return err
		}
	}
	return nil
}

// hasAny reports whether any value is non-blank.
func hasAny(vals []string) bool {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}
