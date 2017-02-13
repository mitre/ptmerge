package merge

import (
	"testing"

	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type MatchTestSuite struct {
	testutil.MongoSuite
}

func TestMatchTestSuite(t *testing.T) {
	suite.Run(t, new(MatchTestSuite))
}

func (m *MatchTestSuite) TestNoMatch() {
	matcher := new(PatientMatchStrategy)
	source1 := testutil.CreateMockPatientObject("../fixtures/patients/bernard_johnston_patient.json")
	source2 := testutil.CreateMockPatientObject("../fixtures/patients/matilda_flatley_patient.json")
	result, err := matcher.Match(source1, source2)
	m.False(result)
	m.Nil(err)
}

func (m *MatchTestSuite) TestMatch() {
	matcher := new(PatientMatchStrategy)
	source1 := testutil.CreateMockPatientObject("../fixtures/patients/bernard_johnston_patient.json")
	source2 := testutil.CreateMockPatientObject("../fixtures/patients/bernard_johnston_patient.json")
	result, err := matcher.Match(source1, source2)
	m.True(result)
	m.Nil(err)
}
