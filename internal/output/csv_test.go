package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintCSV(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"name": "Alice", "age": 30},
		{"name": "Bob", "age": 25},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatCSV}))

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.Equal(t, 3, len(lines)) // header + 2 rows

	// Header should contain column names
	header := lines[0]
	assert.Contains(t, header, "age")
	assert.Contains(t, header, "name")
}

func TestPrintCSV_Empty(t *testing.T) {
	var buf bytes.Buffer

	require.NoError(t, Print(&buf, nil, PrintOptions{Format: FormatCSV}))

	assert.Equal(t, 0, buf.Len())
}

func TestPrintCSV_NestedArrayFlattened(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{
			"name": "Alice",
			"accounts": []any{
				map[string]any{"id": "1", "site": "A"},
				map[string]any{"id": "2", "site": "B"},
			},
		},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatCSV}))

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Equal(t, 3, len(lines)) // header + 2 expanded rows
	assert.Contains(t, lines[0], "accounts.id")
	assert.Contains(t, out, "Alice")
}

func TestPrintCSV_NestedObjectFlattened(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{
			"name":    "Alice",
			"ds_info": map[string]any{"ds_id": "GA", "name": "Google Analytics"},
		},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatCSV}))

	out := buf.String()
	assert.Contains(t, out, "ds_info.ds_id")
	assert.Contains(t, out, "GA")
}

func TestPrintCSV_PrimitiveArrayJoined(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{
			"name":   "Alice",
			"scopes": []any{"read", "write", "admin"},
		},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatCSV}))

	out := buf.String()
	assert.Contains(t, out, "read;write;admin")
}

func TestPrintCSV_EmptyArray(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{
			"name":   "Alice",
			"scopes": []any{},
		},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatCSV}))

	// Empty array should produce empty value, not "0 items"
	out := buf.String()
	assert.NotContains(t, out, "0 items")
}

func TestPrintCSV_MultipleArrayFields_NoExpansion(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{
			"name":  "Alice",
			"listA": []any{map[string]any{"x": 1}},
			"listB": []any{map[string]any{"y": 2}},
		},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatCSV}))

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Should NOT expand when multiple array-of-objects fields exist
	require.Equal(t, 2, len(lines)) // header + 1 row
}

func TestPrintCSV_NestedObjectInsideExpandedArray(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{
			"user": "Alice",
			"items": []any{
				map[string]any{"id": "1", "meta": map[string]any{"status": "active"}},
			},
		},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatCSV}))

	out := buf.String()
	assert.Contains(t, out, "items.meta.status")
	assert.Contains(t, out, "active")
}

func TestPrintCSV_InconsistentArrayElementShapes(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{
			"user": "Alice",
			"items": []any{
				map[string]any{"id": "1", "extra": "yes"},
				map[string]any{"id": "2"},
			},
		},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatCSV}))

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Equal(t, 3, len(lines))
	// Header should include all keys from all elements
	assert.Contains(t, lines[0], "items.extra")
}

func TestPrintCSV_NullValues(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{
			"name":  "Alice",
			"email": nil,
		},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatCSV}))

	out := buf.String()
	assert.NotContains(t, out, "nil")
	assert.NotContains(t, out, "<nil>")
}
