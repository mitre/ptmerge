package merge

import (
	"reflect"
	"testing"
	"time"

	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/fhirutil"
	"github.com/stretchr/testify/suite"
)

type MatcherTestSuite struct {
	suite.Suite
}

func TestMatcherTestSuite(t *testing.T) {
	suite.Run(t, new(MatcherTestSuite))
}

type FooType struct {
	Resource
	Value int `json:"value,omitempty"`
}

type Resource struct {
	Id           string `json:"id,omitempty"`
	ResourceType string `json:"resourceType,omitempty"`
}

// ========================================================================= //
// TEST MATCH                                                                //
// ========================================================================= //

func (m *MatcherTestSuite) TestMatchBundlesPerfectMatch() {
	var err error

	fix, err := fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	leftBundle, ok := fix.(*models.Bundle)
	m.True(ok)

	fix, err = fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	rightBundle, ok := fix.(*models.Bundle)
	m.True(ok)

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.Match(leftBundle, rightBundle)
	m.NoError(err)

	// There should be no unmatchables and the same number of matches as there are
	// resources in one of the bundles.
	m.Len(matches, len(leftBundle.Entry))
	m.Len(unmatchables, 0)
}

func (m *MatcherTestSuite) TestMatchBundlesGoodMatch() {
	var err error

	// These are the same person, with a few discrepancies in their records.
	fix, err := fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	leftBundle, ok := fix.(*models.Bundle)
	m.True(ok)

	fix, err = fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_unmarried_bundle.json")
	m.NoError(err)
	rightBundle, ok := fix.(*models.Bundle)
	m.True(ok)

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.Match(leftBundle, rightBundle)
	m.NoError(err)

	// Most things will match, including the Encounter that has a different timestamp.
	m.Len(matches, 6)

	for _, match := range matches {
		if match.ResourceType == "Encounter" {
			if fhirutil.GetResourceID(match.Left) == "58a4904e97bba945de21eac0" {
				left, ok := match.Left.(*models.Encounter)
				m.True(ok)
				right, ok := match.Right.(*models.Encounter)
				m.True(ok)
				m.False(left.Period.Start.Time.Equal(right.Period.Start.Time))
				m.False(left.Period.End.Time.Equal(right.Period.End.Time))

				m.True(fuzzyTimeMatch(*left.Period.Start, *right.Period.Start))
				m.True(fuzzyTimeMatch(*left.Period.End, *right.Period.End))
			}
		}
	}

	// A MedicationStatement in the left not present in the right, and is therefore unmatchable.
	m.Len(unmatchables, 1)
	um := unmatchables[0]
	m.Equal("MedicationStatement", fhirutil.GetResourceType(um))
	m.Equal("58a4904e97bba945de21fac0", fhirutil.GetResourceID(um))
}

func (m *MatcherTestSuite) TestMatchBundlesPartialMatch() {
	var err error

	// These are not the same person, but they happened to have similar medications.
	fix, err := fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	leftBundle, ok := fix.(*models.Bundle)
	m.True(ok)

	fix, err = fhirutil.LoadResource("Bundle", "../fixtures/bundles/clint_abbott_bundle.json")
	m.NoError(err)
	rightBundle, ok := fix.(*models.Bundle)
	m.True(ok)

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.Match(leftBundle, rightBundle)
	m.NoError(err)

	// Most things will not match, but the MedicationStatements will.
	m.Len(matches, 2)
	m.Equal("MedicationStatement", matches[0].ResourceType)
	m.Equal("MedicationStatement", matches[1].ResourceType)

	// Most things will not match. Most striking here is that the Patient didn't match,
	// so there will be 2 of them.
	m.Len(unmatchables, 9)
	pcount := 0
	for _, resource := range unmatchables {
		if fhirutil.GetResourceType(resource) == "Patient" {
			pcount++
		}
	}
	m.Equal(2, pcount)

	// There should also be some encounters.
	ecount := 0
	for _, resource := range unmatchables {
		if fhirutil.GetResourceType(resource) == "Encounter" {
			ecount++
		}
	}
	m.Equal(3, ecount)

	// And 2 different procedures.
	pcount = 0
	for _, resource := range unmatchables {
		if fhirutil.GetResourceType(resource) == "Procedure" {
			pcount++
		}
	}
	m.Equal(2, pcount)
}

