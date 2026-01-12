// Package comparator provides pluggable value comparison strategies for drift detection.
//
// This package implements the Open/Closed Principle by allowing new comparison
// strategies to be added without modifying existing code. Comparators can be
// registered for specific types and used by the drift detector.
//
// Example usage:
//
//	registry := comparator.NewRegistry()
//	registry.Register("custom", &MyCustomComparator{})
//	equal := registry.Compare(value1, value2)
package comparator

import (
	"reflect"
	"sort"
	"sync"
)

// Comparator defines the interface for attribute comparison.
// Implementations provide specific comparison logic for different value types.
type Comparator interface {
	// Compare returns true if the two values are considered equal.
	Compare(a, b any) bool

	// Name returns the comparator's name for identification.
	Name() string
}

// Registry holds registered comparators and provides comparison operations.
// It is safe for concurrent use.
type Registry struct {
	mu          sync.RWMutex
	comparators map[string]Comparator
	typeMap     map[string]string // maps Go type name to comparator name
	defaultComp Comparator
}

// NewRegistry creates a new comparator registry with built-in comparators.
func NewRegistry() *Registry {
	r := &Registry{
		comparators: make(map[string]Comparator),
		typeMap:     make(map[string]string),
		defaultComp: &DeepEqualComparator{},
	}

	// Register built-in comparators
	r.Register(&StringComparator{})
	r.Register(&SliceComparator{IgnoreOrder: true})
	r.Register(&MapComparator{})
	r.Register(&DeepEqualComparator{})

	// Map Go types to comparators
	r.typeMap["string"] = "string"
	r.typeMap["[]string"] = "slice"
	r.typeMap["map[string]string"] = "map"

	return r
}

// Register adds a comparator to the registry.
func (r *Registry) Register(c Comparator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.comparators[c.Name()] = c
}

// RegisterForType associates a comparator with a specific Go type.
func (r *Registry) RegisterForType(typeName, comparatorName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.typeMap[typeName] = comparatorName
}

// Get retrieves a comparator by name.
func (r *Registry) Get(name string) (Comparator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.comparators[name]
	return c, ok
}

// Compare compares two values using the appropriate comparator.
// It selects the comparator based on the type of the first value.
func (r *Registry) Compare(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	r.mu.RLock()
	typeName := reflect.TypeOf(a).String()
	comparatorName, ok := r.typeMap[typeName]
	var comp Comparator
	if ok {
		comp = r.comparators[comparatorName]
	}
	if comp == nil {
		comp = r.defaultComp
	}
	r.mu.RUnlock()

	return comp.Compare(a, b)
}

// SetDefault sets the default comparator for unregistered types.
func (r *Registry) SetDefault(c Comparator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultComp = c
}

// --- Built-in Comparators ---

// StringComparator compares string values.
type StringComparator struct{}

func (c *StringComparator) Name() string { return "string" }

func (c *StringComparator) Compare(a, b any) bool {
	aStr, aOK := a.(string)
	bStr, bOK := b.(string)
	if !aOK || !bOK {
		return false
	}
	return aStr == bStr
}

// SliceComparator compares slices with optional order independence.
type SliceComparator struct {
	// IgnoreOrder determines if slice order should be ignored during comparison.
	IgnoreOrder bool
}

func (c *SliceComparator) Name() string { return "slice" }

func (c *SliceComparator) Compare(a, b any) bool {
	aSlice, aOK := a.([]string)
	bSlice, bOK := b.([]string)
	if !aOK || !bOK {
		// Fall back to deep equal for non-string slices
		return reflect.DeepEqual(a, b)
	}

	if len(aSlice) != len(bSlice) {
		return false
	}

	if c.IgnoreOrder {
		aSorted := make([]string, len(aSlice))
		bSorted := make([]string, len(bSlice))
		copy(aSorted, aSlice)
		copy(bSorted, bSlice)
		sort.Strings(aSorted)
		sort.Strings(bSorted)

		for i := range aSorted {
			if aSorted[i] != bSorted[i] {
				return false
			}
		}
		return true
	}

	for i := range aSlice {
		if aSlice[i] != bSlice[i] {
			return false
		}
	}
	return true
}

// MapComparator compares maps.
type MapComparator struct{}

func (c *MapComparator) Name() string { return "map" }

func (c *MapComparator) Compare(a, b any) bool {
	aMap, aOK := a.(map[string]string)
	bMap, bOK := b.(map[string]string)
	if !aOK || !bOK {
		// Fall back to deep equal for non-string maps
		return reflect.DeepEqual(a, b)
	}

	if len(aMap) != len(bMap) {
		return false
	}

	for k, v := range aMap {
		if bMap[k] != v {
			return false
		}
	}
	return true
}

// DeepEqualComparator uses reflect.DeepEqual for comparison.
type DeepEqualComparator struct{}

func (c *DeepEqualComparator) Name() string { return "deep" }

func (c *DeepEqualComparator) Compare(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// TagComparator compares tags while ignoring specified keys.
// This is useful for ignoring AWS-managed tags.
type TagComparator struct {
	// IgnoreKeys is a list of tag keys to ignore during comparison.
	IgnoreKeys []string
}

func (c *TagComparator) Name() string { return "tags" }

func (c *TagComparator) Compare(a, b any) bool {
	aMap, aOK := a.(map[string]string)
	bMap, bOK := b.(map[string]string)
	if !aOK || !bOK {
		return reflect.DeepEqual(a, b)
	}

	// Create filtered copies
	aFiltered := c.filterTags(aMap)
	bFiltered := c.filterTags(bMap)

	if len(aFiltered) != len(bFiltered) {
		return false
	}

	for k, v := range aFiltered {
		if bFiltered[k] != v {
			return false
		}
	}
	return true
}

func (c *TagComparator) filterTags(tags map[string]string) map[string]string {
	result := make(map[string]string)
	ignoreSet := make(map[string]bool)
	for _, key := range c.IgnoreKeys {
		ignoreSet[key] = true
	}

	for k, v := range tags {
		if !ignoreSet[k] {
			result[k] = v
		}
	}
	return result
}

// Verify interface compliance at compile time.
var (
	_ Comparator = (*StringComparator)(nil)
	_ Comparator = (*SliceComparator)(nil)
	_ Comparator = (*MapComparator)(nil)
	_ Comparator = (*DeepEqualComparator)(nil)
	_ Comparator = (*TagComparator)(nil)
)
