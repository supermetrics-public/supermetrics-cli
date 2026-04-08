package output

import (
	"fmt"
	"io"
	"strings"
)

func printTable(w io.Writer, data any, useColor bool, flatten bool) error {
	items := toSliceOfMaps(data)
	if len(items) == 0 {
		fmt.Fprintln(w, "No results.")
		return nil
	}

	if flatten {
		items = flattenItems(items)
	}

	if len(items) == 1 {
		return printVerticalTable(w, items[0], useColor)
	}
	return printHorizontalTable(w, items, useColor)
}

func printVerticalTable(w io.Writer, item map[string]any, useColor bool) error {
	keys := sortedKeys(item)
	maxKeyLen := 0
	for _, k := range keys {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	}

	maxValLen := 0
	for _, k := range keys {
		vl := len(formatValue(item[k], false))
		if vl > maxValLen {
			maxValLen = vl
		}
	}
	if maxValLen > 50 {
		maxValLen = 50
	}

	// Top border
	fmt.Fprintf(w, "┌%s┬%s┐\n", strings.Repeat("─", maxKeyLen+2), strings.Repeat("─", maxValLen+2))

	for _, k := range keys {
		plain := formatValue(item[k], false)
		display := formatValue(item[k], useColor)
		if len(plain) > maxValLen {
			plain = plain[:maxValLen-1] + "~"
			display = plain
		}
		padding := maxValLen - len(plain)

		if useColor {
			fmt.Fprintf(w, "│ %s%-*s%s │ %s%s │\n", colorGreen, maxKeyLen, k, colorReset, display, strings.Repeat(" ", padding))
		} else {
			fmt.Fprintf(w, "│ %-*s │ %s%s │\n", maxKeyLen, k, display, strings.Repeat(" ", padding))
		}
	}

	// Bottom border
	fmt.Fprintf(w, "└%s┴%s┘\n", strings.Repeat("─", maxKeyLen+2), strings.Repeat("─", maxValLen+2))
	return nil
}

func printHorizontalTable(w io.Writer, items []map[string]any, useColor bool) error {
	colSet := make(map[string]bool)
	for _, item := range items {
		for k := range item {
			colSet[k] = true
		}
	}
	cols := sortedKeysFromSet(colSet)

	widths := make([]int, len(cols))
	for i, col := range cols {
		widths[i] = len(col)
	}
	for _, item := range items {
		for i, col := range cols {
			val := formatValue(item[col], false)
			if len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}
	for i := range widths {
		if widths[i] > 50 {
			widths[i] = 50
		}
	}

	// Top border
	printHorizontalBorder(w, widths, "┌", "┬", "┐")

	// Header row
	for i, col := range cols {
		fmt.Fprint(w, "│ ")
		header := strings.ToUpper(col)
		if useColor {
			fmt.Fprintf(w, "%s%-*s%s", colorGreen, widths[i], header, colorReset)
		} else {
			fmt.Fprintf(w, "%-*s", widths[i], header)
		}
		fmt.Fprint(w, " ")
	}
	fmt.Fprintln(w, "│")

	// Header/body separator
	printHorizontalBorder(w, widths, "├", "┼", "┤")

	// Data rows
	for _, item := range items {
		for i, col := range cols {
			fmt.Fprint(w, "│ ")
			plain := formatValue(item[col], false)
			display := formatValue(item[col], useColor)
			if len(plain) > widths[i] {
				plain = plain[:widths[i]-1] + "~"
				display = plain
			}
			padding := widths[i] - len(plain)
			fmt.Fprint(w, display)
			if padding > 0 {
				fmt.Fprint(w, strings.Repeat(" ", padding))
			}
			fmt.Fprint(w, " ")
		}
		fmt.Fprintln(w, "│")
	}

	// Bottom border
	printHorizontalBorder(w, widths, "└", "┴", "┘")
	return nil
}

func printHorizontalBorder(w io.Writer, widths []int, left, mid, right string) {
	fmt.Fprint(w, left)
	for i, width := range widths {
		if i > 0 {
			fmt.Fprint(w, mid)
		}
		fmt.Fprint(w, strings.Repeat("─", width+2))
	}
	fmt.Fprintln(w, right)
}
