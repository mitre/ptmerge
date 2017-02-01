package merge

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/server"
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
	server.RegisterRoutes(fhirEngine, nil, server.NewMongoDataAccessLayer(ms, nil), server.Config{})
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

func (m *MergerTestSuite) TestMerge() {
	// This test is a shell since the merger is only mocked up right now.
	merger := NewMerger(m.FHIRServer.URL)
	source1 := m.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	source2 := m.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, outcome, err := merger.Merge(source1, source2)
	m.Equal("", mergeID)
	m.Nil(outcome)
	m.NotNil(err)
	m.Equal(fmt.Sprintf("Resource %s not found", source1), err.Error())
}

func (m *MergerTestSuite) TestResolveConflict() {
	// This test is a shell since the merger is only mocked up right now.
	merger := NewMerger(m.FHIRServer.URL)
	outcome, err := merger.ResolveConflict("12345", "67890", nil)
	m.Nil(outcome)
	m.NotNil(err)
	m.Equal("Unknown resource <nil>", err.Error())
}

func (m *MergerTestSuite) TestAbort() {
	// This test is a shell since the merger is only mocked up right now.
	merger := NewMerger(m.FHIRServer.URL)
	err := merger.Abort([]string{})
	m.Nil(err)
}

func (m *MergerTestSuite) TestDeleteOperationOutcomeResource() {
	// Testing with an OperationOutcome
	oo, err := testutil.PostOperationOutcome(m.FHIRServer.URL, "../fixtures/one_conflict/conflict_oo.json")
	m.NoError(err)

	uri := m.FHIRServer.URL + "/OperationOutcome/" + oo.Resource.Id
	err = deleteResource(uri)
	m.NoError(err)

	// Attempt to get the resource to confirm it's deleted
	res, err := http.Get(uri)
	m.NoError(err)
	m.Equal(404, res.StatusCode)
}

func (m *MergerTestSuite) TestDeleteBundleResource() {
	// Also testing with a Bundle
	bundle, err := testutil.PostPatientBundle(m.FHIRServer.URL, "../fixtures/john_peters_bundle.json")
	m.NoError(err)

	uri := m.FHIRServer.URL + "/Bundle/" + bundle.Resource.Id
	err = deleteResource(uri)
	m.NoError(err)

	// Attempt to get the resource to confirm it's deleted
	res, err := http.Get(uri)
	m.NoError(err)
	m.Equal(404, res.StatusCode)
}
