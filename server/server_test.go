package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/intervention-engine/fhir/server"
	"github.com/mitre/ptmerge/fhirutil"
	"github.com/mitre/ptmerge/merge"
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
	config := server.DefaultConfig
	config.DatabaseName = "ptmerge-test"
	server.RegisterRoutes(fhirEngine, nil, server.NewMongoDataAccessLayer(ms, nil, config), config)
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
	// Post the source bundles.
	created, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	s.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	s.True(ok)

	created, err = fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	s.NoError(err)
	rightBundle, ok := created.(*models.Bundle)
	s.True(ok)

	// Get a count of the number of merge states in Mongo.
	mergeCount, err := s.DB().C("merges").Count()
	s.NoError(err)

	// Make the merge request.
	source1 := s.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := s.FHIRServer.URL + "/Bundle/" + rightBundle.Id
	url := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	req, err := http.NewRequest("POST", url, nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusOK, res.StatusCode)

	// Unmarshal and check the body.
	bundle := models.Bundle{}
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &bundle)
	s.NoError(err)

	s.Len(bundle.Entry, 7)

	// Check that the merges in mongo weren't updated or changed.
	newCount, err := s.DB().C("merges").Count()
	s.NoError(err)
	s.Equal(mergeCount, newCount)
}

func (s *ServerTestSuite) TestMergeSomeConflicts() {
	// The outcome should be a set of conflicts.
	created, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	s.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	s.True(ok)

	created2, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_unmarried_bundle.json")
	s.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	s.True(ok)

	// Get a count of the number of merge states in Mongo.
	mergeCount, err := s.DB().C("merges").Count()
	s.NoError(err)

	// Make the merge request.
	source1 := s.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := s.FHIRServer.URL + "/Bundle/" + rightBundle.Id
	url := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	req, err := http.NewRequest("POST", url, nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusCreated, res.StatusCode)

	// Unmarshal and check the body.
	outcome := models.Bundle{}
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &outcome)
	s.NoError(err)

	// Validate the bundle of OperationOutcomes. There should be 2 conflicts:
	// 2 paths in the Patient resource and 2 paths in an Encounter resource.
	var patientConflictID, encounterConflictID string
	var patientTargetID, encounterTargetID string

	s.Len(outcome.Entry, 2)
	for _, entry := range outcome.Entry {
		oo, ok := entry.Resource.(*models.OperationOutcome)
		s.True(ok)

		s.Len(oo.Issue, 1)
		issue := oo.Issue[0]
		s.Equal("information", issue.Severity)
		s.Equal("conflict", issue.Code)
		s.Len(issue.Location, 2)
		s.NotEmpty(issue.Diagnostics)

		// Validate the Patient conflicts.
		if strings.Contains(issue.Diagnostics, "Patient") {
			// Reference to the new Patient resource in the target bundle.
			s.NotEmpty(issue.Diagnostics)
			parts := strings.SplitN(issue.Diagnostics, ":", 2)
			s.Equal("Patient", parts[0])
			s.NotEmpty(parts[1])
			s.Len(parts[1], len(bson.NewObjectId().Hex()))

			for _, loc := range issue.Location {
				s.True(contains([]string{"maritalStatus.coding[0].display", "maritalStatus.coding[0].code"}, loc))
			}
			patientConflictID = oo.Id
			patientTargetID = parts[1]
			continue
		}

		// Validate the Encounter conflicts.
		if strings.Contains(issue.Diagnostics, "Encounter") {
			// Reference to the new Encounter resource in the target bundle.
			s.NotEmpty(issue.Diagnostics)
			parts := strings.SplitN(issue.Diagnostics, ":", 2)
			s.Equal("Encounter", parts[0])
			s.NotEmpty(parts[1])
			s.Len(parts[1], len(bson.NewObjectId().Hex()))

			for _, loc := range issue.Location {
				s.True(contains([]string{"period.start", "period.end"}, loc))
			}
			encounterConflictID = oo.Id
			encounterTargetID = parts[1]
			continue
		}
	}

	// Check that the merge ID is in the Location header
	mergeID := res.Header.Get("Location")
	s.NotEmpty(mergeID)

	// Check that the target bundle exists and contains the expected resources.
	res, err = http.Get(s.PTMergeServer.URL + "/merge/" + mergeID + "/target")
	s.NoError(err)

	// Unmarshal and check the body.
	targetBundle := models.Bundle{}
	body, err = ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &targetBundle)
	s.NoError(err)

	s.Len(targetBundle.Entry, 7)

	// There should be one Patient.
	pcount := 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Patient" {
			pcount++
		}
	}
	s.Equal(1, pcount)

	// There should also be 2 Encounters.
	ecount := 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Encounter" {
			ecount++
		}
	}
	s.Equal(2, ecount)

	// And 1 Procedure.
	pcount = 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Procedure" {
			pcount++
		}
	}
	s.Equal(1, pcount)

	// And 2 MedicationStatements.
	mcount := 0
	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "MedicationStatement" {
			mcount++
		}
	}
	s.Equal(2, mcount)

	// Check that the merge metadata was updated.
	newMergeCount, err := s.DB().C("merges").Count()
	s.NoError(err)
	s.Equal(mergeCount+1, newMergeCount)

	// Get the merge metadata.
	mergeState := &state.MergeState{}
	err = s.DB().C("merges").FindId(mergeID).One(mergeState)
	s.NoError(err)

	// Validate the mergeState.
	s.Equal(mergeID, mergeState.MergeID)
	s.Equal(source1, mergeState.Source1URL)
	s.Equal(source2, mergeState.Source2URL)
	s.Equal(s.FHIRServer.URL+"/Bundle/"+targetBundle.Id, mergeState.TargetURL)
	s.False(mergeState.Completed)
	s.NotNil(mergeState.Start)
	s.Nil(mergeState.End)
	s.Len(mergeState.Conflicts, 2)

	// Patient conflict metadata.
	pc, ok := mergeState.Conflicts[patientConflictID]
	s.True(ok)
	s.Equal(s.FHIRServer.URL+"/OperationOutcome/"+patientConflictID, pc.OperationOutcomeURL)
	s.False(pc.Resolved)
	s.Equal("Patient", pc.TargetResource.ResourceType)
	s.Equal(patientTargetID, pc.TargetResource.ResourceID)

	// Encounter conflict metadata.
	ec, ok := mergeState.Conflicts[encounterConflictID]
	s.True(ok)
	s.Equal(s.FHIRServer.URL+"/OperationOutcome/"+encounterConflictID, ec.OperationOutcomeURL)
	s.False(ec.Resolved)
	s.Equal("Encounter", ec.TargetResource.ResourceType)
	s.Equal(encounterTargetID, ec.TargetResource.ResourceID)
}

