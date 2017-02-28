package merge

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/fhirutil"
)

var (
	// PathsUnsuitableForComparison are paths that don't offer meaningful comparison
	// between two resources. For example, URLs or code systems.
	PathsUnsuitableForComparison = []string{"url", "system", "id", "_id", "text", "reference"}

	// FloatTolerance identifies the minumum difference between 2 floats that still
	// allows them to be considered a match.
	FloatTolerance = 0.0001

	// MatchThreshold is the total percentage of non-nil paths in a resource that must
	// match for the whole resource to be considered a match.
	MatchThreshold = 0.8
)

// Matcher provides tools for identifying all resources in 2 source bundles that "match".
type Matcher struct{}

// Match iterates through all resources in the two source bundles and attempts to find resources that "match". These
// resources can then be compared to each other to see what conflicts may still exist between them. All matches are
// returned as a slice of Match pairs. Resources without a match are returned separately as a slice of "unmatchables".
func (m *Matcher) Match(leftBundle, rightBundle *models.Bundle) (matches []Match, unmatchables []interface{}, err error) {

	// First collect all resources in the bundle that we'll attempt to match.
	leftResources, err := m.collectResources(leftBundle)
	if err != nil {
		return nil, nil, err
	}

	rightResources, err := m.collectResources(rightBundle)
	if err != nil {
		return nil, nil, err
	}

	// There are now three sets of resource types to consider:
	// 1. L intersect R - these are all the resource types we'll attempt to match
	// 2. L not in R - these are "unmatchable" left resource types
	// 3. R not in L - these are "unmatchable" right resource types
	matchableResourceTypes := intersection(leftResources.Keys(), rightResources.Keys())
	leftUnmatchableResourceTypes := setDiff(leftResources.Keys(), rightResources.Keys())  // L not in R
	rightUnmatchableResourceTypes := setDiff(rightResources.Keys(), leftResources.Keys()) // R not in L

	// If the Patient resource is not in matchableResourceTypes, return an error.
	// Minimally we need a single, matching Patient object to perform a merge.
	if !contains(matchableResourceTypes, "Patient") {
		return nil, nil, errors.New("Patient resource not found in one or both bundles")
	}

	// Handle all of the unmatchable resource types.
	for _, key := range leftUnmatchableResourceTypes {
		unmatchables = append(unmatchables, leftResources[key]...)
	}

	for _, key := range rightUnmatchableResourceTypes {
		unmatchables = append(unmatchables, rightResources[key]...)
	}

	// Handle all of the matchable resource types. Since the resourceType key existed
	// in both the leftResources and rightResources, there is guaranteed to be at least
	// one resource of each resourceType in both sets.
	for _, resourceType := range matchableResourceTypes {
		lefts := leftResources[resourceType]
		rights := rightResources[resourceType]

		// Performs matching without replacement, always comparing the next available left
		// to the remaining rights.
		someMatches, someUnmatchables, err := m.matchWithoutReplacement(lefts, rights)
		if err != nil {
			return nil, nil, err
		}

		if resourceType == "Patient" && len(someMatches) == 0 {
			// There was no matching Patient resource.
			return nil, nil, errors.New("Patient resource(s) do not match")
		}

		matches = append(matches, someMatches...)
		unmatchables = append(unmatchables, someUnmatchables...)
	}
	return matches, unmatchables, nil
}

// Collects all resources in a bundle into structs that match their resource types.
func (m *Matcher) collectResources(bundle *models.Bundle) (resources ResourceMap, err error) {
	resources = make(ResourceMap)
	for _, entry := range bundle.Entry {
		// Get the entry.Resource's type.
		resourceType := fhirutil.GetResourceType(entry.Resource)
		// Make sure it's a known FHIR type.
		s := models.StructForResourceName(resourceType)
		if s == nil {
			return nil, fmt.Errorf("Unknown resource type %s", resourceType)
		}

		resources[resourceType] = append(resources[resourceType], entry.Resource)
	}
	return resources, nil
}

// Performs matching without replacement. If a match is found between a left resource and a
// right resource, a new Match is created and the left and right are removed from their respective
// slices. Matching stops when there are no elements remaining in one of the slices. The original
// slices are copied before being modified.
func (m *Matcher) matchWithoutReplacement(lefts, rights []interface{}) (matches []Match, unmatchables []interface{}, err error) {

	// Make copies since these slices will be mutated.
	leftResources := make([]interface{}, len(lefts))
	copy(leftResources, lefts)
	rightResources := make([]interface{}, len(rights))
	copy(rightResources, rights)

	// Build a PathMap for each resource that can be used to compare them. We do this only
	// once at the start of matching to minimize the use of reflection.
	leftPathMaps := m.traverseResources(leftResources)
	rightPathMaps := m.traverseResources(rightResources)

	for len(leftPathMaps) > 0 {
		// For consistency we always start with the first resource in the left slice.
		// Remove the resource and PathMap from the front of each slice, then compare
		// to everything remaining in the right slice.
		leftResource := leftResources[0]
		leftResources = leftResources[1:]

		leftPathMap := leftPathMaps[0]
		leftPathMaps = leftPathMaps[1:]

		matchIdx := 0
		matchFound := false
		for i := 0; i < len(rightResources); i++ {
			rightResource := rightResources[i]
			rightPathMap := rightPathMaps[i]

			// Left and right must be the same type of resource.
			leftResourceType := fhirutil.GetResourceType(leftResource)
			rightResourceType := fhirutil.GetResourceType(rightResource)
			if leftResourceType != rightResourceType {
				return nil, nil, fmt.Errorf("Mismatched resource types %s and %s, cannot compare", leftResourceType, rightResourceType)
			}

			matchFound = m.comparePaths(leftPathMap, rightPathMap)
			if matchFound {
				matches = append(matches, Match{
					ResourceType: leftResourceType, // could also use rightResourceType
					Left:         leftResource,
					Right:        rightResource,
				})
				break
			}

			matchIdx++
		}

		if matchFound {
			// Remove the right resource and PathMap from their slices.
			rightResources = append(rightResources[:matchIdx], rightResources[matchIdx+1:]...)
			rightPathMaps = append(rightPathMaps[:matchIdx], rightPathMaps[matchIdx+1:]...)
		} else {
			// No match was found, so the left resource is unmatchable.
			unmatchables = append(unmatchables, leftResource)
		}
	}

	// If there are any rightResources left, they're unmatchable.
	if len(rightResources) > 0 {
		unmatchables = append(unmatchables, rightResources...)
	}

	return matches, unmatchables, nil
}

