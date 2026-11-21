package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/alexandrmotologa/needlescope/pkg/profiler"
)

// ANSI color escape sequences
const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	dim       = "\033[2m"
	greenBg   = "\033[48;5;28m\033[38;5;255m"  // 95-100%
	yellowBg  = "\033[48;5;178m\033[38;5;232m" // 70-94%
	orangeBg  = "\033[48;5;208m\033[38;5;232m" // 50-69%
	redBg     = "\033[48;5;160m\033[38;5;255m" // 0-49%
	headerBg  = "\033[48;5;236m\033[38;5;255m"
	cyanFg    = "\033[38;5;51m"
	grayFg    = "\033[38;5;244m"
)

// PrintTerminal outputs an ANSI colored 2D heatmap matrix and attention metrics.
func PrintTerminal(report *profiler.MatrixReport, w io.Writer) {
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "%s%s needlescope Attention Degradation Matrix%s\n", bold, cyanFg, reset)
	fmt.Fprintf(w, "%sModel: %s%s | %sProvider: %s%s | %sTrials: %d%s\n", dim, report.Model, reset, dim, report.Provider, reset, dim, report.TotalTrials, reset)
	fmt.Fprintln(w, strings.Repeat("─", 82))

	// Summary Cards
	fmt.Fprintf(w, " Overall Recall: %s%5.1f%%%s  │  Primacy (<=15%%): %s%5.1f%%%s  │  Recency (>=85%%): %s%5.1f%%%s\n",
		bold, report.OverallRecall*100, reset,
		cyanFg, report.PrimacyRecall*100, reset,
		cyanFg, report.RecencyRecall*100, reset,
	)
	fmt.Fprintf(w, " Attention Valley: %s%5.1f%%%s │  Degradation Delta: %s%5.1f%%%s│  U-Curve Alpha: %s%.3f%s\n",
		redBg, report.MiddleRecall*100, reset,
		bold, report.DegradationDelta*100, reset,
		cyanFg, report.GlobalUCurve.Alpha, reset,
	)

	if report.CriticalBlindSpot.Detected {
		fmt.Fprintf(w, "%s! Critical Blind Spot Detected: [%.0f%% .. %.0f%%] depth (Min Recall: %.1f%%)%s\n",
			orangeBg, report.CriticalBlindSpot.MinDepth*100, report.CriticalBlindSpot.MaxDepth*100, report.CriticalBlindSpot.MinRecall*100, reset)
	}

	fmt.Fprintln(w, strings.Repeat("─", 82))
	fmt.Fprintf(w, "\n%s2D Recall Heatmap Grid (Context Length vs Insertion Depth)%s\n\n", bold, reset)

	// Header row: Depths
	fmt.Fprintf(w, "%-9s ", "Context")
	for _, d := range report.Depths {
		fmt.Fprintf(w, " %3.0f%% ", d*100)
	}
	fmt.Fprintln(w, "")

	fmt.Fprintf(w, "──────────")
	for range report.Depths {
		fmt.Fprintf(w, "──────")
	}
	fmt.Fprintln(w, "")

	// Matrix Rows: Context Lengths
	for _, length := range report.ContextLengths {
		lenLabel := formatTokenCount(length)
		fmt.Fprintf(w, "%-9s ", lenLabel)

		for _, depth := range report.Depths {
			key := profiler.CellKey(length, depth)
			cell, exists := report.Cells[key]
			if !exists {
				fmt.Fprint(w, "   -  ")
				continue
			}

			val := cell.RecallRate * 100
			color := colorForRecall(cell.RecallRate)
			fmt.Fprintf(w, "%s %3.0f%% %s", color, val, reset)
		}
		fmt.Fprintln(w, "")
	}

	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Legend: %s 100%% %s %s >=70%% %s %s >=50%% %s %s <50%% %s\n",
		greenBg, reset, yellowBg, reset, orangeBg, reset, redBg, reset)
	fmt.Fprintln(w, "")
}

func formatTokenCount(n int) string {
	if n >= 1024*1024 {
		return fmt.Sprintf("%dM", n/(1024*1024))
	}
	if n >= 1024 {
		return fmt.Sprintf("%dk", n/1024)
	}
	return fmt.Sprintf("%dt", n)
}

func colorForRecall(r float64) string {
	if r >= 0.95 {
		return greenBg
	}
	if r >= 0.70 {
		return yellowBg
	}
	if r >= 0.50 {
		return orangeBg
	}
	return redBg
}
