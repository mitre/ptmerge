package server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/merge"
	"github.com/mitre/ptmerge/state"
)

// MergeController manages the resource handlers for a Merge. MergeController
// is also responsible for maintaining the state of a Merge using the mongo
// database.
type MergeController struct {
	session  *mgo.Session
	dbname   string
	fhirHost string
}

// NewMergeController returns a pointer to a newly initialized MergeController.
func NewMergeController(session *mgo.Session, dbname string, fhirHost string) *MergeController {
	return &MergeController{
		session:  session,
		dbname:   dbname,
		fhirHost: fhirHost,
	}
}

// ========================================================================= //
// MERGE HANDLERS                                                            //
// ========================================================================= //

// Merge attempts to merge 2 FHIR bundles of patient resources given the URLs to both bundles.
func (m *MergeController) Merge(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	source1 := c.Query("source1")
	source2 := c.Query("source2")

	if source1 == "" || source2 == "" {
		c.String(http.StatusBadRequest, "URL(s) referencing source bundles were not provided")
		return
	}

	merger := merge.NewMerger(m.fhirHost)
	outcome, targetURL, err := merger.Merge(source1, source2)

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if targetURL == "" {
		// The merge had no conflicts, just return the merged bundle.
		c.JSON(http.StatusOK, outcome)
		return
	}

	// Check outcome to get a list of merge conflicts.
	conflictMap := make(state.ConflictMap)
	for _, entry := range outcome.Entry {
		if entryIsOperationOutcome(entry) {
			conflictID := getConflictID(entry)
			conflictMap[conflictID] = state.ConflictState{
				URL:      m.fhirHost + "/OperationOutcome/" + conflictID,
				Resolved: false,
			}
		}
	}

	// Some conflicts exist, create a new record in mongo to manage this merge's state.
	mergeID := bson.NewObjectId().Hex()
	err = worker.DB(m.dbname).C("merges").Insert(&state.MergeState{
		MergeID:   mergeID,
		Completed: false,
		TargetURL: targetURL,
		Conflicts: conflictMap,
	})

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Return the bundle of conflicts to resolve. The mergeID is passed in the Location header.
	c.Header("Location", mergeID)
	c.JSON(http.StatusCreated, outcome)
}

// Resolve attempts to resolve a single merge confict given the mergeID, conflictID,
// and the complete resource that resolve the conflict.
func (m *MergeController) Resolve(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")
	conflictID := c.Param("conflict_id")

	// Extract the resource from the request body.
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	reader := bytes.NewReader(body)
	decoder := json.NewDecoder(reader)

	// To determine the type of resource.
	var resource *models.Resource
	err = decoder.Decode(&resource)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	resourceType := resource.ResourceType

	// Now we can unmarshal the body into the proper resource struct.
	updatedResource := models.NewStructForResourceName(resourceType)
	reader.Reset(body) // Need to replenish the reader
	err = decoder.Decode(&updatedResource)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Check that the merge is incomplete.
	if mergeState.Completed {
		c.String(http.StatusBadRequest, "Merge %s is complete, no remaining conflicts to resolve", mergeID)
		return
	}

	// Check that the conflictID exists and is part of this merge.
	conflict, found := mergeState.Conflicts[conflictID]
	if !found {
		c.String(http.StatusNotFound, "Merge conflict %s not found for merge %s", conflictID, mergeID)
		return
	}

	// Check that the conflict wasn't already resolved.
	if conflict.Resolved {
		c.String(http.StatusBadRequest, "Merge conflict %s was already resolved for merge %s", conflictID, mergeID)
		return
	}

	merger := merge.NewMerger(m.fhirHost)
	outcome, err := merger.ResolveConflict(mergeState.TargetURL, conflict.URL, updatedResource)

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// If the outcome is a patient bundle (no OperationOutcomes), then the merge
	// is complete and we can delete the saved state of the merge, it's conflicts,
	// and the target. The call to Merger.ResolveConflict() already deleted those
	// resources on the host FHIR server.
	if !isOperationOutcomeBundle(outcome) {
		err = worker.DB(m.dbname).C("merges").Update(
			bson.M{"_id": mergeID},                    // query
			bson.M{"$set": bson.M{"completed": true}}, // update instruction
		)

		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}

		// TODO: Wipe the OperationOutcomes from the FHIR server?
	}

	// If the outcome is a bundle of OperationOutcomes describing the
	// remaining conflicts, update the latest conflict to "resolved".
	if isOperationOutcomeBundle(outcome) {
		// Set the conflict to "resolved"
		key := "conflicts." + conflictID + ".resolved"
		err = worker.DB(m.dbname).C("merges").Update(
			bson.M{"_id": mergeID},            // query
			bson.M{"$set": bson.M{key: true}}, // update instruction
		)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, outcome)
}

