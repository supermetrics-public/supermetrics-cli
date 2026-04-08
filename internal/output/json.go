package output

import (
	"encoding/json"
	"io"
	"strings"
)

func printJSON(w io.Writer, data any, useColor bool) error {
	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if !useColor {
		_, err = w.Write(buf)
		if err == nil {
			_, err = w.Write([]byte("\n"))
		}
		return err
	}

	_, err = w.Write([]byte(colorizeJSON(string(buf))))
	if err == nil {
		_, err = w.Write([]byte("\n"))
	}
	return err
}

// colorizeJSON applies ANSI color codes to a pre-formatted JSON string.
// Keys are blue, strings green, numbers cyan, booleans/null yellow.
func colorizeJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)

	i := 0
	for i < len(s) {
		ch := s[i]

		if ch == '"' {
			// Read the full string
			str, end := readJSONString(s, i)
			// Check if this string is an object key (followed by ':' after optional whitespace)
			isKey := false
			for j := end; j < len(s); j++ {
				if s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r' {
					continue
				}
				isKey = s[j] == ':'
				break
			}
			if isKey {
				b.WriteString(colorBlue)
			} else {
				b.WriteString(colorGreen)
			}
			b.WriteString(str)
			b.WriteString(colorReset)
			i = end
		} else if (ch >= '0' && ch <= '9') || ch == '-' {
			// Number
			start := i
			i++
			for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.' || s[i] == 'e' || s[i] == 'E' || s[i] == '+' || s[i] == '-') {
				i++
			}
			b.WriteString(colorCyan)
			b.WriteString(s[start:i])
			b.WriteString(colorReset)
		} else if i+4 <= len(s) && s[i:i+4] == "true" {
			b.WriteString(colorYellow)
			b.WriteString("true")
			b.WriteString(colorReset)
			i += 4
		} else if i+5 <= len(s) && s[i:i+5] == "false" {
			b.WriteString(colorYellow)
			b.WriteString("false")
			b.WriteString(colorReset)
			i += 5
		} else if i+4 <= len(s) && s[i:i+4] == "null" {
			b.WriteString(colorYellow)
			b.WriteString("null")
			b.WriteString(colorReset)
			i += 4
		} else {
			b.WriteByte(ch)
			i++
		}
	}
	return b.String()
}

// readJSONString reads a JSON string starting at position i (which must be '"').
// Returns the full string including quotes and the index after the closing quote.
func readJSONString(s string, i int) (string, int) {
	j := i + 1
	for j < len(s) {
		if s[j] == '\\' {
			j += 2 // skip escaped character
			continue
		}
		if s[j] == '"' {
			j++
			return s[i:j], j
		}
		j++
	}
	return s[i:], len(s)
}
