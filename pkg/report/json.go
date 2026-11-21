package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/alexandrmotologa/needlescope/pkg/profiler"
)

// ExportJSON encodes the report to indented JSON in the target writer.
func ExportJSON(report *profiler.MatrixReport, w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// SaveJSON writes the report to a specified file path.
func SaveJSON(report *profiler.MatrixReport, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer f.Close()

	return ExportJSON(report, f)
}
