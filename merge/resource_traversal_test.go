package merge

import (
	"reflect"
	"testing"
	"time"

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
	deceased := true
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
			Time:      time.Date(1946, 9, 5, 12, 0, 0, 0, time.UTC),
			Precision: models.Date,
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

	patientValue := reflect.ValueOf(patient)
	pathmap := make(PathMap)
	traverse(patientValue, pathmap, "")

	// Name
	family, ok := pathmap["name[0].family"]
	rt.True(ok)
	rt.Equal("Mercury", family.String())
	given, ok := pathmap["name[0].given[0]"]
	rt.True(ok)
	rt.Equal("Freddy", given.String())

	// Gender
	gender, ok := pathmap["gender"]
	rt.True(ok)
	rt.Equal("Male", gender.String())

	// Birthdate
	birthDate, ok := pathmap["birthDate"]
	rt.True(ok)
	asTime, ok := birthDate.Interface().(models.FHIRDateTime)
	rt.True(ok)
	expectedTime := models.FHIRDateTime{
		Time:      time.Date(1946, 9, 5, 12, 0, 0, 0, time.UTC),
		Precision: models.Timestamp,
	}
	rt.True(expectedTime.Time.Equal(asTime.Time))

	// Deceased
	deceasedBoolean, ok := pathmap["deceasedBoolean"]
	rt.True(ok)
	rt.Equal(true, deceasedBoolean.Bool())

	// Address
	line, ok := pathmap["address[0].line[0]"]
	rt.True(ok)
	rt.Equal("1 London Way", line.String())
	city, ok := pathmap["address[0].city"]
	rt.True(ok)
	rt.Equal("London", city.String())
	country, ok := pathmap["address[0].country"]
	rt.True(ok)
	rt.Equal("UK", country.String())

	// Marital status
	system, ok := pathmap["maritalStatus.coding[0].system"]
	rt.True(ok)
	rt.Equal("http://hl7.org/fhir/v3/MaritalStatus", system.String())
	code, ok := pathmap["maritalStatus.coding[0].code"]
	rt.True(ok)
	rt.Equal("M", code.String())
	display, ok := pathmap["maritalStatus.coding[0].display"]
	rt.True(ok)
	rt.Equal("Married", display.String())

	// Contained resource
	value, ok := pathmap["contained[0].value"]
	rt.True(ok)
	rt.Equal(float64(0), value.Float())
	unit, ok := pathmap["contained[0].unit"]
	rt.True(ok)
	rt.Equal("Songs", unit.String())
}
