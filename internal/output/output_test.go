package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrint_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := Print(&buf, "data", PrintOptions{Format: "xml"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "xml")
}

func TestFormatValue_NestedArray(t *testing.T) {
	arr := []any{
		map[string]any{"id": 1},
		map[string]any{"id": 2},
		map[string]any{"id": 3},
	}
	plain := formatValue(arr, false)
	assert.Equal(t, "3 items", plain)

	colored := formatValue(arr, true)
	assert.Contains(t, colored, "3 items")
	assert.Contains(t, colored, colorCyan)
}

func TestFormatValue_NestedArraySingular(t *testing.T) {
	arr := []any{map[string]any{"id": 1}}
	plain := formatValue(arr, false)
	assert.Equal(t, "1 item", plain)
}

func TestFormatValue_NestedObject(t *testing.T) {
	obj := map[string]any{"a": 1, "b": 2}
	plain := formatValue(obj, false)
	assert.Equal(t, "{2 fields}", plain)
}

func TestFormatValue_NestedObjectSingular(t *testing.T) {
	obj := map[string]any{"a": 1}
	plain := formatValue(obj, false)
	assert.Equal(t, "{1 field}", plain)
}

func TestPrintError_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	e := APIErrorFields{
		Message:     "Invalid data source ID",
		Description: `Parameter "ds_id" is expected to be a valid data source ID.`,
		Code:        "PARAM_TYPE_INVALID",
		RequestID:   "abc123",
	}

	PrintError(&buf, e, FormatTable, false)
	out := buf.String()

	assert.Contains(t, out, "Error: Invalid data source ID")
	assert.Contains(t, out, `Parameter "ds_id"`)
	assert.Contains(t, out, "Code:       PARAM_TYPE_INVALID")
	assert.Contains(t, out, "Request ID: abc123")
}

func TestPrintError_TextFormatWithColor(t *testing.T) {
	var buf bytes.Buffer
	e := APIErrorFields{
		Message: "Something went wrong",
		Code:    "ERR",
	}

	PrintError(&buf, e, FormatTable, true)
	out := buf.String()

	assert.Contains(t, out, colorRed+"Error:"+colorReset)
}

func TestPrintError_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	e := APIErrorFields{
		Message:   "Not found",
		Code:      "NOT_FOUND",
		RequestID: "xyz",
	}

	PrintError(&buf, e, FormatJSON, false)
	out := buf.String()

	var parsed struct {
		Error APIErrorFields `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &parsed))
	assert.Equal(t, "Not found", parsed.Error.Message)
	assert.Equal(t, "NOT_FOUND", parsed.Error.Code)
}

func TestPrintError_MessageOnly(t *testing.T) {
	var buf bytes.Buffer
	e := APIErrorFields{Message: "Something failed"}

	PrintError(&buf, e, FormatTable, false)
	out := buf.String()

	assert.Contains(t, out, "Error: Something failed")
	// No code or request ID — should not have metadata section
	assert.NotContains(t, out, "Code:")
	assert.NotContains(t, out, "Request ID:")
}

func TestPrintError_JSONFormatWithColor(t *testing.T) {
	var buf bytes.Buffer
	e := APIErrorFields{
		Message: "Not found",
		Code:    "NOT_FOUND",
	}

	PrintError(&buf, e, FormatJSON, true)
	out := buf.String()

	assert.Contains(t, out, "Not found")
	assert.Contains(t, out, "NOT_FOUND")
	// Should contain ANSI color codes from colorizeJSON
	assert.Contains(t, out, "\033[")
}

// --- arrayOfArraysToMaps tests ---

func TestArrayOfArraysToMaps_Valid(t *testing.T) {
	data := []any{
		[]any{"id", "name"},
		[]any{float64(1), "Alice"},
		[]any{float64(2), "Bob"},
	}
	result := arrayOfArraysToMaps(data)

	require.Len(t, result, 2)
	assert.Equal(t, float64(1), result[0]["id"])
	assert.Equal(t, "Alice", result[0]["name"])
	assert.Equal(t, float64(2), result[1]["id"])
	assert.Equal(t, "Bob", result[1]["name"])
}

func TestArrayOfArraysToMaps_SingleDataRow(t *testing.T) {
	data := []any{
		[]any{"col"},
		[]any{"val"},
	}
	result := arrayOfArraysToMaps(data)

	require.Len(t, result, 1)
	assert.Equal(t, "val", result[0]["col"])
}

func TestArrayOfArraysToMaps_TooFewRows(t *testing.T) {
	// Only header, no data
	assert.Nil(t, arrayOfArraysToMaps([]any{[]any{"id"}}))
	// Empty slice
	assert.Nil(t, arrayOfArraysToMaps([]any{}))
}

func TestArrayOfArraysToMaps_FirstRowNotArray(t *testing.T) {
	data := []any{
		map[string]any{"id": 1},
		[]any{1},
	}
	assert.Nil(t, arrayOfArraysToMaps(data))
}

func TestArrayOfArraysToMaps_NonStringHeader(t *testing.T) {
	data := []any{
		[]any{"id", float64(42)},
		[]any{1, 2},
	}
	assert.Nil(t, arrayOfArraysToMaps(data))
}

func TestArrayOfArraysToMaps_DataRowWrongLength(t *testing.T) {
	data := []any{
		[]any{"id", "name"},
		[]any{1}, // too short
	}
	assert.Nil(t, arrayOfArraysToMaps(data))
}

func TestArrayOfArraysToMaps_DataRowNotArray(t *testing.T) {
	data := []any{
		[]any{"id"},
		"not-an-array",
	}
	assert.Nil(t, arrayOfArraysToMaps(data))
}

// --- toSliceOfMaps tests ---

func TestToSliceOfMaps_Nil(t *testing.T) {
	assert.Nil(t, toSliceOfMaps(nil))
}

func TestToSliceOfMaps_SingleMap(t *testing.T) {
	m := map[string]any{"id": 1}
	result := toSliceOfMaps(m)
	require.Len(t, result, 1)
	assert.Equal(t, 1, result[0]["id"])
}

func TestToSliceOfMaps_SliceOfMaps(t *testing.T) {
	s := []map[string]any{{"a": 1}, {"b": 2}}
	result := toSliceOfMaps(s)
	assert.Equal(t, s, result)
}

func TestToSliceOfMaps_SliceOfAnyWithMaps(t *testing.T) {
	s := []any{
		map[string]any{"x": 10},
		map[string]any{"x": 20},
	}
	result := toSliceOfMaps(s)
	require.Len(t, result, 2)
	assert.Equal(t, 10, result[0]["x"])
	assert.Equal(t, 20, result[1]["x"])
}

func TestToSliceOfMaps_ArrayOfArrays(t *testing.T) {
	s := []any{
		[]any{"id", "name"},
		[]any{float64(1), "Alice"},
	}
	result := toSliceOfMaps(s)
	require.Len(t, result, 1)
	assert.Equal(t, float64(1), result[0]["id"])
	assert.Equal(t, "Alice", result[0]["name"])
}

func TestToSliceOfMaps_Struct(t *testing.T) {
	type item struct {
		Name string `json:"name"`
	}
	result := toSliceOfMaps(item{Name: "test"})
	require.Len(t, result, 1)
	assert.Equal(t, "test", result[0]["name"])
}

func TestStructToMap(t *testing.T) {
	type Sample struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	m := structToMap(Sample{Name: "test", Count: 5})
	assert.Equal(t, "test", m["name"])
	// JSON numbers unmarshal as float64
	assert.Equal(t, float64(5), m["count"])
}
