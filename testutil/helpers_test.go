package testutil

import (
	"testing"

	"github.com/intervention-engine/fhir/models"
	"github.com/stretchr/testify/suite"
)

type TestutilTestSuite struct {
	suite.Suite
}

func TestTestutilTestSuite(t *testing.T) {
	suite.Run(t, new(TestutilTestSuite))
}

func (m *TestutilTestSuite) TestLoadFixture() {
	fix, err := LoadFixture("Patient", "../fixtures/patients/foo_bar.json")
	m.NoError(err)
	patient, ok := fix.(*models.Patient)
	m.True(ok)
	m.Len(patient.Name, 1)
	m.Len(patient.Name[0].Given, 1)
	m.Equal("Foo", patient.Name[0].Given[0])
	m.Equal("Bar", patient.Name[0].Family)
}
