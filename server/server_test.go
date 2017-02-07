package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/intervention-engine/fhir/server"
	"github.com/mitre/ptmerge/state"
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
	// We can't test Merge until that feature is built.
	s.T().Skip()
}

func (s *ServerTestSuite) TestMergeWithConflicts() {
	// We can't test Merge until that feature is built.
	s.T().Skip()
}

func (s *ServerTestSuite) TestMergeInvalidRequest() {
	// Make the merge request.
	req := s.PTMergeServer.URL + "/merge?foo=bar"
	errResponse, err := http.Post(req, "", nil)
	s.NoError(err)
	defer errResponse.Body.Close()

	// Malformed requests should get a 4xx response.
	s.Equal(http.StatusBadRequest, errResponse.StatusCode)
	body, err := ioutil.ReadAll(errResponse.Body)
	s.Equal("URL(s) referencing source bundles were not provided", string(body))
}

func (s *ServerTestSuite) TestMergeResourcesNotFound() {
	// We can't test Merge until that feature is built.
	s.T().Skip()
}

// ========================================================================= //
// TEST RESOLVE CONFLICTS                                                    //
// ========================================================================= //

func (s *ServerTestSuite) TestResolveConflictNoMoreConflicts() {
	// We can't test ResolveConflict until that feature is built.
	s.T().Skip()
}

func (s *ServerTestSuite) TestResolveConflictMoreConflicts() {
	// We can't test ResolveConflict until that feature is built.
	s.T().Skip()
}

func (s *ServerTestSuite) TestResolveConflictMergeNotFound() {
	var err error

	// Insert some merges so there's something to query against.
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	conflicts := state.ConflictMap{
		cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + cid1,
			Resolved: false,
		},
		cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + cid2,
			Resolved: false,
		},
	}
	targetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetURL, conflicts)
	s.NoError(err)

	targetURL = s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetURL, conflicts)
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

	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge %s not found", mergeID), string(body))
}

func (s *ServerTestSuite) TestResolveConflictConflictNotFound() {
	var err error

	// Insert some merges so there's something to query against
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	conflicts := state.ConflictMap{
		cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + cid1,
			Resolved: false,
		},
		cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + cid2,
			Resolved: false,
		},
	}
	targetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(targetURL, conflicts)
	s.NoError(err)

	targetURL = s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetURL, conflicts)
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

	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge conflict %s not found for merge %s", conflictID, mergeID), string(body))
}

func (s *ServerTestSuite) TestResolveConflictConflictAlreadyResolved() {
	var err error

	// Insert some merges so there's something to query against.
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	conflicts := state.ConflictMap{
		cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + cid1,
			Resolved: false,
		},
		cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + cid2,
			Resolved: true,
		},
	}
	targetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetURL, conflicts)
	s.NoError(err)

	targetURL = s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(targetURL, conflicts)
	s.NoError(err)

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

	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolve/" + cid2
	res, err := http.Post(req, "application/json", bytes.NewBuffer(resource))
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusBadRequest, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge conflict %s was already resolved for merge %s", cid2, mergeID), string(body))
}

func (s *ServerTestSuite) TestResolveConflictConflictAlreadyDeleted() {
	var err error

	// In practice this scenario should never happen (a conflict should never be deleted before it
	// is resolved). However, this is still worth checking for.

	// Insert some merges so there's something to query against.
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	conflicts := state.ConflictMap{
		cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + cid1,
			Resolved: true,
			Deleted:  true,
		},
		cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + cid2,
			Resolved: false,
			Deleted:  true,
		},
	}
	targetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetURL, conflicts)
	s.NoError(err)

	targetURL = s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(targetURL, conflicts)
	s.NoError(err)

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

	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolve/" + cid2
	res, err := http.Post(req, "application/json", bytes.NewBuffer(resource))
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusBadRequest, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge conflict %s was already resolved and deleted for merge %s", cid2, mergeID), string(body))
}

// ========================================================================= //
// TEST ABORT MERGE                                                          //
// ========================================================================= //

