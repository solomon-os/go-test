package formatter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/solomon-os/go-test/internal/models"
)

func TestRegistry(t *testing.T) {
	t.Run("NewRegistry creates registry with built-in formatters", func(t *testing.T) {
		r := NewRegistry()

		if _, ok := r.Get("json"); !ok {
			t.Error("expected json formatter to be registered")
		}
		if _, ok := r.Get("table"); !ok {
			t.Error("expected table formatter to be registered")
		}
		if _, ok := r.Get("text"); !ok {
			t.Error("expected text formatter to be registered")
		}
	})

	t.Run("Register adds custom formatter", func(t *testing.T) {
		r := NewRegistry()

		r.Register(&CompactFormatter{})

		c, ok := r.Get("compact")
		if !ok {
			t.Error("expected compact formatter to be registered")
		}
		if c.Name() != "compact" {
			t.Errorf("expected name 'compact', got %s", c.Name())
		}
	})

	t.Run("List returns all formatter names", func(t *testing.T) {
		r := NewRegistry()

		names := r.List()
		if len(names) < 3 {
			t.Errorf("expected at least 3 formatters, got %d", len(names))
		}
	})
}

func TestJSONFormatter(t *testing.T) {
	f := &JSONFormatter{}

	t.Run("Name returns json", func(t *testing.T) {
		if f.Name() != "json" {
			t.Errorf("expected 'json', got %s", f.Name())
		}
	})

	t.Run("Description returns description", func(t *testing.T) {
		if f.Description() == "" {
			t.Error("expected non-empty description")
		}
	})

	t.Run("Format produces valid JSON", func(t *testing.T) {
		report := &models.DriftReport{
			TotalInstances:   2,
			DriftedInstances: 1,
			Results: []models.DriftResult{
				{InstanceID: "i-123", HasDrift: true},
				{InstanceID: "i-456", HasDrift: false},
			},
		}

		var buf bytes.Buffer
		err := f.Format(&buf, report)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify output is valid JSON
		var decoded models.DriftReport
		if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
			t.Errorf("output is not valid JSON: %v", err)
		}

		if decoded.TotalInstances != 2 {
			t.Errorf("expected TotalInstances 2, got %d", decoded.TotalInstances)
		}
	})

	t.Run("Format respects custom indent", func(t *testing.T) {
		f := &JSONFormatter{Indent: "\t"}
		report := &models.DriftReport{TotalInstances: 1}

		var buf bytes.Buffer
		_ = f.Format(&buf, report)

		if !strings.Contains(buf.String(), "\t") {
			t.Error("expected output to contain tab indentation")
		}
	})
}

func TestTableFormatter(t *testing.T) {
	f := &TableFormatter{}

	t.Run("Name returns table", func(t *testing.T) {
		if f.Name() != "table" {
			t.Errorf("expected 'table', got %s", f.Name())
		}
	})

	t.Run("Format produces table output", func(t *testing.T) {
		report := &models.DriftReport{
			TotalInstances:   2,
			DriftedInstances: 1,
			Results: []models.DriftResult{
				{
					InstanceID: "i-123",
					HasDrift:   true,
					DriftedAttrs: []models.DriftedAttr{
						{Path: "instance_type"},
					},
				},
				{InstanceID: "i-456", HasDrift: false},
			},
		}

		var buf bytes.Buffer
		err := f.Format(&buf, report)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()

		// Check for header
		if !strings.Contains(output, "INSTANCE ID") {
			t.Error("expected table header")
		}

		// Check for instance IDs
		if !strings.Contains(output, "i-123") {
			t.Error("expected i-123 in output")
		}
		if !strings.Contains(output, "i-456") {
			t.Error("expected i-456 in output")
		}

		// Check for drift status
		if !strings.Contains(output, "Yes") {
			t.Error("expected 'Yes' for drifted instance")
		}
		if !strings.Contains(output, "No") {
			t.Error("expected 'No' for non-drifted instance")
		}

		// Check for summary
		if !strings.Contains(output, "1/2") {
			t.Error("expected summary with 1/2")
		}
	})

	t.Run("Format shows error for failed checks", func(t *testing.T) {
		report := &models.DriftReport{
			TotalInstances: 1,
			Results: []models.DriftResult{
				{InstanceID: "i-123", Error: "connection failed"},
			},
		}

		var buf bytes.Buffer
		_ = f.Format(&buf, report)

		if !strings.Contains(buf.String(), "ERROR: connection failed") {
			t.Error("expected error message in output")
		}
	})
}

