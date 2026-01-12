package comparator

import (
	"testing"
)

func TestRegistry(t *testing.T) {
	t.Run("NewRegistry creates registry with built-in comparators", func(t *testing.T) {
		r := NewRegistry()

		// Check built-in comparators are registered
		if _, ok := r.Get("string"); !ok {
			t.Error("expected string comparator to be registered")
		}
		if _, ok := r.Get("slice"); !ok {
			t.Error("expected slice comparator to be registered")
		}
		if _, ok := r.Get("map"); !ok {
			t.Error("expected map comparator to be registered")
		}
		if _, ok := r.Get("deep"); !ok {
			t.Error("expected deep comparator to be registered")
		}
	})

	t.Run("Register adds custom comparator", func(t *testing.T) {
		r := NewRegistry()

		custom := &customComparator{name: "custom"}
		r.Register(custom)

		c, ok := r.Get("custom")
		if !ok {
			t.Error("expected custom comparator to be registered")
		}
		if c.Name() != "custom" {
			t.Errorf("expected name 'custom', got %s", c.Name())
		}
	})

	t.Run("RegisterForType associates type with comparator", func(t *testing.T) {
		r := NewRegistry()

		r.RegisterForType("int", "deep")

		// Compare should use deep comparator for int
		if !r.Compare(42, 42) {
			t.Error("expected equal ints to match")
		}
	})

	t.Run("Compare uses type-specific comparator", func(t *testing.T) {
		r := NewRegistry()

		// String comparison
		if !r.Compare("hello", "hello") {
			t.Error("expected equal strings to match")
		}
		if r.Compare("hello", "world") {
			t.Error("expected different strings to not match")
		}

		// Slice comparison (order-independent)
		if !r.Compare([]string{"a", "b"}, []string{"b", "a"}) {
			t.Error("expected equal slices to match (order-independent)")
		}

		// Map comparison
		if !r.Compare(map[string]string{"a": "1"}, map[string]string{"a": "1"}) {
			t.Error("expected equal maps to match")
		}
	})

	t.Run("Compare handles nil values", func(t *testing.T) {
		r := NewRegistry()

		if !r.Compare(nil, nil) {
			t.Error("expected nil == nil")
		}
		if r.Compare(nil, "value") {
			t.Error("expected nil != value")
		}
		if r.Compare("value", nil) {
			t.Error("expected value != nil")
		}
	})

	t.Run("SetDefault changes default comparator", func(t *testing.T) {
		r := NewRegistry()

		custom := &alwaysTrueComparator{}
		r.SetDefault(custom)

		// Unknown type should use custom default
		if !r.Compare(struct{}{}, struct{}{}) {
			t.Error("expected custom default to return true")
		}
	})
}

func TestStringComparator(t *testing.T) {
	c := &StringComparator{}

	t.Run("Name returns string", func(t *testing.T) {
		if c.Name() != "string" {
			t.Errorf("expected 'string', got %s", c.Name())
		}
	})

	t.Run("Compare equal strings", func(t *testing.T) {
		if !c.Compare("hello", "hello") {
			t.Error("expected equal strings to match")
		}
	})

	t.Run("Compare different strings", func(t *testing.T) {
		if c.Compare("hello", "world") {
			t.Error("expected different strings to not match")
		}
	})

	t.Run("Compare non-string types", func(t *testing.T) {
		if c.Compare("hello", 42) {
			t.Error("expected string and int to not match")
		}
		if c.Compare(42, "hello") {
			t.Error("expected int and string to not match")
		}
	})
}

func TestSliceComparator(t *testing.T) {
	t.Run("Compare equal slices with order independence", func(t *testing.T) {
		c := &SliceComparator{IgnoreOrder: true}

		if !c.Compare([]string{"a", "b", "c"}, []string{"c", "b", "a"}) {
			t.Error("expected equal slices to match (order-independent)")
		}
	})

	t.Run("Compare equal slices with order dependence", func(t *testing.T) {
		c := &SliceComparator{IgnoreOrder: false}

		if !c.Compare([]string{"a", "b", "c"}, []string{"a", "b", "c"}) {
			t.Error("expected equal slices to match")
		}
		if c.Compare([]string{"a", "b", "c"}, []string{"c", "b", "a"}) {
			t.Error("expected different order to not match when IgnoreOrder is false")
		}
	})

	t.Run("Compare different length slices", func(t *testing.T) {
		c := &SliceComparator{IgnoreOrder: true}

		if c.Compare([]string{"a", "b"}, []string{"a", "b", "c"}) {
			t.Error("expected different length slices to not match")
		}
	})

	t.Run("Compare non-string slices falls back to DeepEqual", func(t *testing.T) {
		c := &SliceComparator{IgnoreOrder: true}

		if !c.Compare([]int{1, 2, 3}, []int{1, 2, 3}) {
			t.Error("expected equal int slices to match")
		}
	})

	t.Run("Name returns slice", func(t *testing.T) {
		c := &SliceComparator{}
		if c.Name() != "slice" {
			t.Errorf("expected 'slice', got %s", c.Name())
		}
	})
}

