package merge

import (
	"reflect"
	"testing"
	"time"

	"gopkg.in/mgo.v2/bson"

	"github.com/intervention-engine/fhir/models"
	"github.com/stretchr/testify/suite"
)

type DetectorTestSuite struct {
	suite.Suite
}

func TestDetectorTestSuite(t *testing.T) {
	suite.Run(t, new(DetectorTestSuite))
}

// ========================================================================= //
// TEST CONFLICTS                                                            //
// ========================================================================= //

func (d *DetectorTestSuite) TestConflicts() {

	// We expect these to conflict by first name, Gender, ID, and BirthDate.
	tz, err := time.LoadLocation("America/New_York")
	d.NoError(err)

	match := &Match{
		ResourceType: "Patient",
		Left: &models.Patient{
			DomainResource: models.DomainResource{
				Resource: models.Resource{
					Id:           bson.NewObjectId().Hex(),
					ResourceType: "Patient",
				},
			},
			Gender: "Male",
			Name: []models.HumanName{
				models.HumanName{
					Family: "Smith",
					Given:  []string{"John"},
				},
			},
			BirthDate: &models.FHIRDateTime{
				Time:      time.Date(1976, 12, 2, 0, 0, 0, 0, tz),
				Precision: models.Date,
			},
		},
		Right: &models.Patient{
			DomainResource: models.DomainResource{
				Resource: models.Resource{
					Id:           bson.NewObjectId().Hex(),
					ResourceType: "Patient",
				},
			},
			Gender: "Female",
			Name: []models.HumanName{
				models.HumanName{
					Family: "Smith",
					Given:  []string{"Jane"},
				},
			},
			BirthDate: &models.FHIRDateTime{
				Time:      time.Date(1976, 11, 1, 0, 0, 0, 0, tz),
				Precision: models.Date,
			},
		},
	}

	detector := new(Detector)
	targetResource, oo := detector.Conflicts(match)
	d.NotNil(oo)
	d.NotNil(targetResource)

	// Validate the OperationOutcome first.
	d.Len(oo.Issue, 1)
	d.Len(oo.Issue[0].Location, 4)
	d.Len(oo.Issue[0].Diagnostics, len(bson.NewObjectId().Hex())) // Contains the conflictID

	expected := []string{"id", "gender", "birthDate", "name[0].given[0]"}
	for _, x := range expected {
		d.True(contains(oo.Issue[0].Location, x))
	}

	// Validate the target resource. Should be the same as the left.
	targetPatient, ok := targetResource.(*models.Patient)
	d.True(ok)
	left, ok := match.Left.(*models.Patient)
	d.True(ok)

	d.Equal(left.Gender, targetPatient.Gender)
	d.Equal(left.Name[0].Given[0], targetPatient.Name[0].Given[0])
	d.Equal(left.Name[0].Family, targetPatient.Name[0].Family)
	d.True(left.BirthDate.Time.Equal(targetPatient.BirthDate.Time))
}

// ========================================================================= //
// TEST FIND CONFLICT PATHS                                                  //
// ========================================================================= //

func (d *DetectorTestSuite) TestFindConflictsPatientResource() {

	match := &Match{
		ResourceType: "Patient",
		Left: &models.Patient{
			DomainResource: models.DomainResource{
				Resource: models.Resource{
					Id:           bson.NewObjectId().Hex(),
					ResourceType: "Patient",
				},
			},
			Gender: "Male",
			Name: []models.HumanName{
				models.HumanName{
					Family: "Smith",
					Given:  []string{"John"},
				},
			},
			BirthDate: &models.FHIRDateTime{
				Time:      time.Date(1976, time.December, 2, 0, 0, 0, 0, time.UTC),
				Precision: models.Date,
			},
		},
		Right: &models.Patient{
			DomainResource: models.DomainResource{
				Resource: models.Resource{
					Id:           bson.NewObjectId().Hex(),
					ResourceType: "Patient",
				},
			},
			Gender: "Female",
			Name: []models.HumanName{
				models.HumanName{
					Family: "Smith",
					Given:  []string{"Jane"},
				},
			},
			BirthDate: &models.FHIRDateTime{
				Time:      time.Date(1976, time.November, 1, 0, 0, 0, 0, time.UTC),
				Precision: models.Date,
			},
		},
	}

	detector := new(Detector)
	conflicts := detector.findConflictPaths(match)

	expected := []string{
		"id",
		"gender",
		"name[0].given[0]",
		"birthDate",
	}
	d.Len(conflicts, 4)
	for _, p := range expected {
		d.True(contains(conflicts, p))
	}
}

