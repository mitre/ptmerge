package merge

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type DetectorTestSuite struct {
	suite.Suite
}

func TestDetectorTestSuite(t *testing.T) {
	suite.Run(t, new(DetectorTestSuite))
}

type BarType struct {
	A string    `json:"a,omitempty"`
	B *uint32   `json:"b,omitempty"`
	C time.Time `json:"c,omitempty"`
	D *BazType  `json:"d,omitempty"`
}

type BazType struct {
	X string `json:"x,omitempty"`
	Y *bool  `json:"y,omitempty"`
}

// ========================================================================= //
// TEST CONFLICT DETECTION                                                   //
// ========================================================================= //

func (d *DetectorTestSuite) TestPerfectMatchNoConflicts() {
	leftNum := uint32(1)
	rightNum := uint32(1)
	leftBool := false
	rightBool := false
	t := time.Now().UTC()

	match := &Match{
		ResourceType: "BarType",
		Left: &BarType{
			A: "A",
			B: &leftNum,
			C: t,
			D: &BazType{
				X: "X",
				Y: &leftBool,
			},
		},
		Right: &BarType{
			A: "A",
			B: &rightNum,
			C: t,
			D: &BazType{
				X: "X",
				Y: &rightBool,
			},
		},
	}

	detector := new(Detector)
	conflict, err := detector.findConflicts(match)
	d.NoError(err)

	// A perfect match has no conflicts.
	d.Nil(conflict)
}

func (d *DetectorTestSuite) TestPartialMatchSomeConflicts() {
	leftNum := uint32(1)
	rightNum := uint32(2)
	leftBool := false
	rightBool := true
	t := time.Now().UTC()

	match := &Match{
		ResourceType: "BarType",
		Left: &BarType{
			A: "A",
			B: &leftNum,
			C: t,
			D: &BazType{
				X: "Y",
				Y: &leftBool,
			},
		},
		Right: &BarType{
			A: "B",
			B: &rightNum,
			C: t,
			D: &BazType{
				X: "X",
				Y: &rightBool,
			},
		},
	}

	detector := new(Detector)
	conflict, err := detector.findConflicts(match)
	d.NoError(err)
	d.NotNil(conflict)

	d.Len(conflict.Issue, 1)
	issue := conflict.Issue[0]

	d.Len(issue.Location, 4)
}

func (d *DetectorTestSuite) TestUncommonPathsSomeConflicts() {
	leftNum := uint32(1)
	rightNum := uint32(1)
	rightBool := false
	t := time.Now().UTC()

	// The left resource has fields not in the right, the right has fields
	// not in the left. These should automatically be conflicts.
	match := &Match{
		ResourceType: "BarType",
		Left: &BarType{
			A: "A",
			B: &leftNum,
			C: t,
		},
		Right: &BarType{
			B: &rightNum,
			C: t,
			D: &BazType{
				X: "X",
				Y: &rightBool,
			},
		},
	}

	detector := new(Detector)
	conflict, err := detector.findConflicts(match)
	d.NoError(err)
	d.NotNil(conflict)

	// Should be 1 conflict with 3 locations: left a, right d.x, right d.y
	d.Len(conflict.Issue, 1)
	d.Len(conflict.Issue[0].Location, 3)

	expectedPaths := []string{"a", "d.x", "d.y"}
	for _, loc := range conflict.Issue[0].Location {
		d.True(contains(expectedPaths, loc))
	}
}

