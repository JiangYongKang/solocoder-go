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
		sb.WriteString(fmt.Sprintf("  Mean Allocs:    %d bytes/op\n", s.MeanAllocBytes))
		sb.WriteString(fmt.Sprintf("  Mean Alloc Cnt: %d allocs/op\n", s.MeanAllocCount))
		sb.WriteString(fmt.Sprintf("  Allocs/Op:      %.2f\n", s.AllocsPerOp))
		sb.WriteString(fmt.Sprintf("  Bytes/Op:       %.2f\n\n", s.BytesPerOp))
	}

	return sb.String()
}

func (r *TextReporter) ReportComparison(report ComparisonReport) string {
	var sb strings.Builder

	sb.WriteString("=== Comparison Report ===\n")
	sb.WriteString(fmt.Sprintf("Baseline: %s\n", report.Baseline))
	sb.WriteString(fmt.Sprintf("Generated at: %s\n\n", report.GeneratedAt.Format(time.RFC3339)))

	sb.WriteString(fmt.Sprintf("%-20s %-20s %-15s %-15s %s\n",
		"Group", "Mean Duration", "Alloc Bytes", "Alloc Count", "vs Baseline"))
	sb.WriteString(strings.Repeat("-", 80) + "\n")

	for _, item := range report.Items {
		var marker string
		if item.VsBaselinePct > 0 {
			marker = " slower"
		} else if item.VsBaselinePct < 0 {
			marker = " faster"
		}
		sb.WriteString(fmt.Sprintf("%-20s %-20v %-15d %-15d %+.2f%%%s\n",
			item.Group,
			item.MeanDuration,
			item.MeanAllocBytes,
			item.MeanAllocCount,
			item.VsBaselinePct,
			marker))
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
