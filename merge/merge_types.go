package merge

import (
	"reflect"
)

// Match pairs up to FHIR resources that "match". These resources should be of the same
// resource type (e.g. Patient).
type Match struct {
	ResourceType string
	Left         interface{}
	Right        interface{}
}

// ResourceMap is used to map a list of resources to their specific type.
type ResourceMap map[string][]interface{}

func (r ResourceMap) Keys() []string {
	keys := make([]string, len(r))
	i := 0
	for k := range r {
		keys[i] = k
		i++
	}
	return keys
}

// PathMap captures all non-nil paths in a resource and the values at those paths.
type PathMap map[string]reflect.Value

func (p PathMap) Keys() []string {
	keys := make([]string, len(p))
	i := 0
	for k := range p {
		keys[i] = k
		i++
	}
	return keys
}

// intersection returns all elements in left that are also in right.
func intersection(left, right []string) []string {
	both := []string{}
	for _, el := range left {
		if contains(right, el) {
			both = append(both, el)
		}
	}
	return both
}

// setDiff returns all elements in left that are NOT in right.
func setDiff(left, right []string) []string {
	diffs := []string{}
	for _, el := range left {
		if !contains(right, el) {
			diffs = append(diffs, el)
		}
	}
	return diffs
}

// contains tests if an element is in the set.
func contains(set []string, el string) bool {
	for _, item := range set {
		if item == el {
			return true
		}
	}
	return false
}