func (m *MatcherTestSuite) TestMatchBundlesNoMatch() {
	var err error

	// There's absolutely nothing in common, so everything should be unmatchable.
	fix, err := fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	leftBundle, ok := fix.(*models.Bundle)
	m.True(ok)

	fix, err = fhirutil.LoadResource("Bundle", "../fixtures/bundles/joey_chestnut_bundle.json")
	m.NoError(err)
	rightBundle, ok := fix.(*models.Bundle)
	m.True(ok)

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.Match(leftBundle, rightBundle)
	m.NoError(err)
	m.Len(matches, 0)
	m.Len(unmatchables, 12)

	// Collectively, all of the left bundle entires and right bundle entries should
	// be in the unmatchables.
	for _, entry := range leftBundle.Entry {
		id := fhirutil.GetResourceID(entry.Resource)
		found := false
		for _, um := range unmatchables {
			if id == fhirutil.GetResourceID(um) {
				found = true
			}
		}
		m.True(found)
	}
}

// ========================================================================= //
// TEST COLLECTION AND TRAVERSAL                                             //
// ========================================================================= //

func (m *MatcherTestSuite) TestCollectResources() {
	fix, err := fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	m.NotNil(fix)
	bundle, ok := fix.(*models.Bundle)
	m.True(ok)
	matcher := new(Matcher)
	resourceMap, err := matcher.collectResources(bundle)
	m.NoError(err)
	m.NotNil(resourceMap)

	expectedResourceTypes := []string{"Patient", "Encounter", "Procedure", "Condition", "MedicationStatement"}
	for _, resourceType := range resourceMap.Keys() {
		m.True(contains(expectedResourceTypes, resourceType))
	}
}

func (m *MatcherTestSuite) TestTraverseResources() {
	fix, err := fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	m.NotNil(fix)
	bundle, ok := fix.(*models.Bundle)
	m.True(ok)
	matcher := new(Matcher)
	resourceMap, err := matcher.collectResources(bundle)
	m.NoError(err)
	m.NotNil(resourceMap)

	// Traverse the encounters
	encounterPaths := matcher.traverseResources(resourceMap["Encounter"])
	m.Len(encounterPaths, 2)

	// Check the paths in one of the resources.
	expected := []string{
		"id",
		"class.code",
		"type[0].coding[0].system",
		"type[0].coding[0].code",
		"patient.type",
		"patient.referenceid",
		"period.end",
		"resourceType",
		"status",
		"type[0].coding[0].display",
		"patient.reference",
		"patient.external",
		"period.start",
	}

	for _, path := range encounterPaths[0].Keys() {
		m.True(contains(expected, path))
	}
}

func (m *MatcherTestSuite) TestStripUnsuitablePaths() {
	paths := []string{
		"foo",
		"id",
		"bar",
		"code[0].system",
		"_id",
		"extension[0].text",
		"fullUrl",
		"hello",
		"extension[1].url",
		"world",
	}

	// Should eliminate "id", "_id", "text", "system", and both "url"s.
	matcher := new(Matcher)
	remaining := matcher.stripUnsuitablePaths(paths)

	expected := []string{
		"foo",
		"bar",
		"hello",
		"world",
	}

	for _, path := range remaining {
		m.True(contains(expected, path))
	}
}

// ========================================================================= //
// TEST MATCHING WITHOUT REPLACEMENT                                         //
// ========================================================================= //

func (m *MatcherTestSuite) TestOneLeftMatchesOneRightNoneRemaining() {
	leftResources := []interface{}{
		&FooType{Value: 1},
	}
	rightResources := []interface{}{
		&FooType{Value: 1},
	}

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources)

	m.NoError(err)
	m.Len(matches, 1)
	m.Equal(Match{Left: leftResources[0], Right: rightResources[0]}, matches[0])

	m.Len(unmatchables, 0)
}

func (m *MatcherTestSuite) TestOneLeftDoesntMatchOneRight() {
	leftResources := []interface{}{
		&FooType{Value: 1},
	}
	rightResources := []interface{}{
		&FooType{Value: 2},
	}

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources)

	m.NoError(err)
	m.Len(matches, 0)

	m.Len(unmatchables, 2)
	m.Equal(append(leftResources, rightResources...), unmatchables)
}