func (d *DetectorTestSuite) TestMultipleMatchesSomeConflicts() {
	leftNum1 := uint32(1)
	rightNum1 := uint32(2)
	leftBool1 := true
	rightBool1 := false
	t1 := time.Now().UTC()

	// Conflicts: b, d.y
	match1 := &Match{
		ResourceType: "BarType",
		Left: &BarType{
			A: "A",
			B: &leftNum1,
			C: t1,
			D: &BazType{
				X: "X",
				Y: &leftBool1,
			},
		},
		Right: &BarType{
			A: "A",
			B: &rightNum1,
			C: t1,
			D: &BazType{
				X: "X",
				Y: &rightBool1,
			},
		},
	}

	rightNum2 := uint32(14)
	leftBool2 := true
	t2 := time.Now().UTC()

	// Conflicts: a, b, c, d.y
	match2 := &Match{
		ResourceType: "BarType",
		Left: &BarType{
			A: "B",
			C: t1,
			D: &BazType{
				X: "X",
				Y: &leftBool2,
			},
		},
		Right: &BarType{
			A: "A",
			B: &rightNum2,
			C: t2,
			D: &BazType{
				X: "X",
			},
		},
	}

	detector := new(Detector)
	conflicts, err := detector.AllConflicts([]Match{*match1, *match2})
	d.NoError(err)

	d.Len(conflicts, 2)
	conflict0 := conflicts[0]
	d.Len(conflict0.Issue, 1)
	conflict1 := conflicts[1]
	d.Len(conflict1.Issue, 1)

	// Validate the first conflict.
	expectedPaths := []string{"b", "d.y"}
	for _, loc := range conflict0.Issue[0].Location {
		d.True(contains(expectedPaths, loc))
	}

	// Validate the second conflict.
	expectedPaths = []string{"a", "b", "c", "d.y"}
	for _, loc := range conflict1.Issue[0].Location {
		d.True(contains(expectedPaths, loc))
	}
}

// ========================================================================= //
// TEST REFLECTION VALUE COMPARISON                                          //
// ========================================================================= //

func (rt *ResourceTraversalTestSuite) TestCompareStringValues() {
	d := new(Detector)

	v1 := reflect.ValueOf("hello")
	v2 := reflect.ValueOf("hello")
	rt.True(d.compareValues(v1, v2))

	v3 := reflect.ValueOf("world")
	rt.False(d.compareValues(v1, v3))
}

func (rt *ResourceTraversalTestSuite) TestCompareIntegerValues() {
	d := new(Detector)

	i1 := reflect.ValueOf(uint32(2))
	i2 := reflect.ValueOf(uint32(2))
	rt.True(d.compareValues(i1, i2))

	i3 := reflect.ValueOf(uint32(0))
	rt.False(d.compareValues(i1, i3))

	i4 := reflect.ValueOf(uint32(0))
	rt.True(d.compareValues(i3, i4))
}

func (rt *ResourceTraversalTestSuite) TestCompareFloatValues() {
	d := new(Detector)

	i1 := reflect.ValueOf(float64(5.2))
	i2 := reflect.ValueOf(float64(5.2))
	rt.True(d.compareValues(i1, i2))

	i3 := reflect.ValueOf(float64(0))
	rt.False(d.compareValues(i1, i3))

	i4 := reflect.ValueOf(float64(0))
	rt.True(d.compareValues(i3, i4))
}

func (rt *ResourceTraversalTestSuite) TestCompareBooleanValues() {
	d := new(Detector)

	v1 := reflect.ValueOf(false)
	v2 := reflect.ValueOf(false)
	rt.True(d.compareValues(v1, v2))

	v3 := reflect.ValueOf(true)
	rt.False(d.compareValues(v1, v3))
}

func (rt *ResourceTraversalTestSuite) TestCompareTimeValues() {
	d := new(Detector)

	t := time.Now().UTC()
	t1 := reflect.ValueOf(t)
	t2 := reflect.ValueOf(t)
	rt.True(d.compareValues(t1, t2))

	t3 := reflect.ValueOf(time.Now().UTC())
	rt.False(d.compareValues(t1, t3))
}

func (rt *ResourceTraversalTestSuite) TestCompareDifferentKindsAlwaysFalse() {
	d := new(Detector)

	v1 := reflect.ValueOf("foo")
	v2 := reflect.ValueOf(3)
	rt.False(d.compareValues(v1, v2))
}
