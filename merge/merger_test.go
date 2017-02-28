package merge

import (
	"net/http/httptest"
	"testing"

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
	fix, err := fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	created, err := fhirutil.PostResource(m.FHIRServer.URL, "Bundle", fix)
	leftBundle, ok := created.(*models.Bundle)
	m.True(ok)

	fix, err = fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	created, err = fhirutil.PostResource(m.FHIRServer.URL, "Bundle", fix)
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
	fix, err := fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	m.NoError(err)
	created, err := fhirutil.PostResource(m.FHIRServer.URL, "Bundle", fix)
	leftBundle, ok := created.(*models.Bundle)
	m.True(ok)

	fix2, err := fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_unmarried_bundle.json")
	m.NoError(err)
	created2, err := fhirutil.PostResource(m.FHIRServer.URL, "Bundle", fix2)
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

	// TODO: Validate the bundle of operation outcomes.
}

func (m *MergerTestSuite) TestMergePoorMatch() {
	m.T().Skip()
}

// ========================================================================= //
// TEST RESOLVE CONFLICT                                                     //
// ========================================================================= //

func (m *MergerTestSuite) TestResolveConflict() {
	m.T().Skip()
}