func (m *MatcherTestSuite) TestOneLeftMatchesOneRightRightsRemaining() {
	leftResources := []interface{}{
		&FooType{Value: 1},
	}
	rightResources := []interface{}{
		&FooType{Value: 0},
		&FooType{Value: 1},
		&FooType{Value: 2},
	}

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources)

	m.NoError(err)
	m.Len(matches, 1)
	m.Equal(Match{Left: leftResources[0], Right: rightResources[1]}, matches[0])

	m.Len(unmatchables, 2)
	m.Equal([]interface{}{rightResources[0], rightResources[2]}, unmatchables)
}

func (m *MatcherTestSuite) TestOneLeftMatchesOneRightLeftsRemaining() {
	leftResources := []interface{}{
		&FooType{Value: 3},
		&FooType{Value: 2},
		&FooType{Value: 1},
	}
	rightResources := []interface{}{
		&FooType{Value: 1},
	}

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources)

	m.NoError(err)
	m.Len(matches, 1)
	m.Equal(Match{Left: leftResources[2], Right: rightResources[0]}, matches[0])

	m.Len(unmatchables, 2)
	m.Equal(leftResources[:2], unmatchables)
}

func (m *MatcherTestSuite) TestMultipleMatchesRightsRemaining() {
	leftResources := []interface{}{
		&FooType{Value: 1},
		&FooType{Value: 4},
	}
	rightResources := []interface{}{
		&FooType{Value: 1},
		&FooType{Value: 4},
		&FooType{Value: 7},
		&FooType{Value: 8},
	}

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources)

	m.NoError(err)
	m.Len(matches, 2)
	m.Equal(Match{Left: leftResources[0], Right: rightResources[0]}, matches[0])
	m.Equal(Match{Left: leftResources[1], Right: rightResources[1]}, matches[1])

	m.Len(unmatchables, 2)
	m.Equal(rightResources[2:], unmatchables)
}

func (m *MatcherTestSuite) TestMultipleMatchesLeftsRemaining() {
	leftResources := []interface{}{
		&FooType{Value: 1},
		&FooType{Value: 2},
		&FooType{Value: 3},
		&FooType{Value: 4},
	}
	rightResources := []interface{}{
		&FooType{Value: 2},
		&FooType{Value: 3},
	}

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources)

	m.NoError(err)
	m.Len(matches, 2)
	m.Equal(Match{Left: leftResources[1], Right: rightResources[0]}, matches[0])
	m.Equal(Match{Left: leftResources[2], Right: rightResources[1]}, matches[1])

	m.Len(unmatchables, 2)
	m.Equal([]interface{}{leftResources[0], leftResources[3]}, unmatchables)
}

func (m *MatcherTestSuite) TestMultipleMatchesBothRemaining() {
	leftResources := []interface{}{
		&FooType{Value: 1},
		&FooType{Value: 2},
		&FooType{Value: 4},
		&FooType{Value: 3},
	}
	rightResources := []interface{}{
		&FooType{Value: 5},
		&FooType{Value: 2},
		&FooType{Value: 6},
		&FooType{Value: 3},
	}

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources)

	m.NoError(err)
	m.Len(matches, 2)
	m.Equal(Match{Left: leftResources[1], Right: rightResources[1]}, matches[0])
	m.Equal(Match{Left: leftResources[3], Right: rightResources[3]}, matches[1])

	m.Len(unmatchables, 4)
	m.Equal([]interface{}{leftResources[0], leftResources[2], rightResources[0], rightResources[2]}, unmatchables)
}

func (m *MatcherTestSuite) TestMultipleMatchesOrderOfPreference() {
	// Since we always start with the next left first,
	// the order of the unmatchables should be deterministic.
	leftResources := []interface{}{
		&FooType{Value: 1},
		&FooType{Value: 2},
	}
	rightResources := []interface{}{
		&FooType{Value: 0},
		&FooType{Value: 2}, // should specifically match this one
		&FooType{Value: 2},
	}

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources)

	m.NoError(err)
	m.Len(matches, 1)
	m.Equal(Match{Left: leftResources[1], Right: rightResources[1]}, matches[0])

	m.Len(unmatchables, 3)
	m.Equal([]interface{}{leftResources[0], rightResources[0], rightResources[2]}, unmatchables)
}

