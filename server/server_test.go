package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/intervention-engine/fhir/server"
	"github.com/mitre/ptmerge/fhirutil"
	"github.com/mitre/ptmerge/state"
	"github.com/mitre/ptmerge/testutil"
	"github.com/stretchr/testify/suite"
)

type ServerTestSuite struct {
	// The MongoSuite borrowed from IE provides useful features for starting/stopping
	// a test mongo database. The same database is used by the mock FHIR server and
	// mock PTMergeServer.
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
	server.RegisterRoutes(fhirEngine, nil, server.NewMongoDataAccessLayer(ms, nil, true), server.Config{})
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
	s.T().Skip()
}

func (s *ServerTestSuite) TestMergeSomeConflicts() {
	s.T().Skip()
}

func (s *ServerTestSuite) TestMergeImbalanceLeftAndRight() {
	// There are many resources in the left not in the right, and vise versa.
	s.T().Skip()
}

// ========================================================================= //
// TEST RESOLVE CONFLICTS                                                    //
// ========================================================================= //

func (s *ServerTestSuite) TestResolveConflictConflictResolved() {
	s.T().Skip()
}

func (s *ServerTestSuite) TestResolveConflictNoMoreConflicts() {
	s.T().Skip()
}

func (s *ServerTestSuite) TestResolveConflictMergeNotFound() {
	var err error

	// Insert some merges so there's something to query against.
	c1 := make(state.ConflictMap)
	cid1 := bson.NewObjectId().Hex()
	c1[cid1] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m1)
	s.NoError(err)

	c2 := make(state.ConflictMap)
	cid2 := bson.NewObjectId().Hex()
	c2[cid2] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m2 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m2)
	s.NoError(err)

	// Try to resolve a conflict for a merge (and a conflict) that doesn't exist.
	// MergeID is checked before conflictIDs so the conflictID in this case doesn't matter.
	mergeID := bson.NewObjectId().Hex()
	conflictID := bson.NewObjectId().Hex()
	resource := []byte(
		`{
			"resourceType": "Patient",
			"name": [{
				"family": "Abbott",
				"given": ["Clint"]
			}],
			"gender": "male",
			"birthDate": "1950-09-02"
		}`)

	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolve/" + conflictID
	res, err := http.Post(req, "application/json", bytes.NewReader(resource))
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge %s not found", mergeID), string(body))
}

func (s *ServerTestSuite) TestResolveConflictConflictNotFound() {
	var err error

	// Insert some merges so there's something to query against.
	c1 := make(state.ConflictMap)
	cid1 := bson.NewObjectId().Hex()
	c1[cid1] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m1)
	s.NoError(err)

	c2 := make(state.ConflictMap)
	cid2 := bson.NewObjectId().Hex()
	c2[cid2] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m2 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m2)
	s.NoError(err)

	// Try to resolve a conflict for a that doesn't exist (but the merge does).
	conflictID := bson.NewObjectId().Hex()
	resource := []byte(
		`{
			"resourceType": "Patient",
			"name": [{
				"family": "Abbott",
				"given": ["Clint"]
			}],
			"gender": "male",
			"birthDate": "1950-09-02"
		}`)

	req := s.PTMergeServer.URL + "/merge/" + m1.MergeID + "/resolve/" + conflictID
	res, err := http.Post(req, "application/json", bytes.NewReader(resource))
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge conflict %s not found for merge %s", conflictID, m1.MergeID), string(body))
}

func (s *ServerTestSuite) TestResolveConflictConflictAlreadyResolved() {
	var err error

	// Insert some merges so there's something to query against.
	c1 := make(state.ConflictMap)
	cid1 := bson.NewObjectId().Hex()
	c1[cid1] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m1)
	s.NoError(err)

	c2 := make(state.ConflictMap)
	cid2 := bson.NewObjectId().Hex()
	c2[cid2] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
		Resolved:            true,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m2 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c2,
	}
	mergeID2, err := s.insertMergeState(m2)
	s.NoError(err)

	resource := []byte(
		`{
			"resourceType": "Patient",
			"name": [{
				"family": "Abbott",
				"given": ["Clint"]
			}],
			"gender": "male",
			"birthDate": "1950-09-02"
		}`)

	req := s.PTMergeServer.URL + "/merge/" + mergeID2 + "/resolve/" + cid2
	res, err := http.Post(req, "application/json", bytes.NewReader(resource))
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusBadRequest, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge conflict %s was already resolved for merge %s", cid2, mergeID2), string(body))
}

