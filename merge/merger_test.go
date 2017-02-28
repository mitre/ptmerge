package merge

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/intervention-engine/fhir/server"
	"github.com/mitre/ptmerge/fhirutil"
	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type MergerTestSuite struct {
	testutil.MongoSuite
	FHIRServer *httptest.Server
}

func TestMergerTestSuite(t *testing.T) {
	suite.Run(t, new(MergerTestSuite))
}

func (m *MergerTestSuite) SetupSuite() {
	// Set gin to release mode (less verbose output).
	gin.SetMode(gin.ReleaseMode)

	// Create a mock FHIR server to check the ptmerge service's outgoing requests. The first
	// call to s.DB() stands up the mock Mongo server, see testutil/mongo_suite.go for more.
	fhirEngine := gin.New()
	ms := server.NewMasterSession(m.DB().Session, "ptmerge-test")
	server.RegisterRoutes(fhirEngine, nil, server.NewMongoDataAccessLayer(ms, nil, true), server.Config{})
	m.FHIRServer = httptest.NewServer(fhirEngine)
}

func (m *MergerTestSuite) TearDownSuite() {
	m.FHIRServer.Close()
	// Clean up and remove all temporary files from the mocked database.
	// See testutil/mongo_suite.go for more.
	m.TearDownDBServer()
}

func (m *MergerTestSuite) TearDownTest() {
	// Cleanup any saved merge states.
	m.DB().C("merges").DropCollection()
}

// ========================================================================= //
// TEST MERGE                                                                //
// ========================================================================= //

func (m *MergerTestSuite) TestMergePerfectMatch() {
	var err error

	// Two identical bundles should result in a merge without conflicts, just returning
	// the merged bundle.
	created, err := fhirutil.LoadAndPostResource(m.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	m.True(ok)

	created, err = fhirutil.LoadAndPostResource(m.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	rightBundle, ok := created.(*models.Bundle)
	m.True(ok)

	merger := NewMerger(m.FHIRServer.URL)
	source1 := m.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := m.FHIRServer.URL + "/Bundle/" + rightBundle.Id
	outcome, targetURL, err := merger.Merge(source1, source2)
	m.NoError(err)
	m.NotNil(outcome)
	m.Empty(targetURL) // No target was created

	// The outcome should be a bundle containing the merged resources.
	m.Len(outcome.Entry, 7)
}

func (m *MergerTestSuite) TestMergePartialMatch() {
	// The outcome should be a set of conflicts.
	created, err := fhirutil.LoadAndPostResource(m.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	m.True(ok)

	created2, err := fhirutil.LoadAndPostResource(m.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_unmarried_bundle.json")
	m.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	m.True(ok)

	merger := NewMerger(m.FHIRServer.URL)
	source1 := m.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := m.FHIRServer.URL + "/Bundle/" + rightBundle.Id

	outcome, targetURL, err := merger.Merge(source1, source2)
	m.NoError(err)
	m.NotNil(outcome)
	m.NotEmpty(targetURL)

	// Check that the target bundle exists and contains the expected resources.
	target, err := fhirutil.GetResourceByURL("Bundle", targetURL)
	m.NoError(err)
	targetBundle, ok := target.(*models.Bundle)
	m.True(ok)
	m.Len(targetBundle.Entry, 7)

	// There should be one Patient.
	pcount := 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Patient" {
			pcount++
		}
	}
	m.Equal(1, pcount)

	// There should also be 2 Encounters.
	ecount := 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Encounter" {
			ecount++
		}
	}
	m.Equal(2, ecount)

	// And 1 Procedure.
	pcount = 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Procedure" {
			pcount++
		}
	}
	m.Equal(1, pcount)

	// And 2 MedicationStatements.
	mcount := 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "MedicationStatement" {
			mcount++
		}
	}
	m.Equal(2, mcount)

	// Validate the bundle of OperationOutcomes. There should be 2 conflicts:
	// 2 paths in the Patient resource and 2 paths in an Encounter resource.
	m.Len(outcome.Entry, 2)
	for _, entry := range outcome.Entry {
		oo, ok := entry.Resource.(*models.OperationOutcome)
		m.True(ok)

		m.Len(oo.Issue, 1)
		issue := oo.Issue[0]
		m.Equal("information", issue.Severity)
		m.Equal("conflict", issue.Code)
		m.Len(issue.Location, 2)
		m.NotEmpty(issue.Diagnostics)

		// Validate the Patient conflicts.
		if strings.Contains(issue.Diagnostics, "Patient") {
			// Reference to the new Patient resource in the target bundle.
			m.Len(issue.Diagnostics, len("Patient:"+bson.NewObjectId().Hex()))

			for _, loc := range issue.Location {
				m.True(contains([]string{"maritalStatus.coding[0].display", "maritalStatus.coding[0].code"}, loc))
			}
			continue
		}

		// Validate the Encounter conflicts.
		if strings.Contains(issue.Diagnostics, "Encounter") {
			// Reference to the new Encounter resource in the target bundle.
			m.Len(issue.Diagnostics, len("Encounter:"+bson.NewObjectId().Hex()))

			for _, loc := range issue.Location {
				m.True(contains([]string{"period.start", "period.end"}, loc))
			}
			continue
		}
	}
}