func TestMapComparator(t *testing.T) {
	c := &MapComparator{}

	t.Run("Name returns map", func(t *testing.T) {
		if c.Name() != "map" {
			t.Errorf("expected 'map', got %s", c.Name())
		}
	})

	t.Run("Compare equal maps", func(t *testing.T) {
		a := map[string]string{"key1": "val1", "key2": "val2"}
		b := map[string]string{"key2": "val2", "key1": "val1"}

		if !c.Compare(a, b) {
			t.Error("expected equal maps to match")
		}
	})

	t.Run("Compare maps with different values", func(t *testing.T) {
		a := map[string]string{"key": "val1"}
		b := map[string]string{"key": "val2"}

		if c.Compare(a, b) {
			t.Error("expected different values to not match")
		}
	})

	t.Run("Compare maps with different keys", func(t *testing.T) {
		a := map[string]string{"key1": "val"}
		b := map[string]string{"key2": "val"}

		if c.Compare(a, b) {
			t.Error("expected different keys to not match")
		}
	})

	t.Run("Compare maps with different lengths", func(t *testing.T) {
		a := map[string]string{"key1": "val1"}
		b := map[string]string{"key1": "val1", "key2": "val2"}

		if c.Compare(a, b) {
			t.Error("expected different length maps to not match")
		}
	})

	t.Run("Compare non-string maps falls back to DeepEqual", func(t *testing.T) {
		a := map[string]int{"key": 1}
		b := map[string]int{"key": 1}

		if !c.Compare(a, b) {
			t.Error("expected equal int maps to match")
		}
	})
}

func TestDeepEqualComparator(t *testing.T) {
	c := &DeepEqualComparator{}

	t.Run("Name returns deep", func(t *testing.T) {
		if c.Name() != "deep" {
			t.Errorf("expected 'deep', got %s", c.Name())
		}
	})

	t.Run("Compare equal structs", func(t *testing.T) {
		type testStruct struct {
			Name  string
			Value int
		}
		a := testStruct{Name: "test", Value: 42}
		b := testStruct{Name: "test", Value: 42}

		if !c.Compare(a, b) {
			t.Error("expected equal structs to match")
		}
	})

	t.Run("Compare different structs", func(t *testing.T) {
		type testStruct struct {
			Name  string
			Value int
		}
		a := testStruct{Name: "test", Value: 42}
		b := testStruct{Name: "test", Value: 43}

		if c.Compare(a, b) {
			t.Error("expected different structs to not match")
		}
	})

	t.Run("Compare primitives", func(t *testing.T) {
		if !c.Compare(42, 42) {
			t.Error("expected equal ints to match")
		}
		if c.Compare(42, 43) {
			t.Error("expected different ints to not match")
		}
		if !c.Compare(true, true) {
			t.Error("expected equal bools to match")
		}
	})
}

func TestTagComparator(t *testing.T) {
	t.Run("Compare tags ignoring specified keys", func(t *testing.T) {
		c := &TagComparator{
			IgnoreKeys: []string{"aws:autoscaling:groupName", "timestamp"},
		}

		a := map[string]string{
			"Name":                      "my-instance",
			"Environment":               "production",
			"aws:autoscaling:groupName": "group-1",
			"timestamp":                 "2024-01-01",
		}
		b := map[string]string{
			"Name":                      "my-instance",
			"Environment":               "production",
			"aws:autoscaling:groupName": "group-2", // Different but ignored
			"timestamp":                 "2024-02-01", // Different but ignored
		}

		if !c.Compare(a, b) {
			t.Error("expected tags to match when ignoring specified keys")
		}
	})

	t.Run("Compare tags with non-ignored difference", func(t *testing.T) {
		c := &TagComparator{
			IgnoreKeys: []string{"timestamp"},
		}

		a := map[string]string{"Name": "instance-1", "timestamp": "2024-01-01"}
		b := map[string]string{"Name": "instance-2", "timestamp": "2024-02-01"}

		if c.Compare(a, b) {
			t.Error("expected tags to not match when Name differs")
		}
	})

	t.Run("Name returns tags", func(t *testing.T) {
		c := &TagComparator{}
		if c.Name() != "tags" {
			t.Errorf("expected 'tags', got %s", c.Name())
		}
	})

	t.Run("Compare non-map types falls back to DeepEqual", func(t *testing.T) {
		c := &TagComparator{}

		if !c.Compare("hello", "hello") {
			t.Error("expected equal strings to match")
		}
	})
}

// Helper test types

type customComparator struct {
	name string
}

func (c *customComparator) Name() string { return c.name }
func (c *customComparator) Compare(a, b any) bool {
	return a == b
}

type alwaysTrueComparator struct{}

func (c *alwaysTrueComparator) Name() string         { return "always-true" }
func (c *alwaysTrueComparator) Compare(a, b any) bool { return true }
