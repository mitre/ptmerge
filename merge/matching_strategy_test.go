package merge

import (
	"testing"

	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type MatchingTestSuite struct {
	testutil.MongoSuite
}

func TestMatchingTestSuite(t *testing.T) {
	suite.Run(t, new(MatchingTestSuite))
}

func (m *MatchingTestSuite) TestPatientMatchingNoMatch() {
	source1, err := testutil.LoadFixture("Patient", "../fixtures/patients/bernard_johnston_patient.json")
	m.NoError(err)
	source2, err := testutil.LoadFixture("Patient", "../fixtures/patients/matilda_flatley_patient.json")
	m.NoError(err)

	strategy := new(PatientMatchingStrategy)
	result, err := strategy.Match(source1, source2)
	m.NoError(err)
	m.True(result)
}

func (m *MatchingTestSuite) TestPatientMatchingMatch() {
	source1, err := testutil.LoadFixture("Patient", "../fixtures/patients/bernard_johnston_patient.json")
	m.NoError(err)
	source2, err := testutil.LoadFixture("Patient", "../fixtures/patients/bernard_johnston_patient.json")
	m.NoError(err)

	strategy := new(PatientMatchingStrategy)
	result, err := strategy.Match(source1, source2)
	m.NoError(err)
	m.True(result)
}

func (m *MatchingTestSuite) TestPatientMatchingPartialMatch() {
	source1, err := testutil.LoadFixture("Patient", "../fixtures/patients/bernard_johnston_patient.json")
	m.NoError(err)
	source2, err := testutil.LoadFixture("Patient", "../fixtures/patients/bernard_johnson_patient.json")
	m.NoError(err)

	strategy := new(PatientMatchingStrategy)
	result, err := strategy.Match(source1, source2)
	m.NoError(err)
	m.True(result)
}

func (m *MatchingTestSuite) TestPatientMatchingWrongResourceType() {
	source1, err := testutil.LoadFixture("Encounter", "../fixtures/encounters/encounter1.json")
	m.NoError(err)
	source2, err := testutil.LoadFixture("Encounter", "../fixtures/encounters/encounter1.json")
	m.NoError(err)

	strategy := new(PatientMatchingStrategy)
	result, err := strategy.Match(source1, source2)
	m.Error(err)
	m.False(result)
}