// ========================================================================= //
// TEST ABORT MERGE                                                          //
// ========================================================================= //

func (s *ServerTestSuite) TestAbortMerge() {
	var err error

	// Put a target bundle and a conflict on the host FHIR server.
	fix, err := fhirutil.LoadResource("OperationOutcome", "../fixtures/operation_outcomes/oo_0.json")
	resource, err := fhirutil.PostResource(s.FHIRServer.URL, "OperationOutcome", fix)
	s.NoError(err)
	conflict, ok := resource.(*models.OperationOutcome)
	s.True(ok)

	fix, err = fhirutil.LoadResource("Bundle", "../fixtures/bundles/john_peters_bundle.json")
	resource, err = fhirutil.PostResource(s.FHIRServer.URL, "Bundle", fix)
	s.NoError(err)
	target, ok := resource.(*models.Bundle)
	s.True(ok)

	// Put the merge state in mongo.
	c1 := make(state.ConflictMap)
	c1[conflict.Id] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + conflict.Id,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + target.Id,
		Conflicts: c1,
	}
	mergeID, err := s.insertMergeState(m1)
	s.NoError(err)

	// Make the request.
	res, err := http.Post(s.PTMergeServer.URL+"/merge/"+mergeID+"/abort", "", nil)

	// Check the response. There should be no response body.
	s.Equal(http.StatusNoContent, res.StatusCode)
}

func (s *ServerTestSuite) TestAbortMergeMergeNotFound() {
	var err error

	// Insert some merges so there's something to query against.
	c1 := make(state.ConflictMap)
	cid1 := bson.NewObjectId().Hex()
	c1[cid1] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m1)
	s.NoError(err)

	c2 := make(state.ConflictMap)
	cid2 := bson.NewObjectId().Hex()
	c2[cid2] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
		Resolved:            true,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m2 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m2)
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
	c1 := make(state.ConflictMap)
	cid1 := bson.NewObjectId().Hex()
	c1[cid1] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m1)
	s.NoError(err)

	// Merge 2 metadata.
	c2 := make(state.ConflictMap)
	cid2 := bson.NewObjectId().Hex()
	c2[cid2] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
		Resolved:            true,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m2 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m2)
	s.NoError(err)

	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge")
	s.NoError(err)
	defer res.Body.Close()
	s.Equal(http.StatusOK, res.StatusCode)

	// Umnarshal the response body.
	metadata := state.Merges{}
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &metadata)
	s.NoError(err)

	s.NotEmpty(metadata.Timestamp)
	s.Len(metadata.Merges, 2)

	// Validate the first merge.
	var merge1, merge2 state.MergeState
	if metadata.Merges[0].MergeID == m1.MergeID {
		merge1 = metadata.Merges[0]
		merge2 = metadata.Merges[1]
	} else {
		merge1 = metadata.Merges[1]
		merge2 = metadata.Merges[0]
	}
	s.Equal(m1.MergeID, merge1.MergeID)
	s.False(merge1.Completed)
	s.Equal(m1.TargetURL, merge1.TargetURL)
	s.Len(merge1.Conflicts, 1)

	// Validate the conflicts.
	conflict, ok := merge1.Conflicts[merge1.Conflicts.Keys()[0]]
	s.True(ok)
	s.Equal(s.FHIRServer.URL+"/OperationOutcome/"+m1.Conflicts.Keys()[0], conflict.OperationOutcomeURL)
	s.False(conflict.Resolved)

	// Validate the second merge.
	s.Equal(m2.MergeID, merge2.MergeID)
	s.False(merge2.Completed)
	s.Equal(m2.TargetURL, merge2.TargetURL)
	s.Len(merge2.Conflicts, 1)

	// Validate the conflicts.
	conflict, ok = merge2.Conflicts[merge2.Conflicts.Keys()[0]]
	s.True(ok)
	s.Equal(s.FHIRServer.URL+"/OperationOutcome/"+m2.Conflicts.Keys()[0], conflict.OperationOutcomeURL)
	s.False(conflict.Resolved)
}

