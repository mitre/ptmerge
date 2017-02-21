package merge

import (
	"reflect"
	"testing"
	"time"

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
// STRUCT TRAVERSAL                                                          //
// ========================================================================= //

func (d *DetectorTestSuite) TestStructTraversal() {
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
			// We don't traverse into time objects, so there will only be one path here
			Time: time.Now(),
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

	paths := []string{}
	// The order is deterministic. We expect these fields to be non-nil
	// in the JSON equivalent of this patient object.
	expected := []string{
		"contained[0].value",
		"contained[0].unit",
		"name[0].family",
		"name[0].given[0]",
		"gender", "birthDate",
		"deceasedBoolean",
		"address[0].line[0]",
		"address[0].city",
		"address[0].country",
		"maritalStatus.coding[0].system",
		"maritalStatus.coding[0].code",
		"maritalStatus.coding[0].display",
	}
	value := reflect.ValueOf(*patient)
	traverse(&paths, value, "")
	d.Equal(expected, paths)
}
