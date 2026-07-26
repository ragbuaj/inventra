// This file implements the template-generation stage of the bulk-import
// engine. By default the XLSX template carries two sheets: "Data Impor"
// (header-only, the sheet the user fills in and re-uploads) and "Contoh
// Penggunaan" (the same header plus one worked example row). A target may
// customize this via TemplateProvider/TemplateSpec: custom sheet names, an
// example note, explicit example rows, dropdowns (data validation) sourced
// from the caller's scope, and a "*" marker on required headers. The parser
// reads the first sheet on re-upload (see parser.go), so the fill-in sheet is
// always first. The legacy CSV format stays a single header-only line.
package importer

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Default template sheet names. The fill-in sheet must be first: Parse reads
// sheet index 0 on re-upload.
const (
	sheetData    = "Data Impor"
	sheetExample = "Contoh Penggunaan"
	// sheetOptions holds dropdown option lists; hidden and never parsed.
	sheetOptions = "_pilihan"
	// dropdownRows is how many data rows below the header a dropdown covers.
	dropdownRows = 2000
)

// BuildTemplate produces the default template (no dropdowns, default sheet
// names, one example row built from each ColumnSpec.Example).
func BuildTemplate(format string, cols []ColumnSpec) (body []byte, contentType, ext string, err error) {
	return BuildTemplateWith(format, cols, TemplateSpec{})
}

// BuildTemplateWith produces a template file body for the given format ("csv"
// or "xlsx") from cols and an optional TemplateSpec. It returns the file body,
// its content type, its file extension (without a leading dot), and any error.
// Unknown formats return ErrBadFormat. The CSV form is always a single
// header-only line with bare column names (spec extras are XLSX-only).
func BuildTemplateWith(format string, cols []ColumnSpec, spec TemplateSpec) (body []byte, contentType, ext string, err error) {
	switch strings.ToLower(format) {
	case "csv":
		names := make([]string, len(cols))
		for i, c := range cols {
			names[i] = c.Name
		}
		// UTF-8 BOM so Excel on a Windows locale reads it as UTF-8.
		body = []byte("\xEF\xBB\xBF" + strings.Join(names, ",") + "\n")
		return body, "text/csv", "csv", nil
	case "xlsx":
		return buildTemplateXLSX(cols, spec)
	default:
		return nil, "", "", ErrBadFormat
	}
}

// buildTemplateXLSX renders the (possibly customized) two-sheet XLSX template.
func buildTemplateXLSX(cols []ColumnSpec, spec TemplateSpec) (body []byte, contentType, ext string, err error) {
	dataSheet := orDefault(spec.DataSheet, sheetData)
	exampleSheet := orDefault(spec.ExampleSheet, sheetExample)

	// Displayed header labels: bare names, or "name*" for required columns when
	// the spec asks for the marker (the parser strips the trailing "*").
	headers := make([]string, len(cols))
	for i, c := range cols {
		if spec.MarkRequired && c.Required {
			headers[i] = c.Name + "*"
		} else {
			headers[i] = c.Name
		}
	}

	// Example rows: explicit rows from the spec, else one row from Example values.
	exampleRows := spec.ExampleRows
	if exampleRows == nil {
		row := make([]string, len(cols))
		any := false
		for i, c := range cols {
			row[i] = c.Example
			if strings.TrimSpace(c.Example) != "" {
				any = true
			}
		}
		if any {
			exampleRows = [][]string{row}
		}
	}

	f := excelize.NewFile()
	defer f.Close()

	headerStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return nil, "", "", err
	}
	noteStyle, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "C80000"}})
	if err != nil {
		return nil, "", "", err
	}

	// Sheet 1: fill-in sheet — MUST stay first (parsed on re-upload).
	if err = f.SetSheetName(f.GetSheetName(0), dataSheet); err != nil {
		return nil, "", "", err
	}
	if err = writeRow(f, dataSheet, 1, headers); err != nil {
		return nil, "", "", err
	}

	// Sheet 2: example sheet — optional note, then header, then example rows.
	if _, err = f.NewSheet(exampleSheet); err != nil {
		return nil, "", "", err
	}
	exHeaderRow := 1
	if spec.ExampleNote != "" {
		if err = writeRow(f, exampleSheet, 1, []string{spec.ExampleNote}); err != nil {
			return nil, "", "", err
		}
		if err = f.SetCellStyle(exampleSheet, "A1", "A1", noteStyle); err != nil {
			return nil, "", "", err
		}
		exHeaderRow = 2
	}
	if err = writeRow(f, exampleSheet, exHeaderRow, headers); err != nil {
		return nil, "", "", err
	}
	for r, row := range exampleRows {
		if err = writeRow(f, exampleSheet, exHeaderRow+1+r, row); err != nil {
			return nil, "", "", err
		}
	}

	// Bold header row + readable column widths on both sheets.
	var firstExample []string
	if len(exampleRows) > 0 {
		firstExample = exampleRows[0]
	}
	if err = styleHeader(f, dataSheet, len(headers), 1, headerStyle); err != nil {
		return nil, "", "", err
	}
	if err = styleHeader(f, exampleSheet, len(headers), exHeaderRow, headerStyle); err != nil {
		return nil, "", "", err
	}
	for _, sheet := range []string{dataSheet, exampleSheet} {
		if err = autoWidth(f, sheet, headers, firstExample); err != nil {
			return nil, "", "", err
		}
	}

	// Dropdowns (data validation) on the fill sheet, backed by a hidden sheet.
	if err = applyDropdowns(f, dataSheet, cols, spec.Dropdowns); err != nil {
		return nil, "", "", err
	}

	buf, wErr := f.WriteToBuffer()
	if wErr != nil {
		return nil, "", "", wErr
	}
	return buf.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx", nil
}

