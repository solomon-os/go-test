package reporter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/solomon-os/go-test/internal/models"
)

func TestNew(t *testing.T) {
	buf := &bytes.Buffer{}
	r := New(buf, FormatJSON)

	if r == nil {
		t.Fatal("New() returned nil")
	}
	if r.format != FormatJSON {
		t.Fatalf("format = %s, want %s", r.format, FormatJSON)
	}
}

func TestReporter_Report_JSON(t *testing.T) {
	buf := &bytes.Buffer{}
	r := New(buf, FormatJSON)

	report := &models.DriftReport{
		TotalInstances:   2,
		DriftedInstances: 1,
		Results: []models.DriftResult{
			{
				InstanceID: "i-123",
				HasDrift:   true,
				DriftedAttrs: []models.DriftedAttr{
					{
						Path:           "instance_type",
						AWSValue:       "t2.large",
						TerraformValue: "t2.micro",
					},
				},
			},
			{
				InstanceID: "i-456",
				HasDrift:   false,
			},
		},
	}

	err := r.Report(report)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	// Verify JSON output
	var output models.DriftReport
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if output.TotalInstances != 2 {
		t.Errorf("TotalInstances = %d, want 2", output.TotalInstances)
	}
	if output.DriftedInstances != 1 {
		t.Errorf("DriftedInstances = %d, want 1", output.DriftedInstances)
	}
	if len(output.Results) != 2 {
		t.Errorf("Results count = %d, want 2", len(output.Results))
	}
}

func TestReporter_Report_Table(t *testing.T) {
	buf := &bytes.Buffer{}
	r := New(buf, FormatTable)

	report := &models.DriftReport{
		TotalInstances:   2,
		DriftedInstances: 1,
		Results: []models.DriftResult{
			{
				InstanceID: "i-123",
				HasDrift:   true,
				DriftedAttrs: []models.DriftedAttr{
					{Path: "instance_type"},
					{Path: "ami"},
				},
			},
			{
				InstanceID: "i-456",
				HasDrift:   false,
			},
		},
	}

	err := r.Report(report)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := buf.String()

	// Check for table headers
	if !strings.Contains(output, "INSTANCE ID") {
		t.Error("Table output missing INSTANCE ID header")
	}
	if !strings.Contains(output, "DRIFT DETECTED") {
		t.Error("Table output missing DRIFT DETECTED header")
	}

	// Check for instance data
	if !strings.Contains(output, "i-123") {
		t.Error("Table output missing instance i-123")
	}
	if !strings.Contains(output, "Yes") {
		t.Error("Table output missing 'Yes' for drift")
	}
	if !strings.Contains(output, "No") {
		t.Error("Table output missing 'No' for no drift")
	}

	// Check for summary
	if !strings.Contains(output, "1/2 instances with drift") {
		t.Error("Table output missing summary")
	}
}

func TestReporter_Report_Text(t *testing.T) {
	buf := &bytes.Buffer{}
	r := New(buf, FormatText)

	report := &models.DriftReport{
		TotalInstances:   2,
		DriftedInstances: 1,
		Results: []models.DriftResult{
			{
				InstanceID: "i-123",
				HasDrift:   true,
				DriftedAttrs: []models.DriftedAttr{
					{
						Path:           "instance_type",
						AWSValue:       "t2.large",
						TerraformValue: "t2.micro",
					},
				},
			},
			{
				InstanceID: "i-456",
				HasDrift:   false,
			},
		},
	}

	err := r.Report(report)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := buf.String()

	// Check for report header
	if !strings.Contains(output, "EC2 Drift Detection Report") {
		t.Error("Text output missing report header")
	}

	// Check for drift details
	if !strings.Contains(output, "DRIFT DETECTED") {
		t.Error("Text output missing DRIFT DETECTED status")
	}
	if !strings.Contains(output, "instance_type") {
		t.Error("Text output missing drifted attribute name")
	}
	if !strings.Contains(output, "t2.large") {
		t.Error("Text output missing AWS value")
	}
	if !strings.Contains(output, "t2.micro") {
		t.Error("Text output missing Terraform value")
	}

	// Check for no drift message
	if !strings.Contains(output, "No drift detected") {
		t.Error("Text output missing 'No drift detected' status")
	}

	// Check for summary
	if !strings.Contains(output, "Total instances checked: 2") {
		t.Error("Text output missing total instances count")
	}
}