func (s *ServerTestSuite) TestAbortMerge() {
	// Put a target bundle and a conflict on the host FHIR server.
	conflict, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_2.json")
	s.NoError(err)
	s.NotNil(conflict)

	target, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/two_conflicts/john_peters_bundle_1.json")

	// Put the merge state in mongo.
	conflicts := make(state.ConflictMap)
	conflictID := conflict.Resource.Id
	conflicts[conflictID] = &state.ConflictState{
		URL:      s.FHIRServer.URL + "/OperationOutcome/" + conflictID,
		Resolved: false,
	}
	targetURL := s.FHIRServer.URL + "/Bundle/" + target.Resource.Id
	mergeID, err := s.insertMergeState(targetURL, conflicts)
	s.NoError(err)
	s.NotEmpty(mergeID)

	// Make the request.
	res, err := http.Post(s.PTMergeServer.URL+"/merge/"+mergeID+"/abort", "", nil)

	// Check the response. There should be no response body.
	s.Equal(http.StatusNoContent, res.StatusCode)
}

func (s *ServerTestSuite) TestAbortMergeMergeNotFound() {
	var err error

	// Insert some merges so there's something to query against.
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	conflicts := state.ConflictMap{
		cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + cid1,
			Resolved: false,
		},
		cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + cid2,
			Resolved: false,
		},
	}
	targetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetURL, conflicts)
	s.NoError(err)

	targetURL = s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	_, err = s.insertMergeState(targetURL, conflicts)
	s.NoError(err)

	// Try to abort a merge that doesn't exist.
	mergeID := bson.NewObjectId().Hex()
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/abort"
	res, err := http.Post(req, "", nil)
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge %s not found", mergeID), string(body))
}

// ========================================================================= //
// TEST ALL MERGES                                                           //
// ========================================================================= //

func (s *ServerTestSuite) TestAllMerges() {
	var err error

	// Merge 1 metadata.
	m1cid1 := bson.NewObjectId().Hex()
	m1cid2 := bson.NewObjectId().Hex()
	m1targetID := bson.NewObjectId().Hex()

	conflicts := state.ConflictMap{
		m1cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid1,
			Resolved: false,
		},
		m1cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid2,
			Resolved: false,
		},
	}
	m1TargetURL := s.FHIRServer.URL + "/Bundle/" + m1targetID
	m1mergeID, err := s.insertMergeState(m1TargetURL, conflicts)
	s.NoError(err)

	// Merge 2 metadata.
	m2cid1 := bson.NewObjectId().Hex()
	m2cid2 := bson.NewObjectId().Hex()
	m2targetID := bson.NewObjectId().Hex()

	conflicts = state.ConflictMap{
		m2cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m2cid1,
			Resolved: false,
		},
		m2cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m2cid2,
			Resolved: true,
		},
	}
	m2TargetURL := s.FHIRServer.URL + "/Bundle/" + m2targetID
	m2mergeID, err := s.insertMergeState(m2TargetURL, conflicts)
	s.NoError(err)

	// Merge 3 metadata.
	m3cid1 := bson.NewObjectId().Hex()
	m3cid2 := bson.NewObjectId().Hex()
	m3targetID := bson.NewObjectId().Hex()

	conflicts = state.ConflictMap{
		m3cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m3cid1,
			Resolved: true,
			Deleted:  true,
		},
		m3cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m3cid2,
			Resolved: true,
			Deleted:  true,
		},
	}
	m3TargetURL := s.FHIRServer.URL + "/Bundle/" + m3targetID
	m3mergeID, err := s.insertMergeState(m3TargetURL, conflicts)
	err = s.DB().C("merges").Update(
		bson.M{"_id": m3mergeID},
		bson.M{"$set": bson.M{"completed": true}},
	)
	s.NoError(err)

	// Make the request.
	req := s.PTMergeServer.URL + "/merge"
	res, err := http.Get(req)

	s.NoError(err)
	s.Equal(http.StatusOK, res.StatusCode)

	// Umnarshal the response body.
	metadata := state.Merges{}
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &metadata)
	s.NoError(err)

	s.NotEmpty(metadata.Timestamp)
	s.Len(metadata.Merges, 3)

	// Validate the first merge.
	merge1 := metadata.Merges[0]
	s.Equal(m1mergeID, merge1.MergeID)
	s.False(merge1.Completed)
	s.Equal(m1TargetURL, merge1.TargetURL)
	s.Len(merge1.Conflicts, 2)

	// Just validate one of the conflicts. If it was sent correctly, the rest were too.
	conflict, ok := merge1.Conflicts[m1cid1]
	s.True(ok)
	s.Equal(s.FHIRServer.URL+"/OperationOutcome/"+m1cid1, conflict.URL)
	s.False(conflict.Resolved)
	s.False(conflict.Deleted)

	// Validate the second merge.
	merge2 := metadata.Merges[1]
	s.Equal(m2mergeID, merge2.MergeID)
	s.False(merge2.Completed)
	s.Equal(m2TargetURL, merge2.TargetURL)
	s.Len(merge2.Conflicts, 2)

	// Validate the third merge.
	merge3 := metadata.Merges[2]
	s.Equal(m3mergeID, merge3.MergeID)
	s.True(merge3.Completed)
	s.Len(merge3.Conflicts, 2)

	s.True(merge3.Conflicts[m3cid1].Resolved)
	s.True(merge3.Conflicts[m3cid1].Deleted)

	s.True(merge3.Conflicts[m3cid2].Resolved)
	s.True(merge3.Conflicts[m3cid2].Deleted)
}