// applyDropdowns writes each dropdown's option list into a hidden sheet and
// attaches an Excel data-validation drop list (referencing that range) to the
// matching column of the fill sheet. Referencing a hidden range rather than an
// inline list avoids Excel's ~255-char inline limit for large DB-backed lists.
func applyDropdowns(f *excelize.File, fillSheet string, cols []ColumnSpec, dropdowns map[string][]string) error {
	if len(dropdowns) == 0 {
		return nil
	}
	if _, err := f.NewSheet(sheetOptions); err != nil {
		return err
	}
	optCol := 0
	for i, c := range cols {
		opts := dropdowns[c.Name]
		if len(opts) == 0 {
			continue
		}
		optCol++
		optLetter, err := excelize.ColumnNumberToName(optCol)
		if err != nil {
			return err
		}
		if err := f.SetCellValue(sheetOptions, optLetter+"1", c.Name); err != nil {
			return err
		}
		for j, o := range opts {
			cell, cErr := excelize.CoordinatesToCellName(optCol, j+2)
			if cErr != nil {
				return cErr
			}
			if err := f.SetCellValue(sheetOptions, cell, o); err != nil {
				return err
			}
		}
		fillLetter, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return err
		}
		dv := excelize.NewDataValidation(true)
		dv.Sqref = fmt.Sprintf("%s2:%s%d", fillLetter, fillLetter, 1+dropdownRows)
		ref := fmt.Sprintf("'%s'!$%s$2:$%s$%d", sheetOptions, optLetter, optLetter, len(opts)+1)
		dv.SetSqrefDropList(ref)
		if err := f.AddDataValidation(fillSheet, dv); err != nil {
			return err
		}
	}
	return f.SetSheetVisible(sheetOptions, false)
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

// styleHeader applies the given style to the header cells of a sheet on the
// given 1-based row.
func styleHeader(f *excelize.File, sheet string, ncols, row, style int) error {
	if ncols == 0 {
		return nil
	}
	first, err := excelize.CoordinatesToCellName(1, row)
	if err != nil {
		return err
	}
	last, err := excelize.CoordinatesToCellName(ncols, row)
	if err != nil {
		return err
	}
	return f.SetCellStyle(sheet, first, last, style)
}

// autoWidth sizes each column to fit the wider of its header or its example
// value, within sane bounds, so the template is legible without resizing.
func autoWidth(f *excelize.File, sheet string, headers, example []string) error {
	for i, name := range headers {
		w := len(name)
		if i < len(example) && len(example[i]) > w {
			w = len(example[i])
		}
		w += 2
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

// orDefault returns v when non-empty, else def.
func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