// Abort terminates an in-progress merge given the mergeID.
func (m *MergeController) Abort(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Extract an array of resource URLs to delete.
	URLs := make([]string, len(mergeState.Conflicts)+1)
	URLs[0] = mergeState.TargetURL
	for i, key := range mergeState.Conflicts.Keys() {
		URLs[i+1] = mergeState.Conflicts[key].URL
	}

	merger := merge.NewMerger(m.fhirHost)
	err = merger.Abort(URLs)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// All other Gin response handlers try to add a response body.
	// 204 responses explicity do not have response body.
	c.AbortWithStatus(204)
}

// ========================================================================= //
// CONVENIENCE HANDLERS                                                      //
// ========================================================================= //

// AllMerges returns the metadata for all merges we have a record of.
func (m *MergeController) AllMerges(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	// Get all merges from mongo.
	var merges []state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(nil).All(&merges)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Package up the merges metadata.
	meta := &state.Merges{
		Timestamp: time.Now(),
		Merges:    merges,
	}

	c.JSON(http.StatusOK, meta)
}

// GetMerge returns the metadata for a single merge.
func (m *MergeController) GetMerge(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Package up the merge metadata.
	meta := &state.Merge{
		Timestamp: time.Now(),
		Merge:     mergeState,
	}

	c.JSON(http.StatusOK, meta)
}

// GetRemainingConflicts returns all unresolved merge conflicts for a given mergeID.
func (m *MergeController) GetRemainingConflicts(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Extract the URLs to all unresolved conflicts.
	conflictURLs := []string{}
	for _, key := range mergeState.Conflicts.Keys() {
		if !mergeState.Conflicts[key].Resolved {
			conflictURLs = append(conflictURLs, mergeState.Conflicts[key].URL)
		}
	}

	// Get a bundle of OperationOutcome conflicts from the host FHIR server.
	merger := merge.NewMerger(m.fhirHost)
	conflicts, err := merger.GetConflicts(conflictURLs)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, conflicts)
}

// GetResolvedConflicts returns all resolved merge conflicts for a given mergeID.
func (m *MergeController) GetResolvedConflicts(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Extract the URLs to all resolved conflicts.
	resolvedURLs := []string{}
	for _, key := range mergeState.Conflicts.Keys() {
		if mergeState.Conflicts[key].Resolved {
			resolvedURLs = append(resolvedURLs, mergeState.Conflicts[key].URL)
		}
	}

	// Get a bundle of OperationOutcome conflicts from the host FHIR server.
	merger := merge.NewMerger(m.fhirHost)
	conflicts, err := merger.GetConflicts(resolvedURLs)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, conflicts)
}

// GetTarget returns the (partially complete) merge target given a mergeID.
func (m *MergeController) GetTarget(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var mergeState state.MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&mergeState)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Get the target from the host FHIR server.
	merger := merge.NewMerger(m.fhirHost)
	targetBundle, err := merger.GetTarget(mergeState.TargetURL)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, targetBundle)
}

// ========================================================================= //
// HELPER FUNCTIONS                                                          //
// ========================================================================= //

func isOperationOutcomeBundle(bundle *models.Bundle) bool {
	for _, entry := range bundle.Entry {
		if entryIsOperationOutcome(entry) {
			return true
		}
	}
	return false
}

func entryIsOperationOutcome(entry models.BundleEntryComponent) bool {
	_, ok := entry.Resource.(*models.OperationOutcome)
	return ok
}

func getConflictID(entry models.BundleEntryComponent) string {
	return entry.Resource.(*models.OperationOutcome).Id
}
