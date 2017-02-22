package merge

import (
	"fmt"

	"github.com/intervention-engine/fhir/models"
)

// MatchingStrategies maps a custom matching strategy to a specific resource type.
// For example, "Patient": &PatientMatchingStrategy{}. To be used by Matcher.Match()
// any new custom matching strategy must be registered here.
var MatchingStrategies = map[string]MatchingStrategy{}

// Matcher provides tools for identifying all resources in 2 source bundles that "match".
type Matcher struct{}

// Match iterates through all resources in the two source bundles and attempts to find resources that "match". These
// resources can then be compared to each other to see what conflicts may still exist between them. All matches are
// returned as a slice of Match pairs. Resources without a match are returned separately as a slice of "unmatchables".
func (m *Matcher) Match(leftBundle *models.Bundle, rightBundle *models.Bundle) (matches []Match, unmatchables []interface{}, err error) {

	// First collect and separate out all resources we can or cannot match.
	leftMatchables, leftUnmatchables, err := m.collectMatchableResources(leftBundle)
	if err != nil {
		return nil, nil, err
	}
	unmatchables = append(unmatchables, leftUnmatchables...)

	rightMatchables, rightUnmatchables, err := m.collectMatchableResources(rightBundle)
	if err != nil {
		return nil, nil, err
	}
	unmatchables = append(unmatchables, rightUnmatchables...)

	// There are now three sets to consider:
	// 1. L intersect R - these are all the resource types we'll attempt to match
	// 2. L not in R - these are "unmatchable" left resource types
	// 3. R not in L - these are "unmatchable" right resource types
	matchableResourceTypes := intersection(leftMatchables.Keys(), rightMatchables.Keys())
	leftUnmatchableResourceTypes := setDiff(leftMatchables.Keys(), rightMatchables.Keys())  // L not in R
	rightUnmatchableResourceTypes := setDiff(rightMatchables.Keys(), leftMatchables.Keys()) // R not in L

	// Handle all of the unmatchable resource types.
	for _, key := range leftUnmatchableResourceTypes {
		unmatchables = append(unmatchables, leftMatchables[key]...)
	}

	for _, key := range rightUnmatchableResourceTypes {
		unmatchables = append(unmatchables, rightMatchables[key]...)
	}

	// Handle all of the matchable resource types. Since the resourceType key existed
	// in both the leftMatchables and rightMatchables, there is guaranteed to be at least
	// one resource of each resourceType in both sets.
	for _, resourceType := range matchableResourceTypes {
		leftResources := leftMatchables[resourceType]
		rightResources := rightMatchables[resourceType]

		// Performs matching without replacement.
		someMatches, someUnmatchables, err := m.matchWithoutReplacement(leftResources, rightResources, MatchingStrategies[resourceType])
		if err != nil {
			return nil, nil, err
		}
		matches = append(matches, someMatches...)
		unmatchables = append(unmatchables, someUnmatchables...)
	}
	return matches, unmatchables, nil
}

// Processes all resources in a bundle and determines if they can be "matched". A resource can
// only be matched if a MatchingStrategy is implemented and registered for its resource type.
func (m *Matcher) collectMatchableResources(bundle *models.Bundle) (matchables ResourceMap, unmatchables []interface{}, err error) {
	matchables = make(ResourceMap)

	for _, entry := range bundle.Entry {
		// Get the entry.Resource's type.
		resourceType := getResourceType(entry.Resource)
		// Make sure it's a known FHIR type.
		s := models.StructForResourceName(resourceType)
		if s == nil {
			return nil, nil, fmt.Errorf("Unknown resource type %s", resourceType)
		}

		if m.supportsMatchingStrategyForResourceType(resourceType) {
			matchables[resourceType] = append(matchables[resourceType], entry.Resource)
		} else {
			unmatchables = append(unmatchables, entry.Resource)
		}
	}
	return matchables, unmatchables, nil
}

// Performs matching without replacement using the given strategy. If a match is found between a left resource and a
// right resource, a new Match is created and the left and right are removed from their respective slices. Matching stops
// when there are no elements remaining in one of the slices. The original slices are copied before being modified.
func (m *Matcher) matchWithoutReplacement(left, right []interface{}, strategy MatchingStrategy) (matches []Match, unmatchables []interface{}, err error) {

	// Make copies since these slices will be mutated.
	leftResources := make([]interface{}, len(left))
	copy(leftResources, left)
	rightResources := make([]interface{}, len(right))
	copy(rightResources, right)

	for len(leftResources) > 0 {
		// For consistency we always start with the first element in the left slice.
		// Remove the resource from the front of the slice, then compare it to everything
		// remaining in the right slice.
		leftResource := leftResources[0]
		leftResources = leftResources[1:]

		matchFound := false
		matchIdx := 0

		for _, rightResource := range rightResources {
			matchFound, err = strategy.Match(leftResource, rightResource)
			if err != nil {
				return nil, nil, err
			}

			if matchFound {
				matches = append(matches, Match{
					ResourceType: strategy.SupportedResourceType(),
					Left:         leftResource,
					Right:        rightResource,
				})
				break
			}

			matchIdx++
		}

		if matchFound {
			// Remove the rightResource from its slice.
			rightResources = append(rightResources[:matchIdx], rightResources[matchIdx+1:]...)
		} else {
			// No match was found, so the leftResource has no match. Right resources remain unchanged.
			unmatchables = append(unmatchables, leftResource)
		}
	}

	// If there are any rightResources left, they're unmatchable.
	if len(rightResources) > 0 {
		unmatchables = append(unmatchables, rightResources...)
	}

	return matches, unmatchables, nil
}

// Returns true if a known matching strategy is implemented for the given resourceType.
// All matching strategies should be registered in the MatchingStrategies map.
func (m *Matcher) supportsMatchingStrategyForResourceType(resourceType string) bool {
	for key := range MatchingStrategies {
		if key == resourceType {
			return true
		}
	}
	return false
}