func (s *ServerTestSuite) TestAllMergesNoMerges() {
	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge")
	s.NoError(err)
	defer res.Body.Close()
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
	c1 := make(state.ConflictMap)
	cid1 := bson.NewObjectId().Hex()
	c1[cid1] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m1)
	s.NoError(err)

	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge/" + m1.MergeID)
	s.NoError(err)
	defer res.Body.Close()
	s.Equal(http.StatusOK, res.StatusCode)

	// Umnarshal the response body.
	metadata := state.Merge{}
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &metadata)
	s.NoError(err)

	s.NotEmpty(metadata.Timestamp)
	merge := metadata.Merge

	s.Equal(m1.MergeID, merge.MergeID)
	s.Equal(m1.TargetURL, merge.TargetURL)
	s.Len(merge.Conflicts, 1)

	conflict1, ok := merge.Conflicts[merge.Conflicts.Keys()[0]]
	s.True(ok)
	s.Equal(s.FHIRServer.URL+"/OperationOutcome/"+m1.Conflicts.Keys()[0], conflict1.OperationOutcomeURL)
	s.False(conflict1.Resolved)
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
	var err error

	// Put the OperationOutcomes on the FHIR server.
	oo1 := fhirutil.OperationOutcome(
		"Patient",
		bson.NewObjectId().Hex(),
		[]string{"foo", "bar.x", "bar.y"},
	)
	created1, err := fhirutil.PostResource(s.FHIRServer.URL, "OperationOutcome", oo1)
	s.NoError(err)
	coo1, ok := created1.(*models.OperationOutcome)
	s.True(ok)

	oo2 := fhirutil.OperationOutcome(
		"Patient",
		bson.NewObjectId().Hex(),
		[]string{"foo", "bar.u", "bar.v"},
	)

	created2, err := fhirutil.PostResource(s.FHIRServer.URL, "OperationOutcome", oo2)
	s.NoError(err)
	coo2, ok := created2.(*models.OperationOutcome)
	s.True(ok)

	// Create a merge with 2 conflicts, one resolved.
	c1 := make(state.ConflictMap)
	c1[coo1.Id] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + coo1.Id,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   strings.SplitN(coo1.Issue[0].Diagnostics, ":", 2)[1],
			ResourceType: "Patient",
		},
	}
	c1[coo2.Id] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + coo2.Id,
		Resolved:            true,
		TargetResource: state.TargetResource{
			ResourceID:   strings.SplitN(coo2.Issue[0].Diagnostics, ":", 2)[1],
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	mergeID, err := s.insertMergeState(m1)
	s.NoError(err)

	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge/" + mergeID + "/conflicts")
	s.NoError(err)
	defer res.Body.Close()

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
	oo, ok := entry.Resource.(*models.OperationOutcome)
	s.True(ok)

	s.Equal(coo1.Id, oo.Id)
	s.Len(oo.Issue, 1)
	s.Len(oo.Issue[0].Location, 3)
	s.Equal([]string{"foo", "bar.x", "bar.y"}, oo.Issue[0].Location)
	s.Equal("Patient:"+c1[coo1.Id].TargetResource.ResourceID, oo.Issue[0].Diagnostics)
}