func (s *ServerTestSuite) TestAllMergesNoMerges() {
	// Make the request.
	req := s.PTMergeServer.URL + "/merge"
	res, err := http.Get(req)

	s.NoError(err)
	s.Equal(http.StatusOK, res.StatusCode)

	// Umnarshal the response body.
	metadata := state.Merges{}
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &metadata)
	s.NoError(err)

	s.NotEmpty(metadata.Timestamp)
	s.Len(metadata.Merges, 0)
}

// ========================================================================= //
// TEST GET MERGE                                                            //
// ========================================================================= //

func (s *ServerTestSuite) TestGetMerge() {
	var err error

	// Merge metadata.
	m1cid1 := bson.NewObjectId().Hex()
	m1cid2 := bson.NewObjectId().Hex()
	m1targetID := bson.NewObjectId().Hex()

	conflicts := state.ConflictMap{
		m1cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid1,
			Resolved: false,
		},
		m1cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid2,
			Resolved: true,
		},
	}
	m1TargetURL := s.FHIRServer.URL + "/Bundle/" + m1targetID
	mergeID, err := s.insertMergeState(m1TargetURL, conflicts)
	s.NoError(err)

	// Make the request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID
	res, err := http.Get(req)

	s.NoError(err)
	s.Equal(http.StatusOK, res.StatusCode)

	// Umnarshal the response body.
	metadata := state.Merge{}
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &metadata)
	s.NoError(err)

	s.NotEmpty(metadata.Timestamp)
	merge := metadata.Merge

	s.Equal(mergeID, merge.MergeID)
	s.Equal(m1TargetURL, merge.TargetURL)
	s.Len(merge.Conflicts, 2)

	conflict1, ok := merge.Conflicts[m1cid1]
	s.True(ok)
	s.Equal(s.FHIRServer.URL+"/OperationOutcome/"+m1cid1, conflict1.URL)
	s.False(conflict1.Resolved)

	conflict2, ok := merge.Conflicts[m1cid2]
	s.True(ok)
	s.Equal(s.FHIRServer.URL+"/OperationOutcome/"+m1cid2, conflict2.URL)
	s.True(conflict2.Resolved)
}

func (s *ServerTestSuite) TestGetMergeNotFound() {
	// Make the request.
	req := s.PTMergeServer.URL + "/merge/" + bson.NewObjectId().Hex()
	res, err := http.Get(req)

	s.NoError(err)
	s.Equal(http.StatusNotFound, res.StatusCode)
}

// ========================================================================= //
// TEST GET REMAINING CONFLICTS                                              //
// ========================================================================= //

func (s *ServerTestSuite) TestGetRemainingConflicts() {
	// Create a merge with 2 conflicts, one resolved.
	conflict1, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_1.json")
	s.NoError(err)
	s.NotNil(conflict1)

	conflict2, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_2.json")
	s.NoError(err)
	s.NotNil(conflict2)

	// Put the merge state in mongo.
	m1cid1 := conflict1.Resource.Id
	m1cid2 := conflict2.Resource.Id

	conflicts := state.ConflictMap{
		m1cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid1,
			Resolved: false,
		},
		m1cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid2,
			Resolved: true, // This conflict is resolved
		},
	}
	m1TargetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(m1TargetURL, conflicts)
	s.NoError(err)

	// Make the request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/conflicts"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusOK, res.StatusCode)
	decoder := json.NewDecoder(res.Body)
	ooBundle := &models.Bundle{}
	err = decoder.Decode(ooBundle)
	s.NoError(err)

	s.Equal("transaction-response", ooBundle.Type)
	s.Equal(uint32(1), *ooBundle.Total)
	s.Len(ooBundle.Entry, 1)

	// Validate the one remaining conflict.
	entry := ooBundle.Entry[0]
	conflict, ok := entry.Resource.(*models.OperationOutcome)
	s.True(ok)
	s.Equal(m1cid1, conflict.Resource.Id)
}

