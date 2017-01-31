package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gopkg.in/mgo.v2/bson"

	"io/ioutil"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/intervention-engine/fhir/server"
	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type ServerTestSuite struct {
	// The MongoSuite from IE provides useful features for starting/stopping
	// a test mongo database, as well as inserting test fixtures.
	testutil.MongoSuite
	PTMergeServer *httptest.Server
	FHIRServer    *httptest.Server
}

func TestServerTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}

func (s *ServerTestSuite) SetupSuite() {
	// Set gin to release mode (less verbose output).
	gin.SetMode(gin.ReleaseMode)

	// Create a mock FHIR server to check the ptmerge service's outgoing requests. The first
	// call to s.DB() stands up the mock Mongo server, see testutil/mongo_suite.go for more.
	fhirEngine := gin.New()
	ms := server.NewMasterSession(s.DB().Session, "ptmerge-test")
	server.RegisterRoutes(fhirEngine, nil, server.NewMongoDataAccessLayer(ms, nil), server.Config{})
	s.FHIRServer = httptest.NewServer(fhirEngine)

	// Create a mock PTMergeServer.
	ptmergeEngine := gin.New()
	RegisterRoutes(ptmergeEngine, s.DB().Session, "ptmerge-test", s.FHIRServer.URL)
	s.PTMergeServer = httptest.NewServer(ptmergeEngine)
}

func (s *ServerTestSuite) TearDownSuite() {
	s.PTMergeServer.Close()
	s.FHIRServer.Close()
	// Clean up and remove all temporary files from the mocked database.
	// See testutil/mongo_suite.go for more.
	s.TearDownDBServer()
}

func (s *ServerTestSuite) TearDownTest() {
	// Cleanup any saved merge states.
	s.DB().C("merges").DropCollection()
}

// ========================================================================= //
// TEST MERGE                                                                //
// ========================================================================= //

func (s *ServerTestSuite) TestMergeNoConflicts() {
	testBundle, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/john_peters_bundle.json")
	s.NoError(err)
	s.NotNil(testBundle)

	// Make the merge request. Merging identical bundles should never result in conflicts.
	source1 := s.FHIRServer.URL + "/Bundle/" + testBundle.Resource.Id
	req := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source1)

	mergeResponse, err := http.Post(req, "", nil)
	defer mergeResponse.Body.Close()
	s.NoError(err)

	s.Equal(200, mergeResponse.StatusCode)
	decoder := json.NewDecoder(mergeResponse.Body)
	mergedBundle := &models.Bundle{}
	err = decoder.Decode(mergedBundle)
	s.NoError(err)

	// It's tedious to validate the WHOLE bundle. Instead, check that it contains the Patient
	// resource that we expect and that the bundle doesn't contain any OperationOutcomes.
	for _, entry := range mergedBundle.Entry {
		s.False(entryIsOperationOutcome(entry))
		patient, ok := entry.Resource.(*models.Patient)
		if ok {
			// This Entry is the patient resource, let's validate it
			s.Equal("John", patient.Name[0].Given[0])
			s.Equal("Peters", patient.Name[0].Family[0])
		}
	}
}

