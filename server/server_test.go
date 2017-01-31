package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

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
	s.NoError(err)
	defer mergeResponse.Body.Close()

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
	s.NoError(err)
	defer conflictResponse.Body.Close()

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
	s.Equal(2, len(state.Conflicts.Keys()))
	s.Equal(uint32(2), *conflictBundle.Total)

	for _, entry := range conflictBundle.Entry {
		s.True(entryIsOperationOutcome(entry))
		oo, ok := entry.Resource.(*models.OperationOutcome)
		s.True(ok)
		s.Equal(state.Conflicts[oo.Id], s.FHIRServer.URL+"/OperationOutcome/"+oo.Id)
	}
}

func (s *ServerTestSuite) TestMergeInvalidRequest() {
	// Make the merge request.
	req := s.PTMergeServer.URL + "/merge?foo=bar"
	errResponse, err := http.Post(req, "", nil)
	s.NoError(err)
	defer errResponse.Body.Close()

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
	s.NoError(err)
	defer errResponse.Body.Close()

	// A valid request with resources not found on the specified FHIR server gets a 5xx response.
	s.Equal(500, errResponse.StatusCode)
	body, err := ioutil.ReadAll(errResponse.Body)
	s.Equal(fmt.Sprintf("Resource %s not found", dne1), string(body))
}

// ========================================================================= //
// TEST RESOLVE CONFLICTS                                                    //
// ========================================================================= //

func (s *ServerTestSuite) TestResolveConflictNoMoreConflicts() {
	// Insert fixtures s/t there is only one merge conflict. The conflict in these
	// fixtures is the Given name of the patient: "Clint" != "Chip".
	testBundle1, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/one_conflict/clint_abbot_bundle_1.json")
	s.NoError(err)
	s.NotNil(testBundle1)
	testBundle2, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/one_conflict/clint_abbot_bundle_2.json")
	s.NoError(err)
	s.NotNil(testBundle2)
	oo, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/one_conflict/conflict_oo.json")
	s.NoError(err)
	s.NotNil(oo)

	// The target bundle is bundle #1.
	target, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/one_conflict/clint_abbot_bundle_1.json")

	// Put the merge state in mongo.
	conflictID := oo.Resource.Id
	conflicts := make(ConflictMap)
	conflicts[conflictID] = s.FHIRServer.URL + "/OperationOutcome/" + conflictID
	targetBundle := s.FHIRServer.URL + "/Bundle/" + target.Resource.Id
	mergeID, err := s.insertMergeState(targetBundle, conflicts)
	s.NoError(err)
	s.NotEmpty(mergeID)

	// Resource that resolves the conflict.
	resource := []byte(
		`{
			"resourceType": "Patient",
			"name": [{
				"family": ["Abbott"],
				"given": ["Clint"]
			}],
			"gender": "male",
			"birthDate": "1950-09-02"
		}`)

	// Make the resoultion request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolve/" + conflictID
	res, err := http.Post(req, "application/json", bytes.NewBuffer(resource))
	s.NoError(err)
	defer res.Body.Close()

	// Response should be the complete target bundle, with all conflicts resolved.
	s.Equal(200, res.StatusCode)
	decoder := json.NewDecoder(res.Body)
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
			s.Equal("Clint", patient.Name[0].Given[0])
			s.Equal("Abbott", patient.Name[0].Family[0])
		}
	}

	// Since the merge was completed, check that the state is no longer in mongo.
	var state MergeState
	err = s.DB().C("merges").Find(bson.M{"_id": mergeID}).One(&state)
	s.NotNil(err)
	s.Equal(mgo.ErrNotFound, err)

	// Also check that the target Bundle and conflict OperationOutcome are no longer
	// on the FHIR server.
	res, err = http.Get(targetBundle)
	s.NoError(err)
	s.Equal(404, res.StatusCode)

	res, err = http.Get(conflicts[conflictID])
	s.NoError(err)
	s.Equal(404, res.StatusCode)
}

