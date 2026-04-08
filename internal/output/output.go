package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	FormatJSON  = "json"
	FormatTable = "table"
	FormatCSV   = "csv"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// PrintOptions controls output formatting behavior.
type PrintOptions struct {
	Format   string
	UseColor bool
	Flatten  bool     // Flatten nested data in table output (CSV always flattens)
	Fields   []string // Filter output to only these fields (dot-notation supported)
}

// Print formats data according to the specified options and writes to w.
// When UseColor is true, nested values in table output are highlighted.
func Print(w io.Writer, data any, opts PrintOptions) error {
	if len(opts.Fields) > 0 {
		data = FilterFields(data, opts.Fields)
	}
	switch opts.Format {
	case FormatJSON:
		return printJSON(w, data, opts.UseColor)
	case FormatTable:
		return printTable(w, data, opts.UseColor, opts.Flatten)
	case FormatCSV:
		return printCSV(w, data)
	default:
		return fmt.Errorf("unknown output format: %s", opts.Format)
	}
}

// APIErrorFields holds structured error information for rendering.
type APIErrorFields struct {
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
	Code        string `json:"code,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
}

// PrintError renders a structured API error to w based on the output format.
// For JSON format, it writes a structured JSON object.
// For table/CSV format, it writes a human-readable multi-line format.
func PrintError(w io.Writer, e APIErrorFields, format string, useColor bool) {
	if format == FormatJSON {
		printErrorJSON(w, e, useColor)
		return
	}
	printErrorText(w, e, useColor)
}

func printErrorJSON(w io.Writer, e APIErrorFields, useColor bool) {
	wrapper := struct {
		Error APIErrorFields `json:"error"`
	}{Error: e}

	buf, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		fmt.Fprintf(w, "Error: %s\n", e.Message)
		return
	}

	if useColor {
		fmt.Fprintln(w, colorizeJSON(string(buf)))
	} else {
		fmt.Fprintln(w, string(buf))
	}
}

func printErrorText(w io.Writer, e APIErrorFields, useColor bool) {
	// Headline
	if useColor {
		fmt.Fprintf(w, "%sError:%s %s\n", colorRed, colorReset, e.Message)
	} else {
		fmt.Fprintf(w, "Error: %s\n", e.Message)
	}

	// Description
	if e.Description != "" {
		fmt.Fprintln(w)
		for _, line := range wrapText(e.Description, 76) {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}

	// Metadata
	if e.Code != "" || e.RequestID != "" {
		fmt.Fprintln(w)
		if e.Code != "" {
			if useColor {
				fmt.Fprintf(w, "  %sCode:       %s%s\n", colorDim, e.Code, colorReset)
			} else {
				fmt.Fprintf(w, "  Code:       %s\n", e.Code)
			}
		}
		if e.RequestID != "" {
			if useColor {
				fmt.Fprintf(w, "  %sRequest ID: %s%s\n", colorDim, e.RequestID, colorReset)
			} else {
				fmt.Fprintf(w, "  Request ID: %s\n", e.RequestID)
			}
		}
	}
}

// wrapText splits text into lines of at most maxWidth characters,
// breaking at word boundaries.
func wrapText(text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > maxWidth {
			lines = append(lines, line)
			line = word
		} else {
			line += " " + word
		}
	}
	lines = append(lines, line)
	return lines
}

func toSliceOfMaps(data any) []map[string]any {
	if data == nil {
		return nil
	}
	if m, ok := data.(map[string]any); ok {
		return []map[string]any{m}
	}
	if s, ok := data.([]map[string]any); ok {
		return s
	}
	if s, ok := data.([]any); ok {
		// Detect array-of-arrays with header row (e.g. query data responses).
		if items := arrayOfArraysToMaps(s); items != nil {
			return items
		}
		var result []map[string]any
		for _, item := range s {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
			} else {
				result = append(result, structToMap(item))
			}
		}
		return result
	}
	return []map[string]any{structToMap(data)}
}

// arrayOfArraysToMaps converts a [][]any where the first row is string headers
// into []map[string]any. Returns nil if the data doesn't match this shape.
func arrayOfArraysToMaps(data []any) []map[string]any {
	if len(data) < 2 {
		return nil
	}
	// First row must be all strings (headers).
	headerRow, ok := data[0].([]any)
	if !ok {
		return nil
	}
	headers := make([]string, len(headerRow))
	for i, h := range headerRow {
		s, ok := h.(string)
		if !ok {
			return nil
		}
		headers[i] = s
	}
	// Remaining rows must be arrays of the same length.
	var result []map[string]any
	for _, row := range data[1:] { //nolint:gosec // len(data) >= 2 checked on line 176
		arr, ok := row.([]any)
		if !ok || len(arr) != len(headers) {
			return nil
		}
		m := make(map[string]any, len(headers))
		for i, h := range headers {
			m[h] = arr[i]
		}
		result = append(result, m)
	}
	return result
}

func structToMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]any{"value": fmt.Sprintf("%v", v)}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{"value": fmt.Sprintf("%v", v)}
	}
	return m
}

func formatValue(v any, useColor bool) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case []any:
		n := len(val)
		label := "items"
		if n == 1 {
			label = "item"
		}
		if useColor {
			return fmt.Sprintf("%s%d %s%s", colorCyan, n, label, colorReset)
		}
		return fmt.Sprintf("%d %s", n, label)
	case map[string]any:
		n := len(val)
		label := "fields"
		if n == 1 {
			label = "field"
		}
		if useColor {
			return fmt.Sprintf("%s{%d %s}%s", colorCyan, n, label, colorReset)
		}
		return fmt.Sprintf("{%d %s}", n, label)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// flattenItems applies all flattening transformations to a slice of maps:
// 1. Flatten nested objects to dot-notation keys
// 2. Expand the single array-of-objects field into multiple rows (if exactly one exists)
// 3. Join remaining primitive arrays with semicolons
func flattenItems(items []map[string]any) []map[string]any {
	// Step 1: flatten nested objects
	for i, item := range items {
		items[i] = flattenNestedObjects(item, "")
	}

	// Step 2: find the single expandable array-of-objects field
	expandField := findExpandableArrayField(items)
	if expandField != "" {
		items = expandArrayField(items, expandField)
	}

	// Step 3: join remaining arrays as semicolons
	for i, item := range items {
		items[i] = joinArrayValues(item)
	}

	return items
}

// flattenNestedObjects recursively flattens nested map[string]any values
// into dot-notation keys. Arrays are left untouched.
func flattenNestedObjects(item map[string]any, prefix string) map[string]any {
	result := make(map[string]any)
	for k, v := range item {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if m, ok := v.(map[string]any); ok {
			for fk, fv := range flattenNestedObjects(m, key) {
				result[fk] = fv
			}
		} else {
			result[key] = v
		}
	}
	return result
}

// findExpandableArrayField returns the name of the single array-of-objects
// field shared by all items, or "" if there isn't exactly one.
func findExpandableArrayField(items []map[string]any) string {
	if len(items) == 0 {
		return ""
	}

	// Count array-of-objects fields across all items
	candidates := make(map[string]bool)
	for _, item := range items {
		for k, v := range item {
			arr, ok := v.([]any)
			if !ok || len(arr) == 0 {
				continue
			}
			if _, isMap := arr[0].(map[string]any); isMap {
				candidates[k] = true
			}
		}
	}

	if len(candidates) == 1 {
		for k := range candidates {
			return k
		}
	}
	return ""
}

// expandArrayField expands one array-of-objects field into multiple rows.
// Parent scalar fields are repeated on each expanded row.
func expandArrayField(items []map[string]any, field string) []map[string]any {
	var result []map[string]any
	for _, item := range items {
		arr, ok := item[field].([]any)
		if !ok || len(arr) == 0 {
			// No array to expand — keep the row, remove the field
			row := make(map[string]any)
			for k, v := range item {
				if k != field {
					row[k] = v
				}
			}
			result = append(result, row)
			continue
		}

		for _, elem := range arr {
			row := make(map[string]any)
			// Copy parent fields (skip the expanded array)
			for k, v := range item {
				if k != field {
					row[k] = v
				}
			}
			// Merge child fields with prefix
			if m, ok := elem.(map[string]any); ok {
				flat := flattenNestedObjects(m, field)
				for k, v := range flat {
					row[k] = v
				}
			}
			result = append(result, row)
		}
	}
	return result
}

// joinArrayValues converts any remaining []any values to semicolon-joined strings.
// Arrays of mixed/complex types fall back to the "N items" summary.
func joinArrayValues(item map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range item {
		arr, ok := v.([]any)
		if !ok {
			result[k] = v
			continue
		}
		if len(arr) == 0 {
			result[k] = ""
			continue
		}
		// Check if all elements are primitives (not maps or arrays)
		allPrimitive := true
		for _, elem := range arr {
			switch elem.(type) {
			case map[string]any, []any:
				allPrimitive = false
			}
			if !allPrimitive {
				break
			}
		}
		if allPrimitive {
			parts := make([]string, len(arr))
			for i, elem := range arr {
				parts[i] = fmt.Sprintf("%v", elem)
			}
			result[k] = strings.Join(parts, ";")
		} else {
			// Fall back to summary for complex arrays
			n := len(arr)
			label := "items"
			if n == 1 {
				label = "item"
			}
			result[k] = fmt.Sprintf("%d %s", n, label)
		}
	}
	return result
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysFromSet(s map[string]bool) []string {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