func (s *ServerTestSuite) TestGetRemainingConflictsNoneRemaining() {
	// Create a merge with 2 conflicts, both resolved.
	conflict1, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_1.json")
	s.NoError(err)
	s.NotNil(conflict1)

	conflict2, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_2.json")
	s.NoError(err)
	s.NotNil(conflict2)

	// Put the merge state in mongo.
	m1cid1 := conflict1.Resource.Id
	m1cid2 := conflict2.Resource.Id

	conflicts := state.ConflictMap{
		m1cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid1,
			Resolved: true,
		},
		m1cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid2,
			Resolved: true,
		},
	}
	m1TargetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(m1TargetURL, conflicts)
	s.NoError(err)

	// Make the request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/conflicts"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusOK, res.StatusCode)
	decoder := json.NewDecoder(res.Body)
	ooBundle := &models.Bundle{}
	err = decoder.Decode(ooBundle)
	s.NoError(err)

	s.Equal("transaction-response", ooBundle.Type)
	s.Equal(uint32(0), *ooBundle.Total)
	s.Len(ooBundle.Entry, 0)
}

func (s *ServerTestSuite) TestGetRemainingConflictsConflictNotFound() {
	// Put the merge state in mongo.
	m1cid1 := bson.NewObjectId().Hex()
	m1cid2 := bson.NewObjectId().Hex()

	conflicts := state.ConflictMap{
		m1cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid1,
			Resolved: false,
		},
		m1cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid2,
			Resolved: true,
		},
	}
	m1TargetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(m1TargetURL, conflicts)
	s.NoError(err)

	// Make the request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/conflicts"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	// Check the response.
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusInternalServerError, res.StatusCode)
	s.Equal(fmt.Sprintf("Resource %s not found", s.FHIRServer.URL+"/OperationOutcome/"+m1cid1), string(body))
}

func (s *ServerTestSuite) TestGetRemainingConflictsMergeNotFound() {
	// Make the request.
	mergeID := bson.NewObjectId().Hex()
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/conflicts"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge %s not found", mergeID), string(body))
}

// ========================================================================= //
// TEST GET RESOLVED CONFLICTS                                               //
// ========================================================================= //

func (s *ServerTestSuite) TestGetResolvedConflicts() {
	// Create a merge with 2 conflicts, one resolved.
	conflict1, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_1.json")
	s.NoError(err)
	s.NotNil(conflict1)

	conflict2, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_2.json")
	s.NoError(err)
	s.NotNil(conflict2)

	// Put the merge state in mongo.
	m1cid1 := conflict1.Resource.Id
	m1cid2 := conflict2.Resource.Id

	conflicts := state.ConflictMap{
		m1cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid1,
			Resolved: false,
		},
		m1cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid2,
			Resolved: true, // This conflict is resolved
		},
	}
	m1TargetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(m1TargetURL, conflicts)
	s.NoError(err)

	// Make the request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolved"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusOK, res.StatusCode)
	decoder := json.NewDecoder(res.Body)
	ooBundle := &models.Bundle{}
	err = decoder.Decode(ooBundle)
	s.NoError(err)

	s.Equal("transaction-response", ooBundle.Type)
	s.Equal(uint32(1), *ooBundle.Total)
	s.Len(ooBundle.Entry, 1)

	// Validate the one remaining conflict.
	entry := ooBundle.Entry[0]
	conflict, ok := entry.Resource.(*models.OperationOutcome)
	s.True(ok)
	s.Equal(m1cid2, conflict.Resource.Id)
}

