// Package reporter provides functionality to output drift detection results.
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/solomon-os/go-test/internal/models"
)

// Format represents the output format for reports.
type Format string

const (
	FormatJSON  Format = "json"
	FormatTable Format = "table"
	FormatText  Format = "text"
)

// DriftReporter defines the interface for outputting drift reports.
type DriftReporter interface {
	Report(report *models.DriftReport) error
	ReportSingle(result *models.DriftResult) error
}

// Reporter outputs drift detection results in various formats.
type Reporter struct {
	writer io.Writer
	format Format
}

func New(w io.Writer, format Format) *Reporter {
	return &Reporter{
		writer: w,
		format: format,
	}
}

func (r *Reporter) Report(report *models.DriftReport) error {
	switch r.format {
	case FormatJSON:
		return r.reportJSON(report)
	case FormatTable:
		return r.reportTable(report)
	case FormatText:
		return r.reportText(report)
	default:
		return r.reportText(report)
	}
}

func (r *Reporter) ReportSingle(result *models.DriftResult) error {
	report := &models.DriftReport{
		TotalInstances:   1,
		DriftedInstances: 0,
		Results:          []models.DriftResult{*result},
	}
	if result.HasDrift {
		report.DriftedInstances = 1
	}
	return r.Report(report)
}

func (r *Reporter) reportJSON(report *models.DriftReport) error {
	encoder := json.NewEncoder(r.writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func (r *Reporter) reportTable(report *models.DriftReport) error {
	w := tabwriter.NewWriter(r.writer, 0, 0, 2, ' ', 0)

	_, _ = fmt.Fprintf(w, "INSTANCE ID\tDRIFT DETECTED\tDRIFTED ATTRIBUTES\n")
	_, _ = fmt.Fprintf(w, "-----------\t--------------\t------------------\n")

	for _, result := range report.Results {
		driftStatus := "No"
		if result.HasDrift {
			driftStatus = "Yes"
		}

		attrs := "-"
		if len(result.DriftedAttrs) > 0 {
			attrNames := make([]string, len(result.DriftedAttrs))
			for i, a := range result.DriftedAttrs {
				attrNames[i] = a.Path
			}
			attrs = strings.Join(attrNames, ", ")
		}

		if result.Error != "" {
			attrs = fmt.Sprintf("ERROR: %s", result.Error)
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", result.InstanceID, driftStatus, attrs)
	}

	_, _ = fmt.Fprintf(w, "\n")
	_, _ = fmt.Fprintf(w, "Summary: %d/%d instances with drift\n", report.DriftedInstances, report.TotalInstances)

	return w.Flush()
}

func (r *Reporter) reportText(report *models.DriftReport) error {
	_, _ = fmt.Fprintf(r.writer, "EC2 Drift Detection Report\n")
	_, _ = fmt.Fprintf(r.writer, "==========================\n\n")

	for _, result := range report.Results {
		_, _ = fmt.Fprintf(r.writer, "Instance: %s\n", result.InstanceID)

		if result.Error != "" {
			_, _ = fmt.Fprintf(r.writer, "  Error: %s\n\n", result.Error)
			continue
		}

		if !result.HasDrift {
			_, _ = fmt.Fprintf(r.writer, "  Status: No drift detected\n\n")
			continue
		}

		_, _ = fmt.Fprintf(r.writer, "  Status: DRIFT DETECTED\n")
		_, _ = fmt.Fprintf(r.writer, "  Drifted Attributes:\n")

		for _, attr := range result.DriftedAttrs {
			_, _ = fmt.Fprintf(r.writer, "    - %s:\n", attr.Path)
			_, _ = fmt.Fprintf(r.writer, "        AWS:       %v\n", formatValue(attr.AWSValue))
			_, _ = fmt.Fprintf(r.writer, "        Terraform: %v\n", formatValue(attr.TerraformValue))
		}
		_, _ = fmt.Fprintf(r.writer, "\n")
	}

	_, _ = fmt.Fprintf(r.writer, "Summary\n")
	_, _ = fmt.Fprintf(r.writer, "-------\n")
	_, _ = fmt.Fprintf(r.writer, "Total instances checked: %d\n", report.TotalInstances)
	_, _ = fmt.Fprintf(r.writer, "Instances with drift:    %d\n", report.DriftedInstances)
	_, _ = fmt.Fprintf(r.writer, "Instances without drift: %d\n", report.TotalInstances-report.DriftedInstances)

	return nil
}

func formatValue(v any) string {
	switch val := v.(type) {
	case []string:
		if len(val) == 0 {
			return "[]"
		}
		return fmt.Sprintf("[%s]", strings.Join(val, ", "))
	case map[string]string:
		if len(val) == 0 {
			return "{}"
		}
		pairs := make([]string, 0, len(val))
		for k, v := range val {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
		}
		return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
	case string:
		if val == "" {
			return "(empty)"
		}
		return val
	default:
		return fmt.Sprintf("%v", v)
	}
}
