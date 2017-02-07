package merge

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
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

// ========================================================================= //
// TEST MERGE                                                                //
// ========================================================================= //

func (m *MergerTestSuite) TestMerge() {
	merger := NewMerger(m.FHIRServer.URL)
	source1 := m.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	source2 := m.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	outcome, targetURL, err := merger.Merge(source1, source2)
	m.Nil(outcome)
	m.Equal("", targetURL)
	m.Nil(err)
}

// ========================================================================= //
// TEST RESOLVE CONFLICT                                                     //
// ========================================================================= //

func (m *MergerTestSuite) TestResolveConflict() {
	merger := NewMerger(m.FHIRServer.URL)
	targetURL := m.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	opOutcomeURL := m.FHIRServer.URL + "/OperationOutcome/" + bson.NewObjectId().Hex()
	outcome, err := merger.ResolveConflict(targetURL, opOutcomeURL, nil)
	m.Nil(outcome)
	m.Nil(err)
}

// ========================================================================= //
// TEST THE HELPER FUNCTIONS                                                 //
// ========================================================================= //

func (m *MergerTestSuite) TestGetOperationOutcome() {
	// Testing with an OperationOutcome.
	oo, err := testutil.PostOperationOutcome(m.FHIRServer.URL, "../fixtures/one_conflict/conflict_oo.json")
	m.NoError(err)

	uri := m.FHIRServer.URL + "/OperationOutcome/" + oo.Resource.Id
	resource, err := GetResource("OperationOutcome", uri)
	m.NoError(err)

	// Check that the resource returned is what we expect.
	resOpOutcome, ok := resource.(*models.OperationOutcome)
	m.True(ok)
	m.Equal(oo.Resource.Id, resOpOutcome.Resource.Id)
}

func (m *MergerTestSuite) TestGetBundleResource() {
	// Also testing with a Bundle.
	bundle, err := testutil.PostPatientBundle(m.FHIRServer.URL, "../fixtures/john_peters_bundle.json")
	m.NoError(err)

	uri := m.FHIRServer.URL + "/Bundle/" + bundle.Resource.Id
	resource, err := GetResource("Bundle", uri)
	m.NoError(err)

	// Check that the resource returned is what we expect.
	resBundle, ok := resource.(*models.Bundle)
	m.True(ok)
	m.Len(resBundle.Entry, 19)
}

func (m *MergerTestSuite) TestGetResourceNotFound() {
	// Attempting to get a resource that doesn't exist should throw an error.
	resourceID := bson.NewObjectId().Hex()
	uri := m.FHIRServer.URL + "/Patient/" + resourceID
	_, err := GetResource("Patient", uri)
	m.Error(err)
	m.Equal(fmt.Sprintf("Resource %s not found", uri), err.Error())
}

func (m *MergerTestSuite) TestGetResourceMismatchedType() {
	bundle, err := testutil.PostPatientBundle(m.FHIRServer.URL, "../fixtures/john_peters_bundle.json")
	m.NoError(err)

	uri := m.FHIRServer.URL + "/Bundle/" + bundle.Resource.Id
	_, err = GetResource("Patient", uri) // Patient type does not match the resource
	m.Error(err)
	m.Equal("Expected resourceType to be Patient, instead received Bundle", err.Error())
}

func (m *MergerTestSuite) TestDeleteOperationOutcomeResource() {
	// Testing with an OperationOutcome.
	oo, err := testutil.PostOperationOutcome(m.FHIRServer.URL, "../fixtures/one_conflict/conflict_oo.json")
	m.NoError(err)

	uri := m.FHIRServer.URL + "/OperationOutcome/" + oo.Resource.Id
	err = DeleteResource(uri)
	m.NoError(err)

	// Attempt to get the resource to confirm it's deleted.
	res, err := http.Get(uri)
	m.NoError(err)
	m.Equal(404, res.StatusCode)
}

func (m *MergerTestSuite) TestDeleteBundleResource() {
	// Also testing with a Bundle.
	bundle, err := testutil.PostPatientBundle(m.FHIRServer.URL, "../fixtures/john_peters_bundle.json")
	m.NoError(err)

	uri := m.FHIRServer.URL + "/Bundle/" + bundle.Resource.Id
	err = DeleteResource(uri)
	m.NoError(err)

	// Attempt to get the resource to confirm it's deleted.
	res, err := http.Get(uri)
	m.NoError(err)
	m.Equal(404, res.StatusCode)
}

func (m *MergerTestSuite) TestDeleteResourceNotFound() {
	// Attempting to delete a resource that already doesn't exist will not throw an error.
	resourceID := bson.NewObjectId().Hex()
	uri := m.FHIRServer.URL + "/Patient/" + resourceID
	err := DeleteResource(uri)
	m.NoError(err)
}
