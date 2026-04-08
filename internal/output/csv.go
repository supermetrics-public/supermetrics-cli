package output

import (
	"encoding/csv"
	"io"
)

func printCSV(w io.Writer, data any) error {
	items := toSliceOfMaps(data)
	if len(items) == 0 {
		return nil
	}

	// CSV always flattens nested data
	items = flattenItems(items)

	colSet := make(map[string]bool)
	for _, item := range items {
		for k := range item {
			colSet[k] = true
		}
	}
	cols := sortedKeysFromSet(colSet)

	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write(cols); err != nil {
		return err
	}
	for _, item := range items {
		row := make([]string, len(cols))
		for i, col := range cols {
			row[i] = formatValue(item[col], false)
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	return nil
}
