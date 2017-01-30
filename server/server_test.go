package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

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
	patientIDs    []string
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

	// POST test fixtures (bundles) to the mock FHIR server.
	s.patientIDs = append(s.patientIDs, s.postPatientBundle("../fixtures/john_peters_bundle.json"))
	s.patientIDs = append(s.patientIDs, s.postPatientBundle("../fixtures/clint_abbot_bundle.json"))

	// Create a mock PTMergeServer.
	ptmergeEngine := gin.New()
	RegisterRoutes(ptmergeEngine, s.DB().Session, s.FHIRServer.URL)
	s.PTMergeServer = httptest.NewServer(ptmergeEngine)
}

// Inserts a fixture into the mock FHIR server.
func (s *ServerTestSuite) postPatientBundle(fixturePath string) string {
	// Load and POST the fixture.
	data, err := os.Open(fixturePath)
	s.NoError(err)
	defer data.Close()
	res, err := http.Post(s.FHIRServer.URL+"/", "application/json", data)
	s.NoError(err)
	defer res.Body.Close()

	// Check the response.
	s.Equal(http.StatusOK, res.StatusCode)
	decoder := json.NewDecoder(res.Body)
	responseBundle := &models.Bundle{}
	err = decoder.Decode(responseBundle)
	s.NoError(err)
	s.Equal("transaction-response", responseBundle.Type)

	// Return the new ID of the Patient resource.
	patient, ok := responseBundle.Entry[0].Resource.(*models.Patient)
	s.True(ok)
	s.Equal("Patient", patient.Resource.ResourceType)
	return patient.Resource.Id
}

func (s *ServerTestSuite) TeardownSuite() {
	s.PTMergeServer.Close()
	s.FHIRServer.Close()
	// Clean up and remove all temporary files from the mocked database.
	// See testutil/mongo_suite.go for more.
	s.TearDownDBServer()
}

func (s *ServerTestSuite) TestInitiateMerge() {
	source1 := s.FHIRServer.URL + "/Patient?_id=" + s.patientIDs[0] + "&_include=*&_revinclude=*"
	source2 := source1
	req := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	res, err := http.Post(req, "", nil)
	defer res.Body.Close()
	s.NoError(err)

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(200, res.StatusCode)
	s.Equal(fmt.Sprintf("Merging records %s and %s", source1, source2), string(body))
}

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
