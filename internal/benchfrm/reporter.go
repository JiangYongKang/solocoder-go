package benchfrm

import (
	"fmt"
	"strings"
	"time"
)

type TextReporter struct{}

func NewTextReporter() *TextReporter {
	return &TextReporter{}
}

func (r *TextReporter) Report(stats []GroupStatistics) string {
	var sb strings.Builder

	sb.WriteString("=== Benchmark Results ===\n")
	sb.WriteString(fmt.Sprintf("Generated at: %s\n\n", time.Now().Format(time.RFC3339)))

	for _, s := range stats {
		sb.WriteString(fmt.Sprintf("Group: %s\n", s.Name))
		sb.WriteString(fmt.Sprintf("  Iterations:     %d\n", s.Iterations))
		sb.WriteString(fmt.Sprintf("  Mean Duration:  %v\n", s.MeanDuration))
		sb.WriteString(fmt.Sprintf("  Min Duration:   %v\n", s.MinDuration))
		sb.WriteString(fmt.Sprintf("  Max Duration:   %v\n", s.MaxDuration))
		sb.WriteString(fmt.Sprintf("  Std Dev:        %v\n", s.StdDevDuration))
		sb.WriteString(fmt.Sprintf("  Mean Alloc Bytes: %d bytes/op\n", s.MeanAllocBytes))
		sb.WriteString(fmt.Sprintf("  Mean Alloc Count: %d allocs/op\n\n", s.MeanAllocCount))
	}

	return sb.String()
}

func (r *TextReporter) ReportComparison(report ComparisonReport) string {
	var sb strings.Builder

	sb.WriteString("=== Comparison Report ===\n")
	sb.WriteString(fmt.Sprintf("Baseline: %s\n", report.Baseline))
	sb.WriteString(fmt.Sprintf("Generated at: %s\n\n", report.GeneratedAt.Format(time.RFC3339)))

	sb.WriteString(fmt.Sprintf("%-20s %-18s %-15s %-15s %-12s %-12s\n",
		"Group", "Mean Duration", "Duration Δ%", "Alloc Bytes", "Bytes Δ%", "Allocs Δ%"))
	sb.WriteString(strings.Repeat("-", 95) + "\n")

	for _, item := range report.Items {
		var durMarker, bytesMarker, allocsMarker string

		if item.VsBaselinePct > 0 {
			durMarker = " ↑"
		} else if item.VsBaselinePct < 0 {
			durMarker = " ↓"
		}
		if item.AllocBytesPct > 0 {
			bytesMarker = " ↑"
		} else if item.AllocBytesPct < 0 {
			bytesMarker = " ↓"
		}
		if item.AllocCountPct > 0 {
			allocsMarker = " ↑"
		} else if item.AllocCountPct < 0 {
			allocsMarker = " ↓"
		}

		sb.WriteString(fmt.Sprintf("%-20s %-18v %-15s %-15s %-12s %-12s\n",
			item.Group,
			item.MeanDuration,
			fmt.Sprintf("%+.2f%%%s", item.VsBaselinePct, durMarker),
			fmt.Sprintf("%d", item.MeanAllocBytes),
			fmt.Sprintf("%+.2f%%%s", item.AllocBytesPct, bytesMarker),
			fmt.Sprintf("%+.2f%%%s", item.AllocCountPct, allocsMarker)))
	}

	return sb.String()
}

func (r *TextReporter) ReportRegression(report RegressionReport) string {
	var sb strings.Builder

	sb.WriteString("=== Regression Check Report ===\n")
	sb.WriteString(fmt.Sprintf("Generated at: %s\n", report.GeneratedAt.Format(time.RFC3339)))
	if report.IsRegression {
		sb.WriteString("⚠️  PERFORMANCE REGRESSION DETECTED!\n\n")
	} else {
		sb.WriteString("✅ No performance regression detected.\n\n")
	}

	sb.WriteString(fmt.Sprintf("%-20s %-15s %-15s %-15s %s\n",
		"Metric", "Current", "Baseline", "Degradation %", "Status"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for _, check := range report.Checks {
		status := "OK"
		if check.IsDegraded {
			status = "⚠️  REGRESSED"
		}
		sb.WriteString(fmt.Sprintf("%-20s %-15.2f %-15.2f %-15.2f%% %s\n",
			check.MetricName,
			check.CurrentValue,
			check.BaselineValue,
			check.DegradationPct,
			status))
	}

	return sb.String()
}
