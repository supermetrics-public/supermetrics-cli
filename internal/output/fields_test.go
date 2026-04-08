package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractField_TopLevel(t *testing.T) {
	item := map[string]any{"id": 1, "name": "test"}
	val, ok := extractField(item, "id")
	assert.True(t, ok)
	assert.Equal(t, 1, val)
}

func TestExtractField_Nested(t *testing.T) {
	item := map[string]any{
		"error": map[string]any{
			"message": "not found",
			"code":    404,
		},
	}
	val, ok := extractField(item, "error.message")
	assert.True(t, ok)
	assert.Equal(t, "not found", val)
}

func TestExtractField_DeeplyNested(t *testing.T) {
	item := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "deep",
			},
		},
	}
	val, ok := extractField(item, "a.b.c")
	assert.True(t, ok)
	assert.Equal(t, "deep", val)
}

func TestFilterFields_MultipleSiblingsFromNestedObject(t *testing.T) {
	data := map[string]any{
		"id": 1,
		"connections": map[string]any{
			"popular": map[string]any{
				"shared":  10,
				"private": 5,
				"total":   15,
			},
		},
	}
	result := FilterFields(data, []string{"id", "connections.popular.shared", "connections.popular.total"})

	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{
		"id":                         1,
		"connections.popular.shared": 10,
		"connections.popular.total":  15,
	}, m)
}

func TestExtractField_Missing(t *testing.T) {
	item := map[string]any{"id": 1}
	_, ok := extractField(item, "name")
	assert.False(t, ok)
}

func TestExtractField_MissingNested(t *testing.T) {
	item := map[string]any{"id": 1}
	_, ok := extractField(item, "error.message")
	assert.False(t, ok)
}

func TestExtractField_NonMapIntermediate(t *testing.T) {
	item := map[string]any{"id": "string_value"}
	_, ok := extractField(item, "id.nested")
	assert.False(t, ok)
}

func TestFilterFields_SingleMap(t *testing.T) {
	data := map[string]any{"id": 1, "name": "test", "status": "active"}
	result := FilterFields(data, []string{"id", "status"})

	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"id": 1, "status": "active"}, m)
}

func TestFilterFields_SingleMapNested(t *testing.T) {
	data := map[string]any{
		"id":    1,
		"error": map[string]any{"message": "fail", "code": 400},
		"name":  "test",
	}
	result := FilterFields(data, []string{"id", "error.message"})

	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"id": 1, "error.message": "fail"}, m)
}

func TestFilterFields_SliceOfMaps(t *testing.T) {
	data := []map[string]any{
		{"id": 1, "name": "a", "status": "ok"},
		{"id": 2, "name": "b", "status": "err"},
	}
	result := FilterFields(data, []string{"id", "status"})

	items, ok := result.([]map[string]any)
	require.True(t, ok)
	assert.Len(t, items, 2)
	assert.Equal(t, map[string]any{"id": 1, "status": "ok"}, items[0])
	assert.Equal(t, map[string]any{"id": 2, "status": "err"}, items[1])
}

func TestFilterFields_SliceOfAny(t *testing.T) {
	data := []any{
		map[string]any{"id": 1, "name": "a"},
		map[string]any{"id": 2, "name": "b"},
	}
	result := FilterFields(data, []string{"id"})

	items, ok := result.([]map[string]any)
	require.True(t, ok)
	assert.Len(t, items, 2)
	assert.Equal(t, map[string]any{"id": 1}, items[0])
	assert.Equal(t, map[string]any{"id": 2}, items[1])
}

func TestFilterFields_ArrayOfArrays(t *testing.T) {
	// Query response shape: first row is headers, rest are data
	data := []any{
		[]any{"id", "name", "status"},
		[]any{1, "a", "ok"},
		[]any{2, "b", "err"},
	}
	result := FilterFields(data, []string{"id", "status"})

	items, ok := result.([]map[string]any)
	require.True(t, ok)
	assert.Len(t, items, 2)
	assert.Equal(t, map[string]any{"id": 1, "status": "ok"}, items[0])
	assert.Equal(t, map[string]any{"id": 2, "status": "err"}, items[1])
}

func TestFilterFields_EmptyFields(t *testing.T) {
	data := map[string]any{"id": 1, "name": "test"}
	result := FilterFields(data, nil)
	assert.Equal(t, data, result)
}

func TestFilterFields_AllMissing(t *testing.T) {
	data := map[string]any{"id": 1, "name": "test"}
	result := FilterFields(data, []string{"foo", "bar"})

	m, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, m)
}

func TestFilterFields_NonMapData(t *testing.T) {
	// Non-map/slice data should be returned as-is
	data := "plain string"
	result := FilterFields(data, []string{"id"})
	assert.Equal(t, data, result)
}

func TestFilterFields_IntegrationWithPrint(t *testing.T) {
	data := map[string]any{"id": 1, "name": "test", "status": "active"}
	var buf bytes.Buffer
	err := Print(&buf, data, PrintOptions{
		Format: FormatJSON,
		Fields: []string{"id", "status"},
	})
	require.NoError(t, err)

	var result map[string]any
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"id": float64(1), "status": "active"}, result)
}

func TestFilterFields_IntegrationWithPrintTable(t *testing.T) {
	data := []map[string]any{
		{"id": 1, "name": "a", "status": "ok"},
		{"id": 2, "name": "b", "status": "err"},
	}
	var buf bytes.Buffer
	err := Print(&buf, data, PrintOptions{
		Format: FormatTable,
		Fields: []string{"id", "status"},
	})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "ID")
	assert.Contains(t, out, "STATUS")
	assert.NotContains(t, out, "NAME")
}

func TestFilterFields_IntegrationWithPrintCSV(t *testing.T) {
	data := []map[string]any{
		{"id": 1, "name": "a", "status": "ok"},
		{"id": 2, "name": "b", "status": "err"},
	}
	var buf bytes.Buffer
	err := Print(&buf, data, PrintOptions{
		Format: FormatCSV,
		Fields: []string{"id", "status"},
	})
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "id")
	assert.Contains(t, out, "status")
	assert.NotContains(t, out, "name")
}
