package server

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type MergeControllerSuite struct {
	suite.Suite
}

func TestMergeControllerSuite(t *testing.T) {
	suite.Run(t, new(MergeControllerSuite))
}

func (m *MergeControllerSuite) TestIsOperationOutcomeBundle() {
	// Returns true for a bundle of conflicts
	conflictBundle := testutil.CreateMockConflictBundle(2)
	m.True(isOperationOutcomeBundle(conflictBundle))

	// Returns false for a patient bundle
	patientBundle := &models.Bundle{}
	data, err := os.Open("../fixtures/clint_abbot_bundle.json")
	m.NoError(err)
	decoder := json.NewDecoder(data)
	err = decoder.Decode(patientBundle)
	m.NoError(err)
	m.False(isOperationOutcomeBundle(patientBundle))
}
