package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintJSON_Map(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{"name": "test", "id": 42}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatJSON}))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "test", parsed["name"])
}

func TestPrintJSON_ColorizedKeys(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{"name": "Alice"}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatJSON, UseColor: true}))

	out := buf.String()
	// Keys should be blue
	assert.Contains(t, out, colorBlue+`"name"`)
	// String values should be green
	assert.Contains(t, out, colorGreen+`"Alice"`)
}

func TestPrintJSON_ColorizedTypes(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{
		"count":   42,
		"active":  true,
		"deleted": false,
		"note":    nil,
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatJSON, UseColor: true}))

	out := buf.String()
	assert.Contains(t, out, colorCyan+"42"+colorReset)
	assert.Contains(t, out, colorYellow+"true"+colorReset)
	assert.Contains(t, out, colorYellow+"false"+colorReset)
	assert.Contains(t, out, colorYellow+"null"+colorReset)
}

func TestPrintJSON_NoColorProducesValidJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{"key": "value", "n": 1, "ok": true}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatJSON}))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
}

func TestPrintJSON_EscapedStrings(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{"msg": `he said "hello"`}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatJSON, UseColor: true}))

	out := buf.String()
	// Should contain the escaped quote without breaking color
	assert.Contains(t, out, `\"hello\"`)
}

func TestPrintJSON_Slice(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"a": 1},
		{"a": 2},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatJSON}))

	var parsed []any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, 2, len(parsed))
}
