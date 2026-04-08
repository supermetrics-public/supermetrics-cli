package output

import "strings"

// FilterFields filters data to include only the specified fields.
// Supports dot-notation for nested field access (e.g. "error.message").
// Nested fields are promoted to flat output keys using the dot-notation path.
// Returns filtered data preserving the original shape (single map or slice).
func FilterFields(data any, fields []string) any {
	if len(fields) == 0 {
		return data
	}

	switch v := data.(type) {
	case map[string]any:
		return filterMap(v, fields)
	case []map[string]any:
		result := make([]map[string]any, len(v))
		for i, item := range v {
			result[i] = filterMap(item, fields)
		}
		return result
	case []any:
		// Convert to []map[string]any first (handles array-of-arrays, etc.)
		items := toSliceOfMaps(v)
		if items == nil {
			return data
		}
		result := make([]map[string]any, len(items))
		for i, item := range items {
			result[i] = filterMap(item, fields)
		}
		return result
	default:
		return data
	}
}

// filterMap extracts only the requested fields from a single map.
func filterMap(item map[string]any, fields []string) map[string]any {
	result := make(map[string]any, len(fields))
	for _, field := range fields {
		if val, ok := extractField(item, field); ok {
			result[field] = val
		}
	}
	return result
}

// extractField traverses a nested map using a dot-separated path.
func extractField(item map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = item
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
