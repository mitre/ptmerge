package merge

import (
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/intervention-engine/fhir/models"
	"github.com/stretchr/testify/suite"
)

type ResourceTraversalTestSuite struct {
	suite.Suite
}

func TestResourceTraversalTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceTraversalTestSuite))
}

// ========================================================================= //
// TEST STRUCT TRAVERSAL                                                     //
// ========================================================================= //

func (rt *ResourceTraversalTestSuite) TestStructTraversal() {
	deceased := false
	// Zero value pointers and strings are ignored, but zero numeric values should
	// be preserved. The three major "zero" types are covered in this example:
	// 1. nil pointers
	// 2. empty strings
	// 3. numeric values of 0
	patient := &models.Patient{
		Name: []models.HumanName{
			models.HumanName{
				Family: "Mercury",
				Given:  []string{"Freddy"},
			},
		},
		Gender:          "Male",
		DeceasedBoolean: &deceased,
		MaritalStatus: &models.CodeableConcept{
			Coding: []models.Coding{
				models.Coding{
					System:  "http://hl7.org/fhir/v3/MaritalStatus",
					Code:    "M",
					Display: "Married",
				},
			},
		},
		Address: []models.Address{
			models.Address{
				Line:    []string{"1 London Way"},
				City:    "London",
				Country: "UK",
			},
		},
		BirthDate: &models.FHIRDateTime{
			// We don't traverse into time objects, so there will only be one path here.
			Time: time.Now().UTC(),
		},
	}

	// Adding a contained quantity object to test the handling of zero-value numeric types.
	quantity := float64(0)
	patient.Contained = []interface{}{
		models.Quantity{
			Value: &quantity,
			Unit:  "Songs",
		},
	}

	// The order is deterministic. We expect these fields to be non-nil
	// in the JSON equivalent of this patient object.
	expected := []string{
		"contained[0].value",
		"contained[0].unit",
		"name[0].family",
		"name[0].given[0]",
		"gender",
		"birthDate",
		"deceasedBoolean",
		"address[0].line[0]",
		"address[0].city",
		"address[0].country",
		"maritalStatus.coding[0].system",
		"maritalStatus.coding[0].code",
		"maritalStatus.coding[0].display",
	}
	value := reflect.ValueOf(*patient)
	pathmap := make(PathMap)
	traverse(value, pathmap, "")

	spew.Dump(pathmap.Keys())

	// The order of the keys is not deterministic, so we need to use contains().
	for _, k := range pathmap.Keys() {
		rt.True(contains(expected, k))
	}
}

// ========================================================================= //
// TEST REFLECTION VALUE COMPARISON                                          //
// ========================================================================= //

func (rt *ResourceTraversalTestSuite) TestCompareStringValues() {
	v1 := reflect.ValueOf("hello")
	v2 := reflect.ValueOf("hello")
	rt.True(compareValues(v1, v2))

	v3 := reflect.ValueOf("world")
	rt.False(compareValues(v1, v3))
}

func (rt *ResourceTraversalTestSuite) TestCompareIntegerValues() {
	i1 := reflect.ValueOf(uint32(2))
	i2 := reflect.ValueOf(uint32(2))
	rt.True(compareValues(i1, i2))

	i3 := reflect.ValueOf(uint32(0))
	rt.False(compareValues(i1, i3))

	i4 := reflect.ValueOf(uint32(0))
	rt.True(compareValues(i3, i4))
}

func (rt *ResourceTraversalTestSuite) TestCompareFloatValues() {
	i1 := reflect.ValueOf(float64(5.2))
	i2 := reflect.ValueOf(float64(5.2))
	rt.True(compareValues(i1, i2))

	i3 := reflect.ValueOf(float64(0))
	rt.False(compareValues(i1, i3))

	i4 := reflect.ValueOf(float64(0))
	rt.True(compareValues(i3, i4))
}

func (rt *ResourceTraversalTestSuite) TestCompareBooleanValues() {
	v1 := reflect.ValueOf(false)
	v2 := reflect.ValueOf(false)
	rt.True(compareValues(v1, v2))

	v3 := reflect.ValueOf(true)
	rt.False(compareValues(v1, v3))
}

func (rt *ResourceTraversalTestSuite) TestCompareTimeValues() {
	t := time.Now().UTC()
	t1 := reflect.ValueOf(t)
	t2 := reflect.ValueOf(t)
	rt.True(compareValues(t1, t2))

	t3 := reflect.ValueOf(time.Now().UTC())
	rt.False(compareValues(t1, t3))
}

func (rt *ResourceTraversalTestSuite) TestCompareDifferentKindsAlwaysFalse() {
	v1 := reflect.ValueOf("foo")
	v2 := reflect.ValueOf(3)
	rt.False(compareValues(v1, v2))
}