func (s *ServerTestSuite) TestMergeFromBatch() {
	// The bundles here come from queries of the from /Patient/:patient_id/$everything.
	// In practice, these are bundles formed by the server as a union of _include and _revinclude.
	// The actually merge process should be agnostic of this and work the same as before.
	created, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "", "../fixtures/batch/lowell_abbott_bundle.json")
	s.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	s.True(ok)

	created2, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "", "../fixtures/batch/lowell_abbott_unmarried_bundle.json")
	s.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	s.True(ok)

	// Get a count of the number of merge states in Mongo.
	mergeCount, err := s.DB().C("merges").Count()
	s.NoError(err)

	// Get the ID of the patient in the leftBundle
	var leftPatientID, rightPatientID string

	for _, entry := range leftBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Patient" {
			patient, ok := entry.Resource.(*models.Patient)
			s.True(ok)
			leftPatientID = patient.Id
		}
	}

	// And the right
	for _, entry := range rightBundle.Entry {
		if fhirutil.GetResourceType(entry.Resource) == "Patient" {
			patient, ok := entry.Resource.(*models.Patient)
			s.True(ok)
			rightPatientID = patient.Id
		}
	}

	// Make the merge request.
	source1 := s.FHIRServer.URL + "/Patient?_id=" + leftPatientID + "&_include=*&_revinclude=*"
	source2 := s.FHIRServer.URL + "/Patient?_id=" + rightPatientID + "&_include=*&_revinclude=*"
	url := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	req, err := http.NewRequest("POST", url, nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusCreated, res.StatusCode)

	// Mongo should have been updated.
	newMergeCount, err := s.DB().C("merges").Count()
	s.NoError(err)
	s.Equal(mergeCount+1, newMergeCount)
}