func (s *ServerTestSuite) TestGetRemainingConflictsNoneRemaining() {
	var err error

	c1 := make(state.ConflictMap)
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	c1[cid1] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		Resolved:            true,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	c1[cid2] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
		Resolved:            true,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	mergeID, err := s.insertMergeState(m1)
	s.NoError(err)

	// Put the OperationOutcomes on the FHIR server.
	_, err = fhirutil.PostResource(
		s.FHIRServer.URL, "OperationOutcome",
		fhirutil.OperationOutcome(
			"Patient",
			c1[cid1].TargetResource.ResourceID,
			[]string{"foo", "bar.x", "bar.y"},
		),
	)
	s.NoError(err)

	_, err = fhirutil.PostResource(
		s.FHIRServer.URL, "OperationOutcome",
		fhirutil.OperationOutcome(
			"Patient",
			c1[cid2].TargetResource.ResourceID,
			[]string{"foo", "bar.u", "bar.v"},
		),
	)
	s.NoError(err)

	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge/" + mergeID + "/conflicts")
	s.NoError(err)
	defer res.Body.Close()

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
	var err error

	// Create a merge with 2 conflicts, one resolved.
	c1 := make(state.ConflictMap)
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	c1[cid1] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	c1[cid2] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
		Resolved:            true,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	mergeID, err := s.insertMergeState(m1)
	s.NoError(err)

	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge/" + mergeID + "/conflicts")
	s.NoError(err)
	defer res.Body.Close()

	// Check the response.
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusInternalServerError, res.StatusCode)
	s.Equal(fmt.Sprintf("Resource %s not found", "OperationOutcome:"+cid1), string(body))
}

func (s *ServerTestSuite) TestGetRemainingConflictsMergeNotFound() {
	// Make the request.
	mergeID := bson.NewObjectId().Hex()
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/conflicts"

	res, err := http.Get(req)
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge %s not found", mergeID), string(body))
}

// ========================================================================= //
// TEST GET RESOLVED CONFLICTS                                               //
// ========================================================================= //

func (s *ServerTestSuite) TestGetResolvedConflicts() {
	var err error

	// Put the OperationOutcomes on the FHIR server.
	oo1 := fhirutil.OperationOutcome(
		"Patient",
		bson.NewObjectId().Hex(),
		[]string{"foo", "bar.x", "bar.y"},
	)
	created1, err := fhirutil.PostResource(s.FHIRServer.URL, "OperationOutcome", oo1)
	s.NoError(err)
	coo1, ok := created1.(*models.OperationOutcome)
	s.True(ok)

	oo2 := fhirutil.OperationOutcome(
		"Patient",
		bson.NewObjectId().Hex(),
		[]string{"foo", "bar.u", "bar.v"},
	)

	created2, err := fhirutil.PostResource(s.FHIRServer.URL, "OperationOutcome", oo2)
	s.NoError(err)
	coo2, ok := created2.(*models.OperationOutcome)
	s.True(ok)

	// Create a merge with 2 conflicts, one resolved.
	c1 := make(state.ConflictMap)
	c1[coo1.Id] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + coo1.Id,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   strings.SplitN(coo1.Issue[0].Diagnostics, ":", 2)[1],
			ResourceType: "Patient",
		},
	}
	c1[coo2.Id] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + coo2.Id,
		Resolved:            true,
		TargetResource: state.TargetResource{
			ResourceID:   strings.SplitN(coo2.Issue[0].Diagnostics, ":", 2)[1],
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	mergeID, err := s.insertMergeState(m1)
	s.NoError(err)

	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge/" + mergeID + "/resolved")
	s.NoError(err)
	defer res.Body.Close()

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
	oo, ok := entry.Resource.(*models.OperationOutcome)
	s.True(ok)

	s.Equal(coo2.Id, oo.Id)
	s.Len(oo.Issue, 1)
	s.Len(oo.Issue[0].Location, 3)
	s.Equal([]string{"foo", "bar.u", "bar.v"}, oo.Issue[0].Location)
	s.Equal("Patient:"+c1[coo2.Id].TargetResource.ResourceID, oo.Issue[0].Diagnostics)
}