type BarType struct {
	A string               `json:"a,omitempty"`
	B *uint32              `json:"b,omitempty"`
	C *models.FHIRDateTime `json:"c,omitempty"`
	D *BazType             `json:"d,omitempty"`
}

type BazType struct {
	X string `json:"x,omitempty"`
	Y *bool  `json:"y,omitempty"`
}

func (d *DetectorTestSuite) TestFindConflictsNoConflicts() {
	leftNum := uint32(1)
	rightNum := uint32(1)
	leftBool := false
	rightBool := false
	t1 := &models.FHIRDateTime{
		Time:      time.Date(1845, 6, 4, 12, 33, 0, 0, time.UTC),
		Precision: models.Timestamp,
	}
	t2 := &models.FHIRDateTime{
		Time:      time.Date(1845, 6, 4, 12, 33, 0, 0, time.UTC),
		Precision: models.Timestamp,
	}

	match := &Match{
		ResourceType: "BarType",
		Left: &BarType{
			A: "A",
			B: &leftNum,
			C: t1,
			D: &BazType{
				X: "X",
				Y: &leftBool,
			},
		},
		Right: &BarType{
			A: "A",
			B: &rightNum,
			C: t2,
			D: &BazType{
				X: "X",
				Y: &rightBool,
			},
		},
	}

	detector := new(Detector)
	conflicts := detector.findConflictPaths(match)

	// A perfect match has no conflicts.
	d.Len(conflicts, 0)
}

func (d *DetectorTestSuite) TestFindConflictsSomeConflicts() {
	leftNum := uint32(1)
	rightNum := uint32(2)
	leftBool := false
	rightBool := true

	tz, err := time.LoadLocation("America/New_York")
	d.NoError(err)

	t1 := &models.FHIRDateTime{
		Time:      time.Date(1994, time.January, 6, 12, 58, 00, 00, tz),
		Precision: models.Timestamp,
	}
	t2 := &models.FHIRDateTime{
		Time:      time.Date(1994, time.January, 6, 12, 58, 00, 00, tz),
		Precision: models.Timestamp,
	}

	match := &Match{
		ResourceType: "BarType",
		Left: &BarType{
			A: "A",
			B: &leftNum,
			C: t1,
			D: &BazType{
				X: "Y",
				Y: &leftBool,
			},
		},
		Right: &BarType{
			A: "B",
			B: &rightNum,
			C: t2,
			D: &BazType{
				X: "X",
				Y: &rightBool,
			},
		},
	}

	detector := new(Detector)
	conflicts := detector.findConflictPaths(match)
	d.Len(conflicts, 4)
}

func (d *DetectorTestSuite) TestFindConflictsUncommonPathsSomeConflicts() {
	leftNum := uint32(1)
	rightNum := uint32(1)
	rightBool := false
	t := &models.FHIRDateTime{
		Time:      time.Now().UTC(),
		Precision: models.Timestamp,
	}

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
	conflicts := detector.findConflictPaths(match)

	// Should be 3 locations: left a, right d.x, right d.y
	d.Len(conflicts, 3)

	expectedPaths := []string{"a", "d.x", "d.y"}
	for _, loc := range conflicts {
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

	t := time.Now()
	t1 := models.FHIRDateTime{
		Time:      t,
		Precision: models.Timestamp,
	}
	t2 := models.FHIRDateTime{
		Time:      t,
		Precision: models.Timestamp,
	}
	rt.True(d.compareValues(reflect.ValueOf(t1), reflect.ValueOf(t2)))

	t3 := models.FHIRDateTime{
		Time:      time.Now(),
		Precision: models.Timestamp,
	}
	rt.False(d.compareValues(reflect.ValueOf(t1), reflect.ValueOf(t3)))
}

func (rt *ResourceTraversalTestSuite) TestCompareDateValues() {
	d := new(Detector)

	d1 := models.FHIRDateTime{
		Time:      time.Date(1976, 12, 1, 0, 0, 0, 0, time.UTC),
		Precision: models.Date,
	}

	d2 := models.FHIRDateTime{
		Time:      time.Date(1976, 11, 1, 0, 0, 0, 0, time.UTC),
		Precision: models.Date,
	}
	rt.True(d.compareValues(reflect.ValueOf(d1), reflect.ValueOf(d1)))
	rt.False(d.compareValues(reflect.ValueOf(d1), reflect.ValueOf(d2)))
}

func (rt *ResourceTraversalTestSuite) TestCompareDifferentKindsAlwaysFalse() {
	d := new(Detector)

	v1 := reflect.ValueOf("foo")
	v2 := reflect.ValueOf(3)
	rt.False(d.compareValues(v1, v2))
}