func (s *ServerTestSuite) TestMergeBadSources() {
	// One of the source bundles is missing a Patient resource, so no merge can be performed.
	created, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	s.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	s.True(ok)

	created2, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/no_patient_bundle.json")
	s.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	s.True(ok)

	// Make the merge request.
	source1 := s.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := s.FHIRServer.URL + "/Bundle/" + rightBundle.Id
	url := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	req, err := http.NewRequest("POST", url, nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusBadRequest, res.StatusCode)

	// Unmarshal and check the body.
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	s.Equal(merge.ErrNoPatientResource.Error(), string(body))
}

// ========================================================================= //
// TEST RESOLVE CONFLICTS                                                    //
// ========================================================================= //

func (s *ServerTestSuite) TestResolveConflictConflictResolved() {
	var err error

	// Setup a merge with unresolved conflicts.
	created, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	s.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	s.True(ok)

	created2, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_unmarried_bundle.json")
	s.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	s.True(ok)

	// Make the merge request.
	source1 := s.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := s.FHIRServer.URL + "/Bundle/" + rightBundle.Id
	url := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	req, err := http.NewRequest("POST", url, nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusCreated, res.StatusCode)

	// Unmarshal and check the body.
	outcome := models.Bundle{}
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &outcome)
	s.NoError(err)

	// Get a count of the number of merge states in Mongo.
	mergeCount, err := s.DB().C("merges").Count()
	s.NoError(err)

	// Get the diagnostic info from the Patient conflict.
	found := false
	var targetPatientID string
	var oo *models.OperationOutcome
	for _, entry := range outcome.Entry {
		oo, ok = entry.Resource.(*models.OperationOutcome)
		s.True(ok)
		s.Len(oo.Issue, 1)
		if strings.Contains(oo.Issue[0].Diagnostics, "Patient") {
			found = true
			parts := strings.SplitN(oo.Issue[0].Diagnostics, ":", 2)
			targetPatientID = parts[1]
			break
		}
	}
	s.True(found)
	s.NotEmpty(targetPatientID)

	// Get the merge ID.
	mergeID := res.Header.Get("Location")
	s.NotEmpty(mergeID)

	// Post the patient resource that resolves the conflict.
	patientResource, err := fhirutil.LoadResource("Patient", "../fixtures/patients/lowell_abbott.json")
	data, err := json.Marshal(patientResource)
	s.NoError(err)
	s.NotEmpty(data)

	req, err = http.NewRequest("POST", s.PTMergeServer.URL+"/merge/"+mergeID+"/resolve/"+oo.Id, bytes.NewReader(data))
	s.NoError(err)
	res, err = http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()
	s.Equal(http.StatusOK, res.StatusCode)

	// Get the target bundle and check that it was updated.
	target, err := fhirutil.GetResourceByURL("Bundle", s.PTMergeServer.URL+"/merge/"+mergeID+"/target")
	s.NoError(err)
	targetBundle, ok := target.(*models.Bundle)
	s.True(ok)

	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceID(entry.Resource) == targetPatientID {
			postedPatient, ok := patientResource.(*models.Patient)
			s.True(ok)
			targetPatient, ok := entry.Resource.(*models.Patient)
			s.True(ok)
			s.Equal(postedPatient.Name, targetPatient.Name)
			s.Equal(postedPatient.Telecom, targetPatient.Telecom)
			s.Equal(postedPatient.BirthDate, targetPatient.BirthDate)
			s.Equal(postedPatient.MaritalStatus, targetPatient.MaritalStatus)
			break
		}
	}

	// Check that the merge metadata was updated.
	newMergeCount, err := s.DB().C("merges").Count()
	s.NoError(err)
	s.Equal(mergeCount, newMergeCount)

	// Get the merge metadata.
	mergeState := &state.MergeState{}
	err = s.DB().C("merges").FindId(mergeID).One(mergeState)
	s.NoError(err)

	// Validate the mergeState.
	s.Equal(mergeID, mergeState.MergeID)
	s.Equal(s.FHIRServer.URL+"/Bundle/"+targetBundle.Id, mergeState.TargetURL)
	s.False(mergeState.Completed)
	s.Len(mergeState.Conflicts, 2)

	// The patient conflict should now be resolved.
	pc, ok := mergeState.Conflicts[oo.Id]
	s.True(ok)
	s.Equal(s.FHIRServer.URL+"/OperationOutcome/"+oo.Id, pc.OperationOutcomeURL)
	s.True(pc.Resolved)
	s.Equal("Patient", pc.TargetResource.ResourceType)
	s.Equal(targetPatientID, pc.TargetResource.ResourceID)
}

