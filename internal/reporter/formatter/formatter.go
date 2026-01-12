// Package formatter provides extensible report output formats.
//
// This package implements the Open/Closed Principle by allowing new output
// formats to be added without modifying existing code. Formatters can be
// registered with the registry and selected by name.
//
// Example usage:
//
//	registry := formatter.NewRegistry()
//	registry.Register(&YAMLFormatter{})
//
//	f, _ := registry.Get("yaml")
//	f.Format(os.Stdout, report)
package formatter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/solomon-os/go-test/internal/models"
)

// Formatter defines the interface for report output formatting.
type Formatter interface {
	// Format writes the report to the given writer.
	Format(w io.Writer, report *models.DriftReport) error

	// Name returns the formatter's name for identification.
	Name() string

	// Description returns a human-readable description of the format.
	Description() string
}

// Registry holds registered formatters and provides lookup operations.
// It is safe for concurrent use.
type Registry struct {
	mu         sync.RWMutex
	formatters map[string]Formatter
}

// NewRegistry creates a new formatter registry with built-in formatters.
func NewRegistry() *Registry {
	r := &Registry{
		formatters: make(map[string]Formatter),
	}

	// Register built-in formatters
	r.Register(&JSONFormatter{})
	r.Register(&TableFormatter{})
	r.Register(&TextFormatter{})

	return r
}

// Register adds a formatter to the registry.
func (r *Registry) Register(f Formatter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.formatters[f.Name()] = f
}

// Get retrieves a formatter by name.
func (r *Registry) Get(name string) (Formatter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.formatters[name]
	return f, ok
}

// List returns all registered formatter names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.formatters))
	for name := range r.formatters {
		names = append(names, name)
	}
	return names
}

// --- Built-in Formatters ---

// JSONFormatter outputs reports in JSON format.
type JSONFormatter struct {
	// Indent specifies the indentation string. Empty means no indentation.
	Indent string
}

func (f *JSONFormatter) Name() string        { return "json" }
func (f *JSONFormatter) Description() string { return "JSON output format" }

func (f *JSONFormatter) Format(w io.Writer, report *models.DriftReport) error {
	encoder := json.NewEncoder(w)
	if f.Indent != "" {
		encoder.SetIndent("", f.Indent)
	} else {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(report)
}

// TableFormatter outputs reports in a tabular format.
type TableFormatter struct{}

func (f *TableFormatter) Name() string        { return "table" }
func (f *TableFormatter) Description() string { return "Tabular output format" }

func (f *TableFormatter) Format(w io.Writer, report *models.DriftReport) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	writef(tw, "INSTANCE ID\tDRIFT DETECTED\tDRIFTED ATTRIBUTES\n")
	writef(tw, "-----------\t--------------\t------------------\n")

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

		writef(tw, "%s\t%s\t%s\n", result.InstanceID, driftStatus, attrs)
	}

	writef(tw, "\n")
	writef(tw, "Summary: %d/%d instances with drift\n",
		report.DriftedInstances, report.TotalInstances)

	return tw.Flush()
}

// TextFormatter outputs reports in a human-readable text format.
type TextFormatter struct{}

func (f *TextFormatter) Name() string        { return "text" }
func (f *TextFormatter) Description() string { return "Human-readable text output" }

func (f *TextFormatter) Format(w io.Writer, report *models.DriftReport) error {
	writef(w, "EC2 Drift Detection Report\n")
	writef(w, "==========================\n\n")

	for _, result := range report.Results {
		writef(w, "Instance: %s\n", result.InstanceID)

		if result.Error != "" {
			writef(w, "  Error: %s\n\n", result.Error)
			continue
		}

		if !result.HasDrift {
			writef(w, "  Status: No drift detected\n\n")
			continue
		}

		writef(w, "  Status: DRIFT DETECTED\n")
		writef(w, "  Drifted Attributes:\n")

		for _, attr := range result.DriftedAttrs {
			writef(w, "    - %s:\n", attr.Path)
			writef(w, "        AWS:       %v\n", formatValue(attr.AWSValue))
			writef(w, "        Terraform: %v\n", formatValue(attr.TerraformValue))
		}
		writef(w, "\n")
	}

	writef(w, "Summary\n")
	writef(w, "-------\n")
	writef(w, "Total instances checked: %d\n", report.TotalInstances)
	writef(w, "Instances with drift:    %d\n", report.DriftedInstances)
	writef(w, "Instances without drift: %d\n",
		report.TotalInstances-report.DriftedInstances)

	return nil
}

// CompactFormatter outputs a compact single-line summary.
type CompactFormatter struct{}

func (f *CompactFormatter) Name() string        { return "compact" }
func (f *CompactFormatter) Description() string { return "Compact single-line summary" }

func (f *CompactFormatter) Format(w io.Writer, report *models.DriftReport) error {
	if report.DriftedInstances == 0 {
		writef(w, "OK: No drift detected in %d instances\n", report.TotalInstances)
	} else {
		writef(w, "DRIFT: %d/%d instances have drift\n",
			report.DriftedInstances, report.TotalInstances)
	}
	return nil
}

// Helper functions

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
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

// Verify interface compliance at compile time.
var (
	_ Formatter = (*JSONFormatter)(nil)
	_ Formatter = (*TableFormatter)(nil)
	_ Formatter = (*TextFormatter)(nil)
	_ Formatter = (*CompactFormatter)(nil)
)