// traverses a list of resources, generating a PathMap for each.
func (m *Matcher) traverseResources(resources []interface{}) []PathMap {
	pathMaps := make([]PathMap, len(resources))

	for i, resource := range resources {
		pathmap := make(PathMap)
		traverse(reflect.ValueOf(resource), pathmap, "")
		pathMaps[i] = pathmap
	}
	return pathMaps
}

// comparePaths compares all common paths between two resources. If enough values at those
// paths "match", the resources are considered a match.
func (m *Matcher) comparePaths(leftPathMap, rightPathMap PathMap) bool {
	// We can only match on paths in both resources.
	commonPaths := intersection(leftPathMap.Keys(), rightPathMap.Keys())

	// But don't match on every path - some are unsuitable for matching.
	matchablePaths := m.stripUnsuitablePaths(commonPaths)

	totalCriteria := float64(len(matchablePaths))
	matchCounter := 0.0

	if totalCriteria == 0 {
		// There is nothing in-common to match on.
		return false
	}

	for _, mp := range matchablePaths {
		if m.matchValues(leftPathMap[mp], rightPathMap[mp]) {
			matchCounter++
		}
	}

	// Test how many of the common paths were a match. If the percentage of matches exceeds
	// the configurable MatchThreshold, we've got a match. At this point totalCriteria is
	// guaranteed to be greater than 0, making division by 0 impossible.
	if (matchCounter / totalCriteria) >= MatchThreshold {
		return true
	}
	return false
}

// stripUnsuitablePaths elminiates any paths that are unsuitable for matching,
// for example IDs, internal URLs, or code system identifiers.
func (m *Matcher) stripUnsuitablePaths(paths []string) []string {
	matchablePaths := make([]string, 0, len(paths))

	for _, path := range paths {
		if !ciPathContainsAny(path, PathsUnsuitableForComparison) {
			matchablePaths = append(matchablePaths, path)
		}
	}
	return matchablePaths
}

// matchValues compares 2 reflected values obtained by traversing FHIR resources. The values
// must be of the same kind to do a comparison. matchValues should only be used to match up values
// collected by traverse(). Traverse ensures that only primitive go types (strings, bools, ints, etc.)
// are collected as valid paths for comparison. Matching may be imperfect, or "fuzzy".
func (m *Matcher) matchValues(left, right reflect.Value) bool {
	if left.Kind() != right.Kind() {
		return false
	}

	switch left.Kind() {
	case reflect.String:
		return left.String() == right.String()

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return left.Uint() == right.Uint()

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return left.Int() == right.Int()

	case reflect.Float32, reflect.Float64:
		return fuzzyFloatMatch(left.Float(), right.Float())

	case reflect.Bool:
		return left.Bool() == right.Bool()

	// This is only for models.FHIRDateTime objects, all other structs should have been traversed.
	case reflect.Struct:
		leftTime, ok := left.Interface().(models.FHIRDateTime)
		if !ok {
			return false
		}
		rightTime, ok := right.Interface().(models.FHIRDateTime)
		if !ok {
			return false
		}
		return fuzzyTimeMatch(leftTime, rightTime)

	default:
		return false
	}
}

func fuzzyFloatMatch(leftFloat, rightFloat float64) bool {
	// Floats are matched to within a given tolerance (e.g. 0.00001).
	if math.Abs(leftFloat-rightFloat) <= FloatTolerance {
		return true
	}
	return false
}

func fuzzyTimeMatch(leftTime, rightTime models.FHIRDateTime) bool {
	// Check that the times both use the same location.
	if leftTime.Time.Location() != rightTime.Time.Location() {
		// If they don't, force them to UTC.
		leftTime.Time = leftTime.Time.UTC()
		rightTime.Time = rightTime.Time.UTC()
	}

	// Timestamps are a "match" if they occur on the same calendar day.
	leftY, leftM, leftD := leftTime.Time.Date()
	rightY, rightM, rightD := rightTime.Time.Date()
	return ((leftY == rightY) && (leftM == rightM) && (leftD == rightD))
}

// test if a path contains any of these items, case-insensitive.
func ciPathContainsAny(path string, items []string) bool {
	lowercasePath := strings.ToLower(path)
	for _, item := range items {
		if strings.Contains(lowercasePath, strings.ToLower(item)) {
			return true
		}
	}
	return false
}