func (s *ServerTestSuite) TestResolveConflictNoMoreConflicts() {
	var err error

	// Setup a merge with unresolved conflicts.
	created, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	s.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	s.True(ok)

	created2, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_unmarried_bundle.json")
	s.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	s.True(ok)

	// Make the merge request.
	source1 := s.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := s.FHIRServer.URL + "/Bundle/" + rightBundle.Id
	url := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	req, err := http.NewRequest("POST", url, nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusCreated, res.StatusCode)

	// Unmarshal and check the body.
	outcome := models.Bundle{}
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &outcome)
	s.NoError(err)

	// Get a count of the number of merge states in Mongo.
	mergeCount, err := s.DB().C("merges").Count()
	s.NoError(err)

	// Get the diagnostic info from the Patient conflict.
	found := false
	var patientConflictID, encounterConflictID string
	var targetPatientID, targetEncounterID string
	var oo *models.OperationOutcome
	for _, entry := range outcome.Entry {
		oo, ok = entry.Resource.(*models.OperationOutcome)
		s.True(ok)
		s.Len(oo.Issue, 1)
		if strings.Contains(oo.Issue[0].Diagnostics, "Patient") {
			found = true
			parts := strings.SplitN(oo.Issue[0].Diagnostics, ":", 2)
			targetPatientID = parts[1]
			patientConflictID = oo.Id
		}
		if strings.Contains(oo.Issue[0].Diagnostics, "Encounter") {
			found = true
			parts := strings.SplitN(oo.Issue[0].Diagnostics, ":", 2)
			targetEncounterID = parts[1]
			encounterConflictID = oo.Id
		}
		s.True(found)
	}
	s.NotEmpty(targetPatientID)
	s.NotEmpty(targetEncounterID)

	// Get the merge ID.
	mergeID := res.Header.Get("Location")
	s.NotEmpty(mergeID)

	// Post the patient resource that resolves the conflict.
	patientResource, err := fhirutil.LoadResource("Patient", "../fixtures/patients/lowell_abbott.json")
	data, err := json.Marshal(patientResource)
	s.NoError(err)
	s.NotEmpty(data)

	req, err = http.NewRequest("POST", s.PTMergeServer.URL+"/merge/"+mergeID+"/resolve/"+patientConflictID, bytes.NewReader(data))
	s.NoError(err)
	res, err = http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusOK, res.StatusCode)

	// Post the encounter resource that resolves the other conflict.
	encounterResource, err := fhirutil.LoadResource("Encounter", "../fixtures/encounters/encounter_1.json")
	data, err = json.Marshal(encounterResource)
	s.NoError(err)
	s.NotEmpty(data)

	req, err = http.NewRequest("POST", s.PTMergeServer.URL+"/merge/"+mergeID+"/resolve/"+encounterConflictID, bytes.NewReader(data))
	s.NoError(err)
	res, err = http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusOK, res.StatusCode)

	// The body of this second response should be the target bundle, since all conflicts are now resolved.
	outcome = models.Bundle{}
	body, err = ioutil.ReadAll(res.Body)
	s.NoError(err)
	err = json.Unmarshal(body, &outcome)
	s.NoError(err)

	// Get the target bundle and check that it was updated.
	target, err := fhirutil.GetResourceByURL("Bundle", s.PTMergeServer.URL+"/merge/"+mergeID+"/target")
	s.NoError(err)
	targetBundle, ok := target.(*models.Bundle)
	s.True(ok)

	// The target bundle and outcome should match.
	s.Equal(len(outcome.Entry), 7)
	s.Equal(len(targetBundle.Entry), len(outcome.Entry))

	// The merge metadata should still be available.
	newMergeCount, err := s.DB().C("merges").Count()
	s.NoError(err)
	s.Equal(mergeCount, newMergeCount)

	// Get the merge metadata.
	mergeState := &state.MergeState{}
	err = s.DB().C("merges").FindId(mergeID).One(mergeState)
	s.NoError(err)

	// Validate the mergeState.
	s.Equal(mergeID, mergeState.MergeID)
	s.Equal(source1, mergeState.Source1URL)
	s.Equal(source2, mergeState.Source2URL)
	s.Equal(s.FHIRServer.URL+"/Bundle/"+targetBundle.Id, mergeState.TargetURL)
	s.True(mergeState.Completed)
	s.NotNil(mergeState.Start)
	s.NotNil(mergeState.End)
	s.Len(mergeState.Conflicts, 2)

	// The patient conflict should now be resolved.
	for _, conflictID := range mergeState.Conflicts.Keys() {
		conflict := mergeState.Conflicts[conflictID]
		s.True(conflict.Resolved)
	}
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
	resource, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "OperationOutcome", "../fixtures/operation_outcomes/oo_0.json")
	s.NoError(err)
	conflict, ok := resource.(*models.OperationOutcome)
	s.True(ok)

	resource, err = fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/joey_chestnut_bundle.json")
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
// TEST DELETE UNRESOLVED CONFLICT                                           //
// ========================================================================= //

