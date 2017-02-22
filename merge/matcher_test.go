package merge

import (
	"reflect"
	"testing"
	"time"

	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type MatcherTestSuite struct {
	suite.Suite
}

func TestMatcherTestSuite(t *testing.T) {
	suite.Run(t, new(MatcherTestSuite))
}

type FooType struct {
	Value int `json:"value,omitempty"`
}

// ========================================================================= //
// TEST MATCH                                                                //
// ========================================================================= //

// TODO: Implement a series of fixtures and tests to adequately validate matching.

// ========================================================================= //
// TEST PRIVATE METHODS                                                      //
// ========================================================================= //

func (m *MatcherTestSuite) TestCollectResources() {
	fix, err := testutil.LoadFixture("Bundle", "../fixtures/clint_abbot_bundle.json")
	m.NoError(err)
	m.NotNil(fix)
	bundle, ok := fix.(*models.Bundle)
	m.True(ok)
	matcher := new(Matcher)
	resourceMap, err := matcher.collectResources(bundle)
	m.NoError(err)
	m.NotNil(resourceMap)

	expectedResourceTypes := []string{"Patient", "Encounter", "Condition", "MedicationStatement"}
	for _, resourceType := range resourceMap.Keys() {
		m.True(contains(expectedResourceTypes, resourceType))
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
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[0], Right: rightResources[0]}, matches[0])

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
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[0], Right: rightResources[1]}, matches[0])

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
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[2], Right: rightResources[0]}, matches[0])

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
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[0], Right: rightResources[0]}, matches[0])
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[1], Right: rightResources[1]}, matches[1])

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
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[1], Right: rightResources[0]}, matches[0])
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[2], Right: rightResources[1]}, matches[1])

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
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[1], Right: rightResources[1]}, matches[0])
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[3], Right: rightResources[3]}, matches[1])

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
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[1], Right: rightResources[1]}, matches[0])

	m.Len(unmatchables, 3)
	m.Equal([]interface{}{leftResources[0], rightResources[0], rightResources[2]}, unmatchables)
}

// ========================================================================= //
// TEST COMPARING RESOURCES                                                  //
// ========================================================================= //

func (m *MatcherTestSuite) TestComparePathsMatchAboveThreshold() {

}

func (m *MatcherTestSuite) TestComparePathsNoMatchBelowThreshold() {

}

func (m *MatcherTestSuite) TestComparePathsMatchLowThresholdNoMatchHighThreshold() {

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
	t1 := reflect.ValueOf(t)
	t2 := reflect.ValueOf(t)
	m.True(matcher.matchValues(t1, t2))

	t3 := reflect.ValueOf(time.Now().UTC())
	m.True(matcher.matchValues(t1, t3))
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

}

func (m *MatcherTestSuite) TestFuzzyTimeMatchUTC() {

}

func (m *MatcherTestSuite) TestFuzzyTimeMatchEST() {

}
