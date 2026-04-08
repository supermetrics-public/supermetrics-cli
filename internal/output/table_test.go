package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintTable_SingleObject(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{"name": "Alice", "role": "admin"}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatTable}))

	out := buf.String()
	assert.Contains(t, out, "│ name")
	assert.Contains(t, out, "Alice")
	assert.Contains(t, out, "│ role")
	assert.Contains(t, out, "┌")
	assert.Contains(t, out, "└")
}

func TestPrintTable_MultipleObjects(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"id": 1, "name": "Alice"},
		{"id": 2, "name": "Bob"},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatTable}))

	out := buf.String()
	// Should have uppercase headers
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "NAME")
	// Should have box-drawing separator
	assert.Contains(t, out, "├")
	// Should have both rows
	assert.Contains(t, out, "Alice")
	assert.Contains(t, out, "Bob")
}

func TestPrintTable_Empty(t *testing.T) {
	var buf bytes.Buffer

	require.NoError(t, Print(&buf, nil, PrintOptions{Format: FormatTable}))

	assert.Contains(t, buf.String(), "No results")
}

func TestPrintTable_NestedArrayInColumn(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{
			"name":     "Alice",
			"accounts": []any{map[string]any{"id": 1}, map[string]any{"id": 2}},
		},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatTable}))

	out := buf.String()
	assert.Contains(t, out, "2 items")
}

func TestPrintTable_FlattenFlag(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{
			"name": "Alice",
			"accounts": []any{
				map[string]any{"id": "1"},
				map[string]any{"id": "2"},
			},
		},
	}

	// Without flatten — should show summary
	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatTable}))
	out := buf.String()
	assert.Contains(t, out, "2 items")

	// With flatten — should expand
	buf.Reset()
	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatTable, Flatten: true}))
	out = buf.String()
	assert.NotContains(t, out, "2 items")
	assert.Contains(t, out, "ACCOUNTS.ID")
}

func TestPrintTable_HorizontalColoredHeaders(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"id": 1, "name": "Alice"},
		{"id": 2, "name": "Bob"},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatTable, UseColor: true}))

	out := buf.String()
	assert.Contains(t, out, colorGreen+"ID")
	assert.Contains(t, out, colorGreen+"NAME")
}

func TestPrintTable_VerticalColoredKeys(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]any{"name": "Alice", "role": "admin"}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatTable, UseColor: true}))

	out := buf.String()
	assert.Contains(t, out, colorGreen+"name")
}

func TestPrintTable_HorizontalBorders(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]any{
		{"id": 1, "name": "Alice"},
		{"id": 2, "name": "Bob"},
	}

	require.NoError(t, Print(&buf, data, PrintOptions{Format: FormatTable}))

	out := buf.String()
	for _, ch := range []string{"┌", "┐", "├", "┤", "└", "┘", "│", "─"} {
		assert.Contains(t, out, ch)
	}
}