func (s *ServerTestSuite) TestDeleteUnresolvedConflict() {
	var err error

	// Setup a merge with unresolved conflicts.
	created, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	s.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	s.True(ok)

	created2, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_unmarried_bundle.json")
	s.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	s.True(ok)

	// Make the merge request.
	source1 := s.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := s.FHIRServer.URL + "/Bundle/" + rightBundle.Id
	url := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	req, err := http.NewRequest("POST", url, nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusCreated, res.StatusCode)

	// Get a list of conflict IDs.
	decoder := json.NewDecoder(res.Body)
	ooBundle := &models.Bundle{}
	err = decoder.Decode(ooBundle)
	s.NoError(err)

	conflictIDs := make([]string, len(ooBundle.Entry))
	for i, entry := range ooBundle.Entry {
		oo := entry.Resource.(*models.OperationOutcome)
		conflictIDs[i] = oo.Id
	}
	s.True(len(conflictIDs) > 0)
	conflictID := conflictIDs[0]

	// Get the merge ID.
	mergeID := res.Header.Get("Location")
	s.NotEmpty(mergeID)

	// Delete one of the conflicts.
	req, err = http.NewRequest("DELETE", s.PTMergeServer.URL+"/merge/"+mergeID+"/conflicts/"+conflictID, nil)
	s.NoError(err)
	res, err = http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)
	s.Equal(http.StatusNoContent, res.StatusCode)
	fmt.Println(string(body))

	// The OperationOutcome should not exist.
	_, err = fhirutil.GetResource(s.FHIRServer.URL, "OperationOutcome", conflictID)
	s.Equal(fmt.Sprintf("Resource OperationOutcome:%s not found", conflictID), err.Error())

	// And the merge state should also reflect that.
	mergeState := state.MergeState{}
	err = s.DB().C("merges").FindId(mergeID).One(&mergeState)
	s.NoError(err)
	s.True(!contains(mergeState.Conflicts.Keys(), conflictID))
}