// ========================================================================= //
// TEST COMPARING RESOURCES                                                  //
// ========================================================================= //

func (m *MatcherTestSuite) TestComparePathsMatchAboveThreshold() {
	fix1, err := fhirutil.LoadResource("Patient", "../fixtures/patients/bernard_johnston_patient.json")
	m.NoError(err)

	fix2, err := fhirutil.LoadResource("Patient", "../fixtures/patients/bernard_johnson_patient.json")
	m.NoError(err)

	// Build paths for each patient.
	matcher := new(Matcher)
	pathmaps := matcher.traverseResources([]interface{}{fix1, fix2})
	m.Len(pathmaps, 2)
	m.True(matcher.comparePaths(pathmaps[0], pathmaps[1]))
}

func (m *MatcherTestSuite) TestComparePathsNoMatchBelowThreshold() {
	fix1, err := fhirutil.LoadResource("Patient", "../fixtures/patients/bernard_johnston_patient.json")
	m.NoError(err)

	fix2, err := fhirutil.LoadResource("Patient", "../fixtures/patients/bernard_johnstone_patient.json")
	m.NoError(err)

	// Build paths for each patient.
	matcher := new(Matcher)
	pathmaps := matcher.traverseResources([]interface{}{fix1, fix2})
	m.Len(pathmaps, 2)
	m.False(matcher.comparePaths(pathmaps[0], pathmaps[1]))
}

func (m *MatcherTestSuite) TestComparePathsMatchLowThresholdNoMatchHighThreshold() {
	fix1, err := fhirutil.LoadResource("Patient", "../fixtures/patients/bernard_johnston_patient.json")
	m.NoError(err)

	fix2, err := fhirutil.LoadResource("Patient", "../fixtures/patients/bernard_johnstone_patient.json")
	m.NoError(err)

	matcher := new(Matcher)
	pathmaps := matcher.traverseResources([]interface{}{fix1, fix2})
	m.Len(pathmaps, 2)

	originalThreshold := MatchThreshold

	// Matches with a low threshold.
	MatchThreshold = 0.5
	m.True(matcher.comparePaths(pathmaps[0], pathmaps[1]))

	// But not with a higher threshold.
	MatchThreshold = 0.9
	m.False(matcher.comparePaths(pathmaps[0], pathmaps[1]))

	// Revert to the original setting.
	MatchThreshold = originalThreshold
}

// ========================================================================= //
// TEST MATCH VALUES                                                         //
// ========================================================================= //

func (m *MatcherTestSuite) TestMatchStringValues() {
	matcher := new(Matcher)

	v1 := reflect.ValueOf("hello")
	v2 := reflect.ValueOf("hello")
	m.True(matcher.matchValues(v1, v2))

	v3 := reflect.ValueOf("world")
	m.False(matcher.matchValues(v1, v3))
}

func (m *MatcherTestSuite) TestMatchIntegerValues() {
	matcher := new(Matcher)

	i1 := reflect.ValueOf(uint32(2))
	i2 := reflect.ValueOf(uint32(2))
	m.True(matcher.matchValues(i1, i2))

	i3 := reflect.ValueOf(uint32(0))
	m.False(matcher.matchValues(i1, i3))

	i4 := reflect.ValueOf(uint32(0))
	m.True(matcher.matchValues(i3, i4))
}

func (m *MatcherTestSuite) TestMatchFloatValues() {
	matcher := new(Matcher)

	i1 := reflect.ValueOf(float64(5.2))
	i2 := reflect.ValueOf(float64(5.2))
	m.True(matcher.matchValues(i1, i2))

	i3 := reflect.ValueOf(float64(0))
	m.False(matcher.matchValues(i1, i3))

	i4 := reflect.ValueOf(float64(0))
	m.True(matcher.matchValues(i3, i4))
}

func (m *MatcherTestSuite) TestMatchBooleanValues() {
	matcher := new(Matcher)

	v1 := reflect.ValueOf(false)
	v2 := reflect.ValueOf(false)
	m.True(matcher.matchValues(v1, v2))

	v3 := reflect.ValueOf(true)
	m.False(matcher.matchValues(v1, v3))
}