func TestReporter_Report_WithError(t *testing.T) {
	buf := &bytes.Buffer{}
	r := New(buf, FormatText)

	report := &models.DriftReport{
		TotalInstances:   1,
		DriftedInstances: 0,
		Results: []models.DriftResult{
			{
				InstanceID: "i-123",
				HasDrift:   false,
				Error:      "instance not found in AWS",
			},
		},
	}

	err := r.Report(report)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Error: instance not found in AWS") {
		t.Error("Text output missing error message")
	}
}

func TestReporter_Report_TableWithError(t *testing.T) {
	buf := &bytes.Buffer{}
	r := New(buf, FormatTable)

	report := &models.DriftReport{
		TotalInstances:   1,
		DriftedInstances: 1,
		Results: []models.DriftResult{
			{
				InstanceID: "i-123",
				HasDrift:   true,
				Error:      "connection timeout",
			},
		},
	}

	err := r.Report(report)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ERROR: connection timeout") {
		t.Error("Table output missing error message")
	}
}

func TestReporter_ReportSingle(t *testing.T) {
	buf := &bytes.Buffer{}
	r := New(buf, FormatJSON)

	result := &models.DriftResult{
		InstanceID: "i-single",
		HasDrift:   true,
		DriftedAttrs: []models.DriftedAttr{
			{Path: "ami"},
		},
	}

	err := r.ReportSingle(result)
	if err != nil {
		t.Fatalf("ReportSingle() error = %v", err)
	}

	var output models.DriftReport
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if output.TotalInstances != 1 {
		t.Errorf("TotalInstances = %d, want 1", output.TotalInstances)
	}
	if output.DriftedInstances != 1 {
		t.Errorf("DriftedInstances = %d, want 1", output.DriftedInstances)
	}
}

func TestReporter_ReportSingle_NoDrift(t *testing.T) {
	buf := &bytes.Buffer{}
	r := New(buf, FormatJSON)

	result := &models.DriftResult{
		InstanceID: "i-single",
		HasDrift:   false,
	}

	err := r.ReportSingle(result)
	if err != nil {
		t.Fatalf("ReportSingle() error = %v", err)
	}

	var output models.DriftReport
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON output: %v", err)
	}

	if output.DriftedInstances != 0 {
		t.Errorf("DriftedInstances = %d, want 0", output.DriftedInstances)
	}
}

func TestReporter_DefaultFormat(t *testing.T) {
	buf := &bytes.Buffer{}
	r := New(buf, Format("unknown"))

	report := &models.DriftReport{
		TotalInstances: 1,
		Results: []models.DriftResult{
			{InstanceID: "i-123", HasDrift: false},
		},
	}

	err := r.Report(report)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}

	// Should default to text format
	if !strings.Contains(buf.String(), "EC2 Drift Detection Report") {
		t.Error("Expected text format for unknown format type")
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"empty string", "", "(empty)"},
		{"non-empty string", "test", "test"},
		{"empty slice", []string{}, "[]"},
		{"string slice", []string{"a", "b"}, "[a, b]"},
		{"empty map", map[string]string{}, "{}"},
		{"string map", map[string]string{"k": "v"}, "{k=v}"},
		{"integer", 42, "42"},
		{"boolean true", true, "true"},
		{"boolean false", false, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatValue(tt.value)
			if got != tt.want {
				t.Errorf("formatValue() = %s, want %s", got, tt.want)
			}
		})
	}
}