func (s *ServerTestSuite) TestResolveConflictConflictResolved() {
	// Insert fixtures s/t there are multiple merge conflicts. The conflicts in
	// this case are two Encounters that have different start times.
	testBundle1, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/two_conflicts/john_peters_bundle_1.json")
	s.NoError(err)
	s.NotNil(testBundle1)
	testBundle2, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/two_conflicts/john_peters_bundle_2.json")
	s.NoError(err)
	s.NotNil(testBundle2)
	conflict1, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_1.json")
	s.NoError(err)
	s.NotNil(conflict1)
	conflict2, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_2.json")
	s.NoError(err)
	s.NotNil(conflict2)

	// The target bundle is bundle #1.
	target, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/two_conflicts/john_peters_bundle_1.json")

	// Put the merge state in mongo.
	conflicts := make(ConflictMap)
	conflictID1 := conflict1.Resource.Id
	conflicts[conflictID1] = s.FHIRServer.URL + "/OperationOutcome/" + conflictID1
	conflictID2 := conflict2.Resource.Id
	conflicts[conflictID2] = s.FHIRServer.URL + "/OperationOutcome/" + conflictID2
	targetBundle := s.FHIRServer.URL + "/Bundle/" + target.Resource.Id
	mergeID, err := s.insertMergeState(targetBundle, conflicts)
	s.NoError(err)
	s.NotEmpty(mergeID)

	// Resource that resolves the conflict.
	resource := []byte(
		`{
			"resourceType": "Encounter",
			"status": "finished",
			"type": [{
				"coding": [{
					"system": "http://www.ama-assn.org/go/cpt",
					"code": "99201"
				}],
				"text": "Encounter, Performed: Office Visit (Code List: 2.16.840.1.113883.3.464.1003.101.12.1001)"
			}],
			"patient": {
				"reference": "urn:uuid:61ebe359-bfdc-4613-8bf2-c5e300945f0a"
			},
			"period": {
				"start": "2011-11-01T08:05:00-04:00",
				"end": "2011-11-01T09:00:00-04:00"
			}
		}`)

	// Make the resoultion request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolve/" + conflictID1
	res, err := http.Post(req, "application/json", bytes.NewBuffer(resource))
	s.NoError(err)
	defer res.Body.Close()

	// Response should be the bundle containing the one remaining conflict.
	s.Equal(200, res.StatusCode)
	decoder := json.NewDecoder(res.Body)
	conflictBundle := &models.Bundle{}
	err = decoder.Decode(conflictBundle)
	s.NoError(err)

	// Check the bundle contents.
	s.Equal(1, len(conflictBundle.Entry))
	s.Equal(uint32(1), *conflictBundle.Total)
	s.True(isOperationOutcomeBundle(conflictBundle))

	// Check that mongo was updated.
	var state MergeState
	err = s.DB().C("merges").Find(bson.M{"_id": mergeID}).One(&state)
	s.Nil(err)
	s.Equal(1, len(state.Conflicts))
	s.Equal(conflictID2, state.Conflicts.Keys()[0])

	// Also check that the first conflict's OperationOutcome is no longer on
	// the FHIR server.
	res, err = http.Get(conflicts[conflictID1])
	s.NoError(err)
	s.Equal(404, res.StatusCode)
}

func (s *ServerTestSuite) TestResolveConflictConflictNotResolved() {
	// TODO
}

func (s *ServerTestSuite) TestResolveConflictMergeNotFound() {
	// Insert some merges into mongo so there's something to query against.
	var err error
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	conflicts := ConflictMap{
		cid1: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		cid2: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
	}
	targetBundle := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetBundle, conflicts)
	s.NoError(err)
	targetBundle = s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetBundle, conflicts)
	s.NoError(err)
	targetBundle = s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetBundle, conflicts)
	s.NoError(err)

	// Try to resolve a conflict for a merge (and a conflict) that doesn't exist.
	// MergeID is checked before conflictIDs so the conflictID in this case doesn't matter.
	mergeID := bson.NewObjectId().Hex()
	conflictID := bson.NewObjectId().Hex()
	resource := []byte(
		`{
			"resourceType": "Patient",
			"name": [{
				"family": ["Abbott"],
				"given": ["Clint"]
			}],
			"gender": "male",
			"birthDate": "1950-09-02"
		}`)

	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolve/" + conflictID
	res, err := http.Post(req, "application/json", bytes.NewBuffer(resource))
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(404, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge %s not found", mergeID), string(body))
}

func (s *ServerTestSuite) TestResolveConflictConflictNotFound() {
	// Insert some merges into mongo so there's something to query against.
	var err error
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	conflicts := ConflictMap{
		cid1: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		cid2: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
	}
	targetBundle := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(targetBundle, conflicts)
	s.NoError(err)
	targetBundle = s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetBundle, conflicts)
	s.NoError(err)
	targetBundle = s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetBundle, conflicts)
	s.NoError(err)

	// Try to resolve a conflict for a that doesn't exist (but the merge does).
	conflictID := bson.NewObjectId().Hex()
	resource := []byte(
		`{
			"resourceType": "Patient",
			"name": [{
				"family": ["Abbott"],
				"given": ["Clint"]
			}],
			"gender": "male",
			"birthDate": "1950-09-02"
		}`)

	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolve/" + conflictID
	res, err := http.Post(req, "application/json", bytes.NewBuffer(resource))
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(404, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge conflict %s not found for merge %s", conflictID, mergeID), string(body))
}

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

// ========================================================================= //
// TEST HELPERS                                                              //
// ========================================================================= //

// insertMergeState inserts a MergeState into the test mongo database. This
// helper uses the "ptmerge-test" database only.
func (s *ServerTestSuite) insertMergeState(targetBundle string, conflicts ConflictMap) (mergeID string, err error) {
	mergeID = bson.NewObjectId().Hex()
	mergeState := &MergeState{
		MergeID:      mergeID,
		TargetBundle: targetBundle,
		Conflicts:    conflicts,
	}
	err = s.DB().C("merges").Insert(mergeState)
	if err != nil {
		return "", err
	}
	return mergeID, nil
}
