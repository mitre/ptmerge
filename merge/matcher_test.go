package merge

import (
	"testing"

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

// ========================================================================= //
// MATCHING STRATEGY INTERFACE MOCKS                                         //
// ========================================================================= //

type FooType struct {
	Value int
}

type FooMatchingStrategy struct{}

func (f *FooMatchingStrategy) SupportedResourceType() string {
	return "FooType"
}

func (f *FooMatchingStrategy) Match(left interface{}, right interface{}) (isMatch bool, err error) {
	l := left.(*FooType)
	r := right.(*FooType)
	return l.Value == r.Value, nil
}

// ========================================================================= //
// TEST MATCH                                                                //
// ========================================================================= //

func (m *MatcherTestSuite) TestMatch() {
	fix1, err := testutil.LoadFixture("Bundle", "../fixtures/clint_abbot_bundle.json")
	m.NoError(err)
	m.NotNil(fix1)
	bundle1, ok := fix1.(*models.Bundle)
	m.True(ok)

	fix2, err := testutil.LoadFixture("Bundle", "../fixtures/john_peters_bundle.json")
	m.NoError(err)
	m.NotNil(fix2)
	bundle2, ok := fix2.(*models.Bundle)
	m.True(ok)

	matcher := new(Matcher)
	matches, unmatchables, err := matcher.Match(bundle1, bundle2)
	m.NoError(err)

	// There are no MatchingStrategies implemented yet, so this should return the union
	// of both bundles, left first.
	m.Len(matches, 0)
	m.Len(unmatchables, len(bundle1.Entry)+len(bundle2.Entry))
	m.Equal(bundle1.Entry[0].Resource, unmatchables[0])
}

// ========================================================================= //
// TEST PRIVATE METHODS                                                      //
// ========================================================================= //

func (m *MatcherTestSuite) TestSupportsMatchingStrategyForResourceType() {
	MatchingStrategies["Foo"] = new(FooMatchingStrategy)
	matcher := new(Matcher)
	m.True(matcher.supportsMatchingStrategyForResourceType("Foo"))

	delete(MatchingStrategies, "Foo")
	m.False(matcher.supportsMatchingStrategyForResourceType("Foo"))
}

func (m *MatcherTestSuite) TestCollectMatchableResources() {
	fix, err := testutil.LoadFixture("Bundle", "../fixtures/clint_abbot_bundle.json")
	m.NoError(err)
	m.NotNil(fix)
	bundle, ok := fix.(*models.Bundle)
	m.True(ok)
	matcher := new(Matcher)
	matchables, unmatchables, err := matcher.collectMatchableResources(bundle)
	m.NoError(err)
	// No custom matchers have been implemented yet, so everything should be "unmatchable".
	m.NotNil(matchables)
	m.Equal([]string{}, matchables.Keys())
	m.NotNil(unmatchables)
	m.Equal(len(bundle.Entry), len(unmatchables))
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
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources, &FooMatchingStrategy{})

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
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources, &FooMatchingStrategy{})

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
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources, &FooMatchingStrategy{})

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
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources, &FooMatchingStrategy{})

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
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources, &FooMatchingStrategy{})

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
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources, &FooMatchingStrategy{})

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
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources, &FooMatchingStrategy{})

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
	matches, unmatchables, err := matcher.matchWithoutReplacement(leftResources, rightResources, &FooMatchingStrategy{})

	m.NoError(err)
	m.Len(matches, 1)
	m.Equal(Match{ResourceType: "FooType", Left: leftResources[1], Right: rightResources[1]}, matches[0])

	m.Len(unmatchables, 3)
	m.Equal([]interface{}{leftResources[0], rightResources[0], rightResources[2]}, unmatchables)
}
