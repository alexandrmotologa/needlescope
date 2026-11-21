package report

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"github.com/alexandrmotologa/needlescope/pkg/profiler"
)

// ExportCSV writes the 2D degradation matrix as a structured CSV dataset.
func ExportCSV(report *profiler.MatrixReport, w io.Writer) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	header := []string{
		"context_length",
		"depth",
		"total_trials",
		"successful_trials",
		"recall_rate",
		"average_score",
		"mean_latency_ms",
		"p95_latency_ms",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, length := range report.ContextLengths {
		for _, depth := range report.Depths {
			key := profiler.CellKey(length, depth)
			cell, exists := report.Cells[key]
			if !exists {
				continue
			}

			row := []string{
				fmt.Sprintf("%d", cell.ContextLength),
				fmt.Sprintf("%.2f", cell.Depth),
				fmt.Sprintf("%d", cell.TotalTrials),
				fmt.Sprintf("%d", cell.SuccessfulTrials),
				fmt.Sprintf("%.4f", cell.RecallRate),
				fmt.Sprintf("%.4f", cell.AverageScore),
				fmt.Sprintf("%.2f", cell.MeanLatencyMs),
				fmt.Sprintf("%.2f", cell.P95LatencyMs),
			}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
	}

	return nil
}

// SaveCSV writes the report as CSV to a file.
func SaveCSV(report *profiler.MatrixReport, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create CSV file %s: %w", filePath, err)
	}
	defer f.Close()

	return ExportCSV(report, f)
}