func (m *MergerTestSuite) TestMergePoorMatch() {
	// Minimally the Patient resource matches, but everything else doesn't.
	created, err := fhirutil.LoadAndPostResource(m.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	m.True(ok)

	created2, err := fhirutil.LoadAndPostResource(m.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_jr_bundle.json")
	m.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	m.True(ok)

	merger := NewMerger(m.FHIRServer.URL)
	source1 := m.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := m.FHIRServer.URL + "/Bundle/" + rightBundle.Id

	outcome, targetURL, err := merger.Merge(source1, source2)
	m.NoError(err)
	m.NotNil(outcome)
	m.NotEmpty(targetURL)

	// The outcome should be one set of conflicts for the matched Patient resource.
	m.Len(outcome.Entry, 1)
	oo, ok := outcome.Entry[0].Resource.(*models.OperationOutcome)
	m.True(ok)
	m.Len(oo.Issue, 1)

	issue := oo.Issue[0]
	m.Len(issue.Location, 7)
	for _, loc := range issue.Location {
		m.True(contains(
			[]string{
				"id",
				"birthDate",
				"address[0].line[0]",
				"telecom[0].use",
				"telecom[0].system",
				"telecom[0].value",
				"name[0].suffix[0]",
			},
			loc,
		))
	}
	m.Len(issue.Diagnostics, len("Patient:"+bson.NewObjectId().Hex()))

	// The target should exist and contain one Patient, plus everything else.
	target, err := fhirutil.GetResourceByURL("Bundle", targetURL)
	m.NoError(err)
	targetBundle, ok := target.(*models.Bundle)
	m.True(ok)
	m.Len(targetBundle.Entry, 11)

	// There should be one Patient.
	pcount := 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Patient" {
			pcount++
		}
	}
	m.Equal(1, pcount)

	// There should also be 3 Encounters.
	ecount := 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Encounter" {
			ecount++
		}
	}
	m.Equal(3, ecount)

	// And 2 Procedures.
	pcount = 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Procedure" {
			pcount++
		}
	}
	m.Equal(2, pcount)

	// And 3 MedicationStatements.
	mcount := 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "MedicationStatement" {
			mcount++
		}
	}
	m.Equal(3, mcount)
}

func (m *MergerTestSuite) TestGodawfulMatch() {
	// A match so bad, the Patient resource doesn't even match. In this case
	// we return an error since the target would end up with 2 Patient
	// resource in it.
	created, err := fhirutil.LoadAndPostResource(m.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	m.True(ok)

	created2, err := fhirutil.LoadAndPostResource(m.FHIRServer.URL, "Bundle", "../fixtures/bundles/joey_chestnut_bundle.json")
	m.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	m.True(ok)

	merger := NewMerger(m.FHIRServer.URL)
	source1 := m.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := m.FHIRServer.URL + "/Bundle/" + rightBundle.Id

	outcome, targetURL, err := merger.Merge(source1, source2)
	m.Error(err)
	m.Nil(outcome)
	m.Empty(targetURL)

	m.Equal(errors.New("Patient resource(s) do not match"), err)
}

// ========================================================================= //
// TEST RESOLVE CONFLICT                                                     //
// ========================================================================= //

func (m *MergerTestSuite) TestResolveConflict() {
	m.T().Skip()
}