func (s *ServerTestSuite) TestGetResolvedConflictsNoResolvedConflicts() {
	var err error

	// Create a merge with 2 conflicts, none resolved.
	c1 := make(state.ConflictMap)
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	c1[cid1] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	c1[cid2] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	mergeID, err := s.insertMergeState(m1)
	s.NoError(err)

	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge/" + mergeID + "/resolved")
	s.NoError(err)
	defer res.Body.Close()

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

func (s *ServerTestSuite) TestGetResolvedConflictsConflictNotFound() {
	var err error

	// Create a merge with 2 conflicts, one resolved.
	c1 := make(state.ConflictMap)
	cid1 := bson.NewObjectId().Hex()
	cid2 := bson.NewObjectId().Hex()
	c1[cid1] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid1,
		Resolved:            false,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	c1[cid2] = &state.ConflictState{
		OperationOutcomeURL: s.FHIRServer.URL + "/OperationOutcome/" + cid2,
		Resolved:            true,
		TargetResource: state.TargetResource{
			ResourceID:   bson.NewObjectId().Hex(),
			ResourceType: "Patient",
		},
	}
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	mergeID, err := s.insertMergeState(m1)
	s.NoError(err)

	// Put the OperationOutcomes on the FHIR server.
	_, err = fhirutil.PostResource(
		s.FHIRServer.URL, "OperationOutcome",
		fhirutil.OperationOutcome(
			"Patient",
			c1[cid1].TargetResource.ResourceID,
			[]string{"foo", "bar.x", "bar.y"},
		),
	)
	s.NoError(err)

	_, err = fhirutil.PostResource(
		s.FHIRServer.URL, "OperationOutcome",
		fhirutil.OperationOutcome(
			"Patient",
			c1[cid2].TargetResource.ResourceID,
			[]string{"foo", "bar.u", "bar.v"},
		),
	)
	s.NoError(err)

	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge/" + mergeID + "/resolved")
	s.NoError(err)
	defer res.Body.Close()

	// Check the response.
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	s.Equal(http.StatusInternalServerError, res.StatusCode)
	s.Equal(fmt.Sprintf("Resource %s not found", "OperationOutcome:"+cid2), string(body))
}

func (s *ServerTestSuite) TestGetResolvedConflictsMergeNotFound() {
	// Make the request.
	mergeID := bson.NewObjectId().Hex()
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/resolved"

	res, err := http.Get(req)
	s.NoError(err)
	defer res.Body.Close()

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
	fix, err := fhirutil.LoadResource("Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	s.NoError(err)
	created, err := fhirutil.PostResource(s.FHIRServer.URL, "Bundle", fix)
	s.NoError(err)
	target, ok := created.(*models.Bundle)
	s.True(ok)

	// Put the merge state in mongo.
	c1 := make(state.ConflictMap)
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + target.Id,
		Conflicts: c1,
	}
	mergeID, err := s.insertMergeState(m1)
	s.NoError(err)

	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge/" + mergeID + "/target")
	s.NoError(err)
	defer res.Body.Close()

	// Check the response.
	s.Equal(http.StatusOK, res.StatusCode)
	decoder := json.NewDecoder(res.Body)
	ooBundle := &models.Bundle{}
	err = decoder.Decode(ooBundle)
	s.NoError(err)

	s.Len(ooBundle.Entry, 7)
}

func (s *ServerTestSuite) TestGetMergeTargetTargetNotFound() {
	var err error

	// Put the merge state in mongo.
	c1 := make(state.ConflictMap)
	m1 := &state.MergeState{
		MergeID:   bson.NewObjectId().Hex(),
		Completed: false,
		TargetURL: s.FHIRServer.URL + "/Bundle/" + bson.NewObjectId().Hex(),
		Conflicts: c1,
	}
	_, err = s.insertMergeState(m1)
	s.NoError(err)

	// Make the request.
	res, err := http.Get(s.PTMergeServer.URL + "/merge/" + m1.MergeID + "/target")
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusInternalServerError, res.StatusCode)
	s.Equal(fmt.Sprintf("Resource %s not found", m1.TargetURL), string(body))
}

func (s *ServerTestSuite) TestGetMergeTargetMergeNotFound() {
	// Make the request.
	mergeID := bson.NewObjectId().Hex()
	req := s.PTMergeServer.URL + "/merge/" + mergeID + "/target"

	res, err := http.Get(req)
	s.NoError(err)
	defer res.Body.Close()

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
func (s *ServerTestSuite) insertMergeState(mergeState *state.MergeState) (mergeID string, err error) {
	err = s.DB().C("merges").Insert(mergeState)
	if err != nil {
		return "", err
	}
	return mergeState.MergeID, nil
}