func (s *ServerTestSuite) TestDeleteUnresolvedConflictConflictNotFound() {
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
	conflictID := bson.NewObjectId().Hex()
	req, err := http.NewRequest("DELETE", s.PTMergeServer.URL+"/merge/"+m1.MergeID+"/conflicts/"+conflictID, nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge conflict %s not found for merge %s", conflictID, m1.MergeID), string(body))
}

func (s *ServerTestSuite) TestDeleteUnresolvedConflictMergeNotFound() {
	// Make the request.
	mergeID := bson.NewObjectId().Hex()
	req, err := http.NewRequest("DELETE", s.PTMergeServer.URL+"/merge/"+mergeID+"/conflicts/"+bson.NewObjectId().Hex(), nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge %s not found", mergeID), string(body))
}

// ========================================================================= //
// TEST GET MERGE TARGET                                                     //
// ========================================================================= //

func (s *ServerTestSuite) TestGetMergeTarget() {
	// Create a target bundle.
	created, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
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
// TEST UPDATE TARGET RESOURCE                                               //
// ========================================================================= //

func (s *ServerTestSuite) TestUpdateTargetResource() {
	var err error

	// Setup a merge with unresolved conflicts.
	created, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	s.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	s.True(ok)

	created2, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_unmarried_bundle.json")
	s.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	s.True(ok)

	// Make the merge request.
	source1 := s.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := s.FHIRServer.URL + "/Bundle/" + rightBundle.Id
	url := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	req, err := http.NewRequest("POST", url, nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusCreated, res.StatusCode)

	// Get a list of conflict resource IDs.
	decoder := json.NewDecoder(res.Body)
	ooBundle := &models.Bundle{}
	err = decoder.Decode(ooBundle)
	s.NoError(err)

	conflictResourceIDs := make([]string, len(ooBundle.Entry))
	for i, entry := range ooBundle.Entry {
		oo := entry.Resource.(*models.OperationOutcome)
		parts := strings.SplitN(oo.Issue[0].Diagnostics, ":", 2)
		conflictResourceIDs[i] = parts[1]
	}

	// Get the merge ID.
	mergeID := res.Header.Get("Location")
	s.NotEmpty(mergeID)

	// Get the target bundle.
	target, err := fhirutil.GetResourceByURL("Bundle", s.PTMergeServer.URL+"/merge/"+mergeID+"/target")
	s.NoError(err)
	targetBundle, ok := target.(*models.Bundle)
	s.True(ok)

	// Pick the first MedicationStatement resource to replace.
	var targetResourceID string
	for _, entry := range targetBundle.Entry {
		targetResourceType := fhirutil.GetResourceType(entry.Resource)
		targetResourceID = fhirutil.GetResourceID(entry.Resource)
		if targetResourceType == "MedicationStatement" {
			break
		}
	}
	s.NotEmpty(targetResourceID)
	s.False(contains(conflictResourceIDs, targetResourceID))

	// Post the MedicationStatement resource that updates it.
	medResource, err := fhirutil.LoadResource("MedicationStatement", "../fixtures/medication_statements/medication_statement_1.json")
	data, err := json.Marshal(medResource)
	s.NoError(err)
	s.NotEmpty(data)

	req, err = http.NewRequest("POST", s.PTMergeServer.URL+"/merge/"+mergeID+"/target/resources/"+targetResourceID, bytes.NewReader(data))
	s.NoError(err)
	res, err = http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()
	s.Equal(http.StatusOK, res.StatusCode)

	for _, entry := range targetBundle.Entry {
		if fhirutil.GetResourceID(entry.Resource) == targetResourceID {
			postedMed, ok := medResource.(*models.MedicationStatement)
			s.True(ok)
			targetMed, ok := entry.Resource.(*models.MedicationStatement)
			s.True(ok)
			s.Equal(postedMed.Status, targetMed.Status)
			s.Equal(postedMed.MedicationCodeableConcept.Coding[0].Code, targetMed.MedicationCodeableConcept.Coding[0].Code)
			break
		}
	}
}

func (s *ServerTestSuite) TestUpdateTargetResourceTargetNotFound() {
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
	targeResourceID := bson.NewObjectId().Hex()
	res, err := http.Post(s.PTMergeServer.URL+"/merge/"+m1.MergeID+"/target/resources/"+targeResourceID, "application/json", bytes.NewReader([]byte("{\"resourceType\":\"MedicationStatement\"}")))
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusInternalServerError, res.StatusCode)
	s.Equal(fmt.Sprintf("Resource %s not found", m1.TargetURL), string(body))
}

func (s *ServerTestSuite) TestUpdateTargetResourceMergeNotFound() {
	// Make the request.
	mergeID := bson.NewObjectId().Hex()
	res, err := http.Post(s.PTMergeServer.URL+"/merge/"+mergeID+"/target/resources/"+bson.NewObjectId().Hex(), "application/json", nil)
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusNotFound, res.StatusCode)
	s.Equal(fmt.Sprintf("Merge %s not found", mergeID), string(body))
}

// ========================================================================= //
// TEST DELETE TARGET RESOURCE                                               //
// ========================================================================= //

func (s *ServerTestSuite) TestDeleteTargetResource() {
	var err error

	// Setup a merge with unresolved conflicts.
	created, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_bundle.json")
	s.NoError(err)
	leftBundle, ok := created.(*models.Bundle)
	s.True(ok)

	created2, err := fhirutil.LoadAndPostResource(s.FHIRServer.URL, "Bundle", "../fixtures/bundles/lowell_abbott_unmarried_bundle.json")
	s.NoError(err)
	rightBundle, ok := created2.(*models.Bundle)
	s.True(ok)

	// Make the merge request.
	source1 := s.FHIRServer.URL + "/Bundle/" + leftBundle.Id
	source2 := s.FHIRServer.URL + "/Bundle/" + rightBundle.Id
	url := s.PTMergeServer.URL + "/merge?source1=" + url.QueryEscape(source1) + "&source2=" + url.QueryEscape(source2)

	req, err := http.NewRequest("POST", url, nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	s.Equal(http.StatusCreated, res.StatusCode)

	// Get a list of conflict resource IDs.
	decoder := json.NewDecoder(res.Body)
	ooBundle := &models.Bundle{}
	err = decoder.Decode(ooBundle)
	s.NoError(err)

	conflictResourceIDs := make([]string, len(ooBundle.Entry))
	for i, entry := range ooBundle.Entry {
		oo := entry.Resource.(*models.OperationOutcome)
		parts := strings.SplitN(oo.Issue[0].Diagnostics, ":", 2)
		conflictResourceIDs[i] = parts[1]
	}

	// Get the merge ID.
	mergeID := res.Header.Get("Location")
	s.NotEmpty(mergeID)

	// Get the target bundle.
	target, err := fhirutil.GetResourceByURL("Bundle", s.PTMergeServer.URL+"/merge/"+mergeID+"/target")
	s.NoError(err)
	targetBundle, ok := target.(*models.Bundle)
	s.True(ok)

	// Pick the first MedicationStatement resource to delete.
	var targetResourceID string
	for _, entry := range targetBundle.Entry {
		targetResourceType := fhirutil.GetResourceType(entry.Resource)
		targetResourceID = fhirutil.GetResourceID(entry.Resource)
		if targetResourceType == "MedicationStatement" {
			break
		}
	}
	s.NotEmpty(targetResourceID)
	s.False(contains(conflictResourceIDs, targetResourceID))

	// DELETE it.
	req, err = http.NewRequest("DELETE", s.PTMergeServer.URL+"/merge/"+mergeID+"/target/resources/"+targetResourceID, nil)
	s.NoError(err)
	res, err = http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()
	s.Equal(http.StatusNoContent, res.StatusCode)
}

func (s *ServerTestSuite) TestDeleteTargetResourceTargetNotFound() {
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
	req, err := http.NewRequest("DELETE", s.PTMergeServer.URL+"/merge/"+m1.MergeID+"/target/resources/"+bson.NewObjectId().Hex(), nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
	s.NoError(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	s.NoError(err)

	// Check the response.
	s.Equal(http.StatusInternalServerError, res.StatusCode)
	s.Equal(fmt.Sprintf("Resource %s not found", m1.TargetURL), string(body))
}

func (s *ServerTestSuite) TestDeleteTargetResourceMergeNotFound() {
	// Make the request.
	mergeID := bson.NewObjectId().Hex()
	req, err := http.NewRequest("DELETE", s.PTMergeServer.URL+"/merge/"+mergeID+"/target/resources/"+bson.NewObjectId().Hex(), nil)
	s.NoError(err)
	res, err := http.DefaultClient.Do(req)
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

// contains tests if an element is in the set.
func contains(set []string, el string) bool {
	for _, item := range set {
		if item == el {
			return true
		}
	}
	return false
}