func (s *ServerTestSuite) TestMergeWithConflicts() {
	testBundle1, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/john_peters_bundle.json")
	s.NoError(err)
	s.NotNil(testBundle1)

	testBundle2, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/clint_abbot_bundle.json")
	s.NoError(err)
	s.NotNil(testBundle2)

	// Make the merge request.
	source1 := s.FHIRServer.URL + "/Bundle/" + testBundle1.Resource.Id
	source2 := s.FHIRServer.URL + "/Bundle/" + testBundle2.Resource.Id
	req := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	conflictResponse, err := http.Post(req, "", nil)
	defer conflictResponse.Body.Close()
	s.NoError(err)

	s.Equal(201, conflictResponse.StatusCode)
	decoder := json.NewDecoder(conflictResponse.Body)
	conflictBundle := &models.Bundle{}
	err = decoder.Decode(conflictBundle)
	s.NoError(err)

	// Check that the database now has a record of this merge in progress.
	// There should only be one saved merge state in the database.
	n, err := s.DB().C("merges").Count()
	s.NoError(err)
	s.Equal(1, n)

	var state MergeState
	query := s.DB().C("merges").Find(bson.M{})
	err = query.One(&state)
	s.NoError(err)

	// Check that the "Location" header has the mergeID.
	location, ok := conflictResponse.Header["Location"]
	s.True(ok)
	s.Equal(state.MergeID, location[0])

	// Check that the bundle contains the expected OperationOutcomes.
	// For now these are just mocked up.
	s.Equal(2, len(state.ConflictIDs))
	s.Equal(uint32(2), *conflictBundle.Total)

	for i, entry := range conflictBundle.Entry {
		s.True(entryIsOperationOutcome(entry))
		oo, ok := entry.Resource.(*models.OperationOutcome)
		s.True(ok)
		s.Equal(state.ConflictIDs[i], oo.Id)
	}
}

func (s *ServerTestSuite) TestMergeInvalidRequest() {
	// Make the merge request.
	req := s.PTMergeServer.URL + "/merge?foo=bar"
	errResponse, err := http.Post(req, "", nil)
	defer errResponse.Body.Close()
	s.NoError(err)

	// Malformed requests should get a 4xx response.
	s.Equal(400, errResponse.StatusCode)
	body, err := ioutil.ReadAll(errResponse.Body)
	s.Equal("URL(s) referencing bundles to merge were not provided", string(body))
}

func (s *ServerTestSuite) TestMergeResourcesNotFound() {
	// Make the request using bundles that don't exist.
	dne1 := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	dne2 := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	req := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(dne1) + "&source2=" + url.QueryEscape(dne2)

	errResponse, err := http.Post(req, "", nil)
	defer errResponse.Body.Close()
	s.NoError(err)

	// A valid request with resources not found on the specified FHIR server gets a 5xx response.
	s.Equal(500, errResponse.StatusCode)
	body, err := ioutil.ReadAll(errResponse.Body)
	s.Equal(fmt.Sprintf("Resource %s not found", dne1), string(body))
}

// ========================================================================= //
// TEST RESOLVE CONFLICT(S)                                                  //
// ========================================================================= //

func (s *ServerTestSuite) TestResolveConflict() {
	mergeID := "12345"
	conflictID := "67890"
	resource := []byte(`
	{
		"resourceType": "Patient",
		"id": "61ebe359-bfdc-4613-8bf2-c5e300945f2a",
		"name": [{
			"family": ["Abbott"],
			"given": ["Clint"]
		}],
		"gender": "male",
		"birthDate": "1950-09-02"
	}
	`)

	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolve/" + conflictID

	res, err := http.Post(req, "application/json", bytes.NewBuffer(resource))
	defer res.Body.Close()
	s.NoError(err)

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(200, res.StatusCode)
	s.Equal(fmt.Sprintf("Resolving conflict %s for merge %s", conflictID, mergeID), string(body))
}

// func (s *ServerTestSuite) TestResolveConflictMergeNotFound() {
// 	mergeID := bson.NewObjectId().Hex()
// }

// ========================================================================= //
// TEST ABORT MERGE                                                          //
// ========================================================================= //

func (s *ServerTestSuite) TestAbortMerge() {
	mergeID := "12345"
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/abort"

	res, err := http.Post(req, "", nil)
	defer res.Body.Close()
	s.NoError(err)

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(200, res.StatusCode)
	s.Equal(fmt.Sprintf("Aborting merge %s", mergeID), string(body))
}

// ========================================================================= //
// TEST CONVENIENCE ROUTES                                                   //
// ========================================================================= //

func (s *ServerTestSuite) TestGetConflicts() {
	mergeID := "12345"
	req := s.PTMergeServer.URL + "/merge/" + mergeID

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(200, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge conflicts for merge %s", mergeID), string(body))
}