func (m *MatcherTestSuite) TestMatchTimeValues() {
	matcher := new(Matcher)

	t := time.Now().UTC()
	t1 := models.FHIRDateTime{
		Time:      t,
		Precision: models.Timestamp,
	}
	t2 := models.FHIRDateTime{
		Time:      t,
		Precision: models.Timestamp,
	}
	m.True(matcher.matchValues(reflect.ValueOf(t1), reflect.ValueOf(t2)))

	t3 := models.FHIRDateTime{
		Time:      time.Now().UTC(),
		Precision: models.Timestamp,
	}
	m.True(matcher.matchValues(reflect.ValueOf(t1), reflect.ValueOf(t3)))
}

func (m *MatcherTestSuite) TestMatchDifferentKindsAlwaysFalse() {
	matcher := new(Matcher)

	v1 := reflect.ValueOf("foo")
	v2 := reflect.ValueOf(3)
	m.False(matcher.matchValues(v1, v2))
}

// ========================================================================= //
// TEST FUZZY MATCHERS                                                       //
// ========================================================================= //

func (m *MatcherTestSuite) TestFuzzyFloatMatch() {
	// Exact match.
	f1 := float64(5.4)
	f2 := float64(5.4)
	m.True(fuzzyFloatMatch(f1, f2))

	// Exact match within the tolerance.
	f3 := float64(4.33334)
	f4 := float64(4.33333)
	m.True(fuzzyFloatMatch(f3, f4))

	// Difference exceeds the tolerance for a match.
	f5 := float64(13.2)
	f6 := float64(13.3)
	m.False(fuzzyFloatMatch(f5, f6))
}

func (m *MatcherTestSuite) TestFuzzyTimeMatchUTC() {
	// Same day, different time of day.
	t1 := models.FHIRDateTime{
		Time:      time.Date(2017, 2, 10, 4, 13, 54, 0, time.UTC),
		Precision: models.Timestamp,
	}
	t2 := models.FHIRDateTime{
		Time:      time.Date(2017, 2, 10, 13, 24, 11, 0, time.UTC),
		Precision: models.Timestamp,
	}
	m.True(fuzzyTimeMatch(t1, t2))

	// Different days should fail.
	t3 := models.FHIRDateTime{
		Time:      time.Date(2017, 2, 9, 6, 45, 33, 0, time.UTC),
		Precision: models.Timestamp,
	}
	m.False(fuzzyTimeMatch(t1, t3))

	// The extremes of a callendar day should still match.
	t4 := models.FHIRDateTime{
		Time:      time.Date(2017, 1, 31, 0, 0, 0, 0, time.UTC),
		Precision: models.Timestamp,
	}
	t5 := models.FHIRDateTime{
		Time:      time.Date(2017, 1, 31, 23, 59, 59, 0, time.UTC),
		Precision: models.Timestamp,
	}
	m.True(fuzzyTimeMatch(t4, t5))
}

func (m *MatcherTestSuite) TestFuzzyTimeMatchEST() {
	loc, err := time.LoadLocation("America/New_York")
	m.NoError(err)

	// Same day, different time of day.
	t1 := models.FHIRDateTime{
		Time:      time.Date(2017, 2, 10, 4, 13, 54, 0, loc),
		Precision: models.Timestamp,
	}
	t2 := models.FHIRDateTime{
		Time:      time.Date(2017, 2, 10, 13, 24, 11, 0, loc),
		Precision: models.Timestamp,
	}
	m.True(fuzzyTimeMatch(t1, t2))

	// Different days should fail.
	t3 := models.FHIRDateTime{
		Time:      time.Date(2017, 2, 9, 6, 45, 33, 0, loc),
		Precision: models.Timestamp,
	}
	m.False(fuzzyTimeMatch(t1, t3))

	// The extremes of a callendar day should still match.
	t4 := models.FHIRDateTime{
		Time:      time.Date(2017, 1, 31, 0, 0, 0, 0, loc),
		Precision: models.Timestamp,
	}
	t5 := models.FHIRDateTime{
		Time:      time.Date(2017, 1, 31, 23, 59, 59, 0, loc),
		Precision: models.Timestamp,
	}
	m.True(fuzzyTimeMatch(t4, t5))

	// Can reliably match times from different locations.
	t6 := models.FHIRDateTime{
		Time:      time.Date(2017, 1, 31, 4, 15, 32, 0, time.UTC),
		Precision: models.Timestamp,
	}
	m.True(fuzzyTimeMatch(t4, t6))
}