func TestTextFormatter(t *testing.T) {
	f := &TextFormatter{}

	t.Run("Name returns text", func(t *testing.T) {
		if f.Name() != "text" {
			t.Errorf("expected 'text', got %s", f.Name())
		}
	})

	t.Run("Format produces text output", func(t *testing.T) {
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
				{InstanceID: "i-456", HasDrift: false},
			},
		}

		var buf bytes.Buffer
		err := f.Format(&buf, report)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()

		// Check for title
		if !strings.Contains(output, "EC2 Drift Detection Report") {
			t.Error("expected report title")
		}

		// Check for instance section
		if !strings.Contains(output, "Instance: i-123") {
			t.Error("expected instance i-123")
		}

		// Check for drift status
		if !strings.Contains(output, "DRIFT DETECTED") {
			t.Error("expected DRIFT DETECTED status")
		}
		if !strings.Contains(output, "No drift detected") {
			t.Error("expected 'No drift detected' status")
		}

		// Check for attribute details
		if !strings.Contains(output, "instance_type") {
			t.Error("expected attribute path")
		}
		if !strings.Contains(output, "t2.large") {
			t.Error("expected AWS value")
		}
		if !strings.Contains(output, "t2.micro") {
			t.Error("expected Terraform value")
		}

		// Check for summary
		if !strings.Contains(output, "Total instances checked: 2") {
			t.Error("expected total instances in summary")
		}
		if !strings.Contains(output, "Instances with drift:    1") {
			t.Error("expected drifted instances in summary")
		}
	})

	t.Run("Format shows error for failed checks", func(t *testing.T) {
		report := &models.DriftReport{
			TotalInstances: 1,
			Results: []models.DriftResult{
				{InstanceID: "i-123", Error: "instance not found"},
			},
		}

		var buf bytes.Buffer
		_ = f.Format(&buf, report)

		if !strings.Contains(buf.String(), "Error: instance not found") {
			t.Error("expected error message in output")
		}
	})
}

func TestCompactFormatter(t *testing.T) {
	f := &CompactFormatter{}

	t.Run("Name returns compact", func(t *testing.T) {
		if f.Name() != "compact" {
			t.Errorf("expected 'compact', got %s", f.Name())
		}
	})

	t.Run("Format shows OK for no drift", func(t *testing.T) {
		report := &models.DriftReport{
			TotalInstances:   5,
			DriftedInstances: 0,
		}

		var buf bytes.Buffer
		err := f.Format(&buf, report)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(buf.String(), "OK:") {
			t.Error("expected OK status")
		}
		if !strings.Contains(buf.String(), "5 instances") {
			t.Error("expected instance count")
		}
	})

	t.Run("Format shows DRIFT for drift detected", func(t *testing.T) {
		report := &models.DriftReport{
			TotalInstances:   5,
			DriftedInstances: 2,
		}

		var buf bytes.Buffer
		_ = f.Format(&buf, report)

		if !strings.Contains(buf.String(), "DRIFT:") {
			t.Error("expected DRIFT status")
		}
		if !strings.Contains(buf.String(), "2/5") {
			t.Error("expected 2/5 in output")
		}
	})
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"empty string", "", "(empty)"},
		{"non-empty string", "hello", "hello"},
		{"empty slice", []string{}, "[]"},
		{"string slice", []string{"a", "b"}, "[a, b]"},
		{"empty map", map[string]string{}, "{}"},
		{"string map", map[string]string{"key": "val"}, "{key=val}"},
		{"int", 42, "42"},
		{"bool", true, "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatValue(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
