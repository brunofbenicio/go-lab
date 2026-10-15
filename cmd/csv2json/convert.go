package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// options controls the conversion.
type options struct {
	comma    rune
	noHeader bool // the first row is data; columns are named col1, col2, ...
	types    bool // turn numbers, booleans and empty cells into JSON values
	compact  bool // single-line output
}

// jsonNumber matches exactly the numbers that JSON allows, so "007", "+5" and
// "NaN" stay strings.
var jsonNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// jsonString encodes s without escaping <, > and &.
func jsonString(s string) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // encoding a string cannot fail
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// jsonValue encodes one cell. With typed output, numbers are written as they
// appear in the input (keeping their exact digits).
func jsonValue(cell string, typed bool) []byte {
	if typed {
		switch {
		case cell == "":
			return []byte("null")
		case cell == "true" || cell == "false":
			return []byte(cell)
		case jsonNumber.MatchString(cell):
			return []byte(cell)
		}
	}
	return jsonString(cell)
}

// header returns the column names and the data rows of records.
func header(records [][]string, noHeader bool) (names []string, rows [][]string, err error) {
	if noHeader {
		names = make([]string, len(records[0]))
		for i := range names {
			names[i] = "col" + strconv.Itoa(i+1)
		}
		return names, records, nil
	}
	names = records[0]
	names[0] = strings.TrimPrefix(names[0], "\xef\xbb\xbf") // byte order mark
	seen := make(map[string]bool, len(names))
	for i, name := range names {
		switch {
		case name == "":
			return nil, nil, fmt.Errorf("column %d has an empty name", i+1)
		case seen[name]:
			return nil, nil, fmt.Errorf("duplicate column name %q", name)
		}
		seen[name] = true
	}
	return names, records[1:], nil
}

// convert reads CSV from r and returns a JSON array with one object per row.
// Object keys follow the column order of the input.
func convert(r io.Reader, opt options) ([]byte, error) {
	cr := csv.NewReader(r)
	cr.Comma = opt.comma
	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return []byte("[]"), nil
	}
	names, rows, err := header(records, opt.noHeader)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, row := range rows {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('{')
		for j, cell := range row {
			if j > 0 {
				buf.WriteByte(',')
			}
			buf.Write(jsonString(names[j]))
			buf.WriteByte(':')
			buf.Write(jsonValue(cell, opt.types))
		}
		buf.WriteByte('}')
	}
	buf.WriteByte(']')

	if opt.compact || len(rows) == 0 {
		return buf.Bytes(), nil
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, buf.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	return pretty.Bytes(), nil
}
