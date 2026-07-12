// This file implements the parser stage of the bulk-import engine: reading an
// already-downloaded CSV or XLSX file body, matching its header row against a
// target's ColumnSpec list (case-insensitive, order-insensitive), and
// producing one RawRow per data row for downstream validation.
package importer

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"

	"github.com/xuri/excelize/v2"
)

var (
	ErrBadFormat   = errors.New("importer: unsupported format")
	ErrBadHeader   = errors.New("importer: header does not match template")
	ErrTooManyRows = errors.New("importer: row count exceeds limit")
	ErrEmptyFile   = errors.New("importer: file has no data rows")
)

// Parse reads an already-downloaded file body and returns rows keyed by
// column name. format is "csv" or "xlsx". cols defines the required header
// (order-insensitive, case-insensitive).
func Parse(format string, body []byte, cols []ColumnSpec, maxRows int) ([]RawRow, error) {
	var records [][]string
	switch strings.ToLower(format) {
	case "csv":
		r := csv.NewReader(bytes.NewReader(body))
		r.FieldsPerRecord = -1
		recs, err := r.ReadAll()
		if err != nil {
			return nil, ErrBadFormat
		}
		records = recs
	case "xlsx":
		f, err := excelize.OpenReader(bytes.NewReader(body))
		if err != nil {
			return nil, ErrBadFormat
		}
		defer f.Close()
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil, ErrBadFormat
		}
		recs, err := f.GetRows(sheets[0])
		if err != nil {
			return nil, ErrBadFormat
		}
		records = recs
	default:
		return nil, ErrBadFormat
	}
	if len(records) == 0 {
		return nil, ErrBadHeader
	}

	header := records[0]
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, c := range cols {
		if _, ok := idx[strings.ToLower(c.Name)]; !ok {
			return nil, ErrBadHeader
		}
	}

	data := records[1:]
	if len(data) == 0 {
		return nil, ErrEmptyFile
	}
	if len(data) > maxRows {
		return nil, ErrTooManyRows
	}

	out := make([]RawRow, 0, len(data))
	for i, rec := range data {
		cells := map[string]string{}
		for _, c := range cols {
			j := idx[strings.ToLower(c.Name)]
			v := ""
			if j < len(rec) {
				v = strings.TrimSpace(rec[j])
			}
			cells[c.Name] = v
		}
		out = append(out, RawRow{RowNo: i + 1, Cells: cells})
	}
	return out, nil
}

// errorKeyFor maps a parser sentinel to an i18n key stored on the job.
func errorKeyFor(err error) string {
	switch {
	case errors.Is(err, ErrBadHeader):
		return "badHeader"
	case errors.Is(err, ErrTooManyRows):
		return "tooManyRows"
	case errors.Is(err, ErrEmptyFile):
		return "emptyFile"
	case errors.Is(err, ErrBadFormat):
		return "badFormat"
	default:
		return "parseFailed"
	}
}
