package merge

import (
	"reflect"
	"testing"
	"time"

	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/fhirutil"
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
	fix, err := fhirutil.LoadResource("Patient", "../fixtures/patients/lowell_abbott.json")
	rt.NoError(err)
	patient, ok := fix.(*models.Patient)
	rt.True(ok)
	patientValue := reflect.ValueOf(patient)
	pathmap := make(PathMap)
	traverse(patientValue, pathmap, "")

	// Name
	family, ok := pathmap["name[0].family"]
	rt.True(ok)
	rt.Equal("Abbott", family.String())
	given, ok := pathmap["name[0].given[0]"]
	rt.True(ok)
	rt.Equal("Lowell", given.String())

	// Gender
	gender, ok := pathmap["gender"]
	rt.True(ok)
	rt.Equal("male", gender.String())

	// Birthdate
	birthDate, ok := pathmap["birthDate"]
	rt.True(ok)
	asTime, ok := birthDate.Interface().(models.FHIRDateTime)
	rt.True(ok)

	expectedTime := models.FHIRDateTime{
		Time:      time.Date(1950, 9, 2, 16, 9, 32, 0, time.UTC),
		Precision: models.Timestamp,
	}
	rt.Equal(expectedTime, asTime)

	// Address
	line, ok := pathmap["address[0].line[0]"]
	rt.True(ok)
	rt.Equal("1 MITRE Way", line.String())
	city, ok := pathmap["address[0].city"]
	rt.True(ok)
	rt.Equal("Bedford", city.String())
	country, ok := pathmap["address[0].state"]
	rt.True(ok)
	rt.Equal("MA", country.String())

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
}