func (s *ServerTestSuite) TestGetResolvedConflictsNoneFound() {
	// Create a merge with 2 conflicts, neither resolved.
	conflict1, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_1.json")
	s.NoError(err)
	s.NotNil(conflict1)

	conflict2, err := testutil.PostOperationOutcome(s.FHIRServer.URL, "../fixtures/two_conflicts/conflict_2.json")
	s.NoError(err)
	s.NotNil(conflict2)

	// Put the merge state in mongo.
	m1cid1 := conflict1.Resource.Id
	m1cid2 := conflict2.Resource.Id

	conflicts := state.ConflictMap{
		m1cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid1,
			Resolved: false,
		},
		m1cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid2,
			Resolved: false,
		},
	}
	m1TargetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(m1TargetURL, conflicts)
	s.NoError(err)

	// Make the request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolved"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusOK, res.StatusCode)
	decoder := json.NewDecoder(res.Body)
	ooBundle := &models.Bundle{}
	err = decoder.Decode(ooBundle)
	s.NoError(err)

	s.Equal("transaction-response", ooBundle.Type)
	s.Equal(uint32(0), *ooBundle.Total)
	s.Len(ooBundle.Entry, 0)
}

func (s *ServerTestSuite) TestGetResolvedConflictsConflictsNotFound() {
	// Put the merge state in mongo.
	m1cid1 := bson.NewObjectId().Hex()
	m1cid2 := bson.NewObjectId().Hex()

	conflicts := state.ConflictMap{
		m1cid1: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid1,
			Resolved: true,
		},
		m1cid2: &state.ConflictState{
			URL:      s.FHIRServer.URL + "/OperationOutcome/" + m1cid2,
			Resolved: false,
		},
	}
	m1TargetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(m1TargetURL, conflicts)
	s.NoError(err)

	// Make the request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolved"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusInternalServerError, res.StatusCode)
	s.Equal(fmt.Sprintf("Resource %s not found", s.FHIRServer.URL+"/OperationOutcome/"+m1cid1), string(body))
}

func (s *ServerTestSuite) TestGetResolvedConflictsMergeNotFound() {
	// Make the request.
	mergeID := bson.NewObjectId().Hex()
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolved"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge %s not found", mergeID), string(body))
}

// ========================================================================= //
// TEST GET MERGE TARGET                                                     //
// ========================================================================= //

func (s *ServerTestSuite) TestGetMergeTarget() {
	// Create a target bundle.
	target, err := testutil.PostPatientBundle(s.FHIRServer.URL, "../fixtures/two_conflicts/john_peters_bundle_1.json")
	s.NoError(err)

	// Put the merge state in mongo.
	conflicts := state.ConflictMap{}
	targetURL := s.FHIRServer.URL + "/Bundle/" + target.Resource.Id
	mergeID, err := s.insertMergeState(targetURL, conflicts)
	s.NoError(err)

	// Make the request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/target"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusOK, res.StatusCode)
	decoder := json.NewDecoder(res.Body)
	ooBundle := &models.Bundle{}
	err = decoder.Decode(ooBundle)
	s.NoError(err)

	s.Equal("collection", ooBundle.Type)
	s.Equal(uint32(19), *ooBundle.Total)
	s.Len(ooBundle.Entry, 19)
}

func (s *ServerTestSuite) TestGetMergeTargetTargetNotFound() {
	// Put the merge state in mongo.
	conflicts := state.ConflictMap{}
	targetURL := s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex()
	mergeID, err := s.insertMergeState(targetURL, conflicts)
	s.NoError(err)

	// Make the request.
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/target"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusInternalServerError, res.StatusCode)
	s.Equal(fmt.Sprintf("Resource %s not found", targetURL), string(body))
}

func (s *ServerTestSuite) TestGetMergeTargetMergeNotFound() {
	// Make the request.
	mergeID := bson.NewObjectId().Hex()
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/target"

	res, err := http.Get(req)
	defer res.Body.Close()
	s.NoError(err)

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge %s not found", mergeID), string(body))
}

// ========================================================================= //
// TEST HELPERS                                                              //
// ========================================================================= //

// insertMergeState inserts a MergeState into the test mongo database. This
// helper uses the "ptmerge-test" database only.
func (s *ServerTestSuite) insertMergeState(targetURL string, conflicts state.ConflictMap) (mergeID string, err error) {
	mergeID = bson.NewObjectId().Hex()
	mergeState := &state.MergeState{
		MergeID:   mergeID,
		TargetURL: targetURL,
		Conflicts: conflicts,
	}
	err = s.DB().C("merges").Insert(mergeState)
	if err != nil {
		return "", err
	}
	return mergeID, nil
}
