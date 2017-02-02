package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/gin-gonic/gin"
	"github.com/intervention-engine/fhir/models"
	"github.com/mitre/ptmerge/merge"
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

// Merges 2 FHIR bundles of patient resources given the URLs to both bundles.
func (m *MergeController) merge(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	source1 := c.Query("source1")
	source2 := c.Query("source2")

	if source1 == "" || source2 == "" {
		c.String(http.StatusBadRequest, "URL(s) referencing bundles to merge were not provided")
		return
	}

	merger := merge.NewMerger(m.fhirHost)
	targetBundle, outcome, err := merger.Merge(source1, source2)

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	if targetBundle == "" {
		// The merge had no conflicts, just return the merged bundle.
		c.JSON(http.StatusOK, outcome)
		return
	}

	// Check outcome to get a list of merge conflicts.
	conflictMap := make(ConflictMap)
	for _, entry := range outcome.Entry {
		if entryIsOperationOutcome(entry) {
			conflictID := getConflictID(entry)
			conflictMap[conflictID] = m.fhirHost + "/OperationOutcome/" + conflictID
		}
	}

	// Some conflicts exist, create a new record in mongo to manage this merge's state.
	mergeID := bson.NewObjectId().Hex()
	err = worker.DB(m.dbname).C("merges").Insert(&MergeState{
		MergeID:      mergeID,
		TargetBundle: targetBundle,
		Conflicts:    conflictMap,
	})

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Return the bundle of conflicts to resolve. The mergeID is passed in the Location header.
	c.Header("Location", mergeID)
	c.JSON(http.StatusCreated, outcome)
}

// Resolves a single merge confict given the OperationOutcome ID and the correct resource
// that resolves the merge.
func (m *MergeController) resolve(c *gin.Context) {
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
	var state MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&state)
	if err != nil {
		if err == mgo.ErrNotFound {
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Check that the conflictID exists and is part of this merge.
	conflict, found := state.Conflicts[conflictID]
	if !found {
		c.String(http.StatusNotFound, "Merge conflict %s not found for merge %s", conflictID, mergeID)
		return
	}

	merger := merge.NewMerger(m.fhirHost)
	outcome, err := merger.ResolveConflict(state.TargetBundle, conflict, updatedResource)

	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// If the outcome is a patient bundle (no OperationOutcomes), then the merge
	// is complete and we can delete the saved state of the merge, it's conflicts,
	// and the target. The call to Merger.ResolveConflict() already deleted those
	// resources on the host FHIR server.
	if !isOperationOutcomeBundle(outcome) {
		err = worker.DB(m.dbname).C("merges").RemoveId(state.MergeID)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}

	// The outcome is still a bundle of OperationOutcomes describing the
	// remaining conflicts. To check that the current conflict was actually
	// resolved, make sure it's not in this new outcome bundle.
	if mergeConflictWasResolved(outcome, conflictID) {
		// Remove that conflictID from the saved state
		err = worker.DB(m.dbname).C("merges").Update(
			bson.M{"_id": mergeID},                                    // query
			bson.M{"$unset": bson.M{("conflicts." + conflictID): ""}}, // update instruction
		)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
	}

	c.JSON(http.StatusOK, outcome)
}

// Aborts a merge given the merge's ID.
func (m *MergeController) abort(c *gin.Context) {
	var err error
	worker := m.session.Copy()
	defer worker.Close()

	mergeID := c.Param("merge_id")

	// Get the merge state from mongo.
	var state MergeState
	err = worker.DB(m.dbname).C("merges").Find(bson.M{"_id": mergeID}).One(&state)
	if err != nil {
		if err == mgo.ErrNotFound {
			fmt.Printf("Got here\n")
			c.String(http.StatusNotFound, "Merge %s not found", mergeID)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// Extract an array of resource URIs to delete.
	uris := make([]string, len(state.Conflicts)+1)
	uris[0] = state.TargetBundle
	for i, key := range state.Conflicts.Keys() {
		uris[i+1] = state.Conflicts[key]
	}

	merger := merge.NewMerger(m.fhirHost)
	err = merger.Abort(uris)
	if err != nil {
		c.String(http.StatusInternalServerError, err.Error())
		return
	}

	// All other Gin response handlers try to add a response body.
	// 204 responses explicity do not have response body.
	c.AbortWithStatus(204)
}

// Returns all unresolved merge conflicts for a given merge ID.
func (m *MergeController) getConflicts(c *gin.Context) {
	mergeID := c.Param("merge_id")
	c.String(http.StatusOK, "Merge conflicts for merge %s", mergeID)
}

func mergeConflictWasResolved(ooBundle *models.Bundle, conflictID string) bool {
	if !isOperationOutcomeBundle(ooBundle) {
		return false
	}

	resolved := true
	for _, entry := range ooBundle.Entry {
		if entryIsOperationOutcome(entry) {
			if getConflictID(entry) == conflictID {
				resolved = false
			}
		}
	}
	return resolved
}

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
